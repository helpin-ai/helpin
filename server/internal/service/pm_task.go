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

// PMTaskService contains task business logic.
type PMTaskService struct {
	taskRepo           *repository.PMTaskRepository
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

// NewPMTaskService creates a new PMTaskService.
func NewPMTaskService(taskRepo *repository.PMTaskRepository, workspaceRepo *repository.WorkspaceRepository, workflowRepo *repository.PMWorkflowRepository, epicRepo *repository.PMEpicRepository, sprintRepo *repository.PMSprintRepository, labelRepo *repository.PMLabelRepository, checklistRepo *repository.PMChecklistItemRepository, externalLinkRepo *repository.PMExternalLinkRepository, attachmentRepo *repository.PMAttachmentRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher, automationService *PMAutomationService, notificationService *NotificationService, followerService *FollowerService) *PMTaskService {
	return &PMTaskService{
		taskRepo:           taskRepo,
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
func (s *PMTaskService) SetRuleEngine(engine *AutomationRuleEngine) *PMTaskService {
	s.ruleEngine = engine
	return s
}

// SetAgentService sets the agent service (breaks circular dependency).
func (s *PMTaskService) SetAgentService(svc *AgentService) {
	s.agentService = svc
}

// SetRecurringService sets the recurring template service (breaks circular dependency).
func (s *PMTaskService) SetRecurringService(svc *PMRecurringTemplateService) {
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
func (s *PMTaskService) requireCanEdit(ctx context.Context, workspaceID, actorID string) error {
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
func (s *PMTaskService) requireAdmin(ctx context.Context, workspaceID, actorID string) error {
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
func (s *PMTaskService) List(ctx context.Context, workspaceID string, filters model.PMTaskFilters, pagination model.PMPagination) ([]model.BoardTask, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	return s.taskRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns story detail.
func (s *PMTaskService) GetByID(ctx context.Context, id string) (*model.TaskDetail, error) {
	story, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if story == nil {
		return nil, fmt.Errorf("story not found")
	}
	if err := requireTeamAccess(ctx, story.Task.TeamID); err != nil {
		return nil, fmt.Errorf("story not found")
	}
	return story, nil
}

// GetByDisplayID returns story detail by display ID.
func (s *PMTaskService) GetByDisplayID(ctx context.Context, workspaceID string, displayID int) (*model.TaskDetail, error) {
	story, err := s.taskRepo.GetByDisplayID(ctx, workspaceID, displayID)
	if err != nil {
		return nil, err
	}
	if story == nil {
		return nil, fmt.Errorf("story not found")
	}
	if err := requireTeamAccess(ctx, story.Task.TeamID); err != nil {
		return nil, fmt.Errorf("story not found")
	}
	return story, nil
}

// Create creates a story.
func (s *PMTaskService) Create(ctx context.Context, req model.CreateTaskRequest, actorID string) (*model.TaskDetail, error) {
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

	storyType := req.TaskType
	if storyType == "" {
		storyType = model.PMTaskTypeFeature
	}
	if !isValidTaskType(storyType) {
		return nil, fmt.Errorf("invalid task_type")
	}

	priority := model.PMTaskPriorityNone
	if req.Priority != nil && *req.Priority != "" {
		priority = *req.Priority
	}
	if !isValidTaskPriority(priority) {
		return nil, fmt.Errorf("invalid priority")
	}

	severity := model.PMTaskSeverityNone
	if req.Severity != nil && *req.Severity != "" {
		severity = *req.Severity
	}
	if !isValidTaskSeverity(severity) {
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

	story := &model.PMTask{
		WorkspaceID:       req.WorkspaceID,
		Name:              strings.TrimSpace(req.Name),
		Description:       req.Description,
		TaskType:         storyType,
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
		position, err := s.taskRepo.NextPosition(ctx, req.WorkspaceID, stateID)
		if err != nil {
			return nil, err
		}
		story.Position = position
	}

	if err := s.taskRepo.Create(ctx, story); err != nil {
		return nil, err
	}
	if len(req.AttachmentIDs) > 0 && s.attachmentRepo != nil {
		if err := s.attachmentRepo.ReassignToEntity(ctx, req.AttachmentIDs, "task", story.ID); err != nil {
			s.logger.ErrorContext(ctx, "failed to reassign attachments to task", "error", err, "task_id", story.ID, "attachment_ids", req.AttachmentIDs)
		}
	}

	ownerIDs := dedupeIDs(req.OwnerIDs)
	if story.OwnerID != nil {
		ownerIDs = append(ownerIDs, *story.OwnerID)
		ownerIDs = dedupeIDs(ownerIDs)
	}
	for _, ownerID := range ownerIDs {
		if err := s.taskRepo.AddOwner(ctx, story.ID, ownerID); err != nil {
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
		if err := s.taskRepo.AddFollower(ctx, story.ID, followerID); err != nil {
			return nil, err
		}
	}

	labelIDs := dedupeIDs(req.LabelIDs)
	if err := validateLabelScope(ctx, s.labelRepo, req.WorkspaceID, labelIDs, allowedTeamIDs(req.TeamID)); err != nil {
		return nil, err
	}
	for _, labelID := range labelIDs {
		if err := s.taskRepo.AddLabel(ctx, story.ID, labelID); err != nil {
			return nil, err
		}
	}

	// Create checklist items from template.
	if s.checklistRepo != nil && len(req.ChecklistItems) > 0 {
		for i, ci := range req.ChecklistItems {
			item := &model.PMChecklistItem{
				TaskID:     story.ID,
				Text:       strings.TrimSpace(ci.Text),
				Position:   i,
				AssigneeID: ci.AssigneeID,
			}
			if ci.Position != nil {
				item.Position = *ci.Position
			}
			if item.Text != "" {
				if err := s.checklistRepo.Create(ctx, item); err != nil {
					s.logger.ErrorContext(ctx, "failed to create checklist item from template", "error", err, "task_id", story.ID)
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
				TaskID:      story.ID,
				URL:         linkURL,
				Title:       title,
				CreatedByID: actorID,
			}
			if err := s.externalLinkRepo.Create(ctx, link); err != nil {
				s.logger.ErrorContext(ctx, "failed to create external link", "error", err, "task_id", story.ID)
			}
		}
	}

	if err := s.taskRepo.UpdateStartedCompleted(ctx, story.ID); err != nil {
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
			TriggerType: model.TriggerTaskStateEntered,
			TaskID:      story.ID,
			StoryID:     story.ID,
			StateID:     story.WorkflowStateID,
		}, nil)
	}

	createdAction := "created this task"
	if st, _ := s.workflowRepo.GetStateByID(ctx, story.WorkflowStateID); st != nil {
		createdAction = "created this task in " + st.Name
	}
	if err := s.activityService.Log(ctx, story.WorkspaceID, "task", story.ID, optionalActor(actorID), createdAction, nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task create", "error", err, "task_id", story.ID, "workspace_id", story.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "task", EntityID: story.ID, WorkspaceID: story.WorkspaceID, ActorID: actorID})

	// Auto-follow the creator and emit notification.
	if s.followerService != nil && actorID != "" {
		if err := s.followerService.Follow(ctx, actorID, "task", story.ID, story.WorkspaceID, "creator"); err != nil {
			s.logger.ErrorContext(ctx, "failed to auto-follow task for creator", "error", err, "task_id", story.ID, "actor_id", actorID)
		}
	}
	if s.notificationService != nil {
		var mentionedUserIDs []string
		if story.Description != nil {
			mentions := extractMentions(*story.Description)
			slog.InfoContext(ctx, "task created with mentions",
				"task_id", story.ID,
				"workspace_id", story.WorkspaceID,
				"mentions", mentions,
			)
			var err error
			mentionedUserIDs, err = resolveMentionRecipients(ctx, s.workspaceRepo, story.WorkspaceID, *story.Description, actorID, mentionScopeForTeamID(story.TeamID))
			if err != nil {
				s.logger.ErrorContext(ctx, "failed to resolve task mention recipients", "error", err, "task_id", story.ID)
				mentionedUserIDs = nil
			}
		}

		eventType := "task.created"
		category := "activity"
		priority := "normal"
		if len(mentionedUserIDs) > 0 {
			eventType = "task.mention"
			category = "mention"
			priority = "high"
		}

		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID:        story.WorkspaceID,
			ActorID:            actorID,
			EventType:          eventType,
			EntityType:         "task",
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
				"type":       story.TaskType,
			},
		}); err != nil {
			slog.ErrorContext(ctx, "failed to emit task created notification", "error", err, "task_id", story.ID)
		}
	}

	s.logger.InfoContext(ctx, "task created", "task_id", story.ID, "workspace_id", story.WorkspaceID, "actor_id", actorID)
	return s.taskRepo.GetByID(ctx, story.ID)
}

// Update updates story fields.
func (s *PMTaskService) Update(ctx context.Context, id string, req model.UpdateTaskRequest, actorID string) (*model.TaskDetail, error) {
	current, err := s.taskRepo.GetRawByID(ctx, id)
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

	previousDetail, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if previousDetail == nil {
		return nil, fmt.Errorf("story not found")
	}

	stateChanged := false
	oldPriority := current.Priority
	oldSeverity := current.Severity
	oldStoryType := current.TaskType
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
	if req.TaskType != nil {
		if !isValidTaskType(*req.TaskType) {
			return nil, fmt.Errorf("invalid task_type")
		}
		current.TaskType = *req.TaskType
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
		if !isValidTaskPriority(*req.Priority) {
			return nil, fmt.Errorf("invalid priority")
		}
		current.Priority = *req.Priority
	}
	if req.Severity != nil {
		if !isValidTaskSeverity(*req.Severity) {
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

	if err := s.taskRepo.Update(ctx, current); err != nil {
		return nil, err
	}

	if req.OwnerIDs != nil {
		owners := dedupeIDs(req.OwnerIDs)
		if current.OwnerID != nil {
			owners = append(owners, *current.OwnerID)
			owners = dedupeIDs(owners)
		}
		if err := s.taskRepo.ReplaceOwners(ctx, current.ID, owners); err != nil {
			return nil, err
		}
		if req.FollowerIDs == nil {
			// Auto-follow owners if explicit follower list was not provided.
			for _, ownerID := range owners {
				if err := s.taskRepo.AddFollower(ctx, current.ID, ownerID); err != nil {
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
		if err := s.taskRepo.ReplaceFollowers(ctx, current.ID, followers); err != nil {
			return nil, err
		}
	}
	if req.LabelIDs != nil {
		labelIDs := dedupeIDs(req.LabelIDs)
		if err := validateLabelScope(ctx, s.labelRepo, current.WorkspaceID, labelIDs, allowedTeamIDs(current.TeamID)); err != nil {
			return nil, err
		}
		if err := s.taskRepo.ReplaceLabels(ctx, current.ID, labelIDs); err != nil {
			return nil, err
		}
	}

	if stateChanged {
		if err := s.taskRepo.UpdateStartedCompleted(ctx, current.ID); err != nil {
			return nil, err
		}
		// Legacy path: evaluate epic automations from pm_automations table.
		if s.automationService != nil {
			s.automationService.OnStoryStateChange(ctx, current, current.WorkflowStateID)
		}
		if s.recurringService != nil {
			if err := s.recurringService.HandleStoryProgress(ctx, current.ID); err != nil {
				s.logger.ErrorContext(ctx, "failed to process recurring template task progress", "error", err, "task_id", current.ID)
			}
		}
		// Evaluate automation rules for the state change (epic auto-start/complete, etc.)
		if s.ruleEngine != nil {
			s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
				WorkspaceID: current.WorkspaceID,
				TriggerType: model.TriggerTaskStateEntered,
				TaskID:      current.ID,
				StoryID:     current.ID,
				StateID:     current.WorkflowStateID,
			}, nil)
		}
	}

	updatedDetail, err := s.taskRepo.GetByID(ctx, current.ID)
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
		action := "moved this task to " + newName
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task state change", "error", err, "task_id", current.ID, "workspace_id", current.WorkspaceID)
		}
	}
	if req.Priority != nil && *req.Priority != oldPriority {
		action := "changed priority from " + oldPriority + " to " + *req.Priority
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task priority change", "error", err, "task_id", current.ID)
		}
	}
	if req.Severity != nil && *req.Severity != oldSeverity {
		action := "changed severity from " + oldSeverity + " to " + *req.Severity
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task severity change", "error", err, "task_id", current.ID)
		}
	}
	if req.TaskType != nil && *req.TaskType != oldStoryType {
		action := "changed type from " + oldStoryType + " to " + *req.TaskType
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task type change", "error", err, "task_id", current.ID)
		}
	}
	if req.Blocked != nil && *req.Blocked != oldBlocked {
		if *req.Blocked {
			if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "marked this task as blocked", nil, nil, nil, nil); err != nil {
				s.logger.ErrorContext(ctx, "failed to log activity for task blocked", "error", err, "task_id", current.ID)
			}
		} else {
			if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "unblocked this task", nil, nil, nil, nil); err != nil {
				s.logger.ErrorContext(ctx, "failed to log activity for task unblocked", "error", err, "task_id", current.ID)
			}
		}
	}
	if req.Archived != nil && *req.Archived {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "archived this task", nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task archived", "error", err, "task_id", current.ID)
		}
	}
	oldTeamName, err := resolveTaskTeamName(ctx, s.workspaceRepo, current.WorkspaceID, previousDetail.Task.TeamID)
	if err != nil {
		return nil, err
	}
	newTeamName, err := resolveTaskTeamName(ctx, s.workspaceRepo, current.WorkspaceID, updatedDetail.Task.TeamID)
	if err != nil {
		return nil, err
	}
	if action := teamActivityAction(oldTeamName, newTeamName); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task team change", "error", err, "task_id", current.ID)
		}
	}
	if action := memberActivityAction("owner", taskMemberName(previousDetail.OwnerMember), taskMemberName(updatedDetail.OwnerMember)); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task owner change", "error", err, "task_id", current.ID)
		}
	}
	if action := memberActivityAction("requester", taskMemberName(previousDetail.RequesterMember), taskMemberName(updatedDetail.RequesterMember)); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task requester change", "error", err, "task_id", current.ID)
		}
	}
	if action := planningLinkActivityAction("epic", derefString(previousDetail.EpicName), derefString(updatedDetail.EpicName)); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task epic change", "error", err, "task_id", current.ID)
		}
	}
	if action := planningLinkActivityAction("sprint", derefString(previousDetail.SprintName), derefString(updatedDetail.SprintName)); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task sprint change", "error", err, "task_id", current.ID)
		}
	}
	if action := estimateActivityAction(oldEstimate, current.Estimate); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task estimate change", "error", err, "task_id", current.ID)
		}
	}
	if action := deadlineActivityAction(oldDeadline, current.Deadline); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task due date change", "error", err, "task_id", current.ID)
		}
	}
	if action := blockerReasonActivityAction(oldBlocker, current.Blocker, oldBlocked, current.Blocked); action != "" {
		if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
			s.logger.ErrorContext(ctx, "failed to log activity for task blocker change", "error", err, "task_id", current.ID)
		}
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: current.ID, WorkspaceID: current.WorkspaceID, ActorID: actorID})

	// Emit notification for significant updates.
	if s.notificationService != nil && (stateChanged || (req.Priority != nil && *req.Priority != oldPriority) || (req.Blocked != nil && *req.Blocked != oldBlocked)) {
		eventType := "task.updated"
		title := "Task updated: " + current.Name
		category := "activity"
		priority := "normal"
		if stateChanged {
			eventType = "task.status_changed"
			title = "Task moved: " + current.Name
			category = "status_change"
		}
		if req.Priority != nil && *req.Priority == "urgent" {
			priority = "high"
		}
		if req.Blocked != nil && *req.Blocked && !oldBlocked {
			eventType = "task.blocked"
			title = "Task blocked: " + current.Name
			category = "status_change"
			priority = "high"
		}
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: current.WorkspaceID,
			ActorID:     actorID,
			EventType:   eventType,
			EntityType:  "task",
			EntityID:    current.ID,
			Title:       title,
			Category:    category,
			Priority:    priority,
			TeamID:      derefString(current.TeamID),
			EntitySnapshot: model.JSONB{
				"title":      current.Name,
				"display_id": current.DisplayID,
				"type":       current.TaskType,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit notification for task update", "error", err, "task_id", current.ID, "event_type", eventType)
		}
	}

	// Emit mention notification when description is updated with @mentions.
	if s.notificationService != nil && req.Description != nil {
		mentions := extractMentions(*req.Description)
		slog.InfoContext(ctx, "task updated with mentions",
			"task_id", current.ID,
			"workspace_id", current.WorkspaceID,
			"mentions", mentions,
		)
		if len(mentions) > 0 {
			if _, err := emitMentionNotification(ctx, s.notificationService, s.workspaceRepo, pmMentionNotificationInput{
				WorkspaceID:     current.WorkspaceID,
				ActorID:         actorID,
				Body:            *req.Description,
				EventType:       "task.mention",
				EntityType:      "task",
				EntityID:        current.ID,
				Title:           "mentioned you in " + current.Name,
				TeamID:          derefString(current.TeamID),
				ReadableTeamIDs: mentionScopeForTeamID(current.TeamID),
				EntitySnapshot: model.JSONB{
					"title":      current.Name,
					"display_id": current.DisplayID,
					"type":       current.TaskType,
				},
			}); err != nil {
				slog.ErrorContext(ctx, "failed to emit task mention notification", "error", err, "task_id", current.ID)
			}
		}
	}

	s.logger.InfoContext(ctx, "task updated", "task_id", current.ID, "workspace_id", current.WorkspaceID, "actor_id", actorID)
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
func (s *PMTaskService) Delete(ctx context.Context, id, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireAdmin(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.taskRepo.Delete(ctx, id); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "archived this task", nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task delete", "error", err, "task_id", current.ID, "workspace_id", current.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "task", EntityID: id, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	s.logger.InfoContext(ctx, "task deleted", "task_id", id, "workspace_id", current.WorkspaceID, "actor_id", actorID)
	return nil
}

// MoveToState moves story to another state.
func (s *PMTaskService) MoveToState(ctx context.Context, id string, req model.MoveTaskRequest, actorID string) (*model.TaskDetail, error) {
	current, err := s.taskRepo.GetRawByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("story not found")
	}
	s.logger.InfoContext(ctx, "[pm-dnd] service move start",
		"trace_id", req.DebugTraceID,
		"task_id", current.ID,
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
	if err := s.taskRepo.MoveToState(ctx, current.ID, req.StateID, req.Position, req.DebugTraceID); err != nil {
		return nil, err
	}
	if err := s.taskRepo.UpdateStartedCompleted(ctx, current.ID); err != nil {
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
	action := "moved this task to " + newStateName
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), action, nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task move", "error", err, "task_id", current.ID, "workspace_id", current.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      "moved",
		Entity:      "task",
		EntityID:    current.ID,
		WorkspaceID: current.WorkspaceID,
		ActorID:     actorID,
		Data:        pmDnDWebsocketData(req.DebugTraceID),
	})

	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: current.WorkspaceID,
			ActorID:     actorID,
			EventType:   "task.status_changed",
			EntityType:  "task",
			EntityID:    current.ID,
			Title:       "moved " + current.Name + " to " + newStateName,
			Category:    "status_change",
			Priority:    "normal",
			TeamID:      derefString(current.TeamID),
			EntitySnapshot: model.JSONB{
				"title":      current.Name,
				"display_id": current.DisplayID,
				"type":       current.TaskType,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit notification for task move", "error", err, "task_id", current.ID)
		}
	}

	// Evaluate automation rules for the state entry event.
	// If called from the rule engine's executeMoveToState, the chain context
	// is carried via ctx to preserve depth tracking and prevent double-firing.
	if s.ruleEngine != nil {
		execCtx := ruleExecCtxFromContext(ctx)
		s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
			WorkspaceID: current.WorkspaceID,
			TriggerType: model.TriggerTaskStateEntered,
			TaskID:      current.ID,
			StoryID:     current.ID,
			StateID:     req.StateID,
		}, execCtx)
	}

	// Auto-start pre-assigned LLM agent on state change.
	if s.agentService != nil && current.AssignedAgentID != nil && *current.AssignedAgentID != "" {
		if _, err := s.agentService.RunAgent(ctx, current.WorkspaceID, current.ID, "system"); err != nil {
			if !errors.Is(err, ErrTaskDeliveryTargetRequired) {
				s.logger.WarnContext(ctx, "auto-start agent on state change failed",
					"error", err, "task_id", current.ID, "agent_id", *current.AssignedAgentID)
			}
		}
	}

	s.logger.InfoContext(ctx, "task moved", "task_id", current.ID, "workspace_id", current.WorkspaceID, "new_state", newStateName, "actor_id", actorID)
	detail, err := s.taskRepo.GetByID(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	if detail != nil {
		s.logger.InfoContext(ctx, "[pm-dnd] service move result",
			"trace_id", req.DebugTraceID,
			"task_id", detail.Task.ID,
			"final_state_id", detail.Task.WorkflowStateID,
			"final_position", detail.Task.Position,
			"completed", detail.Task.Completed,
			"completed_at", detail.Task.CompletedAt,
			"moved_at", detail.Task.MovedAt,
			"updated_at", detail.Task.UpdatedAt,
		)
	}
	return detail, nil
}

