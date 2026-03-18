package temporalapp

import (
	"context"
	"log/slog"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// SprintAutomationInput is the input for the sprint automation cron workflow.
type SprintAutomationInput struct{}

// SprintAutomationCronWorkflow runs hourly to evaluate sprint automations.
func SprintAutomationCronWorkflow(ctx workflow.Context, input SprintAutomationInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    10 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	if err := workflow.ExecuteActivity(ctx, "SprintAutomationActivities.RunSprintAutomationsActivity", input).Get(ctx, nil); err != nil {
		return err
	}

	return nil
}

// SprintAutomationRunner abstracts the sprint automation logic to avoid import cycles.
type SprintAutomationRunner interface {
	RunSprintAutomations(ctx context.Context)
}

// SprintAutomationActivities contains sprint automation activities.
type SprintAutomationActivities struct {
	runner SprintAutomationRunner
}

// NewSprintAutomationActivities creates sprint automation activities.
func NewSprintAutomationActivities(runner SprintAutomationRunner) *SprintAutomationActivities {
	return &SprintAutomationActivities{runner: runner}
}

// RunSprintAutomationsActivity executes the sprint automation logic.
func (a *SprintAutomationActivities) RunSprintAutomationsActivity(ctx context.Context, input SprintAutomationInput) error {
	if a.runner == nil {
		slog.WarnContext(ctx, "sprint automation runner not configured, skipping")
		return nil
	}
	a.runner.RunSprintAutomations(ctx)
	return nil
}
