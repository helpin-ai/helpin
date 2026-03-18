package websocket

import (
	"testing"
)

func strPtr(s string) *string { return &s }

func TestShouldReceive_InternalClient(t *testing.T) {
	hub := NewHub()
	client := &Client{UserID: "user-1", WorkspaceID: "ws-1", IsWidget: false}

	// Internal clients receive everything
	event := Event{Entity: "support_conversation_message", ParentID: "conv-1", WorkspaceID: "ws-1"}
	if !hub.shouldReceive(client, event) {
		t.Error("internal client should receive support_conversation_message")
	}

	event2 := Event{Entity: "pm_story", EntityID: "story-1", WorkspaceID: "ws-1"}
	if !hub.shouldReceive(client, event2) {
		t.Error("internal client should receive pm_story")
	}
}

func TestShouldReceive_WidgetClient_MatchingConversation(t *testing.T) {
	hub := NewHub()
	convID := "conv-1"
	client := &Client{UserID: "widget:sess-1", WorkspaceID: "ws-1", IsWidget: true, ConversationID: &convID}

	// Widget client receives messages for its conversation
	event := Event{
		Entity:      "support_conversation_message",
		ParentID:    "conv-1",
		WorkspaceID: "ws-1",
		Data:        []byte(`{"id":"msg-1"}`),
	}
	if !hub.shouldReceive(client, event) {
		t.Error("widget client should receive message for its conversation")
	}

	// Widget client receives conversation events for its conversation
	event2 := Event{Entity: "support_conversation", EntityID: "conv-1", WorkspaceID: "ws-1"}
	if !hub.shouldReceive(client, event2) {
		t.Error("widget client should receive conversation event for its conversation")
	}
}

func TestShouldReceive_WidgetClient_DifferentConversation(t *testing.T) {
	hub := NewHub()
	convID := "conv-1"
	client := &Client{UserID: "widget:sess-1", WorkspaceID: "ws-1", IsWidget: true, ConversationID: &convID}

	// Widget client does NOT receive messages for other conversations
	event := Event{Entity: "support_conversation_message", ParentID: "conv-2", WorkspaceID: "ws-1"}
	if hub.shouldReceive(client, event) {
		t.Error("widget client should NOT receive message for different conversation")
	}

	event2 := Event{Entity: "support_conversation", EntityID: "conv-2", WorkspaceID: "ws-1"}
	if hub.shouldReceive(client, event2) {
		t.Error("widget client should NOT receive conversation event for different conversation")
	}
}

func TestShouldReceive_WidgetClient_NoConversation(t *testing.T) {
	hub := NewHub()
	client := &Client{UserID: "widget:sess-1", WorkspaceID: "ws-1", IsWidget: true, ConversationID: nil}

	event := Event{Entity: "support_conversation_message", ParentID: "conv-1", WorkspaceID: "ws-1"}
	if hub.shouldReceive(client, event) {
		t.Error("widget client with no conversation should NOT receive any messages")
	}
}

func TestShouldReceive_WidgetClient_NonSupportEvents(t *testing.T) {
	hub := NewHub()
	convID := "conv-1"
	client := &Client{UserID: "widget:sess-1", WorkspaceID: "ws-1", IsWidget: true, ConversationID: &convID}

	// Widget client does NOT receive non-support events
	event := Event{Entity: "pm_story", EntityID: "story-1", WorkspaceID: "ws-1"}
	if hub.shouldReceive(client, event) {
		t.Error("widget client should NOT receive pm_story events")
	}

	event2 := Event{Entity: "crm_contact", EntityID: "contact-1", WorkspaceID: "ws-1"}
	if hub.shouldReceive(client, event2) {
		t.Error("widget client should NOT receive crm_contact events")
	}
}

func TestShouldReceive_InternalClient_TypingFromAllExceptSelf(t *testing.T) {
	hub := NewHub()
	client := &Client{UserID: "user-1", WorkspaceID: "ws-1", IsWidget: false}

	customerTyping := Event{
		Action:      "typing_started",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "widget:sess-1",
	}
	if !hub.shouldReceive(client, customerTyping) {
		t.Error("internal client should receive customer typing events")
	}

	otherAgentTyping := Event{
		Action:      "typing_started",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "user-2",
	}
	if !hub.shouldReceive(client, otherAgentTyping) {
		t.Error("internal client should receive other agent typing events")
	}

	ownTyping := Event{
		Action:      "typing_started",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "user-1",
	}
	if hub.shouldReceive(client, ownTyping) {
		t.Error("internal client should NOT receive own typing events")
	}
}

