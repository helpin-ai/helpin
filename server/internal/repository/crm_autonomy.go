package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMAutonomyRepository handles DB operations for CRM autonomy settings.
type CRMAutonomyRepository struct {
	db *gorm.DB
}

// NewCRMAutonomyRepository creates a new CRMAutonomyRepository.
func NewCRMAutonomyRepository(db *gorm.DB) *CRMAutonomyRepository {
	return &CRMAutonomyRepository{db: db}
}

// GetByWorkspace returns autonomy settings for a workspace.
func (r *CRMAutonomyRepository) GetByWorkspace(ctx context.Context, workspaceID string) (*model.CRMAutonomySettings, error) {
	var settings model.CRMAutonomySettings
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&settings).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get autonomy settings: %w", err)
	}
	return &settings, nil
}

// Upsert creates or updates autonomy settings for a workspace.
func (r *CRMAutonomyRepository) Upsert(ctx context.Context, settings *model.CRMAutonomySettings) error {
	var existing model.CRMAutonomySettings
	err := r.db.WithContext(ctx).Where("workspace_id = ?", settings.WorkspaceID).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check autonomy settings: %w", err)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := r.db.WithContext(ctx).Create(settings).Error; err != nil {
			return fmt.Errorf("create autonomy settings: %w", err)
		}
		return nil
	}

	settings.ID = existing.ID
	if err := r.db.WithContext(ctx).Save(settings).Error; err != nil {
		return fmt.Errorf("update autonomy settings: %w", err)
	}
	return nil
}
