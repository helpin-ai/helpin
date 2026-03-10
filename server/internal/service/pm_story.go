package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PMStoryService contains story business logic.
type PMStoryService struct {
	storyRepo           *repository.PMStoryRepository
	workspaceRepo       *repository.WorkspaceRepository
	workflowRepo        *repository.PMWorkflowRepository
	labelRepo           *repository.PMLabelRepository
	activityService     *PMActivityService
	wsPublisher         *websocket.Publisher
	automationService   *PMAutomationService
	notificationService *NotificationService
	followerService     *FollowerService
}

// NewPMStoryService creates a new PMStoryService.
func NewPMStoryService(storyRepo *repository.PMStoryRepository, workspaceRepo *repository.WorkspaceRepository, workflowRepo *repository.PMWorkflowRepository, labelRepo *repository.PMLabelRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher, automationService *PMAutomationService, notificationService *NotificationService, followerService *FollowerService) *PMStoryService {
	return &PMStoryService{
		storyRepo:           storyRepo,
		workspaceRepo:       workspaceRepo,
		workflowRepo:        workflowRepo,
		labelRepo:           labelRepo,
		activityService:     activityService,
		wsPublisher:         wsPublisher,
		automationService:   automationService,
		notificationService: notificationService,
		followerService:     followerService,
	}
}

// requireCanEdit checks that the actor has owner, admin, or manager role.
func (s *PMStoryService) requireCanEdit(ctx context.Context, workspaceID, actorID string) error {
	if workspaceID == "" || actorID == "" {
		return &model.ErrForbidden{Message: "workspace_id and user_id are required"}
	}
	role, err := s.workspaceRepo.GetMemberRole(ctx, workspaceID, actorID)
	if err != nil {
		return err
	}
	if role != model.RoleOwner && role != model.RoleAdmin && role != model.RoleManager {
		return &model.ErrForbidden{Message: "manager access or above required"}
	}
	return nil
}

// requireAdmin checks that the actor has owner or admin role.
func (s *PMStoryService) requireAdmin(ctx context.Context, workspaceID, actorID string) error {
	if workspaceID == "" || actorID == "" {
		return &model.ErrForbidden{Message: "workspace_id and user_id are required"}
	}
	role, err := s.workspaceRepo.GetMemberRole(ctx, workspaceID, actorID)
	if err != nil {
		return err
	}
	if role != model.RoleOwner && role != model.RoleAdmin {
		return &model.ErrForbidden{Message: "admin access or above required"}
	}
	return nil
}

