package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Key TTL constants for Redis presence state.
const (
	agentConnTTL   = 90 * time.Second
	agentSeenTTL   = 24 * time.Hour
	viewingConnTTL = 60 * time.Second
	docViewingTTL  = 60 * time.Second
	docEditingTTL  = 20 * time.Second
	typingTTL      = 15 * time.Second
	visitorConnTTL = 90 * time.Second
)

// RedisPresence implements PresenceProvider using Redis for shared state
// across multiple pods. All methods are safe for concurrent use.
type RedisPresence struct {
	rdb   *redis.Client
	podID string
}

// NewRedisPresence creates a Redis-backed presence provider.
func NewRedisPresence(rdb *redis.Client, podID string) *RedisPresence {
	return &RedisPresence{rdb: rdb, podID: podID}
}

// --- Key builders ---

// support:viewing:conn:{workspaceID}:{conversationID}:{userID}:{connID}
func viewingConnKey(workspaceID, conversationID, userID, connID string) string {
	return fmt.Sprintf("support:viewing:conn:%s:%s:%s:%s", workspaceID, conversationID, userID, connID)
}

// support:agents:conn:{workspaceID}:{userID}:{podID}:{connID}
func agentConnKey(workspaceID, userID, podID, connID string) string {
	return fmt.Sprintf("support:agents:conn:%s:%s:%s:%s", workspaceID, userID, podID, connID)
}

// support:agents:last_seen:{workspaceID}:{userID}
func agentLastSeenKey(workspaceID, userID string) string {
	return fmt.Sprintf("support:agents:last_seen:%s:%s", workspaceID, userID)
}

// support:viewing:active:{workspaceID}:{userID}:{connID}
func viewingActiveKey(workspaceID, userID, connID string) string {
	return fmt.Sprintf("support:viewing:active:%s:%s:%s", workspaceID, userID, connID)
}

// support:viewing:{workspaceID}:{conversationID}
func viewingSetKey(workspaceID, conversationID string) string {
	return fmt.Sprintf("support:viewing:%s:%s", workspaceID, conversationID)
}

// docs:viewing:conn:{workspaceID}:{documentID}:{userID}:{connID}
func docViewingConnKey(workspaceID, documentID, userID, connID string) string {
	return fmt.Sprintf("docs:viewing:conn:%s:%s:%s:%s", workspaceID, documentID, userID, connID)
}

// docs:viewing:active:{workspaceID}:{userID}:{connID}
func docViewingActiveKey(workspaceID, userID, connID string) string {
	return fmt.Sprintf("docs:viewing:active:%s:%s:%s", workspaceID, userID, connID)
}

// docs:viewing:{workspaceID}:{documentID}
func docViewingSetKey(workspaceID, documentID string) string {
	return fmt.Sprintf("docs:viewing:%s:%s", workspaceID, documentID)
}

// docs:editing:{workspaceID}:{documentID}:{userID}
func docEditingKey(workspaceID, documentID, userID string) string {
	return fmt.Sprintf("docs:editing:%s:%s:%s", workspaceID, documentID, userID)
}

// docs:editing:active:{workspaceID}:{userID}:{connID}
func docEditingActiveKey(workspaceID, userID, connID string) string {
	return fmt.Sprintf("docs:editing:active:%s:%s:%s", workspaceID, userID, connID)
}

// support:typing:{workspaceID}:{conversationID}:{userID}
func typingKey(workspaceID, conversationID, userID string) string {
	return fmt.Sprintf("support:typing:%s:%s:%s", workspaceID, conversationID, userID)
}

// support:visitors:conn:{workspaceID}:{anonymousID}:{podID}:{connID}
func visitorConnKey(workspaceID, anonymousID, podID, connID string) string {
	return fmt.Sprintf("support:visitors:conn:%s:%s:%s:%s", workspaceID, anonymousID, podID, connID)
}

// support:visitors:online:{workspaceID}
func visitorSetKey(workspaceID string) string {
	return fmt.Sprintf("support:visitors:online:%s", workspaceID)
}

// support:visitors:online:{workspaceID}:{anonymousID}
func visitorOnlineKey(workspaceID, anonymousID string) string {
	return fmt.Sprintf("support:visitors:online:%s:%s", workspaceID, anonymousID)
}

// --- Agent online presence ---