func TestShouldReceive_WidgetClient_TypingOnlyFromAgentsInSameConversation(t *testing.T) {
	hub := NewHub()
	convID := "conv-1"
	client := &Client{UserID: "widget:sess-1", WorkspaceID: "ws-1", IsWidget: true, ConversationID: &convID}

	agentTyping := Event{
		Action:      "typing_started",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "user-2",
	}
	if !hub.shouldReceive(client, agentTyping) {
		t.Error("widget client should receive agent typing for its conversation")
	}

	customerTyping := Event{
		Action:      "typing_started",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "widget:sess-2",
	}
	if hub.shouldReceive(client, customerTyping) {
		t.Error("widget client should NOT receive customer typing events")
	}

	otherConversation := Event{
		Action:      "typing_started",
		Entity:      "support_conversation",
		EntityID:    "conv-2",
		WorkspaceID: "ws-1",
		ActorID:     "user-2",
	}
	if hub.shouldReceive(client, otherConversation) {
		t.Error("widget client should NOT receive typing for a different conversation")
	}
}

func TestShouldReceive_ViewingStarted_InternalClient_SelfFiltered(t *testing.T) {
	hub := NewHub()
	client := &Client{UserID: "user-1", WorkspaceID: "ws-1", IsWidget: false}

	// Self-viewing should be filtered
	selfViewing := Event{
		Action:      "viewing_started",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "user-1",
	}
	if hub.shouldReceive(client, selfViewing) {
		t.Error("internal client should NOT receive own viewing_started events")
	}
}

func TestShouldReceive_ViewingStarted_InternalClient_OtherAgentAllowed(t *testing.T) {
	hub := NewHub()
	client := &Client{UserID: "user-1", WorkspaceID: "ws-1", IsWidget: false}

	// Other agent viewing should be allowed
	otherViewing := Event{
		Action:      "viewing_started",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "user-2",
	}
	if !hub.shouldReceive(client, otherViewing) {
		t.Error("internal client should receive other agent's viewing_started events")
	}
}

func TestShouldReceive_ViewingStopped_InternalClient(t *testing.T) {
	hub := NewHub()
	client := &Client{UserID: "user-1", WorkspaceID: "ws-1", IsWidget: false}

	// Self viewing_stopped should be filtered
	selfStop := Event{
		Action:      "viewing_stopped",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "user-1",
	}
	if hub.shouldReceive(client, selfStop) {
		t.Error("internal client should NOT receive own viewing_stopped events")
	}

	// Other agent viewing_stopped should be received
	otherStop := Event{
		Action:      "viewing_stopped",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "user-2",
	}
	if !hub.shouldReceive(client, otherStop) {
		t.Error("internal client should receive other agent's viewing_stopped events")
	}
}

func TestShouldReceive_ViewingEvents_WidgetClientFiltered(t *testing.T) {
	hub := NewHub()
	convID := "conv-1"
	client := &Client{UserID: "widget:sess-1", WorkspaceID: "ws-1", IsWidget: true, ConversationID: &convID}

	// Widget clients should NOT receive viewing events (they don't need them)
	agentViewing := Event{
		Action:      "viewing_started",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "user-2",
	}
	if hub.shouldReceive(client, agentViewing) {
		t.Error("widget client should NOT receive viewing_started events")
	}

	viewingStopped := Event{
		Action:      "viewing_stopped",
		Entity:      "support_conversation",
		EntityID:    "conv-1",
		WorkspaceID: "ws-1",
		ActorID:     "user-2",
	}
	if hub.shouldReceive(client, viewingStopped) {
		t.Error("widget client should NOT receive viewing_stopped events")
	}
}

