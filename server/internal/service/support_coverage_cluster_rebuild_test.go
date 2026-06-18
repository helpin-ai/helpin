package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

type failingCoverageEmbeddingProvider struct {
	err error
}

func (f failingCoverageEmbeddingProvider) CreateEmbeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, errors.New("embedding failed")
}

func TestSupportCoverageClusterRebuildSuggestsVerySimilarEmbeddedGaps(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	svc := NewSupportCoverageClusterRebuildService(repo, &fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}, {0.8, 0.6, 0}}}, "")
	ctx := context.Background()
	now := time.Now()

	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-reset",
		WorkspaceID:   "ws-1",
		DedupeKey:     "reset",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryKnowledge,
		Title:         "Users cannot reset passwords",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.86,
		EvidenceCount: 3,
		FirstSeenAt:   now.Add(-2 * time.Hour),
		LastSeenAt:    now.Add(-1 * time.Hour),
	})
	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-password",
		WorkspaceID:   "ws-1",
		DedupeKey:     "password",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryKnowledge,
		Title:         "Password reset instructions are missing",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.82,
		EvidenceCount: 2,
		FirstSeenAt:   now.Add(-90 * time.Minute),
		LastSeenAt:    now.Add(-30 * time.Minute),
	})

	result, err := svc.RebuildWorkspace(ctx, "ws-1")
	if err != nil {
		t.Fatalf("RebuildWorkspace: %v", err)
	}
	if result.GapsScanned != 2 {
		t.Fatalf("GapsScanned=%d, want 2", result.GapsScanned)
	}
	if result.SuggestionsCreated != 1 {
		t.Fatalf("SuggestionsCreated=%d, want 1", result.SuggestionsCreated)
	}
	if result.AutoMerged != 0 {
		t.Fatalf("AutoMerged=%d, want 0", result.AutoMerged)
	}

	var target model.SupportCoverageGap
	if err := db.First(&target, "id = ?", "gap-reset").Error; err != nil {
		t.Fatalf("load target gap: %v", err)
	}
	if target.EvidenceCount != 3 {
		t.Fatalf("EvidenceCount=%d, want 3", target.EvidenceCount)
	}
	var source model.SupportCoverageGap
	if err := db.First(&source, "id = ?", "gap-password").Error; err != nil {
		t.Fatalf("load source gap: %v", err)
	}
	if source.Status != model.SupportCoverageGapStatusOpen {
		t.Fatalf("source status=%q, want open", source.Status)
	}

	suggestions, err := repo.ListMergeSuggestionsForGap(ctx, "ws-1", "gap-reset")
	if err != nil {
		t.Fatalf("ListMergeSuggestionsForGap: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("suggestions=%d, want 1", len(suggestions))
	}
}

func TestSupportCoverageClusterRebuildDoesNotSuggestDifferentNoSearchResultObjects(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	svc := NewSupportCoverageClusterRebuildService(repo, &fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}, {0.8, 0.6, 0}}}, "")
	ctx := context.Background()
	now := time.Now()

	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-analytics-search",
		WorkspaceID:   "ws-1",
		DedupeKey:     "analytics-search",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryUnknown,
		Title:         "No search results: analytics",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.3,
		EvidenceCount: 1,
		FirstSeenAt:   now.Add(-2 * time.Hour),
		LastSeenAt:    now.Add(-1 * time.Hour),
	})
	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-invoices-search",
		WorkspaceID:   "ws-1",
		DedupeKey:     "invoices-search",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryUnknown,
		Title:         "No search results: invoices",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.3,
		EvidenceCount: 1,
		FirstSeenAt:   now.Add(-90 * time.Minute),
		LastSeenAt:    now.Add(-30 * time.Minute),
	})

	result, err := svc.RebuildWorkspace(ctx, "ws-1")
	if err != nil {
		t.Fatalf("RebuildWorkspace: %v", err)
	}
	if result.SuggestionsCreated != 0 {
		t.Fatalf("SuggestionsCreated=%d, want 0", result.SuggestionsCreated)
	}

	suggestions, err := repo.ListMergeSuggestionsForGap(ctx, "ws-1", "gap-analytics-search")
	if err != nil {
		t.Fatalf("ListMergeSuggestionsForGap: %v", err)
	}
	if len(suggestions) != 0 {
		t.Fatalf("suggestions=%d, want 0", len(suggestions))
	}
}

