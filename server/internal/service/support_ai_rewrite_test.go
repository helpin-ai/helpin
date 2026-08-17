package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type scriptedSupportRewriteLLM struct {
	response llm.ChatResponse
	lastReq  llm.ChatRequest
}

func (f *scriptedSupportRewriteLLM) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.lastReq = req
	return &f.response, nil
}

func TestRewriteSupportDraftUsesSmallTierModelAndReturnsContent(t *testing.T) {
	fakeLLM := &scriptedSupportRewriteLLM{
		response: llm.ChatResponse{
			Content: `{"content":"Thanks for reaching out. We have updated your billing details and everything is set now."}`,
		},
	}
	svc := &SupportAIService{
		llmProvider: fakeLLM,
	}

	resp, err := svc.RewriteSupportDraft(context.Background(), "ws-1", "conv-1", model.SupportAIRewriteDraftRequest{
		Content:   "updated billing, all set",
		Operation: supportRewriteExpand,
	})
	if err != nil {
		t.Fatalf("RewriteSupportDraft() error = %v", err)
	}
	if resp.Content == "" {
		t.Fatal("expected rewritten content")
	}
	if resp.Provider != supportRewriteProvider {
		t.Fatalf("provider = %q, want %q", resp.Provider, supportRewriteProvider)
	}
	if resp.Model != supportRewriteModel {
		t.Fatalf("model = %q, want %q", resp.Model, supportRewriteModel)
	}
	if fakeLLM.lastReq.Provider != supportRewriteProvider {
		t.Fatalf("chat provider = %q, want %q", fakeLLM.lastReq.Provider, supportRewriteProvider)
	}
	if fakeLLM.lastReq.Model != supportRewriteModel {
		t.Fatalf("chat model = %q, want %q", fakeLLM.lastReq.Model, supportRewriteModel)
	}
	if !fakeLLM.lastReq.JSONMode {
		t.Fatal("expected JSONMode to be enabled")
	}
}

func TestRewriteSupportDraftWithoutConversationUsesDraftOnly(t *testing.T) {
	fakeLLM := &scriptedSupportRewriteLLM{
		response: llm.ChatResponse{
			Content: `{"content":"Hello Jane, thanks for reaching out."}`,
		},
	}
	svc := &SupportAIService{
		llmProvider: fakeLLM,
	}

	resp, err := svc.RewriteSupportDraftWithoutConversation(context.Background(), "ws-1", model.SupportAIRewriteDraftRequest{
		Content:   "hi jane",
		Operation: supportRewriteFriendly,
	})
	if err != nil {
		t.Fatalf("RewriteSupportDraftWithoutConversation() error = %v", err)
	}
	if resp.Content != "Hello Jane, thanks for reaching out." {
		t.Fatalf("content = %q", resp.Content)
	}
	if len(fakeLLM.lastReq.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(fakeLLM.lastReq.Messages))
	}
}

func TestRewriteSupportDraftRejectsUnsupportedOperation(t *testing.T) {
	svc := &SupportAIService{
		llmProvider: &scriptedSupportRewriteLLM{},
	}

	_, err := svc.RewriteSupportDraft(context.Background(), "ws-1", "conv-1", model.SupportAIRewriteDraftRequest{
		Content:   "hello",
		Operation: "predict",
	})
	if err == nil {
		t.Fatal("expected error for unsupported operation")
	}
	if !errors.Is(err, ErrSupportRewriteInvalidInput) {
		t.Fatalf("expected ErrSupportRewriteInvalidInput, got %v", err)
	}
}
