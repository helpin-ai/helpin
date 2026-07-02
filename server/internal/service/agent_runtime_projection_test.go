package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type fakeAgentRuntimeProjectionRunRepo struct {
	byID          map[string]*model.AgentRun
	byExternal    map[string]*model.AgentRun
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