// List returns stories with filters/pagination.
func (s *PMStoryService) List(ctx context.Context, workspaceID string, filters model.PMStoryFilters, pagination model.PMPagination) ([]model.BoardStory, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.storyRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns story detail.
func (s *PMStoryService) GetByID(ctx context.Context, id string) (*model.StoryDetail, error) {
	story, err := s.storyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if story == nil {
		return nil, fmt.Errorf("story not found")
	}
	return story, nil
}

// GetByDisplayID returns story detail by display ID.
func (s *PMStoryService) GetByDisplayID(ctx context.Context, workspaceID string, displayID int) (*model.StoryDetail, error) {
	story, err := s.storyRepo.GetByDisplayID(ctx, workspaceID, displayID)
	if err != nil {
		return nil, err
	}
	if story == nil {
		return nil, fmt.Errorf("story not found")
	}
	return story, nil
}

// Create creates a story.
func (s *PMStoryService) Create(ctx context.Context, req model.CreateStoryRequest, actorID string) (*model.StoryDetail, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if err := s.requireCanEdit(ctx, req.WorkspaceID, actorID); err != nil {
		return nil, err
	}

	workflowID := req.WorkflowID
	stateID := req.WorkflowStateID
	if workflowID == "" {
		defaultWorkflow, err := s.workflowRepo.GetDefaultWorkflow(ctx, req.WorkspaceID)
		if err != nil {
			return nil, err
		}
		if defaultWorkflow == nil {
			seeded, err := s.workflowRepo.SeedDefaultWorkflow(ctx, req.WorkspaceID)
			if err != nil {
				return nil, err
			}
			defaultWorkflow = seeded
		}
		workflowID = defaultWorkflow.Workflow.ID
		if stateID == "" {
			if defaultWorkflow.Workflow.DefaultStateID != nil {
				stateID = *defaultWorkflow.Workflow.DefaultStateID
			} else if len(defaultWorkflow.States) > 0 {
				stateID = defaultWorkflow.States[0].ID
			}
		}
	}
	if stateID == "" {
		return nil, fmt.Errorf("workflow_state_id is required")
	}

	ok, err := s.workflowRepo.StateBelongsToWorkflow(ctx, stateID, workflowID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("workflow_state_id must belong to workflow_id")
	}

	storyType := req.StoryType
	if storyType == "" {
		storyType = model.PMStoryTypeFeature
	}
	if !isValidStoryType(storyType) {
		return nil, fmt.Errorf("invalid story_type")
	}

	priority := model.PMStoryPriorityNone
	if req.Priority != nil && *req.Priority != "" {
		priority = *req.Priority
	}
	if !isValidStoryPriority(priority) {
		return nil, fmt.Errorf("invalid priority")
	}

	severity := model.PMStorySeverityNone
	if req.Severity != nil && *req.Severity != "" {
		severity = *req.Severity
	}
	if !isValidStorySeverity(severity) {
		return nil, fmt.Errorf("invalid severity")
	}

	ownerMember, err := resolveWorkspaceMember(ctx, s.workspaceRepo, req.WorkspaceID, req.OwnerMemberID, req.OwnerID)
	if err != nil {
		return nil, err
	}

	requesterMember, err := resolveWorkspaceMember(ctx, s.workspaceRepo, req.WorkspaceID, req.RequesterMemberID, req.RequesterID)
	if err != nil {
		return nil, err
	}
	if requesterMember == nil && actorID != "" {
		requesterMember, err = resolveWorkspaceMember(ctx, s.workspaceRepo, req.WorkspaceID, nil, &actorID)
		if err != nil {
			return nil, err
		}
	}

	blocked := false
	if req.Blocked != nil {
		blocked = *req.Blocked
	}
	if req.Blocker != nil && strings.TrimSpace(*req.Blocker) != "" {
		blocked = true
	}

	story := &model.PMStory{
		WorkspaceID:       req.WorkspaceID,
		Name:              strings.TrimSpace(req.Name),
		Description:       req.Description,
		StoryType:         storyType,
		WorkflowID:        workflowID,
		WorkflowStateID:   stateID,
		EpicID:            req.EpicID,
		SprintID:          req.SprintID,
		TeamID:            req.TeamID,
		OwnerID:           memberUserIDPtr(ownerMember),
		OwnerMemberID:     memberIDPtr(ownerMember),
		RequesterID:       memberUserIDPtr(requesterMember),
		RequesterMemberID: memberIDPtr(requesterMember),
		Estimate:          req.Estimate,
		Priority:          priority,
		Severity:          severity,
		Deadline:          req.Deadline,
		Blocked:           blocked,
		Blocker:           req.Blocker,
		TemplateID:        req.TemplateID,
		ExternalID:        req.ExternalID,
	}
	if req.Position != nil {
		story.Position = *req.Position
	}

	if err := s.storyRepo.Create(ctx, story); err != nil {
		return nil, err
	}

	ownerIDs := dedupeIDs(req.OwnerIDs)
	if story.OwnerID != nil {
		ownerIDs = append(ownerIDs, *story.OwnerID)
		ownerIDs = dedupeIDs(ownerIDs)
	}
	for _, ownerID := range ownerIDs {
		if err := s.storyRepo.AddOwner(ctx, story.ID, ownerID); err != nil {
			return nil, err
		}
	}

	followerIDs := dedupeIDs(req.FollowerIDs)
	if story.RequesterID != nil {
		followerIDs = append(followerIDs, *story.RequesterID)
	}
	for _, ownerID := range ownerIDs {
		followerIDs = append(followerIDs, ownerID)
	}
	followerIDs = dedupeIDs(followerIDs)
	for _, followerID := range followerIDs {
		if err := s.storyRepo.AddFollower(ctx, story.ID, followerID); err != nil {
			return nil, err
		}
	}

	labelIDs := dedupeIDs(req.LabelIDs)
	if err := validateLabelScope(ctx, s.labelRepo, req.WorkspaceID, labelIDs, allowedTeamIDs(req.TeamID)); err != nil {
		return nil, err
	}
	for _, labelID := range labelIDs {
		if err := s.storyRepo.AddLabel(ctx, story.ID, labelID); err != nil {
			return nil, err
		}
	}

	if err := s.storyRepo.UpdateStartedCompleted(ctx, story.ID); err != nil {
		return nil, err
	}

	if s.automationService != nil {
		s.automationService.OnStoryStateChange(ctx, story, story.WorkflowStateID)
	}

	createdAction := "created this story"
	if st, _ := s.workflowRepo.GetStateByID(ctx, story.WorkflowStateID); st != nil {
		createdAction = "created this story in " + st.Name
	}
	_ = s.activityService.Log(ctx, story.WorkspaceID, "story", story.ID, optionalActor(actorID), createdAction, nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "story", EntityID: story.ID, WorkspaceID: story.WorkspaceID, ActorID: actorID})

	// Auto-follow the creator and emit notification.
	if s.followerService != nil && actorID != "" {
		_ = s.followerService.Follow(ctx, actorID, "story", story.ID, story.WorkspaceID, "creator")
	}
	if s.notificationService != nil {
		// Check for @mentions in description.
		var mentionedUserIDs []string
		if story.Description != nil {
			mentions := extractMentions(*story.Description)
			slog.InfoContext(ctx, "story created with mentions",
				"story_id", story.ID,
				"workspace_id", story.WorkspaceID,
				"mentions", mentions,
			)
			if len(mentions) > 0 && s.workspaceRepo != nil {
				for _, handle := range mentions {
					uid, err := s.workspaceRepo.GetUserIDByHandle(ctx, story.WorkspaceID, handle)
					if err != nil {
						slog.ErrorContext(ctx, "failed to resolve mention in story", "handle", handle, "error", err)
						continue
					}
					if uid == "" {
						slog.WarnContext(ctx, "mention handle not found in story", "handle", handle, "workspace_id", story.WorkspaceID)
						continue
					}
					slog.InfoContext(ctx, "story mention resolved", "handle", handle, "user_id", uid)
					mentionedUserIDs = append(mentionedUserIDs, uid)
				}
			}
		}

		eventType := "story.created"
		category := "activity"
		priority := "normal"
		if len(mentionedUserIDs) > 0 {
			eventType = "story.mention"
			category = "mention"
			priority = "high"
		}

		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID:        story.WorkspaceID,
			ActorID:            actorID,
			EventType:          eventType,
			EntityType:         "story",
			EntityID:           story.ID,
			Title:              "created " + story.Name,
			Category:           category,
			Priority:           priority,
			TeamID:             derefString(story.TeamID),
			ExplicitRecipients: mentionedUserIDs,
			EntitySnapshot: model.JSONB{
				"title":      story.Name,
				"display_id": story.DisplayID,
				"type":       story.StoryType,
			},
		}); err != nil {
			slog.ErrorContext(ctx, "failed to emit story created notification", "error", err, "story_id", story.ID)
		}
	}

	return s.storyRepo.GetByID(ctx, story.ID)
}

