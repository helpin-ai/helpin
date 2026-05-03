package temporalapp

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CommandBarPlanWorkflowInput identifies a durable command-bar parent plan.
type CommandBarPlanWorkflowInput struct {
	WorkspaceID string
	ActorID     string
	PlanID      string
	Prompt      string
	PageContext model.CommandBarPageContext
	Steps       []model.CommandBarPlanStep
}

// CommandBarRunCompletedSignal is sent after a child agent run reaches a terminal state.
type CommandBarRunCompletedSignal struct {
	RunID string `json:"run_id"`
}

// CommandBarPlanProgress summarizes the current scheduler state for the parent workflow.
type CommandBarPlanProgress struct {
	Terminal bool
	Status   string
	Started  []string
}

// CommandBarPlanWorkflow schedules command-bar DAG steps through normal AgentRunWorkflow children.
func CommandBarPlanWorkflow(ctx workflow.Context, input CommandBarPlanWorkflowInput) error {
	stage := "starting"
	_ = workflow.SetQueryHandler(ctx, "current_step", func() (string, error) {
		return stage, nil
	})

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    3 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    8,
		},
	}
	activityCtx := workflow.WithActivityOptions(ctx, activityOptions)
	signalCh := workflow.GetSignalChannel(ctx, WorkflowSignalCommandBarRun)

	for {
		stage = "scheduling"
		var progress CommandBarPlanProgress
		if err := workflow.ExecuteActivity(activityCtx, "AgentRunActivities.StartReadyCommandBarPlanStepsActivity", input).Get(ctx, &progress); err != nil {
			return err
		}
		if progress.Terminal {
			stage = progress.Status
			return nil
		}

		stage = "waiting"
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(signalCh, func(c workflow.ReceiveChannel, more bool) {
			var ignored CommandBarRunCompletedSignal
			c.Receive(ctx, &ignored)
		})
		selector.AddFuture(workflow.NewTimer(ctx, time.Minute), func(workflow.Future) {})
		selector.Select(ctx)
	}
}
