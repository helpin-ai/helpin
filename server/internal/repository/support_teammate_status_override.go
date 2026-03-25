package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type SupportTeammateStatusOverrideRepository struct {
	db *gorm.DB
}

func NewSupportTeammateStatusOverrideRepository(db *gorm.DB) *SupportTeammateStatusOverrideRepository {
	return &SupportTeammateStatusOverrideRepository{db: db}
}

func (r *SupportTeammateStatusOverrideRepository) GetForUser(ctx context.Context, workspaceID, userID string) (*model.SupportTeammateStatusOverride, error) {
	var override model.SupportTeammateStatusOverride
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		First(&override).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get support teammate status override: %w", err)
	}
	return &override, nil
}

func (r *SupportTeammateStatusOverrideRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.SupportTeammateStatusOverride, error) {
	var overrides []model.SupportTeammateStatusOverride
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Find(&overrides).Error; err != nil {
		return nil, fmt.Errorf("list support teammate status overrides: %w", err)
	}
	return overrides, nil
}

func (r *SupportTeammateStatusOverrideRepository) Upsert(ctx context.Context, workspaceID, userID, manualStatus string) (*model.SupportTeammateStatusOverride, error) {
	override, err := r.GetForUser(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if override == nil {
		override = &model.SupportTeammateStatusOverride{
			WorkspaceID:  workspaceID,
			UserID:       userID,
			ManualStatus: manualStatus,
		}
		if err := r.db.WithContext(ctx).Create(override).Error; err != nil {
			return nil, fmt.Errorf("create support teammate status override: %w", err)
		}
		return override, nil
	}

	override.ManualStatus = manualStatus
	if err := r.db.WithContext(ctx).Save(override).Error; err != nil {
		return nil, fmt.Errorf("update support teammate status override: %w", err)
	}
	return override, nil
}

func (r *SupportTeammateStatusOverrideRepository) DeleteForUser(ctx context.Context, workspaceID, userID string) error {
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Delete(&model.SupportTeammateStatusOverride{}).Error; err != nil {
		return fmt.Errorf("delete support teammate status override: %w", err)
	}
	return nil
}
