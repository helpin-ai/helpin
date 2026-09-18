package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const internalCommandDateOnlyLayout = "2006-01-02"

type pmEpicCreateCommandInput struct {
	Name                 string   `json:"name"`
	Description          *string  `json:"description"`
	EpicStateID          *string  `json:"epic_state_id"`
	OwnerID              *string  `json:"owner_id"`
	OwnerMemberID        *string  `json:"owner_member_id"`
	TeamID               *string  `json:"team_id"`
	PlannedStartDate     *string  `json:"planned_start_date"`
	Deadline             *string  `json:"deadline"`
	Color                *string  `json:"color"`
	Health               *string  `json:"health"`
	HealthComment        *string  `json:"health_comment"`
	LabelIDs             []string `json:"label_ids"`
	PlanningRepositoryID *string  `json:"planning_repository_id"`
}

type pmEpicUpdateCommandInput struct {
	EpicID               string    `json:"epic_id"`
	Name                 *string   `json:"name"`
	Description          *string   `json:"description"`
	EpicStateID          *string   `json:"epic_state_id"`
	OwnerID              *string   `json:"owner_id"`
	OwnerMemberID        *string   `json:"owner_member_id"`
	TeamID               *string   `json:"team_id"`
	PlannedStartDate     *string   `json:"planned_start_date"`
	Deadline             *string   `json:"deadline"`
	Color                *string   `json:"color"`
	Health               *string   `json:"health"`
	HealthComment        *string   `json:"health_comment"`
	LabelIDs             *[]string `json:"label_ids"`
	PlanningRepositoryID *string   `json:"planning_repository_id"`
}

func (s *InternalCommandService) registerPMEpicCommands() {
	if s == nil {
		return
	}
	s.register(InternalCommandDefinition{
		Name:                 "pm.list_epics",
		Module:               "pm",
		Mutating:             false,
		SupportedTargetTypes: []string{"workspace", "epic"},
		Tool:                 mustCommandToolMetadata("pm.list_epics"),
		Execute:              s.executePMListEpics,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.get_epic",
		Module:               "pm",
		Mutating:             false,
		SupportedTargetTypes: []string{"workspace", "epic"},
		Tool:                 mustCommandToolMetadata("pm.get_epic"),
		Execute:              s.executePMGetEpic,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.create_epic",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "epic"},
		Tool:                 mustCommandToolMetadata("pm.create_epic"),
		Execute:              s.executePMCreateEpic,
	})
	s.register(InternalCommandDefinition{
		Name:                 "pm.update_epic",
		Module:               "pm",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "epic"},
		Tool:                 mustCommandToolMetadata("pm.update_epic"),
		Execute:              s.executePMUpdateEpic,
	})
}

