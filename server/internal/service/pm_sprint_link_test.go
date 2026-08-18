package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

func seedSprintTaskLinkFixture(t *testing.T, db *gorm.DB, workspaceID string) {
	t.Helper()
	now := time.Now().UTC()
	for _, team := range []struct{ id, name string }{{"team-sprint-link-a", "Alpha"}, {"team-sprint-link-b", "Beta"}} {
		mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, team.id, workspaceID, team.name, now, now)
	}
	for _, workflow := range []struct{ id, teamID, stateID string }{{"wf-sprint-link-a", "team-sprint-link-a", "state-sprint-link-a"}, {"wf-sprint-link-b", "team-sprint-link-b", "state-sprint-link-b"}} {
		mustExec(t, db, `INSERT INTO pm_workflows (id, workspace_id, name, team_id, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, workflow.id, workspaceID, workflow.id, workflow.teamID, workflow.stateID, now, now)
		mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, workflow.stateID, workflow.id, "Todo", model.PMStateTypeUnstarted, 0, true, now, now)
	}
	for _, sprint := range []struct {
		id, name string
		teamID   *string
	}{{"sprint-link-target", "Target sprint", stringPtr("team-sprint-link-a")}, {"sprint-link-old", "Old sprint", stringPtr("team-sprint-link-a")}, {"sprint-link-no-team", "Legacy sprint", nil}} {
		mustExec(t, db, `INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, sprint.id, workspaceID, sprint.name, now, now.AddDate(0, 0, 14), sprint.teamID, false, now, now)
	}
	mustExec(t, db, `INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "sprint-link-completed", workspaceID, "Completed sprint", now.AddDate(0, 0, -14), now.AddDate(0, 0, -7), "team-sprint-link-a", false, now, now)
	seedPMCommandTask(t, db, "task-sprint-link-backlog", workspaceID, "team-sprint-link-a", "wf-sprint-link-a", "state-sprint-link-a", "", "", 201)
	seedPMCommandTask(t, db, "task-sprint-link-move", workspaceID, "team-sprint-link-a", "wf-sprint-link-a", "state-sprint-link-a", "", "sprint-link-old", 202)
	seedPMCommandTask(t, db, "task-sprint-link-other-team", workspaceID, "team-sprint-link-b", "wf-sprint-link-b", "state-sprint-link-b", "", "", 203)
	seedPMCommandTask(t, db, "task-sprint-link-archived", workspaceID, "team-sprint-link-a", "wf-sprint-link-a", "state-sprint-link-a", "", "", 204)
	mustExec(t, db, `UPDATE pm_tasks SET archived = TRUE WHERE id = ?`, "task-sprint-link-archived")
}

func TestPMSprintServiceLinkTasksLinksBacklogAndMovesSameTeamTasks(t *testing.T) {
	svc, db, workspaceID := newSprintTestEnvWithDB(t)
	seedSprintTaskLinkFixture(t, db, workspaceID)

	result, err := svc.LinkTasks(context.Background(), workspaceID, "sprint-link-target", []string{"task-sprint-link-backlog", "task-sprint-link-move", "task-sprint-link-backlog"}, "actor-1")
	if err != nil {
		t.Fatalf("LinkTasks: %v", err)
	}
	if result.LinkedCount != 2 || result.MovedCount != 1 {
		t.Fatalf("result = %#v, want linked=2 moved=1", result)
	}
	for _, taskID := range []string{"task-sprint-link-backlog", "task-sprint-link-move"} {
		var sprintID *string
		if err := db.Model(&model.PMTask{}).Select("sprint_id").Where("id = ?", taskID).Scan(&sprintID).Error; err != nil {
			t.Fatalf("load %s: %v", taskID, err)
		}
		if sprintID == nil || *sprintID != "sprint-link-target" {
			t.Fatalf("%s sprint_id = %v, want sprint-link-target", taskID, sprintID)
		}
	}
}

func TestPMSprintServiceLinkTasksRejectsMixedTeamsAtomically(t *testing.T) {
	svc, db, workspaceID := newSprintTestEnvWithDB(t)
	seedSprintTaskLinkFixture(t, db, workspaceID)

	_, err := svc.LinkTasks(context.Background(), workspaceID, "sprint-link-target", []string{"task-sprint-link-backlog", "task-sprint-link-other-team"}, "actor-1")
	if err == nil || !strings.Contains(err.Error(), "same team") {
		t.Fatalf("error = %v, want same-team validation", err)
	}
	var sprintID *string
	if err := db.Model(&model.PMTask{}).Select("sprint_id").Where("id = ?", "task-sprint-link-backlog").Scan(&sprintID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if sprintID != nil {
		t.Fatalf("backlog task mutated after rejected batch: %v", *sprintID)
	}
}

func TestPMSprintServiceLinkTasksRejectsSprintWithoutTeam(t *testing.T) {
	svc, db, workspaceID := newSprintTestEnvWithDB(t)
	seedSprintTaskLinkFixture(t, db, workspaceID)

	_, err := svc.LinkTasks(context.Background(), workspaceID, "sprint-link-no-team", []string{"task-sprint-link-backlog"}, "actor-1")
	if err == nil || !strings.Contains(err.Error(), "assign the sprint to a team") {
		t.Fatalf("error = %v, want team assignment guidance", err)
	}
}

func TestPMSprintServiceLinkTasksRejectsCompletedSprint(t *testing.T) {
	svc, db, workspaceID := newSprintTestEnvWithDB(t)
	seedSprintTaskLinkFixture(t, db, workspaceID)

	_, err := svc.LinkTasks(context.Background(), workspaceID, "sprint-link-completed", []string{"task-sprint-link-backlog"}, "actor-1")
	if err == nil || !strings.Contains(err.Error(), "completed sprints") {
		t.Fatalf("error = %v, want completed-sprint validation", err)
	}
}

func TestPMSprintServiceLinkTasksRejectsArchivedTask(t *testing.T) {
	svc, db, workspaceID := newSprintTestEnvWithDB(t)
	seedSprintTaskLinkFixture(t, db, workspaceID)

	_, err := svc.LinkTasks(context.Background(), workspaceID, "sprint-link-target", []string{"task-sprint-link-archived"}, "actor-1")
	if err == nil || !strings.Contains(err.Error(), "archived") {
		t.Fatalf("error = %v, want archived-task validation", err)
	}
}

func TestPMSprintServiceLinkTasksRejectsMissingTaskAtomically(t *testing.T) {
	svc, db, workspaceID := newSprintTestEnvWithDB(t)
	seedSprintTaskLinkFixture(t, db, workspaceID)

	_, err := svc.LinkTasks(context.Background(), workspaceID, "sprint-link-target", []string{"task-sprint-link-backlog", "task-sprint-link-missing"}, "actor-1")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v, want missing-task validation", err)
	}
	var sprintID *string
	if err := db.Model(&model.PMTask{}).Select("sprint_id").Where("id = ?", "task-sprint-link-backlog").Scan(&sprintID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if sprintID != nil {
		t.Fatalf("backlog task mutated after rejected batch: %v", *sprintID)
	}
}

func TestPMSprintServiceLinkTasksRejectsActorOutsideSprintTeam(t *testing.T) {
	svc, db, workspaceID := newSprintTestEnvWithDB(t)
	seedSprintTaskLinkFixture(t, db, workspaceID)
	ctx := authorization.WithActor(context.Background(), &authorization.Actor{
		UserID: "user-sprint-link", WorkspaceID: workspaceID, Role: model.RoleMember,
		TeamMemberships: []authorization.TeamRole{{TeamID: "team-sprint-link-b", Role: model.RoleMember}},
	})

	_, err := svc.LinkTasks(ctx, workspaceID, "sprint-link-target", []string{"task-sprint-link-backlog"}, "user-sprint-link")
	if err == nil {
		t.Fatal("expected team access error")
	}
}
