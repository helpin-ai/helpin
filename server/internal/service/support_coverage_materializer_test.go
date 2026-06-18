package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
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

func TestCoverageMaterializerReclassifiesStrongKBMatchAsRetrievalFailure(t *testing.T) {
	db := setupCoverageFindingUpsertTestDB(t)
	analysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
	coverageRepo := repository.NewSupportCoverageRepository(db)
	seedMaterializerDocsSpace(t, db, "space-public", "ws-1")
	if err := db.Create(&model.DocsChunk{
		ID:          "doc-chunk-reset",
		WorkspaceID: "ws-1",
		SpaceID:     "space-public",
		DocumentID:  "doc-reset",
		ChunkIndex:  0,
		Title:       "Password reset email troubleshooting",
		Content:     "Customers can resend password reset emails from account settings and should check spam folders.",
		UpdatedAt:   time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed docs chunk: %v", err)
	}
	analyzer := NewSupportCoverageDailyAnalyzer(nil, "", "").
		SetCoverageRepositories(coverageRepo, analysisRepo).
		SetEmbeddingProvider(&fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}}}, "").
		SetKnowledgeMatcher(
			NewCoverageKnowledgeMatcher(repository.NewDocsChunkRepository(db), nil, nil, ""),
			repository.NewDocsSpaceRepository(db),
			nil,
		)
	ctx := context.Background()
	now := time.Now()

	seedMaterializerAnalysis(t, analysisRepo, "analysis-1", "run-1", "conversation-1", "Customers need password reset emails that arrive.", "Password reset email troubleshooting", now)

	result, err := analyzer.materializeRunFindings(ctx, "ws-1", "run-1")
	if err != nil {
		t.Fatalf("materializeRunFindings: %v", err)
	}
	if result.NewGapsCreated != 1 {
		t.Fatalf("NewGapsCreated=%d, want 1", result.NewGapsCreated)
	}
	var gap model.SupportCoverageGap
	if err := db.First(&gap).Error; err != nil {
		t.Fatalf("load gap: %v", err)
	}
	if gap.FailureMode != model.SupportCoverageFailureNoRetrieval {
		t.Fatalf("FailureMode=%q, want %q", gap.FailureMode, model.SupportCoverageFailureNoRetrieval)
	}
	if gap.NearestContentScore <= 0 || gap.NearestContentDocumentID == nil || *gap.NearestContentDocumentID != "doc-reset" {
		t.Fatalf("nearest content not persisted: score=%f document=%v title=%q", gap.NearestContentScore, gap.NearestContentDocumentID, gap.NearestContentTitle)
	}
}

func TestCoverageMaterializerFlagsRecurrenceAfterDoneGap(t *testing.T) {
	db := setupCoverageFindingUpsertTestDB(t)
	analysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
	coverageRepo := repository.NewSupportCoverageRepository(db)
	analyzer := NewSupportCoverageDailyAnalyzer(nil, "", "").
		SetCoverageRepositories(coverageRepo, analysisRepo).
		SetEmbeddingProvider(&fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}}}, "")
	ctx := context.Background()
	now := time.Now()
	closedAt := now.Add(-24 * time.Hour)

	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:                  "gap-done",
		WorkspaceID:         "ws-1",
		DedupeKey:           "done",
		GapKind:             "content",
		GapCategory:         model.SupportCoverageGapCategoryKnowledge,
		Title:               "Password reset email troubleshooting",
		Status:              model.SupportCoverageGapStatusDone,
		Confidence:          0.9,
		EvidenceCount:       3,
		ClosedAt:            &closedAt,
		Metadata:            json.RawMessage(`{}`),
		FirstSeenAt:         now.Add(-48 * time.Hour),
		LastSeenAt:          closedAt,
		Embedding:           "[1,0,0]",
		EmbeddingProvider:   coverageEmbeddingProviderName,
		EmbeddingModel:      coverageDefaultEmbeddingModel,
		EmbeddingVersion:    coverageGapEmbeddingVersion,
		EmbeddingDimensions: 3,
		EmbeddingTextHash:   "done-hash",
		EmbeddingUpdatedAt:  &closedAt,
	})
	seedMaterializerAnalysis(t, analysisRepo, "analysis-1", "run-1", "conversation-1", "Customers need password reset emails that arrive.", "Password reset email troubleshooting", now)

	result, err := analyzer.materializeRunFindings(ctx, "ws-1", "run-1")
	if err != nil {
		t.Fatalf("materializeRunFindings: %v", err)
	}
	if result.NewGapsCreated != 0 || result.ExistingGapAttached != 1 {
		t.Fatalf("result=%+v, want recurrence evidence attached without new gap", result)
	}
	var gap model.SupportCoverageGap
	if err := db.First(&gap, "id = ?", "gap-done").Error; err != nil {
		t.Fatalf("load done gap: %v", err)
	}
	if gap.Status != model.SupportCoverageGapStatusDone {
		t.Fatalf("status=%q, want done until recurrence threshold", gap.Status)
	}
	var metadata map[string]any
	if err := json.Unmarshal(gap.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal gap metadata: %v", err)
	}
	if metadata["recurrence_watch"] != true || metadata["post_close_evidence_count"] != float64(1) {
		t.Fatalf("recurrence metadata missing: %+v", metadata)
	}
	var evidenceCount int64
	if err := db.Model(&model.SupportGapEvidence{}).Where("gap_id = ?", "gap-done").Count(&evidenceCount).Error; err != nil {
		t.Fatalf("count evidence: %v", err)
	}
	if evidenceCount != 1 {
		t.Fatalf("evidenceCount=%d, want 1", evidenceCount)
	}
}

