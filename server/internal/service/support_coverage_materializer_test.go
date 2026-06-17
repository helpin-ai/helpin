package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeCoverageEmbeddingProvider struct {
	vectors [][]float32
}

func (f *fakeCoverageEmbeddingProvider) CreateEmbeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	if len(f.vectors) >= len(req.Inputs) {
		return &llm.EmbeddingResponse{Vectors: f.vectors[:len(req.Inputs)]}, nil
	}
	vectors := make([][]float32, 0, len(req.Inputs))
	for idx := range req.Inputs {
		if idx < len(f.vectors) {
			vectors = append(vectors, f.vectors[idx])
			continue
		}
		vectors = append(vectors, []float32{1, 0, 0})
	}
	return &llm.EmbeddingResponse{Vectors: vectors}, nil
}

func TestCoverageMaterializerClustersSameRunFindingsIntoOneGap(t *testing.T) {
	db := setupCoverageFindingUpsertTestDB(t)
	analysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
	coverageRepo := repository.NewSupportCoverageRepository(db)
	analyzer := NewSupportCoverageDailyAnalyzer(nil, "", "").
		SetCoverageRepositories(coverageRepo, analysisRepo).
		SetEmbeddingProvider(&fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}, {0.99, 0.01, 0}}}, "")
	ctx := context.Background()
	now := time.Now()

	seedMaterializerAnalysis(t, analysisRepo, "analysis-1", "run-1", "conversation-1", "Customers need reset emails that arrive.", "Password reset email delivery", now)
	seedMaterializerAnalysis(t, analysisRepo, "analysis-2", "run-1", "conversation-2", "Customers need reset emails that arrive.", "Reset emails are not delivered", now.Add(time.Minute))

	result, err := analyzer.materializeRunFindings(ctx, "ws-1", "run-1")
	if err != nil {
		t.Fatalf("materializeRunFindings: %v", err)
	}
	if result.NewGapsCreated != 1 {
		t.Fatalf("NewGapsCreated=%d, want 1", result.NewGapsCreated)
	}
	if result.EvidenceInserted != 2 {
		t.Fatalf("EvidenceInserted=%d, want 2", result.EvidenceInserted)
	}

	var gaps []model.SupportCoverageGap
	if err := db.Find(&gaps).Error; err != nil {
		t.Fatalf("list gaps: %v", err)
	}
	if len(gaps) != 1 {
		t.Fatalf("gaps=%d, want 1", len(gaps))
	}
	if gaps[0].EvidenceCount != 2 {
		t.Fatalf("EvidenceCount=%d, want 2", gaps[0].EvidenceCount)
	}
	var analyses []model.SupportCoverageConversationAnalysis
	if err := db.Order("id ASC").Find(&analyses).Error; err != nil {
		t.Fatalf("list analyses: %v", err)
	}
	for _, analysis := range analyses {
		if analysis.GapID == nil || *analysis.GapID != gaps[0].ID {
			t.Fatalf("analysis %s gap_id=%v, want %s", analysis.ID, analysis.GapID, gaps[0].ID)
		}
	}
}

func TestCoverageMaterializerRetryDoesNotDuplicateEvidenceOrCount(t *testing.T) {
	db := setupCoverageFindingUpsertTestDB(t)
	analysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
	coverageRepo := repository.NewSupportCoverageRepository(db)
	analyzer := NewSupportCoverageDailyAnalyzer(nil, "", "").
		SetCoverageRepositories(coverageRepo, analysisRepo).
		SetEmbeddingProvider(&fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}}}, "")
	ctx := context.Background()
	now := time.Now()

	seedMaterializerAnalysis(t, analysisRepo, "analysis-1", "run-1", "conversation-1", "Customers need refund exception rules.", "Refund exception criteria", now)
	if _, err := analyzer.materializeRunFindings(ctx, "ws-1", "run-1"); err != nil {
		t.Fatalf("first materializeRunFindings: %v", err)
	}
	if _, err := analyzer.materializeRunFindings(ctx, "ws-1", "run-1"); err != nil {
		t.Fatalf("second materializeRunFindings: %v", err)
	}

	var gap model.SupportCoverageGap
	if err := db.First(&gap).Error; err != nil {
		t.Fatalf("load gap: %v", err)
	}
	if gap.EvidenceCount != 1 {
		t.Fatalf("EvidenceCount=%d, want 1", gap.EvidenceCount)
	}
	var evidenceCount int64
	if err := db.Model(&model.SupportGapEvidence{}).Count(&evidenceCount).Error; err != nil {
		t.Fatalf("count evidence: %v", err)
	}
	if evidenceCount != 1 {
		t.Fatalf("evidence rows=%d, want 1", evidenceCount)
	}
}

