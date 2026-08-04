package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var pmSprintCommandTargetTypes = []string{"workspace", "sprint"}

// SetPMSprintOperationalService wires sprint commands without expanding the
// already-large internal-command constructor.
func (s *InternalCommandService) SetPMSprintOperationalService(sprintService *PMSprintService) {
	if s == nil {
		return
	}
	s.sprintService = sprintService
}

func (s *InternalCommandService) registerPMSprintCommands() {
	definitions := []struct {
		name     string
		mutating bool
	}{
		{name: "pm.list_sprints"},
		{name: "pm.get_sprint"},
		{name: "pm.list_sprint_tasks"},
		{name: "pm.create_sprint", mutating: true},
		{name: "pm.update_sprint", mutating: true},
	}
	for _, item := range definitions {
		name := item.name
		s.register(InternalCommandDefinition{
			Name:                 name,
			Module:               "pm",
			Mutating:             item.mutating,
			SupportedTargetTypes: pmSprintCommandTargetTypes,
			Tool:                 mustCommandToolMetadata(name),
			Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
				return s.executePMSprintCommand(ctx, meta, name, input)
			},
		})
	}
}

func (s *InternalCommandService) executePMSprintCommand(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
	switch name {
	case "pm.list_sprints":
		return s.executePMListSprints(ctx, meta, input)
	case "pm.get_sprint":
		return s.executePMGetSprint(ctx, meta, input)
	case "pm.list_sprint_tasks":
		return s.executePMListSprintTasks(ctx, meta, input)
	case "pm.create_sprint":
		return s.executePMCreateSprint(ctx, meta, input)
	case "pm.update_sprint":
		return s.executePMUpdateSprint(ctx, meta, input)
	default:
		return nil, fmt.Errorf("unsupported sprint command %q", name)
	}
}

func (s *InternalCommandService) executePMListSprints(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.sprintService == nil {
		return nil, fmt.Errorf("sprint service is not configured")
	}
	var req struct {
		TeamID   *string `json:"team_id"`
		Status   *string `json:"status"`
		Archived *bool   `json:"archived"`
		Page     int     `json:"page"`
		PerPage  int     `json:"per_page"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse list sprints input: %w", err)
	}
	if req.TeamID != nil {
		teamID := strings.TrimSpace(*req.TeamID)
		if err := s.validatePMSprintTeam(ctx, meta.WorkspaceID, teamID); err != nil {
			return nil, err
		}
		req.TeamID = &teamID
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		switch status {
		case model.PMSprintStatusUnstarted, model.PMSprintStatusStarted, model.PMSprintStatusDone:
		default:
			return nil, fmt.Errorf("status must be one of unstarted, started, done")
		}
		req.Status = &status
	}
	sprints, total, page, perPage, err := s.sprintService.ListPage(ctx, meta.WorkspaceID, model.PMSprintListFilters{TeamID: req.TeamID, Status: req.Status, Archived: req.Archived}, model.PMPagination{Page: req.Page, PerPage: req.PerPage})
	if err != nil {
		return nil, fmt.Errorf("list sprints: %w", err)
	}
	items := make([]map[string]any, 0, len(sprints))
	for _, sprint := range sprints {
		items = append(items, compactCommandSprint(&sprint))
	}
	return mustJSON(map[string]any{"sprints": items, "page": page, "per_page": perPage, "total": total}), nil
}

func (s *InternalCommandService) executePMGetSprint(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		SprintID string `json:"sprint_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse get sprint input: %w", err)
	}
	sprintID, err := resolvePMSprintCommandID(meta, req.SprintID, false)
	if err != nil {
		return nil, err
	}
	sprint, err := s.loadCommandSprint(ctx, meta.WorkspaceID, sprintID)
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCommandSprint(sprint)), nil
}

