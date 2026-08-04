package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func newPMSprintCommandTestEnv(t *testing.T) (*InternalCommandService, *PMSprintService, *gorm.DB, string) {
	t.Helper()
	sprintService, db, workspaceID := newSprintTestEnvWithDB(t)
	commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	commandService.SetSettingsRepository(repository.NewSettingsRepository(db))
	commandService.SetPMOperationalServices(nil, nil, sprintService, nil, nil, nil)
	return commandService, sprintService, db, workspaceID
}

func executePMSprintTestCommand(t *testing.T, svc *InternalCommandService, ctx context.Context, meta model.InternalCommandContext, name, input string) map[string]any {
	t.Helper()
	output, err := svc.Execute(ctx, meta, name, json.RawMessage(input))
	if err != nil {
		t.Fatalf("Execute(%s): %v", name, err)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("decode %s output: %v (%s)", name, err, output)
	}
	return decoded
}

func TestPMCommandSprintDefinitions(t *testing.T) {
	t.Parallel()
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	tests := []struct {
		name     string
		mutating bool
		targets  []string
	}{
		{name: "pm.list_sprints", targets: []string{"workspace", "sprint"}},
		{name: "pm.get_sprint", targets: []string{"workspace", "sprint"}},
		{name: "pm.list_sprint_tasks", targets: []string{"workspace", "sprint"}},
		{name: "pm.create_sprint", mutating: true, targets: []string{"workspace", "sprint"}},
		{name: "pm.update_sprint", mutating: true, targets: []string{"workspace", "sprint"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, ok := svc.Definition(tt.name)
			if !ok {
				t.Fatalf("missing command definition %q", tt.name)
			}
			if def.Module != "pm" || def.Mutating != tt.mutating {
				t.Fatalf("unexpected command policy: module=%q mutating=%t", def.Module, def.Mutating)
			}
			if len(def.SupportedTargetTypes) != len(tt.targets) {
				t.Fatalf("targets = %#v, want %#v", def.SupportedTargetTypes, tt.targets)
			}
			for index := range tt.targets {
				if def.SupportedTargetTypes[index] != tt.targets[index] {
					t.Fatalf("targets = %#v, want %#v", def.SupportedTargetTypes, tt.targets)
				}
			}
		})
	}
}

func TestPMCommandCreateSprintStrictDatesAndPrevalidatesScope(t *testing.T) {
	t.Run("strict dates", func(t *testing.T) {
		svc, _, db, workspaceID := newPMSprintCommandTestEnv(t)
		seedPMSprintCommandTeam(t, db, workspaceID, "team-command-date")
		_, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: workspaceID, TargetType: "workspace", TargetID: workspaceID}, "pm.create_sprint", json.RawMessage(`{
			"name":"Bad date","start_date":"2026-08-04T10:00:00Z","end_date":"2026-08-18","team_id":"team-command-date"
		}`))
		if err == nil || !strings.Contains(err.Error(), "start_date must be YYYY-MM-DD") {
			t.Fatalf("expected strict start-date error, got %v", err)
		}
		var count int64
		if err := db.Model(&model.PMSprint{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
			t.Fatalf("count sprints: %v", err)
		}
		if count != 0 {
			t.Fatalf("malformed input persisted %d sprints", count)
		}
	})

	t.Run("cross-workspace team", func(t *testing.T) {
		svc, _, db, workspaceID := newPMSprintCommandTestEnv(t)
		seedWorkspace(t, db, "ws-command-other", "Other", "command-other", "owner-2")
		seedPMSprintCommandTeam(t, db, "ws-command-other", "team-command-other")
		_, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: workspaceID, TargetType: "workspace", TargetID: workspaceID}, "pm.create_sprint", json.RawMessage(`{
			"name":"Wrong team","start_date":"2026-09-01","end_date":"2026-09-15","team_id":"team-command-other"
		}`))
		if err == nil || !strings.Contains(err.Error(), "team not found in this workspace") {
			t.Fatalf("expected cross-workspace team error, got %v", err)
		}
		var count int64
		if err := db.Model(&model.PMSprint{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
			t.Fatalf("count sprints: %v", err)
		}
		if count != 0 {
			t.Fatalf("invalid team persisted %d sprints", count)
		}
	})

	t.Run("valid compact result", func(t *testing.T) {
		svc, _, db, workspaceID := newPMSprintCommandTestEnv(t)
		seedPMSprintCommandTeam(t, db, workspaceID, "team-command-valid")
		if err := db.Create(&model.PMLabel{ID: "label-command-valid", WorkspaceID: workspaceID, Name: "Launch", Color: commandStringPtr("#112233")}).Error; err != nil {
			t.Fatalf("seed label: %v", err)
		}
		result := executePMSprintTestCommand(t, svc, context.Background(), model.InternalCommandContext{WorkspaceID: workspaceID, TargetType: "workspace", TargetID: workspaceID}, "pm.create_sprint", `{
			"name":"Launch sprint","description":"Ship it","start_date":"2026-09-01","end_date":"2026-09-15","team_id":"team-command-valid","label_ids":["label-command-valid"]
		}`)
		if result["name"] != "Launch sprint" || result["start_date"] != "2026-09-01" || result["end_date"] != "2026-09-15" {
			t.Fatalf("unexpected compact sprint: %#v", result)
		}
		if result["sprint_id"] == "" || result["team_id"] != "team-command-valid" {
			t.Fatalf("missing canonical IDs: %#v", result)
		}
	})
}

