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

func TestPMTaskRepository_MemberBoardOrdering(t *testing.T) {
	t.Parallel()

	db := newPMTaskMemberBoardTestDB(t)
	repo := NewPMTaskRepository(db)
	ctx := context.Background()

	const (
		workspaceID  = "ws-member-board"
		workflowID   = "wf-member-board"
		todoStateID  = "state-todo"
		doingStateID = "state-doing"
		doneStateID  = "state-done"
		userID       = "user-member-board"
		memberID     = "member-member-board"
	)

	seedPMTaskMemberBoardUser(t, db, userID, "member-board@test.com", "Member Board User")
	seedPMTaskMemberBoardWorkspace(t, db, workspaceID, userID)
	seedPMTaskMemberBoardMember(t, db, memberID, workspaceID, userID, "Member Board User")
	seedPMTaskMemberBoardWorkflow(t, db, workflowID, workspaceID, todoStateID, doingStateID, doneStateID)

	now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	insertPMTaskMemberBoardTask(t, db, "story-started", workspaceID, workflowID, doingStateID, memberID, 1, 0, now.Add(3*time.Minute))
	insertPMTaskMemberBoardTask(t, db, "story-todo-1", workspaceID, workflowID, todoStateID, memberID, 2, 1, now.Add(1*time.Minute))
	insertPMTaskMemberBoardTask(t, db, "story-todo-2", workspaceID, workflowID, todoStateID, memberID, 3, 5, now.Add(2*time.Minute))

	t.Run("ListByMember orders by workflow state then story position", func(t *testing.T) {
		columns, err := repo.ListByMember(ctx, workspaceID, workflowID, model.PMTaskFilters{}, 10, false, nil)
		if err != nil {
			t.Fatalf("ListByMember: %v", err)
		}
		if len(columns) != 1 {
			t.Fatalf("columns = %d, want 1", len(columns))
		}

		got := []string{
			columns[0].Stories[0].ID,
			columns[0].Stories[1].ID,
			columns[0].Stories[2].ID,
		}
		want := []string{"story-todo-1", "story-todo-2", "story-started"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("stories[%d] = %q, want %q (got full order %v)", i, got[i], want[i], got)
			}
		}
	})

	t.Run("ListMemberColumnStories uses the same deterministic ordering", func(t *testing.T) {
		stories, total, err := repo.ListMemberColumnStories(ctx, workspaceID, workflowID, testStringPtr(memberID), model.PMTaskFilters{}, 0, 10)
		if err != nil {
			t.Fatalf("ListMemberColumnStories: %v", err)
		}
		if total != 3 {
			t.Fatalf("total = %d, want 3", total)
		}

		got := []string{stories[0].ID, stories[1].ID, stories[2].ID}
		want := []string{"story-todo-1", "story-todo-2", "story-started"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("stories[%d] = %q, want %q (got full order %v)", i, got[i], want[i], got)
			}
		}
	})
}

func newPMTaskMemberBoardTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:pm-story-member-board-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			full_name TEXT NOT NULL,
			avatar_url TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			owner_id TEXT NOT NULL,
			timezone TEXT NOT NULL DEFAULT 'UTC',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_members (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			user_id TEXT,
			email TEXT NOT NULL,
			display_name TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_workflows (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			default_state_id TEXT,
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
			is_default BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_stories (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			display_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			story_type TEXT NOT NULL DEFAULT 'feature',
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
		`CREATE TABLE pm_story_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			source_story_id TEXT NOT NULL,
			target_story_id TEXT NOT NULL,
			link_type TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_labels (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			color TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_story_labels (
			story_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (story_id, label_id)
		)`,
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_sprints (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
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

func seedPMTaskMemberBoardUser(t *testing.T, db *gorm.DB, id, email, fullName string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, email, "hash", fullName, now, now,
	).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func seedPMTaskMemberBoardWorkspace(t *testing.T, db *gorm.DB, workspaceID, ownerID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO workspaces (id, name, slug, owner_id, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		workspaceID, "Board Workspace", "board-workspace", ownerID, "UTC", now, now,
	).Error; err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
}

func seedPMTaskMemberBoardMember(t *testing.T, db *gorm.DB, memberID, workspaceID, userID, displayName string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO workspace_members (id, workspace_id, user_id, email, display_name, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		memberID, workspaceID, userID, "member-board@test.com", displayName, "member", "active", now, now,
	).Error; err != nil {
		t.Fatalf("seed workspace member: %v", err)
	}
}

func seedPMTaskMemberBoardWorkflow(t *testing.T, db *gorm.DB, workflowID, workspaceID, todoStateID, doingStateID, doneStateID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO pm_workflows (id, workspace_id, name, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		workflowID, workspaceID, "Board Workflow", todoStateID, now, now,
	).Error; err != nil {
		t.Fatalf("seed workflow: %v", err)
	}

	states := []struct {
		id        string
		name      string
		stateType string
		position  int
		isDefault bool
	}{
		{todoStateID, "To Do", "unstarted", 0, true},
		{doingStateID, "In Progress", "started", 1, false},
		{doneStateID, "Done", "done", 2, false},
	}
	for _, state := range states {
		if err := db.Exec(
			`INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			state.id, workflowID, state.name, state.stateType, state.position, state.isDefault, now, now,
		).Error; err != nil {
			t.Fatalf("seed workflow state: %v", err)
		}
	}
}

func insertPMTaskMemberBoardTask(t *testing.T, db *gorm.DB, storyID, workspaceID, workflowID, stateID, ownerMemberID string, displayID, position int, updatedAt time.Time) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO pm_stories (
			id, workspace_id, display_id, name, workflow_id, workflow_state_id, owner_member_id,
			position, priority, severity, story_type, started, completed, blocked, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		storyID, workspaceID, displayID, storyID, workflowID, stateID, ownerMemberID,
		position, "none", "none", "feature", false, false, false, false, updatedAt, updatedAt,
	).Error; err != nil {
		t.Fatalf("seed story %s: %v", storyID, err)
	}
}

func testStringPtr(value string) *string {
	return &value
}
