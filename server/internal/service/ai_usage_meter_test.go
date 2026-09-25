package service

import (
	"context"
	"errors"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"testing"
)

func TestCoverageAIUsageFeatureComesFromSupportAIPolicy(t *testing.T) {
	feature, ok := AIUsageFeature(BillingFeatureCoverageGapAnalysis)
	if !ok {
		t.Fatal("coverage usage feature missing")
	}
	if feature.Category != string(aipolicy.CategorySupportAI) {
		t.Fatalf("coverage category = %q, want %q", feature.Category, aipolicy.CategorySupportAI)
	}
	action, ok := aipolicy.DefaultRegistry().Lookup(aipolicy.ActionSupportCoverageAnalyze)
	if !ok || action.FeatureKey != feature.FeatureKey {
		t.Fatalf("coverage action/feature drift: action=%+v feature=%+v", action, feature)
	}
}

func TestAgentRunAIUsageFeatureLabelsAskAgentAsAskChat(t *testing.T) {
	agent := &model.Agent{IsSystem: true, PresetKey: model.AgentPresetAskAgent}
	got := AgentRunAIUsageFeature(agent)
	if got != BillingFeatureAskChat {
		t.Fatalf("AgentRunAIUsageFeature() = %q, want %q", got, BillingFeatureAskChat)
	}
	feature, ok := AIUsageFeature(got)
	if !ok || feature.Label != "Ask Chat" {
		t.Fatalf("Ask Chat feature = %#v, found=%v", feature, ok)
	}
}

func TestCommandBarAnswerIsNotASeparateUsageFeature(t *testing.T) {
	if feature, ok := AIUsageFeature("command_bar_answer"); ok {
		t.Fatalf("command_bar_answer feature = %#v, want command-bar work billed by its agent run", feature)
	}
}