// Update updates story fields.
func (s *PMStoryService) Update(ctx context.Context, id string, req model.UpdateStoryRequest, actorID string) (*model.StoryDetail, error) {
	current, err := s.storyRepo.GetRawByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return nil, err
	}

	stateChanged := false
	oldPriority := current.Priority
	oldSeverity := current.Severity
	oldStoryType := current.StoryType
	oldBlocked := current.Blocked

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		current.Name = name
	}
	if req.Description != nil {
		current.Description = req.Description
	}
	if req.StoryType != nil {
		if !isValidStoryType(*req.StoryType) {
			return nil, fmt.Errorf("invalid story_type")
		}
		current.StoryType = *req.StoryType
	}

	workflowID := current.WorkflowID
	if req.WorkflowID != nil {
		workflowID = *req.WorkflowID
	}
	stateID := current.WorkflowStateID
	if req.WorkflowStateID != nil {
		stateID = *req.WorkflowStateID
	}
	if workflowID != current.WorkflowID || stateID != current.WorkflowStateID {
		stateChanged = true
	}

	ok, err := s.workflowRepo.StateBelongsToWorkflow(ctx, stateID, workflowID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("workflow_state_id must belong to workflow_id")
	}
	current.WorkflowID = workflowID
	current.WorkflowStateID = stateID

	if req.EpicID != nil {
		current.EpicID = req.EpicID
	}
	if req.SprintID != nil {
		current.SprintID = req.SprintID
	}
	if req.TeamID != nil {
		current.TeamID = req.TeamID
	}
	if req.OwnerID != nil {
		// Handled below via workspace member resolution.
	}
	if req.RequesterID != nil {
		// Handled below via workspace member resolution.
	}
	if req.Estimate != nil {
		current.Estimate = req.Estimate
	}
	if req.Priority != nil {
		if !isValidStoryPriority(*req.Priority) {
			return nil, fmt.Errorf("invalid priority")
		}
		current.Priority = *req.Priority
	}
	if req.Severity != nil {
		if !isValidStorySeverity(*req.Severity) {
			return nil, fmt.Errorf("invalid severity")
		}
		current.Severity = *req.Severity
	}
	if req.Deadline != nil {
		current.Deadline = req.Deadline
	}
	if req.Position != nil {
		current.Position = *req.Position
	}
	if req.Blocked != nil {
		current.Blocked = *req.Blocked
	}
	if req.Blocker != nil {
		current.Blocker = req.Blocker
		current.Blocked = strings.TrimSpace(*req.Blocker) != ""
	}
	if req.Archived != nil {
		current.Archived = *req.Archived
	}
	if req.TemplateID != nil {
		current.TemplateID = req.TemplateID
	}
	if req.ExternalID != nil {
		current.ExternalID = req.ExternalID
	}

	if req.OwnerID != nil || req.OwnerMemberID != nil {
		ownerMember, err := resolveWorkspaceMember(ctx, s.workspaceRepo, current.WorkspaceID, req.OwnerMemberID, req.OwnerID)
		if err != nil {
			return nil, err
		}
		current.OwnerMemberID = memberIDPtr(ownerMember)
		current.OwnerID = memberUserIDPtr(ownerMember)
	}
	if req.RequesterID != nil || req.RequesterMemberID != nil {
		requesterMember, err := resolveWorkspaceMember(ctx, s.workspaceRepo, current.WorkspaceID, req.RequesterMemberID, req.RequesterID)
		if err != nil {
			return nil, err
		}
		current.RequesterMemberID = memberIDPtr(requesterMember)
		current.RequesterID = memberUserIDPtr(requesterMember)
	}

	if stateChanged {
		now := time.Now().UTC()
		current.MovedAt = &now
	}

	if err := s.storyRepo.Update(ctx, current); err != nil {
		return nil, err
	}

	if req.OwnerIDs != nil {
		owners := dedupeIDs(req.OwnerIDs)
		if current.OwnerID != nil {
			owners = append(owners, *current.OwnerID)
			owners = dedupeIDs(owners)
		}
		if err := s.storyRepo.ReplaceOwners(ctx, current.ID, owners); err != nil {
			return nil, err
		}
		if req.FollowerIDs == nil {
			// Auto-follow owners if explicit follower list was not provided.
			for _, ownerID := range owners {
				if err := s.storyRepo.AddFollower(ctx, current.ID, ownerID); err != nil {
					return nil, err
				}
			}
		}
	}
	if req.FollowerIDs != nil {
		followers := dedupeIDs(req.FollowerIDs)
		if current.OwnerID != nil {
			followers = append(followers, *current.OwnerID)
		}
		if current.RequesterID != nil {
			followers = append(followers, *current.RequesterID)
		}
		followers = dedupeIDs(followers)
		if err := s.storyRepo.ReplaceFollowers(ctx, current.ID, followers); err != nil {
			return nil, err
		}
	}
	if req.LabelIDs != nil {
		labelIDs := dedupeIDs(req.LabelIDs)
		if err := validateLabelScope(ctx, s.labelRepo, current.WorkspaceID, labelIDs, allowedTeamIDs(current.TeamID)); err != nil {
			return nil, err
		}
		if err := s.storyRepo.ReplaceLabels(ctx, current.ID, labelIDs); err != nil {
			return nil, err
		}
	}

	if stateChanged {
		if err := s.storyRepo.UpdateStartedCompleted(ctx, current.ID); err != nil {
			return nil, err
		}
		if s.automationService != nil {
			s.automationService.OnStoryStateChange(ctx, current, current.WorkflowStateID)
		}
	}

	// Only log meaningful field changes with descriptive messages
	if stateChanged {
		newName := current.WorkflowStateID
		if st, _ := s.workflowRepo.GetStateByID(ctx, current.WorkflowStateID); st != nil {
			newName = st.Name
		}
		action := "moved this story to " + newName
		_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil)
	}
	if req.Priority != nil && *req.Priority != oldPriority {
		action := "changed priority from " + oldPriority + " to " + *req.Priority
		_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil)
	}
	if req.Severity != nil && *req.Severity != oldSeverity {
		action := "changed severity from " + oldSeverity + " to " + *req.Severity
		_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil)
	}
	if req.StoryType != nil && *req.StoryType != oldStoryType {
		action := "changed type from " + oldStoryType + " to " + *req.StoryType
		_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil)
	}
	if req.Blocked != nil && *req.Blocked != oldBlocked {
		if *req.Blocked {
			_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "marked this story as blocked", nil, nil, nil, nil)
		} else {
			_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "unblocked this story", nil, nil, nil, nil)
		}
	}
	if req.Archived != nil && *req.Archived {
		_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "archived this story", nil, nil, nil, nil)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: current.ID, WorkspaceID: current.WorkspaceID, ActorID: actorID})

	// Emit notification for significant updates.
	if s.notificationService != nil && (stateChanged || (req.Priority != nil && *req.Priority != oldPriority) || (req.Blocked != nil && *req.Blocked != oldBlocked)) {
		eventType := "story.updated"
		title := "Story updated: " + current.Name
		category := "activity"
		priority := "normal"
		if stateChanged {
			eventType = "story.status_changed"
			title = "Story moved: " + current.Name
			category = "status_change"
		}
		if req.Priority != nil && *req.Priority == "urgent" {
			priority = "high"
		}
		if req.Blocked != nil && *req.Blocked && !oldBlocked {
			eventType = "story.blocked"
			title = "Story blocked: " + current.Name
			category = "status_change"
			priority = "high"
		}
		_ = s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: current.WorkspaceID,
			ActorID:     actorID,
			EventType:   eventType,
			EntityType:  "story",
			EntityID:    current.ID,
			Title:       title,
			Category:    category,
			Priority:    priority,
			TeamID:      derefString(current.TeamID),
			EntitySnapshot: model.JSONB{
				"title":      current.Name,
				"display_id": current.DisplayID,
				"type":       current.StoryType,
			},
		})
	}

	// Emit mention notification when description is updated with @mentions.
	if s.notificationService != nil && req.Description != nil {
		mentions := extractMentions(*req.Description)
		slog.InfoContext(ctx, "story updated with mentions",
			"story_id", current.ID,
			"workspace_id", current.WorkspaceID,
			"mentions", mentions,
		)
		if len(mentions) > 0 && s.workspaceRepo != nil {
			var mentionedUserIDs []string
			for _, handle := range mentions {
				uid, err := s.workspaceRepo.GetUserIDByHandle(ctx, current.WorkspaceID, handle)
				if err != nil {
					slog.ErrorContext(ctx, "failed to resolve mention in story update", "handle", handle, "error", err)
					continue
				}
				if uid == "" {
					slog.WarnContext(ctx, "mention handle not found in story update", "handle", handle, "workspace_id", current.WorkspaceID)
					continue
				}
				slog.InfoContext(ctx, "story update mention resolved", "handle", handle, "user_id", uid)
				mentionedUserIDs = append(mentionedUserIDs, uid)
			}
			if len(mentionedUserIDs) > 0 {
				if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
					WorkspaceID:        current.WorkspaceID,
					ActorID:            actorID,
					EventType:          "story.mention",
					EntityType:         "story",
					EntityID:           current.ID,
					Title:              "mentioned you in " + current.Name,
					Category:           "mention",
					Priority:           "high",
					TeamID:             derefString(current.TeamID),
					ExplicitRecipients: mentionedUserIDs,
					EntitySnapshot: model.JSONB{
						"title":      current.Name,
						"display_id": current.DisplayID,
						"type":       current.StoryType,
					},
				}); err != nil {
					slog.ErrorContext(ctx, "failed to emit story mention notification", "error", err, "story_id", current.ID)
				}
			}
		}
	}

	return s.storyRepo.GetByID(ctx, current.ID)
}

