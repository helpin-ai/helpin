package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// RewardSprintRepository handles database operations for the sprints table.
type RewardSprintRepository struct {
	db *gorm.DB
}

// NewRewardSprintRepository creates a new RewardSprintRepository.
func NewRewardSprintRepository(db *gorm.DB) *RewardSprintRepository {
	return &RewardSprintRepository{db: db}
}

// BulkCreate inserts multiple sprints in a single transaction.
func (r *RewardSprintRepository) BulkCreate(ctx context.Context, sprints []model.RewardSprint) ([]model.RewardSprint, error) {
	var created []model.RewardSprint
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, s := range sprints {
			out := model.RewardSprint{
				QuarterID:    s.QuarterID,
				WorkspaceID:  s.WorkspaceID,
				SprintNumber: s.SprintNumber,
				StartDate:    s.StartDate,
				EndDate:      s.EndDate,
			}
			if err := tx.Create(&out).Error; err != nil {
				return fmt.Errorf("insert sprint %d: %w", s.SprintNumber, err)
			}
			created = append(created, out)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// List returns all sprints for a quarter.
func (r *RewardSprintRepository) List(ctx context.Context, quarterID string) ([]model.RewardSprint, error) {
	var sprints []model.RewardSprint
	err := r.db.WithContext(ctx).
		Where("quarter_id = ?", quarterID).
		Order("sprint_number ASC").
		Find(&sprints).Error
	if err != nil {
		return nil, fmt.Errorf("list sprints: %w", err)
	}
	return sprints, nil
}

// GetByID returns a sprint by its ID.
func (r *RewardSprintRepository) GetByID(ctx context.Context, id string) (*model.RewardSprint, error) {
	s := &model.RewardSprint{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get sprint by id: %w", err)
	}
	return s, nil
}

// Lock sets the sprint status to locked and records who locked it and when.
func (r *RewardSprintRepository) Lock(ctx context.Context, id, lockedBy string) (*model.RewardSprint, error) {
	now := time.Now()
	updates := map[string]interface{}{
		"status":    "locked",
		"locked_at": now,
		"locked_by": lockedBy,
	}
	if err := r.db.WithContext(ctx).Model(&model.RewardSprint{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("lock sprint: %w", err)
	}

	s := &model.RewardSprint{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(s).Error; err != nil {
		return nil, fmt.Errorf("lock sprint: %w", err)
	}
	return s, nil
}

// Unlock removes the lock from a sprint.
func (r *RewardSprintRepository) Unlock(ctx context.Context, id string) (*model.RewardSprint, error) {
	updates := map[string]interface{}{
		"status":    "active",
		"locked_at": nil,
		"locked_by": nil,
	}
	if err := r.db.WithContext(ctx).Model(&model.RewardSprint{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("unlock sprint: %w", err)
	}

	s := &model.RewardSprint{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(s).Error; err != nil {
		return nil, fmt.Errorf("unlock sprint: %w", err)
	}
	return s, nil
}
