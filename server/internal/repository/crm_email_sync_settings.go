package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMEmailSyncSettingsRepository handles DB operations for CRM email sync settings.
type CRMEmailSyncSettingsRepository struct {
	db *gorm.DB
}

// NewCRMEmailSyncSettingsRepository creates a new CRMEmailSyncSettingsRepository.
func NewCRMEmailSyncSettingsRepository(db *gorm.DB) *CRMEmailSyncSettingsRepository {
	return &CRMEmailSyncSettingsRepository{db: db}
}

// GetByWorkspace returns email sync settings for a workspace.
func (r *CRMEmailSyncSettingsRepository) GetByWorkspace(ctx context.Context, workspaceID string) (*model.CRMEmailSyncSettings, error) {
	var settings model.CRMEmailSyncSettings
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&settings).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get email sync settings: %w", err)
	}
	return &settings, nil
}

// Upsert creates or updates email sync settings for a workspace.
func (r *CRMEmailSyncSettingsRepository) Upsert(ctx context.Context, settings *model.CRMEmailSyncSettings) error {
	var existing model.CRMEmailSyncSettings
	err := r.db.WithContext(ctx).Where("workspace_id = ?", settings.WorkspaceID).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check email sync settings: %w", err)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := r.db.WithContext(ctx).Create(settings).Error; err != nil {
			return fmt.Errorf("create email sync settings: %w", err)
		}
		return nil
	}

	settings.ID = existing.ID
	if err := r.db.WithContext(ctx).Save(settings).Error; err != nil {
		return fmt.Errorf("update email sync settings: %w", err)
	}
	return nil
}
