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
	event := Event{Entity: "support_conversation_message", ParentID: "conv-1", WorkspaceID: "ws-1"}
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
