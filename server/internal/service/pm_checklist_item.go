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
	taskRepo            *repository.PMTaskRepository
	wsPublisher         *websocket.Publisher
	notificationService *NotificationService
	workspaceRepo       *repository.WorkspaceRepository
}

// NewPMChecklistItemService creates a new PMChecklistItemService.
func NewPMChecklistItemService(
	repo *repository.PMChecklistItemRepository,
	taskRepo *repository.PMTaskRepository,
	wsPublisher *websocket.Publisher,
	notificationService *NotificationService,
	workspaceRepo *repository.WorkspaceRepository,
) *PMChecklistItemService {
	return &PMChecklistItemService{
		repo:                repo,
		taskRepo:            taskRepo,
		wsPublisher:         wsPublisher,
		notificationService: notificationService,
		workspaceRepo:       workspaceRepo,
	}
}

func (s *PMChecklistItemService) requireAccessibleTask(ctx context.Context, taskID, workspaceID string) (*model.PMTask, error) {
	if s.taskRepo == nil {
		return nil, fmt.Errorf("task repository is not configured")
	}
	task, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil || task.WorkspaceID != workspaceID {
		return nil, errCommandNotFound("task")
	}
	if err := requireTeamAccess(ctx, task.TeamID); err != nil {
		return nil, errCommandNotFound("task")
	}
	return task, nil
}

func (s *PMChecklistItemService) validateAssignee(ctx context.Context, workspaceID string, assigneeID *string, allowEmpty bool) (*string, error) {
	if assigneeID == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*assigneeID)
	if trimmed == "" && allowEmpty {
		return nil, nil
	}
	if trimmed == "" {
		return nil, errCommandInput("assignee_id must be an active workspace member")
	}
	if s.workspaceRepo == nil {
		return nil, fmt.Errorf("workspace repository is not configured")
	}
	membership, err := s.workspaceRepo.GetMembership(ctx, workspaceID, trimmed)
	if err != nil {
		return nil, err
	}
	if membership == nil {
		return nil, errCommandInput("assignee_id must be an active workspace member")
	}
	return &trimmed, nil
}

// List returns checklist items for an accessible task in the workspace.
func (s *PMChecklistItemService) List(ctx context.Context, taskID, workspaceID string) ([]model.PMChecklistItem, error) {
	if taskID == "" {
		return nil, errCommandInput("task_id is required")
	}
	if _, err := s.requireAccessibleTask(ctx, taskID, workspaceID); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, taskID)
}

// ListBounded returns at most 100 checklist items plus full count metadata.
func (s *PMChecklistItemService) ListBounded(ctx context.Context, taskID, workspaceID string, limit int) ([]model.PMChecklistItem, int64, bool, error) {
	if taskID == "" {
		return nil, 0, false, errCommandInput("task_id is required")
	}
	if _, err := s.requireAccessibleTask(ctx, taskID, workspaceID); err != nil {
		return nil, 0, false, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		return nil, 0, false, errCommandInput("limit must be between 1 and 100")
	}
	items, total, err := s.repo.ListPage(ctx, taskID, limit, 0)
	if err != nil {
		return nil, 0, false, err
	}
	return items, total, int64(len(items)) < total, nil
}

// Get returns one checklist item after validating its parent task boundary.
func (s *PMChecklistItemService) Get(ctx context.Context, id, workspaceID string) (*model.PMChecklistItem, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil || item == nil {
		return item, err
	}
	if _, err := s.requireAccessibleTask(ctx, item.TaskID, workspaceID); err != nil {
		return nil, err
	}
	return item, nil
}

// Create creates a checklist item.
func (s *PMChecklistItemService) Create(ctx context.Context, taskID string, req model.CreateChecklistItemRequest, workspaceID, actorID string) (*model.PMChecklistItem, error) {
	if taskID == "" {
		return nil, errCommandInput("task_id is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return nil, errCommandInput("text is required")
	}
	if _, err := s.requireAccessibleTask(ctx, taskID, workspaceID); err != nil {
		return nil, err
	}
	assigneeID, err := s.validateAssignee(ctx, workspaceID, req.AssigneeID, false)
	if err != nil {
		return nil, err
	}

	item := &model.PMChecklistItem{
		TaskID:     taskID,
		Text:       strings.TrimSpace(req.Text),
		AssigneeID: assigneeID,
		DueDate:    req.DueDate,
	}
	if req.Position != nil {
		item.Position = *req.Position
	}

	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "checklist_item", EntityID: item.ID, WorkspaceID: workspaceID, ActorID: actorID, ParentType: "task", ParentID: taskID})

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
		return nil, errCommandNotFound("checklist item")
	}
	if _, err := s.requireAccessibleTask(ctx, item.TaskID, workspaceID); err != nil {
		return nil, err
	}
	assigneeID, err := s.validateAssignee(ctx, workspaceID, req.AssigneeID, true)
	if err != nil {
		return nil, err
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
		item.AssigneeID = assigneeID
	}
	if req.DueDate != nil || req.DueDateSet {
		item.DueDate = req.DueDate
	}

	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "checklist_item", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: "task", ParentID: item.TaskID})

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
		return errCommandNotFound("checklist item")
	}
	if _, err := s.requireAccessibleTask(ctx, item.TaskID, workspaceID); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "checklist_item", EntityID: id, WorkspaceID: workspaceID, ActorID: actorID, ParentType: "task", ParentID: item.TaskID})
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

	entityTitle := item.TaskID
	var entityTeamID string
	readableTeamIDs := []string(nil)
	if s.taskRepo != nil {
		if task, _ := s.taskRepo.GetRawByID(ctx, item.TaskID); task != nil {
			entityTitle = task.Name
			entityTeamID = derefString(task.TeamID)
			readableTeamIDs = mentionScopeForTeamID(task.TeamID)
		}
	}

	mentionedUserIDs, err := emitMentionNotification(ctx, s.notificationService, s.workspaceRepo, pmMentionNotificationInput{
		WorkspaceID:      workspaceID,
		ActorID:          actorID,
		Body:             item.Text,
		EventType:        "checklist.mention",
		EntityType:       "task",
		EntityID:         item.TaskID,
		Title:            "mentioned you in a checklist item on " + entityTitle,
		TeamID:           entityTeamID,
		ReadableTeamIDs:  readableTeamIDs,
		EntitySnapshot:   model.JSONB{"title": entityTitle, "checklist_item": item.Text},
		NotificationBody: truncate(item.Text, 200),
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to emit checklist mention notification", "error", err, "task_id", item.TaskID)
		return
	}
	if len(mentionedUserIDs) == 0 {
		return
	}

	slog.InfoContext(ctx, "emitting checklist mention notification",
		"checklist_item_id", item.ID,
		"task_id", item.TaskID,
		"mentioned_user_ids", mentionedUserIDs,
	)
}
