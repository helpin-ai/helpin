package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCodingSessionRealtimeMessageEventUsesRealtimeSource(t *testing.T) {
	run := &model.AgentRun{ID: "run-1", RuntimeKind: "native_sdk"}
	message := &model.AgentRunMessage{
		ID:         "message-1",
		RunID:      run.ID,
		Role:       "assistant",
		SequenceNo: 14,
		CreatedAt:  time.Date(2026, 8, 21, 19, 52, 14, 0, time.UTC),
	}

	event := codingSessionRealtimeMessageEvent(run, message)

	if event.RuntimeMetadata["source"] != "agent_run_message_realtime" {
		t.Fatalf("expected realtime source marker, got %#v", event.RuntimeMetadata)
	}
	if event.SequenceNo != message.SequenceNo || event.Payload["sequence_no"] != message.SequenceNo {
		t.Fatalf("expected message sequence to be preserved, got %#v", event)
	}
}