// SetAgentOnline marks an internal agent connection as online.
func (p *RedisPresence) SetAgentOnline(ctx context.Context, workspaceID, userID, connID string) (bool, error) {
	existing, err := p.scanKeys(ctx, agentConnKey(workspaceID, userID, "*", "*"), 1)
	if err != nil {
		return false, fmt.Errorf("redis presence SetAgentOnline scan: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	pipe := p.rdb.Pipeline()
	pipe.Set(ctx, agentConnKey(workspaceID, userID, p.podID, connID), "1", agentConnTTL)
	pipe.Set(ctx, agentLastSeenKey(workspaceID, userID), now, agentSeenTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("redis presence SetAgentOnline: %w", err)
	}

	return len(existing) == 0, nil
}

// SetAgentOffline removes an internal agent connection. Returns true if this was the last connection.
func (p *RedisPresence) SetAgentOffline(ctx context.Context, workspaceID, userID, connID string) (bool, error) {
	p.rdb.Del(ctx, agentConnKey(workspaceID, userID, p.podID, connID))

	remaining, err := p.scanKeys(ctx, agentConnKey(workspaceID, userID, "*", "*"), 1)
	if err != nil {
		return false, fmt.Errorf("redis presence SetAgentOffline scan: %w", err)
	}
	return len(remaining) == 0, nil
}

// GetOnlineAgents returns the list of internal user IDs with at least one active connection.
func (p *RedisPresence) GetOnlineAgents(ctx context.Context, workspaceID string) ([]string, error) {
	keys, err := p.scanKeys(ctx, fmt.Sprintf("support:agents:conn:%s:*", workspaceID), 10_000)
	if err != nil {
		return nil, fmt.Errorf("redis presence GetOnlineAgents: %w", err)
	}
	if len(keys) == 0 {
		return []string{}, nil
	}

	seen := make(map[string]struct{})
	agents := make([]string, 0, len(keys))
	prefix := fmt.Sprintf("support:agents:conn:%s:", workspaceID)
	for _, key := range keys {
		rest := strings.TrimPrefix(key, prefix)
		parts := strings.SplitN(rest, ":", 3)
		if len(parts) < 3 || parts[0] == "" {
			continue
		}
		userID := parts[0]
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		agents = append(agents, userID)
	}
	return agents, nil
}

// GetAgentLastSeen returns recent activity timestamps for internal agents in a workspace.
func (p *RedisPresence) GetAgentLastSeen(ctx context.Context, workspaceID string) (map[string]time.Time, error) {
	keys, err := p.scanKeys(ctx, fmt.Sprintf("support:agents:last_seen:%s:*", workspaceID), 10_000)
	if err != nil {
		return nil, fmt.Errorf("redis presence GetAgentLastSeen scan: %w", err)
	}
	if len(keys) == 0 {
		return map[string]time.Time{}, nil
	}

	vals, err := p.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis presence GetAgentLastSeen MGet: %w", err)
	}

	out := make(map[string]time.Time, len(keys))
	prefix := fmt.Sprintf("support:agents:last_seen:%s:", workspaceID)
	for i, key := range keys {
		raw, ok := vals[i].(string)
		if !ok || raw == "" {
			continue
		}
		userID := strings.TrimPrefix(key, prefix)
		ts, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil || userID == "" {
			continue
		}
		out[userID] = ts.UTC()
	}
	return out, nil
}

// RefreshAgentOnline refreshes the TTL for an internal agent connection without
// mutating the last-activity timestamp.
func (p *RedisPresence) RefreshAgentOnline(ctx context.Context, workspaceID, userID, connID string) error {
	if err := p.rdb.Set(ctx, agentConnKey(workspaceID, userID, p.podID, connID), "1", agentConnTTL).Err(); err != nil {
		return fmt.Errorf("redis presence RefreshAgentOnline: %w", err)
	}
	return nil
}

// TouchAgentActivity records recent agent activity for an active connection.
func (p *RedisPresence) TouchAgentActivity(ctx context.Context, workspaceID, userID, connID string) error {
	connExists, err := p.rdb.Exists(ctx, agentConnKey(workspaceID, userID, p.podID, connID)).Result()
	if err != nil {
		return fmt.Errorf("redis presence TouchAgentActivity exists: %w", err)
	}
	if connExists == 0 {
		return nil
	}
	if err := p.rdb.Set(ctx, agentLastSeenKey(workspaceID, userID), time.Now().UTC().Format(time.RFC3339Nano), agentSeenTTL).Err(); err != nil {
		return fmt.Errorf("redis presence TouchAgentActivity last_seen: %w", err)
	}
	return nil
}

// --- Viewing ---

// SetViewing marks an agent connection as viewing a conversation.
// If this connection was viewing a different conversation, it is implicitly cleared.
// Returns true if the aggregate viewer set changed (new user added).
func (p *RedisPresence) SetViewing(ctx context.Context, workspaceID, conversationID, userID, connID string) (bool, error) {
	// Check if this connection was already viewing something else.
	activeKey := viewingActiveKey(workspaceID, userID, connID)
	prevConv, err := p.rdb.Get(ctx, activeKey).Result()
	if err == nil && prevConv != "" && prevConv != conversationID {
		// Implicitly clear the previous conversation.
		if _, clearErr := p.ClearViewing(ctx, workspaceID, prevConv, userID, connID); clearErr != nil {
			slog.Warn("redis presence: clear previous viewing",
				"error", clearErr, "prev_conversation", prevConv)
		}
	}

	pipe := p.rdb.Pipeline()

	// Set the conn key with TTL.
	connKey := viewingConnKey(workspaceID, conversationID, userID, connID)
	pipe.Set(ctx, connKey, "1", viewingConnTTL)

	// Set the active reverse-index key with TTL.
	pipe.Set(ctx, activeKey, conversationID, viewingConnTTL)

	// Add to aggregate set.
	setKey := viewingSetKey(workspaceID, conversationID)
	pipe.SAdd(ctx, setKey, userID)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("redis presence SetViewing: %w", err)
	}

	// The aggregate set add returns whether the member was newly added.
	// Since we're using a pipeline, check the SAdd result.
	// Re-check: was this user already in the set?
	// For simplicity, we report changed=true if no error. The caller uses this
	// to decide whether to broadcast; broadcasting an extra viewing_started
	// for an already-viewing user is harmless (idempotent on the frontend).
	return true, nil
}