func (s *InternalCommandService) executePMListEpics(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requirePMEpicCommandDependencies(); err != nil {
		return nil, err
	}
	var req struct {
		Query    string `json:"query"`
		TeamID   string `json:"team_id"`
		StateID  string `json:"state_id"`
		LabelID  string `json:"label_id"`
		Archived *bool  `json:"archived"`
		Page     int    `json:"page"`
		PerPage  int    `json:"per_page"`
		Limit    int    `json:"limit"`
		Offset   *int   `json:"offset"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse list epics input: %w", err)
		}
	}
	agentTeams, err := commandAgentTeamFilter(meta)
	if err != nil {
		return nil, err
	}
	pagination, offset, limit, err := normalizeCommandPagination(req.Page, req.PerPage, req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	teamID := strings.TrimSpace(req.TeamID)
	stateID := strings.TrimSpace(req.StateID)
	labelID := strings.TrimSpace(req.LabelID)
	if teamID != "" {
		if _, err := s.validatePMCommandEpicTeam(ctx, meta.WorkspaceID, teamID, false); err != nil {
			return nil, err
		}
	}
	if stateID != "" {
		if err := s.validatePMCommandEpicState(ctx, meta.WorkspaceID, stateID); err != nil {
			return nil, err
		}
	}
	if labelID != "" {
		label, err := s.validatePMCommandEpicLabel(ctx, meta.WorkspaceID, labelID)
		if err != nil {
			return nil, err
		}
		if label.TeamID != nil {
			if err := requireTeamAccess(ctx, label.TeamID); err != nil {
				return nil, fmt.Errorf("label not found")
			}
		}
		if teamID != "" && label.TeamID != nil && strings.TrimSpace(*label.TeamID) != teamID {
			return nil, fmt.Errorf("label does not belong to team_id")
		}
	}
	filters := model.PMEpicListFilters{
		Search: stringPtrOrNil(req.Query), TeamID: stringPtrOrNil(teamID), StateID: stringPtrOrNil(stateID), LabelID: stringPtrOrNil(labelID), Archived: req.Archived,
	}
	if len(agentTeams) > 0 {
		filters.AccessibleTeamIDs = agentTeams
	}
	epics, total, err := s.epicService.ListPage(ctx, meta.WorkspaceID, filters, pagination)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(epics))
	for i := range epics {
		items = append(items, compactPMCommandEpic(&epics[i], false))
	}
	response := commandPaginationOutput(total, offset, limit, len(items))
	response["epics"] = items
	if req.Page > 0 || req.PerPage > 0 {
		response["page"] = pagination.Page
		response["per_page"] = pagination.PerPage
	}
	return mustJSON(response), nil
}

func (s *InternalCommandService) executePMGetEpic(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requirePMEpicCommandDependencies(); err != nil {
		return nil, err
	}
	var req struct {
		EpicID string `json:"epic_id"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, fmt.Errorf("parse get epic input: %w", err)
		}
	}
	epicID, err := resolvePMCommandEpicID(meta, req.EpicID, false)
	if err != nil {
		return nil, err
	}
	epic, err := s.epicService.GetByID(ctx, epicID)
	if err != nil {
		return nil, err
	}
	if epic == nil || epic.Epic.WorkspaceID != meta.WorkspaceID {
		return nil, fmt.Errorf("epic not found")
	}
	if err := requireCommandAgentTeam(meta, epic.Epic.TeamID); err != nil {
		return nil, err
	}
	return mustJSON(compactPMCommandEpic(epic, true)), nil
}

