package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// PresenceState tracks who is viewing and typing in each conversation.
// Thread-safe — used by the Hub and WS handlers concurrently.
// Implements PresenceProvider for in-memory (single-pod / dev / tests).
// ConnID is tracked for correctness but TTL/refresh are no-ops (keys never expire).
type PresenceState struct {
	mu sync.RWMutex
	// workspaceID → conversationID → set of userIDs
	viewing map[string]map[string]map[string]struct{}
	// workspaceID → conversationID → userID → draft content
	typing map[string]map[string]map[string]string
	// workspaceID → conversationID → userID → connID → struct{} (conn-level tracking)
	viewingConns map[string]map[string]map[string]map[string]struct{}
	// workspaceID → userID → connID → conversationID (reverse index: what is this conn viewing?)
	activeViewing map[string]map[string]map[string]string
	// workspaceID → conversationID → userID → connID (typing ownership)
	typingOwner map[string]map[string]map[string]string
	// workspaceID → anonymousID → connID → struct{} (visitor connections)
	visitorConns map[string]map[string]map[string]struct{}
	// workspaceID → userID → connID → struct{} (internal agent connections)
	agentConns map[string]map[string]map[string]struct{}
	// workspaceID → userID → last seen time
	agentLastSeen map[string]map[string]time.Time
}

// NewPresenceState creates an empty presence registry.
func NewPresenceState() *PresenceState {
	return &PresenceState{
		viewing:       make(map[string]map[string]map[string]struct{}),
		typing:        make(map[string]map[string]map[string]string),
		viewingConns:  make(map[string]map[string]map[string]map[string]struct{}),
		activeViewing: make(map[string]map[string]map[string]string),
		typingOwner:   make(map[string]map[string]map[string]string),
		visitorConns:  make(map[string]map[string]map[string]struct{}),
		agentConns:    make(map[string]map[string]map[string]struct{}),
		agentLastSeen: make(map[string]map[string]time.Time),
	}
}

// --- Agent online presence ---

// SetAgentOnline marks an internal agent connection as online.
func (p *PresenceState) SetAgentOnline(_ context.Context, workspaceID, userID, connID string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.agentConns[workspaceID] == nil {
		p.agentConns[workspaceID] = make(map[string]map[string]struct{})
	}
	if p.agentConns[workspaceID][userID] == nil {
		p.agentConns[workspaceID][userID] = make(map[string]struct{})
	}
	firstConn := len(p.agentConns[workspaceID][userID]) == 0
	p.agentConns[workspaceID][userID][connID] = struct{}{}

	if p.agentLastSeen[workspaceID] == nil {
		p.agentLastSeen[workspaceID] = make(map[string]time.Time)
	}
	p.agentLastSeen[workspaceID][userID] = time.Now().UTC()

	return firstConn, nil
}

// SetAgentOffline removes an internal agent connection. Returns true if this was the last connection.
func (p *PresenceState) SetAgentOffline(_ context.Context, workspaceID, userID, connID string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	lastConn := false
	if ws := p.agentConns[workspaceID]; ws != nil {
		if conns := ws[userID]; conns != nil {
			delete(conns, connID)
			if len(conns) == 0 {
				delete(ws, userID)
				lastConn = true
				if len(ws) == 0 {
					delete(p.agentConns, workspaceID)
				}
			}
		}
	}

	if p.agentLastSeen[workspaceID] == nil {
		p.agentLastSeen[workspaceID] = make(map[string]time.Time)
	}
	p.agentLastSeen[workspaceID][userID] = time.Now().UTC()

	return lastConn, nil
}

// GetOnlineAgents returns the list of internal user IDs with at least one active connection.
func (p *PresenceState) GetOnlineAgents(_ context.Context, workspaceID string) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ws := p.agentConns[workspaceID]
	if ws == nil {
		return nil, nil
	}
	agents := make([]string, 0, len(ws))
	for userID, conns := range ws {
		if len(conns) > 0 {
			agents = append(agents, userID)
		}
	}
	return agents, nil
}

// GetAgentLastSeen returns the most recent activity timestamps for internal agents in a workspace.
func (p *PresenceState) GetAgentLastSeen(_ context.Context, workspaceID string) (map[string]time.Time, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ws := p.agentLastSeen[workspaceID]
	if ws == nil {
		return map[string]time.Time{}, nil
	}
	out := make(map[string]time.Time, len(ws))
	for userID, ts := range ws {
		out[userID] = ts
	}
	return out, nil
}

