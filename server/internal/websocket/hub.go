package websocket

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// Event is the lightweight notification sent to clients.
type Event struct {
	EventID     string          `json:"event_id,omitempty"`
	SentAt      time.Time       `json:"sent_at,omitempty"`
	Action      string          `json:"action"`
	Entity      string          `json:"entity"`
	EntityID    string          `json:"entity_id"`
	WorkspaceID string          `json:"workspace_id"`
	ActorID     string          `json:"actor_id"`
	ParentType  string          `json:"parent_type,omitempty"`
	ParentID    string          `json:"parent_id,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"` // hydrated payload for widget clients
}

// Client represents a single WebSocket connection.
type Client struct {
	Conn           *websocket.Conn
	ConnID         string // unique per connection, generated at accept time
	UserID         string
	WorkspaceID    string
	IsWidget       bool    // true for widget clients, false for internal (agent) clients
	ConversationID *string // set for widget clients, scopes which events they receive
	AnonymousID    string  // set for widget clients, used for visitor online tracking
}

// Hub manages all active WebSocket clients grouped by workspace.
type Hub struct {
	mu             sync.RWMutex
	clients        map[string]map[*Client]struct{} // workspaceID -> set of clients
	Presence       PresenceProvider
	onlineVisitors map[string]map[string]int // workspaceID → anonymousID → connection count
	relay          *RedisRelay               // nil in local-only mode (no Redis)
}

// NewHub creates an empty hub with an in-memory PresenceProvider as default.
func NewHub() *Hub {
	return &Hub{
		clients:        make(map[string]map[*Client]struct{}),
		Presence:       NewPresenceState(),
		onlineVisitors: make(map[string]map[string]int),
	}
}

// SetPresenceProvider replaces the default in-memory presence with an
// alternative implementation (e.g. RedisPresence for multi-pod).
// Called during DI wiring in main.go.
func (h *Hub) SetPresenceProvider(p PresenceProvider) {
	h.Presence = p
}

// SetRelay configures the Redis relay for cross-pod event broadcasting.
// When set, BroadcastAll will publish events to both the local Hub and Redis.
// Called during DI wiring in main.go. relay may be nil for local-only mode.
func (h *Hub) SetRelay(relay *RedisRelay) {
	h.relay = relay
}

// BroadcastAll sends an event to local clients via Broadcast and, if a Redis
// relay is configured, publishes to Redis for delivery to other pods.
// Use this instead of Broadcast when the event must reach all pods.
func (h *Hub) BroadcastAll(event Event) {
	go h.Broadcast(event)
	if h.relay != nil {
		go h.relay.Publish(context.Background(), event)
	}
}

// SetVisitorOnline increments the connection count for a visitor.
func (h *Hub) SetVisitorOnline(workspaceID, anonymousID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.onlineVisitors[workspaceID] == nil {
		h.onlineVisitors[workspaceID] = make(map[string]int)
	}
	h.onlineVisitors[workspaceID][anonymousID]++
}

// SetVisitorOffline decrements the connection count for a visitor. Removes entry at 0.
func (h *Hub) SetVisitorOffline(workspaceID, anonymousID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ws, ok := h.onlineVisitors[workspaceID]; ok {
		ws[anonymousID]--
		if ws[anonymousID] <= 0 {
			delete(ws, anonymousID)
		}
		if len(ws) == 0 {
			delete(h.onlineVisitors, workspaceID)
		}
	}
}

// IsVisitorOnline returns true if the visitor has at least one active connection.
func (h *Hub) IsVisitorOnline(workspaceID, anonymousID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if ws, ok := h.onlineVisitors[workspaceID]; ok {
		return ws[anonymousID] > 0
	}
	return false
}

// GetOnlineVisitors returns the list of online anonymous_ids for a workspace.
func (h *Hub) GetOnlineVisitors(workspaceID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ws, ok := h.onlineVisitors[workspaceID]
	if !ok {
		return nil
	}
	visitors := make([]string, 0, len(ws))
	for id := range ws {
		visitors = append(visitors, id)
	}
	return visitors
}

