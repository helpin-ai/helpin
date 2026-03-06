package service

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type PMImportService struct {
	db            *gorm.DB
	workspaceRepo *repository.WorkspaceRepository
	workflowRepo  *repository.PMWorkflowRepository
}

var shortcutImportLabelColors = []string{
	"#3b82f6",
	"#16a34a",
	"#ec4899",
	"#64748b",
	"#ef4444",
	"#f97316",
	"#eab308",
	"#14b8a6",
	"#8b5cf6",
	"#6366f1",
	"#06b6d4",
	"#d946ef",
	"#84cc16",
	"#f43f5e",
	"#0ea5e9",
	"#a855f7",
}

func NewPMImportService(db *gorm.DB, workspaceRepo *repository.WorkspaceRepository, workflowRepo *repository.PMWorkflowRepository) *PMImportService {
	return &PMImportService{
		db:            db,
		workspaceRepo: workspaceRepo,
		workflowRepo:  workflowRepo,
	}
}

func (s *PMImportService) PreviewShortcut(ctx context.Context, workspaceID, actorID string, csvData []byte) (*model.ShortcutImportPreviewResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	data, err := parseShortcutCSV(csvData)
	if err != nil {
		return nil, err
	}

	members, err := s.workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	memberByEmail := make(map[string]model.MemberWithUser, len(members))
	for _, member := range members {
		memberByEmail[normalizeShortcutName(member.Email)] = member
	}

	storyTypeCounts := map[string]int{}
	epicIDs := map[string]struct{}{}
	objectiveIDs := map[string]struct{}{}
	sprintIDs := map[string]struct{}{}
	labelNames := map[string]struct{}{}
	teamCounts := map[string]int{}
	workflowStateCounts := map[string]map[string]int{}
	emails := map[string]struct{}{}
	checklistCount := 0
	for _, row := range data.Rows {
		storyTypeCounts[row.Type]++
		if row.EpicID != "" {
			epicIDs[row.EpicID] = struct{}{}
		}
		if row.ObjectiveID != "" {
			objectiveIDs[row.ObjectiveID] = struct{}{}
		}
		if row.IterationID != "" {
			sprintIDs[row.IterationID] = struct{}{}
		}
		for _, label := range shortcutLabelNames(row.Labels) {
			labelNames[normalizeShortcutName(label)] = struct{}{}
		}
		for _, label := range shortcutLabelNames(row.EpicLabels) {
			labelNames[normalizeShortcutName(label)] = struct{}{}
		}
		if row.Team != "" {
			teamCounts[row.Team]++
		}
		if workflowStateCounts[row.Workflow] == nil {
			workflowStateCounts[row.Workflow] = map[string]int{}
		}
		workflowStateCounts[row.Workflow][row.State]++
		if row.Requester != "" {
			emails[normalizeShortcutName(row.Requester)] = struct{}{}
		}
		for _, owner := range shortcutOwnerEmails(row.Owners) {
			emails[normalizeShortcutName(owner)] = struct{}{}
		}
		checklistCount += len(parseShortcutChecklist(row.Tasks))
	}

	duplicateStories, err := s.countStoryDuplicates(ctx, workspaceID, mapKeys(storyExternalIDs(data.Rows)))
	if err != nil {
		return nil, err
	}

	users := make([]model.ShortcutUserMatch, 0, len(emails))
	for _, email := range sortedSetKeys(emails) {
		match := model.ShortcutUserMatch{Email: email}
		if member, ok := memberByEmail[email]; ok {
			match.MatchedUserID = &member.UserID
			match.MatchedName = &member.FullName
		}
		users = append(users, match)
	}

	teams := make([]model.ShortcutTeamPreview, 0, len(teamCounts))
	for _, name := range sortKeysByCount(teamCounts) {
		teams = append(teams, model.ShortcutTeamPreview{Name: name, StoryCount: teamCounts[name]})
	}

	workflows := make([]model.ShortcutWorkflowPreview, 0, len(workflowStateCounts))
	workflowNames := make([]string, 0, len(workflowStateCounts))
	for name := range workflowStateCounts {
		workflowNames = append(workflowNames, name)
	}
	sort.Strings(workflowNames)
	stateTypeOrder := map[string]int{
		model.PMStateTypeBacklog:   0,
		model.PMStateTypeUnstarted: 1,
		model.PMStateTypeStarted:   2,
		model.PMStateTypeDone:      3,
	}
	for _, workflowName := range workflowNames {
		stateCounts := workflowStateCounts[workflowName]
		states := make([]model.ShortcutWorkflowStatePreview, 0, len(stateCounts))
		total := 0
		for _, stateName := range sortKeysByCount(stateCounts) {
			total += stateCounts[stateName]
			states = append(states, model.ShortcutWorkflowStatePreview{
				Name:          stateName,
				SuggestedType: suggestedStateType(stateName),
				StoryCount:    stateCounts[stateName],
			})
		}
		// Sort states by logical workflow order (backlog → unstarted → started → done)
		sort.SliceStable(states, func(i, j int) bool {
			return stateTypeOrder[states[i].SuggestedType] < stateTypeOrder[states[j].SuggestedType]
		})
		workflows = append(workflows, model.ShortcutWorkflowPreview{
			Name:       workflowName,
			StoryCount: total,
			States:     states,
		})
	}

	return &model.ShortcutImportPreviewResponse{
		Summary: model.ShortcutImportPreviewSummary{
			TotalStories:        len(data.Rows),
			StoriesByType:       storyTypeCounts,
			EpicsCount:          len(epicIDs),
			ObjectivesCount:     len(objectiveIDs),
			SprintsCount:        len(sprintIDs),
			LabelsCount:         len(labelNames),
			TeamsCount:          len(teamCounts),
			WorkflowsCount:      len(workflowStateCounts),
			WorkflowStatesCount: countWorkflowStates(workflowStateCounts),
			ChecklistItemsCount: checklistCount,
			DuplicateStories:    duplicateStories,
		},
		Users:     users,
		Teams:     teams,
		Workflows: workflows,
		Warnings:  append([]string(nil), data.Warnings...),
	}, nil
}