// RefreshAgentOnline updates the last-seen timestamp for an internal agent connection.
func (p *PresenceState) RefreshAgentOnline(_ context.Context, workspaceID, userID, connID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ws := p.agentConns[workspaceID]; ws != nil {
		if conns := ws[userID]; conns != nil {
			if _, ok := conns[connID]; ok {
				if p.agentLastSeen[workspaceID] == nil {
					p.agentLastSeen[workspaceID] = make(map[string]time.Time)
				}
				p.agentLastSeen[workspaceID][userID] = time.Now().UTC()
			}
		}
	}

	return nil
}

// --- Viewing ---

// SetViewing marks an agent connection as viewing a conversation.
// If this connection was viewing a different conversation, it is implicitly cleared.
// Returns true if the aggregate viewer set changed (user newly added).
func (p *PresenceState) SetViewing(_ context.Context, workspaceID, conversationID, userID, connID string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if this connection was already viewing something else.
	if wsActive, ok := p.activeViewing[workspaceID]; ok {
		if userActive, ok := wsActive[userID]; ok {
			if prevConv, ok := userActive[connID]; ok && prevConv != conversationID {
				// Implicitly clear the previous conversation (inline, already holding lock).
				p.clearViewingLocked(workspaceID, prevConv, userID, connID)
			}
		}
	}

	// Initialize nested maps.
	if p.viewing[workspaceID] == nil {
		p.viewing[workspaceID] = make(map[string]map[string]struct{})
	}
	if p.viewing[workspaceID][conversationID] == nil {
		p.viewing[workspaceID][conversationID] = make(map[string]struct{})
	}
	if p.viewingConns[workspaceID] == nil {
		p.viewingConns[workspaceID] = make(map[string]map[string]map[string]struct{})
	}
	if p.viewingConns[workspaceID][conversationID] == nil {
		p.viewingConns[workspaceID][conversationID] = make(map[string]map[string]struct{})
	}
	if p.viewingConns[workspaceID][conversationID][userID] == nil {
		p.viewingConns[workspaceID][conversationID][userID] = make(map[string]struct{})
	}
	if p.activeViewing[workspaceID] == nil {
		p.activeViewing[workspaceID] = make(map[string]map[string]string)
	}
	if p.activeViewing[workspaceID][userID] == nil {
		p.activeViewing[workspaceID][userID] = make(map[string]string)
	}

	// Track connection.
	p.viewingConns[workspaceID][conversationID][userID][connID] = struct{}{}
	p.activeViewing[workspaceID][userID][connID] = conversationID

	// Add to aggregate set — check if this is a new user.
	_, existed := p.viewing[workspaceID][conversationID][userID]
	p.viewing[workspaceID][conversationID][userID] = struct{}{}

	return !existed, nil
}

// ClearViewing removes a connection's viewing state. Returns true if user removed from aggregate set.
func (p *PresenceState) ClearViewing(_ context.Context, workspaceID, conversationID, userID, connID string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.clearViewingLocked(workspaceID, conversationID, userID, connID), nil
}

// clearViewingLocked is the lock-held implementation of ClearViewing.
func (p *PresenceState) clearViewingLocked(workspaceID, conversationID, userID, connID string) bool {
	// Remove conn key.
	if ws := p.viewingConns[workspaceID]; ws != nil {
		if conv := ws[conversationID]; conv != nil {
			if conns := conv[userID]; conns != nil {
				delete(conns, connID)
				if len(conns) == 0 {
					delete(conv, userID)
				}
			}
			if len(conv) == 0 {
				delete(ws, conversationID)
			}
		}
	}

	// Remove active viewing reverse index.
	if ws := p.activeViewing[workspaceID]; ws != nil {
		if user := ws[userID]; user != nil {
			delete(user, connID)
			if len(user) == 0 {
				delete(ws, userID)
			}
		}
	}

	// Check if any other connections remain for this user+conversation.
	hasOtherConns := false
	if ws := p.viewingConns[workspaceID]; ws != nil {
		if conv := ws[conversationID]; conv != nil {
			if conns := conv[userID]; len(conns) > 0 {
				hasOtherConns = true
			}
		}
	}

	if !hasOtherConns {
		// Remove from aggregate set.
		if ws := p.viewing[workspaceID]; ws != nil {
			if conv := ws[conversationID]; conv != nil {
				if _, exists := conv[userID]; exists {
					delete(conv, userID)
					if len(conv) == 0 {
						delete(ws, conversationID)
					}
					return true
				}
			}
		}
	}

	return false
}

