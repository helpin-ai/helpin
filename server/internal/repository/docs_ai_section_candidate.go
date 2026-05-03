package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type DocsAISectionCandidateRepository struct {
	db *gorm.DB
}

func NewDocsAISectionCandidateRepository(db *gorm.DB) *DocsAISectionCandidateRepository {
	return &DocsAISectionCandidateRepository{db: db}
}

func (r *DocsAISectionCandidateRepository) Create(ctx context.Context, candidate *model.DocsAISectionCandidate) (*model.DocsAISectionCandidate, error) {
	if err := r.db.WithContext(ctx).Create(candidate).Error; err != nil {
		return nil, fmt.Errorf("create docs ai section candidate: %w", err)
	}
	return candidate, nil
}

func (r *DocsAISectionCandidateRepository) LatestOpen(ctx context.Context, documentID, blockID string) (*model.DocsAISectionCandidate, error) {
	var candidate model.DocsAISectionCandidate
	if err := r.db.WithContext(ctx).
		Where("document_id = ? AND block_id = ? AND status = ?", documentID, blockID, model.DocsAISectionCandidateStatusReady).
		Order("created_at DESC").
		First(&candidate).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("latest docs ai section candidate: %w", err)
	}
	return &candidate, nil
}

func (r *DocsAISectionCandidateRepository) GetByID(ctx context.Context, id string) (*model.DocsAISectionCandidate, error) {
	var candidate model.DocsAISectionCandidate
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&candidate).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs ai section candidate: %w", err)
	}
	return &candidate, nil
}

func (r *DocsAISectionCandidateRepository) UpdateStatus(ctx context.Context, id, status, actorID string) error {
	updates := map[string]any{"status": status}
	if status == model.DocsAISectionCandidateStatusApproved && actorID != "" {
		updates["approved_by"] = actorID
	}
	if err := r.db.WithContext(ctx).Model(&model.DocsAISectionCandidate{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update docs ai section candidate status: %w", err)
	}
	return nil
}