func (s *InternalCommandService) executePMListSprintTasks(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		SprintID string `json:"sprint_id"`
		Page     int    `json:"page"`
		PerPage  int    `json:"per_page"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse list sprint tasks input: %w", err)
	}
	sprintID, err := resolvePMSprintCommandID(meta, req.SprintID, false)
	if err != nil {
		return nil, err
	}
	if _, err := s.loadCommandSprint(ctx, meta.WorkspaceID, sprintID); err != nil {
		return nil, err
	}
	tasks, total, page, perPage, err := s.sprintService.ListTasksPage(ctx, sprintID, model.PMPagination{Page: req.Page, PerPage: req.PerPage})
	if err != nil {
		return nil, fmt.Errorf("list sprint tasks: %w", err)
	}
	items := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, compactCommandSprintTask(task))
	}
	return mustJSON(map[string]any{"sprint_id": sprintID, "tasks": items, "page": page, "per_page": perPage, "total": total}), nil
}

func (s *InternalCommandService) executePMCreateSprint(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.sprintService == nil {
		return nil, fmt.Errorf("sprint service is not configured")
	}
	var req struct {
		Name        string   `json:"name"`
		Description *string  `json:"description"`
		StartDate   string   `json:"start_date"`
		EndDate     string   `json:"end_date"`
		TeamID      string   `json:"team_id"`
		LabelIDs    []string `json:"label_ids"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse create sprint input: %w", err)
	}
	teamID := strings.TrimSpace(req.TeamID)
	if err := s.validatePMSprintTeam(ctx, meta.WorkspaceID, teamID); err != nil {
		return nil, err
	}
	startDate, err := parsePMSprintCommandDate("start_date", req.StartDate)
	if err != nil {
		return nil, err
	}
	endDate, err := parsePMSprintCommandDate("end_date", req.EndDate)
	if err != nil {
		return nil, err
	}
	labelIDs, err := validatePMSprintCommandLabels(ctx, s.sprintService, meta.WorkspaceID, req.LabelIDs, &teamID)
	if err != nil {
		return nil, err
	}
	result, err := s.sprintService.Create(ctx, model.CreateSprintRequest{
		WorkspaceID: meta.WorkspaceID,
		Name:        req.Name,
		Description: req.Description,
		StartDate:   startDate,
		EndDate:     endDate,
		TeamID:      &teamID,
		LabelIDs:    labelIDs,
	}, fallbackActor(meta))
	if err != nil {
		return nil, fmt.Errorf("create sprint: %w", err)
	}
	return mustJSON(compactCommandSprint(result)), nil
}

