package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// QuarterRepository handles database operations for the quarters table.
type QuarterRepository struct {
	db *gorm.DB
}

// NewQuarterRepository creates a new QuarterRepository.
func NewQuarterRepository(db *gorm.DB) *QuarterRepository {
	return &QuarterRepository{db: db}
}

// Create inserts a new quarter.
func (r *QuarterRepository) Create(ctx context.Context, workspaceID, name, startDate, endDate string, createdBy *string) (*model.Quarter, error) {
	q := &model.Quarter{
		WorkspaceID: workspaceID,
		Name:        name,
		StartDate:   startDate,
		EndDate:     endDate,
		CreatedBy:   createdBy,
	}
	if err := r.db.WithContext(ctx).Create(q).Error; err != nil {
		return nil, fmt.Errorf("create quarter: %w", err)
	}
	return q, nil
}

// List returns all quarters for a workspace.
func (r *QuarterRepository) List(ctx context.Context, workspaceID string) ([]model.Quarter, error) {
	var quarters []model.Quarter
	err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("start_date DESC").
		Find(&quarters).Error
	if err != nil {
		return nil, fmt.Errorf("list quarters: %w", err)
	}
	return quarters, nil
}

// GetByID returns a quarter by its ID.
func (r *QuarterRepository) GetByID(ctx context.Context, id string) (*model.Quarter, error) {
	q := &model.Quarter{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(q).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get quarter by id: %w", err)
	}
	return q, nil
}

// UpdateStatus updates a quarter's status.
func (r *QuarterRepository) UpdateStatus(ctx context.Context, id, status string) (*model.Quarter, error) {
	if err := r.db.WithContext(ctx).Model(&model.Quarter{}).Where("id = ?", id).Update("status", status).Error; err != nil {
		return nil, fmt.Errorf("update quarter status: %w", err)
	}

	q := &model.Quarter{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(q).Error; err != nil {
		return nil, fmt.Errorf("update quarter status: %w", err)
	}
	return q, nil
}
