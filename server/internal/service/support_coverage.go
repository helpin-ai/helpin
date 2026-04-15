package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// SupportCoverageService orchestrates gap detection, evidence
// collection, and summary computation from support events.
type SupportCoverageService struct {
	coverageRepo *repository.SupportCoverageRepository
	logger       *slog.Logger
}

// NewSupportCoverageService creates a new SupportCoverageService.
func NewSupportCoverageService(
	coverageRepo *repository.SupportCoverageRepository,
) *SupportCoverageService {
	return &SupportCoverageService{
		coverageRepo: coverageRepo,
		logger:       slog.Default().With("service", "support_coverage"),
	}
}

// ProcessSupportEvent evaluates a persisted event against v1 rules
// and creates/upserts gaps with evidence.
func (s *SupportCoverageService) ProcessSupportEvent(ctx context.Context, event *model.SupportEvent) error {
	rule := classifyEvent(event)
	if rule == nil {
		return nil // No gap-producing rule matched.
	}

	now := time.Now()

	// Upsert topic if issue key is available.
	var topicID *string
	if event.IssueKey != "" {
		topic, err := s.coverageRepo.UpsertTopicByIssueKey(ctx, event.WorkspaceID, event.IssueKey, event.IssueSummary)
		if err != nil {
			s.logger.WarnContext(ctx, "upsert topic failed", "error", err)
		} else if topic != nil {
			topicID = &topic.ID
		}
	}

	// Upsert gap.
	gap := &model.SupportCoverageGap{
		WorkspaceID:  event.WorkspaceID,
		TopicID:      topicID,
		DedupeKey:    rule.DedupeKey,
		GapCategory:  rule.GapCategory,
		V1GapType:    rule.V1GapType,
		Title:        rule.Title,
		IssueKey:     event.IssueKey,
		Status:       model.SupportCoverageGapStatusOpen,
		Confidence:   rule.Confidence,
		FailureMode:  event.FailureMode,
		SourceSignal: event.SourceSignal,
		CanAnswer:    event.CanAnswer,
		CanResolve:   event.CanResolve,
		FirstSeenAt:  now,
		LastSeenAt:   now,
	}

	upserted, _, err := s.coverageRepo.UpsertGapByDedupeKey(ctx, gap)
	if err != nil {
		return err
	}

	// Create evidence.
	evidence := &model.SupportGapEvidence{
		GapID:           upserted.ID,
		WorkspaceID:     event.WorkspaceID,
		EvidenceType:    event.EventType,
		ConversationID:  event.ConversationID,
		MessageID:       event.MessageID,
		WidgetSessionID: event.WidgetSessionID,
		DocumentID:      event.DocumentID,
		ArticlePublicID: event.ArticlePublicID,
		SourceSignal:    event.SourceSignal,
		Excerpt:         coverageTruncate(event.IssueSummary, 500),
		CreatedAt:       now,
	}
	if err := s.coverageRepo.CreateEvidence(ctx, evidence); err != nil {
		s.logger.WarnContext(ctx, "create evidence failed", "error", err)
	}

	// Link related article if document ID is present.
	if event.DocumentID != nil && *event.DocumentID != "" {
		if err := s.coverageRepo.LinkGapArticle(ctx, upserted.ID, *event.DocumentID, event.WorkspaceID); err != nil {
			s.logger.WarnContext(ctx, "link gap article failed", "error", err)
		}
	}

	return nil
}

// GetSummary returns aggregate coverage metrics for a workspace.
func (s *SupportCoverageService) GetSummary(ctx context.Context, workspaceID string) (*model.SupportCoverageSummary, error) {
	return s.coverageRepo.GetSummary(ctx, workspaceID)
}

// ListGaps returns gaps for a workspace.
func (s *SupportCoverageService) ListGaps(ctx context.Context, workspaceID string, filter model.SupportCoverageGapFilter) ([]model.SupportCoverageGapListItem, int64, error) {
	return s.coverageRepo.ListGaps(ctx, workspaceID, filter)
}

// GetGapDetail returns a gap with its evidence, suggestions, and related articles.
func (s *SupportCoverageService) GetGapDetail(ctx context.Context, workspaceID, gapID string) (*model.SupportCoverageGapDetail, error) {
	return s.coverageRepo.GetGapDetail(ctx, workspaceID, gapID)
}

// UpdateGapStatus sets the status of a gap.
func (s *SupportCoverageService) UpdateGapStatus(ctx context.Context, workspaceID, gapID, status string) error {
	return s.coverageRepo.UpdateGapStatus(ctx, workspaceID, gapID, status)
}

// ReclassifyGap changes the v1 gap type.
func (s *SupportCoverageService) ReclassifyGap(ctx context.Context, workspaceID, gapID, v1GapType string) error {
	return s.coverageRepo.ReclassifyGap(ctx, workspaceID, gapID, v1GapType)
}

// MergeGaps merges source gap into target.
func (s *SupportCoverageService) MergeGaps(ctx context.Context, workspaceID, sourceGapID, targetGapID string) error {
	return s.coverageRepo.MergeGaps(ctx, workspaceID, sourceGapID, targetGapID)
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
