package model

import (
	"strings"
	"time"
)

// Returns false for delayed events from an older turn. Sequence-watermark
// checks at the projector handle duplicates within a turn.
func applyCodingSessionTurnState(snapshot *CodingSessionStreamSnapshot, eventType string, payload map[string]any, timestamp time.Time) bool {
	id := trimmedSnapshotString(payload["turn_id"])
	if id == "" || trimmedSnapshotString(payload["completion_mode"]) != "explicit" {
		return true
	}
	startedAt, err := time.Parse(time.RFC3339Nano, trimmedSnapshotString(payload["turn_started_at"]))
	if err != nil {
		startedAt = timestamp
	}
	current := snapshot.TurnState
	if current == nil || current.TurnID != id {
		if current != nil && !startedAt.After(current.StartedAt) {
			return false
		}
		current = &CodingSessionTurnState{TurnID: id, Phase: "working", StartedAt: startedAt}
		snapshot.TurnState = current
	}
	// Neither late progress nor run cleanup may undo accepted answer delivery.
	if current.Phase == "answered" {
		return true
	}
	switch eventType {
	case "assistant.message.completed":
		if trimmedSnapshotString(payload["message_type"]) == "assistant_final" &&
			strings.TrimSpace(firstNonEmptySnapshotRawString(payload["content"], payload["text"])) != "" {
			current.Phase = "answered"
			current.AnswerMessageID = trimmedSnapshotString(payload["message_id"])
			completedAt, err := time.Parse(time.RFC3339Nano, trimmedSnapshotString(payload["answer_completed_at"]))
			if err != nil {
				completedAt = timestamp
			}
			current.CompletedAt = timePtr(completedAt)
		}
	case "run.paused":
		current.Phase = "waiting"
		if trimmedSnapshotString(payload["pause_reason"]) == "awaiting_user_message" {
			current.Phase = "missing_answer"
		}
		current.CompletedAt = timePtr(timestamp)
	case "run.completed":
		current.Phase = "missing_answer"
		current.CompletedAt = timePtr(timestamp)
	case "run.failed", "run.cancelled":
		current.Phase = strings.TrimPrefix(eventType, "run.")
		current.CompletedAt = timePtr(timestamp)
	}
	return true
}
