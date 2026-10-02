package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
)

// ErrReanalysisAlreadyRunning is returned when a reanalysis workflow is
// already in progress for the workspace.
var ErrReanalysisAlreadyRunning = errors.New("reanalysis already in progress")

const (
	coverageDailyBatchWorkflowID = "coverage-gap-daily-batch"
	coverageDailyBatchSchedule   = "0 3 * * *"

	SupportCoverageAgentOutcomeResolved    = "resolved"
	SupportCoverageAgentOutcomeReviewReady = "review_ready"
	SupportCoverageAgentOutcomeRouted      = "routed"
	SupportCoverageAgentOutcomeBlocked     = "blocked"

	// Compatibility values accepted from in-flight runs using the original
	// completion contract. New tool schemas do not advertise these values.
	SupportCoverageAgentOutcomeProposalSubmitted = "proposal_submitted"
	SupportCoverageAgentOutcomeHandoff           = "handoff"

	SupportCoverageAgentActionDocumentCreated        = "document_created"
	SupportCoverageAgentActionDocumentUpdated        = "document_updated"
	SupportCoverageAgentActionProposalSubmitted      = "proposal_submitted"
	SupportCoverageAgentActionExistingDocsSufficient = "existing_docs_sufficient"
	SupportCoverageAgentActionFeatureNotFound        = "feature_not_found"
	SupportCoverageAgentActionNonDocGap              = "non_doc_gap"
	SupportCoverageAgentActionSourceUnavailable      = "source_unavailable"

	SupportCoverageAgentSourceVerified      = "verified"
	SupportCoverageAgentSourceNotFound      = "not_found"
	SupportCoverageAgentSourceNotApplicable = "not_applicable"
	SupportCoverageAgentSourceUnavailable   = "unavailable"
)

// SupportCoverageService orchestrates gap detection, evidence
// collection, and summary computation from support events.
type SupportCoverageService struct {
	coverageRepo *repository.SupportCoverageRepository
	clusterer    *SupportCoverageClusterer
	temporal     tclient.Client
	workflow     coverageWorkflowRunner
	logger       *slog.Logger
}

type coverageWorkflowRunner interface {
	StartEnrichment(ctx context.Context, topicID string, unique bool) error
	StartDailyBatch(ctx context.Context) error
}

type temporalCoverageWorkflowRunner struct {
	client tclient.Client
}

func (r *temporalCoverageWorkflowRunner) StartEnrichment(ctx context.Context, topicID string, unique bool) error {
	if r == nil || r.client == nil {
		return nil
	}
	workflowID := "coverage-gap-enrich-" + topicID
	if unique {
		workflowID = workflowID + "-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	_, err := r.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: temporalapp.QueueAutomation,
	}, temporalapp.CoverageGapEnrichmentWorkflowType, topicID)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("start coverage gap enrichment workflow: %w", err)
	}
	return nil
}

func (r *temporalCoverageWorkflowRunner) StartDailyBatch(ctx context.Context) error {
	if r == nil || r.client == nil {
		return nil
	}
	_, err := r.client.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:           coverageDailyBatchWorkflowID,
		TaskQueue:    temporalapp.QueueAutomation,
		CronSchedule: coverageDailyBatchSchedule,
	}, temporalapp.CoverageGapDailyBatchWorkflowType)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("start coverage gap daily enrichment workflow: %w", err)
	}
	return nil
}

// NewSupportCoverageService creates a new SupportCoverageService.
func NewSupportCoverageService(
	coverageRepo *repository.SupportCoverageRepository,
) *SupportCoverageService {
	return &SupportCoverageService{
		coverageRepo: coverageRepo,
		clusterer:    NewSupportCoverageClusterer(coverageRepo),
		logger:       slog.Default().With("service", "support_coverage"),
	}
}

func (s *SupportCoverageService) SetTemporalClient(client tclient.Client) {
	s.temporal = client
	s.workflow = &temporalCoverageWorkflowRunner{client: client}
}

func (s *SupportCoverageService) SetCoverageWorkflowRunner(runner coverageWorkflowRunner) {
	s.workflow = runner
}

func (s *SupportCoverageService) SetDocsBlockService(blockSvc *DocsBlockService) {
	if s.clusterer != nil {
		s.clusterer.SetDocsBlockService(blockSvc)
	}
}

func (s *SupportCoverageService) EnsureDailyEnrichment(ctx context.Context) error {
	if s.workflow == nil {
		return nil
	}
	return s.workflow.StartDailyBatch(ctx)
}

