package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAICompletionServiceGovernanceRequiresAndAuditsCoverageAction(t *testing.T) {
	provider := &scriptedAICompletionProvider{}
	audit := &gatewayFakeAudit{}
	service := newTestAICompletionService(t, provider, &fakeAIUsageStore{}).
		SetGovernance(aipolicy.DefaultRegistry(), audit)
	request := AICompletionRequest{
		WorkspaceID: "ws-1", FeatureKey: BillingFeatureCoverageGapAnalysis,
		IdempotencyKey: "coverage:1", Chat: llm.ChatRequest{MaxTokens: 100},
	}
	if _, err := service.Complete(context.Background(), request); !errors.Is(err, aipolicy.ErrActionRequired) {
		t.Fatalf("missing action error = %v, want ErrActionRequired", err)
	}
	if len(provider.requests) != 0 {
		t.Fatalf("provider requests = %d, want 0", len(provider.requests))
	}

	request.ActionKey = aipolicy.ActionSupportCoverageAnalyze
	if _, err := service.Complete(context.Background(), request); err != nil {
		t.Fatalf("Complete governed coverage: %v", err)
	}
	if len(audit.started) != 1 || len(audit.finished) != 1 {
		t.Fatalf("audit start/finish = %d/%d, want 1/1", len(audit.started), len(audit.finished))
	}
	if audit.started[0].Category != string(aipolicy.CategorySupportAI) ||
		audit.started[0].Origin != "coverage" {
		t.Fatalf("coverage audit = %+v", audit.started[0])
	}
}

type scriptedAICompletionProvider struct {
	requests  []llm.ChatRequest
	responses map[string]*llm.ChatResponse
	errors    map[string]error
}

func (p *scriptedAICompletionProvider) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.requests = append(p.requests, req)
	if err := p.errors[req.Model]; err != nil {
		return nil, err
	}
	if response := p.responses[req.Model]; response != nil {
		copy := *response
		return &copy, nil
	}
	return &llm.ChatResponse{
		Content: "ok", TokensUsed: llm.TokenUsage{InputTokensTotal: 10, CompletionTokensTotal: 5, OutputTokens: 5},
	}, nil
}

func newTestAICompletionService(t *testing.T, provider llm.Provider, store *fakeAIUsageStore) *AICompletionService {
	t.Helper()
	catalog, err := aiusage.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	return NewAICompletionService(provider, NewAIUsageService(catalog, store, nil), DefaultAICompletionRouteRegistry())
}

func validAICompletionRequest() AICompletionRequest {
	return AICompletionRequest{
		WorkspaceID: "ws-1", FeatureKey: BillingFeatureCRMSummary,
		IdempotencyKey: "crm-summary:1", Chat: llm.ChatRequest{Messages: []llm.Message{{Role: "user", Content: "summarize"}}, MaxTokens: 100},
	}
}

func TestAICompletionServiceUsesFeatureDefaultAndReconcilesExactRoute(t *testing.T) {
	store := &fakeAIUsageStore{}
	provider := &scriptedAICompletionProvider{}
	service := newTestAICompletionService(t, provider, store)

	response, err := service.Complete(context.Background(), validAICompletionRequest())
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "ok" || len(provider.requests) != 1 {
		t.Fatalf("response=%#v requests=%d", response, len(provider.requests))
	}
	request := provider.requests[0]
	if request.Provider != "openrouter" || request.Model != "deepseek/deepseek-v4-flash-0731" {
		t.Fatalf("resolved request = %q/%q", request.Provider, request.Model)
	}
	if store.reconcile.Entry.Provider != "openrouter" || store.reconcile.Entry.CanonicalModel != "deepseek-v4-flash-0731" {
		t.Fatalf("ledger route = %#v", store.reconcile.Entry)
	}
}

func TestAICompletionServiceRetriesDeclaredFallbackAfterRetryableProviderFailure(t *testing.T) {
	store := &fakeAIUsageStore{}
	provider := &scriptedAICompletionProvider{errors: map[string]error{
		"deepseek/deepseek-v4-flash-0731": &llm.ProviderError{Provider: "openrouter", StatusCode: 429},
	}}
	service := newTestAICompletionService(t, provider, store)

	response, err := service.Complete(context.Background(), validAICompletionRequest())
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "ok" || len(provider.requests) != 2 {
		t.Fatalf("response=%#v requests=%#v", response, provider.requests)
	}
	if provider.requests[1].Model != "openai/gpt-5.6-luna" {
		t.Fatalf("fallback model = %q", provider.requests[1].Model)
	}
	if store.releasedID == "" {
		t.Fatal("failed primary reservation was not released")
	}
}

