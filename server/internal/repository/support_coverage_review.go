package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrCoverageDraftReview = errors.New("coverage proposal needs review")

// WithCoverageProposal locks the gap and rechecks the proposal before any write.
// Document creation/content and the receipt share the same transaction, so retries
// cannot append twice and a failed receipt cannot leave a partially applied fix.
func (r *SupportCoverageRepository) WithCoverageProposal(ctx context.Context, workspaceID, suggestionID string, apply func(*gorm.DB, *model.SupportGapSuggestion) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var suggestion model.SupportGapSuggestion
		if err := tx.Where("id = ? AND workspace_id = ?", suggestionID, workspaceID).First(&suggestion).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: proposal not found", ErrCoverageDraftReview)
			}
			return err
		}
		var gap model.SupportCoverageGap
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", suggestion.GapID, workspaceID).First(&gap).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", suggestionID, workspaceID).First(&suggestion).Error; err != nil {
			return err
		}
		if !suggestion.IsActive || suggestion.SupersededAt != nil {
			return fmt.Errorf("%w: proposal has been superseded; review the latest proposal", ErrCoverageDraftReview)
		}
		if suggestion.Status != model.SupportCoverageSuggestionStatusDraft {
			return fmt.Errorf("%w: proposal is no longer in draft state", ErrCoverageDraftReview)
		}
		if gap.Status != model.SupportCoverageGapStatusOpen {
			return fmt.Errorf("%w: reopen the gap before saving a proposal", ErrCoverageDraftReview)
		}
		return apply(tx, &suggestion)
	})
}

func (r *SupportCoverageRepository) CreateActiveDraftSuggestion(ctx context.Context, suggestion *model.SupportGapSuggestion) (*model.SupportGapSuggestion, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var gap model.SupportCoverageGap
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", suggestion.GapID, suggestion.WorkspaceID).First(&gap).Error; err != nil {
			return err
		}
		if gap.Status != model.SupportCoverageGapStatusOpen {
			return fmt.Errorf("%w: gap is no longer open", ErrCoverageDraftReview)
		}
		now := time.Now()
		if err := tx.Model(&model.SupportGapSuggestion{}).Where("gap_id = ? AND workspace_id = ? AND status = ? AND is_active", gap.ID, gap.WorkspaceID, model.SupportCoverageSuggestionStatusDraft).Updates(map[string]any{"is_active": false, "superseded_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		suggestion.IsActive = true
		_, err := NewSupportCoverageRepository(tx).CreateSuggestion(ctx, suggestion)
		return err
	})
	return suggestion, err
}

func (r *SupportCoverageRepository) RecordReviewedProposal(ctx context.Context, suggestion *model.SupportGapSuggestion, documentID string) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&model.SupportGapSuggestion{}).Where("id = ? AND workspace_id = ?", suggestion.ID, suggestion.WorkspaceID).Updates(map[string]any{
		"title": suggestion.Title, "content": suggestion.Content, "suggestion_type": suggestion.SuggestionType,
		"target_space_id": suggestion.TargetSpaceID, "target_collection_id": suggestion.TargetCollectionID,
		"target_document_id": suggestion.TargetDocumentID, "result_document_id": documentID,
		"status": model.SupportCoverageSuggestionStatusApplied, "applied_at": now, "updated_at": now,
	}).Error; err != nil {
		return fmt.Errorf("save reviewed proposal: %w", err)
	}
	return r.LinkGapArticle(ctx, suggestion.GapID, documentID, suggestion.WorkspaceID)
}

func (r *SupportCoverageRepository) ValidateCoverageDraftDestination(ctx context.Context, workspaceID, spaceID string, collectionID *string) error {
	var count int64
	if err := r.db.WithContext(ctx).Table("docs_spaces").Where("id = ? AND workspace_id = ? AND deleted_at IS NULL", spaceID, workspaceID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("%w: choose a docs space in this workspace", ErrCoverageDraftReview)
	}
	if collectionID == nil || *collectionID == "" {
		return nil
	}
	if err := r.db.WithContext(ctx).Table("docs_collections").Where("id = ? AND workspace_id = ? AND space_id = ? AND deleted_at IS NULL", *collectionID, workspaceID, spaceID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("%w: choose a collection in the selected space", ErrCoverageDraftReview)
	}
	return nil
}
