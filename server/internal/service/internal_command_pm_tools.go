package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var pmDiscoveryTargets = []string{"workspace", "task", "story", "epic", "sprint", "objective"}

type pmCommandPage struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

func (p pmCommandPage) normalized() (int, int) {
	page := p.Page
	if page <= 0 {
		page = 1
	}
	perPage := p.PerPage
	if perPage <= 0 {
		perPage = 50
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}

func boundedPMCommandPage[T any](values []T, page, perPage int) []T {
	if page <= 0 || perPage <= 0 || page-1 > math.MaxInt/perPage {
		return []T{}
	}
	start := (page - 1) * perPage
	if start >= len(values) {
		return []T{}
	}
	end := start + perPage
	if end > len(values) {
		end = len(values)
	}
	return values[start:end]
}

func (s *InternalCommandService) registerPMOperationalCommands() {
	if s == nil {
		return
	}
	s.registerPMDiscoveryCommands()
	s.registerPMTaskCommands()
	s.registerPMEpicCommands()
	s.registerPMSprintCommands()
	s.registerPMObjectiveCommands()
	s.extendExistingPMTaskCommands()
}

func (s *InternalCommandService) registerPMDiscoveryCommands() {
	s.register(InternalCommandDefinition{
		Name:                   "workspace.list_members",
		Module:                 "workspace",
		Mutating:               false,
		SupportedTargetTypes:   pmDiscoveryTargets,
		RequiredPermissionsAll: []authorization.Permission{authorization.PermWorkspaceRead, authorization.PermPMRead},
		Tool:                   mustCommandToolMetadata("workspace.list_members"),
		Execute:                s.executeListWorkspaceMembers,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.list_labels",
		Module:               "pm",
		Mutating:             false,
		SupportedTargetTypes: pmDiscoveryTargets,
		Tool:                 mustCommandToolMetadata("pm.list_labels"),
		Execute:              s.executeListPMLabels,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.list_team_workflows_with_stages",
		Module:               "pm",
		Mutating:             false,
		SupportedTargetTypes: pmDiscoveryTargets,
		Tool:                 mustCommandToolMetadata("pm.list_team_workflows_with_stages"),
		Execute:              s.executeListPMTeamWorkflows,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.list_tasks",
		Module:               "pm",
		Mutating:             false,
		SupportedTargetTypes: pmDiscoveryTargets,
		Tool:                 mustCommandToolMetadata("pm.list_tasks"),
		Execute:              s.executeListPMTasks,
	})
}

func (s *InternalCommandService) executeListWorkspaceMembers(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.workspaceRepo == nil {
		return nil, fmt.Errorf("workspace repository is not configured")
	}
	var req pmCommandPage
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse list workspace members input: %w", err)
		}
	}
	members, err := s.workspaceRepo.ListAssignableMembers(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	teamsByUser, err := s.workspaceRepo.ListActiveTeamIDsByUser(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(members))
	for _, member := range members {
		if member.Status != model.WorkspaceMemberStatusActive || member.UserID == nil || strings.TrimSpace(*member.UserID) == "" {
			continue
		}
		teamIDs := teamsByUser[strings.TrimSpace(*member.UserID)]
		sort.Strings(teamIDs)
		rows = append(rows, map[string]any{
			"member_id":    member.ID,
			"user_id":      strings.TrimSpace(*member.UserID),
			"display_name": member.DisplayName,
			"role":         member.Role,
			"team_ids":     teamIDs,
		})
	}
	page, perPage := req.normalized()
	return mustJSON(map[string]any{
		"members":  boundedPMCommandPage(rows, page, perPage),
		"total":    len(rows),
		"page":     page,
		"per_page": perPage,
	}), nil
}

func (s *InternalCommandService) executeListPMLabels(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.labelService == nil {
		return nil, fmt.Errorf("label service is not configured")
	}
	var req struct {
		pmCommandPage
		TeamID string `json:"team_id"`
		Name   string `json:"name"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse list labels input: %w", err)
		}
	}
	teamID := strings.TrimSpace(req.TeamID)
	agentTeams, err := commandAgentTeamFilter(meta)
	if err != nil {
		return nil, err
	}
	if teamID != "" && !canAccessTeam(ctx, &teamID) {
		return nil, fmt.Errorf("team not found")
	}
	labels, err := s.labelService.ListByWorkspace(ctx, meta.WorkspaceID, stringPtrOrNil(teamID), true)
	if err != nil {
		return nil, err
	}
	nameFilter := strings.ToLower(strings.TrimSpace(req.Name))
	rows := make([]map[string]any, 0, len(labels))
	for _, label := range labels {
		if label.Archived {
			continue
		}
		if label.TeamID != nil && !canAccessTeam(ctx, label.TeamID) {
			continue
		}
		if label.TeamID != nil && len(agentTeams) > 0 && !containsCommandTeam(agentTeams, label.TeamID) {
			continue
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(label.Name), nameFilter) {
			continue
		}
		rows = append(rows, map[string]any{
			"label_id": label.ID,
			"name":     label.Name,
			"team_id":  label.TeamID,
			"color":    label.Color,
		})
	}
	page, perPage := req.normalized()
	return mustJSON(map[string]any{
		"labels":   boundedPMCommandPage(rows, page, perPage),
		"total":    len(rows),
		"page":     page,
		"per_page": perPage,
	}), nil
}

func (s *InternalCommandService) executeListPMTeamWorkflows(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.workflowService == nil || s.workspaceRepo == nil {
		return nil, fmt.Errorf("workflow service is not configured")
	}
	var req struct {
		pmCommandPage
		TeamID string `json:"team_id"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse list team workflows input: %w", err)
		}
	}
	requestedTeamID := strings.TrimSpace(req.TeamID)
	agentTeams, err := commandAgentTeamFilter(meta)
	if err != nil {
		return nil, err
	}
	if requestedTeamID != "" && len(agentTeams) > 0 && !containsCommandTeam(agentTeams, &requestedTeamID) {
		return nil, fmt.Errorf("team not found")
	}
	teams, err := s.workspaceRepo.ListTeams(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	workflows, err := s.workflowService.ListByWorkspace(ctx, meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(teams))
	for _, team := range teams {
		if requestedTeamID != "" && team.ID != requestedTeamID {
			continue
		}
		if !canAccessTeam(ctx, &team.ID) {
			continue
		}
		if len(agentTeams) > 0 && !containsCommandTeam(agentTeams, &team.ID) {
			continue
		}
		workflow := resolvedCommandTeamWorkflow(workflows, team.ID)
		if workflow == nil || workflow.Workflow.WorkspaceID != meta.WorkspaceID {
			continue
		}
		states := make([]map[string]any, 0, len(workflow.States))
		for _, state := range workflow.States {
			states = append(states, map[string]any{
				"state_id": state.ID,
				"name":     state.Name,
				"type":     state.StateType,
				"position": state.Position,
				"default":  state.IsDefault,
			})
		}
		rows = append(rows, map[string]any{
			"team_id":          team.ID,
			"team_name":        team.Name,
			"workflow_id":      workflow.Workflow.ID,
			"workflow_name":    workflow.Workflow.Name,
			"default_state_id": workflow.Workflow.DefaultStateID,
			"states":           states,
		})
	}
	if requestedTeamID != "" && len(rows) == 0 {
		return nil, fmt.Errorf("team not found")
	}
	page, perPage := req.normalized()
	return mustJSON(map[string]any{
		"workflows": boundedPMCommandPage(rows, page, perPage),
		"total":     len(rows),
		"page":      page,
		"per_page":  perPage,
	}), nil
}

