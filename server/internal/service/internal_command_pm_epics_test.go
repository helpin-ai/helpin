package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

type pmEpicCommandTestEnv struct {
	service       *InternalCommandService
	epicService   *PMEpicService
	db            *gorm.DB
	workspaceID   string
	foreignWSID   string
	adminUserID   string
	memberUserID  string
	memberID      string
	teamA         string
	teamB         string
	stateID       string
	labelA        string
	labelB        string
	repositoryID  string
	adminContext  context.Context
	memberContext context.Context
}

func newPMEpicCommandTestEnv(t *testing.T) *pmEpicCommandTestEnv {
	t.Helper()
	epicService, db, workspaceID, adminUserID := newEpicTestEnvWithDB(t)
	now := time.Now().UTC()
	env := &pmEpicCommandTestEnv{
		epicService:  epicService,
		db:           db,
		workspaceID:  workspaceID,
		foreignWSID:  "ws-command-epic-foreign",
		adminUserID:  adminUserID,
		memberUserID: "user-command-epic-member",
		memberID:     "member-command-epic-member",
		teamA:        "team-command-epic-a",
		teamB:        "team-command-epic-b",
		stateID:      "state-command-epic",
		labelA:       "label-command-epic-a",
		labelB:       "label-command-epic-b",
		repositoryID: "repo-command-epic",
	}
	seedWorkspace(t, db, env.foreignWSID, "Foreign Epic Command", "foreign-command-epic", adminUserID)
	seedUser(t, db, env.memberUserID, "epic-command-member@test.com", "Epic Command Member", "hash")
	seedWorkspaceMember(t, db, env.memberID, workspaceID, env.memberUserID, "epic-command-member@test.com", "Epic Command Member", model.RoleMember)
	for _, team := range []struct{ id, name string }{{env.teamA, "Command A"}, {env.teamB, "Command B"}} {
		mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, team.id, workspaceID, team.name, now, now)
	}
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"membership-command-epic", env.teamA, env.memberID, "member", now, now)
	mustExec(t, db, `INSERT INTO pm_epic_workflow_states (id, workspace_id, name, state_type, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		env.stateID, workspaceID, "In Progress", model.PMStateTypeStarted, 0, now, now)
	for _, label := range []model.PMLabel{
		{ID: env.labelA, WorkspaceID: workspaceID, TeamID: &env.teamA, Name: "Alpha"},
		{ID: env.labelB, WorkspaceID: workspaceID, TeamID: &env.teamA, Name: "Beta"},
	} {
		if err := db.Create(&label).Error; err != nil {
			t.Fatalf("create label: %v", err)
		}
	}
	if err := db.Create(&model.GitRepository{
		ID: env.repositoryID, WorkspaceID: workspaceID, IntegrationID: "integration-command-epic",
		Provider: "github", ExternalID: "command-epic", FullName: "helpin/command-epic",
		Permissions: json.RawMessage(`{}`), Active: true, Selected: true,
	}).Error; err != nil {
		t.Fatalf("create repository: %v", err)
	}

	workspaceRepo := repository.NewWorkspaceRepository(db)
	workflowService := NewPMWorkflowService(repository.NewPMWorkflowRepository(db), repository.NewPMTaskRepository(db), repository.NewPMLabelRepository(db), nil)
	commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	commandService.SetPMOperationalServices(workspaceRepo, epicService, nil, nil, workflowService, nil)
	commandService.SetGitService(&GitService{repoRepo: repository.NewGitRepositoryRepository(db)})
	env.service = commandService
	env.adminContext = authorization.WithActor(context.Background(), &authorization.Actor{
		UserID: adminUserID, WorkspaceID: workspaceID, WorkspaceMemberID: "member-epic-001", Role: model.RoleAdmin,
	})
	env.memberContext = authorization.WithActor(context.Background(), &authorization.Actor{
		UserID: env.memberUserID, WorkspaceID: workspaceID, WorkspaceMemberID: env.memberID, Role: model.RoleMember,
		TeamMemberships: []authorization.TeamRole{{TeamID: env.teamA, Role: "member"}},
	})
	return env
}

func (e *pmEpicCommandTestEnv) meta(targetType, targetID, actorID string) model.InternalCommandContext {
	return model.InternalCommandContext{WorkspaceID: e.workspaceID, ActorID: actorID, TargetType: targetType, TargetID: targetID}
}

func TestPMCommandListEpicsReturnsCompactFilteredStatsAndTeamScope(t *testing.T) {
	env := newPMEpicCommandTestEnv(t)
	start := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	epicA, err := env.epicService.Create(env.adminContext, model.CreateEpicRequest{
		WorkspaceID: env.workspaceID, Name: "Epic A", TeamID: &env.teamA, EpicStateID: &env.stateID,
		PlannedStartDate: &start, LabelIDs: []string{env.labelA},
	}, env.adminUserID)
	if err != nil {
		t.Fatalf("create epic A: %v", err)
	}
	epicB, err := env.epicService.Create(env.adminContext, model.CreateEpicRequest{WorkspaceID: env.workspaceID, Name: "Epic B", TeamID: &env.teamB}, env.adminUserID)
	if err != nil {
		t.Fatalf("create epic B: %v", err)
	}
	seedWorkflowForStoryTest(t, env.db, env.workspaceID, "workflow-command-epic", "workflow-state-command-epic")
	if err := env.db.Create(&model.PMTask{
		ID: "task-command-epic", WorkspaceID: env.workspaceID, DisplayID: 1, Name: "Epic task",
		TaskType: model.PMTaskTypeFeature, WorkflowID: "workflow-command-epic", WorkflowStateID: "workflow-state-command-epic",
		EpicID: &epicA.Epic.ID, TeamID: &env.teamA, Priority: model.PMTaskPriorityNone, Severity: model.PMTaskSeverityNone,
	}).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	output, err := env.service.Execute(env.memberContext, env.meta("workspace", env.workspaceID, env.memberUserID), "pm.list_epics", json.RawMessage(`{"team_id":"`+env.teamA+`","state_id":"`+env.stateID+`","label_id":"`+env.labelA+`","archived":false,"page":1,"per_page":1}`))
	if err != nil {
		t.Fatalf("pm.list_epics: %v", err)
	}
	var result struct {
		Epics []struct {
			EpicID           string            `json:"epic_id"`
			PlannedStartDate string            `json:"planned_start_date"`
			Stats            model.PMEpicStats `json:"stats"`
		} `json:"epics"`
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, output)
	}
	if result.Total != 1 || len(result.Epics) != 1 || result.Epics[0].EpicID != epicA.Epic.ID || result.Epics[0].Stats.TaskCount != 1 {
		t.Fatalf("unexpected list result: %#v", result)
	}
	if result.Epics[0].PlannedStartDate != "2026-08-10" || result.Page != 1 || result.PerPage != 1 {
		t.Fatalf("unexpected date/pagination: %#v", result)
	}
	queryResult, err := env.service.Execute(env.adminContext, env.meta("workspace", env.workspaceID, env.adminUserID), "pm.list_epics", json.RawMessage(`{"query":"epic b","page":1,"per_page":10}`))
	if err != nil {
		t.Fatalf("query epics: %v", err)
	}
	if !strings.Contains(string(queryResult), `"total":1`) || !strings.Contains(string(queryResult), epicB.Epic.ID) {
		t.Fatalf("query epics = %s", queryResult)
	}
}

func TestPMCommandListEpicsPagesBeforeBoundedBatchEnrichment(t *testing.T) {
	env := newPMEpicCommandTestEnv(t)
	for i := 1; i <= 5; i++ {
		position := i
		if _, err := env.epicService.Create(env.adminContext, model.CreateEpicRequest{
			WorkspaceID: env.workspaceID,
			Name:        fmt.Sprintf("Paged Epic %d", i),
			Position:    &position,
		}, env.adminUserID); err != nil {
			t.Fatalf("create epic %d: %v", i, err)
		}
	}

	queryCount := 0
	const callbackName = "test:count_pm_list_epics_queries"
	if err := env.db.Callback().Query().Before("gorm:query").Register(callbackName, func(*gorm.DB) {
		queryCount++
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	t.Cleanup(func() { _ = env.db.Callback().Query().Remove(callbackName) })

	output, err := env.service.Execute(env.adminContext, env.meta("workspace", env.workspaceID, env.adminUserID), "pm.list_epics", json.RawMessage(`{"page":2,"per_page":1}`))
	if err != nil {
		t.Fatalf("pm.list_epics: %v", err)
	}
	var result struct {
		Epics []struct {
			Name string `json:"name"`
		} `json:"epics"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, output)
	}
	if result.Total != 5 || len(result.Epics) != 1 || result.Epics[0].Name != "Paged Epic 2" {
		t.Fatalf("unexpected page: %#v", result)
	}
	if queryCount > 6 {
		t.Fatalf("pm.list_epics queries = %d, want at most 6 regardless of off-page rows", queryCount)
	}
}

