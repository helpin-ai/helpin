package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestSupportCoverageClusterRebuildSuggestsVerySimilarTitleOnlyGaps(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	svc := NewSupportCoverageClusterRebuildService(repo, nil, "")
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

func TestSupportCoverageClusterRebuildAutoMergesSemanticClusterWithoutRelatedArticle(t *testing.T) {
	_, _, db := setupCoverageTestEnv(t)
	repo := repository.NewSupportCoverageRepository(db)
	svc := NewSupportCoverageClusterRebuildService(repo, nil, "")
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
	svc := NewSupportCoverageClusterRebuildService(repo, nil, "")
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
