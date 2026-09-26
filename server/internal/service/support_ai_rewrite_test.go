package service

import (
	"context"
	"errors"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"testing"
	"time"
)

type scriptedSupportRewriteLLM struct {
	response llm.ChatResponse
	lastReq  llm.ChatRequest
}

func TestRewriteSupportDraftUsesLowReasoningAndBoundedDeadline(t *testing.T) {
	provider := &rewriteDeadlineProvider{}
	svc := &SupportAIService{llmProvider: provider}
	_, err := svc.RewriteDraftForSurface(context.Background(), "ws-1", "support reply", BillingFeatureSupportReplyRewrite, model.SupportAIRewriteDraftRequest{
		Content: "hello", Operation: supportRewriteFixGrammar,
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.request.Reasoning == nil || provider.request.Reasoning.Effort != "low" {
		t.Errorf("reasoning = %+v, want low effort", provider.request.Reasoning)
	}
	if provider.remaining <= 0 || provider.remaining > 45*time.Second {
		t.Errorf("request deadline = %v, want at most 45 seconds", provider.remaining)
	}
}

type rewriteDeadlineProvider struct {
	request   llm.ChatRequest
	remaining time.Duration
}

func (p *rewriteDeadlineProvider) ChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.request = req
	if deadline, ok := ctx.Deadline(); ok {
		p.remaining = time.Until(deadline)
	}
	return &llm.ChatResponse{Content: `{"content":"Hello."}`, Provider: req.Provider, Model: req.Model}, nil
}

func (f *scriptedSupportRewriteLLM) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.lastReq = req
	f.response.Provider = req.Provider
	f.response.Model = req.Model
	return &f.response, nil
}

func TestRewriteSupportDraftReportsActualModelAndReturnsContent(t *testing.T) {
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
	policy, ok := DefaultAICompletionRouteRegistry().Policy(BillingFeatureSupportReplyRewrite, "")
	if !ok {
		t.Fatal("support rewrite route policy is missing")
	}
	if resp.Provider != policy.Primary.Provider || resp.Model != policy.Primary.Model {
		t.Fatalf("reported route = %s/%s, want %s/%s", resp.Provider, resp.Model, policy.Primary.Provider, policy.Primary.Model)
	}
	if fakeLLM.lastReq.Provider != policy.Primary.Provider {
		t.Fatalf("chat provider = %q, want %q", fakeLLM.lastReq.Provider, policy.Primary.Provider)
	}
	if fakeLLM.lastReq.Model != policy.Primary.Model {
		t.Fatalf("chat model = %q, want %q", fakeLLM.lastReq.Model, policy.Primary.Model)
	}
	if !fakeLLM.lastReq.JSONMode {
		t.Fatal("expected JSONMode to be enabled")
	}
}

func TestRewriteDraftForSurfaceUsesDraftOnly(t *testing.T) {
	fakeLLM := &scriptedSupportRewriteLLM{
		response: llm.ChatResponse{
			Content: `{"content":"Hello Jane, thanks for reaching out."}`,
		},
	}
	svc := &SupportAIService{
		llmProvider: fakeLLM,
	}

	resp, err := svc.RewriteDraftForSurface(context.Background(), "ws-1", "support reply", BillingFeatureSupportReplyRewrite, model.SupportAIRewriteDraftRequest{
		Content:   "hi jane",
		Operation: supportRewriteFriendly,
	})
	if err != nil {
		t.Fatalf("RewriteDraftForSurface() error = %v", err)
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
