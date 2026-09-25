package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type recordingCommunityUsage struct {
	entries []model.AIExecutionUsage
	fail    bool
}

func (s *recordingCommunityUsage) RecordExecutionUsage(_ context.Context, entry model.AIExecutionUsage, _ model.JSONBlob) error {
	if s.fail {
		return errors.New("store unavailable")
	}
	s.entries = append(s.entries, entry)
	return nil
}

func TestCommunityUsageUnpricedModelLifecycle(t *testing.T) {
	store := &recordingCommunityUsage{}
	usage := NewCommunityAIUsage(store)
	ctx := context.Background()
	accepted, err := usage.Preflight(ctx, PreflightRequest{Metering: MeteringRequest{
		WorkspaceID: "ws", Provider: "openai", Model: "unpriced-custom-model", IdempotencyKey: "request",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.ReservationID != "" || accepted.PricingVersion != "" || accepted.MaxBillableMicrousd != 0 {
		t.Fatalf("financial context: %+v", accepted)
	}
	for _, action := range []func() error{
		func() error { return usage.Heartbeat(ctx, *accepted) },
		func() error { return usage.SuspendReservation(ctx, *accepted) },
		func() error { return usage.Heartbeat(ctx, *accepted) },
		func() error { return usage.Fail(ctx, "") },
		func() error { return usage.Release(ctx, "", "complete") },
	} {
		if err := action(); err != nil {
			t.Fatal(err)
		}
	}
	result, err := usage.Reconcile(ctx, CompletionUsage{Context: *accepted, Telemetry: aiusage.TokenTelemetry{
		InputTokensTotal: 100, CacheReadTokens: 20, CompletionTokensTotal: 50, ReasoningTokens: 10, CompletionIncludesReasoning: true,
	}, PaidTools: []aiusage.PaidToolUsage{{Key: "unpriced-tool", Count: 1}}, MeasurementStatus: "actual"})
	if err != nil {
		t.Fatal(err)
	}
	if *result != (UsageResult{}) || len(store.entries) != 1 {
		t.Fatalf("result=%+v entries=%+v", result, store.entries)
	}
	entry := store.entries[0]
	if entry.InputTokens != 100 || entry.OutputTokens != 40 || entry.ReasoningTokens != 10 || entry.CacheReadTokens != 20 {
		t.Fatalf("lost raw usage: %+v", entry)
	}
}

func TestCommunityRunCheckpointSurvivesPauseAndResume(t *testing.T) {
	store := &recordingCommunityUsage{}
	usage := NewCommunityAIUsage(store)
	meter := NewTokenPricedAIUsageMeter(usage)
	agent := &model.Agent{Provider: communityString("openai"), Model: communityString("custom")}
	run := &model.AgentRun{ID: "run", WorkspaceID: "ws", Input: json.RawMessage(`{}`)}
	if err := PreflightAgentRunAIUsage(context.Background(), meter, run, agent); err != nil {
		t.Fatal(err)
	}
	if err := meter.checkpointAgentRun(context.Background(), run, agentRuntimeUsagePayload{InputTokens: 100, OutputTokens: 20}); err != nil {
		t.Fatal(err)
	}
	// Reconstruct the adapter and run, as on a different worker after a pause.
	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	var resumed model.AgentRun
	if err := json.Unmarshal(raw, &resumed); err != nil {
		t.Fatal(err)
	}
	meter = NewTokenPricedAIUsageMeter(NewCommunityAIUsage(store))
	if err := meter.reconcileAgentRun(context.Background(), &resumed, agentRuntimeUsagePayload{InputTokens: 130, OutputTokens: 25}); err != nil {
		t.Fatal(err)
	}
	if len(store.entries) != 2 || store.entries[1].InputTokens != 30 || store.entries[1].OutputTokens != 5 {
		t.Fatalf("resume usage=%+v", store.entries)
	}
	if store.entries[0].IdempotencyKey == store.entries[1].IdempotencyKey {
		t.Fatal("turn identities collide")
	}
}

func TestCommunityCheckpointFailureRestoresWatermark(t *testing.T) {
	store := &recordingCommunityUsage{fail: true}
	meter := NewTokenPricedAIUsageMeter(NewCommunityAIUsage(store))
	run := &model.AgentRun{ID: "run", WorkspaceID: "ws", Input: json.RawMessage(`{}`)}
	agent := &model.Agent{Provider: communityString("openai"), Model: communityString("custom")}
	if err := PreflightAgentRunAIUsage(context.Background(), meter, run, agent); err != nil {
		t.Fatal(err)
	}
	before := string(run.OutputSummary)
	if err := meter.checkpointAgentRun(context.Background(), run, agentRuntimeUsagePayload{InputTokens: 100}); err == nil {
		t.Fatal("store failure ignored")
	}
	if string(run.OutputSummary) != before {
		t.Fatal("failed checkpoint advanced watermark")
	}
}

func communityString(value string) *string { return &value }
