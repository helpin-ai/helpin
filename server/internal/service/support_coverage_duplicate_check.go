package service

import (
	"context"
	"log/slog"
)

// coverageDuplicateChecker groups near-duplicate gaps.
type coverageDuplicateChecker interface {
	RebuildWorkspace(ctx context.Context, workspaceID string) (*SupportCoverageClusterRebuildResult, error)
}

// SetDuplicateChecker runs the duplicate check after each analysis that adds
// gaps, so near-duplicates are merged without anyone pressing a button.
func (s *SupportCoverageDailyAnalyzer) SetDuplicateChecker(checker coverageDuplicateChecker) *SupportCoverageDailyAnalyzer {
	if s != nil {
		s.duplicateChecker = checker
	}
	return s
}

// checkDuplicatesAfterRun never fails the analysis run; a failed check is
// retried after the next run that adds gaps, or by the manual check.
func (s *SupportCoverageDailyAnalyzer) checkDuplicatesAfterRun(ctx context.Context, workspaceID string, materialized *CoverageMaterializationResult) {
	if s.duplicateChecker == nil || materialized == nil {
		return
	}
	if materialized.NewGapsCreated == 0 && coverageReanalysisRequest(ctx) == "" {
		return
	}
	result, err := s.duplicateChecker.RebuildWorkspace(ctx, workspaceID)
	if err != nil {
		slog.WarnContext(ctx, "coverage duplicate check after analysis failed", "workspace_id", workspaceID, "error", err)
		return
	}
	slog.InfoContext(ctx, "coverage duplicate check after analysis", "workspace_id", workspaceID,
		"merged", result.AutoMerged, "kept_separate", result.KeptSeparate, "left_for_review", result.SuggestionsCreated)
}
