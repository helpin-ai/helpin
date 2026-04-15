package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// SupportEventInput is the service-level input for recording a support event.
type SupportEventInput struct {
	WorkspaceID     string
	EventType       string
	ConversationID  *string
	MessageID       *string
	WidgetSessionID *string
	AnonymousID     *string
	DocumentID      *string
	ArticleID       *string
	ArticlePublicID *string
	ActorType       string
	Channel         string
	Source          string
	IssueKey        string
	IssueSummary    string
	FailureMode     string
	SourceSignal    string
	CanAnswer       string
	CanResolve      string
	Metadata        map[string]any
	OccurredAt      time.Time
}

// SupportEventService records shared operational support events.
type SupportEventService struct {
	eventRepo   *repository.SupportEventRepository
	coverageSvc *SupportCoverageService
	logger      *slog.Logger
}

// NewSupportEventService creates a new SupportEventService.
func NewSupportEventService(
	eventRepo *repository.SupportEventRepository,
	coverageSvc *SupportCoverageService,
) *SupportEventService {
	return &SupportEventService{
		eventRepo:   eventRepo,
		coverageSvc: coverageSvc,
		logger:      slog.Default().With("service", "support_events"),
	}
}

// RecordEvent writes a support event and passes it to coverage for
// gap derivation. Returns errors for tests and API use.
func (s *SupportEventService) RecordEvent(ctx context.Context, input SupportEventInput) error {
	event := s.inputToEvent(input)

	if err := s.eventRepo.Create(ctx, event); err != nil {
		return err
	}

	// Pass to coverage service for gap derivation.
	if s.coverageSvc != nil {
		if err := s.coverageSvc.ProcessSupportEvent(ctx, event); err != nil {
			s.logger.WarnContext(ctx, "coverage processing failed",
				"event_type", event.EventType, "error", err)
			// Non-fatal: event is already persisted.
		}
	}

	return nil
}

func (s *SupportEventService) inputToEvent(input SupportEventInput) *model.SupportEvent {
	occurred := input.OccurredAt
	if occurred.IsZero() {
		occurred = time.Now()
	}

	var metadataJSON json.RawMessage
	if input.Metadata != nil {
		if b, err := json.Marshal(input.Metadata); err == nil {
			metadataJSON = b
		}
	}
	if metadataJSON == nil {
		metadataJSON = []byte("{}")
	}

	var canAnswer, canResolve *string
	if input.CanAnswer != "" {
		canAnswer = &input.CanAnswer
	}
	if input.CanResolve != "" {
		canResolve = &input.CanResolve
	}

	return &model.SupportEvent{
		WorkspaceID:     input.WorkspaceID,
		EventType:       input.EventType,
		ConversationID:  input.ConversationID,
		MessageID:       input.MessageID,
		WidgetSessionID: input.WidgetSessionID,
		AnonymousID:     input.AnonymousID,
		DocumentID:      input.DocumentID,
		ArticleID:       input.ArticleID,
		ArticlePublicID: input.ArticlePublicID,
		ActorType:       input.ActorType,
		Channel:         input.Channel,
		Source:          input.Source,
		IssueKey:        input.IssueKey,
		IssueSummary:    input.IssueSummary,
		FailureMode:     input.FailureMode,
		SourceSignal:    input.SourceSignal,
		CanAnswer:       canAnswer,
		CanResolve:      canResolve,
		Metadata:        metadataJSON,
		OccurredAt:      occurred,
	}
}

// SupportEventRecorder is the interface consumed by hot-path services.
// Nil-safe: callers should check for nil before calling.
type SupportEventRecorder interface {
	RecordEventBestEffort(input SupportEventInput)
}

// SupportEventAsyncRecorder buffers events and writes them
// asynchronously. Dropping events on full queue prevents backpressure
// on support/docs hot paths.
type SupportEventAsyncRecorder struct {
	ch     chan SupportEventInput
	svc    *SupportEventService
	logger *slog.Logger
	done   chan struct{}
}

// NewSupportEventAsyncRecorder creates an async recorder with a bounded buffer.
func NewSupportEventAsyncRecorder(svc *SupportEventService, bufferSize int) *SupportEventAsyncRecorder {
	if bufferSize <= 0 {
		bufferSize = 250
	}
	r := &SupportEventAsyncRecorder{
		ch:     make(chan SupportEventInput, bufferSize),
		svc:    svc,
		logger: slog.Default().With("component", "support_event_recorder"),
		done:   make(chan struct{}),
	}
	go r.worker()
	return r
}

// RecordEventBestEffort enqueues an event for async writing.
// Drops and logs if the queue is full.
func (r *SupportEventAsyncRecorder) RecordEventBestEffort(input SupportEventInput) {
	if r == nil || r.svc == nil {
		return
	}
	select {
	case r.ch <- input:
	default:
		r.logger.Warn("support event queue full, dropping event",
			"workspace_id", input.WorkspaceID,
			"event_type", input.EventType,
			"reason", "support_event_queue_full")
	}
}

// Close drains the queue and stops the worker.
func (r *SupportEventAsyncRecorder) Close() {
	close(r.ch)
	<-r.done
}

func (r *SupportEventAsyncRecorder) worker() {
	defer close(r.done)
	for input := range r.ch {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := r.svc.RecordEvent(ctx, input); err != nil {
			r.logger.Warn("async event write failed",
				"workspace_id", input.WorkspaceID,
				"event_type", input.EventType,
				"error", err)
		}
		cancel()
	}
}
