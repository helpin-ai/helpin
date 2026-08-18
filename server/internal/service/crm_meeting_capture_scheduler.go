package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

const (
	calendarAutoJoinLeadTime   = 30 * time.Second
	calendarAutoCaptureKeyBase = "calendar-auto:"
	calendarAutoFailureCode    = "calendar_auto_capture_failed"
)

type meetingCaptureScheduler interface {
	ScheduleMeetingCapture(ctx context.Context, workspaceID, meetingID string, scheduledStart time.Time) error
	CancelMeetingCapture(ctx context.Context, meetingID string) error
}

type temporalMeetingCaptureScheduler struct {
	client tclient.Client
}

// NewTemporalMeetingCaptureScheduler creates the durable automatic join scheduler.
func NewTemporalMeetingCaptureScheduler(client tclient.Client) meetingCaptureScheduler {
	return &temporalMeetingCaptureScheduler{client: client}
}

// SetCaptureScheduler injects durable automatic calendar joining.
func (s *CRMMeetingService) SetCaptureScheduler(scheduler meetingCaptureScheduler) *CRMMeetingService {
	if s != nil {
		s.captureScheduler = scheduler
	}
	return s
}

func (s *temporalMeetingCaptureScheduler) ScheduleMeetingCapture(
	ctx context.Context,
	workspaceID, meetingID string,
	scheduledStart time.Time,
) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("automatic meeting joining is unavailable")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	meetingID = strings.TrimSpace(meetingID)
	if workspaceID == "" || meetingID == "" || scheduledStart.IsZero() {
		return fmt.Errorf("workspace_id, meeting_id, and scheduled_start are required")
	}
	input := temporalapp.CRMMeetingCaptureScheduleInput{
		WorkspaceID: workspaceID,
		MeetingID:   meetingID,
		JoinAt:      scheduledStart.UTC().Add(-calendarAutoJoinLeadTime),
	}
	_, err := s.client.SignalWithStartWorkflow(
		ctx,
		workflowIDForCRMMeetingCapture(meetingID),
		temporalapp.CRMMeetingCaptureScheduleSignal,
		temporalapp.CRMMeetingCaptureScheduleCommand{
			WorkspaceID: input.WorkspaceID,
			MeetingID:   input.MeetingID,
			JoinAt:      input.JoinAt,
		},
		tclient.StartWorkflowOptions{
			ID:                    workflowIDForCRMMeetingCapture(meetingID),
			TaskQueue:             temporalapp.QueueAutomation,
			WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		},
		temporalapp.CRMMeetingCaptureScheduleWorkflow,
		input,
	)
	if err != nil {
		return fmt.Errorf("schedule automatic meeting join: %w", err)
	}
	return nil
}

func (s *temporalMeetingCaptureScheduler) CancelMeetingCapture(ctx context.Context, meetingID string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("automatic meeting joining is unavailable")
	}
	meetingID = strings.TrimSpace(meetingID)
	if meetingID == "" {
		return fmt.Errorf("meeting_id is required")
	}
	err := s.client.SignalWorkflow(
		ctx,
		workflowIDForCRMMeetingCapture(meetingID),
		"",
		temporalapp.CRMMeetingCaptureScheduleSignal,
		temporalapp.CRMMeetingCaptureScheduleCommand{MeetingID: meetingID, Cancel: true},
	)
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cancel automatic meeting join: %w", err)
	}
	return nil
}

func workflowIDForCRMMeetingCapture(meetingID string) string {
	return "crm-meeting-auto-capture-" + strings.TrimSpace(meetingID)
}

// StartScheduledMeetingCapture serializes with calendar opt-out, then revalidates
// the meeting immediately before the provider launch.
func (s *CRMMeetingService) StartScheduledMeetingCapture(ctx context.Context, workspaceID, meetingID string) error {
	meeting, err := s.repo.GetByID(ctx, workspaceID, meetingID)
	if err != nil || meeting == nil || meeting.CalendarEventID == nil {
		return err
	}
	launchErr := s.repo.WithCaptureLaunchLock(ctx, workspaceID, "calendar-event:"+*meeting.CalendarEventID, func(lockedRepo *repository.CRMMeetingRepository) error {
		lockedService := *s
		lockedService.repo = lockedRepo
		return lockedService.startScheduledMeetingCaptureLocked(ctx, workspaceID, meetingID)
	})
	if launchErr == nil {
		return nil
	}
	meeting, lookupErr := s.repo.GetByID(ctx, workspaceID, meetingID)
	if lookupErr != nil {
		return fmt.Errorf("reload failed automatic meeting capture: %v: %w", lookupErr, launchErr)
	}
	if meeting != nil && meeting.Status != model.CRMMeetingStatusCancelled {
		if markErr := s.markScheduledCaptureFailure(ctx, meeting, "Helpin could not automatically join this meeting", launchErr); markErr != nil && !errors.Is(markErr, launchErr) {
			return fmt.Errorf("persist failed automatic meeting capture: %v: %w", markErr, launchErr)
		}
	}
	return launchErr
}

func (s *CRMMeetingService) startScheduledMeetingCaptureLocked(ctx context.Context, workspaceID, meetingID string) error {
	meeting, err := s.repo.GetByID(ctx, workspaceID, meetingID)
	if err != nil {
		return err
	}
	if meeting == nil || meeting.CalendarEventID == nil || meeting.Status == model.CRMMeetingStatusCancelled {
		return nil
	}
	if meeting.Status != model.CRMMeetingStatusScheduled &&
		!(meeting.Status == model.CRMMeetingStatusFailed && meeting.FailureCode != nil && *meeting.FailureCode == calendarAutoFailureCode) {
		return nil
	}
	now := time.Now().UTC()
	if meeting.ScheduledEndAt != nil && now.After(meeting.ScheduledEndAt.UTC()) {
		return s.markScheduledCaptureFailure(ctx, meeting, "Helpin could not join before the meeting ended", nil)
	}
	if meeting.ScheduledEndAt == nil && meeting.ScheduledStartAt != nil && now.After(meeting.ScheduledStartAt.UTC().Add(15*time.Minute)) {
		return s.markScheduledCaptureFailure(ctx, meeting, "Helpin missed the scheduled meeting start", nil)
	}
	meeting.Status = model.CRMMeetingStatusScheduled
	meeting.FailureCode = nil
	meeting.FailureMessage = nil
	if err := s.repo.Update(ctx, meeting); err != nil {
		return err
	}
	if _, err := s.startCaptureLocked(ctx, s.repo, workspaceID, meetingID, calendarAutoCaptureKeyBase+meetingID); err != nil {
		return err
	}
	return nil
}

func (s *CRMMeetingService) markScheduledCaptureFailure(ctx context.Context, meeting *model.CRMMeeting, message string, cause error) error {
	code := calendarAutoFailureCode
	meeting.Status = model.CRMMeetingStatusFailed
	meeting.FailureCode = &code
	meeting.FailureMessage = &message
	if err := s.repo.Update(ctx, meeting); err != nil {
		return err
	}
	return cause
}
