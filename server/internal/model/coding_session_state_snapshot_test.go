package model

import (
	"testing"
	"time"
)

func TestApplyCodingSessionStreamEventMaintainsOrderedLiveTurnSegments(t *testing.T) {
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	var snapshot *CodingSessionStreamSnapshot
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "assistant.message.started", map[string]any{
		"message_id": "assistant-1",
	}, base)
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "assistant.message.delta", map[string]any{
		"message_id": "assistant-1",
		"text":       "Inspecting files.\n",
	}, base.Add(1*time.Second))
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "tool.call.started", map[string]any{
		"parent_message_id": "assistant-1",
		"tool_call_id":      "tool-1",
		"tool_name":         "read_file",
		"args_text":         `{"path":"a.go"}`,
	}, base.Add(2*time.Second))
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "tool.call.completed", map[string]any{
		"parent_message_id": "assistant-1",
		"tool_call_id":      "tool-1",
		"tool_name":         "read_file",
		"output_summary":    "package main",
		"duration_ms":       int64(21),
	}, base.Add(3*time.Second))
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "assistant.message.delta", map[string]any{
		"message_id": "assistant-1",
		"text":       "Patched the call site.",
	}, base.Add(4*time.Second))

	if snapshot == nil || snapshot.LiveAssistantMessage == nil {
		t.Fatalf("expected live assistant snapshot, got %#v", snapshot)
	}
	if got := snapshot.LiveAssistantMessage.Content; got != "Inspecting files.\nPatched the call site." {
		t.Fatalf("unexpected assistant content %q", got)
	}
	if len(snapshot.LiveTurnSegments) != 3 {
		t.Fatalf("expected 3 live turn segments, got %#v", snapshot.LiveTurnSegments)
	}
	if snapshot.LiveTurnSegments[0].Kind != "assistant_message" || snapshot.LiveTurnSegments[0].AssistantMessage == nil || snapshot.LiveTurnSegments[0].AssistantMessage.Content != "Inspecting files.\n" {
		t.Fatalf("unexpected first segment %#v", snapshot.LiveTurnSegments[0])
	}
	if snapshot.LiveTurnSegments[1].Kind != "tool_call" || snapshot.LiveTurnSegments[1].ToolCall == nil || snapshot.LiveTurnSegments[1].ToolCall.ToolCallID != "tool-1" {
		t.Fatalf("unexpected second segment %#v", snapshot.LiveTurnSegments[1])
	}
	if snapshot.LiveTurnSegments[2].Kind != "assistant_message" || snapshot.LiveTurnSegments[2].AssistantMessage == nil || snapshot.LiveTurnSegments[2].AssistantMessage.Content != "Patched the call site." {
		t.Fatalf("unexpected third segment %#v", snapshot.LiveTurnSegments[2])
	}
}

func TestApplyCodingSessionStreamEventStitchesWordTokenDeltas(t *testing.T) {
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	var snapshot *CodingSessionStreamSnapshot
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "assistant.message.started", map[string]any{
		"message_id": "assistant-1",
	}, base)
	for index, token := range []string{"inspect", " the", " Rust", " crate", " first,", " then", " update", " the", " dependency", " and", " build", " against", " the", " new", " API.", " If", " it", " breaks", " I", "'ll", " patch", "."} {
		snapshot = ApplyCodingSessionStreamEvent(snapshot, "assistant.message.delta", map[string]any{
			"message_id": "assistant-1",
			"text":       token,
		}, base.Add(time.Duration(index+1)*time.Second))
	}

	if snapshot == nil || snapshot.LiveAssistantMessage == nil {
		t.Fatalf("expected live assistant snapshot, got %#v", snapshot)
	}
	if got := snapshot.LiveAssistantMessage.Content; got != "inspect the Rust crate first, then update the dependency and build against the new API. If it breaks I'll patch." {
		t.Fatalf("unexpected stitched assistant content %q", got)
	}
	if len(snapshot.LiveTurnSegments) != 1 || snapshot.LiveTurnSegments[0].AssistantMessage == nil {
		t.Fatalf("expected one assistant segment, got %#v", snapshot.LiveTurnSegments)
	}
	if got := snapshot.LiveTurnSegments[0].AssistantMessage.Content; got != snapshot.LiveAssistantMessage.Content {
		t.Fatalf("segment content did not match live assistant content: %q", got)
	}
}

