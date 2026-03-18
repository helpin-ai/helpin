package service

import (
	"context"
	"errors"
	"fmt"

	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"

	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

type recurringTemplateWorkflowRunner interface {
	StartScheduler(ctx context.Context) error
}

type temporalRecurringTemplateWorkflowRunner struct {
	client tclient.Client
}

func (r *temporalRecurringTemplateWorkflowRunner) StartScheduler(ctx context.Context) error {
	if r == nil || r.client == nil {
		return nil
	}
	_, err := r.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:           temporalapp.PMRecurringSchedulerWorkflowID,
		TaskQueue:    temporalapp.QueueAutomation,
		CronSchedule: temporalapp.PMRecurringSchedulerCronSchedule,
	}, temporalapp.PMRecurringTemplateSchedulerWorkflow)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("start recurring scheduler workflow: %w", err)
	}
	return nil
}

// PMRecurringTemplateActivities adapts the service for Temporal worker registration.
type PMRecurringTemplateActivities struct {
	recurringService *PMRecurringTemplateService
}

func NewPMRecurringTemplateActivities(recurringService *PMRecurringTemplateService) *PMRecurringTemplateActivities {
	return &PMRecurringTemplateActivities{recurringService: recurringService}
}

func (a *PMRecurringTemplateActivities) ProcessDueTemplatesActivity(ctx context.Context) (int, error) {
	if a == nil || a.recurringService == nil {
		return 0, nil
	}
	return a.recurringService.ProcessDueTemplates(ctx, 50)
}

func (s *PMRecurringTemplateService) SetWorkflowRunner(runner recurringTemplateWorkflowRunner) {
	if s == nil {
		return
	}
	if runner == nil {
		s.workflowRunner = nil
		return
	}
	s.workflowRunner = runner
}

func (s *PMRecurringTemplateService) SetTemporalClient(client tclient.Client) {
	if s == nil {
		return
	}
	if client == nil {
		s.workflowRunner = nil
		return
	}
	s.workflowRunner = &temporalRecurringTemplateWorkflowRunner{client: client}
}

func (s *PMRecurringTemplateService) EnsureScheduler(ctx context.Context) error {
	if s == nil || s.workflowRunner == nil {
		return nil
	}
	return s.workflowRunner.StartScheduler(ctx)
}