func TestAIUsagePayloadIdempotencyKeyTracksSourceSnapshot(t *testing.T) {
	first := aiUsagePayloadIdempotencyKey([]byte(`{"value":"first"}`), "ws-1", "summary", "contact-1")
	retry := aiUsagePayloadIdempotencyKey([]byte(`{"value":"first"}`), "ws-1", "summary", "contact-1")
	changed := aiUsagePayloadIdempotencyKey([]byte(`{"value":"changed"}`), "ws-1", "summary", "contact-1")

	if first != retry {
		t.Fatalf("same source snapshot produced different keys: %q != %q", first, retry)
	}
	if first == changed {
		t.Fatalf("changed source snapshot reused key %q", first)
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
}

func TestMeteredLLMProviderConsumesUsageFromContext(t *testing.T) {
	consumer := &recordingAIUsageConsumer{}
	meter := NewTokenPricedAIUsageMeter(consumer)
	provider := NewMeteredLLMProvider(scriptedMeteredLLMProvider{
		response: &llm.ChatResponse{
			Content: "ok",
			TokensUsed: llm.TokenUsage{
				InputTokens:       3000,
				InputTokensTotal:  3000,
				CachedInputTokens: 1000,
				CacheReadTokens:   1000,
				CacheWriteTokens:  500,
				OutputTokens:      5000,
				ReasoningTokens:   1000,
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
	if consumer.input.Telemetry.OutputTokens != 5000 {
		t.Fatalf("credits = %d, want 5000 output tokens", consumer.input.Telemetry.OutputTokens)
	}
	if consumer.input.Context.FeatureKey != BillingFeatureDocsArticleGeneration {
		t.Fatalf("feature = %q", consumer.input.Context.FeatureKey)
	}
	if consumer.input.Context.IdempotencyKey != "feature-1" {
		t.Fatalf("idempotency key = %q", consumer.input.Context.IdempotencyKey)
	}
	if consumer.input.Telemetry.CacheWriteTokens != 500 {
		t.Fatalf("cache write metadata = %#v, want 500", consumer.input.Telemetry.CacheWriteTokens)
	}
}

func TestMeteredLLMProviderPreflightsUsageBeforeCallingProvider(t *testing.T) {
	consumer := &recordingAIUsageConsumer{
		preflightErr: errTestPreflightBlocked,
	}
	meter := NewTokenPricedAIUsageMeter(consumer)
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
	if consumer.preflight.WorkspaceID != "ws-1" {
		t.Fatalf("preflight input = %#v, want workspace ws-1 and support reply floor 8", consumer.preflight)
	}
	if consumer.input.Context.WorkspaceID != "" {
		t.Fatalf("usage was consumed despite failed preflight: %#v", consumer.input)
	}
}

func TestMeteredLLMProviderRejectsUsageWithoutContext(t *testing.T) {
	consumer := &recordingAIUsageConsumer{}
	meter := NewTokenPricedAIUsageMeter(consumer)
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
	if consumer.input.Context.WorkspaceID != "" {
		t.Fatalf("expected no usage consumption, got %#v", consumer.input)
	}
}

func TestMeteredLLMProviderAllowsExplicitUsageExemption(t *testing.T) {
	consumer := &recordingAIUsageConsumer{}
	meter := NewTokenPricedAIUsageMeter(consumer)
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
	if consumer.input.Context.WorkspaceID != "" || consumer.preflight.WorkspaceID != "" {
		t.Fatalf("expected no usage checks for explicit exemption, got consume=%#v preflight=%#v", consumer.input, consumer.preflight)
	}
}

func TestPreflightAgentRunAIUsageUsesAgentFeatureFloor(t *testing.T) {
	consumer := &recordingAIUsageConsumer{}
	meter := NewTokenPricedAIUsageMeter(consumer)
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
	if consumer.preflight.WorkspaceID != "ws-1" || consumer.preflight.FeatureKey != BillingFeatureForgeRun {
		t.Fatalf("preflight = %#v, want Forge floor for workspace", consumer.preflight)
	}
	if consumer.input.Context.WorkspaceID != "" {
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

// recordingAIUsageConsumer exercises the production lifecycle boundary.
type recordingAIUsageConsumer struct {
	CommunityAIUsage
	input          CompletionUsage
	preflight      MeteringRequest
	preflightErr   error
	consumeErr     error
	charge         int64
	heartbeatCalls int
	heartbeatLimit int64
	consumeInputs  []CompletionUsage
}

func (c *recordingAIUsageConsumer) Preflight(ctx context.Context, input PreflightRequest) (*MeteringContext, error) {
	c.preflight = input.Metering
	if c.preflightErr != nil {
		return nil, c.preflightErr
	}
	m, err := c.ResolveMeteringContext(input.Metering)
	return &m, err
}
func (c *recordingAIUsageConsumer) Reconcile(_ context.Context, input CompletionUsage) (*UsageResult, error) {
	c.input = input
	c.consumeInputs = append(c.consumeInputs, input)
	return &UsageResult{}, c.consumeErr
}
func (c *recordingAIUsageConsumer) ChargeForTokens(MeteringContext, aiusage.NormalizedTokens) (int64, error) {
	return c.charge, nil
}
func (c *recordingAIUsageConsumer) Heartbeat(_ context.Context, metering MeteringContext) error {
	c.heartbeatCalls++
	if c.heartbeatLimit > 0 && metering.MaxBillableMicrousd > c.heartbeatLimit {
		return model.ErrAIUsageExhausted
	}
	return nil
}
func (p scriptedMeteredLLMProvider) ResolvePricingIdentity(req llm.ChatRequest) (llm.ChatPricingIdentity, error) {
	return testMeterIdentity(req), nil
}
func (p *recordingMeteredLLMProvider) ResolvePricingIdentity(req llm.ChatRequest) (llm.ChatPricingIdentity, error) {
	return testMeterIdentity(req), nil
}
func testMeterIdentity(req llm.ChatRequest) llm.ChatPricingIdentity {
	return llm.ChatPricingIdentity{Provider: "openai", Model: "custom-model", Route: "custom-model", ServiceTier: "standard"}
}

func TestMissingUsageLifecycleCannotMarkTerminalUsageSettled(t *testing.T) {
	run := &model.AgentRun{ID: "run", OutputSummary: []byte(`{}`)}
	projection := &AgentRuntimeProjectionService{usageMeter: &AIUsageMeter{}}
	if _, err := projection.settleTerminalUsage(context.Background(), run, nil, AgentRuntimeEventEnvelope{}, agentRuntimeUsagePayload{InputTokens: 1}); err == nil {
		t.Fatal("missing lifecycle accepted settlement")
	}
	if runtimeUsageAlreadyConsumed(run.OutputSummary) {
		t.Fatal("missing lifecycle advanced settlement marker")
	}
}
