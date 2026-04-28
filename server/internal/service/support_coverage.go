package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
)

const (
	coverageDailyBatchWorkflowID = "coverage-gap-daily-batch"
	coverageDailyBatchSchedule   = "0 3 * * *"
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

func (s *SupportCoverageService) EnsureDailyEnrichment(ctx context.Context) error {
	if s.workflow == nil {
		return nil
	}
	return s.workflow.StartDailyBatch(ctx)
}

// ProcessSupportEvent evaluates a persisted event against v1 rules
// and creates/upserts gaps with evidence.
func (s *SupportCoverageService) ProcessSupportEvent(ctx context.Context, event *model.SupportEvent) error {
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
				Excerpt:        coverageTruncate(event.IssueSummary, 500),
				CreatedAt:      now,
			}
			if err := s.coverageRepo.CreateEvidence(ctx, evidence); err != nil {
				s.logger.WarnContext(ctx, "attach human reply evidence failed", "error", err)
			}
			return nil // Evidence attached to existing gap, no new gap needed.
		}
		// No existing gap for this conversation — fall through to normal rule processing.
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
	return s.coverageRepo.ListGaps(ctx, workspaceID, filter)
}

func (s *SupportCoverageService) ListWorkspacesWithOpenGaps(ctx context.Context) ([]string, error) {
	return s.coverageRepo.ListWorkspacesWithOpenGaps(ctx)
}

func (s *SupportCoverageService) ListTopicsDueForEnrichment(ctx context.Context, workspaceID string, olderThan time.Duration, minEvidence int) ([]string, error) {
	return s.coverageRepo.ListTopicsDueForEnrichment(ctx, workspaceID, olderThan, minEvidence)
}

// GetGapDetail returns a gap with its evidence, suggestions, and related articles.
func (s *SupportCoverageService) GetGapDetail(ctx context.Context, workspaceID, gapID string) (*model.SupportCoverageGapDetail, error) {
	return s.coverageRepo.GetGapDetail(ctx, workspaceID, gapID)
}

// UpdateGapStatus sets the status of a gap.
func (s *SupportCoverageService) UpdateGapStatus(ctx context.Context, workspaceID, gapID, status, userID string, issueResolved *bool) error {
	return s.coverageRepo.UpdateGapStatus(ctx, workspaceID, gapID, status, userID, issueResolved)
}

// ReclassifyGap changes the v1 gap type.
func (s *SupportCoverageService) ReclassifyGap(ctx context.Context, workspaceID, gapID, v1GapType string) error {
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
