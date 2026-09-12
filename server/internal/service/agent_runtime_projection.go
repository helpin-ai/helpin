package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"github.com/nats-io/nats.go"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

const (
	agentRuntimeProjectionDurable                 = "helpin-agent-runtime-projection"
	agentRuntimeExecutionStageUsageOverageCancel  = "usage_overage_cancel_requested"
	agentRuntimeUsageOverageCancellationErrorText = "agent runtime run cancelled because workspace AI usage is exhausted"
	agentRuntimeUsageConsumedSummaryKey           = "agent_runtime_usage_consumed"
	agentRuntimeUsageConsumedAtSummaryKey         = "agent_runtime_usage_consumed_at"
	agentRuntimeTranscriptReconciledVersionKey    = "agent_runtime_transcript_reconciled_runtime_updated_at"
	agentRuntimeV2ReplayThroughSummaryKey         = "agent_runtime_v2_replay_through"
	agentRuntimeLatestUsageSummaryKey             = "agent_runtime_latest_usage"
	agentRuntimeV2ReplayPageSize                  = 250
	agentRuntimeExecutionStageAuthCompleted       = "auth_completed"
	agentRuntimeExecutionStageAwaitingAuth        = "awaiting_auth"
	agentRuntimeCoverageCompletionError           = "the agent finished without a durable support coverage disposition; create or update review-ready documentation, record a routed or blocked finding, then call complete_support_coverage_gap"
)

var errAgentRuntimeProjectionRunNotFound = errors.New("agent runtime projection run not found")

var maxIntValue = int64(^uint(0) >> 1)

type agentRuntimeProjectionRunRepository interface {
	GetByIDAny(ctx context.Context, id string) (*model.AgentRun, error)
	GetByExternalRuntimeID(ctx context.Context, externalRuntime, externalRuntimeID string) (*model.AgentRun, error)
	ListActiveByExternalRuntime(ctx context.Context, externalRuntime string, olderThan time.Time, limit int) ([]model.AgentRun, error)
	UpdateRuntimeProjection(ctx context.Context, run *model.AgentRun) error
	UpdateRuntimeSummaryMarker(ctx context.Context, workspaceID, runID, key string, value json.RawMessage) error
	Notify(ctx context.Context, run *model.AgentRun)
}

type agentRuntimeProjectionAgentRepository interface {
	GetByID(ctx context.Context, workspaceID, id string) (*model.Agent, error)
}

type agentRuntimeProjectionMessageRepository interface {
	ListByRun(ctx context.Context, workspaceID, runID string) ([]model.AgentRunMessage, error)
	NextSequence(ctx context.Context, workspaceID, runID string) (int, error)
	Create(ctx context.Context, message *model.AgentRunMessage) error
	Update(ctx context.Context, message *model.AgentRunMessage) error
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

type agentRuntimeProjectionSessionSnapshotRepository interface {
	GetByRun(ctx context.Context, workspaceID, runID string) (*model.CodingSessionStateSnapshot, error)
	UpsertIfNewer(ctx context.Context, snapshot *model.CodingSessionStateSnapshot) (bool, error)
	DeleteByRun(ctx context.Context, workspaceID, runID string) error
}

type agentRuntimeV2ReplayClient interface {
	ListV2Events(ctx context.Context, runtimeRunID string, afterSequence int64) (*AgentRuntimeEventListResponse, error)
}

type agentRuntimeV2PagedReplayClient interface {
	ListV2EventPage(ctx context.Context, runtimeRunID string, afterSequence int64, pageSize int) (*AgentRuntimeEventListResponse, error)
}

type agentRuntimeToolCallClient interface {
	ListToolCalls(ctx context.Context, runtimeRunID string) ([]AgentRuntimeToolCall, error)
}

// AgentRuntimeProjectionService projects Agent Runtime lifecycle events back
// into Helpin's agent_runs table and existing realtime fanout.
type AgentRuntimeProjectionService struct {
	runRepo             agentRuntimeProjectionRunRepository
	agentRepo           agentRuntimeProjectionAgentRepository
	runMessageRepo      agentRuntimeProjectionMessageRepository
	artifactRepo        agentRuntimeProjectionArtifactRepository
	interactionRepo     agentRuntimeProjectionInteractionRepository
	sessionSnapshotRepo agentRuntimeProjectionSessionSnapshotRepository
	usageMeter          *AIUsageMeter
	agentRuntimeClient  agentRuntimeSignalClient
	runFinalizers       *AgentRunFinalizerService
	playbookExecution   atomic.Pointer[CRMPlaybookExecutionService]
	// supportChatPauseHook is set after the NATS consumer may already be
	// running, so access is atomic.
	supportChatPauseHook atomic.Pointer[supportChatPauseHookFunc]
	wsPublisher          websocket.EventPublisher
	appID                string
	eventProtocol        string
	now                  func() time.Time
	v2ReplayMu           sync.Mutex
	v2ReplayThrough      map[string]int64
	projectionLocks      [128]sync.Mutex
}

// SetCRMPlaybookExecution attaches durable follow-through without changing ordinary finalizers.
func (s *AgentRuntimeProjectionService) SetCRMPlaybookExecution(execution *CRMPlaybookExecutionService) {
	s.playbookExecution.Store(execution)
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
		runRepo:       runRepo,
		appID:         resolvedAppID,
		eventProtocol: "v1",
		now:           time.Now,
	}
}

// SetEventProtocol selects the app-scoped Agent Runtime event contract.
func (s *AgentRuntimeProjectionService) SetEventProtocol(protocol string) *AgentRuntimeProjectionService {
	if s == nil {
		return s
	}
	if strings.EqualFold(strings.TrimSpace(protocol), "v2") {
		s.eventProtocol = "v2"
	} else {
		s.eventProtocol = "v1"
	}
	return s
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

func (s *AgentRuntimeProjectionService) SetCodingSessionSnapshotRepository(sessionSnapshotRepo *repository.CodingSessionStateSnapshotRepository) *AgentRuntimeProjectionService {
	if s == nil {
		return s
	}
	s.sessionSnapshotRepo = sessionSnapshotRepo
	return s
}

func (s *AgentRuntimeProjectionService) SetWebSocketPublisher(wsPublisher websocket.EventPublisher) *AgentRuntimeProjectionService {
	if s == nil {
		return s
	}
	s.wsPublisher = wsPublisher
	return s
}

// SetRunFinalizers wires the product side-effect finalizers dispatched when a
// delegated run transitions into a terminal status.
// SetSupportChatPauseHook wires the support chat lifecycle callback invoked
// after a run pauses awaiting the next user message (turn settlement check,
// thinking indicator, deferred-message drain).
type supportChatPauseHookFunc = func(context.Context, *model.AgentRun)

func (s *AgentRuntimeProjectionService) SetSupportChatPauseHook(hook supportChatPauseHookFunc) *AgentRuntimeProjectionService {
	if s != nil && hook != nil {
		s.supportChatPauseHook.Store(&hook)
	}
	return s
}

func (s *AgentRuntimeProjectionService) SetRunFinalizers(finalizers *AgentRunFinalizerService) *AgentRuntimeProjectionService {
	if s == nil {
		return s
	}
	s.runFinalizers = finalizers
	return s
}

func (s *AgentRuntimeProjectionService) StartNATSConsumer(
	ctx context.Context,
	js nats.JetStreamContext,
) (returnErr error) {
	if s == nil || s.runRepo == nil || js == nil {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "agent runtime projection consumer panic", "panic", recovered)
			returnErr = fmt.Errorf("agent runtime projection consumer panic: %v", recovered)
		}
	}()
	durable := agentRuntimeProjectionDurable
	subject := ""
	if s.eventProtocol == "v2" {
		durable += "-v2"
		subject = fmt.Sprintf("agent-runtime.events.v2.%s.>", runtimeSubjectToken(s.appID))
	}
	consumer := agentruntime.NewNATSConsumer(agentruntime.NATSConsumerConfig{
		JetStream:    js,
		AppID:        s.appID,
		Durable:      durable,
		Subject:      subject,
		Stream:       agentruntime.DefaultNATSStreamName,
		EnsureStream: true,
		Logger:       slog.Default(),
	})
	slog.Info("agent runtime projection consumer starting",
		"app_id", s.appID,
		"event_protocol", s.eventProtocol,
		"subject", firstNonEmptyString(subject, agentruntime.AppEventSubject(s.appID)),
		"durable", durable,
	)
	return consumer.Run(ctx, func(ctx context.Context, event AgentRuntimeEventEnvelope) error {
		if err := s.ApplyEvent(ctx, event); err != nil {
			if errors.Is(err, errAgentRuntimeProjectionRunNotFound) {
				slog.WarnContext(ctx, "agent runtime projection waiting for mapped run",
					"runtime_run_id", event.RunID,
					"host_run_id", event.HostRunID,
					"event_type", event.Type,
				)
				return agentruntime.ErrRetryEvent
			}
			slog.ErrorContext(ctx, "agent runtime projection failed",
				"runtime_run_id", event.RunID,
				"host_run_id", event.HostRunID,
				"event_type", event.Type,
				"error", err,
			)
			return err
		}
		return nil
	})
}

