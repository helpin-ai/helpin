package temporalapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ScheduledEventsWorkflowID is namespace-scoped, independent of user Flow schedules.
const ScheduledEventsWorkflowID = "automation-scheduled-events"

// ScheduledEventsWorkflow drains due product events on the existing Automation queue.
// The database is the outbox and attempt authority; Temporal provides durable ticks.
func ScheduledEventsWorkflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: 5 * time.Second, MaximumInterval: 15 * time.Second, MaximumAttempts: 3,
		},
	})
	return workflow.ExecuteActivity(ctx, "ScheduledEventsActivities.DispatchDue").Get(ctx, nil)
}

// ScheduledEventsDispatcher processes a bounded batch of shared product events.
type ScheduledEventsDispatcher interface {
	DispatchDue(context.Context) error
}

// ScheduledEventsActivities hosts the existing product scheduler's event dispatcher.
type ScheduledEventsActivities struct {
	dispatcher ScheduledEventsDispatcher
	additional ScheduledEventsDispatcher
}

// NewScheduledEventsActivities binds explicit product-event handlers through the dispatcher.
func NewScheduledEventsActivities(dispatcher ScheduledEventsDispatcher) *ScheduledEventsActivities {
	return &ScheduledEventsActivities{dispatcher: dispatcher}
}

// SetAdditionalDispatcher adds a bounded product dispatcher to each scheduler tick.
func (a *ScheduledEventsActivities) SetAdditionalDispatcher(dispatcher ScheduledEventsDispatcher) *ScheduledEventsActivities {
	a.additional = dispatcher
	return a
}

// DispatchDue fails visibly if the dispatcher is missing instead of claiming success.
func (a *ScheduledEventsActivities) DispatchDue(ctx context.Context) error {
	if a == nil || a.dispatcher == nil {
		return temporal.NewNonRetryableApplicationError("scheduled event dispatcher is not configured", "scheduled_events_unavailable", nil)
	}
	err := a.dispatcher.DispatchDue(ctx)
	if a.additional != nil {
		err = errors.Join(err, a.additional.DispatchDue(ctx))
	}
	return err
}

// EnsureScheduledEvents starts a stable shared schedule without modifying user Flows.
func (e *RunEngine) EnsureScheduledEvents(ctx context.Context) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}
	_, err := e.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID: ScheduledEventsWorkflowID, TaskQueue: QueueAutomation, CronSchedule: "* * * * *",
	}, ScheduledEventsWorkflow)
	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ensure shared scheduled events: %w", err)
	}
	return nil
}
