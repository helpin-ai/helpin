package temporalapp

import (
	"context"
	"errors"
	"testing"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/activity"
	tclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
)

type scheduledEventsTestDispatcher struct {
	calls int
	err   error
}

func (d *scheduledEventsTestDispatcher) DispatchDue(context.Context) error {
	d.calls++
	return d.err
}

func TestScheduledEventsWorkflowDispatchAndRetry(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		wantCalls int
	}{
		{name: "success", wantCalls: 1},
		{name: "bounded dispatcher retries", err: errors.New("database unavailable"), wantCalls: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			suite := &testsuite.WorkflowTestSuite{}
			env := suite.NewTestWorkflowEnvironment()
			dispatcher := &scheduledEventsTestDispatcher{err: tc.err}
			activities := NewScheduledEventsActivities(dispatcher)
			env.RegisterActivityWithOptions(activities.DispatchDue, activity.RegisterOptions{Name: "ScheduledEventsActivities.DispatchDue"})
			env.ExecuteWorkflow(ScheduledEventsWorkflow)
			if !env.IsWorkflowCompleted() || (env.GetWorkflowError() != nil) != (tc.err != nil) || dispatcher.calls != tc.wantCalls {
				t.Fatalf("unexpected workflow receipt: calls=%d err=%v", dispatcher.calls, env.GetWorkflowError())
			}
		})
	}
}

func TestScheduledEventsWorkflowMissingDispatcherFailsWithoutRetry(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	calls := 0
	activities := NewScheduledEventsActivities(nil)
	env.RegisterActivityWithOptions(func(ctx context.Context) error { calls++; return activities.DispatchDue(ctx) }, activity.RegisterOptions{Name: "ScheduledEventsActivities.DispatchDue"})
	env.ExecuteWorkflow(ScheduledEventsWorkflow)
	if !env.IsWorkflowCompleted() || env.GetWorkflowError() == nil || calls != 1 {
		t.Fatalf("missing dispatcher was treated as success or retried: calls=%d err=%v", calls, env.GetWorkflowError())
	}
}

type scheduledEventsTestClient struct {
	tclient.Client
	options tclient.StartWorkflowOptions
	args    []interface{}
	err     error
}

func (c *scheduledEventsTestClient) ExecuteWorkflow(_ context.Context, options tclient.StartWorkflowOptions, _ interface{}, args ...interface{}) (tclient.WorkflowRun, error) {
	c.options, c.args = options, args
	return nil, c.err
}

func TestEnsureScheduledEventsStableIdentity(t *testing.T) {
	failure := errors.New("temporal unavailable")
	for _, err := range []error{nil, serviceerror.NewWorkflowExecutionAlreadyStarted("already scheduled", "request", "run"), failure} {
		client := &scheduledEventsTestClient{err: err}
		engine := NewRunEngine(client, "test")
		result := engine.EnsureScheduledEvents(context.Background())
		if (result != nil) != errors.Is(err, failure) {
			t.Fatalf("startup error handling: %v", result)
		}
		if client.options.ID != ScheduledEventsWorkflowID || client.options.TaskQueue != QueueAutomation || client.options.CronSchedule != "* * * * *" || len(client.args) != 0 {
			t.Fatalf("shared schedule changed user Flow configuration: %#v", client.options)
		}
	}
	if err := (*RunEngine)(nil).EnsureScheduledEvents(context.Background()); err == nil {
		t.Fatal("missing engine reported success")
	}
}
