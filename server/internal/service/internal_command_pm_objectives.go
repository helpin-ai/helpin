package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var pmObjectiveCommandTargetTypes = []string{"workspace", "objective"}

const pmObjectiveCommandCollectionLimit = 100

type pmObjectiveCreateCommandInput struct {
	Name             string   `json:"name"`
	Description      *string  `json:"description"`
	ObjectiveType    string   `json:"objective_type"`
	State            *string  `json:"state"`
	PlannedStartDate *string  `json:"planned_start_date"`
	Deadline         *string  `json:"deadline"`
	Health           *string  `json:"health"`
	HealthComment    *string  `json:"health_comment"`
	TeamIDs          []string `json:"team_ids"`
	OwnerIDs         []string `json:"owner_ids"`
	OwnerMemberIDs   []string `json:"owner_member_ids"`
	LabelIDs         []string `json:"label_ids"`
	EpicIDs          []string `json:"epic_ids"`
}

type pmObjectiveUpdateCommandInput struct {
	ObjectiveID      string    `json:"objective_id"`
	Name             *string   `json:"name"`
	Description      *string   `json:"description"`
	ObjectiveType    *string   `json:"objective_type"`
	State            *string   `json:"state"`
	PlannedStartDate *string   `json:"planned_start_date"`
	Deadline         *string   `json:"deadline"`
	Health           *string   `json:"health"`
	HealthComment    *string   `json:"health_comment"`
	TeamIDs          *[]string `json:"team_ids"`
	OwnerIDs         *[]string `json:"owner_ids"`
	OwnerMemberIDs   *[]string `json:"owner_member_ids"`
	LabelIDs         *[]string `json:"label_ids"`
	EpicIDs          *[]string `json:"epic_ids"`
}

type pmCreateKeyResultCommandInput struct {
	Name         string  `json:"name"`
	ResultType   string  `json:"result_type"`
	InitialValue float64 `json:"initial_value"`
	CurrentValue float64 `json:"current_value"`
	TargetValue  float64 `json:"target_value"`
	Note         *string `json:"note"`
}

func (s *InternalCommandService) registerPMObjectiveCommands() {
	for _, item := range []struct {
		name     string
		mutating bool
		targets  []string
	}{
		{name: "pm.list_objectives", targets: pmObjectiveCommandTargetTypes},
		{name: "pm.get_objective", targets: pmObjectiveCommandTargetTypes},
		{name: "pm.create_objective", mutating: true, targets: pmObjectiveCommandTargetTypes},
		{name: "pm.update_objective", mutating: true, targets: pmObjectiveCommandTargetTypes},
		{name: "pm.create_key_result", mutating: true, targets: []string{"objective"}},
		{name: "pm.update_key_result", mutating: true, targets: []string{"objective"}},
	} {
		name := item.name
		s.register(InternalCommandDefinition{
			Name:                 name,
			Module:               "pm",
			Mutating:             item.mutating,
			SupportedTargetTypes: item.targets,
			Tool:                 mustCommandToolMetadata(name),
			Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
				return s.executePMObjectiveCommand(ctx, meta, name, input)
			},
		})
	}
}

func (s *InternalCommandService) executePMObjectiveCommand(ctx context.Context, meta model.InternalCommandContext, name string, input json.RawMessage) (json.RawMessage, error) {
	switch name {
	case "pm.list_objectives":
		return s.executePMListObjectives(ctx, meta, input)
	case "pm.get_objective":
		return s.executePMGetObjective(ctx, meta, input)
	case "pm.create_objective":
		return s.executePMCreateObjective(ctx, meta, input)
	case "pm.update_objective":
		return s.executePMUpdateObjective(ctx, meta, input)
	case "pm.create_key_result":
		return s.executePMCreateKeyResult(ctx, meta, input)
	case "pm.update_key_result":
		return s.executePMUpdateKeyResult(ctx, meta, input)
	default:
		return nil, fmt.Errorf("unsupported objective command %q", name)
	}
}

