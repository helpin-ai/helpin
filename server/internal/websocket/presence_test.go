package websocket

import (
	"context"
	"testing"
)

func TestPresence_SetViewing(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	changed, err := p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("SetViewing: %v", err)
	}
	if !changed {
		t.Error("SetViewing should return true on first call")
	}

	// Same user, same conn — still reports changed=true (idempotent broadcast is harmless)
	changed, _ = p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	// No assertion on changed for duplicate — implementation may vary.

	changed, _ = p.SetViewing(ctx, "ws-1", "conv-1", "user-2", "conn-2")
	if !changed {
		t.Error("SetViewing should return true for different user")
	}
	snap, err := p.GetSnapshot(ctx, "ws-1", "conv-1")
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	if len(snap.Viewers) != 2 {
		t.Errorf("expected 2 viewers, got %d", len(snap.Viewers))
	}
}

func TestPresence_ClearViewing(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	p.SetViewing(ctx, "ws-1", "conv-1", "user-2", "conn-2")

	cleared, err := p.ClearViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("ClearViewing: %v", err)
	}
	if !cleared {
		t.Error("ClearViewing should return true when removing last conn of user")
	}

	cleared, _ = p.ClearViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if cleared {
		t.Error("ClearViewing should return false when user already removed")
	}

	snap, _ := p.GetSnapshot(ctx, "ws-1", "conv-1")
	if len(snap.Viewers) != 1 {
		t.Errorf("expected 1 viewer, got %d", len(snap.Viewers))
	}
}

func TestPresence_ClearViewing_MultipleConns(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	// Two connections from the same user viewing the same conversation.
	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-2")

	// Clear one conn — user still has another conn, NOT removed from aggregate set.
	cleared, _ := p.ClearViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if cleared {
		t.Error("ClearViewing should NOT report cleared when another conn remains")
	}

	viewers, _ := p.GetViewers(ctx, "ws-1", "conv-1")
	if len(viewers) != 1 {
		t.Errorf("expected 1 viewer (user-1 still has conn-2), got %d", len(viewers))
	}

	// Clear the second conn — now removed from set.
	cleared, _ = p.ClearViewing(ctx, "ws-1", "conv-1", "user-1", "conn-2")
	if !cleared {
		t.Error("ClearViewing should report cleared on last conn")
	}

	viewers, _ = p.GetViewers(ctx, "ws-1", "conv-1")
	if len(viewers) != 0 {
		t.Errorf("expected 0 viewers, got %d", len(viewers))
	}
}

func TestPresence_SetTyping(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	err := p.SetTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1", "hello")
	if err != nil {
		t.Fatalf("SetTyping: %v", err)
	}

	err = p.SetTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1", "hello world")
	if err != nil {
		t.Fatalf("SetTyping update: %v", err)
	}

	snap, _ := p.GetSnapshot(ctx, "ws-1", "conv-1")
	if snap.Typers["user-1"] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", snap.Typers["user-1"])
	}
}

func TestPresence_ClearTyping(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	p.SetTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1", "draft")

	cleared, err := p.ClearTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("ClearTyping: %v", err)
	}
	if !cleared {
		t.Error("ClearTyping should return true for owning connection")
	}

	cleared, _ = p.ClearTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if cleared {
		t.Error("ClearTyping should return false when already cleared")
	}

	snap, _ := p.GetSnapshot(ctx, "ws-1", "conv-1")
	if len(snap.Typers) != 0 {
		t.Errorf("expected 0 typers, got %d", len(snap.Typers))
	}
}