// ProcessSupportEvent evaluates a persisted event against v1 rules
// and creates/upserts gaps with evidence.
func (s *SupportCoverageService) ProcessSupportEvent(ctx context.Context, event *model.SupportEvent) error {
	if event.EventType == model.SupportEventAIAnswerFeedback {
		return s.attachVisitorAnswerFeedback(ctx, event)
	}
	now := time.Now()

	// For human_reply_after_ai and conversation_resolved_by_human, try to
	// attach evidence to the existing gap for this conversation before
	// creating a new one. This keeps the agent's answer (and the
	// resolution marker) alongside the original AI failure evidence, so
	// one conversation produces one gap regardless of how many gap-worthy
	// signals fired during its lifetime.
	isResolvedByHumanSignal := event.EventType == model.SupportEventConversationResolved &&
		event.SourceSignal == model.SupportCoverageSourceConversationResolvedByHuman
	if (event.EventType == model.SupportEventHumanReplyAfterAI || isResolvedByHumanSignal) && event.ConversationID != nil {
		sourceKey := coverageEventEvidenceSourceKey(event.ID)
		if sourceKey != "" {
			alreadyLinked, err := s.coverageRepo.FindGapByEvidenceSourceKey(ctx, event.WorkspaceID, sourceKey)
			if err != nil {
				s.logger.WarnContext(ctx, "find gap by event evidence source key failed", "error", err, "workspace_id", event.WorkspaceID, "event_id", event.ID)
			}
			if alreadyLinked != nil {
				return nil
			}
		}
		existing, err := s.coverageRepo.FindOpenGapByConversation(ctx, event.WorkspaceID, *event.ConversationID)
		if err != nil {
			s.logger.WarnContext(ctx, "find gap by conversation failed", "error", err)
		}
		if existing != nil {
			evidence := &model.SupportGapEvidence{
				GapID:          existing.ID,
				WorkspaceID:    event.WorkspaceID,
				EvidenceType:   event.EventType,
				ConversationID: event.ConversationID,
				MessageID:      event.MessageID,
				SourceSignal:   event.SourceSignal,
				SourceKey:      sourceKey,
				Excerpt:        coverageTruncate(event.IssueSummary, 500),
				CreatedAt:      now,
			}
			inserted, err := s.coverageRepo.CreateEvidenceIfAbsent(ctx, evidence)
			if err != nil {
				s.logger.WarnContext(ctx, "attach human reply evidence failed", "error", err)
				return nil
			}
			if inserted {
				if err := s.coverageRepo.IncrementGapEvidenceAfterInsert(ctx, event.WorkspaceID, existing.ID, now); err != nil {
					s.logger.WarnContext(ctx, "increment human reply evidence count failed", "error", err, "gap_id", existing.ID)
				}
			}
			return nil // Evidence attached to existing gap, no new gap needed.
		}
		// No existing gap for this conversation — fall through to normal rule processing,
		// unless the daily LLM analyzer is already active for this workspace.
		analyzed, err := s.coverageRepo.HasCompletedAnalysisRun(ctx, event.WorkspaceID)
		if err != nil {
			s.logger.WarnContext(ctx, "check analysis run status", "error", err, "workspace_id", event.WorkspaceID)
		}
		if analyzed {
			return nil
		}
	}

	gap, err := s.clusterer.UpsertTopicGap(ctx, event)
	if err != nil {
		return err
	}
	s.maybeTriggerSpikeEnrichment(ctx, gap)
	return nil
}

func (s *SupportCoverageService) maybeTriggerSpikeEnrichment(ctx context.Context, gap *model.SupportCoverageGap) {
	if gap == nil || gap.TopicID == nil || s.workflow == nil {
		return
	}
	count, err := s.coverageRepo.CountEvidenceSince(ctx, gap.ID, time.Now().Add(-time.Hour))
	if err != nil {
		s.logger.WarnContext(ctx, "spike trigger evidence count failed", "error", err, "gap_id", gap.ID)
		return
	}
	if count < 5 {
		return
	}
	topic, err := s.coverageRepo.GetTopic(ctx, *gap.TopicID)
	if err != nil {
		s.logger.WarnContext(ctx, "spike trigger topic load failed", "error", err, "topic_id", *gap.TopicID)
		return
	}
	if topic == nil {
		return
	}
	if topic.CooldownUntil != nil && topic.CooldownUntil.After(time.Now()) {
		return
	}
	if err := s.workflow.StartEnrichment(ctx, topic.ID, false); err != nil {
		s.logger.ErrorContext(ctx, "spike trigger workflow start failed", "error", err, "topic_id", topic.ID)
	}
}

