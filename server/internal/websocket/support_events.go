package websocket

import (
	"encoding/json"
	"time"
	"unicode"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	// SupportAIResponseStreamEntity is a transient, widget-only support stream.
	SupportAIResponseStreamEntity = "support_ai_response_stream"
	supportAIResponseChunkRunes   = 48
)

// SupportMessageEvent builds the standard websocket event for a support message.
// Internal notes intentionally omit hydrated payload data so widget clients do
// not receive them.
func SupportMessageEvent(workspaceID string, msg *model.SupportMessage, actorID string) Event {
	return supportMessageEvent(workspaceID, msg, actorID, "created")
}

// SupportMessageUpdatedEvent signals that metadata (for example, a link
// preview) was enriched after the message was initially created.
func SupportMessageUpdatedEvent(workspaceID string, msg *model.SupportMessage, actorID string) Event {
	return supportMessageEvent(workspaceID, msg, actorID, "updated")
}

func supportMessageEvent(workspaceID string, msg *model.SupportMessage, actorID, action string) Event {
	if msg == nil {
		return Event{}
	}

	event := Event{
		Action:      action,
		Entity:      "support_conversation_message",
		EntityID:    msg.ID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ParentType:  "support_conversation",
		ParentID:    msg.ConversationID,
	}
	if !msg.WidgetVisible() {
		return event
	}

	payload, err := json.Marshal(model.WidgetMessageReceivedPayload{
		ID:                        msg.ID,
		ClientMessageID:           msg.ClientMessageID,
		ConversationID:            msg.ConversationID,
		Content:                   msg.Content,
		SenderType:                msg.SenderType,
		MessageType:               msg.MessageType,
		SystemEventType:           msg.SystemEventType,
		SenderName:                msg.SenderDisplayName,
		SenderAvatar:              msg.SenderAvatarURL,
		Metadata:                  nilIfEmpty(msg.Metadata),
		CreatedAt:                 msg.CreatedAt.Format(time.RFC3339),
		ViaChannel:                derefStr(msg.ViaChannel),
		Attachments:               msg.Attachments,
		EmailVisibleText:          msg.EmailVisibleText,
		EmailQuotedText:           msg.EmailQuotedText,
		EmailHasQuotedContent:     msg.EmailHasQuotedContent,
		EmailProjectionConfidence: msg.EmailProjectionConfidence,
		EmailProjectionVersion:    msg.EmailProjectionVersion,
	})
	if err == nil {
		event.Data = payload
	}
	return event
}

func widgetSafeSupportMessageEventData(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return data
	}
	// Accept legacy event payloads, then rebuild the visitor projection.
	var payload model.SupportMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	var names struct {
		Name   *string `json:"sender_name"`
		Avatar *string `json:"sender_avatar"`
	}
	if err := json.Unmarshal(data, &names); err != nil {
		return nil
	}
	if names.Name != nil {
		payload.SenderDisplayName = names.Name
	}
	if names.Avatar != nil {
		payload.SenderAvatarURL = names.Avatar
	}
	public := model.PublicWidgetMessage(&payload)
	if public == nil {
		return nil
	}
	encoded, err := json.Marshal(public)
	if err != nil {
		return nil
	}
	return encoded
}

// SupportMessageDeletedEvent builds the standard websocket event for a support
// message soft-delete. Clients invalidate the same message-list cache they use
// for created events.
func SupportMessageDeletedEvent(workspaceID, conversationID, messageID, actorID string) Event {
	return Event{
		Action:      "deleted",
		Entity:      "support_conversation_message",
		EntityID:    messageID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ParentType:  "support_conversation",
		ParentID:    conversationID,
	}
}

// SupportAIProgressEvent builds a customer-safe transient progress event.
func SupportAIProgressEvent(workspaceID, conversationID, actorID, stage, label string) Event {
	payload, _ := json.Marshal(map[string]string{
		"conversation_id": conversationID,
		"stage":           stage,
		"label":           label,
	})
	return Event{
		Action:      "progress",
		Entity:      SupportAIResponseStreamEntity,
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ParentType:  "support_conversation",
		ParentID:    conversationID,
		Data:        payload,
	}
}

// SupportAIResponseStartEvents builds ordered start and content-delta events
// from an already validated, persisted customer-facing support message.
func SupportAIResponseStartEvents(workspaceID string, msg *model.SupportMessage, actorID string) []Event {
	if msg == nil || !msg.WidgetVisible() || msg.ID == "" || msg.ConversationID == "" || msg.Content == "" {
		return nil
	}

	startPayload, _ := json.Marshal(map[string]any{
		"response_id":     msg.ID,
		"message_id":      msg.ID,
		"conversation_id": msg.ConversationID,
		"sender_type":     msg.SenderType,
		"sender_name":     msg.SenderDisplayName,
		"sender_avatar":   msg.SenderAvatarURL,
		"created_at":      msg.CreatedAt.Format(time.RFC3339),
	})
	events := []Event{{
		Action:      "response_started",
		Entity:      SupportAIResponseStreamEntity,
		EntityID:    msg.ID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ParentType:  "support_conversation",
		ParentID:    msg.ConversationID,
		Data:        startPayload,
	}}

	for index, chunk := range splitSupportAIResponseContent(msg.Content, supportAIResponseChunkRunes) {
		payload, _ := json.Marshal(map[string]any{
			"response_id":     msg.ID,
			"message_id":      msg.ID,
			"conversation_id": msg.ConversationID,
			"sequence":        index + 1,
			"delta":           chunk,
		})
		events = append(events, Event{
			Action:      "response_delta",
			Entity:      SupportAIResponseStreamEntity,
			EntityID:    msg.ID,
			WorkspaceID: workspaceID,
			ActorID:     actorID,
			ParentType:  "support_conversation",
			ParentID:    msg.ConversationID,
			Data:        payload,
		})
	}
	return events
}

// SupportAIResponseCompleteEvent marks the persisted message as the canonical
// end of a transient response stream.
func SupportAIResponseCompleteEvent(workspaceID string, msg *model.SupportMessage, actorID string) Event {
	if msg == nil || !msg.WidgetVisible() || msg.ID == "" || msg.ConversationID == "" {
		return Event{}
	}
	payload, _ := json.Marshal(map[string]string{
		"response_id":     msg.ID,
		"message_id":      msg.ID,
		"conversation_id": msg.ConversationID,
	})
	return Event{
		Action:      "response_completed",
		Entity:      SupportAIResponseStreamEntity,
		EntityID:    msg.ID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ParentType:  "support_conversation",
		ParentID:    msg.ConversationID,
		Data:        payload,
	}
}

func splitSupportAIResponseContent(content string, maxRunes int) []string {
	if content == "" {
		return nil
	}
	if maxRunes <= 0 {
		maxRunes = supportAIResponseChunkRunes
	}

	remaining := []rune(content)
	chunks := make([]string, 0, len(remaining)/maxRunes+1)
	for len(remaining) > maxRunes {
		cut := maxRunes
		for index := maxRunes; index >= maxRunes/2; index-- {
			if unicode.IsSpace(remaining[index-1]) || isSupportAIChunkBoundary(remaining[index-1]) {
				cut = index
				break
			}
		}
		chunks = append(chunks, string(remaining[:cut]))
		remaining = remaining[cut:]
	}
	if len(remaining) > 0 {
		chunks = append(chunks, string(remaining))
	}
	return chunks
}

func isSupportAIChunkBoundary(value rune) bool {
	switch value {
	case '.', ',', '!', '?', ';', ':', '\n':
		return true
	default:
		return false
	}
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
