package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

var attachmentIDAttrPattern = regexp.MustCompile(`data-attachment-id=["']([^"']+)["']`)

// PMTaskService contains task business logic.
type PMTaskService struct {
	taskRepo *repository.PMTaskRepository
	productAnalyticsEmitter
	templateRepo        *repository.PMTaskTemplateRepository
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
	gitService          *GitService
	recurringService    *PMRecurringTemplateService
	logger              *slog.Logger
}

// NewPMTaskService creates a new PMTaskService.
func NewPMTaskService(taskRepo *repository.PMTaskRepository, workspaceRepo *repository.WorkspaceRepository, workflowRepo *repository.PMWorkflowRepository, epicRepo *repository.PMEpicRepository, sprintRepo *repository.PMSprintRepository, labelRepo *repository.PMLabelRepository, checklistRepo *repository.PMChecklistItemRepository, externalLinkRepo *repository.PMExternalLinkRepository, attachmentRepo *repository.PMAttachmentRepository, activityService *PMActivityService, wsPublisher *websocket.Publisher, automationService *PMAutomationService, notificationService *NotificationService, followerService *FollowerService) *PMTaskService {
	return &PMTaskService{
		taskRepo:            taskRepo,
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
		logger:              slog.Default().With("service", "pm_task"),
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

// SetGitService sets the git service used for task delivery target inheritance.
func (s *PMTaskService) SetGitService(svc *GitService) {
	s.gitService = svc
}

// SetRecurringService sets the recurring template service (breaks circular dependency).
func (s *PMTaskService) SetRecurringService(svc *PMRecurringTemplateService) {
	s.recurringService = svc
}

// SetTaskTemplateRepository sets the template repository used by task-to-template actions.
func (s *PMTaskService) SetTaskTemplateRepository(repo *repository.PMTaskTemplateRepository) {
	s.templateRepo = repo
}

func (s *PMTaskService) inheritEpicDeliveryTarget(ctx context.Context, workspaceID, taskID, epicID, actorID string) {
	if s.gitService == nil || strings.TrimSpace(epicID) == "" {
		return
	}
	if _, changed, err := s.gitService.SyncTaskDeliveryTargetToEpic(ctx, workspaceID, taskID, epicID, actorID, false); err != nil {
		s.logger.WarnContext(ctx, "failed to inherit epic delivery target", "error", err, "workspace_id", workspaceID, "task_id", taskID, "epic_id", epicID)
	} else if changed {
		s.logger.InfoContext(ctx, "task delivery target inherited from epic", "workspace_id", workspaceID, "task_id", taskID, "epic_id", epicID)
	}
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

// getWorkspaceKey returns the workspace key for a given workspace ID, using a
// simple per-call cache to avoid N+1 queries when populating task keys.
func (s *PMTaskService) getWorkspaceKey(ctx context.Context, workspaceID string) string {
	ws, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil || ws == nil {
		return ""
	}
	return ws.WorkspaceKey
}

// GetWorkspaceKey exposes the workspace key lookup for callers outside this
// service that need to format task keys (e.g. enriching agent run payloads).
func (s *PMTaskService) GetWorkspaceKey(ctx context.Context, workspaceID string) string {
	return s.getWorkspaceKey(ctx, workspaceID)
}

// populateTaskKey sets the computed TaskKey field on a single PMTask.
func (s *PMTaskService) populateTaskKey(ctx context.Context, task *model.PMTask) {
	if task.WorkspaceID == "" {
		return
	}
	key := s.getWorkspaceKey(ctx, task.WorkspaceID)
	task.TaskKey = model.FormatTaskKey(key, task.DisplayID)
	for i := range task.BlockedByTasks {
		task.BlockedByTasks[i].TaskKey = model.FormatTaskKey(key, task.BlockedByTasks[i].DisplayID)
	}
	for i := range task.BlockingTasks {
		task.BlockingTasks[i].TaskKey = model.FormatTaskKey(key, task.BlockingTasks[i].DisplayID)
	}
}

// populateTaskDetail sets TaskKey on a TaskDetail and its embedded task.
func (s *PMTaskService) populateTaskDetail(ctx context.Context, detail *model.TaskDetail) {
	if detail == nil {
		return
	}
	s.populateTaskKey(ctx, &detail.Task)
}

// populateBoardTasks sets TaskKey on a slice of BoardTasks.
func (s *PMTaskService) populateBoardTasks(ctx context.Context, workspaceID string, tasks []model.BoardTask) {
	if len(tasks) == 0 {
		return
	}
	key := s.getWorkspaceKey(ctx, workspaceID)
	for i := range tasks {
		tasks[i].TaskKey = model.FormatTaskKey(key, tasks[i].DisplayID)
		for j := range tasks[i].BlockedByTasks {
			tasks[i].BlockedByTasks[j].TaskKey = model.FormatTaskKey(key, tasks[i].BlockedByTasks[j].DisplayID)
		}
		for j := range tasks[i].BlockingTasks {
			tasks[i].BlockingTasks[j].TaskKey = model.FormatTaskKey(key, tasks[i].BlockingTasks[j].DisplayID)
		}
	}
}

// populateStateColumns sets TaskKey on all tasks within TaskStateColumn slices.
func (s *PMTaskService) populateStateColumns(ctx context.Context, columns []model.TaskStateColumn) {
	if len(columns) == 0 {
		return
	}
	// Find workspace ID from first task in any column.
	var wsID string
	for i := range columns {
		if len(columns[i].Tasks) > 0 {
			wsID = columns[i].Tasks[0].WorkspaceID
			break
		}
	}
	if wsID == "" {
		return
	}
	key := s.getWorkspaceKey(ctx, wsID)
	for i := range columns {
		for j := range columns[i].Tasks {
			columns[i].Tasks[j].TaskKey = model.FormatTaskKey(key, columns[i].Tasks[j].DisplayID)
		}
	}
}

// populateMemberColumns sets TaskKey on all tasks within TaskMemberColumn slices.
func (s *PMTaskService) populateMemberColumns(ctx context.Context, workspaceID string, columns []model.TaskMemberColumn) {
	if len(columns) == 0 {
		return
	}
	key := s.getWorkspaceKey(ctx, workspaceID)
	for i := range columns {
		for j := range columns[i].Tasks {
			columns[i].Tasks[j].TaskKey = model.FormatTaskKey(key, columns[i].Tasks[j].DisplayID)
		}
	}
}

// List returns tasks with filters/pagination.
func (s *PMTaskService) List(ctx context.Context, workspaceID string, filters model.PMTaskFilters, pagination model.PMPagination) ([]model.BoardTask, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	filters.AccessibleTeamIDs = intersectAccessibleTeamIDs(filters.AccessibleTeamIDs, accessibleTeamIDs(ctx))
	tasks, total, err := s.taskRepo.List(ctx, workspaceID, filters, pagination)
	if err != nil {
		return nil, 0, err
	}
	s.populateBoardTasks(ctx, workspaceID, tasks)
	return tasks, total, nil
}

// GetByID returns task detail.
func (s *PMTaskService) GetByID(ctx context.Context, id string) (*model.TaskDetail, error) {
	detail, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, fmt.Errorf("task not found")
	}
	if err := requireTeamAccess(ctx, detail.Task.TeamID); err != nil {
		return nil, fmt.Errorf("task not found")
	}
	s.populateTaskDetail(ctx, detail)
	return detail, nil
}

// GetByDisplayID returns task detail by display ID.
func (s *PMTaskService) GetByDisplayID(ctx context.Context, workspaceID string, displayID int) (*model.TaskDetail, error) {
	detail, err := s.taskRepo.GetByDisplayID(ctx, workspaceID, displayID)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, fmt.Errorf("task not found")
	}
	if err := requireTeamAccess(ctx, detail.Task.TeamID); err != nil {
		return nil, fmt.Errorf("task not found")
	}
	s.populateTaskDetail(ctx, detail)
	return detail, nil
}

// Create creates a task.
func (s *PMTaskService) Create(ctx context.Context, req model.CreateTaskRequest, actorID string) (*model.TaskDetail, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if err := s.requireCanEdit(ctx, req.WorkspaceID, actorID); err != nil {
		return nil, err
	}
	if err := s.applyTemplateDefaultsToCreateRequest(ctx, &req); err != nil {
		return nil, err
	}
	if err := requireTeamMembershipForCreate(ctx, req.TeamID); err != nil {
		return nil, err
	}

	workflowID := req.WorkflowID
	stateID := req.WorkflowStateID
	if workflowID == "" && stateID != "" {
		state, err := s.workflowRepo.GetStateByID(ctx, stateID)
		if err != nil {
			return nil, err
		}
		if state != nil {
			workflowID = state.WorkflowID
		}
	}
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

	taskType := req.TaskType
	if taskType == "" {
		taskType = model.PMTaskTypeFeature
	}
	if !isValidTaskType(taskType) {
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

	newTask := &model.PMTask{
		WorkspaceID:       req.WorkspaceID,
		Name:              strings.TrimSpace(req.Name),
		Description:       req.Description,
		TaskType:          taskType,
		WorkflowID:        workflowID,
		WorkflowStateID:   stateID,
		EpicID:            req.EpicID,
		SprintID:          req.SprintID,
		TeamID:            req.TeamID,
		RequesterID:       memberUserIDPtr(requesterMember),
		RequesterMemberID: memberIDPtr(requesterMember),
		Estimate:          req.Estimate,
		Priority:          priority,
		Severity:          severity,
		Deadline:          req.Deadline,
		Blocked:           blocked,
		Blocker:           req.Blocker,
		AssignedAgentID:   nullableString(req.AssignedAgentID),
		TemplateID:        req.TemplateID,
		ExternalID:        req.ExternalID,
	}
	if newTask.AssignedAgentID != nil && s.agentService != nil {
		if err := s.agentService.ValidateRunnableTargetAgent(ctx, req.WorkspaceID, *newTask.AssignedAgentID, "task", newTask.TeamID); err != nil {
			return nil, err
		}
	}
	if err := validateEpicScope(ctx, s.epicRepo, req.WorkspaceID, newTask.EpicID, newTask.TeamID); err != nil {
		return nil, err
	}
	if err := validateSprintScope(ctx, s.sprintRepo, req.WorkspaceID, newTask.SprintID, newTask.TeamID); err != nil {
		return nil, err
	}
	// Resolve and validate every referenced row before opening the mutation
	// transaction. This prevents a late invalid owner/label/checklist assignee
	// from leaving behind a partially-created task.
	ownerIDs, err := s.resolveOwnerUserIDs(ctx, req.WorkspaceID, req.OwnerMemberIDs, req.OwnerIDs)
	if err != nil {
		return nil, err
	}
	labelIDs := dedupeIDs(req.LabelIDs)
	if err := validateLabelScope(ctx, s.labelRepo, req.WorkspaceID, labelIDs, allowedTeamIDs(req.TeamID)); err != nil {
		return nil, err
	}
	followerIDs := dedupeIDs(req.FollowerIDs)
	if newTask.RequesterID != nil {
		followerIDs = append(followerIDs, *newTask.RequesterID)
	}
	followerIDs = dedupeIDs(append(followerIDs, ownerIDs...))
	checklistItems := make([]model.PMChecklistItem, 0, len(req.ChecklistItems))
	for i, ci := range req.ChecklistItems {
		text := strings.TrimSpace(ci.Text)
		if text == "" {
			continue
		}
		if ci.AssigneeID != nil && strings.TrimSpace(*ci.AssigneeID) != "" {
			membership, err := s.workspaceRepo.GetMembership(ctx, req.WorkspaceID, strings.TrimSpace(*ci.AssigneeID))
			if err != nil {
				return nil, err
			}
			if membership == nil {
				return nil, fmt.Errorf("checklist assignee must be an active workspace member")
			}
		}
		position := i
		if ci.Position != nil {
			position = *ci.Position
		}
		checklistItems = append(checklistItems, model.PMChecklistItem{Text: text, Position: position, AssigneeID: ci.AssigneeID, DueDate: ci.DueDate})
	}

	if err := s.taskRepo.WithMutationTransaction(ctx, func(tasks *repository.PMTaskRepository, checklist *repository.PMChecklistItemRepository) error {
		if err := tasks.CreateWithPosition(ctx, newTask, req.Position); err != nil {
			return err
		}
		if err := tasks.ReplaceOwners(ctx, newTask.ID, ownerIDs); err != nil {
			return err
		}
		if err := tasks.ReplaceFollowers(ctx, newTask.ID, followerIDs); err != nil {
			return err
		}
		if err := tasks.ReplaceLabels(ctx, newTask.ID, labelIDs); err != nil {
			return err
		}
		for i := range checklistItems {
			checklistItems[i].TaskID = newTask.ID
			if err := checklist.Create(ctx, &checklistItems[i]); err != nil {
				return err
			}
		}
		return tasks.UpdateStartedCompleted(ctx, newTask.ID)
	}); err != nil {
		return nil, err
	}
	if newTask.EpicID != nil && strings.TrimSpace(*newTask.EpicID) != "" {
		s.inheritEpicDeliveryTarget(ctx, newTask.WorkspaceID, newTask.ID, strings.TrimSpace(*newTask.EpicID), actorID)
	}
	if len(req.AttachmentIDs) > 0 && s.attachmentRepo != nil {
		if err := s.attachmentRepo.ReassignToEntity(ctx, req.AttachmentIDs, "task", newTask.ID); err != nil {
			s.logger.ErrorContext(ctx, "failed to reassign attachments to task", "error", err, "task_id", newTask.ID, "attachment_ids", req.AttachmentIDs)
		}
	}
	if req.TemplateID != nil && strings.TrimSpace(*req.TemplateID) != "" && s.attachmentRepo != nil {
		attachmentClones, err := s.attachmentRepo.CloneUploadedFromEntityToEntityWithSources(ctx, "task_template", strings.TrimSpace(*req.TemplateID), "task", newTask.ID)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to clone template attachments to task", "error", err, "task_id", newTask.ID, "template_id", *req.TemplateID)
		} else {
			if missingInlineIDs := missingInlineAttachmentIDs(newTask.Description, attachmentIDRewriteMap(attachmentClones)); len(missingInlineIDs) > 0 {
				inlineClones, err := s.attachmentRepo.CloneUploadedByIDToEntity(ctx, missingInlineIDs, "task", newTask.ID)
				if err != nil {
					s.logger.ErrorContext(ctx, "failed to clone inline template attachments to task", "error", err, "task_id", newTask.ID, "template_id", *req.TemplateID)
				} else {
					attachmentClones = append(attachmentClones, inlineClones...)
				}
			}
			if rewrittenDescription := rewriteAttachmentIDs(newTask.Description, attachmentIDRewriteMap(attachmentClones)); !stringPtrEqual(rewrittenDescription, newTask.Description) {
				newTask.Description = rewrittenDescription
				if err := s.taskRepo.UpdateFields(ctx, newTask.ID, map[string]interface{}{"description": rewrittenDescription}); err != nil {
					s.logger.ErrorContext(ctx, "failed to rewrite template attachment ids in task description", "error", err, "task_id", newTask.ID, "template_id", *req.TemplateID)
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
			taskID := newTask.ID
			link := &model.PMExternalLink{
				TaskID:      &taskID,
				EntityType:  "task",
				EntityID:    newTask.ID,
				URL:         linkURL,
				Title:       title,
				CreatedByID: actorID,
			}
			if err := s.externalLinkRepo.Create(ctx, link); err != nil {
				s.logger.ErrorContext(ctx, "failed to create external link", "error", err, "task_id", newTask.ID)
			}
		}
	}

	// Legacy path: evaluate epic automations from pm_automations table.
	// Kept during transition until migration 052 is validated and pm_automations dropped.
	if s.automationService != nil {
		s.automationService.OnStoryStateChange(ctx, newTask, newTask.WorkflowStateID)
	}

	// Evaluate automation rules for the initial state entry.
	if s.ruleEngine != nil {
		s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
			WorkspaceID: newTask.WorkspaceID,
			TriggerType: model.TriggerTaskStateEntered,
			TaskID:      newTask.ID,
			StoryID:     newTask.ID,
			StateID:     newTask.WorkflowStateID,
		}, nil)
	}

	createdAction := "created this task"
	if st, _ := s.workflowRepo.GetStateByID(ctx, newTask.WorkflowStateID); st != nil {
		createdAction = "created this task in " + st.Name
	}
	if err := s.activityService.Log(ctx, newTask.WorkspaceID, "task", newTask.ID, optionalActor(actorID), createdAction, nil, nil, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task create", "error", err, "task_id", newTask.ID, "workspace_id", newTask.WorkspaceID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "created", Entity: "task", EntityID: newTask.ID, WorkspaceID: newTask.WorkspaceID, ActorID: actorID, TaskKey: model.FormatTaskKey(s.getWorkspaceKey(ctx, newTask.WorkspaceID), newTask.DisplayID)})

	// Auto-follow the creator and emit notification.
	if s.followerService != nil && actorID != "" {
		if err := s.followerService.Follow(ctx, actorID, "task", newTask.ID, newTask.WorkspaceID, "creator"); err != nil {
			s.logger.ErrorContext(ctx, "failed to auto-follow task for creator", "error", err, "task_id", newTask.ID, "actor_id", actorID)
		}
	}
	if s.notificationService != nil {
		var mentionedUserIDs []string
		if newTask.Description != nil {
			mentions := extractMentions(*newTask.Description)
			slog.InfoContext(ctx, "task created with mentions",
				"task_id", newTask.ID,
				"workspace_id", newTask.WorkspaceID,
				"mentions", mentions,
			)
			var err error
			mentionedUserIDs, err = resolveMentionRecipients(ctx, s.workspaceRepo, newTask.WorkspaceID, *newTask.Description, actorID, mentionScopeForTeamID(newTask.TeamID))
			if err != nil {
				s.logger.ErrorContext(ctx, "failed to resolve task mention recipients", "error", err, "task_id", newTask.ID)
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
			WorkspaceID:        newTask.WorkspaceID,
			ActorID:            actorID,
			EventType:          eventType,
			EntityType:         "task",
			EntityID:           newTask.ID,
			Title:              "created " + newTask.Name,
			Category:           category,
			Priority:           priority,
			TeamID:             derefString(newTask.TeamID),
			ExplicitRecipients: mentionedUserIDs,
			SkipFollowers:      len(mentionedUserIDs) > 0,
			EntitySnapshot: model.JSONB{
				"title":      newTask.Name,
				"display_id": model.FormatTaskKey(s.getWorkspaceKey(ctx, newTask.WorkspaceID), newTask.DisplayID),
				"type":       newTask.TaskType,
			},
		}); err != nil {
			slog.ErrorContext(ctx, "failed to emit task created notification", "error", err, "task_id", newTask.ID)
		}
	}

	s.logger.InfoContext(ctx, "task created", "task_id", newTask.ID, "workspace_id", newTask.WorkspaceID, "actor_id", actorID)
	detail, err := s.taskRepo.GetByID(ctx, newTask.ID)
	if err != nil {
		return nil, err
	}
	s.populateTaskDetail(ctx, detail)
	s.trackProductEvent(ctx, ProductAnalyticsEvent{
		SemanticKey: "task_created:" + newTask.ID, UserID: actorID,
		WorkspaceID: newTask.WorkspaceID, Name: "task_created", Source: "api",
		OccurredAt: newTask.CreatedAt,
		Attributes: map[string]any{"entity_id": newTask.ID, "task_type": newTask.TaskType, "priority": newTask.Priority, "team_id": newTask.TeamID, "epic_id": newTask.EpicID, "module": "pm"},
	})
	return detail, nil
}

// CreateWithAgentRun creates a task and optionally starts the assigned agent.
func (s *PMTaskService) CreateWithAgentRun(ctx context.Context, req model.CreateTaskRequest, actorID string) (*model.CreateTaskResponse, error) {
	detail, err := s.Create(ctx, req, actorID)
	if err != nil {
		return nil, err
	}
	resp := &model.CreateTaskResponse{Task: *detail}

	assignedAgentID := nullableString(req.AssignedAgentID)
	if !req.RunOnCreate || assignedAgentID == nil {
		return resp, nil
	}
	if s.agentService == nil {
		msg := "agent service is not configured"
		resp.AgentRunError = &msg
		return resp, nil
	}
	run, err := s.agentService.RunTaskAgent(ctx, detail.Task.WorkspaceID, detail.Task.ID, actorID, model.StartAgentRunRequest{
		AgentID: *assignedAgentID,
	})
	if err != nil {
		msg := err.Error()
		resp.AgentRunError = &msg
		return resp, nil
	}
	resp.AgentRun = run
	return resp, nil
}

type taskTemplateChecklistItem struct {
	Text     string `json:"text"`
	Position int    `json:"position,omitempty"`
}

type taskTemplateExternalLink struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// SaveAsTemplate creates a reusable template from an existing task.
func (s *PMTaskService) SaveAsTemplate(ctx context.Context, taskID string, req model.SaveTaskAsTemplateRequest) (*model.PMTaskTemplate, error) {
	if s.templateRepo == nil {
		return nil, fmt.Errorf("task template repository is not configured")
	}
	detail, err := s.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	task := detail.Task
	if err := requireCanManage(ctx, task.TeamID); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(task.Name)
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name = strings.TrimSpace(*req.Name)
	}
	if name == "" {
		return nil, fmt.Errorf("template name is required")
	}
	existing, err := s.templateRepo.GetByName(ctx, task.WorkspaceID, task.TeamID, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("template name already exists in this scope")
	}

	labelIDs := make([]string, 0, len(detail.Labels))
	for _, label := range detail.Labels {
		labelIDs = append(labelIDs, label.ID)
	}
	labelIDsJSON, err := optionalTaskTemplateJSON(labelIDs)
	if err != nil {
		return nil, err
	}

	var checklistItems []taskTemplateChecklistItem
	if s.checklistRepo != nil {
		items, err := s.checklistRepo.List(ctx, task.ID)
		if err != nil {
			return nil, err
		}
		checklistItems = make([]taskTemplateChecklistItem, 0, len(items))
		for _, item := range items {
			text := strings.TrimSpace(item.Text)
			if text == "" {
				continue
			}
			checklistItems = append(checklistItems, taskTemplateChecklistItem{
				Text:     text,
				Position: item.Position,
			})
		}
	}
	checklistJSON, err := optionalTaskTemplateJSON(checklistItems)
	if err != nil {
		return nil, err
	}

	var externalLinks []taskTemplateExternalLink
	if s.externalLinkRepo != nil {
		links, err := s.externalLinkRepo.List(ctx, task.ID)
		if err != nil {
			return nil, err
		}
		externalLinks = make([]taskTemplateExternalLink, 0, len(links))
		for _, link := range links {
			linkURL := strings.TrimSpace(link.URL)
			if linkURL == "" {
				continue
			}
			externalLinks = append(externalLinks, taskTemplateExternalLink{
				URL:   linkURL,
				Title: strings.TrimSpace(link.Title),
			})
		}
	}
	externalLinksJSON, err := optionalTaskTemplateJSON(externalLinks)
	if err != nil {
		return nil, err
	}

	taskType := task.TaskType
	priority := task.Priority
	severity := task.Severity
	deadline := optionalTaskTemplateDate(task.Deadline)
	var ownerMemberID *string
	if len(task.OwnerMemberIDs) > 0 && strings.TrimSpace(task.OwnerMemberIDs[0]) != "" {
		ownerMemberID = &task.OwnerMemberIDs[0]
	}
	ownerMemberIDsJSON, err := optionalTaskTemplateJSON(dedupeIDs(task.OwnerMemberIDs))
	if err != nil {
		return nil, err
	}

	tmpl := &model.PMTaskTemplate{
		WorkspaceID:     task.WorkspaceID,
		TeamID:          task.TeamID,
		Name:            name,
		Description:     task.Description,
		TaskType:        &taskType,
		Priority:        &priority,
		Severity:        &severity,
		Estimate:        task.Estimate,
		LabelIDs:        labelIDsJSON,
		OwnerMemberID:   ownerMemberID,
		OwnerMemberIDs:  ownerMemberIDsJSON,
		EpicID:          task.EpicID,
		SprintID:        task.SprintID,
		WorkflowStateID: &task.WorkflowStateID,
		Deadline:        deadline,
		ChecklistItems:  checklistJSON,
		ExternalLinks:   externalLinksJSON,
	}
	if err := s.templateRepo.Create(ctx, tmpl); err != nil {
		return nil, err
	}
	if s.attachmentRepo != nil {
		attachmentClones, err := s.attachmentRepo.CloneUploadedFromEntityToEntityWithSources(ctx, "task", task.ID, "task_template", tmpl.ID)
		if err != nil {
			return nil, err
		}
		if missingInlineIDs := missingInlineAttachmentIDs(tmpl.Description, attachmentIDRewriteMap(attachmentClones)); len(missingInlineIDs) > 0 {
			inlineClones, err := s.attachmentRepo.CloneUploadedByIDToEntity(ctx, missingInlineIDs, "task_template", tmpl.ID)
			if err != nil {
				return nil, err
			}
			attachmentClones = append(attachmentClones, inlineClones...)
		}
		if rewrittenDescription := rewriteAttachmentIDs(tmpl.Description, attachmentIDRewriteMap(attachmentClones)); !stringPtrEqual(rewrittenDescription, tmpl.Description) {
			tmpl.Description = rewrittenDescription
			if err := s.templateRepo.Update(ctx, tmpl); err != nil {
				return nil, err
			}
		}
	}
	publishWorkspaceEvent(s.wsPublisher, "created", "story_template", tmpl.ID, task.WorkspaceID, "")
	return tmpl, nil
}

func attachmentIDRewriteMap(clones []repository.PMAttachmentClone) map[string]string {
	if len(clones) == 0 {
		return nil
	}
	replacements := make(map[string]string, len(clones))
	for _, clone := range clones {
		replacements[clone.Source.ID] = clone.Clone.ID
	}
	return replacements
}

func missingInlineAttachmentIDs(description *string, replacements map[string]string) []string {
	ids := extractInlineAttachmentIDs(description)
	if len(ids) == 0 {
		return nil
	}
	missing := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := replacements[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}

func extractInlineAttachmentIDs(description *string) []string {
	if description == nil || *description == "" {
		return nil
	}
	matches := attachmentIDAttrPattern.FindAllStringSubmatch(*description, -1)
	if len(matches) == 0 {
		return nil
	}
	ids := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		id := strings.TrimSpace(match[1])
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func rewriteAttachmentIDs(description *string, replacements map[string]string) *string {
	if description == nil || len(replacements) == 0 {
		return description
	}
	rewritten := *description
	for sourceID, targetID := range replacements {
		if sourceID == "" || targetID == "" || sourceID == targetID {
			continue
		}
		rewritten = strings.ReplaceAll(rewritten, `data-attachment-id="`+sourceID+`"`, `data-attachment-id="`+targetID+`"`)
		rewritten = strings.ReplaceAll(rewritten, `data-attachment-id='`+sourceID+`'`, `data-attachment-id='`+targetID+`'`)
	}
	if rewritten == *description {
		return description
	}
	return &rewritten
}

func stringPtrEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func optionalTaskTemplateJSON[T any](items []T) (*string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	value := string(payload)
	return &value, nil
}

func optionalTaskTemplateDate(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format("2006-01-02")
	return &formatted
}

func (s *PMTaskService) applyTemplateDefaultsToCreateRequest(ctx context.Context, req *model.CreateTaskRequest) error {
	if req == nil || req.TemplateID == nil || strings.TrimSpace(*req.TemplateID) == "" {
		return nil
	}
	if s.templateRepo == nil {
		return nil
	}
	tmpl, err := s.templateRepo.GetByID(ctx, strings.TrimSpace(*req.TemplateID))
	if err != nil {
		return err
	}
	if tmpl == nil {
		return fmt.Errorf("task template not found")
	}
	if tmpl.WorkspaceID != req.WorkspaceID {
		return fmt.Errorf("task template not found")
	}
	if !canViewTaskTemplate(ctx, tmpl.TeamID) {
		return &model.ErrForbidden{Message: "you do not have access to this template"}
	}

	if req.TeamID == nil {
		req.TeamID = tmpl.TeamID
	}
	if req.Description == nil {
		req.Description = tmpl.Description
	}
	if strings.TrimSpace(req.TaskType) == "" && tmpl.TaskType != nil {
		req.TaskType = *tmpl.TaskType
	}
	if req.Priority == nil {
		req.Priority = tmpl.Priority
	}
	if req.Severity == nil {
		req.Severity = tmpl.Severity
	}
	if req.Estimate == nil {
		req.Estimate = tmpl.Estimate
	}
	if req.EpicID == nil {
		req.EpicID = tmpl.EpicID
	}
	if req.SprintID == nil {
		req.SprintID = tmpl.SprintID
	}
	if strings.TrimSpace(req.WorkflowStateID) == "" && tmpl.WorkflowStateID != nil {
		req.WorkflowStateID = *tmpl.WorkflowStateID
	}
	if req.Deadline == nil && tmpl.Deadline != nil && strings.TrimSpace(*tmpl.Deadline) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*tmpl.Deadline))
		if err != nil {
			return fmt.Errorf("invalid task template deadline: %w", err)
		}
		req.Deadline = &parsed
	}
	if len(req.LabelIDs) == 0 {
		labelIDs, err := parseTaskTemplateStringSlice(tmpl.LabelIDs)
		if err != nil {
			return err
		}
		req.LabelIDs = labelIDs
	}
	if len(req.OwnerMemberIDs) == 0 {
		ownerMemberIDs, err := parseTaskTemplateOwnerMemberIDs(tmpl)
		if err != nil {
			return err
		}
		req.OwnerMemberIDs = ownerMemberIDs
	}
	if len(req.ChecklistItems) == 0 {
		checklistItems, err := parseTaskTemplateChecklistItems(tmpl.ChecklistItems)
		if err != nil {
			return err
		}
		req.ChecklistItems = checklistItems
	}
	if len(req.ExternalLinks) == 0 {
		externalLinks, err := parseTaskTemplateExternalLinks(tmpl.ExternalLinks)
		if err != nil {
			return err
		}
		req.ExternalLinks = externalLinks
	}
	return nil
}