// GetSummary returns aggregate coverage metrics for a workspace.
func (s *SupportCoverageService) GetSummary(ctx context.Context, workspaceID string) (*model.SupportCoverageSummary, error) {
	return s.coverageRepo.GetSummary(ctx, workspaceID)
}

// ListGaps returns gaps for a workspace.
func (s *SupportCoverageService) ListGaps(ctx context.Context, workspaceID string, filter model.SupportCoverageGapFilter) ([]model.SupportCoverageGapListItem, int64, error) {
	items, total, err := s.coverageRepo.ListGaps(ctx, workspaceID, filter)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].ImpactTier = ImpactTier(items[i].Evidence30d)
	}
	return items, total, nil
}

func ImpactTier(evidence30d int) string {
	switch {
	case evidence30d >= 10:
		return "high"
	case evidence30d >= 3:
		return "medium"
	default:
		return "low"
	}
}

func (s *SupportCoverageService) ListWorkspacesWithOpenGaps(ctx context.Context) ([]string, error) {
	return s.coverageRepo.ListWorkspacesWithOpenGaps(ctx)
}

func (s *SupportCoverageService) ListTopicsDueForEnrichment(ctx context.Context, workspaceID string, olderThan time.Duration, minEvidence int) ([]string, error) {
	return s.coverageRepo.ListTopicsDueForEnrichment(ctx, workspaceID, olderThan, minEvidence)
}

// GetGapDetail returns a gap with its evidence, suggestions, and related articles.
func (s *SupportCoverageService) GetGapDetail(ctx context.Context, workspaceID, gapID string) (*model.SupportCoverageGapDetail, error) {
	detail, err := s.coverageRepo.GetGapDetail(ctx, workspaceID, gapID)
	if detail != nil {
		detail.ImpactTier = ImpactTier(detail.Evidence30d)
	}
	return detail, err
}

func (s *SupportCoverageService) RegenerateGap(ctx context.Context, workspaceID, gapID string) error {
	if s.workflow == nil {
		return fmt.Errorf("enrichment workflow is not configured")
	}
	detail, err := s.coverageRepo.GetGapDetail(ctx, workspaceID, gapID)
	if err != nil {
		return err
	}
	if detail == nil {
		return fmt.Errorf("gap not found")
	}
	if detail.TopicID == nil || *detail.TopicID == "" {
		return fmt.Errorf("gap is not topic-scoped")
	}
	return s.workflow.StartEnrichment(ctx, *detail.TopicID, true)
}

// UpdateGapStatus sets the status of a gap.
func (s *SupportCoverageService) UpdateGapStatus(ctx context.Context, workspaceID, gapID, status, userID string, issueResolved *bool) error {
	return s.coverageRepo.UpdateGapStatus(ctx, workspaceID, gapID, status, userID, issueResolved)
}

func (s *SupportCoverageService) RejectGap(ctx context.Context, workspaceID, gapID, userID string, rejectionReason *string) error {
	evidence30d, err := s.coverageRepo.CountEvidence30d(ctx, gapID)
	if err != nil {
		return err
	}
	return s.coverageRepo.MarkGapRejected(ctx, workspaceID, gapID, userID, rejectionReason, evidence30d)
}

func (s *SupportCoverageService) AddDocumentToGap(ctx context.Context, workspaceID, gapID, documentID string) error {
	if documentID == "" {
		return fmt.Errorf("document_id is required")
	}
	evidence30d, err := s.coverageRepo.CountEvidence30d(ctx, gapID)
	if err != nil {
		return err
	}
	if err := s.coverageRepo.MarkGapDone(ctx, workspaceID, gapID, documentID, evidence30d); err != nil {
		return err
	}
	return s.coverageRepo.LinkGapArticle(ctx, gapID, documentID, workspaceID)
}

