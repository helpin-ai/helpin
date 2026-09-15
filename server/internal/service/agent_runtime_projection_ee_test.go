//go:build ee

package service

import (
	"context"
	"encoding/json"
	"errors"
	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"testing"
	"time"
)

func TestAgentRuntimeProjectionCheckpointsDockChatUsageOnUserMessagePause(t *testing.T) {
	store := &fakeAIUsageStore{}
	usageService := newTestAIUsageService(t, store)
	metering, err := usageService.ResolveMeteringContext(MeteringRequest{
		WorkspaceID: "ws-1", TaskNature: "general", FeatureKey: BillingFeatureBuiltInLightAgentRun,
		Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731", ServiceTier: "standard",
		IdempotencyKey: "ws-1:agent_run:run-chat",
	})
	if err != nil {
		t.Fatal(err)
	}
	metering.ReservationID = "reservation"
	metering.MaxBillableMicrousd = 1_000_000
	metering.EnforcementMode = model.AIUsageEnforcementStrict
	dockChatID := "chat-1"
	run := &model.AgentRun{
		ID: "run-chat", WorkspaceID: "ws-1", AgentID: "agent-1",
		DockChatID: &dockChatID, Status: model.AgentRunStatusRunning,
		PauseReason: model.AgentRunPauseReasonNone, ExternalRuntime: stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("runtime-chat"), OutputSummary: json.RawMessage(`{}`),
	}
	if err := storeAgentRunMeteringContext(run, metering); err != nil {
		t.Fatal(err)
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|runtime-chat": run},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:    runRepo,
		usageMeter: &AIUsageMeter{usage: usageService},
		now:        time.Now,
	}
	usageEvent := AgentRuntimeEventEnvelope{
		RunID: "runtime-chat", Type: agentruntime.EventUsageCheckpoint,
		Data: map[string]any{
			"usage": map[string]any{
				"input_tokens": float64(100), "cached_input_tokens": float64(20),
				"output_tokens": float64(10), "reasoning_output_tokens": float64(3), "total_tokens": float64(110),
			},
			"usage_semantic": agentruntime.UsageSemanticCumulative,
		},
	}
	if err := svc.ApplyEvent(context.Background(), usageEvent); err != nil {
		t.Fatal(err)
	}
	if store.checkpoints != 0 {
		t.Fatalf("running usage checkpoint calls = %d, want 0 until the turn pauses", store.checkpoints)
	}
	pauseEvent := AgentRuntimeEventEnvelope{
		RunID: "runtime-chat", Type: agentruntime.EventRunPaused,
		Data: map[string]any{"pause_reason": model.AgentRunPauseReasonUserMessage},
	}
	if err := svc.ApplyEvent(context.Background(), pauseEvent); err != nil {
		t.Fatal(err)
	}
	if run.Status != model.AgentRunStatusPaused || run.PauseReason != model.AgentRunPauseReasonUserMessage {
		t.Fatalf("run pause = %s/%s", run.Status, run.PauseReason)
	}
	if store.checkpoints != 1 || store.checkpoint.Entry.InputTokensTotal != 100 || store.checkpoint.Entry.ReasoningTokens != 3 {
		t.Fatalf("checkpoint = %#v, calls=%d", store.checkpoint.Entry, store.checkpoints)
	}
	if store.resizeCalls != 1 || store.resizedID != "reservation" || store.resizedTo != 0 {
		t.Fatalf("paused reservation resize = id %q target %d calls %d", store.resizedID, store.resizedTo, store.resizeCalls)
	}
	if got := agentRunUsageCheckpointFromSummary(run.OutputSummary); got.Turn != 1 || got.InputTokens != 100 {
		t.Fatalf("checkpoint summary = %#v", got)
	}
	for _, event := range []AgentRuntimeEventEnvelope{pauseEvent, usageEvent} {
		t.Run("unchanged "+event.Type, func(t *testing.T) {
			beforeSummary := string(run.OutputSummary)
			beforeUpdates, beforeNotifications := runRepo.updates, runRepo.notifications
			if err := svc.ApplyEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
			if string(run.OutputSummary) != beforeSummary {
				t.Error("unchanged checkpoint altered output summary")
			}
			if runRepo.updates != beforeUpdates || runRepo.notifications != beforeNotifications {
				t.Errorf("unchanged checkpoint added %d updates and %d notifications, want 0/0", runRepo.updates-beforeUpdates, runRepo.notifications-beforeNotifications)
			}
			if store.checkpoints != 1 {
				t.Errorf("unchanged checkpoint charges = %d, want 1", store.checkpoints)
			}
		})
	}

	t.Run("increased usage", func(t *testing.T) {
		beforeUpdates, beforeNotifications := runRepo.updates, runRepo.notifications
		beforeSuspensions := store.resizeCalls
		usageEvent.Data["usage"] = map[string]any{
			"input_tokens": float64(160), "cached_input_tokens": float64(30),
			"output_tokens": float64(25), "reasoning_output_tokens": float64(5), "total_tokens": float64(185),
		}
		if err := svc.ApplyEvent(context.Background(), usageEvent); err != nil {
			t.Fatal(err)
		}
		if runRepo.updates != beforeUpdates+1 || runRepo.notifications != beforeNotifications+1 {
			t.Errorf("increased usage added %d updates and %d notifications, want 1/1", runRepo.updates-beforeUpdates, runRepo.notifications-beforeNotifications)
		}
		if store.checkpoints != 2 || store.checkpoint.Entry.InputTokensTotal != 60 ||
			store.checkpoint.Entry.CacheReadTokens != 10 || store.checkpoint.Entry.OutputTokens != 13 ||
			store.checkpoint.Entry.ReasoningTokens != 2 {
			t.Errorf("increased usage checkpoint = %#v, calls=%d", store.checkpoint.Entry, store.checkpoints)
		}
		if got := agentRunUsageCheckpointFromSummary(run.OutputSummary); got.Turn != 2 || got.InputTokens != 160 || got.OutputTokens != 25 {
			t.Errorf("persisted checkpoint = %#v", got)
		}
		if string(store.checkpoint.RunOutputSummary) != string(run.OutputSummary) {
			t.Error("billing checkpoint did not persist current run summary")
		}
		if store.resizeCalls != beforeSuspensions+1 || store.resizedTo != 0 {
			t.Error("increased usage did not suspend reservation")
		}
	})

	t.Run("stale checkpoint stops projection", func(t *testing.T) {
		beforeUpdates, beforeNotifications := runRepo.updates, runRepo.notifications
		beforeSuspensions := store.resizeCalls
		store.checkpointErr = repository.ErrAIUsageWatermarkChanged
		defer func() { store.checkpointErr = nil }()
		usageEvent.Data["usage"] = map[string]any{"input_tokens": float64(170), "output_tokens": float64(25)}
		if err := svc.ApplyEvent(context.Background(), usageEvent); !errors.Is(err, repository.ErrAIUsageWatermarkChanged) {
			t.Fatalf("stale checkpoint error = %v", err)
		}
		if runRepo.updates != beforeUpdates || runRepo.notifications != beforeNotifications || store.resizeCalls != beforeSuspensions {
			t.Fatal("stale checkpoint persisted, notified, or suspended a reservation")
		}
	})

	t.Run("failed checkpoint retries retained usage", func(t *testing.T) {
		previousCheckpoint := agentRunUsageCheckpointFromSummary(run.OutputSummary)
		beforeSuspensions := store.resizeCalls
		store.checkpointErr = errors.New("checkpoint unavailable")
		usageEvent.Data["usage"] = map[string]any{
			"input_tokens": float64(180), "cached_input_tokens": float64(30),
			"output_tokens": float64(25), "reasoning_output_tokens": float64(5), "total_tokens": float64(205),
		}
		if err := svc.ApplyEvent(context.Background(), usageEvent); err != nil {
			t.Fatal(err)
		}
		if got := agentRunUsageCheckpointFromSummary(run.OutputSummary); got != previousCheckpoint {
			t.Errorf("failed checkpoint changed billed usage: %#v", got)
		}
		if got, ok := latestAgentRuntimeUsage(run); !ok || got.InputTokens != 180 {
			t.Errorf("latest usage was not retained for retry: %#v", got)
		}
		if store.resizeCalls != beforeSuspensions {
			t.Error("failed checkpoint suspended reservation")
		}
		failedIdempotencyKey := store.checkpoint.Entry.IdempotencyKey
		beforeUpdates, beforeNotifications := runRepo.updates, runRepo.notifications
		beforeCheckpoints := store.checkpoints
		store.checkpointErr = nil
		if err := svc.ApplyEvent(context.Background(), pauseEvent); err != nil {
			t.Fatal(err)
		}
		if store.checkpoints != beforeCheckpoints+1 || store.checkpoint.Entry.IdempotencyKey != failedIdempotencyKey ||
			store.checkpoint.Entry.InputTokensTotal != int64(180-previousCheckpoint.InputTokens) {
			t.Error("retry did not checkpoint retained usage with the same idempotency key")
		}
		if got := agentRunUsageCheckpointFromSummary(run.OutputSummary); got.Turn != previousCheckpoint.Turn+1 || got.InputTokens != 180 {
			t.Errorf("retried checkpoint = %#v", got)
		}
		if runRepo.updates != beforeUpdates+1 || runRepo.notifications != beforeNotifications+1 {
			t.Error("checkpoint recovery did not persist and notify")
		}
		if store.resizeCalls != beforeSuspensions+1 || store.resizedTo != 0 {
			t.Error("successful retry did not suspend reservation")
		}
	})
}

