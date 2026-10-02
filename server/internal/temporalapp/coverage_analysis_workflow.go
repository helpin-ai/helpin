package temporalapp

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	CoverageDailyAnalysisWorkflowType        = "CoverageDailyAnalysisWorkflow"
	CoverageWorkspaceAnalysisWorkflowType    = "CoverageWorkspaceAnalysisWorkflow"
	CoverageListAnalysisWorkspacesActivity   = "CoverageAnalysisActivities.ListWorkspacesActivity"
	CoverageRunWorkspaceAnalysisActivityName = "CoverageAnalysisActivities.RunWorkspaceAnalysisActivity"

	CoverageAnalysisMaxWorkspaceChildren = 10
)

type CoverageWorkspaceAnalysisInput struct {
	WorkspaceID string    `json:"workspace_id"`
	Reanalyze   bool      `json:"reanalyze,omitempty"`
	RequestID   string    `json:"request_id,omitempty"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
}

type coverageDailyAnalyzer interface {
	ListWorkspacesForDailyAnalysis(ctx context.Context) ([]string, error)
	RunWorkspaceDailyAnalysis(ctx context.Context, workspaceID string, windowStart, windowEnd time.Time) error
}

type CoverageAnalysisActivities struct {
	analyzer coverageDailyAnalyzer
}

func NewCoverageAnalysisActivities(analyzer coverageDailyAnalyzer) *CoverageAnalysisActivities {
	return &CoverageAnalysisActivities{analyzer: analyzer}
}

func (a *CoverageAnalysisActivities) ListWorkspacesActivity(ctx context.Context) ([]string, error) {
	activity.GetLogger(ctx).Info("listing workspaces for coverage daily analysis")
	return a.analyzer.ListWorkspacesForDailyAnalysis(ctx)
}

func (a *CoverageAnalysisActivities) RunWorkspaceAnalysisActivity(ctx context.Context, input CoverageWorkspaceAnalysisInput) error {
	activity.GetLogger(ctx).Info("running workspace coverage daily analysis", "workspace_id", input.WorkspaceID)
	activity.RecordHeartbeat(ctx, input.WorkspaceID, "started")
	if input.Reanalyze {
		analyzer, ok := a.analyzer.(interface {
			RunWorkspaceReanalysis(context.Context, string, time.Time, time.Time, string) error
		})
		if !ok || input.RequestID == "" {
			return fmt.Errorf("coverage reanalysis is not configured")
		}
		return analyzer.RunWorkspaceReanalysis(ctx, input.WorkspaceID, input.WindowStart, input.WindowEnd, input.RequestID)
	}
	return a.analyzer.RunWorkspaceDailyAnalysis(ctx, input.WorkspaceID, input.WindowStart, input.WindowEnd)
}

func CoverageDailyAnalysisWorkflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	})

	var workspaceIDs []string
	if err := workflow.ExecuteActivity(ctx, CoverageListAnalysisWorkspacesActivity).Get(ctx, &workspaceIDs); err != nil {
		return err
	}

	windowEnd := workflow.Now(ctx).UTC()
	windowStart := windowEnd.Add(-3 * time.Hour)
	selector := workflow.NewSelector(ctx)
	inflight := 0
	for _, workspaceID := range workspaceIDs {
		if inflight >= CoverageAnalysisMaxWorkspaceChildren {
			selector.Select(ctx)
			inflight--
		}
		input := CoverageWorkspaceAnalysisInput{
			WorkspaceID: workspaceID,
			WindowStart: windowStart,
			WindowEnd:   windowEnd,
		}
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("coverage-analysis-ws-%s-%d", workspaceID, windowEnd.Unix()),
		})
		future := workflow.ExecuteChildWorkflow(childCtx, CoverageWorkspaceAnalysisWorkflowType, input)
		selector.AddFuture(future, func(f workflow.Future) {
			if err := f.Get(ctx, nil); err != nil {
				workflow.GetLogger(ctx).Error("coverage workspace child failed", "error", err)
			}
		})
		inflight++
	}
	for inflight > 0 {
		selector.Select(ctx)
		inflight--
	}
	return nil
}

func CoverageWorkspaceAnalysisWorkflow(ctx workflow.Context, input CoverageWorkspaceAnalysisInput) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    30 * time.Second,
			BackoffCoefficient: 2,
			MaximumAttempts:    3,
		},
	})
	return workflow.ExecuteActivity(ctx, CoverageRunWorkspaceAnalysisActivityName, input).Get(ctx, nil)
}