func (s *InternalCommandService) executePMUpdateSprint(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.sprintService == nil {
		return nil, fmt.Errorf("sprint service is not configured")
	}
	var req struct {
		SprintID    string    `json:"sprint_id"`
		Name        *string   `json:"name"`
		Description *string   `json:"description"`
		StartDate   *string   `json:"start_date"`
		EndDate     *string   `json:"end_date"`
		TeamID      *string   `json:"team_id"`
		LabelIDs    *[]string `json:"label_ids"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse update sprint input: %w", err)
	}
	if req.Name == nil && req.Description == nil && req.StartDate == nil && req.EndDate == nil && req.TeamID == nil && req.LabelIDs == nil {
		return nil, fmt.Errorf("at least one editable field is required")
	}
	sprintID, err := resolvePMSprintCommandID(meta, req.SprintID, true)
	if err != nil {
		return nil, err
	}
	current, err := s.loadCommandSprint(ctx, meta.WorkspaceID, sprintID)
	if err != nil {
		return nil, err
	}
	update := model.UpdateSprintRequest{Name: req.Name, Description: req.Description}
	resolvedTeamID := current.Sprint.TeamID
	if req.TeamID != nil {
		teamID := strings.TrimSpace(*req.TeamID)
		if err := s.validatePMSprintTeam(ctx, meta.WorkspaceID, teamID); err != nil {
			return nil, err
		}
		if err := requireCanManage(ctx, &teamID); err != nil {
			return nil, err
		}
		update.TeamID = &teamID
		resolvedTeamID = &teamID
	}
	if req.StartDate != nil {
		parsed, err := parsePMSprintCommandDate("start_date", *req.StartDate)
		if err != nil {
			return nil, err
		}
		update.StartDate = &parsed
	}
	if req.EndDate != nil {
		parsed, err := parsePMSprintCommandDate("end_date", *req.EndDate)
		if err != nil {
			return nil, err
		}
		update.EndDate = &parsed
	}
	if req.LabelIDs != nil {
		labelIDs, err := validatePMSprintCommandLabels(ctx, s.sprintService, meta.WorkspaceID, *req.LabelIDs, resolvedTeamID)
		if err != nil {
			return nil, err
		}
		if labelIDs == nil {
			labelIDs = []string{}
		}
		update.LabelIDs = labelIDs
	}
	result, err := s.sprintService.Update(ctx, sprintID, update, fallbackActor(meta))
	if err != nil {
		return nil, fmt.Errorf("update sprint: %w", err)
	}
	return mustJSON(compactCommandSprint(result)), nil
}

func (s *InternalCommandService) validatePMSprintTeam(ctx context.Context, workspaceID, teamID string) error {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return fmt.Errorf("team_id is required")
	}
	if s == nil || s.settingsRepo == nil {
		return fmt.Errorf("team repository is not configured")
	}
	team, err := s.settingsRepo.GetTeamByID(ctx, teamID)
	if err != nil {
		return fmt.Errorf("get sprint team: %w", err)
	}
	if team == nil || team.WorkspaceID != strings.TrimSpace(workspaceID) {
		return fmt.Errorf("team not found in this workspace")
	}
	return nil
}

func (s *InternalCommandService) loadCommandSprint(ctx context.Context, workspaceID, sprintID string) (*model.SprintWithStats, error) {
	if s == nil || s.sprintService == nil {
		return nil, fmt.Errorf("sprint service is not configured")
	}
	sprint, err := s.sprintService.GetByID(ctx, strings.TrimSpace(sprintID))
	if err != nil || sprint == nil || sprint.Sprint.WorkspaceID != strings.TrimSpace(workspaceID) {
		return nil, fmt.Errorf("sprint not found in this workspace")
	}
	return sprint, nil
}

func resolvePMSprintCommandID(meta model.InternalCommandContext, explicit string, mutating bool) (string, error) {
	explicit = strings.TrimSpace(explicit)
	targetID := ""
	if strings.TrimSpace(meta.TargetType) == "sprint" {
		targetID = strings.TrimSpace(meta.TargetID)
	}
	if mutating && explicit != "" && targetID != "" && explicit != targetID {
		return "", fmt.Errorf("sprint_id conflicts with current sprint target")
	}
	if explicit != "" {
		return explicit, nil
	}
	if targetID != "" {
		return targetID, nil
	}
	return "", fmt.Errorf("sprint_id is required")
}

func parsePMSprintCommandDate(field, value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD", field)
	}
	return parsed.UTC(), nil
}

func validatePMSprintCommandLabels(ctx context.Context, sprintService *PMSprintService, workspaceID string, labelIDs []string, teamID *string) ([]string, error) {
	trimmed := commandTrimStringSlice(labelIDs)
	if len(labelIDs) > 0 && len(trimmed) != len(labelIDs) {
		return nil, fmt.Errorf("label_ids must not contain empty values")
	}
	if sprintService == nil {
		return nil, fmt.Errorf("sprint service is not configured")
	}
	if err := validateLabelScope(ctx, sprintService.labelRepo, workspaceID, trimmed, allowedTeamIDs(teamID)); err != nil {
		return nil, err
	}
	return trimmed, nil
}

func compactCommandSprint(value *model.SprintWithStats) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	labels := make([]map[string]any, 0, len(value.Labels))
	for _, label := range value.Labels {
		labels = append(labels, map[string]any{
			"id": label.ID, "name": label.Name, "color": commandDerefString(label.Color), "team_id": commandDerefString(label.TeamID),
		})
	}
	return map[string]any{
		"id":           value.Sprint.ID,
		"sprint_id":    value.Sprint.ID,
		"workspace_id": value.Sprint.WorkspaceID,
		"name":         value.Sprint.Name,
		"description":  commandDerefString(value.Sprint.Description),
		"start_date":   pmSprintCommandDateString(value.Sprint.StartDate),
		"end_date":     pmSprintCommandDateString(value.Sprint.EndDate),
		"status":       value.Sprint.Status,
		"team_id":      commandDerefString(value.Sprint.TeamID),
		"labels":       labels,
		"stats":        value.Stats,
	}
}

func compactCommandSprintTask(task model.BoardTask) map[string]any {
	labels := make([]map[string]any, 0, len(task.Labels))
	for _, label := range task.Labels {
		labels = append(labels, map[string]any{
			"id":      label.ID,
			"name":    label.Name,
			"color":   commandDerefString(label.Color),
			"team_id": commandDerefString(label.TeamID),
		})
	}
	return map[string]any{
		"id": task.ID, "task_id": task.ID, "display_id": task.DisplayID, "task_key": task.TaskKey,
		"name": task.Name, "task_type": task.TaskType, "priority": task.Priority, "severity": task.Severity,
		"team_id": commandDerefString(task.TeamID), "workflow_id": task.WorkflowID, "state_id": task.WorkflowStateID,
		"state_name": commandDerefString(task.StateName), "state_type": commandDerefString(task.StateType),
		"epic_id": commandDerefString(task.EpicID), "sprint_id": commandDerefString(task.SprintID),
		"owner_member_ids": task.OwnerMemberIDs, "labels": labels, "estimate": task.Estimate,
		"completed": task.Completed, "blocked": task.Blocked, "updated_at": task.UpdatedAt,
	}
}

func pmSprintCommandDateString(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format("2006-01-02")
}