// ClearViewing removes a connection's viewing state for a conversation.
// Returns true if the user was removed from the aggregate set (no other conn keys remain).
func (p *RedisPresence) ClearViewing(ctx context.Context, workspaceID, conversationID, userID, connID string) (bool, error) {
	// Delete the conn key.
	connKey := viewingConnKey(workspaceID, conversationID, userID, connID)
	p.rdb.Del(ctx, connKey)

	// Delete the active reverse-index key.
	activeKey := viewingActiveKey(workspaceID, userID, connID)
	p.rdb.Del(ctx, activeKey)

	// Check if any other conn keys remain for this user+conversation.
	pattern := viewingConnKey(workspaceID, conversationID, userID, "*")
	keys, err := p.scanKeys(ctx, pattern, 1)
	if err != nil {
		return false, fmt.Errorf("redis presence ClearViewing scan: %w", err)
	}

	if len(keys) == 0 {
		// No other connections — remove from aggregate set.
		setKey := viewingSetKey(workspaceID, conversationID)
		removed, err := p.rdb.SRem(ctx, setKey, userID).Result()
		if err != nil {
			return false, fmt.Errorf("redis presence ClearViewing SRem: %w", err)
		}
		return removed > 0, nil
	}

	return false, nil
}

// GetViewers returns the list of user IDs currently viewing a conversation.
func (p *RedisPresence) GetViewers(ctx context.Context, workspaceID, conversationID string) ([]string, error) {
	setKey := viewingSetKey(workspaceID, conversationID)
	members, err := p.rdb.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, fmt.Errorf("redis presence GetViewers: %w", err)
	}
	return members, nil
}

// GetActiveViewing returns the conversation this connection is currently viewing.
// Returns empty string if not viewing anything.
func (p *RedisPresence) GetActiveViewing(ctx context.Context, workspaceID, userID, connID string) (string, error) {
	activeKey := viewingActiveKey(workspaceID, userID, connID)
	val, err := p.rdb.Get(ctx, activeKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("redis presence GetActiveViewing: %w", err)
	}
	return val, nil
}

// RefreshViewing extends the TTL on viewing conn key and active key.
func (p *RedisPresence) RefreshViewing(ctx context.Context, workspaceID, conversationID, userID, connID string) error {
	pipe := p.rdb.Pipeline()
	connKey := viewingConnKey(workspaceID, conversationID, userID, connID)
	activeKey := viewingActiveKey(workspaceID, userID, connID)
	pipe.Expire(ctx, connKey, viewingConnTTL)
	pipe.Expire(ctx, activeKey, viewingConnTTL)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis presence RefreshViewing: %w", err)
	}
	return nil
}

// --- Typing ---

// SetTyping marks a user as typing in a conversation. The value stores the
// connID and draft content as "{connID}|{content}" for ownership tracking.
func (p *RedisPresence) SetTyping(ctx context.Context, workspaceID, conversationID, userID, connID, content string) error {
	key := typingKey(workspaceID, conversationID, userID)
	val := connID + "|" + content
	err := p.rdb.Set(ctx, key, val, typingTTL).Err()
	if err != nil {
		return fmt.Errorf("redis presence SetTyping: %w", err)
	}
	return nil
}

