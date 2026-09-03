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
	"github.com/helpin-ai/helpin/server/internal/requestmeta"
)

// WidgetService defines the service methods needed by the widget WS handler.
type WidgetService interface {
	GetInstallationByWidgetKey(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, error)
	CreateWidgetSession(ctx context.Context, widgetKey string, anonymousID string, customerName, customerEmail *string, userAgent, pageURL, timezone, locale *string) (*model.SupportWidgetSession, error)
	UpdateSessionPageURL(ctx context.Context, sessionToken, url string) error
	GetWidgetSession(ctx context.Context, token string) (*model.SupportWidgetSession, error)
	TouchWidgetSessionActivity(ctx context.Context, sessionToken string) error
	GetVisitorConversations(ctx context.Context, workspaceID, anonymousID string) ([]model.SupportConversation, error)
	ListConversationMessages(ctx context.Context, workspaceID, conversationID string, includeInternal bool) ([]model.SupportMessage, error)
	WidgetCreateMessage(ctx context.Context, sessionToken, content string, attachmentIDs []string) (*model.SupportMessage, error)
	UpgradeWidgetSession(ctx context.Context, sessionToken string, identity model.WidgetIdentityPayload) error
	RevokeWidgetSession(ctx context.Context, sessionToken string) error
	ClearSessionConversation(ctx context.Context, sessionToken string) error
	SetSessionConversation(ctx context.Context, sessionToken, conversationID string) error
	MarkConversationReadByVisitor(ctx context.Context, workspaceID, conversationID, anonymousID string) error
	EscalateConversation(ctx context.Context, workspaceID, conversationID, reason string) error
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
	ctx := r.Context()
	if clientIP, ok := requestmeta.ExtractClientIP(r); ok {
		ctx = requestmeta.WithClientIP(ctx, clientIP)
	}

	widgetKey := r.URL.Query().Get("key")

	// Backwards compat: also accept session_token for legacy clients
	legacyToken := r.URL.Query().Get("session_token")
	if widgetKey == "" && legacyToken != "" {
		h.serveLegacy(ctx, w, r, legacyToken)
		return
	}

	if widgetKey == "" {
		http.Error(w, "missing key parameter", http.StatusBadRequest)
		return
	}

	// Validate widget_key exists
	_, err := h.service.GetInstallationByWidgetKey(ctx, widgetKey)
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
	firstMsgCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
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
		session, err = h.handleSessionCreate(ctx, widgetKey, msg, conn)
	case "session:restore":
		session, err = h.handleSessionRestore(ctx, widgetKey, msg, conn)
	default:
		slog.Warn("widget ws: unexpected first message type", "type", msg.Type)
		conn.Close(websocket.StatusPolicyViolation, "expected session:create or session:restore")
		return
	}

	if err != nil || session == nil {
		return // error already handled and sent to client
	}

	// Register client and enter bidirectional message loop
	h.handleConnection(ctx, conn, session, widgetKey)
}

// serveLegacy handles legacy widget connections that pass session_token in the URL.
func (h *WidgetHandler) serveLegacy(ctx context.Context, w http.ResponseWriter, r *http.Request, sessionToken string) {
	session, err := h.service.GetWidgetSession(ctx, sessionToken)
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
		ConnID:         generateConnID(),
		UserID:         "widget:" + session.ID,
		WorkspaceID:    session.WorkspaceID,
		IsWidget:       true,
		ConversationID: session.ConversationID,
		AnonymousID:    session.AnonymousID,
	}

	h.hub.Register(client)
	if session.AnonymousID != "" {
		h.hub.SetVisitorOnline(session.WorkspaceID, session.AnonymousID)
		if err := h.hub.Presence.SetVisitorOnline(ctx, session.WorkspaceID, session.AnonymousID, client.ConnID); err != nil {
			slog.Error("presence SetVisitorOnline (legacy)", "error", err)
		}
		h.hub.BroadcastAll(Event{
			Action:      "visitor_online",
			Entity:      "support_visitor",
			EntityID:    session.AnonymousID,
			WorkspaceID: session.WorkspaceID,
		})
	}
	defer func() {
		if err := h.service.TouchWidgetSessionActivity(ctx, session.SessionToken); err != nil {
			slog.Warn("widget ws: touch activity on disconnect failed", "error", err)
		}
		h.hub.Unregister(client)
		if session.AnonymousID != "" {
			h.hub.SetVisitorOffline(session.WorkspaceID, session.AnonymousID)
			lastConn, err := h.hub.Presence.SetVisitorOffline(ctx, session.WorkspaceID, session.AnonymousID, client.ConnID)
			if err != nil {
				slog.Error("presence SetVisitorOffline (legacy)", "error", err)
			}
			if shouldBroadcastVisitorOffline(h.hub.IsVisitorOnline(session.WorkspaceID, session.AnonymousID), lastConn, err) {
				h.hub.BroadcastAll(Event{
					Action:      "visitor_offline",
					Entity:      "support_visitor",
					EntityID:    session.AnonymousID,
					WorkspaceID: session.WorkspaceID,
				})
			}
		}
		conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	// Legacy read loop: keep alive
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if session.AnonymousID != "" {
			if err := h.hub.Presence.RefreshVisitorOnline(ctx, session.WorkspaceID, session.AnonymousID, client.ConnID); err != nil {
				slog.Error("presence RefreshVisitorOnline on legacy widget activity", "error", err)
			}
		}
	}
}

