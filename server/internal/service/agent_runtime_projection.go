package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"github.com/nats-io/nats.go"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	agentRuntimeEventsStreamName                  = "AGENT_RUNTIME_EVENTS"
	agentRuntimeEventsSubjectAll                  = "agent-runtime.events.>"
	agentRuntimeProjectionDurable                 = "helpin-agent-runtime-projection"
	agentRuntimeExecutionStageUsageOverageCancel  = "usage_overage_cancel_requested"
	agentRuntimeUsageOverageCancellationErrorText = "agent runtime run cancelled because workspace AI usage is exhausted"
	agentRuntimeUsageConsumedSummaryKey           = "agent_runtime_usage_consumed"
	agentRuntimeUsageConsumedAtSummaryKey         = "agent_runtime_usage_consumed_at"
	agentRuntimeTranscriptReconciledVersionKey    = "agent_runtime_transcript_reconciled_runtime_updated_at"
	agentRuntimeEventCodexAuthStateChanged        = "codex_auth.state_changed"
	agentRuntimeExecutionStageAuthCompleted       = "auth_completed"
	agentRuntimeExecutionStageAwaitingAuth        = "awaiting_auth"
)

var errAgentRuntimeProjectionRunNotFound = errors.New("agent runtime projection run not found")

var maxIntValue = int64(^uint(0) >> 1)

type agentRuntimeProjectionRunRepository interface {
	GetByIDAny(ctx context.Context, id string) (*model.AgentRun, error)
	GetByExternalRuntimeID(ctx context.Context, externalRuntime, externalRuntimeID string) (*model.AgentRun, error)
	ListActiveByExternalRuntime(ctx context.Context, externalRuntime string, olderThan time.Time, limit int) ([]model.AgentRun, error)
	Update(ctx context.Context, run *model.AgentRun) error
	UpdateOutputSummary(ctx context.Context, runID string, outputSummary json.RawMessage) error
	Notify(ctx context.Context, run *model.AgentRun)
}

type agentRuntimeProjectionAgentRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (*model.Agent, error)
}

type agentRuntimeProjectionMessageRepository interface {
	ListByRun(ctx context.Context, workspaceID, runID string) ([]model.AgentRunMessage, error)
	NextSequence(ctx context.Context, workspaceID, runID string) (int, error)
	Create(ctx context.Context, message *model.AgentRunMessage) error
}

type agentRuntimeProjectionArtifactRepository interface {
	ListByRun(ctx context.Context, workspaceID, runID string) ([]model.AgentRunArtifact, error)
	NextSequence(ctx context.Context, workspaceID, runID string) (int, error)
	Create(ctx context.Context, artifact *model.AgentRunArtifact) error
}

type agentRuntimeProjectionInteractionRepository interface {
	ListByRun(ctx context.Context, workspaceID, runID string) ([]model.AgentRunInteraction, error)
	Create(ctx context.Context, interaction *model.AgentRunInteraction) error
	Update(ctx context.Context, interaction *model.AgentRunInteraction) error
}

// AgentRuntimeProjectionService projects Agent Runtime lifecycle events back
// into Helpin's agent_runs table and existing realtime fanout.
type AgentRuntimeProjectionService struct {
	runRepo            agentRuntimeProjectionRunRepository
	agentRepo          agentRuntimeProjectionAgentRepository
	runMessageRepo     agentRuntimeProjectionMessageRepository
	artifactRepo       agentRuntimeProjectionArtifactRepository
	interactionRepo    agentRuntimeProjectionInteractionRepository
	usageMeter         *AIUsageMeter
	agentRuntimeClient agentRuntimeSignalClient
	runFinalizers      *AgentRunFinalizerService
	appID              string
	now                func() time.Time
}

type agentRuntimeUsagePayload struct {
	TotalTokens           int
	InputTokens           int
	CachedInputTokens     int
	OutputTokens          int
	ReasoningOutputTokens int
}

func NewAgentRuntimeProjectionService(runRepo *repository.AgentRunRepository, appID ...string) *AgentRuntimeProjectionService {
	resolvedAppID := "helpin"
	if len(appID) > 0 && strings.TrimSpace(appID[0]) != "" {
		resolvedAppID = strings.TrimSpace(appID[0])
	}
	return &AgentRuntimeProjectionService{
		runRepo: runRepo,
		appID:   resolvedAppID,
		now:     time.Now,
	}
}

func (s *AgentRuntimeProjectionService) SetOverageDependencies(agentRepo *repository.AgentRepository, usageMeter *AIUsageMeter, runtimeClient agentRuntimeSignalClient) *AgentRuntimeProjectionService {
	if s == nil {
		return s
	}
	s.agentRepo = agentRepo
	s.usageMeter = usageMeter
	s.agentRuntimeClient = runtimeClient
	return s
}

func (s *AgentRuntimeProjectionService) SetTranscriptRepositories(runMessageRepo *repository.AgentRunMessageRepository, artifactRepo *repository.AgentRunArtifactRepository, interactionRepo *repository.AgentRunInteractionRepository) *AgentRuntimeProjectionService {
	if s == nil {
		return s
	}
	s.runMessageRepo = runMessageRepo
	s.artifactRepo = artifactRepo
	s.interactionRepo = interactionRepo
	return s
}

// SetRunFinalizers wires the product side-effect finalizers dispatched when a
// delegated run transitions into a terminal status.
func (s *AgentRuntimeProjectionService) SetRunFinalizers(finalizers *AgentRunFinalizerService) *AgentRuntimeProjectionService {
	if s == nil {
		return s
	}
	s.runFinalizers = finalizers
	return s
}

func (s *AgentRuntimeProjectionService) StartNATSConsumer(ctx context.Context, js nats.JetStreamContext) error {
	if s == nil || s.runRepo == nil || js == nil {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "agent runtime projection consumer panic", "panic", recovered)
		}
	}()
	var sub *nats.Subscription
	for sub == nil {
		candidate, err := s.subscribeNATS(js)
		if err != nil {
			slog.WarnContext(ctx, "agent runtime projection consumer waiting for stream", "stream", agentRuntimeEventsStreamName, "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(10 * time.Second):
				continue
			}
		}
		sub = candidate
	}

	slog.Info("agent runtime projection consumer started")
	for {
		select {
		case <-ctx.Done():
			slog.Info("agent runtime projection consumer shutting down")
			return sub.Drain()
		default:
		}

		msgs, err := sub.Fetch(8, nats.MaxWait(5*time.Second))
		if err != nil {
			if err == nats.ErrTimeout {
				continue
			}
			slog.ErrorContext(ctx, "agent runtime projection fetch failed", "error", err)
			continue
		}
		for _, msg := range msgs {
			s.processNATSMessage(ctx, msg)
		}
	}
}

