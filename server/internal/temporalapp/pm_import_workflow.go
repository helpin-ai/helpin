package temporalapp

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"
)

type ShortcutImportWorkflowInput struct {
	ImportID string `json:"import_id"`
}

func WorkflowIDForShortcutImport(importID string) string {
	return fmt.Sprintf("pm-import-shortcut-%s", strings.TrimSpace(importID))
}

func ShortcutImportWorkflow(ctx workflow.Context, input ShortcutImportWorkflowInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Hour,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	return workflow.ExecuteActivity(ctx, "PMImportActivities.ExecuteShortcutAPIImportActivity", input).Get(ctx, nil)
}