// Register adds a client to the hub. If a Redis relay is configured,
// ensures the pod is subscribed to the client's workspace channel.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.WorkspaceID] == nil {
		h.clients[c.WorkspaceID] = make(map[*Client]struct{})
	}
	h.clients[c.WorkspaceID][c] = struct{}{}
	log.Printf("[ws] client registered: user=%s workspace=%s widget=%v", c.UserID, c.WorkspaceID, c.IsWidget)

	if h.relay != nil {
		h.relay.EnsureWorkspaceSubscription(c.WorkspaceID)
	}
}

// Unregister removes a client from the hub and cleans up presence.
// If a Redis relay is configured, releases the workspace subscription
// and broadcasts disconnect events to all pods via BroadcastAll.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if ws, ok := h.clients[c.WorkspaceID]; ok {
		delete(ws, c)
		if len(ws) == 0 {
			delete(h.clients, c.WorkspaceID)
		}
	}
	log.Printf("[ws] client unregistered: user=%s workspace=%s", c.UserID, c.WorkspaceID)

	if h.relay != nil {
		h.relay.ReleaseWorkspaceSubscription(c.WorkspaceID)
	}

	// Clean up presence and broadcast stop events for internal (agent) clients.
	// Use BroadcastAll so disconnect events reach other pods too.
	if !c.IsWidget {
		ctx := context.Background()
		viewingCleared, typingCleared, err := h.Presence.ClearAllForConn(ctx, c.WorkspaceID, c.UserID, c.ConnID)
		if err != nil {
			slog.Error("presence cleanup on disconnect", "error", err,
				"user_id", c.UserID, "workspace_id", c.WorkspaceID, "conn_id", c.ConnID)
		}
		for _, convID := range viewingCleared {
			h.BroadcastAll(Event{
				Action:      "viewing_stopped",
				Entity:      "support_conversation",
				EntityID:    convID,
				WorkspaceID: c.WorkspaceID,
				ActorID:     c.UserID,
			})
		}
		for _, convID := range typingCleared {
			h.BroadcastAll(Event{
				Action:      "typing_stopped",
				Entity:      "support_conversation",
				EntityID:    convID,
				WorkspaceID: c.WorkspaceID,
				ActorID:     c.UserID,
			})
		}
	}
}