func (s *AgentRuntimeProjectionService) StartReconciliationSweep(ctx context.Context, interval, staleAfter time.Duration, limit int) error {
	if s == nil || s.runRepo == nil || s.agentRuntimeClient == nil {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "agent runtime reconciliation sweep panic", "panic", recovered)
		}
	}()
	if interval <= 0 {
		interval = time.Minute
	}
	if staleAfter <= 0 {
		staleAfter = 2 * time.Minute
	}
	if limit <= 0 {
		limit = 50
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.ReconcileMappedRuns(ctx, staleAfter, limit); err != nil {
			slog.ErrorContext(ctx, "agent runtime reconciliation sweep failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *AgentRuntimeProjectionService) ReconcileMappedRuns(ctx context.Context, staleAfter time.Duration, limit int) error {
	if s == nil || s.runRepo == nil || s.agentRuntimeClient == nil {
		return nil
	}
	if staleAfter <= 0 {
		staleAfter = 2 * time.Minute
	}
	if limit <= 0 {
		limit = 50
	}
	runs, err := s.runRepo.ListActiveByExternalRuntime(ctx, agentRuntimeName, s.nowUTC().Add(-staleAfter), limit)
	if err != nil {
		return err
	}
	for _, run := range runs {
		runtimeRunID := strings.TrimSpace(derefString(run.ExternalRuntimeID))
		if runtimeRunID == "" {
			continue
		}
		runtimeRun, err := s.agentRuntimeClient.GetRun(ctx, runtimeRunID)
		if err != nil {
			slog.WarnContext(ctx, "agent runtime reconciliation fetch failed",
				"workspace_id", run.WorkspaceID,
				"run_id", run.ID,
				"runtime_run_id", runtimeRunID,
				"error", err,
			)
			continue
		}
		event, ok := reconciliationEventForRuntimeRun(runtimeRun, run, s.nowUTC())
		if !ok {
			continue
		}
		if err := s.ApplyEvent(ctx, event); err != nil {
			slog.WarnContext(ctx, "agent runtime reconciliation apply failed",
				"workspace_id", run.WorkspaceID,
				"run_id", run.ID,
				"runtime_run_id", runtimeRunID,
				"event_type", event.Type,
				"error", err,
			)
		}
		if isTerminalRuntimeEvent(event.Type) || !shouldReconcileRuntimeTranscriptFromSweep(run, runtimeRun) {
			continue
		}
		projectedRun, err := s.resolveRun(ctx, event)
		if err != nil {
			slog.WarnContext(ctx, "agent runtime reconciliation transcript run lookup failed",
				"workspace_id", run.WorkspaceID,
				"run_id", run.ID,
				"runtime_run_id", runtimeRunID,
				"event_type", event.Type,
				"error", err,
			)
			continue
		}
		if err := s.reconcileRuntimeTranscript(ctx, projectedRun, runtimeRunID); err != nil {
			slog.WarnContext(ctx, "agent runtime transcript reconciliation failed",
				"workspace_id", run.WorkspaceID,
				"run_id", run.ID,
				"runtime_run_id", runtimeRunID,
				"error", err,
			)
			continue
		}
		if markRuntimeTranscriptReconciled(projectedRun, runtimeRun) {
			if err := s.runRepo.UpdateOutputSummary(ctx, projectedRun.ID, projectedRun.OutputSummary); err != nil {
				slog.WarnContext(ctx, "agent runtime transcript reconciliation marker update failed",
					"workspace_id", run.WorkspaceID,
					"run_id", run.ID,
					"runtime_run_id", runtimeRunID,
					"error", err,
				)
			}
		}
	}
	return nil
}

func (s *AgentRuntimeProjectionService) subscribeNATS(js nats.JetStreamContext) (*nats.Subscription, error) {
	if err := ensureAgentRuntimeEventsStream(js); err != nil {
		return nil, err
	}
	subject := s.eventSubject()
	if info, err := js.ConsumerInfo(agentRuntimeEventsStreamName, agentRuntimeProjectionDurable); err == nil && info != nil {
		if strings.TrimSpace(info.Config.FilterSubject) != subject {
			if err := js.DeleteConsumer(agentRuntimeEventsStreamName, agentRuntimeProjectionDurable); err != nil {
				return nil, err
			}
		}
	}
	sub, err := js.PullSubscribe(
		subject,
		agentRuntimeProjectionDurable,
		nats.Bind(agentRuntimeEventsStreamName, agentRuntimeProjectionDurable),
	)
	if err == nil {
		return sub, nil
	}
	consumerCfg := &nats.ConsumerConfig{
		Durable:       agentRuntimeProjectionDurable,
		FilterSubject: subject,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       45 * time.Second,
		MaxDeliver:    5,
		BackOff: []time.Duration{
			5 * time.Second,
			15 * time.Second,
			45 * time.Second,
			2 * time.Minute,
			5 * time.Minute,
		},
		DeliverPolicy: nats.DeliverAllPolicy,
		MaxAckPending: 64,
	}
	if _, addErr := js.AddConsumer(agentRuntimeEventsStreamName, consumerCfg); addErr != nil {
		return nil, addErr
	}
	return js.PullSubscribe(
		subject,
		agentRuntimeProjectionDurable,
		nats.Bind(agentRuntimeEventsStreamName, agentRuntimeProjectionDurable),
	)
}

func ensureAgentRuntimeEventsStream(js nats.JetStreamContext) error {
	if js == nil {
		return fmt.Errorf("jetstream context is nil")
	}
	cfg := agentRuntimeEventsStreamConfig()
	if _, err := js.StreamInfo(cfg.Name); err != nil {
		if !errors.Is(err, nats.ErrStreamNotFound) {
			return err
		}
		_, err = js.AddStream(cfg)
		return err
	}
	// The agent runtime owns the stream configuration; when the stream
	// already exists, leave its config untouched.
	return nil
}

func agentRuntimeEventsStreamConfig() *nats.StreamConfig {
	return &nats.StreamConfig{
		Name:       agentRuntimeEventsStreamName,
		Subjects:   []string{agentRuntimeEventsSubjectAll},
		Storage:    nats.FileStorage,
		Retention:  nats.LimitsPolicy,
		Discard:    nats.DiscardOld,
		Duplicates: 2 * time.Minute,
		MaxAge:     7 * 24 * time.Hour,
		MaxBytes:   512 * 1024 * 1024,
	}
}

func (s *AgentRuntimeProjectionService) eventSubject() string {
	appID := strings.TrimSpace(s.appID)
	if appID == "" {
		appID = "helpin"
	}
	appID = strings.ReplaceAll(appID, ".", "_")
	appID = strings.ReplaceAll(appID, "*", "_")
	appID = strings.ReplaceAll(appID, ">", "_")
	return "agent-runtime.events." + appID + ".>"
}

func reconciliationEventForRuntimeRun(runtimeRun *AgentRuntimeRun, localRun model.AgentRun, fallback time.Time) (AgentRuntimeEventEnvelope, bool) {
	if runtimeRun == nil {
		return AgentRuntimeEventEnvelope{}, false
	}
	runID := strings.TrimSpace(runtimeRun.ID)
	if runID == "" {
		runID = strings.TrimSpace(derefString(localRun.ExternalRuntimeID))
	}
	if runID == "" {
		return AgentRuntimeEventEnvelope{}, false
	}
	hostRunID := strings.TrimSpace(runtimeRun.HostRunID)
	if hostRunID == "" {
		hostRunID = strings.TrimSpace(localRun.ID)
	}
	event := AgentRuntimeEventEnvelope{
		AppID:     strings.TrimSpace(runtimeRun.AppID),
		RunID:     runID,
		HostRunID: hostRunID,
		Data:      map[string]any{},
		SentAt:    runtimeRunEventTime(runtimeRun, fallback),
	}
	switch strings.TrimSpace(runtimeRun.Status) {
	case model.AgentRunStatusQueued:
		event.Type = agentruntime.EventRunQueued
	case model.AgentRunStatusRunning:
		event.Type = agentruntime.EventRunStarted
	case model.AgentRunStatusPaused:
		event.Type = agentruntime.EventRunPaused
		if strings.TrimSpace(runtimeRun.PauseReason) != "" {
			event.Data["pause_reason"] = strings.TrimSpace(runtimeRun.PauseReason)
		}
	case model.AgentRunStatusCompleted:
		event.Type = agentruntime.EventRunCompleted
	case model.AgentRunStatusFailed:
		event.Type = agentruntime.EventRunFailed
		if strings.TrimSpace(runtimeRun.ErrorMessage) != "" {
			event.Data["error"] = strings.TrimSpace(runtimeRun.ErrorMessage)
		}
	case model.AgentRunStatusCancelled:
		event.Type = agentruntime.EventRunCancelled
	default:
		return AgentRuntimeEventEnvelope{}, false
	}
	if usage, ok := usageFromRuntimeOutputSummary(runtimeRun.OutputSummary); ok {
		event.Data["usage"] = map[string]any{
			"total_tokens":            usage.TotalTokens,
			"input_tokens":            usage.InputTokens,
			"cached_input_tokens":     usage.CachedInputTokens,
			"output_tokens":           usage.OutputTokens,
			"reasoning_output_tokens": usage.ReasoningOutputTokens,
		}
		event.Data["usage_semantic"] = agentruntime.UsageSemanticCumulative
	}
	return event, true
}

func runtimeRunEventTime(runtimeRun *AgentRuntimeRun, fallback time.Time) time.Time {
	if runtimeRun != nil {
		if runtimeRun.CompletedAt != nil && !runtimeRun.CompletedAt.IsZero() {
			return runtimeRun.CompletedAt.UTC()
		}
		if runtimeRun.StartedAt != nil && !runtimeRun.StartedAt.IsZero() {
			return runtimeRun.StartedAt.UTC()
		}
		if !runtimeRun.UpdatedAt.IsZero() {
			return runtimeRun.UpdatedAt.UTC()
		}
	}
	if fallback.IsZero() {
		return time.Now().UTC()
	}
	return fallback.UTC()
}

func (s *AgentRuntimeProjectionService) processNATSMessage(ctx context.Context, msg *nats.Msg) {
	parsed, err := agentruntime.ParseEventEnvelope(msg.Data)
	if err != nil {
		slog.WarnContext(ctx, "agent runtime projection invalid payload", "subject", msg.Subject, "error", err)
		_ = msg.Ack()
		return
	}
	event := *parsed
	if err := s.ApplyEvent(ctx, event); err != nil {
		if errors.Is(err, errAgentRuntimeProjectionRunNotFound) {
			meta, _ := msg.Metadata()
			if meta != nil && meta.NumDelivered >= 5 {
				slog.ErrorContext(ctx, "agent runtime projection dropping event without mapped run",
					"runtime_run_id", event.RunID,
					"host_run_id", event.HostRunID,
					"event_type", event.Type,
				)
				_ = msg.Ack()
				return
			}
			slog.WarnContext(ctx, "agent runtime projection waiting for mapped run",
				"runtime_run_id", event.RunID,
				"host_run_id", event.HostRunID,
				"event_type", event.Type,
			)
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		slog.ErrorContext(ctx, "agent runtime projection failed",
			"runtime_run_id", event.RunID,
			"host_run_id", event.HostRunID,
			"event_type", event.Type,
			"error", err,
		)
		_ = msg.NakWithDelay(5 * time.Second)
		return
	}
	_ = msg.Ack()
}

func (s *AgentRuntimeProjectionService) ApplyEvent(ctx context.Context, event AgentRuntimeEventEnvelope) error {
	if s == nil || s.runRepo == nil {
		return fmt.Errorf("agent runtime projection service is not configured")
	}
	run, err := s.resolveRun(ctx, event)
	if err != nil {
		return err
	}
	// Prior persisted status, captured before the event is applied: product
	// finalizers fire only on the transition into a terminal status, so
	// redelivered or reconciled terminal events on an already-terminal run
	// are no-ops.
	wasTerminal := isTerminalAgentRunStatus(run.Status)
	changed := false
	runtimeName := agentRuntimeName
	if run.ExternalRuntime == nil || strings.TrimSpace(*run.ExternalRuntime) != runtimeName {
		run.ExternalRuntime = &runtimeName
		changed = true
	}
	if strings.TrimSpace(event.RunID) != "" && (run.ExternalRuntimeID == nil || strings.TrimSpace(*run.ExternalRuntimeID) != strings.TrimSpace(event.RunID)) {
		runtimeRunID := strings.TrimSpace(event.RunID)
		run.ExternalRuntimeID = &runtimeRunID
		changed = true
	}

	now := s.eventTime(event)
	suppressLifecycle := isTerminalAgentRunStatus(run.Status) && isPreTerminalRuntimeEvent(event.Type)
	switch strings.TrimSpace(event.Type) {
	case agentruntime.EventRunQueued:
		if !suppressLifecycle {
			changed = setRunStatus(run, model.AgentRunStatusQueued, model.AgentRunPauseReasonNone) || changed
		}
	case agentruntime.EventRunStarted:
		if !suppressLifecycle {
			if run.StartedAt == nil {
				run.StartedAt = &now
				changed = true
			}
			run.CompletedAt = nil
			changed = setRunStatus(run, model.AgentRunStatusRunning, model.AgentRunPauseReasonNone) || changed
		}
	case agentruntime.EventRunResumed:
		if !suppressLifecycle {
			run.CompletedAt = nil
			changed = setRunStatus(run, model.AgentRunStatusRunning, model.AgentRunPauseReasonNone) || changed
		}
	case agentruntime.EventRunPaused:
		if !suppressLifecycle {
			pauseReason := normalizeRuntimePauseReason(eventDataString(event.Data, "pause_reason"))
			changed = setRunStatus(run, model.AgentRunStatusPaused, pauseReason) || changed
		}
	case agentruntime.EventRunCompleted:
		if run.CompletedAt == nil {
			run.CompletedAt = &now
			changed = true
		}
		changed = setRunStatus(run, model.AgentRunStatusCompleted, model.AgentRunPauseReasonNone) || changed
	case agentruntime.EventRunFailed:
		if run.CompletedAt == nil {
			run.CompletedAt = &now
			changed = true
		}
		if message := firstNonEmptyString(eventDataString(event.Data, "error"), eventDataString(event.Data, "error_message")); message != "" {
			if run.ErrorMessage == nil || *run.ErrorMessage != message {
				run.ErrorMessage = &message
				changed = true
			}
		}
		changed = setRunStatus(run, model.AgentRunStatusFailed, model.AgentRunPauseReasonNone) || changed
	case agentruntime.EventRunCancelled:
		if run.CompletedAt == nil {
			run.CompletedAt = &now
			changed = true
		}
		changed = setRunStatus(run, model.AgentRunStatusCancelled, model.AgentRunPauseReasonNone) || changed
	case agentruntime.EventUsageCheckpoint:
	case agentruntime.EventAssistantMessageCompleted:
		if err := s.mirrorAssistantMessageCompleted(ctx, run, event); err != nil {
			return err
		}
	case agentruntime.EventToolCallStarted, agentruntime.EventToolCallResult, agentruntime.EventToolCallFinished:
		if err := s.mirrorRuntimeEventArtifact(ctx, run, event, model.AgentRunArtifactTypeToolCall); err != nil {
			return err
		}
	case agentruntime.EventToolCallArgsDelta:
		return nil
	case agentruntime.EventPlanUpdated:
		if err := s.mirrorRuntimePlanUpdated(ctx, run, event); err != nil {
			return err
		}
	case agentRuntimeEventCodexAuthStateChanged:
		authChanged, err := s.applyCodexAuthStateChanged(ctx, run, event)
		if err != nil {
			return err
		}
		changed = authChanged || changed
	default:
		return nil
	}

	if isTerminalRuntimeEvent(event.Type) {
		if runtimeRunID := strings.TrimSpace(derefString(run.ExternalRuntimeID)); runtimeRunID != "" {
			if err := s.reconcileRuntimeTranscript(ctx, run, runtimeRunID); err != nil {
				return err
			}
		}
	}
	if usage, ok := eventUsage(event); ok {
		if applyRuntimeUsage(run, usage) {
			changed = true
		}
		terminalUsageChanged, err := s.maybeConsumeTerminalUsage(ctx, run, event, usage)
		if err != nil {
			slog.ErrorContext(ctx, "agent runtime terminal usage consumption failed",
				"error", err,
				"workspace_id", run.WorkspaceID,
				"run_id", run.ID,
				"runtime_run_id", strings.TrimSpace(derefString(run.ExternalRuntimeID)),
				"event_type", event.Type,
			)
			terminalUsageChanged = false
		}
		if terminalUsageChanged {
			changed = true
		}
		overageChanged, err := s.maybeCancelOverage(ctx, run, event, usage)
		if err != nil {
			return err
		}
		if overageChanged {
			changed = true
		}
	}
	if s.runFinalizers != nil && isTerminalRuntimeEvent(event.Type) && !wasTerminal && isTerminalAgentRunStatus(run.Status) {
		// Dispatch before the final Update persists the terminal status: a
		// crash mid-dispatch leaves the row non-terminal, so the redelivered
		// event recomputes the transition and per-finalizer idempotency
		// markers skip the side effects that already fired. Finalizer errors
		// are logged inside the dispatch and never block status projection.
		runtimeSummaryAvailable := s.mergeRuntimeOutputSummaryForFinalizers(ctx, run)
		s.runFinalizers.FinalizeTerminalRun(ctx, run, runtimeSummaryAvailable)
	}
	if !changed {
		return nil
	}
	model.NormalizeAgentRunPauseState(run)
	if err := s.runRepo.Update(ctx, run); err != nil {
		return err
	}
	s.runRepo.Notify(ctx, run)
	return nil
}

// mergeRuntimeOutputSummaryForFinalizers fetches the terminal runtime run and
// merges its adapter-owned OutputSummary (the delegated summary contract,
// e.g. draft_reply) into the local run summary. Host-reserved keys (prefix
// "agent_runtime_") always win. Returns false when the runtime summary could
// not be fetched so summary-dependent finalizers are skipped without markers
// and a later duplicate terminal event can retry them.
func (s *AgentRuntimeProjectionService) mergeRuntimeOutputSummaryForFinalizers(ctx context.Context, run *model.AgentRun) bool {
	if s == nil || run == nil {
		return false
	}
	if s.agentRuntimeClient == nil {
		// Without a runtime client the local summary is all we have; treat
		// it as authoritative rather than blocking every finalizer.
		return true
	}
	runtimeRunID := strings.TrimSpace(derefString(run.ExternalRuntimeID))
	if runtimeRunID == "" {
		return true
	}
	runtimeRun, err := s.agentRuntimeClient.GetRun(ctx, runtimeRunID)
	if err != nil {
		slog.ErrorContext(ctx, "agent runtime terminal output summary fetch failed",
			"error", err,
			"workspace_id", run.WorkspaceID,
			"run_id", run.ID,
			"runtime_run_id", runtimeRunID,
		)
		return false
	}
	if runtimeRun == nil {
		return true
	}
	run.OutputSummary = mergeRuntimeOutputSummaryPayload(run.OutputSummary, runtimeRun.OutputSummary)
	return true
}

// mergeRuntimeOutputSummaryPayload overlays runtime summary keys onto the
// local summary, preserving host-reserved marker keys.
func mergeRuntimeOutputSummaryPayload(local, runtime json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(runtime))
	if trimmed == "" || trimmed == "null" {
		return local
	}
	runtimeBody := map[string]any{}
	if err := json.Unmarshal(runtime, &runtimeBody); err != nil {
		return local
	}
	localBody := map[string]any{}
	if len(local) > 0 {
		_ = json.Unmarshal(local, &localBody)
	}
	for key, value := range runtimeBody {
		if strings.HasPrefix(key, "agent_runtime_") {
			continue
		}
		localBody[key] = value
	}
	payload, err := json.Marshal(localBody)
	if err != nil {
		return local
	}
	return payload
}

func (s *AgentRuntimeProjectionService) maybeCancelOverage(ctx context.Context, run *model.AgentRun, event AgentRuntimeEventEnvelope, usage agentRuntimeUsagePayload) (bool, error) {
	if s == nil || run == nil || s.usageMeter == nil || s.agentRepo == nil || s.agentRuntimeClient == nil {
		return false, nil
	}
	if strings.TrimSpace(event.Type) != agentruntime.EventUsageCheckpoint {
		return false, nil
	}
	if runtimeUsageSemantic(event) != agentruntime.UsageSemanticCumulative {
		return false, nil
	}
	if isTerminalAgentRunStatus(run.Status) || strings.TrimSpace(derefString(run.ExecutionStage)) == agentRuntimeExecutionStageUsageOverageCancel {
		return false, nil
	}
	runtimeRunID, ok := agentRuntimeRunID(run)
	if !ok {
		return false, nil
	}
	agent, err := s.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil {
		return false, err
	}
	err = s.usageMeter.PreflightUsage(ctx, AIUsageMeterInput{
		WorkspaceID:       run.WorkspaceID,
		FeatureKey:        AgentRunAIUsageFeature(agent),
		InputTokens:       usage.InputTokens,
		OutputTokens:      usage.OutputTokens,
		ReasoningTokens:   usage.ReasoningOutputTokens,
		CachedInputTokens: usage.CachedInputTokens,
		Metadata: map[string]interface{}{
			"run_id":         run.ID,
			"runtime_run_id": runtimeRunID,
			"agent_id":       run.AgentID,
			"checkpoint":     true,
		},
	})
	if err == nil {
		return false, nil
	}
	if !isAIUsageCreditLimitError(err) {
		return false, err
	}
	if _, cancelErr := s.agentRuntimeClient.CancelRun(ctx, runtimeRunID); cancelErr != nil {
		return false, fmt.Errorf("cancel over-budget agent runtime run: %w", cancelErr)
	}
	run.ExecutionStage = strPtr(agentRuntimeExecutionStageUsageOverageCancel)
	message := agentRuntimeUsageOverageCancellationErrorText
	run.ErrorMessage = &message
	return true, nil
}

func (s *AgentRuntimeProjectionService) maybeConsumeTerminalUsage(ctx context.Context, run *model.AgentRun, event AgentRuntimeEventEnvelope, usage agentRuntimeUsagePayload) (bool, error) {
	if s == nil || run == nil || s.usageMeter == nil || s.agentRepo == nil {
		return false, nil
	}
	if !isTerminalRuntimeEvent(event.Type) {
		return false, nil
	}
	semantic := runtimeUsageSemantic(event)
	if semantic != "" && semantic != agentruntime.UsageSemanticCumulative {
		return false, nil
	}
	if runtimeUsageAlreadyConsumed(run.OutputSummary) {
		return false, nil
	}
	agent, err := s.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil {
		return false, err
	}
	runtimeRunID := strings.TrimSpace(derefString(run.ExternalRuntimeID))
	_, err = s.usageMeter.Consume(ctx, AIUsageMeterInput{
		WorkspaceID:       run.WorkspaceID,
		FeatureKey:        AgentRunAIUsageFeature(agent),
		IdempotencyKey:    aiUsageIdempotencyKey(run.WorkspaceID, "agent-runtime", run.ID, "terminal-usage"),
		InputTokens:       usage.InputTokens,
		OutputTokens:      usage.OutputTokens,
		ReasoningTokens:   usage.ReasoningOutputTokens,
		CachedInputTokens: usage.CachedInputTokens,
		AllowOverage:      true,
		Metadata: map[string]interface{}{
			"run_id":         run.ID,
			"runtime_run_id": runtimeRunID,
			"agent_id":       run.AgentID,
			"terminal_event": strings.TrimSpace(event.Type),
			"delegated":      true,
		},
	})
	if err != nil {
		return false, err
	}
	run.OutputSummary = markRuntimeUsageConsumed(run.OutputSummary, s.eventTime(event))
	return true, nil
}

func (s *AgentRuntimeProjectionService) mirrorAssistantMessageCompleted(ctx context.Context, run *model.AgentRun, event AgentRuntimeEventEnvelope) error {
	if s == nil || s.runMessageRepo == nil || run == nil {
		return nil
	}
	data, ok, err := event.AssistantMessage()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	runtimeMessageID := strings.TrimSpace(data.MessageID)
	if runtimeMessageID == "" {
		return nil
	}
	content := firstNonEmptyString(data.Content, data.Text)
	if content == "" {
		return nil
	}
	existing, err := s.runMessageRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	for _, message := range existing {
		if agentRunMessageHasRuntimeMessageID(message, runtimeMessageID) {
			return nil
		}
	}
	sequenceNo, err := s.runMessageRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	contentBlocks := annotateRuntimeMessageBlocks(nil, runtimeMessageID, content)
	message := &model.AgentRunMessage{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		Role:          "assistant",
		Content:       content,
		MessageType:   "assistant_turn",
		ContentBlocks: contentBlocks,
		SequenceNo:    sequenceNo,
	}
	if err := s.runMessageRepo.Create(ctx, message); err != nil {
		return err
	}
	s.runRepo.Notify(ctx, run)
	return nil
}

func (s *AgentRuntimeProjectionService) mirrorRuntimePlanUpdated(ctx context.Context, run *model.AgentRun, event AgentRuntimeEventEnvelope) error {
	content := firstNonEmptyString(eventDataString(event.Data, "content"), eventDataJSON(event.Data))
	if content == "" {
		return nil
	}
	return s.createRuntimeArtifact(ctx, run, AgentRuntimeArtifact{
		ID:            runtimeEventIdentity(event),
		ArtifactType:  model.AgentRunArtifactTypeRunPlan,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: content,
		Metadata:      agentRuntimeProjectionMustJSON(map[string]any{"source": "agent-runtime-event", "runtime_event_type": event.Type}),
		CreatedAt:     s.eventTime(event),
	})
}

func (s *AgentRuntimeProjectionService) mirrorRuntimeEventArtifact(ctx context.Context, run *model.AgentRun, event AgentRuntimeEventEnvelope, artifactType string) error {
	toolCallID := eventDataString(event.Data, "tool_call_id")
	if data, ok, err := event.ToolCall(); err != nil {
		return err
	} else if ok {
		toolCallID = firstNonEmptyString(data.ToolCallID, toolCallID)
	}
	return s.createRuntimeArtifact(ctx, run, AgentRuntimeArtifact{
		ID:            runtimeEventIdentity(event),
		ArtifactType:  artifactType,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: eventDataJSON(event.Data),
		Metadata: agentRuntimeProjectionMustJSON(map[string]any{
			"source":             "agent-runtime-event",
			"runtime_event_type": event.Type,
			"tool_call_id":       toolCallID,
		}),
		CreatedAt: s.eventTime(event),
	})
}

func (s *AgentRuntimeProjectionService) applyCodexAuthStateChanged(ctx context.Context, run *model.AgentRun, event AgentRuntimeEventEnvelope) (bool, error) {
	if err := s.createRuntimeArtifact(ctx, run, AgentRuntimeArtifact{
		ID:            runtimeEventIdentity(event),
		ArtifactType:  model.AgentRunArtifactTypeCodexAuthState,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: eventDataJSON(event.Data),
		Metadata: agentRuntimeProjectionMustJSON(map[string]any{
			"source":             "agent-runtime-event",
			"runtime_event_type": event.Type,
		}),
		CreatedAt: s.eventTime(event),
	}); err != nil {
		return false, err
	}

	state := strings.TrimSpace(eventDataString(event.Data, "state"))
	switch state {
	case model.CodexAuthStateRequired, model.CodexAuthStatePending:
		if run.ExecutionStage == nil || strings.TrimSpace(*run.ExecutionStage) != agentRuntimeExecutionStageAwaitingAuth {
			run.ExecutionStage = strPtr(agentRuntimeExecutionStageAwaitingAuth)
			return true, nil
		}
		return false, nil
	case model.CodexAuthStateConnected:
		if run.ExecutionStage != nil && strings.TrimSpace(*run.ExecutionStage) == agentRuntimeExecutionStageAuthCompleted {
			return false, nil
		}
		runtimeRunID := strings.TrimSpace(event.RunID)
		if runtimeRunID == "" {
			runtimeRunID = strings.TrimSpace(derefString(run.ExternalRuntimeID))
		}
		if runtimeRunID == "" {
			return false, fmt.Errorf("agent runtime auth connected event missing run id")
		}
		if s.agentRuntimeClient == nil {
			return false, fmt.Errorf("agent runtime client is not configured")
		}
		if _, err := s.agentRuntimeClient.ResumeRun(ctx, runtimeRunID, AgentRuntimeResumeRunRequest{
			Intent: model.AgentRunResumeIntentAuthCompleted,
		}); err != nil {
			return false, err
		}
		run.ExecutionStage = strPtr(agentRuntimeExecutionStageAuthCompleted)
		return true, nil
	default:
		return false, nil
	}
}

func (s *AgentRuntimeProjectionService) reconcileRuntimeTranscript(ctx context.Context, run *model.AgentRun, runtimeRunID string) error {
	if s == nil || run == nil || s.agentRuntimeClient == nil {
		return nil
	}
	if s.runMessageRepo != nil {
		messages, err := s.agentRuntimeClient.ListMessages(ctx, runtimeRunID)
		if err != nil {
			return fmt.Errorf("list runtime messages: %w", err)
		}
		for _, message := range messages {
			if err := s.createRuntimeMessage(ctx, run, message); err != nil {
				return err
			}
		}
	}
	if s.artifactRepo != nil {
		artifacts, err := s.agentRuntimeClient.ListArtifacts(ctx, runtimeRunID)
		if err != nil {
			return fmt.Errorf("list runtime artifacts: %w", err)
		}
		for _, artifact := range artifacts {
			if err := s.createRuntimeArtifact(ctx, run, artifact); err != nil {
				return err
			}
		}
	}
	if s.interactionRepo != nil {
		interactions, err := s.agentRuntimeClient.ListInteractions(ctx, runtimeRunID)
		if err != nil {
			return fmt.Errorf("list runtime interactions: %w", err)
		}
		for _, interaction := range interactions {
			if err := s.upsertRuntimeInteraction(ctx, run, interaction); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *AgentRuntimeProjectionService) createRuntimeMessage(ctx context.Context, run *model.AgentRun, runtimeMessage AgentRuntimeMessage) error {
	if s == nil || s.runMessageRepo == nil || run == nil {
		return nil
	}
	runtimeMessageID := runtimeMessageIdentity(runtimeMessage)
	if runtimeMessageID == "" {
		return nil
	}
	existing, err := s.runMessageRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	for _, message := range existing {
		if agentRunMessageHasRuntimeMessageID(message, runtimeMessageID) || agentRunMessageMatchesRuntimeMessage(message, runtimeMessage, runtimeMessageID) {
			return nil
		}
	}
	sequenceNo, err := s.runMessageRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	messageType := strings.TrimSpace(runtimeMessage.MessageType)
	if messageType == "" {
		messageType = "message"
	}
	role := strings.TrimSpace(runtimeMessage.Role)
	if role == "" {
		role = "assistant"
	}
	message := &model.AgentRunMessage{
		WorkspaceID:     run.WorkspaceID,
		RunID:           run.ID,
		Role:            role,
		Content:         strings.TrimSpace(runtimeMessage.Content),
		MessageType:     messageType,
		ContentBlocks:   annotateRuntimeMessageBlocks(runtimeMessage.ContentBlocks, runtimeMessageID, runtimeMessage.Content),
		ToolInvocations: runtimeMessage.ToolInvocations,
		SequenceNo:      sequenceNo,
	}
	if !runtimeMessage.CreatedAt.IsZero() {
		message.CreatedAt = runtimeMessage.CreatedAt.UTC()
	}
	if err := s.runMessageRepo.Create(ctx, message); err != nil {
		return err
	}
	s.runRepo.Notify(ctx, run)
	return nil
}

func (s *AgentRuntimeProjectionService) createRuntimeArtifact(ctx context.Context, run *model.AgentRun, runtimeArtifact AgentRuntimeArtifact) error {
	if s == nil || s.artifactRepo == nil || run == nil {
		return nil
	}
	runtimeArtifactID := strings.TrimSpace(runtimeArtifact.ID)
	if runtimeArtifactID == "" {
		return nil
	}
	existing, err := s.artifactRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	for _, artifact := range existing {
		if agentRunArtifactHasRuntimeArtifactID(artifact, runtimeArtifactID) {
			return nil
		}
	}
	sequenceNo, err := s.artifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	artifactType := strings.TrimSpace(runtimeArtifact.ArtifactType)
	if artifactType == "" {
		artifactType = "runtime_artifact"
	}
	format := strings.TrimSpace(runtimeArtifact.Format)
	if format == "" {
		format = "json"
	}
	storageMode := strings.TrimSpace(runtimeArtifact.StorageMode)
	if storageMode == "" {
		storageMode = "inline"
	}
	metadata := mergeRuntimeMetadata(runtimeArtifact.Metadata, map[string]any{
		"source":                "agent-runtime",
		"runtime_artifact_id":   runtimeArtifactID,
		"runtime_run_id":        strings.TrimSpace(derefString(run.ExternalRuntimeID)),
		"runtime_sequence_no":   runtimeArtifact.SequenceNo,
		"runtime_artifact_type": artifactType,
	})
	content := strings.TrimSpace(runtimeArtifact.InlineContent)
	artifact := &model.AgentRunArtifact{
		WorkspaceID:  run.WorkspaceID,
		RunID:        run.ID,
		ArtifactType: artifactType,
		Format:       format,
		StorageMode:  storageMode,
		Metadata:     metadata,
		SequenceNo:   sequenceNo,
	}
	if content != "" {
		artifact.InlineContent = &content
	}
	if !runtimeArtifact.CreatedAt.IsZero() {
		artifact.CreatedAt = runtimeArtifact.CreatedAt.UTC()
	}
	if err := s.artifactRepo.Create(ctx, artifact); err != nil {
		return err
	}
	s.runRepo.Notify(ctx, run)
	return nil
}

func (s *AgentRuntimeProjectionService) upsertRuntimeInteraction(ctx context.Context, run *model.AgentRun, runtimeInteraction AgentRuntimeInteraction) error {
	if s == nil || s.interactionRepo == nil || run == nil {
		return nil
	}
	runtimeInteractionID := strings.TrimSpace(runtimeInteraction.ID)
	if runtimeInteractionID == "" {
		return nil
	}
	existing, err := s.interactionRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	for index := range existing {
		if !agentRunInteractionHasRuntimeInteractionID(existing[index], runtimeInteractionID) {
			continue
		}
		updated := existing[index]
		if applyRuntimeInteraction(&updated, runtimeInteraction, run) {
			return s.interactionRepo.Update(ctx, &updated)
		}
		return nil
	}
	interaction := &model.AgentRunInteraction{
		WorkspaceID:          run.WorkspaceID,
		RunID:                run.ID,
		RuntimeKind:          firstNonEmptyString(strings.TrimSpace(runtimeInteraction.RuntimeKind), strings.TrimSpace(run.RuntimeKind)),
		InteractionKind:      normalizeRuntimeInteractionKind(runtimeInteraction.InteractionKind),
		Status:               normalizeRuntimeInteractionStatus(runtimeInteraction.Status),
		RequestSchemaVersion: model.AgentRunInteractionSchemaVersionHelpinV1,
		RequestID:            strPtr(runtimeInteractionID),
		Title:                stringPtrIfNotEmpty(runtimeInteraction.Title),
		Summary:              stringPtrIfNotEmpty(runtimeInteraction.Summary),
		RequestPayload:       jsonOrEmptyObject(runtimeInteraction.RequestPayload),
		ResponsePayload:      jsonOrNil(runtimeInteraction.ResponsePayload),
		RuntimeMetadata: mergeRuntimeMetadata(nil, map[string]any{
			"source":                 "agent-runtime",
			"runtime_interaction_id": runtimeInteractionID,
			"runtime_run_id":         strings.TrimSpace(derefString(run.ExternalRuntimeID)),
		}),
		ResolvedBy: nil,
		ResolvedAt: runtimeInteraction.ResolvedAt,
	}
	if strings.TrimSpace(runtimeInteraction.ResolvedByExternalID) != "" {
		interaction.RuntimeMetadata = mergeRuntimeMetadata(interaction.RuntimeMetadata, map[string]any{
			"resolved_by_external_id": strings.TrimSpace(runtimeInteraction.ResolvedByExternalID),
		})
	}
	if !runtimeInteraction.CreatedAt.IsZero() {
		interaction.CreatedAt = runtimeInteraction.CreatedAt.UTC()
	}
	if !runtimeInteraction.UpdatedAt.IsZero() {
		interaction.UpdatedAt = runtimeInteraction.UpdatedAt.UTC()
	}
	if err := s.interactionRepo.Create(ctx, interaction); err != nil {
		return err
	}
	s.runRepo.Notify(ctx, run)
	return nil
}

func agentRunMessageHasRuntimeMessageID(message model.AgentRunMessage, runtimeMessageID string) bool {
	runtimeMessageID = strings.TrimSpace(runtimeMessageID)
	if runtimeMessageID == "" {
		return false
	}
	return jsonRawContainsStringField(message.ContentBlocks, "runtime_message_id", runtimeMessageID)
}

func agentRunMessageMatchesRuntimeMessage(message model.AgentRunMessage, runtimeMessage AgentRuntimeMessage, runtimeMessageID string) bool {
	if strings.TrimSpace(runtimeMessage.ID) != "" && strings.TrimSpace(runtimeMessage.ID) != strings.TrimSpace(runtimeMessageID) && agentRunMessageHasRuntimeMessageID(message, runtimeMessage.ID) {
		return true
	}
	role := strings.TrimSpace(runtimeMessage.Role)
	if role == "" {
		role = "assistant"
	}
	if strings.TrimSpace(message.Role) != role {
		return false
	}
	if strings.TrimSpace(message.Content) != strings.TrimSpace(runtimeMessage.Content) {
		return false
	}
	messageType := strings.TrimSpace(runtimeMessage.MessageType)
	if messageType == "" {
		messageType = "message"
	}
	localMessageType := strings.TrimSpace(message.MessageType)
	if localMessageType == messageType {
		return true
	}
	if role == "assistant" && localMessageType == "assistant_turn" {
		return true
	}
	return role == "user" && messageType == "message" && isLocalRuntimeResumeMessageType(localMessageType)
}

func isLocalRuntimeResumeMessageType(messageType string) bool {
	switch strings.TrimSpace(messageType) {
	case "user_reply", "approval", "request_changes":
		return true
	default:
		return false
	}
}

func runtimeMessageIdentity(runtimeMessage AgentRuntimeMessage) string {
	if id := strings.TrimSpace(runtimeMessage.RuntimeMessageID); id != "" {
		return id
	}
	return strings.TrimSpace(runtimeMessage.ID)
}

func agentRunArtifactHasRuntimeArtifactID(artifact model.AgentRunArtifact, runtimeArtifactID string) bool {
	return jsonRawContainsStringField(artifact.Metadata, "runtime_artifact_id", runtimeArtifactID)
}

func agentRunInteractionHasRuntimeInteractionID(interaction model.AgentRunInteraction, runtimeInteractionID string) bool {
	runtimeInteractionID = strings.TrimSpace(runtimeInteractionID)
	if runtimeInteractionID == "" {
		return false
	}
	if interaction.RequestID != nil && strings.TrimSpace(*interaction.RequestID) == runtimeInteractionID {
		return true
	}
	return jsonRawContainsStringField(interaction.RuntimeMetadata, "runtime_interaction_id", runtimeInteractionID)
}

func applyRuntimeInteraction(interaction *model.AgentRunInteraction, runtimeInteraction AgentRuntimeInteraction, run *model.AgentRun) bool {
	if interaction == nil {
		return false
	}
	changed := false
	if value := firstNonEmptyString(strings.TrimSpace(runtimeInteraction.RuntimeKind), strings.TrimSpace(run.RuntimeKind)); strings.TrimSpace(interaction.RuntimeKind) != value {
		interaction.RuntimeKind = value
		changed = true
	}
	if value := normalizeRuntimeInteractionKind(runtimeInteraction.InteractionKind); strings.TrimSpace(interaction.InteractionKind) != value {
		interaction.InteractionKind = value
		changed = true
	}
	if value := normalizeRuntimeInteractionStatus(runtimeInteraction.Status); strings.TrimSpace(interaction.Status) != value {
		interaction.Status = value
		changed = true
	}
	if value := stringPtrIfNotEmpty(runtimeInteraction.Title); !stringPointersEqual(interaction.Title, value) {
		interaction.Title = value
		changed = true
	}
	if value := stringPtrIfNotEmpty(runtimeInteraction.Summary); !stringPointersEqual(interaction.Summary, value) {
		interaction.Summary = value
		changed = true
	}
	if raw := jsonOrEmptyObject(runtimeInteraction.RequestPayload); !agentRuntimeProjectionJSONRawEqual(interaction.RequestPayload, raw) {
		interaction.RequestPayload = raw
		changed = true
	}
	if raw := jsonOrNil(runtimeInteraction.ResponsePayload); !agentRuntimeProjectionJSONRawEqual(interaction.ResponsePayload, raw) {
		interaction.ResponsePayload = raw
		changed = true
	}
	if !timePointersEqual(interaction.ResolvedAt, runtimeInteraction.ResolvedAt) {
		interaction.ResolvedAt = runtimeInteraction.ResolvedAt
		changed = true
	}
	metadata := mergeRuntimeMetadata(interaction.RuntimeMetadata, map[string]any{
		"source":                 "agent-runtime",
		"runtime_interaction_id": strings.TrimSpace(runtimeInteraction.ID),
		"runtime_run_id":         strings.TrimSpace(derefString(run.ExternalRuntimeID)),
	})
	if strings.TrimSpace(runtimeInteraction.ResolvedByExternalID) != "" {
		metadata = mergeRuntimeMetadata(metadata, map[string]any{
			"resolved_by_external_id": strings.TrimSpace(runtimeInteraction.ResolvedByExternalID),
		})
	}
	if !agentRuntimeProjectionJSONRawEqual(interaction.RuntimeMetadata, metadata) {
		interaction.RuntimeMetadata = metadata
		changed = true
	}
	return changed
}

func annotateRuntimeMessageBlocks(raw json.RawMessage, runtimeMessageID string, content string) json.RawMessage {
	runtimeMessageID = strings.TrimSpace(runtimeMessageID)
	var blocks []map[string]any
	if len(raw) > 0 && json.Unmarshal(raw, &blocks) == nil && len(blocks) > 0 {
		blocks[0]["runtime_message_id"] = runtimeMessageID
		blocks[0]["source"] = "agent-runtime"
		payload, _ := json.Marshal(blocks)
		return payload
	}
	block := map[string]any{
		"type":               "text",
		"text":               strings.TrimSpace(content),
		"runtime_message_id": runtimeMessageID,
		"source":             "agent-runtime",
	}
	payload, _ := json.Marshal([]map[string]any{block})
	return payload
}

func normalizeRuntimeInteractionKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "human_input", model.AgentRunInteractionKindRequestUserInput:
		return model.AgentRunInteractionKindRequestUserInput
	case "human_approval", model.AgentRunInteractionKindApprovalRequest:
		return model.AgentRunInteractionKindApprovalRequest
	case "authentication", model.AgentRunInteractionKindAuthRequired:
		return model.AgentRunInteractionKindAuthRequired
	case model.AgentRunInteractionKindCommandExecutionApproval,
		model.AgentRunInteractionKindFileChangeApproval,
		model.AgentRunInteractionKindPermissionsApproval,
		model.AgentRunInteractionKindReviewCheckpoint:
		return strings.TrimSpace(kind)
	default:
		if strings.TrimSpace(kind) == "" {
			return model.AgentRunInteractionKindRequestUserInput
		}
		return strings.TrimSpace(kind)
	}
}

func normalizeRuntimeInteractionStatus(status string) string {
	switch strings.TrimSpace(status) {
	case model.AgentRunInteractionStatusResolved:
		return model.AgentRunInteractionStatusResolved
	case model.AgentRunInteractionStatusCancelled:
		return model.AgentRunInteractionStatusCancelled
	default:
		return model.AgentRunInteractionStatusPending
	}
}

func jsonRawContainsStringField(raw json.RawMessage, key, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" || len(raw) == 0 {
		return false
	}
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err == nil {
		for _, block := range blocks {
			if strings.TrimSpace(fmt.Sprint(block[key])) == expected {
				return true
			}
		}
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err == nil {
		if strings.TrimSpace(fmt.Sprint(body[key])) == expected {
			return true
		}
	}
	return false
}

func runtimeUsageAlreadyConsumed(summary json.RawMessage) bool {
	if len(summary) == 0 {
		return false
	}
	var body map[string]any
	if err := json.Unmarshal(summary, &body); err != nil {
		return false
	}
	value, _ := body[agentRuntimeUsageConsumedSummaryKey].(bool)
	return value
}

func shouldReconcileRuntimeTranscriptFromSweep(localRun model.AgentRun, runtimeRun *AgentRuntimeRun) bool {
	version := runtimeTranscriptVersion(runtimeRun)
	if version.IsZero() {
		return false
	}
	previous := runtimeTranscriptReconciledVersion(localRun.OutputSummary)
	return previous.IsZero() || version.After(previous)
}

func markRuntimeTranscriptReconciled(run *model.AgentRun, runtimeRun *AgentRuntimeRun) bool {
	if run == nil {
		return false
	}
	version := runtimeTranscriptVersion(runtimeRun)
	if version.IsZero() {
		return false
	}
	if previous := runtimeTranscriptReconciledVersion(run.OutputSummary); !previous.IsZero() && !version.After(previous) {
		return false
	}
	body := map[string]any{}
	if len(run.OutputSummary) > 0 {
		_ = json.Unmarshal(run.OutputSummary, &body)
	}
	body[agentRuntimeTranscriptReconciledVersionKey] = version.UTC().Format(time.RFC3339Nano)
	payload, err := json.Marshal(body)
	if err != nil {
		return false
	}
	run.OutputSummary = payload
	return true
}

func runtimeTranscriptVersion(runtimeRun *AgentRuntimeRun) time.Time {
	if runtimeRun == nil {
		return time.Time{}
	}
	if !runtimeRun.UpdatedAt.IsZero() {
		return runtimeRun.UpdatedAt.UTC()
	}
	if runtimeRun.CompletedAt != nil && !runtimeRun.CompletedAt.IsZero() {
		return runtimeRun.CompletedAt.UTC()
	}
	if runtimeRun.StartedAt != nil && !runtimeRun.StartedAt.IsZero() {
		return runtimeRun.StartedAt.UTC()
	}
	return time.Time{}
}

func runtimeTranscriptReconciledVersion(summary json.RawMessage) time.Time {
	if len(summary) == 0 {
		return time.Time{}
	}
	var body map[string]any
	if err := json.Unmarshal(summary, &body); err != nil {
		return time.Time{}
	}
	raw := strings.TrimSpace(fmt.Sprint(body[agentRuntimeTranscriptReconciledVersionKey]))
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func markRuntimeUsageConsumed(summary json.RawMessage, consumedAt time.Time) json.RawMessage {
	body := map[string]any{}
	if len(summary) > 0 {
		_ = json.Unmarshal(summary, &body)
	}
	body[agentRuntimeUsageConsumedSummaryKey] = true
	if !consumedAt.IsZero() {
		body[agentRuntimeUsageConsumedAtSummaryKey] = consumedAt.UTC().Format(time.RFC3339Nano)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return summary
	}
	return payload
}

func mergeRuntimeMetadata(raw json.RawMessage, values map[string]any) json.RawMessage {
	body := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	for key, value := range values {
		if strings.TrimSpace(fmt.Sprint(value)) == "" {
			continue
		}
		body[key] = value
	}
	payload, _ := json.Marshal(body)
	return payload
}

func eventDataJSON(data map[string]any) string {
	if len(data) == 0 {
		return "{}"
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func agentRuntimeProjectionMustJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func runtimeEventIdentity(event AgentRuntimeEventEnvelope) string {
	if strings.TrimSpace(event.EventID) != "" {
		return strings.TrimSpace(event.EventID)
	}
	return strings.TrimSpace(event.Type) + ":" + strings.TrimSpace(event.RunID) + ":" + eventDataString(event.Data, "tool_call_id")
}

func jsonOrEmptyObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), raw...)
}

func jsonOrNil(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func agentRuntimeProjectionJSONRawEqual(left, right json.RawMessage) bool {
	return strings.TrimSpace(string(left)) == strings.TrimSpace(string(right))
}

func stringPointersEqual(left, right *string) bool {
	return strings.TrimSpace(derefString(left)) == strings.TrimSpace(derefString(right))
}

func timePointersEqual(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.UTC().Equal(right.UTC())
}

func isAIUsageCreditLimitError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, model.ErrAIUsageExhausted) ||
		errors.Is(err, model.ErrExtraAIUsageUnavailable) ||
		errors.Is(err, model.ErrBillingWorkspaceLocked)
}

func (s *AgentRuntimeProjectionService) resolveRun(ctx context.Context, event AgentRuntimeEventEnvelope) (*model.AgentRun, error) {
	hostRunID := strings.TrimSpace(event.HostRunID)
	if hostRunID == "" {
		hostRunID = eventDataString(event.Data, "host_run_id")
	}
	if hostRunID != "" {
		run, err := s.runRepo.GetByIDAny(ctx, hostRunID)
		if err != nil {
			return nil, err
		}
		if run != nil {
			return run, nil
		}
	}
	runtimeRunID := strings.TrimSpace(event.RunID)
	if runtimeRunID != "" {
		run, err := s.runRepo.GetByExternalRuntimeID(ctx, agentRuntimeName, runtimeRunID)
		if err != nil {
			return nil, err
		}
		if run != nil {
			return run, nil
		}
	}
	return nil, errAgentRuntimeProjectionRunNotFound
}

func (s *AgentRuntimeProjectionService) nowUTC() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func (s *AgentRuntimeProjectionService) eventTime(event AgentRuntimeEventEnvelope) time.Time {
	if !event.SentAt.IsZero() {
		return event.SentAt.UTC()
	}
	return s.nowUTC()
}

func isTerminalAgentRunStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case model.AgentRunStatusCompleted, model.AgentRunStatusFailed, model.AgentRunStatusCancelled:
		return true
	default:
		return false
	}
}

func isPreTerminalRuntimeEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case agentruntime.EventRunQueued, agentruntime.EventRunStarted, agentruntime.EventRunResumed, agentruntime.EventRunPaused:
		return true
	default:
		return false
	}
}

func isTerminalRuntimeEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case agentruntime.EventRunCompleted, agentruntime.EventRunFailed, agentruntime.EventRunCancelled:
		return true
	default:
		return false
	}
}

func setRunStatus(run *model.AgentRun, status, pauseReason string) bool {
	changed := false
	if strings.TrimSpace(run.Status) != status {
		run.Status = status
		changed = true
	}
	if strings.TrimSpace(run.PauseReason) != pauseReason {
		run.PauseReason = pauseReason
		changed = true
	}
	return changed
}

func normalizeRuntimePauseReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case model.AgentRunPauseReasonHumanApproval:
		return model.AgentRunPauseReasonHumanApproval
	case model.AgentRunPauseReasonAuthentication, "auth":
		return model.AgentRunPauseReasonAuthentication
	case model.AgentRunPauseReasonNone:
		return model.AgentRunPauseReasonNone
	default:
		return model.AgentRunPauseReasonHumanInput
	}
}

func eventDataString(data map[string]any, key string) string {
	if len(data) == 0 {
		return ""
	}
	value, ok := data[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func eventUsage(event AgentRuntimeEventEnvelope) (agentRuntimeUsagePayload, bool) {
	if usage, ok, err := event.UsageCheckpoint(); err == nil && ok {
		return usagePayloadFromSDK(usage.Usage)
	}
	return eventUsageFromData(event.Data)
}

func runtimeUsageSemantic(event AgentRuntimeEventEnvelope) string {
	if usage, ok, err := event.UsageCheckpoint(); err == nil && ok {
		return strings.TrimSpace(usage.UsageSemantic)
	}
	return strings.TrimSpace(eventDataString(event.Data, "usage_semantic"))
}

func usagePayloadFromSDK(usage agentruntime.Usage) (agentRuntimeUsagePayload, bool) {
	payload := agentRuntimeUsagePayload{
		TotalTokens:           clampInt64(usage.TotalTokens),
		InputTokens:           clampInt64(usage.InputTokens),
		CachedInputTokens:     clampInt64(usage.CachedInputTokens),
		OutputTokens:          clampInt64(usage.OutputTokens),
		ReasoningOutputTokens: clampInt64(usage.ReasoningOutputTokens),
	}
	if payload.TotalTokens == 0 {
		payload.TotalTokens = payload.InputTokens + payload.CachedInputTokens + payload.OutputTokens + payload.ReasoningOutputTokens
	}
	if payload.TotalTokens == 0 && payload.InputTokens == 0 && payload.CachedInputTokens == 0 && payload.OutputTokens == 0 && payload.ReasoningOutputTokens == 0 {
		return agentRuntimeUsagePayload{}, false
	}
	return payload, true
}

func eventUsageFromData(data map[string]any) (agentRuntimeUsagePayload, bool) {
	if len(data) == 0 {
		return agentRuntimeUsagePayload{}, false
	}
	raw, ok := data["usage"]
	if !ok || raw == nil {
		return agentRuntimeUsagePayload{}, false
	}
	usageMap, ok := raw.(map[string]any)
	if !ok {
		payload, err := json.Marshal(raw)
		if err != nil {
			return agentRuntimeUsagePayload{}, false
		}
		if err := json.Unmarshal(payload, &usageMap); err != nil {
			return agentRuntimeUsagePayload{}, false
		}
	}
	usage := agentRuntimeUsagePayload{
		TotalTokens:           mapInt(usageMap, "total_tokens"),
		InputTokens:           mapInt(usageMap, "input_tokens"),
		CachedInputTokens:     mapInt(usageMap, "cached_input_tokens"),
		OutputTokens:          mapInt(usageMap, "output_tokens"),
		ReasoningOutputTokens: mapInt(usageMap, "reasoning_output_tokens"),
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.CachedInputTokens + usage.OutputTokens + usage.ReasoningOutputTokens
	}
	if usage.TotalTokens == 0 && usage.InputTokens == 0 && usage.CachedInputTokens == 0 && usage.OutputTokens == 0 && usage.ReasoningOutputTokens == 0 {
		return agentRuntimeUsagePayload{}, false
	}
	return usage, true
}

func usageFromRuntimeOutputSummary(summary json.RawMessage) (agentRuntimeUsagePayload, bool) {
	if len(summary) == 0 || strings.TrimSpace(string(summary)) == "" || strings.TrimSpace(string(summary)) == "null" {
		return agentRuntimeUsagePayload{}, false
	}
	var body map[string]any
	if err := json.Unmarshal(summary, &body); err != nil {
		return agentRuntimeUsagePayload{}, false
	}
	return usageFromMap(body)
}

func usageFromMap(values map[string]any) (agentRuntimeUsagePayload, bool) {
	usage := agentRuntimeUsagePayload{
		TotalTokens:           mapInt(values, "total_tokens"),
		InputTokens:           mapInt(values, "input_tokens"),
		CachedInputTokens:     mapInt(values, "cached_input_tokens"),
		OutputTokens:          mapInt(values, "output_tokens"),
		ReasoningOutputTokens: mapInt(values, "reasoning_output_tokens"),
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.CachedInputTokens + usage.OutputTokens + usage.ReasoningOutputTokens
	}
	if usage.TotalTokens == 0 && usage.InputTokens == 0 && usage.CachedInputTokens == 0 && usage.OutputTokens == 0 && usage.ReasoningOutputTokens == 0 {
		return agentRuntimeUsagePayload{}, false
	}
	return usage, true
}

func applyRuntimeUsage(run *model.AgentRun, usage agentRuntimeUsagePayload) bool {
	changed := false
	if run.CachedInputTokens != usage.CachedInputTokens {
		run.CachedInputTokens = usage.CachedInputTokens
		changed = true
	}
	if run.InputTokens != usage.InputTokens {
		run.InputTokens = usage.InputTokens
		changed = true
	}
	if run.OutputTokens != usage.OutputTokens {
		run.OutputTokens = usage.OutputTokens
		changed = true
	}
	if run.TokensUsed != usage.TotalTokens {
		run.TokensUsed = usage.TotalTokens
		changed = true
	}
	return changed
}

func mapInt(values map[string]any, key string) int {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return clampInt64(typed)
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0
		}
		return clampInt64(int64(typed))
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return clampInt64(parsed)
		}
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
			return clampInt64(parsed)
		}
	}
	return 0
}

func clampInt64(value int64) int {
	if value <= 0 {
		return 0
	}
	if value > maxIntValue {
		return int(maxIntValue)
	}
	return int(value)
}
