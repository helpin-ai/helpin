package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakeAgentRuntimeProjectionRunRepo struct {
	byID          map[string]*model.AgentRun
	byExternal    map[string]*model.AgentRun
	active        []model.AgentRun
	updates       int
	notifications int
}

func (r *fakeAgentRuntimeProjectionRunRepo) GetByIDAny(_ context.Context, id string) (*model.AgentRun, error) {
	if r.byID == nil {
		return nil, nil
	}
	return r.byID[id], nil
}

func (r *fakeAgentRuntimeProjectionRunRepo) GetByExternalRuntimeID(_ context.Context, externalRuntime, externalRuntimeID string) (*model.AgentRun, error) {
	if r.byExternal == nil {
		return nil, nil
	}
	return r.byExternal[externalRuntime+"|"+externalRuntimeID], nil
}

func (r *fakeAgentRuntimeProjectionRunRepo) ListActiveByExternalRuntime(_ context.Context, _ string, _ time.Time, _ int) ([]model.AgentRun, error) {
	return r.active, nil
}

func (r *fakeAgentRuntimeProjectionRunRepo) Update(_ context.Context, run *model.AgentRun) error {
	r.updates++
	if r.byID == nil {
		r.byID = map[string]*model.AgentRun{}
	}
	r.byID[run.ID] = run
	if run.ExternalRuntime != nil && run.ExternalRuntimeID != nil {
		if r.byExternal == nil {
			r.byExternal = map[string]*model.AgentRun{}
		}
		r.byExternal[*run.ExternalRuntime+"|"+*run.ExternalRuntimeID] = run
	}
	return nil
}

func (r *fakeAgentRuntimeProjectionRunRepo) Notify(_ context.Context, _ *model.AgentRun) {
	r.notifications++
}

type fakeAgentRuntimeProjectionAgentRepo struct {
	agent *model.Agent
	err   error
}

func (r *fakeAgentRuntimeProjectionAgentRepo) GetByID(_ context.Context, _, _ string) (*model.Agent, error) {
	return r.agent, r.err
}

type fakeAgentRuntimeProjectionUsageConsumer struct {
	preflightErr    error
	preflightInputs []BillingCreditPreflight
	consumeErr      error
	consumeInputs   []BillingCreditConsumption
}

func (c *fakeAgentRuntimeProjectionUsageConsumer) PreflightCredits(_ context.Context, input BillingCreditPreflight) error {
	c.preflightInputs = append(c.preflightInputs, input)
	return c.preflightErr
}

func (c *fakeAgentRuntimeProjectionUsageConsumer) ConsumeCredits(_ context.Context, input BillingCreditConsumption) (*BillingSummary, error) {
	c.consumeInputs = append(c.consumeInputs, input)
	if c.consumeErr != nil {
		return nil, c.consumeErr
	}
	return nil, nil
}

type fakeAgentRuntimeProjectionMessageRepo struct {
	messages []model.AgentRunMessage
	creates  int
}

func (r *fakeAgentRuntimeProjectionMessageRepo) ListByRun(_ context.Context, _, _ string) ([]model.AgentRunMessage, error) {
	return append([]model.AgentRunMessage(nil), r.messages...), nil
}

func (r *fakeAgentRuntimeProjectionMessageRepo) NextSequence(_ context.Context, _, _ string) (int, error) {
	return len(r.messages) + 1, nil
}

func (r *fakeAgentRuntimeProjectionMessageRepo) Create(_ context.Context, message *model.AgentRunMessage) error {
	r.creates++
	r.messages = append(r.messages, *message)
	return nil
}

type fakeAgentRuntimeProjectionArtifactRepo struct {
	artifacts []model.AgentRunArtifact
	creates   int
}

func (r *fakeAgentRuntimeProjectionArtifactRepo) ListByRun(_ context.Context, _, _ string) ([]model.AgentRunArtifact, error) {
	return append([]model.AgentRunArtifact(nil), r.artifacts...), nil
}

func (r *fakeAgentRuntimeProjectionArtifactRepo) NextSequence(_ context.Context, _, _ string) (int, error) {
	return len(r.artifacts) + 1, nil
}

