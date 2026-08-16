package service

import (
	"context"
	"time"
)

type fakeMeetingCaptureScheduler struct {
	scheduledMeetingID string
	scheduledStart     time.Time
	cancelledMeetingID string
	err                error
}

func (f *fakeMeetingCaptureScheduler) ScheduleMeetingCapture(
	_ context.Context,
	_ string,
	meetingID string,
	scheduledStart time.Time,
) error {
	f.scheduledMeetingID = meetingID
	f.scheduledStart = scheduledStart
	return f.err
}

func (f *fakeMeetingCaptureScheduler) CancelMeetingCapture(_ context.Context, meetingID string) error {
	f.cancelledMeetingID = meetingID
	return f.err
}