func (s *InternalCommandService) executePMListObjectives(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requirePMObjectiveCommandDependencies(false); err != nil {
		return nil, err
	}
	if _, err := commandAgentTeamFilter(meta); err != nil {
		return nil, err
	}
	var req struct {
		TeamID        *string `json:"team_id"`
		LabelID       *string `json:"label_id"`
		ObjectiveType *string `json:"objective_type"`
		State         *string `json:"state"`
		Archived      *bool   `json:"archived"`
		Page          int     `json:"page"`
		PerPage       int     `json:"per_page"`
	}
	if len(input) > 0 {
		if err := decodePMObjectiveCommandInput(input, &req); err != nil {
			return nil, fmt.Errorf("parse list objectives input: %w", err)
		}
	}
	page, perPage, err := normalizePMCommandPagination(req.Page, req.PerPage)
	if err != nil {
		return nil, err
	}
	if req.TeamID, err = s.validatePMObjectiveOptionalTeam(ctx, meta.WorkspaceID, req.TeamID); err != nil {
		return nil, err
	}
	if req.LabelID, err = s.validatePMObjectiveOptionalLabel(ctx, meta.WorkspaceID, req.LabelID); err != nil {
		return nil, err
	}
	if req.ObjectiveType != nil {
		value := strings.TrimSpace(*req.ObjectiveType)
		if value != model.PMObjectiveTypeTactical && value != model.PMObjectiveTypeStrategic {
			return nil, fmt.Errorf("invalid objective_type")
		}
		req.ObjectiveType = &value
	}
	if req.State != nil {
		value := strings.TrimSpace(*req.State)
		if value != model.PMObjectiveStateNotStarted && value != model.PMObjectiveStateActive && value != model.PMObjectiveStateClosed {
			return nil, fmt.Errorf("invalid state")
		}
		req.State = &value
	}
	objectives, total, err := s.objectiveService.ListPage(ctx, meta.WorkspaceID, model.PMObjectiveListFilters{
		TeamID: req.TeamID, LabelID: req.LabelID, ObjectiveType: req.ObjectiveType, State: req.State, Archived: req.Archived,
	}, model.PMPagination{Page: page, PerPage: perPage})
	if err != nil {
		return nil, fmt.Errorf("list objectives: %w", err)
	}
	items := make([]map[string]any, 0, len(objectives))
	for i := range objectives {
		items = append(items, compactPMCommandObjective(&objectives[i], false))
	}
	return mustJSON(map[string]any{"objectives": items, "total": total, "page": page, "per_page": perPage}), nil
}

func (s *InternalCommandService) executePMGetObjective(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requirePMObjectiveCommandDependencies(false); err != nil {
		return nil, err
	}
	if _, err := commandAgentTeamFilter(meta); err != nil {
		return nil, err
	}
	var req struct {
		ObjectiveID string `json:"objective_id"`
	}
	if len(input) > 0 {
		if err := decodePMObjectiveCommandInput(input, &req); err != nil {
			return nil, fmt.Errorf("parse get objective input: %w", err)
		}
	}
	objectiveID, err := resolvePMObjectiveCommandID(meta, req.ObjectiveID, false)
	if err != nil {
		return nil, err
	}
	objective, err := s.loadPMCommandObjective(ctx, meta, objectiveID, false)
	if err != nil {
		return nil, err
	}
	return mustJSON(compactPMCommandObjective(objective, true)), nil
}

