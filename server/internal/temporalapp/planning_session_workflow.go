package temporalapp

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	QueuePlanningInteractive              = "planning-interactive"
	WorkflowSignalPlanningSession         = "PlanningSessionSignal"
	PlanningSessionSignalTypeMessage      = "message"
	PlanningSessionSignalTypeFinalize     = "finalize"
	PlanningSessionSignalTypeAbandon      = "abandon"
)

// PlanningSessionWorkflowInput identifies the session to run.
type PlanningSessionWorkflowInput struct {
	SessionID string
}

// PlanningSessionSignal carries a user action to the workflow.
type PlanningSessionSignal struct {
	Type    string `json:"type"`    // "message", "finalize", "abandon"
	ActorID string `json:"actor_id"`
}

// WorkflowIDForPlanningSession returns the Temporal workflow ID for a planning session.
func WorkflowIDForPlanningSession(sessionID string) string {
	return "planning-session-" + sessionID
}

// PlanningSessionWorkflow is the Temporal workflow for an interactive planning session.
// It clones the repo once, then loops processing signals (messages, finalize, abandon).
func PlanningSessionWorkflow(ctx workflow.Context, input PlanningSessionWorkflowInput) error {
	sessionID := input.SessionID

	// Use a Temporal session to pin all activities to the same worker,
	// keeping the cloned repo available across message turns.
	sessCtx, err := workflow.CreateSession(ctx, &workflow.SessionOptions{
		CreationTimeout:  2 * time.Minute,
		ExecutionTimeout: 4 * time.Hour,
	})
	if err != nil {
		return err
	}
	defer workflow.CompleteSession(sessCtx)

	// Activity options for workspace prep.
	prepareAO := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		HeartbeatTimeout:    60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}

	// Activity options for agent turns (longer timeout for Claude streaming).
	turnAO := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}

	// 1. Prepare workspace (clone repo).
	prepareCtx := workflow.WithActivityOptions(sessCtx, prepareAO)
	var workDir string
	if err := workflow.ExecuteActivity(prepareCtx, "PlanningSessionActivities.PrepareWorkspaceActivity", sessionID).Get(ctx, &workDir); err != nil {
		return err
	}

	// Ensure cleanup runs on workflow exit.
	defer func() {
		cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
		cleanupCtx = workflow.WithActivityOptions(cleanupCtx, workflow.ActivityOptions{
			StartToCloseTimeout: 30 * time.Second,
		})
		_ = workflow.ExecuteActivity(cleanupCtx, "PlanningSessionActivities.CleanupWorkspaceActivity", workDir).Get(cleanupCtx, nil)
	}()

	// 2. Run initial agent turn (agent greets / asks questions).
	turnCtx := workflow.WithActivityOptions(sessCtx, turnAO)
	if err := workflow.ExecuteActivity(turnCtx, "PlanningSessionActivities.RunTurnActivity", sessionID, workDir).Get(ctx, nil); err != nil {
		// Non-fatal: log and continue to signal loop.
		workflow.GetLogger(ctx).Error("initial turn failed", "error", err)
	}

	// 3. Signal loop — process messages until finalize or abandon.
	signalCh := workflow.GetSignalChannel(ctx, WorkflowSignalPlanningSession)
	for {
		var signal PlanningSessionSignal
		signalCh.Receive(ctx, &signal)

		switch signal.Type {
		case PlanningSessionSignalTypeMessage:
			// Run agent turn (user already saved their message via the API).
			if err := workflow.ExecuteActivity(turnCtx, "PlanningSessionActivities.RunTurnActivity", sessionID, workDir).Get(ctx, nil); err != nil {
				workflow.GetLogger(ctx).Error("turn failed", "error", err)
			}

		case PlanningSessionSignalTypeFinalize:
			// Finalization is now handled synchronously by the API — just clean up.
			return nil

		case PlanningSessionSignalTypeAbandon:
			// Abandon — cleanup handled by defer.
			return nil
		}
	}
}