func (s *PMImportService) ExecuteShortcut(ctx context.Context, workspaceID, actorID, fileName string, csvData []byte, req model.ShortcutImportExecuteRequest) (*model.ShortcutImportExecuteResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	if _, err := parseShortcutCSV(csvData); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	job := &model.PMImportJob{
		WorkspaceID: workspaceID,
		Source:      model.PMImportSourceShortcut,
		Status:      model.PMImportStatusPending,
		FileName:    fileName,
		StartedBy:   actorID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, fmt.Errorf("create import job: %w", err)
	}

	go s.runShortcutImport(job.ID, workspaceID, actorID, csvData, req)

	return &model.ShortcutImportExecuteResponse{
		ImportID: job.ID,
		Status:   model.PMImportStatusProcessing,
	}, nil
}

func (s *PMImportService) GetShortcutStatus(ctx context.Context, workspaceID, actorID, importID string) (*model.ShortcutImportStatusResponse, error) {
	if err := s.requireWorkspaceAdmin(ctx, workspaceID, actorID); err != nil {
		return nil, err
	}
	var job model.PMImportJob
	if err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND source = ?", importID, workspaceID, model.PMImportSourceShortcut).
		First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("import job not found")
		}
		return nil, fmt.Errorf("get import job: %w", err)
	}
	resp := &model.ShortcutImportStatusResponse{
		ImportID: job.ID,
		Status:   job.Status,
		Progress: model.ShortcutImportStatusProgress{
			CurrentStep:       job.CurrentStep,
			StepsCompleted:    job.StepsCompleted,
			StepsTotal:        job.StepsTotal,
			EntitiesProcessed: job.EntitiesProcessed,
			EntitiesTotal:     job.EntitiesTotal,
		},
		Error: job.Error,
	}
	if job.Result != nil && *job.Result != "" {
		var result model.ShortcutImportResult
		if err := json.Unmarshal([]byte(*job.Result), &result); err == nil {
			resp.Result = &result
		}
	}
	return resp, nil
}

func (s *PMImportService) runShortcutImport(jobID, workspaceID, actorID string, csvData []byte, req model.ShortcutImportExecuteRequest) {
	ctx := context.Background()
	if err := s.updateJob(ctx, jobID, map[string]interface{}{
		"status":             model.PMImportStatusProcessing,
		"current_step":       "parse",
		"steps_total":        8,
		"updated_at":         time.Now().UTC(),
		"entities_total":     0,
		"entities_processed": 0,
	}); err != nil {
		return
	}

	result, totalRows, err := s.executeShortcutImport(ctx, workspaceID, actorID, csvData, req, jobID)
	if err != nil {
		errText := err.Error()
		_ = s.updateJob(ctx, jobID, map[string]interface{}{
			"status":       model.PMImportStatusFailed,
			"error":        &errText,
			"completed_at": timePtr(time.Now().UTC()),
			"updated_at":   time.Now().UTC(),
		})
		return
	}
	raw, _ := json.Marshal(result)
	rawStr := string(raw)
	completed := time.Now().UTC()
	_ = s.updateJob(ctx, jobID, map[string]interface{}{
		"status":             model.PMImportStatusCompleted,
		"total_rows":         totalRows,
		"progress":           100,
		"current_step":       "completed",
		"steps_completed":    8,
		"entities_processed": totalRows,
		"result":             &rawStr,
		"completed_at":       &completed,
		"updated_at":         completed,
	})
}

