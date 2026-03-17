package websocket

import (
	"encoding/json"
	"log"
	"net/http"

	"nhooyr.io/websocket"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
)

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
		h.hub.Unregister(client)
		conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	// Read loop: keep connection alive by consuming pings/messages.
	// Parse client messages for session subscribe/unsubscribe commands.
	for {
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		var msg struct {
			Type      string `json:"type"`
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal(data, &msg) == nil {
			switch msg.Type {
			case "subscribe_session":
				if msg.SessionID != "" {
					h.hub.SubscribeSession(client, msg.SessionID)
				}
			case "unsubscribe_session":
				if msg.SessionID != "" {
					h.hub.UnsubscribeSession(client, msg.SessionID)
				}
			}
		}
	}
}