func (s *InternalCommandService) executePMCreateEpic(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requirePMEpicCommandDependencies(); err != nil {
		return nil, err
	}
	var req pmEpicCreateCommandInput
	if err := decodeStrictInternalCommandInput(input, &req); err != nil {
		return nil, fmt.Errorf("parse create epic input: %w", err)
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.TeamID == nil || strings.TrimSpace(*req.TeamID) == "" {
		return nil, fmt.Errorf("team_id is required")
	}
	teamID, err := s.validatePMCommandOptionalEpicTeam(ctx, meta.WorkspaceID, req.TeamID, true)
	if err != nil {
		return nil, err
	}
	if err := requireCommandAgentTeam(meta, teamID); err != nil {
		return nil, err
	}
	stateID, err := s.validatePMCommandOptionalEpicState(ctx, meta.WorkspaceID, req.EpicStateID)
	if err != nil {
		return nil, err
	}
	ownerID, ownerMemberID, err := s.validatePMCommandEpicOwner(ctx, meta.WorkspaceID, req.OwnerID, req.OwnerMemberID)
	if err != nil {
		return nil, err
	}
	labels, err := s.validatePMCommandEpicLabels(ctx, meta.WorkspaceID, req.LabelIDs, teamID)
	if err != nil {
		return nil, err
	}
	repositoryID, err := s.validatePMCommandOptionalPlanningRepository(ctx, meta.WorkspaceID, req.PlanningRepositoryID)
	if err != nil {
		return nil, err
	}
	plannedStartDate, err := parsePMCommandDateOnly(req.PlannedStartDate, "planned_start_date")
	if err != nil {
		return nil, err
	}
	deadline, err := parsePMCommandDateOnly(req.Deadline, "deadline")
	if err != nil {
		return nil, err
	}
	epic, err := s.epicService.Create(ctx, model.CreateEpicRequest{
		WorkspaceID: meta.WorkspaceID, Name: strings.TrimSpace(req.Name), Description: req.Description,
		EpicStateID: stateID, OwnerID: ownerID, OwnerMemberID: ownerMemberID, TeamID: teamID,
		PlannedStartDate: plannedStartDate, Deadline: deadline, Color: req.Color, Health: req.Health,
		HealthComment: req.HealthComment, LabelIDs: labels, PlanningRepositoryID: repositoryID,
	}, fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	return mustJSON(compactPMCommandEpic(epic, true)), nil
}

func (s *InternalCommandService) executePMUpdateEpic(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requirePMEpicCommandDependencies(); err != nil {
		return nil, err
	}
	var req pmEpicUpdateCommandInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse update epic input: %w", err)
	}
	epicID, err := resolvePMCommandEpicID(meta, req.EpicID, true)
	if err != nil {
		return nil, err
	}
	if !pmEpicUpdateHasEditableField(req) {
		return nil, fmt.Errorf("at least one editable field is required")
	}
	current, err := s.epicService.GetByID(ctx, epicID)
	if err != nil {
		return nil, err
	}
	if current == nil || current.Epic.WorkspaceID != meta.WorkspaceID {
		return nil, fmt.Errorf("epic not found")
	}
	if err := requireCommandAgentTeam(meta, current.Epic.TeamID); err != nil {
		return nil, err
	}
	update := model.UpdateEpicRequest{
		Name: req.Name, Description: req.Description, Color: req.Color, Health: req.Health, HealthComment: req.HealthComment,
	}
	effectiveTeamID := current.Epic.TeamID
	if req.TeamID != nil {
		update.TeamIDSet = true
		update.TeamID, err = s.validatePMCommandOptionalEpicTeam(ctx, meta.WorkspaceID, req.TeamID, true)
		if err != nil {
			return nil, err
		}
		effectiveTeamID = update.TeamID
	}
	if err := requireCommandAgentTeam(meta, effectiveTeamID); err != nil {
		return nil, err
	}
	if req.EpicStateID != nil {
		update.EpicStateIDSet = true
		update.EpicStateID, err = s.validatePMCommandOptionalEpicState(ctx, meta.WorkspaceID, req.EpicStateID)
		if err != nil {
			return nil, err
		}
	}
	if req.OwnerID != nil || req.OwnerMemberID != nil {
		update.OwnerSet = true
		update.OwnerID, update.OwnerMemberID, err = s.validatePMCommandEpicOwner(ctx, meta.WorkspaceID, req.OwnerID, req.OwnerMemberID)
		if err != nil {
			return nil, err
		}
	}
	if req.PlannedStartDate != nil {
		update.PlannedStartDateSet = true
		update.PlannedStartDate, err = parsePMCommandDateOnly(req.PlannedStartDate, "planned_start_date")
		if err != nil {
			return nil, err
		}
	}
	if req.Deadline != nil {
		update.DeadlineSet = true
		update.Deadline, err = parsePMCommandDateOnly(req.Deadline, "deadline")
		if err != nil {
			return nil, err
		}
	}
	if req.PlanningRepositoryID != nil {
		update.PlanningRepositoryIDSet = true
		update.PlanningRepositoryID, err = s.validatePMCommandOptionalPlanningRepository(ctx, meta.WorkspaceID, req.PlanningRepositoryID)
		if err != nil {
			return nil, err
		}
	}
	if req.LabelIDs != nil {
		update.LabelIDs, err = s.validatePMCommandEpicLabels(ctx, meta.WorkspaceID, *req.LabelIDs, effectiveTeamID)
		if err != nil {
			return nil, err
		}
	} else if req.TeamID != nil && len(current.Labels) > 0 {
		currentLabelIDs := make([]string, 0, len(current.Labels))
		for _, label := range current.Labels {
			currentLabelIDs = append(currentLabelIDs, label.ID)
		}
		if _, err := s.validatePMCommandEpicLabels(ctx, meta.WorkspaceID, currentLabelIDs, effectiveTeamID); err != nil {
			return nil, err
		}
	}
	epic, err := s.epicService.Update(ctx, epicID, update, fallbackActor(meta))
	if err != nil {
		return nil, err
	}
	return mustJSON(compactPMCommandEpic(epic, true)), nil
}

func (s *InternalCommandService) requirePMEpicCommandDependencies() error {
	if s == nil || s.epicService == nil || s.workspaceRepo == nil || s.workflowService == nil {
		return fmt.Errorf("epic command services are not configured")
	}
	return nil
}

func resolvePMCommandEpicID(meta model.InternalCommandContext, explicit string, rejectConflict bool) (string, error) {
	explicit = strings.TrimSpace(explicit)
	targetID := ""
	if strings.TrimSpace(meta.TargetType) == "epic" {
		targetID = strings.TrimSpace(meta.TargetID)
	}
	if rejectConflict && explicit != "" && targetID != "" && explicit != targetID {
		return "", fmt.Errorf("epic_id conflicts with the current epic target")
	}
	epicID := firstNonEmptyCommand(explicit, targetID)
	if epicID == "" {
		return "", fmt.Errorf("epic_id is required")
	}
	return epicID, nil
}

func parsePMCommandDateOnly(value *string, field string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(internalCommandDateOnlyLayout, strings.TrimSpace(*value))
	if err != nil {
		return nil, fmt.Errorf("%s must be YYYY-MM-DD", field)
	}
	return &parsed, nil
}

func (s *InternalCommandService) validatePMCommandEpicTeam(ctx context.Context, workspaceID, teamID string, write bool) (*string, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, nil
	}
	team, err := s.workspaceRepo.GetTeamByID(ctx, workspaceID, teamID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, fmt.Errorf("team not found in this workspace")
	}
	if write {
		if err := requireCanEditTeamEpics(ctx, &teamID); err != nil {
			return nil, err
		}
	} else if err := requireTeamAccess(ctx, &teamID); err != nil {
		return nil, fmt.Errorf("team not found in this workspace")
	}
	return &teamID, nil
}