func TestPMCommandGetEpicDefaultsTargetAndRejectsCrossWorkspace(t *testing.T) {
	env := newPMEpicCommandTestEnv(t)
	created, err := env.epicService.Create(env.adminContext, model.CreateEpicRequest{WorkspaceID: env.workspaceID, Name: "Target Epic", TeamID: &env.teamA}, env.adminUserID)
	if err != nil {
		t.Fatalf("create target epic: %v", err)
	}
	output, err := env.service.Execute(env.adminContext, env.meta("epic", created.Epic.ID, env.adminUserID), "pm.get_epic", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("pm.get_epic target default: %v", err)
	}
	if !strings.Contains(string(output), created.Epic.ID) {
		t.Fatalf("output does not contain epic id: %s", output)
	}
	foreign, err := env.epicService.Create(context.Background(), model.CreateEpicRequest{WorkspaceID: env.foreignWSID, Name: "Foreign Epic"}, env.adminUserID)
	if err != nil {
		t.Fatalf("create foreign epic: %v", err)
	}
	_, err = env.service.Execute(env.adminContext, env.meta("workspace", env.workspaceID, env.adminUserID), "pm.get_epic", mustJSON(map[string]any{"epic_id": foreign.Epic.ID}))
	if err == nil || !strings.Contains(err.Error(), "epic not found") {
		t.Fatalf("cross-workspace get error = %v", err)
	}
}

