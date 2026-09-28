package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PMKeyResultRepository handles DB operations for key results.
type PMKeyResultRepository struct {
	db *gorm.DB
}

// NewPMKeyResultRepository creates a new PMKeyResultRepository.
func NewPMKeyResultRepository(db *gorm.DB) *PMKeyResultRepository {
	return &PMKeyResultRepository{db: db}
}

// List returns key results for an objective.
func (r *PMKeyResultRepository) List(ctx context.Context, objectiveID string) ([]model.PMKeyResult, error) {
	var results []model.PMKeyResult
	if err := r.db.WithContext(ctx).
		Where("objective_id = ?", objectiveID).
		Order("position ASC, created_at ASC").
		Find(&results).Error; err != nil {
		return nil, fmt.Errorf("list key results: %w", err)
	}
	return results, nil
}

// GetByID returns a key result by ID.
// When workspaceID is provided, the query is scoped to that workspace
// via the parent objective to prevent cross-workspace data access.
func (r *PMKeyResultRepository) GetByID(ctx context.Context, id string, workspaceID ...string) (*model.PMKeyResult, error) {
	var kr model.PMKeyResult
	q := r.db.WithContext(ctx).Where("id = ?", id)
	if len(workspaceID) > 0 && workspaceID[0] != "" {
		q = q.Where("objective_id IN (SELECT id FROM pm_objectives WHERE workspace_id = ?)", workspaceID[0])
	}
	if err := q.First(&kr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get key result: %w", err)
	}
	return &kr, nil
}

// Create inserts a key result.
func (r *PMKeyResultRepository) Create(ctx context.Context, kr *model.PMKeyResult) error {
	if err := r.db.WithContext(ctx).Create(kr).Error; err != nil {
		return fmt.Errorf("create key result: %w", err)
	}
	return nil
}

// UpdateWithActivity applies a mutation to the locked latest result and commits its
// value history in the same transaction.
func (r *PMKeyResultRepository) UpdateWithActivity(ctx context.Context, id string, mutate func(*model.PMKeyResult) (*model.PMActivityLog, error)) (*model.PMKeyResult, error) {
	var result model.PMKeyResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&result, "id = ?", id).Error; err != nil {
			return err
		}
		activity, err := mutate(&result)
		if err != nil {
			return err
		}
		if err := tx.Save(&result).Error; err != nil {
			return err
		}
		if activity != nil {
			return NewPMActivityRepository(tx).Create(ctx, activity)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update key result: %w", err)
	}
	return &result, nil
}

// Delete hard-deletes a key result.
func (r *PMKeyResultRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&model.PMKeyResult{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete key result: %w", err)
	}
	return nil
}