// ClearTyping removes typing state only if the current connID owns the key.
// Returns true if the key was actually deleted.
func (p *RedisPresence) ClearTyping(ctx context.Context, workspaceID, conversationID, userID, connID string) (bool, error) {
	key := typingKey(workspaceID, conversationID, userID)
	val, err := p.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis presence ClearTyping GET: %w", err)
	}

	// Only delete if this connection owns the key.
	if strings.HasPrefix(val, connID+"|") {
		deleted, err := p.rdb.Del(ctx, key).Result()
		if err != nil {
			return false, fmt.Errorf("redis presence ClearTyping DEL: %w", err)
		}
		return deleted > 0, nil
	}

	// Another tab has overwritten — leave it alone.
	return false, nil
}

// GetTypers returns a map of userID → draft content for all users typing in a conversation.
func (p *RedisPresence) GetTypers(ctx context.Context, workspaceID, conversationID string) (map[string]string, error) {
	pattern := typingKey(workspaceID, conversationID, "*")
	keys, err := p.scanKeys(ctx, pattern, 100)
	if err != nil {
		return nil, fmt.Errorf("redis presence GetTypers scan: %w", err)
	}

	typers := make(map[string]string)
	if len(keys) == 0 {
		return typers, nil
	}

	vals, err := p.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis presence GetTypers MGet: %w", err)
	}

	// Key format: support:typing:{ws}:{conv}:{userID}
	prefix := typingKey(workspaceID, conversationID, "")
	for i, key := range keys {
		if vals[i] == nil {
			continue
		}
		userID := strings.TrimPrefix(key, prefix)
		raw, ok := vals[i].(string)
		if !ok {
			continue
		}
		// Value format: "{connID}|{content}" — extract content.
		if idx := strings.Index(raw, "|"); idx >= 0 {
			typers[userID] = raw[idx+1:]
		}
	}

	return typers, nil
}

// --- Disconnect cleanup ---

// ClearAllForConn clears all viewing and typing state for a user+conn.
// Returns the conversation IDs that were cleared for viewing and typing.
func (p *RedisPresence) ClearAllForConn(ctx context.Context, workspaceID, userID, connID string) ([]string, []string, error) {
	var viewingCleared []string
	var typingCleared []string

	// Look up what conversation this connection was viewing.
	activeConv, err := p.GetActiveViewing(ctx, workspaceID, userID, connID)
	if err != nil {
		return nil, nil, fmt.Errorf("redis presence ClearAllForConn GetActiveViewing: %w", err)
	}

	if activeConv != "" {
		changed, err := p.ClearViewing(ctx, workspaceID, activeConv, userID, connID)
		if err != nil {
			slog.Warn("redis presence ClearAllForConn: clear viewing",
				"error", err, "conversation_id", activeConv)
		}
		if changed {
			viewingCleared = append(viewingCleared, activeConv)
		}

		// Also try to clear typing for this conversation.
		cleared, err := p.ClearTyping(ctx, workspaceID, activeConv, userID, connID)
		if err != nil {
			slog.Warn("redis presence ClearAllForConn: clear typing",
				"error", err, "conversation_id", activeConv)
		}
		if cleared {
			typingCleared = append(typingCleared, activeConv)
		}
	}

	return viewingCleared, typingCleared, nil
}

// --- Snapshot ---

// GetSnapshot returns the current viewers and typers for a conversation.
func (p *RedisPresence) GetSnapshot(ctx context.Context, workspaceID, conversationID string) (PresenceSnapshot, error) {
	snap := PresenceSnapshot{
		Viewers: make([]string, 0),
		Typers:  make(map[string]string),
	}

	viewers, err := p.GetViewers(ctx, workspaceID, conversationID)
	if err != nil {
		return snap, err
	}
	snap.Viewers = viewers

	typers, err := p.GetTypers(ctx, workspaceID, conversationID)
	if err != nil {
		return snap, err
	}
	snap.Typers = typers

	return snap, nil
}

func (p *RedisPresence) SetDocViewing(ctx context.Context, workspaceID, documentID, userID, connID string) (bool, error) {
	activeKey := docViewingActiveKey(workspaceID, userID, connID)
	prevDoc, err := p.rdb.Get(ctx, activeKey).Result()
	if err == nil && prevDoc != "" && prevDoc != documentID {
		if _, clearErr := p.ClearDocViewing(ctx, workspaceID, prevDoc, userID, connID); clearErr != nil {
			slog.Warn("redis presence: clear previous doc viewing",
				"error", clearErr, "prev_document", prevDoc)
		}
	}

	pipe := p.rdb.Pipeline()
	connKey := docViewingConnKey(workspaceID, documentID, userID, connID)
	pipe.Set(ctx, connKey, "1", docViewingTTL)
	pipe.Set(ctx, activeKey, documentID, docViewingTTL)
	setKey := docViewingSetKey(workspaceID, documentID)
	pipe.SAdd(ctx, setKey, userID)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("redis presence SetDocViewing: %w", err)
	}
	return true, nil
}