func unmarshalWidgetData[T any](data map[string]any) (T, error) {
	var result T
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(dataBytes, &result)
	return result, err
}

// shouldBroadcastVisitorOffline treats the shared presence provider as the
// authority whenever it responds successfully. A replica's local hub only
// knows about sockets connected to that replica, so it cannot decide that a
// visitor is offline while another replica may still own a connection.
func shouldBroadcastVisitorOffline(localOnline, lastConn bool, presenceErr error) bool {
	if presenceErr == nil {
		return lastConn
	}
	return !localOnline
}

func (h *WidgetHandler) handleSessionCreate(ctx context.Context, widgetKey string, msg model.WidgetWSMessage, conn *websocket.Conn) (*model.SupportWidgetSession, error) {
	typed, err := unmarshalWidgetData[model.WidgetSessionCreateData](msg.Data)
	if err != nil {
		slog.Warn("widget ws: invalid session:create data", "error", err)
		SendToClient(conn, "session:error", map[string]string{"code": "invalid_data", "message": "malformed session:create payload"})
		conn.Close(websocket.StatusPolicyViolation, "invalid data")
		return nil, err
	}
	anonymousID := typed.AnonymousID
	pageURL := typed.PageURL
	userAgent := typed.UserAgent
	timezone := typed.Timezone
	locale := typed.Locale

	var pageURLPtr, uaPtr, tzPtr, localePtr *string
	if pageURL != "" {
		pageURLPtr = &pageURL
	}
	if userAgent != "" {
		uaPtr = &userAgent
	}
	if timezone != "" {
		tzPtr = &timezone
	}
	if locale != "" {
		localePtr = &locale
	}

	session, err := h.service.CreateWidgetSession(ctx, widgetKey, anonymousID, nil, nil, uaPtr, pageURLPtr, tzPtr, localePtr)
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
	typed, err := unmarshalWidgetData[model.WidgetSessionRestoreData](msg.Data)
	if err != nil {
		slog.Warn("widget ws: invalid session:restore data", "error", err)
		SendToClient(conn, "session:error", map[string]string{"code": "invalid_data", "message": "malformed session:restore payload"})
		conn.Close(websocket.StatusPolicyViolation, "invalid data")
		return nil, err
	}
	token := typed.SessionToken
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

	var activeTeammate *model.WidgetActiveTeammate
	if session.ConversationID != nil {
		for i := range conversations {
			if conversations[i].ID == *session.ConversationID {
				activeTeammate = widgetActiveTeammateFromConversation(&conversations[i])
				break
			}
		}
	}

	var customerEmail string
	if session.CustomerEmail != nil {
		customerEmail = *session.CustomerEmail
	}

	return SendToClient(conn, "session:joined", model.WidgetSessionJoinedPayload{
		SessionToken:   session.SessionToken,
		ExpiresAt:      session.ExpiresAt.Format(time.RFC3339),
		IsAnonymous:    session.IsAnonymous,
		CustomerEmail:  customerEmail,
		Conversations:  conversations,
		Messages:       messages,
		ActiveTeammate: activeTeammate,
	})
}

