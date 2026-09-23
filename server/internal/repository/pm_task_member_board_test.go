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

	t.Run("ListByMember orders by workflow state then task position", func(t *testing.T) {
		columns, err := repo.ListByMember(ctx, workspaceID, workflowID, model.PMTaskFilters{}, 10, false, nil)
		if err != nil {
			t.Fatalf("ListByMember: %v", err)
		}
		if len(columns) != 1 {
			t.Fatalf("columns = %d, want 1", len(columns))
		}

		got := []string{
			columns[0].Tasks[0].ID,
			columns[0].Tasks[1].ID,
			columns[0].Tasks[2].ID,
		}
		want := []string{"story-todo-1", "story-todo-2", "story-started"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("tasks[%d] = %q, want %q (got full order %v)", i, got[i], want[i], got)
			}
		}
	})

	t.Run("ListMemberColumnTasks uses the same deterministic ordering", func(t *testing.T) {
		tasks, total, err := repo.ListMemberColumnTasks(ctx, workspaceID, workflowID, testStringPtr(memberID), model.PMTaskFilters{}, 0, 10)
		if err != nil {
			t.Fatalf("ListMemberColumnTasks: %v", err)
		}
		if total != 3 {
			t.Fatalf("total = %d, want 3", total)
		}

		got := []string{tasks[0].ID, tasks[1].ID, tasks[2].ID}
		want := []string{"story-todo-1", "story-todo-2", "story-started"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("tasks[%d] = %q, want %q (got full order %v)", i, got[i], want[i], got)
			}
		}
	})

	t.Run("List filters tasks across workflows by canonical state type", func(t *testing.T) {
		stateType := model.PMStateTypeStarted
		tasks, total, err := repo.List(
			ctx,
			workspaceID,
			model.PMTaskFilters{StateType: &stateType},
			model.PMPagination{Page: 1, PerPage: 20},
		)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if total != 1 || len(tasks) != 1 || tasks[0].ID != "story-started" {
			t.Fatalf("tasks = %#v, total = %d; want only story-started", taskIDsFromBoardTasks(tasks), total)
		}
	})
}

func TestPMTaskRepository_TaskSummaryCollectionsOmitRichFields(t *testing.T) {
	t.Parallel()

	db := newPMTaskMemberBoardTestDB(t)
	repo := NewPMTaskRepository(db)
	ctx := context.Background()

	const (
		workspaceID = "ws-task-summary"
		workflowID  = "wf-task-summary"
		todoStateID = "state-task-summary-todo"
		userID      = "user-task-summary"
		memberID    = "member-task-summary"
		taskID      = "task-summary-rich"
	)

	seedPMTaskMemberBoardUser(t, db, userID, "task-summary@test.com", "Task Summary User")
	seedPMTaskMemberBoardWorkspace(t, db, workspaceID, userID)
	seedPMTaskMemberBoardMember(t, db, memberID, workspaceID, userID, "Task Summary User")
	seedPMTaskMemberBoardWorkflow(t, db, workflowID, workspaceID, todoStateID, "state-task-summary-doing", "state-task-summary-done")
	insertPMTaskMemberBoardTask(t, db, taskID, workspaceID, workflowID, todoStateID, memberID, 1, 0, time.Now().UTC())

	const description = "large rich-text description"
	const implementationBrief = `{"summary":"implementation details"}`
	if err := db.Model(&model.PMTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"description":          description,
			"implementation_brief": []byte(implementationBrief),
		}).Error; err != nil {
		t.Fatalf("seed task rich fields: %v", err)
	}

	assertSummary := func(t *testing.T, task model.BoardTask) {
		t.Helper()
		if task.ID != taskID {
			t.Fatalf("task id = %q, want %q", task.ID, taskID)
		}
		if task.Description != nil {
			t.Fatalf("summary description = %q, want nil", *task.Description)
		}
		if len(task.ImplementationBrief) != 0 {
			t.Fatalf("summary implementation_brief = %s, want empty", task.ImplementationBrief)
		}
	}

	t.Run("list", func(t *testing.T) {
		tasks, total, err := repo.ListSummary(ctx, workspaceID, model.PMTaskFilters{}, model.PMPagination{Page: 1, PerPage: 10})
		if err != nil {
			t.Fatalf("ListSummary: %v", err)
		}
		if total != 1 || len(tasks) != 1 {
			t.Fatalf("total/tasks = %d/%d, want 1/1", total, len(tasks))
		}
		assertSummary(t, tasks[0])
	})

	t.Run("workflow state board", func(t *testing.T) {
		columns, err := repo.ListByWorkflowState(ctx, workflowID, model.PMTaskFilters{}, 10)
		if err != nil {
			t.Fatalf("ListByWorkflowState: %v", err)
		}
		if len(columns) == 0 || len(columns[0].Tasks) != 1 {
			t.Fatalf("first column task count = %d, want 1", len(columns[0].Tasks))
		}
		assertSummary(t, columns[0].Tasks[0])
	})

	t.Run("workflow state column", func(t *testing.T) {
		tasks, _, total, err := repo.ListColumnTasks(ctx, todoStateID, model.PMTaskFilters{}, 0, 10)
		if err != nil {
			t.Fatalf("ListColumnTasks: %v", err)
		}
		if total != 1 || len(tasks) != 1 {
			t.Fatalf("total/tasks = %d/%d, want 1/1", total, len(tasks))
		}
		assertSummary(t, tasks[0])
	})

	t.Run("member board", func(t *testing.T) {
		columns, err := repo.ListByMember(ctx, workspaceID, workflowID, model.PMTaskFilters{}, 10, false, nil)
		if err != nil {
			t.Fatalf("ListByMember: %v", err)
		}
		if len(columns) != 1 || len(columns[0].Tasks) != 1 {
			t.Fatalf("columns/tasks = %d/%d, want 1/1", len(columns), len(columns[0].Tasks))
		}
		assertSummary(t, columns[0].Tasks[0])
	})

	t.Run("member column", func(t *testing.T) {
		tasks, total, err := repo.ListMemberColumnTasks(ctx, workspaceID, workflowID, testStringPtr(memberID), model.PMTaskFilters{}, 0, 10)
		if err != nil {
			t.Fatalf("ListMemberColumnTasks: %v", err)
		}
		if total != 1 || len(tasks) != 1 {
			t.Fatalf("total/tasks = %d/%d, want 1/1", total, len(tasks))
		}
		assertSummary(t, tasks[0])
	})

	t.Run("task detail retains rich fields", func(t *testing.T) {
		detail, err := repo.GetByID(ctx, taskID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if detail == nil || detail.Task.Description == nil || *detail.Task.Description != description {
			t.Fatalf("detail description = %#v, want %q", detail, description)
		}
		if string(detail.Task.ImplementationBrief) != implementationBrief {
			t.Fatalf("detail implementation_brief = %s, want %s", detail.Task.ImplementationBrief, implementationBrief)
		}
	})
}

