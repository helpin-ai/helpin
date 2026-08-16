package temporalapp

import (
	"context"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestCRMMeetingCaptureScheduleWorkflowStartsDueMeeting(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	calls := 0
	env.RegisterActivityWithOptions(func(context.Context, CRMMeetingProcessingInput) error {
		calls++
		return nil
	}, activity.RegisterOptions{Name: CRMMeetingStartScheduledActivityName})

	env.ExecuteWorkflow(CRMMeetingCaptureScheduleWorkflow, CRMMeetingCaptureScheduleInput{
		WorkspaceID: "workspace-1",
		MeetingID:   "meeting-1",
		JoinAt:      time.Now().UTC().Add(-time.Minute),
	})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("activity calls = %d, want 1", calls)
	}
}

func TestCRMMeetingCaptureScheduleWorkflowCanBeCancelled(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(context.Context, CRMMeetingProcessingInput) error {
		t.Fatal("capture activity should not run after cancellation")
		return nil
	}, activity.RegisterOptions{Name: CRMMeetingStartScheduledActivityName})
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(CRMMeetingCaptureScheduleSignal, CRMMeetingCaptureScheduleCommand{MeetingID: "meeting-2", Cancel: true})
	}, time.Minute)

	env.ExecuteWorkflow(CRMMeetingCaptureScheduleWorkflow, CRMMeetingCaptureScheduleInput{
		WorkspaceID: "workspace-1",
		MeetingID:   "meeting-2",
		JoinAt:      time.Now().UTC().Add(time.Hour),
	})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}
}