func TestAgentRuntimeProjectionCheckpointsUsageArrivingAfterDockChatPause(t *testing.T) {
	store := &fakeAIUsageStore{}
	usageService := newTestAIUsageService(t, store)
	metering, err := usageService.ResolveMeteringContext(MeteringRequest{
		WorkspaceID: "ws-1", TaskNature: "support", FeatureKey: BillingFeatureAskChat,
		Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731", ServiceTier: "standard",
		IdempotencyKey: "ws-1:agent_run:run-late-usage",
	})
	if err != nil {
		t.Fatal(err)
	}
	metering.ReservationID = "late-reservation"
	metering.MaxBillableMicrousd = 25_000
	metering.EnforcementMode = model.AIUsageEnforcementStrict
	dockChatID := "chat-late"
	run := &model.AgentRun{
		ID: "run-late-usage", WorkspaceID: "ws-1", AgentID: "ask-agent",
		DockChatID: &dockChatID, Status: model.AgentRunStatusRunning,
		PauseReason: model.AgentRunPauseReasonNone, ExternalRuntime: stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("runtime-late-usage"), OutputSummary: json.RawMessage(`{}`),
	}
	if err := storeAgentRunMeteringContext(run, metering); err != nil {
		t.Fatal(err)
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|runtime-late-usage": run},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo: repo, usageMeter: &AIUsageMeter{usage: usageService}, now: time.Now,
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "runtime-late-usage", Type: agentruntime.EventRunPaused,
		Data: map[string]any{"pause_reason": model.AgentRunPauseReasonUserMessage},
	}); err != nil {
		t.Fatal(err)
	}
	if store.checkpoints != 0 || store.resizeCalls != 1 || store.resizedTo != 0 {
		t.Fatalf("pause before telemetry = checkpoints %d resize target %d calls %d", store.checkpoints, store.resizedTo, store.resizeCalls)
	}
	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "runtime-late-usage", Type: agentruntime.EventUsageCheckpoint,
		Data: map[string]any{
			"usage": map[string]any{
				"input_tokens": float64(600), "cached_input_tokens": float64(400),
				"output_tokens": float64(50), "total_tokens": float64(650),
			},
			"usage_semantic": agentruntime.UsageSemanticCumulative,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if store.checkpoints != 1 || store.checkpoint.Entry.InputTokensTotal != 600 || store.checkpoint.Entry.OutputTokens != 50 {
		t.Fatalf("late usage checkpoint = %#v, calls=%d", store.checkpoint.Entry, store.checkpoints)
	}
	if store.resizeCalls != 2 || store.resizedID != "late-reservation" || store.resizedTo != 0 {
		t.Fatalf("late usage reservation suspension = id %q target %d calls %d", store.resizedID, store.resizedTo, store.resizeCalls)
	}
}

