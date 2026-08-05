package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

type pmObjectiveCommandTestEnv struct {
	db               *gorm.DB
	commands         *InternalCommandService
	objectives       *PMObjectiveService
	workspaceID      string
	otherWorkspaceID string
	teamA            string
	teamB            string
	teamOther        string
}

func newPMObjectiveCommandTestEnv(t *testing.T) *pmObjectiveCommandTestEnv {
	t.Helper()
	db := newTestDB(t)
	workspaceID := "ws-objective-command"
	otherWorkspaceID := "ws-objective-command-other"
	seedUser(t, db, "objective-admin", "objective-admin@example.com", "Objective Admin", "hash")
	seedUser(t, db, "objective-manager", "objective-manager@example.com", "Objective Manager", "hash")
	seedUser(t, db, "objective-owner", "objective-owner@example.com", "Objective Owner", "hash")
	seedUser(t, db, "objective-other", "objective-other@example.com", "Objective Other", "hash")
	seedWorkspace(t, db, workspaceID, "Objective Workspace", "objective-workspace", "objective-admin")
	seedWorkspace(t, db, otherWorkspaceID, "Other Objective Workspace", "other-objective-workspace", "objective-other")
	seedWorkspaceMember(t, db, "objective-member-admin", workspaceID, "objective-admin", "objective-admin@example.com", "Objective Admin", model.RoleAdmin)
	seedWorkspaceMember(t, db, "objective-member-manager", workspaceID, "objective-manager", "objective-manager@example.com", "Objective Manager", "manager")
	seedWorkspaceMember(t, db, "objective-member-owner", workspaceID, "objective-owner", "objective-owner@example.com", "Objective Owner", model.RoleMember)
	seedWorkspaceMember(t, db, "objective-member-other", otherWorkspaceID, "objective-other", "objective-other@example.com", "Objective Other", model.RoleAdmin)

	now := time.Now().UTC()
	for _, team := range []struct{ id, workspaceID string }{
		{"objective-team-a", workspaceID}, {"objective-team-b", workspaceID}, {"objective-team-other", otherWorkspaceID},
	} {
		mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, team.id, team.workspaceID, team.id, now, now)
	}
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, "objective-team-manager-a", "objective-team-a", "objective-member-manager", "owner", now, now)
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, "objective-team-member-a", "objective-team-a", "objective-member-owner", "member", now, now)

	labelRepo := repository.NewPMLabelRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	objectiveRepo := repository.NewPMObjectiveRepository(db)
	activity := NewPMActivityService(repository.NewPMActivityRepository(db))
	objectiveService := NewPMObjectiveService(objectiveRepo, repository.NewPMKeyResultRepository(db), labelRepo, nil, workspaceRepo, activity, nil, nil)
	taskRepo := repository.NewPMTaskRepository(db)
	epicService := NewPMEpicService(repository.NewPMEpicRepository(db), taskRepo, labelRepo, nil, nil, workspaceRepo, activity, nil, nil)
	commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, taskRepo, repository.NewPMTaskLinkRepository(db))
	commands.SetPMOperationalServices(workspaceRepo, epicService, nil, objectiveService, nil, nil)
	commands.SetPMLabelService(NewPMLabelService(labelRepo, nil))
	return &pmObjectiveCommandTestEnv{db: db, commands: commands, objectives: objectiveService, workspaceID: workspaceID, otherWorkspaceID: otherWorkspaceID, teamA: "objective-team-a", teamB: "objective-team-b", teamOther: "objective-team-other"}
}

func (e *pmObjectiveCommandTestEnv) meta(targetType, targetID string) model.InternalCommandContext {
	return model.InternalCommandContext{WorkspaceID: e.workspaceID, ActorID: "objective-admin", ActorRole: model.RoleAdmin, AgentID: "objective-agent", AgentScopeResolved: true, TargetType: targetType, TargetID: targetID}
}

