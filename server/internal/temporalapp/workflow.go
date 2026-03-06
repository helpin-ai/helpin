package temporalapp

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// AgentRunWorkflowInput identifies the run to execute.
type AgentRunWorkflowInput struct {
	RunID string
}

// ExecuteRunResult summarizes the execution activity outcome.
type ExecuteRunResult struct {
	WaitForApproval bool
}

// AgentRunWorkflow is the Temporal workflow for a single agent run.
func AgentRunWorkflow(ctx workflow.Context, input AgentRunWorkflowInput) error {
	currentStage := "queued"
	waitingApproval := false

	_ = workflow.SetQueryHandler(ctx, "current_step", func() (string, error) {
		return currentStage, nil
	})
	_ = workflow.SetQueryHandler(ctx, "approval_wait_state", func() (bool, error) {
		return waitingApproval, nil
	})

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Hour,
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	currentStage = "preparing"
	if err := workflow.ExecuteActivity(ctx, "AgentRunActivities.PrepareRunActivity", input.RunID).Get(ctx, nil); err != nil {
		return err
	}

	currentStage = "executing"
	var result ExecuteRunResult
	if err := workflow.ExecuteActivity(ctx, "AgentRunActivities.ExecuteRunActivity", input.RunID).Get(ctx, &result); err != nil {
		return err
	}

	if result.WaitForApproval {
		waitingApproval = true
		currentStage = "awaiting_approval"
		approveCh := workflow.GetSignalChannel(ctx, WorkflowSignalApprove)
		handoffCh := workflow.GetSignalChannel(ctx, WorkflowSignalHandoff)
		for waitingApproval {
			selector := workflow.NewSelector(ctx)
			selector.AddReceive(approveCh, func(c workflow.ReceiveChannel, more bool) {
				var ignored struct{}
				c.Receive(ctx, &ignored)
				waitingApproval = false
				currentStage = "approved"
			})
			selector.AddReceive(handoffCh, func(c workflow.ReceiveChannel, more bool) {
				var ignored any
				c.Receive(ctx, &ignored)
				currentStage = "handoff_recorded"
			})
			selector.Select(ctx)
		}
	}

	currentStage = "completed"
	return nil
}
