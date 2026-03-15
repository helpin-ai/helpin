package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"nhooyr.io/websocket"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// WidgetService defines the service methods needed by the widget WS handler.
type WidgetService interface {
	GetInstallationByWidgetKey(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, error)
	CreateWidgetSession(ctx context.Context, widgetKey string, anonymousID string, customerName, customerEmail *string, userAgent, pageURL *string) (*model.SupportWidgetSession, error)
	GetWidgetSession(ctx context.Context, token string) (*model.SupportWidgetSession, error)
	GetVisitorConversations(ctx context.Context, workspaceID, anonymousID string) ([]model.SupportConversation, error)
	ListConversationMessages(ctx context.Context, workspaceID, conversationID string, includeInternal bool) ([]model.SupportMessage, error)
	WidgetCreateMessage(ctx context.Context, sessionToken, content string) (*model.SupportMessage, error)
	UpgradeWidgetSession(ctx context.Context, sessionToken, email, name string) error
	RevokeWidgetSession(ctx context.Context, sessionToken string) error
}

// WidgetHandler upgrades HTTP connections to WebSocket for widget clients.
// Accepts with just widget_key (unauthenticated), then requires session:create or session:restore.
type WidgetHandler struct {
	hub     *Hub
	service WidgetService
}

// NewWidgetHandler creates a WebSocket handler for widget clients.
func NewWidgetHandler(hub *Hub, service WidgetService) *WidgetHandler {
	return &WidgetHandler{hub: hub, service: service}
}

// ServeHTTP handles the WebSocket upgrade for widget connections.
func (h *WidgetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	widgetKey := r.URL.Query().Get("key")

	// Backwards compat: also accept session_token for legacy clients
	legacyToken := r.URL.Query().Get("session_token")
	if widgetKey == "" && legacyToken != "" {
		h.serveLegacy(w, r, legacyToken)
		return
	}

	if widgetKey == "" {
		http.Error(w, "missing key parameter", http.StatusBadRequest)
		return
	}

	// Validate widget_key exists
	_, err := h.service.GetInstallationByWidgetKey(r.Context(), widgetKey)
	if err != nil {
		http.Error(w, "invalid widget key", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow cross-origin widget connections
	})
	if err != nil {
		slog.Error("widget ws: accept error", "error", err)
		return
	}

	// Wait for first message: session:create or session:restore (10s timeout)
	firstMsgCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	_, data, err := conn.Read(firstMsgCtx)
	cancel()
	if err != nil {
		slog.Warn("widget ws: handshake timeout or read error", "error", err)
		conn.Close(websocket.StatusPolicyViolation, "handshake timeout")
		return
	}

	var msg model.WidgetWSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		slog.Warn("widget ws: invalid handshake message", "error", err)
		conn.Close(websocket.StatusPolicyViolation, "invalid message")
		return
	}

	var session *model.SupportWidgetSession
	switch msg.Type {
	case "session:create":
		session, err = h.handleSessionCreate(r.Context(), widgetKey, msg, conn)
	case "session:restore":
		session, err = h.handleSessionRestore(r.Context(), widgetKey, msg, conn)
	default:
		slog.Warn("widget ws: unexpected first message type", "type", msg.Type)
		conn.Close(websocket.StatusPolicyViolation, "expected session:create or session:restore")
		return
	}

	if err != nil || session == nil {
		return // error already handled and sent to client
	}

	// Register client and enter bidirectional message loop
	h.handleConnection(r.Context(), conn, session, widgetKey)
}

