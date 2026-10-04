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

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DailyContentSourceSyncWorkflow queues the daily refresh of website knowledge.
func DailyContentSourceSyncWorkflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 10 * time.Second, MaximumAttempts: 3},
	})
	return workflow.ExecuteActivity(ctx, "ContentSourceSyncActivities.QueueDailySourceSync").Get(ctx, nil)
}

// EnsureDailyContentSourceSync installs one durable daily job per namespace.
func (e *RunEngine) EnsureDailyContentSourceSync(ctx context.Context) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}
	_, err := e.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID: "support-content-daily-sync", TaskQueue: QueueAutomation,
		CronSchedule: fmt.Sprintf("0 %d * * *", model.ContentSourceAutoSyncHourUTC),
	}, DailyContentSourceSyncWorkflow)
	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ensure daily website sync: %w", err)
	}
	return nil
}

// QueueDailySourceSync dispatches fresh source workflows rather than rebuilding
// embeddings from previously fetched content.
func (a *ContentSourceSyncActivities) QueueDailySourceSync(ctx context.Context) error {
	if a == nil {
		return fmt.Errorf("content source sync runner is not configured")
	}
	runner, ok := a.runner.(interface{ QueueDailySourceSync(context.Context) error })
	if !ok {
		return fmt.Errorf("daily content sync runner is not configured")
	}
	return runner.QueueDailySourceSync(ctx)
}
