package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCompactDockChatMessagePageKeepsOnlyFinalAssistantResponse(t *testing.T) {
	chatID := "chat-1"
	base := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	toolSegments, err := json.Marshal([]model.CodingSessionLiveTurnSegment{
		{
			SegmentID: "progress",
			Kind:      "assistant_message",
			AssistantMessage: &model.CodingSessionLiveAssistantMessage{
				MessageID: "progress",
				Content:   "I will inspect the repository.",
				Status:    "completed",
			},
		},
		{
			SegmentID: "tool-1",
			Kind:      "tool_call",
			ToolCall: &model.CodingSessionLiveToolCall{
				ToolCallID: "tool-1",
				ToolName:   "read_file",
				Status:     "completed",
			},
		},
		{
			SegmentID: "final",
			Kind:      "assistant_message",
			AssistantMessage: &model.CodingSessionLiveAssistantMessage{
				MessageID: "final",
				Content:   "The repository is healthy.",
				Status:    "completed",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal segments: %v", err)
	}
	messages := []model.AgentRunMessage{
		{ID: "user-1", DockChatID: &chatID, Role: "user", Content: "Check it", DockChatSequence: int64Ptr(1), CreatedAt: base},
		{ID: "assistant-progress", DockChatID: &chatID, Role: "assistant", Content: "Starting now", DockChatSequence: int64Ptr(2), CreatedAt: base.Add(time.Second)},
		{ID: "assistant-final", DockChatID: &chatID, Role: "assistant", Content: "stale aggregate", TurnSegments: toolSegments, DockChatSequence: int64Ptr(3), CreatedAt: base.Add(9 * time.Second)},
	}

	compact := compactDockChatMessagePage(messages)
	if len(compact) != 3 {
		t.Fatalf("compact messages = %#v, want user + work summary + final", compact)
	}
	if compact[0].ID != "user-1" || compact[1].MessageType != dockChatWorkSummaryMessageType || compact[2].ID != "assistant-final" {
		t.Fatalf("unexpected compact order: %#v", compact)
	}
	if compact[2].Content != "The repository is healthy." {
		t.Fatalf("final content = %q", compact[2].Content)
	}
	if len(compact[2].TurnSegments) != 0 || len(compact[2].ToolInvocations) != 0 {
		t.Fatal("compact final response leaked historical work payload")
	}
	if compact[1].DockWorkSummary == nil || compact[1].DockWorkSummary.MessageID != "assistant-final" {
		t.Fatalf("work summary = %#v", compact[1].DockWorkSummary)
	}
	if compact[1].DockWorkSummary.DurationMs != 9000 {
		t.Fatalf("duration = %d, want 9000", compact[1].DockWorkSummary.DurationMs)
	}
}

func TestCompactDockChatMessagePageLeavesDirectAnswerWithoutWorkDisclosure(t *testing.T) {
	chatID := "chat-1"
	messages := []model.AgentRunMessage{
		{ID: "user-1", DockChatID: &chatID, Role: "user", Content: "Hello", DockChatSequence: int64Ptr(1)},
		{ID: "assistant-1", DockChatID: &chatID, Role: "assistant", Content: "Hi", DockChatSequence: int64Ptr(2)},
	}

	compact := compactDockChatMessagePage(messages)
	if len(compact) != 2 || compact[1].ID != "assistant-1" || compact[1].DockWorkSummary != nil {
		t.Fatalf("direct answer changed unexpectedly: %#v", compact)
	}
}

func int64Ptr(value int64) *int64 { return &value }