func TestAICompletionServiceDoesNotFallbackAfterNonRetryableProviderFailure(t *testing.T) {
	provider := &scriptedAICompletionProvider{errors: map[string]error{
		"deepseek/deepseek-v4-flash-0731": &llm.ProviderError{Provider: "openrouter", StatusCode: 400},
	}}
	service := newTestAICompletionService(t, provider, &fakeAIUsageStore{})

	_, err := service.Complete(context.Background(), validAICompletionRequest())
	if err == nil {
		t.Fatal("Complete() error = nil")
	}
	if len(provider.requests) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(provider.requests))
	}
}

func TestAICompletionServiceRejectsIncompleteOutputAndUsesFallback(t *testing.T) {
	provider := &scriptedAICompletionProvider{responses: map[string]*llm.ChatResponse{
		"deepseek/deepseek-v4-flash-0731": {
			Content: "partial", FinishReason: "length",
			TokensUsed: llm.TokenUsage{InputTokensTotal: 10, CompletionTokensTotal: 100, OutputTokens: 100},
		},
	}}
	service := newTestAICompletionService(t, provider, &fakeAIUsageStore{})
	request := validAICompletionRequest()
	request.RequireComplete = true

	response, err := service.Complete(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "ok" || len(provider.requests) != 2 {
		t.Fatalf("response=%#v requests=%d", response, len(provider.requests))
	}
}

func TestAICompletionServiceRejectsMissingIdentityAndTechnicalChatOverrides(t *testing.T) {
	service := newTestAICompletionService(t, &scriptedAICompletionProvider{}, &fakeAIUsageStore{})
	for _, request := range []AICompletionRequest{
		{},
		{WorkspaceID: "ws", FeatureKey: BillingFeatureCRMSummary},
		{
			WorkspaceID: "ws", FeatureKey: BillingFeatureCRMSummary, IdempotencyKey: "key",
			Chat: llm.ChatRequest{Provider: "openai", Model: "gpt-5.5"},
		},
	} {
		if _, err := service.Complete(context.Background(), request); err == nil {
			t.Fatalf("Complete(%#v) error = nil", request)
		}
	}
}

func TestAICompletionServiceDoesNotFallbackOnUsagePreflightFailure(t *testing.T) {
	store := &fakeAIUsageStore{reserveErr: model.ErrAIUsageExhausted}
	provider := &scriptedAICompletionProvider{}
	service := newTestAICompletionService(t, provider, store)

	_, err := service.Complete(context.Background(), validAICompletionRequest())
	if !errors.Is(err, model.ErrAIUsageExhausted) {
		t.Fatalf("Complete() error = %v", err)
	}
	if len(provider.requests) != 0 {
		t.Fatalf("provider requests = %d, want 0", len(provider.requests))
	}
}

func TestAICompletionServiceAppliesPerRouteOpenRouterProviderSelection(t *testing.T) {
	catalog, err := aiusage.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	provider := &scriptedAICompletionProvider{errors: map[string]error{
		"deepseek/deepseek-v4-flash-0731": &llm.ProviderError{Provider: "openrouter", StatusCode: 429},
	}}
	registry := NewAICompletionRouteRegistry(CRMCompletionRouteConfig{
		Primary: AICompletionRoute{
			Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731", OpenRouterProvider: "together",
		},
		Fallback: AICompletionRoute{
			Provider: "openrouter", Model: "openai/gpt-5.6-luna", OpenRouterProvider: "openai",
		},
	})
	service := NewAICompletionService(provider, NewAIUsageService(catalog, &fakeAIUsageStore{}, nil), registry)
	request := validAICompletionRequest()
	request.Chat.ProviderOptions = json.RawMessage(`{"require_parameters":true}`)

	if _, err := service.Complete(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(provider.requests))
	}
	for index, wantProvider := range []string{"together", "openai"} {
		var options struct {
			Order             []string `json:"order"`
			AllowFallbacks    bool     `json:"allow_fallbacks"`
			RequireParameters bool     `json:"require_parameters"`
		}
		if err := json.Unmarshal(provider.requests[index].ProviderOptions, &options); err != nil {
			t.Fatalf("request %d provider options: %v", index, err)
		}
		if len(options.Order) != 1 || options.Order[0] != wantProvider || options.AllowFallbacks || !options.RequireParameters {
			t.Fatalf("request %d provider options = %#v", index, options)
		}
	}
}
