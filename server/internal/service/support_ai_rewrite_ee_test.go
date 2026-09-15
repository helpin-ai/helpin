//go:build ee

package service

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"testing"
)

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