func (p *RedisPresence) ClearDocViewing(ctx context.Context, workspaceID, documentID, userID, connID string) (bool, error) {
	connKey := docViewingConnKey(workspaceID, documentID, userID, connID)
	p.rdb.Del(ctx, connKey)

	activeKey := docViewingActiveKey(workspaceID, userID, connID)
	p.rdb.Del(ctx, activeKey)

	pattern := docViewingConnKey(workspaceID, documentID, userID, "*")
	keys, err := p.scanKeys(ctx, pattern, 1)
	if err != nil {
		return false, fmt.Errorf("redis presence ClearDocViewing scan: %w", err)
	}

	if len(keys) == 0 {
		setKey := docViewingSetKey(workspaceID, documentID)
		removed, err := p.rdb.SRem(ctx, setKey, userID).Result()
		if err != nil {
			return false, fmt.Errorf("redis presence ClearDocViewing SRem: %w", err)
		}
		return removed > 0, nil
	}

	return false, nil
}

func (p *RedisPresence) GetDocViewers(ctx context.Context, workspaceID, documentID string) ([]string, error) {
	setKey := docViewingSetKey(workspaceID, documentID)
	members, err := p.rdb.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, fmt.Errorf("redis presence GetDocViewers: %w", err)
	}
	return members, nil
}

func (p *RedisPresence) GetActiveDocViewing(ctx context.Context, workspaceID, userID, connID string) (string, error) {
	activeKey := docViewingActiveKey(workspaceID, userID, connID)
	val, err := p.rdb.Get(ctx, activeKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("redis presence GetActiveDocViewing: %w", err)
	}
	return val, nil
}

func (p *RedisPresence) RefreshDocViewing(ctx context.Context, workspaceID, documentID, userID, connID string) error {
	pipe := p.rdb.Pipeline()
	connKey := docViewingConnKey(workspaceID, documentID, userID, connID)
	activeKey := docViewingActiveKey(workspaceID, userID, connID)
	pipe.Expire(ctx, connKey, docViewingTTL)
	pipe.Expire(ctx, activeKey, docViewingTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis presence RefreshDocViewing: %w", err)
	}
	return nil
}

func (p *RedisPresence) ClearAllDocViewingForConn(ctx context.Context, workspaceID, userID, connID string) ([]string, error) {
	var viewingCleared []string

	activeDoc, err := p.GetActiveDocViewing(ctx, workspaceID, userID, connID)
	if err != nil {
		return nil, fmt.Errorf("redis presence ClearAllDocViewingForConn GetActiveDocViewing: %w", err)
	}
	if activeDoc != "" {
		changed, err := p.ClearDocViewing(ctx, workspaceID, activeDoc, userID, connID)
		if err != nil {
			slog.Warn("redis presence ClearAllDocViewingForConn: clear doc viewing",
				"error", err, "document_id", activeDoc)
		}
		if changed {
			viewingCleared = append(viewingCleared, activeDoc)
		}
	}

	return viewingCleared, nil
}

func (p *RedisPresence) SetDocEditing(ctx context.Context, workspaceID, documentID, userID, connID, area, section string) (bool, error) {
	prev, err := p.GetActiveDocEditing(ctx, workspaceID, userID, connID)
	if err != nil {
		return false, fmt.Errorf("redis presence SetDocEditing active: %w", err)
	}
	if prev.DocumentID != "" && prev.DocumentID != documentID {
		if _, err := p.ClearDocEditing(ctx, workspaceID, prev.DocumentID, userID, connID); err != nil {
			slog.Warn("redis presence SetDocEditing: clear previous editing", "error", err, "document_id", prev.DocumentID)
		}
	}

	next := DocEditorPresence{
		Area:    area,
		Section: section,
	}
	nextBytes, err := json.Marshal(struct {
		ConnID  string `json:"conn_id"`
		Area    string `json:"area"`
		Section string `json:"section,omitempty"`
	}{
		ConnID:  connID,
		Area:    area,
		Section: section,
	})
	if err != nil {
		return false, fmt.Errorf("redis presence SetDocEditing marshal state: %w", err)
	}
	activeBytes, err := json.Marshal(DocEditorPresenceRef{
		DocumentID: documentID,
		Area:       area,
		Section:    section,
	})
	if err != nil {
		return false, fmt.Errorf("redis presence SetDocEditing marshal active: %w", err)
	}

	key := docEditingKey(workspaceID, documentID, userID)
	existing, err := p.rdb.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return false, fmt.Errorf("redis presence SetDocEditing GET: %w", err)
	}

	pipe := p.rdb.Pipeline()
	pipe.Set(ctx, key, nextBytes, docEditingTTL)
	pipe.Set(ctx, docEditingActiveKey(workspaceID, userID, connID), activeBytes, docEditingTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("redis presence SetDocEditing SET: %w", err)
	}

	changed := true
	if existing != "" {
		var prevState struct {
			ConnID  string `json:"conn_id"`
			Area    string `json:"area"`
			Section string `json:"section,omitempty"`
		}
		if json.Unmarshal([]byte(existing), &prevState) == nil &&
			prevState.ConnID == connID &&
			prevState.Area == next.Area &&
			prevState.Section == next.Section {
			changed = false
		}
	}
	return changed, nil
}