func (r *fakeAgentRuntimeProjectionArtifactRepo) Create(_ context.Context, artifact *model.AgentRunArtifact) error {
	r.creates++
	r.artifacts = append(r.artifacts, *artifact)
	return nil
}

type fakeAgentRuntimeProjectionInteractionRepo struct {
	interactions []model.AgentRunInteraction
	creates      int
	updates      int
}

func (r *fakeAgentRuntimeProjectionInteractionRepo) ListByRun(_ context.Context, _, _ string) ([]model.AgentRunInteraction, error) {
	return append([]model.AgentRunInteraction(nil), r.interactions...), nil
}

func (r *fakeAgentRuntimeProjectionInteractionRepo) Create(_ context.Context, interaction *model.AgentRunInteraction) error {
	r.creates++
	r.interactions = append(r.interactions, *interaction)
	return nil
}

func (r *fakeAgentRuntimeProjectionInteractionRepo) Update(_ context.Context, interaction *model.AgentRunInteraction) error {
	r.updates++
	for index := range r.interactions {
		if r.interactions[index].ID == interaction.ID {
			r.interactions[index] = *interaction
			return nil
		}
	}
	r.interactions = append(r.interactions, *interaction)
	return nil
}

func TestAgentRuntimeProjectionMapsLifecycleByHostRunID(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	sentAt := now.Add(-2 * time.Minute)
	run := &model.AgentRun{
		ID:          "helpin-run-1",
		Status:      model.AgentRunStatusQueued,
		PauseReason: model.AgentRunPauseReasonNone,
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{run.ID: run}}
	svc := &AgentRuntimeProjectionService{
		runRepo: repo,
		now:     func() time.Time { return now },
	}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:     "run_runtime_1",
		HostRunID: run.ID,
		Type:      "run.started",
		SentAt:    sentAt,
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusRunning || run.PauseReason != model.AgentRunPauseReasonNone {
		t.Fatalf("expected running/none, got %s/%s", run.Status, run.PauseReason)
	}
	if run.ExternalRuntime == nil || *run.ExternalRuntime != agentRuntimeName {
		t.Fatalf("expected external runtime to be set, got %#v", run.ExternalRuntime)
	}
	if run.ExternalRuntimeID == nil || *run.ExternalRuntimeID != "run_runtime_1" {
		t.Fatalf("expected external runtime id, got %#v", run.ExternalRuntimeID)
	}
	if run.StartedAt == nil || !run.StartedAt.Equal(sentAt) {
		t.Fatalf("expected started_at %s, got %#v", sentAt, run.StartedAt)
	}
	if repo.updates != 1 || repo.notifications != 1 {
		t.Fatalf("expected one update/notify, got %d/%d", repo.updates, repo.notifications)
	}

	err = svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_1",
		Type:  "run.paused",
		Data:  map[string]any{"pause_reason": model.AgentRunPauseReasonHumanApproval},
	})
	if err != nil {
		t.Fatalf("ApplyEvent paused returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusPaused || run.PauseReason != model.AgentRunPauseReasonHumanApproval {
		t.Fatalf("expected paused/human_approval, got %s/%s", run.Status, run.PauseReason)
	}
}

func TestAgentRuntimeProjectionMirrorsAssistantMessageCompletedIdempotently(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-message",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_message"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_message": run},
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{}
	svc := &AgentRuntimeProjectionService{
		runRepo:        runRepo,
		runMessageRepo: messageRepo,
		now:            time.Now,
	}
	event := AgentRuntimeEventEnvelope{
		RunID:     "run_runtime_message",
		HostRunID: run.ID,
		Type:      "assistant_message_completed",
		Data: map[string]any{
			"message_id": "runtime-message-1",
			"content":    "Done with the task.",
		},
	}
	if err := svc.ApplyEvent(context.Background(), event); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if err := svc.ApplyEvent(context.Background(), event); err != nil {
		t.Fatalf("ApplyEvent redelivery returned error: %v", err)
	}
	if messageRepo.creates != 1 || len(messageRepo.messages) != 1 {
		t.Fatalf("expected exactly one mirrored message, creates=%d messages=%#v", messageRepo.creates, messageRepo.messages)
	}
	message := messageRepo.messages[0]
	if message.Role != "assistant" || message.MessageType != "assistant_turn" || message.Content != "Done with the task." || message.SequenceNo != 1 {
		t.Fatalf("unexpected mirrored message: %#v", message)
	}
	if !agentRunMessageHasRuntimeMessageID(message, "runtime-message-1") {
		t.Fatalf("mirrored message missing runtime id in content blocks: %s", string(message.ContentBlocks))
	}
}

