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

func TestSupportCoverageClusterRebuildCreatesMergeSuggestionForSimilarGaps(t *testing.T) {
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

	suggestions, err := repo.ListMergeSuggestionsForGap(ctx, "ws-1", "gap-reset")
	if err != nil {
		t.Fatalf("ListMergeSuggestionsForGap: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("suggestions=%d, want 1", len(suggestions))
	}
	if suggestions[0].Status != model.SupportCoverageMergeSuggestionStatusPending {
		t.Fatalf("status=%q, want pending", suggestions[0].Status)
	}
	if suggestions[0].CombinedEvidenceCount != 5 {
		t.Fatalf("CombinedEvidenceCount=%d, want 5", suggestions[0].CombinedEvidenceCount)
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
