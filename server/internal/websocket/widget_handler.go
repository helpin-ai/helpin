package websocket

import (
	"context"
	"log/slog"
	"net/http"

	"nhooyr.io/websocket"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SessionValidator validates widget session tokens.
type SessionValidator interface {
	GetWidgetSession(ctx context.Context, token string) (*model.SupportWidgetSession, error)
}

// WidgetHandler upgrades HTTP connections to WebSocket for widget clients.
// Authenticates via session_token instead of JWT.
type WidgetHandler struct {
	hub              *Hub
	sessionValidator SessionValidator
}

// NewWidgetHandler creates a WebSocket handler for widget clients.
func NewWidgetHandler(hub *Hub, sessionValidator SessionValidator) *WidgetHandler {
	return &WidgetHandler{hub: hub, sessionValidator: sessionValidator}
}

// ServeHTTP handles the WebSocket upgrade for widget connections.
func (h *WidgetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sessionToken := r.URL.Query().Get("session_token")
	if sessionToken == "" {
		http.Error(w, "missing session_token", http.StatusUnauthorized)
		return
	}

	session, err := h.sessionValidator.GetWidgetSession(r.Context(), sessionToken)
	if err != nil {
		slog.Warn("widget ws: invalid session", "error", err)
		http.Error(w, "invalid or expired session", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow cross-origin widget connections
	})
	if err != nil {
		slog.Error("widget ws: accept error", "error", err)
		return
	}

	// Use session ID as the user ID for widget clients so messages
	// broadcast to the workspace reach this connection.
	client := &Client{
		Conn:        conn,
		UserID:      "widget:" + session.ID,
		WorkspaceID: session.WorkspaceID,
	}

	h.hub.Register(client)
	defer func() {
		h.hub.Unregister(client)
		conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	// Read loop: keep alive by consuming pings/messages.
	for {
		_, _, err := conn.Read(r.Context())
		if err != nil {
			return
		}
	}
}
