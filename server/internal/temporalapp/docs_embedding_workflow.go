package temporalapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const WorkflowSignalDocsEmbeddingSync = "DocsEmbeddingSync"

// DocsEmbeddingSyncInput identifies the help-center space to index.
type DocsEmbeddingSyncInput struct {
	WorkspaceID string `json:"workspace_id"`
	SpaceID     string `json:"space_id"`
}

// WorkflowIDForDocsEmbeddingSpace returns the workflow ID for one indexed help-center space.
func WorkflowIDForDocsEmbeddingSpace(workspaceID, spaceID string) string {
	return fmt.Sprintf("docs-embedding-%s-%s", workspaceID, spaceID)
}

// DocsEmbeddingSyncWorkflow runs chunking + embedding sync for one help-center space.
// Repeated sync requests for the same space are buffered through workflow signals and coalesced
// into a rerun after the current activity completes.
func DocsEmbeddingSyncWorkflow(ctx workflow.Context, input DocsEmbeddingSyncInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 45 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	signalCh := workflow.GetSignalChannel(ctx, WorkflowSignalDocsEmbeddingSync)
	current := input
	if strings.TrimSpace(current.WorkspaceID) == "" || strings.TrimSpace(current.SpaceID) == "" {
		signalCh.Receive(ctx, &current)
	}

	for {
		if err := workflow.ExecuteActivity(ctx, "DocsEmbeddingActivities.SyncSpaceActivity", current).Get(ctx, nil); err != nil {
			return err
		}

		var next DocsEmbeddingSyncInput
		hasPending := false
		for signalCh.ReceiveAsync(&next) {
			if strings.TrimSpace(next.WorkspaceID) != "" && strings.TrimSpace(next.SpaceID) != "" {
				current = next
				hasPending = true
			}
		}
		if !hasPending {
			return nil
		}
	}
}

// docsEmbeddingRunner is implemented by service.DocsEmbeddingService.
type docsEmbeddingRunner interface {
	RunSpaceSync(ctx context.Context, workspaceID, spaceID string) error
}

// DocsEmbeddingActivities contains durable activities for help-center indexing.
type DocsEmbeddingActivities struct {
	runner docsEmbeddingRunner
}

// NewDocsEmbeddingActivities creates docs embedding Temporal activities.
func NewDocsEmbeddingActivities(runner docsEmbeddingRunner) *DocsEmbeddingActivities {
	if runner == nil {
		return nil
	}
	return &DocsEmbeddingActivities{runner: runner}
}

// SyncSpaceActivity runs a single chunk/vector sync for one help-center space.
func (a *DocsEmbeddingActivities) SyncSpaceActivity(ctx context.Context, input DocsEmbeddingSyncInput) error {
	if a == nil || a.runner == nil {
		return fmt.Errorf("docs embedding activity runner is not configured")
	}
	if strings.TrimSpace(input.WorkspaceID) == "" || strings.TrimSpace(input.SpaceID) == "" {
		return fmt.Errorf("workspace_id and space_id are required")
	}
	return a.runner.RunSpaceSync(ctx, input.WorkspaceID, input.SpaceID)
}
