package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/d4interactive/teampulse/server/internal/model"
)

// GoalRepository handles database operations for goals, contributions, and sprint goals.
type GoalRepository struct {
	db *gorm.DB
}

// NewGoalRepository creates a new GoalRepository.
func NewGoalRepository(db *gorm.DB) *GoalRepository {
	return &GoalRepository{db: db}
}

// CreateCompanyGoal inserts a company goal and its team contributions in a transaction.
func (r *GoalRepository) CreateCompanyGoal(ctx context.Context, req model.CreateGoalRequest, createdBy string) (*model.CompanyGoalWithContributions, error) {
	var result model.CompanyGoalWithContributions

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		goal := model.CompanyGoal{
			WorkspaceID: req.WorkspaceID,
			QuarterID:   req.QuarterID,
			Title:       req.Title,
			Description: req.Description,
			GoalType:    req.GoalType,
			Baseline:    req.Baseline,
			Target:      req.Target,
			Unit:        req.Unit,
			CreatedBy:   &createdBy,
		}
		if err := tx.Create(&goal).Error; err != nil {
			return fmt.Errorf("insert company goal: %w", err)
		}

		var contributions []model.GoalTeamContribution
		for _, tc := range req.TeamContributions {
			c := model.GoalTeamContribution{
				GoalID:          goal.ID,
				TeamID:          tc.TeamID,
				ContributionPct: tc.ContributionPct,
				TargetValue:     tc.TargetValue,
				Rationale:       tc.Rationale,
			}
			if err := tx.Create(&c).Error; err != nil {
				return fmt.Errorf("insert team contribution: %w", err)
			}
			contributions = append(contributions, c)
		}

		result = model.CompanyGoalWithContributions{
			CompanyGoal:       goal,
			TeamContributions: contributions,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ListCompanyGoals returns all company goals for a workspace/quarter with their team contributions.
func (r *GoalRepository) ListCompanyGoals(ctx context.Context, workspaceID, quarterID string) ([]model.CompanyGoalWithContributions, error) {
	var goals []model.CompanyGoal
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND quarter_id = ?", workspaceID, quarterID).
		Order("created_at ASC").
		Find(&goals).Error
	if err != nil {
		return nil, fmt.Errorf("list company goals: %w", err)
	}

	var result []model.CompanyGoalWithContributions
	for _, g := range goals {
		var contribs []model.GoalTeamContribution
		if err := r.db.WithContext(ctx).
			Where("goal_id = ?", g.ID).
			Order("created_at ASC").
			Find(&contribs).Error; err != nil {
			return nil, fmt.Errorf("list contributions: %w", err)
		}
		result = append(result, model.CompanyGoalWithContributions{
			CompanyGoal:       g,
			TeamContributions: contribs,
		})
	}
	return result, nil
}

// UpsertSprintGoal inserts or updates a sprint goal.
func (r *GoalRepository) UpsertSprintGoal(ctx context.Context, req model.UpsertSprintGoalRequest) (*model.SprintGoal, error) {
	sg := &model.SprintGoal{
		SprintID:    req.SprintID,
		TeamID:      req.TeamID,
		Title:       req.Title,
		Description: req.Description,
		Weight:      req.Weight,
		Done:        req.Done,
		KRID:        req.KRID,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sprint_id"}, {Name: "team_id"}, {Name: "title"}},
			DoUpdates: clause.AssignmentColumns([]string{"description", "weight", "done", "kr_id"}),
		}).
		Create(sg).Error
	if err != nil {
		return nil, fmt.Errorf("upsert sprint goal: %w", err)
	}
	// Re-fetch to get correct ID and timestamps after upsert.
	result := &model.SprintGoal{}
	if err := r.db.WithContext(ctx).
		Where("sprint_id = ? AND team_id = ? AND title = ?", req.SprintID, req.TeamID, req.Title).
		First(result).Error; err != nil {
		return nil, fmt.Errorf("upsert sprint goal: %w", err)
	}
	return result, nil
}

// ListSprintGoals returns all sprint goals for a given sprint, optionally filtered by team.
func (r *GoalRepository) ListSprintGoals(ctx context.Context, sprintID string, teamID *string) ([]model.SprintGoal, error) {
	query := r.db.WithContext(ctx).Where("sprint_id = ?", sprintID)
	if teamID != nil && *teamID != "" {
		query = query.Where("team_id = ?", *teamID)
	}

	var goals []model.SprintGoal
	if err := query.Order("created_at ASC").Find(&goals).Error; err != nil {
		return nil, fmt.Errorf("list sprint goals: %w", err)
	}
	return goals, nil
}
