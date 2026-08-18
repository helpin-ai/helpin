package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const calendarMeetingHorizon = 30 * 24 * time.Hour

// SetCalendarIntegration enables upcoming-event capture configuration.
func (s *CRMMeetingService) SetCalendarIntegration(
	calendarRepo *repository.CRMCalendarRepository,
	emailRepo *repository.CRMEmailRepository,
) *CRMMeetingService {
	if s != nil {
		s.calendarRepo = calendarRepo
		s.emailRepo = emailRepo
	}
	return s
}

// ListUpcomingCalendarMeetings returns synced calendar events and their capture state.
func (s *CRMMeetingService) ListUpcomingCalendarMeetings(
	ctx context.Context,
	workspaceID string,
	now time.Time,
) ([]model.CRMCalendarMeetingCandidate, error) {
	if s == nil || s.calendarRepo == nil {
		return []model.CRMCalendarMeetingCandidate{}, nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	now = now.UTC()
	status := model.CRMCalendarEventStatusConfirmed
	events, _, err := s.calendarRepo.List(ctx, workspaceID, model.CRMCalendarEventListFilters{
		StartAfter:  &now,
		StartBefore: calendarTimePointer(now.Add(calendarMeetingHorizon)),
		Status:      &status,
		Ascending:   true,
	}, model.PMPagination{Page: 1, PerPage: 200})
	if err != nil {
		return nil, err
	}
	eventIDs := make([]string, 0, len(events))
	for _, event := range events {
		eventIDs = append(eventIDs, event.ID)
	}
	meetings, err := s.repo.ListByCalendarEventIDs(ctx, workspaceID, eventIDs)
	if err != nil {
		return nil, err
	}
	settings, err := s.repo.GetSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	preferences, err := s.calendarRepo.ListSeriesPreferences(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	preferencesBySeries := make(map[string]*model.CRMCalendarSeriesPreference, len(preferences))
	for index := range preferences {
		preference := &preferences[index]
		preferencesBySeries[calendarSeriesPreferenceKey(preference.EmailAccountID, preference.SeriesExternalID)] = preference
	}
	candidates := make([]model.CRMCalendarMeetingCandidate, 0, len(events))
	for _, event := range events {
		var meeting *model.CRMMeeting
		if meetingValue, exists := meetings[event.ID]; exists {
			meetingCopy := meetingValue
			meeting = &meetingCopy
		}
		preference := preferencesBySeries[calendarSeriesPreferenceKey(event.EmailAccountID, calendarStringValue(event.RecurringSeriesID))]
		candidate := calendarMeetingCandidate(event, settings, preference, meeting)
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

// UpdateCalendarMeetingCapture enables or disables durable automatic joining for one synced event.
func (s *CRMMeetingService) updateCalendarMeetingCaptureLocked(
	ctx context.Context,
	workspaceID, calendarEventID, actorID string,
	req model.UpdateCRMCalendarMeetingCaptureRequest,
) (*model.CRMCalendarMeetingCandidate, error) {
	if s == nil || s.calendarRepo == nil {
		return nil, fmt.Errorf("calendar meeting integration is unavailable")
	}
	event, err := s.calendarRepo.GetByID(ctx, workspaceID, calendarEventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, fmt.Errorf("calendar event not found")
	}
	existing, err := s.repo.GetByCalendarEventID(ctx, workspaceID, event.ID)
	if err != nil {
		return nil, err
	}
	settings, err := s.repo.GetSettings(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	var preference *model.CRMCalendarSeriesPreference
	if event.RecurringSeriesID != nil {
		preference, err = s.calendarRepo.GetSeriesPreference(ctx, workspaceID, event.EmailAccountID, *event.RecurringSeriesID)
		if err != nil {
			return nil, err
		}
	}
	if !req.Enabled {
		if existing != nil {
			if s.captureScheduler == nil {
				return nil, fmt.Errorf("automatic meeting joining is unavailable")
			}
			if existing.Status != model.CRMMeetingStatusScheduled && existing.Status != model.CRMMeetingStatusFailed {
				return nil, fmt.Errorf("capture cannot be disabled after the meeting has started")
			}
			if err := s.captureScheduler.CancelMeetingCapture(ctx, existing.ID); err != nil {
				return nil, err
			}
			if err := s.Delete(ctx, workspaceID, existing.ID); err != nil {
				return nil, err
			}
		}
		candidate := calendarMeetingCandidate(*event, settings, preference, nil)
		return &candidate, nil
	}

	if s.captureScheduler == nil {
		return nil, fmt.Errorf("automatic meeting joining is unavailable")
	}
	candidate := calendarMeetingCandidate(*event, settings, preference, existing)
	if !candidate.Eligible {
		return nil, fmt.Errorf("%s", calendarStringValue(candidate.IneligibilityReason))
	}
	if req.DealID != nil {
		event.DealID = trimStringPtr(req.DealID)
		if err := s.calendarRepo.Update(ctx, event); err != nil {
			return nil, err
		}
	}
	recordAudio := settings.RecordAudioByDefault
	if req.RecordAudio != nil {
		recordAudio = *req.RecordAudio
	}
	ownerMemberID, err := s.calendarOwnerMemberID(ctx, event)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		detail, createErr := s.Create(ctx, model.CreateCRMMeetingRequest{
			WorkspaceID:      workspaceID,
			Title:            event.Title,
			MeetingURL:       calendarStringValue(event.MeetingURL),
			CalendarEventID:  &event.ID,
			OwnerMemberID:    ownerMemberID,
			ScheduledStartAt: &event.StartTime,
			ScheduledEndAt:   &event.EndTime,
			RecordAudio:      &recordAudio,
			StartNow:         false,
		}, actorID, "")
		if createErr != nil {
			return nil, createErr
		}
		existing = &detail.Meeting
	} else if existing.Status == model.CRMMeetingStatusScheduled || existing.Status == model.CRMMeetingStatusFailed {
		existing.Title = event.Title
		existing.MeetingURL = calendarStringValue(event.MeetingURL)
		existing.ScheduledStartAt = &event.StartTime
		existing.ScheduledEndAt = &event.EndTime
		existing.OwnerMemberID = ownerMemberID
		existing.RecordAudio = recordAudio
		existing.Status = model.CRMMeetingStatusScheduled
		existing.FailureCode = nil
		existing.FailureMessage = nil
		platform, nativeID, parseErr := ParseMeetingURL(existing.MeetingURL)
		if parseErr != nil {
			return nil, parseErr
		}
		existing.Platform = platform
		existing.NativeMeetingID = nativeID
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
	}
	if err := s.associateCalendarMeeting(ctx, existing, event); err != nil {
		return nil, err
	}
	if err := s.captureScheduler.ScheduleMeetingCapture(ctx, workspaceID, existing.ID, event.StartTime); err != nil {
		message := "Helpin could not schedule automatic joining"
		code := calendarAutoFailureCode
		existing.Status = model.CRMMeetingStatusFailed
		existing.FailureCode = &code
		existing.FailureMessage = &message
		if updateErr := s.repo.Update(ctx, existing); updateErr != nil {
			return nil, updateErr
		}
		return nil, err
	}
	candidate.Meeting = existing
	return &candidate, nil
}

func calendarMeetingCandidate(
	event model.CRMCalendarEvent,
	settings *model.CRMMeetingSettings,
	preference *model.CRMCalendarSeriesPreference,
	meeting *model.CRMMeeting,
) model.CRMCalendarMeetingCandidate {
	candidate := model.CRMCalendarMeetingCandidate{
		Event:          event,
		Meeting:        meeting,
		Eligible:       true,
		AutoJoinSource: "workspace",
	}
	reason := ""
	switch {
	case settings == nil || !settings.Enabled:
		reason = "Meeting notes are turned off"
	case event.Status == model.CRMCalendarEventStatusCancelled:
		reason = "This event was cancelled"
	case event.AllDay:
		reason = "All-day events cannot be captured"
	case event.MeetingURL == nil || strings.TrimSpace(*event.MeetingURL) == "":
		reason = "No supported meeting link"
	default:
		if _, _, err := ParseMeetingURL(*event.MeetingURL); err != nil {
			reason = err.Error()
		}
	}
	if reason != "" {
		candidate.Eligible = false
		candidate.IneligibilityReason = &reason
	}

	if preference != nil {
		seriesAutoJoin := preference.AutoJoin
		candidate.SeriesAutoJoin = &seriesAutoJoin
	}
	switch {
	case event.AutoJoinOverride != nil:
		candidate.EffectiveAutoJoin = *event.AutoJoinOverride
		candidate.AutoJoinSource = "occurrence"
	case preference != nil:
		candidate.EffectiveAutoJoin = preference.AutoJoin
		candidate.AutoJoinSource = "series"
	case settings != nil && settings.AutoJoinMode == "all":
		candidate.EffectiveAutoJoin = true
	case settings != nil && settings.AutoJoinMode == "external":
		candidate.EffectiveAutoJoin = len(event.ContactIDs) > 0
	}
	if !candidate.Eligible {
		candidate.EffectiveAutoJoin = false
	}
	return candidate
}

func calendarSeriesPreferenceKey(emailAccountID, seriesExternalID string) string {
	return strings.TrimSpace(emailAccountID) + ":" + strings.TrimSpace(seriesExternalID)
}

func (s *CRMMeetingService) calendarOwnerMemberID(ctx context.Context, event *model.CRMCalendarEvent) (*string, error) {
	if s.emailRepo == nil || event == nil || strings.TrimSpace(event.EmailAccountID) == "" {
		return nil, nil
	}
	account, err := s.emailRepo.GetAccountByID(ctx, event.EmailAccountID)
	if err != nil {
		return nil, err
	}
	if account == nil || account.WorkspaceID != event.WorkspaceID {
		return nil, nil
	}
	return trimStringPtr(&account.MemberID), nil
}

func (s *CRMMeetingService) associateCalendarMeeting(ctx context.Context, meeting *model.CRMMeeting, event *model.CRMCalendarEvent) error {
	if s.associationRepo == nil || meeting == nil || event == nil {
		return nil
	}
	companyIDs := make(map[string]struct{})
	for _, contactID := range event.ContactIDs {
		if err := s.createMeetingAssociation(ctx, meeting.WorkspaceID, meeting.ID, model.CRMObjectContact, contactID); err != nil {
			return err
		}
		contactAssociations, err := s.associationRepo.ListByObject(ctx, meeting.WorkspaceID, model.CRMObjectContact, contactID)
		if err != nil {
			return err
		}
		for _, association := range contactAssociations {
			objectType, objectID := otherAssociationSide(association, model.CRMObjectContact, contactID)
			if objectType == model.CRMObjectCompany && objectID != "" {
				companyIDs[objectID] = struct{}{}
			}
		}
	}
	for companyID := range companyIDs {
		if err := s.createMeetingAssociation(ctx, meeting.WorkspaceID, meeting.ID, model.CRMObjectCompany, companyID); err != nil {
			return err
		}
	}
	if event.DealID != nil && strings.TrimSpace(*event.DealID) != "" {
		if err := s.createMeetingAssociation(ctx, meeting.WorkspaceID, meeting.ID, model.CRMObjectDeal, *event.DealID); err != nil {
			return err
		}
	}
	return nil
}

func (s *CRMMeetingService) createMeetingAssociation(ctx context.Context, workspaceID, meetingID, objectType, objectID string) error {
	objectID = strings.TrimSpace(objectID)
	if objectID == "" {
		return nil
	}
	return s.associationRepo.Create(ctx, &model.CRMAssociation{
		WorkspaceID:    workspaceID,
		FromObjectType: model.CRMObjectMeeting,
		FromObjectID:   meetingID,
		ToObjectType:   objectType,
		ToObjectID:     objectID,
	})
}

func calendarTimePointer(value time.Time) *time.Time { return &value }

func calendarStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
