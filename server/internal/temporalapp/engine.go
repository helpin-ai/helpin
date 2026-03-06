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
