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

	"github.com/nats-io/nats.go"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	agentRuntimeEventsStreamName  = "AGENT_RUNTIME_EVENTS"
	agentRuntimeProjectionDurable = "helpin-agent-runtime-projection"
)

var errAgentRuntimeProjectionRunNotFound = errors.New("agent runtime projection run not found")

var maxIntValue = int64(^uint(0) >> 1)

type agentRuntimeProjectionRunRepository interface {
	GetByIDAny(ctx context.Context, id string) (*model.AgentRun, error)
	GetByExternalRuntimeID(ctx context.Context, externalRuntime, externalRuntimeID string) (*model.AgentRun, error)
	Update(ctx context.Context, run *model.AgentRun) error
	Notify(ctx context.Context, run *model.AgentRun)
}

// AgentRuntimeProjectionService projects Agent Runtime lifecycle events back
// into Helpin's agent_runs table and existing realtime fanout.
type AgentRuntimeProjectionService struct {
	runRepo agentRuntimeProjectionRunRepository
	appID   string
	now     func() time.Time
}

type AgentRuntimeEventEnvelope struct {
	EventID    string         `json:"event_id"`
	SentAt     time.Time      `json:"sent_at"`
	SequenceNo int64          `json:"sequence_no"`
	AppID      string         `json:"app_id"`
	RunID      string         `json:"run_id"`
	HostRunID  string         `json:"host_run_id,omitempty"`
	Type       string         `json:"type"`
	Data       map[string]any `json:"data,omitempty"`
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

func (s *AgentRuntimeProjectionService) subscribeNATS(js nats.JetStreamContext) (*nats.Subscription, error) {
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

func (s *AgentRuntimeProjectionService) processNATSMessage(ctx context.Context, msg *nats.Msg) {
	var event AgentRuntimeEventEnvelope
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		slog.WarnContext(ctx, "agent runtime projection invalid payload", "subject", msg.Subject, "error", err)
		_ = msg.Ack()
		return
	}
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
	case "run.queued":
		if !suppressLifecycle {
			changed = setRunStatus(run, model.AgentRunStatusQueued, model.AgentRunPauseReasonNone) || changed
		}
	case "run.started":
		if !suppressLifecycle {
			if run.StartedAt == nil {
				run.StartedAt = &now
				changed = true
			}
			run.CompletedAt = nil
			changed = setRunStatus(run, model.AgentRunStatusRunning, model.AgentRunPauseReasonNone) || changed
		}
	case "run.resumed":
		run.CompletedAt = nil
		changed = setRunStatus(run, model.AgentRunStatusRunning, model.AgentRunPauseReasonNone) || changed
	case "run.paused":
		if !suppressLifecycle {
			pauseReason := normalizeRuntimePauseReason(eventDataString(event.Data, "pause_reason"))
			changed = setRunStatus(run, model.AgentRunStatusPaused, pauseReason) || changed
		}
	case "run.completed":
		if run.CompletedAt == nil {
			run.CompletedAt = &now
			changed = true
		}
		changed = setRunStatus(run, model.AgentRunStatusCompleted, model.AgentRunPauseReasonNone) || changed
	case "run.failed":
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
	case "run.cancelled":
		if run.CompletedAt == nil {
			run.CompletedAt = &now
			changed = true
		}
		changed = setRunStatus(run, model.AgentRunStatusCancelled, model.AgentRunPauseReasonNone) || changed
	case "usage.checkpoint":
	default:
		return nil
	}

	if usage, ok := eventUsage(event.Data); ok {
		if applyRuntimeUsage(run, usage) {
			changed = true
		}
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
	case "run.queued", "run.started", "run.paused":
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

func eventUsage(data map[string]any) (agentRuntimeUsagePayload, bool) {
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
