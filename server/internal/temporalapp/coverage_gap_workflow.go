package temporalapp

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	CoverageGapEnrichmentWorkflowType   = "CoverageGapEnrichmentWorkflow"
	CoverageGapDailyBatchWorkflowType   = "CoverageGapDailyBatchWorkflow"
	CoverageGapPerWorkspaceWorkflowType = "CoverageGapPerWorkspaceWorkflow"

	CoverageGapEnrichmentActivityName     = "CoverageGapActivities.EnrichTopicActivity"
	CoverageGapListBatchActivityName      = "CoverageGapActivities.ListTopicsForBatchActivity"
	CoverageGapListWorkspacesActivityName = "CoverageGapActivities.ListWorkspacesActivity"
)

type coverageGapEnricher interface {
	EnrichTopic(ctx context.Context, topicID string) error
}

type coverageGapBatcher interface {
	ListWorkspacesWithOpenGaps(ctx context.Context) ([]string, error)
	ListTopicsDueForEnrichment(ctx context.Context, workspaceID string, olderThan time.Duration, minEvidence int) ([]string, error)
}

type CoverageGapActivities struct {
	enricher coverageGapEnricher
	coverage coverageGapBatcher
}

func NewCoverageGapActivities(enricher coverageGapEnricher, coverage coverageGapBatcher) *CoverageGapActivities {
	return &CoverageGapActivities{enricher: enricher, coverage: coverage}
}

func newCoverageGapActivitiesForTest(enricher coverageGapEnricher, coverage coverageGapBatcher) *CoverageGapActivities {
	return NewCoverageGapActivities(enricher, coverage)
}

func (a *CoverageGapActivities) EnrichTopicActivity(ctx context.Context, topicID string) error {
	activity.GetLogger(ctx).Info("enriching coverage topic", "topic_id", topicID)
	return a.enricher.EnrichTopic(ctx, topicID)
}

func (a *CoverageGapActivities) ListWorkspacesActivity(ctx context.Context) ([]string, error) {
	return a.coverage.ListWorkspacesWithOpenGaps(ctx)
}

func (a *CoverageGapActivities) ListTopicsForBatchActivity(ctx context.Context, workspaceID string) ([]string, error) {
	return a.coverage.ListTopicsDueForEnrichment(ctx, workspaceID, 24*time.Hour, 2)
}

func CoverageGapEnrichmentWorkflow(ctx workflow.Context, topicID string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumAttempts:    3,
		},
	})
	return workflow.ExecuteActivity(ctx, CoverageGapEnrichmentActivityName, topicID).Get(ctx, nil)
}

func CoverageGapDailyBatchWorkflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	})

	var workspaces []string
	if err := workflow.ExecuteActivity(ctx, CoverageGapListWorkspacesActivityName).Get(ctx, &workspaces); err != nil {
		return err
	}

	selector := workflow.NewSelector(ctx)
	inflight := 0
	const maxInflight = 10
	for _, workspaceID := range workspaces {
		if inflight >= maxInflight {
			selector.Select(ctx)
			inflight--
		}
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: "coverage-gap-batch-ws-" + workspaceID,
		})
		future := workflow.ExecuteChildWorkflow(childCtx, CoverageGapPerWorkspaceWorkflowType, workspaceID)
		selector.AddFuture(future, func(workflow.Future) {})
		inflight++
	}
	for inflight > 0 {
		selector.Select(ctx)
		inflight--
	}
	return nil
}

func CoverageGapPerWorkspaceWorkflow(ctx workflow.Context, workspaceID string) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	})

	var topicIDs []string
	if err := workflow.ExecuteActivity(ctx, CoverageGapListBatchActivityName, workspaceID).Get(ctx, &topicIDs); err != nil {
		return err
	}
	for _, topicID := range topicIDs {
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: "coverage-gap-enrich-" + topicID,
		})
		_ = workflow.ExecuteChildWorkflow(childCtx, CoverageGapEnrichmentWorkflowType, topicID).Get(ctx, nil)
	}
	return nil
}