func (e *pmObjectiveCommandTestEnv) seedObjective(t *testing.T, id, workspaceID, name string, teamIDs []string) {
	t.Helper()
	now := time.Now().UTC()
	mustExec(t, e.db, `INSERT INTO pm_objectives (id, workspace_id, name, objective_type, state, health, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, workspaceID, name, model.PMObjectiveTypeStrategic, model.PMObjectiveStateActive, model.PMObjectiveHealthOnTrack, false, now, now)
	for index, teamID := range teamIDs {
		mustExec(t, e.db, `INSERT INTO pm_objective_teams (objective_id, team_id, created_at) VALUES (?, ?, ?)`, id, teamID, now.Add(time.Duration(index)*time.Second))
	}
}

func executePMObjectiveTestCommand(t *testing.T, svc *InternalCommandService, ctx context.Context, meta model.InternalCommandContext, command, input string) map[string]any {
	t.Helper()
	out, err := svc.Execute(ctx, meta, command, json.RawMessage(input))
	if err != nil {
		t.Fatalf("%s: %v", command, err)
	}
	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("decode %s: %v (%s)", command, err, out)
	}
	return result
}

func TestPMCommandObjectiveReadsAreWorkspaceWideAndAgentScopeFailClosed(t *testing.T) {
	env := newPMObjectiveCommandTestEnv(t)
	env.seedObjective(t, "objective-read-a", env.workspaceID, "Team A", []string{env.teamA})
	env.seedObjective(t, "objective-read-b", env.workspaceID, "Team B", []string{env.teamB})
	env.seedObjective(t, "objective-read-other", env.otherWorkspaceID, "Other", []string{env.teamOther})

	unresolved := env.meta("workspace", env.workspaceID)
	unresolved.AgentScopeResolved = false
	if _, err := env.commands.Execute(context.Background(), unresolved, "pm.list_objectives", json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "agent team scope is unresolved") {
		t.Fatalf("unresolved scope error = %v", err)
	}

	meta := env.meta("workspace", env.workspaceID)
	meta.AgentTeamIDs = []string{env.teamA}
	result := executePMObjectiveTestCommand(t, env.commands, context.Background(), meta, "pm.list_objectives", `{"page":1,"per_page":100}`)
	items := result["objectives"].([]any)
	if result["total"] != float64(2) || len(items) != 2 {
		t.Fatalf("team-scoped agent did not retain workspace-wide objective read: %#v", result)
	}
	query := executePMObjectiveTestCommand(t, env.commands, context.Background(), meta, "pm.list_objectives", `{"query":"team b","page":1,"per_page":100}`)
	if query["total"] != float64(1) || len(query["objectives"].([]any)) != 1 || query["objectives"].([]any)[0].(map[string]any)["objective_id"] != "objective-read-b" {
		t.Fatalf("queried objectives = %#v", query)
	}
	get := executePMObjectiveTestCommand(t, env.commands, context.Background(), meta, "pm.get_objective", `{"objective_id":"objective-read-b"}`)
	if get["objective_id"] != "objective-read-b" {
		t.Fatalf("workspace-wide get = %#v", get)
	}
	if _, err := env.commands.Execute(context.Background(), meta, "pm.get_objective", json.RawMessage(`{"objective_id":"objective-read-other"}`)); err == nil || !strings.Contains(err.Error(), "objective not found") {
		t.Fatalf("cross-workspace get error = %v", err)
	}
}

func TestPMCommandListObjectivesPagesBeforeEnrichment(t *testing.T) {
	env := newPMObjectiveCommandTestEnv(t)
	for i := 1; i <= 5; i++ {
		env.seedObjective(t, fmt.Sprintf("objective-page-%d", i), env.workspaceID, fmt.Sprintf("Objective %d", i), []string{env.teamA})
	}
	queryCount := 0
	const callbackName = "test:count_pm_list_objectives_queries"
	if err := env.db.Callback().Query().Before("gorm:query").Register(callbackName, func(*gorm.DB) { queryCount++ }); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	t.Cleanup(func() { _ = env.db.Callback().Query().Remove(callbackName) })
	meta := env.meta("workspace", env.workspaceID)
	result := executePMObjectiveTestCommand(t, env.commands, context.Background(), meta, "pm.list_objectives", `{"page":2,"per_page":1}`)
	if result["total"] != float64(5) || len(result["objectives"].([]any)) != 1 {
		t.Fatalf("unexpected objective page: %#v", result)
	}
	if queryCount > 12 {
		t.Fatalf("pm.list_objectives queries = %d, want at most 12 regardless of off-page rows", queryCount)
	}
}

func TestPMCommandListObjectivesHugePositivePageReturnsEmpty(t *testing.T) {
	env := newPMObjectiveCommandTestEnv(t)
	env.seedObjective(t, "objective-huge-page", env.workspaceID, "Huge page guard", []string{env.teamA})
	meta := env.meta("workspace", env.workspaceID)
	result := executePMObjectiveTestCommand(t, env.commands, context.Background(), meta, "pm.list_objectives", fmt.Sprintf(`{"page":%d,"per_page":100}`, math.MaxInt))
	if result["total"] != float64(1) || len(result["objectives"].([]any)) != 0 {
		t.Fatalf("huge command page returned rows: %#v", result)
	}
}

func TestPMCommandCreateAndUpdateObjectiveAuthorizationAssociationsAndDates(t *testing.T) {
	env := newPMObjectiveCommandTestEnv(t)
	now := time.Now().UTC()
	for _, label := range []model.PMLabel{
		{ID: "objective-label-shared", WorkspaceID: env.workspaceID, Name: "Shared"},
		{ID: "objective-label-a", WorkspaceID: env.workspaceID, TeamID: &env.teamA, Name: "A"},
		{ID: "objective-label-other", WorkspaceID: env.otherWorkspaceID, TeamID: &env.teamOther, Name: "Other"},
	} {
		if err := env.db.Create(&label).Error; err != nil {
			t.Fatalf("seed label: %v", err)
		}
	}
	mustExec(t, env.db, `INSERT INTO pm_epics (id, workspace_id, name, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, "objective-epic-a", env.workspaceID, "Epic A", env.teamA, false, now, now)
	mustExec(t, env.db, `INSERT INTO pm_epics (id, workspace_id, name, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, "objective-epic-other", env.otherWorkspaceID, "Epic Other", env.teamOther, false, now, now)

	manager := &authorization.Actor{UserID: "objective-manager", WorkspaceID: env.workspaceID, Role: "manager", TeamMemberships: []authorization.TeamRole{{TeamID: env.teamA, Role: "owner"}}}
	managerCtx := authorization.WithActor(context.Background(), manager)
	managerMeta := env.meta("workspace", env.workspaceID)
	managerMeta.ActorID, managerMeta.ActorRole, managerMeta.AgentTeamIDs = manager.UserID, manager.Role, []string{"objective-team-unrelated", env.teamA}
	created := executePMObjectiveTestCommand(t, env.commands, managerCtx, managerMeta, "pm.create_objective", `{
		"name":"Grow adoption","objective_type":"strategic","state":"active","planned_start_date":"2026-09-01","deadline":"2026-12-31",
		"team_ids":["`+env.teamA+`","`+env.teamB+`"],"owner_member_ids":["objective-member-owner"],
		"label_ids":["objective-label-shared","objective-label-a"],"epic_ids":["objective-epic-a"]
	}`)
	objectiveID := created["objective_id"].(string)
	if objectiveID == "" || len(created["teams"].([]any)) != 2 || created["planned_start_date"] != "2026-09-01" {
		t.Fatalf("created objective = %#v", created)
	}

	objectiveMeta := managerMeta
	objectiveMeta.TargetType, objectiveMeta.TargetID = "objective", objectiveID
	updated := executePMObjectiveTestCommand(t, env.commands, managerCtx, objectiveMeta, "pm.update_objective", `{"name":"Grow adoption faster","owner_member_ids":[],"label_ids":[],"epic_ids":[],"planned_start_date":""}`)
	if updated["name"] != "Grow adoption faster" || updated["planned_start_date"] != "" || updated["deadline"] != "2026-12-31" || len(updated["owners"].([]any)) != 0 || len(updated["labels"].([]any)) != 0 || len(updated["epics"].([]any)) != 0 {
		t.Fatalf("updated objective = %#v", updated)
	}
	updated = executePMObjectiveTestCommand(t, env.commands, managerCtx, objectiveMeta, "pm.update_objective", `{"deadline":""}`)
	if updated["planned_start_date"] != "" || updated["deadline"] != "" {
		t.Fatalf("independent deadline clear changed the other date: %#v", updated)
	}

	if _, err := env.commands.Execute(managerCtx, objectiveMeta, "pm.update_objective", json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "at least one editable field") {
		t.Fatalf("no-op error = %v", err)
	}
	if _, err := env.commands.Execute(managerCtx, objectiveMeta, "pm.update_objective", json.RawMessage(`{"team_ids":["`+env.teamB+`"]}`)); err == nil || !strings.Contains(err.Error(), "only team managers") {
		t.Fatalf("destination manager error = %v", err)
	}
	if _, err := env.commands.Execute(managerCtx, objectiveMeta, "pm.update_objective", json.RawMessage(`{"team_ids":[]}`)); err == nil {
		t.Fatal("manager cleared objective teams")
	}
	if _, err := env.commands.Execute(managerCtx, managerMeta, "pm.create_objective", json.RawMessage(`{"name":"Unthemed","objective_type":"tactical"}`)); err == nil {
		t.Fatal("manager created unthemed objective")
	}
	if _, err := env.commands.Execute(managerCtx, managerMeta, "pm.create_objective", json.RawMessage(`{"name":"Cross workspace","objective_type":"tactical","team_ids":["`+env.teamOther+`"]}`)); err == nil || !strings.Contains(err.Error(), "team not found") {
		t.Fatalf("cross-workspace team error = %v", err)
	}
	if _, err := env.commands.Execute(managerCtx, managerMeta, "pm.create_objective", json.RawMessage(`{"name":"Cross workspace","objective_type":"tactical","team_ids":["`+env.teamA+`"],"owner_member_ids":["objective-member-other"]}`)); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("cross-workspace owner error = %v", err)
	}
	if _, err := env.commands.Execute(managerCtx, managerMeta, "pm.create_objective", json.RawMessage(`{"name":"Cross workspace","objective_type":"tactical","team_ids":["`+env.teamA+`"],"owner_ids":["objective-other"]}`)); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("cross-workspace owner user error = %v", err)
	}
	if _, err := env.commands.Execute(managerCtx, managerMeta, "pm.create_objective", json.RawMessage(`{"name":"Cross workspace","objective_type":"tactical","team_ids":["`+env.teamA+`"],"label_ids":["objective-label-other"]}`)); err == nil || !strings.Contains(err.Error(), "label") {
		t.Fatalf("cross-workspace label error = %v", err)
	}
	if _, err := env.commands.Execute(managerCtx, managerMeta, "pm.create_objective", json.RawMessage(`{"name":"Cross workspace","objective_type":"tactical","team_ids":["`+env.teamA+`"],"epic_ids":["objective-epic-other"]}`)); err == nil || !strings.Contains(err.Error(), "epic") {
		t.Fatalf("cross-workspace epic error = %v", err)
	}
	for _, invalidUpdate := range []string{
		`{"name":"Must Roll Back","owner_member_ids":["objective-member-other"]}`,
		`{"name":"Must Roll Back","owner_ids":["objective-other"]}`,
		`{"name":"Must Roll Back","label_ids":["objective-label-other"]}`,
		`{"name":"Must Roll Back","epic_ids":["objective-epic-other"]}`,
	} {
		if _, err := env.commands.Execute(managerCtx, objectiveMeta, "pm.update_objective", json.RawMessage(invalidUpdate)); err == nil {
			t.Fatalf("invalid association update succeeded: %s", invalidUpdate)
		}
		persisted, err := env.objectives.GetByID(context.Background(), objectiveID, env.workspaceID)
		if err != nil || persisted.Objective.Name != "Grow adoption faster" {
			t.Fatalf("invalid association update changed scalar: objective=%#v err=%v", persisted, err)
		}
	}
}

func TestPMCommandObjectiveWritesIntersectResolvedAgentScope(t *testing.T) {
	env := newPMObjectiveCommandTestEnv(t)
	env.seedObjective(t, "objective-scope-a", env.workspaceID, "A", []string{env.teamA, env.teamB})
	env.seedObjective(t, "objective-scope-b", env.workspaceID, "B", []string{env.teamB})
	admin := &authorization.Actor{UserID: "objective-admin", WorkspaceID: env.workspaceID, Role: model.RoleAdmin}
	ctx := authorization.WithActor(context.Background(), admin)
	meta := env.meta("workspace", env.workspaceID)
	meta.AgentTeamIDs = []string{"objective-team-unrelated", env.teamA}

	if _, err := env.commands.Execute(ctx, meta, "pm.update_objective", json.RawMessage(`{"objective_id":"objective-scope-a","name":"Allowed"}`)); err != nil {
		t.Fatalf("multi-team intersecting update: %v", err)
	}
	if _, err := env.commands.Execute(ctx, meta, "pm.update_objective", json.RawMessage(`{"objective_id":"objective-scope-b","name":"Denied"}`)); err == nil || !strings.Contains(err.Error(), "agent does not have access") {
		t.Fatalf("out-of-scope update error = %v", err)
	}
	if _, err := env.commands.Execute(ctx, meta, "pm.create_objective", json.RawMessage(`{"name":"Denied","objective_type":"tactical","team_ids":["`+env.teamB+`"]}`)); err == nil || !strings.Contains(err.Error(), "agent does not have access") {
		t.Fatalf("out-of-scope create error = %v", err)
	}
	if _, err := env.commands.Execute(ctx, meta, "pm.create_objective", json.RawMessage(`{"name":"Unthemed denied","objective_type":"tactical","team_ids":[]}`)); err == nil || !strings.Contains(err.Error(), "agent does not have access") {
		t.Fatalf("team-scoped agent unthemed create error = %v", err)
	}
	if _, err := env.commands.Execute(ctx, meta, "pm.update_objective", json.RawMessage(`{"objective_id":"objective-scope-a","team_ids":[]}`)); err == nil || !strings.Contains(err.Error(), "agent does not have access") {
		t.Fatalf("team-scoped agent clear teams error = %v", err)
	}
	env.seedObjective(t, "objective-scope-unteamed", env.workspaceID, "Unthemed", nil)
	if _, err := env.commands.Execute(ctx, meta, "pm.update_objective", json.RawMessage(`{"objective_id":"objective-scope-unteamed","name":"Denied"}`)); err == nil || !strings.Contains(err.Error(), "agent does not have access") {
		t.Fatalf("unteamed objective update error = %v", err)
	}
}

func TestPMCommandObjectiveWritesUseFallbackAuditActor(t *testing.T) {
	env := newPMObjectiveCommandTestEnv(t)
	meta := env.meta("workspace", env.workspaceID)
	meta.ActorID = ""
	meta.AuditActorID = "objective-admin"
	created := executePMObjectiveTestCommand(t, env.commands, context.Background(), meta, "pm.create_objective", `{"name":"Audited","objective_type":"tactical","team_ids":["`+env.teamA+`"]}`)
	objectiveID := created["objective_id"].(string)
	objective, err := env.objectives.GetByID(context.Background(), objectiveID, env.workspaceID)
	if err != nil || objective.Objective.CreatedBy == nil || *objective.Objective.CreatedBy != "objective-admin" {
		t.Fatalf("objective audit actor = %#v err=%v", objective, err)
	}
	meta.TargetType, meta.TargetID = "objective", objectiveID
	keyResult := executePMObjectiveTestCommand(t, env.commands, context.Background(), meta, "pm.create_key_result", `{"name":"Audited KR","result_type":"boolean"}`)
	var persisted model.PMKeyResult
	if err := env.db.First(&persisted, "id = ?", keyResult["key_result_id"]).Error; err != nil || persisted.UpdatedBy == nil || *persisted.UpdatedBy != "objective-admin" {
		t.Fatalf("key result audit actor = %#v err=%v", persisted, err)
	}
	if _, err := env.commands.Execute(context.Background(), meta, "pm.create_key_result", json.RawMessage(`{"name":"Hidden field","position":4}`)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("hidden key-result field error = %v", err)
	}
}

func TestPMCommandObjectiveTeamReplacementValidatesPreservedLabels(t *testing.T) {
	env := newPMObjectiveCommandTestEnv(t)
	if err := env.db.Create(&model.PMLabel{ID: "objective-preserved-label-a", WorkspaceID: env.workspaceID, TeamID: &env.teamA, Name: "Team A only"}).Error; err != nil {
		t.Fatalf("seed label: %v", err)
	}
	admin := &authorization.Actor{UserID: "objective-admin", WorkspaceID: env.workspaceID, Role: model.RoleAdmin}
	ctx := authorization.WithActor(context.Background(), admin)
	meta := env.meta("workspace", env.workspaceID)
	meta.AgentTeamIDs = nil
	created := executePMObjectiveTestCommand(t, env.commands, ctx, meta, "pm.create_objective", `{"name":"Preserve label","objective_type":"tactical","team_ids":["`+env.teamA+`"],"label_ids":["objective-preserved-label-a"]}`)
	objectiveID := created["objective_id"].(string)
	if _, err := env.commands.Execute(ctx, meta, "pm.update_objective", json.RawMessage(`{"objective_id":"`+objectiveID+`","name":"Must not persist","team_ids":["`+env.teamB+`"]}`)); err == nil || !strings.Contains(err.Error(), "existing objective labels") {
		t.Fatalf("preserved-label destination error = %v", err)
	}
	persisted, err := env.objectives.GetByID(context.Background(), objectiveID, env.workspaceID)
	if err != nil || persisted.Objective.Name != "Preserve label" || len(persisted.Teams) != 1 || persisted.Teams[0] != env.teamA {
		t.Fatalf("invalid destination partially updated objective: %#v err=%v", persisted, err)
	}
}

func TestPMCommandKeyResultsRequireCurrentObjectiveParent(t *testing.T) {
	env := newPMObjectiveCommandTestEnv(t)
	env.seedObjective(t, "objective-kr-a", env.workspaceID, "A", []string{env.teamA})
	env.seedObjective(t, "objective-kr-b", env.workspaceID, "B", []string{env.teamB})
	env.seedObjective(t, "objective-kr-other", env.otherWorkspaceID, "Other", []string{env.teamOther})
	admin := &authorization.Actor{UserID: "objective-admin", WorkspaceID: env.workspaceID, Role: model.RoleAdmin}
	ctx := authorization.WithActor(context.Background(), admin)
	meta := env.meta("objective", "objective-kr-a")
	created := executePMObjectiveTestCommand(t, env.commands, ctx, meta, "pm.create_key_result", `{"name":"Activation","result_type":"numeric","initial_value":10,"current_value":20,"target_value":50}`)
	keyResultID := created["key_result_id"].(string)
	if keyResultID == "" || created["progress"] != float64(25) {
		t.Fatalf("created key result = %#v", created)
	}
	updated := executePMObjectiveTestCommand(t, env.commands, ctx, meta, "pm.update_key_result", `{"key_result_id":"`+keyResultID+`","current_value":0}`)
	if updated["current_value"] != float64(0) {
		t.Fatalf("zero numeric update lost: %#v", updated)
	}

	now := time.Now().UTC()
	mustExec(t, env.db, `INSERT INTO pm_key_results (id, objective_id, name, result_type, initial_value, current_value, target_value, progress, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "kr-sibling", "objective-kr-b", "Sibling", model.PMKeyResultTypeBoolean, 0, 0, 1, 0, now, now)
	mustExec(t, env.db, `INSERT INTO pm_key_results (id, objective_id, name, result_type, initial_value, current_value, target_value, progress, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "kr-other", "objective-kr-other", "Other", model.PMKeyResultTypeBoolean, 0, 0, 1, 0, now, now)
	for _, id := range []string{"kr-sibling", "kr-other"} {
		if _, err := env.commands.Execute(ctx, meta, "pm.update_key_result", json.RawMessage(`{"key_result_id":"`+id+`","current_value":1}`)); err == nil || !strings.Contains(err.Error(), "does not belong") {
			t.Fatalf("parent scope error for %s = %v", id, err)
		}
	}
	if _, err := env.commands.Execute(ctx, meta, "pm.update_key_result", json.RawMessage(`{"key_result_id":"`+keyResultID+`"}`)); err == nil || !strings.Contains(err.Error(), "at least one editable field") {
		t.Fatalf("key result no-op error = %v", err)
	}
	workspaceMeta := env.meta("workspace", env.workspaceID)
	if _, err := env.commands.Execute(ctx, workspaceMeta, "pm.create_key_result", json.RawMessage(`{"name":"No parent"}`)); err == nil || !strings.Contains(err.Error(), "does not support target type") {
		t.Fatalf("workspace key result error = %v", err)
	}
	crossWorkspaceMeta := env.meta("objective", "objective-kr-other")
	if _, err := env.commands.Execute(ctx, crossWorkspaceMeta, "pm.create_key_result", json.RawMessage(`{"name":"Cross workspace"}`)); err == nil || !strings.Contains(err.Error(), "objective not found") {
		t.Fatalf("cross-workspace create key result error = %v", err)
	}
}