// Delete archives a story.
func (s *PMStoryService) Delete(ctx context.Context, id, actorID string) error {
	current, err := s.storyRepo.GetRawByID(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireAdmin(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.storyRepo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "archived this story", nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "story", EntityID: id, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// MoveToState moves story to another state.
func (s *PMStoryService) MoveToState(ctx context.Context, id string, req model.MoveStoryRequest, actorID string) (*model.StoryDetail, error) {
	current, err := s.storyRepo.GetRawByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return nil, err
	}
	if req.StateID == "" {
		return nil, fmt.Errorf("state_id is required")
	}
	ok, err := s.workflowRepo.StateBelongsToWorkflow(ctx, req.StateID, current.WorkflowID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("state_id must belong to story workflow")
	}

	position := current.Position
	if req.Position != nil {
		position = *req.Position
	}
	if err := s.storyRepo.MoveToState(ctx, current.ID, req.StateID, position); err != nil {
		return nil, err
	}
	if err := s.storyRepo.UpdateStartedCompleted(ctx, current.ID); err != nil {
		return nil, err
	}
	if s.automationService != nil {
		s.automationService.OnStoryStateChange(ctx, current, req.StateID)
	}
	newStateName := req.StateID
	if st, _ := s.workflowRepo.GetStateByID(ctx, req.StateID); st != nil {
		newStateName = st.Name
	}
	action := "moved this story to " + newStateName
	_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "moved", Entity: "story", EntityID: current.ID, WorkspaceID: current.WorkspaceID, ActorID: actorID})

	if s.notificationService != nil {
		_ = s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: current.WorkspaceID,
			ActorID:     actorID,
			EventType:   "story.status_changed",
			EntityType:  "story",
			EntityID:    current.ID,
			Title:       "moved " + current.Name + " to " + newStateName,
			Category:    "status_change",
			Priority:    "normal",
			TeamID:      derefString(current.TeamID),
			EntitySnapshot: model.JSONB{
				"title":      current.Name,
				"display_id": current.DisplayID,
				"type":       current.StoryType,
			},
		})
	}

	return s.storyRepo.GetByID(ctx, current.ID)
}

