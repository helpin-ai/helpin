package temporalapp

import (
	"context"
	"log/slog"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DealManagementInput identifies the workspace to process.
type DealManagementInput struct {
	WorkspaceID string
}

// DealManagementCronWorkflow runs hourly to evaluate deal progression.
func DealManagementCronWorkflow(ctx workflow.Context, input DealManagementInput) error {
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

	if err := workflow.ExecuteActivity(ctx, "DealManagementActivities.EvaluateProgressionActivity", input).Get(ctx, nil); err != nil {
		return err
	}

	return nil
}

// DealProgressionEvaluator abstracts the deal progression evaluation to avoid import cycles.
type DealProgressionEvaluator interface {
	EvaluateDealProgression(ctx context.Context, workspaceID string) error
}

// DealManagementActivities contains deal management activities.
type DealManagementActivities struct {
	evaluator DealProgressionEvaluator
}

// NewDealManagementActivities creates deal management activities.
func NewDealManagementActivities(evaluator DealProgressionEvaluator) *DealManagementActivities {
	return &DealManagementActivities{evaluator: evaluator}
}

// EvaluateProgressionActivity evaluates all active deals for progression.
func (a *DealManagementActivities) EvaluateProgressionActivity(ctx context.Context, input DealManagementInput) error {
	if err := a.evaluator.EvaluateDealProgression(ctx, input.WorkspaceID); err != nil {
		slog.ErrorContext(ctx, "deal progression evaluation failed", "error", err, "workspace_id", input.WorkspaceID)
		return err
	}
	return nil
}