func parseTaskTemplateStringSlice(value *string) ([]string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	var result []string
	if err := json.Unmarshal([]byte(*value), &result); err != nil {
		return nil, err
	}
	return dedupeIDs(result), nil
}

func parseTaskTemplateOwnerMemberIDs(tmpl *model.PMTaskTemplate) ([]string, error) {
	if tmpl == nil {
		return nil, nil
	}
	ownerMemberIDs, err := parseTaskTemplateStringSlice(tmpl.OwnerMemberIDs)
	if err != nil {
		return nil, err
	}
	if len(ownerMemberIDs) == 0 && tmpl.OwnerMemberID != nil {
		ownerMemberIDs = append(ownerMemberIDs, *tmpl.OwnerMemberID)
	}
	return dedupeIDs(ownerMemberIDs), nil
}

func parseTaskTemplateChecklistItems(value *string) ([]model.CreateChecklistItemRequest, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	var raw []taskTemplateChecklistItem
	if err := json.Unmarshal([]byte(*value), &raw); err != nil {
		return nil, err
	}
	items := make([]model.CreateChecklistItemRequest, 0, len(raw))
	for _, item := range raw {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		position := item.Position
		items = append(items, model.CreateChecklistItemRequest{Text: text, Position: &position})
	}
	return items, nil
}

