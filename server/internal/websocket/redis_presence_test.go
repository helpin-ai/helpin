package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupRedisPresence(t *testing.T) (*RedisPresence, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewRedisPresence(rdb, "pod-test"), mr
}

func TestRedisPresence_SetViewing_ClearViewing_GetViewers(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	// Set user-1 viewing conv-1.
	changed, err := p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("SetViewing: %v", err)
	}
	if !changed {
		t.Error("SetViewing should report changed on first call")
	}

	// Verify viewer is in the set.
	viewers, err := p.GetViewers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetViewers: %v", err)
	}
	if len(viewers) != 1 || viewers[0] != "user-1" {
		t.Errorf("expected [user-1], got %v", viewers)
	}

	// Add a second user.
	_, err = p.SetViewing(ctx, "ws-1", "conv-1", "user-2", "conn-2")
	if err != nil {
		t.Fatalf("SetViewing user-2: %v", err)
	}
	viewers, err = p.GetViewers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetViewers after user-2: %v", err)
	}
	if len(viewers) != 2 {
		t.Errorf("expected 2 viewers, got %d", len(viewers))
	}

	// Clear user-1 — only conn for user-1, so removed from set.
	cleared, err := p.ClearViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("ClearViewing: %v", err)
	}
	if !cleared {
		t.Error("ClearViewing should report cleared for last conn of user")
	}

	viewers, err = p.GetViewers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetViewers after clear: %v", err)
	}
	if len(viewers) != 1 {
		t.Errorf("expected 1 viewer after clear, got %d", len(viewers))
	}
}

func TestRedisPresence_GetViewers_PrunesExpiredViewingConnections(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	if _, err := p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1"); err != nil {
		t.Fatalf("SetViewing: %v", err)
	}

	mr.FastForward(viewingConnTTL + time.Second)

	viewers, err := p.GetViewers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetViewers: %v", err)
	}
	if len(viewers) != 0 {
		t.Fatalf("expected expired viewer to be omitted, got %v", viewers)
	}

	isMember, err := p.rdb.SIsMember(ctx, viewingSetKey("ws-1", "conv-1"), "user-1").Result()
	if err != nil {
		t.Fatalf("SIsMember: %v", err)
	}
	if isMember {
		t.Fatal("expected expired viewer to be pruned from aggregate set")
	}
}

func TestRedisPresence_SetViewing_MultipleConnsSameUser(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	// Two connections from the same user viewing the same conversation.
	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-2")

	viewers, err := p.GetViewers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetViewers: %v", err)
	}
	// Should still be 1 viewer (user-1) in the aggregate set.
	if len(viewers) != 1 {
		t.Errorf("expected 1 viewer (deduplicated), got %d", len(viewers))
	}

	// Clear one conn — user still has another conn, so NOT removed from set.
	cleared, err := p.ClearViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("ClearViewing conn-1: %v", err)
	}
	if cleared {
		t.Error("ClearViewing should NOT report cleared when another conn remains")
	}

	// Clear the second conn — now removed from set.
	cleared, err = p.ClearViewing(ctx, "ws-1", "conv-1", "user-1", "conn-2")
	if err != nil {
		t.Fatalf("ClearViewing conn-2: %v", err)
	}
	if !cleared {
		t.Error("ClearViewing should report cleared on last conn")
	}

	viewers, err = p.GetViewers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetViewers after full clear: %v", err)
	}
	if len(viewers) != 0 {
		t.Errorf("expected 0 viewers, got %d", len(viewers))
	}
}

func TestRedisPresence_GetActiveViewing(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	// Initially empty.
	conv, err := p.GetActiveViewing(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("GetActiveViewing: %v", err)
	}
	if conv != "" {
		t.Errorf("expected empty, got %q", conv)
	}

	// Set viewing.
	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	conv, err = p.GetActiveViewing(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("GetActiveViewing after set: %v", err)
	}
	if conv != "conv-1" {
		t.Errorf("expected conv-1, got %q", conv)
	}

	// Switch to a different conversation — implicit clear of previous.
	p.SetViewing(ctx, "ws-1", "conv-2", "user-1", "conn-1")
	conv, err = p.GetActiveViewing(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("GetActiveViewing after switch: %v", err)
	}
	if conv != "conv-2" {
		t.Errorf("expected conv-2, got %q", conv)
	}

	// conv-1 should no longer have user-1 as a viewer.
	viewers, err := p.GetViewers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetViewers conv-1: %v", err)
	}
	if len(viewers) != 0 {
		t.Errorf("expected 0 viewers on conv-1 after switch, got %d", len(viewers))
	}
}