// widgetMessage is the wire format widget clients expect: {type, data}.
type widgetMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Broadcast sends an event to all eligible clients in the event's workspace.
// Internal clients receive the raw Event JSON.
// Widget clients receive a translated {type, data} message they can render directly.
func (h *Hub) Broadcast(event Event) {
	// Marshal the standard event for internal (agent) clients.
	agentData, err := json.Marshal(event)
	if err != nil {
		log.Printf("[ws] failed to marshal event: %v", err)
		return
	}

	// Prepare widget-formatted payload when applicable.
	var widgetData []byte
	switch {
	case event.Entity == "support_conversation_message" && event.Action == "created" && len(event.Data) > 0:
		wm := widgetMessage{Type: "message:received", Data: event.Data}
		widgetData, _ = json.Marshal(wm)
	case event.Entity == "support_conversation" && event.Action == "typing_started":
		widgetData, _ = json.Marshal(widgetMessage{Type: "typing:start", Data: event.Data})
	case event.Entity == "support_conversation" && event.Action == "typing_stopped":
		widgetData, _ = json.Marshal(widgetMessage{Type: "typing:stop"})
	case event.Entity == "support_conversation" && event.Action == "ai_thinking_started":
		widgetData, _ = json.Marshal(widgetMessage{Type: "ai:thinking:start"})
	case event.Entity == "support_conversation" && event.Action == "ai_thinking_stopped":
		widgetData, _ = json.Marshal(widgetMessage{Type: "ai:thinking:stop"})
	case event.Entity == "support_widget" && event.Action == "config_updated" && len(event.Data) > 0:
		widgetData, _ = json.Marshal(widgetMessage{Type: "config:updated", Data: event.Data})
	case event.Entity == "support_visitor_conversations" && event.Action == "updated" && len(event.Data) > 0:
		widgetData, _ = json.Marshal(widgetMessage{Type: "conversations:listed", Data: event.Data})
	}

	// Copy targets under read lock.
	h.mu.RLock()
	targets := make([]*Client, 0, len(h.clients[event.WorkspaceID]))
	for c := range h.clients[event.WorkspaceID] {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	isTyping := event.Entity == "support_conversation" && isTypingEvent(event.Action)
	if isTyping {
		slog.Debug("[ws] broadcasting typing event",
			"action", event.Action, "conversation_id", event.EntityID,
			"workspace_id", event.WorkspaceID, "actor_id", event.ActorID,
			"target_count", len(targets))
	}

	for _, c := range targets {
		if !h.shouldReceive(c, event) {
			if isTyping {
				slog.Debug("[ws] typing event filtered out",
					"client_user", c.UserID, "is_widget", c.IsWidget)
			}
			continue
		}
		payload := agentData
		if c.IsWidget && widgetData != nil {
			payload = widgetData
		}
		if isTyping {
			slog.Debug("[ws] delivering typing event to client",
				"client_user", c.UserID, "is_widget", c.IsWidget)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := c.Conn.Write(ctx, websocket.MessageText, payload)
		cancel()
		if err != nil {
			log.Printf("[ws] write failed for user=%s, evicting: %v", c.UserID, err)
			h.Unregister(c)
			c.Conn.Close(websocket.StatusGoingAway, "write failed")
		}
	}
}

// shouldReceive determines if a client should receive an event.
// Internal clients receive all workspace typing events except their own.
// Widget clients only receive their own conversation's events, and only
// agent-origin typing indicators.
func (h *Hub) shouldReceive(client *Client, event Event) bool {
	// Visitor online/offline events go to internal (agent) clients only
	if event.Entity == "support_visitor" {
		return !client.IsWidget
	}

	// Visitor-scoped conversation list refresh: deliver to all widget clients matching the visitor's AnonymousID
	if event.Entity == "support_visitor_conversations" {
		return client.IsWidget && client.AnonymousID == event.EntityID
	}

	// Widget config updates go to all widget clients in the workspace
	if event.Entity == "support_widget" && event.Action == "config_updated" {
		return client.IsWidget
	}

	if event.Entity == "support_conversation" && (isTypingEvent(event.Action) || isViewingEvent(event.Action)) {
		if client.IsWidget {
			// Widget clients only get agent-origin typing for their conversation
			if !isTypingEvent(event.Action) {
				return false // widgets don't need viewing events
			}
			return client.ConversationID != nil &&
				*client.ConversationID == event.EntityID &&
				!isWidgetActor(event.ActorID)
		}
		// Internal clients receive all typing/viewing except their own
		return event.ActorID != client.UserID
	}

	if !client.IsWidget {
		return true // internal clients see everything else in their workspace
	}

	// Don't echo message events back to the originating widget client —
	// the handler already sends a direct response to the sender.
	if event.Entity == "support_conversation_message" &&
		event.ActorID != "" && client.UserID == event.ActorID {
		return false
	}

	switch event.Entity {
	case "support_conversation_message":
		return len(event.Data) > 0 && client.ConversationID != nil && *client.ConversationID == event.ParentID
	case "support_conversation":
		return client.ConversationID != nil && *client.ConversationID == event.EntityID
	default:
		return false // widget doesn't need PM/CRM/other events
	}
}

func isTypingEvent(action string) bool {
	return action == "typing_started" || action == "typing_stopped"
}

func isViewingEvent(action string) bool {
	return action == "viewing_started" || action == "viewing_stopped"
}

func isWidgetActor(actorID string) bool {
	return strings.HasPrefix(actorID, "widget:")
}

// SetWidgetConversation updates a widget client's conversation_id by UserID.
func (h *Hub) SetWidgetConversation(userID string, conversationID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, clients := range h.clients {
		for client := range clients {
			if client.UserID == userID {
				client.ConversationID = &conversationID
			}
		}
	}
}

// SendToClient sends a typed WS message directly to a specific connection.
func SendToClient(conn *websocket.Conn, msgType string, data interface{}) error {
	payload := map[string]interface{}{
		"type": msgType,
	}
	if data != nil {
		payload["data"] = data
	}
	msg, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, msg)
}
