package temporalapp

import (
	"context"
	"testing"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestDailyContentSourceScheduleIsStable(t *testing.T) {
	for _, startErr := range []error{nil, serviceerror.NewWorkflowExecutionAlreadyStarted("exists", "request", "run")} {
		client := &scheduledEventsTestClient{err: startErr}
		if err := NewRunEngine(client, "test").EnsureDailyContentSourceSync(context.Background()); err != nil {
			t.Fatal(err)
		}
		if client.options.ID != "support-content-daily-sync" || client.options.CronSchedule != "0 2 * * *" || client.options.TaskQueue != QueueAutomation {
			t.Fatalf("daily schedule=%+v", client.options)
		}
	}
}

func TestDailyContentSourceWorkflowQueuesRefreshes(t *testing.T) {
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	calls := 0
	env.RegisterActivityWithOptions(func(context.Context) error { calls++; return nil }, activity.RegisterOptions{Name: "ContentSourceSyncActivities.QueueDailySourceSync"})
	env.ExecuteWorkflow(DailyContentSourceSyncWorkflow)
	if !env.IsWorkflowCompleted() || env.GetWorkflowError() != nil || calls != 1 {
		t.Fatalf("daily workflow calls=%d error=%v", calls, env.GetWorkflowError())
	}
}