func TestPresence_ClearTyping_ConnIDOwnership(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	// Set typing from conn-1.
	p.SetTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1", "hello")

	// Try to clear from conn-2 — should NOT clear (not the owner).
	cleared, _ := p.ClearTyping(ctx, "ws-1", "conv-1", "user-1", "conn-2")
	if cleared {
		t.Error("ClearTyping should return false for non-owning connection")
	}

	typers, _ := p.GetTypers(ctx, "ws-1", "conv-1")
	if typers["user-1"] != "hello" {
		t.Errorf("expected 'hello' to still be present, got %q", typers["user-1"])
	}

	// Overwrite from conn-2 (last-writer-wins).
	p.SetTyping(ctx, "ws-1", "conv-1", "user-1", "conn-2", "world")

	// Now conn-1 can't clear it.
	cleared, _ = p.ClearTyping(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if cleared {
		t.Error("ClearTyping should return false — conn-2 now owns the key")
	}

	// conn-2 can clear it.
	cleared, _ = p.ClearTyping(ctx, "ws-1", "conv-1", "user-1", "conn-2")
	if !cleared {
		t.Error("ClearTyping should return true for owning conn-2")
	}
}

func TestPresence_ClearAllForConn(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	p.SetViewing(ctx, "ws-1", "conv-2", "user-1", "conn-1")
	// Note: SetViewing to conv-2 with same connID should implicitly clear conv-1.
	// So user-1 is only viewing conv-2 now.
	p.SetTyping(ctx, "ws-1", "conv-2", "user-1", "conn-1", "draft")
	p.SetViewing(ctx, "ws-1", "conv-1", "user-2", "conn-2")
	p.SetTyping(ctx, "ws-1", "conv-1", "user-2", "conn-2", "other")

	viewCleared, typeCleared, err := p.ClearAllForConn(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("ClearAllForConn: %v", err)
	}
	if len(viewCleared) != 1 || viewCleared[0] != "conv-2" {
		t.Errorf("expected [conv-2] viewing cleared, got %v", viewCleared)
	}
	if len(typeCleared) != 1 || typeCleared[0] != "conv-2" {
		t.Errorf("expected [conv-2] typing cleared, got %v", typeCleared)
	}

	snap, _ := p.GetSnapshot(ctx, "ws-1", "conv-1")
	if len(snap.Viewers) != 1 {
		t.Errorf("user-2 should still be viewing, got %d viewers", len(snap.Viewers))
	}
	if snap.Typers["user-2"] != "other" {
		t.Error("user-2 typing should still be present")
	}
}

func TestPresence_GetSnapshot_Empty(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	snap, err := p.GetSnapshot(ctx, "ws-1", "nonexistent")
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	if len(snap.Viewers) != 0 {
		t.Error("expected 0 viewers for nonexistent conversation")
	}
	if len(snap.Typers) != 0 {
		t.Error("expected 0 typers for nonexistent conversation")
	}
}

func TestPresence_MultipleWorkspaces(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	p.SetViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	p.SetViewing(ctx, "ws-2", "conv-1", "user-1", "conn-2")

	p.ClearAllForConn(ctx, "ws-1", "user-1", "conn-1")

	snap1, _ := p.GetSnapshot(ctx, "ws-1", "conv-1")
	if len(snap1.Viewers) != 0 {
		t.Error("ws-1 should be cleared")
	}
	snap2, _ := p.GetSnapshot(ctx, "ws-2", "conv-1")
	if len(snap2.Viewers) != 1 {
		t.Error("ws-2 should still have user-1")
	}
}

func TestPresence_GetActiveViewing(t *testing.T) {
	p := NewPresenceState()
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
	conv, _ = p.GetActiveViewing(ctx, "ws-1", "user-1", "conn-1")
	if conv != "conv-1" {
		t.Errorf("expected conv-1, got %q", conv)
	}

	// Switch to a different conversation — implicit clear of previous.
	p.SetViewing(ctx, "ws-1", "conv-2", "user-1", "conn-1")
	conv, _ = p.GetActiveViewing(ctx, "ws-1", "user-1", "conn-1")
	if conv != "conv-2" {
		t.Errorf("expected conv-2, got %q", conv)
	}

	// conv-1 should no longer have user-1 as a viewer.
	viewers, _ := p.GetViewers(ctx, "ws-1", "conv-1")
	if len(viewers) != 0 {
		t.Errorf("expected 0 viewers on conv-1 after switch, got %d", len(viewers))
	}
}

func TestPresence_AgentOnlineOfflineAndLastSeen(t *testing.T) {
	p := NewPresenceState()
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
		t.Fatal("expected second connection to not be reported as first")
	}

	onlineAgents, err := p.GetOnlineAgents(ctx, "ws-1")
	if err != nil {
		t.Fatalf("GetOnlineAgents: %v", err)
	}
	if len(onlineAgents) != 1 || onlineAgents[0] != "user-1" {
		t.Fatalf("online agents = %v, want [user-1]", onlineAgents)
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
	if lastSeen["user-1"].Before(firstSeen) {
		t.Fatal("expected refreshed last_seen to be >= original")
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

	onlineAgents, err = p.GetOnlineAgents(ctx, "ws-1")
	if err != nil {
		t.Fatalf("GetOnlineAgents after disconnect: %v", err)
	}
	if len(onlineAgents) != 0 {
		t.Fatalf("expected no online agents after disconnect, got %v", onlineAgents)
	}
}

func TestPresence_VisitorOnlineOffline(t *testing.T) {
	p := NewPresenceState()
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
	p.SetVisitorOnline(ctx, "ws-1", "anon-1", "conn-1")
	online, _ = p.IsVisitorOnline(ctx, "ws-1", "anon-1")
	if !online {
		t.Error("visitor should be online")
	}

	visitors, _ := p.GetOnlineVisitors(ctx, "ws-1")
	if len(visitors) != 1 || visitors[0] != "anon-1" {
		t.Errorf("expected [anon-1], got %v", visitors)
	}

	// Multiple connections.
	p.SetVisitorOnline(ctx, "ws-1", "anon-1", "conn-2")

	// Disconnect one — NOT the last.
	lastConn, _ := p.SetVisitorOffline(ctx, "ws-1", "anon-1", "conn-1")
	if lastConn {
		t.Error("should NOT be last conn — conn-2 still active")
	}
	online, _ = p.IsVisitorOnline(ctx, "ws-1", "anon-1")
	if !online {
		t.Error("visitor should still be online with remaining conn-2")
	}

	// Disconnect last.
	lastConn, _ = p.SetVisitorOffline(ctx, "ws-1", "anon-1", "conn-2")
	if !lastConn {
		t.Error("should be last conn")
	}
	online, _ = p.IsVisitorOnline(ctx, "ws-1", "anon-1")
	if online {
		t.Error("visitor should be offline after all connections closed")
	}
}

func TestPresence_RefreshAllForConn_NoOp(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	// RefreshAllForConn should be a no-op (no error).
	err := p.RefreshAllForConn(ctx, "ws-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("RefreshAllForConn: %v", err)
	}
}

func TestPresence_RefreshViewing_NoOp(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	err := p.RefreshViewing(ctx, "ws-1", "conv-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("RefreshViewing: %v", err)
	}
}

func TestPresence_RefreshVisitorOnline_NoOp(t *testing.T) {
	p := NewPresenceState()
	ctx := context.Background()

	err := p.RefreshVisitorOnline(ctx, "ws-1", "anon-1", "conn-1")
	if err != nil {
		t.Fatalf("RefreshVisitorOnline: %v", err)
	}
}

func TestShouldReceive_ViewingEvents(t *testing.T) {
	hub := NewHub()
	agent := &Client{UserID: "user-1", WorkspaceID: "ws-1", IsWidget: false}
	convID := "conv-1"
	widget := &Client{UserID: "widget:sess-1", WorkspaceID: "ws-1", IsWidget: true, ConversationID: &convID}

	otherViewing := Event{Action: "viewing_started", Entity: "support_conversation", EntityID: "conv-1", WorkspaceID: "ws-1", ActorID: "user-2"}
	if !hub.shouldReceive(agent, otherViewing) {
		t.Error("agent should receive other agent's viewing events")
	}

	selfViewing := Event{Action: "viewing_started", Entity: "support_conversation", EntityID: "conv-1", WorkspaceID: "ws-1", ActorID: "user-1"}
	if hub.shouldReceive(agent, selfViewing) {
		t.Error("agent should NOT receive own viewing events")
	}

	if hub.shouldReceive(widget, otherViewing) {
		t.Error("widget should NOT receive viewing events")
	}

	stoppedEvent := Event{Action: "viewing_stopped", Entity: "support_conversation", EntityID: "conv-1", WorkspaceID: "ws-1", ActorID: "user-2"}
	if !hub.shouldReceive(agent, stoppedEvent) {
		t.Error("agent should receive other agent's viewing_stopped events")
	}
}
