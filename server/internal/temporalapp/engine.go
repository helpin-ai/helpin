package temporalapp

import (
	"context"
	"errors"
	"fmt"
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

// SignalMessage resumes an awaiting-input workflow with a new user message.
func (e *RunEngine) SignalMessage(ctx context.Context, workflowID, workflowRunID, content string) error {
	if e == nil || e.client == nil || workflowID == "" {
		return nil
	}
	return e.client.SignalWorkflow(ctx, workflowID, workflowRunID, WorkflowSignalMessage, RunMessageSignal{
		Content: content,
	})
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
