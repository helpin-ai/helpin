package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// PMLabelRepository handles DB operations for labels.
type PMLabelRepository struct {
	db *gorm.DB
}

// NewPMLabelRepository creates a new PMLabelRepository.
func NewPMLabelRepository(db *gorm.DB) *PMLabelRepository {
	return &PMLabelRepository{db: db}
}

// ListByWorkspace lists labels by workspace.
func (r *PMLabelRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.PMLabel, error) {
	var labels []model.PMLabel
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("name ASC").
		Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	return labels, nil
}

// GetByID returns a label by ID.
func (r *PMLabelRepository) GetByID(ctx context.Context, id string) (*model.PMLabel, error) {
	var label model.PMLabel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&label).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get label: %w", err)
	}
	return &label, nil
}

// GetByName returns a label by workspace/name.
func (r *PMLabelRepository) GetByName(ctx context.Context, workspaceID, name string) (*model.PMLabel, error) {
	var label model.PMLabel
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND LOWER(name) = LOWER(?)", workspaceID, name).
		First(&label).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get label by name: %w", err)
	}
	return &label, nil
}

// Create inserts a label.
func (r *PMLabelRepository) Create(ctx context.Context, label *model.PMLabel) error {
	if err := r.db.WithContext(ctx).Create(label).Error; err != nil {
		return fmt.Errorf("create label: %w", err)
	}
	return nil
}

// Update updates a label.
func (r *PMLabelRepository) Update(ctx context.Context, label *model.PMLabel) error {
	if err := r.db.WithContext(ctx).Save(label).Error; err != nil {
		return fmt.Errorf("update label: %w", err)
	}
	return nil
}

// Delete hard-deletes a label.
func (r *PMLabelRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMLabel{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete label: %w", err)
	}
	return nil
}
