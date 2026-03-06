package service

import (
	"context"
	"fmt"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// RewardGoalService handles goal business logic.
type RewardGoalService struct {
	goalRepo *repository.RewardGoalRepository
}

// NewRewardGoalService creates a new RewardGoalService.
func NewRewardGoalService(goalRepo *repository.RewardGoalRepository) *RewardGoalService {
	return &RewardGoalService{goalRepo: goalRepo}
}

// Create creates a new company goal with team contributions.
func (s *RewardGoalService) Create(ctx context.Context, req model.CreateRewardGoalRequest, createdBy string) (*model.RewardCompanyGoalWithContributions, error) {
	if req.WorkspaceID == "" || req.QuarterID == "" || req.Title == "" || req.GoalType == "" {
		return nil, fmt.Errorf("workspace_id, quarter_id, title, and goal_type are required")
	}
	return s.goalRepo.CreateCompanyGoal(ctx, req, createdBy)
}

// List returns all company goals with team contributions for a workspace/quarter.
func (s *RewardGoalService) List(ctx context.Context, workspaceID, quarterID string) ([]model.RewardCompanyGoalWithContributions, error) {
	return s.goalRepo.ListCompanyGoals(ctx, workspaceID, quarterID)
}

// UpsertSprintGoal creates or updates a sprint goal.
func (s *RewardGoalService) UpsertSprintGoal(ctx context.Context, req model.UpsertRewardSprintGoalRequest) (*model.RewardSprintGoal, error) {
	if req.SprintID == "" || req.TeamID == "" || req.Title == "" {
		return nil, fmt.Errorf("sprint_id, team_id, and title are required")
	}
	return s.goalRepo.UpsertSprintGoal(ctx, req)
}

// ListSprintGoals returns sprint goals for a sprint, optionally filtered by team.
func (s *RewardGoalService) ListSprintGoals(ctx context.Context, sprintID string, teamID *string) ([]model.RewardSprintGoal, error) {
	return s.goalRepo.ListSprintGoals(ctx, sprintID, teamID)
}
