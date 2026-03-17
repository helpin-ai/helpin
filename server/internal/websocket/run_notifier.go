package websocket

import (
	"context"

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
func (n *RunNotifier) PublishRunEvent(_ context.Context, run *model.AgentRun) {
	if n == nil || n.publisher == nil || run == nil {
		return
	}
	n.publisher.Publish(Event{
		Action:      "updated",
		Entity:      "agent_run",
		EntityID:    run.ID,
		WorkspaceID: run.WorkspaceID,
		ParentType:  run.TargetType,
		ParentID:    run.TargetID,
	})
}