func (p *RedisPresence) ClearDocEditing(ctx context.Context, workspaceID, documentID, userID, connID string) (bool, error) {
	key := docEditingKey(workspaceID, documentID, userID)
	val, err := p.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		p.rdb.Del(ctx, docEditingActiveKey(workspaceID, userID, connID))
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("redis presence ClearDocEditing GET: %w", err)
	}

	var state struct {
		ConnID string `json:"conn_id"`
	}
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return false, fmt.Errorf("redis presence ClearDocEditing unmarshal: %w", err)
	}
	if state.ConnID != connID {
		p.rdb.Del(ctx, docEditingActiveKey(workspaceID, userID, connID))
		return false, nil
	}

	pipe := p.rdb.Pipeline()
	pipe.Del(ctx, key)
	pipe.Del(ctx, docEditingActiveKey(workspaceID, userID, connID))
	res, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("redis presence ClearDocEditing DEL: %w", err)
	}

	var deleted int64
	for _, cmd := range res {
		if intCmd, ok := cmd.(*redis.IntCmd); ok {
			deleted += intCmd.Val()
		}
	}
	return deleted > 0, nil
}

func (p *RedisPresence) GetActiveDocEditing(ctx context.Context, workspaceID, userID, connID string) (DocEditorPresenceRef, error) {
	val, err := p.rdb.Get(ctx, docEditingActiveKey(workspaceID, userID, connID)).Result()
	if err == redis.Nil {
		return DocEditorPresenceRef{}, nil
	}
	if err != nil {
		return DocEditorPresenceRef{}, fmt.Errorf("redis presence GetActiveDocEditing: %w", err)
	}

	var ref DocEditorPresenceRef
	if err := json.Unmarshal([]byte(val), &ref); err != nil {
		return DocEditorPresenceRef{}, fmt.Errorf("redis presence GetActiveDocEditing unmarshal: %w", err)
	}
	return ref, nil
}

func (p *RedisPresence) RefreshDocEditing(ctx context.Context, workspaceID, documentID, userID, connID string) error {
	activeKey := docEditingActiveKey(workspaceID, userID, connID)
	val, err := p.rdb.Get(ctx, docEditingKey(workspaceID, documentID, userID)).Result()
	if err == redis.Nil {
		p.rdb.Del(ctx, activeKey)
		return nil
	}
	if err != nil {
		return fmt.Errorf("redis presence RefreshDocEditing GET: %w", err)
	}

	var state struct {
		ConnID string `json:"conn_id"`
	}
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return fmt.Errorf("redis presence RefreshDocEditing unmarshal: %w", err)
	}
	if state.ConnID != connID {
		p.rdb.Del(ctx, activeKey)
		return nil
	}

	pipe := p.rdb.Pipeline()
	pipe.Expire(ctx, docEditingKey(workspaceID, documentID, userID), docEditingTTL)
	pipe.Expire(ctx, activeKey, docEditingTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis presence RefreshDocEditing: %w", err)
	}
	return nil
}

func (p *RedisPresence) ClearAllDocEditingForConn(ctx context.Context, workspaceID, userID, connID string) ([]string, error) {
	var editingCleared []string

	active, err := p.GetActiveDocEditing(ctx, workspaceID, userID, connID)
	if err != nil {
		return nil, fmt.Errorf("redis presence ClearAllDocEditingForConn GetActiveDocEditing: %w", err)
	}
	if active.DocumentID != "" {
		changed, err := p.ClearDocEditing(ctx, workspaceID, active.DocumentID, userID, connID)
		if err != nil {
			slog.Warn("redis presence ClearAllDocEditingForConn: clear doc editing",
				"error", err, "document_id", active.DocumentID)
		}
		if changed {
			editingCleared = append(editingCleared, active.DocumentID)
		}
	}

	return editingCleared, nil
}

