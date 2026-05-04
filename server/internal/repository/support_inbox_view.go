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
		Where("workspace_id = ? AND view_type = ? AND (is_shared = true OR created_by = ?)", workspaceID, model.SupportInboxViewTypeCustom, userID).
		Order("LOWER(name) ASC, created_at ASC").
		Find(&views).Error; err != nil {
		return nil, fmt.Errorf("list support inbox views: %w", err)
	}
	return views, nil
}

func (r *SupportInboxViewRepository) ListBuiltinByUser(ctx context.Context, workspaceID, userID string) ([]model.SupportInboxView, error) {
	var views []model.SupportInboxView
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND created_by = ? AND view_type IN ?", workspaceID, userID, []string{model.SupportInboxViewTypeDefault, model.SupportInboxViewTypeTeam}).
		Order("view_type ASC, view_key ASC").
		Find(&views).Error; err != nil {
		return nil, fmt.Errorf("list support inbox builtin views: %w", err)
	}
	return views, nil
}

func (r *SupportInboxViewRepository) UpsertBuiltin(ctx context.Context, view *model.SupportInboxView) error {
	if view == nil || view.ViewKey == nil {
		return fmt.Errorf("view key is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.SupportInboxView
		err := tx.
			Where("workspace_id = ? AND created_by = ? AND view_type = ? AND view_key = ?", view.WorkspaceID, view.CreatedBy, view.ViewType, *view.ViewKey).
			First(&existing).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("get support inbox builtin view: %w", err)
			}
			if err := tx.Create(view).Error; err != nil {
				return fmt.Errorf("create support inbox builtin view: %w", err)
			}
			return nil
		}
		existing.Name = view.Name
		existing.Filters = view.Filters
		existing.IsShared = false
		if err := tx.Save(&existing).Error; err != nil {
			return fmt.Errorf("update support inbox builtin view: %w", err)
		}
		*view = existing
		return nil
	})
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
