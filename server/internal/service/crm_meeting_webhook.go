package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/meetingcapture"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// HandleWebhook authenticates and applies one provider lifecycle event.
func (s *CRMMeetingService) HandleWebhook(
	ctx context.Context,
	providerName string,
	headers http.Header,
	payload []byte,
) error {
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	provider := s.providers[providerName]
	if provider == nil || !provider.Configured() {
		return fmt.Errorf("%s meeting provider is not configured", providerName)
	}
	if err := provider.VerifyWebhook(headers, payload); err != nil {
		return err
	}
	normalized, err := provider.NormalizeWebhook(headers, payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(normalized.ProviderCaptureID) == "" || strings.TrimSpace(normalized.ProviderCaptureID) == ":" {
		return fmt.Errorf("meeting webhook is missing a provider capture id")
	}
	capture, err := s.repo.GetCaptureByProviderID(ctx, providerName, normalized.ProviderCaptureID)
	if err != nil {
		return err
	}
	if capture == nil {
		return fmt.Errorf("meeting capture not found")
	}
	meeting, err := s.repo.GetByID(ctx, capture.WorkspaceID, capture.MeetingID)
	if err != nil {
		return err
	}
	if meeting == nil {
		return fmt.Errorf("meeting not found")
	}
	event := &model.CRMMeetingProviderEvent{
		WorkspaceID: capture.WorkspaceID,
		Provider:    providerName,
		EventID:     normalized.EventID,
		EventType:   normalized.EventType,
		MeetingID:   &capture.MeetingID,
		CaptureID:   &capture.ID,
		Payload:     append(model.JSONBlob(nil), payload...),
	}
	created, err := s.repo.CreateProviderEvent(ctx, event)
	if err != nil {
		return err
	}
	if !created {
		existing, lookupErr := s.repo.GetProviderEvent(ctx, providerName, normalized.EventID)
		if lookupErr != nil {
			return lookupErr
		}
		if existing == nil {
			return fmt.Errorf("meeting provider event replay could not be loaded")
		}
		if existing.ProcessedAt != nil {
			return nil
		}
		event = existing
	}
	latestCapture, err := s.repo.GetLatestCapture(ctx, capture.WorkspaceID, capture.MeetingID)
	if err != nil {
		return err
	}
	isLatestCapture := latestCapture != nil && latestCapture.ID == capture.ID
	applyMeetingCaptureEvent(capture, normalized)
	if isLatestCapture {
		applyMeetingProviderEvent(meeting, normalized)
	}
	if err := s.repo.UpdateCapture(ctx, capture); err != nil {
		return err
	}
	if isLatestCapture {
		if err := s.repo.Update(ctx, meeting); err != nil {
			return err
		}
	}
	if normalized.TranscriptReady && isLatestCapture {
		if s.processing == nil {
			return fmt.Errorf("meeting processing is unavailable")
		}
		if err := s.processing.StartMeetingProcessing(ctx, capture.WorkspaceID, capture.MeetingID); err != nil {
			return err
		}
	}
	return s.repo.MarkProviderEventProcessed(ctx, event.ID)
}

func applyMeetingCaptureEvent(capture *model.CRMMeetingCapture, event *meetingcapture.ProviderEvent) {
	if strings.TrimSpace(event.ProviderStatus) != "" {
		capture.ProviderStatus = event.ProviderStatus
	}
	if strings.TrimSpace(event.Status) != "" && meetingStatusCanTransition(capture.Status, event.Status) {
		capture.Status = event.Status
	}
	if strings.TrimSpace(event.ProviderRecordingID) != "" {
		capture.ProviderRecordingID = trimStringPtr(&event.ProviderRecordingID)
	}
	if strings.TrimSpace(event.ProviderTranscriptID) != "" {
		capture.ProviderTranscriptID = trimStringPtr(&event.ProviderTranscriptID)
	}
	if strings.TrimSpace(event.FailureCode) != "" {
		capture.FailureCode = trimStringPtr(&event.FailureCode)
	}
	if strings.TrimSpace(event.FailureMessage) != "" {
		capture.FailureMessage = trimStringPtr(&event.FailureMessage)
	}
	transitionAt := meetingLifecycleTime(event.OccurredAt)
	if event.Status == model.CRMMeetingStatusRecording && capture.StartedAt == nil {
		capture.StartedAt = &transitionAt
	}
	if meetingLifecycleEnded(event.Status) {
		capture.EndedAt = &transitionAt
	}
}

func applyMeetingProviderEvent(meeting *model.CRMMeeting, event *meetingcapture.ProviderEvent) {
	if !meetingStatusCanTransition(meeting.Status, event.Status) {
		return
	}
	if strings.TrimSpace(event.Status) != "" {
		meeting.Status = event.Status
	}
	if strings.TrimSpace(event.FailureCode) != "" {
		meeting.FailureCode = trimStringPtr(&event.FailureCode)
	}
	if strings.TrimSpace(event.FailureMessage) != "" {
		meeting.FailureMessage = trimStringPtr(&event.FailureMessage)
	}
	transitionAt := meetingLifecycleTime(event.OccurredAt)
	if event.Status == model.CRMMeetingStatusRecording && meeting.ActualStartAt == nil {
		meeting.ActualStartAt = &transitionAt
	}
	if meetingLifecycleEnded(event.Status) {
		meeting.ActualEndAt = &transitionAt
		if meeting.ActualStartAt != nil && transitionAt.After(*meeting.ActualStartAt) {
			meeting.DurationSeconds = int(transitionAt.Sub(*meeting.ActualStartAt).Seconds())
		}
	}
	if event.TranscriptReady {
		meeting.Status = model.CRMMeetingStatusProcessing
		meeting.SummaryStatus = model.CRMMeetingSummaryProcessing
	} else if event.Status == model.CRMMeetingStatusFailed {
		meeting.SummaryStatus = model.CRMMeetingSummaryFailed
	}
}

func meetingStatusCanTransition(current, next string) bool {
	if strings.TrimSpace(next) == "" || current == next {
		return true
	}
	if current == model.CRMMeetingStatusReady {
		return false
	}
	if current == model.CRMMeetingStatusFailed || current == model.CRMMeetingStatusCancelled {
		return next == model.CRMMeetingStatusJoining || next == model.CRMMeetingStatusWaiting || next == model.CRMMeetingStatusRecording ||
			next == model.CRMMeetingStatusProcessing
	}
	rank := map[string]int{
		model.CRMMeetingStatusScheduled:  0,
		model.CRMMeetingStatusJoining:    1,
		model.CRMMeetingStatusWaiting:    2,
		model.CRMMeetingStatusRecording:  3,
		model.CRMMeetingStatusFinalizing: 4,
		model.CRMMeetingStatusProcessing: 5,
		model.CRMMeetingStatusReady:      6,
	}
	if next == model.CRMMeetingStatusFailed || next == model.CRMMeetingStatusCancelled {
		return true
	}
	currentRank, currentKnown := rank[current]
	nextRank, nextKnown := rank[next]
	if !currentKnown || !nextKnown {
		return false
	}
	return nextRank >= currentRank
}

func meetingLifecycleTime(occurredAt *time.Time) time.Time {
	if occurredAt != nil && !occurredAt.IsZero() {
		return occurredAt.UTC()
	}
	return time.Now().UTC()
}

func meetingLifecycleEnded(status string) bool {
	switch status {
	case model.CRMMeetingStatusFinalizing, model.CRMMeetingStatusProcessing, model.CRMMeetingStatusReady,
		model.CRMMeetingStatusFailed, model.CRMMeetingStatusCancelled:
		return true
	default:
		return false
	}
}
