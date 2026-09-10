package temporalapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CRMMeetingFollowUpRoutingWorkflowID prevents overlapping historical classification batches.
const CRMMeetingFollowUpRoutingWorkflowID = "crm-meeting-follow-up-routing"

// CRMMeetingFollowUpRoutingWorkflow runs separately so AI latency never delays due product events.
func CRMMeetingFollowUpRoutingWorkflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})
	return workflow.ExecuteActivity(ctx, "CRMMeetingActivities.RouteMeetingFollowUps").Get(ctx, nil)
}

type meetingFollowUpRouter interface{ BackfillFollowUpRouting(context.Context) error }

// SetFollowUpRouter injects the bounded legacy classifier independently of meeting generation.
func (a *CRMMeetingActivities) SetFollowUpRouter(router meetingFollowUpRouter) *CRMMeetingActivities {
	a.followUpRouter = router
	return a
}

// RouteMeetingFollowUps classifies existing drafts without recreating meeting artifacts.
func (a *CRMMeetingActivities) RouteMeetingFollowUps(ctx context.Context) error {
	if a.followUpRouter == nil {
		return temporal.NewNonRetryableApplicationError("meeting follow-up router is not configured", "meeting_follow_up_router_unavailable", nil)
	}
	return a.followUpRouter.BackfillFollowUpRouting(ctx)
}

// EnsureMeetingFollowUpRouting creates one independent background cron with no overlapping batches.
func (e *RunEngine) EnsureMeetingFollowUpRouting(ctx context.Context) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}
	_, err := e.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID: CRMMeetingFollowUpRoutingWorkflowID, TaskQueue: QueueAutomation, CronSchedule: "*/5 * * * *",
	}, CRMMeetingFollowUpRoutingWorkflow)
	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ensure meeting follow-up routing: %w", err)
	}
	return nil
}
