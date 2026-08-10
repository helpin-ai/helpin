package temporalapp

import (
	"context"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestDocsImportWorkflowExecutesDurableActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(
		func(context.Context, DocsImportWorkflowInput) error { return nil },
		activity.RegisterOptions{Name: DocsImportExecuteActivityName},
	)

	env.ExecuteWorkflow(DocsImportWorkflow, DocsImportWorkflowInput{ImportID: "import-123"})

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("DocsImportWorkflow() error = %v", err)
	}
	if got := WorkflowIDForDocsImport(" import-123 "); got != "docs-import-helpscout-import-123" {
		t.Fatalf("workflow id = %q", got)
	}
}
