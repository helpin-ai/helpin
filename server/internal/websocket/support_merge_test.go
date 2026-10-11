package websocket

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestMergedWidgetMessageUsesExplicitHistoryGrant(t *testing.T) {
	source := "source"
	other := "other"
	h := NewHub()
	msg := &model.SupportMessage{ID: "reply", ConversationID: "primary", MessageType: "reply", SenderType: "user", Content: "Shared reply", WidgetConversationIDs: []string{"source"}}
	event := SupportMessageEvent("ws", msg, "teammate")
	if !h.shouldReceive(&Client{IsWidget: true, ConversationID: &source}, event) {
		t.Fatal("merged source missed granted reply")
	}
	if h.shouldReceive(&Client{IsWidget: true, ConversationID: &other}, event) {
		t.Fatal("unrelated customer received reply")
	}
	msg.IsInternal = true
	event = SupportMessageEvent("ws", msg, "teammate")
	if h.shouldReceive(&Client{IsWidget: true, ConversationID: &source}, event) {
		t.Fatal("note exposed through merge")
	}
}

func TestMergedWidgetPayloadRetainsOriginalConversationScope(t *testing.T) {
	data := json.RawMessage(`{"id":"reply","conversation_id":"primary","content":"Hello"}`)
	got := widgetMergeMessageData(data, "source")
	var payload map[string]any
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["conversation_id"] != "source" {
		t.Fatal("widget redirected to private primary history")
	}
}