func (s *PMImportService) executeShortcutImport(ctx context.Context, workspaceID, actorID string, csvData []byte, req model.ShortcutImportExecuteRequest, jobID string) (*model.ShortcutImportResult, int, error) {
	data, err := parseShortcutCSV(csvData)
	if err != nil {
		return nil, 0, err
	}
	rows := filterShortcutRows(data.Rows, req.Options)
	_ = s.updateJob(ctx, jobID, map[string]interface{}{
		"total_rows":     len(rows),
		"entities_total": len(rows),
		"updated_at":     time.Now().UTC(),
	})

	members, err := s.workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, 0, err
	}
	userByEmail := make(map[string]string, len(members))
	for _, member := range members {
		userByEmail[normalizeShortcutName(member.Email)] = member.UserID
	}
	for email, userID := range req.UserMappings {
		if strings.TrimSpace(userID) != "" {
			userByEmail[normalizeShortcutName(email)] = userID
		}
	}

	result := &model.ShortcutImportResult{
		Warnings: append([]string(nil), data.Warnings...),
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		teamMap, teamsCreated, err := s.ensureTeams(ctx, tx, workspaceID, rows)
		if err != nil {
			return err
		}
		result.TeamsCreated = teamsCreated
		_ = s.markStep(ctx, jobID, "teams", 1, 0)

		workflowMap, stateMap, workflowsCreated, statesCreated, err := s.resolveWorkflowMappings(ctx, tx, workspaceID, req.WorkflowStateMappings)
		if err != nil {
			return err
		}
		result.WorkflowsCreated = workflowsCreated
		result.WorkflowStatesCreated = statesCreated
		_ = s.markStep(ctx, jobID, "workflows", 2, 0)

		labelMap, labelsCreated, err := s.ensureLabels(ctx, tx, workspaceID, rows)
		if err != nil {
			return err
		}
		result.LabelsCreated = labelsCreated
		_ = s.markStep(ctx, jobID, "labels", 3, 0)

		objectiveMap, objectivesCreated, err := s.ensureObjectives(ctx, tx, workspaceID, rows)
		if err != nil {
			return err
		}
		result.ObjectivesCreated = objectivesCreated
		_ = s.markStep(ctx, jobID, "objectives", 4, 0)

		epicMap, epicsCreated, err := s.ensureEpics(ctx, tx, workspaceID, rows, objectiveMap, labelMap, teamMap)
		if err != nil {
			return err
		}
		result.EpicsCreated = epicsCreated
		_ = s.markStep(ctx, jobID, "epics", 5, 0)

		sprintMap, sprintsCreated, sprintWarnings, err := s.ensureSprints(ctx, tx, workspaceID, rows, teamMap)
		if err != nil {
			return err
		}
		result.SprintsCreated = sprintsCreated
		result.Warnings = append(result.Warnings, sprintWarnings...)
		_ = s.markStep(ctx, jobID, "sprints", 6, 0)

		if err := s.createStories(ctx, tx, workspaceID, rows, workflowMap, stateMap, teamMap, userByEmail, epicMap, sprintMap, labelMap, result, jobID); err != nil {
			return err
		}
		_ = s.markStep(ctx, jobID, "stories", 7, len(rows))
		return nil
	})
	if err != nil {
		return nil, len(rows), err
	}
	return result, len(rows), nil
}

func (s *PMImportService) ensureTeams(ctx context.Context, tx *gorm.DB, workspaceID string, rows []shortcutCSVRow) (map[string]string, int, error) {
	needed := map[string]string{}
	for _, row := range rows {
		if row.Team == "" {
			continue
		}
		needed[normalizeShortcutName(row.Team)] = row.Team
	}

	var existing []model.WorkspaceTeam
	if err := tx.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&existing).Error; err != nil {
		return nil, 0, fmt.Errorf("list teams: %w", err)
	}
	teamMap := map[string]string{}
	for _, team := range existing {
		teamMap[normalizeShortcutName(team.Name)] = team.ID
	}
	created := 0
	for key, name := range needed {
		if _, ok := teamMap[key]; ok {
			continue
		}
		team := model.WorkspaceTeam{WorkspaceID: workspaceID, Name: name}
		if err := tx.WithContext(ctx).Create(&team).Error; err != nil {
			return nil, 0, fmt.Errorf("create team: %w", err)
		}
		teamMap[key] = team.ID
		created++
	}
	return teamMap, created, nil
}