func (s *InternalCommandService) executePMCreateObjective(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requirePMObjectiveCommandDependencies(true); err != nil {
		return nil, err
	}
	var req pmObjectiveCreateCommandInput
	if err := decodePMObjectiveCommandInput(input, &req); err != nil {
		return nil, fmt.Errorf("parse create objective input: %w", err)
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(req.ObjectiveType) == "" {
		return nil, fmt.Errorf("objective_type is required")
	}
	teamIDs, err := s.validatePMObjectiveTeams(ctx, meta.WorkspaceID, req.TeamIDs)
	if err != nil {
		return nil, err
	}
	if err := requireCanManageTeams(ctx, teamIDs); err != nil {
		return nil, err
	}
	if err := requireCommandAgentTeams(meta, teamIDs); err != nil {
		return nil, err
	}
	ownerIDs, ownerMemberIDs, err := s.validatePMObjectiveOwners(ctx, meta.WorkspaceID, req.OwnerIDs, req.OwnerMemberIDs)
	if err != nil {
		return nil, err
	}
	labelIDs, err := s.validatePMObjectiveLabels(ctx, meta.WorkspaceID, req.LabelIDs, teamIDs)
	if err != nil {
		return nil, err
	}
	epicIDs, err := s.validatePMObjectiveEpics(ctx, meta.WorkspaceID, req.EpicIDs)
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
	created, err := s.objectiveService.Create(ctx, model.CreateObjectiveRequest{
		WorkspaceID: meta.WorkspaceID, Name: req.Name, Description: req.Description, ObjectiveType: strings.TrimSpace(req.ObjectiveType),
		State: req.State, PlannedStartDate: plannedStartDate, Deadline: deadline, Health: req.Health, HealthComment: req.HealthComment,
		TeamIDs: teamIDs, OwnerIDs: ownerIDs, OwnerMemberIDs: ownerMemberIDs, LabelIDs: labelIDs, EpicIDs: epicIDs,
	}, fallbackActor(meta))
	if err != nil {
		return nil, fmt.Errorf("create objective: %w", err)
	}
	return mustJSON(compactPMCommandObjective(created, true)), nil
}

func (s *InternalCommandService) executePMUpdateObjective(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requirePMObjectiveCommandDependencies(true); err != nil {
		return nil, err
	}
	var req pmObjectiveUpdateCommandInput
	if err := decodePMObjectiveCommandInput(input, &req); err != nil {
		return nil, fmt.Errorf("parse update objective input: %w", err)
	}
	if !pmObjectiveUpdateHasEditableField(req) {
		return nil, fmt.Errorf("at least one editable field is required")
	}
	objectiveID, err := resolvePMObjectiveCommandID(meta, req.ObjectiveID, true)
	if err != nil {
		return nil, err
	}
	current, err := s.loadPMCommandObjective(ctx, meta, objectiveID, true)
	if err != nil {
		return nil, err
	}
	update := model.UpdateObjectiveRequest{
		Name: req.Name, Description: req.Description, ObjectiveType: req.ObjectiveType, State: req.State,
		Health: req.Health, HealthComment: req.HealthComment,
	}
	effectiveTeams := append([]string(nil), current.Teams...)
	if req.TeamIDs != nil {
		teamIDs, validateErr := s.validatePMObjectiveTeams(ctx, meta.WorkspaceID, *req.TeamIDs)
		if validateErr != nil {
			return nil, validateErr
		}
		if err := requireCanManageTeams(ctx, teamIDs); err != nil {
			return nil, err
		}
		if err := requireCommandAgentTeams(meta, teamIDs); err != nil {
			return nil, err
		}
		update.TeamIDs = teamIDs
		effectiveTeams = teamIDs
	}
	if req.OwnerIDs != nil || req.OwnerMemberIDs != nil {
		ownerIDs := derefStringSlice(req.OwnerIDs)
		ownerMemberIDs := derefStringSlice(req.OwnerMemberIDs)
		update.OwnerIDs, update.OwnerMemberIDs, err = s.validatePMObjectiveOwners(ctx, meta.WorkspaceID, ownerIDs, ownerMemberIDs)
		if err != nil {
			return nil, err
		}
		if req.OwnerIDs == nil {
			update.OwnerIDs = nil
		}
		if req.OwnerMemberIDs == nil {
			update.OwnerMemberIDs = nil
		}
		// At least one slice must remain non-nil so the service performs the replacement.
		if req.OwnerIDs != nil && update.OwnerIDs == nil {
			update.OwnerIDs = []string{}
		}
		if req.OwnerMemberIDs != nil && update.OwnerMemberIDs == nil {
			update.OwnerMemberIDs = []string{}
		}
	}
	if req.LabelIDs != nil {
		update.LabelIDs, err = s.validatePMObjectiveLabels(ctx, meta.WorkspaceID, *req.LabelIDs, effectiveTeams)
		if err != nil {
			return nil, err
		}
	} else if req.TeamIDs != nil {
		currentLabelIDs := make([]string, 0, len(current.Labels))
		for _, label := range current.Labels {
			currentLabelIDs = append(currentLabelIDs, label.ID)
		}
		if _, err := s.validatePMObjectiveLabels(ctx, meta.WorkspaceID, currentLabelIDs, effectiveTeams); err != nil {
			return nil, fmt.Errorf("existing objective labels are incompatible with destination teams: %w", err)
		}
	}
	if req.EpicIDs != nil {
		update.EpicIDs, err = s.validatePMObjectiveEpics(ctx, meta.WorkspaceID, *req.EpicIDs)
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
	updated, err := s.objectiveService.Update(ctx, objectiveID, update, fallbackActor(meta))
	if err != nil {
		return nil, fmt.Errorf("update objective: %w", err)
	}
	return mustJSON(compactPMCommandObjective(updated, true)), nil
}

func (s *InternalCommandService) executePMCreateKeyResult(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requirePMObjectiveCommandDependencies(false); err != nil {
		return nil, err
	}
	var req pmCreateKeyResultCommandInput
	if err := decodePMObjectiveCommandInput(input, &req); err != nil {
		return nil, fmt.Errorf("parse create key result input: %w", err)
	}
	if err := validatePMKeyResultNumbers(req.InitialValue, req.CurrentValue, req.TargetValue); err != nil {
		return nil, err
	}
	objectiveID, err := resolvePMObjectiveCommandID(meta, "", true)
	if err != nil {
		return nil, err
	}
	if _, err := s.loadPMCommandObjective(ctx, meta, objectiveID, true); err != nil {
		return nil, err
	}
	created, err := s.objectiveService.CreateKeyResult(ctx, objectiveID, model.CreateKeyResultRequest{
		Name: req.Name, ResultType: req.ResultType, InitialValue: req.InitialValue, CurrentValue: req.CurrentValue,
		TargetValue: req.TargetValue, Note: req.Note,
	}, fallbackActor(meta))
	if err != nil {
		return nil, fmt.Errorf("create key result: %w", err)
	}
	return mustJSON(compactPMCommandKeyResult(created)), nil
}

func (s *InternalCommandService) executePMUpdateKeyResult(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if err := s.requirePMObjectiveCommandDependencies(false); err != nil {
		return nil, err
	}
	var req struct {
		KeyResultID  string   `json:"key_result_id"`
		Name         *string  `json:"name"`
		ResultType   *string  `json:"result_type"`
		InitialValue *float64 `json:"initial_value"`
		CurrentValue *float64 `json:"current_value"`
		TargetValue  *float64 `json:"target_value"`
		Note         *string  `json:"note"`
	}
	if err := decodePMObjectiveCommandInput(input, &req); err != nil {
		return nil, fmt.Errorf("parse update key result input: %w", err)
	}
	if req.Name == nil && req.ResultType == nil && req.InitialValue == nil && req.CurrentValue == nil && req.TargetValue == nil && req.Note == nil {
		return nil, fmt.Errorf("at least one editable field is required")
	}
	for _, value := range []*float64{req.InitialValue, req.CurrentValue, req.TargetValue} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return nil, fmt.Errorf("key result numeric values must be finite")
		}
	}
	keyResultID := strings.TrimSpace(req.KeyResultID)
	if keyResultID == "" {
		return nil, fmt.Errorf("key_result_id is required")
	}
	objectiveID, err := resolvePMObjectiveCommandID(meta, "", true)
	if err != nil {
		return nil, err
	}
	objective, err := s.loadPMCommandObjective(ctx, meta, objectiveID, true)
	if err != nil {
		return nil, err
	}
	belongs := false
	for i := range objective.KeyResults {
		if objective.KeyResults[i].ID == keyResultID {
			belongs = true
			break
		}
	}
	if !belongs {
		return nil, fmt.Errorf("key result does not belong to current objective target")
	}
	updated, err := s.objectiveService.UpdateKeyResult(ctx, keyResultID, model.UpdateKeyResultRequest{
		Name: req.Name, ResultType: req.ResultType, InitialValue: req.InitialValue, CurrentValue: req.CurrentValue,
		TargetValue: req.TargetValue, Note: req.Note,
	}, fallbackActor(meta))
	if err != nil {
		return nil, fmt.Errorf("update key result: %w", err)
	}
	return mustJSON(compactPMCommandKeyResult(updated)), nil
}