func TestRedisPresence_SetTyping_ClearTyping(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	// Set typing.
	err := p.SetTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1", "hello")
	if err != nil {
		t.Fatalf("SetTyping: %v", err)
	}

	typers, err := p.GetTypers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetTypers: %v", err)
	}
	if typers["user-1"] != "hello" {
		t.Errorf("expected 'hello', got %q", typers["user-1"])
	}

	// Clear typing — owned by conn-1.
	cleared, err := p.ClearTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("ClearTyping: %v", err)
	}
	if !cleared {
		t.Error("ClearTyping should return true for owning connection")
	}

	typers, err = p.GetTypers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetTypers after clear: %v", err)
	}
	if len(typers) != 0 {
		t.Errorf("expected 0 typers after clear, got %d", len(typers))
	}
}

func TestRedisPresence_ClearTyping_ConnIDOwnership(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	// Set typing from conn-1.
	p.SetTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1", "hello")

	// Try to clear from conn-2 — should NOT clear (not the owner).
	cleared, err := p.ClearTyping(ctx, "ws-1", "conv-1", "user-1", "conn-2")
	if err != nil {
		t.Fatalf("ClearTyping from non-owner: %v", err)
	}
	if cleared {
		t.Error("ClearTyping should return false for non-owning connection")
	}

	// Key should still exist.
	typers, err := p.GetTypers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetTypers: %v", err)
	}
	if typers["user-1"] != "hello" {
		t.Errorf("expected 'hello' to still be present, got %q", typers["user-1"])
	}

	// Overwrite from conn-2 (last-writer-wins).
	p.SetTyping(ctx, "ws-1", "conv-1", "user-1", "conn-2", "world")

	// Now conn-1 can't clear it.
	cleared, err = p.ClearTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("ClearTyping conn-1 after overwrite: %v", err)
	}
	if cleared {
		t.Error("ClearTyping should return false — conn-2 now owns the key")
	}

	// conn-2 can clear it.
	cleared, err = p.ClearTyping(ctx, "ws-1", "conv-1", "user-1", "conn-2")
	if err != nil {
		t.Fatalf("ClearTyping conn-2: %v", err)
	}
	if !cleared {
		t.Error("ClearTyping should return true for owning conn-2")
	}
}

