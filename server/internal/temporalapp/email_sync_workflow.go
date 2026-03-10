package temporalapp

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EmailSyncWorkflowInput identifies the email account to sync.
type EmailSyncWorkflowInput struct {
	AccountID string
}

// EmailSyncWorkflow is a long-running Temporal workflow that syncs emails for an account.
func EmailSyncWorkflow(ctx workflow.Context, input EmailSyncWorkflowInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    10 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Initial backfill
	var backfillResult EmailSyncResult
	if err := workflow.ExecuteActivity(ctx, "EmailSyncActivities.BackfillEmailsActivity", input.AccountID).Get(ctx, &backfillResult); err != nil {
		return err
	}

	// Incremental sync loop — poll every 5 minutes
	for {
		if err := workflow.Sleep(ctx, 5*time.Minute); err != nil {
			return err
		}

		var syncResult EmailSyncResult
		if err := workflow.ExecuteActivity(ctx, "EmailSyncActivities.IncrementalSyncActivity", input.AccountID).Get(ctx, &syncResult); err != nil {
			// Log but continue — transient failures shouldn't stop the workflow
			continue
		}
	}
}

// EmailSyncResult contains the result of a sync operation.
type EmailSyncResult struct {
	MessagesProcessed int
	NewHistoryID      string
}
