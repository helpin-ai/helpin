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

// PMEpicService contains epic business logic.
type PMEpicService struct {
	epicRepo        *repository.PMEpicRepository
	storyRepo       *repository.PMStoryRepository
	activityService *PMActivityService
	wsPublisher     *websocket.Publisher
}

// NewPMEpicService creates a new PMEpicService.
func NewPMEpicService(epicRepo *repository.PMEpicRepository, storyRepo *repository.PMStoryRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher) *PMEpicService {
	return &PMEpicService{
		epicRepo:        epicRepo,
		storyRepo:       storyRepo,
		activityService: activityService,
		wsPublisher:     wsPublisher,
	}
}

// List returns epics and computed stats.
func (s *PMEpicService) List(ctx context.Context, workspaceID string, filters model.PMEpicListFilters) ([]model.EpicWithStats, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	epics, err := s.epicRepo.List(ctx, workspaceID, filters)
	if err != nil {
		return nil, err
	}

	result := make([]model.EpicWithStats, 0, len(epics))
	for _, epic := range epics {
		withStats, err := s.epicRepo.GetWithStats(ctx, epic.ID)
		if err != nil {
			return nil, err
		}
		if withStats != nil {
			result = append(result, *withStats)
		}
	}
	return result, nil
}

// GetByID returns one epic with stats.
func (s *PMEpicService) GetByID(ctx context.Context, id string) (*model.EpicWithStats, error) {
	epic, err := s.epicRepo.GetWithStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if epic == nil {
		return nil, fmt.Errorf("epic not found")
	}
	return epic, nil
}

// Create creates an epic.
func (s *PMEpicService) Create(ctx context.Context, req model.CreateEpicRequest, actorID string) (*model.EpicWithStats, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	health := model.PMEpicHealthOnTrack
	if req.Health != nil && *req.Health != "" {
		health = *req.Health
	}
	if !isValidEpicHealth(health) {
		return nil, fmt.Errorf("invalid health value")
	}

	epic := &model.PMEpic{
		WorkspaceID:      req.WorkspaceID,
		Name:             strings.TrimSpace(req.Name),
		Description:      req.Description,
		EpicStateID:      req.EpicStateID,
		OwnerID:          req.OwnerID,
		TeamID:           req.TeamID,
		PlannedStartDate: req.PlannedStartDate,
		Deadline:         req.Deadline,
		Color:            req.Color,
		Health:           health,
		HealthComment:    req.HealthComment,
	}
	if req.Position != nil {
		epic.Position = *req.Position
	}
	if actorID != "" {
		epic.CreatedBy = &actorID
	}

	if err := s.epicRepo.Create(ctx, epic); err != nil {
		return nil, err
	}
	if len(req.LabelIDs) > 0 {
		if err := s.epicRepo.ReplaceLabels(ctx, epic.ID, req.LabelIDs); err != nil {
			return nil, err
		}
	}

	if err := s.syncProgress(ctx, epic.ID); err != nil {
		return nil, err
	}
	_ = s.activityService.Log(ctx, epic.WorkspaceID, "epic", epic.ID, optionalActor(actorID), "created", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "epic", EntityID: epic.ID, WorkspaceID: epic.WorkspaceID, ActorID: actorID})

	return s.epicRepo.GetWithStats(ctx, epic.ID)
}

// Update updates an epic.
func (s *PMEpicService) Update(ctx context.Context, id string, req model.UpdateEpicRequest, actorID string) (*model.EpicWithStats, error) {
	current, err := s.epicRepo.GetWithStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := current.Epic

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		epic.Name = name
	}
	if req.Description != nil {
		epic.Description = req.Description
	}
	if req.EpicStateID != nil {
		epic.EpicStateID = req.EpicStateID
	}
	if req.OwnerID != nil {
		epic.OwnerID = req.OwnerID
	}
	if req.TeamID != nil {
		epic.TeamID = req.TeamID
	}
	if req.PlannedStartDate != nil {
		epic.PlannedStartDate = req.PlannedStartDate
	}
	if req.Deadline != nil {
		epic.Deadline = req.Deadline
	}
	if req.Position != nil {
		epic.Position = *req.Position
	}
	if req.Color != nil {
		epic.Color = req.Color
	}
	if req.Archived != nil {
		epic.Archived = *req.Archived
	}
	if req.Health != nil {
		if !isValidEpicHealth(*req.Health) {
			return nil, fmt.Errorf("invalid health value")
		}
		epic.Health = *req.Health
	}
	if req.HealthComment != nil {
		epic.HealthComment = req.HealthComment
	}

	if err := s.epicRepo.Update(ctx, &epic); err != nil {
		return nil, err
	}
	if req.LabelIDs != nil {
		if err := s.epicRepo.ReplaceLabels(ctx, epic.ID, req.LabelIDs); err != nil {
			return nil, err
		}
	}

	if err := s.syncProgress(ctx, epic.ID); err != nil {
		return nil, err
	}
	_ = s.activityService.Log(ctx, epic.WorkspaceID, "epic", epic.ID, optionalActor(actorID), "updated", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "epic", EntityID: epic.ID, WorkspaceID: epic.WorkspaceID, ActorID: actorID})
	return s.epicRepo.GetWithStats(ctx, epic.ID)
}

