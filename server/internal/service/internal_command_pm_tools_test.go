package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

type pmCommandTestEnv struct {
	db          *gorm.DB
	service     *InternalCommandService
	workspaceID string
	actorID     string
}

func newPMCommandTestEnv(t *testing.T) *pmCommandTestEnv {
	t.Helper()
	db := newTestDB(t)
	now := time.Now().UTC()
	seedUser(t, db, "actor-1", "actor@example.com", "Actor", "hash")
	seedUser(t, db, "user-2", "second@example.com", "Second User", "hash")
	seedUser(t, db, "inactive-user", "inactive@example.com", "Inactive User", "hash")
	seedWorkspace(t, db, "ws-1", "Workspace", "workspace", "actor-1")
	seedWorkspace(t, db, "ws-2", "Other", "other", "actor-1")
	seedWorkspaceMember(t, db, "member-1", "ws-1", "actor-1", "actor@example.com", "Actor", model.RoleAdmin)
	seedWorkspaceMember(t, db, "member-2", "ws-1", "user-2", "second@example.com", "Second User", model.RoleMember)
	seedWorkspaceMember(t, db, "member-inactive", "ws-1", "inactive-user", "inactive@example.com", "Inactive User", model.RoleMember)
	mustExec(t, db, `UPDATE workspace_members SET status = ? WHERE id = ?`, model.WorkspaceMemberStatusInactive, "member-inactive")
	for _, team := range []struct{ id, workspaceID, name string }{
		{"team-a", "ws-1", "Alpha"}, {"team-b", "ws-1", "Beta"}, {"team-other", "ws-2", "Other"},
	} {
		mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, team_type, default_task_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, team.id, team.workspaceID, team.name, "engineering", "feature", now, now)
	}
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, "tm-1", "team-a", "member-1", "owner", now, now)
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, "tm-2", "team-a", "member-2", "member", now, now)

	for _, wf := range []struct{ id, workspaceID, teamID, stateID string }{
		{"wf-a", "ws-1", "team-a", "state-a"}, {"wf-b", "ws-1", "team-b", "state-b"}, {"wf-other", "ws-2", "team-other", "state-other"},
	} {
		mustExec(t, db, `INSERT INTO pm_workflows (id, workspace_id, name, team_id, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, wf.id, wf.workspaceID, wf.id, wf.teamID, wf.stateID, now, now)
		mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, wf.stateID, wf.id, "Todo", model.PMStateTypeUnstarted, 0, true, now, now)
	}
	mustExec(t, db, `INSERT INTO pm_epics (id, workspace_id, name, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, "epic-a", "ws-1", "Epic A", "team-a", false, now, now)
	mustExec(t, db, `INSERT INTO pm_epics (id, workspace_id, name, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, "epic-b", "ws-1", "Epic B", "team-b", false, now, now)
	mustExec(t, db, `INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "sprint-a", "ws-1", "Sprint A", now, now.AddDate(0, 0, 14), "team-a", false, now, now)
	mustExec(t, db, `INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "sprint-b", "ws-1", "Sprint B", now, now.AddDate(0, 0, 14), "team-b", false, now, now)
	mustExec(t, db, `INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "sprint-other", "ws-2", "Sprint Other", now, now.AddDate(0, 0, 14), "team-other", false, now, now)

	labels := []struct {
		id, workspaceID, teamID, name string
		archived                      bool
	}{
		{"label-shared", "ws-1", "", "Shared", false},
		{"label-a", "ws-1", "team-a", "Alpha", false},
		{"label-b", "ws-1", "team-b", "Beta", false},
		{"label-archived", "ws-1", "team-a", "Archived", true},
	}
	for _, label := range labels {
		var teamID any
		if label.teamID != "" {
			teamID = label.teamID
		}
		mustExec(t, db, `INSERT INTO pm_labels (id, workspace_id, team_id, name, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, label.id, label.workspaceID, teamID, label.name, label.archived, now, now)
	}

	taskRepo := repository.NewPMTaskRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	sprintRepo := repository.NewPMSprintRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	checklistRepo := repository.NewPMChecklistItemRepository(db)
	activityService := NewPMActivityService(repository.NewPMActivityRepository(db))
	taskService := NewPMTaskService(taskRepo, workspaceRepo, workflowRepo, epicRepo, sprintRepo, labelRepo, checklistRepo, nil, nil, activityService, nil, nil, nil, nil)
	commentService := NewPMCommentService(repository.NewPMCommentRepository(db), taskRepo, nil, activityService, nil, nil, workspaceRepo, nil)
	checklistService := NewPMChecklistItemService(checklistRepo, taskRepo, nil, nil, workspaceRepo)
	commandService := NewInternalCommandService(nil, taskService, nil, nil, nil, nil, taskRepo, repository.NewPMTaskLinkRepository(db))
	commandService.SetPMLabelService(NewPMLabelService(labelRepo, nil))
	commandService.SetPMCommentService(commentService)
	commandService.SetPMOperationalServices(
		workspaceRepo,
		NewPMEpicService(epicRepo, taskRepo, labelRepo, nil, nil, workspaceRepo, activityService, nil, nil),
		NewPMSprintService(sprintRepo, labelRepo, nil, workspaceRepo, repository.NewSettingsRepository(db), activityService, nil, nil, nil),
		nil,
		NewPMWorkflowService(workflowRepo, taskRepo, labelRepo, nil),
		checklistService,
	)
	return &pmCommandTestEnv{db: db, service: commandService, workspaceID: "ws-1", actorID: "actor-1"}
}

func (e *pmCommandTestEnv) meta(targetType, targetID string) model.InternalCommandContext {
	return model.InternalCommandContext{WorkspaceID: e.workspaceID, ActorID: e.actorID, ActorRole: model.RoleAdmin, TargetType: targetType, TargetID: targetID}
}

func seedPMCommandTask(t *testing.T, db *gorm.DB, id, workspaceID, teamID, workflowID, stateID, epicID, sprintID string, displayID int) {
	t.Helper()
	now := time.Now().UTC()
	var epic, sprint any
	if epicID != "" {
		epic = epicID
	}
	if sprintID != "" {
		sprint = sprintID
	}
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, epic_id, sprint_id, team_id, priority, severity, completed, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, workspaceID, displayID, id, model.PMTaskTypeFeature, workflowID, stateID, epic, sprint, teamID, model.PMTaskPriorityMedium, model.PMTaskSeverityNone, false, false, now, now)
}

func TestPMCommandListWorkspaceMembers(t *testing.T) {
	env := newPMCommandTestEnv(t)
	out, err := env.service.Execute(context.Background(), env.meta("workspace", env.workspaceID), "workspace.list_members", json.RawMessage(`{"page":1,"per_page":100}`))
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if strings.Contains(string(out), "@example.com") {
		t.Fatalf("member output leaked email: %s", out)
	}
	var result struct {
		Members []struct {
			MemberID    string   `json:"member_id"`
			UserID      string   `json:"user_id"`
			DisplayName string   `json:"display_name"`
			TeamIDs     []string `json:"team_ids"`
		} `json:"members"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("decode members: %v", err)
	}
	if result.Total != 2 || len(result.Members) != 2 || result.Members[0].MemberID != "member-1" || len(result.Members[0].TeamIDs) != 1 || result.Members[0].TeamIDs[0] != "team-a" {
		t.Fatalf("unexpected members: %#v", result)
	}

	def, ok := env.service.Definition("workspace.list_members")
	if !ok || len(def.RequiredPermissionsAll) != 2 || def.RequiredPermissionsAll[0] != authorization.PermWorkspaceRead || def.RequiredPermissionsAll[1] != authorization.PermPMRead {
		t.Fatalf("member permissions = %#v", def.RequiredPermissionsAll)
	}
	env.service.SetAuthorizationService(authorization.NewAuthzService(nil, nil, nil))
	denied := env.meta("workspace", env.workspaceID)
	denied.ActorRole = "role-without-one-required-permission"
	if _, err := env.service.Execute(context.Background(), denied, "workspace.list_members", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected member discovery to deny an actor missing required permissions")
	}
}

func TestPMCommandListLabels(t *testing.T) {
	env := newPMCommandTestEnv(t)
	ctx := authorization.WithActor(context.Background(), &authorization.Actor{UserID: env.actorID, WorkspaceID: env.workspaceID, Role: model.RoleMember, TeamMemberships: []authorization.TeamRole{{TeamID: "team-a", Role: "member"}}})
	out, err := env.service.Execute(ctx, model.InternalCommandContext{WorkspaceID: env.workspaceID, ActorID: env.actorID, ActorRole: model.RoleMember, ActorTeamIDs: []string{"team-a"}, TargetType: "sprint", TargetID: "sprint-a"}, "pm.list_labels", json.RawMessage(`{"page":1,"per_page":100}`))
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	var result struct {
		Labels []struct {
			ID   string  `json:"label_id"`
			Name string  `json:"name"`
			Team *string `json:"team_id"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("decode labels: %v", err)
	}
	if len(result.Labels) != 2 || result.Labels[0].ID != "label-shared" || result.Labels[1].ID != "label-a" {
		t.Fatalf("labels = %#v, want shared and team-a only", result.Labels)
	}
}

func TestPMCommandListTasksFilters(t *testing.T) {
	env := newPMCommandTestEnv(t)
	seedPMCommandTask(t, env.db, "task-a", "ws-1", "team-a", "wf-a", "state-a", "epic-a", "sprint-a", 1)
	seedPMCommandTask(t, env.db, "task-b", "ws-1", "team-b", "wf-b", "state-b", "epic-b", "sprint-b", 2)
	ctx := authorization.WithActor(context.Background(), &authorization.Actor{UserID: env.actorID, WorkspaceID: env.workspaceID, Role: model.RoleMember, TeamMemberships: []authorization.TeamRole{{TeamID: "team-a", Role: "member"}}})
	meta := model.InternalCommandContext{WorkspaceID: env.workspaceID, ActorID: env.actorID, ActorRole: model.RoleMember, ActorTeamIDs: []string{"team-a"}, TargetType: "workspace", TargetID: env.workspaceID}
	out, err := env.service.Execute(ctx, meta, "pm.list_tasks", json.RawMessage(`{"epic_id":"epic-a","sprint_id":"sprint-a","workflow_id":"wf-a","state_id":"state-a","task_type":"feature","priority":"medium","severity":"none","completed":false,"archived":false,"page":1,"per_page":25}`))
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	var result struct {
		Tasks []struct {
			TaskID string `json:"task_id"`
		} `json:"tasks"`
		Total   int64 `json:"total"`
		Page    int   `json:"page"`
		PerPage int   `json:"per_page"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("decode tasks: %v", err)
	}
	if result.Total != 1 || len(result.Tasks) != 1 || result.Tasks[0].TaskID != "task-a" || result.Page != 1 || result.PerPage != 25 {
		t.Fatalf("unexpected filtered tasks: %#v", result)
	}

	out, err = env.service.Execute(ctx, meta, "pm.list_tasks", json.RawMessage(`{"archived":false,"page":1,"per_page":100}`))
	if err != nil {
		t.Fatalf("team-scoped list tasks: %v", err)
	}
	if strings.Contains(string(out), "task-b") {
		t.Fatalf("team-scoped task list leaked task-b: %s", out)
	}
}

func TestPMCommandCreateTask(t *testing.T) {
	env := newPMCommandTestEnv(t)
	out, err := env.service.Execute(context.Background(), env.meta("sprint", "sprint-a"), "pm.create_task", json.RawMessage(`{
		"name":"Sprint-scoped task","team_id":"team-a","task_type":"bug","severity":"major","blocked":true,"blocker":"Awaiting API"
	}`))
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	var result struct {
		TaskID   string  `json:"task_id"`
		SprintID *string `json:"sprint_id"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("decode create task: %v", err)
	}
	if result.TaskID == "" || result.SprintID == nil || *result.SprintID != "sprint-a" {
		t.Fatalf("create result = %#v", result)
	}
	var created model.PMTask
	if err := env.db.First(&created, "id = ?", result.TaskID).Error; err != nil {
		t.Fatalf("load created task: %v", err)
	}
	if created.SprintID == nil || *created.SprintID != "sprint-a" || created.Severity != model.PMTaskSeverityMajor || !created.Blocked || created.Blocker == nil || *created.Blocker != "Awaiting API" {
		t.Fatalf("created task = %#v", created)
	}

	if _, err := env.service.Execute(context.Background(), env.meta("sprint", "sprint-a"), "pm.create_task", json.RawMessage(`{"name":"Conflict","team_id":"team-b","sprint_id":"sprint-b"}`)); err == nil || !strings.Contains(err.Error(), "target") {
		t.Fatalf("expected sprint target conflict, got %v", err)
	}
	if _, err := env.service.Execute(context.Background(), env.meta("sprint", "sprint-a"), "pm.create_task", json.RawMessage(`{"name":"Mismatch","team_id":"team-b"}`)); err == nil {
		t.Fatal("expected sprint/team mismatch")
	}
	if _, err := env.service.Execute(context.Background(), env.meta("epic", "epic-a"), "pm.create_task", json.RawMessage(`{"name":"Conflict","team_id":"team-b","epic_id":"epic-b"}`)); err == nil || !strings.Contains(err.Error(), "target") {
		t.Fatalf("expected epic target conflict, got %v", err)
	}
}

func TestPMCommandGetTask(t *testing.T) {
	env := newPMCommandTestEnv(t)
	seedPMCommandTask(t, env.db, "task-a", "ws-1", "team-a", "wf-a", "state-a", "epic-a", "sprint-a", 1)
	seedPMCommandTask(t, env.db, "task-other", "ws-2", "team-other", "wf-other", "state-other", "", "sprint-other", 1)
	out, err := env.service.Execute(context.Background(), env.meta("task", "task-a"), "pm.get_task", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !strings.Contains(string(out), `"task_id":"task-a"`) || !strings.Contains(string(out), `"sprint_id":"sprint-a"`) {
		t.Fatalf("unexpected task output: %s", out)
	}
	if _, err := env.service.Execute(context.Background(), env.meta("workspace", "ws-1"), "pm.get_task", json.RawMessage(`{"task_id":"task-other"}`)); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected cross-workspace rejection, got %v", err)
	}
}

func TestPMCommandUpdateTask(t *testing.T) {
	env := newPMCommandTestEnv(t)
	seedPMCommandTask(t, env.db, "task-a", "ws-1", "team-a", "wf-a", "state-a", "epic-a", "sprint-a", 1)
	mustExec(t, env.db, `UPDATE pm_tasks SET deadline = ?, blocker = ?, blocked = ? WHERE id = ?`, "2026-08-20", "Old blocker", true, "task-a")
	out, err := env.service.Execute(context.Background(), env.meta("task", "task-a"), "pm.update_task", json.RawMessage(`{
		"name":"Updated task","task_type":"chore","epic_id":"","sprint_id":"","priority":"urgent","severity":"critical","deadline":"","blocker":"","blocked":false,"owner_member_ids":[],"label_ids":[]
	}`))
	if err != nil {
		t.Fatalf("update task: %v", err)
	}
	if !strings.Contains(string(out), `"name":"Updated task"`) {
		t.Fatalf("unexpected update output: %s", out)
	}
	var updated model.PMTask
	if err := env.db.First(&updated, "id = ?", "task-a").Error; err != nil {
		t.Fatalf("load updated task: %v", err)
	}
	if updated.EpicID != nil || updated.SprintID != nil || updated.Deadline != nil || updated.Blocker != nil || updated.Blocked || updated.Priority != model.PMTaskPriorityUrgent || updated.Severity != model.PMTaskSeverityCritical {
		t.Fatalf("updated task = %#v", updated)
	}
	if _, err := env.service.Execute(context.Background(), env.meta("task", "task-a"), "pm.update_task", json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "editable") {
		t.Fatalf("expected no-op rejection, got %v", err)
	}
	def, _ := env.service.Definition("pm.update_task")
	properties := def.Tool.InputSchema["properties"].(map[string]any)
	for _, forbidden := range []string{"team_id", "workflow_id", "state_id", "archived"} {
		if _, ok := properties[forbidden]; ok {
			t.Fatalf("update task schema exposes forbidden field %q", forbidden)
		}
	}
}

func TestPMCommandChecklist(t *testing.T) {
	env := newPMCommandTestEnv(t)
	seedPMCommandTask(t, env.db, "task-a", "ws-1", "team-a", "wf-a", "state-a", "epic-a", "sprint-a", 1)
	createOut, err := env.service.Execute(context.Background(), env.meta("task", "task-a"), "pm.create_task_checklist_item", json.RawMessage(`{"text":"Write tests","position":2,"assignee_id":"actor-1"}`))
	if err != nil {
		t.Fatalf("create checklist item: %v", err)
	}
	var created struct {
		ChecklistItemID string `json:"checklist_item_id"`
	}
	if err := json.Unmarshal(createOut, &created); err != nil || created.ChecklistItemID == "" {
		t.Fatalf("decode checklist result: %#v, %v", created, err)
	}
	listOut, err := env.service.Execute(context.Background(), env.meta("task", "task-a"), "pm.list_task_checklist", json.RawMessage(`{}`))
	if err != nil || !strings.Contains(string(listOut), "Write tests") {
		t.Fatalf("list checklist = %s, %v", listOut, err)
	}
	updateOut, err := env.service.Execute(context.Background(), env.meta("task", "task-a"), "pm.update_task_checklist_item", json.RawMessage(`{"checklist_item_id":"`+created.ChecklistItemID+`","text":"Ship tests","completed":true,"assignee_id":""}`))
	if err != nil || !strings.Contains(string(updateOut), "Ship tests") {
		t.Fatalf("update checklist = %s, %v", updateOut, err)
	}
	var item model.PMChecklistItem
	if err := env.db.First(&item, "id = ?", created.ChecklistItemID).Error; err != nil {
		t.Fatalf("load checklist item: %v", err)
	}
	if item.Text != "Ship tests" || !item.Completed || item.AssigneeID != nil {
		t.Fatalf("updated checklist item = %#v", item)
	}
}

func TestPMCommandAddComment(t *testing.T) {
	env := newPMCommandTestEnv(t)
	seedPMCommandTask(t, env.db, "task-a", "ws-1", "team-a", "wf-a", "state-a", "epic-a", "sprint-a", 1)
	out, err := env.service.Execute(context.Background(), env.meta("task", "task-a"), "pm.add_comment", json.RawMessage(`{"content":"## Status\n\nReady"}`))
	if err != nil || !strings.Contains(string(out), `"entity_type":"task"`) {
		t.Fatalf("add target-defaulted comment = %s, %v", out, err)
	}
	if _, err := env.service.Execute(context.Background(), env.meta("workspace", "ws-1"), "pm.add_comment", json.RawMessage(`{"entity_type":"invalid","entity_id":"task-a","content":"No"}`)); err == nil || !strings.Contains(err.Error(), "entity_type") {
		t.Fatalf("expected entity type rejection, got %v", err)
	}
}

func TestPMCommandParentChild(t *testing.T) {
	env := newPMCommandTestEnv(t)
	seedPMCommandTask(t, env.db, "task-a", "ws-1", "team-a", "wf-a", "state-a", "epic-a", "sprint-a", 1)
	seedPMCommandTask(t, env.db, "task-b", "ws-1", "team-b", "wf-b", "state-b", "epic-b", "sprint-b", 2)

	for _, tc := range []struct {
		name, command, input string
	}{
		{"update", "pm.update_task", `{"task_id":"task-b","name":"No"}`},
		{"state", "pm.update_task_state", `{"task_id":"task-b","state_id":"state-b"}`},
		{"comment", "pm.add_task_comment", `{"task_id":"task-b","content":"No"}`},
		{"checklist", "pm.create_task_checklist_item", `{"task_id":"task-b","text":"No"}`},
		{"dependencies", "pm.set_task_dependencies", `{"dependencies":[{"source_task_id":"task-a","target_task_id":"task-b"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := env.service.Execute(context.Background(), env.meta("sprint", "sprint-a"), tc.command, json.RawMessage(tc.input)); err == nil || !strings.Contains(err.Error(), "target") {
				t.Fatalf("expected parent-child rejection, got %v", err)
			}
		})
	}

	for _, command := range []string{"pm.update_task_state", "pm.add_task_comment"} {
		def, ok := env.service.Definition(command)
		if !ok || !containsCommandTarget(def.SupportedTargetTypes, "epic") || !containsCommandTarget(def.SupportedTargetTypes, "sprint") {
			t.Fatalf("%s targets = %#v", command, def.SupportedTargetTypes)
		}
	}
	dependencyDef, _ := env.service.Definition("pm.set_task_dependencies")
	if !containsCommandTarget(dependencyDef.SupportedTargetTypes, "workspace") || !containsCommandTarget(dependencyDef.SupportedTargetTypes, "sprint") {
		t.Fatalf("dependency targets = %#v", dependencyDef.SupportedTargetTypes)
	}
	getContextDef, _ := env.service.Definition("release.get_task_context")
	for _, target := range []string{"workspace", "task", "epic", "sprint", "objective"} {
		if !containsCommandTarget(getContextDef.SupportedTargetTypes, target) {
			t.Fatalf("get task context missing target %q: %#v", target, getContextDef.SupportedTargetTypes)
		}
	}
	teamDef, _ := env.service.Definition("workspace.list_teams")
	if !containsCommandTarget(teamDef.SupportedTargetTypes, "sprint") || !containsCommandTarget(teamDef.SupportedTargetTypes, "objective") {
		t.Fatalf("workspace teams targets = %#v", teamDef.SupportedTargetTypes)
	}
}

func TestPMCommandListTeamWorkflows(t *testing.T) {
	env := newPMCommandTestEnv(t)
	out, err := env.service.Execute(context.Background(), env.meta("objective", "objective-a"), "pm.list_team_workflows_with_stages", json.RawMessage(`{"team_id":"team-a"}`))
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	if !strings.Contains(string(out), `"workflow_id":"wf-a"`) || !strings.Contains(string(out), `"state_id":"state-a"`) {
		t.Fatalf("unexpected workflows output: %s", out)
	}
}

func containsCommandTarget(targets []string, want string) bool {
	for _, target := range targets {
		if target == want {
			return true
		}
	}
	return false
}
