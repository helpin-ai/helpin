package service

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
)

var (
	ErrVoiceUnavailable  = errors.New("voice input is not configured")
	ErrVoiceInvalidAudio = errors.New("record a clip between 0.25 seconds and 5 minutes")
	ErrVoiceBusy         = errors.New("a recording is already being transcribed; try again shortly")
	ErrVoiceProvider     = errors.New("could not transcribe the recording; please try again")
)

const MaxVoiceUploadBytes = 44 + int(aiusage.MaxVoiceMilliseconds)*32

type VoiceTranscriber interface {
	Transcribe(context.Context, []byte) (llm.TranscriptionResult, error)
}
type VoiceInputService struct {
	provider VoiceTranscriber
	usage    aiusage.AIUsageLifecycle
	registry *aipolicy.Registry
	audit    aipolicy.ExecutionAudit
	mu       sync.Mutex
	active   map[string]bool
}

func NewVoiceInputService(provider VoiceTranscriber, usage aiusage.AIUsageLifecycle, registry *aipolicy.Registry, audit aipolicy.ExecutionAudit) *VoiceInputService {
	return &VoiceInputService{provider: provider, usage: usage, registry: registry, audit: audit, active: map[string]bool{}}
}
func (s *VoiceInputService) Available() bool {
	return s != nil && s.provider != nil && s.usage != nil && s.audit != nil && s.registry != nil
}

func (s *VoiceInputService) Transcribe(ctx context.Context, workspaceID, userID string, audio []byte) (llm.TranscriptionResult, error) {
	if !s.Available() {
		return llm.TranscriptionResult{}, ErrVoiceUnavailable
	}
	duration, err := voiceWAVDuration(audio)
	if err != nil {
		return llm.TranscriptionResult{}, err
	}
	if workspaceID == "" || userID == "" {
		return llm.TranscriptionResult{}, ErrVoiceUnavailable
	}
	key := workspaceID + ":" + userID
	s.mu.Lock()
	if s.active[key] || len(s.active) >= 16 {
		s.mu.Unlock()
		return llm.TranscriptionResult{}, ErrVoiceBusy
	}
	s.active[key] = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.active, key); s.mu.Unlock() }()
	identity := "voice:" + uuid.NewString()
	action, err := aipolicy.ResolveExecution(s.registry, aipolicy.ExecutionContext{WorkspaceID: workspaceID, ActionKey: aipolicy.ActionVoiceTranscription, IdempotencyKey: identity, Attempt: 1}, aipolicy.Route{Provider: "openrouter", Model: llm.OpenRouterTranscriptionModel})
	if err != nil {
		return llm.TranscriptionResult{}, ErrVoiceUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, action.Timeout)
	defer cancel()
	metering, err := s.usage.Preflight(ctx, aiusage.PreflightRequest{Metering: aiusage.MeteringRequest{
		WorkspaceID: workspaceID, TaskNature: "voice", FeatureKey: action.FeatureKey, OperationKey: aiusage.OperationVoiceTranscription,
		Provider: "openrouter", Model: llm.OpenRouterTranscriptionModel, IdempotencyKey: identity, ExecutionID: identity,
		FundingMode: aiusage.FundingHelpinHosted, AudioMillisecondsEstimate: duration,
	}})
	if err != nil {
		return llm.TranscriptionResult{}, err
	}
	// Finalize accounting even if the browser disconnects after OpenRouter responds.
	cleanup := func(fn func(context.Context) error) {
		finalCtx, finalCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer finalCancel()
		if e := fn(finalCtx); e != nil {
			slog.ErrorContext(finalCtx, "voice input accounting failed", "execution", identity)
		}
	}
	metadata, _ := json.Marshal(map[string]any{"audio_milliseconds": duration, "user_id": userID})
	execution, err := s.audit.Start(ctx, &model.AIActionExecution{
		WorkspaceID: workspaceID, ActionKey: action.Key, PolicyVersion: action.PolicyVersion, FeatureKey: action.FeatureKey,
		Category: string(action.Category), Origin: action.Origin, Modality: string(action.Modality), Provider: "openrouter", Model: llm.OpenRouterTranscriptionModel,
		IdempotencyKey: identity, Attempt: 1, Status: model.AIActionExecutionRunning, Metadata: metadata, StartedAt: time.Now().UTC(),
	})
	if err != nil {
		cleanup(func(c context.Context) error { return s.usage.Fail(c, metering.ReservationID) })
		return llm.TranscriptionResult{}, ErrVoiceUnavailable
	}
	result, callErr := s.provider.Transcribe(ctx, audio)
	finalResult := aipolicy.ExecutionResult{Status: model.AIActionExecutionSucceeded, CompletedAt: time.Now().UTC()}
	if callErr != nil {
		finalResult.Status = model.AIActionExecutionFailed
		finalResult.FailureClass = "provider_error"
		finalResult.FailureMessage = ErrVoiceProvider.Error()
		cleanup(func(c context.Context) error { return s.usage.Fail(c, metering.ReservationID) })
	} else {
		if result.AudioMilliseconds == 0 {
			// OpenRouter may omit duration. Validated PCM samples are an exact
			// local measurement, not a guessed token or duration estimate.
			result.AudioMilliseconds = duration
		}
		cleanup(func(c context.Context) error {
			_, e := s.usage.Reconcile(c, aiusage.CompletionUsage{Context: *metering, AudioMilliseconds: result.AudioMilliseconds, MeasurementStatus: "actual"})
			if e != nil {
				callErr = e
				finalResult.Status = model.AIActionExecutionFailed
				finalResult.FailureClass = "usage_error"
				finalResult.FailureMessage = "Unable to record usage"
			}
			return e
		})
	}
	cleanup(func(c context.Context) error { return s.audit.Finish(c, execution.ID, finalResult) })
	if callErr != nil {
		return llm.TranscriptionResult{}, ErrVoiceProvider
	}
	return result, nil
}

// Accept only the canonical 16 kHz mono PCM WAV emitted by our recorder. Checking
// the actual sample count prevents an untrusted duration field bypassing admission.
func voiceWAVDuration(b []byte) (int64, error) {
	if len(b) < 44 ||
		len(b) > MaxVoiceUploadBytes ||
		string(b[:4]) != "RIFF" ||
		int(binary.LittleEndian.Uint32(b[4:8])) != len(b)-8 ||
		string(b[8:16]) != "WAVEfmt " ||
		binary.LittleEndian.Uint32(b[16:20]) != 16 ||
		binary.LittleEndian.Uint16(b[20:22]) != 1 ||
		binary.LittleEndian.Uint16(b[22:24]) != 1 ||
		binary.LittleEndian.Uint32(b[24:28]) != 16000 ||
		binary.LittleEndian.Uint32(b[28:32]) != 32000 ||
		binary.LittleEndian.Uint16(b[32:34]) != 2 ||
		binary.LittleEndian.Uint16(b[34:36]) != 16 ||
		string(b[36:40]) != "data" ||
		int(binary.LittleEndian.Uint32(b[40:44])) != len(b)-44 ||
		(len(b)-44)%2 != 0 {
		return 0, ErrVoiceInvalidAudio
	}
	duration := (int64(len(b)-44)*1000 + 31999) / 32000
	if duration < 250 ||
		duration > aiusage.MaxVoiceMilliseconds {
		return 0, ErrVoiceInvalidAudio
	}
	return duration, nil
}
