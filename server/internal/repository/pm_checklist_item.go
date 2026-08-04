package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMChecklistItemRepository handles DB operations for checklist items.
type PMChecklistItemRepository struct {
	db *gorm.DB
}

// NewPMChecklistItemRepository creates a new PMChecklistItemRepository.
func NewPMChecklistItemRepository(db *gorm.DB) *PMChecklistItemRepository {
	return &PMChecklistItemRepository{db: db}
}

// List returns checklist items for a task ordered by position.
func (r *PMChecklistItemRepository) List(ctx context.Context, taskID string) ([]model.PMChecklistItem, error) {
	var items []model.PMChecklistItem
	if err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("position ASC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list checklist items: %w", err)
	}
	return items, nil
}

// ListPage returns a bounded checklist page and the full task checklist count.
func (r *PMChecklistItemRepository) ListPage(ctx context.Context, taskID string, limit, offset int) ([]model.PMChecklistItem, int64, error) {
	var total int64
	query := r.db.WithContext(ctx).Model(&model.PMChecklistItem{}).Where("task_id = ?", taskID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count checklist items: %w", err)
	}
	var items []model.PMChecklistItem
	if err := query.Order("position ASC, created_at ASC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list checklist item page: %w", err)
	}
	return items, total, nil
}

// GetByID returns a checklist item.
func (r *PMChecklistItemRepository) GetByID(ctx context.Context, id string) (*model.PMChecklistItem, error) {
	var item model.PMChecklistItem
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get checklist item: %w", err)
	}
	return &item, nil
}

// Create inserts a checklist item.
func (r *PMChecklistItemRepository) Create(ctx context.Context, item *model.PMChecklistItem) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create checklist item: %w", err)
	}
	return nil
}

// Update updates a checklist item.
func (r *PMChecklistItemRepository) Update(ctx context.Context, item *model.PMChecklistItem) error {
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return fmt.Errorf("update checklist item: %w", err)
	}
	return nil
}

// Delete removes a checklist item.
func (r *PMChecklistItemRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMChecklistItem{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete checklist item: %w", err)
	}
	return nil
}

// Count returns number of checklist items for a task.
func (r *PMChecklistItemRepository) Count(ctx context.Context, taskID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.PMChecklistItem{}).Where("task_id = ?", taskID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count checklist items: %w", err)
	}
	return count, nil
}
