package websocket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// RunNotifier adapts an EventPublisher into a repository.AgentRunNotifier.
// It translates an AgentRun into a WebSocket Event and publishes it.
type RunNotifier struct {
	publisher EventPublisher
}

// NewRunNotifier creates a RunNotifier backed by the given publisher.
func NewRunNotifier(publisher EventPublisher) *RunNotifier {
	if publisher == nil {
		return nil
	}
	return &RunNotifier{publisher: publisher}
}

// PublishRunEvent builds a WebSocket event from an AgentRun and publishes it.
func (n *RunNotifier) PublishRunEvent(ctx context.Context, run *model.AgentRun) {
	n.PublishRunChange(ctx, run, "")
}

func (n *RunNotifier) PublishRunChange(_ context.Context, run *model.AgentRun, kind model.AgentRunChangeKind) {
	if n == nil || n.publisher == nil || run == nil {
		return
	}
	status, pauseReason := model.NormalizeAgentRunStatus(run.Status, run.PauseReason, run.ApprovalState, run.ExecutionStage)
	payload := map[string]string{
		"agent_id":       run.AgentID,
		"status":         status,
		"pause_reason":   pauseReason,
		"approval_state": run.ApprovalState,
	}
	if kind != "" {
		payload["change_kind"] = string(kind)
	}
	if kind == model.AgentRunChangeState && !run.UpdatedAt.IsZero() {
		payload["state_revision"] = run.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	if run.DockChatID != nil {
		payload["dock_chat_id"] = *run.DockChatID
	}
	if run.ExecutionStage != nil {
		payload["execution_stage"] = *run.ExecutionStage
	}
	data, _ := json.Marshal(payload)
	n.publisher.Publish(Event{
		Action:      "updated",
		Entity:      "agent_run",
		EntityID:    run.ID,
		WorkspaceID: run.WorkspaceID,
		ParentType:  run.TargetType,
		ParentID:    run.TargetID,
		Data:        data,
	})
	n.publisher.Publish(Event{
		Action:      "updated",
		Entity:      "coding_session",
		EntityID:    run.ID,
		WorkspaceID: run.WorkspaceID,
		ParentType:  run.TargetType,
		ParentID:    run.TargetID,
		Data:        data,
	})
}
