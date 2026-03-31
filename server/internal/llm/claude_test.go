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
	schema, ok := req.Tools[0].InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("expected object schema, got %#v", req.Tools[0].InputSchema)
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required fields, got %#v", schema["required"])
	}
	if len(required) != 4 {
		t.Fatalf("expected 4 required fields, got %#v", required)
	}
	if schema["additionalProperties"] != false {
		t.Fatalf("expected additionalProperties=false, got %#v", schema["additionalProperties"])
	}
	if req.MaxTokens != 512 {
		t.Fatalf("expected max tokens to be preserved, got %d", req.MaxTokens)
	}
}

func TestBuildClaudeMessageRequestUsesCustomJSONSchemaWhenProvided(t *testing.T) {
	customSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target_mailbox_handle": map[string]any{"type": "string"},
			"reason":                map[string]any{"type": "string"},
		},
		"required":             []string{"target_mailbox_handle", "reason"},
		"additionalProperties": false,
	}

	req := buildClaudeMessageRequest(ChatRequest{
		Messages:   []Message{{Role: "user", Content: "Need refund help"}},
		JSONMode:   true,
		JSONSchema: customSchema,
	})

	if len(req.Tools) != 1 {
		t.Fatalf("expected one tool, got %d", len(req.Tools))
	}
	gotSchema, ok := req.Tools[0].InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("expected map schema, got %#v", req.Tools[0].InputSchema)
	}
	if gotSchema["additionalProperties"] != false {
		t.Fatalf("expected additionalProperties=false, got %#v", gotSchema["additionalProperties"])
	}
	required, ok := gotSchema["required"].([]string)
	if !ok {
		t.Fatalf("expected required fields, got %#v", gotSchema["required"])
	}
	if len(required) != 2 || required[0] != "target_mailbox_handle" || required[1] != "reason" {
		t.Fatalf("unexpected required fields: %#v", required)
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

func TestBuildClaudeMessageContentUsesImageBlocks(t *testing.T) {
	content := buildClaudeMessageContent(Message{
		Role: "user",
		ContentParts: []ContentPart{
			{Type: "text", Text: "Inspect this screenshot"},
			{Type: "image_url", ImageURL: &ImageURLPart{URL: "https://assets.example.com/example.png"}},
		},
	})

	blocks, ok := content.([]workerpkg.ContentBlock)
	if !ok {
		t.Fatalf("expected content blocks, got %#v", content)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 content blocks, got %#v", blocks)
	}
	if blocks[0].Type != "text" {
		t.Fatalf("expected first block text, got %#v", blocks[0])
	}
	if blocks[1].Type != "image" {
		t.Fatalf("expected second block image, got %#v", blocks[1])
	}
}