func (s *InternalCommandService) requirePMObjectiveCommandDependencies(requireEpic bool) error {
	if s.objectiveService == nil {
		return fmt.Errorf("objective service is not configured")
	}
	if s.workspaceRepo == nil {
		return fmt.Errorf("workspace repository is not configured")
	}
	if requireEpic && s.epicService == nil {
		return fmt.Errorf("epic service is not configured")
	}
	return nil
}

func (s *InternalCommandService) loadPMCommandObjective(ctx context.Context, meta model.InternalCommandContext, objectiveID string, write bool) (*model.ObjectiveWithDetails, error) {
	objective, err := s.objectiveService.GetByID(ctx, objectiveID, meta.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("objective not found")
	}
	if write {
		if err := requireCanManageTeams(ctx, objective.Teams); err != nil {
			return nil, fmt.Errorf("objective not found")
		}
		if err := requireCommandAgentTeams(meta, objective.Teams); err != nil {
			return nil, err
		}
	}
	return objective, nil
}

func resolvePMObjectiveCommandID(meta model.InternalCommandContext, explicit string, rejectConflict bool) (string, error) {
	explicit = strings.TrimSpace(explicit)
	targetID := ""
	if strings.TrimSpace(meta.TargetType) == "objective" {
		targetID = strings.TrimSpace(meta.TargetID)
	}
	if rejectConflict && explicit != "" && targetID != "" && explicit != targetID {
		return "", fmt.Errorf("objective_id conflicts with the current objective target")
	}
	objectiveID := firstNonEmptyCommand(explicit, targetID)
	if objectiveID == "" {
		return "", fmt.Errorf("objective_id is required")
	}
	return objectiveID, nil
}