// Reorder changes story position in its state.
func (s *PMStoryService) Reorder(ctx context.Context, id string, req model.ReorderStoryRequest, actorID string) error {
	current, err := s.storyRepo.GetRawByID(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if req.Position < 0 {
		return fmt.Errorf("position must be >= 0")
	}
	if err := s.storyRepo.Reorder(ctx, id, req.Position); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "reordered", stringPtr("position"), nil, nil, map[string]interface{}{"position": req.Position})
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: id, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// AddOwner adds an owner and auto-follows them.
func (s *PMStoryService) AddOwner(ctx context.Context, storyID, userID, actorID string) error {
	current, err := s.storyRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}
	if err := s.storyRepo.AddOwner(ctx, storyID, userID); err != nil {
		return err
	}
	if err := s.storyRepo.AddFollower(ctx, storyID, userID); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "owner_added", stringPtr("owner"), nil, &userID, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})

	// Auto-follow and notify the assigned user.
	if s.followerService != nil {
		_ = s.followerService.Follow(ctx, userID, "story", storyID, current.WorkspaceID, "assigned")
	}
	if s.notificationService != nil {
		_ = s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID:        current.WorkspaceID,
			ActorID:            actorID,
			EventType:          "story.assigned",
			EntityType:         "story",
			EntityID:           storyID,
			Title:              "assigned you to " + current.Name,
			Category:           "assignment",
			Priority:           "normal",
			TeamID:             derefString(current.TeamID),
			ExplicitRecipients: []string{userID},
			EntitySnapshot: model.JSONB{
				"title":      current.Name,
				"display_id": current.DisplayID,
				"type":       current.StoryType,
			},
		})
	}

	return nil
}

