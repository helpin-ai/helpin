package temporalapp

import (
	"context"
	"fmt"
	"time"

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
	if e == nil || e.client == nil || workflowID == "" {
		return nil
	}
	return e.client.SignalWorkflow(ctx, workflowID, workflowRunID, WorkflowSignalApprove, struct{}{})
}

// SignalHandoff notifies a workflow that a handoff was recorded.
func (e *RunEngine) SignalHandoff(ctx context.Context, workflowID, workflowRunID string, payload model.HandoffAgentRunRequest) error {
	if e == nil || e.client == nil || workflowID == "" {
		return nil
	}
	return e.client.SignalWorkflow(ctx, workflowID, workflowRunID, WorkflowSignalHandoff, payload)
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

// StartSchedule starts a cron-scheduled workflow for an agent.
func (e *RunEngine) StartSchedule(ctx context.Context, agentID, workspaceID, schedule string) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}
	workflowID := WorkflowIDForSchedule(agentID)
	options := tclient.StartWorkflowOptions{
		ID:           workflowID,
		TaskQueue:    QueueAutomation,
		CronSchedule: schedule,
	}
	_, err := e.client.ExecuteWorkflow(ctx, options, ScheduledAgentWorkflow, ScheduledAgentInput{
		AgentID:     agentID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return fmt.Errorf("start schedule workflow: %w", err)
	}
	return nil
}

// StopSchedule terminates a cron-scheduled workflow for an agent.
// Silently ignores errors if no schedule workflow exists.
func (e *RunEngine) StopSchedule(ctx context.Context, agentID string) error {
	if e == nil || e.client == nil {
		return nil
	}
	workflowID := WorkflowIDForSchedule(agentID)
	_ = e.client.TerminateWorkflow(ctx, workflowID, "", "schedule removed")
	return nil
}

// StartPlanningSession starts a Temporal workflow for an interactive planning session.
func (e *RunEngine) StartPlanningSession(ctx context.Context, sessionID string) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}
	workflowID := WorkflowIDForPlanningSession(sessionID)
	options := tclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: QueuePlanningInteractive,
	}
	_, err := e.client.ExecuteWorkflow(ctx, options, PlanningSessionWorkflow, PlanningSessionWorkflowInput{
		SessionID: sessionID,
	})
	if err != nil {
		return fmt.Errorf("start planning session workflow: %w", err)
	}
	return nil
}

// SignalPlanningSession sends a signal (message, finalize, abandon) to a running planning session workflow.
func (e *RunEngine) SignalPlanningSession(ctx context.Context, sessionID string, signal PlanningSessionSignal) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}
	workflowID := WorkflowIDForPlanningSession(sessionID)
	return e.client.SignalWorkflow(ctx, workflowID, "", WorkflowSignalPlanningSession, signal)
}

// CancelPlanningSession cancels a planning session workflow.
func (e *RunEngine) CancelPlanningSession(ctx context.Context, sessionID string) error {
	if e == nil || e.client == nil {
		return nil
	}
	workflowID := WorkflowIDForPlanningSession(sessionID)
	return e.client.CancelWorkflow(ctx, workflowID, "")
}

// StartFlowRun starts the Temporal workflow for a durable flow run.
func (e *RunEngine) StartFlowRun(ctx context.Context, flowRunID, actorID string) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}
	options := tclient.StartWorkflowOptions{
		ID:                       WorkflowIDForFlowRun(flowRunID),
		TaskQueue:                QueueFlowOrchestrator,
		WorkflowExecutionTimeout: FlowMaxLifetime,
	}
	_, err := e.client.ExecuteWorkflow(ctx, options, FlowRunWorkflow, FlowRunWorkflowInput{
		FlowRunID: flowRunID,
		ActorID:   actorID,
	})
	if err != nil {
		return fmt.Errorf("start flow workflow: %w", err)
	}
	return nil
}

// TerminateFlowRun forcefully terminates a flow run workflow.
func (e *RunEngine) TerminateFlowRun(ctx context.Context, flowRunID, reason string) error {
	if e == nil || e.client == nil {
		return nil
	}
	workflowID := WorkflowIDForFlowRun(flowRunID)
	return e.client.TerminateWorkflow(ctx, workflowID, "", reason)
}

// SignalFlowRun notifies an in-flight flow workflow of a user action.
func (e *RunEngine) SignalFlowRun(ctx context.Context, flowRunID string, signal FlowRunSignal) error {
	if e == nil || e.client == nil {
		return fmt.Errorf("temporal run engine is not configured")
	}
	return e.client.SignalWorkflow(ctx, WorkflowIDForFlowRun(flowRunID), "", WorkflowSignalFlowRun, signal)
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
