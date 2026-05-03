package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type SupportInboxViewRepository struct {
	db *gorm.DB
}

func NewSupportInboxViewRepository(db *gorm.DB) *SupportInboxViewRepository {
	return &SupportInboxViewRepository{db: db}
}

func (r *SupportInboxViewRepository) ListByWorkspace(ctx context.Context, workspaceID, userID string) ([]model.SupportInboxView, error) {
	var views []model.SupportInboxView
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND (is_shared = true OR created_by = ?)", workspaceID, userID).
		Order("LOWER(name) ASC, created_at ASC").
		Find(&views).Error; err != nil {
		return nil, fmt.Errorf("list support inbox views: %w", err)
	}
	return views, nil
}

func (r *SupportInboxViewRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.SupportInboxView, error) {
	var view model.SupportInboxView
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&view).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get support inbox view: %w", err)
	}
	return &view, nil
}

func (r *SupportInboxViewRepository) Create(ctx context.Context, view *model.SupportInboxView) error {
	if err := r.db.WithContext(ctx).Create(view).Error; err != nil {
		return fmt.Errorf("create support inbox view: %w", err)
	}
	return nil
}

func (r *SupportInboxViewRepository) Update(ctx context.Context, view *model.SupportInboxView) error {
	if err := r.db.WithContext(ctx).Save(view).Error; err != nil {
		return fmt.Errorf("update support inbox view: %w", err)
	}
	return nil
}

func (r *SupportInboxViewRepository) Delete(ctx context.Context, workspaceID, id string) error {
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, strings.TrimSpace(id)).Delete(&model.SupportInboxView{}).Error; err != nil {
		return fmt.Errorf("delete support inbox view: %w", err)
	}
	return nil
}