func TestSupportCoverageClusterRebuildAutoMergesSemanticClusterWithoutRelatedArticle(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	svc := NewSupportCoverageClusterRebuildService(repo, &fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}, {0.99, 0.01, 0}}}, "")
	ctx := context.Background()
	now := time.Now()

	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-cancel-action",
		WorkspaceID:   "ws-1",
		DedupeKey:     "cancel-action",
		GapKind:       "action",
		GapCategory:   model.SupportCoverageGapCategoryAction,
		Title:         "Cancel subscription action is unavailable",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.9,
		EvidenceCount: 4,
		FirstSeenAt:   now.Add(-4 * time.Hour),
		LastSeenAt:    now.Add(-1 * time.Hour),
	})
	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-cancel-workflow",
		WorkspaceID:   "ws-1",
		DedupeKey:     "cancel-workflow",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryKnowledge,
		Title:         "Customer cannot cancel subscription",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.88,
		EvidenceCount: 3,
		FirstSeenAt:   now.Add(-3 * time.Hour),
		LastSeenAt:    now.Add(-30 * time.Minute),
	})
	seedCoverageEvidenceForRebuild(t, db, model.SupportGapEvidence{
		ID:             "ev-cancel-action",
		GapID:          "gap-cancel-action",
		WorkspaceID:    "ws-1",
		EvidenceType:   model.SupportCoverageGapSourceDailyConversationAnalysis,
		SourceSignal:   model.SupportCoverageGapSourceDailyConversationAnalysis,
		Excerpt:        "Customer needed to cancel their subscription and the AI could not perform the cancellation.",
		Metadata:       json.RawMessage(`{"customer_need":"Customer needed to cancel their subscription from the account settings."}`),
		ConversationID: strPtr("conversation-1"),
		CreatedAt:      now.Add(-2 * time.Hour),
	})
	seedCoverageEvidenceForRebuild(t, db, model.SupportGapEvidence{
		ID:             "ev-cancel-workflow",
		GapID:          "gap-cancel-workflow",
		WorkspaceID:    "ws-1",
		EvidenceType:   model.SupportCoverageGapSourceDailyConversationAnalysis,
		SourceSignal:   model.SupportCoverageGapSourceDailyConversationAnalysis,
		Excerpt:        "Customer wanted cancellation steps and needed help canceling the subscription.",
		Metadata:       json.RawMessage(`{"customer_need":"Customer needed to cancel their subscription from the account settings."}`),
		ConversationID: strPtr("conversation-2"),
		CreatedAt:      now.Add(-90 * time.Minute),
	})

	result, err := svc.RebuildWorkspace(ctx, "ws-1")
	if err != nil {
		t.Fatalf("RebuildWorkspace: %v", err)
	}
	if result.AutoMerged != 1 {
		t.Fatalf("AutoMerged=%d, want 1", result.AutoMerged)
	}

	var source model.SupportCoverageGap
	if err := db.First(&source, "id = ?", "gap-cancel-workflow").Error; err != nil {
		t.Fatalf("load source gap: %v", err)
	}
	if source.Status != model.SupportCoverageGapStatusMerged {
		t.Fatalf("source status=%q, want merged", source.Status)
	}
	var target model.SupportCoverageGap
	if err := db.First(&target, "id = ?", "gap-cancel-action").Error; err != nil {
		t.Fatalf("load target gap: %v", err)
	}
	if target.EvidenceCount != 7 {
		t.Fatalf("target EvidenceCount=%d, want 7", target.EvidenceCount)
	}
}

