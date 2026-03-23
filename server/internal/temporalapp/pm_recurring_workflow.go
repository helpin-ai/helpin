package temporalapp

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

const PMRecurringSchedulerWorkflowID = "pm-recurring-template-scheduler"
const PMRecurringSchedulerCronSchedule = "*/5 * * * *"

// PMRecurringTemplateSchedulerWorkflow periodically processes due recurring templates.
func PMRecurringTemplateSchedulerWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var processed int
	return workflow.ExecuteActivity(ctx, "PMRecurringTemplateActivities.ProcessDueTemplatesActivity").Get(ctx, &processed)
}