func TestShouldReceive_WidgetClient_DoesNotEchoOwnMessages(t *testing.T) {
	hub := NewHub()
	convID := "conv-1"
	client := &Client{UserID: "widget:sess-1", WorkspaceID: "ws-1", IsWidget: true, ConversationID: &convID}

	// Widget should not echo own messages back
	ownMessage := Event{
		Entity:      "support_conversation_message",
		Action:      "created",
		EntityID:    "msg-1",
		WorkspaceID: "ws-1",
		ActorID:     "widget:sess-1",
		ParentType:  "support_conversation",
		ParentID:    "conv-1",
	}
	if hub.shouldReceive(client, ownMessage) {
		t.Error("widget client should NOT echo its own messages")
	}

	// But should receive messages from agents
	agentMessage := Event{
		Entity:      "support_conversation_message",
		Action:      "created",
		EntityID:    "msg-2",
		WorkspaceID: "ws-1",
		ActorID:     "user-1",
		ParentType:  "support_conversation",
		ParentID:    "conv-1",
		Data:        []byte(`{"id":"msg-2"}`),
	}
	if !hub.shouldReceive(client, agentMessage) {
		t.Error("widget client should receive agent messages for its conversation")
	}
}

func TestShouldReceive_VisitorOnlineEvents_OnlyInternalClients(t *testing.T) {
	hub := NewHub()
	agentClient := &Client{UserID: "user-1", WorkspaceID: "ws-1", IsWidget: false}
	convID := "conv-1"
	widgetClient := &Client{UserID: "widget:sess-1", WorkspaceID: "ws-1", IsWidget: true, ConversationID: &convID}

	visitorOnline := Event{
		Action:      "visitor_online",
		Entity:      "support_visitor",
		EntityID:    "anon-123",
		WorkspaceID: "ws-1",
	}

	if !hub.shouldReceive(agentClient, visitorOnline) {
		t.Error("internal client should receive visitor_online events")
	}

	if hub.shouldReceive(widgetClient, visitorOnline) {
		t.Error("widget client should NOT receive visitor_online events")
	}

	visitorOffline := Event{
		Action:      "visitor_offline",
		Entity:      "support_visitor",
		EntityID:    "anon-123",
		WorkspaceID: "ws-1",
	}

	if !hub.shouldReceive(agentClient, visitorOffline) {
		t.Error("internal client should receive visitor_offline events")
	}

	if hub.shouldReceive(widgetClient, visitorOffline) {
		t.Error("widget client should NOT receive visitor_offline events")
	}
}

func TestVisitorOnlineTracking(t *testing.T) {
	hub := NewHub()

	// Initially no visitors online
	if hub.IsVisitorOnline("ws-1", "anon-1") {
		t.Error("visitor should not be online initially")
	}
	visitors := hub.GetOnlineVisitors("ws-1")
	if len(visitors) != 0 {
		t.Errorf("expected 0 visitors, got %d", len(visitors))
	}

	// Set visitor online
	hub.SetVisitorOnline("ws-1", "anon-1")
	if !hub.IsVisitorOnline("ws-1", "anon-1") {
		t.Error("visitor should be online after SetVisitorOnline")
	}
	visitors = hub.GetOnlineVisitors("ws-1")
	if len(visitors) != 1 || visitors[0] != "anon-1" {
		t.Errorf("expected [anon-1], got %v", visitors)
	}

	// Multiple connections from same visitor
	hub.SetVisitorOnline("ws-1", "anon-1")
	if !hub.IsVisitorOnline("ws-1", "anon-1") {
		t.Error("visitor should still be online with 2 connections")
	}

	// Disconnect one — still online
	hub.SetVisitorOffline("ws-1", "anon-1")
	if !hub.IsVisitorOnline("ws-1", "anon-1") {
		t.Error("visitor should still be online with 1 remaining connection")
	}

	// Disconnect last — offline
	hub.SetVisitorOffline("ws-1", "anon-1")
	if hub.IsVisitorOnline("ws-1", "anon-1") {
		t.Error("visitor should be offline after all connections closed")
	}
	visitors = hub.GetOnlineVisitors("ws-1")
	if len(visitors) != 0 {
		t.Errorf("expected 0 visitors after all disconnected, got %d", len(visitors))
	}
}

func TestSetWidgetConversation(t *testing.T) {
	hub := NewHub()
	client := &Client{UserID: "widget:sess-1", WorkspaceID: "ws-1", IsWidget: true, ConversationID: nil}

	hub.Register(client)
	defer hub.Unregister(client)

	if client.ConversationID != nil {
		t.Error("conversation ID should be nil initially")
	}

	hub.SetWidgetConversation("widget:sess-1", "conv-123")

	if client.ConversationID == nil || *client.ConversationID != "conv-123" {
		t.Errorf("expected conversation ID conv-123, got %v", client.ConversationID)
	}
}
