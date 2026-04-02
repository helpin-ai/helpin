package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMExternalLinkRepository handles DB operations for external links.
type PMExternalLinkRepository struct {
	db *gorm.DB
}

// NewPMExternalLinkRepository creates a new PMExternalLinkRepository.
func NewPMExternalLinkRepository(db *gorm.DB) *PMExternalLinkRepository {
	return &PMExternalLinkRepository{db: db}
}

// List returns external links for a task.
func (r *PMExternalLinkRepository) List(ctx context.Context, storyID string) ([]model.PMExternalLink, error) {
	var links []model.PMExternalLink
	if err := r.db.WithContext(ctx).
		Where("task_id = ?", storyID).
		Order("created_at ASC").
		Find(&links).Error; err != nil {
		return nil, fmt.Errorf("list external links: %w", err)
	}
	return links, nil
}

// GetByID returns an external link.
func (r *PMExternalLinkRepository) GetByID(ctx context.Context, id string) (*model.PMExternalLink, error) {
	var link model.PMExternalLink
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get external link: %w", err)
	}
	return &link, nil
}

// Create inserts an external link.
func (r *PMExternalLinkRepository) Create(ctx context.Context, link *model.PMExternalLink) error {
	if err := r.db.WithContext(ctx).Create(link).Error; err != nil {
		return fmt.Errorf("create external link: %w", err)
	}
	return nil
}

// Update updates an external link.
func (r *PMExternalLinkRepository) Update(ctx context.Context, link *model.PMExternalLink) error {
	if err := r.db.WithContext(ctx).Save(link).Error; err != nil {
		return fmt.Errorf("update external link: %w", err)
	}
	return nil
}

// Delete removes an external link.
func (r *PMExternalLinkRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMExternalLink{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete external link: %w", err)
	}
	return nil
}

// Count returns number of external links for a task.
func (r *PMExternalLinkRepository) Count(ctx context.Context, storyID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.PMExternalLink{}).Where("task_id = ?", storyID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count external links: %w", err)
	}
	return count, nil
}
