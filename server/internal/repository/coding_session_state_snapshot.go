package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CodingSessionStateSnapshotRepository handles DB operations for live coding-session stream state.
type CodingSessionStateSnapshotRepository struct {
	db *gorm.DB
}

// NewCodingSessionStateSnapshotRepository creates a new repository.
func NewCodingSessionStateSnapshotRepository(db *gorm.DB) *CodingSessionStateSnapshotRepository {
	return &CodingSessionStateSnapshotRepository{db: db}
}

// GetByRun returns the latest snapshot row for a run.
func (r *CodingSessionStateSnapshotRepository) GetByRun(ctx context.Context, workspaceID, runID string) (*model.CodingSessionStateSnapshot, error) {
	var snapshot model.CodingSessionStateSnapshot
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id = ?", workspaceID, runID).
		First(&snapshot).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get coding session state snapshot: %w", err)
	}
	return &snapshot, nil
}

// Upsert creates or updates the snapshot row for a run.
func (r *CodingSessionStateSnapshotRepository) Upsert(ctx context.Context, snapshot *model.CodingSessionStateSnapshot) error {
	if snapshot == nil {
		return nil
	}
	if snapshot.SchemaVersion == "" {
		snapshot.SchemaVersion = model.CodingSessionStateSnapshotSchemaVersionV1
	}
	sanitizeCodingSessionStateSnapshotForPostgres(snapshot)
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "workspace_id"},
			{Name: "run_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"schema_version",
			"snapshot_payload",
			"updated_at",
		}),
	}).Create(snapshot).Error; err != nil {
		return fmt.Errorf("upsert coding session state snapshot: %w", err)
	}
	return nil
}

// DeleteByRun removes the snapshot row for a run.
func (r *CodingSessionStateSnapshotRepository) DeleteByRun(ctx context.Context, workspaceID, runID string) error {
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id = ?", workspaceID, runID).
		Delete(&model.CodingSessionStateSnapshot{}).Error; err != nil {
		return fmt.Errorf("delete coding session state snapshot: %w", err)
	}
	return nil
}
