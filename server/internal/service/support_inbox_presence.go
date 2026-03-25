package service

import (
	"context"
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PublishTypingIndicator publishes a typing indicator event via WebSocket.
// actorID identifies whether the sender is a widget visitor or an internal agent.
func (s *SupportInboxService) PublishTypingIndicator(ctx context.Context, workspaceID, conversationID, actorID string, isTyping bool, content string) {
	action := "typing_stopped"
	if isTyping {
		action = "typing_started"
	}
	var eventData json.RawMessage
	if content != "" {
		eventData, _ = json.Marshal(map[string]string{"content": content})
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      action,
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		Data:        eventData,
	})
}

// PublishViewingPresence broadcasts a viewing_started or viewing_stopped event.
func (s *SupportInboxService) PublishViewingPresence(ctx context.Context, workspaceID, conversationID, actorID string, viewing bool) {
	action := "viewing_stopped"
	if viewing {
		action = "viewing_started"
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      action,
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})
}

// PublishSupportTeammatePresence broadcasts a teammate presence update.
func PublishSupportTeammatePresence(ctx context.Context, publisher *websocket.Publisher, workspaceID string, status model.SupportTeammatePresenceStatus) {
	if publisher == nil {
		return
	}
	data, _ := json.Marshal(status)
	publisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_teammate_presence",
		EntityID:    status.UserID,
		WorkspaceID: workspaceID,
		ActorID:     status.UserID,
		Data:        data,
	})
}
