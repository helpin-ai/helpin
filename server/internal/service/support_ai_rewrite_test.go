package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type scriptedSupportRewriteLLM struct {
	response llm.ChatResponse
	lastReq  llm.ChatRequest
}

func TestRewriteSupportDraftUsesLowReasoningAndBoundedDeadline(t *testing.T) {
	provider := &rewriteDeadlineProvider{}
	svc := &SupportAIService{llmProvider: provider}
	_, err := svc.RewriteSupportDraftWithoutConversation(context.Background(), "ws-1", model.SupportAIRewriteDraftRequest{
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

func TestRewriteSupportDraftFallsBackOnInvalidOutput(t *testing.T) {
	for _, tt := range []struct {
		name     string
		response llm.ChatResponse
	}{
		{name: "empty", response: llm.ChatResponse{FinishReason: "stop"}},
		{name: "truncated", response: llm.ChatResponse{Content: `{"content":"Partial`, FinishReason: "length"}},
		{name: "malformed JSON", response: llm.ChatResponse{Content: `{"content":`, FinishReason: "stop"}},
		{name: "missing content", response: llm.ChatResponse{Content: `{}`, FinishReason: "stop"}},
		{name: "blank content", response: llm.ChatResponse{Content: `{"content":" "}`, FinishReason: "stop"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := &scriptedAICompletionProvider{responses: map[string]*llm.ChatResponse{
				"deepseek/deepseek-v4-flash-0731": &tt.response,
				openRouterLunaRoute.Model:         {Content: `{"content":"Hello."}`, Provider: "openrouter", Model: openRouterLunaRoute.Model, FinishReason: "stop"},
			}}
			svc := &SupportAIService{llmProvider: newTestAICompletionService(t, provider, &fakeAIUsageStore{})}
			response, err := svc.RewriteSupportDraftWithoutConversation(context.Background(), "ws-1", model.SupportAIRewriteDraftRequest{
				Content: "hello", Operation: supportRewriteFixGrammar,
			})
			if err != nil {
				t.Fatal(err)
			}
			if response.Content != "Hello." || response.Provider != "openrouter" || response.Model != openRouterLunaRoute.Model {
				t.Errorf("response = %+v, want valid fallback content and actual route", response)
			}
			if len(provider.requests) != 2 {
				t.Errorf("provider requests = %d, want 2", len(provider.requests))
			}
		})
	}
}

func TestRewriteSupportDraftRejectsInvalidOutputFromBothRoutes(t *testing.T) {
	provider := &scriptedAICompletionProvider{responses: map[string]*llm.ChatResponse{
		"deepseek/deepseek-v4-flash-0731": {Content: `{}`, FinishReason: "stop"},
		openRouterLunaRoute.Model:         {Content: `{"content":`, FinishReason: "stop"},
	}}
	svc := &SupportAIService{llmProvider: newTestAICompletionService(t, provider, &fakeAIUsageStore{})}
	response, err := svc.RewriteSupportDraftWithoutConversation(context.Background(), "ws-1", model.SupportAIRewriteDraftRequest{
		Content: "hello", Operation: supportRewriteFixGrammar,
	})
	if err == nil || response != nil {
		t.Fatalf("response = %+v, error = %v, want failure without a replacement draft", response, err)
	}
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

func TestRewriteSupportDraftUsesPolicyAndMetersSmallTierUsage(t *testing.T) {
	provider := &rewriteDeadlineProvider{}
	store := &fakeAIUsageStore{}
	audit := &gatewayFakeAudit{}
	completer := newTestAICompletionService(t, provider, store).SetGovernance(aipolicy.DefaultRegistry(), audit)
	svc := &SupportAIService{llmProvider: completer}
	_, err := svc.RewriteSupportDraftWithoutConversation(context.Background(), "ws-1", model.SupportAIRewriteDraftRequest{
		Content: "hello", Operation: supportRewriteFixGrammar,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(audit.started) != 1 || len(audit.finished) != 1 {
		t.Fatalf("audit start/finish = %d/%d, want 1/1", len(audit.started), len(audit.finished))
	}
	if audit.started[0].ActionKey != "feature.support_reply_rewrite.v1" || audit.started[0].Model != "deepseek/deepseek-v4-flash-0731" {
		t.Errorf("unexpected action audit: %+v", audit.started[0])
	}
	if store.reserveCalls != 1 || store.reconcile.Entry.ModelTier != "small" || store.reconcile.Entry.FeatureKey != BillingFeatureSupportReplyRewrite || store.reconcile.Entry.CanonicalModel != "deepseek-v4-flash-0731" {
		t.Errorf("unexpected metering: reserves=%d, ledger=%+v", store.reserveCalls, store.reconcile.Entry)
	}
}
