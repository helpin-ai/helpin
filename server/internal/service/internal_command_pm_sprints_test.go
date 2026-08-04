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

func newPMSprintCommandTestEnv(t *testing.T) (*InternalCommandService, *PMSprintService, *gorm.DB, string) {
	t.Helper()
	sprintService, db, workspaceID := newSprintTestEnvWithDB(t)
	commandService := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	commandService.SetSettingsRepository(repository.NewSettingsRepository(db))
	commandService.SetPMSprintOperationalService(sprintService)
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

	meta := model.InternalCommandContext{WorkspaceID: workspaceID, TargetType: "sprint", TargetID: first.Sprint.ID}
	get := executePMSprintTestCommand(t, svc, context.Background(), meta, "pm.get_sprint", `{}`)
	if get["sprint_id"] != first.Sprint.ID || get["name"] != "First" {
		t.Fatalf("target defaulting failed: %#v", get)
	}

	seedPMSprintPlanningServiceState(t, db, "state-command-list", "workflow-command-list", "Todo", model.PMStateTypeUnstarted, 0)
	seedPMSprintPlanningServiceStory(t, db, "task-command-list", workspaceID, "workflow-command-list", "state-command-list", first.Sprint.ID, teamID, "Sprint task", 9001, 1, 3)
	tasks := executePMSprintTestCommand(t, svc, context.Background(), meta, "pm.list_sprint_tasks", `{}`)
	items := tasks["tasks"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["name"] != "Sprint task" || items[0].(map[string]any)["estimate"] != float64(3) {
		t.Fatalf("unexpected sprint tasks: %#v", tasks)
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