func TestAgentRuntimeProjectionCancelsOnCumulativeUsageOverage(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-overage",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_overage"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_overage": run},
	}
	usageConsumer := &fakeAgentRuntimeProjectionUsageConsumer{preflightErr: model.ErrAIUsageExhausted}
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentRuntimeProjectionService{
		runRepo: repo,
		agentRepo: &fakeAgentRuntimeProjectionAgentRepo{agent: &model.Agent{
			ID:        "agent-1",
			PresetKey: model.AgentPresetCodeBuilder,
			IsSystem:  true,
		}},
		usageMeter:         &AIUsageMeter{consumer: usageConsumer},
		agentRuntimeClient: runtimeClient,
		now:                time.Now,
	}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_overage",
		Type:  "usage.checkpoint",
		Data: map[string]any{
			"usage": map[string]any{
				"total_tokens":        float64(12000),
				"input_tokens":        float64(10000),
				"output_tokens":       float64(2000),
				"cached_input_tokens": float64(0),
			},
			"usage_semantic": "cumulative",
		},
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(runtimeClient.cancelCalls) != 1 || runtimeClient.cancelCalls[0] != "run_runtime_overage" {
		t.Fatalf("expected runtime cancel call, got %#v", runtimeClient.cancelCalls)
	}
	if run.ExecutionStage == nil || *run.ExecutionStage != agentRuntimeExecutionStageUsageOverageCancel {
		t.Fatalf("expected overage cancel stage, got %#v", run.ExecutionStage)
	}
	if run.Status != model.AgentRunStatusRunning {
		t.Fatalf("expected status to remain projection-owned running, got %q", run.Status)
	}
	if len(usageConsumer.preflightInputs) != 1 || usageConsumer.preflightInputs[0].FeatureKey != BillingFeatureForgeRun {
		t.Fatalf("expected Forge preflight input, got %#v", usageConsumer.preflightInputs)
	}
	if repo.updates != 1 || repo.notifications != 1 {
		t.Fatalf("expected one update/notify, got %d/%d", repo.updates, repo.notifications)
	}
}

func TestAgentRuntimeProjectionDoesNotCancelOverageForNonCumulativeUsage(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-delta",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_delta"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_delta": run},
	}
	usageConsumer := &fakeAgentRuntimeProjectionUsageConsumer{preflightErr: model.ErrAIUsageExhausted}
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	svc := &AgentRuntimeProjectionService{
		runRepo:            repo,
		agentRepo:          &fakeAgentRuntimeProjectionAgentRepo{agent: &model.Agent{ID: "agent-1"}},
		usageMeter:         &AIUsageMeter{consumer: usageConsumer},
		agentRuntimeClient: runtimeClient,
		now:                time.Now,
	}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_delta",
		Type:  "usage.checkpoint",
		Data: map[string]any{
			"usage": map[string]any{
				"input_tokens":  float64(10000),
				"output_tokens": float64(2000),
			},
			"usage_semantic": "delta",
		},
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(runtimeClient.cancelCalls) != 0 {
		t.Fatalf("expected no runtime cancel calls, got %#v", runtimeClient.cancelCalls)
	}
	if len(usageConsumer.preflightInputs) != 0 {
		t.Fatalf("expected no preflight for non-cumulative checkpoint, got %#v", usageConsumer.preflightInputs)
	}
}

