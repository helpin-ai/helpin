package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// Event is the lightweight notification sent to clients.
type Event struct {
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
	UserID         string
	WorkspaceID    string
	IsWidget       bool    // true for widget clients, false for internal (agent) clients
	ConversationID *string // set for widget clients, scopes which events they receive
}

// Hub manages all active WebSocket clients grouped by workspace.
type Hub struct {
	mu               sync.RWMutex
	clients          map[string]map[*Client]struct{} // workspaceID -> set of clients
	sessionSubsMu    sync.RWMutex
	sessionSubs      map[string]map[*Client]struct{} // sessionID -> set of clients subscribed to stream
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{
		clients:     make(map[string]map[*Client]struct{}),
		sessionSubs: make(map[string]map[*Client]struct{}),
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.WorkspaceID] == nil {
		h.clients[c.WorkspaceID] = make(map[*Client]struct{})
	}
	h.clients[c.WorkspaceID][c] = struct{}{}
	log.Printf("[ws] client registered: user=%s workspace=%s widget=%v", c.UserID, c.WorkspaceID, c.IsWidget)
}

// Unregister removes a client from the hub.
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
}

// Broadcast sends an event to all eligible clients in the event's workspace.
// Widget clients are filtered by conversation scope via shouldReceive().
func (h *Hub) Broadcast(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[ws] failed to marshal event: %v", err)
		return
	}

	// Copy targets under read lock.
	h.mu.RLock()
	targets := make([]*Client, 0, len(h.clients[event.WorkspaceID]))
	for c := range h.clients[event.WorkspaceID] {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		if !h.shouldReceive(c, event) {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := c.Conn.Write(ctx, websocket.MessageText, data)
		cancel()
		if err != nil {
			log.Printf("[ws] write failed for user=%s, evicting: %v", c.UserID, err)
			h.Unregister(c)
			c.Conn.Close(websocket.StatusGoingAway, "write failed")
		}
	}
}

// shouldReceive determines if a client should receive an event.
// Internal clients (agents) receive everything in their workspace.
// Widget clients only receive their own conversation's events.
func (h *Hub) shouldReceive(client *Client, event Event) bool {
	if !client.IsWidget {
		return true // internal clients see everything in their workspace
	}

	switch event.Entity {
	case "support_conversation_message":
		return client.ConversationID != nil && *client.ConversationID == event.ParentID
	case "support_conversation":
		return client.ConversationID != nil && *client.ConversationID == event.EntityID
	default:
		return false // widget doesn't need PM/CRM/other events
	}
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

// SubscribeSession registers a client to receive stream events for a planning session.
func (h *Hub) SubscribeSession(client *Client, sessionID string) {
	h.sessionSubsMu.Lock()
	defer h.sessionSubsMu.Unlock()
	if h.sessionSubs[sessionID] == nil {
		h.sessionSubs[sessionID] = make(map[*Client]struct{})
	}
	h.sessionSubs[sessionID][client] = struct{}{}
}

// UnsubscribeSession removes a client from a planning session's stream events.
func (h *Hub) UnsubscribeSession(client *Client, sessionID string) {
	h.sessionSubsMu.Lock()
	defer h.sessionSubsMu.Unlock()
	if subs, ok := h.sessionSubs[sessionID]; ok {
		delete(subs, client)
		if len(subs) == 0 {
			delete(h.sessionSubs, sessionID)
		}
	}
}

// UnsubscribeAllSessions removes a client from all session subscriptions.
func (h *Hub) UnsubscribeAllSessions(client *Client) {
	h.sessionSubsMu.Lock()
	defer h.sessionSubsMu.Unlock()
	for sessionID, subs := range h.sessionSubs {
		delete(subs, client)
		if len(subs) == 0 {
			delete(h.sessionSubs, sessionID)
		}
	}
}

// SendToSession sends a stream event to all clients subscribed to a planning session.
func (h *Hub) SendToSession(sessionID string, event interface{}) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[ws] failed to marshal stream event: %v", err)
		return
	}

	h.sessionSubsMu.RLock()
	targets := make([]*Client, 0)
	for c := range h.sessionSubs[sessionID] {
		targets = append(targets, c)
	}
	h.sessionSubsMu.RUnlock()

	for _, c := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := c.Conn.Write(ctx, websocket.MessageText, data)
		cancel()
		if err != nil {
			log.Printf("[ws] stream write failed for user=%s session=%s, evicting: %v", c.UserID, sessionID, err)
			h.UnsubscribeSession(c, sessionID)
		}
	}
}