func (s *PMImportService) resolveWorkflowMappings(ctx context.Context, tx *gorm.DB, workspaceID string, mappings []model.ShortcutWorkflowStateMappingPayload) (map[string]string, map[string]string, int, int, error) {
	workflowMap := map[string]string{}
	shortcutStateMap := map[string]string{}
	var existingWorkflows []model.PMWorkflow
	if err := tx.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&existingWorkflows).Error; err != nil {
		return nil, nil, 0, 0, fmt.Errorf("list workflows: %w", err)
	}
	workflowByName := map[string]model.PMWorkflow{}
	for _, workflow := range existingWorkflows {
		workflowByName[normalizeShortcutName(workflow.Name)] = workflow
	}
	workflowsCreated := 0
	statesCreated := 0

	for _, mapping := range mappings {
		key := normalizeShortcutName(mapping.ShortcutWorkflowName)
		switch mapping.Mode {
		case "create_new":
			name := strings.TrimSpace(mapping.NewWorkflowName)
			if name == "" {
				name = mapping.ShortcutWorkflowName
			}
			existing, ok := workflowByName[normalizeShortcutName(name)]
			if !ok {
				existing = model.PMWorkflow{WorkspaceID: workspaceID, Name: name}
				if err := tx.WithContext(ctx).Create(&existing).Error; err != nil {
					return nil, nil, 0, 0, fmt.Errorf("create workflow: %w", err)
				}
				workflowByName[normalizeShortcutName(name)] = existing
				workflowsCreated++
			}
			workflowMap[key] = existing.ID

			var states []model.PMWorkflowState
			if err := tx.WithContext(ctx).Where("workflow_id = ?", existing.ID).Find(&states).Error; err != nil {
				return nil, nil, 0, 0, fmt.Errorf("list workflow states: %w", err)
			}
			stateByName := map[string]model.PMWorkflowState{}
			for _, state := range states {
				stateByName[normalizeShortcutName(state.Name)] = state
			}
			// Sort states by logical workflow order before assigning positions
			stateTypePos := map[string]int{
				model.PMStateTypeBacklog:   0,
				model.PMStateTypeUnstarted: 1,
				model.PMStateTypeStarted:   2,
				model.PMStateTypeDone:      3,
			}
			sort.SliceStable(mapping.States, func(i, j int) bool {
				return stateTypePos[mapping.States[i].StateType] < stateTypePos[mapping.States[j].StateType]
			})
			var defaultStateID *string
			for idx, stateMapping := range mapping.States {
				name := strings.TrimSpace(stateMapping.NewStateName)
				if name == "" {
					name = stateMapping.ShortcutState
				}
				state, ok := stateByName[normalizeShortcutName(name)]
				if ok {
					if err := tx.WithContext(ctx).Model(&model.PMWorkflowState{}).Where("id = ?", state.ID).Updates(map[string]interface{}{
						"state_type": stateMapping.StateType,
						"position":   idx,
					}).Error; err != nil {
						return nil, nil, 0, 0, fmt.Errorf("update workflow state: %w", err)
					}
				} else {
					state = model.PMWorkflowState{
						WorkflowID: existing.ID,
						Name:       name,
						StateType:  stateMapping.StateType,
						Position:   idx,
					}
					if err := tx.WithContext(ctx).Create(&state).Error; err != nil {
						return nil, nil, 0, 0, fmt.Errorf("create workflow state: %w", err)
					}
					statesCreated++
				}
				if defaultStateID == nil && stateMapping.StateType != model.PMStateTypeDone {
					defaultStateID = &state.ID
				}
				shortcutStateMap[key+"::"+normalizeShortcutName(stateMapping.ShortcutState)] = state.ID
			}
			if defaultStateID != nil {
				if err := tx.WithContext(ctx).Model(&model.PMWorkflow{}).Where("id = ?", existing.ID).Update("default_state_id", *defaultStateID).Error; err != nil {
					return nil, nil, 0, 0, fmt.Errorf("set workflow default state: %w", err)
				}
			}
		case "use_existing":
			var workflow model.PMWorkflow
			if err := tx.WithContext(ctx).Where("id = ? AND workspace_id = ?", mapping.ExistingWorkflowID, workspaceID).First(&workflow).Error; err != nil {
				return nil, nil, 0, 0, fmt.Errorf("existing workflow not found for %s", mapping.ShortcutWorkflowName)
			}
			workflowMap[key] = workflow.ID
			var states []model.PMWorkflowState
			if err := tx.WithContext(ctx).Where("workflow_id = ?", workflow.ID).Find(&states).Error; err != nil {
				return nil, nil, 0, 0, fmt.Errorf("list existing workflow states: %w", err)
			}
			validStates := map[string]struct{}{}
			for _, state := range states {
				validStates[state.ID] = struct{}{}
			}
			for _, stateMapping := range mapping.States {
				if _, ok := validStates[stateMapping.ExistingStateID]; !ok {
					return nil, nil, 0, 0, fmt.Errorf("state %s does not belong to workflow %s", stateMapping.ExistingStateID, workflow.ID)
				}
				shortcutStateMap[key+"::"+normalizeShortcutName(stateMapping.ShortcutState)] = stateMapping.ExistingStateID
			}
		default:
			return nil, nil, 0, 0, fmt.Errorf("unsupported workflow mapping mode: %s", mapping.Mode)
		}
	}
	return workflowMap, shortcutStateMap, workflowsCreated, statesCreated, nil
}

func (s *PMImportService) ensureLabels(ctx context.Context, tx *gorm.DB, workspaceID string, rows []shortcutCSVRow) (map[string]string, int, error) {
	needed := map[string]string{}
	for _, row := range rows {
		for _, label := range append(shortcutLabelNames(row.Labels), shortcutLabelNames(row.EpicLabels)...) {
			needed[normalizeShortcutName(label)] = label
		}
	}
	var existing []model.PMLabel
	if err := tx.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&existing).Error; err != nil {
		return nil, 0, fmt.Errorf("list labels: %w", err)
	}
	labelMap := map[string]string{}
	for _, label := range existing {
		if label.Color == nil || strings.TrimSpace(*label.Color) == "" {
			color := shortcutImportLabelColor(label.Name)
			if err := tx.WithContext(ctx).Model(&model.PMLabel{}).Where("id = ?", label.ID).Update("color", *color).Error; err != nil {
				return nil, 0, fmt.Errorf("backfill label color: %w", err)
			}
		}
		labelMap[normalizeShortcutName(label.Name)] = label.ID
	}
	created := 0
	for key, name := range needed {
		if _, ok := labelMap[key]; ok {
			continue
		}
		label := model.PMLabel{WorkspaceID: workspaceID, Name: name, Color: shortcutImportLabelColor(name)}
		if err := tx.WithContext(ctx).Create(&label).Error; err != nil {
			return nil, 0, fmt.Errorf("create label: %w", err)
		}
		labelMap[key] = label.ID
		created++
	}
	return labelMap, created, nil
}

func shortcutImportLabelColor(name string) *string {
	if len(shortcutImportLabelColors) == 0 {
		return nil
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(normalizeShortcutName(name)))
	color := shortcutImportLabelColors[hasher.Sum32()%uint32(len(shortcutImportLabelColors))]
	return &color
}

