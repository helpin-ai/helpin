package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCalculateAIUsageUnitsUsesSixXOutputAndReasoning(t *testing.T) {
	units := CalculateAIUsageUnits(AIUsageCalculation{
		FeatureKey:        BillingFeatureSupportAIReply,
		InputTokens:       1000,
		OutputTokens:      100,
		ReasoningTokens:   50,
		CachedInputTokens: 200,
	})

	if units != 8 {
		t.Fatalf("usage units = %d, want support reply floor 8", units)
	}
}

func TestCalculateAIUsageUnitsCanExceedFloorFromTokenUsage(t *testing.T) {
	units := CalculateAIUsageUnits(AIUsageCalculation{
		FeatureKey:        BillingFeatureDocsArticleGeneration,
		InputTokens:       3000,
		OutputTokens:      5000,
		ReasoningTokens:   1000,
		CachedInputTokens: 1000,
	})

	if units != 39 {
		t.Fatalf("usage units = %d, want ceil weighted token usage 39", units)
	}
}

func TestAIUsageFloorsDoNotExceedOneHundred(t *testing.T) {
	for _, feature := range AIUsageFeatures() {
		if feature.FloorUnits > 100 {
			t.Fatalf("feature %s floor = %d, want <= 100", feature.FeatureKey, feature.FloorUnits)
		}
	}
}

func TestSetupAIUsageFeaturesAreNotChargeable(t *testing.T) {
	for _, key := range []string{
		BillingFeatureCustomAgentDraft,
		BillingFeatureAutomationSetup,
		BillingFeatureFlowSetup,
		BillingFeatureAgentPromptImprovement,
		BillingFeatureDataImportSetup,
		BillingFeatureCompanyProductContext,
	} {
		feature, ok := AIUsageFeature(key)
		if !ok {
			t.Fatalf("missing setup feature %s", key)
		}
		if feature.Chargeable {
			t.Fatalf("feature %s is chargeable, want free setup action", key)
		}
		if got := CalculateAIUsageUnits(AIUsageCalculation{FeatureKey: key, InputTokens: 10000, OutputTokens: 10000}); got != 0 {
			t.Fatalf("feature %s usage units = %d, want 0", key, got)
		}
	}
}

func TestDockChatTitleAIUsageIsNotChargeable(t *testing.T) {
	feature, ok := AIUsageFeature(BillingFeatureDockChatTitle)
	if !ok {
		t.Fatal("missing dock chat title feature")
	}
	if feature.Chargeable {
		t.Fatal("dock chat title feature is chargeable, want product chrome to be free")
	}
	if got := CalculateAIUsageUnits(AIUsageCalculation{FeatureKey: BillingFeatureDockChatTitle, InputTokens: 100, OutputTokens: 20}); got != 0 {
		t.Fatalf("dock chat title usage units = %d, want 0", got)
	}
}

func TestBillingCreditsForFeatureReturnsAIUsageFloors(t *testing.T) {
	cases := map[string]int{
		BillingFeatureSupportAIReply:        8,
		BillingFeatureDocsGeneration:        15,
		BillingFeaturePlanningRun:           80,
		BillingFeatureCodingRun:             100,
		BillingFeatureDocsArticleGeneration: 20,
	}

	for featureKey, want := range cases {
		if got := BillingCreditsForFeature(featureKey); got != want {
			t.Fatalf("BillingCreditsForFeature(%s) = %d, want %d", featureKey, got, want)
		}
	}
}

