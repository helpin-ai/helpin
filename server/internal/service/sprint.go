package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// RewardSprintService handles sprint business logic.
type RewardSprintService struct {
	sprintRepo  *repository.RewardSprintRepository
	scoringRepo *repository.RewardScoringRepository
}

// NewRewardSprintService creates a new RewardSprintService.
func NewRewardSprintService(sprintRepo *repository.RewardSprintRepository, scoringRepo *repository.RewardScoringRepository) *RewardSprintService {
	return &RewardSprintService{sprintRepo: sprintRepo, scoringRepo: scoringRepo}
}

// List returns all sprints for a quarter.
func (s *RewardSprintService) List(ctx context.Context, quarterID string) ([]model.RewardSprint, error) {
	return s.sprintRepo.List(ctx, quarterID)
}

// Get returns a sprint by ID.
func (s *RewardSprintService) Get(ctx context.Context, id string) (*model.RewardSprint, error) {
	sprint, err := s.sprintRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sprint == nil {
		return nil, fmt.Errorf("sprint not found")
	}
	return sprint, nil
}

// GetIndividualChecks returns all individual checks for a sprint.
func (s *RewardSprintService) GetIndividualChecks(ctx context.Context, sprintID string) ([]model.RewardIndividualCheck, error) {
	return s.scoringRepo.GetChecksBySprint(ctx, sprintID)
}

// UpsertIndividualCheck creates or updates an individual check.
func (s *RewardSprintService) UpsertIndividualCheck(ctx context.Context, req model.UpsertRewardCheckRequest, scoredBy string) (*model.RewardIndividualCheck, error) {
	if req.SprintID == "" || req.WorkspaceID == "" || req.EmployeeID == "" || req.CriteriaID == "" {
		return nil, fmt.Errorf("sprint_id, workspace_id, employee_id, and criteria_id are required")
	}
	return s.scoringRepo.UpsertCheck(ctx, req, scoredBy)
}

// Lock locks a sprint.
func (s *RewardSprintService) Lock(ctx context.Context, id, lockedBy string) (*model.RewardSprint, error) {
	return s.sprintRepo.Lock(ctx, id, lockedBy)
}

// Unlock unlocks a sprint.
func (s *RewardSprintService) Unlock(ctx context.Context, id string) (*model.RewardSprint, error) {
	return s.sprintRepo.Unlock(ctx, id)
}
