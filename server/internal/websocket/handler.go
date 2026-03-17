package websocket

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"

	"nhooyr.io/websocket"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
)

// agentMessage is the envelope for client→server messages from agent WS connections.
type agentMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// agentViewingData is the payload for support:viewing:start/stop.
type agentViewingData struct {
	ConversationID string `json:"conversation_id"`
}

// agentTypingData is the payload for support:typing:start/update/stop.
type agentTypingData struct {
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content,omitempty"`
}

// Handler upgrades HTTP connections to WebSocket.
type Handler struct {
	hub          *Hub
	jwtManager   *auth.JWTManager
	authzService *authorization.AuthzService
}

// NewHandler creates a WebSocket handler.
func NewHandler(hub *Hub, jwtManager *auth.JWTManager) *Handler {
	return &Handler{hub: hub, jwtManager: jwtManager}
}

// SetAuthzService injects the authorization service for workspace access checks.
func (h *Handler) SetAuthzService(authz *authorization.AuthzService) {
	h.authzService = authz
}

// ServeHTTP handles the WebSocket upgrade and connection lifecycle.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Authenticate via query params (browsers can't set WS headers).
	token := r.URL.Query().Get("token")
	workspaceID := r.URL.Query().Get("workspace_id")

	if token == "" || workspaceID == "" {
		http.Error(w, "missing token or workspace_id", http.StatusUnauthorized)
		return
	}

	claims, err := h.jwtManager.ValidateToken(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Verify workspace membership and ws.connect permission.
	if h.authzService != nil {
		actor, err := h.authzService.ResolveActor(r.Context(), workspaceID, claims.UserID)
		if err != nil {
			http.Error(w, "not authorized for this workspace", http.StatusForbidden)
			return
		}
		if !h.authzService.Can(actor, authorization.PermWSConnect) {
			http.Error(w, "insufficient permissions", http.StatusForbidden)
			return
		}
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // CORS is handled at the router level
	})
	if err != nil {
		log.Printf("[ws] accept error: %v", err)
		return
	}

	client := &Client{
		Conn:        conn,
		UserID:      claims.UserID,
		WorkspaceID: workspaceID,
	}

	h.hub.Register(client)
	defer func() {
		h.hub.UnsubscribeAllSessions(client)
		h.hub.Unregister(client) // also cleans up presence + broadcasts stop events
		conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	// Send current online visitors as initial snapshot
	visitors := h.hub.GetOnlineVisitors(workspaceID)
	if len(visitors) > 0 {
		data, _ := json.Marshal(map[string]any{"visitors": visitors})
		SendToClient(conn, "support:online_visitors", json.RawMessage(data))
	}

	// Read loop: process client messages for session subscriptions and support presence/typing.
	for {
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}

		var msg agentMessage
		if json.Unmarshal(data, &msg) != nil {
			continue
		}

		switch msg.Type {
		case "subscribe_session":
			var sessionMsg struct {
				SessionID string `json:"session_id"`
			}
			if json.Unmarshal(data, &sessionMsg) == nil && sessionMsg.SessionID != "" {
				h.hub.SubscribeSession(client, sessionMsg.SessionID)
			}
		case "unsubscribe_session":
			var sessionMsg struct {
				SessionID string `json:"session_id"`
			}
			if json.Unmarshal(data, &sessionMsg) == nil && sessionMsg.SessionID != "" {
				h.hub.UnsubscribeSession(client, sessionMsg.SessionID)
			}

		case "support:viewing:start":
			var d agentViewingData
			if json.Unmarshal(msg.Data, &d) != nil || d.ConversationID == "" {
				continue
			}
			if h.hub.Presence.SetViewing(workspaceID, d.ConversationID, client.UserID) {
				h.hub.BroadcastAll(Event{
					Action:      "viewing_started",
					Entity:      "support_conversation",
					EntityID:    d.ConversationID,
					WorkspaceID: workspaceID,
					ActorID:     client.UserID,
				})
			}
			// Send presence snapshot to this client
			snap := h.hub.Presence.MarshalSnapshot(workspaceID, d.ConversationID)
			SendToClient(conn, "support:presence_snapshot", json.RawMessage(snap))

		case "support:viewing:stop":
			var d agentViewingData
			if json.Unmarshal(msg.Data, &d) != nil || d.ConversationID == "" {
				continue
			}
			if h.hub.Presence.ClearViewing(workspaceID, d.ConversationID, client.UserID) {
				h.hub.BroadcastAll(Event{
					Action:      "viewing_stopped",
					Entity:      "support_conversation",
					EntityID:    d.ConversationID,
					WorkspaceID: workspaceID,
					ActorID:     client.UserID,
				})
			}

		case "support:typing:start", "support:typing:update":
			var d agentTypingData
			if json.Unmarshal(msg.Data, &d) != nil || d.ConversationID == "" {
				continue
			}
			if h.hub.Presence.SetTyping(workspaceID, d.ConversationID, client.UserID, d.Content) {
				var eventData json.RawMessage
				if d.Content != "" {
					eventData, _ = json.Marshal(map[string]string{"content": d.Content})
				}
				h.hub.BroadcastAll(Event{
					Action:      "typing_started",
					Entity:      "support_conversation",
					EntityID:    d.ConversationID,
					WorkspaceID: workspaceID,
					ActorID:     client.UserID,
					Data:        eventData,
				})
			}

		case "support:typing:stop":
			var d agentTypingData
			if json.Unmarshal(msg.Data, &d) != nil || d.ConversationID == "" {
				continue
			}
			if h.hub.Presence.ClearTyping(workspaceID, d.ConversationID, client.UserID) {
				h.hub.BroadcastAll(Event{
					Action:      "typing_stopped",
					Entity:      "support_conversation",
					EntityID:    d.ConversationID,
					WorkspaceID: workspaceID,
					ActorID:     client.UserID,
				})
			}

		default:
			slog.Debug("[ws] unknown agent message type", "type", msg.Type, "user", client.UserID)
		}
	}
}