// RecordAgentOutcome idempotently applies the durable result of a
// documentation-agent run to its support coverage gap.
func (s *SupportCoverageService) RecordAgentOutcome(ctx context.Context, workspaceID, gapID, outcome, documentID string) error {
	if s == nil || s.coverageRepo == nil {
		return fmt.Errorf("support coverage service is not configured")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	gapID = strings.TrimSpace(gapID)
	outcome = strings.TrimSpace(outcome)
	documentID = strings.TrimSpace(documentID)
	if workspaceID == "" || gapID == "" {
		return fmt.Errorf("workspace_id and gap_id are required")
	}
	detail, err := s.coverageRepo.GetGapDetail(ctx, workspaceID, gapID)
	if err != nil {
		return err
	}
	if detail == nil {
		return fmt.Errorf("gap not found")
	}

	switch outcome {
	case SupportCoverageAgentOutcomeRouted:
		if documentID != "" {
			return s.coverageRepo.LinkGapArticle(ctx, gapID, documentID, workspaceID)
		}
		return nil
	case SupportCoverageAgentOutcomeBlocked, SupportCoverageAgentOutcomeHandoff:
		return nil
	case SupportCoverageAgentOutcomeReviewReady, SupportCoverageAgentOutcomeProposalSubmitted:
		if documentID == "" {
			return fmt.Errorf("document_id is required for review_ready")
		}
		return s.coverageRepo.LinkGapArticle(ctx, gapID, documentID, workspaceID)
	case SupportCoverageAgentOutcomeResolved:
		if documentID == "" {
			return fmt.Errorf("document_id is required for resolved")
		}
		if detail.Status == model.SupportCoverageGapStatusDone {
			if detail.ResultDocumentID != nil && strings.TrimSpace(*detail.ResultDocumentID) != documentID {
				return fmt.Errorf("gap is already resolved with a different document")
			}
			return s.coverageRepo.LinkGapArticle(ctx, gapID, documentID, workspaceID)
		}
		if detail.Status != model.SupportCoverageGapStatusOpen {
			return fmt.Errorf("gap is not open")
		}
		return s.AddDocumentToGap(ctx, workspaceID, gapID, documentID)
	default:
		return fmt.Errorf("outcome must be resolved, review_ready, routed, or blocked")
	}
}

func IsGapResolutionConflict(err error) bool {
	return errors.Is(err, repository.ErrGapAlreadyClosed)
}

// ReclassifyGap changes the v1 gap type.
func (s *SupportCoverageService) ReclassifyGap(ctx context.Context, workspaceID, gapID, v1GapType string) error {
	switch v1GapType {
	case model.SupportCoverageV1GapMissingArticle, model.SupportCoverageV1GapWeakArticle,
		model.SupportCoverageV1GapOutdatedOrConflictingArticle, model.SupportCoverageV1GapNeedsReview:
	default:
		return fmt.Errorf("unsupported gap classification")
	}
	return s.coverageRepo.ReclassifyGap(ctx, workspaceID, gapID, v1GapType)
}

// MergeGaps merges source gap into target.
func (s *SupportCoverageService) MergeGaps(ctx context.Context, workspaceID, sourceGapID, targetGapID string) error {
	return s.coverageRepo.MergeGaps(ctx, workspaceID, sourceGapID, targetGapID)
}

// DiscardSuggestion rejects a suggestion and reverts the gap to open.
func (s *SupportCoverageService) DiscardSuggestion(ctx context.Context, workspaceID, suggestionID string) error {
	return s.coverageRepo.DiscardSuggestion(ctx, suggestionID, workspaceID)
}

// TriggerReanalysis starts a workspace coverage analysis workflow with
// a 30-day window so that old conversations are re-evaluated against the
// current analyzer version.
func (s *SupportCoverageService) TriggerReanalysis(ctx context.Context, workspaceID string) error {
	if s.temporal == nil {
		return fmt.Errorf("temporal client is not configured")
	}
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	now := time.Now().UTC()
	windowStart := now.Add(-30 * 24 * time.Hour)
	input := temporalapp.CoverageWorkspaceAnalysisInput{
		WorkspaceID: workspaceID,
		WindowStart: windowStart,
		WindowEnd:   now,
	}
	workflowID := "coverage-reanalysis-" + workspaceID
	_, err := s.temporal.ExecuteWorkflow(ctx, tclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: temporalapp.QueueAutomation,
	}, temporalapp.CoverageWorkspaceAnalysisWorkflowType, input)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return ErrReanalysisAlreadyRunning
		}
		return fmt.Errorf("start coverage reanalysis workflow: %w", err)
	}
	slog.InfoContext(ctx, "triggered coverage reanalysis", "workspace_id", workspaceID, "workflow_id", workflowID)
	return nil
}

// GetConversationCoverageState checks docs-issue feedback state.
func (s *SupportCoverageService) GetConversationCoverageState(ctx context.Context, workspaceID, conversationID string) (*model.SupportConversationCoverageState, error) {
	return s.coverageRepo.GetConversationCoverageState(ctx, workspaceID, conversationID)
}

func coverageTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
