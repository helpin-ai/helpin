package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type scriptedSupportPreviewLLM struct {
	responses []llm.ChatResponse
	errs      []error
	requests  []llm.ChatRequest
}

func (f *scriptedSupportPreviewLLM) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(f.responses) == 0 {
		return &llm.ChatResponse{}, nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return &resp, nil
}

func TestGenerateResponseRejectsTemplatePlaceholder(t *testing.T) {
	svc := &SupportAIService{
		llmProvider: &scriptedSupportPreviewLLM{
			responses: []llm.ChatResponse{
				{
					Content:    `{"content":"Your answer in markdown","can_answer":true,"source_doc_ids":[],"confidence":0.85}`,
					TokensUsed: llm.TokenUsage{InputTokens: 11, OutputTokens: 7},
				},
			},
		},
	}

	resp, tokensUsed, err := svc.generateResponse(
		context.Background(),
		&model.Agent{Name: "Support"},
		nil,
		nil,
		"",
		model.SupportMessage{SenderType: "customer", Content: "Does it have AI features?"},
		model.AgentModelProviderAnthropic,
		"claude-sonnet-4-6",
	)
	if err != nil {
		t.Fatalf("generateResponse() error = %v", err)
	}
	if resp.CanAnswer {
		t.Fatal("expected template placeholder response to be rejected")
	}
	if resp.Content != "" {
		t.Fatalf("content = %q, want empty", resp.Content)
	}
	if resp.Confidence != 0 {
		t.Fatalf("confidence = %v, want 0", resp.Confidence)
	}
	if tokensUsed != 18 {
		t.Fatalf("tokensUsed = %d, want 18", tokensUsed)
	}
}