func TestAgentRuntimeProjectionReconcileRepairsPausedDockChatUsage(t *testing.T) {
	store := &fakeAIUsageStore{}
	usageService := newTestAIUsageService(t, store)
	metering, err := usageService.ResolveMeteringContext(MeteringRequest{
		WorkspaceID: "ws-1", TaskNature: "support", FeatureKey: BillingFeatureAskChat,
		Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731", ServiceTier: "standard",
		IdempotencyKey: "ws-1:agent_run:run-recovered-chat",
	})
	if err != nil {
		t.Fatal(err)
	}
	metering.ReservationID = "recovered-reservation"
	metering.MaxBillableMicrousd = 25_000
	metering.EnforcementMode = model.AIUsageEnforcementSoft
	dockChatID := "recovered-chat"
	run := &model.AgentRun{
		ID: "run-recovered-chat", WorkspaceID: "ws-1", AgentID: "ask-agent",
		DockChatID: &dockChatID, Status: model.AgentRunStatusPaused,
		PauseReason: model.AgentRunPauseReasonUserMessage, ExternalRuntime: stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("runtime-recovered-chat"), OutputSummary: json.RawMessage(`{}`),
		InputTokens: 2_910_695, CachedInputTokens: 2_301_952, OutputTokens: 21_196, TokensUsed: 2_931_891,
	}
	if err := storeAgentRunMeteringContext(run, metering); err != nil {
		t.Fatal(err)
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|runtime-recovered-chat": run},
		active:     []model.AgentRun{*run},
	}
	runtimeClient := &fakeAgentRuntimeSignalClient{getRuns: map[string]*AgentRuntimeRun{
		"runtime-recovered-chat": {
			ID: "runtime-recovered-chat", HostRunID: run.ID, Status: model.AgentRunStatusPaused,
			PauseReason: model.AgentRunPauseReasonUserMessage,
		},
	}}
	svc := &AgentRuntimeProjectionService{
		runRepo: repo, agentRuntimeClient: runtimeClient,
		usageMeter: &AIUsageMeter{usage: usageService}, now: time.Now,
	}

	if err := svc.ReconcileMappedRuns(context.Background(), time.Minute, 10); err != nil {
		t.Fatalf("ReconcileMappedRuns returned error: %v", err)
	}
	if store.checkpoints != 1 || store.checkpoint.Entry.InputTokensTotal != 2_910_695 || store.checkpoint.Entry.CacheReadTokens != 2_301_952 {
		t.Fatalf("recovered usage checkpoint = %#v, calls=%d", store.checkpoint.Entry, store.checkpoints)
	}
	if store.resizeCalls != 1 || store.resizedID != "recovered-reservation" || store.resizedTo != 0 {
		t.Fatalf("recovered reservation suspension = id %q target %d calls %d", store.resizedID, store.resizedTo, store.resizeCalls)
	}
}