func TestPMCommandActorlessCreateUsesHumanAuditActorNotAgentID(t *testing.T) {
	svc, _, db, workspaceID := newPMSprintCommandTestEnv(t)
	seedPMSprintCommandTeam(t, db, workspaceID, "team-command-audit")
	if err := db.Exec(`INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"user-command-audit", "audit@example.com", "hash", "Audit User", time.Now().UTC(), time.Now().UTC()).Error; err != nil {
		t.Fatalf("seed audit user: %v", err)
	}
	if err := db.Exec(`
		CREATE TRIGGER enforce_sprint_created_by_user
		BEFORE INSERT ON pm_sprints
		WHEN NEW.created_by IS NOT NULL
		 AND NOT EXISTS (SELECT 1 FROM users WHERE id = NEW.created_by)
		BEGIN
			SELECT RAISE(ABORT, 'pm_sprints_created_by_fkey');
		END
	`).Error; err != nil {
		t.Fatalf("create FK compatibility trigger: %v", err)
	}

	created := executePMSprintTestCommand(t, svc, context.Background(), model.InternalCommandContext{
		WorkspaceID:        workspaceID,
		AgentID:            "agent-command-audit",
		AgentScopeResolved: true,
		AuditActorID:       "user-command-audit",
		TargetType:         "workspace",
		TargetID:           workspaceID,
	}, "pm.create_sprint", `{"name":"Audited","start_date":"2027-05-01","end_date":"2027-05-15","team_id":"team-command-audit"}`)
	var audited model.PMSprint
	if err := db.Where("id = ?", created["sprint_id"]).First(&audited).Error; err != nil {
		t.Fatalf("load audited sprint: %v", err)
	}
	if audited.CreatedBy == nil || *audited.CreatedBy != "user-command-audit" {
		t.Fatalf("created_by = %#v, want human audit user", audited.CreatedBy)
	}

	created = executePMSprintTestCommand(t, svc, context.Background(), model.InternalCommandContext{
		WorkspaceID:        workspaceID,
		AgentID:            "agent-command-audit",
		AgentScopeResolved: true,
		TargetType:         "workspace",
		TargetID:           workspaceID,
	}, "pm.create_sprint", `{"name":"Actorless","start_date":"2027-06-01","end_date":"2027-06-15","team_id":"team-command-audit"}`)
	var actorless model.PMSprint
	if err := db.Where("id = ?", created["sprint_id"]).First(&actorless).Error; err != nil {
		t.Fatalf("load actorless sprint: %v", err)
	}
	if actorless.CreatedBy != nil {
		t.Fatalf("actorless created_by = %#v, want nil rather than agent ID", actorless.CreatedBy)
	}
}

func TestPMCommandUpdateSprintReplacementConflictAndDestinationPermission(t *testing.T) {
	svc, sprintService, db, workspaceID := newPMSprintCommandTestEnv(t)
	seedPMSprintCommandTeam(t, db, workspaceID, "team-command-current")
	seedPMSprintCommandTeam(t, db, workspaceID, "team-command-destination")
	teamID := "team-command-current"
	if err := db.Create(&model.PMLabel{ID: "label-command-existing", WorkspaceID: workspaceID, Name: "Existing"}).Error; err != nil {
		t.Fatalf("seed label: %v", err)
	}
	created, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: workspaceID,
		Name:        "Original",
		StartDate:   commandMustDate(t, "2026-10-01"),
		EndDate:     commandMustDate(t, "2026-10-15"),
		TeamID:      &teamID,
		LabelIDs:    []string{"label-command-existing"},
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	second, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: workspaceID,
		Name:        "Second",
		StartDate:   commandMustDate(t, "2026-11-01"),
		EndDate:     commandMustDate(t, "2026-11-15"),
		TeamID:      &teamID,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}

	meta := model.InternalCommandContext{WorkspaceID: workspaceID, TargetType: "sprint", TargetID: created.Sprint.ID}
	result := executePMSprintTestCommand(t, svc, context.Background(), meta, "pm.update_sprint", `{"name":"Renamed"}`)
	if result["name"] != "Renamed" {
		t.Fatalf("name not updated: %#v", result)
	}
	reloaded, err := sprintService.GetByID(context.Background(), created.Sprint.ID)
	if err != nil || len(reloaded.Labels) != 1 {
		t.Fatalf("omitted labels were not preserved: sprint=%#v err=%v", reloaded, err)
	}

	result = executePMSprintTestCommand(t, svc, context.Background(), meta, "pm.update_sprint", `{"label_ids":[]}`)
	labels, ok := result["labels"].([]any)
	if !ok || len(labels) != 0 {
		t.Fatalf("explicit empty labels did not clear: %#v", result)
	}

	_, err = svc.Execute(context.Background(), meta, "pm.update_sprint", json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "at least one editable field is required") {
		t.Fatalf("expected no-op rejection, got %v", err)
	}
	_, err = svc.Execute(context.Background(), meta, "pm.update_sprint", json.RawMessage(`{"sprint_id":"`+second.Sprint.ID+`","name":"Conflict"}`))
	if err == nil || !strings.Contains(err.Error(), "conflicts with current sprint target") {
		t.Fatalf("expected target conflict, got %v", err)
	}

	actor := &authorization.Actor{UserID: "manager-command", WorkspaceID: workspaceID, Role: "manager", TeamMemberships: []authorization.TeamRole{{TeamID: teamID, Role: "owner"}}}
	ctx := authorization.WithActor(context.Background(), actor)
	_, err = svc.Execute(ctx, meta, "pm.update_sprint", json.RawMessage(`{"team_id":"team-command-destination"}`))
	if err == nil || !strings.Contains(err.Error(), "only team managers") {
		t.Fatalf("expected destination management error, got %v", err)
	}
}

func TestPMCommandListGetAndListSprintTasks(t *testing.T) {
	svc, sprintService, db, workspaceID := newPMSprintCommandTestEnv(t)
	seedPMSprintCommandTeam(t, db, workspaceID, "team-command-list")
	teamID := "team-command-list"
	first, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: workspaceID, Name: "First", StartDate: commandMustDate(t, "2026-12-01"), EndDate: commandMustDate(t, "2026-12-15"), TeamID: &teamID,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}
	if _, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: workspaceID, Name: "Second", StartDate: commandMustDate(t, "2027-01-01"), EndDate: commandMustDate(t, "2027-01-15"), TeamID: &teamID,
	}, "actor-1"); err != nil {
		t.Fatalf("Create second: %v", err)
	}

	list := executePMSprintTestCommand(t, svc, context.Background(), model.InternalCommandContext{WorkspaceID: workspaceID, TargetType: "workspace", TargetID: workspaceID}, "pm.list_sprints", `{"page":1,"per_page":1}`)
	if list["total"] != float64(2) || list["page"] != float64(1) || len(list["sprints"].([]any)) != 1 {
		t.Fatalf("unexpected paginated list: %#v", list)
	}
	query := executePMSprintTestCommand(t, svc, context.Background(), model.InternalCommandContext{WorkspaceID: workspaceID, TargetType: "workspace", TargetID: workspaceID}, "pm.list_sprints", `{"query":"first","page":1,"per_page":10}`)
	if query["total"] != float64(1) || len(query["sprints"].([]any)) != 1 || query["sprints"].([]any)[0].(map[string]any)["name"] != "First" {
		t.Fatalf("unexpected queried sprints: %#v", query)
	}

	meta := model.InternalCommandContext{WorkspaceID: workspaceID, TargetType: "sprint", TargetID: first.Sprint.ID}
	get := executePMSprintTestCommand(t, svc, context.Background(), meta, "pm.get_sprint", `{}`)
	if get["sprint_id"] != first.Sprint.ID || get["name"] != "First" {
		t.Fatalf("target defaulting failed: %#v", get)
	}

	seedPMSprintPlanningServiceState(t, db, "state-command-list", "workflow-command-list", "Todo", model.PMStateTypeUnstarted, 0)
	seedPMSprintPlanningServiceStory(t, db, "task-command-list", workspaceID, "workflow-command-list", "state-command-list", first.Sprint.ID, teamID, "Sprint task", 9001, 1, 3)
	labelDescription := "internal label description must not leak"
	labelColor := "#778899"
	if err := db.Create(&model.PMLabel{
		ID: "label-command-task", WorkspaceID: workspaceID, TeamID: &teamID, Name: "Task label", Description: &labelDescription, Color: &labelColor,
	}).Error; err != nil {
		t.Fatalf("seed task label: %v", err)
	}
	if err := db.Exec(`INSERT INTO pm_task_labels (task_id, label_id, created_at) VALUES (?, ?, ?)`, "task-command-list", "label-command-task", time.Now().UTC()).Error; err != nil {
		t.Fatalf("link task label: %v", err)
	}
	tasks := executePMSprintTestCommand(t, svc, context.Background(), meta, "pm.list_sprint_tasks", `{}`)
	items := tasks["tasks"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["name"] != "Sprint task" || items[0].(map[string]any)["estimate"] != float64(3) {
		t.Fatalf("unexpected sprint tasks: %#v", tasks)
	}
	labels, ok := items[0].(map[string]any)["labels"].([]any)
	if !ok || len(labels) != 1 {
		t.Fatalf("unexpected compact task labels: %#v", items[0].(map[string]any)["labels"])
	}
	label, ok := labels[0].(map[string]any)
	if !ok {
		t.Fatalf("compact task label = %#v", labels[0])
	}
	if len(label) != 4 || label["id"] != "label-command-task" || label["name"] != "Task label" || label["color"] != labelColor || label["team_id"] != teamID {
		t.Fatalf("compact task label leaked or omitted fields: %#v", label)
	}
	filteredTasks := executePMSprintTestCommand(t, svc, context.Background(), meta, "pm.list_sprint_tasks", `{"query":"does-not-match"}`)
	if filteredTasks["total"] != float64(0) || len(filteredTasks["tasks"].([]any)) != 0 {
		t.Fatalf("unexpected filtered sprint tasks: %#v", filteredTasks)
	}

	seedWorkspace(t, db, "ws-command-list-other", "Other", "command-list-other", "owner-2")
	seedPMSprintCommandTeam(t, db, "ws-command-list-other", "team-command-list-other")
	otherTeamID := "team-command-list-other"
	other, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: "ws-command-list-other", Name: "Other", StartDate: commandMustDate(t, "2027-02-01"), EndDate: commandMustDate(t, "2027-02-15"), TeamID: &otherTeamID,
	}, "actor-2")
	if err != nil {
		t.Fatalf("Create other: %v", err)
	}
	_, err = svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: workspaceID, TargetType: "workspace", TargetID: workspaceID}, "pm.get_sprint", json.RawMessage(`{"sprint_id":"`+other.Sprint.ID+`"}`))
	if err == nil || !strings.Contains(err.Error(), "sprint not found in this workspace") {
		t.Fatalf("expected workspace isolation error, got %v", err)
	}
}

func TestPMCommandAgentTeamScopeReadList(t *testing.T) {
	svc, _, _, workspaceID, teamA, teamB, sprintA, sprintB := newPMSprintCommandAgentScopeEnv(t)
	_, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: workspaceID, AgentID: "agent-unresolved", TargetType: "workspace", TargetID: workspaceID,
	}, "pm.list_sprints", json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "agent team scope is unresolved") {
		t.Fatalf("unresolved agent scope error = %v", err)
	}
	teamAMeta := model.InternalCommandContext{
		WorkspaceID: workspaceID, AgentID: "agent-team-a", AgentScopeResolved: true, AgentTeamIDs: []string{teamA},
		TargetType: "workspace", TargetID: workspaceID,
	}

	list := executePMSprintTestCommand(t, svc, context.Background(), teamAMeta, "pm.list_sprints", `{}`)
	items := list["sprints"].([]any)
	if list["total"] != float64(1) || len(items) != 1 || items[0].(map[string]any)["sprint_id"] != sprintA.Sprint.ID {
		t.Fatalf("team-A agent list leaked scope: %#v", list)
	}

	for name, meta := range map[string]model.InternalCommandContext{
		"explicit get": teamAMeta,
		"target default get": {
			WorkspaceID: workspaceID, AgentID: "agent-team-a", AgentScopeResolved: true, AgentTeamIDs: []string{teamA},
			TargetType: "sprint", TargetID: sprintB.Sprint.ID,
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := `{"sprint_id":"` + sprintB.Sprint.ID + `"}`
			if name == "target default get" {
				input = `{}`
			}
			_, err := svc.Execute(context.Background(), meta, "pm.get_sprint", json.RawMessage(input))
			if err == nil || !strings.Contains(err.Error(), "agent does not have access") {
				t.Fatalf("get team-B sprint error = %v", err)
			}
		})
	}

	targetBMeta := teamAMeta
	targetBMeta.TargetType = "sprint"
	targetBMeta.TargetID = sprintB.Sprint.ID
	_, err = svc.Execute(context.Background(), targetBMeta, "pm.list_sprint_tasks", json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "agent does not have access") {
		t.Fatalf("list team-B sprint tasks error = %v", err)
	}

	workspaceMeta := model.InternalCommandContext{
		WorkspaceID: workspaceID, AgentID: "agent-workspace", AgentScopeResolved: true,
		TargetType: "workspace", TargetID: workspaceID,
	}
	list = executePMSprintTestCommand(t, svc, context.Background(), workspaceMeta, "pm.list_sprints", `{}`)
	if list["total"] != float64(2) || len(list["sprints"].([]any)) != 2 {
		t.Fatalf("workspace-scoped agent list = %#v", list)
	}
	get := executePMSprintTestCommand(t, svc, context.Background(), workspaceMeta, "pm.get_sprint", `{"sprint_id":"`+sprintB.Sprint.ID+`"}`)
	if get["sprint_id"] != sprintB.Sprint.ID || get["team_id"] != teamB {
		t.Fatalf("workspace-scoped agent could not read team B: %#v", get)
	}
}

func TestPMCommandAgentTeamScopeMutations(t *testing.T) {
	svc, _, _, workspaceID, teamA, teamB, sprintA, sprintB := newPMSprintCommandAgentScopeEnv(t)
	meta := model.InternalCommandContext{
		WorkspaceID: workspaceID, AgentID: "agent-team-a", AgentScopeResolved: true, AgentTeamIDs: []string{teamA},
		TargetType: "workspace", TargetID: workspaceID,
	}

	_, err := svc.Execute(context.Background(), meta, "pm.create_sprint", json.RawMessage(`{
		"name":"Forbidden B","start_date":"2029-03-01","end_date":"2029-03-15","team_id":"`+teamB+`"
	}`))
	if err == nil || !strings.Contains(err.Error(), "agent does not have access") {
		t.Fatalf("create in team B error = %v", err)
	}

	_, err = svc.Execute(context.Background(), meta, "pm.update_sprint", json.RawMessage(`{"sprint_id":"`+sprintB.Sprint.ID+`","name":"Forbidden rename"}`))
	if err == nil || !strings.Contains(err.Error(), "agent does not have access") {
		t.Fatalf("update team-B sprint error = %v", err)
	}

	_, err = svc.Execute(context.Background(), meta, "pm.update_sprint", json.RawMessage(`{"sprint_id":"`+sprintA.Sprint.ID+`","team_id":"`+teamB+`"}`))
	if err == nil || !strings.Contains(err.Error(), "agent does not have access") {
		t.Fatalf("move team-A sprint to B error = %v", err)
	}
}

func TestPMCommandAgentTeamScopeIntersectsHuman(t *testing.T) {
	svc, _, _, workspaceID, teamA, teamB, sprintA, sprintB := newPMSprintCommandAgentScopeEnv(t)
	meta := model.InternalCommandContext{
		WorkspaceID: workspaceID, AgentID: "agent-team-a", AgentScopeResolved: true, AgentTeamIDs: []string{teamA},
		TargetType: "workspace", TargetID: workspaceID,
	}

	humanAB := &authorization.Actor{UserID: "human-ab", WorkspaceID: workspaceID, Role: "member", TeamMemberships: []authorization.TeamRole{
		{TeamID: teamA, Role: "manager"}, {TeamID: teamB, Role: "manager"},
	}}
	list := executePMSprintTestCommand(t, svc, authorization.WithActor(context.Background(), humanAB), meta, "pm.list_sprints", `{}`)
	items := list["sprints"].([]any)
	if list["total"] != float64(1) || len(items) != 1 || items[0].(map[string]any)["sprint_id"] != sprintA.Sprint.ID {
		t.Fatalf("human AB widened team-A agent: %#v", list)
	}
	_, err := svc.Execute(authorization.WithActor(context.Background(), humanAB), meta, "pm.get_sprint", json.RawMessage(`{"sprint_id":"`+sprintB.Sprint.ID+`"}`))
	if err == nil || !strings.Contains(err.Error(), "agent does not have access") {
		t.Fatalf("human AB widened agent get scope: %v", err)
	}
	_, err = svc.Execute(authorization.WithActor(context.Background(), humanAB), meta, "pm.create_sprint", json.RawMessage(`{
		"name":"Human forbidden B","start_date":"2029-04-01","end_date":"2029-04-15","team_id":"`+teamB+`"
	}`))
	if err == nil || !strings.Contains(err.Error(), "agent does not have access") {
		t.Fatalf("human AB widened agent create scope: %v", err)
	}

	humanB := &authorization.Actor{UserID: "human-b", WorkspaceID: workspaceID, Role: "member", TeamMemberships: []authorization.TeamRole{{TeamID: teamB, Role: "manager"}}}
	list = executePMSprintTestCommand(t, svc, authorization.WithActor(context.Background(), humanB), meta, "pm.list_sprints", `{}`)
	if list["total"] != float64(0) || len(list["sprints"].([]any)) != 0 {
		t.Fatalf("human/agent team intersection = %#v, want empty", list)
	}
}

func newPMSprintCommandAgentScopeEnv(t *testing.T) (*InternalCommandService, *PMSprintService, *gorm.DB, string, string, string, *model.SprintWithStats, *model.SprintWithStats) {
	t.Helper()
	svc, sprintService, db, workspaceID := newPMSprintCommandTestEnv(t)
	teamA := "team-command-agent-a"
	teamB := "team-command-agent-b"
	seedPMSprintCommandTeam(t, db, workspaceID, teamA)
	seedPMSprintCommandTeam(t, db, workspaceID, teamB)
	sprintA, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: workspaceID, Name: "Team A sprint", StartDate: commandMustDate(t, "2029-01-01"), EndDate: commandMustDate(t, "2029-01-15"), TeamID: &teamA,
	}, "actor-1")
	if err != nil {
		t.Fatalf("create team-A sprint: %v", err)
	}
	sprintB, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: workspaceID, Name: "Team B sprint", StartDate: commandMustDate(t, "2029-02-01"), EndDate: commandMustDate(t, "2029-02-15"), TeamID: &teamB,
	}, "actor-1")
	if err != nil {
		t.Fatalf("create team-B sprint: %v", err)
	}
	seedPMSprintPlanningServiceState(t, db, "state-command-agent-scope", "workflow-command-agent-scope", "Todo", model.PMStateTypeUnstarted, 0)
	seedPMSprintPlanningServiceStory(t, db, "task-command-agent-b", workspaceID, "workflow-command-agent-scope", "state-command-agent-scope", sprintB.Sprint.ID, teamB, "Team B task", 9201, 1, 1)
	return svc, sprintService, db, workspaceID, teamA, teamB, sprintA, sprintB
}

func TestPMCommandSprintListsPageBeforeEnrichment(t *testing.T) {
	t.Run("sprints", func(t *testing.T) {
		svc, sprintService, db, workspaceID := newPMSprintCommandTestEnv(t)
		seedPMSprintCommandTeam(t, db, workspaceID, "team-command-page-sprints")
		teamID := "team-command-page-sprints"
		for index, name := range []string{"First", "Second", "Third"} {
			start := commandMustDate(t, fmt.Sprintf("2028-0%d-01", index+1))
			end := commandMustDate(t, fmt.Sprintf("2028-0%d-15", index+1))
			if _, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
				WorkspaceID: workspaceID, Name: name, StartDate: start, EndDate: end, TeamID: &teamID,
			}, "actor-1"); err != nil {
				t.Fatalf("Create sprint %s: %v", name, err)
			}
		}

		queries := capturePMSprintCommandQueries(t, db)
		result := executePMSprintTestCommand(t, svc, context.Background(), model.InternalCommandContext{
			WorkspaceID: workspaceID, TargetType: "workspace", TargetID: workspaceID,
		}, "pm.list_sprints", `{"page":1,"per_page":1}`)
		if result["total"] != float64(3) || len(result["sprints"].([]any)) != 1 {
			t.Fatalf("unexpected paginated result: %#v", result)
		}

		baseQueries := 0
		pageQueryHasLimit := false
		for _, query := range *queries {
			upper := strings.ToUpper(query)
			if strings.Contains(upper, "FROM `PM_SPRINTS`") && strings.Contains(upper, "WORKSPACE_ID") {
				baseQueries++
				if strings.Contains(upper, "LIMIT 1") {
					pageQueryHasLimit = true
				}
			}
		}
		if baseQueries > 2 || !pageQueryHasLimit {
			t.Fatalf("sprint page was not bounded before enrichment (base queries=%d, limited=%t):\n%s", baseQueries, pageQueryHasLimit, strings.Join(*queries, "\n"))
		}
	})

	t.Run("tasks", func(t *testing.T) {
		svc, sprintService, db, workspaceID := newPMSprintCommandTestEnv(t)
		seedPMSprintCommandTeam(t, db, workspaceID, "team-command-page-tasks")
		teamID := "team-command-page-tasks"
		sprint, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
			WorkspaceID: workspaceID, Name: "Task page", StartDate: commandMustDate(t, "2028-05-01"), EndDate: commandMustDate(t, "2028-05-15"), TeamID: &teamID,
		}, "actor-1")
		if err != nil {
			t.Fatalf("Create sprint: %v", err)
		}
		seedPMSprintPlanningServiceState(t, db, "state-command-page", "workflow-command-page", "Todo", model.PMStateTypeUnstarted, 0)
		for index := 1; index <= 3; index++ {
			seedPMSprintPlanningServiceStory(t, db, fmt.Sprintf("task-command-page-%d", index), workspaceID, "workflow-command-page", "state-command-page", sprint.Sprint.ID, teamID, fmt.Sprintf("Task %d", index), 9100+index, index, 1)
		}

		queries := capturePMSprintCommandQueries(t, db)
		result := executePMSprintTestCommand(t, svc, context.Background(), model.InternalCommandContext{
			WorkspaceID: workspaceID, TargetType: "sprint", TargetID: sprint.Sprint.ID,
		}, "pm.list_sprint_tasks", `{"page":1,"per_page":1}`)
		if result["total"] != float64(3) || len(result["tasks"].([]any)) != 1 {
			t.Fatalf("unexpected paginated result: %#v", result)
		}

		pageQueryHasLimit := false
		for _, query := range *queries {
			upper := strings.ToUpper(query)
			if strings.Contains(upper, "FROM `PM_TASKS`") && strings.Contains(upper, "ORDER BY POSITION") && strings.Contains(upper, "LIMIT 1") {
				pageQueryHasLimit = true
			}
		}
		if !pageQueryHasLimit {
			t.Fatalf("task page was not bounded before enrichment:\n%s", strings.Join(*queries, "\n"))
		}

		*queries = nil
		huge := executePMSprintTestCommand(t, svc, context.Background(), model.InternalCommandContext{
			WorkspaceID: workspaceID, TargetType: "sprint", TargetID: sprint.Sprint.ID,
		}, "pm.list_sprint_tasks", `{"page":9223372036854775807,"per_page":100}`)
		if huge["total"] != float64(3) || len(huge["tasks"].([]any)) != 0 {
			t.Fatalf("unexpected huge task page: %#v", huge)
		}
		for _, query := range *queries {
			upper := strings.ToUpper(query)
			if strings.Contains(upper, "FROM `PM_TASKS`") && strings.Contains(upper, "ORDER BY POSITION") {
				t.Fatalf("huge empty task page fetched tasks for enrichment: %s", query)
			}
		}
	})
}

func TestPMCommandListSprintsHugePageReturnsEmptyPage(t *testing.T) {
	svc, sprintService, db, workspaceID := newPMSprintCommandTestEnv(t)
	seedPMSprintCommandTeam(t, db, workspaceID, "team-command-huge-page")
	teamID := "team-command-huge-page"
	if _, err := sprintService.Create(context.Background(), model.CreateSprintRequest{
		WorkspaceID: workspaceID,
		Name:        "Only sprint",
		StartDate:   commandMustDate(t, "2027-03-01"),
		EndDate:     commandMustDate(t, "2027-03-15"),
		TeamID:      &teamID,
	}, "actor-1"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	queries := capturePMSprintCommandQueries(t, db)
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("huge page panicked: %v", recovered)
		}
	}()
	list := executePMSprintTestCommand(t, svc, context.Background(), model.InternalCommandContext{
		WorkspaceID: workspaceID,
		TargetType:  "workspace",
		TargetID:    workspaceID,
	}, "pm.list_sprints", `{"page":`+strconv.Itoa(int(^uint(0)>>1))+`,"per_page":100}`)
	items, ok := list["sprints"].([]any)
	if !ok || len(items) != 0 {
		t.Fatalf("huge page returned %#v, want empty sprints", list["sprints"])
	}
	for _, query := range *queries {
		upper := strings.ToUpper(query)
		if strings.Contains(upper, "PM_SPRINT_LABELS") || (strings.Contains(upper, "FROM PM_TASKS") && strings.Contains(upper, "GROUP BY")) {
			t.Fatalf("huge empty page performed enrichment query: %s", query)
		}
	}
}

func capturePMSprintCommandQueries(t *testing.T, db *gorm.DB) *[]string {
	t.Helper()
	queries := []string{}
	const callbackName = "test:capture_pm_sprint_command_queries"
	if err := db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if sql := strings.TrimSpace(tx.Statement.SQL.String()); sql != "" {
			queries = append(queries, sql)
		}
	}); err != nil {
		t.Fatalf("register query capture: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(callbackName)
	})
	return &queries
}

func seedPMSprintCommandTeam(t *testing.T, db *gorm.DB, workspaceID, teamID string) {
	t.Helper()
	now := time.Now().UTC()
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, teamID, workspaceID, teamID, now, now)
}

func commandMustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("parse test date: %v", err)
	}
	return parsed
}

func commandStringPtr(value string) *string { return &value }