func resolvedCommandTeamWorkflow(workflows []model.WorkflowWithStates, teamID string) *model.WorkflowWithStates {
	var fallback *model.WorkflowWithStates
	for index := range workflows {
		workflow := &workflows[index]
		if workflow.Workflow.TeamID != nil && strings.TrimSpace(*workflow.Workflow.TeamID) == teamID {
			return workflow
		}
		if workflow.Workflow.TeamID == nil || strings.TrimSpace(*workflow.Workflow.TeamID) == "" {
			if fallback == nil {
				fallback = workflow
			}
		}
	}
	return fallback
}

func (s *InternalCommandService) executeListPMTasks(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.taskService == nil {
		return nil, fmt.Errorf("task service is not configured")
	}
	var req struct {
		pmCommandPage
		LabelID             string   `json:"label_id"`
		TeamID              string   `json:"team_id"`
		EpicID              string   `json:"epic_id"`
		SprintID            string   `json:"sprint_id"`
		WorkflowID          string   `json:"workflow_id"`
		StateID             string   `json:"state_id"`
		TaskType            string   `json:"task_type"`
		Priority            string   `json:"priority"`
		Severity            string   `json:"severity"`
		Completed           *bool    `json:"completed"`
		Archived            *bool    `json:"archived"`
		UpdatedAfter        string   `json:"updated_after"`
		TaskID              string   `json:"task_id"`
		OwnerMemberIDs      []string `json:"owner_member_ids"`
		OwnedByActor        bool     `json:"owned_by_actor"`
		OpenOnly            bool     `json:"open_only"`
		IncludeDescriptions bool     `json:"include_descriptions"`
		IncludeComments     bool     `json:"include_comments"`
		DetailLevel         string   `json:"detail_level"`
		Limit               int      `json:"limit"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse list tasks input: %w", err)
		}
	}
	req.TaskID = strings.TrimSpace(firstNonEmptyCommand(req.TaskID, commandTaskTargetID(meta)))
	if strings.TrimSpace(meta.TargetType) == "epic" && strings.TrimSpace(req.EpicID) == "" {
		req.EpicID = strings.TrimSpace(meta.TargetID)
	}
	if strings.TrimSpace(meta.TargetType) == "sprint" && strings.TrimSpace(req.SprintID) == "" {
		req.SprintID = strings.TrimSpace(meta.TargetID)
	}
	if req.TaskID != "" {
		return s.listSingleTaskCommand(ctx, meta, req.TaskID, req.OpenOnly, req.IncludeDescriptions)
	}
	req.OwnerMemberIDs = commandTrimStringSlice(req.OwnerMemberIDs)
	if req.OwnedByActor {
		workspaceRepo := s.workspaceRepo
		if workspaceRepo == nil && s.taskService != nil {
			workspaceRepo = s.taskService.workspaceRepo
		}
		if workspaceRepo == nil {
			return nil, fmt.Errorf("owned_by_actor requires workspace membership lookup")
		}
		member, err := workspaceRepo.GetMembership(ctx, meta.WorkspaceID, meta.ActorID)
		if err != nil {
			return nil, err
		}
		if member == nil {
			return nil, fmt.Errorf("owned_by_actor requires an active workspace member actor")
		}
		req.OwnerMemberIDs = commandTrimStringSlice(append(req.OwnerMemberIDs, member.ID))
	}
	if strings.TrimSpace(req.UpdatedAfter) != "" {
		if _, err := time.Parse("2006-01-02", strings.TrimSpace(req.UpdatedAfter)); err != nil {
			return nil, fmt.Errorf("updated_after must be YYYY-MM-DD")
		}
	}
	page, perPage := req.normalized()
	if req.Limit > 0 && req.PerPage <= 0 {
		perPage = req.Limit
		if perPage > 100 {
			perPage = 100
		}
	}
	archived := req.Archived
	completed := req.Completed
	if archived == nil {
		includeArchived := false
		archived = &includeArchived
	}
	if req.OpenOnly {
		open := false
		completed = &open
		archived = &open
	}
	filters := model.PMTaskFilters{
		LabelID:         stringPtrOrNil(req.LabelID),
		TeamID:          stringPtrOrNil(req.TeamID),
		EpicID:          stringPtrOrNil(req.EpicID),
		SprintID:        stringPtrOrNil(req.SprintID),
		WorkflowID:      stringPtrOrNil(req.WorkflowID),
		WorkflowStateID: stringPtrOrNil(req.StateID),
		TaskType:        stringPtrOrNil(req.TaskType),
		OwnerMemberIDs:  req.OwnerMemberIDs,
		Priority:        stringPtrOrNil(req.Priority),
		Severity:        stringPtrOrNil(req.Severity),
		Completed:       completed,
		UpdatedAfter:    stringPtrOrNil(req.UpdatedAfter),
		Archived:        archived,
	}
	if agentTeams, err := commandAgentTeamFilter(meta); err != nil {
		return nil, err
	} else if len(agentTeams) > 0 {
		filters.AccessibleTeamIDs = agentTeams
	}
	tasks, total, err := s.taskService.List(ctx, meta.WorkspaceID, filters, model.PMPagination{Page: page, PerPage: perPage})
	if err != nil {
		return nil, err
	}
	commentsByTask := map[string][]model.CommentWithAuthor{}
	detailLevel := strings.ToLower(strings.TrimSpace(req.DetailLevel))
	if detailLevel != "" && detailLevel != "summary" && detailLevel != "compact" && detailLevel != "full" {
		return nil, fmt.Errorf("detail_level must be summary, compact, or full")
	}
	if detailLevel == "compact" || req.IncludeComments {
		if s.commentService == nil {
			return nil, fmt.Errorf("comment service is not configured")
		}
		taskIDs := make([]string, 0, len(tasks))
		for _, task := range tasks {
			taskIDs = append(taskIDs, task.ID)
		}
		commentsByTask, err = s.commentService.ListByEntityIDs(ctx, "task", taskIDs)
		if err != nil {
			return nil, err
		}
	}
	rows := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		if detailLevel == "compact" {
			rows = append(rows, buildCompactTaskItem(task, commentsByTask[task.ID]))
			continue
		}
		row := compactCommandBoardTask(task)
		if req.IncludeDescriptions {
			row["description"] = task.Description
		}
		if req.IncludeComments {
			row["comments"] = compactTaskComments(commentsByTask[task.ID], 10)
		}
		rows = append(rows, row)
	}
	if detailLevel == "compact" {
		return marshalCompactTaskResponse(rows, total, perPage)
	}
	return mustJSON(map[string]any{
		"tasks":        rows,
		"total":        total,
		"page":         page,
		"per_page":     perPage,
		"detail_level": detailLevel,
	}), nil
}

func compactCommandBoardTask(task model.BoardTask) map[string]any {
	return map[string]any{
		"task_id":          task.ID,
		"display_id":       task.DisplayID,
		"task_key":         task.TaskKey,
		"name":             task.Name,
		"task_type":        task.TaskType,
		"team_id":          task.TeamID,
		"workflow_id":      task.WorkflowID,
		"state_id":         task.WorkflowStateID,
		"state_name":       task.StateName,
		"epic_id":          task.EpicID,
		"sprint_id":        task.SprintID,
		"owner_member_ids": task.OwnerMemberIDs,
		"completed":        task.Completed,
		"archived":         task.Archived,
		"priority":         task.Priority,
		"severity":         task.Severity,
		"blocked":          task.Blocked,
		"blocker":          task.Blocker,
		"deadline":         task.Deadline,
		"labels":           task.Labels,
		"updated_at":       task.UpdatedAt,
	}
}

func (s *InternalCommandService) registerPMTaskCommands() {
	s.register(InternalCommandDefinition{
		Name:                 "pm.create_task",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "epic", "sprint"},
		Tool:                 mustCommandToolMetadata("pm.create_task"),
		Execute:              s.executeCreatePMTask,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.get_task",
		Module:               "pm",
		Mutating:             false,
		SupportedTargetTypes: pmDiscoveryTargets,
		Tool:                 mustCommandToolMetadata("pm.get_task"),
		Execute:              s.executeGetPMTask,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.update_task",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "task", "story", "epic", "sprint"},
		Tool:                 mustCommandToolMetadata("pm.update_task"),
		Execute:              s.executeUpdatePMTask,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.list_task_checklist",
		Module:               "pm",
		Mutating:             false,
		SupportedTargetTypes: []string{"workspace", "task", "story", "epic", "sprint"},
		Tool:                 mustCommandToolMetadata("pm.list_task_checklist"),
		Execute:              s.executeListTaskChecklist,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.create_task_checklist_item",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "task", "story", "epic", "sprint"},
		Tool:                 mustCommandToolMetadata("pm.create_task_checklist_item"),
		Execute:              s.executeCreateTaskChecklistItem,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.update_task_checklist_item",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "task", "story", "epic", "sprint"},
		Tool:                 mustCommandToolMetadata("pm.update_task_checklist_item"),
		Execute:              s.executeUpdateTaskChecklistItem,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.add_comment",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "task", "story", "epic", "sprint", "objective"},
		Tool:                 mustCommandToolMetadata("pm.add_comment"),
		Execute:              s.executeAddPMComment,
	})
}

func (s *InternalCommandService) extendExistingPMTaskCommands() {
	s.register(InternalCommandDefinition{
		Name:                 "pm.update_task_state",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "task", "story", "epic", "sprint"},
		Tool:                 mustCommandToolMetadata("pm.update_task_state"),
		Execute:              s.executeUpdatePMTaskState,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.add_task_comment",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "task", "story", "epic", "sprint"},
		Tool:                 mustCommandToolMetadata("pm.add_task_comment"),
		Execute:              s.executeAddTaskComment,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.set_task_dependencies",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "epic", "sprint"},
		Tool:                 mustCommandToolMetadata("pm.set_task_dependencies"),
		Execute:              s.executeSetPMTaskDependencies,
	})

	if def, ok := s.definitions["release.get_task_context"]; ok {
		def.SupportedTargetTypes = pmDiscoveryTargets
		s.definitions[def.Name] = def
	}
	if def, ok := s.definitions["workspace.list_teams"]; ok {
		def.SupportedTargetTypes = pmDiscoveryTargets
		s.definitions[def.Name] = def
	}
}

func (s *InternalCommandService) executeCreatePMTask(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.taskService == nil {
		return nil, fmt.Errorf("task service is not configured")
	}
	var req struct {
		Name           string   `json:"name"`
		Description    *string  `json:"description"`
		TaskType       string   `json:"task_type"`
		Estimate       *int     `json:"estimate"`
		Priority       *string  `json:"priority"`
		Severity       *string  `json:"severity"`
		EpicID         *string  `json:"epic_id"`
		SprintID       *string  `json:"sprint_id"`
		TeamID         string   `json:"team_id"`
		WorkflowID     *string  `json:"workflow_id"`
		StateID        *string  `json:"state_id"`
		OwnerMemberIDs []string `json:"owner_member_ids"`
		LabelIDs       []string `json:"label_ids"`
		Deadline       *string  `json:"deadline"`
		Blocked        *bool    `json:"blocked"`
		Blocker        *string  `json:"blocker"`
		ChecklistItems []struct {
			Text       string  `json:"text"`
			Position   *int    `json:"position"`
			AssigneeID *string `json:"assignee_id"`
			DueDate    *string `json:"due_date"`
		} `json:"checklist_items"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse create task input: %w", err)
	}
	req.Name = strings.TrimSpace(req.Name)
	req.TeamID = strings.TrimSpace(req.TeamID)
	if req.Name == "" || req.TeamID == "" {
		return nil, fmt.Errorf("name and team_id are required")
	}
	if err := requireCommandAgentTeam(meta, &req.TeamID); err != nil {
		return nil, err
	}
	epicID, err := resolveCommandParentAssociation(meta, commandDerefString(req.EpicID), "epic")
	if err != nil {
		return nil, err
	}
	sprintID, err := resolveCommandParentAssociation(meta, commandDerefString(req.SprintID), "sprint")
	if err != nil {
		return nil, err
	}
	if err := s.validateCommandAgentParentScope(ctx, meta, epicID, sprintID); err != nil {
		return nil, err
	}
	var deadline *time.Time
	if req.Deadline != nil {
		deadline, err = parseStrictPMCommandDate(*req.Deadline, "deadline", true)
		if err != nil {
			return nil, err
		}
	}
	workflowID, stateID, err := s.resolveTaskCreationWorkflow(ctx, meta.WorkspaceID, req.TeamID, stringPtrOrNil(commandDerefString(req.WorkflowID)), stringPtrOrNil(commandDerefString(req.StateID)))
	if err != nil {
		return nil, err
	}
	checklistItems := make([]model.CreateChecklistItemRequest, 0, len(req.ChecklistItems))
	for index, item := range req.ChecklistItems {
		var dueDate *time.Time
		if item.DueDate != nil {
			dueDate, err = parseStrictPMCommandDate(*item.DueDate, fmt.Sprintf("checklist_items[%d].due_date", index), false)
			if err != nil {
				return nil, err
			}
		}
		if err := s.validateChecklistAssignee(ctx, meta.WorkspaceID, item.AssigneeID, false); err != nil {
			return nil, fmt.Errorf("checklist_items[%d]: %w", index, err)
		}
		checklistItems = append(checklistItems, model.CreateChecklistItemRequest{Text: item.Text, Position: item.Position, AssigneeID: normalizeOptionalCommandString(item.AssigneeID), DueDate: dueDate})
	}
	createReq := model.CreateTaskRequest{
		WorkspaceID:     meta.WorkspaceID,
		Name:            req.Name,
		Description:     normalizeTaskDescriptionRichText(req.Description),
		TaskType:        strings.TrimSpace(req.TaskType),
		WorkflowID:      workflowID,
		WorkflowStateID: stateID,
		EpicID:          stringPtrOrNil(epicID),
		SprintID:        stringPtrOrNil(sprintID),
		TeamID:          stringPtrOrNil(req.TeamID),
		OwnerMemberIDs:  commandTrimStringSlice(req.OwnerMemberIDs),
		Estimate:        req.Estimate,
		Priority:        normalizeOptionalCommandString(req.Priority),
		Severity:        normalizeOptionalCommandString(req.Severity),
		Deadline:        deadline,
		Blocked:         req.Blocked,
		Blocker:         normalizeOptionalCommandString(req.Blocker),
		LabelIDs:        commandTrimStringSlice(req.LabelIDs),
		ChecklistItems:  checklistItems,
	}
	detail, err := s.taskService.Create(ctx, createReq, fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCommandTask(detail)), nil
}

