package temporalapp

import (
	"context"
	"errors"
	"testing"

	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

type meetingFollowUpTestRouter struct{ calls int }

func (r *meetingFollowUpTestRouter) BackfillFollowUpRouting(context.Context) error {
	r.calls++
	return nil
}

func TestMeetingFollowUpRoutingHasIndependentScheduleAndBoundedActivity(t *testing.T) {
	client := &scheduledEventsTestClient{}
	if err := NewRunEngine(client, "test").EnsureMeetingFollowUpRouting(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.options.ID != CRMMeetingFollowUpRoutingWorkflowID || client.options.ID == ScheduledEventsWorkflowID || client.options.CronSchedule != "*/5 * * * *" || client.options.TaskQueue != QueueAutomation {
		t.Fatalf("routing delays product scheduler: %+v", client.options)
	}
	client.err = serviceerror.NewWorkflowExecutionAlreadyStarted("already running", "request", "run")
	if err := NewRunEngine(client, "test").EnsureMeetingFollowUpRouting(context.Background()); err != nil {
		t.Fatal(err)
	}
	client.err = errors.New("unavailable")
	if err := NewRunEngine(client, "test").EnsureMeetingFollowUpRouting(context.Background()); err == nil {
		t.Fatal("schedule creation failure hidden")
	}
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	router := &meetingFollowUpTestRouter{}
	activities := NewCRMMeetingActivities(nil).SetFollowUpRouter(router)
	env.RegisterActivityWithOptions(activities.RouteMeetingFollowUps, activity.RegisterOptions{Name: "CRMMeetingActivities.RouteMeetingFollowUps"})
	env.ExecuteWorkflow(CRMMeetingFollowUpRoutingWorkflow)
	if !env.IsWorkflowCompleted() || env.GetWorkflowError() != nil || router.calls != 1 {
		t.Fatalf("routing calls=%d err=%v", router.calls, env.GetWorkflowError())
	}
}