// Reorder changes story position in its state.
func (s *PMTaskService) Reorder(ctx context.Context, id string, req model.ReorderTaskRequest, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	s.logger.InfoContext(ctx, "[pm-dnd] service reorder start",
		"trace_id", req.DebugTraceID,
		"task_id", current.ID,
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
	if err := s.taskRepo.Reorder(ctx, id, req.Position, req.DebugTraceID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "reordered", stringPtr("position"), nil, nil, map[string]interface{}{"position": req.Position}); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task reorder", "error", err, "task_id", current.ID)
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      "reordered",
		Entity:      "task",
		EntityID:    id,
		WorkspaceID: current.WorkspaceID,
		ActorID:     actorID,
		Data:        pmDnDWebsocketData(req.DebugTraceID),
	})
	if raw, err := s.taskRepo.GetRawByID(ctx, id); err == nil && raw != nil {
		s.logger.InfoContext(ctx, "[pm-dnd] service reorder result",
			"trace_id", req.DebugTraceID,
			"task_id", raw.ID,
			"state_id", raw.WorkflowStateID,
			"final_position", raw.Position,
		)
	}
	return nil
}

// AddOwner adds an owner and auto-follows them.
func (s *PMTaskService) AddOwner(ctx context.Context, storyID, userID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, storyID)
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
	if err := s.taskRepo.AddOwner(ctx, storyID, userID); err != nil {
		return err
	}
	if err := s.taskRepo.AddFollower(ctx, storyID, userID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "owner_added", stringPtr("owner"), nil, &userID, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task owner add", "error", err, "task_id", storyID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})

	// Auto-follow and notify the assigned user.
	if s.followerService != nil {
		if err := s.followerService.Follow(ctx, userID, "task", storyID, current.WorkspaceID, "assigned"); err != nil {
			s.logger.ErrorContext(ctx, "failed to auto-follow task for assigned owner", "error", err, "task_id", storyID, "user_id", userID)
		}
	}
	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID:        current.WorkspaceID,
			ActorID:            actorID,
			EventType:          "task.assigned",
			EntityType:         "task",
			EntityID:           storyID,
			Title:              "assigned you to " + current.Name,
			Category:           "assignment",
			Priority:           "normal",
			TeamID:             derefString(current.TeamID),
			ExplicitRecipients: []string{userID},
			EntitySnapshot: model.JSONB{
				"title":      current.Name,
				"display_id": current.DisplayID,
				"type":       current.TaskType,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit notification for task assignment", "error", err, "task_id", storyID, "user_id", userID)
		}
	}

	return nil
}

