package temporalapp

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const DocsImportExecuteActivityName = "DocsImportActivities.ExecuteHelpScoutImportActivity"

// DocsImportWorkflowInput identifies a durable docs import job.
type DocsImportWorkflowInput struct {
	ImportID string `json:"import_id"`
}

// WorkflowIDForDocsImport returns the stable workflow ID for a docs import job.
func WorkflowIDForDocsImport(importID string) string {
	return fmt.Sprintf("docs-import-helpscout-%s", strings.TrimSpace(importID))
}

// DocsImportWorkflow executes a release-safe HelpScout import activity.
func DocsImportWorkflow(ctx workflow.Context, input DocsImportWorkflowInput) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 48 * time.Hour,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    10,
		},
	})
	return workflow.ExecuteActivity(ctx, DocsImportExecuteActivityName, input).Get(ctx, nil)
}
