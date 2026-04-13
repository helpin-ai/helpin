package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func TestPMAutomationService_SprintCloseouts(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	ctx := context.Background()

	const (
		workspaceID    = "ws-automation-closeout"
		teamID         = "team-automation-closeout"
		workflowID     = "wf-automation-closeout"
		doneStateID    = "state-automation-done"
		startedStateID = "state-automation-started"
		endedSprintID  = "sprint-ended"
		nextSprintID   = "sprint-next"
		doneTaskID     = "task-done"
		rollTaskOneID  = "task-roll-1"
		rollTaskTwoID  = "task-roll-2"
	)

	ensurePMSprintCloseoutTables(t, db)
	seedWorkspace(t, db, workspaceID, "Automation Workspace", "automation-ws", "owner-1")
	seedSprintAutomationTeam(t, db, teamID, workspaceID, "Delivery")
	seedSprintAutomationState(t, db, doneStateID, workflowID, "Done", model.PMStateTypeDone, 0)
	seedSprintAutomationState(t, db, startedStateID, workflowID, "In Progress", model.PMStateTypeStarted, 1)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	seedSprintAutomationSprint(t, db, endedSprintID, workspaceID, teamID, "Sprint 12", today.AddDate(0, 0, -14), today.AddDate(0, 0, -1))
	seedSprintAutomationSprint(t, db, nextSprintID, workspaceID, teamID, "Sprint 13", today.AddDate(0, 0, 1), today.AddDate(0, 0, 14))
	seedSprintAutomationTask(t, db, doneTaskID, workspaceID, workflowID, doneStateID, endedSprintID, teamID, 1001, 5)
	seedSprintAutomationTask(t, db, rollTaskOneID, workspaceID, workflowID, startedStateID, endedSprintID, teamID, 1002, 3)
	seedSprintAutomationTask(t, db, rollTaskTwoID, workspaceID, workflowID, startedStateID, endedSprintID, teamID, 1003, 8)
	seedSprintMoveUnfinishedAutomation(t, db, workspaceID, teamID)

	closeoutRepo := repository.NewPMSprintCloseoutRepository(db)
	sprintRepo := repository.NewPMSprintRepository(db)
	automationService := NewPMAutomationService(
		repository.NewPMAutomationRepository(db),
		nil,
		repository.NewPMTaskRepository(db),
		sprintRepo,
		repository.NewPMWorkflowRepository(db),
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		closeoutRepo,
	)

	automationService.RunSprintMoveUnfinished(ctx)

	closeout, closeoutTasks, err := closeoutRepo.GetCloseoutBySprintID(ctx, endedSprintID)
	if err != nil {
		t.Fatalf("GetCloseoutBySprintID: %v", err)
	}
	if closeout == nil {
		t.Fatal("expected sprint closeout")
	}
	if closeout.CommittedCount != 3 || closeout.CompletedCount != 1 || closeout.UnfinishedCount != 2 || closeout.RolledOverCount != 2 {
		t.Fatalf("closeout counts = %+v", closeout)
	}
	if closeout.RolledToSprintID == nil || *closeout.RolledToSprintID != nextSprintID {
		t.Fatalf("rolled_to_sprint_id = %v, want %q", closeout.RolledToSprintID, nextSprintID)
	}
	if len(closeoutTasks) != 3 {
		t.Fatalf("closeout task rows = %d, want 3", len(closeoutTasks))
	}

	endedTasks, err := sprintRepo.ListTasks(ctx, endedSprintID)
	if err != nil {
		t.Fatalf("ListTasks ended: %v", err)
	}
	if len(endedTasks) != 1 || endedTasks[0].ID != doneTaskID {
		t.Fatalf("ended sprint tasks = %+v, want only done task", endedTasks)
	}

	nextTasks, err := sprintRepo.ListTasks(ctx, nextSprintID)
	if err != nil {
		t.Fatalf("ListTasks next: %v", err)
	}
	if len(nextTasks) != 2 {
		t.Fatalf("next sprint tasks = %d, want 2", len(nextTasks))
	}

	automationService.RunSprintMoveUnfinished(ctx)
	closeoutAgain, closeoutTasksAgain, err := closeoutRepo.GetCloseoutBySprintID(ctx, endedSprintID)
	if err != nil {
		t.Fatalf("GetCloseoutBySprintID second pass: %v", err)
	}
	if closeoutAgain == nil || closeoutAgain.ID != closeout.ID {
		t.Fatalf("closeout on rerun = %+v, want existing closeout %q", closeoutAgain, closeout.ID)
	}
	if len(closeoutTasksAgain) != 3 {
		t.Fatalf("closeout task rows on rerun = %d, want 3", len(closeoutTasksAgain))
	}
}

func ensurePMSprintCloseoutTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	statements := []string{
		`CREATE TABLE pm_sprint_closeouts (
			id TEXT PRIMARY KEY,
			sprint_id TEXT NOT NULL UNIQUE,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			rolled_to_sprint_id TEXT,
			committed_count INTEGER NOT NULL DEFAULT 0,
			completed_count INTEGER NOT NULL DEFAULT 0,
			unfinished_count INTEGER NOT NULL DEFAULT 0,
			rolled_over_count INTEGER NOT NULL DEFAULT 0,
			committed_points INTEGER NOT NULL DEFAULT 0,
			completed_points INTEGER NOT NULL DEFAULT 0,
			unfinished_points INTEGER NOT NULL DEFAULT 0,
			rolled_over_points INTEGER NOT NULL DEFAULT 0,
			closed_at DATETIME NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_sprint_closeout_tasks (
			id TEXT PRIMARY KEY,
			closeout_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			outcome TEXT NOT NULL,
			estimate INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			UNIQUE(closeout_id, task_id)
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create closeout schema: %v", err)
		}
	}
}

func seedSprintAutomationTeam(t *testing.T, db *gorm.DB, teamID, workspaceID, name string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO workspace_teams (id, workspace_id, name, sprints_enabled, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?)`,
		teamID, workspaceID, name, now, now,
	).Error; err != nil {
		t.Fatalf("seed team: %v", err)
	}
}

func seedSprintAutomationState(t *testing.T, db *gorm.DB, id, workflowID, name, stateType string, position int) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 0, ?, ?)`,
		id, workflowID, name, stateType, position, now, now,
	).Error; err != nil {
		t.Fatalf("seed state: %v", err)
	}
}

func seedSprintAutomationSprint(t *testing.T, db *gorm.DB, id, workspaceID, teamID, name string, startDate, endDate time.Time) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		id, workspaceID, name, startDate, endDate, teamID, now, now,
	).Error; err != nil {
		t.Fatalf("seed sprint: %v", err)
	}
}

func seedSprintAutomationTask(t *testing.T, db *gorm.DB, id, workspaceID, workflowID, workflowStateID, sprintID, teamID string, displayID, estimate int) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO pm_tasks (id, workspace_id, display_id, name, workflow_id, workflow_state_id, sprint_id, team_id, estimate, priority, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		id, workspaceID, displayID, id, workflowID, workflowStateID, sprintID, teamID, estimate, model.PMTaskPriorityMedium, now, now,
	).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
}

func seedSprintMoveUnfinishedAutomation(t *testing.T, db *gorm.DB, workspaceID, teamID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO pm_automations (id, workspace_id, automation_type, enabled, team_id, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?, ?)`,
		"automation-rollover", workspaceID, model.PMAutomationTypeSprintMoveUnfinished, teamID, now, now,
	).Error; err != nil {
		t.Fatalf("seed automation: %v", err)
	}
}