func TestCoverageMaterializerAttachesHighConfidenceFindingToExistingGap(t *testing.T) {
	db := setupCoverageFindingUpsertTestDB(t)
	analysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
	coverageRepo := repository.NewSupportCoverageRepository(db)
	analyzer := NewSupportCoverageDailyAnalyzer(nil, "", "").
		SetCoverageRepositories(coverageRepo, analysisRepo).
		SetEmbeddingProvider(&fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}}}, "")
	ctx := context.Background()
	now := time.Now()

	existing, _, err := coverageRepo.UpsertOpenGapByDedupeKeyNoBump(ctx, &model.SupportCoverageGap{
		ID:                  "gap-existing",
		WorkspaceID:         "ws-1",
		DedupeKey:           "semantic:existing",
		Title:               "Refund exception criteria",
		Status:              model.SupportCoverageGapStatusOpen,
		GapKind:             "content",
		GapCategory:         model.SupportCoverageGapCategoryKnowledge,
		FirstSeenAt:         now.Add(-time.Hour),
		LastSeenAt:          now.Add(-time.Hour),
		Embedding:           "[1,0,0]",
		EmbeddingProvider:   coverageEmbeddingProviderName,
		EmbeddingModel:      coverageDefaultEmbeddingModel,
		EmbeddingVersion:    coverageGapEmbeddingVersion,
		EmbeddingDimensions: 3,
		EmbeddingTextHash:   "existing-hash",
		EmbeddingUpdatedAt:  &now,
	})
	if err != nil {
		t.Fatalf("seed existing gap: %v", err)
	}
	seedMaterializerAnalysis(t, analysisRepo, "analysis-1", "run-1", "conversation-1", "Customers need refund exception criteria.", "Refund exception criteria", now)

	result, err := analyzer.materializeRunFindings(ctx, "ws-1", "run-1")
	if err != nil {
		t.Fatalf("materializeRunFindings: %v", err)
	}
	if result.ExistingGapAttached != 1 {
		t.Fatalf("ExistingGapAttached=%d, want 1", result.ExistingGapAttached)
	}
	if result.NewGapsCreated != 0 {
		t.Fatalf("NewGapsCreated=%d, want 0", result.NewGapsCreated)
	}

	var gapCount int64
	if err := db.Model(&model.SupportCoverageGap{}).Count(&gapCount).Error; err != nil {
		t.Fatalf("count gaps: %v", err)
	}
	if gapCount != 1 {
		t.Fatalf("gap count=%d, want 1", gapCount)
	}
	var analysis model.SupportCoverageConversationAnalysis
	if err := db.First(&analysis, "id = ?", "analysis-1").Error; err != nil {
		t.Fatalf("load analysis: %v", err)
	}
	if analysis.GapID == nil || *analysis.GapID != existing.ID {
		t.Fatalf("analysis gap_id=%v, want %s", analysis.GapID, existing.ID)
	}
}

func seedMaterializerAnalysis(t *testing.T, repo *repository.SupportCoverageAnalysisRepository, id, runID, conversationID, customerNeed, canonicalTitle string, createdAt time.Time) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"has_gap": true, "customer_need": customerNeed, "canonical_title": canonicalTitle})
	if err := repo.RecordConversationAnalysis(context.Background(), &model.SupportCoverageConversationAnalysis{
		ID:              id,
		WorkspaceID:     "ws-1",
		RunID:           runID,
		ConversationID:  conversationID,
		Status:          model.SupportCoverageConversationAnalysisStatusAnalyzed,
		HasGap:          true,
		GapKind:         "content",
		GapCategory:     model.SupportCoverageGapCategoryKnowledge,
		TranscriptHash:  id + "-hash",
		AnalyzerVersion: coverageAnalyzerVersion,
		CanonicalTitle:  canonicalTitle,
		CustomerNeed:    customerNeed,
		DecisionReason:  "The answer should be materialized as a coverage gap.",
		Confidence:      0.86,
		RawOutput:       raw,
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	}); err != nil {
		t.Fatalf("seed analysis %s: %v", id, err)
	}
}
