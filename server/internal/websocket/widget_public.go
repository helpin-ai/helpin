package websocket

import (
	"context"
	"encoding/json"
	"log/slog"

	"nhooyr.io/websocket"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func widgetTypingData(data json.RawMessage) json.RawMessage {
	var typing struct {
		Name   string `json:"agent_name,omitempty"`
		Avatar string `json:"agent_avatar,omitempty"`
	}
	if json.Unmarshal(data, &typing) != nil {
		return json.RawMessage(`{}`)
	}
	result, _ := json.Marshal(typing)
	return result
}

func widgetConversationsData(data json.RawMessage) json.RawMessage {
	var payload struct {
		Conversations []model.WidgetConversation `json:"conversations"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return json.RawMessage(`{"conversations":[]}`)
	}
	if payload.Conversations == nil {
		payload.Conversations = []model.WidgetConversation{}
	}
	result, _ := json.Marshal(payload)
	return result
}

func widgetTeammatePresenceData(data json.RawMessage) json.RawMessage {
	var payload struct {
		UserID string `json:"user_id"`
		Status string `json:"status"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return json.RawMessage(`{}`)
	}
	result, _ := json.Marshal(payload)
	return result
}

// rememberWidgetTeammates tracks identities already included in public conversation/config data.
// It never accepts visitor-supplied IDs as permission to subscribe to staff presence.
func (h *Hub) rememberWidgetTeammates(client *Client, ids ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if client.PublicTeammateIDs == nil {
		client.PublicTeammateIDs = make(map[string]bool)
	}
	for _, id := range ids {
		if id != "" {
			client.PublicTeammateIDs[id] = true
		}
	}
}

func (h *WidgetHandler) rememberConversationTeammates(client *Client, conversations []model.SupportConversation) {
	for _, conversation := range conversations {
		if conversation.OpenedByUserID != nil {
			h.hub.rememberWidgetTeammates(client, *conversation.OpenedByUserID)
		}
	}
}

func sendWidgetOperationError(ctx context.Context, conn *websocket.Conn, kind, code string, err error) {
	slog.WarnContext(ctx, "widget operation failed", "code", code, "error", err)
	message := "Something went wrong. Please try again."
	switch code {
	case "send_failed":
		message = "We couldn't send your message. Please try again."
	case "create_failed", "session_expired":
		message = "Please reconnect to continue this conversation."
	case "load_failed", "conversation_select_failed":
		message = "We couldn't load this conversation. Please try again."
	case "upgrade_failed":
		message = "We couldn't update your details. Please try again."
	}
	SendToClient(conn, kind, map[string]string{"code": code, "message": message})
}