func TestApplyCodingSessionStreamEventRepairsAssistantSegmentOnCompletedText(t *testing.T) {
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	var snapshot *CodingSessionStreamSnapshot
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "assistant.message.started", map[string]any{
		"message_id": "assistant-1",
	}, base)
	for index, token := range []string{"inspect", "the", "Rust", "crate"} {
		snapshot = ApplyCodingSessionStreamEvent(snapshot, "assistant.message.delta", map[string]any{
			"message_id": "assistant-1",
			"text":       token,
		}, base.Add(time.Duration(index+1)*time.Second))
	}
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "assistant.message.completed", map[string]any{
		"message_id": "assistant-1",
		"text":       "Inspect the Rust crate first, then build against the new API.",
	}, base.Add(10*time.Second))

	if snapshot == nil || snapshot.LiveAssistantMessage == nil {
		t.Fatalf("expected live assistant snapshot, got %#v", snapshot)
	}
	const expected = "Inspect the Rust crate first, then build against the new API."
	if got := snapshot.LiveAssistantMessage.Content; got != expected {
		t.Fatalf("completed text did not repair live assistant content %q", got)
	}
	if len(snapshot.LiveTurnSegments) != 1 || snapshot.LiveTurnSegments[0].AssistantMessage == nil {
		t.Fatalf("expected one assistant segment, got %#v", snapshot.LiveTurnSegments)
	}
	if got := snapshot.LiveTurnSegments[0].AssistantMessage.Content; got != expected {
		t.Fatalf("completed text did not repair assistant segment %q", got)
	}
}

func TestApplyCodingSessionStreamEventCompletedTextReplacesLongerDuplicatedLiveText(t *testing.T) {
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	var snapshot *CodingSessionStreamSnapshot
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "assistant.message.delta", map[string]any{
		"message_id": "assistant-duplicate",
		"text":       "I found the Docker build arg issue. I found the Docker build arg issue.",
	}, base)
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "assistant.message.completed", map[string]any{
		"message_id": "assistant-duplicate",
		"text":       "I found the Docker build arg issue.",
	}, base.Add(time.Second))

	if snapshot == nil || snapshot.LiveAssistantMessage == nil {
		t.Fatalf("expected live assistant snapshot, got %#v", snapshot)
	}
	const expected = "I found the Docker build arg issue."
	if got := snapshot.LiveAssistantMessage.Content; got != expected {
		t.Fatalf("completed text did not replace duplicated live content %q", got)
	}
	if len(snapshot.LiveTurnSegments) != 1 || snapshot.LiveTurnSegments[0].AssistantMessage == nil {
		t.Fatalf("expected one assistant segment, got %#v", snapshot.LiveTurnSegments)
	}
	if got := snapshot.LiveTurnSegments[0].AssistantMessage.Content; got != expected {
		t.Fatalf("completed text did not repair duplicated assistant segment %q", got)
	}
}