func TestSupportCoverageClusterRebuildUsesTransitiveClusters(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	svc := NewSupportCoverageClusterRebuildService(repo, &fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}, {0.8, 0.6, 0}, {0.28, 0.96, 0}}}, "")
	ctx := context.Background()
	now := time.Now()

	gaps := []model.SupportCoverageGap{
		{ID: "gap-reset-password", WorkspaceID: "ws-1", DedupeKey: "a", GapKind: "content", GapCategory: model.SupportCoverageGapCategoryKnowledge, Title: "Reset password article is missing", Status: model.SupportCoverageGapStatusOpen, Confidence: 0.86, EvidenceCount: 2, FirstSeenAt: now.Add(-3 * time.Hour), LastSeenAt: now.Add(-3 * time.Hour)},
		{ID: "gap-password-login", WorkspaceID: "ws-1", DedupeKey: "b", GapKind: "content", GapCategory: model.SupportCoverageGapCategoryKnowledge, Title: "Password login recovery steps missing", Status: model.SupportCoverageGapStatusOpen, Confidence: 0.84, EvidenceCount: 2, FirstSeenAt: now.Add(-2 * time.Hour), LastSeenAt: now.Add(-2 * time.Hour)},
		{ID: "gap-login-access", WorkspaceID: "ws-1", DedupeKey: "c", GapKind: "content", GapCategory: model.SupportCoverageGapCategoryKnowledge, Title: "Login access recovery instructions absent", Status: model.SupportCoverageGapStatusOpen, Confidence: 0.83, EvidenceCount: 2, FirstSeenAt: now.Add(-1 * time.Hour), LastSeenAt: now.Add(-1 * time.Hour)},
	}
	for _, gap := range gaps {
		seedCoverageGapForRebuild(t, db, gap)
	}

	result, err := svc.RebuildWorkspace(ctx, "ws-1")
	if err != nil {
		t.Fatalf("RebuildWorkspace: %v", err)
	}
	if result.ClustersFound != 1 {
		t.Fatalf("ClustersFound=%d, want 1 transitive cluster", result.ClustersFound)
	}
	if result.AutoMerged != 0 {
		t.Fatalf("AutoMerged=%d, want 0 for transitive title-only cluster", result.AutoMerged)
	}
}

func TestSupportCoverageClusterRebuildDoesNotRecreateDismissedMergeSuggestion(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	svc := NewSupportCoverageClusterRebuildService(repo, &fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}, {0.8, 0.6, 0}}}, "")
	ctx := context.Background()
	now := time.Now()

	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-reset",
		WorkspaceID:   "ws-1",
		DedupeKey:     "reset",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryKnowledge,
		Title:         "Users cannot reset passwords",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.86,
		EvidenceCount: 3,
		FirstSeenAt:   now.Add(-2 * time.Hour),
		LastSeenAt:    now.Add(-1 * time.Hour),
	})
	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-password",
		WorkspaceID:   "ws-1",
		DedupeKey:     "password",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryKnowledge,
		Title:         "Password recovery article is missing",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.82,
		EvidenceCount: 2,
		FirstSeenAt:   now.Add(-90 * time.Minute),
		LastSeenAt:    now.Add(-30 * time.Minute),
	})

	result, err := svc.RebuildWorkspace(ctx, "ws-1")
	if err != nil {
		t.Fatalf("first RebuildWorkspace: %v", err)
	}
	if result.SuggestionsCreated != 1 {
		t.Fatalf("first SuggestionsCreated=%d, want 1", result.SuggestionsCreated)
	}
	suggestions, err := repo.ListMergeSuggestionsForGap(ctx, "ws-1", "gap-reset")
	if err != nil {
		t.Fatalf("ListMergeSuggestionsForGap: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("suggestions=%d, want 1", len(suggestions))
	}
	if err := svc.DismissMergeSuggestion(ctx, "ws-1", suggestions[0].ID, "user-1"); err != nil {
		t.Fatalf("DismissMergeSuggestion: %v", err)
	}

	result, err = svc.RebuildWorkspace(ctx, "ws-1")
	if err != nil {
		t.Fatalf("second RebuildWorkspace: %v", err)
	}
	if result.SuggestionsCreated != 0 {
		t.Fatalf("second SuggestionsCreated=%d, want 0 after keep separate", result.SuggestionsCreated)
	}
	suggestions, err = repo.ListMergeSuggestionsForGap(ctx, "ws-1", "gap-reset")
	if err != nil {
		t.Fatalf("ListMergeSuggestionsForGap after rebuild: %v", err)
	}
	if len(suggestions) != 0 {
		t.Fatalf("pending suggestions=%d, want 0 after keep separate", len(suggestions))
	}
}

