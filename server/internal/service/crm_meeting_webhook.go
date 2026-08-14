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
		return nil
	}
	applyMeetingProviderEvent(meeting, capture, normalized)
	if err := s.repo.UpdateCapture(ctx, capture); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, meeting); err != nil {
		return err
	}
	if normalized.TranscriptReady {
		if s.processing == nil {
			return fmt.Errorf("meeting processing is unavailable")
		}
		if err := s.processing.StartMeetingProcessing(ctx, capture.WorkspaceID, capture.MeetingID); err != nil {
			return err
		}
	}
	return s.repo.MarkProviderEventProcessed(ctx, event.ID)
}

func applyMeetingProviderEvent(meeting *model.CRMMeeting, capture *model.CRMMeetingCapture, event *meetingcapture.ProviderEvent) {
	if strings.TrimSpace(event.ProviderStatus) != "" {
		capture.ProviderStatus = event.ProviderStatus
	}
	if strings.TrimSpace(event.Status) != "" {
		capture.Status = event.Status
		meeting.Status = event.Status
	}
	if strings.TrimSpace(event.ProviderRecordingID) != "" {
		capture.ProviderRecordingID = trimStringPtr(&event.ProviderRecordingID)
	}
	if strings.TrimSpace(event.ProviderTranscriptID) != "" {
		capture.ProviderTranscriptID = trimStringPtr(&event.ProviderTranscriptID)
	}
	if strings.TrimSpace(event.FailureCode) != "" {
		capture.FailureCode = trimStringPtr(&event.FailureCode)
		meeting.FailureCode = trimStringPtr(&event.FailureCode)
	}
	if strings.TrimSpace(event.FailureMessage) != "" {
		capture.FailureMessage = trimStringPtr(&event.FailureMessage)
		meeting.FailureMessage = trimStringPtr(&event.FailureMessage)
	}
	transitionAt := meetingLifecycleTime(event.OccurredAt)
	if event.Status == model.CRMMeetingStatusRecording {
		if capture.StartedAt == nil {
			capture.StartedAt = &transitionAt
		}
		if meeting.ActualStartAt == nil {
			meeting.ActualStartAt = &transitionAt
		}
	}
	if meetingLifecycleEnded(event.Status) {
		capture.EndedAt = &transitionAt
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