func TestApplyCodingSessionStreamEventStoresCurrentPlan(t *testing.T) {
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	var snapshot *CodingSessionStreamSnapshot
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "plan.updated", map[string]any{
		"content": `{"note":"Working through the task","plan":[{"step":"Inspect files","status":"completed"},{"step":"Patch the handler","status":"in_progress"},{"step":"Run tests","status":"pending"}]}`,
	}, base)

	if snapshot == nil || snapshot.CurrentPlan == nil {
		t.Fatalf("expected current plan snapshot, got %#v", snapshot)
	}
	if got := snapshot.CurrentPlan.Note; got != "Working through the task" {
		t.Fatalf("unexpected plan note %q", got)
	}
	if len(snapshot.CurrentPlan.Plan) != 3 {
		t.Fatalf("expected 3 plan steps, got %#v", snapshot.CurrentPlan.Plan)
	}
	if got := snapshot.CurrentPlan.Plan[1]; got.Step != "Patch the handler" || got.Status != "in_progress" {
		t.Fatalf("unexpected in-progress step %#v", got)
	}
}

func TestCodingSessionSnapshotRetainsFinalAnswerTypeAfterLaterProgress(t *testing.T) {
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	snapshot := ApplyCodingSessionStreamEvent(nil, "assistant.message.completed", map[string]any{"message_id": "answer", "message_type": "assistant_final", "content": "The full answer."}, at)
	snapshot = ApplyCodingSessionStreamEvent(snapshot, "assistant.message.completed", map[string]any{"message_id": "progress", "content": "Here is the answer:"}, at.Add(time.Second))
	raw, err := EncodeCodingSessionStreamSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecodeCodingSessionStreamSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	if restored == nil || len(restored.LiveTurnSegments) != 2 {
		t.Fatalf("missing restored segments: %+v", restored)
	}
	answer := restored.LiveTurnSegments[0].AssistantMessage
	if answer == nil || answer.MessageID != "answer" || answer.MessageType != "assistant_final" || answer.Content != "The full answer." {
		t.Fatalf("lost final answer metadata: %+v", answer)
	}
}

func TestWorkPlanOriginSurvivesProgressAndFollowup(t *testing.T) {
	at := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	apply := func(s *CodingSessionStreamSnapshot, turn, step, status, event string, when time.Time) *CodingSessionStreamSnapshot {
		if s == nil {
			s = &CodingSessionStreamSnapshot{}
		}
		s.TurnState = &CodingSessionTurnState{TurnID: turn, Phase: "working"}
		return ApplyCodingSessionStreamEvent(s, "plan.updated", map[string]any{"content": `{"plan":[{"step":"` + step + `","status":"` + status + `"}]}`, "_plan_event_id": event}, when)
	}
	s := apply(nil, "turn-1", "Inspect", "pending", "event-1", at)
	s = apply(s, "turn-1", "Inspect", "completed", "event-2", at.Add(time.Second))
	s = apply(s, "turn-2", "Inspect", "completed", "event-3", at.Add(2*time.Second))
	if len(s.WorkPlans) != 1 || s.CurrentPlan.Origin.EventID != "event-1" {
		t.Fatalf("plan moved or duplicated: %#v", s)
	}
	s = apply(s, "turn-2", "Revise", "in_progress", "event-4", at.Add(3*time.Second))
	if len(s.WorkPlans) != 2 || s.WorkPlans[0].Plan[0].Status != "completed" {
		t.Fatalf("lost previous plan: %#v", s.WorkPlans)
	}
	raw, err := EncodeCodingSessionStreamSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecodeCodingSessionStreamSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.WorkPlans) != 2 || restored.CurrentPlan.Origin.EventID != "event-4" {
		t.Fatal("lost plan history on reload")
	}
}

func TestWorkPlanLegacyProgressDoesNotInventOrigin(t *testing.T) {
	s := &CodingSessionStreamSnapshot{CurrentPlan: &CodingSessionRunPlan{Plan: []CodingSessionRunPlanStep{{Step: "Inspect", Status: "pending"}}}}
	s = ApplyCodingSessionStreamEvent(s, "plan.updated", map[string]any{"content": `{"plan":[{"step":"Inspect","status":"completed"}]}`}, time.Now())
	if s.CurrentPlan.Origin != nil || len(s.WorkPlans) != 0 {
		t.Fatal("invented an original position for a legacy plan")
	}
}