// RemoveOwner removes an owner.
func (s *PMTaskService) RemoveOwner(ctx context.Context, storyID, userID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.taskRepo.RemoveOwner(ctx, storyID, userID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "owner_removed", stringPtr("owner"), &userID, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task owner remove", "error", err, "task_id", storyID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// AddFollower adds a follower.
func (s *PMTaskService) AddFollower(ctx context.Context, storyID, userID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, storyID)
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
	if err := s.taskRepo.AddFollower(ctx, storyID, userID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "follower_added", stringPtr("follower"), nil, &userID, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task follower add", "error", err, "task_id", storyID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// RemoveFollower removes a follower.
func (s *PMTaskService) RemoveFollower(ctx context.Context, storyID, userID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.taskRepo.RemoveFollower(ctx, storyID, userID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "follower_removed", stringPtr("follower"), &userID, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task follower remove", "error", err, "task_id", storyID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// AddLabel adds a label to a story.
func (s *PMTaskService) AddLabel(ctx context.Context, storyID, labelID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, storyID)
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
	if err := s.taskRepo.AddLabel(ctx, storyID, labelID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "label_added", stringPtr("label"), nil, &labelID, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task label add", "error", err, "task_id", storyID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// RemoveLabel removes a label from a story.
func (s *PMTaskService) RemoveLabel(ctx context.Context, storyID, labelID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("story not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.taskRepo.RemoveLabel(ctx, storyID, labelID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "label_removed", stringPtr("label"), &labelID, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task label remove", "error", err, "task_id", storyID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: storyID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// ListByWorkflowState returns board columns for a workflow with optional filters.
// perStateLimit controls how many stories per column (0 = unlimited).
func (s *PMTaskService) ListByWorkflowState(ctx context.Context, workflowID string, filters model.PMTaskFilters, perStateLimit int) ([]model.TaskStateColumn, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	return s.taskRepo.ListByWorkflowState(ctx, workflowID, filters, perStateLimit)
}

// ListColumnTasks returns a page of tasks for a single board column.
func (s *PMTaskService) ListColumnTasks(ctx context.Context, stateID string, filters model.PMTaskFilters, offset, limit int) ([]model.BoardTask, []model.TaskGroup, int, error) {
	if stateID == "" {
		return nil, nil, 0, fmt.Errorf("state_id is required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	if limit <= 0 {
		limit = 50
	}
	return s.taskRepo.ListColumnStories(ctx, stateID, filters, offset, limit)
}

// ListByMember returns board columns grouped by owner member.
func (s *PMTaskService) ListByMember(ctx context.Context, workspaceID, workflowID string, filters model.PMTaskFilters, perMemberLimit int, includeEmpty bool, memberIDs []string) ([]model.TaskMemberColumn, error) {
	if workspaceID == "" || workflowID == "" {
		return nil, fmt.Errorf("workspace_id and workflow_id are required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	return s.taskRepo.ListByMember(ctx, workspaceID, workflowID, filters, perMemberLimit, includeEmpty, memberIDs)
}

// ListMemberColumnTasks returns a page of tasks for a single member board column.
func (s *PMTaskService) ListMemberColumnTasks(ctx context.Context, workspaceID, workflowID string, memberID *string, filters model.PMTaskFilters, offset, limit int) ([]model.BoardTask, int, error) {
	if workspaceID == "" || workflowID == "" {
		return nil, 0, fmt.Errorf("workspace_id and workflow_id are required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	if limit <= 0 {
		limit = 50
	}
	return s.taskRepo.ListMemberColumnStories(ctx, workspaceID, workflowID, memberID, filters, offset, limit)
}

// CountByState returns state-level story counts for a workflow.
func (s *PMTaskService) CountByState(ctx context.Context, workflowID string) ([]model.TaskStateCount, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}
	return s.taskRepo.CountByState(ctx, workflowID)
}

// ListActivity returns story activity entries.
func (s *PMTaskService) ListActivity(ctx context.Context, storyID string, pagination model.PMPagination) ([]model.ActivityLogEntry, int64, error) {
	return s.activityService.ListEntity(ctx, "task", storyID, pagination)
}

func isValidTaskType(value string) bool {
	switch value {
	case model.PMTaskTypeFeature, model.PMTaskTypeBug, model.PMTaskTypeChore:
		return true
	default:
		return false
	}
}

func isValidTaskPriority(value string) bool {
	switch value {
	case model.PMTaskPriorityNone, model.PMTaskPriorityLow, model.PMTaskPriorityMedium, model.PMTaskPriorityHigh, model.PMTaskPriorityUrgent:
		return true
	default:
		return false
	}
}

func isValidTaskSeverity(value string) bool {
	switch value {
	case model.PMTaskSeverityNone, model.PMTaskSeverityMinor, model.PMTaskSeverityMajor, model.PMTaskSeverityCritical:
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
