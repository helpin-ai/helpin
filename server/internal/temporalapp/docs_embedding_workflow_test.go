package temporalapp

import (
	"context"
	"testing"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

type fakeDocsEmbeddingRunner struct {
	calls []DocsEmbeddingSyncInput
}

func (f *fakeDocsEmbeddingRunner) RunSpaceSync(ctx context.Context, workspaceID, spaceID string) error {
	f.calls = append(f.calls, DocsEmbeddingSyncInput{
		WorkspaceID: workspaceID,
		SpaceID:     spaceID,
	})
	return nil
}

func TestDocsEmbeddingSyncWorkflow_ExecutesFromSignal(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	runner := &fakeDocsEmbeddingRunner{}
	activities := NewDocsEmbeddingActivities(runner)
	env.RegisterActivityWithOptions(activities.SyncSpaceActivity, activity.RegisterOptions{
		Name: "DocsEmbeddingActivities.SyncSpaceActivity",
	})
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(WorkflowSignalDocsEmbeddingSync, DocsEmbeddingSyncInput{
			WorkspaceID: "ws_123",
			SpaceID:     "space_123",
		})
	}, time.Millisecond)

	env.ExecuteWorkflow(DocsEmbeddingSyncWorkflow, DocsEmbeddingSyncInput{})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 sync activity call, got %d", len(runner.calls))
	}
	if runner.calls[0].WorkspaceID != "ws_123" || runner.calls[0].SpaceID != "space_123" {
		t.Fatalf("unexpected sync input: %+v", runner.calls[0])
	}
}
