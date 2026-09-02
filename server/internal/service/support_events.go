package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// maxVisitorAnonymousIDLength bounds a client-supplied browser identifier before
// it is persisted. The value is an opaque token, so anything longer is
// malformed or hostile rather than a longer legitimate id.
const maxVisitorAnonymousIDLength = 128

type supportSignalStarter interface {
	StartSignalDetection(ctx context.Context, sourceKey string, payloads []model.SignalSourcePayload) error
}

// NormalizeVisitorAnonymousID sanitizes an anonymous browser id supplied by an
// unauthenticated caller. It returns nil when the value is absent or unusable,
// so callers can assign the result directly to an optional event field.
//
// The value is never trusted as proof of identity — it only attributes an event
// to the browser that claimed it. Trust is established separately by the widget
// identify flow.
func NormalizeVisitorAnonymousID(raw string) *string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > maxVisitorAnonymousIDLength {
		return nil
	}
	return &trimmed
}

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
	eventRepo        *repository.SupportEventRepository
	coverageSvc      *SupportCoverageService
	coverageV2Repo   *repository.CoverageV2Repository
	summaryRefresh   CompanySummaryRefreshRequester
	messageRepo      *repository.SupportMessageRepository
	conversationRepo *repository.SupportConversationRepository
	signalStarter    supportSignalStarter
	logger           *slog.Logger
}

func (s *SupportEventService) SetCoverageV2Repository(repo *repository.CoverageV2Repository) *SupportEventService {
	if s != nil {
		s.coverageV2Repo = repo
	}
	return s
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

// SetSignalDetection enables CRM-signal detection for customer messages.
func (s *SupportEventService) SetSignalDetection(
	messageRepo *repository.SupportMessageRepository,
	conversationRepo *repository.SupportConversationRepository,
	starter supportSignalStarter,
) *SupportEventService {
	s.messageRepo, s.conversationRepo, s.signalStarter = messageRepo, conversationRepo, starter
	return s
}

// SetCompanySummaryRefresh enables linked-account invalidation for support activity.
func (s *SupportEventService) SetCompanySummaryRefresh(refresh CompanySummaryRefreshRequester) *SupportEventService {
	s.summaryRefresh = refresh
	return s
}

// RecordEvent writes a support event and passes it to coverage for
// gap derivation. Returns errors for tests and API use.
func (s *SupportEventService) RecordEvent(ctx context.Context, input SupportEventInput) error {
	event := s.inputToEvent(input)

	if err := s.eventRepo.Create(ctx, event); err != nil {
		return err
	}

	isQualifiedWidgetSearch := event.EventType == model.SupportEventWidgetSearchPerformed && event.SourceSignal == "no_results"
	routedWidgetSearchV2 := isQualifiedWidgetSearch && s.coverageV2Repo != nil
	if routedWidgetSearchV2 {
		sessionID := stringPointerValue(event.AnonymousID)
		if sessionID == "" {
			sessionID = stringPointerValue(event.WidgetSessionID)
		}
		normalized := strings.ToLower(strings.Join(strings.Fields(event.IssueSummary), " "))
		signal := &model.CoverageUnreviewedSignal{
			WorkspaceID: event.WorkspaceID, SourceKind: "widget_search", SourceID: event.ID,
			SessionID: sessionID, NormalizedQuery: normalized,
			SignalKey:        aiUsageStableHash(event.WorkspaceID + ":" + sessionID + ":" + normalized),
			MeaningfulTokens: MeaningfulCoverageSearchTokens(normalized), Status: model.CoverageSignalUnreviewed,
			ObservedAt: event.OccurredAt, Metadata: []byte(`{"surface":"widget_help","result_count":0}`),
		}
		if err := s.coverageV2Repo.UpsertUnreviewedSignal(ctx, signal); err != nil {
			s.logger.WarnContext(ctx, "coverage widget signal enqueue failed", "error", err, "event_id", event.ID)
		}
	}
	// Qualified searches enter the review queue and never become legacy open
	// gaps merely because a visitor paused while typing.
	if s.coverageSvc != nil && !routedWidgetSearchV2 {
		if err := s.coverageSvc.ProcessSupportEvent(ctx, event); err != nil {
			s.logger.WarnContext(ctx, "coverage processing failed",
				"event_type", event.EventType, "error", err)
			// Non-fatal: event is already persisted.
		}
	}
	if s.summaryRefresh != nil && event.ConversationID != nil && *event.ConversationID != "" {
		if err := s.summaryRefresh.RequestCompanyRefreshForObject(ctx, event.WorkspaceID, model.CRMObjectSupportConversation, *event.ConversationID); err != nil {
			s.logger.WarnContext(ctx, "company summary refresh request failed", "error", err, "conversation_id", *event.ConversationID)
		}
	}
	if event.EventType == model.SupportEventCustomerMessageCreated {
		if err := s.enqueueSupportSignalDetection(ctx, event); err != nil {
			s.logger.WarnContext(ctx, "support CRM signal enqueue failed", "error", err, "message_id", stringPointerValue(event.MessageID))
		}
	}

	return nil
}

func (s *SupportEventService) enqueueSupportSignalDetection(ctx context.Context, event *model.SupportEvent) error {
	if s.signalStarter == nil || s.messageRepo == nil || s.conversationRepo == nil || event.MessageID == nil || event.ConversationID == nil {
		return nil
	}
	message, err := s.messageRepo.GetByID(ctx, *event.MessageID)
	if err != nil || message == nil {
		return err
	}
	if message.SenderType != "customer" || message.IsInternal || message.MessageType != "reply" {
		return nil
	}
	conversation, err := s.conversationRepo.GetByID(ctx, event.WorkspaceID, *event.ConversationID, "", model.RoleOwner)
	if err != nil || conversation == nil {
		return err
	}
	payload := model.PayloadFromSupportMessage(message, conversation)
	return s.signalStarter.StartSignalDetection(ctx, "support-"+message.ID, []model.SignalSourcePayload{payload})
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