// Delete archives an epic.
func (s *PMEpicService) Delete(ctx context.Context, id string, actorID string) error {
	epic, err := s.epicRepo.GetWithStats(ctx, id)
	if err != nil {
		return err
	}
	if epic == nil {
		return fmt.Errorf("epic not found")
	}
	if err := s.epicRepo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, epic.Epic.WorkspaceID, "epic", id, optionalActor(actorID), "archived", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "epic", EntityID: id, WorkspaceID: epic.Epic.WorkspaceID, ActorID: actorID})
	return nil
}

// UpdateHealth updates epic health fields.
func (s *PMEpicService) UpdateHealth(ctx context.Context, id string, req model.UpdateEpicHealthRequest, actorID string) error {
	if !isValidEpicHealth(req.Health) {
		return fmt.Errorf("invalid health value")
	}
	epic, err := s.epicRepo.GetWithStats(ctx, id)
	if err != nil {
		return err
	}
	if epic == nil {
		return fmt.Errorf("epic not found")
	}
	if err := s.epicRepo.UpdateHealth(ctx, id, req.Health, req.Comment); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, epic.Epic.WorkspaceID, "epic", id, optionalActor(actorID), "health_updated", stringPtr("health"), nil, &req.Health, nil)
	return nil
}

// AddLabel attaches a label to an epic.
func (s *PMEpicService) AddLabel(ctx context.Context, epicID, labelID, actorID string) error {
	epic, err := s.epicRepo.GetWithStats(ctx, epicID)
	if err != nil {
		return err
	}
	if epic == nil {
		return fmt.Errorf("epic not found")
	}
	if err := s.epicRepo.AddLabel(ctx, epicID, labelID); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, epic.Epic.WorkspaceID, "epic", epicID, optionalActor(actorID), "label_added", stringPtr("label"), nil, &labelID, nil)
	return nil
}

// RemoveLabel detaches a label from an epic.
func (s *PMEpicService) RemoveLabel(ctx context.Context, epicID, labelID, actorID string) error {
	epic, err := s.epicRepo.GetWithStats(ctx, epicID)
	if err != nil {
		return err
	}
	if epic == nil {
		return fmt.Errorf("epic not found")
	}
	if err := s.epicRepo.RemoveLabel(ctx, epicID, labelID); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, epic.Epic.WorkspaceID, "epic", epicID, optionalActor(actorID), "label_removed", stringPtr("label"), &labelID, nil, nil)
	return nil
}

// ListStories returns stories that belong to an epic.
func (s *PMEpicService) ListStories(ctx context.Context, epicID string) ([]model.PMStory, error) {
	return s.epicRepo.ListStories(ctx, epicID)
}

func (s *PMEpicService) syncProgress(ctx context.Context, epicID string) error {
	withStats, err := s.epicRepo.GetWithStats(ctx, epicID)
	if err != nil {
		return err
	}
	if withStats == nil {
		return fmt.Errorf("epic not found")
	}
	epic := withStats.Epic
	stats := withStats.Stats
	now := time.Now().UTC()

	started := false
	completed := false
	if stats.StoryCount > 0 {
		if stats.DoneStoryCount == stats.StoryCount {
			started = true
			completed = true
		} else if stats.DoneStoryCount > 0 || stats.InProgressCount > 0 {
			started = true
		}
	}

	epic.Started = started
	epic.Completed = completed
	if started {
		if epic.StartedAt == nil {
			epic.StartedAt = &now
		}
	} else {
		epic.StartedAt = nil
	}
	if completed {
		if epic.CompletedAt == nil {
			epic.CompletedAt = &now
		}
	} else {
		epic.CompletedAt = nil
	}

	if err := s.epicRepo.Update(ctx, &epic); err != nil {
		return err
	}
	return nil
}

func isValidEpicHealth(value string) bool {
	switch value {
	case model.PMEpicHealthOnTrack, model.PMEpicHealthAtRisk, model.PMEpicHealthOffTrack:
		return true
	default:
		return false
	}
}

func optionalActor(actorID string) *string {
	if actorID == "" {
		return nil
	}
	return &actorID
}

func stringPtr(value string) *string { return &value }
