package temporalapp

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestFlowRunWorkflow_RefreshThenFinalizeAdvancesRunningWorkflow(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	finalizeCalls := 0
	loadCalls := 0

	env.RegisterWorkflow(FlowRunWorkflow)
	env.RegisterActivityWithOptions(func(ctx context.Context, flowRunID, actorID string) error {
		return nil
	}, activity.RegisterOptions{Name: "FlowRuntimeActivities.BootstrapRunActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, flowRunID string) (FlowRuntimeState, error) {
		loadCalls++
		if loadCalls == 1 {
			return FlowRuntimeState{
				RunID:         flowRunID,
				Status:        model.FlowStatusRunning,
				CurrentNodeID: model.FlowNodeSpecDraft,
			}, nil
		}
		if loadCalls == 2 {
			return FlowRuntimeState{
				RunID:         flowRunID,
				Status:        model.FlowStatusAwaitingInput,
				CurrentNodeID: model.FlowNodeSpecDraft,
			}, nil
		}
		return FlowRuntimeState{
			RunID:         flowRunID,
			Status:        model.FlowStatusCompleted,
			CurrentNodeID: model.FlowNodeDone,
		}, nil
	}, activity.RegisterOptions{Name: "FlowRuntimeActivities.LoadRunStateActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, flowRunID, actorID string) error {
		finalizeCalls++
		return nil
	}, activity.RegisterOptions{Name: "FlowRuntimeActivities.FinalizeInteractiveNodeActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, flowRunID string) (FlowProgressResult, error) {
		return FlowProgressResult{
			State: FlowRuntimeState{
				RunID:         flowRunID,
				Status:        model.FlowStatusRunning,
				CurrentNodeID: model.FlowNodeSpecDraft,
			},
			WaitingChild: false,
		}, nil
	}, activity.RegisterOptions{Name: "FlowRuntimeActivities.ProgressRunStateActivity"})

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(WorkflowSignalFlowRun, FlowRunSignal{
			Type:    FlowSignalTypeRefresh,
			ActorID: "user-1",
		})
		env.SignalWorkflow(WorkflowSignalFlowRun, FlowRunSignal{
			Type:    FlowSignalTypeNodeAction,
			Action:  model.FlowActionFinalize,
			ActorID: "user-1",
		})
	}, time.Second)

	env.ExecuteWorkflow(FlowRunWorkflow, FlowRunWorkflowInput{
		FlowRunID: "flow-1",
		ActorID:   "user-1",
	})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("expected workflow success, got %v", err)
	}
	if finalizeCalls != 1 {
		t.Fatalf("expected finalize activity to run once, got %d", finalizeCalls)
	}
}