func TestRedisPresence_SetVisitorOnline_SetVisitorOffline(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	// Initially not online.
	online, err := p.IsVisitorOnline(ctx, "ws-1", "anon-1")
	if err != nil {
		t.Fatalf("IsVisitorOnline: %v", err)
	}
	if online {
		t.Error("visitor should not be online initially")
	}

	// Set online.
	err = p.SetVisitorOnline(ctx, "ws-1", "anon-1", "conn-1")
	if err != nil {
		t.Fatalf("SetVisitorOnline: %v", err)
	}
	online, err = p.IsVisitorOnline(ctx, "ws-1", "anon-1")
	if err != nil {
		t.Fatalf("IsVisitorOnline after set: %v", err)
	}
	if !online {
		t.Error("visitor should be online")
	}

	// GetOnlineVisitors should return anon-1.
	visitors, err := p.GetOnlineVisitors(ctx, "ws-1")
	if err != nil {
		t.Fatalf("GetOnlineVisitors: %v", err)
	}
	if len(visitors) != 1 || visitors[0] != "anon-1" {
		t.Errorf("expected [anon-1], got %v", visitors)
	}

	// Multiple connections from same visitor.
	err = p.SetVisitorOnline(ctx, "ws-1", "anon-1", "conn-2")
	if err != nil {
		t.Fatalf("SetVisitorOnline conn-2: %v", err)
	}

	// Disconnect one — NOT the last.
	lastConn, err := p.SetVisitorOffline(ctx, "ws-1", "anon-1", "conn-1")
	if err != nil {
		t.Fatalf("SetVisitorOffline conn-1: %v", err)
	}
	if lastConn {
		t.Error("should NOT be last conn — conn-2 still active")
	}
	online, err = p.IsVisitorOnline(ctx, "ws-1", "anon-1")
	if err != nil {
		t.Fatalf("IsVisitorOnline after partial disconnect: %v", err)
	}
	if !online {
		t.Error("visitor should still be online with remaining conn-2")
	}

	// Disconnect last.
	lastConn, err = p.SetVisitorOffline(ctx, "ws-1", "anon-1", "conn-2")
	if err != nil {
		t.Fatalf("SetVisitorOffline conn-2: %v", err)
	}
	if !lastConn {
		t.Error("should be last conn")
	}
	online, err = p.IsVisitorOnline(ctx, "ws-1", "anon-1")
	if err != nil {
		t.Fatalf("IsVisitorOnline after full disconnect: %v", err)
	}
	if online {
		t.Error("visitor should be offline after all connections closed")
	}
	visitors, err = p.GetOnlineVisitors(ctx, "ws-1")
	if err != nil {
		t.Fatalf("GetOnlineVisitors after disconnect: %v", err)
	}
	if len(visitors) != 0 {
		t.Errorf("expected 0 visitors, got %d", len(visitors))
	}
}

func TestRedisPresence_AgentOnlineOfflineAndLastSeen(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	firstConn, err := p.SetAgentOnline(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("SetAgentOnline: %v", err)
	}
	if !firstConn {
		t.Fatal("expected first connection to be reported")
	}

	firstConn, err = p.SetAgentOnline(ctx, "ws-1", "user-1", "conn-2")
	if err != nil {
		t.Fatalf("SetAgentOnline second conn: %v", err)
	}
	if firstConn {
		t.Fatal("expected second connection to not be first")
	}

	agents, err := p.GetOnlineAgents(ctx, "ws-1")
	if err != nil {
		t.Fatalf("GetOnlineAgents: %v", err)
	}
	if len(agents) != 1 || agents[0] != "user-1" {
		t.Fatalf("online agents = %v, want [user-1]", agents)
	}

	lastSeen, err := p.GetAgentLastSeen(ctx, "ws-1")
	if err != nil {
		t.Fatalf("GetAgentLastSeen: %v", err)
	}
	firstSeen := lastSeen["user-1"]
	if firstSeen.IsZero() {
		t.Fatal("expected last_seen for user-1")
	}

	if err := p.RefreshAgentOnline(ctx, "ws-1", "user-1", "conn-2"); err != nil {
		t.Fatalf("RefreshAgentOnline: %v", err)
	}
	lastSeen, _ = p.GetAgentLastSeen(ctx, "ws-1")
	if !lastSeen["user-1"].Equal(firstSeen) {
		t.Fatal("expected keepalive refresh to leave last_seen unchanged")
	}

	time.Sleep(10 * time.Millisecond)
	if err := p.TouchAgentActivity(ctx, "ws-1", "user-1", "conn-2"); err != nil {
		t.Fatalf("TouchAgentActivity: %v", err)
	}
	lastSeen, _ = p.GetAgentLastSeen(ctx, "ws-1")
	if lastSeen["user-1"].Before(firstSeen) {
		t.Fatal("expected activity touch to advance last_seen")
	}

	lastConn, err := p.SetAgentOffline(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("SetAgentOffline conn-1: %v", err)
	}
	if lastConn {
		t.Fatal("expected remaining connection after first disconnect")
	}

	lastConn, err = p.SetAgentOffline(ctx, "ws-1", "user-1", "conn-2")
	if err != nil {
		t.Fatalf("SetAgentOffline conn-2: %v", err)
	}
	if !lastConn {
		t.Fatal("expected last connection on second disconnect")
	}

	agents, err = p.GetOnlineAgents(ctx, "ws-1")
	if err != nil {
		t.Fatalf("GetOnlineAgents after disconnect: %v", err)
	}
	if len(agents) != 0 {
		t.Fatalf("expected no online agents after disconnect, got %v", agents)
	}
}

