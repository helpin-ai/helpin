package websocket

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"nhooyr.io/websocket"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
)

// generateConnID creates a unique connection identifier.
func generateConnID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

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

type presenceSyncData struct {
	ConversationIDs []string `json:"conversation_ids"`
}

type docViewingData struct {
	DocumentID string `json:"document_id"`
}

type docPresenceSyncData struct {
	DocumentIDs []string `json:"document_ids"`
}

type docEditingData struct {
	DocumentID string `json:"document_id"`
	Area       string `json:"area"`
	Section    string `json:"section,omitempty"`
}

type PresenceSnapshotViewer struct {
	UserID string  `json:"user_id"`
	Name   *string `json:"name,omitempty"`
	Avatar *string `json:"avatar,omitempty"`
}

type PresenceSnapshotTyper struct {
	Content string  `json:"content"`
	Name    *string `json:"name,omitempty"`
	Avatar  *string `json:"avatar,omitempty"`
}

type PresenceSnapshotPayload struct {
	ConversationID string                           `json:"conversation_id"`
	Viewers        []PresenceSnapshotViewer         `json:"viewers"`
	Typers         map[string]PresenceSnapshotTyper `json:"typers,omitempty"`
}

type DocPresenceSnapshotPayload struct {
	DocumentID string                       `json:"document_id"`
	Viewers    []PresenceSnapshotViewer     `json:"viewers"`
	Editors    map[string]DocPresenceEditor `json:"editors,omitempty"`
}

type DocPresenceEditor struct {
	UserID  string  `json:"user_id"`
	Area    string  `json:"area"`
	Section string  `json:"section,omitempty"`
	Name    *string `json:"name,omitempty"`
	Avatar  *string `json:"avatar,omitempty"`
}

// MarkReadFunc marks a conversation as read for the given user.
type MarkReadFunc func(ctx context.Context, workspaceID, conversationID, userID string) error

// UserLookupFunc resolves user display info (name, avatar URL) by user ID.
type UserLookupFunc func(ctx context.Context, userID string) (name string, avatar *string)

// Handler upgrades HTTP connections to WebSocket.
type Handler struct {
	hub          *Hub
	jwtManager   *auth.JWTManager
	authzService *authorization.AuthzService
	userLookup   UserLookupFunc
	markRead     MarkReadFunc
}

// NewHandler creates a WebSocket handler.
func NewHandler(hub *Hub, jwtManager *auth.JWTManager) *Handler {
	return &Handler{hub: hub, jwtManager: jwtManager}
}

// SetAuthzService injects the authorization service for workspace access checks.
func (h *Handler) SetAuthzService(authz *authorization.AuthzService) {
	h.authzService = authz
}

// SetUserLookup injects the function used to resolve user display info for typing events.
func (h *Handler) SetUserLookup(fn UserLookupFunc) {
	h.userLookup = fn
}

// SetMarkRead injects the function used to mark conversations as read.
func (h *Handler) SetMarkRead(fn MarkReadFunc) {
	h.markRead = fn
}

func (h *Handler) buildPresenceSnapshotPayload(ctx context.Context, workspaceID, conversationID string) (PresenceSnapshotPayload, error) {
	snap, err := h.hub.Presence.GetSnapshot(ctx, workspaceID, conversationID)
	if err != nil {
		return PresenceSnapshotPayload{}, err
	}

	payload := PresenceSnapshotPayload{
		ConversationID: conversationID,
		Viewers:        make([]PresenceSnapshotViewer, 0, len(snap.Viewers)),
		Typers:         make(map[string]PresenceSnapshotTyper, len(snap.Typers)),
	}

	for _, userID := range snap.Viewers {
		viewer := PresenceSnapshotViewer{UserID: userID}
		if h.userLookup != nil {
			name, avatar := h.userLookup(ctx, userID)
			if name != "" {
				viewer.Name = &name
			}
			if avatar != nil {
				viewer.Avatar = avatar
			}
		}
		payload.Viewers = append(payload.Viewers, viewer)
	}

	for userID, content := range snap.Typers {
		typer := PresenceSnapshotTyper{Content: content}
		if h.userLookup != nil {
			name, avatar := h.userLookup(ctx, userID)
			if name != "" {
				typer.Name = &name
			}
			if avatar != nil {
				typer.Avatar = avatar
			}
		}
		payload.Typers[userID] = typer
	}

	return payload, nil
}

