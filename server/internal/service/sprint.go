package service

import (
	"context"
	"fmt"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// SprintService handles sprint business logic.
type SprintService struct {
	sprintRepo  *repository.SprintRepository
	scoringRepo *repository.ScoringRepository
}

// NewSprintService creates a new SprintService.
func NewSprintService(sprintRepo *repository.SprintRepository, scoringRepo *repository.ScoringRepository) *SprintService {
	return &SprintService{sprintRepo: sprintRepo, scoringRepo: scoringRepo}
}

// List returns all sprints for a quarter.
func (s *SprintService) List(ctx context.Context, quarterID string) ([]model.Sprint, error) {
	return s.sprintRepo.List(ctx, quarterID)
}

// Get returns a sprint by ID.
func (s *SprintService) Get(ctx context.Context, id string) (*model.Sprint, error) {
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
func (s *SprintService) GetIndividualChecks(ctx context.Context, sprintID string) ([]model.IndividualCheck, error) {
	return s.scoringRepo.GetChecksBySprint(ctx, sprintID)
}

// UpsertIndividualCheck creates or updates an individual check.
func (s *SprintService) UpsertIndividualCheck(ctx context.Context, req model.UpsertCheckRequest, scoredBy string) (*model.IndividualCheck, error) {
	if req.SprintID == "" || req.WorkspaceID == "" || req.EmployeeID == "" || req.CriteriaID == "" {
		return nil, fmt.Errorf("sprint_id, workspace_id, employee_id, and criteria_id are required")
	}
	return s.scoringRepo.UpsertCheck(ctx, req, scoredBy)
}

// Lock locks a sprint.
func (s *SprintService) Lock(ctx context.Context, id, lockedBy string) (*model.Sprint, error) {
	return s.sprintRepo.Lock(ctx, id, lockedBy)
}

// Unlock unlocks a sprint.
func (s *SprintService) Unlock(ctx context.Context, id string) (*model.Sprint, error) {
	return s.sprintRepo.Unlock(ctx, id)
}