func (s *InternalCommandService) validatePMObjectiveOptionalTeam(ctx context.Context, workspaceID string, value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	teamIDs, err := s.validatePMObjectiveTeams(ctx, workspaceID, []string{*value})
	if err != nil {
		return nil, err
	}
	return &teamIDs[0], nil
}

func (s *InternalCommandService) validatePMObjectiveTeams(ctx context.Context, workspaceID string, values []string) ([]string, error) {
	teamIDs, err := normalizePMObjectiveCommandIDs(values, "team")
	if err != nil {
		return nil, err
	}
	for _, teamID := range teamIDs {
		team, err := s.workspaceRepo.GetTeamByID(ctx, workspaceID, teamID)
		if err != nil {
			return nil, err
		}
		if team == nil {
			return nil, fmt.Errorf("team not found in this workspace")
		}
	}
	return teamIDs, nil
}

func (s *InternalCommandService) validatePMObjectiveOwners(ctx context.Context, workspaceID string, ownerIDs, ownerMemberIDs []string) ([]string, []string, error) {
	ownerIDs, err := normalizePMObjectiveCommandIDs(ownerIDs, "owner")
	if err != nil {
		return nil, nil, err
	}
	ownerMemberIDs, err = normalizePMObjectiveCommandIDs(ownerMemberIDs, "owner member")
	if err != nil {
		return nil, nil, err
	}
	if _, err := resolveWorkspaceMemberReferences(ctx, s.workspaceRepo, workspaceID, ownerMemberIDs, ownerIDs); err != nil {
		return nil, nil, fmt.Errorf("objective owner not found in this workspace: %w", err)
	}
	return ownerIDs, ownerMemberIDs, nil
}

func (s *InternalCommandService) validatePMObjectiveOptionalLabel(ctx context.Context, workspaceID string, value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	labelID := strings.TrimSpace(*value)
	if labelID == "" {
		return nil, fmt.Errorf("label_id is required when supplied")
	}
	label, err := s.objectiveService.labelRepo.GetByID(ctx, labelID)
	if err != nil {
		return nil, err
	}
	if label == nil || label.WorkspaceID != workspaceID || label.Archived {
		return nil, fmt.Errorf("label not found in this workspace")
	}
	return &labelID, nil
}

func (s *InternalCommandService) validatePMObjectiveLabels(ctx context.Context, workspaceID string, values, teamIDs []string) ([]string, error) {
	labelIDs, err := normalizePMObjectiveCommandIDs(values, "label")
	if err != nil {
		return nil, err
	}
	if err := validateOperationalLabelScope(ctx, s.objectiveService.labelRepo, workspaceID, labelIDs, teamIDs); err != nil {
		return nil, fmt.Errorf("validate objective labels: %w", err)
	}
	return labelIDs, nil
}

