package temporalapp

import (
	"encoding/json"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AgentRunWorkflowInput identifies the run to execute.
type AgentRunWorkflowInput struct {
	RunID string
}

// ExecuteRunResult summarizes the execution activity outcome.
type ExecuteRunResult struct {
	WaitForApproval   bool
	AwaitingInput     bool
	AwaitingAuth      bool
	ContinueExecution bool
}

// RunMessageSignal resumes an interactive run with a new user message.
type RunMessageSignal struct {
	Content string `json:"content"`
}

// RunResumeSignal resumes an interactive run with a generic human intent.
type RunResumeSignal struct {
	Intent          string          `json:"intent"`
	Content         string          `json:"content,omitempty"`
	ResponsePayload json.RawMessage `json:"response_payload,omitempty"`
}

// AgentRunWorkflow is the Temporal workflow for a single agent run.
func AgentRunWorkflow(ctx workflow.Context, input AgentRunWorkflowInput) error {
	currentStage := "queued"
	waitingApproval := false
	waitingInput := false

	_ = workflow.SetQueryHandler(ctx, "current_step", func() (string, error) {
		return currentStage, nil
	})
	_ = workflow.SetQueryHandler(ctx, "approval_wait_state", func() (bool, error) {
		return waitingApproval, nil
	})
	_ = workflow.SetQueryHandler(ctx, "input_wait_state", func() (bool, error) {
		return waitingInput, nil
	})

	prepareAO := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Hour,
		HeartbeatTimeout:    60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	executeAO := prepareAO
	executeAO.RetryPolicy = &temporal.RetryPolicy{
		MaximumAttempts: 1,
	}
	failAO := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}
	advanceAO := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	}

	currentStage = "preparing"
	prepareCtx := workflow.WithActivityOptions(ctx, prepareAO)
	if err := workflow.ExecuteActivity(prepareCtx, "AgentRunActivities.PrepareRunActivity", input.RunID).Get(ctx, nil); err != nil {
		markRunFailed(workflow.WithActivityOptions(ctx, failAO), input.RunID, err)
		return err
	}

	approveCh := workflow.GetSignalChannel(ctx, WorkflowSignalApprove)
	handoffCh := workflow.GetSignalChannel(ctx, WorkflowSignalHandoff)
	messageCh := workflow.GetSignalChannel(ctx, WorkflowSignalMessage)
	resumeCh := workflow.GetSignalChannel(ctx, WorkflowSignalResume)
	executeCtx := workflow.WithActivityOptions(ctx, executeAO)

	for {
		currentStage = "executing"
		var result ExecuteRunResult
		if err := workflow.ExecuteActivity(executeCtx, "AgentRunActivities.ExecuteRunActivity", input.RunID).Get(ctx, &result); err != nil {
			markRunFailed(workflow.WithActivityOptions(ctx, failAO), input.RunID, err)
			return err
		}

		if result.WaitForApproval {
			waitingApproval = true
			currentStage = "awaiting_approval"
			for waitingApproval {
				selector := workflow.NewSelector(ctx)
				selector.AddReceive(resumeCh, func(c workflow.ReceiveChannel, more bool) {
					var signal RunResumeSignal
					c.Receive(ctx, &signal)
					waitingApproval = false
					currentStage = workflowStageForResumeSignal(signal, true)
				})
				selector.AddReceive(approveCh, func(c workflow.ReceiveChannel, more bool) {
					var ignored struct{}
					c.Receive(ctx, &ignored)
					waitingApproval = false
					currentStage = "approval_received"
				})
				selector.AddReceive(messageCh, func(c workflow.ReceiveChannel, more bool) {
					var msg RunMessageSignal
					c.Receive(ctx, &msg)
					waitingApproval = false
					currentStage = "feedback_received"
				})
				selector.AddReceive(handoffCh, func(c workflow.ReceiveChannel, more bool) {
					var ignored any
					c.Receive(ctx, &ignored)
					currentStage = "handoff_recorded"
				})
				selector.Select(ctx)
			}
			continue
		}

		if result.AwaitingInput {
			waitingInput = true
			currentStage = "awaiting_input"
			for waitingInput {
				selector := workflow.NewSelector(ctx)
				selector.AddReceive(resumeCh, func(c workflow.ReceiveChannel, more bool) {
					var signal RunResumeSignal
					c.Receive(ctx, &signal)
					waitingInput = false
					currentStage = workflowStageForResumeSignal(signal, false)
				})
				selector.AddReceive(messageCh, func(c workflow.ReceiveChannel, more bool) {
					var msg RunMessageSignal
					c.Receive(ctx, &msg)
					waitingInput = false
					currentStage = "input_received"
				})
				selector.AddReceive(approveCh, func(c workflow.ReceiveChannel, more bool) {
					var ignored struct{}
					c.Receive(ctx, &ignored)
					waitingInput = false
					currentStage = "approval_received"
				})
				selector.AddReceive(handoffCh, func(c workflow.ReceiveChannel, more bool) {
					var ignored any
					c.Receive(ctx, &ignored)
					currentStage = "handoff_recorded"
				})
				selector.Select(ctx)
			}
			if currentStage == "approval_received" {
				break
			}
			continue
		}

		if result.AwaitingAuth {
			currentStage = "awaiting_auth"
			for currentStage == "awaiting_auth" {
				selector := workflow.NewSelector(ctx)
				selector.AddReceive(resumeCh, func(c workflow.ReceiveChannel, more bool) {
					var signal RunResumeSignal
					c.Receive(ctx, &signal)
					currentStage = workflowStageForResumeSignal(signal, false)
				})
				selector.AddReceive(handoffCh, func(c workflow.ReceiveChannel, more bool) {
					var ignored any
					c.Receive(ctx, &ignored)
					currentStage = "handoff_recorded"
				})
				selector.Select(ctx)
			}
			continue
		}

		if result.ContinueExecution {
			currentStage = "continuing"
			continue
		}

		break
	}

	currentStage = "completed"
	if err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, advanceAO), "AgentRunActivities.AdvanceCommandBarPlanActivity", input.RunID).Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Warn("failed to advance command bar plan after retries", "run_id", input.RunID, "error", err)
	}
	return nil
}

func markRunFailed(ctx workflow.Context, runID string, err error) {
	if err == nil {
		return
	}
	_ = workflow.ExecuteActivity(ctx, "AgentRunActivities.MarkRunFailedActivity", runID, err.Error()).Get(ctx, nil)
}

func workflowStageForResumeSignal(signal RunResumeSignal, waitingApproval bool) string {
	switch signal.Intent {
	case "approve":
		return "approval_received"
	case "request_changes":
		return "feedback_received"
	case "reply":
		if waitingApproval {
			return "feedback_received"
		}
		return "input_received"
	case model.AgentRunResumeIntentAuthCompleted:
		return "auth_completed"
	default:
		if waitingApproval {
			return "feedback_received"
		}
		return "input_received"
	}
}