func TestPMCommandCreateEpicValidatesReferencesBeforeWriting(t *testing.T) {
	env := newPMEpicCommandTestEnv(t)
	input := mustJSON(map[string]any{
		"name": "Command Created", "description": "Created by an agent", "team_id": env.teamA,
		"epic_state_id": env.stateID, "owner_member_id": env.memberID,
		"planned_start_date": "2026-08-04", "deadline": "2026-08-30", "health": model.PMEpicHealthOnTrack,
		"label_ids": []string{env.labelA}, "planning_repository_id": env.repositoryID,
	})
	output, err := env.service.Execute(env.adminContext, env.meta("workspace", env.workspaceID, env.adminUserID), "pm.create_epic", input)
	if err != nil {
		t.Fatalf("pm.create_epic: %v", err)
	}
	var created struct {
		EpicID          string `json:"epic_id"`
		SuggestedHealth string `json:"suggested_health"`
	}
	if err := json.Unmarshal(output, &created); err != nil || created.EpicID == "" {
		t.Fatalf("create output = %s, err = %v", output, err)
	}
	if created.SuggestedHealth != model.PMEpicHealthNone {
		t.Fatalf("create suggested_health = %q, want %q", created.SuggestedHealth, model.PMEpicHealthNone)
	}
	stored, err := env.epicService.GetByID(env.adminContext, created.EpicID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Epic.CreatedBy == nil || *stored.Epic.CreatedBy != env.adminUserID || len(stored.Labels) != 1 || stored.Epic.Deadline == nil {
		t.Fatalf("unexpected stored epic: %#v labels=%#v", stored.Epic, stored.Labels)
	}

	foreignTeam := "team-command-epic-foreign"
	foreignState := "state-command-epic-foreign"
	foreignMemberUser := "user-command-epic-foreign"
	foreignMember := "member-command-epic-foreign"
	foreignLabel := "label-command-epic-foreign"
	foreignRepo := "repo-command-epic-foreign"
	now := time.Now().UTC()
	seedUser(t, env.db, foreignMemberUser, "foreign-command@test.com", "Foreign", "hash")
	seedWorkspaceMember(t, env.db, foreignMember, env.foreignWSID, foreignMemberUser, "foreign-command@test.com", "Foreign", model.RoleMember)
	mustExec(t, env.db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, foreignTeam, env.foreignWSID, "Foreign", now, now)
	mustExec(t, env.db, `INSERT INTO pm_epic_workflow_states (id, workspace_id, name, state_type, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, foreignState, env.foreignWSID, "Foreign", model.PMStateTypeUnstarted, 0, now, now)
	if err := env.db.Create(&model.PMLabel{ID: foreignLabel, WorkspaceID: env.foreignWSID, Name: "Foreign"}).Error; err != nil {
		t.Fatalf("create foreign label: %v", err)
	}
	if err := env.db.Create(&model.GitRepository{ID: foreignRepo, WorkspaceID: env.foreignWSID, IntegrationID: "foreign", Provider: "github", ExternalID: "foreign", FullName: "foreign/repo", Permissions: json.RawMessage(`{}`), Active: true, Selected: true}).Error; err != nil {
		t.Fatalf("create foreign repo: %v", err)
	}
	cases := []map[string]any{
		{"team_id": foreignTeam}, {"epic_state_id": foreignState}, {"owner_member_id": foreignMember},
		{"owner_id": foreignMemberUser}, {"label_ids": []string{foreignLabel}}, {"planning_repository_id": foreignRepo},
	}
	for i, fields := range cases {
		fields["name"] = "Invalid Reference"
		_, err := env.service.Execute(env.adminContext, env.meta("workspace", env.workspaceID, env.adminUserID), "pm.create_epic", mustJSON(fields))
		if err == nil {
			t.Fatalf("case %d accepted cross-workspace reference: %#v", i, fields)
		}
	}
	_, err = env.service.Execute(env.memberContext, env.meta("workspace", env.workspaceID, env.memberUserID), "pm.create_epic", mustJSON(map[string]any{"name": "Wrong Team", "team_id": env.teamB}))
	if err == nil {
		t.Fatal("member created epic in an inaccessible destination team")
	}
	_, err = env.service.Execute(env.adminContext, env.meta("workspace", env.workspaceID, env.adminUserID), "pm.create_epic", json.RawMessage(`{"name":"Bad Date","planned_start_date":"08/04/2026"}`))
	if err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Fatalf("malformed date error = %v", err)
	}
}

func TestPMCommandCreateEpicBatchesLabelValidationReads(t *testing.T) {
	env := newPMEpicCommandTestEnv(t)
	labelQueries := 0
	const callbackName = "test:count_pm_epic_label_validation_queries"
	if err := env.db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "pm_labels" {
			labelQueries++
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	t.Cleanup(func() { _ = env.db.Callback().Query().Remove(callbackName) })

	_, err := env.service.Execute(env.adminContext, env.meta("workspace", env.workspaceID, env.adminUserID), "pm.create_epic", mustJSON(map[string]any{
		"name":      "Batch Label Validation",
		"team_id":   env.teamA,
		"label_ids": []string{env.labelA, env.labelB},
	}))
	if err != nil {
		t.Fatalf("pm.create_epic: %v", err)
	}
	if labelQueries > 2 {
		t.Fatalf("pm_labels validation queries = %d, want at most 2 batch reads", labelQueries)
	}
}

func TestPMCommandCreateEpicRejectsArchivedLabelAtCommandBoundary(t *testing.T) {
	env := newPMEpicCommandTestEnv(t)
	if err := env.db.Model(&model.PMLabel{}).Where("id = ?", env.labelA).Update("archived", true).Error; err != nil {
		t.Fatalf("archive label: %v", err)
	}

	_, err := env.service.Execute(env.adminContext, env.meta("workspace", env.workspaceID, env.adminUserID), "pm.create_epic", mustJSON(map[string]any{
		"name":      "Archived Label Command",
		"team_id":   env.teamA,
		"label_ids": []string{env.labelA},
	}))
	if err == nil || !strings.Contains(err.Error(), "label") {
		t.Fatalf("archived label command error = %v", err)
	}
	var count int64
	if err := env.db.Model(&model.PMEpic{}).Where("workspace_id = ? AND name = ?", env.workspaceID, "Archived Label Command").Count(&count).Error; err != nil {
		t.Fatalf("count epics: %v", err)
	}
	if count != 0 {
		t.Fatalf("persisted epics = %d, want 0", count)
	}
}

func TestPMCommandUpdateEpicPreservesReplacesClearsAndRejectsNoop(t *testing.T) {
	env := newPMEpicCommandTestEnv(t)
	start := time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, time.August, 30, 0, 0, 0, 0, time.UTC)
	created, err := env.epicService.Create(env.adminContext, model.CreateEpicRequest{
		WorkspaceID: env.workspaceID, Name: "Update Me", TeamID: &env.teamA, EpicStateID: &env.stateID,
		OwnerMemberID: &env.memberID, PlannedStartDate: &start, Deadline: &deadline,
		PlanningRepositoryID: &env.repositoryID, LabelIDs: []string{env.labelA},
	}, env.adminUserID)
	if err != nil {
		t.Fatalf("create epic: %v", err)
	}
	meta := env.meta("epic", created.Epic.ID, env.adminUserID)
	if _, err := env.service.Execute(env.adminContext, meta, "pm.update_epic", json.RawMessage(`{"deadline":"August 30"}`)); err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Fatalf("malformed update date error = %v", err)
	}
	updateOutput, err := env.service.Execute(env.adminContext, meta, "pm.update_epic", json.RawMessage(`{"description":"changed"}`))
	if err != nil {
		t.Fatalf("update description: %v", err)
	}
	var updateResult struct {
		SuggestedHealth string `json:"suggested_health"`
	}
	if err := json.Unmarshal(updateOutput, &updateResult); err != nil {
		t.Fatalf("unmarshal update output: %v", err)
	}
	if updateResult.SuggestedHealth != model.PMEpicHealthNone {
		t.Fatalf("update suggested_health = %q, want %q", updateResult.SuggestedHealth, model.PMEpicHealthNone)
	}
	preserved, err := env.epicService.GetByID(env.adminContext, created.Epic.ID)
	if err != nil || len(preserved.Labels) != 1 || preserved.Epic.Deadline == nil {
		t.Fatalf("omitted associations not preserved: epic=%#v labels=%#v err=%v", preserved, preserved.Labels, err)
	}
	if _, err := env.service.Execute(env.adminContext, meta, "pm.update_epic", mustJSON(map[string]any{"label_ids": []string{env.labelB}})); err != nil {
		t.Fatalf("replace labels: %v", err)
	}
	replaced, err := env.epicService.GetByID(env.adminContext, created.Epic.ID)
	if err != nil || len(replaced.Labels) != 1 || replaced.Labels[0].ID != env.labelB {
		t.Fatalf("labels were not replaced: labels=%#v err=%v", replaced.Labels, err)
	}
	clearInput := json.RawMessage(`{"epic_state_id":"","owner_member_id":"","team_id":"","planned_start_date":"","deadline":"","planning_repository_id":"","label_ids":[]}`)
	if _, err := env.service.Execute(env.adminContext, meta, "pm.update_epic", clearInput); err != nil {
		t.Fatalf("clear update: %v", err)
	}
	cleared, err := env.epicService.GetByID(env.adminContext, created.Epic.ID)
	if err != nil {
		t.Fatalf("get cleared epic: %v", err)
	}
	if cleared.Epic.TeamID != nil || cleared.Epic.EpicStateID != nil || cleared.Epic.OwnerMemberID != nil || cleared.Epic.PlannedStartDate != nil || cleared.Epic.Deadline != nil || cleared.Epic.PlanningRepositoryID != nil || len(cleared.Labels) != 0 {
		t.Fatalf("fields not cleared: epic=%#v labels=%#v", cleared.Epic, cleared.Labels)
	}
	if _, err := env.service.Execute(env.adminContext, meta, "pm.update_epic", json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "editable field") {
		t.Fatalf("no-op error = %v", err)
	}
	if _, err := env.service.Execute(env.adminContext, meta, "pm.update_epic", mustJSON(map[string]any{"epic_id": "different", "name": "Conflict"})); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("target conflict error = %v", err)
	}
	definition, ok := env.service.Definition("pm.update_epic")
	if !ok || definition.Tool == nil {
		t.Fatal("missing update epic definition")
	}
	properties := definition.Tool.InputSchema["properties"].(map[string]any)
	if _, exists := properties["archived"]; exists {
		t.Fatal("update_epic schema exposes archived")
	}
}

func TestPMCommandUpdateEpicRejectsAssignedAgentIncompatibleTeamMove(t *testing.T) {
	env := newPMEpicCommandTestEnv(t)
	agentID := "agent-command-epic-team-a"
	seedTeamScopedEpicAgent(t, env.db, env.epicService, env.workspaceID, agentID, env.teamA)
	created, err := env.epicService.Create(env.adminContext, model.CreateEpicRequest{
		WorkspaceID: env.workspaceID, Name: "Command Agent Scoped", TeamID: &env.teamA, AssignedAgentID: &agentID,
	}, env.adminUserID)
	if err != nil {
		t.Fatalf("create epic: %v", err)
	}

	_, err = env.service.Execute(env.adminContext, env.meta("workspace", env.workspaceID, env.adminUserID), "pm.update_epic", mustJSON(map[string]any{
		"epic_id": created.Epic.ID,
		"team_id": env.teamB,
	}))
	if err == nil || !strings.Contains(err.Error(), "restricted to team") {
		t.Fatalf("team move error = %v, want assigned-agent team restriction", err)
	}
	reloaded, err := env.epicService.GetByID(env.adminContext, created.Epic.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if reloaded.Epic.TeamID == nil || *reloaded.Epic.TeamID != env.teamA {
		t.Fatalf("team changed after rejected command: %#v", reloaded.Epic.TeamID)
	}
}

func TestPMCommandUpdateEpicRejectsCrossWorkspaceIDWithoutMutation(t *testing.T) {
	env := newPMEpicCommandTestEnv(t)
	foreign, err := env.epicService.Create(context.Background(), model.CreateEpicRequest{
		WorkspaceID: env.foreignWSID,
		Name:        "Foreign Update Target",
	}, env.adminUserID)
	if err != nil {
		t.Fatalf("create foreign epic: %v", err)
	}

	_, err = env.service.Execute(env.adminContext, env.meta("workspace", env.workspaceID, env.adminUserID), "pm.update_epic", mustJSON(map[string]any{
		"epic_id": foreign.Epic.ID,
		"name":    "Cross Workspace Mutation",
	}))
	if err == nil || !strings.Contains(err.Error(), "epic not found") {
		t.Fatalf("cross-workspace update error = %v", err)
	}
	var stored model.PMEpic
	if err := env.db.Where("id = ?", foreign.Epic.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload foreign epic: %v", err)
	}
	if stored.Name != "Foreign Update Target" {
		t.Fatalf("foreign epic name = %q, want unchanged", stored.Name)
	}
}
