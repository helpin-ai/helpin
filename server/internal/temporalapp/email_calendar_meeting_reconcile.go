package temporalapp

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/meetingcapture"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const calendarMeetingLinkInvalidCode = "calendar_meeting_link_invalid"

// SetMeetingRepository enables calendar reschedule and cancellation projection
// into meetings that have not started yet.
func (a *EmailSyncActivities) SetMeetingRepository(repo *repository.CRMMeetingRepository) *EmailSyncActivities {
	if a != nil {
		a.meetingRepo = repo
	}
	return a
}

func (a *EmailSyncActivities) reconcileScheduledCalendarMeeting(ctx context.Context, event *model.CRMCalendarEvent) error {
	if a == nil || a.meetingRepo == nil || event == nil {
		return nil
	}
	meeting, err := a.meetingRepo.GetByCalendarEventID(ctx, event.WorkspaceID, event.ID)
	if err != nil || meeting == nil {
		return err
	}
	if meeting.ActualStartAt != nil || !calendarMeetingCanBeReconciled(meeting) {
		return nil
	}

	meeting.Title = event.Title
	meeting.ScheduledStartAt = &event.StartTime
	meeting.ScheduledEndAt = &event.EndTime
	if event.Status == model.CRMCalendarEventStatusCancelled {
		meeting.Status = model.CRMMeetingStatusCancelled
		meeting.FailureCode = nil
		meeting.FailureMessage = nil
		if err := a.meetingRepo.Update(ctx, meeting); err != nil {
			return err
		}
		return a.cancelCalendarMeetingCapture(ctx, meeting.ID)
	}

	meetingURL := calendarStringValue(event.MeetingURL)
	platform, nativeID, parseErr := meetingcapture.ParseMeetingURL(meetingURL)
	if parseErr != nil {
		message := "The calendar event no longer has a supported meeting link"
		code := calendarMeetingLinkInvalidCode
		meeting.Status = model.CRMMeetingStatusFailed
		meeting.FailureCode = &code
		meeting.FailureMessage = &message
		if err := a.meetingRepo.Update(ctx, meeting); err != nil {
			return err
		}
		return a.cancelCalendarMeetingCapture(ctx, meeting.ID)
	}
	meeting.MeetingURL = meetingURL
	meeting.Platform = platform
	meeting.NativeMeetingID = nativeID
	meeting.Status = model.CRMMeetingStatusScheduled
	meeting.FailureCode = nil
	meeting.FailureMessage = nil
	if err := a.meetingRepo.Update(ctx, meeting); err != nil {
		return err
	}
	return a.rescheduleCalendarMeetingCapture(ctx, meeting.WorkspaceID, meeting.ID, event.StartTime)
}

func calendarMeetingCanBeReconciled(meeting *model.CRMMeeting) bool {
	if meeting == nil {
		return false
	}
	switch meeting.Status {
	case model.CRMMeetingStatusScheduled, model.CRMMeetingStatusCancelled:
		return true
	case model.CRMMeetingStatusFailed:
		return meeting.FailureCode != nil && strings.TrimSpace(*meeting.FailureCode) == calendarMeetingLinkInvalidCode
	default:
		return false
	}
}

func calendarStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
