package service

import (
	"context"
	"fmt"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// GoalService handles goal business logic.
type GoalService struct {
	goalRepo *repository.GoalRepository
}

// NewGoalService creates a new GoalService.
func NewGoalService(goalRepo *repository.GoalRepository) *GoalService {
	return &GoalService{goalRepo: goalRepo}
}

// Create creates a new company goal with team contributions.
func (s *GoalService) Create(ctx context.Context, req model.CreateGoalRequest, createdBy string) (*model.CompanyGoalWithContributions, error) {
	if req.WorkspaceID == "" || req.QuarterID == "" || req.Title == "" || req.GoalType == "" {
		return nil, fmt.Errorf("workspace_id, quarter_id, title, and goal_type are required")
	}
	return s.goalRepo.CreateCompanyGoal(ctx, req, createdBy)
}

// List returns all company goals with team contributions for a workspace/quarter.
func (s *GoalService) List(ctx context.Context, workspaceID, quarterID string) ([]model.CompanyGoalWithContributions, error) {
	return s.goalRepo.ListCompanyGoals(ctx, workspaceID, quarterID)
}

// UpsertSprintGoal creates or updates a sprint goal.
func (s *GoalService) UpsertSprintGoal(ctx context.Context, req model.UpsertSprintGoalRequest) (*model.SprintGoal, error) {
	if req.SprintID == "" || req.TeamID == "" || req.Title == "" {
		return nil, fmt.Errorf("sprint_id, team_id, and title are required")
	}
	return s.goalRepo.UpsertSprintGoal(ctx, req)
}

// ListSprintGoals returns sprint goals for a sprint, optionally filtered by team.
func (s *GoalService) ListSprintGoals(ctx context.Context, sprintID string, teamID *string) ([]model.SprintGoal, error) {
	return s.goalRepo.ListSprintGoals(ctx, sprintID, teamID)
}