func TestRedisPresence_GetSnapshot(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	// Empty snapshot.
	snap, err := p.GetSnapshot(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetSnapshot empty: %v", err)
	}
	if len(snap.Viewers) != 0 {
		t.Errorf("expected 0 viewers, got %d", len(snap.Viewers))
	}
	if len(snap.Typers) != 0 {
		t.Errorf("expected 0 typers, got %d", len(snap.Typers))
	}

	// Populate.
	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	p.SetViewing(ctx, "ws-1", "conv-1", "user-2", "conn-2")
	p.SetTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1", "draft text")

	snap, err = p.GetSnapshot(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetSnapshot populated: %v", err)
	}
	if len(snap.Viewers) != 2 {
		t.Errorf("expected 2 viewers, got %d", len(snap.Viewers))
	}
	if snap.Typers["user-1"] != "draft text" {
		t.Errorf("expected 'draft text', got %q", snap.Typers["user-1"])
	}
}

func TestRedisPresence_ClearAllForConn(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	// Set viewing + typing.
	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	p.SetTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1", "hello")

	// Also set another user to verify they're not affected.
	p.SetViewing(ctx, "ws-1", "conv-1", "user-2", "conn-2")
	p.SetTyping(ctx, "ws-1", "conv-1", "user-2", "conn-2", "other")

	// Clear all for user-1/conn-1.
	viewCleared, typeCleared, err := p.ClearAllForConn(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("ClearAllForConn: %v", err)
	}
	if len(viewCleared) != 1 || viewCleared[0] != "conv-1" {
		t.Errorf("expected [conv-1] viewing cleared, got %v", viewCleared)
	}
	if len(typeCleared) != 1 || typeCleared[0] != "conv-1" {
		t.Errorf("expected [conv-1] typing cleared, got %v", typeCleared)
	}

	// user-1 should be gone from viewers.
	viewers, err := p.GetViewers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetViewers: %v", err)
	}
	if len(viewers) != 1 {
		t.Errorf("expected 1 viewer (user-2), got %d", len(viewers))
	}

	// user-2 should still be typing.
	typers, err := p.GetTypers(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetTypers: %v", err)
	}
	if typers["user-2"] != "other" {
		t.Errorf("expected 'other' for user-2, got %q", typers["user-2"])
	}
	if _, exists := typers["user-1"]; exists {
		t.Error("user-1 typing should be cleared")
	}
}

func TestRedisPresence_ClearAllForConn_NoActiveViewing(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	// ClearAllForConn with no active viewing should return empty slices.
	viewCleared, typeCleared, err := p.ClearAllForConn(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("ClearAllForConn: %v", err)
	}
	if len(viewCleared) != 0 {
		t.Errorf("expected 0 viewing cleared, got %d", len(viewCleared))
	}
	if len(typeCleared) != 0 {
		t.Errorf("expected 0 typing cleared, got %d", len(typeCleared))
	}
}

func TestRedisPresence_RefreshViewing(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")

	// Refresh should not error.
	err := p.RefreshViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("RefreshViewing: %v", err)
	}

	// Keys should still exist.
	conv, err := p.GetActiveViewing(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("GetActiveViewing: %v", err)
	}
	if conv != "conv-1" {
		t.Errorf("expected conv-1, got %q", conv)
	}
}

func TestRedisPresence_RefreshAllForConn(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	// Set up viewing + typing.
	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	p.SetTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1", "draft")

	// Refresh all.
	err := p.RefreshAllForConn(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("RefreshAllForConn: %v", err)
	}

	// Everything should still be present.
	conv, _ := p.GetActiveViewing(ctx, "ws-1", "user-1", "conn-1")
	if conv != "conv-1" {
		t.Errorf("expected conv-1, got %q", conv)
	}
	typers, _ := p.GetTypers(ctx, "ws-1", "conv-1")
	if typers["user-1"] != "draft" {
		t.Errorf("expected 'draft', got %q", typers["user-1"])
	}
}