func (s *InternalCommandService) executeGetPMTask(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.taskService == nil {
		return nil, fmt.Errorf("task service is not configured")
	}
	var req struct {
		TaskID string `json:"task_id"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse get task input: %w", err)
		}
	}
	taskID, err := resolveCommandEntityID(meta, req.TaskID, "task")
	if err != nil {
		return nil, err
	}
	detail, err := s.taskService.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if detail.Task.WorkspaceID != meta.WorkspaceID {
		return nil, fmt.Errorf("task not found")
	}
	if err := s.validateTaskWithinTarget(ctx, meta, &detail.Task); err != nil {
		return nil, err
	}
	return mustJSON(compactCommandTask(detail)), nil
}

func (s *InternalCommandService) executeUpdatePMTask(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.taskService == nil {
		return nil, fmt.Errorf("task service is not configured")
	}
	var req struct {
		TaskID         string   `json:"task_id"`
		Name           *string  `json:"name"`
		Description    *string  `json:"description"`
		TaskType       *string  `json:"task_type"`
		EpicID         *string  `json:"epic_id"`
		SprintID       *string  `json:"sprint_id"`
		OwnerMemberIDs []string `json:"owner_member_ids"`
		Estimate       *int     `json:"estimate"`
		Priority       *string  `json:"priority"`
		Severity       *string  `json:"severity"`
		Deadline       *string  `json:"deadline"`
		Blocked        *bool    `json:"blocked"`
		Blocker        *string  `json:"blocker"`
		LabelIDs       []string `json:"label_ids"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse update task input: %w", err)
	}
	if req.Name == nil && req.Description == nil && req.TaskType == nil && req.EpicID == nil && req.SprintID == nil && req.OwnerMemberIDs == nil && req.Estimate == nil && req.Priority == nil && req.Severity == nil && req.Deadline == nil && req.Blocked == nil && req.Blocker == nil && req.LabelIDs == nil {
		return nil, fmt.Errorf("at least one editable field is required")
	}
	taskID, err := resolveCommandEntityID(meta, req.TaskID, "task")
	if err != nil {
		return nil, err
	}
	current, err := s.taskService.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if current.Task.WorkspaceID != meta.WorkspaceID {
		return nil, fmt.Errorf("task not found")
	}
	if err := requireCommandAgentTeam(meta, current.Task.TeamID); err != nil {
		return nil, err
	}
	if err := s.validateTaskWithinTarget(ctx, meta, &current.Task); err != nil {
		return nil, err
	}
	if err := validateCommandParentUpdate(meta, "epic", req.EpicID); err != nil {
		return nil, err
	}
	if err := validateCommandParentUpdate(meta, "sprint", req.SprintID); err != nil {
		return nil, err
	}
	if req.EpicID != nil || req.SprintID != nil {
		if err := s.validateCommandAgentParentScope(ctx, meta, commandDerefString(req.EpicID), commandDerefString(req.SprintID)); err != nil {
			return nil, err
		}
	}
	updateReq := model.UpdateTaskRequest{
		Name:           normalizeOptionalCommandString(req.Name),
		Description:    normalizeTaskDescriptionRichText(req.Description),
		TaskType:       normalizeOptionalCommandString(req.TaskType),
		EpicID:         normalizeClearableCommandString(req.EpicID),
		SprintID:       normalizeClearableCommandString(req.SprintID),
		OwnerMemberIDs: trimOptionalCommandSlice(req.OwnerMemberIDs),
		Estimate:       req.Estimate,
		Priority:       normalizeOptionalCommandString(req.Priority),
		Severity:       normalizeOptionalCommandString(req.Severity),
		Blocked:        req.Blocked,
		Blocker:        normalizeClearableCommandString(req.Blocker),
		BlockerSet:     req.Blocker != nil,
		LabelIDs:       trimOptionalCommandSlice(req.LabelIDs),
	}
	if req.Deadline != nil {
		updateReq.Deadline, err = parseStrictPMCommandDate(*req.Deadline, "deadline", true)
		if err != nil {
			return nil, err
		}
		updateReq.DeadlineSet = true
	}
	updated, err := s.taskService.Update(ctx, taskID, updateReq, fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCommandTask(updated)), nil
}