func (p *RedisPresence) GetDocSnapshot(ctx context.Context, workspaceID, documentID string) (DocPresenceSnapshot, error) {
	snap := DocPresenceSnapshot{
		Viewers: make([]string, 0),
		Editors: make(map[string]DocEditorPresence),
	}
	viewers, err := p.GetDocViewers(ctx, workspaceID, documentID)
	if err != nil {
		return snap, err
	}
	snap.Viewers = viewers

	keys, err := p.scanKeys(ctx, docEditingKey(workspaceID, documentID, "*"), 100)
	if err != nil {
		return snap, fmt.Errorf("redis presence GetDocSnapshot editors scan: %w", err)
	}
	if len(keys) > 0 {
		vals, err := p.rdb.MGet(ctx, keys...).Result()
		if err != nil {
			return snap, fmt.Errorf("redis presence GetDocSnapshot editors MGet: %w", err)
		}
		prefix := docEditingKey(workspaceID, documentID, "")
		for i, key := range keys {
			if vals[i] == nil {
				continue
			}
			userID := strings.TrimPrefix(key, prefix)
			raw, ok := vals[i].(string)
			if !ok {
				continue
			}
			var state struct {
				ConnID  string `json:"conn_id"`
				Area    string `json:"area"`
				Section string `json:"section,omitempty"`
			}
			if json.Unmarshal([]byte(raw), &state) != nil {
				continue
			}
			snap.Editors[userID] = DocEditorPresence{
				Area:    state.Area,
				Section: state.Section,
			}
		}
	}
	return snap, nil
}

// --- Online visitors ---

// SetVisitorOnline marks a visitor connection as online.
func (p *RedisPresence) SetVisitorOnline(ctx context.Context, workspaceID, anonymousID, connID string) error {
	pipe := p.rdb.Pipeline()

	// Set conn key with TTL.
	connKey := visitorConnKey(workspaceID, anonymousID, p.podID, connID)
	pipe.Set(ctx, connKey, "1", visitorConnTTL)
	pipe.Set(ctx, visitorOnlineKey(workspaceID, anonymousID), "1", visitorConnTTL)

	// Add to aggregate set.
	setKey := visitorSetKey(workspaceID)
	pipe.SAdd(ctx, setKey, anonymousID)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis presence SetVisitorOnline: %w", err)
	}
	return nil
}

// SetVisitorOffline removes a visitor connection. Returns true if this was the last connection.
func (p *RedisPresence) SetVisitorOffline(ctx context.Context, workspaceID, anonymousID, connID string) (bool, error) {
	// Delete the conn key.
	connKey := visitorConnKey(workspaceID, anonymousID, p.podID, connID)
	p.rdb.Del(ctx, connKey)

	// Check if any other conn keys remain for this visitor (across all pods).
	pattern := fmt.Sprintf("support:visitors:conn:%s:%s:*", workspaceID, anonymousID)
	keys, err := p.scanKeys(ctx, pattern, 1)
	if err != nil {
		return false, fmt.Errorf("redis presence SetVisitorOffline scan: %w", err)
	}

	if len(keys) == 0 {
		// Last connection — remove from aggregate set.
		setKey := visitorSetKey(workspaceID)
		pipe := p.rdb.Pipeline()
		pipe.SRem(ctx, setKey, anonymousID)
		pipe.Del(ctx, visitorOnlineKey(workspaceID, anonymousID))
		if _, err := pipe.Exec(ctx); err != nil {
			return false, fmt.Errorf("redis presence SetVisitorOffline cleanup: %w", err)
		}
		return true, nil
	}

	return false, nil
}

// IsVisitorOnline returns true if a visitor has at least one active connection.
func (p *RedisPresence) IsVisitorOnline(ctx context.Context, workspaceID, anonymousID string) (bool, error) {
	return p.ensureVisitorOnline(ctx, workspaceID, anonymousID)
}

// GetOnlineVisitors returns the list of online visitor IDs for a workspace.
func (p *RedisPresence) GetOnlineVisitors(ctx context.Context, workspaceID string) ([]string, error) {
	setKey := visitorSetKey(workspaceID)
	members, err := p.rdb.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, fmt.Errorf("redis presence GetOnlineVisitors: %w", err)
	}
	online := make([]string, 0, len(members))
	for _, anonymousID := range members {
		isOnline, err := p.ensureVisitorOnline(ctx, workspaceID, anonymousID)
		if err != nil {
			return nil, err
		}
		if isOnline {
			online = append(online, anonymousID)
		}
	}
	return online, nil
}

