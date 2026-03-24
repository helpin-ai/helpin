package worker

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestTrimArtifactContextPreservesCanonicalArtifacts(t *testing.T) {
	preservedContent := strings.Repeat("canonical planning artifact\n", 400)
	trimmedContent := strings.Repeat("ephemeral preview noise\n", 500)

	ctx := &ArtifactContext{
		Entries: []ArtifactContextEntry{
			{
				Label:        "Structured product spec draft artifact",
				Source:       "product_spec_draft",
				Format:       "json",
				Content:      preservedContent,
				PreserveFull: true,
			},
			{
				Label:   "Current preview for prd_draft",
				Source:  "run_preview",
				Format:  "markdown",
				Content: trimmedContent,
			},
		},
	}

	trimmed := TrimArtifactContext(ctx)
	if trimmed == nil || len(trimmed.Entries) != 2 {
		t.Fatalf("unexpected trimmed artifact context %#v", trimmed)
	}
	if trimmed.Entries[0].Content != preservedContent {
		t.Fatal("expected canonical planning artifact to remain untrimmed")
	}
	if trimmed.Entries[1].Content == trimmedContent {
		t.Fatal("expected non-canonical artifact entry to be trimmed")
	}
	if !strings.Contains(trimmed.Entries[1].Content, "[trimmed for prompt efficiency; full artifact remains persisted]") {
		t.Fatalf("expected trim marker, got %q", trimmed.Entries[1].Content)
	}
}

func TestBuildTranscriptSummaryCheckpointCondensesOlderMessages(t *testing.T) {
	messages := make([]model.AgentRunMessage, 0, 14)
	for i := 1; i <= 14; i++ {
		role := "assistant"
		messageType := "assistant_turn"
		content := "Assistant update"
		if i%2 == 1 {
			role = "user"
			messageType = "prompt"
			content = "User request"
		}
		messages = append(messages, model.AgentRunMessage{
			SequenceNo:  i,
			Role:        role,
			MessageType: messageType,
			Content:     content + " " + strings.Repeat("x", i),
		})
	}

	checkpoint := BuildTranscriptSummaryCheckpoint(messages)
	if checkpoint == nil {
		t.Fatal("expected transcript summary checkpoint")
	}
	if checkpoint.CoveredThroughSequenceNo != 6 {
		t.Fatalf("covered_through_sequence_no = %d, want 6", checkpoint.CoveredThroughSequenceNo)
	}
	if checkpoint.SourceMessageCount != 6 {
		t.Fatalf("source_message_count = %d, want 6", checkpoint.SourceMessageCount)
	}
	for _, marker := range []string{
		"Earlier transcript covered 6 messages through sequence 6.",
		"Initial request:",
		"User follow-ups:",
		"Assistant progress:",
	} {
		if !strings.Contains(checkpoint.Summary, marker) {
			t.Fatalf("expected summary to contain %q\n%s", marker, checkpoint.Summary)
		}
	}
}

func TestBuildExecutionHistoryStartsWithUserSummaryWhenCheckpointPresent(t *testing.T) {
	messages := []model.AgentRunMessage{
		{SequenceNo: 1, Role: "user", MessageType: "prompt", Content: "Initial request"},
		{SequenceNo: 2, Role: "assistant", MessageType: "assistant_turn", Content: "Working on it"},
		{SequenceNo: 7, Role: "user", MessageType: "prompt", Content: "Latest reply"},
	}

	history := BuildExecutionHistory(messages, &TranscriptSummaryCheckpoint{
		CoveredThroughSequenceNo: 2,
		SourceMessageCount:       2,
		Summary:                  "Earlier transcript covered 2 messages through sequence 2.",
	})
	if len(history) != 2 {
		t.Fatalf("expected summary plus latest message, got %#v", history)
	}
	if history[0].Role != "user" {
		t.Fatalf("expected synthetic summary to be a user message, got %#v", history[0])
	}
	if !strings.Contains(history[0].Content, "Resume context from earlier turns:") {
		t.Fatalf("unexpected summary content %#v", history[0])
	}
}

func TestExecutionMessageFromRunMessageDerivesContentFromBlocksWhenContentEmpty(t *testing.T) {
	message := model.AgentRunMessage{
		Role:    "tool",
		Content: "",
		ContentBlocks: json.RawMessage(
			`[{"type":"tool_result","tool_call_id":"call-1","tool_name":"read_file","output":"package main"}]`,
		),
	}

	execMessage := executionMessageFromRunMessage(message)
	if execMessage.Content != "package main" {
		t.Fatalf("expected content derived from tool result block, got %#v", execMessage)
	}
}

func TestExecutionMessageFromRunMessageNormalizesEmptyToolCallInput(t *testing.T) {
	message := model.AgentRunMessage{
		Role: "assistant",
		ContentBlocks: json.RawMessage(
			`[{"type":"tool_call","tool_call_id":"call-1","tool_name":"read_file","input":""}]`,
		),
	}

	execMessage := executionMessageFromRunMessage(message)
	if len(execMessage.Blocks) != 1 {
		t.Fatalf("expected tool call block, got %#v", execMessage.Blocks)
	}
	if got := string(execMessage.Blocks[0].Input); got != "{}" {
		t.Fatalf("expected normalized tool args, got %q", got)
	}
}
