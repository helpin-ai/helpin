package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// ScoringRepository handles database operations for individual_checks.
type ScoringRepository struct {
	db *gorm.DB
}

// NewScoringRepository creates a new ScoringRepository.
func NewScoringRepository(db *gorm.DB) *ScoringRepository {
	return &ScoringRepository{db: db}
}

// UpsertCheck inserts or updates an individual check.
func (r *ScoringRepository) UpsertCheck(ctx context.Context, req model.UpsertCheckRequest, scoredBy string) (*model.IndividualCheck, error) {
	ic := &model.IndividualCheck{
		SprintID:    req.SprintID,
		WorkspaceID: req.WorkspaceID,
		EmployeeID:  req.EmployeeID,
		ScoredBy:    &scoredBy,
		CriteriaID:  req.CriteriaID,
		Answer:      req.Answer,
		Notes:       req.Notes,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sprint_id"}, {Name: "employee_id"}, {Name: "criteria_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"answer", "notes", "scored_by"}),
		}).
		Create(ic).Error
	if err != nil {
		return nil, fmt.Errorf("upsert check: %w", err)
	}
	// Re-fetch to get correct ID and timestamps after upsert.
	result := &model.IndividualCheck{}
	if err := r.db.WithContext(ctx).
		Where("sprint_id = ? AND employee_id = ? AND criteria_id = ?", req.SprintID, req.EmployeeID, req.CriteriaID).
		First(result).Error; err != nil {
		return nil, fmt.Errorf("upsert check: %w", err)
	}
	return result, nil
}

// GetChecksBySprint returns all individual checks for a sprint.
func (r *ScoringRepository) GetChecksBySprint(ctx context.Context, sprintID string) ([]model.IndividualCheck, error) {
	var checks []model.IndividualCheck
	err := r.db.WithContext(ctx).
		Where("sprint_id = ?", sprintID).
		Order("employee_id, criteria_id").
		Find(&checks).Error
	if err != nil {
		return nil, fmt.Errorf("get checks by sprint: %w", err)
	}
	return checks, nil
}

// GetChecksByEmployee returns all individual checks for an employee within a sprint.
func (r *ScoringRepository) GetChecksByEmployee(ctx context.Context, sprintID, employeeID string) ([]model.IndividualCheck, error) {
	var checks []model.IndividualCheck
	err := r.db.WithContext(ctx).
		Where("sprint_id = ? AND employee_id = ?", sprintID, employeeID).
		Order("criteria_id").
		Find(&checks).Error
	if err != nil {
		return nil, fmt.Errorf("get checks by employee: %w", err)
	}
	return checks, nil
}