func (s *PMImportService) ensureObjectives(ctx context.Context, tx *gorm.DB, workspaceID string, rows []shortcutCSVRow) (map[string]string, int, error) {
	grouped := map[string]shortcutCSVRow{}
	for _, row := range rows {
		if row.ObjectiveID != "" {
			if _, ok := grouped[row.ObjectiveID]; !ok {
				grouped[row.ObjectiveID] = row
			}
		}
	}
	existingMap, err := s.lookupObjectivesByExternalID(ctx, tx, workspaceID, mapKeys(grouped))
	if err != nil {
		return nil, 0, err
	}
	created := 0
	for externalID, row := range grouped {
		if _, ok := existingMap[externalID]; ok {
			continue
		}
		state := model.PMObjectiveStateNotStarted
		switch normalizeShortcutName(row.ObjectiveState) {
		case "in progress":
			state = model.PMObjectiveStateActive
		case "done":
			state = model.PMObjectiveStateClosed
		}
		ext := externalID
		obj := model.PMObjective{
			WorkspaceID:      workspaceID,
			Name:             fallbackName(row.Objective, fmt.Sprintf("Objective %s", externalID)),
			ExternalID:       &ext,
			State:            state,
			PlannedStartDate: parseShortcutTimestamp(row.ObjectiveStartedAt, row.UTCOffset),
			Deadline:         parseShortcutTimestamp(row.ObjectiveDueDate, row.UTCOffset),
			CreatedAt:        valueOrNow(parseShortcutTimestamp(row.ObjectiveCreatedAt, row.UTCOffset)),
			UpdatedAt:        valueOrNow(parseShortcutTimestamp(row.ObjectiveCreatedAt, row.UTCOffset)),
		}
		if err := tx.WithContext(ctx).Create(&obj).Error; err != nil {
			return nil, 0, fmt.Errorf("create objective: %w", err)
		}
		existingMap[externalID] = obj.ID
		created++
	}
	return existingMap, created, nil
}

func (s *PMImportService) ensureEpics(ctx context.Context, tx *gorm.DB, workspaceID string, rows []shortcutCSVRow, objectiveMap, labelMap, teamMap map[string]string) (map[string]string, int, error) {
	grouped := map[string][]shortcutCSVRow{}
	for _, row := range rows {
		if row.EpicID != "" {
			grouped[row.EpicID] = append(grouped[row.EpicID], row)
		}
	}
	existingMap, err := s.lookupEpicsByExternalID(ctx, tx, workspaceID, mapKeys(grouped))
	if err != nil {
		return nil, 0, err
	}
	epicStates, err := s.ensureEpicWorkflowStates(ctx, tx, workspaceID)
	if err != nil {
		return nil, 0, err
	}
	var startedStateID, doneStateID *string
	for _, state := range epicStates {
		switch state.StateType {
		case model.PMStateTypeStarted:
			if startedStateID == nil {
				startedStateID = &state.ID
			}
		case model.PMStateTypeDone:
			if doneStateID == nil {
				doneStateID = &state.ID
			}
		}
	}
	created := 0
	for externalID, rowsForEpic := range grouped {
		if _, ok := existingMap[externalID]; ok {
			continue
		}
		row := rowsForEpic[0]
		ext := externalID
		teamID := consistentTeamID(rowsForEpic, teamMap)
		epic := model.PMEpic{
			WorkspaceID:      workspaceID,
			Name:             fallbackName(row.Epic, fmt.Sprintf("Epic %s", externalID)),
			ExternalID:       &ext,
			TeamID:           teamID,
			PlannedStartDate: parseShortcutTimestamp(row.EpicPlannedStartDate, row.UTCOffset),
			Deadline:         parseShortcutTimestamp(row.EpicDueDate, row.UTCOffset),
			StartedAt:        parseShortcutTimestamp(row.EpicStartedAt, row.UTCOffset),
			Archived:         parseShortcutBool(row.EpicIsArchived),
			CreatedAt:        valueOrNow(parseShortcutTimestamp(row.EpicCreatedAt, row.UTCOffset)),
			UpdatedAt:        valueOrNow(parseShortcutTimestamp(row.EpicCreatedAt, row.UTCOffset)),
		}
		switch normalizeShortcutName(row.EpicState) {
		case "in progress":
			epic.Started = true
			epic.EpicStateID = startedStateID
		case "done":
			epic.Started = true
			epic.Completed = true
			epic.EpicStateID = doneStateID
		default:
			epic.EpicStateID = startedStateID
		}
		if err := tx.WithContext(ctx).Create(&epic).Error; err != nil {
			return nil, 0, fmt.Errorf("create epic: %w", err)
		}
		existingMap[externalID] = epic.ID
		created++
	}
	for _, rowsForEpic := range grouped {
		row := rowsForEpic[0]
		epicID := existingMap[row.EpicID]
		if row.ObjectiveID != "" {
			if objectiveID, ok := objectiveMap[row.ObjectiveID]; ok {
				if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PMEpicObjective{
					EpicID:      epicID,
					ObjectiveID: objectiveID,
				}).Error; err != nil {
					return nil, 0, fmt.Errorf("link epic objective: %w", err)
				}
			}
		}
		for _, label := range shortcutLabelNames(row.EpicLabels) {
			if labelID, ok := labelMap[normalizeShortcutName(label)]; ok {
				if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PMEpicLabel{
					EpicID:  epicID,
					LabelID: labelID,
				}).Error; err != nil {
					return nil, 0, fmt.Errorf("link epic label: %w", err)
				}
			}
		}
	}
	return existingMap, created, nil
}

func (s *PMImportService) ensureEpicWorkflowStates(ctx context.Context, tx *gorm.DB, workspaceID string) ([]model.PMEpicWorkflowState, error) {
	var states []model.PMEpicWorkflowState
	if err := tx.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("position ASC").
		Find(&states).Error; err != nil {
		return nil, fmt.Errorf("list epic workflow states: %w", err)
	}
	if len(states) > 0 {
		return states, nil
	}

	color := func(c string) *string { return &c }
	states = []model.PMEpicWorkflowState{
		{WorkspaceID: workspaceID, Name: "To Do", StateType: model.PMStateTypeUnstarted, Position: 0, Color: color("#9ca3af"), IsDefault: true},
		{WorkspaceID: workspaceID, Name: "In Progress", StateType: model.PMStateTypeStarted, Position: 1, Color: color("#3b82f6"), IsDefault: false},
		{WorkspaceID: workspaceID, Name: "Done", StateType: model.PMStateTypeDone, Position: 2, Color: color("#22c55e"), IsDefault: false},
	}
	if err := tx.WithContext(ctx).Create(&states).Error; err != nil {
		return nil, fmt.Errorf("seed default epic states: %w", err)
	}
	return states, nil
}

