package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPMSprintPlanningRepository(t *testing.T) {
	t.Parallel()

	db := newPMSprintPlanningTestDB(t)
	repo := NewPMSprintRepository(db)
	ctx := context.Background()

	const (
		workspaceID  = "ws-sprint-planning"
		teamAID      = "team-a"
		teamBID      = "team-b"
		workflowID   = "wf-planning"
		todoStateID  = "state-todo"
		doingStateID = "state-doing"
		doneStateID  = "state-done"
	)
	teamA := teamAID

	now := time.Now().UTC().Truncate(time.Minute)
	seedPMSprintPlanningState(t, db, todoStateID, workflowID, "Todo", model.PMStateTypeUnstarted, 0)
	seedPMSprintPlanningState(t, db, doingStateID, workflowID, "Doing", model.PMStateTypeStarted, 1)
	seedPMSprintPlanningState(t, db, doneStateID, workflowID, "Done", model.PMStateTypeDone, 2)

	activeSprintID := "sprint-active"
	upcomingSprintNearID := "sprint-upcoming-near"
	upcomingSprintFarID := "sprint-upcoming-far"
	completedSprintRecentID := "sprint-completed-recent"
	completedSprintOlderID := "sprint-completed-older"

	seedPMSprintPlanningSprint(t, db, activeSprintID, workspaceID, teamAID, "Active sprint", now.AddDate(0, 0, -2), now.AddDate(0, 0, 5), false)
	seedPMSprintPlanningSprint(t, db, upcomingSprintNearID, workspaceID, teamAID, "Upcoming near sprint", now.AddDate(0, 0, 8), now.AddDate(0, 0, 15), false)
	seedPMSprintPlanningSprint(t, db, upcomingSprintFarID, workspaceID, teamAID, "Upcoming far sprint", now.AddDate(0, 0, 22), now.AddDate(0, 0, 29), false)
	seedPMSprintPlanningSprint(t, db, completedSprintRecentID, workspaceID, teamAID, "Completed recent sprint", now.AddDate(0, 0, -12), now.AddDate(0, 0, -5), false)
	seedPMSprintPlanningSprint(t, db, completedSprintOlderID, workspaceID, teamAID, "Completed older sprint", now.AddDate(0, 0, -24), now.AddDate(0, 0, -17), false)
	seedPMSprintPlanningSprint(t, db, "sprint-other-team", workspaceID, teamBID, "Other team sprint", now.AddDate(0, 0, -1), now.AddDate(0, 0, 3), false)
	seedPMSprintPlanningSprint(t, db, "sprint-archived", workspaceID, teamAID, "Archived sprint", now.AddDate(0, 0, -20), now.AddDate(0, 0, -10), true)

	seedPMSprintPlanningTask(t, db, "story-active-1", workspaceID, workflowID, todoStateID, activeSprintID, teamAID, "Active todo", 1001, 1, 3, now.Add(-5*time.Minute))
	seedPMSprintPlanningTask(t, db, "story-active-2", workspaceID, workflowID, doingStateID, activeSprintID, teamAID, "Active doing", 1002, 2, 5, now.Add(-4*time.Minute))
	seedPMSprintPlanningTask(t, db, "story-active-3", workspaceID, workflowID, doneStateID, activeSprintID, teamAID, "Active done", 1003, 3, 1, now.Add(-3*time.Minute))
	seedPMSprintPlanningTask(t, db, "story-upcoming-1", workspaceID, workflowID, todoStateID, upcomingSprintFarID, teamAID, "Upcoming todo", 1004, 1, 2, now.Add(-2*time.Minute))
	seedPMSprintPlanningTask(t, db, "story-completed-1", workspaceID, workflowID, doneStateID, completedSprintRecentID, teamAID, "Completed done", 1005, 1, 8, now.Add(-1*time.Minute))
	seedPMSprintPlanningTask(t, db, "story-backlog-1", workspaceID, workflowID, todoStateID, "", teamAID, "Backlog one", 1006, 10, 2, now.Add(-6*time.Minute))
	seedPMSprintPlanningTask(t, db, "story-backlog-2", workspaceID, workflowID, doingStateID, "", teamAID, "Backlog two", 1007, 11, 5, now.Add(-7*time.Minute))
	seedPMSprintPlanningTask(t, db, "story-backlog-done", workspaceID, workflowID, doneStateID, "", teamAID, "Backlog done", 1008, 12, 1, now.Add(-8*time.Minute))
	seedPMSprintPlanningTask(t, db, "story-other-team", workspaceID, workflowID, todoStateID, "", teamBID, "Other team backlog", 1009, 13, 3, now.Add(-9*time.Minute))

	workspace, err := repo.ListPlanningWorkspace(ctx, workspaceID, model.PMSprintPlanningFilters{
		TeamID:           &teamA,
		IncludeCompleted: true,
		PreviewTaskLimit: 2,
		BacklogLimit:     10,
	})
	if err != nil {
		t.Fatalf("ListPlanningWorkspace: %v", err)
	}

	if len(workspace.Buckets) != 3 {
		t.Fatalf("buckets = %d, want 3", len(workspace.Buckets))
	}
	if workspace.Buckets[0].Key != "active" {
		t.Fatalf("bucket[0].key = %q, want active", workspace.Buckets[0].Key)
	}
	if workspace.Buckets[1].Key != "upcoming" {
		t.Fatalf("bucket[1].key = %q, want upcoming", workspace.Buckets[1].Key)
	}
	if workspace.Buckets[2].Key != "completed" {
		t.Fatalf("bucket[2].key = %q, want completed", workspace.Buckets[2].Key)
	}

	if len(workspace.Buckets[0].Sprints) != 1 || workspace.Buckets[0].Sprints[0].Sprint.ID != activeSprintID {
		t.Fatalf("active bucket = %+v, want active sprint", workspace.Buckets[0].Sprints)
	}
	if len(workspace.Buckets[1].Sprints) != 2 {
		t.Fatalf("upcoming bucket len = %d, want 2", len(workspace.Buckets[1].Sprints))
	}
	if workspace.Buckets[1].Sprints[0].Sprint.ID != upcomingSprintFarID || workspace.Buckets[1].Sprints[1].Sprint.ID != upcomingSprintNearID {
		t.Fatalf("upcoming bucket order = %+v, want far future then near future", workspace.Buckets[1].Sprints)
	}
	if len(workspace.Buckets[2].Sprints) != 2 {
		t.Fatalf("completed bucket len = %d, want 2", len(workspace.Buckets[2].Sprints))
	}
	if workspace.Buckets[2].Sprints[0].Sprint.ID != completedSprintRecentID || workspace.Buckets[2].Sprints[1].Sprint.ID != completedSprintOlderID {
		t.Fatalf("completed bucket order = %+v, want recent completion then older completion", workspace.Buckets[2].Sprints)
	}

	activeCard := workspace.Buckets[0].Sprints[0]
	if activeCard.Stats.TaskCount != 3 {
		t.Fatalf("active task_count = %d, want 3", activeCard.Stats.TaskCount)
	}
	if len(activeCard.PreviewTasks) != 2 {
		t.Fatalf("active preview tasks = %d, want 2", len(activeCard.PreviewTasks))
	}
	if activeCard.TaskPreviewOverflow != 1 {
		t.Fatalf("active preview overflow = %d, want 1", activeCard.TaskPreviewOverflow)
	}
	if activeCard.PreviewTasks[0].ID != "story-active-1" || activeCard.PreviewTasks[1].ID != "story-active-2" {
		t.Fatalf("active preview order = %#v, want story-active-1 then story-active-2", activeCard.PreviewTasks)
	}

	if workspace.BacklogTotal != 2 {
		t.Fatalf("backlog_total = %d, want 2", workspace.BacklogTotal)
	}
	if len(workspace.BacklogTasks) != 2 {
		t.Fatalf("backlog tasks = %d, want 2", len(workspace.BacklogTasks))
	}
	if workspace.BacklogTasks[0].ID != "story-backlog-1" || workspace.BacklogTasks[1].ID != "story-backlog-2" {
		t.Fatalf("backlog order = %#v, want backlog one then backlog two", workspace.BacklogTasks)
	}
}

func newPMSprintPlanningTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:pm-sprint-planning-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE pm_sprints (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			start_date DATETIME,
			end_date DATETIME,
			team_id TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			task_type TEXT NOT NULL DEFAULT 'feature',
			workflow_id TEXT NOT NULL,
			workflow_state_id TEXT NOT NULL,
			epic_id TEXT,
			sprint_id TEXT,
			team_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			requester_id TEXT,
			requester_member_id TEXT,
			estimate INTEGER,
			priority TEXT NOT NULL DEFAULT 'none',
			severity TEXT NOT NULL DEFAULT 'none',
			deadline DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			moved_at DATETIME,
			blocked BOOLEAN NOT NULL DEFAULT 0,
			blocker TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			assigned_agent_id TEXT,
			plan_document_id TEXT,
			template_id TEXT,
			recurring_template_id TEXT,
			recurring_run_id TEXT,
			recurring_occurrence_number INTEGER,
			external_id TEXT,
			slice_type TEXT,
			implementation_brief TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_workflow_states (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			name TEXT NOT NULL,
			state_type TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			description TEXT,
			wip_limit INTEGER,
			is_default BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}

	return db
}

func seedPMSprintPlanningSprint(t *testing.T, db *gorm.DB, id, workspaceID, teamID, name string, startDate, endDate time.Time, archived bool) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, workspaceID, name, startDate, endDate, teamID, archived, startDate, endDate,
	).Error; err != nil {
		t.Fatalf("seed sprint: %v", err)
	}
}

func seedPMSprintPlanningTask(t *testing.T, db *gorm.DB, id, workspaceID, workflowID, stateID, sprintID, teamID, name string, displayID, position, estimate int, updatedAt time.Time) {
	t.Helper()
	var sprint any
	if sprintID != "" {
		sprint = sprintID
	}
	if err := db.Exec(
		`INSERT INTO pm_tasks (id, workspace_id, display_id, name, workflow_id, workflow_state_id, sprint_id, team_id, estimate, position, priority, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		id, workspaceID, displayID, name, workflowID, stateID, sprint, teamID, estimate, position, model.PMTaskPriorityMedium, updatedAt, updatedAt,
	).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
}

func seedPMSprintPlanningState(t *testing.T, db *gorm.DB, id, workflowID, name, stateType string, position int) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 0, ?, ?)`,
		id, workflowID, name, stateType, position, now, now,
	).Error; err != nil {
		t.Fatalf("seed state: %v", err)
	}
}
