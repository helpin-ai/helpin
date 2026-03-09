package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PMChecklistItemService contains checklist item business logic.
type PMChecklistItemService struct {
	repo                *repository.PMChecklistItemRepository
	storyRepo           *repository.PMStoryRepository
	wsPublisher         *websocket.Publisher
	notificationService *NotificationService
	workspaceRepo       *repository.WorkspaceRepository
}

// NewPMChecklistItemService creates a new PMChecklistItemService.
func NewPMChecklistItemService(
	repo *repository.PMChecklistItemRepository,
	storyRepo *repository.PMStoryRepository,
	wsPublisher *websocket.Publisher,
	notificationService *NotificationService,
	workspaceRepo *repository.WorkspaceRepository,
) *PMChecklistItemService {
	return &PMChecklistItemService{
		repo:                repo,
		storyRepo:           storyRepo,
		wsPublisher:         wsPublisher,
		notificationService: notificationService,
		workspaceRepo:       workspaceRepo,
	}
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
		StoryID:    storyID,
		Text:       strings.TrimSpace(req.Text),
		AssigneeID: req.AssigneeID,
	}
	if req.Position != nil {
		item.Position = *req.Position
	}

	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "checklist_item", EntityID: item.ID, WorkspaceID: workspaceID, ActorID: actorID, ParentType: "story", ParentID: storyID})

	// Emit notifications for @mentions in checklist item text.
	s.emitMentionNotifications(ctx, item, workspaceID, actorID)

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

	textChanged := false
	if req.Text != nil {
		if strings.TrimSpace(*req.Text) == "" {
			return nil, fmt.Errorf("text cannot be empty")
		}
		if item.Text != strings.TrimSpace(*req.Text) {
			textChanged = true
		}
		item.Text = strings.TrimSpace(*req.Text)
	}
	if req.Completed != nil {
		item.Completed = *req.Completed
	}
	if req.Position != nil {
		item.Position = *req.Position
	}
	if req.AssigneeID != nil {
		item.AssigneeID = req.AssigneeID
	}

	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "checklist_item", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: "story", ParentID: item.StoryID})

	// Emit notifications for new @mentions when text changes.
	if textChanged {
		s.emitMentionNotifications(ctx, item, workspaceID, actorID)
	}

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

// emitMentionNotifications extracts @mentions from checklist item text and sends notifications.
func (s *PMChecklistItemService) emitMentionNotifications(ctx context.Context, item *model.PMChecklistItem, workspaceID, actorID string) {
	if s.notificationService == nil {
		return
	}

	mentions := extractMentions(item.Text)
	if len(mentions) == 0 {
		return
	}

	// Resolve handles to user IDs.
	var mentionedUserIDs []string
	if s.workspaceRepo != nil {
		for _, handle := range mentions {
			uid, err := s.workspaceRepo.GetUserIDByHandle(ctx, workspaceID, handle)
			if err != nil {
				slog.ErrorContext(ctx, "failed to resolve checklist mention handle", "handle", handle, "error", err)
				continue
			}
			if uid == "" {
				continue
			}
			mentionedUserIDs = append(mentionedUserIDs, uid)
		}
	}

	if len(mentionedUserIDs) == 0 {
		return
	}

	entityTitle := item.StoryID
	if s.storyRepo != nil {
		if story, _ := s.storyRepo.GetRawByID(ctx, item.StoryID); story != nil {
			entityTitle = story.Name
		}
	}

	slog.InfoContext(ctx, "emitting checklist mention notification",
		"checklist_item_id", item.ID,
		"story_id", item.StoryID,
		"mentioned_user_ids", mentionedUserIDs,
	)

	if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:        workspaceID,
		ActorID:            actorID,
		EventType:          "checklist.mention",
		EntityType:         "story",
		EntityID:           item.StoryID,
		Title:              "mentioned you in a checklist item on " + entityTitle,
		Body:               truncate(item.Text, 200),
		Category:           "mention",
		Priority:           "high",
		ExplicitRecipients: mentionedUserIDs,
		EntitySnapshot: model.JSONB{
			"title":           entityTitle,
			"checklist_item":  item.Text,
		},
	}); err != nil {
		slog.ErrorContext(ctx, "failed to emit checklist mention notification", "error", err, "story_id", item.StoryID)
	}
}