func parseTaskTemplateExternalLinks(value *string) ([]model.CreateExternalLinkRequest, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	var raw []taskTemplateExternalLink
	if err := json.Unmarshal([]byte(*value), &raw); err != nil {
		return nil, err
	}
	links := make([]model.CreateExternalLinkRequest, 0, len(raw))
	for _, link := range raw {
		linkURL := strings.TrimSpace(link.URL)
		if linkURL == "" {
			continue
		}
		links = append(links, model.CreateExternalLinkRequest{URL: linkURL, Title: strings.TrimSpace(link.Title)})
	}
	return links, nil
}

// Duplicate creates a fresh task from reusable content on an existing task.
func (s *PMTaskService) Duplicate(ctx context.Context, taskID, actorID string) (*model.TaskDetail, error) {
	detail, err := s.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, fmt.Errorf("task not found")
	}
	source := detail.Task
	if err := s.requireCanEdit(ctx, source.WorkspaceID, actorID); err != nil {
		return nil, err
	}

	labelIDs := make([]string, 0, len(detail.Labels))
	for _, label := range detail.Labels {
		labelIDs = append(labelIDs, label.ID)
	}

	var checklistItems []model.PMChecklistItem
	if s.checklistRepo != nil {
		checklistItems, err = s.checklistRepo.List(ctx, source.ID)
		if err != nil {
			return nil, err
		}
	}

	externalLinks := []model.CreateExternalLinkRequest{}
	if s.externalLinkRepo != nil {
		links, err := s.externalLinkRepo.List(ctx, source.ID)
		if err != nil {
			return nil, err
		}
		externalLinks = make([]model.CreateExternalLinkRequest, 0, len(links))
		for _, link := range links {
			linkURL := strings.TrimSpace(link.URL)
			if linkURL == "" {
				continue
			}
			externalLinks = append(externalLinks, model.CreateExternalLinkRequest{
				URL:   linkURL,
				Title: strings.TrimSpace(link.Title),
			})
		}
	}

	duplicateName := strings.TrimSpace(source.Name) + " (copy)"
	if strings.TrimSpace(source.Name) == "" {
		duplicateName = "Untitled task (copy)"
	}
	duplicate, err := s.Create(ctx, model.CreateTaskRequest{
		WorkspaceID:       source.WorkspaceID,
		Name:              duplicateName,
		Description:       source.Description,
		TaskType:          source.TaskType,
		WorkflowID:        source.WorkflowID,
		WorkflowStateID:   source.WorkflowStateID,
		EpicID:            source.EpicID,
		SprintID:          source.SprintID,
		TeamID:            source.TeamID,
		OwnerMemberIDs:    dedupeIDs(source.OwnerMemberIDs),
		RequesterID:       source.RequesterID,
		RequesterMemberID: source.RequesterMemberID,
		Estimate:          source.Estimate,
		Priority:          &source.Priority,
		Severity:          &source.Severity,
		Deadline:          source.Deadline,
		Blocked:           &source.Blocked,
		Blocker:           source.Blocker,
		LabelIDs:          labelIDs,
		ExternalLinks:     externalLinks,
	}, actorID)
	if err != nil {
		return nil, err
	}

	if s.checklistRepo != nil && len(checklistItems) > 0 {
		for _, sourceItem := range checklistItems {
			text := strings.TrimSpace(sourceItem.Text)
			if text == "" {
				continue
			}
			item := &model.PMChecklistItem{
				TaskID:     duplicate.Task.ID,
				Text:       text,
				Completed:  sourceItem.Completed,
				Position:   sourceItem.Position,
				AssigneeID: sourceItem.AssigneeID,
				DueDate:    sourceItem.DueDate,
			}
			if err := s.checklistRepo.Create(ctx, item); err != nil {
				return nil, err
			}
		}
	}

	if s.attachmentRepo != nil {
		attachmentClones, err := s.attachmentRepo.CloneUploadedFromEntityToEntityWithSources(ctx, "task", source.ID, "task", duplicate.Task.ID)
		if err != nil {
			return nil, err
		}
		if missingInlineIDs := missingInlineAttachmentIDs(duplicate.Task.Description, attachmentIDRewriteMap(attachmentClones)); len(missingInlineIDs) > 0 {
			inlineClones, err := s.attachmentRepo.CloneUploadedByIDToEntity(ctx, missingInlineIDs, "task", duplicate.Task.ID)
			if err != nil {
				return nil, err
			}
			attachmentClones = append(attachmentClones, inlineClones...)
		}
		if rewrittenDescription := rewriteAttachmentIDs(duplicate.Task.Description, attachmentIDRewriteMap(attachmentClones)); !stringPtrEqual(rewrittenDescription, duplicate.Task.Description) {
			duplicate.Task.Description = rewrittenDescription
			if err := s.taskRepo.UpdateFields(ctx, duplicate.Task.ID, map[string]interface{}{"description": rewrittenDescription}); err != nil {
				return nil, err
			}
		}
	}

	created, err := s.taskRepo.GetByID(ctx, duplicate.Task.ID)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, fmt.Errorf("task not found")
	}
	s.populateTaskDetail(ctx, created)
	return created, nil
}