func TestCoverageMaterializerFlagsPotentialOverAttachment(t *testing.T) {
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
		Title:               "Account access problems",
		Status:              model.SupportCoverageGapStatusOpen,
		GapKind:             "content",
		GapCategory:         model.SupportCoverageGapCategoryKnowledge,
		Metadata:            json.RawMessage(`{}`),
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
	for i, vector := range []string{"[1,0,0]", "[0.99,0.01,0]", "[0.98,0.02,0]", "[0,1,0]", "[0.01,0.99,0]", "[0.02,0.98,0]"} {
		gapID := existing.ID
		analysis := model.SupportCoverageConversationAnalysis{
			ID:                      fmt.Sprintf("seed-analysis-%d", i),
			WorkspaceID:             "ws-1",
			RunID:                   "old-run",
			ConversationID:          fmt.Sprintf("old-conversation-%d", i),
			Status:                  model.SupportCoverageConversationAnalysisStatusAnalyzed,
			HasGap:                  true,
			GapID:                   &gapID,
			TranscriptHash:          fmt.Sprintf("old-hash-%d", i),
			AnalyzerVersion:         coverageAnalyzerVersion,
			CanonicalTitle:          "Account access problems",
			CustomerNeed:            "Customers need account access help.",
			Confidence:              0.9,
			RawOutput:               json.RawMessage(`{}`),
			MaterializationMetadata: json.RawMessage(`{}`),
			Embedding:               vector,
			EmbeddingProvider:       coverageEmbeddingProviderName,
			EmbeddingModel:          coverageDefaultEmbeddingModel,
			EmbeddingVersion:        coverageFindingEmbeddingVersion,
			EmbeddingDimensions:     3,
			EmbeddingTextHash:       fmt.Sprintf("hash-%d", i),
			EmbeddingUpdatedAt:      &now,
		}
		if err := analysisRepo.RecordConversationAnalysis(ctx, &analysis); err != nil {
			t.Fatalf("seed analysis %d: %v", i, err)
		}
	}
	seedMaterializerAnalysis(t, analysisRepo, "analysis-new", "run-1", "conversation-new", "Customers need account access help.", "Account access problems", now)

	if _, err := analyzer.materializeRunFindings(ctx, "ws-1", "run-1"); err != nil {
		t.Fatalf("materializeRunFindings: %v", err)
	}
	var gap model.SupportCoverageGap
	if err := db.First(&gap, "id = ?", existing.ID).Error; err != nil {
		t.Fatalf("load gap: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(gap.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["split_review_needed"] != true {
		t.Fatalf("split review not flagged: %+v", metadata)
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

func seedMaterializerDocsSpace(t *testing.T, db *gorm.DB, id, workspaceID string) {
	t.Helper()
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS docs_spaces (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		team_id TEXT,
		name TEXT NOT NULL,
		slug TEXT NOT NULL,
		icon TEXT,
		visibility TEXT NOT NULL DEFAULT 'workspace_wide',
		type TEXT NOT NULL DEFAULT 'external',
		default_review_days INTEGER,
		is_system BOOLEAN NOT NULL DEFAULT false,
		position INTEGER NOT NULL DEFAULT 0,
		created_by TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create docs_spaces: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS docs_chunks (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		space_id TEXT NOT NULL,
		document_id TEXT NOT NULL,
		block_id TEXT,
		chunk_index INTEGER NOT NULL,
		block_range TEXT,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		content_hash TEXT NOT NULL DEFAULT '',
		embedding TEXT NOT NULL DEFAULT '',
		embedding_provider TEXT NOT NULL DEFAULT 'openai',
		embedding_model TEXT NOT NULL DEFAULT 'text-embedding-3-small',
		embedding_version TEXT NOT NULL DEFAULT 'content-chunk-v1',
		embedding_dimensions INTEGER NOT NULL DEFAULT 1536,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create docs_chunks: %v", err)
	}
	if err := db.Create(&model.DocsSpace{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        "Help center",
		Slug:        "help-center",
		Type:        model.SpaceTypeExternalCapable,
		CreatedBy:   "user-1",
	}).Error; err != nil {
		t.Fatalf("seed docs space: %v", err)
	}
}