func (s *InternalCommandService) validatePMObjectiveEpics(ctx context.Context, workspaceID string, values []string) ([]string, error) {
	epicIDs, err := normalizePMObjectiveCommandIDs(values, "epic")
	if err != nil {
		return nil, err
	}
	for _, epicID := range epicIDs {
		epic, err := s.epicService.GetByID(ctx, epicID)
		if err != nil || epic == nil || epic.Epic.WorkspaceID != workspaceID || epic.Epic.Archived {
			return nil, fmt.Errorf("epic not found in this workspace")
		}
	}
	return epicIDs, nil
}

func normalizePMObjectiveCommandIDs(values []string, field string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, fmt.Errorf("%s ID must not be empty", field)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func pmObjectiveUpdateHasEditableField(req pmObjectiveUpdateCommandInput) bool {
	return req.Name != nil || req.Description != nil || req.ObjectiveType != nil || req.State != nil || req.PlannedStartDate != nil ||
		req.Deadline != nil || req.Health != nil || req.HealthComment != nil || req.TeamIDs != nil || req.OwnerIDs != nil ||
		req.OwnerMemberIDs != nil || req.LabelIDs != nil || req.EpicIDs != nil
}

func derefStringSlice(value *[]string) []string {
	if value == nil {
		return nil
	}
	return *value
}

func compactPMCommandObjective(value *model.ObjectiveWithDetails, detailed bool) map[string]any {
	objective := value.Objective
	result := map[string]any{
		"objective_id": objective.ID, "workspace_id": objective.WorkspaceID, "name": objective.Name,
		"objective_type": objective.ObjectiveType, "state": objective.State,
		"planned_start_date": formatPMObjectiveCommandDate(objective.PlannedStartDate), "deadline": formatPMObjectiveCommandDate(objective.Deadline),
		"health": objective.Health, "archived": objective.Archived, "teams": boundedPMObjectiveCommandStrings(value.Teams),
		"owners": boundedPMObjectiveCommandStrings(value.Owners), "owner_member_ids": boundedPMObjectiveCommandStrings(value.OwnerMemberIDs),
		"labels": compactPMCommandObjectiveLabels(value.Labels), "epics": compactPMCommandObjectiveEpics(value.Epics),
		"stats": value.Stats, "suggested_health": value.SuggestedHealth,
	}
	if detailed {
		result["description"] = derefString(value.Objective.Description)
		result["health_comment"] = derefString(value.Objective.HealthComment)
		limit := min(len(value.KeyResults), pmObjectiveCommandCollectionLimit)
		keyResults := make([]map[string]any, 0, limit)
		for i := range value.KeyResults[:limit] {
			keyResults = append(keyResults, compactPMCommandKeyResult(&value.KeyResults[i]))
		}
		result["key_results"] = keyResults
	}
	return result
}

func compactPMCommandObjectiveLabels(labels []model.PMLabel) []map[string]any {
	limit := min(len(labels), pmObjectiveCommandCollectionLimit)
	result := make([]map[string]any, 0, limit)
	for _, label := range labels[:limit] {
		result = append(result, map[string]any{"label_id": label.ID, "name": label.Name, "team_id": label.TeamID, "color": label.Color})
	}
	return result
}

func compactPMCommandObjectiveEpics(epics []model.EpicWithStats) []map[string]any {
	limit := min(len(epics), pmObjectiveCommandCollectionLimit)
	result := make([]map[string]any, 0, limit)
	for i := range epics[:limit] {
		result = append(result, compactPMCommandEpic(&epics[i], false))
	}
	return result
}

func compactPMCommandKeyResult(value *model.PMKeyResult) map[string]any {
	return map[string]any{
		"key_result_id": value.ID, "objective_id": value.ObjectiveID, "name": value.Name, "result_type": value.ResultType,
		"initial_value": value.InitialValue, "current_value": value.CurrentValue, "target_value": value.TargetValue,
		"progress": value.Progress, "note": derefString(value.Note), "position": value.Position,
	}
}

func formatPMObjectiveCommandDate(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(internalCommandDateOnlyLayout)
}

func nonNilCommandStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func boundedPMObjectiveCommandStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values[:min(len(values), pmObjectiveCommandCollectionLimit)]...)
}

func decodePMObjectiveCommandInput(input json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func validatePMKeyResultNumbers(values ...float64) error {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("key result numeric values must be finite")
		}
	}
	return nil
}