// Seed creates a batch of synthetic tasks for board and list testing.
func (s *PMTaskService) Seed(ctx context.Context, req model.SeedPMTasksRequest) (*model.SeedPMTasksResponse, error) {
	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	count := req.Count
	if count <= 0 {
		count = 500
	}
	if count > 2000 {
		return nil, fmt.Errorf("count cannot exceed 2000")
	}

	workflow, err := s.workflowRepo.GetDefaultWorkflow(ctx, req.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if workflow == nil || len(workflow.States) == 0 {
		workflow, err = s.workflowRepo.SeedDefaultWorkflow(ctx, req.WorkspaceID)
		if err != nil {
			return nil, err
		}
	}
	if workflow == nil || len(workflow.States) == 0 {
		return nil, fmt.Errorf("default workflow is required")
	}

	maxDisplayID, err := s.taskRepo.GetMaxDisplayID(ctx, req.WorkspaceID)
	if err != nil {
		return nil, err
	}

	nextPositionByState := make(map[string]int, len(workflow.States))
	for _, state := range workflow.States {
		position, err := s.taskRepo.NextPosition(ctx, req.WorkspaceID, state.ID)
		if err != nil {
			return nil, err
		}
		nextPositionByState[state.ID] = position
	}

	assignableMembers, err := s.workspaceRepo.ListAssignableMembers(ctx, req.WorkspaceID)
	if err != nil {
		return nil, err
	}
	activeMembers := make([]model.AssignableMember, 0, len(assignableMembers))
	for _, member := range assignableMembers {
		if member.Status == model.WorkspaceMemberStatusActive {
			activeMembers = append(activeMembers, member)
		}
	}

	now := time.Now().UTC()
	tasks := make([]model.PMTask, 0, count)
	for i := 0; i < count; i++ {
		sequence := maxDisplayID + i + 1
		state := seededTaskWorkflowState(workflow.States, i)
		position := nextPositionByState[state.ID]
		nextPositionByState[state.ID] = position + 1

		taskType := seededTaskType(i)
		priority := seededTaskPriority(i)
		severity := seededTaskSeverity(i)
		estimate := seededTaskEstimate(i)
		name := seededTaskName(sequence, taskType)
		description := seededTaskDescription(sequence, taskType, state.Name)
		externalID := fmt.Sprintf("seed-task-%d", sequence)
		createdAt := now.Add(-time.Duration(i) * 11 * time.Minute)
		updatedAt := createdAt.Add(time.Duration((i%5)+1) * time.Minute)

		var (
			started     bool
			startedAt   *time.Time
			completed   bool
			completedAt *time.Time
			movedAt     *time.Time
			deadline    *time.Time
			blocked     bool
			blocker     *string
		)

		switch state.StateType {
		case model.PMStateTypeStarted:
			started = true
			startedAtValue := createdAt.Add(2 * time.Hour)
			startedAt = &startedAtValue
			movedAtValue := updatedAt
			movedAt = &movedAtValue
		case model.PMStateTypeDone:
			started = true
			completed = true
			startedAtValue := createdAt.Add(90 * time.Minute)
			completedAtValue := updatedAt.Add(45 * time.Minute)
			startedAt = &startedAtValue
			completedAt = &completedAtValue
			movedAtValue := completedAtValue
			movedAt = &movedAtValue
		}

		if state.StateType != model.PMStateTypeDone && i%6 != 0 {
			deadlineValue := createdAt.AddDate(0, 0, 3+(i%12))
			deadline = &deadlineValue
		}
		if state.StateType != model.PMStateTypeDone && i%9 == 0 {
			blocked = true
			blockerValue := seededTaskBlocker(i)
			blocker = &blockerValue
		}

		requesterMember := seededTaskMember(activeMembers, i+1)

		tasks = append(tasks, model.PMTask{
			WorkspaceID:       req.WorkspaceID,
			DisplayID:         sequence,
			Name:              name,
			Description:       &description,
			TaskType:          taskType,
			WorkflowID:        workflow.Workflow.ID,
			WorkflowStateID:   state.ID,
			RequesterID:       seededAssignableMemberUserIDPtr(requesterMember),
			RequesterMemberID: seededAssignableMemberIDPtr(requesterMember),
			Estimate:          &estimate,
			Priority:          priority,
			Severity:          severity,
			Deadline:          deadline,
			Position:          position,
			Started:           started,
			StartedAt:         startedAt,
			Completed:         completed,
			CompletedAt:       completedAt,
			MovedAt:           movedAt,
			Blocked:           blocked,
			Blocker:           blocker,
			ExternalID:        &externalID,
			CreatedAt:         createdAt,
			UpdatedAt:         updatedAt,
		})
	}

	if err := s.taskRepo.CreateInBatches(ctx, tasks, 100); err != nil {
		return nil, err
	}

	for i := range tasks {
		ownerMember := seededTaskMember(activeMembers, i)
		if ownerMember == nil || ownerMember.UserID == nil || *ownerMember.UserID == "" {
			continue
		}
		if err := s.taskRepo.AddOwner(ctx, tasks[i].ID, *ownerMember.UserID); err != nil {
			return nil, err
		}
	}

	return &model.SeedPMTasksResponse{Created: len(tasks)}, nil
}

// Update updates task fields.
func (s *PMTaskService) Update(ctx context.Context, id string, req model.UpdateTaskRequest, actorID string) (*model.TaskDetail, error) {
	current, err := s.taskRepo.GetRawByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("task not found")
	}
	if err := requireTeamAccess(ctx, current.TeamID); err != nil {
		return nil, fmt.Errorf("task not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return nil, err
	}

	previousDetail, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if previousDetail == nil {
		return nil, fmt.Errorf("task not found")
	}

	stateChanged := false
	oldPriority := current.Priority
	oldSeverity := current.Severity
	oldTaskType := current.TaskType
	oldBlocked := current.Blocked
	oldEstimate := current.Estimate
	oldDeadline := current.Deadline
	oldBlocker := current.Blocker
	oldEpicID := stringValue(current.EpicID)

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

	if req.WorkflowID != nil || req.WorkflowStateID != nil {
		ok, err := s.workflowRepo.StateBelongsToWorkflow(ctx, stateID, workflowID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("workflow_state_id must belong to workflow_id")
		}
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
	teamChanged := req.TeamID != nil
	if req.EpicID != nil || teamChanged {
		if err := validateEpicScope(ctx, s.epicRepo, current.WorkspaceID, current.EpicID, current.TeamID); err != nil {
			return nil, err
		}
	}
	if req.SprintID != nil || teamChanged {
		if err := validateSprintScope(ctx, s.sprintRepo, current.WorkspaceID, current.SprintID, current.TeamID); err != nil {
			return nil, err
		}
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
	if req.Deadline != nil || req.DeadlineSet {
		current.Deadline = req.Deadline
	}
	if req.Position != nil {
		current.Position = *req.Position
	}
	if req.Blocked != nil {
		current.Blocked = *req.Blocked
	}
	if req.Blocker != nil || req.BlockerSet {
		current.Blocker = nullableString(req.Blocker)
		current.Blocked = req.Blocker != nil && strings.TrimSpace(*req.Blocker) != ""
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
	if req.AssignedAgentID != nil {
		nextAgentID := nullableString(req.AssignedAgentID)
		if nextAgentID != nil && s.agentService != nil {
			if err := s.agentService.ValidateRunnableTargetAgent(ctx, current.WorkspaceID, *nextAgentID, "task", current.TeamID); err != nil {
				return nil, err
			}
		}
		current.AssignedAgentID = nextAgentID
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

	var nextOwnerIDs []string
	var previousOwnerIDs []string
	ownerChangeRequested := req.OwnerMemberIDs != nil || req.OwnerIDs != nil
	if req.OwnerMemberIDs != nil || req.OwnerIDs != nil {
		nextOwnerIDs, err = s.resolveOwnerUserIDs(ctx, current.WorkspaceID, req.OwnerMemberIDs, req.OwnerIDs)
		if err != nil {
			return nil, err
		}
		previousOwnerIDs, err = s.taskRepo.ListOwnerUserIDs(ctx, current.ID)
		if err != nil {
			return nil, err
		}
	}
	var followers []string
	if req.FollowerIDs != nil {
		followers = dedupeIDs(req.FollowerIDs)
		if current.RequesterID != nil {
			followers = append(followers, *current.RequesterID)
		}
		followers = dedupeIDs(followers)
	}
	var labelIDs []string
	if req.LabelIDs != nil {
		labelIDs = dedupeIDs(req.LabelIDs)
		if err := validateLabelScope(ctx, s.labelRepo, current.WorkspaceID, labelIDs, allowedTeamIDs(current.TeamID)); err != nil {
			return nil, err
		}
	}
	if err := s.taskRepo.WithMutationTransaction(ctx, func(tasks *repository.PMTaskRepository, _ *repository.PMChecklistItemRepository) error {
		if err := tasks.Update(ctx, current); err != nil {
			return err
		}
		if ownerChangeRequested {
			if err := tasks.ReplaceOwners(ctx, current.ID, nextOwnerIDs); err != nil {
				return err
			}
			for _, ownerID := range nextOwnerIDs {
				if err := tasks.AddFollower(ctx, current.ID, ownerID); err != nil {
					return err
				}
			}
		}
		if req.FollowerIDs != nil {
			if err := tasks.ReplaceFollowers(ctx, current.ID, followers); err != nil {
				return err
			}
		}
		if req.LabelIDs != nil {
			if err := tasks.ReplaceLabels(ctx, current.ID, labelIDs); err != nil {
				return err
			}
		}
		if stateChanged {
			return tasks.UpdateStartedCompleted(ctx, current.ID)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if req.EpicID != nil {
		nextEpicID := stringValue(current.EpicID)
		if nextEpicID != "" && nextEpicID != oldEpicID {
			s.inheritEpicDeliveryTarget(ctx, current.WorkspaceID, current.ID, nextEpicID, actorID)
		}
	}
	var addedOwnerIDs []string
	if ownerChangeRequested {
		previousSet := stringSet(previousOwnerIDs)
		nextSet := stringSet(nextOwnerIDs)
		for _, ownerID := range nextOwnerIDs {
			if _, existed := previousSet[ownerID]; existed {
				continue
			}
			addedOwnerIDs = append(addedOwnerIDs, ownerID)
			if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "owner_added", stringPtr("owner"), nil, &ownerID, nil); err != nil {
				s.logger.ErrorContext(ctx, "failed to log activity for task owner add", "error", err, "task_id", current.ID)
			}
			if s.followerService != nil {
				if err := s.followerService.Follow(ctx, ownerID, "task", current.ID, current.WorkspaceID, "assigned"); err != nil {
					s.logger.ErrorContext(ctx, "failed to auto-follow task for assigned owner", "error", err, "task_id", current.ID, "user_id", ownerID)
				}
			}
			if s.notificationService != nil {
				if err := s.notificationService.Emit(ctx, model.NotificationEventInput{WorkspaceID: current.WorkspaceID, ActorID: actorID, EventType: "task.assigned", EntityType: "task", EntityID: current.ID, Title: "assigned you to " + current.Name, Category: "assignment", Priority: "normal", TeamID: derefString(current.TeamID), ExplicitRecipients: []string{ownerID}}); err != nil {
					s.logger.ErrorContext(ctx, "failed to emit notification for task assignment", "error", err, "task_id", current.ID, "user_id", ownerID)
				}
			}
		}
		for _, ownerID := range previousOwnerIDs {
			if _, retained := nextSet[ownerID]; retained {
				continue
			}
			if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "owner_removed", stringPtr("owner"), &ownerID, nil, nil); err != nil {
				s.logger.ErrorContext(ctx, "failed to log activity for task owner remove", "error", err, "task_id", current.ID)
			}
		}
	}
	if len(addedOwnerIDs) > 0 {
		s.trackProductEvent(ctx, ProductAnalyticsEvent{
			SemanticKey: fmt.Sprintf("task_assigned:%s:%d", current.ID, current.UpdatedAt.UnixNano()),
			UserID:      actorID, WorkspaceID: current.WorkspaceID, Name: "task_assigned", Source: "api", OccurredAt: current.UpdatedAt,
			Attributes: map[string]any{"entity_id": current.ID, "assigned_user_ids": addedOwnerIDs, "owner_count": len(nextOwnerIDs), "team_id": current.TeamID, "module": "pm"},
		})
	}

	if stateChanged {
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
		return nil, fmt.Errorf("task not found")
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
	if req.TaskType != nil && *req.TaskType != oldTaskType {
		action := "changed type from " + oldTaskType + " to " + *req.TaskType
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
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: current.ID, WorkspaceID: current.WorkspaceID, ActorID: actorID, TaskKey: model.FormatTaskKey(s.getWorkspaceKey(ctx, current.WorkspaceID), current.DisplayID)})

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
				"display_id": model.FormatTaskKey(s.getWorkspaceKey(ctx, current.WorkspaceID), current.DisplayID),
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
					"display_id": model.FormatTaskKey(s.getWorkspaceKey(ctx, current.WorkspaceID), current.DisplayID),
					"type":       current.TaskType,
				},
			}); err != nil {
				slog.ErrorContext(ctx, "failed to emit task mention notification", "error", err, "task_id", current.ID)
			}
		}
	}

	s.logger.InfoContext(ctx, "task updated", "task_id", current.ID, "workspace_id", current.WorkspaceID, "actor_id", actorID)
	s.populateTaskDetail(ctx, updatedDetail)
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

// Delete archives a task.
func (s *PMTaskService) Delete(ctx context.Context, id, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task not found")
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
	s.wsPublisher.Publish(websocket.Event{Action: "deleted", Entity: "task", EntityID: id, WorkspaceID: current.WorkspaceID, ActorID: actorID, TaskKey: model.FormatTaskKey(s.getWorkspaceKey(ctx, current.WorkspaceID), current.DisplayID)})
	s.logger.InfoContext(ctx, "task deleted", "task_id", id, "workspace_id", current.WorkspaceID, "actor_id", actorID)
	return nil
}

// MoveToState moves task to another state.
func (s *PMTaskService) MoveToState(ctx context.Context, id string, req model.MoveTaskRequest, actorID string) (*model.TaskDetail, error) {
	current, err := s.taskRepo.GetRawByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("task not found")
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
		return nil, fmt.Errorf("state_id must belong to task workflow")
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
				"display_id": model.FormatTaskKey(s.getWorkspaceKey(ctx, current.WorkspaceID), current.DisplayID),
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
	s.populateTaskDetail(ctx, detail)
	if detail != nil && !current.Completed && detail.Task.Completed {
		s.trackProductEvent(ctx, ProductAnalyticsEvent{
			SemanticKey: fmt.Sprintf("task_completed:%s:%d", detail.Task.ID, detail.Task.UpdatedAt.UnixNano()),
			UserID:      actorID, WorkspaceID: detail.Task.WorkspaceID,
			Name: "task_completed", Source: "api", OccurredAt: detail.Task.UpdatedAt,
			Attributes: map[string]any{"entity_id": detail.Task.ID, "workflow_state_id": detail.Task.WorkflowStateID, "task_type": detail.Task.TaskType, "team_id": detail.Task.TeamID, "module": "pm"},
		})
	}
	return detail, nil
}

// Reorder changes task position in its state.
func (s *PMTaskService) Reorder(ctx context.Context, id string, req model.ReorderTaskRequest, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task not found")
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
func (s *PMTaskService) AddOwner(ctx context.Context, taskID, userID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}
	if err := s.taskRepo.AddOwner(ctx, taskID, userID); err != nil {
		return err
	}
	if err := s.taskRepo.AddFollower(ctx, taskID, userID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "owner_added", stringPtr("owner"), nil, &userID, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task owner add", "error", err, "task_id", taskID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: taskID, WorkspaceID: current.WorkspaceID, ActorID: actorID, TaskKey: model.FormatTaskKey(s.getWorkspaceKey(ctx, current.WorkspaceID), current.DisplayID)})

	// Auto-follow and notify the assigned user.
	if s.followerService != nil {
		if err := s.followerService.Follow(ctx, userID, "task", taskID, current.WorkspaceID, "assigned"); err != nil {
			s.logger.ErrorContext(ctx, "failed to auto-follow task for assigned owner", "error", err, "task_id", taskID, "user_id", userID)
		}
	}
	if s.notificationService != nil {
		if err := s.notificationService.Emit(ctx, model.NotificationEventInput{
			WorkspaceID:        current.WorkspaceID,
			ActorID:            actorID,
			EventType:          "task.assigned",
			EntityType:         "task",
			EntityID:           taskID,
			Title:              "assigned you to " + current.Name,
			Category:           "assignment",
			Priority:           "normal",
			TeamID:             derefString(current.TeamID),
			ExplicitRecipients: []string{userID},
			EntitySnapshot: model.JSONB{
				"title":      current.Name,
				"display_id": model.FormatTaskKey(s.getWorkspaceKey(ctx, current.WorkspaceID), current.DisplayID),
				"type":       current.TaskType,
			},
		}); err != nil {
			s.logger.ErrorContext(ctx, "failed to emit notification for task assignment", "error", err, "task_id", taskID, "user_id", userID)
		}
	}

	return nil
}

// RemoveOwner removes an owner.
func (s *PMTaskService) RemoveOwner(ctx context.Context, taskID, userID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.taskRepo.RemoveOwner(ctx, taskID, userID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "owner_removed", stringPtr("owner"), &userID, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task owner remove", "error", err, "task_id", taskID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: taskID, WorkspaceID: current.WorkspaceID, ActorID: actorID, TaskKey: model.FormatTaskKey(s.getWorkspaceKey(ctx, current.WorkspaceID), current.DisplayID)})
	return nil
}

// AddFollower adds a follower.
func (s *PMTaskService) AddFollower(ctx context.Context, taskID, userID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}
	if err := s.taskRepo.AddFollower(ctx, taskID, userID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "follower_added", stringPtr("follower"), nil, &userID, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task follower add", "error", err, "task_id", taskID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: taskID, WorkspaceID: current.WorkspaceID, ActorID: actorID, TaskKey: model.FormatTaskKey(s.getWorkspaceKey(ctx, current.WorkspaceID), current.DisplayID)})
	return nil
}

// RemoveFollower removes a follower.
func (s *PMTaskService) RemoveFollower(ctx context.Context, taskID, userID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.taskRepo.RemoveFollower(ctx, taskID, userID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "follower_removed", stringPtr("follower"), &userID, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task follower remove", "error", err, "task_id", taskID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: taskID, WorkspaceID: current.WorkspaceID, ActorID: actorID, TaskKey: model.FormatTaskKey(s.getWorkspaceKey(ctx, current.WorkspaceID), current.DisplayID)})
	return nil
}

// AddLabel adds a label to a task.
func (s *PMTaskService) AddLabel(ctx context.Context, taskID, labelID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := validateLabelScope(ctx, s.labelRepo, current.WorkspaceID, []string{labelID}, allowedTeamIDs(current.TeamID)); err != nil {
		return err
	}
	if err := s.taskRepo.AddLabel(ctx, taskID, labelID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "label_added", stringPtr("label"), nil, &labelID, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task label add", "error", err, "task_id", taskID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: taskID, WorkspaceID: current.WorkspaceID, ActorID: actorID})
	return nil
}

// RemoveLabel removes a label from a task.
func (s *PMTaskService) RemoveLabel(ctx context.Context, taskID, labelID, actorID string) error {
	current, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task not found")
	}
	if err := s.requireCanEdit(ctx, current.WorkspaceID, actorID); err != nil {
		return err
	}
	if err := s.taskRepo.RemoveLabel(ctx, taskID, labelID); err != nil {
		return err
	}
	if err := s.activityService.Log(ctx, current.WorkspaceID, "task", current.ID, optionalActor(actorID), "label_removed", stringPtr("label"), &labelID, nil, nil); err != nil {
		s.logger.ErrorContext(ctx, "failed to log activity for task label remove", "error", err, "task_id", taskID)
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "task", EntityID: taskID, WorkspaceID: current.WorkspaceID, ActorID: actorID, TaskKey: model.FormatTaskKey(s.getWorkspaceKey(ctx, current.WorkspaceID), current.DisplayID)})
	return nil
}

// ListByWorkflowState returns board columns for a workflow with optional filters.
// perStateLimit controls how many tasks per column (0 = unlimited).
func (s *PMTaskService) ListByWorkflowState(ctx context.Context, workflowID string, filters model.PMTaskFilters, perStateLimit int) ([]model.TaskStateColumn, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	columns, err := s.taskRepo.ListByWorkflowState(ctx, workflowID, filters, perStateLimit)
	if err != nil {
		return nil, err
	}
	s.populateStateColumns(ctx, columns)
	return columns, nil
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
	tasks, groups, total, err := s.taskRepo.ListColumnTasks(ctx, stateID, filters, offset, limit)
	if err != nil {
		return nil, nil, 0, err
	}
	if len(tasks) > 0 {
		s.populateBoardTasks(ctx, tasks[0].WorkspaceID, tasks)
	}
	return tasks, groups, total, nil
}

// ListByMember returns board columns grouped by owner member.
func (s *PMTaskService) ListByMember(ctx context.Context, workspaceID, workflowID string, filters model.PMTaskFilters, perMemberLimit int, includeEmpty bool, memberIDs []string) ([]model.TaskMemberColumn, error) {
	if workspaceID == "" || workflowID == "" {
		return nil, fmt.Errorf("workspace_id and workflow_id are required")
	}
	filters.AccessibleTeamIDs = accessibleTeamIDs(ctx)
	columns, err := s.taskRepo.ListByMember(ctx, workspaceID, workflowID, filters, perMemberLimit, includeEmpty, memberIDs)
	if err != nil {
		return nil, err
	}
	s.populateMemberColumns(ctx, workspaceID, columns)
	return columns, nil
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
	tasks, total, err := s.taskRepo.ListMemberColumnTasks(ctx, workspaceID, workflowID, memberID, filters, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	s.populateBoardTasks(ctx, workspaceID, tasks)
	return tasks, total, nil
}

// CountByState returns state-level task counts for a workflow.
func (s *PMTaskService) CountByState(ctx context.Context, workflowID string) ([]model.TaskStateCount, error) {
	if workflowID == "" {
		return nil, fmt.Errorf("workflow_id is required")
	}
	return s.taskRepo.CountByState(ctx, workflowID)
}

// ListActivity returns task activity entries.
func (s *PMTaskService) ListActivity(ctx context.Context, taskID string, pagination model.PMPagination) ([]model.ActivityLogEntry, int64, error) {
	return s.activityService.ListEntity(ctx, "task", taskID, pagination)
}

func seededTaskWorkflowState(states []model.PMWorkflowState, index int) model.PMWorkflowState {
	if len(states) == 0 {
		return model.PMWorkflowState{}
	}
	return states[index%len(states)]
}

func seededTaskType(index int) string {
	types := []string{
		model.PMTaskTypeFeature,
		model.PMTaskTypeBug,
		model.PMTaskTypeChore,
	}
	return types[index%len(types)]
}

func seededTaskPriority(index int) string {
	priorities := []string{
		model.PMTaskPriorityLow,
		model.PMTaskPriorityMedium,
		model.PMTaskPriorityHigh,
		model.PMTaskPriorityUrgent,
		model.PMTaskPriorityNone,
	}
	return priorities[index%len(priorities)]
}

func seededTaskSeverity(index int) string {
	severities := []string{
		model.PMTaskSeverityNone,
		model.PMTaskSeverityMinor,
		model.PMTaskSeverityMajor,
		model.PMTaskSeverityCritical,
	}
	return severities[index%len(severities)]
}

func seededTaskEstimate(index int) int {
	estimates := []int{1, 2, 3, 5, 8, 13}
	return estimates[index%len(estimates)]
}

func seededTaskName(sequence int, taskType string) string {
	prefix := map[string]string{
		model.PMTaskTypeFeature: "Seeded feature",
		model.PMTaskTypeBug:     "Seeded bug",
		model.PMTaskTypeChore:   "Seeded chore",
	}
	return fmt.Sprintf("%s %d", prefix[taskType], sequence)
}

func seededTaskDescription(sequence int, taskType, stateName string) string {
	return fmt.Sprintf("Synthetic %s task %d seeded for workspace testing in %s.", taskType, sequence, stateName)
}

func seededTaskBlocker(index int) string {
	blockers := []string{
		"Waiting on API contract confirmation",
		"Pending customer feedback on scope",
		"Dependency deployment has not completed",
		"Needs design approval before implementation",
	}
	return blockers[index%len(blockers)]
}

func seededTaskMember(members []model.AssignableMember, index int) *model.AssignableMember {
	if len(members) == 0 {
		return nil
	}
	member := members[index%len(members)]
	return &member
}

func seededAssignableMemberIDPtr(member *model.AssignableMember) *string {
	if member == nil || strings.TrimSpace(member.ID) == "" {
		return nil
	}
	id := member.ID
	return &id
}

func seededAssignableMemberUserIDPtr(member *model.AssignableMember) *string {
	if member == nil || member.UserID == nil || strings.TrimSpace(*member.UserID) == "" {
		return nil
	}
	id := *member.UserID
	return &id
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

func (s *PMTaskService) resolveOwnerUserIDs(ctx context.Context, workspaceID string, ownerMemberIDs, ownerIDs []string) ([]string, error) {
	members, err := resolveWorkspaceMemberReferences(ctx, s.workspaceRepo, workspaceID, ownerMemberIDs, ownerIDs)
	if err != nil {
		return nil, err
	}
	userIDs := make([]string, 0, len(members))
	for _, member := range members {
		if member == nil {
			continue
		}
		if member.UserID == nil || strings.TrimSpace(*member.UserID) == "" {
			return nil, fmt.Errorf("workspace member %s does not have an active user", member.ID)
		}
		userIDs = append(userIDs, *member.UserID)
	}
	return dedupeIDs(userIDs), nil
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		set[value] = struct{}{}
	}
	return set
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