func (s *InternalCommandService) executeListTaskChecklist(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.checklistService == nil || s.taskService == nil {
		return nil, fmt.Errorf("checklist service is not configured")
	}
	var req struct {
		TaskID string `json:"task_id"`
		Limit  int    `json:"limit"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse list checklist input: %w", err)
		}
	}
	task, err := s.resolveWritableCommandTask(ctx, meta, req.TaskID)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	items, total, hasMore, err := s.checklistService.ListBounded(ctx, task.ID, meta.WorkspaceID, limit)
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"task_id": task.ID, "items": items, "total": total, "has_more": hasMore}), nil
}

func (s *InternalCommandService) executeCreateTaskChecklistItem(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.checklistService == nil || s.taskService == nil {
		return nil, fmt.Errorf("checklist service is not configured")
	}
	var req struct {
		TaskID     string  `json:"task_id"`
		Text       string  `json:"text"`
		Position   *int    `json:"position"`
		AssigneeID *string `json:"assignee_id"`
		DueDate    *string `json:"due_date"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse create checklist item input: %w", err)
	}
	task, err := s.resolveWritableCommandTask(ctx, meta, req.TaskID)
	if err != nil {
		return nil, err
	}
	if err := s.validateChecklistAssignee(ctx, meta.WorkspaceID, req.AssigneeID, false); err != nil {
		return nil, err
	}
	var dueDate *time.Time
	if req.DueDate != nil {
		dueDate, err = parseStrictPMCommandDate(*req.DueDate, "due_date", false)
		if err != nil {
			return nil, err
		}
	}
	item, err := s.checklistService.Create(ctx, task.ID, model.CreateChecklistItemRequest{Text: req.Text, Position: req.Position, AssigneeID: normalizeOptionalCommandString(req.AssigneeID), DueDate: dueDate}, meta.WorkspaceID, fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCommandChecklistItem(item)), nil
}