// GetViewers returns the list of user IDs currently viewing a conversation.
func (p *PresenceState) GetViewers(_ context.Context, workspaceID, conversationID string) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var viewers []string
	if ws := p.viewing[workspaceID]; ws != nil {
		for uid := range ws[conversationID] {
			viewers = append(viewers, uid)
		}
	}
	if viewers == nil {
		viewers = []string{}
	}
	return viewers, nil
}

// GetActiveViewing returns the conversation this connection is currently viewing.
func (p *PresenceState) GetActiveViewing(_ context.Context, workspaceID, userID, connID string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if ws := p.activeViewing[workspaceID]; ws != nil {
		if user := ws[userID]; user != nil {
			return user[connID], nil
		}
	}
	return "", nil
}

// RefreshViewing is a no-op for in-memory (no TTL to refresh).
func (p *PresenceState) RefreshViewing(_ context.Context, _, _, _, _ string) error {
	return nil
}

// --- Typing ---

// SetTyping marks an agent as typing with content. The connID tracks ownership.
func (p *PresenceState) SetTyping(_ context.Context, workspaceID, conversationID, userID, connID, content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.typing[workspaceID] == nil {
		p.typing[workspaceID] = make(map[string]map[string]string)
	}
	if p.typing[workspaceID][conversationID] == nil {
		p.typing[workspaceID][conversationID] = make(map[string]string)
	}
	if p.typingOwner[workspaceID] == nil {
		p.typingOwner[workspaceID] = make(map[string]map[string]string)
	}
	if p.typingOwner[workspaceID][conversationID] == nil {
		p.typingOwner[workspaceID][conversationID] = make(map[string]string)
	}
	p.typing[workspaceID][conversationID][userID] = content
	p.typingOwner[workspaceID][conversationID][userID] = connID
	return nil
}

// ClearTyping removes typing state only if the connID owns it.
// Returns true if the key was actually deleted.
func (p *PresenceState) ClearTyping(_ context.Context, workspaceID, conversationID, userID, connID string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.clearTypingLocked(workspaceID, conversationID, userID, connID), nil
}

func (p *PresenceState) clearTypingLocked(workspaceID, conversationID, userID, connID string) bool {
	// Check ownership.
	if ws := p.typingOwner[workspaceID]; ws != nil {
		if conv := ws[conversationID]; conv != nil {
			if owner, exists := conv[userID]; exists && owner != connID {
				// Another connection owns the typing key — leave it.
				return false
			}
		}
	}

	ws := p.typing[workspaceID]
	if ws == nil {
		return false
	}
	conv := ws[conversationID]
	if conv == nil {
		return false
	}
	if _, exists := conv[userID]; !exists {
		return false
	}
	delete(conv, userID)
	if len(conv) == 0 {
		delete(ws, conversationID)
	}
	// Clean up owner tracking.
	if ows := p.typingOwner[workspaceID]; ows != nil {
		if oconv := ows[conversationID]; oconv != nil {
			delete(oconv, userID)
			if len(oconv) == 0 {
				delete(ows, conversationID)
			}
		}
	}
	return true
}

// GetTypers returns a map of userID -> draft content for all users typing in a conversation.
func (p *PresenceState) GetTypers(_ context.Context, workspaceID, conversationID string) (map[string]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	typers := make(map[string]string)
	if ws := p.typing[workspaceID]; ws != nil {
		for uid, content := range ws[conversationID] {
			typers[uid] = content
		}
	}
	return typers, nil
}

// --- Disconnect cleanup ---

