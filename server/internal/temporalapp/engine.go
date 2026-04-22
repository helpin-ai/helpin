package temporalapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// RunEngine starts and signals Temporal workflows for agent runs.
type RunEngine struct {
	client    tclient.Client
	namespace string
}

// RunnerQueueHealth summarizes the state of a shared Temporal task queue.
type RunnerQueueHealth struct {
	Name                 string     `json:"name"`
	Concurrency          int        `json:"concurrency"`
	QueuedRuns           int        `json:"queued_runs"`
	RunningRuns          int        `json:"running_runs"`
	AwaitingApprovalRuns int        `json:"awaiting_approval_runs"`
	ActiveRuns           int        `json:"active_runs"`
	LatestHeartbeatAt    *time.Time `json:"latest_heartbeat_at,omitempty"`
}

// RunnerActiveRun summarizes an in-flight run visible to operators.
type RunnerActiveRun struct {
	ID              string     `json:"id"`
	AgentID         string     `json:"agent_id"`
	TargetType      string     `json:"target_type"`
	TargetID        string     `json:"target_id"`
	Status          string     `json:"status"`
	TaskQueue       string     `json:"task_queue"`
	RunnerPool      string     `json:"runner_pool"`
	ExecutionStage  *string    `json:"execution_stage,omitempty"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	WorkflowID      *string    `json:"workflow_id,omitempty"`
	Stale           bool       `json:"stale"`
}

// RunnerHealth is the operator-facing payload for shared runner pools.
type RunnerHealth struct {
	Namespace          string              `json:"namespace"`
	TemporalConfigured bool                `json:"temporal_configured"`
	GeneratedAt        time.Time           `json:"generated_at"`
	Queues             []RunnerQueueHealth `json:"queues"`
	ActiveRuns         []RunnerActiveRun   `json:"active_runs"`
}

// WorkflowExecutionState summarizes a Temporal workflow execution.
type WorkflowExecutionState struct {
	Exists bool
	Open   bool
	Status enumspb.WorkflowExecutionStatus
}

// NewRunEngine creates a Temporal-backed run engine.
func NewRunEngine(client tclient.Client, namespace string) *RunEngine {
	if client == nil {
		return nil
	}
	return &RunEngine{client: client, namespace: namespace}
}

// StartRun starts the workflow for a queued run.
func (e *RunEngine) StartRun(ctx context.Context, run *model.AgentRun) (string, string, error) {
	if e == nil || e.client == nil {
		return "", "", fmt.Errorf("temporal run engine is not configured")
	}
	workflowID := WorkflowIDForRun(run.ID)
	options := tclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: deref(run.TaskQueue, QueueAutomation),
	}
	we, err := e.client.ExecuteWorkflow(ctx, options, AgentRunWorkflow, AgentRunWorkflowInput{
		RunID: run.ID,
	})
	if err != nil {
		return "", "", fmt.Errorf("start temporal workflow: %w", err)
	}
	return we.GetID(), we.GetRunID(), nil
}

// CancelRun cancels an in-flight workflow.
func (e *RunEngine) CancelRun(ctx context.Context, workflowID, workflowRunID string) error {
	if e == nil || e.client == nil || workflowID == "" {
		return nil
	}
	return e.client.CancelWorkflow(ctx, workflowID, workflowRunID)
}

// SignalApprove signals approval to a waiting workflow.
func (e *RunEngine) SignalApprove(ctx context.Context, workflowID, workflowRunID string) error {
	return e.SignalResume(ctx, workflowID, workflowRunID, RunResumeSignal{
		Intent: model.AgentRunResumeIntentApprove,
	})
}

// SignalHandoff notifies a workflow that a handoff was recorded.
func (e *RunEngine) SignalHandoff(ctx context.Context, workflowID, workflowRunID string, payload model.HandoffAgentRunRequest) error {
	if e == nil || e.client == nil || workflowID == "" {
		return nil
	}
	return e.client.SignalWorkflow(ctx, workflowID, workflowRunID, WorkflowSignalHandoff, payload)
}

// SignalMessage resumes an awaiting-input workflow with a new user message.
func (e *RunEngine) SignalMessage(ctx context.Context, workflowID, workflowRunID, content string) error {
	return e.SignalResume(ctx, workflowID, workflowRunID, RunResumeSignal{
		Intent:  model.AgentRunResumeIntentReply,
		Content: content,
	})
}

// SignalResume resumes a paused workflow with a generic human intent.
func (e *RunEngine) SignalResume(ctx context.Context, workflowID, workflowRunID string, payload RunResumeSignal) error {
	if e == nil || e.client == nil {
		return nil
	}
	workflowID = strings.TrimSpace(workflowID)
	workflowRunID = strings.TrimSpace(workflowRunID)
	if workflowID == "" {
		return fmt.Errorf("temporal workflow id is required")
	}
	err := e.client.SignalWorkflow(ctx, workflowID, workflowRunID, WorkflowSignalResume, payload)
	if shouldRetrySignalWithoutRunID(err, workflowRunID) {
		return e.client.SignalWorkflow(ctx, workflowID, "", WorkflowSignalResume, payload)
	}
	return err
}

func shouldRetrySignalWithoutRunID(err error, workflowRunID string) bool {
	if err == nil || strings.TrimSpace(workflowRunID) == "" {
		return false
	}
	var notFound *serviceerror.NotFound
	return errors.As(err, &notFound)
}

// DescribeRun returns the Temporal execution state for a workflow-backed run.
func (e *RunEngine) DescribeRun(ctx context.Context, workflowID, workflowRunID string) (WorkflowExecutionState, error) {
	if e == nil || e.client == nil || workflowID == "" {
		return WorkflowExecutionState{}, nil
	}
	resp, err := e.client.DescribeWorkflowExecution(ctx, workflowID, workflowRunID)
	if err != nil {
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) {
			return WorkflowExecutionState{}, nil
		}
		return WorkflowExecutionState{}, err
	}

	status := enumspb.WORKFLOW_EXECUTION_STATUS_UNSPECIFIED
	if resp != nil && resp.WorkflowExecutionInfo != nil {
		status = resp.WorkflowExecutionInfo.Status
	}
	if status == enumspb.WORKFLOW_EXECUTION_STATUS_UNSPECIFIED {
		status = enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING
	}

	return WorkflowExecutionState{
		Exists: true,
		Open:   status == enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING,
		Status: status,
	}, nil
}

// Health returns the configured shared-runner metadata.
func (e *RunEngine) Health() RunnerHealth {
	queues := SharedQueues()
	queueHealth := make([]RunnerQueueHealth, 0, len(queues))
	for _, queue := range queues {
		queueHealth = append(queueHealth, RunnerQueueHealth{
			Name:        queue.Name,
			Concurrency: queue.Concurrency,
		})
	}
	return RunnerHealth{
		Namespace:          e.namespace,
		TemporalConfigured: e != nil && e.client != nil,
		GeneratedAt:        time.Now(),
		Queues:             queueHealth,
		ActiveRuns:         []RunnerActiveRun{},
	}
}

// StartRuleSchedule starts a cron-scheduled workflow for an automation rule.
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

// WorkflowIDForRun returns the temporal workflow ID for a run.
func WorkflowIDForRun(runID string) string {
	return "agent-run-" + runID
}

func deref(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}