func (s *InternalCommandService) executeUpdateTaskChecklistItem(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.checklistService == nil || s.taskService == nil {
		return nil, fmt.Errorf("checklist service is not configured")
	}
	var req struct {
		ChecklistItemID string  `json:"checklist_item_id"`
		TaskID          string  `json:"task_id"`
		Text            *string `json:"text"`
		Completed       *bool   `json:"completed"`
		Position        *int    `json:"position"`
		AssigneeID      *string `json:"assignee_id"`
		DueDate         *string `json:"due_date"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse update checklist item input: %w", err)
	}
	req.ChecklistItemID = strings.TrimSpace(req.ChecklistItemID)
	if req.ChecklistItemID == "" {
		return nil, fmt.Errorf("checklist_item_id is required")
	}
	if req.Text == nil && req.Completed == nil && req.Position == nil && req.AssigneeID == nil && req.DueDate == nil {
		return nil, fmt.Errorf("at least one editable field is required")
	}
	item, err := s.checklistService.repo.GetByID(ctx, req.ChecklistItemID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("checklist item not found")
	}
	explicitTaskID := strings.TrimSpace(req.TaskID)
	if explicitTaskID != "" && explicitTaskID != item.TaskID {
		return nil, fmt.Errorf("checklist item does not belong to task_id")
	}
	task, err := s.resolveWritableCommandTask(ctx, meta, item.TaskID)
	if err != nil {
		return nil, err
	}
	if err := s.validateChecklistAssignee(ctx, meta.WorkspaceID, req.AssigneeID, true); err != nil {
		return nil, err
	}
	var dueDate *time.Time
	if req.DueDate != nil {
		dueDate, err = parseStrictPMCommandDate(*req.DueDate, "due_date", true)
		if err != nil {
			return nil, err
		}
	}
	updated, err := s.checklistService.Update(ctx, item.ID, model.UpdateChecklistItemRequest{Text: req.Text, Completed: req.Completed, Position: req.Position, AssigneeID: normalizeClearableCommandString(req.AssigneeID), DueDate: dueDate, DueDateSet: req.DueDate != nil}, meta.WorkspaceID, fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	result := compactCommandChecklistItem(updated)
	result["task_id"] = task.ID
	return mustJSON(result), nil
}

func (s *InternalCommandService) executeAddPMComment(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.commentService == nil {
		return nil, fmt.Errorf("comment service is not configured")
	}
	var req struct {
		EntityType string `json:"entity_type"`
		EntityID   string `json:"entity_id"`
		Content    string `json:"content"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse add PM comment input: %w", err)
	}
	entityType := strings.ToLower(strings.TrimSpace(req.EntityType))
	targetType := normalizeCommandBarTargetType(meta.TargetType)
	if targetType == "story" {
		targetType = "task"
	}
	if entityType == "" && targetType != "workspace" {
		entityType = targetType
	}
	if entityType != "task" && entityType != "epic" && entityType != "sprint" && entityType != "objective" {
		return nil, fmt.Errorf("entity_type must be task, epic, sprint, or objective")
	}
	if err := validatePMCommentTarget(meta, entityType); err != nil {
		return nil, err
	}
	entityID, err := resolveCommandEntityID(meta, req.EntityID, entityType)
	if err != nil {
		return nil, err
	}
	if err := s.validatePMCommentEntity(ctx, meta, entityType, entityID); err != nil {
		return nil, err
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if normalized := normalizeTaskDescriptionRichText(&content); normalized != nil {
		content = *normalized
	}
	comment, err := s.commentService.Create(ctx, model.CreateCommentRequest{EntityType: entityType, EntityID: entityID, Body: content}, fallbackActor(meta), meta.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"entity_type": entityType, "entity_id": entityID, "comment_id": comment.Comment.ID}), nil
}