func TestAIUsageMeterConsumesCalculatedUsageUnits(t *testing.T) {
	consumer := &recordingAIUsageConsumer{}
	meter := NewAIUsageMeter(consumer)

	_, err := meter.Consume(context.Background(), AIUsageMeterInput{
		WorkspaceID:       "ws-1",
		FeatureKey:        BillingFeatureDocsArticleGeneration,
		IdempotencyKey:    "docs-article-1",
		InputTokens:       3000,
		OutputTokens:      5000,
		ReasoningTokens:   1000,
		CachedInputTokens: 1000,
		Metadata: map[string]interface{}{
			"document_id": "doc-1",
		},
	})
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}

	if consumer.input.WorkspaceID != "ws-1" {
		t.Fatalf("workspace id = %q, want ws-1", consumer.input.WorkspaceID)
	}
	if consumer.input.FeatureKey != BillingFeatureDocsArticleGeneration {
		t.Fatalf("feature key = %q, want %q", consumer.input.FeatureKey, BillingFeatureDocsArticleGeneration)
	}
	if consumer.input.Credits != 39 {
		t.Fatalf("credits = %d, want 39", consumer.input.Credits)
	}
	if consumer.input.IdempotencyKey != "docs-article-1" {
		t.Fatalf("idempotency key = %q, want docs-article-1", consumer.input.IdempotencyKey)
	}
	if consumer.input.Metadata["usage_label"] != "Help article generation" {
		t.Fatalf("usage label metadata = %#v", consumer.input.Metadata["usage_label"])
	}
	if consumer.input.Metadata["weighted_token_formula"] != "input + output*6 + reasoning*6 - cached_input*0.90" {
		t.Fatalf("formula metadata = %#v", consumer.input.Metadata["weighted_token_formula"])
	}
}

func TestMeteredLLMProviderConsumesUsageFromContext(t *testing.T) {
	consumer := &recordingAIUsageConsumer{}
	meter := NewAIUsageMeter(consumer)
	provider := NewMeteredLLMProvider(scriptedMeteredLLMProvider{
		response: &llm.ChatResponse{
			Content: "ok",
			TokensUsed: llm.TokenUsage{
				InputTokens:  3000,
				OutputTokens: 5000,
			},
		},
	}, meter)

	ctx := WithAIUsageMetering(context.Background(), AIUsageMeteringContext{
		WorkspaceID:    "ws-1",
		FeatureKey:     BillingFeatureDocsArticleGeneration,
		IdempotencyKey: "feature-1",
		Metadata: map[string]interface{}{
			"entity_id": "doc-1",
		},
	})
	resp, err := provider.ChatCompletion(ctx, llm.ChatRequest{})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("content = %q, want ok", resp.Content)
	}
	if consumer.input.Credits != 33 {
		t.Fatalf("credits = %d, want 33", consumer.input.Credits)
	}
	if consumer.input.FeatureKey != BillingFeatureDocsArticleGeneration {
		t.Fatalf("feature = %q", consumer.input.FeatureKey)
	}
	if consumer.input.IdempotencyKey != "feature-1" {
		t.Fatalf("idempotency key = %q", consumer.input.IdempotencyKey)
	}
}

func TestMeteredLLMProviderPreflightsUsageBeforeCallingProvider(t *testing.T) {
	consumer := &recordingAIUsageConsumer{
		preflightErr: errTestPreflightBlocked,
	}
	meter := NewAIUsageMeter(consumer)
	base := &recordingMeteredLLMProvider{
		response: &llm.ChatResponse{Content: "ok"},
	}
	provider := NewMeteredLLMProvider(base, meter)

	ctx := WithAIUsageMetering(context.Background(), AIUsageMeteringContext{
		WorkspaceID:    "ws-1",
		FeatureKey:     BillingFeatureSupportAIReply,
		IdempotencyKey: "feature-1",
	})
	resp, err := provider.ChatCompletion(ctx, llm.ChatRequest{})
	if err == nil || err != errTestPreflightBlocked {
		t.Fatalf("ChatCompletion() error = %v, want preflight error", err)
	}
	if resp != nil {
		t.Fatalf("response = %#v, want nil", resp)
	}
	if base.called {
		t.Fatal("base provider was called before preflight passed")
	}
	if consumer.preflight.WorkspaceID != "ws-1" || consumer.preflight.Credits != 8 {
		t.Fatalf("preflight input = %#v, want workspace ws-1 and support reply floor 8", consumer.preflight)
	}
	if consumer.input.WorkspaceID != "" {
		t.Fatalf("usage was consumed despite failed preflight: %#v", consumer.input)
	}
}