func (s *PMImportService) ensureSprints(ctx context.Context, tx *gorm.DB, workspaceID string, rows []shortcutCSVRow, teamMap map[string]string) (map[string]string, int, []string, error) {
	grouped := map[string][]shortcutCSVRow{}
	for _, row := range rows {
		if row.IterationID != "" {
			grouped[row.IterationID] = append(grouped[row.IterationID], row)
		}
	}
	existingMap, err := s.lookupSprintsByExternalID(ctx, tx, workspaceID, mapKeys(grouped))
	if err != nil {
		return nil, 0, nil, err
	}
	created := 0
	nullDateCount := 0
	for externalID, rowsForSprint := range grouped {
		if _, ok := existingMap[externalID]; ok {
			continue
		}
		row := rowsForSprint[0]
		startDate, endDate := inferShortcutSprintDates(row.Iteration, rowsForSprint)
		if startDate == nil || endDate == nil {
			nullDateCount++
		}
		teamID := consistentTeamID(rowsForSprint, teamMap)
		ext := externalID
		sprint := model.PMSprint{
			WorkspaceID: workspaceID,
			Name:        fallbackName(row.Iteration, fmt.Sprintf("Sprint %s", externalID)),
			ExternalID:  &ext,
			StartDate:   startDate,
			EndDate:     endDate,
			TeamID:      teamID,
			Archived:    false,
			CreatedAt:   valueOrNow(parseShortcutTimestamp(row.CreatedAt, row.UTCOffset)),
			UpdatedAt:   valueOrNow(parseShortcutTimestamp(row.UpdatedAt, row.UTCOffset)),
		}
		if err := tx.WithContext(ctx).Create(&sprint).Error; err != nil {
			return nil, 0, nil, fmt.Errorf("create sprint: %w", err)
		}
		existingMap[externalID] = sprint.ID
		created++
	}
	warnings := []string{}
	if nullDateCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d imported sprints had null dates because iteration names could not be parsed", nullDateCount))
	}
	return existingMap, created, warnings, nil
}

