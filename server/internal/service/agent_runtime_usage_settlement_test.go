package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func terminalUsageFixture(t *testing.T) (*AgentRuntimeProjectionService, *model.AgentRun, *fakeAIUsageStore) {
	t.Helper()
	store := &fakeAIUsageStore{}
	usageService := newTestAIUsageService(t, store)
	metering, err := usageService.ResolveMeteringContext(MeteringRequest{WorkspaceID: "ws-1", TaskNature: "general", FeatureKey: BillingFeatureBuiltInLightAgentRun, Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731", ServiceTier: "standard", IdempotencyKey: "run:test"})
	if err != nil {
		t.Fatal(err)
	}
	metering.ReservationID = "reservation"
	metering.MaxBillableMicrousd = 1_000_000
	run := &model.AgentRun{ID: "host", WorkspaceID: "ws-1", AgentID: "agent", RuntimeKind: "native_sdk", Status: model.AgentRunStatusRunning, ExternalRuntime: stringPointer(agentRuntimeName), ExternalRuntimeID: stringPointer("runtime"), OutputSummary: json.RawMessage(`{}`)}
	if err := storeAgentRunMeteringContext(run, metering); err != nil {
		t.Fatal(err)
	}
	svc := &AgentRuntimeProjectionService{runRepo: &fakeAgentRuntimeProjectionRunRepo{byExternal: map[string]*model.AgentRun{agentRuntimeName + "|runtime": run}}, agentRepo: &fakeAgentRuntimeProjectionAgentRepo{agent: &model.Agent{ID: "agent"}}, usageMeter: &AIUsageMeter{usage: usageService}, now: time.Now}
	return svc, run, store
}

func terminalUsageEvent(kind string, input, output int) AgentRuntimeEventEnvelope {
	return AgentRuntimeEventEnvelope{RunID: "runtime", Type: kind, Data: map[string]any{"usage_semantic": agentruntime.UsageSemanticCumulative, "usage": map[string]any{"input_tokens": input, "output_tokens": output, "total_tokens": input + output}}}
}

func TestTerminalLateUsageSettlesOnlyNewTokens(t *testing.T) {
	svc, run, store := terminalUsageFixture(t)
	for _, event := range []AgentRuntimeEventEnvelope{terminalUsageEvent(agentruntime.EventRunCancelled, 100, 10), terminalUsageEvent(agentruntime.EventUsageCheckpoint, 160, 20), terminalUsageEvent(agentruntime.EventUsageCheckpoint, 160, 20), terminalUsageEvent(agentruntime.EventUsageCheckpoint, 120, 15), terminalUsageEvent(agentruntime.EventRunCancelled, 100, 10)} {
		if err := svc.ApplyEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(store.reconciles) != 2 {
		t.Fatalf("settlements=%d", len(store.reconciles))
	}
	late := store.reconciles[1]
	if late.Entry.InputTokensTotal != 60 || late.Entry.OutputTokens != 10 || !late.AllowLateUsage || late.RunID != run.ID {
		t.Fatalf("late settlement: %+v", late)
	}
	if late.Entry.IdempotencyKey == store.reconciles[0].Entry.IdempotencyKey {
		t.Fatal("late delta reused terminal key")
	}
	if run.Status != model.AgentRunStatusCancelled {
		t.Fatal("late usage changed lifecycle")
	}
	if checkpoint := agentRunUsageCheckpointFromSummary(run.OutputSummary); checkpoint.InputTokens != 160 || checkpoint.OutputTokens != 20 {
		t.Fatalf("watermark regressed: %+v", checkpoint)
	}
}

func TestTerminalWithoutUsageUsesPreviousCheckpoint(t *testing.T) {
	svc, _, store := terminalUsageFixture(t)
	if err := svc.ApplyEvent(context.Background(), terminalUsageEvent(agentruntime.EventUsageCheckpoint, 100, 10)); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{RunID: "runtime", Type: agentruntime.EventRunCancelled}); err != nil {
		t.Fatal(err)
	}
	if len(store.reconciles) != 1 || store.reconcile.Entry.InputTokensTotal != 100 {
		t.Fatal("terminal event dropped known usage")
	}
}

func TestLateUsageAfterZeroUsageCancellation(t *testing.T) {
	svc, _, store := terminalUsageFixture(t)
	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{RunID: "runtime", Type: agentruntime.EventRunCancelled}); err != nil {
		t.Fatal(err)
	}
	if store.releasedID != "reservation" {
		t.Fatal("unused reservation not released")
	}
	if err := svc.ApplyEvent(context.Background(), terminalUsageEvent(agentruntime.EventUsageCheckpoint, 100, 10)); err != nil {
		t.Fatal(err)
	}
	if len(store.reconciles) != 1 || !store.reconcile.AllowLateUsage {
		t.Fatal("late usage after release not settled")
	}
}

func TestTerminalUsageFailureRetriesWithoutMarkingSettled(t *testing.T) {
	svc, run, store := terminalUsageFixture(t)
	store.reconcileErr = errors.New("ledger unavailable")
	event := terminalUsageEvent(agentruntime.EventRunCancelled, 100, 10)
	if err := svc.ApplyEvent(context.Background(), event); err == nil {
		t.Fatal("settlement error swallowed")
	}
	if runtimeUsageAlreadyConsumed(run.OutputSummary) || agentRunUsageCheckpointFromSummary(run.OutputSummary).InputTokens != 0 {
		t.Fatal("failed settlement advanced watermark")
	}
	if run.Status != model.AgentRunStatusCancelled {
		t.Fatal("billing failure blocked cancellation")
	}
	store.reconcileErr = nil
	if err := svc.ApplyEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(store.reconciles) != 2 || store.reconciles[0].Entry.IdempotencyKey != store.reconciles[1].Entry.IdempotencyKey {
		t.Fatal("retry key changed")
	}
}

func TestLegacyLateUsageDoesNotReapplyFeatureFloor(t *testing.T) {
	svc, run, _ := terminalUsageFixture(t)
	consumer := &fakeAgentRuntimeProjectionUsageConsumer{}
	svc.usageMeter = &AIUsageMeter{consumer: consumer}
	for _, event := range []AgentRuntimeEventEnvelope{terminalUsageEvent(agentruntime.EventRunCancelled, 100, 10), terminalUsageEvent(agentruntime.EventUsageCheckpoint, 101, 10), terminalUsageEvent(agentruntime.EventUsageCheckpoint, 100000, 10), terminalUsageEvent(agentruntime.EventUsageCheckpoint, 100000, 10)} {
		if err := svc.ApplyEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(consumer.consumeInputs) != 2 {
		t.Fatalf("legacy settlements=%d", len(consumer.consumeInputs))
	}
	total := consumer.consumeInputs[0].Credits + consumer.consumeInputs[1].Credits
	want := CalculateAIUsageUnits(AIUsageCalculation{FeatureKey: BillingFeatureBuiltInLightAgentRun, InputTokens: 100000, OutputTokens: 10})
	if total != want {
		t.Fatalf("charged=%d want=%d", total, want)
	}
	if run.Status != model.AgentRunStatusCancelled {
		t.Fatal("late usage changed status")
	}
}

func TestLateUsageDoesNotResetStrictChargeCap(t *testing.T) {
	svc, run, store := terminalUsageFixture(t)
	metering, ok := agentRunMeteringContext(run)
	if !ok {
		t.Fatal("missing metering")
	}
	metering.MaxBillableMicrousd = 1
	metering.EnforcementMode = model.AIUsageEnforcementStrict
	if err := storeAgentRunMeteringContext(run, metering); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyEvent(context.Background(), terminalUsageEvent(agentruntime.EventRunCancelled, 1000000, 100)); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyEvent(context.Background(), terminalUsageEvent(agentruntime.EventUsageCheckpoint, 2000000, 200)); err != nil {
		t.Fatal(err)
	}
	if len(store.reconciles) != 2 || store.reconciles[0].ChargedMicrousd != 1 || store.reconciles[1].ChargedMicrousd != 0 {
		t.Fatalf("strict cap reset: %+v", store.reconciles)
	}
}

func TestTerminalUsageConflictDoesNotPersistStaleRun(t *testing.T) {
	svc, _, store := terminalUsageFixture(t)
	store.reconcileErr = repository.ErrAIUsageWatermarkChanged
	err := svc.ApplyEvent(context.Background(), terminalUsageEvent(agentruntime.EventRunCancelled, 100, 10))
	if !errors.Is(err, repository.ErrAIUsageWatermarkChanged) {
		t.Fatalf("conflict not returned: %v", err)
	}
	if repo := svc.runRepo.(*fakeAgentRuntimeProjectionRunRepo); repo.updates != 0 {
		t.Fatal("stale projection persisted over concurrent settlement")
	}
}

type conflictingTerminalProjectionRepo struct {
	*fakeAgentRuntimeProjectionRunRepo
}

func (r *conflictingTerminalProjectionRepo) UpdateRuntimeProjection(context.Context, *model.AgentRun) error {
	return repository.ErrAIUsageWatermarkChanged
}

func TestTerminalUsageConflictAfterSettlementDoesNotNotify(t *testing.T) {
	svc, _, store := terminalUsageFixture(t)
	repo := svc.runRepo.(*fakeAgentRuntimeProjectionRunRepo)
	svc.runRepo = &conflictingTerminalProjectionRepo{fakeAgentRuntimeProjectionRunRepo: repo}
	// Simulate another worker advancing the watermark after our settlement
	// succeeds but before the guarded final projection write.
	err := svc.ApplyEvent(context.Background(), terminalUsageEvent(agentruntime.EventRunCancelled, 100, 10))
	if !errors.Is(err, repository.ErrAIUsageWatermarkChanged) {
		t.Fatalf("final projection conflict not returned: %v", err)
	}
	if len(store.reconciles) != 1 {
		t.Fatal("expected successful settlement before projection conflict")
	}
	if repo.updates != 0 || repo.notifications != 0 {
		t.Fatal("stale projection saved or broadcast")
	}
}
