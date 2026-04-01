package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
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
	epicRepo            *repository.PMEpicRepository
	sprintRepo          *repository.PMSprintRepository
	labelRepo           *repository.PMLabelRepository
	checklistRepo       *repository.PMChecklistItemRepository
	externalLinkRepo    *repository.PMExternalLinkRepository
	attachmentRepo      *repository.PMAttachmentRepository
	activityService     *PMActivityService
	wsPublisher         *websocket.Publisher
	automationService   *PMAutomationService
	notificationService *NotificationService
	followerService     *FollowerService
	ruleEngine          *AutomationRuleEngine
	agentService        *AgentService
	recurringService    *PMRecurringTemplateService
	logger              *slog.Logger
}

// NewPMStoryService creates a new PMStoryService.
func NewPMStoryService(storyRepo *repository.PMStoryRepository, workspaceRepo *repository.WorkspaceRepository, workflowRepo *repository.PMWorkflowRepository, epicRepo *repository.PMEpicRepository, sprintRepo *repository.PMSprintRepository, labelRepo *repository.PMLabelRepository, checklistRepo *repository.PMChecklistItemRepository, externalLinkRepo *repository.PMExternalLinkRepository, attachmentRepo *repository.PMAttachmentRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher, automationService *PMAutomationService, notificationService *NotificationService, followerService *FollowerService) *PMStoryService {
	return &PMStoryService{
		storyRepo:           storyRepo,
		workspaceRepo:       workspaceRepo,
		workflowRepo:        workflowRepo,
		epicRepo:            epicRepo,
		sprintRepo:          sprintRepo,
		labelRepo:           labelRepo,
		checklistRepo:       checklistRepo,
		externalLinkRepo:    externalLinkRepo,
		attachmentRepo:      attachmentRepo,
		activityService:     activityService,
		wsPublisher:         wsPublisher,
		automationService:   automationService,
		notificationService: notificationService,
		followerService:     followerService,
		logger:              slog.Default().With("service", "pm_story"),
	}
}

// SetRuleEngine sets the automation rule engine (breaks circular dependency).
func (s *PMStoryService) SetRuleEngine(engine *AutomationRuleEngine) *PMStoryService {
	s.ruleEngine = engine
	return s
}

// SetAgentService sets the agent service (breaks circular dependency).
func (s *PMStoryService) SetAgentService(svc *AgentService) {
	s.agentService = svc
}

// SetRecurringService sets the recurring template service (breaks circular dependency).
func (s *PMStoryService) SetRecurringService(svc *PMRecurringTemplateService) {
	s.recurringService = svc
}

func pmDnDWebsocketData(traceID string) json.RawMessage {
	if strings.TrimSpace(traceID) == "" {
		return nil
	}
	payload, err := json.Marshal(map[string]string{
		"debug_trace_id": traceID,
	})
	if err != nil {
		return nil
	}
	return payload
}