func TestMeteredLLMProviderRejectsUsageWithoutContext(t *testing.T) {
	consumer := &recordingAIUsageConsumer{}
	meter := NewAIUsageMeter(consumer)
	provider := NewMeteredLLMProvider(scriptedMeteredLLMProvider{
		response: &llm.ChatResponse{
			Content: "ok",
			TokensUsed: llm.TokenUsage{
				InputTokens:  3000,
				OutputTokens: 5000,
			},
		},
	}, meter)

	_, err := provider.ChatCompletion(context.Background(), llm.ChatRequest{})
	if !errors.Is(err, ErrAIUsageMeteringRequired) {
		t.Fatalf("ChatCompletion() error = %v, want ErrAIUsageMeteringRequired", err)
	}
	if consumer.input.WorkspaceID != "" {
		t.Fatalf("expected no usage consumption, got %#v", consumer.input)
	}
}

func TestMeteredLLMProviderAllowsExplicitUsageExemption(t *testing.T) {
	consumer := &recordingAIUsageConsumer{}
	meter := NewAIUsageMeter(consumer)
	provider := NewMeteredLLMProvider(scriptedMeteredLLMProvider{
		response: &llm.ChatResponse{
			Content: "ok",
			TokensUsed: llm.TokenUsage{
				InputTokens:  3000,
				OutputTokens: 5000,
			},
		},
	}, meter)

	resp, err := provider.ChatCompletion(WithAIUsageMeteringExempt(context.Background()), llm.ChatRequest{})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp == nil || resp.Content != "ok" {
		t.Fatalf("response = %#v, want ok", resp)
	}
	if consumer.input.WorkspaceID != "" || consumer.preflight.WorkspaceID != "" {
		t.Fatalf("expected no usage checks for explicit exemption, got consume=%#v preflight=%#v", consumer.input, consumer.preflight)
	}
}

func TestPreflightAgentRunAIUsageUsesAgentFeatureFloor(t *testing.T) {
	consumer := &recordingAIUsageConsumer{}
	meter := NewAIUsageMeter(consumer)
	run := &model.AgentRun{
		ID:          "run-1",
		WorkspaceID: "ws-1",
		AgentID:     "agent-1",
	}
	agent := &model.Agent{
		ID:        "agent-1",
		PresetKey: model.AgentPresetCodeBuilder,
		IsSystem:  true,
	}

	if err := PreflightAgentRunAIUsage(context.Background(), meter, run, agent); err != nil {
		t.Fatalf("PreflightAgentRunAIUsage() error = %v", err)
	}
	if consumer.preflight.WorkspaceID != "ws-1" || consumer.preflight.FeatureKey != BillingFeatureForgeRun || consumer.preflight.Credits != 100 {
		t.Fatalf("preflight = %#v, want Forge floor for workspace", consumer.preflight)
	}
	if consumer.input.WorkspaceID != "" {
		t.Fatalf("usage was consumed during preflight: %#v", consumer.input)
	}
}

type scriptedMeteredLLMProvider struct {
	response *llm.ChatResponse
	err      error
}

func (p scriptedMeteredLLMProvider) ChatCompletion(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.response, nil
}

type recordingMeteredLLMProvider struct {
	response *llm.ChatResponse
	err      error
	called   bool
}

func (p *recordingMeteredLLMProvider) ChatCompletion(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	p.called = true
	if p.err != nil {
		return nil, p.err
	}
	return p.response, nil
}

var errTestPreflightBlocked = &testPreflightError{}

type testPreflightError struct{}

func (e *testPreflightError) Error() string { return "preflight blocked" }

type recordingAIUsageConsumer struct {
	input        BillingCreditConsumption
	preflight    BillingCreditPreflight
	preflightErr error
}

func (c *recordingAIUsageConsumer) PreflightCredits(_ context.Context, input BillingCreditPreflight) error {
	c.preflight = input
	return c.preflightErr
}

func (c *recordingAIUsageConsumer) ConsumeCredits(_ context.Context, input BillingCreditConsumption) (*BillingSummary, error) {
	c.input = input
	return &BillingSummary{WorkspaceID: input.WorkspaceID}, nil
}
