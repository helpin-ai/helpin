package temporalapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const WorkflowSignalContentSourceSync = "ContentSourceSync"

type ContentSourceSyncInput struct {
	WorkspaceID      string `json:"workspace_id"`
	ContentSourceID  string `json:"content_source_id"`
}

func WorkflowIDForContentSource(workspaceID, contentSourceID string) string {
	return fmt.Sprintf("content-source-sync-%s-%s", workspaceID, contentSourceID)
}

func ContentSourceSyncWorkflow(ctx workflow.Context, input ContentSourceSyncInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Hour,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	signalCh := workflow.GetSignalChannel(ctx, WorkflowSignalContentSourceSync)
	current := input
	if strings.TrimSpace(current.WorkspaceID) == "" || strings.TrimSpace(current.ContentSourceID) == "" {
		signalCh.Receive(ctx, &current)
	}

	for {
		if err := workflow.ExecuteActivity(ctx, "ContentSourceSyncActivities.SyncContentSourceActivity", current).Get(ctx, nil); err != nil {
			return err
		}

		var next ContentSourceSyncInput
		hasPending := false
		for signalCh.ReceiveAsync(&next) {
			if strings.TrimSpace(next.WorkspaceID) != "" && strings.TrimSpace(next.ContentSourceID) != "" {
				current = next
				hasPending = true
			}
		}
		if !hasPending {
			return nil
		}
	}
}

type contentSourceSyncRunner interface {
	RunSourceSync(ctx context.Context, workspaceID, contentSourceID string) error
}

type ContentSourceSyncActivities struct {
	runner contentSourceSyncRunner
}

func NewContentSourceSyncActivities(runner contentSourceSyncRunner) *ContentSourceSyncActivities {
	if runner == nil {
		return nil
	}
	return &ContentSourceSyncActivities{runner: runner}
}

func (a *ContentSourceSyncActivities) SyncContentSourceActivity(ctx context.Context, input ContentSourceSyncInput) error {
	if a == nil || a.runner == nil {
		return fmt.Errorf("content source sync activity runner is not configured")
	}
	if strings.TrimSpace(input.WorkspaceID) == "" || strings.TrimSpace(input.ContentSourceID) == "" {
		return fmt.Errorf("workspace_id and content_source_id are required")
	}
	return a.runner.RunSourceSync(ctx, input.WorkspaceID, input.ContentSourceID)
}
