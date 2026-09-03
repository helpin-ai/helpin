package handler

import (
	"strings"
	"testing"
)

func TestDecodeMarkConversationReadRequest(t *testing.T) {
	t.Run("accepts an explicit rendered customer message", func(t *testing.T) {
		request, err := decodeMarkConversationReadRequest(strings.NewReader(`{"through_message_id":"message-123"}`))
		if err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.ThroughMessageID != "message-123" {
			t.Fatalf("through_message_id = %q", request.ThroughMessageID)
		}
	})

	t.Run("keeps empty-body compatibility", func(t *testing.T) {
		request, err := decodeMarkConversationReadRequest(strings.NewReader(""))
		if err != nil {
			t.Fatalf("decode empty request: %v", err)
		}
		if request.ThroughMessageID != "" {
			t.Fatalf("through_message_id = %q, want empty", request.ThroughMessageID)
		}
	})

	t.Run("rejects malformed JSON", func(t *testing.T) {
		if _, err := decodeMarkConversationReadRequest(strings.NewReader("{")); err == nil {
			t.Fatal("expected malformed request error")
		}
	})
}
