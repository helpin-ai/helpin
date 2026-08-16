package temporalapp

import (
	"context"
	"errors"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/helpin-ai/helpin/server/internal/meetingcapture"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	CRMMeetingCaptureScheduleSignal      = "crm-meeting-capture-schedule"
	CRMMeetingStartScheduledActivityName = "CRMMeetingActivities.StartScheduledCaptureActivity"
)

// CRMMeetingCaptureScheduleInput identifies one durable automatic join timer.
type CRMMeetingCaptureScheduleInput struct {
	WorkspaceID string    `json:"workspace_id"`
	MeetingID   string    `json:"meeting_id"`
	JoinAt      time.Time `json:"join_at"`
}

// CRMMeetingCaptureScheduleCommand reschedules or cancels an automatic join.
type CRMMeetingCaptureScheduleCommand struct {
	WorkspaceID string    `json:"workspace_id"`
	MeetingID   string    `json:"meeting_id"`
	JoinAt      time.Time `json:"join_at"`
	Cancel      bool      `json:"cancel"`
}

// CRMMeetingCaptureScheduleWorkflow waits durably and starts the selected
// deployment-owned capture provider at the requested time.
func CRMMeetingCaptureScheduleWorkflow(ctx workflow.Context, input CRMMeetingCaptureScheduleInput) error {
	schedule := input
	signals := workflow.GetSignalChannel(ctx, CRMMeetingCaptureScheduleSignal)

	for {
		delay := schedule.JoinAt.Sub(workflow.Now(ctx))
		if delay <= 0 {
			break
		}

		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		timer := workflow.NewTimer(timerCtx, delay)
		timerFired := false
		var command CRMMeetingCaptureScheduleCommand

		selector := workflow.NewSelector(ctx)
		selector.AddFuture(timer, func(workflow.Future) {
			timerFired = true
		})
		selector.AddReceive(signals, func(channel workflow.ReceiveChannel, _ bool) {
			channel.Receive(ctx, &command)
			cancelTimer()
		})
		selector.Select(ctx)

		if timerFired {
			break
		}
		if command.Cancel {
			return nil
		}
		if command.WorkspaceID != "" && command.MeetingID != "" && !command.JoinAt.IsZero() {
			schedule = CRMMeetingCaptureScheduleInput{
				WorkspaceID: command.WorkspaceID,
				MeetingID:   command.MeetingID,
				JoinAt:      command.JoinAt,
			}
		}
	}

	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    15 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    5,
		},
	})
	return workflow.ExecuteActivity(activityCtx, CRMMeetingStartScheduledActivityName, CRMMeetingProcessingInput{
		WorkspaceID: schedule.WorkspaceID,
		MeetingID:   schedule.MeetingID,
	}).Get(activityCtx, nil)
}

type scheduledMeetingCaptureLauncher interface {
	StartScheduledMeetingCapture(ctx context.Context, workspaceID, meetingID string) error
}

// SetCaptureLauncher enables the automatic meeting join activity.
func (a *CRMMeetingActivities) SetCaptureLauncher(launcher scheduledMeetingCaptureLauncher) *CRMMeetingActivities {
	if a != nil {
		a.captureLauncher = launcher
	}
	return a
}

// StartScheduledCaptureActivity launches an opted-in calendar meeting after
// revalidating its current state in the service layer.
func (a *CRMMeetingActivities) StartScheduledCaptureActivity(ctx context.Context, input CRMMeetingProcessingInput) error {
	if a == nil || a.captureLauncher == nil {
		return temporal.NewNonRetryableApplicationError("scheduled meeting capture is unavailable", "meeting_capture_unavailable", nil)
	}
	err := a.captureLauncher.StartScheduledMeetingCapture(ctx, input.WorkspaceID, input.MeetingID)
	if errors.Is(err, model.ErrAIUsageExhausted) || errors.Is(err, model.ErrExtraAIUsageUnavailable) ||
		errors.Is(err, model.ErrExtraAIUsageDisabled) || errors.Is(err, model.ErrBillingWorkspaceLocked) ||
		errors.Is(err, meetingcapture.ErrNotConfigured) {
		return temporal.NewNonRetryableApplicationError(err.Error(), "meeting_capture_blocked", err)
	}
	return err
}