func (s *PMImportService) createStories(ctx context.Context, tx *gorm.DB, workspaceID string, rows []shortcutCSVRow, workflowMap, stateMap, teamMap, userByEmail, epicMap, sprintMap, labelMap map[string]string, result *model.ShortcutImportResult, jobID string) error {
	existingStories, err := s.lookupStoriesByExternalID(ctx, tx, workspaceID, mapKeys(storyExternalIDs(rows)))
	if err != nil {
		return err
	}
	maxDisplayID, err := s.maxStoryDisplayID(ctx, tx, workspaceID)
	if err != nil {
		return err
	}
	unmappedRequesterCount := 0
	unmappedOwnerCounts := map[string]int{}
	processed := 0

	for _, row := range rows {
		if _, ok := existingStories[row.ID]; ok {
			result.StoriesSkipped++
			processed++
			if processed%100 == 0 {
				_ = s.markStep(ctx, jobID, "stories", 7, processed)
			}
			continue
		}

		workflowID := workflowMap[normalizeShortcutName(row.Workflow)]
		stateID := stateMap[normalizeShortcutName(row.Workflow)+"::"+normalizeShortcutName(row.State)]
		if workflowID == "" || stateID == "" {
			return fmt.Errorf("no workflow/state mapping found for %s / %s", row.Workflow, row.State)
		}

		maxDisplayID++
		description := normalizeShortcutDescription(row.Description)
		var descriptionPtr *string
		if strings.TrimSpace(description) != "" {
			descriptionPtr = &description
		}

		var requesterID *string
		if row.Requester != "" {
			if id, ok := userByEmail[normalizeShortcutName(row.Requester)]; ok && id != "" {
				requesterID = &id
			} else {
				unmappedRequesterCount++
			}
		}

		ownerIDs := make([]string, 0)
		for _, email := range shortcutOwnerEmails(row.Owners) {
			if id, ok := userByEmail[normalizeShortcutName(email)]; ok && id != "" {
				if !containsString(ownerIDs, id) {
					ownerIDs = append(ownerIDs, id)
				}
			} else {
				unmappedOwnerCounts[email]++
			}
		}
		var ownerID *string
		if len(ownerIDs) > 0 {
			ownerID = &ownerIDs[0]
		}

		priority := mapShortcutPriority(row.Priority)
		severity := mapShortcutSeverity(row.Severity)
		teamID := mappedTeamID(row.Team, teamMap)
		epicID := mappedEntityID(row.EpicID, epicMap)
		sprintID := mappedEntityID(row.IterationID, sprintMap)
		startedAt := parseShortcutTimestamp(row.StartedAt, row.UTCOffset)
		completedAt := parseShortcutTimestamp(row.CompletedAt, row.UTCOffset)
		movedAt := parseShortcutTimestamp(row.MovedAt, row.UTCOffset)
		completed := parseShortcutBool(row.IsCompleted)
		started := startedAt != nil || completedAt != nil || completed
		externalID := row.ID
		story := model.PMStory{
			WorkspaceID:     workspaceID,
			DisplayID:       maxDisplayID,
			Name:            fallbackName(row.Name, fmt.Sprintf("Untitled Story (SC-%s)", row.ID)),
			Description:     descriptionPtr,
			StoryType:       mapShortcutStoryType(row.Type),
			WorkflowID:      workflowID,
			WorkflowStateID: stateID,
			EpicID:          epicID,
			SprintID:        sprintID,
			TeamID:          teamID,
			OwnerID:         ownerID,
			RequesterID:     requesterID,
			Estimate:        parseShortcutInt(row.Estimate),
			Priority:        priority,
			Severity:        severity,
			Deadline:        parseShortcutTimestamp(row.DueDate, row.UTCOffset),
			Started:         started,
			StartedAt:       startedAt,
			Completed:       completed,
			CompletedAt:     completedAt,
			MovedAt:         movedAt,
			Blocked:         parseShortcutBool(row.IsBlocked),
			Archived:        parseShortcutBool(row.IsArchived),
			ExternalID:      &externalID,
			CreatedAt:       valueOrNow(parseShortcutTimestamp(row.CreatedAt, row.UTCOffset)),
			UpdatedAt:       valueOrNow(parseShortcutTimestamp(row.UpdatedAt, row.UTCOffset)),
		}
		if err := tx.WithContext(ctx).Create(&story).Error; err != nil {
			return fmt.Errorf("create story %s: %w", row.ID, err)
		}
		existingStories[row.ID] = story.ID
		result.StoriesCreated++

		for _, owner := range ownerIDs {
			if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PMStoryOwner{
				StoryID: story.ID,
				UserID:  owner,
			}).Error; err != nil {
				return fmt.Errorf("link story owner: %w", err)
			}
			result.OwnerLinksCreated++
		}

		for _, label := range shortcutLabelNames(row.Labels) {
			if labelID, ok := labelMap[normalizeShortcutName(label)]; ok {
				if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PMStoryLabel{
					StoryID: story.ID,
					LabelID: labelID,
				}).Error; err != nil {
					return fmt.Errorf("link story label: %w", err)
				}
				result.LabelLinksCreated++
			}
		}

		for idx, item := range parseShortcutChecklist(row.Tasks) {
			checklist := model.PMChecklistItem{
				StoryID:   story.ID,
				Text:      item.Text,
				Completed: item.Completed,
				Position:  idx,
				CreatedAt: story.CreatedAt,
				UpdatedAt: story.UpdatedAt,
			}
			if err := tx.WithContext(ctx).Create(&checklist).Error; err != nil {
				return fmt.Errorf("create checklist item: %w", err)
			}
			result.ChecklistItemsCreated++
		}

		processed++
		if processed%100 == 0 {
			_ = s.markStep(ctx, jobID, "stories", 7, processed)
		}
	}

	if unmappedRequesterCount > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d stories had unmapped requester emails — requester_id set to null", unmappedRequesterCount))
	}
	if len(unmappedOwnerCounts) > 0 {
		type pair struct {
			Email string
			Count int
		}
		pairs := make([]pair, 0, len(unmappedOwnerCounts))
		for email, count := range unmappedOwnerCounts {
			pairs = append(pairs, pair{Email: email, Count: count})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].Count > pairs[j].Count })
		for _, p := range pairs[:min(3, len(pairs))] {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Owner email '%s' not mapped — %d owner links skipped", p.Email, p.Count))
		}
	}
	return nil
}

func (s *PMImportService) countStoryDuplicates(ctx context.Context, workspaceID string, externalIDs []string) (int, error) {
	if len(externalIDs) == 0 {
		return 0, nil
	}
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&model.PMStory{}).
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count duplicate stories: %w", err)
	}
	return int(count), nil
}

func (s *PMImportService) lookupStoriesByExternalID(ctx context.Context, tx *gorm.DB, workspaceID string, externalIDs []string) (map[string]string, error) {
	type row struct{ ID, ExternalID string }
	var rows []row
	if len(externalIDs) == 0 {
		return map[string]string{}, nil
	}
	if err := tx.WithContext(ctx).Model(&model.PMStory{}).
		Select("id, external_id").
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("lookup stories by external id: %w", err)
	}
	out := map[string]string{}
	for _, row := range rows {
		out[row.ExternalID] = row.ID
	}
	return out, nil
}

func (s *PMImportService) lookupEpicsByExternalID(ctx context.Context, tx *gorm.DB, workspaceID string, externalIDs []string) (map[string]string, error) {
	type row struct{ ID, ExternalID string }
	var rows []row
	if len(externalIDs) == 0 {
		return map[string]string{}, nil
	}
	if err := tx.WithContext(ctx).Model(&model.PMEpic{}).
		Select("id, external_id").
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("lookup epics by external id: %w", err)
	}
	out := map[string]string{}
	for _, row := range rows {
		out[row.ExternalID] = row.ID
	}
	return out, nil
}

func (s *PMImportService) lookupObjectivesByExternalID(ctx context.Context, tx *gorm.DB, workspaceID string, externalIDs []string) (map[string]string, error) {
	type row struct{ ID, ExternalID string }
	var rows []row
	if len(externalIDs) == 0 {
		return map[string]string{}, nil
	}
	if err := tx.WithContext(ctx).Model(&model.PMObjective{}).
		Select("id, external_id").
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("lookup objectives by external id: %w", err)
	}
	out := map[string]string{}
	for _, row := range rows {
		out[row.ExternalID] = row.ID
	}
	return out, nil
}

