package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMViewRepository handles DB operations for views.
type PMViewRepository struct {
	db *gorm.DB
}

// NewPMViewRepository creates a new PMViewRepository.
func NewPMViewRepository(db *gorm.DB) *PMViewRepository {
	return &PMViewRepository{db: db}
}

// ListByWorkspace lists views visible to the given user (shared or owned).
func (r *PMViewRepository) ListByWorkspace(ctx context.Context, workspaceID, userID string) ([]model.PMView, error) {
	var views []model.PMView
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND (is_shared = true OR created_by = ?)", workspaceID, userID).
		Order("position ASC, created_at ASC").
		Find(&views).Error; err != nil {
		return nil, fmt.Errorf("list views: %w", err)
	}
	return views, nil
}

// GetByID returns a view by ID.
func (r *PMViewRepository) GetByID(ctx context.Context, id string) (*model.PMView, error) {
	var view model.PMView
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&view).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get view: %w", err)
	}
	return &view, nil
}

// Create inserts a view.
func (r *PMViewRepository) Create(ctx context.Context, view *model.PMView) error {
	if err := r.db.WithContext(ctx).Create(view).Error; err != nil {
		return fmt.Errorf("create view: %w", err)
	}
	return nil
}

// Update updates a view.
func (r *PMViewRepository) Update(ctx context.Context, view *model.PMView) error {
	if err := r.db.WithContext(ctx).Save(view).Error; err != nil {
		return fmt.Errorf("update view: %w", err)
	}
	return nil
}

// Delete hard-deletes a view.
func (r *PMViewRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMView{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete view: %w", err)
	}
	return nil
}