func (s *AgentRuntimeProjectionService) StartReconciliationSweep(
	ctx context.Context,
	interval time.Duration,
	staleAfter time.Duration,
	limit int,
) (returnErr error) {
	if s == nil || s.runRepo == nil || s.agentRuntimeClient == nil {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "agent runtime reconciliation sweep panic", "panic", recovered)
			returnErr = fmt.Errorf("agent runtime reconciliation sweep panic: %v", recovered)
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
		if strings.TrimSpace(derefString(run.ExecutionStage)) == "cancelling" {
			runtimeRun, err := s.agentRuntimeClient.CancelRun(ctx, runtimeRunID)
			if err != nil {
				slog.WarnContext(ctx, "agent runtime cancellation reconciliation failed",
					"workspace_id", run.WorkspaceID,
					"run_id", run.ID,
					"runtime_run_id", runtimeRunID,
					"error", err,
				)
				continue
			}
			event, ok := cancellationAcknowledgementEvent(runtimeRun, run, s.nowUTC())
			if !ok {
				continue
			}
			if err := s.ApplyEvent(ctx, event); err != nil {
				slog.WarnContext(ctx, "agent runtime cancellation acknowledgement apply failed",
					"workspace_id", run.WorkspaceID,
					"run_id", run.ID,
					"runtime_run_id", runtimeRunID,
					"error", err,
				)
			}
			continue
		}
		if s.eventProtocol == "v2" {
			if err := s.replayV2Events(ctx, &run, runtimeRunID); err != nil {
				slog.WarnContext(ctx, "agent runtime v2 event replay failed",
					"workspace_id", run.WorkspaceID,
					"run_id", run.ID,
					"runtime_run_id", runtimeRunID,
					"error", err,
				)
			}
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
			if err := persistRuntimeSummaryMarker(ctx, s.runRepo, projectedRun, agentRuntimeTranscriptReconciledVersionKey); err != nil {
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

func (s *AgentRuntimeProjectionService) replayV2Events(ctx context.Context, run *model.AgentRun, runtimeRunID string) error {
	if run == nil {
		return nil
	}
	afterSequence := s.v2ReplayCursor(run, runtimeRunID)
	if client, ok := s.agentRuntimeClient.(agentRuntimeV2PagedReplayClient); ok {
		for {
			response, err := client.ListV2EventPage(ctx, runtimeRunID, afterSequence, agentRuntimeV2ReplayPageSize)
			if err != nil {
				return err
			}
			if response == nil || len(response.Events) == 0 {
				return nil
			}
			pageStart := afterSequence
			for _, event := range response.Events {
				if event.SequenceNo <= afterSequence {
					continue
				}
				if err := s.ApplyEvent(ctx, event); err != nil {
					return fmt.Errorf("apply sequence %d: %w", event.SequenceNo, err)
				}
				afterSequence = event.SequenceNo
				s.setV2ReplayCursor(runtimeRunID, afterSequence)
			}
			if afterSequence == pageStart {
				return fmt.Errorf("agent runtime v2 event page did not advance beyond sequence %d", pageStart)
			}
			if err := s.persistV2ReplayCursor(ctx, run, runtimeRunID, afterSequence); err != nil {
				return err
			}
			if len(response.Events) < agentRuntimeV2ReplayPageSize {
				return nil
			}
		}
	}
	client, ok := s.agentRuntimeClient.(agentRuntimeV2ReplayClient)
	if !ok {
		return nil
	}
	response, err := client.ListV2Events(ctx, runtimeRunID, afterSequence)
	if err != nil {
		return err
	}
	if response == nil {
		return nil
	}
	for _, event := range response.Events {
		if event.SequenceNo <= afterSequence {
			continue
		}
		if err := s.ApplyEvent(ctx, event); err != nil {
			return fmt.Errorf("apply sequence %d: %w", event.SequenceNo, err)
		}
		s.setV2ReplayCursor(runtimeRunID, event.SequenceNo)
		afterSequence = event.SequenceNo
	}
	if err := s.persistV2ReplayCursor(ctx, run, runtimeRunID, afterSequence); err != nil {
		return err
	}
	return nil
}

func (s *AgentRuntimeProjectionService) v2ReplayCursor(run *model.AgentRun, runtimeRunID string) int64 {
	persisted := int64(0)
	if run != nil {
		persisted = runtimeV2ReplayCursor(run.OutputSummary)
	}
	s.v2ReplayMu.Lock()
	defer s.v2ReplayMu.Unlock()
	if current := s.v2ReplayThrough[strings.TrimSpace(runtimeRunID)]; current > persisted {
		return current
	}
	return persisted
}

func (s *AgentRuntimeProjectionService) setV2ReplayCursor(runtimeRunID string, sequence int64) {
	if sequence <= 0 {
		return
	}
	s.v2ReplayMu.Lock()
	defer s.v2ReplayMu.Unlock()
	if s.v2ReplayThrough == nil {
		s.v2ReplayThrough = make(map[string]int64)
	}
	key := strings.TrimSpace(runtimeRunID)
	if sequence > s.v2ReplayThrough[key] {
		s.v2ReplayThrough[key] = sequence
	}
}

func (s *AgentRuntimeProjectionService) persistV2ReplayCursor(ctx context.Context, run *model.AgentRun, runtimeRunID string, sequence int64) error {
	if run == nil || sequence <= 0 {
		return nil
	}
	latest, err := s.runRepo.GetByIDAny(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("load run before persisting v2 replay cursor: %w", err)
	}
	if latest != nil {
		run.OutputSummary = append(json.RawMessage(nil), latest.OutputSummary...)
	}
	if !markRuntimeV2ReplayCursor(run, sequence) {
		s.setV2ReplayCursor(runtimeRunID, sequence)
		return nil
	}
	if err := persistRuntimeSummaryMarker(ctx, s.runRepo, run, agentRuntimeV2ReplayThroughSummaryKey); err != nil {
		return fmt.Errorf("persist v2 replay cursor %d: %w", sequence, err)
	}
	s.setV2ReplayCursor(runtimeRunID, sequence)
	return nil
}

func markRuntimeV2ReplayCursor(run *model.AgentRun, sequence int64) bool {
	if run == nil || sequence <= runtimeV2ReplayCursor(run.OutputSummary) {
		return false
	}
	body := map[string]json.RawMessage{}
	if len(run.OutputSummary) > 0 {
		_ = json.Unmarshal(run.OutputSummary, &body)
	}
	value, err := json.Marshal(sequence)
	if err != nil {
		return false
	}
	body[agentRuntimeV2ReplayThroughSummaryKey] = value
	payload, err := json.Marshal(body)
	if err != nil {
		return false
	}
	run.OutputSummary = payload
	return true
}

func runtimeV2ReplayCursor(summary json.RawMessage) int64 {
	if len(summary) == 0 {
		return 0
	}
	body := map[string]json.RawMessage{}
	if err := json.Unmarshal(summary, &body); err != nil {
		return 0
	}
	var sequence int64
	if err := json.Unmarshal(body[agentRuntimeV2ReplayThroughSummaryKey], &sequence); err != nil || sequence < 0 {
		return 0
	}
	return sequence
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

// cancellationAcknowledgementEvent treats a successful runtime cancellation
// command as authoritative even when the runtime response still contains its
// pre-cancellation status. The cancellation endpoint accepting the command is
// the durable boundary; asynchronous events remain safe, idempotent replays.
func cancellationAcknowledgementEvent(runtimeRun *AgentRuntimeRun, localRun model.AgentRun, fallback time.Time) (AgentRuntimeEventEnvelope, bool) {
	acknowledged := AgentRuntimeRun{
		ID:        strings.TrimSpace(derefString(localRun.ExternalRuntimeID)),
		HostRunID: strings.TrimSpace(localRun.ID),
		Status:    model.AgentRunStatusCancelled,
	}
	if runtimeRun != nil {
		acknowledged = *runtimeRun
		acknowledged.Status = model.AgentRunStatusCancelled
	}
	if acknowledged.CompletedAt == nil {
		completedAt := fallback
		acknowledged.CompletedAt = &completedAt
	}
	return reconciliationEventForRuntimeRun(&acknowledged, localRun, fallback)
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

func (s *AgentRuntimeProjectionService) ApplyEvent(ctx context.Context, event AgentRuntimeEventEnvelope) error {
	if s == nil || s.runRepo == nil {
		return fmt.Errorf("agent runtime projection service is not configured")
	}
	projectionLock := s.projectionLock(event)
	projectionLock.Lock()
	defer projectionLock.Unlock()

	run, err := s.resolveRun(ctx, event)
	if err != nil {
		return err
	}
	if ignore, err := s.ignoreStaleTurnLifecycle(ctx, run, event); err != nil {
		return err
	} else if ignore {
		return nil
	}
	// Prior persisted status, captured before the event is applied: product
	// finalizers fire only on the transition into a terminal status, so
	// redelivered or reconciled terminal events on an already-terminal run
	// are no-ops.
	wasTerminal := isTerminalAgentRunStatus(run.Status)
	changed, err := seedTerminalUsageBaseline(run)
	if err != nil {
		return err
	}
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
	if strings.HasPrefix(event.Type, "run.") && eventDataString(event.Data, "completion_mode") == "explicit" {
		if s.persistRuntimeCodingSessionStreamSnapshot(ctx, run, event) {
			s.publishRuntimeCodingSessionEvent(run, event)
		}
	}
	var settlementErr error
	suppressLifecycle := isTerminalAgentRunStatus(run.Status) && isPreTerminalRuntimeEvent(event.Type)
	if !suppressLifecycle && isRuntimeWorkProgressEvent(event.Type) {
		changed = clearRuntimeResumeStage(run) || changed
	}
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
			changed = clearRuntimeResumeStage(run) || changed
		}
	case agentruntime.EventRunResumed:
		if !suppressLifecycle {
			run.CompletedAt = nil
			changed = setRunStatus(run, model.AgentRunStatusRunning, model.AgentRunPauseReasonNone) || changed
			changed = clearRuntimeResumeStage(run) || changed
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
		if requiresSupportCoverageCompletion(run) {
			s.mergeRuntimeOutputSummaryForFinalizers(ctx, run)
			if !hasDurableSupportCoverageGapOutcome(run.OutputSummary) && !s.recoverSupportCoverageGapOutcomeFromToolCalls(ctx, run) {
				message := agentRuntimeCoverageCompletionError
				run.ErrorMessage = &message
				changed = setRunStatus(run, model.AgentRunStatusFailed, model.AgentRunPauseReasonNone) || changed
				break
			}
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
		if strings.TrimSpace(derefString(run.ExecutionStage)) == "cancelling" {
			run.ExecutionStage = nil
			changed = true
		}
		changed = setRunStatus(run, model.AgentRunStatusCancelled, model.AgentRunPauseReasonNone) || changed
	case agentruntime.EventUsageCheckpoint:
	case "workspace.prepared":
		if applyRuntimeWorkspacePrepared(run, event) {
			changed = true
		}
	case agentruntime.EventAssistantMessageStarted, agentruntime.EventAssistantMessageDelta:
		if s.persistRuntimeCodingSessionStreamSnapshot(ctx, run, event) {
			s.publishRuntimeCodingSessionEvent(run, event)
		}
	case agentruntime.EventAssistantMessageCompleted:
		if s.persistRuntimeCodingSessionStreamSnapshot(ctx, run, event) {
			s.publishRuntimeCodingSessionEvent(run, event)
		}
		if err := s.mirrorAssistantMessageCompleted(ctx, run, event); err != nil {
			return err
		}
	case agentruntime.EventReasoningMessageStarted, agentruntime.EventReasoningMessageDelta, agentruntime.EventReasoningMessageCompleted:
		if s.persistRuntimeCodingSessionStreamSnapshot(ctx, run, event) {
			s.publishRuntimeCodingSessionEvent(run, event)
		}
	case agentruntime.EventToolCallStarted, agentruntime.EventToolCallResult, agentruntime.EventToolCallFinished:
		// Tool-call activity renders inline in the transcript via message
		// turn_segments (and live via the coding-session snapshot) — do not
		// mirror it as artifacts, which would surface raw JSON in the side
		// panel.
		if s.persistRuntimeCodingSessionStreamSnapshot(ctx, run, event) {
			s.publishRuntimeCodingSessionEvent(run, event)
		}
	case agentruntime.EventToolCallArgsDelta:
		if s.persistRuntimeCodingSessionStreamSnapshot(ctx, run, event) {
			s.publishRuntimeCodingSessionEvent(run, event)
		}
	case agentruntime.EventPlanUpdated:
		if s.persistRuntimeCodingSessionStreamSnapshot(ctx, run, event) {
			s.publishRuntimeCodingSessionEvent(run, event)
		}
		if err := s.mirrorRuntimePlanUpdated(ctx, run, event); err != nil {
			return err
		}

	default:
		return nil
	}

	if strings.TrimSpace(event.Type) == agentruntime.EventRunPaused && isRuntimeHumanPause(run) {
		if runtimeRunID := strings.TrimSpace(derefString(run.ExternalRuntimeID)); runtimeRunID != "" {
			if err := s.reconcileRuntimeTranscript(ctx, run, runtimeRunID); err != nil {
				return err
			}
		}
	}
	if isTerminalRuntimeEvent(event.Type) {
		if err := s.cancelPendingRuntimeInteractions(ctx, run, now); err != nil {
			return err
		}
		if runtimeRunID := strings.TrimSpace(derefString(run.ExternalRuntimeID)); runtimeRunID != "" {
			if err := s.reconcileRuntimeTranscript(ctx, run, runtimeRunID); err != nil {
				return err
			}
		}
		// Keep the v2 stream snapshot after completion. It is the durable ordered
		// timeline for multi-message Codex turns and preserves tool placement while
		// persisted messages are reconciled by stable IDs. Preserve the legacy v1
		// cleanup behavior because those snapshots have no replay watermark.
		if s.eventProtocol != "v2" && eventDataString(event.Data, "completion_mode") != "explicit" && s.sessionSnapshotRepo != nil &&
			(event.Type == agentruntime.EventRunCompleted || event.Type == agentruntime.EventRunCancelled) {
			if err := s.sessionSnapshotRepo.DeleteByRun(ctx, run.WorkspaceID, run.ID); err != nil {
				return err
			}
		}
	}
	usage, hasUsage := eventUsage(event)
	if previous, ok := latestAgentRuntimeUsage(run); ok && (runtimeUsageSemantic(event) == agentruntime.UsageSemanticCumulative || isTerminalRuntimeEvent(event.Type)) {
		usage = maxAgentRuntimeUsage(usage, previous)
		hasUsage = true
	}
	if !hasUsage && isTerminalRuntimeEvent(event.Type) && s.usageMeter != nil && s.usageMeter.usage != nil {
		hasUsage = true
	}
	if hasUsage {
		if applyRuntimeUsage(run, usage) {
			changed = true
		}
		if storeLatestAgentRuntimeUsage(run, usage) {
			changed = true
		}
		terminalUsageChanged, err := s.maybeConsumeTerminalUsage(ctx, run, event, usage)
		if err != nil {
			if errors.Is(err, repository.ErrAIUsageWatermarkChanged) {
				return err
			}
			slog.ErrorContext(ctx, "agent runtime terminal usage consumption failed",
				"error", err,
				"workspace_id", run.WorkspaceID,
				"run_id", run.ID,
				"runtime_run_id", strings.TrimSpace(derefString(run.ExternalRuntimeID)),
				"event_type", event.Type,
			)
			settlementErr = err // Persist lifecycle state, then request redelivery.
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
	if shouldCheckpointPausedDockChat(run, event.Type) && s.usageMeter != nil && s.usageMeter.usage != nil {
		if metering, ok := agentRunMeteringContext(run); ok {
			canSuspendReservation := true
			if usage, hasUsage := latestAgentRuntimeUsage(run); hasUsage {
				previousSummary := run.OutputSummary
				if err := s.usageMeter.checkpointAgentRun(ctx, run, usage); err != nil {
					if errors.Is(err, repository.ErrAIUsageWatermarkChanged) {
						return err
					}
					canSuspendReservation = false
					slog.ErrorContext(ctx, "agent runtime chat-turn usage checkpoint failed",
						"error", err,
						"workspace_id", run.WorkspaceID,
						"run_id", run.ID,
						"runtime_run_id", strings.TrimSpace(derefString(run.ExternalRuntimeID)),
					)
				} else if !agentRuntimeProjectionJSONRawEqual(previousSummary, run.OutputSummary) {
					changed = true
				}
			}
			if canSuspendReservation {
				if err := s.usageMeter.usage.SuspendReservation(ctx, metering); err != nil {
					slog.ErrorContext(ctx, "agent runtime paused chat reservation suspension failed",
						"error", err,
						"workspace_id", run.WorkspaceID,
						"run_id", run.ID,
						"reservation_id", metering.ReservationID,
					)
				}
			}
		}
	}
	if observer := s.playbookExecution.Load(); observer != nil && isTerminalAgentRunStatus(run.Status) {
		if crm, err := crmPlaybookInput(run.Input); err == nil && crm != nil {
			if err := observer.ObserveTerminalRun(ctx, *run); err != nil {
				return err
			}
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
		return settlementErr
	}
	model.NormalizeAgentRunPauseState(run)
	if err := s.runRepo.UpdateRuntimeProjection(ctx, run); err != nil {
		return err
	}
	s.notifyRunChange(ctx, run, model.AgentRunChangeState)
	if hookPtr := s.supportChatPauseHook.Load(); hookPtr != nil && event.Type == agentruntime.EventRunPaused &&
		run.Status == model.AgentRunStatusPaused && run.PauseReason == model.AgentRunPauseReasonUserMessage {
		hook := *hookPtr
		runCopy := *run
		go func() {
			hookCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			defer func() {
				if recovered := recover(); recovered != nil {
					slog.Error("support chat pause hook panic", "panic", recovered)
				}
			}()
			hook(hookCtx, &runCopy)
		}()
	}
	return settlementErr
}

func (s *AgentRuntimeProjectionService) projectionLock(event AgentRuntimeEventEnvelope) *sync.Mutex {
	key := firstNonEmptyString(
		strings.TrimSpace(event.RunID),
		strings.TrimSpace(event.HostRunID),
		strings.TrimSpace(event.EventID),
	)
	var hash uint32 = 2166136261
	for index := 0; index < len(key); index++ {
		hash ^= uint32(key[index])
		hash *= 16777619
	}
	return &s.projectionLocks[hash%uint32(len(s.projectionLocks))]
}

func (s *AgentRuntimeProjectionService) cancelPendingRuntimeInteractions(ctx context.Context, run *model.AgentRun, timestamp time.Time) error {
	if s == nil || s.interactionRepo == nil || run == nil {
		return nil
	}
	interactions, err := s.interactionRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	for index := range interactions {
		if interactions[index].Status != model.AgentRunInteractionStatusPending {
			continue
		}
		updated := interactions[index]
		updated.Status = model.AgentRunInteractionStatusCancelled
		updated.ResolvedAt = &timestamp
		if err := s.interactionRepo.Update(ctx, &updated); err != nil {
			return err
		}
		s.publishRuntimeInteractionEvent(run, updated)
	}
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

// requiresSupportCoverageCompletion scopes the host-side terminal safeguard
// to the target-owned coverage contract. Agent Runtime owns the primary
// required-tool retry; this prevents an older or drifting runtime from
// projecting a prose-only turn as successful in Helpin.
func requiresSupportCoverageCompletion(run *model.AgentRun) bool {
	return run != nil && strings.TrimSpace(run.TargetType) == "support_coverage_gap"
}

func hasDurableSupportCoverageGapOutcome(summary json.RawMessage) bool {
	if len(summary) == 0 {
		return false
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(summary, &body); err != nil {
		return false
	}
	var outcome supportCoverageGapOutcomeSummary
	if err := json.Unmarshal(body[supportCoverageGapOutcomeSummaryKey], &outcome); err != nil {
		return false
	}
	legacy := completeSupportCoverageGapRequest{
		Outcome: outcome.Outcome, Action: outcome.Action, SourceStatus: outcome.SourceStatus,
		DocumentID: outcome.DocumentID, ProposalID: outcome.ProposalID, HandoffOwner: outcome.HandoffOwner,
		DocumentationEvidence: outcome.DocumentationEvidence, SourceEvidence: outcome.SourceEvidence, Summary: outcome.Summary,
	}
	normalizeCompleteSupportCoverageGapRequest(&legacy)
	outcome.Outcome = legacy.Outcome
	outcome.Action = legacy.Action
	outcome.SourceStatus = legacy.SourceStatus
	outcome.HandoffOwner = legacy.HandoffOwner
	outcome.DocumentationEvidence = legacy.DocumentationEvidence
	if strings.TrimSpace(outcome.Summary) == "" || strings.TrimSpace(outcome.Action) == "" {
		return false
	}
	switch strings.TrimSpace(outcome.Outcome) {
	case SupportCoverageAgentOutcomeResolved:
		return strings.TrimSpace(outcome.DocumentID) != "" &&
			(outcome.Action == SupportCoverageAgentActionDocumentCreated || outcome.Action == SupportCoverageAgentActionDocumentUpdated)
	case SupportCoverageAgentOutcomeReviewReady:
		if strings.TrimSpace(outcome.DocumentID) == "" {
			return false
		}
		return outcome.Action != SupportCoverageAgentActionProposalSubmitted || strings.TrimSpace(outcome.ProposalID) != ""
	case SupportCoverageAgentOutcomeRouted:
		return strings.TrimSpace(outcome.HandoffOwner) != ""
	case SupportCoverageAgentOutcomeBlocked:
		return outcome.Action == SupportCoverageAgentActionSourceUnavailable && strings.TrimSpace(outcome.HandoffOwner) != ""
	default:
		return false
	}
}

// recoverSupportCoverageGapOutcomeFromToolCalls prevents a completed coverage
// run from being reported as failed when it already persisted reviewable Docs
// work but omitted the terminal disposition call. The recovery is deliberately
// conservative: it only trusts successful durable Docs mutation/proposal tool
// calls recorded by Agent Runtime, and keeps the gap open as review_ready.
func (s *AgentRuntimeProjectionService) recoverSupportCoverageGapOutcomeFromToolCalls(ctx context.Context, run *model.AgentRun) bool {
	if s == nil || run == nil || s.agentRuntimeClient == nil {
		return false
	}
	client, ok := s.agentRuntimeClient.(agentRuntimeToolCallClient)
	if !ok {
		return false
	}
	runtimeRunID := strings.TrimSpace(derefString(run.ExternalRuntimeID))
	if runtimeRunID == "" {
		return false
	}
	calls, err := client.ListToolCalls(ctx, runtimeRunID)
	if err != nil {
		slog.ErrorContext(ctx, "support coverage tool-call recovery failed",
			"error", err,
			"workspace_id", run.WorkspaceID,
			"run_id", run.ID,
			"runtime_run_id", runtimeRunID,
		)
		return false
	}
	var recovered *supportCoverageGapOutcomeSummary
	for _, call := range calls {
		if strings.TrimSpace(call.Error) != "" || call.ApprovalRequired {
			continue
		}
		toolName := agentcontract.CanonicalToolName(call.ToolName)
		action := ""
		documentID := ""
		proposalID := ""
		switch toolName {
		case "create_document":
			action = SupportCoverageAgentActionDocumentCreated
			documentID = firstRuntimeToolCallString(call.Output, "document_id", "id")
		case "write_document_content", "update_document_block", "insert_document_block", "insert_document_artifact", "insert_document_image":
			action = SupportCoverageAgentActionDocumentUpdated
			documentID = firstRuntimeToolCallString(call.Output, "document_id")
			if documentID == "" {
				documentID = firstRuntimeToolCallString(call.Input, "document_id")
			}
		case agentcontract.ToolPublishDocumentChangeProposal:
			action = SupportCoverageAgentActionProposalSubmitted
			documentID = firstRuntimeToolCallString(call.Output, "document_id")
			proposalID = firstRuntimeToolCallString(call.Output, "proposal_id")
		}
		if action == "" || documentID == "" || (action == SupportCoverageAgentActionProposalSubmitted && proposalID == "") {
			continue
		}
		recovered = &supportCoverageGapOutcomeSummary{
			Outcome: SupportCoverageAgentOutcomeReviewReady, Action: action,
			DocumentID: documentID, ProposalID: proposalID,
			DocumentationEvidence: "Recovered from a successful durable Docs tool call recorded for this run.",
			Summary:               "The agent persisted documentation work but omitted the final disposition; recovered as review-ready for human verification.",
			Recovered:             true,
			RecordedAt:            time.Now().UTC().Format(time.RFC3339),
		}
	}
	if recovered == nil {
		return false
	}
	body := map[string]any{}
	if len(run.OutputSummary) > 0 {
		_ = json.Unmarshal(run.OutputSummary, &body)
	}
	body[supportCoverageGapOutcomeSummaryKey] = recovered
	payload, err := json.Marshal(body)
	if err != nil {
		return false
	}
	run.OutputSummary = payload
	return true
}

func firstRuntimeToolCallString(raw json.RawMessage, keys ...string) string {
	if len(raw) == 0 {
		return ""
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return firstRuntimeToolCallValueString(value, keys, 0)
}

// firstRuntimeToolCallValueString reads both Agent Runtime audit formats:
// native_sdk wraps the command response as JSON text under output, while
// Codex records it under result and may wrap text inside contentItems.
func firstRuntimeToolCallValueString(value any, keys []string, depth int) string {
	if value == nil || depth > 8 {
		return ""
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range keys {
			switch candidate := typed[key].(type) {
			case string:
				if trimmed := strings.TrimSpace(candidate); trimmed != "" {
					return trimmed
				}
			case json.Number:
				if trimmed := strings.TrimSpace(candidate.String()); trimmed != "" {
					return trimmed
				}
			}
		}
		for _, envelopeKey := range []string{"result", "output", "arguments", "contentItems", "content_items", "content", "text", "data", "response"} {
			if candidate, ok := typed[envelopeKey]; ok {
				if found := firstRuntimeToolCallValueString(candidate, keys, depth+1); found != "" {
					return found
				}
			}
		}
	case []any:
		for _, candidate := range typed {
			if found := firstRuntimeToolCallValueString(candidate, keys, depth+1); found != "" {
				return found
			}
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" || !json.Valid([]byte(trimmed)) {
			return ""
		}
		var nested any
		if json.Unmarshal([]byte(trimmed), &nested) == nil {
			return firstRuntimeToolCallValueString(nested, keys, depth+1)
		}
	}
	return ""
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
		if strings.HasPrefix(key, "agent_runtime_") || key == supportCoverageGapOutcomeSummaryKey {
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

func applyRuntimeWorkspacePrepared(run *model.AgentRun, event AgentRuntimeEventEnvelope) bool {
	if run == nil {
		return false
	}
	metadata := eventDataMap(event.Data, "metadata")
	if len(metadata) == 0 {
		return false
	}
	repository := map[string]any{}
	for _, key := range []string{"repository_id", "repo_full_name", "base_branch", "work_branch", "clone_url", "repository_fingerprint"} {
		if value, ok := metadata[key]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
			repository[key] = value
		}
	}
	branchSync := map[string]any{}
	for _, key := range []string{"branch_sync_status", "branch_sync_base_branch", "branch_sync_work_branch", "branch_sync_conflict_files", "branch_sync_backup_branch"} {
		if value, ok := metadata[key]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
			branchSync[key] = value
		}
	}
	if len(branchSync) > 0 {
		repository["branch_sync"] = branchSync
	}
	if len(repository) == 0 {
		return false
	}
	body := map[string]any{}
	if len(run.OutputSummary) > 0 {
		_ = json.Unmarshal(run.OutputSummary, &body)
	}
	body["repository"] = mergeRuntimeSummaryMap(body["repository"], repository)
	payload, _ := json.Marshal(body)
	if agentRuntimeProjectionJSONRawEqual(run.OutputSummary, payload) {
		return false
	}
	run.OutputSummary = payload
	return true
}

func mergeRuntimeSummaryMap(existing any, updates map[string]any) map[string]any {
	out := map[string]any{}
	if existingMap, ok := existing.(map[string]any); ok {
		for key, value := range existingMap {
			out[key] = value
		}
	}
	for key, value := range updates {
		if nested, ok := value.(map[string]any); ok {
			out[key] = mergeRuntimeSummaryMap(out[key], nested)
			continue
		}
		out[key] = value
	}
	return out
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
	if s.usageMeter.usage != nil {
		if metering, ok := agentRunMeteringContext(run); ok {
			if heartbeatErr := s.usageMeter.usage.Heartbeat(ctx, metering); heartbeatErr != nil {
				return false, heartbeatErr
			}
		}
		if !agentRunUsageExceedsBudget(run, usage) {
			return false, nil
		}
		err = model.ErrAIUsageExhausted
	} else {
		err = s.usageMeter.PreflightUsage(ctx, AIUsageMeterInput{
			WorkspaceID:       run.WorkspaceID,
			FeatureKey:        AgentRunAIUsageFeature(agent),
			InputTokens:       usage.InputTokens,
			OutputTokens:      int(agentRunTokenTelemetry(run, usage).OutputTokens),
			ReasoningTokens:   usage.ReasoningOutputTokens,
			CachedInputTokens: usage.CachedInputTokens,
			Metadata: map[string]interface{}{
				"run_id":         run.ID,
				"runtime_run_id": runtimeRunID,
				"agent_id":       run.AgentID,
				"checkpoint":     true,
			},
		})
	}
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
	if !isTerminalRuntimeEvent(event.Type) && !(isTerminalAgentRunStatus(run.Status) && event.Type == agentruntime.EventUsageCheckpoint) {
		return false, nil
	}
	semantic := runtimeUsageSemantic(event)
	if semantic != "" && semantic != agentruntime.UsageSemanticCumulative {
		return false, nil
	}
	if runtimeUsageAlreadyConsumed(run.OutputSummary) && agentRunUsageIsZero(agentRunUsageDelta(usage, agentRunUsageCheckpointFromSummary(run.OutputSummary))) {
		return false, nil
	}
	agent, err := s.agentRepo.GetByID(ctx, run.WorkspaceID, run.AgentID)
	if err != nil {
		return false, err
	}
	return s.settleTerminalUsage(ctx, run, agent, event, usage)
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
	// Enrich from the runtime store row when reachable: it carries the tool
	// invocations for the turn, which drive inline transcript rendering.
	runtimeMessage := s.lookupRuntimeStoreMessage(ctx, run, runtimeMessageID)
	var storeBlocks, toolInvocations json.RawMessage
	messageType := "assistant_turn"
	if kind := eventDataString(event.Data, "message_type"); kind == "assistant_final" || kind == "assistant_progress" {
		messageType = kind
	}
	if runtimeMessage != nil {
		storeBlocks = runtimeMessage.ContentBlocks
		toolInvocations = runtimeMessage.ToolInvocations
		if runtimeMessage.MessageType == "assistant_final" || runtimeMessage.MessageType == "assistant_progress" {
			messageType = runtimeMessage.MessageType
		}
	}
	message := &model.AgentRunMessage{
		WorkspaceID:      run.WorkspaceID,
		RunID:            run.ID,
		DockChatID:       run.DockChatID,
		DeliveryStatus:   "sent",
		RuntimeMessageID: runtimeMessageID,
		Role:             "assistant",
		Content:          content,
		MessageType:      messageType,
		ContentBlocks:    annotateRuntimeMessageBlocks(storeBlocks, runtimeMessageID, content),
		ToolInvocations:  toolInvocations,
		TurnSegments:     runtimeMessageTurnSegments(runtimeMessageID, content, toolInvocations),
		SequenceNo:       sequenceNo,
		// The chat transcript orders by created_at; the envelope's sent_at is
		// the emission time the runtime persisted with the event, so replayed
		// events keep conversation order instead of clustering at insert time.
		CreatedAt: s.eventTime(event),
	}
	if err := s.runMessageRepo.Create(ctx, message); err != nil {
		return err
	}
	if messageType == "assistant_final" {
		slog.InfoContext(ctx, "turn answer projected", "run_id", run.ID, "runtime_run_id", event.RunID,
			"turn_id", eventDataString(event.Data, "turn_id"), "message_id", runtimeMessageID,
			"answer_bytes", len(content), "answer_sha256", fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
			"runtime_revision", eventDataString(event.Data, "runtime_revision"), "event_sequence", event.SequenceNo)
	}
	s.notifyRunChange(ctx, run, model.AgentRunChangeMessage)
	return nil
}

func (s *AgentRuntimeProjectionService) persistRuntimeCodingSessionStreamSnapshot(ctx context.Context, run *model.AgentRun, event AgentRuntimeEventEnvelope) bool {
	if s == nil || s.sessionSnapshotRepo == nil || run == nil {
		return true
	}
	eventType := codingSessionEventTypeFromAgentRuntimeEvent(event)
	if eventType == "" {
		return false
	}

	record, err := s.sessionSnapshotRepo.GetByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		slog.WarnContext(ctx, "load delegated coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"runtime_run_id", strings.TrimSpace(event.RunID),
			"event_type", event.Type,
			"error", err,
		)
		return s.eventProtocol != "v2"
	}

	var snapshot *model.CodingSessionStreamSnapshot
	if record != nil {
		snapshot, err = model.DecodeCodingSessionStreamSnapshot(record.SnapshotPayload)
		if err != nil {
			slog.WarnContext(ctx, "decode delegated coding session stream snapshot failed",
				"run_id", run.ID,
				"workspace_id", run.WorkspaceID,
				"runtime_run_id", strings.TrimSpace(event.RunID),
				"event_type", event.Type,
				"error", err,
			)
			record = nil
		}
		if snapshot != nil && record != nil && record.ThroughSequence > snapshot.ThroughSequence {
			snapshot.ThroughSequence = record.ThroughSequence
		}
	}
	if s.eventProtocol == "v2" && event.SequenceNo > 0 && snapshot != nil && snapshot.ThroughSequence >= event.SequenceNo {
		return false
	}

	snapshot = model.ApplyCodingSessionStreamEvent(snapshot, eventType, event.Data, s.eventTime(event))
	if snapshot != nil && s.eventProtocol == "v2" && event.SequenceNo > snapshot.ThroughSequence {
		snapshot.ThroughSequence = event.SequenceNo
	}
	if snapshot == nil || snapshot.IsEmpty() {
		if record != nil {
			if err := s.sessionSnapshotRepo.DeleteByRun(ctx, run.WorkspaceID, run.ID); err != nil {
				slog.WarnContext(ctx, "delete delegated empty coding session stream snapshot failed",
					"run_id", run.ID,
					"workspace_id", run.WorkspaceID,
					"runtime_run_id", strings.TrimSpace(event.RunID),
					"event_type", event.Type,
					"error", err,
				)
			}
		}
		return s.eventProtocol != "v2"
	}

	encoded, err := model.EncodeCodingSessionStreamSnapshot(snapshot)
	if err != nil {
		slog.WarnContext(ctx, "encode delegated coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"runtime_run_id", strings.TrimSpace(event.RunID),
			"event_type", event.Type,
			"error", err,
		)
		return s.eventProtocol != "v2"
	}

	nextRecord := &model.CodingSessionStateSnapshot{
		WorkspaceID:     run.WorkspaceID,
		RunID:           run.ID,
		SchemaVersion:   model.CodingSessionStateSnapshotSchemaVersionV1,
		ThroughSequence: snapshot.ThroughSequence,
		SnapshotPayload: encoded,
	}
	if record != nil {
		nextRecord.ID = record.ID
		nextRecord.CreatedAt = record.CreatedAt
	}
	applied, err := s.sessionSnapshotRepo.UpsertIfNewer(ctx, nextRecord)
	if err != nil {
		slog.WarnContext(ctx, "persist delegated coding session stream snapshot failed",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"runtime_run_id", strings.TrimSpace(event.RunID),
			"event_type", event.Type,
			"error", err,
		)
		return s.eventProtocol != "v2"
	}
	// Stream snapshots are surfaced through coding_session_event websocket
	// events. Do not emit a generic agent_run update for every transcript
	// delta; that causes run-summary refetch storms in sheet/dock views.
	return applied
}

func (s *AgentRuntimeProjectionService) publishRuntimeCodingSessionEvent(run *model.AgentRun, event AgentRuntimeEventEnvelope) {
	if s == nil || s.wsPublisher == nil || run == nil {
		return
	}
	eventType := codingSessionEventTypeFromAgentRuntimeEvent(event)
	if strings.TrimSpace(eventType) == "" {
		return
	}
	eventID := firstNonEmptyString(strings.TrimSpace(event.EventID), fmt.Sprintf("%s:%d", run.ID, time.Now().UTC().UnixNano()))
	payload := runtimeCodingSessionEventPayload(event)
	source := "agent-runtime-v1"
	if s.eventProtocol == "v2" {
		source = "agent-runtime-v2"
	}
	envelope, _ := json.Marshal(model.CodingSessionEvent{
		ID:              eventID,
		SessionID:       run.ID,
		RunID:           run.ID,
		SequenceNo:      runtimeCodingSessionSequence(event),
		Timestamp:       s.eventTime(event),
		Type:            eventType,
		RuntimeKind:     run.RuntimeKind,
		Payload:         payload,
		RuntimeMetadata: map[string]any{"source": source},
	})
	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "coding_session_event",
		EntityID:    eventID,
		WorkspaceID: run.WorkspaceID,
		ParentType:  "coding_session",
		ParentID:    run.ID,
		Data:        envelope,
	})
}

func runtimeCodingSessionSequence(event AgentRuntimeEventEnvelope) int {
	if event.SequenceNo <= 0 {
		return int(time.Now().UTC().UnixMilli())
	}
	if event.SequenceNo > int64(maxIntValue) {
		return int(maxIntValue)
	}
	return int(event.SequenceNo)
}

func runtimeCodingSessionEventPayload(event AgentRuntimeEventEnvelope) map[string]any {
	payload := make(map[string]any, len(event.Data)+2)
	for key, value := range event.Data {
		payload[key] = value
	}
	// Text deltas are byte fragments, not metadata. Leading whitespace and even
	// a whitespace-only fragment are meaningful token content and must survive
	// the Agent Runtime -> websocket projection unchanged.
	text := eventDataRawString(event.Data, "text")
	if text == "" {
		text = eventDataRawString(event.Data, "content")
	}
	if text != "" {
		payload["text"] = text
		content := eventDataRawString(event.Data, "content")
		if content == "" {
			content = text
		}
		payload["content"] = content
	}
	if strings.TrimSpace(event.HostRunID) != "" {
		payload["host_run_id"] = strings.TrimSpace(event.HostRunID)
	}
	return payload
}

func runtimeSubjectToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9', char == '-', char == '_':
			builder.WriteRune(char)
		default:
			builder.WriteByte('_')
		}
	}
	return builder.String()
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
		if agentRunMessageHasRuntimeMessageID(message, runtimeMessageID) {
			if !applyRuntimeMessageProjection(&message, runtimeMessage, runtimeMessageID) {
				return nil
			}
			if err := s.runMessageRepo.Update(ctx, &message); err != nil {
				return err
			}
			s.notifyRunChange(ctx, run, model.AgentRunChangeMessage)
			return nil
		}
		if agentRunMessageMatchesRuntimeMessage(message, runtimeMessage, runtimeMessageID) {
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
		WorkspaceID:      run.WorkspaceID,
		RunID:            run.ID,
		DockChatID:       run.DockChatID,
		DeliveryStatus:   "sent",
		RuntimeMessageID: runtimeMessageID,
		Role:             role,
		Content:          strings.TrimSpace(runtimeMessage.Content),
		MessageType:      messageType,
		ContentBlocks:    annotateRuntimeMessageBlocks(runtimeMessage.ContentBlocks, runtimeMessageID, runtimeMessage.Content),
		ToolInvocations:  runtimeMessage.ToolInvocations,
		SequenceNo:       sequenceNo,
	}
	if role == "assistant" {
		message.TurnSegments = runtimeMessageTurnSegments(runtimeMessageID, message.Content, runtimeMessage.ToolInvocations)
	}
	if !runtimeMessage.CreatedAt.IsZero() {
		message.CreatedAt = runtimeMessage.CreatedAt.UTC()
	}
	if err := s.runMessageRepo.Create(ctx, message); err != nil {
		return err
	}
	s.notifyRunChange(ctx, run, model.AgentRunChangeMessage)
	return nil
}

func applyRuntimeMessageProjection(message *model.AgentRunMessage, runtimeMessage AgentRuntimeMessage, runtimeMessageID string) bool {
	if message == nil {
		return false
	}
	role := strings.TrimSpace(runtimeMessage.Role)
	if role == "" {
		role = "assistant"
	}
	messageType := strings.TrimSpace(runtimeMessage.MessageType)
	if messageType == "" {
		messageType = "message"
	}
	if role == "assistant" && messageType == "message" && strings.TrimSpace(message.MessageType) == "assistant_turn" {
		messageType = "assistant_turn"
	}
	content := strings.TrimSpace(runtimeMessage.Content)
	if content == "" {
		content = message.Content
	}
	contentBlocks := message.ContentBlocks
	if len(runtimeMessage.ContentBlocks) > 0 {
		contentBlocks = annotateRuntimeMessageBlocks(runtimeMessage.ContentBlocks, runtimeMessageID, content)
	}
	toolInvocations := message.ToolInvocations
	if len(runtimeMessage.ToolInvocations) > 0 {
		toolInvocations = append(json.RawMessage(nil), runtimeMessage.ToolInvocations...)
	}
	var turnSegments json.RawMessage
	if role == "assistant" {
		turnSegments = runtimeMessageTurnSegments(runtimeMessageID, content, toolInvocations)
	}

	changed := false
	if message.RuntimeMessageID != runtimeMessageID {
		message.RuntimeMessageID = runtimeMessageID
		changed = true
	}
	if message.Role != role {
		message.Role = role
		changed = true
	}
	if message.Content != content {
		message.Content = content
		changed = true
	}
	if message.MessageType != messageType {
		message.MessageType = messageType
		changed = true
	}
	if !agentRuntimeProjectionJSONRawEqual(message.ContentBlocks, contentBlocks) {
		message.ContentBlocks = contentBlocks
		changed = true
	}
	if !agentRuntimeProjectionJSONRawEqual(message.TurnSegments, turnSegments) {
		message.TurnSegments = turnSegments
		changed = true
	}
	if !agentRuntimeProjectionJSONRawEqual(message.ToolInvocations, toolInvocations) {
		message.ToolInvocations = toolInvocations
		changed = true
	}
	return changed
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
	s.notifyRunChange(ctx, run, model.AgentRunChangeArtifact)
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
			if err := s.interactionRepo.Update(ctx, &updated); err != nil {
				return err
			}
			s.publishRuntimeInteractionEvent(run, updated)
			return nil
		}
		return nil
	}
	interactionKind := projectedRuntimeInteractionKind(runtimeInteraction, run.RuntimeKind)
	interaction := &model.AgentRunInteraction{
		WorkspaceID:          run.WorkspaceID,
		RunID:                run.ID,
		RuntimeKind:          firstNonEmptyString(strings.TrimSpace(runtimeInteraction.RuntimeKind), strings.TrimSpace(run.RuntimeKind)),
		InteractionKind:      interactionKind,
		Status:               normalizeRuntimeInteractionStatus(runtimeInteraction.Status),
		RequestSchemaVersion: projectedRuntimeInteractionSchemaVersion(runtimeInteraction, run.RuntimeKind, interactionKind),
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
	s.publishRuntimeInteractionEvent(run, *interaction)
	s.notifyRunChange(ctx, run, model.AgentRunChangeInteraction)
	return nil
}

func (s *AgentRuntimeProjectionService) publishRuntimeInteractionEvent(run *model.AgentRun, interaction model.AgentRunInteraction) {
	if s == nil || s.wsPublisher == nil || run == nil {
		return
	}
	for _, item := range codingSessionEventsFromInteraction(interaction) {
		payload, _ := json.Marshal(model.CodingSessionEvent{
			ID:              item.id,
			SessionID:       run.ID,
			RunID:           run.ID,
			SequenceNo:      interactionAssistantSequenceNo(interaction),
			Timestamp:       item.timestamp,
			Type:            item.eventType,
			RuntimeKind:     run.RuntimeKind,
			Payload:         item.payload,
			RuntimeMetadata: item.runtimeMetadata,
		})
		s.wsPublisher.Publish(websocket.Event{
			Action:      "created",
			Entity:      "coding_session_event",
			EntityID:    item.id,
			WorkspaceID: run.WorkspaceID,
			ParentType:  "coding_session",
			ParentID:    run.ID,
			Data:        payload,
		})
	}
}

func agentRunMessageHasRuntimeMessageID(message model.AgentRunMessage, runtimeMessageID string) bool {
	runtimeMessageID = strings.TrimSpace(runtimeMessageID)
	if runtimeMessageID == "" {
		return false
	}
	if strings.TrimSpace(message.RuntimeMessageID) == runtimeMessageID {
		return true
	}
	return jsonRawContainsStringField(message.ContentBlocks, "runtime_message_id", runtimeMessageID)
}

func agentRunMessageMatchesRuntimeMessage(message model.AgentRunMessage, runtimeMessage AgentRuntimeMessage, runtimeMessageID string) bool {
	if strings.TrimSpace(runtimeMessage.ID) != "" && strings.TrimSpace(runtimeMessage.ID) != strings.TrimSpace(runtimeMessageID) && agentRunMessageHasRuntimeMessageID(message, runtimeMessage.ID) {
		return true
	}
	// Different authoritative IDs denote different messages, even when their
	// text is identical (provider prose and a canonical answer, or later turns).
	// Text matching below is only a fallback for uncorrelated legacy messages.
	if strings.TrimSpace(message.RuntimeMessageID) != "" && strings.TrimSpace(runtimeMessage.RuntimeMessageID) != "" {
		return false
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

// lookupRuntimeStoreMessage fetches the runtime's persisted copy of a message
// by its runtime message ID. Best-effort: any failure degrades to the
// event-payload-only mirror rather than blocking projection.
func (s *AgentRuntimeProjectionService) lookupRuntimeStoreMessage(ctx context.Context, run *model.AgentRun, runtimeMessageID string) *AgentRuntimeMessage {
	if s == nil || s.agentRuntimeClient == nil || run == nil {
		return nil
	}
	runtimeRunID := strings.TrimSpace(derefString(run.ExternalRuntimeID))
	if runtimeRunID == "" || strings.TrimSpace(runtimeMessageID) == "" {
		return nil
	}
	messages, err := s.agentRuntimeClient.ListMessages(ctx, runtimeRunID)
	if err != nil {
		slog.WarnContext(ctx, "agent runtime store message lookup failed",
			"workspace_id", run.WorkspaceID,
			"run_id", run.ID,
			"runtime_run_id", runtimeRunID,
			"error", err,
		)
		return nil
	}
	for index := range messages {
		if strings.TrimSpace(messages[index].RuntimeMessageID) == strings.TrimSpace(runtimeMessageID) {
			return &messages[index]
		}
	}
	return nil
}

// runtimeToolInvocation mirrors the runtime's persisted tool invocation shape
// (agent-runtime internal/runtime native tool invocations).
type runtimeToolInvocation struct {
	ToolCallID          string          `json:"tool_call_id"`
	ToolName            string          `json:"tool_name"`
	Input               json.RawMessage `json:"input"`
	OutputSummary       string          `json:"output_summary"`
	DurationMs          int64           `json:"duration_ms"`
	Status              string          `json:"status"`
	Error               string          `json:"error"`
	AssistantBeforeTool bool            `json:"assistant_before_tool"`
}

// runtimeMessageTurnSegments maps runtime tool invocations plus the assistant
// text into the turn_segments shape the transcript UI renders inline
// (model.CodingSessionLiveTurnSegment), matching the local executor's output.
func runtimeMessageTurnSegments(runtimeMessageID, content string, toolInvocations json.RawMessage) json.RawMessage {
	var invocations []runtimeToolInvocation
	if len(toolInvocations) > 0 {
		_ = json.Unmarshal(toolInvocations, &invocations)
	}
	if len(invocations) == 0 {
		return nil
	}
	segments := make([]model.CodingSessionLiveTurnSegment, 0, len(invocations)+1)
	assistantAdded := false
	appendAssistant := func() {
		if assistantAdded {
			return
		}
		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			return
		}
		segments = append(segments, model.CodingSessionLiveTurnSegment{
			SegmentID: runtimeMessageID,
			Kind:      "assistant_message",
			AssistantMessage: &model.CodingSessionLiveAssistantMessage{
				MessageID: runtimeMessageID,
				Content:   trimmed,
				Status:    "completed",
			},
		})
		assistantAdded = true
	}
	for index, invocation := range invocations {
		if invocation.AssistantBeforeTool {
			appendAssistant()
		}
		segmentID := strings.TrimSpace(invocation.ToolCallID)
		if segmentID == "" {
			segmentID = fmt.Sprintf("%s-tool-%d", runtimeMessageID, index)
		}
		status := strings.ToLower(strings.TrimSpace(invocation.Status))
		if status != "failed" {
			if strings.TrimSpace(invocation.Error) != "" {
				status = "failed"
			} else {
				status = "completed"
			}
		}
		toolCall := &model.CodingSessionLiveToolCall{
			ToolCallID: segmentID,
			ToolName:   strings.TrimSpace(invocation.ToolName),
			ArgsText:   strings.TrimSpace(string(invocation.Input)),
			Status:     status,
		}
		if invocation.DurationMs > 0 {
			duration := invocation.DurationMs
			toolCall.DurationMs = &duration
		}
		if summary, errorText := strings.TrimSpace(invocation.OutputSummary), strings.TrimSpace(invocation.Error); summary != "" || errorText != "" {
			toolCall.Result = &model.CodingSessionLiveToolResult{Content: summary}
			if errorText != "" {
				toolCall.Result.Error = &errorText
			}
		}
		segments = append(segments, model.CodingSessionLiveTurnSegment{
			SegmentID: segmentID,
			Kind:      "tool_call",
			ToolCall:  toolCall,
		})
	}
	appendAssistant()
	payload, err := json.Marshal(segments)
	if err != nil {
		return nil
	}
	return payload
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
	projectedKind := projectedRuntimeInteractionKind(runtimeInteraction, run.RuntimeKind)
	if projectedKind == model.AgentRunInteractionKindApprovalRequest && isCodexNativeApprovalInteractionKind(interaction.InteractionKind) {
		projectedKind = strings.TrimSpace(interaction.InteractionKind)
	}
	if strings.TrimSpace(interaction.InteractionKind) != projectedKind {
		interaction.InteractionKind = projectedKind
		changed = true
	}
	if value := projectedRuntimeInteractionSchemaVersion(runtimeInteraction, run.RuntimeKind, projectedKind); strings.TrimSpace(interaction.RequestSchemaVersion) != value {
		interaction.RequestSchemaVersion = value
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

func projectedRuntimeInteractionKind(interaction AgentRuntimeInteraction, fallbackRuntimeKind string) string {
	kind := normalizeRuntimeInteractionKind(interaction.InteractionKind)
	if kind != model.AgentRunInteractionKindApprovalRequest || firstNonEmptyString(strings.TrimSpace(interaction.RuntimeKind), strings.TrimSpace(fallbackRuntimeKind)) != "codex" {
		return kind
	}
	var metadata struct {
		CodexRequestKind string `json:"codex_request_kind"`
	}
	if err := json.Unmarshal(interaction.ResponsePayload, &metadata); err != nil {
		return kind
	}
	switch strings.TrimSpace(metadata.CodexRequestKind) {
	case "command_execution":
		return model.AgentRunInteractionKindCommandExecutionApproval
	case "file_change":
		return model.AgentRunInteractionKindFileChangeApproval
	case "permissions":
		return model.AgentRunInteractionKindPermissionsApproval
	default:
		return kind
	}
}

func projectedRuntimeInteractionSchemaVersion(interaction AgentRuntimeInteraction, fallbackRuntimeKind, kind string) string {
	if firstNonEmptyString(strings.TrimSpace(interaction.RuntimeKind), strings.TrimSpace(fallbackRuntimeKind)) == "codex" && isCodexNativeApprovalInteractionKind(kind) {
		return model.AgentRunInteractionSchemaVersionCodexV2
	}
	return model.AgentRunInteractionSchemaVersionHelpinV1
}

func isCodexNativeApprovalInteractionKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case model.AgentRunInteractionKindCommandExecutionApproval,
		model.AgentRunInteractionKindFileChangeApproval,
		model.AgentRunInteractionKindPermissionsApproval:
		return true
	default:
		return false
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

func codingSessionEventTypeFromAgentRuntimeEvent(event AgentRuntimeEventEnvelope) string {
	switch strings.TrimSpace(event.Type) {
	case "run.started", "run.resumed", "run.paused", "run.completed", "run.failed", "run.cancelled":
		if eventDataString(event.Data, "completion_mode") == "explicit" {
			return event.Type
		}
		return ""
	case agentruntime.EventAssistantMessageStarted:
		return "assistant.message.started"
	case agentruntime.EventAssistantMessageDelta:
		return "assistant.message.delta"
	case agentruntime.EventAssistantMessageCompleted:
		return "assistant.message.completed"
	case agentruntime.EventReasoningMessageStarted:
		return "reasoning.message.started"
	case agentruntime.EventReasoningMessageDelta:
		return "reasoning.message.delta"
	case agentruntime.EventReasoningMessageCompleted:
		return "reasoning.message.completed"
	case agentruntime.EventToolCallStarted:
		return "tool.call.started"
	case agentruntime.EventToolCallArgsDelta:
		return "tool.call.args.delta"
	case agentruntime.EventToolCallResult:
		return "tool.call.result"
	case agentruntime.EventToolCallFinished:
		if strings.TrimSpace(eventDataString(event.Data, "error")) != "" {
			return "tool.call.failed"
		}
		return "tool.call.completed"
	case agentruntime.EventPlanUpdated:
		return "plan.updated"
	default:
		return ""
	}
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

func isRuntimeWorkProgressEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case agentruntime.EventAssistantMessageStarted,
		agentruntime.EventAssistantMessageDelta,
		agentruntime.EventAssistantMessageCompleted,
		agentruntime.EventReasoningMessageStarted,
		agentruntime.EventReasoningMessageDelta,
		agentruntime.EventReasoningMessageCompleted,
		agentruntime.EventToolCallStarted,
		agentruntime.EventToolCallArgsDelta,
		agentruntime.EventToolCallResult,
		agentruntime.EventToolCallFinished,
		agentruntime.EventPlanUpdated,
		agentruntime.EventUsageCheckpoint:
		return true
	default:
		return false
	}
}

func isRuntimeHumanPause(run *model.AgentRun) bool {
	if run == nil || strings.TrimSpace(run.Status) != model.AgentRunStatusPaused {
		return false
	}
	switch strings.TrimSpace(run.PauseReason) {
	case model.AgentRunPauseReasonHumanInput,
		model.AgentRunPauseReasonHumanApproval,
		model.AgentRunPauseReasonUserMessage:
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

func clearRuntimeResumeStage(run *model.AgentRun) bool {
	if run == nil || run.ExecutionStage == nil {
		return false
	}
	switch strings.TrimSpace(*run.ExecutionStage) {
	case "resuming", "approved", "feedback_received", "input_received", agentRuntimeExecutionStageAuthCompleted:
		run.ExecutionStage = nil
		return true
	default:
		return false
	}
}

func normalizeRuntimePauseReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case model.AgentRunPauseReasonHumanApproval:
		return model.AgentRunPauseReasonHumanApproval
	case model.AgentRunPauseReasonUserMessage:
		return model.AgentRunPauseReasonUserMessage
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

func eventDataRawString(data map[string]any, key string) string {
	if len(data) == 0 {
		return ""
	}
	value, ok := data[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func eventDataMap(data map[string]any, key string) map[string]any {
	if len(data) == 0 {
		return nil
	}
	value, ok := data[key]
	if !ok {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
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
		payload.TotalTokens = payload.InputTokens + payload.OutputTokens
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
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
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
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	if usage.TotalTokens == 0 && usage.InputTokens == 0 && usage.CachedInputTokens == 0 && usage.OutputTokens == 0 && usage.ReasoningOutputTokens == 0 {
		return agentRuntimeUsagePayload{}, false
	}
	return usage, true
}

func shouldCheckpointPausedDockChat(run *model.AgentRun, eventType string) bool {
	if run == nil || run.DockChatID == nil || run.Status != model.AgentRunStatusPaused ||
		run.PauseReason != model.AgentRunPauseReasonUserMessage {
		return false
	}
	switch strings.TrimSpace(eventType) {
	case agentruntime.EventRunPaused, agentruntime.EventUsageCheckpoint:
		return true
	default:
		return false
	}
}

func storeLatestAgentRuntimeUsage(run *model.AgentRun, usage agentRuntimeUsagePayload) bool {
	if run == nil {
		return false
	}
	body := map[string]any{}
	if len(run.OutputSummary) > 0 && strings.TrimSpace(string(run.OutputSummary)) != "null" {
		_ = json.Unmarshal(run.OutputSummary, &body)
	}
	if previous, ok := body[agentRuntimeLatestUsageSummaryKey].(map[string]any); ok {
		if stored, ok := usageFromMap(previous); ok && stored == usage {
			return false
		}
	}
	body[agentRuntimeLatestUsageSummaryKey] = map[string]int{
		"total_tokens":            usage.TotalTokens,
		"input_tokens":            usage.InputTokens,
		"cached_input_tokens":     usage.CachedInputTokens,
		"output_tokens":           usage.OutputTokens,
		"reasoning_output_tokens": usage.ReasoningOutputTokens,
	}
	payload, err := json.Marshal(body)
	if err != nil || agentRuntimeProjectionJSONRawEqual(run.OutputSummary, payload) {
		return false
	}
	run.OutputSummary = payload
	return true
}

func latestAgentRuntimeUsage(run *model.AgentRun) (agentRuntimeUsagePayload, bool) {
	if run == nil {
		return agentRuntimeUsagePayload{}, false
	}
	if len(run.OutputSummary) > 0 {
		var body map[string]json.RawMessage
		if json.Unmarshal(run.OutputSummary, &body) == nil {
			var values map[string]any
			if json.Unmarshal(body[agentRuntimeLatestUsageSummaryKey], &values) == nil {
				if usage, ok := usageFromMap(values); ok {
					return usage, true
				}
			}
		}
	}
	usage := agentRuntimeUsagePayload{
		TotalTokens:       run.TokensUsed,
		InputTokens:       run.InputTokens,
		CachedInputTokens: run.CachedInputTokens,
		OutputTokens:      run.OutputTokens,
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	return usage, !agentRunUsageIsZero(usage)
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

// notifyRunChange preserves compatibility with repository adapters that only
// support the original, unspecified run notification.
func (s *AgentRuntimeProjectionService) notifyRunChange(ctx context.Context, run *model.AgentRun, kind model.AgentRunChangeKind) {
	if repo, ok := s.runRepo.(interface {
		NotifyChange(context.Context, *model.AgentRun, model.AgentRunChangeKind)
	}); ok {
		repo.NotifyChange(ctx, run, kind)
		return
	}
	s.runRepo.Notify(ctx, run)
}