func TestAgentRuntimeProjectionConsumesTerminalUsageOnce(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 14, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-consume",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_consume"),
		OutputSummary:     json.RawMessage(`{}`),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_consume": run},
	}
	usageConsumer := &fakeAgentRuntimeProjectionUsageConsumer{}
	svc := &AgentRuntimeProjectionService{
		runRepo: runRepo,
		agentRepo: &fakeAgentRuntimeProjectionAgentRepo{agent: &model.Agent{
			ID:        "agent-1",
			PresetKey: model.AgentPresetCodeBuilder,
			IsSystem:  true,
		}},
		usageMeter: &AIUsageMeter{consumer: usageConsumer},
		now:        func() time.Time { return completedAt },
	}
	event := AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_consume",
		Type:   "run.completed",
		SentAt: completedAt,
		Data: map[string]any{
			"usage": map[string]any{
				"total_tokens":            float64(30),
				"input_tokens":            float64(10),
				"cached_input_tokens":     float64(2),
				"output_tokens":           float64(4),
				"reasoning_output_tokens": float64(1),
			},
			"usage_semantic": "cumulative",
		},
	}
	if err := svc.ApplyEvent(context.Background(), event); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if err := svc.ApplyEvent(context.Background(), event); err != nil {
		t.Fatalf("ApplyEvent redelivery returned error: %v", err)
	}
	if len(usageConsumer.consumeInputs) != 1 {
		t.Fatalf("expected one terminal usage consumption, got %#v", usageConsumer.consumeInputs)
	}
	input := usageConsumer.consumeInputs[0]
	if input.WorkspaceID != "ws-1" || input.FeatureKey != BillingFeatureForgeRun || input.IdempotencyKey != "ws-1:agent-runtime:helpin-run-consume:terminal-usage" {
		t.Fatalf("unexpected consumption input: %#v", input)
	}
	if !runtimeUsageAlreadyConsumed(run.OutputSummary) {
		t.Fatalf("expected output summary marker, got %s", string(run.OutputSummary))
	}
}

func TestAgentRuntimeProjectionTerminalUsageFailureDoesNotBlockStatusProjection(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 14, 30, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-consume-failure",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_consume_failure"),
		OutputSummary:     json.RawMessage(`{}`),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_consume_failure": run},
	}
	usageConsumer := &fakeAgentRuntimeProjectionUsageConsumer{consumeErr: model.ErrBillingWorkspaceLocked}
	svc := &AgentRuntimeProjectionService{
		runRepo: runRepo,
		agentRepo: &fakeAgentRuntimeProjectionAgentRepo{agent: &model.Agent{
			ID:        "agent-1",
			PresetKey: model.AgentPresetCodeBuilder,
			IsSystem:  true,
		}},
		usageMeter: &AIUsageMeter{consumer: usageConsumer},
		now:        func() time.Time { return completedAt },
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_consume_failure",
		Type:   "run.completed",
		SentAt: completedAt,
		Data: map[string]any{
			"usage": map[string]any{
				"total_tokens":  float64(18),
				"input_tokens":  float64(12),
				"output_tokens": float64(6),
			},
			"usage_semantic": "cumulative",
		},
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusCompleted || run.CompletedAt == nil || !run.CompletedAt.Equal(completedAt) {
		t.Fatalf("terminal status not projected after consume failure: status=%s completed_at=%v", run.Status, run.CompletedAt)
	}
	if run.InputTokens != 12 || run.OutputTokens != 6 || run.TokensUsed != 18 {
		t.Fatalf("usage counters not projected after consume failure: input=%d output=%d total=%d", run.InputTokens, run.OutputTokens, run.TokensUsed)
	}
	if runtimeUsageAlreadyConsumed(run.OutputSummary) {
		t.Fatalf("consume marker should remain unset after failed consume, got %s", string(run.OutputSummary))
	}
	if runRepo.updates != 1 || runRepo.notifications != 1 {
		t.Fatalf("expected status update/notify despite consume failure, got %d/%d", runRepo.updates, runRepo.notifications)
	}
}