func (s *InternalCommandService) validatePMCommandOptionalEpicTeam(ctx context.Context, workspaceID string, value *string, write bool) (*string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		if write {
			if err := requireCanEditTeamEpics(ctx, nil); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	return s.validatePMCommandEpicTeam(ctx, workspaceID, *value, write)
}

func (s *InternalCommandService) validatePMCommandEpicState(ctx context.Context, workspaceID, stateID string) error {
	states, err := s.workflowService.ListEpicStates(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, state := range states {
		if state.ID == strings.TrimSpace(stateID) {
			return nil
		}
	}
	return fmt.Errorf("epic state not found in this workspace")
}

func (s *InternalCommandService) validatePMCommandOptionalEpicState(ctx context.Context, workspaceID string, value *string) (*string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	stateID := strings.TrimSpace(*value)
	if err := s.validatePMCommandEpicState(ctx, workspaceID, stateID); err != nil {
		return nil, err
	}
	return &stateID, nil
}

func (s *InternalCommandService) validatePMCommandEpicOwner(ctx context.Context, workspaceID string, ownerID, ownerMemberID *string) (*string, *string, error) {
	var byUser, byMember *model.WorkspaceMember
	var err error
	if ownerID != nil && strings.TrimSpace(*ownerID) != "" {
		byUser, err = s.workspaceRepo.GetMembership(ctx, workspaceID, strings.TrimSpace(*ownerID))
		if err != nil {
			return nil, nil, err
		}
		if byUser == nil || byUser.Status != model.WorkspaceMemberStatusActive {
			return nil, nil, fmt.Errorf("owner not found in this workspace")
		}
	}
	if ownerMemberID != nil && strings.TrimSpace(*ownerMemberID) != "" {
		byMember, err = s.workspaceRepo.GetMembershipByID(ctx, workspaceID, strings.TrimSpace(*ownerMemberID))
		if err != nil {
			return nil, nil, err
		}
		if byMember == nil || byMember.Status != model.WorkspaceMemberStatusActive {
			return nil, nil, fmt.Errorf("owner member not found in this workspace")
		}
	}
	if byUser != nil && byMember != nil && byUser.ID != byMember.ID {
		return nil, nil, fmt.Errorf("owner_id and owner_member_id must reference the same workspace member")
	}
	var normalizedOwnerID, normalizedMemberID *string
	if byUser != nil {
		value := strings.TrimSpace(*ownerID)
		normalizedOwnerID = &value
	}
	if byMember != nil {
		value := strings.TrimSpace(*ownerMemberID)
		normalizedMemberID = &value
	}
	return normalizedOwnerID, normalizedMemberID, nil
}

func (s *InternalCommandService) validatePMCommandEpicLabel(ctx context.Context, workspaceID, labelID string) (*model.PMLabel, error) {
	if s.epicService.labelRepo == nil {
		return nil, fmt.Errorf("epic label repository is not configured")
	}
	label, err := s.epicService.labelRepo.GetByID(ctx, strings.TrimSpace(labelID))
	if err != nil {
		return nil, err
	}
	if label == nil || label.WorkspaceID != workspaceID || label.Archived {
		return nil, fmt.Errorf("label not found in this workspace")
	}
	return label, nil
}

func (s *InternalCommandService) validatePMCommandEpicLabels(ctx context.Context, workspaceID string, labelIDs []string, teamID *string) ([]string, error) {
	trimmed := make([]string, 0, len(labelIDs))
	seen := make(map[string]struct{}, len(labelIDs))
	for _, raw := range labelIDs {
		labelID := strings.TrimSpace(raw)
		if labelID == "" {
			continue
		}
		if _, ok := seen[labelID]; ok {
			continue
		}
		seen[labelID] = struct{}{}
		trimmed = append(trimmed, labelID)
	}
	if err := validateOperationalLabelScope(ctx, s.epicService.labelRepo, workspaceID, trimmed, allowedTeamIDs(teamID)); err != nil {
		return nil, err
	}
	if len(labelIDs) == 0 {
		return []string{}, nil
	}
	return trimmed, nil
}

func (s *InternalCommandService) validatePMCommandOptionalPlanningRepository(ctx context.Context, workspaceID string, value *string) (*string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	if s.gitService == nil {
		return nil, fmt.Errorf("git service is not configured")
	}
	repositoryID := strings.TrimSpace(*value)
	repo, err := s.gitService.GetEnabledRepositoryByID(ctx, workspaceID, repositoryID)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("planning repository not found in this workspace")
	}
	return &repositoryID, nil
}

func pmEpicUpdateHasEditableField(req pmEpicUpdateCommandInput) bool {
	return req.Name != nil || req.Description != nil || req.EpicStateID != nil || req.OwnerID != nil || req.OwnerMemberID != nil ||
		req.TeamID != nil || req.PlannedStartDate != nil || req.Deadline != nil || req.Color != nil || req.Health != nil ||
		req.HealthComment != nil || req.LabelIDs != nil || req.PlanningRepositoryID != nil
}

func compactPMCommandEpic(value *model.EpicWithStats, detailed bool) map[string]any {
	epic := value.Epic
	result := map[string]any{
		"epic_id": epic.ID, "markdown_link": helpinMarkdownLink(epic.Name, "epics", epic.ID), "name": epic.Name, "team_id": epic.TeamID, "epic_state_id": epic.EpicStateID,
		"owner_id": epic.OwnerID, "owner_member_id": epic.OwnerMemberID,
		"planned_start_date": formatPMCommandDateOnly(epic.PlannedStartDate), "deadline": formatPMCommandDateOnly(epic.Deadline),
		"color": epic.Color, "health": epic.Health, "started": epic.Started, "completed": epic.Completed,
		"archived": epic.Archived, "labels": compactPMCommandEpicLabels(value.Labels), "stats": value.Stats,
		"suggested_health": value.SuggestedHealth,
	}
	if detailed {
		result["description"] = epic.Description
		result["health_comment"] = epic.HealthComment
		result["planning_repository_id"] = epic.PlanningRepositoryID
		result["objectives"] = value.Objectives
	}
	return result
}

func compactPMCommandEpicLabels(labels []model.PMLabel) []map[string]any {
	result := make([]map[string]any, 0, len(labels))
	for _, label := range labels {
		result = append(result, map[string]any{"label_id": label.ID, "name": label.Name, "team_id": label.TeamID, "color": label.Color})
	}
	return result
}

func formatPMCommandDateOnly(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(internalCommandDateOnlyLayout)
}
