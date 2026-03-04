package websocket

import (
	"log"
	"net/http"

	"nhooyr.io/websocket"

	"github.com/d4interactive/teampulse/server/internal/auth"
)

// Handler upgrades HTTP connections to WebSocket.
type Handler struct {
	hub        *Hub
	jwtManager *auth.JWTManager
}

// NewHandler creates a WebSocket handler.
func NewHandler(hub *Hub, jwtManager *auth.JWTManager) *Handler {
	return &Handler{hub: hub, jwtManager: jwtManager}
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
		h.hub.Unregister(client)
		conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	// Read loop: keep connection alive by consuming pings/messages.
	// We don't expect any client messages — just read to detect disconnect.
	for {
		_, _, err := conn.Read(r.Context())
		if err != nil {
			return
		}
	}
}
