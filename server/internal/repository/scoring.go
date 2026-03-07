package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// RewardScoringRepository handles database operations for individual_checks.
type RewardScoringRepository struct {
	db *gorm.DB
}

// NewRewardScoringRepository creates a new RewardScoringRepository.
func NewRewardScoringRepository(db *gorm.DB) *RewardScoringRepository {
	return &RewardScoringRepository{db: db}
}

// UpsertCheck inserts or updates an individual check.
func (r *RewardScoringRepository) UpsertCheck(ctx context.Context, req model.UpsertRewardCheckRequest, scoredBy string) (*model.RewardIndividualCheck, error) {
	var result *model.RewardIndividualCheck
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		employeeID, err := resolveRewardEmployeeIDTx(ctx, tx, req.WorkspaceID, req.EmployeeID)
		if err != nil {
			return err
		}
		ic := &model.RewardIndividualCheck{
			SprintID:    req.SprintID,
			WorkspaceID: req.WorkspaceID,
			EmployeeID:  employeeID,
			ScoredBy:    &scoredBy,
			CriteriaID:  req.CriteriaID,
			Answer:      req.Answer,
			Notes:       req.Notes,
		}
		if err := tx.
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "sprint_id"}, {Name: "employee_id"}, {Name: "criteria_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"answer", "notes", "scored_by"}),
			}).
			Create(ic).Error; err != nil {
			return fmt.Errorf("upsert check: %w", err)
		}
		result = &model.RewardIndividualCheck{}
		if err := tx.
			Where("sprint_id = ? AND employee_id = ? AND criteria_id = ?", req.SprintID, employeeID, req.CriteriaID).
			First(result).Error; err != nil {
			return fmt.Errorf("upsert check: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetChecksBySprint returns all individual checks for a sprint.
func (r *RewardScoringRepository) GetChecksBySprint(ctx context.Context, sprintID string) ([]model.RewardIndividualCheck, error) {
	var checks []model.RewardIndividualCheck
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
func (r *RewardScoringRepository) GetChecksByEmployee(ctx context.Context, sprintID, employeeID string) ([]model.RewardIndividualCheck, error) {
	var checks []model.RewardIndividualCheck
	err := r.db.WithContext(ctx).
		Where("sprint_id = ? AND employee_id = ?", sprintID, employeeID).
		Order("criteria_id").
		Find(&checks).Error
	if err != nil {
		return nil, fmt.Errorf("get checks by employee: %w", err)
	}
	return checks, nil
}
