package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportCoverageSnapshotMetrics holds pre-computed coverage metrics.
type SupportCoverageSnapshotMetrics struct {
	TotalOpenGaps              int `json:"total_open_gaps"`
	NewGaps7d                  int `json:"new_gaps_7d"`
	FixedGaps7d                int `json:"fixed_gaps_7d"`
	TotalEvidence              int `json:"total_evidence"`
	HandoffsAfterFixes         int `json:"handoffs_after_fixes"`
	MissingArticleGaps         int `json:"missing_article_gaps"`
	WeakArticleGaps            int `json:"weak_article_gaps"`
	OutdatedOrConflictingGaps  int `json:"outdated_or_conflicting_gaps"`
	NeedsReviewGaps            int `json:"needs_review_gaps"`
}

// RefreshWorkspaceSnapshots computes and stores a coverage snapshot
// for the given workspace. Also cleans snapshots older than 90 days.
func (s *SupportCoverageService) RefreshWorkspaceSnapshots(ctx context.Context, workspaceID string, now time.Time) error {
	summary, err := s.coverageRepo.GetSummary(ctx, workspaceID)
	if err != nil {
		return err
	}

	// Count by gap type.
	var missingCount, weakCount, outdatedCount, reviewCount int64
	countByType := func(v1Type string) int64 {
		gaps, _, err := s.coverageRepo.ListGaps(ctx, workspaceID, model.SupportCoverageGapFilter{
			Status:    model.SupportCoverageGapStatusOpen,
			V1GapType: v1Type,
			PerPage:   1,
		})
		_ = gaps
		if err != nil {
			return 0
		}
		// Use total from list.
		_, total, _ := s.coverageRepo.ListGaps(ctx, workspaceID, model.SupportCoverageGapFilter{
			Status:    model.SupportCoverageGapStatusOpen,
			V1GapType: v1Type,
		})
		return total
	}
	missingCount = countByType(model.SupportCoverageV1GapMissingArticle)
	weakCount = countByType(model.SupportCoverageV1GapWeakArticle)
	outdatedCount = countByType(model.SupportCoverageV1GapOutdatedOrConflictingArticle)
	reviewCount = countByType(model.SupportCoverageV1GapNeedsReview)

	metrics := SupportCoverageSnapshotMetrics{
		TotalOpenGaps:             summary.TotalOpenGaps,
		NewGaps7d:                summary.NewGapsThisWeek,
		FixedGaps7d:              summary.GapsFixedThisWeek,
		MissingArticleGaps:       int(missingCount),
		WeakArticleGaps:          int(weakCount),
		OutdatedOrConflictingGaps: int(outdatedCount),
		NeedsReviewGaps:          int(reviewCount),
	}

	metricsJSON, err := json.Marshal(metrics)
	if err != nil {
		return err
	}

	snapshot := &model.SupportCoverageSnapshot{
		WorkspaceID: workspaceID,
		SnapshotAt:  now,
		Metrics:     metricsJSON,
	}
	if err := s.coverageRepo.CreateSnapshot(ctx, snapshot); err != nil {
		return err
	}

	// Clean old snapshots (90 days retention).
	if err := s.coverageRepo.CleanOldSnapshots(ctx, workspaceID, 90); err != nil {
		slog.WarnContext(ctx, "clean old snapshots failed", "workspace_id", workspaceID, "error", err)
	}

	return nil
}