// ClearAllForConn clears all viewing and typing state for a user+conn.
// Returns the conversation IDs that were cleared for viewing and typing.
func (p *PresenceState) ClearAllForConn(_ context.Context, workspaceID, userID, connID string) ([]string, []string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var viewingCleared []string
	var typingCleared []string

	// Find what this connection was viewing.
	var activeConv string
	if ws := p.activeViewing[workspaceID]; ws != nil {
		if user := ws[userID]; user != nil {
			activeConv = user[connID]
		}
	}

	if activeConv != "" {
		if p.clearViewingLocked(workspaceID, activeConv, userID, connID) {
			viewingCleared = append(viewingCleared, activeConv)
		}
		if p.clearTypingLocked(workspaceID, activeConv, userID, connID) {
			typingCleared = append(typingCleared, activeConv)
		}
	}

	return viewingCleared, typingCleared, nil
}

// --- Snapshot ---

// PresenceSnapshot holds the current viewers and typers for a conversation.
type PresenceSnapshot struct {
	Viewers []string          `json:"viewers"`          // userIDs currently viewing
	Typers  map[string]string `json:"typers,omitempty"` // userID → draft content
}

// GetSnapshot returns the current presence for a conversation.
func (p *PresenceState) GetSnapshot(_ context.Context, workspaceID, conversationID string) (PresenceSnapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	snap := PresenceSnapshot{
		Viewers: make([]string, 0),
		Typers:  make(map[string]string),
	}
	if ws := p.viewing[workspaceID]; ws != nil {
		for uid := range ws[conversationID] {
			snap.Viewers = append(snap.Viewers, uid)
		}
	}
	if ws := p.typing[workspaceID]; ws != nil {
		for uid, content := range ws[conversationID] {
			snap.Typers[uid] = content
		}
	}
	return snap, nil
}

// MarshalSnapshot returns the JSON-encoded snapshot as RawMessage.
func (p *PresenceState) MarshalSnapshot(workspaceID, conversationID string) json.RawMessage {
	snap, _ := p.GetSnapshot(context.Background(), workspaceID, conversationID)
	data, _ := json.Marshal(snap)
	return data
}

// --- Online visitors ---

// SetVisitorOnline marks a visitor connection as online.
func (p *PresenceState) SetVisitorOnline(_ context.Context, workspaceID, anonymousID, connID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.visitorConns[workspaceID] == nil {
		p.visitorConns[workspaceID] = make(map[string]map[string]struct{})
	}
	if p.visitorConns[workspaceID][anonymousID] == nil {
		p.visitorConns[workspaceID][anonymousID] = make(map[string]struct{})
	}
	p.visitorConns[workspaceID][anonymousID][connID] = struct{}{}
	return nil
}

// SetVisitorOffline removes a visitor connection. Returns true if this was the last connection.
func (p *PresenceState) SetVisitorOffline(_ context.Context, workspaceID, anonymousID, connID string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ws := p.visitorConns[workspaceID]; ws != nil {
		if conns := ws[anonymousID]; conns != nil {
			delete(conns, connID)
			if len(conns) == 0 {
				delete(ws, anonymousID)
				if len(ws) == 0 {
					delete(p.visitorConns, workspaceID)
				}
				return true, nil
			}
		}
	}
	return false, nil
}

// IsVisitorOnline returns true if a visitor has at least one active connection.
func (p *PresenceState) IsVisitorOnline(_ context.Context, workspaceID, anonymousID string) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if ws := p.visitorConns[workspaceID]; ws != nil {
		return len(ws[anonymousID]) > 0, nil
	}
	return false, nil
}

// GetOnlineVisitors returns the list of online anonymous_ids for a workspace.
func (p *PresenceState) GetOnlineVisitors(_ context.Context, workspaceID string) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ws := p.visitorConns[workspaceID]
	if ws == nil {
		return nil, nil
	}
	visitors := make([]string, 0, len(ws))
	for id, conns := range ws {
		if len(conns) > 0 {
			visitors = append(visitors, id)
		}
	}
	return visitors, nil
}

// RefreshVisitorOnline is a no-op for in-memory (no TTL to refresh).
func (p *PresenceState) RefreshVisitorOnline(_ context.Context, _, _, _ string) error {
	return nil
}

// RefreshAllForConn is a no-op for in-memory (no TTL to refresh).
func (p *PresenceState) RefreshAllForConn(_ context.Context, _, _, _ string) error {
	return nil
}

// Compile-time check that PresenceState implements PresenceProvider.
var _ PresenceProvider = (*PresenceState)(nil)
