package model

import (
	"testing"
	"time"
)

func TestExplicitTurnSnapshotDeliveryLifecycle(t *testing.T) {
	at := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	var snapshot *CodingSessionStreamSnapshot
	apply := func(kind, id string, start time.Time, offset time.Duration, extra map[string]any) {
		t.Helper()
		data := map[string]any{"turn_id": id, "turn_started_at": start.Format(time.RFC3339Nano), "completion_mode": "explicit"}
		for k, v := range extra {
			data[k] = v
		}
		snapshot = ApplyCodingSessionStreamEvent(snapshot, kind, data, at.Add(offset))
	}
	apply("assistant.message.completed", "one", at, time.Second, map[string]any{"message_id": "preamble", "message_type": "assistant_progress", "content": "Done — here is what changed."})
	if snapshot.TurnState.Phase != "working" {
		t.Fatal("preamble finished turn")
	}
	apply("assistant.message.completed", "one", at, 20*time.Second, map[string]any{"message_id": "answer", "message_type": "assistant_final", "content": "The full answer."})
	completed := *snapshot.TurnState.CompletedAt
	apply("run.paused", "one", at, 40*time.Second, map[string]any{"pause_reason": "awaiting_user_message"})
	if snapshot.TurnState.Phase != "answered" || !snapshot.TurnState.CompletedAt.Equal(completed) {
		t.Fatal("cleanup changed answer completion")
	}
	apply("assistant.message.delta", "one", at, 41*time.Second, map[string]any{"message_id": "preamble", "text": "late text"})
	if len(snapshot.LiveTurnSegments) != 2 {
		t.Fatal("late progress altered accepted transcript")
	}
	encoded, err := EncodeCodingSessionStreamSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = DecodeCodingSessionStreamSnapshot(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.TurnState.AnswerMessageID != "answer" {
		t.Fatal("reload lost final identity")
	}
	apply("run.resumed", "two", at.Add(time.Hour), time.Hour, nil)
	apply("assistant.message.completed", "one", at, time.Hour+time.Second, map[string]any{"message_id": "answer", "message_type": "assistant_final", "content": "The full answer."})
	if snapshot.TurnState.TurnID != "two" || snapshot.TurnState.Phase != "working" {
		t.Fatal("old answer stopped new turn")
	}
	apply("run.paused", "two", at.Add(time.Hour), time.Hour+2*time.Second, map[string]any{"pause_reason": "human_input"})
	if snapshot.TurnState.Phase != "waiting" {
		t.Fatal("lost user input pause")
	}
	apply("run.resumed", "three", at.Add(2*time.Hour), 2*time.Hour, nil)
	apply("run.paused", "three", at.Add(2*time.Hour), 2*time.Hour+time.Second, map[string]any{"pause_reason": "awaiting_user_message"})
	if snapshot.TurnState.Phase != "missing_answer" {
		t.Fatal("missing delivery appeared successful")
	}
	apply("assistant.message.completed", "three", at.Add(2*time.Hour), 2*time.Hour+2*time.Second, map[string]any{"message_id": "recovered", "message_type": "assistant_final", "content": "Recovered full answer."})
	if snapshot.TurnState.Phase != "answered" {
		t.Fatal("late durable recovery failed")
	}
}
