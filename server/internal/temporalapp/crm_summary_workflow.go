package temporalapp

import (
	"context"
	"fmt"
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
	healthObserver interface {
		ObserveSuccess(ctx context.Context, workspaceID, catalogID, scopeType, scopeID string, metrics model.JSONB) error
		ObserveFailure(ctx context.Context, workspaceID, catalogID, scopeType, scopeID, message string, metrics model.JSONB) error
	}
}

// NewCRMSummaryActivities creates CRM summary Temporal activities.
func NewCRMSummaryActivities(summaryService summaryRefresher) *CRMSummaryActivities {
	return &CRMSummaryActivities{summaryService: summaryService}
}

func (a *CRMSummaryActivities) SetHealthObserver(observer interface {
	ObserveSuccess(ctx context.Context, workspaceID, catalogID, scopeType, scopeID string, metrics model.JSONB) error
	ObserveFailure(ctx context.Context, workspaceID, catalogID, scopeType, scopeID, message string, metrics model.JSONB) error
}) *CRMSummaryActivities {
	a.healthObserver = observer
	return a
}

// RefreshSummaryActivity recomputes one CRM summary artifact.
func (a *CRMSummaryActivities) RefreshSummaryActivity(ctx context.Context, input model.CRMEntitySummaryRefreshInput) (*model.CRMEntitySummaryRefreshResult, error) {
	result, err := a.summaryService.RefreshSummary(ctx, input)
	if err != nil {
		a.observeFailure(ctx, input.WorkspaceID, summaryCatalogID(input.EntityType), err, model.JSONB{
			"entity_type": input.EntityType,
			"entity_id":   input.EntityID,
		})
		return result, err
	}
	if result == nil {
		result = &model.CRMEntitySummaryRefreshResult{
			EntityType: input.EntityType,
			EntityID:   input.EntityID,
			Status:     model.CRMEntitySummaryStatusError,
		}
	}
	a.observeSuccess(ctx, input.WorkspaceID, summaryCatalogID(input.EntityType), model.JSONB{
		"entity_type":         input.EntityType,
		"entity_id":           input.EntityID,
		"status":              result.Status,
		"needs_continue":      result.NeedsContinue,
		"highlights":          result.Highlights,
		"source_email_count":  result.SourceEmailCount,
		"source_signal_count": result.SourceSignalCount,
	})
	return result, nil
}

// DailyReconciliationActivity enqueues refreshes for active CRM entities.
func (a *CRMSummaryActivities) DailyReconciliationActivity(ctx context.Context) (*model.CRMSummaryReconciliationResult, error) {
	result, err := a.summaryService.RunDailyReconciliation(ctx)
	if err != nil {
		a.observeFailure(ctx, "", "crm.contact_summary_refresh", err, model.JSONB{"mode": "daily_reconciliation"})
		a.observeFailure(ctx, "", "crm.deal_summary_refresh", err, model.JSONB{"mode": "daily_reconciliation"})
		a.observeFailure(ctx, "", "crm.company_summary_refresh", err, model.JSONB{"mode": "daily_reconciliation"})
		return result, err
	}
	if result != nil {
		metrics := model.JSONB{
			"mode":             "daily_reconciliation",
			"contacts_queued":  result.ContactsQueued,
			"deals_queued":     result.DealsQueued,
			"companies_queued": result.CompaniesQueued,
		}
		a.observeSuccess(ctx, "", "crm.contact_summary_refresh", metrics)
		a.observeSuccess(ctx, "", "crm.deal_summary_refresh", metrics)
		a.observeSuccess(ctx, "", "crm.company_summary_refresh", metrics)
	}
	return result, nil
}

func summaryCatalogID(entityType string) string {
	switch entityType {
	case "contact":
		return "crm.contact_summary_refresh"
	case "deal":
		return "crm.deal_summary_refresh"
	case "company":
		return "crm.company_summary_refresh"
	default:
		return ""
	}
}

func (a *CRMSummaryActivities) observeSuccess(ctx context.Context, workspaceID, catalogID string, metrics model.JSONB) {
	if a == nil || a.healthObserver == nil || catalogID == "" {
		return
	}
	scopeID := workspaceID
	if scopeID == "" {
		if value, ok := metrics["workspace_id"].(string); ok && value != "" {
			scopeID = value
		}
	}
	if scopeID == "" {
		return
	}
	_ = a.healthObserver.ObserveSuccess(ctx, scopeID, catalogID, model.AutomationScopeWorkspace, scopeID, metrics)
}

func (a *CRMSummaryActivities) observeFailure(ctx context.Context, workspaceID, catalogID string, cause error, metrics model.JSONB) {
	if a == nil || a.healthObserver == nil || catalogID == "" || cause == nil {
		return
	}
	scopeID := workspaceID
	if scopeID == "" {
		if value, ok := metrics["workspace_id"].(string); ok && value != "" {
			scopeID = value
		}
	}
	if scopeID == "" {
		return
	}
	message := cause.Error()
	if len(message) > 500 {
		message = fmt.Sprintf("%.500s", message)
	}
	_ = a.healthObserver.ObserveFailure(ctx, scopeID, catalogID, model.AutomationScopeWorkspace, scopeID, message, metrics)
}
