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

func TestPMEpicRepositoryListPageAppliesBoundsAndReturnsFilteredTotal(t *testing.T) {
	t.Parallel()
	db := newPMEpicListPageTestDB(t)
	repo := NewPMEpicRepository(db)
	ctx := context.Background()

	for i := 1; i <= 6; i++ {
		workspaceID := "ws-epic-page"
		if i == 6 {
			workspaceID = "ws-epic-page-foreign"
		}
		if err := db.Exec(`INSERT INTO pm_epics (id, workspace_id, name, position, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("epic-page-%d", i), workspaceID, fmt.Sprintf("Epic %d", i), i, false, time.Date(2026, 8, i, 0, 0, 0, 0, time.UTC), time.Now().UTC()).Error; err != nil {
			t.Fatalf("seed epic %d: %v", i, err)
		}
	}

	epics, total, err := repo.ListPage(ctx, "ws-epic-page", model.PMEpicListFilters{}, model.PMPagination{Page: 2, PerPage: 2})
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(epics) != 2 {
		t.Fatalf("page size = %d, want 2", len(epics))
	}
	if epics[0].ID != "epic-page-3" || epics[1].ID != "epic-page-4" {
		t.Fatalf("page ids = [%s %s], want [epic-page-3 epic-page-4]", epics[0].ID, epics[1].ID)
	}
}

func TestPMEpicRepositoryListPageUsesIDAsFinalTieBreaker(t *testing.T) {
	t.Parallel()
	db := newPMEpicListPageTestDB(t)
	repo := NewPMEpicRepository(db)
	tiedAt := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	for _, epicID := range []string{"epic-tie-z", "epic-tie-a"} {
		if err := db.Exec(`INSERT INTO pm_epics (id, workspace_id, name, position, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			epicID, "ws-epic-tie", epicID, 7, false, tiedAt, tiedAt).Error; err != nil {
			t.Fatalf("seed tied epic %s: %v", epicID, err)
		}
	}

	first, total, err := repo.ListPage(context.Background(), "ws-epic-tie", model.PMEpicListFilters{}, model.PMPagination{Page: 1, PerPage: 1})
	if err != nil {
		t.Fatalf("ListPage first: %v", err)
	}
	second, _, err := repo.ListPage(context.Background(), "ws-epic-tie", model.PMEpicListFilters{}, model.PMPagination{Page: 2, PerPage: 1})
	if err != nil {
		t.Fatalf("ListPage second: %v", err)
	}
	if total != 2 || len(first) != 1 || len(second) != 1 {
		t.Fatalf("unexpected pages: total=%d first=%#v second=%#v", total, first, second)
	}
	if first[0].ID != "epic-tie-a" || second[0].ID != "epic-tie-z" {
		t.Fatalf("tied page ids = [%s %s], want [epic-tie-a epic-tie-z]", first[0].ID, second[0].ID)
	}
}

func newPMEpicListPageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:pm-epic-list-page-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	statements := []string{
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, name TEXT NOT NULL,
			description TEXT, epic_state_id TEXT, owner_id TEXT, owner_member_id TEXT,
			team_id TEXT, planned_start_date DATETIME, deadline DATETIME,
			position INTEGER NOT NULL DEFAULT 0, color TEXT, archived BOOLEAN NOT NULL DEFAULT 0,
			health TEXT NOT NULL DEFAULT 'none', health_comment TEXT, progress INTEGER NOT NULL DEFAULT 0,
			completed_at DATETIME, planning_repository_id TEXT, assigned_agent_id TEXT, created_by TEXT,
			created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_labels (epic_id TEXT NOT NULL, label_id TEXT NOT NULL, created_at DATETIME, PRIMARY KEY (epic_id, label_id))`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func TestPMEpicRepositoryListTasksKeepsWorkflowStatesContiguous(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-epic-task-order"
		userID      = "user-epic-task-order"
		memberID    = "member-epic-task-order"
		epicID      = "epic-task-order"
	)

	db := newPMTaskMemberBoardTestDB(t)
	seedPMTaskMemberBoardUser(t, db, userID, "epic-order@test.com", "Epic Order User")
	seedPMTaskMemberBoardWorkspace(t, db, workspaceID, userID)
	seedPMTaskMemberBoardMember(t, db, memberID, workspaceID, userID, "Epic Order User")
	seedPMTaskMemberBoardWorkflow(t, db, "wf-a", workspaceID, "state-a-todo", "state-a-doing", "state-a-done")
	seedPMTaskMemberBoardWorkflow(t, db, "wf-b", workspaceID, "state-b-todo", "state-b-doing", "state-b-done")

	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	insertPMTaskMemberBoardTask(t, db, "a-pos-0", workspaceID, "wf-a", "state-a-todo", memberID, 68, 0, now)
	insertPMTaskMemberBoardTask(t, db, "a-pos-1", workspaceID, "wf-a", "state-a-todo", memberID, 71, 1, now.Add(time.Minute))
	insertPMTaskMemberBoardTask(t, db, "b-pos-0", workspaceID, "wf-b", "state-b-todo", memberID, 94, 0, now.Add(2*time.Minute))

	if err := db.Exec(`UPDATE pm_tasks SET epic_id = ? WHERE id IN ?`, epicID, []string{"a-pos-0", "a-pos-1", "b-pos-0"}).Error; err != nil {
		t.Fatalf("assign epic: %v", err)
	}

	tasks, err := NewPMEpicRepository(db).ListTasks(context.Background(), epicID)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	got := make([]string, len(tasks))
	for i, task := range tasks {
		got[i] = task.ID
	}
	want := []string{"a-pos-0", "a-pos-1", "b-pos-0"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tasks[%d] = %q, want %q (full order %v)", i, got[i], want[i], got)
		}
	}
}