func TestPMTaskRepository_ListFiltersByOwnerMemberIDsFromJoinTable(t *testing.T) {
	t.Parallel()

	db := newPMTaskMemberBoardTestDB(t)
	repo := NewPMTaskRepository(db)
	ctx := context.Background()

	const (
		workspaceID = "ws-owner-filter"
		workflowID  = "wf-owner-filter"
		todoStateID = "state-owner-filter-todo"
	)

	seedPMTaskMemberBoardUser(t, db, "user-alice", "alice-owner-filter@test.com", "Alice Owner")
	seedPMTaskMemberBoardUser(t, db, "user-bob", "bob-owner-filter@test.com", "Bob Owner")
	seedPMTaskMemberBoardUser(t, db, "user-charlie", "charlie-owner-filter@test.com", "Charlie Owner")
	seedPMTaskMemberBoardWorkspace(t, db, workspaceID, "user-alice")
	seedPMTaskMemberBoardMember(t, db, "member-alice", workspaceID, "user-alice", "Alice Owner")
	seedPMTaskMemberBoardMember(t, db, "member-bob", workspaceID, "user-bob", "Bob Owner")
	seedPMTaskMemberBoardMember(t, db, "member-charlie", workspaceID, "user-charlie", "Charlie Owner")
	seedPMTaskMemberBoardWorkflow(t, db, workflowID, workspaceID, todoStateID, "state-owner-filter-doing", "state-owner-filter-done")

	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	insertPMTaskMemberBoardTask(t, db, "task-owned", workspaceID, workflowID, todoStateID, "", 1, 0, now)
	insertPMTaskMemberBoardTask(t, db, "task-unassigned", workspaceID, workflowID, todoStateID, "", 2, 1, now.Add(time.Minute))
	seedPMTaskOwner(t, db, "task-owned", "user-alice", now)
	seedPMTaskOwner(t, db, "task-owned", "user-bob", now.Add(time.Second))

	t.Run("single owner member matches any task where that member is an owner", func(t *testing.T) {
		tasks, _, err := repo.List(ctx, workspaceID, model.PMTaskFilters{OwnerMemberIDs: []string{"member-alice"}}, model.PMPagination{Page: 1, PerPage: 20})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if got := taskIDsFromBoardTasks(tasks); len(got) != 1 || got[0] != "task-owned" {
			t.Fatalf("task ids = %v, want [task-owned]", got)
		}
	})

	t.Run("multiple owner members use set overlap", func(t *testing.T) {
		tasks, _, err := repo.List(ctx, workspaceID, model.PMTaskFilters{OwnerMemberIDs: []string{"member-bob", "member-charlie"}}, model.PMPagination{Page: 1, PerPage: 20})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if got := taskIDsFromBoardTasks(tasks); len(got) != 1 || got[0] != "task-owned" {
			t.Fatalf("task ids = %v, want [task-owned]", got)
		}
	})

	t.Run("empty owner member filter does not filter", func(t *testing.T) {
		tasks, _, err := repo.List(ctx, workspaceID, model.PMTaskFilters{}, model.PMPagination{Page: 1, PerPage: 20})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got := taskIDsFromBoardTasks(tasks)
		want := []string{"task-owned", "task-unassigned"}
		if len(got) != len(want) {
			t.Fatalf("task ids = %v, want %v", got, want)
		}
		for _, id := range want {
			if !containsString(got, id) {
				t.Fatalf("task ids = %v, want to contain %q", got, id)
			}
		}
	})
}

