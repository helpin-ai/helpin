package websocket

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

func TestWidgetSafeSupportMessageEventDataRemovesLinkSecurity(t *testing.T) {
	data := json.RawMessage(`{"id":"msg-1","metadata":"{\"link_previews\":[{\"url\":\"http://example.com\"}],\"link_security\":[{\"status\":\"malicious\"}]}"}`)

	got := widgetSafeSupportMessageEventData(data)
	if strings.Contains(string(got), "link_security") {
		t.Fatalf("widget event leaked link security: %s", got)
	}
	if !strings.Contains(string(got), "link_previews") {
		t.Fatalf("widget event lost previews: %s", got)
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

func TestSupportAIResponseStartEventsPreserveValidatedContent(t *testing.T) {
	content := "Here is the safe answer ✅ with Unicode café.\n\n- First detail\n- Second detail\n\n" +
		strings.Repeat("A longer explanation keeps the stream moving. ", 4)
	msg := &model.SupportMessage{
		ID:             "msg-stream",
		ConversationID: "conv-stream",
		SenderType:     "ai",
		MessageType:    "reply",
		Content:        content,
		CreatedAt:      time.Date(2026, 8, 11, 19, 0, 0, 0, time.UTC),
	}

	events := SupportAIResponseStartEvents("ws-stream", msg, "ai:agent-1")
	if len(events) < 3 {
		t.Fatalf("expected start plus multiple deltas, got %d events", len(events))
	}
	if events[0].Action != "response_started" {
		t.Fatalf("first action = %q, want response_started", events[0].Action)
	}

	var rebuilt strings.Builder
	for index, event := range events[1:] {
		if event.Action != "response_delta" {
			t.Fatalf("event %d action = %q, want response_delta", index+1, event.Action)
		}
		var payload struct {
			Sequence int    `json:"sequence"`
			Delta    string `json:"delta"`
		}
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			t.Fatalf("unmarshal delta %d: %v", index+1, err)
		}
		if payload.Sequence != index+1 {
			t.Fatalf("sequence = %d, want %d", payload.Sequence, index+1)
		}
		if utf8.RuneCountInString(payload.Delta) > supportAIResponseChunkRunes {
			t.Fatalf("delta %d exceeds %d runes: %q", index+1, supportAIResponseChunkRunes, payload.Delta)
		}
		rebuilt.WriteString(payload.Delta)
	}
	if rebuilt.String() != content {
		t.Fatalf("rebuilt content did not match original\ngot:  %q\nwant: %q", rebuilt.String(), content)
	}
}

func TestSupportAIResponseEventsExcludeInternalMessages(t *testing.T) {
	msg := &model.SupportMessage{
		ID:             "msg-internal",
		ConversationID: "conv-1",
		Content:        "Private tool output",
		IsInternal:     true,
	}
	if events := SupportAIResponseStartEvents("ws-1", msg, "ai:agent-1"); len(events) != 0 {
		t.Fatalf("expected no stream events for internal message, got %#v", events)
	}
	if event := SupportAIResponseCompleteEvent("ws-1", msg, "ai:agent-1"); event.Entity != "" {
		t.Fatalf("expected empty completion event for internal message, got %#v", event)
	}
}

func TestSupportAIProgressEventContainsOnlyCustomerSafeProgress(t *testing.T) {
	event := SupportAIProgressEvent("ws-1", "conv-1", "ai:agent-1", "checking", "Checking the details…")
	if event.Entity != SupportAIResponseStreamEntity || event.ParentID != "conv-1" {
		t.Fatalf("unexpected progress routing: %#v", event)
	}
	var payload map[string]string
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatalf("unmarshal progress payload: %v", err)
	}
	if payload["stage"] != "checking" || payload["label"] != "Checking the details…" {
		t.Fatalf("unexpected progress payload: %#v", payload)
	}
	for _, forbidden := range []string{"reasoning", "tool", "path", "evidence"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("progress payload exposed forbidden field %q", forbidden)
		}
	}
}

func stringPtr(value string) *string {
	return &value
}
