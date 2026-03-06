package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// RewardQuarterRepository handles database operations for the quarters table.
type RewardQuarterRepository struct {
	db *gorm.DB
}

// NewRewardQuarterRepository creates a new RewardQuarterRepository.
func NewRewardQuarterRepository(db *gorm.DB) *RewardQuarterRepository {
	return &RewardQuarterRepository{db: db}
}

// Create inserts a new quarter.
func (r *RewardQuarterRepository) Create(ctx context.Context, workspaceID, name, startDate, endDate string, createdBy *string) (*model.RewardQuarter, error) {
	q := &model.RewardQuarter{
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
func (r *RewardQuarterRepository) List(ctx context.Context, workspaceID string) ([]model.RewardQuarter, error) {
	var quarters []model.RewardQuarter
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
func (r *RewardQuarterRepository) GetByID(ctx context.Context, id string) (*model.RewardQuarter, error) {
	q := &model.RewardQuarter{}
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
func (r *RewardQuarterRepository) UpdateStatus(ctx context.Context, id, status string) (*model.RewardQuarter, error) {
	if err := r.db.WithContext(ctx).Model(&model.RewardQuarter{}).Where("id = ?", id).Update("status", status).Error; err != nil {
		return nil, fmt.Errorf("update quarter status: %w", err)
	}

	q := &model.RewardQuarter{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(q).Error; err != nil {
		return nil, fmt.Errorf("update quarter status: %w", err)
	}
	return q, nil
}
