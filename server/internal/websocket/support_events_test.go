package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportMessageEventIncludesMessageType(t *testing.T) {
	msg := &model.SupportMessage{
		ID:                "msg-1",
		ConversationID:    "conv-1",
		SenderType:        "agent",
		MessageType:       "system",
		SystemEventType:   model.SupportSystemEventTypeStrPtr(model.SystemEventAIEscalated),
		SenderDisplayName: stringPtr("Helpin AI"),
		Content:           "Let me connect you with a team member who can help further.",
		CreatedAt:         time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC),
	}

	event := SupportMessageEvent("ws-1", msg, "ai:escalation")
	if len(event.Data) == 0 {
		t.Fatal("expected widget payload data")
	}

	var payload model.WidgetMessageReceivedPayload
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatalf("unmarshal widget payload: %v", err)
	}

	if payload.MessageType != "system" {
		t.Fatalf("message_type = %q, want system", payload.MessageType)
	}
	if payload.SenderType != "agent" {
		t.Fatalf("sender_type = %q, want agent", payload.SenderType)
	}
	if payload.SystemEventType == nil || *payload.SystemEventType != model.SystemEventAIEscalated {
		t.Fatalf("system_event_type = %v, want %q", payload.SystemEventType, model.SystemEventAIEscalated)
	}
}

// TestSupportMessageEventOmitsSystemEventTypeForReplies ensures only system
// messages carry the event type on the wire — regular replies do not.
func TestSupportMessageEventOmitsSystemEventTypeForReplies(t *testing.T) {
	msg := &model.SupportMessage{
		ID:             "msg-reply",
		ConversationID: "conv-1",
		SenderType:     "user",
		MessageType:    "reply",
		Content:        "Hi there",
		CreatedAt:      time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC),
	}
	event := SupportMessageEvent("ws-1", msg, "user-1")
	var payload model.WidgetMessageReceivedPayload
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.SystemEventType != nil {
		t.Fatalf("system_event_type = %v, want nil for non-system message", *payload.SystemEventType)
	}
}

func TestSupportMessageEventIncludesEmailProjection(t *testing.T) {
	hasQuoted := false
	msg := &model.SupportMessage{
		ID:                        "msg-email",
		ConversationID:            "conv-1",
		SenderType:                "customer",
		MessageType:               "reply",
		Content:                   "Fallback content",
		EmailVisibleText:          "Visible email reply",
		EmailQuotedText:           "",
		EmailHasQuotedContent:     &hasQuoted,
		EmailProjectionConfidence: "none",
		EmailProjectionVersion:    1,
		CreatedAt:                 time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC),
	}

	event := SupportMessageEvent("ws-1", msg, "customer-1")
	var payload model.WidgetMessageReceivedPayload
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.EmailVisibleText != "Visible email reply" {
		t.Fatalf("email_visible_text = %q", payload.EmailVisibleText)
	}
	if payload.EmailHasQuotedContent == nil || *payload.EmailHasQuotedContent {
		t.Fatalf("email_has_quoted_content = %#v, want explicit false", payload.EmailHasQuotedContent)
	}
	if payload.EmailProjectionVersion != 1 {
		t.Fatalf("email_projection_version = %d, want 1", payload.EmailProjectionVersion)
	}
}

func stringPtr(value string) *string {
	return &value
}
