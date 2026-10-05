package service

import (
	"context"
	"fmt"
	"github.com/helpin-ai/helpin/server/internal/model"
	"strings"
	"time"
)

type coverageReanalysisContextKey struct{}

func withCoverageReanalysis(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, coverageReanalysisContextKey{}, requestID)
}
func coverageReanalysisRequest(ctx context.Context) string {
	value, _ := ctx.Value(coverageReanalysisContextKey{}).(string)
	return value
}
func coverageAttemptAnalyzerVersion(ctx context.Context) string {
	if requestID := coverageReanalysisRequest(ctx); requestID != "" {
		return coverageAnalyzerVersion + ":reanalysis:" + requestID
	}
	return coverageAnalyzerVersion
}
func (s *SupportCoverageDailyAnalyzer) RunWorkspaceReanalysis(ctx context.Context, workspaceID string, start, end time.Time, requestID string) error {
	if strings.TrimSpace(requestID) == "" {
		return fmt.Errorf("reanalysis request_id is required")
	}
	return s.RunWorkspaceDailyAnalysis(withCoverageReanalysis(ctx, requestID), workspaceID, start, end)
}
func (s *SupportCoverageDailyAnalyzer) recordCoverageConversationAnalysis(ctx context.Context, analysis *model.SupportCoverageConversationAnalysis) error {
	// A failed reassessment must not replace a previously successful result.
	if coverageReanalysisRequest(ctx) != "" && analysis.Status != model.SupportCoverageConversationAnalysisStatusFailed {
		return s.analysisRepo.RefreshConversationAnalysis(ctx, analysis)
	}
	return s.analysisRepo.RecordConversationAnalysis(ctx, analysis)
}
