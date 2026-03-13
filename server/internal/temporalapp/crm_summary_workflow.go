package temporalapp

import (
	"context"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const crmSummaryDebounceWindow = 2 * time.Minute

// CRMSummaryWorkflowResult captures the outcome of one workflow pass.
type CRMSummaryWorkflowResult struct {
	Status        string `json:"status"`
	NeedsContinue bool   `json:"needs_continue"`
}

// CRMEntitySummaryWorkflow debounces and recomputes one entity summary.
func CRMEntitySummaryWorkflow(ctx workflow.Context, input model.CRMEntitySummaryRefreshInput) (*CRMSummaryWorkflowResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    1 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	if err := workflow.Sleep(ctx, crmSummaryDebounceWindow); err != nil {
		return nil, err
	}

	var result model.CRMEntitySummaryRefreshResult
	if err := workflow.ExecuteActivity(ctx, "CRMSummaryActivities.RefreshSummaryActivity", input).Get(ctx, &result); err != nil {
		return nil, err
	}

	if result.NeedsContinue {
		return nil, workflow.NewContinueAsNewError(ctx, CRMEntitySummaryWorkflow, input)
	}

	return &CRMSummaryWorkflowResult{
		Status:        result.Status,
		NeedsContinue: result.NeedsContinue,
	}, nil
}

// CRMSummaryDailyReconciliationWorkflow requests refreshes for active entities once per day.
func CRMSummaryDailyReconciliationWorkflow(ctx workflow.Context) error {
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

	return workflow.ExecuteActivity(ctx, "CRMSummaryActivities.DailyReconciliationActivity").Get(ctx, nil)
}

type summaryRefresher interface {
	RefreshSummary(ctx context.Context, input model.CRMEntitySummaryRefreshInput) (*model.CRMEntitySummaryRefreshResult, error)
	RunDailyReconciliation(ctx context.Context) (*model.CRMSummaryReconciliationResult, error)
}

// CRMSummaryActivities contains summary generation and reconciliation activities.
type CRMSummaryActivities struct {
	summaryService summaryRefresher
}

// NewCRMSummaryActivities creates CRM summary Temporal activities.
func NewCRMSummaryActivities(summaryService summaryRefresher) *CRMSummaryActivities {
	return &CRMSummaryActivities{summaryService: summaryService}
}

// RefreshSummaryActivity recomputes one CRM summary artifact.
func (a *CRMSummaryActivities) RefreshSummaryActivity(ctx context.Context, input model.CRMEntitySummaryRefreshInput) (*model.CRMEntitySummaryRefreshResult, error) {
	return a.summaryService.RefreshSummary(ctx, input)
}

// DailyReconciliationActivity enqueues refreshes for active deals and recently touched contacts.
func (a *CRMSummaryActivities) DailyReconciliationActivity(ctx context.Context) (*model.CRMSummaryReconciliationResult, error) {
	return a.summaryService.RunDailyReconciliation(ctx)
}
