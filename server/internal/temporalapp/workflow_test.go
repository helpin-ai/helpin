package temporalapp

import (
	"context"
	"errors"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestAgentRunWorkflowPrepareFailureMarksRunFailed(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	markedRunID := ""
	markedError := ""

	env.RegisterWorkflow(AgentRunWorkflow)
	env.RegisterActivityWithOptions(func(ctx context.Context, runID string) error {
		return errors.New("prepare exploded")
	}, activity.RegisterOptions{Name: "AgentRunActivities.PrepareRunActivity"})
	env.RegisterActivityWithOptions(func(ctx context.Context, runID, errMsg string) error {
		markedRunID = runID
		markedError = errMsg
		return nil
	}, activity.RegisterOptions{Name: "AgentRunActivities.MarkRunFailedActivity"})

	env.ExecuteWorkflow(AgentRunWorkflow, AgentRunWorkflowInput{RunID: "run-1"})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if env.GetWorkflowError() == nil {
		t.Fatal("expected workflow error")
	}
	if markedRunID != "run-1" {
		t.Fatalf("expected failure marker for run-1, got %q", markedRunID)
	}
	if markedError == "" {
		t.Fatal("expected failure marker to receive the workflow error")
	}
}