func (s *InternalCommandService) executeAddTaskComment(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		TaskID  string `json:"task_id"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse add task comment input: %w", err)
	}
	return s.executeAddPMComment(ctx, meta, mustJSON(map[string]any{"entity_type": "task", "entity_id": req.TaskID, "content": req.Content}))
}

func (s *InternalCommandService) executeUpdatePMTaskState(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.taskService == nil {
		return nil, fmt.Errorf("task service is not configured")
	}
	var req struct {
		StoryID  string `json:"story_id"`
		TaskID   string `json:"task_id"`
		StateID  string `json:"state_id"`
		Position *int   `json:"position"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse update task state input: %w", err)
	}
	taskID, err := resolveCommandEntityID(meta, firstNonEmptyCommand(req.TaskID, req.StoryID), "task")
	if err != nil {
		return nil, err
	}
	detail, err := s.taskService.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if detail.Task.WorkspaceID != meta.WorkspaceID {
		return nil, fmt.Errorf("task not found")
	}
	if err := s.validateTaskWithinTarget(ctx, meta, &detail.Task); err != nil {
		return nil, err
	}
	stateID := strings.TrimSpace(req.StateID)
	if stateID == "" {
		return nil, fmt.Errorf("state_id is required")
	}
	updated, err := s.taskService.MoveToState(ctx, taskID, model.MoveTaskRequest{StateID: stateID, Position: req.Position}, fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"task_id": updated.Task.ID, "story_id": updated.Task.ID, "state_id": updated.Task.WorkflowStateID}), nil
}

