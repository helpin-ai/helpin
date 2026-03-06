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

// PMSprintService contains sprint business logic.
type PMSprintService struct {
	sprintRepo      *repository.PMSprintRepository
	labelRepo       *repository.PMLabelRepository
	activityService *PMActivityService
	wsPublisher     *websocket.Publisher
}

// NewPMSprintService creates a new PMSprintService.
func NewPMSprintService(sprintRepo *repository.PMSprintRepository, labelRepo *repository.PMLabelRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher) *PMSprintService {
	return &PMSprintService{sprintRepo: sprintRepo, labelRepo: labelRepo, activityService: activityService, wsPublisher: wsPublisher}
}

// List returns sprints with filters.
func (s *PMSprintService) List(ctx context.Context, workspaceID string, filters model.PMSprintListFilters) ([]model.SprintWithStats, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	sprints, err := s.sprintRepo.List(ctx, workspaceID, filters)
	if err != nil {
		return nil, err
	}
	result := make([]model.SprintWithStats, 0, len(sprints))
	for _, sprint := range sprints {
		withStats, err := s.sprintRepo.GetWithStats(ctx, sprint.ID)
		if err != nil {
			return nil, err
		}
		if withStats != nil {
			result = append(result, *withStats)
		}
	}
	return result, nil
}

// GetByID returns sprint with stats.
func (s *PMSprintService) GetByID(ctx context.Context, id string) (*model.SprintWithStats, error) {
	sprint, err := s.sprintRepo.GetWithStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if sprint == nil {
		return nil, fmt.Errorf("sprint not found")
	}
	return sprint, nil
}

// Create creates a sprint.
func (s *PMSprintService) Create(ctx context.Context, req model.CreateSprintRequest, actorID string) (*model.SprintWithStats, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if !req.EndDate.After(req.StartDate) {
		return nil, fmt.Errorf("end_date must be after start_date")
	}
	overlap, err := s.sprintRepo.HasDateOverlap(ctx, req.WorkspaceID, req.TeamID, req.StartDate, req.EndDate, nil)
	if err != nil {
		return nil, err
	}
	if overlap {
		return nil, fmt.Errorf("sprint date range overlaps with another sprint for the same team")
	}

	sprint := &model.PMSprint{
		WorkspaceID: req.WorkspaceID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		TeamID:      req.TeamID,
	}
	if actorID != "" {
		sprint.CreatedBy = &actorID
	}

	if err := s.sprintRepo.Create(ctx, sprint); err != nil {
		return nil, err
	}
	if len(req.LabelIDs) > 0 {
		if err := validateLabelScope(ctx, s.labelRepo, req.WorkspaceID, req.LabelIDs, allowedTeamIDs(req.TeamID)); err != nil {
			return nil, err
		}
		if err := s.sprintRepo.ReplaceLabels(ctx, sprint.ID, req.LabelIDs); err != nil {
			return nil, err
		}
	}

	_ = s.activityService.Log(ctx, sprint.WorkspaceID, "sprint", sprint.ID, optionalActor(actorID), "created", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "sprint", EntityID: sprint.ID, WorkspaceID: sprint.WorkspaceID, ActorID: actorID})
	return s.sprintRepo.GetWithStats(ctx, sprint.ID)
}

// Update updates a sprint.
func (s *PMSprintService) Update(ctx context.Context, id string, req model.UpdateSprintRequest, actorID string) (*model.SprintWithStats, error) {
	current, err := s.sprintRepo.GetWithStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("sprint not found")
	}
	sprint := current.Sprint

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		sprint.Name = name
	}
	if req.Description != nil {
		sprint.Description = req.Description
	}
	if req.StartDate != nil {
		sprint.StartDate = *req.StartDate
	}
	if req.EndDate != nil {
		sprint.EndDate = *req.EndDate
	}
	if req.TeamID != nil {
		sprint.TeamID = req.TeamID
	}
	if req.Archived != nil {
		sprint.Archived = *req.Archived
	}

	if !sprint.EndDate.After(sprint.StartDate) {
		return nil, fmt.Errorf("end_date must be after start_date")
	}
	excludeID := id
	overlap, err := s.sprintRepo.HasDateOverlap(ctx, sprint.WorkspaceID, sprint.TeamID, sprint.StartDate, sprint.EndDate, &excludeID)
	if err != nil {
		return nil, err
	}
	if overlap {
		return nil, fmt.Errorf("sprint date range overlaps with another sprint for the same team")
	}

	if err := s.sprintRepo.Update(ctx, &sprint); err != nil {
		return nil, err
	}
	if req.LabelIDs != nil {
		if err := validateLabelScope(ctx, s.labelRepo, sprint.WorkspaceID, req.LabelIDs, allowedTeamIDs(sprint.TeamID)); err != nil {
			return nil, err
		}
		if err := s.sprintRepo.ReplaceLabels(ctx, sprint.ID, req.LabelIDs); err != nil {
			return nil, err
		}
	}

	_ = s.activityService.Log(ctx, sprint.WorkspaceID, "sprint", sprint.ID, optionalActor(actorID), "updated", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "sprint", EntityID: sprint.ID, WorkspaceID: sprint.WorkspaceID, ActorID: actorID})
	return s.sprintRepo.GetWithStats(ctx, sprint.ID)
}

// Delete deletes a sprint.
func (s *PMSprintService) Delete(ctx context.Context, id string, actorID string) error {
	current, err := s.sprintRepo.GetWithStats(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("sprint not found")
	}
	if err := s.sprintRepo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, current.Sprint.WorkspaceID, "sprint", id, optionalActor(actorID), "deleted", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "sprint", EntityID: id, WorkspaceID: current.Sprint.WorkspaceID, ActorID: actorID})
	return nil
}

// GetCurrentSprint returns active sprint for workspace/team.
func (s *PMSprintService) GetCurrentSprint(ctx context.Context, workspaceID string, teamID *string) (*model.PMSprint, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.sprintRepo.GetCurrentSprint(ctx, workspaceID, teamID)
}

// ListStories returns stories in a sprint.
func (s *PMSprintService) ListStories(ctx context.Context, sprintID string) ([]model.PMStory, error) {
	return s.sprintRepo.ListStories(ctx, sprintID)
}

// ComputeStats returns computed story/point stats for a sprint.
func (s *PMSprintService) ComputeStats(ctx context.Context, sprintID string) (model.PMSprintStats, error) {
	return s.sprintRepo.ComputeStats(ctx, sprintID)
}

func toDay(ts time.Time) time.Time {
	return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
}