func (h *WidgetHandler) handleConnection(ctx context.Context, conn *websocket.Conn, session *model.SupportWidgetSession, widgetKey string) {
	client := &Client{
		Conn:           conn,
		ConnID:         generateConnID(),
		UserID:         "widget:" + session.ID,
		WorkspaceID:    session.WorkspaceID,
		IsWidget:       true,
		ConversationID: session.ConversationID,
		AnonymousID:    session.AnonymousID,
	}

	h.hub.Register(client)
	if session.AnonymousID != "" {
		h.hub.SetVisitorOnline(session.WorkspaceID, session.AnonymousID)
		if err := h.hub.Presence.SetVisitorOnline(ctx, session.WorkspaceID, session.AnonymousID, client.ConnID); err != nil {
			slog.Error("presence SetVisitorOnline", "error", err)
		}
		h.hub.BroadcastAll(Event{
			Action:      "visitor_online",
			Entity:      "support_visitor",
			EntityID:    session.AnonymousID,
			WorkspaceID: session.WorkspaceID,
		})
	}
	defer func() {
		if err := h.service.TouchWidgetSessionActivity(ctx, session.SessionToken); err != nil {
			slog.Warn("widget ws: touch activity on disconnect failed", "error", err)
		}
		h.hub.Unregister(client)
		if session.AnonymousID != "" {
			h.hub.SetVisitorOffline(session.WorkspaceID, session.AnonymousID)
			lastConn, err := h.hub.Presence.SetVisitorOffline(ctx, session.WorkspaceID, session.AnonymousID, client.ConnID)
			if err != nil {
				slog.Error("presence SetVisitorOffline", "error", err)
			}
			if shouldBroadcastVisitorOffline(h.hub.IsVisitorOnline(session.WorkspaceID, session.AnonymousID), lastConn, err) {
				h.hub.BroadcastAll(Event{
					Action:      "visitor_offline",
					Entity:      "support_visitor",
					EntityID:    session.AnonymousID,
					WorkspaceID: session.WorkspaceID,
				})
			}
		}
		conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		// Any successfully received frame proves that this connection is alive.
		// Refresh here as well as on heartbeat frames so active visitors cannot
		// expire merely because a browser delayed its interval timer.
		if session.AnonymousID != "" {
			if err := h.hub.Presence.RefreshVisitorOnline(ctx, session.WorkspaceID, session.AnonymousID, client.ConnID); err != nil {
				slog.Error("presence RefreshVisitorOnline on widget activity", "error", err)
			}
		}

		var msg model.WidgetWSMessage
		if json.Unmarshal(data, &msg) != nil {
			continue
		}

		switch msg.Type {
		case "message:send":
			typed, err := unmarshalWidgetData[model.WidgetMessageSendData](msg.Data)
			if err != nil {
				slog.Warn("widget ws: invalid message:send data", "error", err)
				SendToClient(conn, "connection:error", map[string]string{"code": "invalid_data", "message": "malformed message:send payload"})
				continue
			}
			content := typed.Content
			if strings.TrimSpace(content) == "" && len(typed.AttachmentIDs) == 0 {
				continue
			}
			previousConversationID := derefStr(client.ConversationID)
			result, err := h.service.WidgetCreateMessage(ctx, session.SessionToken, content, typed.AttachmentIDs)
			if err != nil {
				SendToClient(conn, "connection:error", map[string]string{"code": "send_failed", "message": err.Error()})
				continue
			}
			// Re-sync routing from the persisted result instead of the stale handler session.
			if result.ConversationID != "" {
				convID := result.ConversationID
				session.ConversationID = &convID
				client.ConversationID = &convID
				h.hub.SetWidgetConversation(client.UserID, convID)

				if previousConversationID != convID {
					SendToClient(conn, "conversation:created", map[string]string{"conversation_id": convID})
				}
			}
			// Echo back to sender with server-assigned ID
			SendToClient(conn, "message:received", model.WidgetMessageReceivedPayload{
				ID:             result.ID,
				ConversationID: result.ConversationID,
				Content:        result.Content,
				SenderType:     result.SenderType,
				MessageType:    result.MessageType,
				SenderName:     result.SenderDisplayName,
				SenderAvatar:   result.SenderAvatarURL,
				ViaChannel:     derefStr(result.ViaChannel),
				Attachments:    result.Attachments,
				CreatedAt:      result.CreatedAt.Format(time.RFC3339),
			})

		case "typing:start":
			conversationID := derefStr(client.ConversationID)
			if conversationID == "" {
				slog.Debug("widget ws: typing:start dropped — no conversation_id",
					"session_id", session.ID, "workspace_id", session.WorkspaceID)
				continue
			}
			slog.Debug("widget ws: broadcasting typing_started",
				"conversation_id", conversationID, "workspace_id", session.WorkspaceID, "actor", client.UserID)
			typed, _ := unmarshalWidgetData[model.WidgetTypingData](msg.Data)
			var eventData json.RawMessage
			if typed.Content != "" {
				eventData, _ = json.Marshal(map[string]string{"content": typed.Content})
			}
			h.hub.BroadcastAll(Event{
				Action:      "typing_started",
				Entity:      "support_conversation",
				EntityID:    conversationID,
				WorkspaceID: session.WorkspaceID,
				ActorID:     client.UserID,
				Data:        eventData,
			})

		case "typing:stop":
			conversationID := derefStr(client.ConversationID)
			if conversationID == "" {
				continue
			}
			slog.Debug("widget ws: broadcasting typing_stopped",
				"conversation_id", conversationID, "workspace_id", session.WorkspaceID)
			h.hub.BroadcastAll(Event{
				Action:      "typing_stopped",
				Entity:      "support_conversation",
				EntityID:    conversationID,
				WorkspaceID: session.WorkspaceID,
				ActorID:     client.UserID,
			})

		case "session:upgrade":
			typed, err := unmarshalWidgetData[model.WidgetSessionUpgradeData](msg.Data)
			if err != nil {
				slog.Warn("widget ws: invalid session:upgrade data", "error", err)
				SendToClient(conn, "connection:error", map[string]string{"code": "invalid_data", "message": "malformed session:upgrade payload"})
				continue
			}
			email := typed.Email
			name := typed.DisplayName()
			source := typed.Source
			if source == "" {
				source = "widget_prechat"
			}
			typed.Source = source
			// Allow skipping email (empty) — visitor stays anonymous but pre-chat is considered done.
			if email != "" {
				err = h.service.UpgradeWidgetSession(ctx, session.SessionToken, typed)
				if err != nil {
					SendToClient(conn, "connection:error", map[string]string{"code": "upgrade_failed", "message": err.Error()})
					continue
				}
				session.CustomerEmail = &email
				session.IsAnonymous = false
			}
			if name != "" {
				session.CustomerName = &name
			}
			SendToClient(conn, "session:upgraded", nil)

		case "session:revoke":
			h.service.RevokeWidgetSession(ctx, session.SessionToken)
			SendToClient(conn, "session:revoked", nil)
			return // exit loop, connection will close

		case "page:update":
			if pageData, ok := msg.Data["url"].(string); ok && pageData != "" {
				if err := h.service.UpdateSessionPageURL(ctx, session.SessionToken, pageData); err != nil {
					slog.Error("widget ws: page:update failed", "error", err)
				}
			}

		case "conversations:list":
			convs, _ := h.service.GetVisitorConversations(ctx, session.WorkspaceID, session.AnonymousID)
			if convs == nil {
				convs = []model.SupportConversation{}
			}
			SendToClient(conn, "conversations:listed", map[string]any{"conversations": convs})

		case "conversation:escalate":
			if session.ConversationID == nil {
				continue
			}
			if err := h.service.EscalateConversation(ctx, session.WorkspaceID, *session.ConversationID, "customer_requested"); err != nil {
				slog.Error("widget ws: escalate failed", "error", err, "conversation_id", *session.ConversationID)
				SendToClient(conn, "connection:error", map[string]string{"code": "escalate_failed", "message": "Failed to escalate to human"})
				continue
			}
			convs, _ := h.service.GetVisitorConversations(ctx, session.WorkspaceID, session.AnonymousID)
			if convs == nil {
				convs = []model.SupportConversation{}
			}
			var activeTeammate *model.WidgetActiveTeammate
			for i := range convs {
				if convs[i].ID == *session.ConversationID {
					activeTeammate = widgetActiveTeammateFromConversation(&convs[i])
					break
				}
			}
			SendToClient(conn, "conversation:escalated", map[string]any{
				"conversation_id": *session.ConversationID,
				"active_teammate": activeTeammate,
			})
			SendToClient(conn, "conversations:listed", map[string]any{"conversations": convs})

		case "conversation:new":
			// Clear the persisted active conversation before the next message creates a fresh one.
			if err := h.service.ClearSessionConversation(ctx, session.SessionToken); err != nil {
				SendToClient(conn, "connection:error", map[string]string{"code": "conversation_reset_failed", "message": err.Error()})
				continue
			}
			session.ConversationID = nil
			client.ConversationID = nil

		case "ping":
			// Extend an active session near expiry. Presence was refreshed when
			// this frame was received above.
			refreshed, err := h.service.GetWidgetSession(ctx, session.SessionToken)
			if err != nil {
				SendToClient(conn, "connection:error", map[string]string{"code": "session_expired", "message": err.Error()})
				return
			}
			session.ExpiresAt = refreshed.ExpiresAt
			SendToClient(conn, "pong", map[string]string{
				"expires_at": session.ExpiresAt.Format(time.RFC3339),
			})

		case "conversation:read":
			typed, err := unmarshalWidgetData[model.WidgetConversationSelectData](msg.Data)
			if err != nil || typed.ConversationID == "" {
				continue
			}
			if session.AnonymousID != "" {
				if err := h.service.MarkConversationReadByVisitor(ctx, session.WorkspaceID, typed.ConversationID, session.AnonymousID); err != nil {
					slog.Error("widget ws: mark read failed", "error", err, "conversation_id", typed.ConversationID)
				}
			}

		case "conversation:select":
			typed, err := unmarshalWidgetData[model.WidgetConversationSelectData](msg.Data)
			if err != nil {
				slog.Warn("widget ws: invalid conversation:select data", "error", err)
				SendToClient(conn, "connection:error", map[string]string{"code": "invalid_data", "message": "malformed conversation:select payload"})
				continue
			}
			convID := typed.ConversationID
			if convID == "" {
				continue
			}
			if err := h.service.SetSessionConversation(ctx, session.SessionToken, convID); err != nil {
				SendToClient(conn, "connection:error", map[string]string{"code": "conversation_select_failed", "message": err.Error()})
				continue
			}
			// Update session and hub routing to the selected conversation
			session.ConversationID = &convID
			client.ConversationID = &convID
			h.hub.SetWidgetConversation(client.UserID, convID)

			// Mark selected conversation as read for the visitor
			if session.AnonymousID != "" {
				if err := h.service.MarkConversationReadByVisitor(ctx, session.WorkspaceID, convID, session.AnonymousID); err != nil {
					slog.Error("widget ws: mark read on select failed", "error", err, "conversation_id", convID)
				}
			}

			// Load and send messages for the selected conversation
			msgs, err := h.service.ListConversationMessages(ctx, session.WorkspaceID, convID, false)
			if err != nil {
				SendToClient(conn, "connection:error", map[string]string{"code": "load_failed", "message": err.Error()})
				continue
			}
			if msgs == nil {
				msgs = []model.SupportMessage{}
			}
			SendToClient(conn, "conversation:messages", map[string]any{"messages": msgs})
		}
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func widgetActiveTeammateFromConversation(conversation *model.SupportConversation) *model.WidgetActiveTeammate {
	if conversation == nil || conversation.OpenedByUserID == nil || conversation.OpenedByDisplayName == nil {
		return nil
	}
	userID := strings.TrimSpace(*conversation.OpenedByUserID)
	name := strings.TrimSpace(*conversation.OpenedByDisplayName)
	if userID == "" || name == "" {
		return nil
	}
	return &model.WidgetActiveTeammate{
		UserID:    userID,
		Name:      name,
		AvatarURL: conversation.OpenedByAvatarURL,
		Status: func() string {
			if conversation.OpenedByStatus == nil {
				return ""
			}
			return strings.TrimSpace(*conversation.OpenedByStatus)
		}(),
	}
}