// RemoveOwner removes an owner.
func (s *PMStoryService) RemoveOwner(ctx context.Context, storyID, userID, actorID string) error {
	current, err := s.storyRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.storyRepo.RemoveOwner(ctx, storyID, userID); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "owner_removed", stringPtr("owner"), &userID, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// AddFollower adds a follower.
func (s *PMStoryService) AddFollower(ctx context.Context, storyID, userID, actorID string) error {
	current, err := s.storyRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}
	if err := s.storyRepo.AddFollower(ctx, storyID, userID); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "follower_added", stringPtr("follower"), nil, &userID, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// RemoveFollower removes a follower.
func (s *PMStoryService) RemoveFollower(ctx context.Context, storyID, userID, actorID string) error {
	current, err := s.storyRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.storyRepo.RemoveFollower(ctx, storyID, userID); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "follower_removed", stringPtr("follower"), &userID, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// AddLabel adds a label to a story.
func (s *PMStoryService) AddLabel(ctx context.Context, storyID, labelID, actorID string) error {
	current, err := s.storyRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := validateLabelScope(ctx, s.labelRepo, current.WorkspaceID, []string{labelID}, allowedTeamIDs(current.TeamID)); err != nil {
		return err
	}
	if err := s.storyRepo.AddLabel(ctx, storyID, labelID); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "label_added", stringPtr("label"), nil, &labelID, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// RemoveLabel removes a label from a story.
func (s *PMStoryService) RemoveLabel(ctx context.Context, storyID, labelID, actorID string) error {
	current, err := s.storyRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.storyRepo.RemoveLabel(ctx, storyID, labelID); err != nil {
		return err
	}
	_ = s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "label_removed", stringPtr("label"), &labelID, nil, nil)
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// ListByWorkflowState returns board columns for a workflow with optional filters.
// perStateLimit controls how many stories per column (0 = unlimited).
func (s *PMStoryService) ListByWorkflowState(ctx context.Context, workflowID string, filters model.PMStoryFilters, perStateLimit int) ([]model.StoryStateColumn, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}
	return s.storyRepo.ListByWorkflowState(ctx, workflowID, filters, perStateLimit)
}

// ListColumnStories returns a page of stories for a single board column.
func (s *PMStoryService) ListColumnStories(ctx context.Context, stateID string, filters model.PMStoryFilters, offset, limit int) ([]model.BoardStory, []model.StoryGroup, int, error) {
	if stateID == "" {
		return nil, nil, 0, fmt.Errorf("state_id is required")
	}
	if limit <= 0 {
		limit = 50
	}
	return s.storyRepo.ListColumnStories(ctx, stateID, filters, offset, limit)
}

// CountByState returns state-level story counts for a workflow.
func (s *PMStoryService) CountByState(ctx context.Context, workflowID string) ([]model.StoryStateCount, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}
	return s.storyRepo.CountByState(ctx, workflowID)
}

// ListActivity returns story activity entries.
func (s *PMStoryService) ListActivity(ctx context.Context, storyID string, pagination model.PMPagination) ([]model.ActivityLogEntry, int64, error) {
	return s.activityService.ListEntity(ctx, "story", storyID, pagination)
}

func isValidStoryType(value string) bool {
	switch value {
	case model.PMStoryTypeFeature, model.PMStoryTypeBug, model.PMStoryTypeChore:
		return true
	default:
		return false
	}
}

func isValidStoryPriority(value string) bool {
	switch value {
	case model.PMStoryPriorityNone, model.PMStoryPriorityLow, model.PMStoryPriorityMedium, model.PMStoryPriorityHigh, model.PMStoryPriorityUrgent:
		return true
	default:
		return false
	}
}

func isValidStorySeverity(value string) bool {
	switch value {
	case model.PMStorySeverityNone, model.PMStorySeverityMinor, model.PMStorySeverityMajor, model.PMStorySeverityCritical:
		return true
	default:
		return false
	}
}

func dedupeIDs(ids []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
