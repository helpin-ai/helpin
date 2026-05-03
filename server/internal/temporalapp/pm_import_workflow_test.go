package temporalapp

import (
	"context"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestShortcutImportWorkflowExecutesActivity(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	receivedImportID := ""
	env.RegisterActivityWithOptions(func(ctx context.Context, input ShortcutImportWorkflowInput) error {
		receivedImportID = input.ImportID
		return nil
	}, activity.RegisterOptions{Name: "PMImportActivities.ExecuteShortcutAPIImportActivity"})

	env.ExecuteWorkflow(ShortcutImportWorkflow, ShortcutImportWorkflowInput{ImportID: "import-123"})

	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	if receivedImportID != "import-123" {
		t.Fatalf("expected activity import id import-123, got %q", receivedImportID)
	}
}

func TestWorkflowIDForShortcutImportTrimsImportID(t *testing.T) {
	got := WorkflowIDForShortcutImport(" import-123 ")
	if got != "pm-import-shortcut-import-123" {
		t.Fatalf("unexpected workflow id %q", got)
	}
}
