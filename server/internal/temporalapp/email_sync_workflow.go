package temporalapp

import (
	"errors"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const emailSyncNowSignal = "email-sync-now"

// EmailSyncWorkflowInput identifies the email account to sync.
type EmailSyncWorkflowInput struct {
	AccountID   string
	InitialMode string
}

// EmailSyncWorkflow is a long-running Temporal workflow that syncs emails for an account.
func EmailSyncWorkflow(ctx workflow.Context, input EmailSyncWorkflowInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    10 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var initialErr error
	if input.InitialMode == "" {
		var result EmailSyncResult
		initialErr = workflow.ExecuteActivity(ctx, "EmailSyncActivities.BackfillEmailsActivity", input.AccountID).Get(ctx, &result)
	} else {
		initialErr = executeEmailSyncMode(ctx, input.AccountID, input.InitialMode)
	}
	if initialErr != nil {
		return initialErr
	}

	// Incremental sync loop — poll every 5 minutes or wake immediately when a
	// user requests a sync from settings.
	signalChannel := workflow.GetSignalChannel(ctx, emailSyncNowSignal)
	for {
		mode := model.CRMEmailSyncModeIncremental
		selector := workflow.NewSelector(ctx)
		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		selector.AddFuture(workflow.NewTimer(timerCtx, 5*time.Minute), func(workflow.Future) {})
		selector.AddReceive(signalChannel, func(channel workflow.ReceiveChannel, _ bool) {
			channel.Receive(ctx, &mode)
			cancelTimer()
		})
		selector.Select(ctx)
		if mode != model.CRMEmailSyncModeHistorical {
			mode = model.CRMEmailSyncModeIncremental
		}

		if err := executeEmailSyncMode(ctx, input.AccountID, mode); err != nil {
			// Non-retryable errors (e.g. deleted account) should terminate the workflow.
			var appErr *temporal.ApplicationError
			if errors.As(err, &appErr) && !appErr.NonRetryable() {
				continue
			}
			return err
		}
	}
}

func executeEmailSyncMode(ctx workflow.Context, accountID, mode string) error {
	activityName := "EmailSyncActivities.IncrementalSyncActivity"
	if mode == model.CRMEmailSyncModeHistorical {
		activityName = "EmailSyncActivities.HistoricalBackfillEmailsActivity"
	}
	var result EmailSyncResult
	return workflow.ExecuteActivity(ctx, activityName, accountID).Get(ctx, &result)
}

// EmailSyncResult contains the result of a sync operation.
type EmailSyncResult struct {
	MessagesProcessed int
	NewHistoryID      string
}