func TestAgentRuntimeProjectionDoesNotRegressTerminalRunOnLatePreTerminalEvent(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-terminal",
		Status:            model.AgentRunStatusCompleted,
		PauseReason:       model.AgentRunPauseReasonNone,
		CompletedAt:       &completedAt,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_terminal"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_terminal": run},
	}
	svc := &AgentRuntimeProjectionService{runRepo: repo, now: time.Now}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_terminal",
		Type:   "run.started",
		SentAt: completedAt.Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.Status != model.AgentRunStatusCompleted || run.CompletedAt == nil || !run.CompletedAt.Equal(completedAt) {
		t.Fatalf("terminal run regressed: status=%s completed_at=%v", run.Status, run.CompletedAt)
	}
	if repo.updates != 0 || repo.notifications != 0 {
		t.Fatalf("expected no update/notify for late pre-terminal event, got %d/%d", repo.updates, repo.notifications)
	}
}

func TestAgentRuntimeProjectionAppliesCumulativeUsage(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-2",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_2"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_2": run},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo: repo,
		now:     time.Now,
	}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_2",
		Type:  "usage.checkpoint",
		Data: map[string]any{
			"usage": map[string]any{
				"total_tokens":            float64(25),
				"input_tokens":            float64(12),
				"cached_input_tokens":     float64(4),
				"output_tokens":           float64(8),
				"reasoning_output_tokens": float64(5),
			},
			"usage_semantic": "cumulative",
		},
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.InputTokens != 12 || run.CachedInputTokens != 4 || run.OutputTokens != 8 || run.TokensUsed != 25 {
		t.Fatalf("usage not projected: input=%d cached=%d output=%d total=%d", run.InputTokens, run.CachedInputTokens, run.OutputTokens, run.TokensUsed)
	}
	if repo.updates != 1 || repo.notifications != 1 {
		t.Fatalf("expected one update/notify, got %d/%d", repo.updates, repo.notifications)
	}
}

func TestAgentRuntimeProjectionUsageFallbackIncludesCachedTokens(t *testing.T) {
	run := &model.AgentRun{
		ID:                "helpin-run-usage",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_usage"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_usage": run},
	}
	svc := &AgentRuntimeProjectionService{runRepo: repo, now: time.Now}

	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_runtime_usage",
		Type:  "usage.checkpoint",
		Data: map[string]any{
			"usage": map[string]any{
				"input_tokens":            float64(12),
				"cached_input_tokens":     float64(4),
				"output_tokens":           float64(8),
				"reasoning_output_tokens": float64(5),
			},
			"usage_semantic": "cumulative",
		},
	})
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if run.TokensUsed != 29 {
		t.Fatalf("expected fallback total to include cached tokens, got %d", run.TokensUsed)
	}
}

func TestAgentRuntimeProjectionReconcileMappedRunsAppliesFetchedRuntimeState(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 12, 30, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-reconcile",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_reconcile"),
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_reconcile": run},
		active:     []model.AgentRun{*run},
	}
	runtimeClient := &fakeAgentRuntimeSignalClient{
		getRuns: map[string]*AgentRuntimeRun{
			"run_runtime_reconcile": {
				ID:            "run_runtime_reconcile",
				AppID:         "helpin",
				HostRunID:     run.ID,
				Status:        model.AgentRunStatusCompleted,
				CompletedAt:   &completedAt,
				OutputSummary: json.RawMessage(`{"input_tokens":12,"cached_input_tokens":3,"output_tokens":8}`),
			},
		},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:            repo,
		agentRuntimeClient: runtimeClient,
		now:                func() time.Time { return completedAt.Add(time.Minute) },
	}

	if err := svc.ReconcileMappedRuns(context.Background(), time.Minute, 10); err != nil {
		t.Fatalf("ReconcileMappedRuns returned error: %v", err)
	}
	if len(runtimeClient.getCalls) != 1 || runtimeClient.getCalls[0] != "run_runtime_reconcile" {
		t.Fatalf("expected runtime get call, got %#v", runtimeClient.getCalls)
	}
	if run.Status != model.AgentRunStatusCompleted || run.CompletedAt == nil || !run.CompletedAt.Equal(completedAt) {
		t.Fatalf("expected completed run from reconciliation, got status=%s completed_at=%v", run.Status, run.CompletedAt)
	}
	if run.InputTokens != 12 || run.CachedInputTokens != 3 || run.OutputTokens != 8 || run.TokensUsed != 23 {
		t.Fatalf("expected usage from runtime summary, got input=%d cached=%d output=%d total=%d", run.InputTokens, run.CachedInputTokens, run.OutputTokens, run.TokensUsed)
	}
}

