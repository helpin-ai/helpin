package websocket

import (
	"encoding/json"
	"sync"
)

// PresenceState tracks who is viewing and typing in each conversation.
// Thread-safe — used by the Hub and WS handlers concurrently.
type PresenceState struct {
	mu sync.RWMutex
	// workspaceID → conversationID → set of userIDs
	viewing map[string]map[string]map[string]struct{}
	// workspaceID → conversationID → userID → draft content
	typing map[string]map[string]map[string]string
}

// NewPresenceState creates an empty presence registry.
func NewPresenceState() *PresenceState {
	return &PresenceState{
		viewing: make(map[string]map[string]map[string]struct{}),
		typing:  make(map[string]map[string]map[string]string),
	}
}

// SetViewing marks an agent as viewing a conversation. Returns true if state changed.
func (p *PresenceState) SetViewing(workspaceID, conversationID, userID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.viewing[workspaceID] == nil {
		p.viewing[workspaceID] = make(map[string]map[string]struct{})
	}
	if p.viewing[workspaceID][conversationID] == nil {
		p.viewing[workspaceID][conversationID] = make(map[string]struct{})
	}
	if _, exists := p.viewing[workspaceID][conversationID][userID]; exists {
		return false
	}
	p.viewing[workspaceID][conversationID][userID] = struct{}{}
	return true
}

// ClearViewing removes an agent from viewing a conversation. Returns true if state changed.
func (p *PresenceState) ClearViewing(workspaceID, conversationID, userID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	ws := p.viewing[workspaceID]
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
	return true
}

// SetTyping marks an agent as typing with content. Returns true if state changed.
func (p *PresenceState) SetTyping(workspaceID, conversationID, userID, content string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.typing[workspaceID] == nil {
		p.typing[workspaceID] = make(map[string]map[string]string)
	}
	if p.typing[workspaceID][conversationID] == nil {
		p.typing[workspaceID][conversationID] = make(map[string]string)
	}
	prev, existed := p.typing[workspaceID][conversationID][userID]
	p.typing[workspaceID][conversationID][userID] = content
	return !existed || prev != content
}

// ClearTyping removes an agent's typing state. Returns true if state changed.
func (p *PresenceState) ClearTyping(workspaceID, conversationID, userID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
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
	return true
}

// ClearAllForUser removes all viewing and typing state for a user across a workspace.
// Called on disconnect.
func (p *PresenceState) ClearAllForUser(workspaceID, userID string) (viewingCleared []string, typingCleared []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Clear viewing
	if ws := p.viewing[workspaceID]; ws != nil {
		for convID, viewers := range ws {
			if _, exists := viewers[userID]; exists {
				delete(viewers, userID)
				viewingCleared = append(viewingCleared, convID)
				if len(viewers) == 0 {
					delete(ws, convID)
				}
			}
		}
	}
	// Clear typing
	if ws := p.typing[workspaceID]; ws != nil {
		for convID, typers := range ws {
			if _, exists := typers[userID]; exists {
				delete(typers, userID)
				typingCleared = append(typingCleared, convID)
				if len(typers) == 0 {
					delete(ws, convID)
				}
			}
		}
	}
	return
}

// PresenceSnapshot holds the current viewers and typers for a conversation.
type PresenceSnapshot struct {
	Viewers []string          `json:"viewers"`           // userIDs currently viewing
	Typers  map[string]string `json:"typers,omitempty"`  // userID → draft content
}

// GetSnapshot returns the current presence for a conversation.
func (p *PresenceState) GetSnapshot(workspaceID, conversationID string) PresenceSnapshot {
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
	return snap
}

// MarshalSnapshot returns the JSON-encoded snapshot as RawMessage.
func (p *PresenceState) MarshalSnapshot(workspaceID, conversationID string) json.RawMessage {
	snap := p.GetSnapshot(workspaceID, conversationID)
	data, _ := json.Marshal(snap)
	return data
}