func TestRedisPresence_RefreshAllForConn_NoState(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	// Should not error even with no active state.
	err := p.RefreshAllForConn(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("RefreshAllForConn with no state: %v", err)
	}
}

func TestRedisPresence_RefreshVisitorOnline(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	p.SetVisitorOnline(ctx, "ws-1", "anon-1", "conn-1")

	err := p.RefreshVisitorOnline(ctx, "ws-1", "anon-1", "conn-1")
	if err != nil {
		t.Fatalf("RefreshVisitorOnline: %v", err)
	}

	// Visitor should still be online.
	online, _ := p.IsVisitorOnline(ctx, "ws-1", "anon-1")
	if !online {
		t.Error("visitor should still be online after refresh")
	}
}

func TestRedisPresence_RefreshVisitorOnlineRestoresExpiredConnection(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	if err := p.SetVisitorOnline(ctx, "ws-1", "anon-1", "conn-1"); err != nil {
		t.Fatalf("SetVisitorOnline: %v", err)
	}
	mr.FastForward(visitorConnTTL + time.Second)

	if err := p.RefreshVisitorOnline(ctx, "ws-1", "anon-1", "conn-1"); err != nil {
		t.Fatalf("RefreshVisitorOnline: %v", err)
	}

	online, err := p.IsVisitorOnline(ctx, "ws-1", "anon-1")
	if err != nil {
		t.Fatalf("IsVisitorOnline: %v", err)
	}
	if !online {
		t.Fatal("visitor should return online when its live connection refreshes after TTL expiry")
	}
	visitors, err := p.GetOnlineVisitors(ctx, "ws-1")
	if err != nil {
		t.Fatalf("GetOnlineVisitors: %v", err)
	}
	if len(visitors) != 1 || visitors[0] != "anon-1" {
		t.Fatalf("GetOnlineVisitors() = %v, want [anon-1]", visitors)
	}
}

func TestRedisPresence_GetOnlineVisitors_PrunesExpiredVisitorEntries(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	if err := p.SetVisitorOnline(ctx, "ws-1", "anon-1", "conn-1"); err != nil {
		t.Fatalf("SetVisitorOnline: %v", err)
	}

	mr.FastForward(visitorConnTTL + time.Second)

	visitors, err := p.GetOnlineVisitors(ctx, "ws-1")
	if err != nil {
		t.Fatalf("GetOnlineVisitors: %v", err)
	}
	if len(visitors) != 0 {
		t.Fatalf("expected visitor set to self-heal after TTL expiry, got %v", visitors)
	}

	members, err := p.rdb.SMembers(ctx, visitorSetKey("ws-1")).Result()
	if err != nil {
		t.Fatalf("SMembers after prune: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("expected stale aggregate member to be removed, got %v", members)
	}
}

func TestRedisPresence_IsVisitorOnline_CleansUpExpiredMarkerWhenConnGone(t *testing.T) {
	p, mr := setupRedisPresence(t)
	defer mr.Close()
	ctx := context.Background()

	if err := p.SetVisitorOnline(ctx, "ws-1", "anon-1", "conn-1"); err != nil {
		t.Fatalf("SetVisitorOnline: %v", err)
	}

	mr.Del(visitorOnlineKey("ws-1", "anon-1"))

	online, err := p.IsVisitorOnline(ctx, "ws-1", "anon-1")
	if err != nil {
		t.Fatalf("IsVisitorOnline restore path: %v", err)
	}
	if !online {
		t.Fatal("expected active conn to restore visitor online state")
	}

	mr.FastForward(visitorConnTTL + time.Second)

	online, err = p.IsVisitorOnline(ctx, "ws-1", "anon-1")
	if err != nil {
		t.Fatalf("IsVisitorOnline cleanup path: %v", err)
	}
	if online {
		t.Fatal("expected visitor to be offline after conn TTL expiry")
	}

	members, err := p.rdb.SMembers(ctx, visitorSetKey("ws-1")).Result()
	if err != nil {
		t.Fatalf("SMembers after cleanup: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("expected stale aggregate member removed after cleanup, got %v", members)
	}
}