// RefreshVisitorOnline extends the TTL on a visitor's conn key.
func (p *RedisPresence) RefreshVisitorOnline(ctx context.Context, workspaceID, anonymousID, connID string) error {
	connKey := visitorConnKey(workspaceID, anonymousID, p.podID, connID)
	pipe := p.rdb.Pipeline()
	pipe.Expire(ctx, connKey, visitorConnTTL)
	pipe.Expire(ctx, visitorOnlineKey(workspaceID, anonymousID), visitorConnTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis presence RefreshVisitorOnline: %w", err)
	}
	return nil
}

// --- Keepalive ---

// RefreshAllForConn refreshes all active keys for a user+conn.
// Called on support:ping from the agent frontend.
func (p *RedisPresence) RefreshAllForConn(ctx context.Context, workspaceID, userID, connID string) error {
	// Refresh viewing keys.
	activeConv, err := p.GetActiveViewing(ctx, workspaceID, userID, connID)
	if err != nil {
		return fmt.Errorf("redis presence RefreshAllForConn GetActiveViewing: %w", err)
	}
	if activeConv != "" {
		if err := p.RefreshViewing(ctx, workspaceID, activeConv, userID, connID); err != nil {
			return err
		}
	}

	// Refresh typing key if this connection owns it.
	if activeConv != "" {
		key := typingKey(workspaceID, activeConv, userID)
		val, err := p.rdb.Get(ctx, key).Result()
		if err == nil && strings.HasPrefix(val, connID+"|") {
			p.rdb.Expire(ctx, key, typingTTL)
		}
	}

	activeDoc, err := p.GetActiveDocViewing(ctx, workspaceID, userID, connID)
	if err != nil {
		return fmt.Errorf("redis presence RefreshAllForConn GetActiveDocViewing: %w", err)
	}
	if activeDoc != "" {
		if err := p.RefreshDocViewing(ctx, workspaceID, activeDoc, userID, connID); err != nil {
			return err
		}
	}

	activeEditing, err := p.GetActiveDocEditing(ctx, workspaceID, userID, connID)
	if err != nil {
		return fmt.Errorf("redis presence RefreshAllForConn GetActiveDocEditing: %w", err)
	}
	if activeEditing.DocumentID != "" {
		if err := p.RefreshDocEditing(ctx, workspaceID, activeEditing.DocumentID, userID, connID); err != nil {
			return err
		}
	}

	return nil
}

// --- Internal helpers ---

// scanKeys scans Redis for keys matching a pattern. Returns up to limit keys.
// Uses SCAN to avoid blocking Redis with KEYS on large datasets.
func (p *RedisPresence) scanKeys(ctx context.Context, pattern string, limit int) ([]string, error) {
	var allKeys []string
	var cursor uint64
	for {
		keys, nextCursor, err := p.rdb.Scan(ctx, cursor, pattern, int64(limit)).Result()
		if err != nil {
			return nil, err
		}
		allKeys = append(allKeys, keys...)
		if len(allKeys) >= limit {
			return allKeys[:limit], nil
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return allKeys, nil
}

func (p *RedisPresence) ensureVisitorOnline(ctx context.Context, workspaceID, anonymousID string) (bool, error) {
	markerKey := visitorOnlineKey(workspaceID, anonymousID)
	exists, err := p.rdb.Exists(ctx, markerKey).Result()
	if err != nil {
		return false, fmt.Errorf("redis presence ensureVisitorOnline marker: %w", err)
	}
	if exists > 0 {
		return true, nil
	}

	keys, err := p.scanKeys(ctx, fmt.Sprintf("support:visitors:conn:%s:%s:*", workspaceID, anonymousID), 1)
	if err != nil {
		return false, fmt.Errorf("redis presence ensureVisitorOnline scan: %w", err)
	}
	if len(keys) == 0 {
		if err := p.rdb.SRem(ctx, visitorSetKey(workspaceID), anonymousID).Err(); err != nil {
			return false, fmt.Errorf("redis presence ensureVisitorOnline cleanup: %w", err)
		}
		return false, nil
	}

	pipe := p.rdb.Pipeline()
	pipe.SAdd(ctx, visitorSetKey(workspaceID), anonymousID)
	pipe.Set(ctx, markerKey, "1", visitorConnTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("redis presence ensureVisitorOnline restore: %w", err)
	}
	return true, nil
}

// Compile-time check that RedisPresence implements PresenceProvider.
var _ PresenceProvider = (*RedisPresence)(nil)