func TestSupportCoverageClusterRebuildCannotLinkPreventsTransitiveSuggestion(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	svc := NewSupportCoverageClusterRebuildService(repo, &fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}, {0.8, 0.6, 0}}}, "")
	ctx := context.Background()
	now := time.Now()

	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-a",
		WorkspaceID:   "ws-1",
		DedupeKey:     "a",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryKnowledge,
		Title:         "Reset password email never arrives",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.9,
		EvidenceCount: 2,
		FirstSeenAt:   now.Add(-3 * time.Hour),
		LastSeenAt:    now.Add(-3 * time.Hour),
	})
	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-b",
		WorkspaceID:   "ws-1",
		DedupeKey:     "b",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryKnowledge,
		Title:         "Login recovery instructions are missing",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.9,
		EvidenceCount: 1,
		FirstSeenAt:   now.Add(-2 * time.Hour),
		LastSeenAt:    now.Add(-2 * time.Hour),
	})

	if _, err := svc.RebuildWorkspace(ctx, "ws-1"); err != nil {
		t.Fatalf("first RebuildWorkspace: %v", err)
	}
	suggestions, err := repo.ListMergeSuggestionsForGap(ctx, "ws-1", "gap-a")
	if err != nil {
		t.Fatalf("ListMergeSuggestionsForGap: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("suggestions=%d, want 1", len(suggestions))
	}
	if err := svc.DismissMergeSuggestion(ctx, "ws-1", suggestions[0].ID, "user-1"); err != nil {
		t.Fatalf("DismissMergeSuggestion: %v", err)
	}

	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-c",
		WorkspaceID:   "ws-1",
		DedupeKey:     "c",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryKnowledge,
		Title:         "Account access recovery docs missing",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.9,
		EvidenceCount: 10,
		FirstSeenAt:   now.Add(-1 * time.Hour),
		LastSeenAt:    now.Add(-1 * time.Hour),
	})
	svc = NewSupportCoverageClusterRebuildService(repo, &fakeCoverageEmbeddingProvider{vectors: [][]float32{{0.949, 0.316, 0}}}, "")

	result, err := svc.RebuildWorkspace(ctx, "ws-1")
	if err != nil {
		t.Fatalf("second RebuildWorkspace: %v", err)
	}
	if result.SuggestionsCreated != 1 {
		t.Fatalf("SuggestionsCreated=%d, want exactly 1 because gap-a and gap-b are kept separate", result.SuggestionsCreated)
	}

	suggestions, err = repo.ListMergeSuggestionsForGap(ctx, "ws-1", "gap-b")
	if err != nil {
		t.Fatalf("ListMergeSuggestionsForGap for gap-b: %v", err)
	}
	if len(suggestions) != 0 {
		t.Fatalf("gap-b pending suggestions=%d, want 0 because it is kept separate from gap-a", len(suggestions))
	}
}

