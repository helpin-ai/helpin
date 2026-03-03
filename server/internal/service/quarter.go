package service

import (
	"context"
	"fmt"
	"time"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
)

// QuarterService handles quarter business logic.
type QuarterService struct {
	quarterRepo *repository.QuarterRepository
	sprintRepo  *repository.SprintRepository
}

// NewQuarterService creates a new QuarterService.
func NewQuarterService(quarterRepo *repository.QuarterRepository, sprintRepo *repository.SprintRepository) *QuarterService {
	return &QuarterService{quarterRepo: quarterRepo, sprintRepo: sprintRepo}
}

// Create creates a quarter and auto-generates 6 two-week sprints.
func (s *QuarterService) Create(ctx context.Context, req model.CreateQuarterRequest, createdBy string) (*model.Quarter, error) {
	if req.WorkspaceID == "" || req.Name == "" || req.StartDate == "" || req.EndDate == "" {
		return nil, fmt.Errorf("workspace_id, name, start_date, and end_date are required")
	}

	q, err := s.quarterRepo.Create(ctx, req.WorkspaceID, req.Name, req.StartDate, req.EndDate, &createdBy)
	if err != nil {
		return nil, fmt.Errorf("create quarter: %w", err)
	}

	// Auto-create 6 sprints of 2 weeks each.
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("parse start_date: %w", err)
	}

	var sprints []model.Sprint
	for i := 1; i <= 6; i++ {
		sprintStart := startDate.AddDate(0, 0, (i-1)*14)
		sprintEnd := sprintStart.AddDate(0, 0, 13)
		sprints = append(sprints, model.Sprint{
			QuarterID:    q.ID,
			WorkspaceID:  req.WorkspaceID,
			SprintNumber: i,
			StartDate:    sprintStart.Format("2006-01-02"),
			EndDate:      sprintEnd.Format("2006-01-02"),
		})
	}

	_, err = s.sprintRepo.BulkCreate(ctx, sprints)
	if err != nil {
		return nil, fmt.Errorf("create sprints: %w", err)
	}

	return q, nil
}

// List returns all quarters for a workspace.
func (s *QuarterService) List(ctx context.Context, workspaceID string) ([]model.Quarter, error) {
	return s.quarterRepo.List(ctx, workspaceID)
}

// Get returns a quarter by ID.
func (s *QuarterService) Get(ctx context.Context, id string) (*model.Quarter, error) {
	q, err := s.quarterRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, fmt.Errorf("quarter not found")
	}
	return q, nil
}

// UpdateStatus updates a quarter's status.
func (s *QuarterService) UpdateStatus(ctx context.Context, id, status string) (*model.Quarter, error) {
	validStatuses := map[string]bool{"draft": true, "active": true, "completed": true, "archived": true}
	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s", status)
	}
	return s.quarterRepo.UpdateStatus(ctx, id, status)
}