// requireCanEdit checks that the actor has at least member role (owner, admin, or member).
func (s *PMStoryService) requireCanEdit(ctx context.Context, workspaceID, actorID string) error {
	if workspaceID == "" || actorID == "" {
		return &model.ErrForbidden{Message: "workspace_id and user_id are required"}
	}
	role, err := s.workspaceRepo.GetMemberRole(ctx, workspaceID, actorID)
	if err != nil {
		return err
	}
	if role == model.RoleViewer {
		return &model.ErrForbidden{Message: "edit access required"}
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
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
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
	if err := requireTeamAccess(ctx, story.Story.TeamID); err != nil {
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
	if err := requireTeamAccess(ctx, story.Story.TeamID); err != nil {
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
	if err := requireTeamMembershipForCreate(ctx, req.TeamID); err != nil {
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
		if err != nil && !isIgnorableAutoRequesterResolutionError(err) {
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
	if err := validateEpicScope(ctx, s.epicRepo, req.WorkspaceID, story.EpicID, story.TeamID); err != nil {
		return nil, err
	}
	if err := validateSprintScope(ctx, s.sprintRepo, req.WorkspaceID, story.SprintID, story.TeamID); err != nil {
		return nil, err
	}
	if req.Position != nil {
		story.Position = *req.Position
	} else {
		position, err := s.storyRepo.NextPosition(ctx, req.WorkspaceID, stateID)
		if err != nil {
			return nil, err
		}
		story.Position = position
	}

	if err := s.storyRepo.Create(ctx, story); err != nil {
		return nil, err
	}
	if len(req.AttachmentIDs) > 0 && s.attachmentRepo != nil {
		if err := s.attachmentRepo.ReassignToEntity(ctx, req.AttachmentIDs, "story", story.ID); err != nil {
			s.logger.ErrorContext(ctx, "failed to reassign attachments to story", "error", err, "story_id", story.ID, "attachment_ids", req.AttachmentIDs)
		}
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

	// Create checklist items from template.
	if s.checklistRepo != nil && len(req.ChecklistItems) > 0 {
		for i, ci := range req.ChecklistItems {
			item := &model.PMChecklistItem{
				StoryID:    story.ID,
				Text:       strings.TrimSpace(ci.Text),
				Position:   i,
				AssigneeID: ci.AssigneeID,
			}
			if ci.Position != nil {
				item.Position = *ci.Position
			}
			if item.Text != "" {
				if err := s.checklistRepo.Create(ctx, item); err != nil {
					s.logger.ErrorContext(ctx, "failed to create checklist item from template", "error", err, "story_id", story.ID)
				}
			}
		}
	}

	// Create external links.
	if s.externalLinkRepo != nil && len(req.ExternalLinks) > 0 {
		for _, el := range req.ExternalLinks {
			linkURL := strings.TrimSpace(el.URL)
			if linkURL == "" {
				continue
			}
			title := strings.TrimSpace(el.Title)
			if title == "" {
				if u, parseErr := url.Parse(linkURL); parseErr == nil {
					title = u.Hostname()
				}
			}
			link := &model.PMExternalLink{
				StoryID:     story.ID,
				URL:         linkURL,
				Title:       title,
				CreatedByID: actorID,
			}
			if err := s.externalLinkRepo.Create(ctx, link); err != nil {
				s.logger.ErrorContext(ctx, "failed to create external link", "error", err, "story_id", story.ID)
			}
		}
	}

	if err := s.storyRepo.UpdateStartedCompleted(ctx, story.ID); err != nil {
		return nil, err
	}

	// Legacy path: evaluate epic automations from pm_automations table.
	// Kept during transition until migration 052 is validated and pm_automations dropped.
	if s.automationService != nil {
		s.automationService.OnStoryStateChange(ctx, story, story.WorkflowStateID)
	}

	// Evaluate automation rules for the initial state entry.
	if s.ruleEngine != nil {
		s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
			WorkspaceID: story.WorkspaceID,
			TriggerType: model.TriggerStoryStateEntered,
			StoryID:     story.ID,
			StateID:     story.WorkflowStateID,
		}, nil)
	}

	createdAction := "created this story"
	if st, _ := s.workflowRepo.GetStateByID(ctx, story.WorkflowStateID); st != nil {
		createdAction = "created this story in " + st.Name
	}
	if err := s.activityService.Log(ctx, story.WorkspaceID, "story", story.ID, optionalActor(actorID), createdAction, nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for story create", "error", err, "story_id", story.ID, "workspace_id", story.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "story", EntityID: story.ID, WorkspaceID: story.WorkspaceID, ActorID: actorID})

	// Auto-follow the creator and emit notification.
	if s.followerService != nil && actorID != "" {
		if err := s.followerService.Follow(ctx, actorID, "story", story.ID, story.WorkspaceID, "creator"); err != nil {
			s.logger.ErrorContext(ctx, "failed to auto-follow story for creator", "error", err, "story_id", story.ID, "actor_id", actorID)
		}
	}
	if s.notificationService != nil {
		var mentionedUserIDs []string
		if story.Description != nil {
			mentions := extractMentions(*story.Description)
			slog.InfoContext(ctx, "story created with mentions",
				"story_id", story.ID,
				"workspace_id", story.WorkspaceID,
				"mentions", mentions,
			)
			var err error
			mentionedUserIDs, err = resolveMentionRecipients(ctx, s.workspaceRepo, story.WorkspaceID, *story.Description, actorID, mentionScopeForTeamID(story.TeamID))
			if err != nil {
				s.logger.ErrorContext(ctx, "failed to resolve story mention recipients", "error", err, "story_id", story.ID)
				mentionedUserIDs = nil
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
			SkipFollowers:      len(mentionedUserIDs) > 0,
			EntitySnapshot: model.JSONB{
				"title":      story.Name,
				"display_id": story.DisplayID,
				"type":       story.StoryType,
			},
		}); err != nil {
			slog.ErrorContext(ctx, "failed to emit story created notification", "error", err, "story_id", story.ID)
		}
	}

	s.logger.InfoContext(ctx, "story created", "story_id", story.ID, "workspace_id", story.WorkspaceID, "actor_id", actorID)
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
	if err := requireTeamAccess(ctx, current.TeamID); err != nil {
		return nil, fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return nil, err
	}

	previousDetail, err := s.storyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if previousDetail == nil {
		return nil, fmt.Errorf("story not found")
	}

	stateChanged := false
	oldPriority := current.Priority
	oldSeverity := current.Severity
	oldStoryType := current.StoryType
	oldBlocked := current.Blocked
	oldEstimate := current.Estimate
	oldDeadline := current.Deadline
	oldBlocker := current.Blocker

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
		current.EpicID = nullableString(req.EpicID)
	}
	if req.SprintID != nil {
		current.SprintID = nullableString(req.SprintID)
	}
	if req.TeamID != nil {
		current.TeamID = nullableString(req.TeamID)
	}
	if err := validateEpicScope(ctx, s.epicRepo, current.WorkspaceID, current.EpicID, current.TeamID); err != nil {
		return nil, err
	}
	if err := validateSprintScope(ctx, s.sprintRepo, current.WorkspaceID, current.SprintID, current.TeamID); err != nil {
		return nil, err
	}
	if req.OwnerID != nil {
		// Handled below via workspace member resolution.
	}
	if req.RequesterID != nil {
		// Handled below via workspace member resolution.
	}
	if req.Estimate != nil {
		if *req.Estimate < 0 {
			current.Estimate = nil
		} else {
			current.Estimate = req.Estimate
		}
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
		// Legacy path: evaluate epic automations from pm_automations table.
		if s.automationService != nil {
			s.automationService.OnStoryStateChange(ctx, current, current.WorkflowStateID)
		}
		if s.recurringService != nil {
			if err := s.recurringService.HandleStoryProgress(ctx, current.ID); err != nil {
				s.logger.ErrorContext(ctx, "failed to process recurring template story progress", "error", err, "story_id", current.ID)
			}
		}
		// Evaluate automation rules for the state change (epic auto-start/complete, etc.)
		if s.ruleEngine != nil {
			s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
				WorkspaceID: current.WorkspaceID,
				TriggerType: model.TriggerStoryStateEntered,
				StoryID:     current.ID,
				StateID:     current.WorkflowStateID,
			}, nil)
		}
	}

	updatedDetail, err := s.storyRepo.GetByID(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	if updatedDetail == nil {
		return nil, fmt.Errorf("story not found")
	}

	// Only log meaningful field changes with descriptive messages
	if stateChanged {
		newName := current.WorkflowStateID
		if st, _ := s.workflowRepo.GetStateByID(ctx, current.WorkflowStateID); st != nil {
			newName = st.Name
		}
		action := "moved this story to " + newName
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story state change", "error", err, "story_id", current.ID, "workspace_id", current.WorkspaceID)
		}
	}
	if req.Priority != nil && *req.Priority != oldPriority {
		action := "changed priority from " + oldPriority + " to " + *req.Priority
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story priority change", "error", err, "story_id", current.ID)
		}
	}
	if req.Severity != nil && *req.Severity != oldSeverity {
		action := "changed severity from " + oldSeverity + " to " + *req.Severity
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story severity change", "error", err, "story_id", current.ID)
		}
	}
	if req.StoryType != nil && *req.StoryType != oldStoryType {
		action := "changed type from " + oldStoryType + " to " + *req.StoryType
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story type change", "error", err, "story_id", current.ID)
		}
	}
	if req.Blocked != nil && *req.Blocked != oldBlocked {
		if *req.Blocked {
			if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "marked this story as blocked", nil, nil, nil, nil); err != nil {
				s.logger.ErrorContext(ctx, "failed to log activity for story blocked", "error", err, "story_id", current.ID)
			}
		} else {
			if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "unblocked this story", nil, nil, nil, nil); err != nil {
				s.logger.ErrorContext(ctx, "failed to log activity for story unblocked", "error", err, "story_id", current.ID)
			}
		}
	}
	if req.Archived != nil && *req.Archived {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "archived this story", nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story archived", "error", err, "story_id", current.ID)
		}
	}
	oldTeamName, err := resolveStoryTeamName(ctx, s.workspaceRepo, current.WorkspaceID, previousDetail.Story.TeamID)
	if err != nil {
		return nil, err
	}
	newTeamName, err := resolveStoryTeamName(ctx, s.workspaceRepo, current.WorkspaceID, updatedDetail.Story.TeamID)
	if err != nil {
		return nil, err
	}
	if action := teamActivityAction(oldTeamName, newTeamName); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story team change", "error", err, "story_id", current.ID)
		}
	}
	if action := memberActivityAction("owner", storyMemberName(previousDetail.OwnerMember), storyMemberName(updatedDetail.OwnerMember)); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story owner change", "error", err, "story_id", current.ID)
		}
	}
	if action := memberActivityAction("requester", storyMemberName(previousDetail.RequesterMember), storyMemberName(updatedDetail.RequesterMember)); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story requester change", "error", err, "story_id", current.ID)
		}
	}
	if action := planningLinkActivityAction("epic", derefString(previousDetail.EpicName), derefString(updatedDetail.EpicName)); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story epic change", "error", err, "story_id", current.ID)
		}
	}
	if action := planningLinkActivityAction("sprint", derefString(previousDetail.SprintName), derefString(updatedDetail.SprintName)); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story sprint change", "error", err, "story_id", current.ID)
		}
	}
	if action := estimateActivityAction(oldEstimate, current.Estimate); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story estimate change", "error", err, "story_id", current.ID)
		}
	}
	if action := deadlineActivityAction(oldDeadline, current.Deadline); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story due date change", "error", err, "story_id", current.ID)
		}
	}
	if action := blockerReasonActivityAction(oldBlocker, current.Blocker, oldBlocked, current.Blocked); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for story blocker change", "error", err, "story_id", current.ID)
		}
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
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
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
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit notification for story update", "error", err, "story_id", current.ID, "event_type", eventType)
		}
	}

	// Emit mention notification when description is updated with @mentions.
	if s.notificationService != nil && req.Description != nil {
		mentions := extractMentions(*req.Description)
		slog.InfoContext(ctx, "story updated with mentions",
			"story_id", current.ID,
			"workspace_id", current.WorkspaceID,
			"mentions", mentions,
		)
		if len(mentions) > 0 {
			if _, err := emitMentionNotification(ctx, s.notificationService, s.workspaceRepo, pmMentionNotificationInput{
				WorkspaceID:     current.WorkspaceID,
				ActorID:         actorID,
				Body:            *req.Description,
				EventType:       "story.mention",
				EntityType:      "story",
				EntityID:        current.ID,
				Title:           "mentioned you in " + current.Name,
				TeamID:          derefString(current.TeamID),
				ReadableTeamIDs: mentionScopeForTeamID(current.TeamID),
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

	s.logger.InfoContext(ctx, "story updated", "story_id", current.ID, "workspace_id", current.WorkspaceID, "actor_id", actorID)
	return updatedDetail, nil
}

func nullableString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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
	if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "archived this story", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for story delete", "error", err, "story_id", current.ID, "workspace_id", current.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "story", EntityID: id, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	s.logger.InfoContext(ctx, "story deleted", "story_id", id, "workspace_id", current.WorkspaceID, "actor_id", actorID)
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
	s.logger.InfoContext(ctx, "[pm-dnd] service move start",
		"trace_id", req.DebugTraceID,
		"story_id", current.ID,
		"workspace_id", current.WorkspaceID,
		"actor_id", actorID,
		"from_state_id", current.WorkflowStateID,
		"from_position", current.Position,
		"to_state_id", req.StateID,
		"requested_position", req.Position,
	)
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
	if req.Position != nil && *req.Position < 0 {
		return nil, fmt.Errorf("position must be >= 0")
	}
	if err := s.storyRepo.MoveToState(ctx, current.ID, req.StateID, req.Position, req.DebugTraceID); err != nil {
		return nil, err
	}
	if err := s.storyRepo.UpdateStartedCompleted(ctx, current.ID); err != nil {
		return nil, err
	}
	// Legacy path: evaluate epic automations from pm_automations table.
	if s.automationService != nil {
		s.automationService.OnStoryStateChange(ctx, current, req.StateID)
	}
	newStateName := req.StateID
	if st, _ := s.workflowRepo.GetStateByID(ctx, req.StateID); st != nil {
		newStateName = st.Name
	}
	action := "moved this story to " + newStateName
	if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for story move", "error", err, "story_id", current.ID, "workspace_id", current.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      "moved",
		Entity:      "story",
		EntityID:    current.ID,
		WorkspaceID: current.WorkspaceID,
		ActorID:     actorID,
		Data:        pmDnDWebsocketData(req.DebugTraceID),
	})

	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
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
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit notification for story move", "error", err, "story_id", current.ID)
		}
	}

	// Evaluate automation rules for the state entry event.
	// If called from the rule engine's executeMoveToState, the chain context
	// is carried via ctx to preserve depth tracking and prevent double-firing.
	if s.ruleEngine != nil {
		execCtx := ruleExecCtxFromContext(ctx)
		s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
			WorkspaceID: current.WorkspaceID,
			TriggerType: model.TriggerStoryStateEntered,
			StoryID:     current.ID,
			StateID:     req.StateID,
		}, execCtx)
	}

	// Auto-start pre-assigned LLM agent on state change.
	if s.agentService != nil && current.AssignedAgentID != nil && *current.AssignedAgentID != "" {
		if _, err := s.agentService.RunAgent(ctx, current.WorkspaceID, current.ID, "system"); err != nil {
			if !errors.Is(err, ErrStoryDeliveryTargetRequired) {
				s.logger.WarnContext(ctx, "auto-start agent on state change failed",
					"error", err, "story_id", current.ID, "agent_id", *current.AssignedAgentID)
			}
		}
	}

	s.logger.InfoContext(ctx, "story moved", "story_id", current.ID, "workspace_id", current.WorkspaceID, "new_state", newStateName, "actor_id", actorID)
	detail, err := s.storyRepo.GetByID(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	if detail != nil {
		s.logger.InfoContext(ctx, "[pm-dnd] service move result",
			"trace_id", req.DebugTraceID,
			"story_id", detail.Story.ID,
			"final_state_id", detail.Story.WorkflowStateID,
			"final_position", detail.Story.Position,
			"completed", detail.Story.Completed,
			"completed_at", detail.Story.CompletedAt,
			"moved_at", detail.Story.MovedAt,
			"updated_at", detail.Story.UpdatedAt,
		)
	}
	return detail, nil
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
	s.logger.InfoContext(ctx, "[pm-dnd] service reorder start",
		"trace_id", req.DebugTraceID,
		"story_id", current.ID,
		"workspace_id", current.WorkspaceID,
		"actor_id", actorID,
		"state_id", current.WorkflowStateID,
		"from_position", current.Position,
		"requested_position", req.Position,
	)
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if req.Position < 0 {
		return fmt.Errorf("position must be >= 0")
	}
	if err := s.storyRepo.Reorder(ctx, id, req.Position, req.DebugTraceID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "reordered", stringPtr("position"), nil, nil, map[string]interface{}{"position": req.Position}); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for story reorder", "error", err, "story_id", current.ID)
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      "reordered",
		Entity:      "story",
		EntityID:    id,
		WorkspaceID: current.WorkspaceID,
		ActorID:     actorID,
		Data:        pmDnDWebsocketData(req.DebugTraceID),
	})
	if raw, err := s.storyRepo.GetRawByID(ctx, id); err == nil && raw != nil {
		s.logger.InfoContext(ctx, "[pm-dnd] service reorder result",
			"trace_id", req.DebugTraceID,
			"story_id", raw.ID,
			"state_id", raw.WorkflowStateID,
			"final_position", raw.Position,
		)
	}
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
	if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "owner_added", stringPtr("owner"), nil, &userID, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for story owner add", "error", err, "story_id", storyID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})

	// Auto-follow and notify the assigned user.
	if s.followerService != nil {
		if err := s.followerService.Follow(ctx, userID, "story", storyID, current.WorkspaceID, "assigned"); err != nil {
			s.logger.ErrorContext(ctx, "failed to auto-follow story for assigned owner", "error", err, "story_id", storyID, "user_id", userID)
		}
	}
	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
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
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit notification for story assignment", "error", err, "story_id", storyID, "user_id", userID)
		}
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
	if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "owner_removed", stringPtr("owner"), &userID, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for story owner remove", "error", err, "story_id", storyID)
	}
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
	if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "follower_added", stringPtr("follower"), nil, &userID, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for story follower add", "error", err, "story_id", storyID)
	}
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
	if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "follower_removed", stringPtr("follower"), &userID, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for story follower remove", "error", err, "story_id", storyID)
	}
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
	if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "label_added", stringPtr("label"), nil, &labelID, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for story label add", "error", err, "story_id", storyID)
	}
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
	if err := s.activityService.Log(ctx, current.WorkspaceID, "story", current.ID, optionalActor(actorID), "label_removed", stringPtr("label"), &labelID, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for story label remove", "error", err, "story_id", storyID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "story", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// ListByWorkflowState returns board columns for a workflow with optional filters.
// perStateLimit controls how many stories per column (0 = unlimited).
func (s *PMStoryService) ListByWorkflowState(ctx context.Context, workflowID string, filters model.PMStoryFilters, perStateLimit int) ([]model.StoryStateColumn, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	return s.storyRepo.ListByWorkflowState(ctx, workflowID, filters, perStateLimit)
}

// ListColumnStories returns a page of stories for a single board column.
func (s *PMStoryService) ListColumnStories(ctx context.Context, stateID string, filters model.PMStoryFilters, offset, limit int) ([]model.BoardStory, []model.StoryGroup, int, error) {
	if stateID == "" {
		return nil, nil, 0, fmt.Errorf("state_id is required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	if limit <= 0 {
		limit = 50
	}
	return s.storyRepo.ListColumnStories(ctx, stateID, filters, offset, limit)
}

// ListByMember returns board columns grouped by owner member.
func (s *PMStoryService) ListByMember(ctx context.Context, workspaceID, workflowID string, filters model.PMStoryFilters, perMemberLimit int, includeEmpty bool, memberIDs []string) ([]model.StoryMemberColumn, error) {
	if workspaceID == "" || workflowID == "" {
		return nil, fmt.Errorf("workspace_id and workflow_id are required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	return s.storyRepo.ListByMember(ctx, workspaceID, workflowID, filters, perMemberLimit, includeEmpty, memberIDs)
}

// ListMemberColumnStories returns a page of stories for a single member board column.
func (s *PMStoryService) ListMemberColumnStories(ctx context.Context, workspaceID, workflowID string, memberID *string, filters model.PMStoryFilters, offset, limit int) ([]model.BoardStory, int, error) {
	if workspaceID == "" || workflowID == "" {
		return nil, 0, fmt.Errorf("workspace_id and workflow_id are required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	if limit <= 0 {
		limit = 50
	}
	return s.storyRepo.ListMemberColumnStories(ctx, workspaceID, workflowID, memberID, filters, offset, limit)
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

func isIgnorableAutoRequesterResolutionError(err error) bool {
	if err == nil {
		return false
	}
	switch strings.TrimSpace(err.Error()) {
	case "workspace member not found", "workspace member is revoked":
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