// serveLegacy handles legacy widget connections that pass session_token in the URL.
func (h *WidgetHandler) serveLegacy(w http.ResponseWriter, r *http.Request, sessionToken string) {
	session, err := h.service.GetWidgetSession(r.Context(), sessionToken)
	if err != nil {
		slog.Warn("widget ws: invalid session", "error", err)
		http.Error(w, "invalid or expired session", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		slog.Error("widget ws: accept error", "error", err)
		return
	}

	client := &Client{
		Conn:           conn,
		UserID:         "widget:" + session.ID,
		WorkspaceID:    session.WorkspaceID,
		IsWidget:       true,
		ConversationID: session.ConversationID,
	}

	h.hub.Register(client)
	defer func() {
		h.hub.Unregister(client)
		conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	// Legacy read loop: keep alive
	for {
		_, _, err := conn.Read(r.Context())
		if err != nil {
			return
		}
	}
}

func (h *WidgetHandler) handleSessionCreate(ctx context.Context, widgetKey string, msg model.WidgetWSMessage, conn *websocket.Conn) (*model.SupportWidgetSession, error) {
	anonymousID, _ := msg.Data["anonymous_id"].(string)
	pageURL, _ := msg.Data["page_url"].(string)
	userAgent, _ := msg.Data["user_agent"].(string)

	var pageURLPtr, uaPtr *string
	if pageURL != "" {
		pageURLPtr = &pageURL
	}
	if userAgent != "" {
		uaPtr = &userAgent
	}

	session, err := h.service.CreateWidgetSession(ctx, widgetKey, anonymousID, nil, nil, uaPtr, pageURLPtr)
	if err != nil {
		slog.Error("widget ws: session create failed", "error", err)
		SendToClient(conn, "session:error", map[string]string{"code": "create_failed", "message": err.Error()})
		conn.Close(websocket.StatusInternalError, "session create failed")
		return nil, err
	}

	// Send session:joined
	if err := h.sendSessionJoined(ctx, conn, session); err != nil {
		conn.Close(websocket.StatusInternalError, "send failed")
		return nil, err
	}

	return session, nil
}

func (h *WidgetHandler) handleSessionRestore(ctx context.Context, widgetKey string, msg model.WidgetWSMessage, conn *websocket.Conn) (*model.SupportWidgetSession, error) {
	token, _ := msg.Data["session_token"].(string)
	if token == "" {
		SendToClient(conn, "session:error", map[string]string{"code": "invalid_token", "message": "missing session_token"})
		conn.Close(websocket.StatusPolicyViolation, "missing token")
		return nil, nil
	}

	session, err := h.service.GetWidgetSession(ctx, token)
	if err != nil {
		// Token invalid/expired/revoked — tell client to recreate
		slog.Info("widget ws: session restore failed, client should recreate", "error", err)
		SendToClient(conn, "session:error", map[string]string{"code": "invalid_token", "message": "Session expired or revoked"})
		// Don't close connection — let client send session:create as retry
		// Wait for retry message (10s)
		retryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, data, readErr := conn.Read(retryCtx)
		cancel()
		if readErr != nil {
			conn.Close(websocket.StatusPolicyViolation, "retry timeout")
			return nil, readErr
		}

		var retryMsg model.WidgetWSMessage
		if json.Unmarshal(data, &retryMsg) != nil || retryMsg.Type != "session:create" {
			conn.Close(websocket.StatusPolicyViolation, "expected session:create retry")
			return nil, nil
		}

		return h.handleSessionCreate(ctx, widgetKey, retryMsg, conn)
	}

	// Send session:joined
	if err := h.sendSessionJoined(ctx, conn, session); err != nil {
		conn.Close(websocket.StatusInternalError, "send failed")
		return nil, err
	}

	return session, nil
}

func (h *WidgetHandler) sendSessionJoined(ctx context.Context, conn *websocket.Conn, session *model.SupportWidgetSession) error {
	// Get visitor conversations
	var conversations []model.SupportConversation
	if session.AnonymousID != "" {
		convs, err := h.service.GetVisitorConversations(ctx, session.WorkspaceID, session.AnonymousID)
		if err == nil {
			conversations = convs
		}
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}

	// Get messages for active conversation
	var messages []model.SupportMessage
	if session.ConversationID != nil {
		msgs, err := h.service.ListConversationMessages(ctx, session.WorkspaceID, *session.ConversationID, false)
		if err == nil {
			messages = msgs
		}
	}
	if messages == nil {
		messages = []model.SupportMessage{}
	}

	return SendToClient(conn, "session:joined", model.WidgetSessionJoinedPayload{
		SessionToken:  session.SessionToken,
		ExpiresAt:     session.ExpiresAt.Format(time.RFC3339),
		IsAnonymous:   session.IsAnonymous,
		Conversations: conversations,
		Messages:      messages,
	})
}

func (h *WidgetHandler) handleConnection(ctx context.Context, conn *websocket.Conn, session *model.SupportWidgetSession, widgetKey string) {
	client := &Client{
		Conn:           conn,
		UserID:         "widget:" + session.ID,
		WorkspaceID:    session.WorkspaceID,
		IsWidget:       true,
		ConversationID: session.ConversationID,
	}

	h.hub.Register(client)
	defer func() {
		h.hub.Unregister(client)
		conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		var msg model.WidgetWSMessage
		if json.Unmarshal(data, &msg) != nil {
			continue
		}

		switch msg.Type {
		case "message:send":
			content, _ := msg.Data["content"].(string)
			if strings.TrimSpace(content) == "" {
				continue
			}
			result, err := h.service.WidgetCreateMessage(ctx, session.SessionToken, content)
			if err != nil {
				SendToClient(conn, "connection:error", map[string]string{"code": "send_failed", "message": err.Error()})
				continue
			}
			// Update hub routing if new conversation was created
			if client.ConversationID == nil && session.ConversationID != nil {
				h.hub.SetWidgetConversation(client.UserID, *session.ConversationID)
				SendToClient(conn, "conversation:created", map[string]string{"conversation_id": *session.ConversationID})
			}
			// Re-sync client's conversation ID from session
			if session.ConversationID != nil {
				client.ConversationID = session.ConversationID
			}
			// Echo back to sender with server-assigned ID
			SendToClient(conn, "message:received", model.WidgetMessageReceivedPayload{
				ID:             result.ID,
				ConversationID: result.ConversationID,
				Content:        result.Content,
				SenderType:     result.SenderType,
				SenderName:     result.SenderDisplayName,
				CreatedAt:      result.CreatedAt.Format(time.RFC3339),
			})

		case "typing:start":
			h.hub.Broadcast(Event{
				Action:      "typing_started",
				Entity:      "support_conversation",
				EntityID:    derefStr(session.ConversationID),
				WorkspaceID: session.WorkspaceID,
			})

		case "typing:stop":
			h.hub.Broadcast(Event{
				Action:      "typing_stopped",
				Entity:      "support_conversation",
				EntityID:    derefStr(session.ConversationID),
				WorkspaceID: session.WorkspaceID,
			})

		case "session:upgrade":
			email, _ := msg.Data["email"].(string)
			name, _ := msg.Data["name"].(string)
			if email == "" {
				SendToClient(conn, "connection:error", map[string]string{"code": "upgrade_failed", "message": "email is required"})
				continue
			}
			err := h.service.UpgradeWidgetSession(ctx, session.SessionToken, email, name)
			if err != nil {
				SendToClient(conn, "connection:error", map[string]string{"code": "upgrade_failed", "message": err.Error()})
				continue
			}
			session.CustomerEmail = &email
			session.CustomerName = &name
			session.IsAnonymous = false
			SendToClient(conn, "session:upgraded", nil)

		case "session:revoke":
			h.service.RevokeWidgetSession(ctx, session.SessionToken)
			SendToClient(conn, "session:revoked", nil)
			return // exit loop, connection will close

		case "page:update":
			// Update in-memory tracking (no DB write needed)

		case "conversations:list":
			convs, _ := h.service.GetVisitorConversations(ctx, session.WorkspaceID, session.AnonymousID)
			if convs == nil {
				convs = []model.SupportConversation{}
			}
			SendToClient(conn, "conversations:listed", map[string]interface{}{"conversations": convs})
		}
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
