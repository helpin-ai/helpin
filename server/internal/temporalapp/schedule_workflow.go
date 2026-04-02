package temporalapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/workflow"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

// ScheduledAgentInput identifies the agent for each cron tick.
type ScheduledAgentInput struct {
	AgentID     string `json:"agent_id"`
	WorkspaceID string `json:"workspace_id"`
}

// ScheduledRunResult is returned by CreateScheduledRun with the info needed
// to start the child agent-run workflow.
type ScheduledRunResult struct {
	RunID     string `json:"run_id"`
	TaskQueue string `json:"task_queue"`
}

// ScheduledAgentWorkflow is a Temporal cron workflow. Each execution creates
// one agent run and dispatches it as a detached child workflow.
func ScheduledAgentWorkflow(ctx workflow.Context, input ScheduledAgentInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result ScheduledRunResult
	err := workflow.ExecuteActivity(ctx, "ScheduledAgentActivities.CreateScheduledRun", input).Get(ctx, &result)
	if err != nil {
		return err
	}

	// Fire-and-forget: start the agent run as a detached child so the cron
	// workflow completes immediately and the next tick can fire on schedule.
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:        WorkflowIDForRun(result.RunID),
		TaskQueue:         result.TaskQueue,
		ParentClosePolicy: enumspb.PARENT_CLOSE_POLICY_ABANDON,
	})
	workflow.ExecuteChildWorkflow(childCtx, AgentRunWorkflow, AgentRunWorkflowInput{RunID: result.RunID})
	return nil
}

// WorkflowIDForSchedule returns the Temporal workflow ID for an agent schedule.
func WorkflowIDForSchedule(agentID string) string {
	return "agent-schedule-" + agentID
}

// ---------------------------------------------------------------------------
// Activities
// ---------------------------------------------------------------------------

// ScheduledAgentActivities creates agent runs for cron-triggered agents.
type ScheduledAgentActivities struct {
	agentRepo *repository.AgentRepository
	runRepo   *repository.AgentRunRepository
}

// NewScheduledAgentActivities creates a new ScheduledAgentActivities.
func NewScheduledAgentActivities(
	agentRepo *repository.AgentRepository,
	runRepo *repository.AgentRunRepository,
) *ScheduledAgentActivities {
	return &ScheduledAgentActivities{
		agentRepo: agentRepo,
		runRepo:   runRepo,
	}
}

// CreateScheduledRun validates the agent is still scheduled, prevents
// overlapping runs, creates a new AgentRun record, and returns its ID + queue.
func (a *ScheduledAgentActivities) CreateScheduledRun(ctx context.Context, input ScheduledAgentInput) (ScheduledRunResult, error) {
	agent, err := a.agentRepo.GetByID(ctx, input.WorkspaceID, input.AgentID)
	if err != nil || agent == nil {
		return ScheduledRunResult{}, fmt.Errorf("agent not found")
	}
	if agent.Status == "disabled" || agent.Schedule == nil || *agent.Schedule == "" {
		return ScheduledRunResult{}, fmt.Errorf("agent schedule removed, skipping")
	}

	// Prevent overlapping runs for the same scheduled agent.
	activeRun, err := a.runRepo.FindActiveByTarget(ctx, input.WorkspaceID, "scheduled", agent.ID)
	if err != nil {
		return ScheduledRunResult{}, fmt.Errorf("check active runs: %w", err)
	}
	if activeRun != nil && (activeRun.Status == "queued" || activeRun.Status == "running") {
		slog.InfoContext(ctx, "scheduled agent skipped: active run exists",
			"agent_id", agent.ID, "active_run_id", activeRun.ID)
		return ScheduledRunResult{}, fmt.Errorf("agent already has an active run, skipping")
	}

	resolved := workerpkg.ResolveAgentProfile(agent, model.InvocationModeAutonomous)
	approvalState := workerpkg.ResolveApprovalState(resolved)
	taskQueue := resolved.Queue
	now := time.Now().UTC()
	inputPayload := model.AgentRunInputPayload{
		Trigger: &model.AgentRunTriggerContext{
			Source:      model.AgentRunTriggerSourceSchedule,
			TriggerType: model.TriggerCron,
			FiredAt:     &now,
		},
	}
	inputPayload.SetTarget("scheduled", agent.ID)
	inputJSON, err := json.Marshal(inputPayload)
	if err != nil {
		return ScheduledRunResult{}, fmt.Errorf("marshal scheduled run input: %w", err)
	}

	run := &model.AgentRun{
		WorkspaceID:   input.WorkspaceID,
		AgentID:       agent.ID,
		TargetType:    "scheduled",
		TargetID:      agent.ID,
		RuntimeKind:   agent.RuntimeKind,
		ApprovalState: approvalState,
		Status:        "queued",
		TaskQueue:     &taskQueue,
		RunnerPool:    &taskQueue,
		Input:         inputJSON,
		OutputSummary: json.RawMessage("{}"),
	}
	if err := a.runRepo.Create(ctx, run); err != nil {
		return ScheduledRunResult{}, fmt.Errorf("create run: %w", err)
	}

	agent.Status = "working"
	_ = a.agentRepo.Update(ctx, agent)

	slog.InfoContext(ctx, "scheduled agent run created",
		"agent_id", agent.ID, "run_id", run.ID, "queue", taskQueue)

	return ScheduledRunResult{RunID: run.ID, TaskQueue: taskQueue}, nil
}
