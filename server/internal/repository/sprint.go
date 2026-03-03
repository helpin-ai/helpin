package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// SprintRepository handles database operations for the sprints table.
type SprintRepository struct {
	db *gorm.DB
}

// NewSprintRepository creates a new SprintRepository.
func NewSprintRepository(db *gorm.DB) *SprintRepository {
	return &SprintRepository{db: db}
}

// BulkCreate inserts multiple sprints in a single transaction.
func (r *SprintRepository) BulkCreate(ctx context.Context, sprints []model.Sprint) ([]model.Sprint, error) {
	var created []model.Sprint
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, s := range sprints {
			out := model.Sprint{
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
func (r *SprintRepository) List(ctx context.Context, quarterID string) ([]model.Sprint, error) {
	var sprints []model.Sprint
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
func (r *SprintRepository) GetByID(ctx context.Context, id string) (*model.Sprint, error) {
	s := &model.Sprint{}
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
func (r *SprintRepository) Lock(ctx context.Context, id, lockedBy string) (*model.Sprint, error) {
	now := time.Now()
	updates := map[string]interface{}{
		"status":    "locked",
		"locked_at": now,
		"locked_by": lockedBy,
	}
	if err := r.db.WithContext(ctx).Model(&model.Sprint{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("lock sprint: %w", err)
	}

	s := &model.Sprint{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(s).Error; err != nil {
		return nil, fmt.Errorf("lock sprint: %w", err)
	}
	return s, nil
}

// Unlock removes the lock from a sprint.
func (r *SprintRepository) Unlock(ctx context.Context, id string) (*model.Sprint, error) {
	updates := map[string]interface{}{
		"status":    "active",
		"locked_at": nil,
		"locked_by": nil,
	}
	if err := r.db.WithContext(ctx).Model(&model.Sprint{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("unlock sprint: %w", err)
	}

	s := &model.Sprint{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(s).Error; err != nil {
		return nil, fmt.Errorf("unlock sprint: %w", err)
	}
	return s, nil
}
