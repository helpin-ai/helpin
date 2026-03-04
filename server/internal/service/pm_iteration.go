package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
	"github.com/d4interactive/teampulse/server/internal/websocket"
)

// PMIterationService contains iteration business logic.
type PMIterationService struct {
	iterationRepo   *repository.PMIterationRepository
	activityService *PMActivityService
	wsPublisher     *websocket.Publisher
}

// NewPMIterationService creates a new PMIterationService.
func NewPMIterationService(iterationRepo *repository.PMIterationRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher) *PMIterationService {
	return &PMIterationService{iterationRepo: iterationRepo, activityService: activityService, wsPublisher: wsPublisher}
}

// List returns iterations with filters.
func (s *PMIterationService) List(ctx context.Context, workspaceID string, filters model.PMIterationListFilters) ([]model.IterationWithStats, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	iterations, err := s.iterationRepo.List(ctx, workspaceID, filters)
	if err != nil {
		return nil, err
	}
	result := make([]model.IterationWithStats, 0, len(iterations))
	for _, iteration := range iterations {
		withStats, err := s.iterationRepo.GetWithStats(ctx, iteration.ID)
		if err != nil {
			return nil, err
		}
		if withStats != nil {
			result = append(result, *withStats)
		}
	}
	return result, nil
}

// GetByID returns iteration with stats.
func (s *PMIterationService) GetByID(ctx context.Context, id string) (*model.IterationWithStats, error) {
	iteration, err := s.iterationRepo.GetWithStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if iteration == nil {
		return nil, fmt.Errorf("iteration not found")
	}
	return iteration, nil
}

// Create creates an iteration.
func (s *PMIterationService) Create(ctx context.Context, req model.CreateIterationRequest, actorID string) (*model.IterationWithStats, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if !req.EndDate.After(req.StartDate) {
		return nil, fmt.Errorf("end_date must be after start_date")
	}
	overlap, err := s.iterationRepo.HasDateOverlap(ctx, req.WorkspaceID, req.TeamID, req.StartDate, req.EndDate, nil)
	if err != nil {
		return nil, err
	}
	if overlap {
		return nil, fmt.Errorf("iteration date range overlaps with another iteration for the same team")
	}

	iteration := &model.PMIteration{
		WorkspaceID: req.WorkspaceID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		TeamID:      req.TeamID,
	}
	if actorID != "" {
		iteration.CreatedBy = &actorID
	}

	if err := s.iterationRepo.Create(ctx, iteration); err != nil {
		return nil, err
	}
	if len(req.LabelIDs) > 0 {
		if err := s.iterationRepo.ReplaceLabels(ctx, iteration.ID, req.LabelIDs); err != nil {
			return nil, err
		}
	}

	_ = s.activityService.Log(ctx, iteration.WorkspaceID, "iteration", iteration.ID, optionalActor(actorID), "created", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "iteration", EntityID: iteration.ID, WorkspaceID: iteration.WorkspaceID, ActorID: actorID})
	return s.iterationRepo.GetWithStats(ctx, iteration.ID)
}

// Update updates an iteration.
func (s *PMIterationService) Update(ctx context.Context, id string, req model.UpdateIterationRequest, actorID string) (*model.IterationWithStats, error) {
	current, err := s.iterationRepo.GetWithStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("iteration not found")
	}
	iteration := current.Iteration

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		iteration.Name = name
	}
	if req.Description != nil {
		iteration.Description = req.Description
	}
	if req.StartDate != nil {
		iteration.StartDate = *req.StartDate
	}
	if req.EndDate != nil {
		iteration.EndDate = *req.EndDate
	}
	if req.TeamID != nil {
		iteration.TeamID = req.TeamID
	}
	if req.Archived != nil {
		iteration.Archived = *req.Archived
	}

	if !iteration.EndDate.After(iteration.StartDate) {
		return nil, fmt.Errorf("end_date must be after start_date")
	}
	excludeID := id
	overlap, err := s.iterationRepo.HasDateOverlap(ctx, iteration.WorkspaceID, iteration.TeamID, iteration.StartDate, iteration.EndDate, &excludeID)
	if err != nil {
		return nil, err
	}
	if overlap {
		return nil, fmt.Errorf("iteration date range overlaps with another iteration for the same team")
	}

	if err := s.iterationRepo.Update(ctx, &iteration); err != nil {
		return nil, err
	}
	if req.LabelIDs != nil {
		if err := s.iterationRepo.ReplaceLabels(ctx, iteration.ID, req.LabelIDs); err != nil {
			return nil, err
		}
	}

	_ = s.activityService.Log(ctx, iteration.WorkspaceID, "iteration", iteration.ID, optionalActor(actorID), "updated", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "iteration", EntityID: iteration.ID, WorkspaceID: iteration.WorkspaceID, ActorID: actorID})
	return s.iterationRepo.GetWithStats(ctx, iteration.ID)
}

// Delete deletes an iteration.
func (s *PMIterationService) Delete(ctx context.Context, id string, actorID string) error {
	current, err := s.iterationRepo.GetWithStats(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("iteration not found")
	}
	if err := s.iterationRepo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, current.Iteration.WorkspaceID, "iteration", id, optionalActor(actorID), "deleted", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "iteration", EntityID: id, WorkspaceID: current.Iteration.WorkspaceID, ActorID: actorID})
	return nil
}

// GetCurrentIteration returns active iteration for workspace/team.
func (s *PMIterationService) GetCurrentIteration(ctx context.Context, workspaceID string, teamID *string) (*model.PMIteration, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.iterationRepo.GetCurrentIteration(ctx, workspaceID, teamID)
}

// ListStories returns stories in an iteration.
func (s *PMIterationService) ListStories(ctx context.Context, iterationID string) ([]model.PMStory, error) {
	return s.iterationRepo.ListStories(ctx, iterationID)
}

// ComputeStats returns computed story/point stats for an iteration.
func (s *PMIterationService) ComputeStats(ctx context.Context, iterationID string) (model.PMIterationStats, error) {
	return s.iterationRepo.ComputeStats(ctx, iterationID)
}

func toDay(ts time.Time) time.Time {
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
}