func (s *InternalCommandService) executeSetPMTaskDependencies(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.taskRepo == nil || s.taskLinkRepo == nil {
		return nil, fmt.Errorf("task dependency repositories are not configured")
	}
	var req struct {
		Dependencies []struct {
			SourceTaskID string `json:"source_task_id"`
			TargetTaskID string `json:"target_task_id"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse dependency input: %w", err)
	}
	if len(req.Dependencies) == 0 || len(req.Dependencies) > 100 {
		return nil, fmt.Errorf("dependencies must contain between 1 and 100 items")
	}
	type pair struct{ sourceID, targetID string }
	pending := make([]pair, 0, len(req.Dependencies))
	for _, dependency := range req.Dependencies {
		sourceID := strings.TrimSpace(dependency.SourceTaskID)
		targetID := strings.TrimSpace(dependency.TargetTaskID)
		if sourceID == "" || targetID == "" || sourceID == targetID {
			return nil, fmt.Errorf("source_task_id and target_task_id must be distinct task IDs")
		}
		for _, taskID := range []string{sourceID, targetID} {
			task, err := s.taskRepo.GetRawByID(ctx, taskID)
			if err != nil {
				return nil, err
			}
			if task == nil || task.WorkspaceID != meta.WorkspaceID || requireTeamAccess(ctx, task.TeamID) != nil {
				return nil, fmt.Errorf("tasks must belong to the current workspace and be accessible")
			}
			if err := s.validateTaskWithinTarget(ctx, meta, task); err != nil {
				return nil, err
			}
		}
		pending = append(pending, pair{sourceID: sourceID, targetID: targetID})
	}
	existing, err := s.taskLinkRepo.ListByWorkspaceAndType(ctx, meta.WorkspaceID, model.PMTaskLinkTypeBlocks)
	if err != nil {
		return nil, err
	}
	graph := make(map[string][]string, len(existing)+len(pending))
	for _, link := range existing {
		graph[link.SourceTaskID] = append(graph[link.SourceTaskID], link.TargetTaskID)
	}
	for _, dependency := range pending {
		graph[dependency.sourceID] = append(graph[dependency.sourceID], dependency.targetID)
	}
	if taskDependencyGraphHasCycle(graph) {
		return nil, fmt.Errorf("task dependencies contain a cycle")
	}
	if err := s.taskLinkRepo.WithTransaction(ctx, func(links *repository.PMTaskLinkRepository) error {
		for _, dependency := range pending {
			if err := links.Create(ctx, &model.PMTaskLink{WorkspaceID: meta.WorkspaceID, SourceTaskID: dependency.sourceID, TargetTaskID: dependency.targetID, LinkType: model.PMTaskLinkTypeBlocks, CreatedBy: fallbackActor(meta)}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"dependency_count": len(pending)}), nil
}

func resolveCommandParentAssociation(meta model.InternalCommandContext, explicit, entityType string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	targetType := normalizeCommandBarTargetType(meta.TargetType)
	if targetType != entityType {
		return explicit, nil
	}
	targetID := strings.TrimSpace(meta.TargetID)
	if explicit != "" && explicit != targetID {
		return "", fmt.Errorf("%s_id conflicts with the current %s target", entityType, entityType)
	}
	return targetID, nil
}

func validateCommandParentUpdate(meta model.InternalCommandContext, entityType string, explicit *string) error {
	if explicit == nil || normalizeCommandBarTargetType(meta.TargetType) != entityType {
		return nil
	}
	if strings.TrimSpace(*explicit) != strings.TrimSpace(meta.TargetID) {
		return fmt.Errorf("%s_id conflicts with the current %s target", entityType, entityType)
	}
	return nil
}

func validatePMCommentTarget(meta model.InternalCommandContext, entityType string) error {
	targetType := normalizeCommandBarTargetType(meta.TargetType)
	if targetType == "story" {
		targetType = "task"
	}
	if targetType == "workspace" || entityType == targetType {
		return nil
	}
	if entityType == "task" && (targetType == "epic" || targetType == "sprint") {
		return nil
	}
	return fmt.Errorf("entity_type conflicts with the current %s target", targetType)
}

// resolveCommandEntityID defaults an entity ID from a matching run target and
// rejects a conflicting explicit ID for mutating target-aware operations.
func resolveCommandEntityID(meta model.InternalCommandContext, explicit, entityType string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	targetType := normalizeCommandBarTargetType(meta.TargetType)
	if targetType == "story" {
		targetType = "task"
	}
	if targetType == entityType {
		targetID := strings.TrimSpace(meta.TargetID)
		if explicit != "" && explicit != targetID {
			return "", fmt.Errorf("%s_id conflicts with the current %s target", entityType, entityType)
		}
		if explicit == "" {
			explicit = targetID
		}
	}
	if explicit == "" {
		return "", fmt.Errorf("%s_id is required", entityType)
	}
	return explicit, nil
}

func (s *InternalCommandService) resolveWritableCommandTask(ctx context.Context, meta model.InternalCommandContext, explicitTaskID string) (*model.PMTask, error) {
	taskID, err := resolveCommandEntityID(meta, explicitTaskID, "task")
	if err != nil {
		return nil, err
	}
	detail, err := s.taskService.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if detail.Task.WorkspaceID != meta.WorkspaceID {
		return nil, fmt.Errorf("task not found")
	}
	if err := s.validateTaskWithinTarget(ctx, meta, &detail.Task); err != nil {
		return nil, err
	}
	return &detail.Task, nil
}

func (s *InternalCommandService) validateTaskWithinTarget(_ context.Context, meta model.InternalCommandContext, task *model.PMTask) error {
	if task == nil {
		return fmt.Errorf("task not found")
	}
	if err := requireCommandAgentTeam(meta, task.TeamID); err != nil {
		return err
	}
	switch normalizeCommandBarTargetType(meta.TargetType) {
	case "task", "story":
		if task.ID != strings.TrimSpace(meta.TargetID) {
			return fmt.Errorf("task does not belong to the current task target")
		}
	case "epic":
		if task.EpicID == nil || strings.TrimSpace(*task.EpicID) != strings.TrimSpace(meta.TargetID) {
			return fmt.Errorf("task does not belong to the current epic target")
		}
	case "sprint":
		if task.SprintID == nil || strings.TrimSpace(*task.SprintID) != strings.TrimSpace(meta.TargetID) {
			return fmt.Errorf("task does not belong to the current sprint target")
		}
	}
	return nil
}

func (s *InternalCommandService) validateCommandAgentParentScope(ctx context.Context, meta model.InternalCommandContext, epicID, sprintID string) error {
	if epicID != "" && s.epicService != nil {
		e, err := s.epicService.GetByID(ctx, epicID)
		if err != nil || e == nil || e.Epic.WorkspaceID != meta.WorkspaceID {
			return fmt.Errorf("epic not found")
		}
		if err := requireCommandAgentTeam(meta, e.Epic.TeamID); err != nil {
			return err
		}
	}
	if sprintID != "" && s.sprintService != nil {
		s, err := s.sprintService.GetByID(ctx, sprintID)
		if err != nil || s == nil || s.Sprint.WorkspaceID != meta.WorkspaceID {
			return fmt.Errorf("sprint not found")
		}
		if err := requireCommandAgentTeam(meta, s.Sprint.TeamID); err != nil {
			return err
		}
	}
	return nil
}

func compactCommandTask(detail *model.TaskDetail) map[string]any {
	if detail == nil {
		return map[string]any{}
	}
	task := detail.Task
	stateName := ""
	stateType := ""
	if detail.State != nil {
		stateName = detail.State.Name
		stateType = detail.State.StateType
	}
	owners := make([]map[string]any, 0, len(detail.Owners))
	for _, owner := range detail.Owners {
		owners = append(owners, map[string]any{
			"user_id": owner.ID,
			"name":    owner.FullName,
		})
	}
	labels := make([]map[string]any, 0, len(detail.Labels))
	for _, label := range detail.Labels {
		labels = append(labels, map[string]any{
			"label_id": label.ID,
			"name":     label.Name,
			"color":    label.Color,
			"team_id":  label.TeamID,
		})
	}
	return map[string]any{
		"task_id":          task.ID,
		"display_id":       task.DisplayID,
		"task_key":         task.TaskKey,
		"workspace_id":     task.WorkspaceID,
		"name":             task.Name,
		"description":      task.Description,
		"task_type":        task.TaskType,
		"team_id":          task.TeamID,
		"workflow_id":      task.WorkflowID,
		"state_id":         task.WorkflowStateID,
		"state_name":       stateName,
		"state_type":       stateType,
		"epic_id":          task.EpicID,
		"epic_name":        detail.EpicName,
		"sprint_id":        task.SprintID,
		"sprint_name":      detail.SprintName,
		"owner_member_ids": task.OwnerMemberIDs,
		"owners":           owners,
		"labels":           labels,
		"estimate":         task.Estimate,
		"priority":         task.Priority,
		"severity":         task.Severity,
		"deadline":         task.Deadline,
		"blocked":          task.Blocked,
		"blocker":          task.Blocker,
		"completed":        task.Completed,
		"blocked_by_count": task.BlockedByCount,
		"blocking_count":   task.BlockingCount,
		"blocked_by_tasks": task.BlockedByTasks,
		"blocking_tasks":   task.BlockingTasks,
		"updated_at":       task.UpdatedAt,
	}
}

func compactCommandChecklistItem(item *model.PMChecklistItem) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	return map[string]any{
		"checklist_item_id": item.ID,
		"task_id":           item.TaskID,
		"text":              item.Text,
		"completed":         item.Completed,
		"position":          item.Position,
		"assignee_id":       item.AssigneeID,
		"due_date":          item.DueDate,
		"created_at":        item.CreatedAt,
		"updated_at":        item.UpdatedAt,
	}
}

func parseStrictPMCommandDate(value, field string, allowEmpty bool) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" && allowEmpty {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return nil, fmt.Errorf("%s must be YYYY-MM-DD", field)
	}
	return &parsed, nil
}

func normalizeOptionalCommandString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeClearableCommandString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func trimOptionalCommandSlice(values []string) []string {
	if values == nil {
		return nil
	}
	trimmed := commandTrimStringSlice(values)
	if trimmed == nil {
		return []string{}
	}
	return trimmed
}

func (s *InternalCommandService) validateChecklistAssignee(ctx context.Context, workspaceID string, assigneeID *string, allowEmpty bool) error {
	if assigneeID == nil {
		return nil
	}
	assignee := strings.TrimSpace(*assigneeID)
	if assignee == "" && allowEmpty {
		return nil
	}
	if assignee == "" {
		return fmt.Errorf("assignee_id must not be empty")
	}
	if s.workspaceRepo == nil {
		return fmt.Errorf("workspace repository is not configured")
	}
	membership, err := s.workspaceRepo.GetMembership(ctx, workspaceID, assignee)
	if err != nil {
		return err
	}
	if membership == nil {
		return fmt.Errorf("assignee_id does not belong to the current workspace")
	}
	return nil
}

func (s *InternalCommandService) validatePMCommentEntity(ctx context.Context, meta model.InternalCommandContext, entityType, entityID string) error {
	switch entityType {
	case "task":
		if s.taskService == nil {
			return fmt.Errorf("task service is not configured")
		}
		detail, err := s.taskService.GetByID(ctx, entityID)
		if err != nil || detail.Task.WorkspaceID != meta.WorkspaceID {
			return fmt.Errorf("task not found")
		}
		return s.validateTaskWithinTarget(ctx, meta, &detail.Task)
	case "epic":
		if s.epicService == nil {
			return fmt.Errorf("epic service is not configured")
		}
		epic, err := s.epicService.GetByID(ctx, entityID)
		if err != nil || epic.Epic.WorkspaceID != meta.WorkspaceID {
			return fmt.Errorf("epic not found")
		}
		if err := requireCommandAgentTeam(meta, epic.Epic.TeamID); err != nil {
			return err
		}
	case "sprint":
		if s.sprintService == nil {
			return fmt.Errorf("sprint service is not configured")
		}
		sprint, err := s.sprintService.GetByID(ctx, entityID)
		if err != nil || sprint.Sprint.WorkspaceID != meta.WorkspaceID {
			return fmt.Errorf("sprint not found")
		}
		if err := requireCommandAgentTeam(meta, sprint.Sprint.TeamID); err != nil {
			return err
		}
	case "objective":
		if s.objectiveService == nil {
			return fmt.Errorf("objective service is not configured")
		}
		objective, err := s.objectiveService.GetByID(ctx, entityID, meta.WorkspaceID)
		if err != nil {
			return fmt.Errorf("objective not found")
		}
		if err := requireCommandAgentTeams(meta, objective.Teams); err != nil {
			return err
		}
	}
	return nil
}