func TestPMTaskRepository_MemberBoardUsesTaskOwnersJoinTable(t *testing.T) {
	t.Parallel()

	db := newPMTaskMemberBoardTestDB(t)
	repo := NewPMTaskRepository(db)
	ctx := context.Background()

	const (
		workspaceID = "ws-member-board-owners"
		workflowID  = "wf-member-board-owners"
		todoStateID = "state-member-board-owners-todo"
	)

	seedPMTaskMemberBoardUser(t, db, "user-owner-a", "owner-a@test.com", "Owner A")
	seedPMTaskMemberBoardUser(t, db, "user-owner-b", "owner-b@test.com", "Owner B")
	seedPMTaskMemberBoardWorkspace(t, db, workspaceID, "user-owner-a")
	seedPMTaskMemberBoardMember(t, db, "member-owner-a", workspaceID, "user-owner-a", "Owner A")
	seedPMTaskMemberBoardMember(t, db, "member-owner-b", workspaceID, "user-owner-b", "Owner B")
	seedPMTaskMemberBoardWorkflow(t, db, workflowID, workspaceID, todoStateID, "state-member-board-owners-doing", "state-member-board-owners-done")

	now := time.Date(2026, 5, 5, 13, 0, 0, 0, time.UTC)
	insertPMTaskMemberBoardTask(t, db, "task-two-owners", workspaceID, workflowID, todoStateID, "", 1, 0, now)
	insertPMTaskMemberBoardTask(t, db, "task-no-owners", workspaceID, workflowID, todoStateID, "", 2, 1, now.Add(time.Minute))
	seedPMTaskOwner(t, db, "task-two-owners", "user-owner-a", now)
	seedPMTaskOwner(t, db, "task-two-owners", "user-owner-b", now.Add(time.Second))

	columns, err := repo.ListByMember(ctx, workspaceID, workflowID, model.PMTaskFilters{}, 10, false, nil)
	if err != nil {
		t.Fatalf("ListByMember: %v", err)
	}

	tasksByColumn := map[string][]string{}
	for _, column := range columns {
		key := "__unassigned__"
		if column.Member != nil {
			key = column.Member.ID
		}
		tasksByColumn[key] = taskIDsFromBoardTasks(column.Tasks)
	}

	if got := tasksByColumn["member-owner-a"]; len(got) != 1 || got[0] != "task-two-owners" {
		t.Fatalf("member-owner-a tasks = %v, want [task-two-owners]", got)
	}
	if got := tasksByColumn["member-owner-b"]; len(got) != 1 || got[0] != "task-two-owners" {
		t.Fatalf("member-owner-b tasks = %v, want [task-two-owners]", got)
	}
	if got := tasksByColumn["__unassigned__"]; len(got) != 1 || got[0] != "task-no-owners" {
		t.Fatalf("unassigned tasks = %v, want [task-no-owners]", got)
	}

	memberTasks, total, err := repo.ListMemberColumnTasks(ctx, workspaceID, workflowID, testStringPtr("member-owner-b"), model.PMTaskFilters{}, 0, 10)
	if err != nil {
		t.Fatalf("ListMemberColumnTasks member: %v", err)
	}
	if got := taskIDsFromBoardTasks(memberTasks); total != 1 || len(got) != 1 || got[0] != "task-two-owners" {
		t.Fatalf("member column total/tasks = %d/%v, want 1/[task-two-owners]", total, got)
	}

	unassignedTasks, total, err := repo.ListMemberColumnTasks(ctx, workspaceID, workflowID, nil, model.PMTaskFilters{}, 0, 10)
	if err != nil {
		t.Fatalf("ListMemberColumnTasks unassigned: %v", err)
	}
	if got := taskIDsFromBoardTasks(unassignedTasks); total != 1 || len(got) != 1 || got[0] != "task-no-owners" {
		t.Fatalf("unassigned column total/tasks = %d/%v, want 1/[task-no-owners]", total, got)
	}
}