func (h *Handler) sendPresenceSnapshot(ctx context.Context, conn *websocket.Conn, workspaceID, conversationID string) {
	payload, err := h.buildPresenceSnapshotPayload(ctx, workspaceID, conversationID)
	if err != nil {
		slog.Error("presence GetSnapshot", "error", err, "conversation_id", conversationID, "workspace_id", workspaceID)
		return
	}
	slog.Debug("presence snapshot sent",
		"conversation_id", conversationID,
		"workspace_id", workspaceID,
		"viewer_count", len(payload.Viewers),
		"typer_count", len(payload.Typers))
	if err := SendToClient(conn, "support:presence_snapshot", payload); err != nil {
		slog.Error("presence snapshot send failed", "error", err, "conversation_id", conversationID, "workspace_id", workspaceID)
	}
}

func (h *Handler) buildDocPresenceSnapshotPayload(ctx context.Context, workspaceID, documentID string) (DocPresenceSnapshotPayload, error) {
	snap, err := h.hub.Presence.GetDocSnapshot(ctx, workspaceID, documentID)
	if err != nil {
		return DocPresenceSnapshotPayload{}, err
	}

	payload := DocPresenceSnapshotPayload{
		DocumentID: documentID,
		Viewers:    make([]PresenceSnapshotViewer, 0, len(snap.Viewers)),
		Editors:    make(map[string]DocPresenceEditor, len(snap.Editors)),
	}

	for _, userID := range snap.Viewers {
		viewer := PresenceSnapshotViewer{UserID: userID}
		if h.userLookup != nil {
			name, avatar := h.userLookup(ctx, userID)
			if name != "" {
				viewer.Name = &name
			}
			if avatar != nil {
				viewer.Avatar = avatar
			}
		}
		payload.Viewers = append(payload.Viewers, viewer)
	}

	for userID, editorState := range snap.Editors {
		editor := DocPresenceEditor{
			UserID:  userID,
			Area:    editorState.Area,
			Section: editorState.Section,
		}
		if h.userLookup != nil {
			name, avatar := h.userLookup(ctx, userID)
			if name != "" {
				editor.Name = &name
			}
			if avatar != nil {
				editor.Avatar = avatar
			}
		}
		payload.Editors[userID] = editor
	}

	return payload, nil
}