func TestSupportCoverageClusterRebuildBackfillsMissingGapEmbeddings(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	svc := NewSupportCoverageClusterRebuildService(repo, &fakeCoverageEmbeddingProvider{vectors: [][]float32{{1, 0, 0}}}, "")
	ctx := context.Background()
	now := time.Now()

	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-reset",
		WorkspaceID:   "ws-1",
		DedupeKey:     "reset",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryKnowledge,
		Title:         "Reset email is not delivered",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.86,
		EvidenceCount: 3,
		FirstSeenAt:   now.Add(-2 * time.Hour),
		LastSeenAt:    now.Add(-1 * time.Hour),
	})

	result, err := svc.RebuildWorkspace(ctx, "ws-1")
	if err != nil {
		t.Fatalf("RebuildWorkspace: %v", err)
	}
	if result.EmbeddingStatus != "complete" {
		t.Fatalf("EmbeddingStatus=%q, want complete", result.EmbeddingStatus)
	}

	var gap model.SupportCoverageGap
	if err := db.First(&gap, "id = ?", "gap-reset").Error; err != nil {
		t.Fatalf("load gap: %v", err)
	}
	if gap.Embedding == "" || gap.EmbeddingTextHash == "" || gap.EmbeddingVersion != coverageGapEmbeddingVersion || gap.EmbeddingDimensions != 3 {
		t.Fatalf("gap embedding metadata not backfilled: embedding=%q version=%q dimensions=%d hash=%q", gap.Embedding, gap.EmbeddingVersion, gap.EmbeddingDimensions, gap.EmbeddingTextHash)
	}

	var run model.SupportCoverageClusterRebuildRun
	if err := db.First(&run, "id = ?", result.RunID).Error; err != nil {
		t.Fatalf("load run: %v", err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(run.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal run metadata: %v", err)
	}
	if metadata["embeddings_created"] != float64(1) {
		t.Fatalf("embeddings_created=%v, want 1", metadata["embeddings_created"])
	}
	if metadata["embedding_status"] != "complete" {
		t.Fatalf("embedding_status=%v, want complete", metadata["embedding_status"])
	}
}

func TestSupportCoverageClusterRebuildReportsEmbeddingFailureWithoutLexicalFallback(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	svc := NewSupportCoverageClusterRebuildService(repo, failingCoverageEmbeddingProvider{err: errors.New("provider down")}, "")
	ctx := context.Background()
	now := time.Now()

	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-reset",
		WorkspaceID:   "ws-1",
		DedupeKey:     "reset",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryKnowledge,
		Title:         "Users cannot reset passwords",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.86,
		EvidenceCount: 3,
		FirstSeenAt:   now.Add(-2 * time.Hour),
		LastSeenAt:    now.Add(-1 * time.Hour),
	})
	seedCoverageGapForRebuild(t, db, model.SupportCoverageGap{
		ID:            "gap-password",
		WorkspaceID:   "ws-1",
		DedupeKey:     "password",
		GapKind:       "content",
		GapCategory:   model.SupportCoverageGapCategoryKnowledge,
		Title:         "Password reset instructions are missing",
		Status:        model.SupportCoverageGapStatusOpen,
		Confidence:    0.82,
		EvidenceCount: 2,
		FirstSeenAt:   now.Add(-90 * time.Minute),
		LastSeenAt:    now.Add(-30 * time.Minute),
	})

	result, err := svc.RebuildWorkspace(ctx, "ws-1")
	if err == nil {
		t.Fatalf("RebuildWorkspace succeeded, want embedding failure")
	}
	if result != nil {
		t.Fatalf("result=%+v, want nil on embedding failure", result)
	}
	var run model.SupportCoverageClusterRebuildRun
	if err := db.First(&run, "workspace_id = ?", "ws-1").Error; err != nil {
		t.Fatalf("load run: %v", err)
	}
	if run.Status != model.SupportCoverageClusterRebuildStatusFailed {
		t.Fatalf("run status=%q, want failed", run.Status)
	}
	if run.ErrorMessage == nil || *run.ErrorMessage == "" {
		t.Fatalf("run error message missing")
	}
	var suggestionCount int64
	if err := db.Model(&model.SupportCoverageGapMergeSuggestion{}).Count(&suggestionCount).Error; err != nil {
		t.Fatalf("count suggestions: %v", err)
	}
	if suggestionCount != 0 {
		t.Fatalf("suggestions=%d, want 0 when embeddings fail", suggestionCount)
	}
}

func seedCoverageGapForRebuild(t *testing.T, db *gorm.DB, gap model.SupportCoverageGap) {
	t.Helper()
	if len(gap.Metadata) == 0 {
		gap.Metadata = json.RawMessage(`{}`)
	}
	if err := db.Create(&gap).Error; err != nil {
		t.Fatalf("seed gap %s: %v", gap.ID, err)
	}
}

func seedCoverageEvidenceForRebuild(t *testing.T, db *gorm.DB, evidence model.SupportGapEvidence) {
	t.Helper()
	if len(evidence.Metadata) == 0 {
		evidence.Metadata = json.RawMessage(`{}`)
	}
	if err := db.Create(&evidence).Error; err != nil {
		t.Fatalf("seed evidence %s: %v", evidence.ID, err)
	}
}
