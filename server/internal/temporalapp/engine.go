package temporalapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
)

// RunEngine starts Helpin product automation workflows.
type RunEngine struct {
	client    tclient.Client
	namespace string
}

// NewRunEngine creates a Temporal-backed product automation engine.
func NewRunEngine(client tclient.Client, namespace string) *RunEngine {
	if client == nil {
		return nil
	}
	return &RunEngine{client: client, namespace: namespace}
}

func (e *RunEngine) StartRuleSchedule(ctx context.Context, ruleID, workspaceID, schedule string) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}
	workflowID := WorkflowIDForRuleSchedule(ruleID)
	options := tclient.StartWorkflowOptions{
		ID:           workflowID,
		TaskQueue:    QueueAutomation,
		CronSchedule: schedule,
	}
	_, err := e.client.ExecuteWorkflow(ctx, options, ScheduledRuleWorkflow, ScheduledRuleInput{
		RuleID:      ruleID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("start rule schedule workflow: %w", err)
	}
	return nil
}

// StopRuleSchedule terminates a cron-scheduled workflow for an automation rule.
func (e *RunEngine) StopRuleSchedule(ctx context.Context, ruleID string) error {
	if e == nil || e.client == nil {
		return nil
	}
	workflowID := WorkflowIDForRuleSchedule(ruleID)
	_ = e.client.TerminateWorkflow(ctx, workflowID, "", "rule schedule removed")
	return nil
}

// QueueDocsEmbeddingSync enqueues or signals a durable help-center embedding sync for one space.
func (e *RunEngine) QueueDocsEmbeddingSync(ctx context.Context, workspaceID, spaceID string) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}

	request := DocsEmbeddingSyncInput{
		WorkspaceID: workspaceID,
		SpaceID:     spaceID,
	}
	options := tclient.StartWorkflowOptions{
		ID:                       WorkflowIDForDocsEmbeddingSpace(workspaceID, spaceID),
		TaskQueue:                QueueAutomation,
		WorkflowExecutionTimeout: 2 * time.Hour,
	}
	_, err := e.client.SignalWithStartWorkflow(
		ctx,
		options.ID,
		WorkflowSignalDocsEmbeddingSync,
		request,
		options,
		DocsEmbeddingSyncWorkflow,
		DocsEmbeddingSyncInput{},
	)
	if err != nil {
		return fmt.Errorf("queue docs embedding sync: %w", err)
	}
	return nil
}

// QueueContentSourceSync enqueues or signals a durable content crawl+embedding sync for one source.
func (e *RunEngine) QueueContentSourceSync(ctx context.Context, workspaceID, contentSourceID string) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}

	request := ContentSourceSyncInput{
		WorkspaceID:     workspaceID,
		ContentSourceID: contentSourceID,
	}
	options := tclient.StartWorkflowOptions{
		ID:                       WorkflowIDForContentSource(workspaceID, contentSourceID),
		TaskQueue:                QueueAutomation,
		WorkflowExecutionTimeout: 2 * time.Hour,
	}
	_, err := e.client.SignalWithStartWorkflow(
		ctx,
		options.ID,
		WorkflowSignalContentSourceSync,
		request,
		options,
		ContentSourceSyncWorkflow,
		ContentSourceSyncInput{},
	)
	if err != nil {
		return fmt.Errorf("queue content source sync: %w", err)
	}
	return nil
}

// QueueContentSourceReindex enqueues a reindex job that re-chunks and
// re-embeds existing pages without re-crawling the website.
func (e *RunEngine) QueueContentSourceReindex(ctx context.Context, workspaceID, contentSourceID string) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}

	request := ContentSourceSyncInput{
		WorkspaceID:     workspaceID,
		ContentSourceID: contentSourceID,
		Reindex:         true,
	}
	options := tclient.StartWorkflowOptions{
		ID:                       WorkflowIDForContentSource(workspaceID, contentSourceID),
		TaskQueue:                QueueAutomation,
		WorkflowExecutionTimeout: 2 * time.Hour,
	}
	_, err := e.client.SignalWithStartWorkflow(
		ctx,
		options.ID,
		WorkflowSignalContentSourceSync,
		request,
		options,
		ContentSourceSyncWorkflow,
		ContentSourceSyncInput{},
	)
	if err != nil {
		return fmt.Errorf("queue content source reindex: %w", err)
	}
	return nil
}
