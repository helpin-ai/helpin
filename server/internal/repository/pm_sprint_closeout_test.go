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

func TestPMSprintCloseoutRepository(t *testing.T) {
	t.Parallel()

	db := newPMSprintCloseoutTestDB(t)
	repo := NewPMSprintCloseoutRepository(db)
	ctx := context.Background()

	const (
		workspaceID = "ws-closeout"
		teamID      = "team-closeout"
		sprintAID   = "sprint-a"
		sprintBID   = "sprint-b"
		task1ID     = "task-1"
		task2ID     = "task-2"
		task3ID     = "task-3"
	)
	teamIDValue := teamID
	sprintBIDValue := sprintBID

	seedPMSprintCloseoutWorkspace(t, db, workspaceID, "Closeout Workspace")
	seedPMSprintCloseoutTeam(t, db, teamID, workspaceID, "Growth")
	seedPMSprintCloseoutSprint(t, db, sprintAID, workspaceID, teamID, "Sprint 12", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC))
	seedPMSprintCloseoutSprint(t, db, sprintBID, workspaceID, teamID, "Sprint 13", time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC))
	seedPMSprintCloseoutTask(t, db, task1ID)
	seedPMSprintCloseoutTask(t, db, task2ID)
	seedPMSprintCloseoutTask(t, db, task3ID)

	closedAt := time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC)
	closeout := &model.PMSprintCloseout{
		SprintID:         sprintAID,
		WorkspaceID:      workspaceID,
		TeamID:           &teamIDValue,
		RolledToSprintID: &sprintBIDValue,
		CommittedCount:   3,
		CompletedCount:   1,
		UnfinishedCount:  2,
		RolledOverCount:  1,
		CommittedPoints:  13,
		CompletedPoints:  5,
		UnfinishedPoints: 8,
		RolledOverPoints: 3,
		ClosedAt:         closedAt,
	}
	rows := []model.PMSprintCloseoutTask{
		{TaskID: task1ID, Outcome: model.PMSprintCloseoutOutcomeCompleted, Estimate: 5},
		{TaskID: task2ID, Outcome: model.PMSprintCloseoutOutcomeRolledOver, Estimate: 3},
		{TaskID: task3ID, Outcome: model.PMSprintCloseoutOutcomeUnfinishedNotRolled, Estimate: 5},
	}

	created, err := repo.CreateCloseout(ctx, closeout, rows)
	if err != nil {
		t.Fatalf("CreateCloseout: %v", err)
	}
	if created == nil || created.ID == "" {
		t.Fatal("CreateCloseout returned nil or empty id")
	}

	createdAgain, err := repo.CreateCloseout(ctx, closeout, rows)
	if err != nil {
		t.Fatalf("CreateCloseout second call: %v", err)
	}
	if createdAgain == nil || createdAgain.ID != created.ID {
		t.Fatalf("idempotent create returned %+v, want existing closeout %q", createdAgain, created.ID)
	}

	got, gotRows, err := repo.GetCloseoutBySprintID(ctx, sprintAID)
	if err != nil {
		t.Fatalf("GetCloseoutBySprintID: %v", err)
	}
	if got == nil {
		t.Fatal("GetCloseoutBySprintID returned nil closeout")
	}
	if got.CommittedCount != 3 || got.CompletedCount != 1 || got.RolledOverCount != 1 {
		t.Fatalf("closeout counts = %+v", got)
	}
	if len(gotRows) != 3 {
		t.Fatalf("closeout task rows = %d, want 3", len(gotRows))
	}

	summaries, err := repo.ListCloseoutSummaries(ctx, workspaceID, nil)
	if err != nil {
		t.Fatalf("ListCloseoutSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("summary len = %d, want 1", len(summaries))
	}
	if summaries[0].SprintName != "Sprint 12" || summaries[0].TeamName == nil || *summaries[0].TeamName != "Growth" {
		t.Fatalf("summary = %+v, want sprint/team names", summaries[0])
	}

	inbound, err := repo.ListInboundRolloverSummaries(ctx, sprintBID)
	if err != nil {
		t.Fatalf("ListInboundRolloverSummaries: %v", err)
	}
	if len(inbound) != 1 {
		t.Fatalf("inbound len = %d, want 1", len(inbound))
	}
	if inbound[0].SourceSprintID != sprintAID || inbound[0].RolledOverCount != 1 {
		t.Fatalf("inbound summary = %+v", inbound[0])
	}
}

func newPMSprintCloseoutTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:pm-sprint-closeout-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE workspace_teams (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE pm_sprints (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			start_date DATETIME,
			end_date DATETIME,
			team_id TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_tasks (
			id TEXT PRIMARY KEY
		)`,
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
			t.Fatalf("create schema: %v", err)
		}
	}

	return db
}

func seedPMSprintCloseoutWorkspace(t *testing.T, db *gorm.DB, id, name string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, id, name).Error; err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
}

func seedPMSprintCloseoutTeam(t *testing.T, db *gorm.DB, id, workspaceID, name string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO workspace_teams (id, workspace_id, name) VALUES (?, ?, ?)`, id, workspaceID, name).Error; err != nil {
		t.Fatalf("seed team: %v", err)
	}
}

func seedPMSprintCloseoutSprint(t *testing.T, db *gorm.DB, id, workspaceID, teamID, name string, startDate, endDate time.Time) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO pm_sprints (id, workspace_id, name, start_date, end_date, team_id, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		id, workspaceID, name, startDate, endDate, teamID, now, now,
	).Error; err != nil {
		t.Fatalf("seed sprint: %v", err)
	}
}

func seedPMSprintCloseoutTask(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO pm_tasks (id) VALUES (?)`, id).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
}
