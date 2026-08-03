package repository

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SupportRunEvidenceRepository persists per-run knowledge evidence snapshots.
type SupportRunEvidenceRepository struct {
	db *gorm.DB
}

// NewSupportRunEvidenceRepository creates a SupportRunEvidenceRepository.
func NewSupportRunEvidenceRepository(db *gorm.DB) *SupportRunEvidenceRepository {
	return &SupportRunEvidenceRepository{db: db}
}

// UpsertBatch inserts evidence rows, ignoring duplicates for the same
// (run_id, evidence_id) so repeated searches in one run stay idempotent.
func (r *SupportRunEvidenceRepository) UpsertBatch(ctx context.Context, rows []model.SupportRunEvidence) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "run_id"}, {Name: "evidence_id"}},
			DoNothing: true,
		}).
		Create(&rows).Error
}

// ListByRun returns the evidence snapshot for a run.
func (r *SupportRunEvidenceRepository) ListByRun(ctx context.Context, workspaceID, runID string) ([]model.SupportRunEvidence, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	var rows []model.SupportRunEvidence
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id = ?", workspaceID, runID).
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteByRun removes a run's evidence snapshot (called on terminal runs).
func (r *SupportRunEvidenceRepository) DeleteByRun(ctx context.Context, workspaceID, runID string) error {
	if r == nil || r.db == nil {
		return gorm.ErrInvalidDB
	}
	return r.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id = ?", workspaceID, runID).
		Delete(&model.SupportRunEvidence{}).Error
}
