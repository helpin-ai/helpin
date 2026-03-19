package temporalapp

import (
	"context"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

type fakeSprintRunner struct {
	called int
}

func (f *fakeSprintRunner) RunSprintAutomations(ctx context.Context) {
	f.called++
}

func TestSprintCronWorkflow_ExecutesActivity(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	runner := &fakeSprintRunner{}
	activities := NewSprintAutomationActivities(runner)

	env.RegisterActivityWithOptions(activities.RunSprintAutomationsActivity, activity.RegisterOptions{
		Name: "SprintAutomationActivities.RunSprintAutomationsActivity",
	})

	env.ExecuteWorkflow(SprintAutomationCronWorkflow, SprintAutomationInput{})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	if runner.called != 1 {
		t.Errorf("expected RunSprintAutomations to be called 1 time, got %d", runner.called)
	}
}
