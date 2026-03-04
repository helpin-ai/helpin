package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
	"github.com/d4interactive/teampulse/server/internal/websocket"
)

// PMChecklistItemService contains checklist item business logic.
type PMChecklistItemService struct {
	repo        *repository.PMChecklistItemRepository
	wsPublisher *websocket.Publisher
}

// NewPMChecklistItemService creates a new PMChecklistItemService.
func NewPMChecklistItemService(repo *repository.PMChecklistItemRepository, wsPublisher *websocket.Publisher) *PMChecklistItemService {
	return &PMChecklistItemService{repo: repo, wsPublisher: wsPublisher}
}

// List returns checklist items for a story.
func (s *PMChecklistItemService) List(ctx context.Context, storyID string) ([]model.PMChecklistItem, error) {
	if storyID == "" {
		return nil, fmt.Errorf("story_id is required")
	}
	return s.repo.List(ctx, storyID)
}

// Create creates a checklist item.
func (s *PMChecklistItemService) Create(ctx context.Context, storyID string, req model.CreateChecklistItemRequest, workspaceID, actorID string) (*model.PMChecklistItem, error) {
	if storyID == "" {
		return nil, fmt.Errorf("story_id is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return nil, fmt.Errorf("text is required")
	}

	item := &model.PMChecklistItem{
		StoryID: storyID,
		Text:    strings.TrimSpace(req.Text),
	}
	if req.Position != nil {
		item.Position = *req.Position
	}

	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "checklist_item", EntityID: item.ID, WorkspaceID: workspaceID, ActorID: actorID, ParentType: "story", ParentID: storyID})
	return item, nil
}

// Update updates a checklist item.
func (s *PMChecklistItemService) Update(ctx context.Context, id string, req model.UpdateChecklistItemRequest, workspaceID, actorID string) (*model.PMChecklistItem, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("checklist item not found")
	}

	if req.Text != nil {
		if strings.TrimSpace(*req.Text) == "" {
			return nil, fmt.Errorf("text cannot be empty")
		}
		item.Text = strings.TrimSpace(*req.Text)
	}
	if req.Completed != nil {
		item.Completed = *req.Completed
	}
	if req.Position != nil {
		item.Position = *req.Position
	}

	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "checklist_item", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: "story", ParentID: item.StoryID})
	return item, nil
}

// Delete deletes a checklist item.
func (s *PMChecklistItemService) Delete(ctx context.Context, id string, workspaceID, actorID string) error {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("checklist item not found")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "checklist_item", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: "story", ParentID: item.StoryID})
	return nil
}