func (h *Handler) sendDocPresenceSnapshot(ctx context.Context, conn *websocket.Conn, workspaceID, documentID string) {
	payload, err := h.buildDocPresenceSnapshotPayload(ctx, workspaceID, documentID)
	if err != nil {
		slog.Error("doc presence GetSnapshot", "error", err, "document_id", documentID, "workspace_id", workspaceID)
		return
	}
	slog.Debug("doc presence snapshot sent",
		"document_id", documentID,
		"workspace_id", workspaceID,
		"viewer_count", len(payload.Viewers),
		"editor_count", len(payload.Editors))
	if err := SendToClient(conn, "docs:presence_snapshot", payload); err != nil {
		slog.Error("doc presence snapshot send failed", "error", err, "document_id", documentID, "workspace_id", workspaceID)
	}
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
		ConnID:      generateConnID(),
		UserID:      claims.UserID,
		WorkspaceID: workspaceID,
	}

	h.hub.Register(client)
	defer func() {
		h.hub.Unregister(client) // also cleans up presence + broadcasts stop events
		conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	firstConn, err := h.hub.Presence.SetAgentOnline(r.Context(), workspaceID, client.UserID, client.ConnID)
	if err != nil {
		slog.Error("presence SetAgentOnline", "error", err, "user_id", client.UserID, "workspace_id", workspaceID)
	} else if firstConn {
		data, _ := json.Marshal(map[string]any{
			"user_id":      client.UserID,
			"status":       "online",
			"last_seen_at": time.Now().UTC(),
		})
		h.hub.BroadcastAll(Event{
			Action:      "updated",
			Entity:      "support_teammate_presence",
			EntityID:    client.UserID,
			WorkspaceID: workspaceID,
			ActorID:     client.UserID,
			Data:        data,
		})
	}

	// Send current online visitors as initial snapshot.
	// Try PresenceProvider first (shared across pods), fall back to Hub's in-memory map.
	visitors, err := h.hub.Presence.GetOnlineVisitors(r.Context(), workspaceID)
	if err != nil {
		slog.Error("presence GetOnlineVisitors", "error", err)
		visitors = h.hub.GetOnlineVisitors(workspaceID)
	}
	if len(visitors) > 0 {
		data, _ := json.Marshal(map[string]any{"visitors": visitors})
		SendToClient(conn, "support:online_visitors", json.RawMessage(data))
	}

	// Read loop: process client messages for support presence/typing.
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
		case "docs:viewing:start":
			var d docViewingData
			if json.Unmarshal(msg.Data, &d) != nil || d.DocumentID == "" {
				continue
			}
			ctx := r.Context()
			slog.Debug("doc presence viewing start",
				"document_id", d.DocumentID,
				"workspace_id", workspaceID,
				"user_id", client.UserID,
				"conn_id", client.ConnID)
			changed, err := h.hub.Presence.SetDocViewing(ctx, workspaceID, d.DocumentID, client.UserID, client.ConnID)
			if err != nil {
				slog.Error("doc presence SetDocViewing", "error", err)
			}
			if changed {
				var eventData json.RawMessage
				if h.userLookup != nil {
					payload := map[string]string{}
					name, avatar := h.userLookup(ctx, client.UserID)
					if name != "" {
						payload["viewer_name"] = name
					}
					if avatar != nil {
						payload["viewer_avatar"] = *avatar
					}
					if len(payload) > 0 {
						eventData, _ = json.Marshal(payload)
					}
				}
				h.hub.BroadcastAll(Event{
					Action:      "viewing_started",
					Entity:      "docs_document_presence",
					EntityID:    d.DocumentID,
					WorkspaceID: workspaceID,
					ActorID:     client.UserID,
					Data:        eventData,
				})
			}
			h.sendDocPresenceSnapshot(ctx, conn, workspaceID, d.DocumentID)

		case "docs:viewing:stop":
			var d docViewingData
			if json.Unmarshal(msg.Data, &d) != nil || d.DocumentID == "" {
				continue
			}
			slog.Debug("doc presence viewing stop",
				"document_id", d.DocumentID,
				"workspace_id", workspaceID,
				"user_id", client.UserID,
				"conn_id", client.ConnID)
			cleared, err := h.hub.Presence.ClearDocViewing(r.Context(), workspaceID, d.DocumentID, client.UserID, client.ConnID)
			if err != nil {
				slog.Error("doc presence ClearDocViewing", "error", err)
			}
			if cleared {
				h.hub.BroadcastAll(Event{
					Action:      "viewing_stopped",
					Entity:      "docs_document_presence",
					EntityID:    d.DocumentID,
					WorkspaceID: workspaceID,
					ActorID:     client.UserID,
				})
			}

		case "docs:editing:update":
			var d docEditingData
			if json.Unmarshal(msg.Data, &d) != nil || d.DocumentID == "" || d.Area == "" {
				continue
			}
			ctx := r.Context()
			slog.Debug("doc presence editing update",
				"document_id", d.DocumentID,
				"workspace_id", workspaceID,
				"user_id", client.UserID,
				"conn_id", client.ConnID,
				"area", d.Area,
				"section", d.Section)
			changed, err := h.hub.Presence.SetDocEditing(ctx, workspaceID, d.DocumentID, client.UserID, client.ConnID, d.Area, d.Section)
			if err != nil {
				slog.Error("doc presence SetDocEditing", "error", err)
				continue
			}
			if changed {
				payload := map[string]string{
					"editor_area": d.Area,
				}
				if d.Section != "" {
					payload["editor_section"] = d.Section
				}
				if h.userLookup != nil {
					name, avatar := h.userLookup(ctx, client.UserID)
					if name != "" {
						payload["editor_name"] = name
					}
					if avatar != nil {
						payload["editor_avatar"] = *avatar
					}
				}
				eventData, _ := json.Marshal(payload)
				h.hub.BroadcastAll(Event{
					Action:      "editing_updated",
					Entity:      "docs_document_presence",
					EntityID:    d.DocumentID,
					WorkspaceID: workspaceID,
					ActorID:     client.UserID,
					Data:        eventData,
				})
			}

		case "docs:editing:stop":
			var d docEditingData
			if json.Unmarshal(msg.Data, &d) != nil || d.DocumentID == "" {
				continue
			}
			slog.Debug("doc presence editing stop",
				"document_id", d.DocumentID,
				"workspace_id", workspaceID,
				"user_id", client.UserID,
				"conn_id", client.ConnID)
			cleared, err := h.hub.Presence.ClearDocEditing(r.Context(), workspaceID, d.DocumentID, client.UserID, client.ConnID)
			if err != nil {
				slog.Error("doc presence ClearDocEditing", "error", err)
				continue
			}
			if cleared {
				h.hub.BroadcastAll(Event{
					Action:      "editing_stopped",
					Entity:      "docs_document_presence",
					EntityID:    d.DocumentID,
					WorkspaceID: workspaceID,
					ActorID:     client.UserID,
				})
			}

		case "docs:presence:sync":
			var d docPresenceSyncData
			if json.Unmarshal(msg.Data, &d) != nil || len(d.DocumentIDs) == 0 {
				continue
			}
			slog.Debug("doc presence sync requested",
				"workspace_id", workspaceID,
				"user_id", client.UserID,
				"conn_id", client.ConnID,
				"document_count", len(d.DocumentIDs))
			seen := make(map[string]struct{}, len(d.DocumentIDs))
			for _, documentID := range d.DocumentIDs {
				if documentID == "" {
					continue
				}
				if _, ok := seen[documentID]; ok {
					continue
				}
				seen[documentID] = struct{}{}
				h.sendDocPresenceSnapshot(r.Context(), conn, workspaceID, documentID)
			}

		case "support:viewing:start":
			var d agentViewingData
			if json.Unmarshal(msg.Data, &d) != nil || d.ConversationID == "" {
				continue
			}
			ctx := r.Context()
			slog.Debug("presence viewing start",
				"conversation_id", d.ConversationID,
				"workspace_id", workspaceID,
				"user_id", client.UserID,
				"conn_id", client.ConnID)
			changed, err := h.hub.Presence.SetViewing(ctx, workspaceID, d.ConversationID, client.UserID, client.ConnID)
			if err != nil {
				slog.Error("presence SetViewing", "error", err)
			}
			if changed {
				h.hub.BroadcastAll(Event{
					Action:      "viewing_started",
					Entity:      "support_conversation",
					EntityID:    d.ConversationID,
					WorkspaceID: workspaceID,
					ActorID:     client.UserID,
				})
			}
			h.sendPresenceSnapshot(ctx, conn, workspaceID, d.ConversationID)

		case "support:viewing:stop":
			var d agentViewingData
			if json.Unmarshal(msg.Data, &d) != nil || d.ConversationID == "" {
				continue
			}
			slog.Debug("presence viewing stop",
				"conversation_id", d.ConversationID,
				"workspace_id", workspaceID,
				"user_id", client.UserID,
				"conn_id", client.ConnID)
			cleared, err := h.hub.Presence.ClearViewing(r.Context(), workspaceID, d.ConversationID, client.UserID, client.ConnID)
			if err != nil {
				slog.Error("presence ClearViewing", "error", err)
			}
			if cleared {
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
			slog.Debug("presence typing update",
				"message_type", msg.Type,
				"conversation_id", d.ConversationID,
				"workspace_id", workspaceID,
				"user_id", client.UserID,
				"conn_id", client.ConnID,
				"has_content", d.Content != "")
			if err := h.hub.Presence.SetTyping(r.Context(), workspaceID, d.ConversationID, client.UserID, client.ConnID, d.Content); err != nil {
				slog.Error("presence SetTyping", "error", err)
			}
			// Include agent identity so the widget can show who is typing.
			typingPayload := map[string]string{}
			if d.Content != "" {
				typingPayload["content"] = d.Content
			}
			if h.userLookup != nil {
				name, avatar := h.userLookup(r.Context(), client.UserID)
				if name != "" {
					typingPayload["agent_name"] = name
				}
				if avatar != nil {
					typingPayload["agent_avatar"] = *avatar
				}
			}
			var eventData json.RawMessage
			if len(typingPayload) > 0 {
				eventData, _ = json.Marshal(typingPayload)
			}
			h.hub.BroadcastAll(Event{
				Action:      "typing_started",
				Entity:      "support_conversation",
				EntityID:    d.ConversationID,
				WorkspaceID: workspaceID,
				ActorID:     client.UserID,
				Data:        eventData,
			})

		case "support:typing:stop":
			var d agentTypingData
			if json.Unmarshal(msg.Data, &d) != nil || d.ConversationID == "" {
				continue
			}
			slog.Debug("presence typing stop",
				"conversation_id", d.ConversationID,
				"workspace_id", workspaceID,
				"user_id", client.UserID,
				"conn_id", client.ConnID)
			cleared, err := h.hub.Presence.ClearTyping(r.Context(), workspaceID, d.ConversationID, client.UserID, client.ConnID)
			if err != nil {
				slog.Error("presence ClearTyping", "error", err)
			}
			if cleared {
				h.hub.BroadcastAll(Event{
					Action:      "typing_stopped",
					Entity:      "support_conversation",
					EntityID:    d.ConversationID,
					WorkspaceID: workspaceID,
					ActorID:     client.UserID,
				})
			}

		case "support:conversation:read":
			var d agentViewingData // reuse struct: has ConversationID field
			if json.Unmarshal(msg.Data, &d) != nil || d.ConversationID == "" {
				continue
			}
			if h.markRead != nil {
				if err := h.markRead(r.Context(), workspaceID, d.ConversationID, client.UserID); err != nil {
					slog.Error("mark conversation read via ws", "error", err,
						"conversation_id", d.ConversationID, "user_id", client.UserID)
				}
			}

		case "support:ping":
			slog.Debug("presence ping",
				"workspace_id", workspaceID,
				"user_id", client.UserID,
				"conn_id", client.ConnID)
			if err := h.hub.Presence.RefreshAgentOnline(r.Context(), workspaceID, client.UserID, client.ConnID); err != nil {
				slog.Error("presence RefreshAgentOnline", "error", err)
			}
			// Refresh all active presence keys for this agent connection (keepalive).
			if err := h.hub.Presence.RefreshAllForConn(r.Context(), workspaceID, client.UserID, client.ConnID); err != nil {
				slog.Error("presence RefreshAllForConn", "error", err)
			}

		case "support:presence:sync":
			var d presenceSyncData
			if json.Unmarshal(msg.Data, &d) != nil || len(d.ConversationIDs) == 0 {
				continue
			}
			slog.Debug("presence sync requested",
				"workspace_id", workspaceID,
				"user_id", client.UserID,
				"conn_id", client.ConnID,
				"conversation_count", len(d.ConversationIDs))
			seen := make(map[string]struct{}, len(d.ConversationIDs))
			for _, conversationID := range d.ConversationIDs {
				if conversationID == "" {
					continue
				}
				if _, ok := seen[conversationID]; ok {
					continue
				}
				seen[conversationID] = struct{}{}
				h.sendPresenceSnapshot(r.Context(), conn, workspaceID, conversationID)
			}

		default:
			slog.Debug("[ws] unknown agent message type", "type", msg.Type, "user", client.UserID)
		}
	}
}
