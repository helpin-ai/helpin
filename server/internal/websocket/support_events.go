package websocket

import (
	"encoding/json"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportMessageEvent builds the standard websocket event for a support message.
// Internal notes intentionally omit hydrated payload data so widget clients do
// not receive them.
func SupportMessageEvent(workspaceID string, msg *model.SupportMessage, actorID string) Event {
	if msg == nil {
		return Event{}
	}

	event := Event{
		Action:      "created",
		Entity:      "support_conversation_message",
		EntityID:    msg.ID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ParentType:  "support_conversation",
		ParentID:    msg.ConversationID,
	}
	if msg.IsInternal || msg.MessageType == "system" {
		return event
	}

	payload, err := json.Marshal(model.WidgetMessageReceivedPayload{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		Content:        msg.Content,
		SenderType:     msg.SenderType,
		SenderName:     msg.SenderDisplayName,
		SenderAvatar:   msg.SenderAvatarURL,
		CreatedAt:      msg.CreatedAt.Format(time.RFC3339),
	})
	if err == nil {
		event.Data = payload
	}
	return event
}
