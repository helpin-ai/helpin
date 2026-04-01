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