func (s *PMImportService) lookupSprintsByExternalID(ctx context.Context, tx *gorm.DB, workspaceID string, externalIDs []string) (map[string]string, error) {
	type row struct{ ID, ExternalID string }
	var rows []row
	if len(externalIDs) == 0 {
		return map[string]string{}, nil
	}
	if err := tx.WithContext(ctx).Model(&model.PMSprint{}).
		Select("id, external_id").
		Where("workspace_id = ? AND external_id IN ?", workspaceID, externalIDs).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("lookup sprints by external id: %w", err)
	}
	out := map[string]string{}
	for _, row := range rows {
		out[row.ExternalID] = row.ID
	}
	return out, nil
}

func (s *PMImportService) maxStoryDisplayID(ctx context.Context, tx *gorm.DB, workspaceID string) (int, error) {
	var maxID int
	if err := tx.WithContext(ctx).Model(&model.PMStory{}).
		Where("workspace_id = ?", workspaceID).
		Select("COALESCE(MAX(display_id), 0)").
		Scan(&maxID).Error; err != nil {
		return 0, fmt.Errorf("load max display id: %w", err)
	}
	return maxID, nil
}

func (s *PMImportService) requireWorkspaceAdmin(ctx context.Context, workspaceID, actorID string) error {
	if workspaceID == "" || actorID == "" {
		return fmt.Errorf("workspace_id and user_id are required")
	}
	role, err := s.workspaceRepo.GetMemberRole(ctx, workspaceID, actorID)
	if err != nil {
		return err
	}
	if role != model.RoleOwner && role != model.RoleAdmin {
		return fmt.Errorf("workspace admin access required")
	}
	return nil
}

func (s *PMImportService) updateJob(ctx context.Context, jobID string, updates map[string]interface{}) error {
	if strings.TrimSpace(jobID) == "" {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.PMImportJob{}).Where("id = ?", jobID).Updates(updates).Error
}

func (s *PMImportService) markStep(ctx context.Context, jobID, step string, completed, entitiesProcessed int) error {
	progress := completed * 100 / 8
	return s.updateJob(ctx, jobID, map[string]interface{}{
		"current_step":       step,
		"steps_completed":    completed,
		"progress":           progress,
		"entities_processed": entitiesProcessed,
		"updated_at":         time.Now().UTC(),
	})
}

func filterShortcutRows(rows []shortcutCSVRow, options model.ShortcutImportOptions) []shortcutCSVRow {
	filtered := make([]shortcutCSVRow, 0, len(rows))
	for _, row := range rows {
		if !options.ImportArchived && parseShortcutBool(row.IsArchived) {
			continue
		}
		if !options.ImportCompleted && parseShortcutBool(row.IsCompleted) {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func storyExternalIDs(rows []shortcutCSVRow) map[string]struct{} {
	out := map[string]struct{}{}
	for _, row := range rows {
		if row.ID != "" {
			out[row.ID] = struct{}{}
		}
	}
	return out
}

func mapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSetKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func countWorkflowStates(workflows map[string]map[string]int) int {
	total := 0
	for _, states := range workflows {
		total += len(states)
	}
	return total
}

func valueOrNow(ts *time.Time) time.Time {
	if ts != nil {
		return *ts
	}
	return time.Now().UTC()
}

func timePtr(ts time.Time) *time.Time {
	return &ts
}

func fallbackName(name, fallback string) string {
	if strings.TrimSpace(name) == "" {
		return fallback
	}
	return strings.TrimSpace(name)
}

func mappedTeamID(teamName string, teamMap map[string]string) *string {
	if teamName == "" {
		return nil
	}
	if id, ok := teamMap[normalizeShortcutName(teamName)]; ok {
		return &id
	}
	return nil
}

func consistentTeamID(rows []shortcutCSVRow, teamMap map[string]string) *string {
	var current string
	for _, row := range rows {
		if row.Team == "" {
			continue
		}
		id, ok := teamMap[normalizeShortcutName(row.Team)]
		if !ok {
			continue
		}
		if current == "" {
			current = id
			continue
		}
		if current != id {
			return nil
		}
	}
	if current == "" {
		return nil
	}
	return &current
}

func mappedEntityID(externalID string, entityMap map[string]string) *string {
	if externalID == "" {
		return nil
	}
	if id, ok := entityMap[externalID]; ok {
		return &id
	}
	return nil
}

func mapShortcutPriority(raw string) string {
	switch normalizeShortcutName(raw) {
	case "highest":
		return model.PMStoryPriorityUrgent
	case "high":
		return model.PMStoryPriorityHigh
	case "medium":
		return model.PMStoryPriorityMedium
	case "low", "lowest":
		return model.PMStoryPriorityLow
	default:
		return model.PMStoryPriorityNone
	}
}

func mapShortcutSeverity(raw string) string {
	switch normalizeShortcutName(raw) {
	case "severity 0":
		return model.PMStorySeverityCritical
	case "severity 1":
		return model.PMStorySeverityMajor
	case "severity 2":
		return model.PMStorySeverityMinor
	default:
		return model.PMStorySeverityNone
	}
}

func mapShortcutStoryType(raw string) string {
	switch normalizeShortcutName(raw) {
	case model.PMStoryTypeBug:
		return model.PMStoryTypeBug
	case model.PMStoryTypeChore:
		return model.PMStoryTypeChore
	default:
		return model.PMStoryTypeFeature
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