func TestPMTaskRepository_GetByIDHydratesOwnerMemberIDsFromJoinTable(t *testing.T) {
	t.Parallel()

	db := newPMTaskMemberBoardTestDB(t)
	repo := NewPMTaskRepository(db)
	ctx := context.Background()

	const (
		workspaceID = "ws-task-detail-owners"
		workflowID  = "wf-task-detail-owners"
		todoStateID = "state-task-detail-owners-todo"
	)

	seedPMTaskMemberBoardUser(t, db, "user-detail-a", "detail-a@test.com", "Detail A")
	seedPMTaskMemberBoardUser(t, db, "user-detail-b", "detail-b@test.com", "Detail B")
	seedPMTaskMemberBoardWorkspace(t, db, workspaceID, "user-detail-a")
	seedPMTaskMemberBoardMember(t, db, "member-detail-a", workspaceID, "user-detail-a", "Detail A")
	seedPMTaskMemberBoardMember(t, db, "member-detail-b", workspaceID, "user-detail-b", "Detail B")
	seedPMTaskMemberBoardWorkflow(t, db, workflowID, workspaceID, todoStateID, "state-task-detail-owners-doing", "state-task-detail-owners-done")

	now := time.Date(2026, 5, 5, 14, 0, 0, 0, time.UTC)
	insertPMTaskMemberBoardTask(t, db, "task-detail-owners", workspaceID, workflowID, todoStateID, "", 1, 0, now)
	seedPMTaskOwner(t, db, "task-detail-owners", "user-detail-a", now)
	seedPMTaskOwner(t, db, "task-detail-owners", "user-detail-b", now.Add(time.Second))

	detail, err := repo.GetByID(ctx, "task-detail-owners")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if detail == nil {
		t.Fatal("GetByID returned nil detail")
	}
	want := []string{"member-detail-a", "member-detail-b"}
	got := detail.Task.OwnerMemberIDs
	if len(got) != len(want) {
		t.Fatalf("owner_member_ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("owner_member_ids = %v, want %v", got, want)
		}
	}
}

func newPMTaskMemberBoardTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:pm-task-member-board-%d?mode=memory&cache=shared", time.Now().UnixNano())
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
			avatar_style TEXT,
			avatar_seed TEXT,
			avatar_background_mode TEXT,
			avatar_background_color TEXT,
			is_platform_admin BOOLEAN NOT NULL DEFAULT 0,
			is_server_admin BOOLEAN NOT NULL DEFAULT 0,
			signup_verification_pending BOOLEAN NOT NULL DEFAULT 0,
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
		`CREATE TABLE pm_task_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			source_task_id TEXT NOT NULL,
			target_task_id TEXT NOT NULL,
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
		`CREATE TABLE pm_task_labels (
			task_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (task_id, label_id)
		)`,
		`CREATE TABLE pm_task_owners (
			task_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (task_id, user_id)
		)`,
		`CREATE TABLE pm_task_followers (
			task_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (task_id, user_id)
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

func insertPMTaskMemberBoardTask(t *testing.T, db *gorm.DB, taskID, workspaceID, workflowID, stateID, ownerMemberID string, displayID, position int, updatedAt time.Time) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO pm_tasks (
			id, workspace_id, display_id, name, workflow_id, workflow_state_id, owner_member_id,
			position, priority, severity, task_type, started, completed, blocked, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID, workspaceID, displayID, taskID, workflowID, stateID, ownerMemberID,
		position, "none", "none", "feature", false, false, false, false, updatedAt, updatedAt,
	).Error; err != nil {
		t.Fatalf("seed task %s: %v", taskID, err)
	}
	if ownerMemberID != "" {
		if err := db.Exec(
			`INSERT INTO pm_task_owners (task_id, user_id, created_at)
			 SELECT ?, user_id, ?
			 FROM workspace_members
			 WHERE id = ?`,
			taskID, updatedAt, ownerMemberID,
		).Error; err != nil {
			t.Fatalf("seed task owner %s: %v", taskID, err)
		}
	}
}

func seedPMTaskOwner(t *testing.T, db *gorm.DB, taskID, userID string, createdAt time.Time) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO pm_task_owners (task_id, user_id, created_at) VALUES (?, ?, ?)`,
		taskID, userID, createdAt,
	).Error; err != nil {
		t.Fatalf("seed task owner: %v", err)
	}
}

func taskIDsFromBoardTasks(tasks []model.BoardTask) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func testStringPtr(value string) *string {
	return &value
}