func TestAgentRuntimeProjectionReconcileMirrorsRuntimeTranscriptCollections(t *testing.T) {
	now := time.Date(2026, 7, 2, 13, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-transcript",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		RuntimeKind:       "native_sdk",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_transcript"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byID:       map[string]*model.AgentRun{run.ID: run},
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_transcript": run},
		active:     []model.AgentRun{*run},
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{}
	artifactRepo := &fakeAgentRuntimeProjectionArtifactRepo{}
	interactionRepo := &fakeAgentRuntimeProjectionInteractionRepo{}
	resolvedAt := now.Add(time.Minute)
	runtimeClient := &fakeAgentRuntimeSignalClient{
		getRuns: map[string]*AgentRuntimeRun{
			"run_runtime_transcript": {
				ID:        "run_runtime_transcript",
				AppID:     "helpin",
				HostRunID: run.ID,
				Status:    model.AgentRunStatusRunning,
				UpdatedAt: now,
			},
		},
		messages: map[string][]AgentRuntimeMessage{
			"run_runtime_transcript": {{
				ID:          "msg-runtime-1",
				Role:        "assistant",
				Content:     "I checked the repository.",
				MessageType: "assistant_turn",
				CreatedAt:   now,
			}},
		},
		artifacts: map[string][]AgentRuntimeArtifact{
			"run_runtime_transcript": {{
				ID:            "art-runtime-1",
				ArtifactType:  model.AgentRunArtifactTypeRunPlan,
				Format:        "json",
				StorageMode:   "inline",
				InlineContent: `{"plan":[]}`,
				SequenceNo:    1,
				CreatedAt:     now,
			}},
		},
		interactions: map[string][]AgentRuntimeInteraction{
			"run_runtime_transcript": {{
				ID:                   "int-runtime-1",
				RuntimeKind:          "native_sdk",
				InteractionKind:      "human_input",
				Status:               model.AgentRunInteractionStatusResolved,
				Title:                "Input requested",
				Summary:              "Pick one",
				RequestPayload:       json.RawMessage(`{"prompt":"Pick one"}`),
				ResponsePayload:      json.RawMessage(`{"answer":"A"}`),
				ResolvedByExternalID: "user-1",
				ResolvedAt:           &resolvedAt,
				CreatedAt:            now,
				UpdatedAt:            resolvedAt,
			}},
		},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		runMessageRepo:     messageRepo,
		artifactRepo:       artifactRepo,
		interactionRepo:    interactionRepo,
		agentRuntimeClient: runtimeClient,
		now:                func() time.Time { return now.Add(2 * time.Minute) },
	}

	if err := svc.ReconcileMappedRuns(context.Background(), time.Minute, 10); err != nil {
		t.Fatalf("ReconcileMappedRuns returned error: %v", err)
	}
	if err := svc.ReconcileMappedRuns(context.Background(), time.Minute, 10); err != nil {
		t.Fatalf("second ReconcileMappedRuns returned error: %v", err)
	}
	if messageRepo.creates != 1 || len(messageRepo.messages) != 1 {
		t.Fatalf("expected one mirrored message, creates=%d messages=%#v", messageRepo.creates, messageRepo.messages)
	}
	if !agentRunMessageHasRuntimeMessageID(messageRepo.messages[0], "msg-runtime-1") {
		t.Fatalf("message missing runtime id: %s", string(messageRepo.messages[0].ContentBlocks))
	}
	if artifactRepo.creates != 1 || len(artifactRepo.artifacts) != 1 {
		t.Fatalf("expected one mirrored artifact, creates=%d artifacts=%#v", artifactRepo.creates, artifactRepo.artifacts)
	}
	if !agentRunArtifactHasRuntimeArtifactID(artifactRepo.artifacts[0], "art-runtime-1") {
		t.Fatalf("artifact missing runtime id: %s", string(artifactRepo.artifacts[0].Metadata))
	}
	if interactionRepo.creates != 1 || interactionRepo.updates != 0 || len(interactionRepo.interactions) != 1 {
		t.Fatalf("expected one mirrored interaction and no second update, creates=%d updates=%d interactions=%#v", interactionRepo.creates, interactionRepo.updates, interactionRepo.interactions)
	}
	interaction := interactionRepo.interactions[0]
	if interaction.InteractionKind != model.AgentRunInteractionKindRequestUserInput || interaction.Status != model.AgentRunInteractionStatusResolved {
		t.Fatalf("unexpected interaction projection: %#v", interaction)
	}
	if !agentRunInteractionHasRuntimeInteractionID(interaction, "int-runtime-1") {
		t.Fatalf("interaction missing runtime id: %s", string(interaction.RuntimeMetadata))
	}
}

func TestAgentRuntimeProjectionTerminalEventBackfillsRuntimeTranscript(t *testing.T) {
	completedAt := time.Date(2026, 7, 2, 15, 0, 0, 0, time.UTC)
	run := &model.AgentRun{
		ID:                "helpin-run-terminal-transcript",
		WorkspaceID:       "ws-1",
		AgentID:           "agent-1",
		RuntimeKind:       "native_sdk",
		Status:            model.AgentRunStatusRunning,
		PauseReason:       model.AgentRunPauseReasonNone,
		ExternalRuntime:   stringPointer(agentRuntimeName),
		ExternalRuntimeID: stringPointer("run_runtime_terminal_transcript"),
	}
	runRepo := &fakeAgentRuntimeProjectionRunRepo{
		byExternal: map[string]*model.AgentRun{agentRuntimeName + "|run_runtime_terminal_transcript": run},
	}
	messageRepo := &fakeAgentRuntimeProjectionMessageRepo{}
	runtimeClient := &fakeAgentRuntimeSignalClient{
		messages: map[string][]AgentRuntimeMessage{
			"run_runtime_terminal_transcript": {{
				ID:          "msg-runtime-terminal",
				Role:        "assistant",
				Content:     "Terminal transcript backfill.",
				MessageType: "assistant_turn",
				CreatedAt:   completedAt,
			}},
		},
	}
	svc := &AgentRuntimeProjectionService{
		runRepo:            runRepo,
		runMessageRepo:     messageRepo,
		agentRuntimeClient: runtimeClient,
		now:                func() time.Time { return completedAt },
	}

	if err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID:  "run_runtime_terminal_transcript",
		Type:   "run.completed",
		SentAt: completedAt,
	}); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(runtimeClient.listMessageCalls) != 1 || runtimeClient.listMessageCalls[0] != "run_runtime_terminal_transcript" {
		t.Fatalf("expected transcript list on terminal event, got %#v", runtimeClient.listMessageCalls)
	}
	if messageRepo.creates != 1 || len(messageRepo.messages) != 1 {
		t.Fatalf("expected one mirrored terminal message, creates=%d messages=%#v", messageRepo.creates, messageRepo.messages)
	}
	if !agentRunMessageHasRuntimeMessageID(messageRepo.messages[0], "msg-runtime-terminal") {
		t.Fatalf("message missing runtime id: %s", string(messageRepo.messages[0].ContentBlocks))
	}
}

func TestAgentRuntimeProjectionReturnsNotFoundForUnmappedRun(t *testing.T) {
	svc := &AgentRuntimeProjectionService{
		runRepo: &fakeAgentRuntimeProjectionRunRepo{},
		now:     time.Now,
	}
	err := svc.ApplyEvent(context.Background(), AgentRuntimeEventEnvelope{
		RunID: "run_missing",
		Type:  "run.completed",
	})
	if !errors.Is(err, errAgentRuntimeProjectionRunNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestAgentRuntimeProjectionSubjectIsScopedToApp(t *testing.T) {
	svc := NewAgentRuntimeProjectionService(nil, "helpin")
	if got := svc.eventSubject(); got != "agent-runtime.events.helpin.>" {
		t.Fatalf("expected app-scoped subject, got %q", got)
	}
}

func stringPointer(value string) *string {
	return &value
}
