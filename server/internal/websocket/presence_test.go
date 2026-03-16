package websocket

import (
	"testing"
)

func TestPresence_SetViewing(t *testing.T) {
	p := NewPresenceState()
	if !p.SetViewing("ws-1", "conv-1", "user-1") {
		t.Error("SetViewing should return true on first call")
	}
	if p.SetViewing("ws-1", "conv-1", "user-1") {
		t.Error("SetViewing should return false for duplicate")
	}
	if !p.SetViewing("ws-1", "conv-1", "user-2") {
		t.Error("SetViewing should return true for different user")
	}
	snap := p.GetSnapshot("ws-1", "conv-1")
	if len(snap.Viewers) != 2 {
		t.Errorf("expected 2 viewers, got %d", len(snap.Viewers))
	}
}

func TestPresence_ClearViewing(t *testing.T) {
	p := NewPresenceState()
	p.SetViewing("ws-1", "conv-1", "user-1")
	p.SetViewing("ws-1", "conv-1", "user-2")
	if !p.ClearViewing("ws-1", "conv-1", "user-1") {
		t.Error("ClearViewing should return true when removing existing user")
	}
	if p.ClearViewing("ws-1", "conv-1", "user-1") {
		t.Error("ClearViewing should return false when user already removed")
	}
	snap := p.GetSnapshot("ws-1", "conv-1")
	if len(snap.Viewers) != 1 {
		t.Errorf("expected 1 viewer, got %d", len(snap.Viewers))
	}
}

func TestPresence_SetTyping(t *testing.T) {
	p := NewPresenceState()
	if !p.SetTyping("ws-1", "conv-1", "user-1", "hello") {
		t.Error("SetTyping should return true on first call")
	}
	if p.SetTyping("ws-1", "conv-1", "user-1", "hello") {
		t.Error("SetTyping should return false for same content")
	}
	if !p.SetTyping("ws-1", "conv-1", "user-1", "hello world") {
		t.Error("SetTyping should return true for updated content")
	}
	snap := p.GetSnapshot("ws-1", "conv-1")
	if snap.Typers["user-1"] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", snap.Typers["user-1"])
	}
}

func TestPresence_ClearTyping(t *testing.T) {
	p := NewPresenceState()
	p.SetTyping("ws-1", "conv-1", "user-1", "draft")
	if !p.ClearTyping("ws-1", "conv-1", "user-1") {
		t.Error("ClearTyping should return true for existing user")
	}
	if p.ClearTyping("ws-1", "conv-1", "user-1") {
		t.Error("ClearTyping should return false when already cleared")
	}
	snap := p.GetSnapshot("ws-1", "conv-1")
	if len(snap.Typers) != 0 {
		t.Errorf("expected 0 typers, got %d", len(snap.Typers))
	}
}

func TestPresence_ClearAllForUser(t *testing.T) {
	p := NewPresenceState()
	p.SetViewing("ws-1", "conv-1", "user-1")
	p.SetViewing("ws-1", "conv-2", "user-1")
	p.SetTyping("ws-1", "conv-1", "user-1", "draft")
	p.SetViewing("ws-1", "conv-1", "user-2")
	p.SetTyping("ws-1", "conv-1", "user-2", "other")

	viewCleared, typeCleared := p.ClearAllForUser("ws-1", "user-1")
	if len(viewCleared) != 2 {
		t.Errorf("expected 2 viewing cleared, got %d", len(viewCleared))
	}
	if len(typeCleared) != 1 {
		t.Errorf("expected 1 typing cleared, got %d", len(typeCleared))
	}
	snap := p.GetSnapshot("ws-1", "conv-1")
	if len(snap.Viewers) != 1 {
		t.Errorf("user-2 should still be viewing, got %d viewers", len(snap.Viewers))
	}
	if snap.Typers["user-2"] != "other" {
		t.Error("user-2 typing should still be present")
	}
}

func TestPresence_GetSnapshot_Empty(t *testing.T) {
	p := NewPresenceState()
	snap := p.GetSnapshot("ws-1", "nonexistent")
	if len(snap.Viewers) != 0 {
		t.Error("expected 0 viewers for nonexistent conversation")
	}
	if len(snap.Typers) != 0 {
		t.Error("expected 0 typers for nonexistent conversation")
	}
}

func TestPresence_MultipleWorkspaces(t *testing.T) {
	p := NewPresenceState()
	p.SetViewing("ws-1", "conv-1", "user-1")
	p.SetViewing("ws-2", "conv-1", "user-1")
	p.ClearAllForUser("ws-1", "user-1")
	if len(p.GetSnapshot("ws-1", "conv-1").Viewers) != 0 {
		t.Error("ws-1 should be cleared")
	}
	if len(p.GetSnapshot("ws-2", "conv-1").Viewers) != 1 {
		t.Error("ws-2 should still have user-1")
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
