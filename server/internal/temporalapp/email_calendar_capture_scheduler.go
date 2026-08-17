package temporalapp

import (
	"context"
	"time"
)

type calendarMeetingCaptureScheduler interface {
	ScheduleMeetingCapture(ctx context.Context, workspaceID, meetingID string, scheduledStart time.Time) error
	CancelMeetingCapture(ctx context.Context, meetingID string) error
}

type calendarMeetingPolicyReconciler interface {
	ReconcileCalendarMeetingPolicy(ctx context.Context, workspaceID, calendarEventID, actorID string) error
}

// SetMeetingPolicyReconciler materializes occurrence, series, and workspace
// auto-join policy when new calendar occurrences enter the sync horizon.
func (a *EmailSyncActivities) SetMeetingPolicyReconciler(reconciler calendarMeetingPolicyReconciler) *EmailSyncActivities {
	if a != nil {
		a.meetingPolicyReconciler = reconciler
	}
	return a
}

// SetMeetingCaptureScheduler keeps durable automatic joins aligned with synced
// Google Calendar changes.
func (a *EmailSyncActivities) SetMeetingCaptureScheduler(scheduler calendarMeetingCaptureScheduler) *EmailSyncActivities {
	if a != nil {
		a.meetingCaptureScheduler = scheduler
	}
	return a
}

func (a *EmailSyncActivities) rescheduleCalendarMeetingCapture(
	ctx context.Context,
	workspaceID, meetingID string,
	scheduledStart time.Time,
) error {
	if a.meetingCaptureScheduler == nil {
		return nil
	}
	return a.meetingCaptureScheduler.ScheduleMeetingCapture(ctx, workspaceID, meetingID, scheduledStart)
}

func (a *EmailSyncActivities) cancelCalendarMeetingCapture(ctx context.Context, meetingID string) error {
	if a.meetingCaptureScheduler == nil {
		return nil
	}
	return a.meetingCaptureScheduler.CancelMeetingCapture(ctx, meetingID)
}
