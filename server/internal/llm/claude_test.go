package llm

import (
	"encoding/json"
	"testing"

	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func TestBuildClaudeMessageRequestUsesToolChoiceForJSONMode(t *testing.T) {
	req := buildClaudeMessageRequest(ChatRequest{
		SystemPrompt: "Return JSON only",
		Messages: []Message{
			{Role: "user", Content: "Pricing"},
		},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 512,
		JSONMode:  true,
	})

	if req.ToolChoice == nil {
		t.Fatalf("expected tool choice for JSON mode")
	}
	if req.ToolChoice.Type != "tool" || req.ToolChoice.Name != claudeJSONToolName {
		t.Fatalf("unexpected tool choice: %#v", req.ToolChoice)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("expected one JSON tool, got %d", len(req.Tools))
	}
	if req.Tools[0].Name != claudeJSONToolName {
		t.Fatalf("unexpected tool name: %#v", req.Tools[0])
	}
	if req.MaxTokens != 512 {
		t.Fatalf("expected max tokens to be preserved, got %d", req.MaxTokens)
	}
}

func TestBuildClaudeMessageRequestOmitsToolChoiceForPlainText(t *testing.T) {
	req := buildClaudeMessageRequest(ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if req.ToolChoice != nil {
		t.Fatalf("did not expect tool choice for plain-text mode")
	}
	if len(req.Tools) != 0 {
		t.Fatalf("did not expect tools for plain-text mode, got %#v", req.Tools)
	}
	if req.MaxTokens != 4096 {
		t.Fatalf("expected default max tokens, got %d", req.MaxTokens)
	}
}

func TestExtractClaudeResponseContentUsesToolInputForJSONMode(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"content":        "Pricing details",
		"can_answer":     true,
		"source_doc_ids": []string{"content:abc"},
		"confidence":     0.92,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	resp := &workerpkg.CreateMessageResponse{
		Content: []workerpkg.ContentBlock{
			{Type: "text", Text: "ignored"},
			{Type: "tool_use", Name: claudeJSONToolName, Input: payload},
		},
	}

	got := extractClaudeResponseContent(resp, true)
	if got != string(payload) {
		t.Fatalf("expected tool payload, got %q", got)
	}
}

func TestExtractClaudeResponseContentFallsBackToTextWhenToolInputMissing(t *testing.T) {
	resp := &workerpkg.CreateMessageResponse{
		Content: []workerpkg.ContentBlock{
			{Type: "text", Text: "Hello "},
			{Type: "text", Text: "world"},
		},
	}

	got := extractClaudeResponseContent(resp, true)
	if got != "Hello world" {
		t.Fatalf("expected text fallback, got %q", got)
	}
}
