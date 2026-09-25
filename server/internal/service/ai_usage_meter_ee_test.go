//go:build ee

package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/model"
	"testing"
)

func TestPreflightAgentRunAIUsageStoresTokenPricedReservation(t *testing.T) {
	store := &fakeAIUsageStore{}
	meter := NewTokenPricedAIUsageMeter(newTestAIUsageService(t, store))
	provider, modelID := "openrouter-responses", "openai/gpt-5.6-terra"
	run := &model.AgentRun{ID: "run-token", WorkspaceID: "ws-1", Input: []byte(`{"additional_context":"plan it"}`), OutputSummary: []byte(`{}`)}
	agent := &model.Agent{ID: "agent-1", PresetKey: model.AgentPresetCodeBuilder, Provider: &provider, Model: &modelID}

	if err := PreflightAgentRunAIUsage(context.Background(), meter, run, agent); err != nil {
		t.Fatal(err)
	}
	context, ok := agentRunMeteringContext(run)
	if !ok {
		t.Fatal("expected durable token-priced metering context")
	}
	if context.ReservationID != "reservation" || context.Route.Tier != "large" || context.MaxBillableMicrousd <= 0 {
		t.Fatalf("metering context = %#v", context)
	}
	if store.reservation.ExecutionID != run.ID || store.reservation.IdempotencyKey != "ws-1:agent_run:run-token" {
		t.Fatalf("reservation = %#v", store.reservation)
	}
}

func TestAgentRunUsageCheckpointsChargeOnlyCumulativeDelta(t *testing.T) {
	store := &fakeAIUsageStore{}
	usageService := newTestAIUsageService(t, store)
	metering, err := usageService.ResolveMeteringContext(MeteringRequest{
		WorkspaceID: "ws-1", TaskNature: "general", FeatureKey: BillingFeatureBuiltInLightAgentRun,
		Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731", ServiceTier: "standard",
		IdempotencyKey: "ws-1:agent_run:run-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	metering.ReservationID = "reservation"
	metering.MaxBillableMicrousd = 1_000_000
	metering.EnforcementMode = model.AIUsageEnforcementStrict
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1", OutputSummary: json.RawMessage(`{}`)}
	if err := storeAgentRunMeteringContext(run, metering); err != nil {
		t.Fatal(err)
	}
	meter := &AIUsageMeter{usage: usageService}
	first := agentRuntimeUsagePayload{InputTokens: 100, CachedInputTokens: 20, OutputTokens: 10}
	if err := meter.checkpointAgentRun(context.Background(), run, first); err != nil {
		t.Fatal(err)
	}
	if store.checkpoints != 1 || store.checkpoint.Entry.InputTokensTotal != 100 || store.checkpoint.Entry.OutputTokens != 10 {
		t.Fatalf("first checkpoint = %#v, calls=%d", store.checkpoint.Entry, store.checkpoints)
	}
	if err := meter.checkpointAgentRun(context.Background(), run, first); err != nil {
		t.Fatal(err)
	}
	if store.checkpoints != 1 {
		t.Fatalf("duplicate cumulative checkpoint calls = %d, want 1", store.checkpoints)
	}

	second := agentRuntimeUsagePayload{InputTokens: 160, CachedInputTokens: 30, OutputTokens: 25}
	if err := meter.checkpointAgentRun(context.Background(), run, second); err != nil {
		t.Fatal(err)
	}
	if store.checkpoints != 2 || store.checkpoint.Entry.InputTokensTotal != 60 ||
		store.checkpoint.Entry.CacheReadTokens != 10 || store.checkpoint.Entry.OutputTokens != 15 {
		t.Fatalf("second checkpoint = %#v, calls=%d", store.checkpoint.Entry, store.checkpoints)
	}
	if got := agentRunUsageCheckpointFromSummary(run.OutputSummary); got.Turn != 2 || got.InputTokens != 160 || got.OutputTokens != 25 {
		t.Fatalf("checkpoint summary = %#v", got)
	}

	terminal := agentRuntimeUsagePayload{InputTokens: 180, CachedInputTokens: 32, OutputTokens: 30}
	if err := meter.reconcileAgentRun(context.Background(), run, terminal); err != nil {
		t.Fatal(err)
	}
	if store.reconcile.Entry.InputTokensTotal != 20 || store.reconcile.Entry.CacheReadTokens != 2 || store.reconcile.Entry.OutputTokens != 5 {
		t.Fatalf("terminal reconciliation = %#v", store.reconcile.Entry)
	}
}

func TestAgentRunUsageTerminalAfterCheckpointOnlyReleasesReservation(t *testing.T) {
	store := &fakeAIUsageStore{}
	usageService := newTestAIUsageService(t, store)
	metering, err := usageService.ResolveMeteringContext(MeteringRequest{
		WorkspaceID: "ws-1", TaskNature: "general", FeatureKey: BillingFeatureBuiltInLightAgentRun,
		Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731", ServiceTier: "standard",
		IdempotencyKey: "ws-1:agent_run:run-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	metering.ReservationID = "reservation"
	metering.MaxBillableMicrousd = 1_000_000
	metering.EnforcementMode = model.AIUsageEnforcementStrict
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1", OutputSummary: json.RawMessage(`{}`)}
	if err := storeAgentRunMeteringContext(run, metering); err != nil {
		t.Fatal(err)
	}
	meter := &AIUsageMeter{usage: usageService}
	usage := agentRuntimeUsagePayload{InputTokens: 100, CachedInputTokens: 20, OutputTokens: 10}
	if err := meter.checkpointAgentRun(context.Background(), run, usage); err != nil {
		t.Fatal(err)
	}
	if err := meter.reconcileAgentRun(context.Background(), run, usage); err != nil {
		t.Fatal(err)
	}
	if store.releasedID != "reservation" {
		t.Fatalf("released reservation = %q, want reservation", store.releasedID)
	}
	if store.reconcile.Entry.IdempotencyKey != "" {
		t.Fatalf("unexpected terminal ledger entry: %#v", store.reconcile.Entry)
	}
}

func TestTokenPricedAgentRunReconcilesTerminalCumulativeUsage(t *testing.T) {
	store := &fakeAIUsageStore{mode: model.AIUsageEnforcementExtra}
	meter := NewTokenPricedAIUsageMeter(newTestAIUsageService(t, store))
	provider, modelID := "openrouter", "openai/gpt-5.6-terra"
	run := &model.AgentRun{ID: "run-token", WorkspaceID: "ws-1", OutputSummary: []byte(`{}`)}
	agent := &model.Agent{PresetKey: model.AgentPresetCodeBuilder, Provider: &provider, Model: &modelID}
	if err := PreflightAgentRunAIUsage(context.Background(), meter, run, agent); err != nil {
		t.Fatal(err)
	}
	if err := meter.reconcileAgentRun(context.Background(), run, agentRuntimeUsagePayload{InputTokens: 1000, CachedInputTokens: 100, OutputTokens: 200, ReasoningOutputTokens: 50}); err != nil {
		t.Fatal(err)
	}
	if store.reconcile.ReservationID != "reservation" || store.reconcile.Entry.InputTokensTotal != 1000 || store.reconcile.Entry.ReasoningTokens != 50 {
		t.Fatalf("reconcile = %#v", store.reconcile)
	}
}
