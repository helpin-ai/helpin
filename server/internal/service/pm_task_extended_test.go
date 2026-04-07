package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

// taskTestEnv bundles the service, DB, and common IDs used across task tests.
type taskTestEnv struct {
	svc          *PMTaskService
	db           *gorm.DB
	wsID         string
	userID       string
	wfID         string
	stTodo       string // default "unstarted" state
	stInProgress string
	stDone       string
}

// newTaskTestEnv creates a fresh test environment for PMTaskService tests:
// workspace, user, workspace member (admin role), workflow with three states.
func newTaskTestEnv(t *testing.T) taskTestEnv {
	t.Helper()
	db := newTestDB(t)

	wsID := "ws-story-001"
	userID := "user-story-001"
	memberID := "member-story-001"
	wfID := "wf-story-001"
	stTodo := "state-todo-001"
	stInProgress := "state-inprogress-001"
	stDone := "state-done-001"

	// Seed additional tables needed by dependency queries (gracefully ignored if missing).
	db.Exec(`CREATE TABLE IF NOT EXISTS pm_task_links (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		source_task_id TEXT NOT NULL,
		target_task_id TEXT NOT NULL,
		link_type TEXT NOT NULL,
		created_by TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS crm_associations (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		from_object_type TEXT NOT NULL,
		from_object_id TEXT NOT NULL,
		to_object_type TEXT NOT NULL,
		to_object_id TEXT NOT NULL,
		association_label TEXT,
		created_at DATETIME
	)`)

	seedUser(t, db, userID, "storyadmin@test.com", "Story Admin", "hash")
	seedWorkspace(t, db, wsID, "Story Workspace", "story-ws", userID)
	seedWorkspaceMember(t, db, memberID, wsID, userID, "storyadmin@test.com", "Story Admin", model.RoleAdmin)

	// Seed workflow with three states: To Do (unstarted), In Progress (started), Done (done).
	now := time.Now()
	mustExec(t, db, `INSERT INTO pm_workflows (id, workspace_id, name, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		wfID, wsID, "Test Workflow", stTodo, now, now)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		stTodo, wfID, "To Do", "unstarted", 0, true, now, now)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		stInProgress, wfID, "In Progress", "started", 1, false, now, now)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		stDone, wfID, "Done", "done", 2, false, now, now)

	storyRepo := repository.NewPMTaskRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)
	activityService := NewPMActivityService(activityRepo)

	// wsPublisher is nil-safe (Publish is a no-op on nil receiver).
	svc := NewPMTaskService(
		storyRepo,
		workspaceRepo,
		workflowRepo,
		repository.NewPMEpicRepository(db),
		repository.NewPMSprintRepository(db),
		labelRepo,
		nil,
		nil,
		repository.NewPMAttachmentRepository(db),
		activityService,
		nil,
		nil,
		nil,
		nil,
	)

	return taskTestEnv{
		svc:          svc,
		db:           db,
		wsID:         wsID,
		userID:       userID,
		wfID:         wfID,
		stTodo:       stTodo,
		stInProgress: stInProgress,
		stDone:       stDone,
	}
}

// createTestTask is a helper that creates a story with minimal fields.
func createTestTask(t *testing.T, env taskTestEnv, name string) *model.TaskDetail {
	t.Helper()
	story, err := env.svc.Create(context.Background(), model.CreateTaskRequest{
		WorkspaceID:     env.wsID,
		Name:            name,
		WorkflowID:      env.wfID,
		WorkflowStateID: env.stTodo,
	}, env.userID)
	if err != nil {
		t.Fatalf("createTestTask(%q): %v", name, err)
	}
	return story
}

func seedTaskTeam(t *testing.T, env taskTestEnv, teamID, name string) {
	t.Helper()
	now := time.Now()
	mustExec(t, env.db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		teamID, env.wsID, name, now, now)
}

func seedTaskEpic(t *testing.T, env taskTestEnv, epicID, teamID, name string) {
	t.Helper()
	now := time.Now()
	mustExec(t, env.db, `INSERT INTO pm_epics (id, workspace_id, name, team_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		epicID, env.wsID, name, nullableTestString(teamID), now, now)
}

func seedStorySprint(t *testing.T, env taskTestEnv, sprintID, teamID, name string) {
	t.Helper()
	now := time.Now()
	mustExec(t, env.db, `INSERT INTO pm_sprints (id, workspace_id, name, team_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		sprintID, env.wsID, name, nullableTestString(teamID), now, now)
}

func nullableTestString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func latestStoryActivityAction(t *testing.T, env taskTestEnv, storyID string) string {
	t.Helper()
	entries, total, err := env.svc.ListActivity(context.Background(), storyID, model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	if total == 0 || len(entries) == 0 {
		t.Fatal("expected activity entries")
	}
	return entries[0].Activity.Action
}

// ---------------------------------------------------------------------------
// 1. Create story
// ---------------------------------------------------------------------------

func TestPMTaskService_Create(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	t.Run("basic create", func(t *testing.T) {
		story, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "My Story",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
		}, env.userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if story == nil {
			t.Fatal("expected non-nil story detail")
		}
		if story.Task.Name != "My Story" {
			t.Errorf("name = %q, want %q", story.Task.Name, "My Story")
		}
		if story.Task.WorkspaceID != env.wsID {
			t.Errorf("workspace_id = %q, want %q", story.Task.WorkspaceID, env.wsID)
		}
		if story.Task.ID == "" {
			t.Error("expected non-empty ID")
		}
		if story.Task.DisplayID < 1 {
			t.Errorf("display_id = %d, want >= 1", story.Task.DisplayID)
		}
		if story.Task.WorkflowStateID != env.stTodo {
			t.Errorf("workflow_state_id = %q, want %q", story.Task.WorkflowStateID, env.stTodo)
		}
		if story.Task.TaskType != model.PMTaskTypeFeature {
			t.Errorf("story_type = %q, want %q", story.Task.TaskType, model.PMTaskTypeFeature)
		}
		if story.Task.Priority != model.PMTaskPriorityNone {
			t.Errorf("priority = %q, want %q", story.Task.Priority, model.PMTaskPriorityNone)
		}
		if story.Task.Severity != model.PMTaskSeverityNone {
			t.Errorf("severity = %q, want %q", story.Task.Severity, model.PMTaskSeverityNone)
		}
		if story.Task.Archived {
			t.Error("expected archived = false")
		}
	})

	t.Run("non-member system actor skips auto requester fallback", func(t *testing.T) {
		story, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "System-created story",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
		}, "agent-system-001")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if story.Task.RequesterMemberID != nil {
			t.Fatalf("expected requester_member_id to remain nil for non-member actor, got %#v", story.Task.RequesterMemberID)
		}
	})

	t.Run("create with explicit type bug", func(t *testing.T) {
		story, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Bug Report",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
			TaskType:       model.PMTaskTypeBug,
		}, env.userID)
		if err != nil {
			t.Fatalf("Create bug: %v", err)
		}
		if story.Task.TaskType != model.PMTaskTypeBug {
			t.Errorf("story_type = %q, want %q", story.Task.TaskType, model.PMTaskTypeBug)
		}
	})

	t.Run("create rejects epic from another team", func(t *testing.T) {
		teamA := "team-story-a"
		teamB := "team-story-b"
		epicB := "epic-story-b"
		seedTaskTeam(t, env, teamA, "Team A")
		seedTaskTeam(t, env, teamB, "Team B")
		seedTaskEpic(t, env, epicB, teamB, "Epic B")

		_, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Scoped Story",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
			TeamID:          stringPtr(teamA),
			EpicID:          stringPtr(epicB),
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for cross-team epic assignment")
		}
	})

	t.Run("create rejects sprint from another team", func(t *testing.T) {
		teamA := "team-story-c"
		teamB := "team-story-d"
		sprintB := "sprint-story-b"
		seedTaskTeam(t, env, teamA, "Team C")
		seedTaskTeam(t, env, teamB, "Team D")
		seedStorySprint(t, env, sprintB, teamB, "Sprint B")

		_, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Scoped Story Sprint",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
			TeamID:          stringPtr(teamA),
			SprintID:        stringPtr(sprintB),
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for cross-team sprint assignment")
		}
	})

	t.Run("create with priority and severity", func(t *testing.T) {
		prio := model.PMTaskPriorityHigh
		sev := model.PMTaskSeverityMajor
		story, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Important Bug",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
			Priority:        &prio,
			Severity:        &sev,
		}, env.userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if story.Task.Priority != model.PMTaskPriorityHigh {
			t.Errorf("priority = %q, want %q", story.Task.Priority, model.PMTaskPriorityHigh)
		}
		if story.Task.Severity != model.PMTaskSeverityMajor {
			t.Errorf("severity = %q, want %q", story.Task.Severity, model.PMTaskSeverityMajor)
		}
	})

	t.Run("create reassigns temporary attachment ids", func(t *testing.T) {
		seedTemporaryAttachment(t, env.db, "attachment-story-1", env.wsID, env.wsID, env.userID)

		story, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Story With Image",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
			AttachmentIDs:   []string{"attachment-story-1"},
		}, env.userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		attachmentRepo := repository.NewPMAttachmentRepository(env.db)
		attachment, err := attachmentRepo.GetByID(ctx, "attachment-story-1")
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if attachment == nil {
			t.Fatal("expected attachment")
		}
		if attachment.EntityType != "task" {
			t.Fatalf("entity_type = %q, want %q", attachment.EntityType, "task")
		}
		if attachment.EntityID != story.Task.ID {
			t.Fatalf("entity_id = %q, want %q", attachment.EntityID, story.Task.ID)
		}
	})

	t.Run("display_id increments per workspace", func(t *testing.T) {
		s1 := createTestTask(t, env, "First")
		s2 := createTestTask(t, env, "Second")
		if s2.Task.DisplayID != s1.Task.DisplayID+1 {
			t.Errorf("display_id: first=%d, second=%d; expected consecutive", s1.Task.DisplayID, s2.Task.DisplayID)
		}
	})

	t.Run("create without explicit position appends to end of state column", func(t *testing.T) {
		now := time.Now().UTC()
		mustExec(t, env.db, `INSERT INTO pm_tasks (
			id, workspace_id, display_id, name, workflow_id, workflow_state_id, position, task_type, priority, severity, started, completed, blocked, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"create-position-a", env.wsID, 2001, "Create Position A", env.wfID, env.stTodo, 0, "feature", "none", "none", false, false, false, false, now, now,
		)
		mustExec(t, env.db, `INSERT INTO pm_tasks (
			id, workspace_id, display_id, name, workflow_id, workflow_state_id, position, task_type, priority, severity, started, completed, blocked, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"create-position-b", env.wsID, 2002, "Create Position B", env.wfID, env.stTodo, 1, "feature", "none", "none", false, false, false, false, now, now,
		)
		expectedPosition, err := repository.NewPMTaskRepository(env.db).NextPosition(ctx, env.wsID, env.stTodo)
		if err != nil {
			t.Fatalf("NextPosition: %v", err)
		}

		story, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Create Position C",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
		}, env.userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if story.Task.Position != expectedPosition {
			t.Fatalf("position = %d, want %d", story.Task.Position, expectedPosition)
		}
	})

	t.Run("create defaults workflow and state if omitted", func(t *testing.T) {
		// When workflowID is empty, Create should use GetDefaultWorkflow / SeedDefaultWorkflow.
		// Our seeded workflow is already the default for the workspace.
		story, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID: env.wsID,
			Name:        "Auto Workflow Story",
		}, env.userID)
		if err != nil {
			t.Fatalf("Create without workflow: %v", err)
		}
		if story.Task.WorkflowID == "" {
			t.Error("expected non-empty workflow_id")
		}
		if story.Task.WorkflowStateID == "" {
			t.Error("expected non-empty workflow_state_id")
		}
	})
}

// ---------------------------------------------------------------------------
// 2. Create story validation
// ---------------------------------------------------------------------------

func TestPMTaskService_CreateValidation(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	t.Run("missing name", func(t *testing.T) {
		_, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("whitespace-only name", func(t *testing.T) {
		_, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "   ",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for whitespace-only name")
		}
	})

	t.Run("missing workspace_id", func(t *testing.T) {
		_, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     "",
			Name:            "No WS",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for empty workspace_id")
		}
	})

	t.Run("invalid story_type", func(t *testing.T) {
		_, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Bad Type",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
			TaskType:       "invalid_type",
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for invalid story_type")
		}
	})

	t.Run("invalid priority", func(t *testing.T) {
		badPrio := "super_urgent"
		_, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Bad Prio",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
			Priority:        &badPrio,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for invalid priority")
		}
	})

	t.Run("invalid severity", func(t *testing.T) {
		badSev := "catastrophic"
		_, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Bad Sev",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
			Severity:        &badSev,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for invalid severity")
		}
	})

	t.Run("state not belonging to workflow", func(t *testing.T) {
		_, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Wrong State",
			WorkflowID:      env.wfID,
			WorkflowStateID: "nonexistent-state",
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for state not in workflow")
		}
	})

	t.Run("forbidden for viewer role", func(t *testing.T) {
		viewerID := "user-viewer-001"
		seedUser(t, env.db, viewerID, "viewer@test.com", "Viewer User", "hash")
		seedWorkspaceMember(t, env.db, "member-viewer-001", env.wsID, viewerID, "viewer@test.com", "Viewer User", "viewer")

		_, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Viewer Attempt",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
		}, viewerID)
		if err == nil {
			t.Fatal("expected forbidden error for viewer")
		}
	})
}

// ---------------------------------------------------------------------------
// 3. List stories
// ---------------------------------------------------------------------------

func TestPMTaskService_List(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	createTestTask(t, env, "Story Alpha")
	createTestTask(t, env, "Story Beta")

	stories, total, err := env.svc.List(ctx, env.wsID, model.PMTaskFilters{}, model.PMPagination{Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(stories) != 2 {
		t.Errorf("len(stories) = %d, want 2", len(stories))
	}
}

func TestPMTaskService_ListEmptyWorkspace(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	stories, total, err := env.svc.List(ctx, env.wsID, model.PMTaskFilters{}, model.PMPagination{Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(stories) != 0 {
		t.Errorf("len(stories) = %d, want 0", len(stories))
	}
}

func TestPMTaskService_ListAssociationFilters(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	contactStory := createTestTask(t, env, "Story With Contact")
	supportStory := createTestTask(t, env, "Story With Support")
	now := time.Now().UTC()

	mustExec(t, env.db, `INSERT INTO crm_associations (
		id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"assoc-story-contact-1", env.wsID, model.CRMObjectTask, contactStory.Task.ID, model.CRMObjectContact, "contact-123", now,
	)

	mustExec(t, env.db, `INSERT INTO support_conversations (
		id, workspace_id, display_id, subject, status, priority, channel, source, linked_task_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"support-conv-123", env.wsID, 9001, "Customer cannot log in", "open", "medium", "widget", "internal", supportStory.Task.ID, now, now,
	)

	stories, total, err := env.svc.List(ctx, env.wsID, model.PMTaskFilters{
		ContactID: strPtr("contact-123"),
	}, model.PMPagination{Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("List by contact association: %v", err)
	}
	if total != 1 {
		t.Fatalf("contact filter total = %d, want 1", total)
	}
	if len(stories) != 1 || stories[0].ID != contactStory.Task.ID {
		t.Fatalf("contact filter returned %+v, want only %s", stories, contactStory.Task.ID)
	}

	stories, total, err = env.svc.List(ctx, env.wsID, model.PMTaskFilters{
		SupportConversationID: strPtr("support-conv-123"),
	}, model.PMPagination{Page: 1, PerPage: 50})
	if err != nil {
		t.Fatalf("List by support association: %v", err)
	}
	if total != 1 {
		t.Fatalf("support filter total = %d, want 1", total)
	}
	if len(stories) != 1 || stories[0].ID != supportStory.Task.ID {
		t.Fatalf("support filter returned %+v, want only %s", stories, supportStory.Task.ID)
	}
}

func TestPMTaskService_ListRequiresWorkspaceID(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	_, _, err := env.svc.List(ctx, "", model.PMTaskFilters{}, model.PMPagination{})
	if err == nil {
		t.Fatal("expected error for empty workspace_id")
	}
}

// ---------------------------------------------------------------------------
// 4. GetByID
// ---------------------------------------------------------------------------

func TestPMTaskService_GetByID(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	created := createTestTask(t, env, "Get Me")

	got, err := env.svc.GetByID(ctx, created.Task.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil story")
	}
	if got.Task.ID != created.Task.ID {
		t.Errorf("id = %q, want %q", got.Task.ID, created.Task.ID)
	}
	if got.Task.Name != "Get Me" {
		t.Errorf("name = %q, want %q", got.Task.Name, "Get Me")
	}
	if got.Task.WorkspaceID != env.wsID {
		t.Errorf("workspace_id = %q, want %q", got.Task.WorkspaceID, env.wsID)
	}
	if got.Task.DisplayID != created.Task.DisplayID {
		t.Errorf("display_id = %d, want %d", got.Task.DisplayID, created.Task.DisplayID)
	}
}

func TestPMTaskService_GetByDisplayID(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	created := createTestTask(t, env, "Display ID Story")

	got, err := env.svc.GetByDisplayID(ctx, env.wsID, created.Task.DisplayID)
	if err != nil {
		t.Fatalf("GetByDisplayID: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil story")
	}
	if got.Task.ID != created.Task.ID {
		t.Errorf("id = %q, want %q", got.Task.ID, created.Task.ID)
	}
}

// ---------------------------------------------------------------------------
// 5. GetByID not found
// ---------------------------------------------------------------------------

func TestPMTaskService_GetByIDNotFound(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	_, err := env.svc.GetByID(ctx, "nonexistent-story-id")
	if err == nil {
		t.Fatal("expected error for unknown story ID")
	}
}

func TestPMTaskService_GetByDisplayIDNotFound(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	_, err := env.svc.GetByDisplayID(ctx, env.wsID, 99999)
	if err == nil {
		t.Fatal("expected error for unknown display ID")
	}
}

// ---------------------------------------------------------------------------
// 6. Update story
// ---------------------------------------------------------------------------

func TestPMTaskService_Update(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	created := createTestTask(t, env, "Original Name")

	t.Run("update name", func(t *testing.T) {
		newName := "Updated Name"
		updated, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			Name: &newName,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update name: %v", err)
		}
		if updated.Task.Name != "Updated Name" {
			t.Errorf("name = %q, want %q", updated.Task.Name, "Updated Name")
		}
	})

	t.Run("update priority", func(t *testing.T) {
		prio := model.PMTaskPriorityUrgent
		updated, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			Priority: &prio,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update priority: %v", err)
		}
		if updated.Task.Priority != model.PMTaskPriorityUrgent {
			t.Errorf("priority = %q, want %q", updated.Task.Priority, model.PMTaskPriorityUrgent)
		}
	})

	t.Run("update severity", func(t *testing.T) {
		sev := model.PMTaskSeverityCritical
		updated, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			Severity: &sev,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update severity: %v", err)
		}
		if updated.Task.Severity != model.PMTaskSeverityCritical {
			t.Errorf("severity = %q, want %q", updated.Task.Severity, model.PMTaskSeverityCritical)
		}
	})

	t.Run("update story_type", func(t *testing.T) {
		st := model.PMTaskTypeChore
		updated, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			TaskType: &st,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update story_type: %v", err)
		}
		if updated.Task.TaskType != model.PMTaskTypeChore {
			t.Errorf("story_type = %q, want %q", updated.Task.TaskType, model.PMTaskTypeChore)
		}
	})

	t.Run("update description", func(t *testing.T) {
		desc := "Updated description text"
		updated, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			Description: &desc,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update description: %v", err)
		}
		if updated.Task.Description == nil || *updated.Task.Description != desc {
			t.Errorf("description mismatch")
		}
	})

	t.Run("clear epic with empty string", func(t *testing.T) {
		epicID := "epic-001"
		seedTaskEpic(t, env, epicID, "", "Shared Epic")
		updated, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			EpicID: &epicID,
		}, env.userID)
		if err != nil {
			t.Fatalf("set epic: %v", err)
		}
		if updated.Task.EpicID == nil || *updated.Task.EpicID != epicID {
			t.Fatalf("epic_id = %v, want %q", updated.Task.EpicID, epicID)
		}

		emptyEpicID := ""
		cleared, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			EpicID: &emptyEpicID,
		}, env.userID)
		if err != nil {
			t.Fatalf("clear epic: %v", err)
		}
		if cleared.Task.EpicID != nil {
			t.Fatalf("expected epic_id to be cleared, got %v", *cleared.Task.EpicID)
		}
	})

	t.Run("update rejects epic from another team", func(t *testing.T) {
		teamA := "team-story-upd-a"
		teamB := "team-story-upd-b"
		epicB := "epic-story-upd-b"
		seedTaskTeam(t, env, teamA, "Update Team A")
		seedTaskTeam(t, env, teamB, "Update Team B")
		seedTaskEpic(t, env, epicB, teamB, "Update Epic B")

		_, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			TeamID: stringPtr(teamA),
			EpicID: stringPtr(epicB),
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for cross-team epic assignment")
		}
	})

	t.Run("update rejects sprint from another team", func(t *testing.T) {
		teamA := "team-story-upd-c"
		teamB := "team-story-upd-d"
		sprintB := "sprint-story-upd-b"
		seedTaskTeam(t, env, teamA, "Update Team C")
		seedTaskTeam(t, env, teamB, "Update Team D")
		seedStorySprint(t, env, sprintB, teamB, "Update Sprint B")

		_, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			TeamID:   stringPtr(teamA),
			SprintID: stringPtr(sprintB),
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for cross-team sprint assignment")
		}
	})

	t.Run("update blocked", func(t *testing.T) {
		blocked := true
		updated, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			Blocked: &blocked,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update blocked: %v", err)
		}
		if !updated.Task.Blocked {
			t.Error("expected blocked = true")
		}

		// Unblock.
		unblocked := false
		updated2, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			Blocked: &unblocked,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update unblocked: %v", err)
		}
		if updated2.Task.Blocked {
			t.Error("expected blocked = false")
		}
	})

	t.Run("update blocker sets blocked", func(t *testing.T) {
		blocker := "waiting on backend API"
		updated, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			Blocker: &blocker,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update blocker: %v", err)
		}
		if !updated.Task.Blocked {
			t.Error("expected blocked = true when blocker is set")
		}
		if updated.Task.Blocker == nil || *updated.Task.Blocker != blocker {
			t.Errorf("blocker mismatch")
		}
	})

	t.Run("empty name rejected", func(t *testing.T) {
		emptyName := ""
		_, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			Name: &emptyName,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("invalid priority rejected", func(t *testing.T) {
		badPrio := "extreme"
		_, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			Priority: &badPrio,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for invalid priority")
		}
	})

	t.Run("invalid severity rejected", func(t *testing.T) {
		badSev := "apocalyptic"
		_, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			Severity: &badSev,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for invalid severity")
		}
	})

	t.Run("invalid story_type rejected", func(t *testing.T) {
		badType := "epic_story"
		_, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			TaskType: &badType,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for invalid story_type")
		}
	})

	t.Run("update nonexistent story", func(t *testing.T) {
		newName := "Nope"
		_, err := env.svc.Update(ctx, "nonexistent-story-id", model.UpdateTaskRequest{
			Name: &newName,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for nonexistent story")
		}
	})

	t.Run("forbidden for viewer role", func(t *testing.T) {
		viewerID := "user-viewer-upd-001"
		seedUser(t, env.db, viewerID, "viewer-upd@test.com", "Viewer User", "hash")
		seedWorkspaceMember(t, env.db, "wm-viewer-upd-001", env.wsID, viewerID, "viewer-upd@test.com", "Viewer User", "viewer")

		newName := "Forbidden Update"
		_, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
			Name: &newName,
		}, viewerID)
		if err == nil {
			t.Fatal("expected forbidden error for viewer role")
		}
	})
}

func TestPMTaskService_UpdateActivityLogging(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	teamID := "team-activity-001"
	seedTaskTeam(t, env, teamID, "Growth")
	epicID := "epic-activity-001"
	seedTaskEpic(t, env, epicID, "", "Launch")
	sprintID := "sprint-activity-001"
	seedStorySprint(t, env, sprintID, "", "Sprint 8")

	ownerUserID := "user-owner-activity-001"
	ownerMemberID := "member-owner-activity-001"
	seedUser(t, env.db, ownerUserID, "owner@test.com", "Alice Owner", "hash")
	seedWorkspaceMember(t, env.db, ownerMemberID, env.wsID, ownerUserID, "owner@test.com", "Alice Owner", model.RoleMember)

	requesterUserID := "user-requester-activity-001"
	requesterMemberID := "member-requester-activity-001"
	seedUser(t, env.db, requesterUserID, "requester@test.com", "Rita Requester", "hash")
	seedWorkspaceMember(t, env.db, requesterMemberID, env.wsID, requesterUserID, "requester@test.com", "Rita Requester", model.RoleMember)

	t.Run("team assignment logs activity", func(t *testing.T) {
		story := createTestTask(t, env, "Team Activity Story")
		_, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			TeamID: stringPtr(teamID),
		}, env.userID)
		if err != nil {
			t.Fatalf("Update team: %v", err)
		}
		if got := latestStoryActivityAction(t, env, story.Task.ID); got != "assigned this story to team Growth" {
			t.Fatalf("latest activity = %q", got)
		}
	})

	t.Run("owner assignment logs activity", func(t *testing.T) {
		story := createTestTask(t, env, "Owner Activity Story")
		_, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			OwnerMemberID: stringPtr(ownerMemberID),
		}, env.userID)
		if err != nil {
			t.Fatalf("Update owner: %v", err)
		}
		if got := latestStoryActivityAction(t, env, story.Task.ID); got != "assigned owner Alice Owner" {
			t.Fatalf("latest activity = %q", got)
		}
	})

	t.Run("requester assignment logs activity", func(t *testing.T) {
		story := createTestTask(t, env, "Requester Activity Story")
		_, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			RequesterMemberID: stringPtr(requesterMemberID),
		}, env.userID)
		if err != nil {
			t.Fatalf("Update requester: %v", err)
		}
		if got := latestStoryActivityAction(t, env, story.Task.ID); got != "changed requester from Story Admin to Rita Requester" {
			t.Fatalf("latest activity = %q", got)
		}
	})

	t.Run("epic assignment logs activity", func(t *testing.T) {
		story := createTestTask(t, env, "Epic Activity Story")
		_, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			EpicID: stringPtr(epicID),
		}, env.userID)
		if err != nil {
			t.Fatalf("Update epic: %v", err)
		}
		if got := latestStoryActivityAction(t, env, story.Task.ID); got != "added this story to epic Launch" {
			t.Fatalf("latest activity = %q", got)
		}
	})

	t.Run("sprint assignment logs activity", func(t *testing.T) {
		story := createTestTask(t, env, "Sprint Activity Story")
		_, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			SprintID: stringPtr(sprintID),
		}, env.userID)
		if err != nil {
			t.Fatalf("Update sprint: %v", err)
		}
		if got := latestStoryActivityAction(t, env, story.Task.ID); got != "added this story to sprint Sprint 8" {
			t.Fatalf("latest activity = %q", got)
		}
	})

	t.Run("estimate assignment logs activity", func(t *testing.T) {
		story := createTestTask(t, env, "Estimate Activity Story")
		estimate := 5
		_, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			Estimate: &estimate,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update estimate: %v", err)
		}
		if got := latestStoryActivityAction(t, env, story.Task.ID); got != "set estimate to 5" {
			t.Fatalf("latest activity = %q", got)
		}
	})

	t.Run("deadline assignment logs activity", func(t *testing.T) {
		story := createTestTask(t, env, "Deadline Activity Story")
		deadline := time.Date(2026, time.April, 3, 0, 0, 0, 0, time.UTC)
		_, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			Deadline: &deadline,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update deadline: %v", err)
		}
		if got := latestStoryActivityAction(t, env, story.Task.ID); got != "set due date to 2026-04-03" {
			t.Fatalf("latest activity = %q", got)
		}
	})

	t.Run("blocker text update logs activity when already blocked", func(t *testing.T) {
		story := createTestTask(t, env, "Blocker Activity Story")
		initialBlocker := "Waiting on API"
		_, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			Blocker: &initialBlocker,
		}, env.userID)
		if err != nil {
			t.Fatalf("Set blocker: %v", err)
		}

		nextBlocker := "Waiting on API review"
		_, err = env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			Blocker: &nextBlocker,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update blocker: %v", err)
		}
		if got := latestStoryActivityAction(t, env, story.Task.ID); got != "updated blocker reason" {
			t.Fatalf("latest activity = %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// 7. Update story state (via Update and MoveToState)
// ---------------------------------------------------------------------------

func TestPMTaskService_UpdateState(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	stInProgress := "state-inprogress-001"
	stDone := "state-done-001"

	t.Run("move to in_progress via Update", func(t *testing.T) {
		story := createTestTask(t, env, "State Change Story")

		updated, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			WorkflowStateID: &stInProgress,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update state: %v", err)
		}
		if updated.Task.WorkflowStateID != stInProgress {
			t.Errorf("workflow_state_id = %q, want %q", updated.Task.WorkflowStateID, stInProgress)
		}
		// Verify raw row to check started/completed flags.
		var raw model.PMTask
		if err := env.db.Where("id = ?", story.Task.ID).First(&raw).Error; err != nil {
			t.Fatalf("raw query: %v", err)
		}
		if !raw.Started {
			t.Error("expected started = true after moving to 'started' state")
		}
		if raw.Completed {
			t.Error("expected completed = false for 'started' state")
		}
	})

	t.Run("move to done via Update", func(t *testing.T) {
		story := createTestTask(t, env, "Done Story")

		updated, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			WorkflowStateID: &stDone,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update state to done: %v", err)
		}
		if updated.Task.WorkflowStateID != stDone {
			t.Errorf("workflow_state_id = %q, want %q", updated.Task.WorkflowStateID, stDone)
		}
		var raw model.PMTask
		if err := env.db.Where("id = ?", story.Task.ID).First(&raw).Error; err != nil {
			t.Fatalf("raw query: %v", err)
		}
		if !raw.Started {
			t.Error("expected started = true after moving to 'done' state")
		}
		if !raw.Completed {
			t.Error("expected completed = true after moving to 'done' state")
		}
		if raw.CompletedAt == nil {
			t.Error("expected completed_at to be set")
		}
	})

	t.Run("move back to unstarted clears started/completed", func(t *testing.T) {
		story := createTestTask(t, env, "Back To Todo")

		// First move to done.
		_, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			WorkflowStateID: &stDone,
		}, env.userID)
		if err != nil {
			t.Fatalf("Move to done: %v", err)
		}

		// Move back to todo.
		todoState := env.stTodo
		_, err = env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			WorkflowStateID: &todoState,
		}, env.userID)
		if err != nil {
			t.Fatalf("Move back to todo: %v", err)
		}

		var raw model.PMTask
		if err := env.db.Where("id = ?", story.Task.ID).First(&raw).Error; err != nil {
			t.Fatalf("raw query: %v", err)
		}
		if raw.Started {
			t.Error("expected started = false after moving back to 'unstarted' state")
		}
		if raw.Completed {
			t.Error("expected completed = false after moving back to 'unstarted' state")
		}
	})

	t.Run("state not in workflow rejected", func(t *testing.T) {
		story := createTestTask(t, env, "Bad State Move")
		badState := "nonexistent-state-id"
		_, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			WorkflowStateID: &badState,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for state not in workflow")
		}
	})
}

func TestPMTaskService_MoveToState(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	stInProgress := "state-inprogress-001"
	stDone := "state-done-001"

	t.Run("move to in_progress", func(t *testing.T) {
		story := createTestTask(t, env, "Move Via MoveToState")

		moved, err := env.svc.MoveToState(ctx, story.Task.ID, model.MoveTaskRequest{
			StateID: stInProgress,
		}, env.userID)
		if err != nil {
			t.Fatalf("MoveToState: %v", err)
		}
		if moved.Task.WorkflowStateID != stInProgress {
			t.Errorf("workflow_state_id = %q, want %q", moved.Task.WorkflowStateID, stInProgress)
		}
	})

	t.Run("move to done", func(t *testing.T) {
		story := createTestTask(t, env, "Move To Done")

		moved, err := env.svc.MoveToState(ctx, story.Task.ID, model.MoveTaskRequest{
			StateID: stDone,
		}, env.userID)
		if err != nil {
			t.Fatalf("MoveToState: %v", err)
		}
		if moved.Task.WorkflowStateID != stDone {
			t.Errorf("workflow_state_id = %q, want %q", moved.Task.WorkflowStateID, stDone)
		}

		var raw model.PMTask
		if err := env.db.Where("id = ?", story.Task.ID).First(&raw).Error; err != nil {
			t.Fatalf("raw query: %v", err)
		}
		if !raw.Completed {
			t.Error("expected completed = true")
		}
	})

	t.Run("move with position", func(t *testing.T) {
		env := newTaskTestEnv(t)
		story := createTestTask(t, env, "Move With Position")
		now := time.Now().UTC()
		mustExec(t, env.db, `INSERT INTO pm_tasks (
			id, workspace_id, display_id, name, workflow_id, workflow_state_id, position, task_type, priority, severity, started, completed, blocked, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"position-target-a", env.wsID, 1000, "Position Target A", env.wfID, env.stInProgress, 0, "feature", "none", "none", true, false, false, false, now, now,
		)
		pos := 1

		moved, err := env.svc.MoveToState(ctx, story.Task.ID, model.MoveTaskRequest{
			StateID:  env.stInProgress,
			Position: &pos,
		}, env.userID)
		if err != nil {
			t.Fatalf("MoveToState with position: %v", err)
		}
		if moved.Task.WorkflowStateID != env.stInProgress {
			t.Errorf("workflow_state_id = %q, want %q", moved.Task.WorkflowStateID, env.stInProgress)
		}
		var raw model.PMTask
		if err := env.db.Where("id = ?", story.Task.ID).First(&raw).Error; err != nil {
			t.Fatalf("raw query: %v", err)
		}
		if raw.Position != 1 {
			t.Errorf("position = %d, want 1", raw.Position)
		}
	})

	t.Run("move without position appends to end of target column", func(t *testing.T) {
		env := newTaskTestEnv(t)
		story := createTestTask(t, env, "Move Without Position")
		now := time.Now().UTC()
		mustExec(t, env.db, `INSERT INTO pm_tasks (
			id, workspace_id, display_id, name, workflow_id, workflow_state_id, position, task_type, priority, severity, started, completed, blocked, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"state-target-a", env.wsID, 1001, "Target A", env.wfID, env.stInProgress, 0, "feature", "none", "none", true, false, false, false, now, now,
		)
		mustExec(t, env.db, `INSERT INTO pm_tasks (
			id, workspace_id, display_id, name, workflow_id, workflow_state_id, position, task_type, priority, severity, started, completed, blocked, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"state-target-b", env.wsID, 1002, "Target B", env.wfID, env.stInProgress, 1, "feature", "none", "none", true, false, false, false, now, now,
		)

		moved, err := env.svc.MoveToState(ctx, story.Task.ID, model.MoveTaskRequest{
			StateID: env.stInProgress,
		}, env.userID)
		if err != nil {
			t.Fatalf("MoveToState without position: %v", err)
		}
		if moved.Task.WorkflowStateID != env.stInProgress {
			t.Errorf("workflow_state_id = %q, want %q", moved.Task.WorkflowStateID, env.stInProgress)
		}
		if moved.Task.Position != 2 {
			t.Fatalf("moved position = %d, want 2", moved.Task.Position)
		}

		var stories []model.PMTask
		if err := env.db.
			Where("workflow_state_id = ? AND id IN ?", env.stInProgress, []string{"state-target-a", "state-target-b", story.Task.ID}).
			Order("position ASC").
			Find(&stories).Error; err != nil {
			t.Fatalf("query moved stories: %v", err)
		}

		gotIDs := []string{stories[0].ID, stories[1].ID, stories[2].ID}
		wantIDs := []string{"state-target-a", "state-target-b", story.Task.ID}
		for i := range wantIDs {
			if gotIDs[i] != wantIDs[i] {
				t.Fatalf("stories[%d] = %q, want %q (full order %v)", i, gotIDs[i], wantIDs[i], gotIDs)
			}
			if stories[i].Position != i {
				t.Fatalf("stories[%d] position = %d, want %d", i, stories[i].Position, i)
			}
		}
	})

	t.Run("empty state_id rejected", func(t *testing.T) {
		story := createTestTask(t, env, "No State ID")

		_, err := env.svc.MoveToState(ctx, story.Task.ID, model.MoveTaskRequest{
			StateID: "",
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for empty state_id")
		}
	})

	t.Run("state not in workflow rejected", func(t *testing.T) {
		story := createTestTask(t, env, "Bad State")

		_, err := env.svc.MoveToState(ctx, story.Task.ID, model.MoveTaskRequest{
			StateID: "nonexistent-state",
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for state not in workflow")
		}
	})

	t.Run("nonexistent story", func(t *testing.T) {
		_, err := env.svc.MoveToState(ctx, "nonexistent-story", model.MoveTaskRequest{
			StateID: stInProgress,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for nonexistent story")
		}
	})
}

// ---------------------------------------------------------------------------
// 8. Delete story
// ---------------------------------------------------------------------------

func TestPMTaskService_Delete(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	t.Run("delete archives story", func(t *testing.T) {
		story := createTestTask(t, env, "To Be Deleted")

		err := env.svc.Delete(ctx, story.Task.ID, env.userID)
		if err != nil {
			t.Fatalf("Delete: %v", err)
		}

		// Verify archived flag is set.
		var raw model.PMTask
		if err := env.db.Where("id = ?", story.Task.ID).First(&raw).Error; err != nil {
			t.Fatalf("query after delete: %v", err)
		}
		if !raw.Archived {
			t.Error("expected archived = true after Delete")
		}
	})

	t.Run("delete nonexistent story", func(t *testing.T) {
		err := env.svc.Delete(ctx, "nonexistent-story-id", env.userID)
		if err == nil {
			t.Fatal("expected error for nonexistent story")
		}
	})

	t.Run("delete requires admin", func(t *testing.T) {
		story := createTestTask(t, env, "Admin Only Delete")

		managerID := "user-mgr-del-001"
		seedUser(t, env.db, managerID, "mgr-del@test.com", "Manager Del", "hash")
		seedWorkspaceMember(t, env.db, "wm-mgr-del-001", env.wsID, managerID, "mgr-del@test.com", "Manager Del", "manager")

		err := env.svc.Delete(ctx, story.Task.ID, managerID)
		if err == nil {
			t.Fatal("expected forbidden error for manager role on Delete")
		}
	})

	t.Run("delete allowed for admin", func(t *testing.T) {
		story := createTestTask(t, env, "Admin Can Delete")

		err := env.svc.Delete(ctx, story.Task.ID, env.userID)
		if err != nil {
			t.Fatalf("Delete by admin: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 9. Reorder story
// ---------------------------------------------------------------------------

func TestPMTaskService_Reorder(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	t.Run("reorder changes position", func(t *testing.T) {
		env := newTaskTestEnv(t)
		story := createTestTask(t, env, "Reorder Me")
		now := time.Now().UTC()
		mustExec(t, env.db, `INSERT INTO pm_tasks (
			id, workspace_id, display_id, name, workflow_id, workflow_state_id, position, task_type, priority, severity, started, completed, blocked, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"reorder-peer-a", env.wsID, 1003, "Reorder Peer A", env.wfID, env.stTodo, 1, "feature", "none", "none", false, false, false, false, now, now,
		)
		mustExec(t, env.db, `INSERT INTO pm_tasks (
			id, workspace_id, display_id, name, workflow_id, workflow_state_id, position, task_type, priority, severity, started, completed, blocked, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"reorder-peer-b", env.wsID, 1004, "Reorder Peer B", env.wfID, env.stTodo, 2, "feature", "none", "none", false, false, false, false, now, now,
		)

		err := env.svc.Reorder(ctx, story.Task.ID, model.ReorderTaskRequest{
			Position: 2,
		}, env.userID)
		if err != nil {
			t.Fatalf("Reorder: %v", err)
		}

		var raw model.PMTask
		if err := env.db.Where("id = ?", story.Task.ID).First(&raw).Error; err != nil {
			t.Fatalf("raw query: %v", err)
		}
		if raw.Position != 2 {
			t.Errorf("position = %d, want 2", raw.Position)
		}
	})

	t.Run("negative position rejected", func(t *testing.T) {
		story := createTestTask(t, env, "Negative Pos")

		err := env.svc.Reorder(ctx, story.Task.ID, model.ReorderTaskRequest{
			Position: -1,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for negative position")
		}
	})

	t.Run("nonexistent story", func(t *testing.T) {
		err := env.svc.Reorder(ctx, "nonexistent-story", model.ReorderTaskRequest{
			Position: 5,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for nonexistent story")
		}
	})
}

// ---------------------------------------------------------------------------
// 10. Owner management
// ---------------------------------------------------------------------------

func TestPMTaskService_Owners(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	t.Run("add and remove owner", func(t *testing.T) {
		story := createTestTask(t, env, "Owner Test")

		// Add the actor user as owner.
		err := env.svc.AddOwner(ctx, story.Task.ID, env.userID, env.userID)
		if err != nil {
			t.Fatalf("AddOwner: %v", err)
		}

		// Verify owner exists in pivot table.
		var count int64
		env.db.Table("pm_task_owners").Where("task_id = ? AND user_id = ?", story.Task.ID, env.userID).Count(&count)
		if count != 1 {
			t.Errorf("owner count = %d, want 1", count)
		}

		// Also verify auto-follow.
		var followerCount int64
		env.db.Table("pm_task_followers").Where("task_id = ? AND user_id = ?", story.Task.ID, env.userID).Count(&followerCount)
		if followerCount < 1 {
			t.Error("expected user to be auto-followed when added as owner")
		}

		// Remove owner.
		err = env.svc.RemoveOwner(ctx, story.Task.ID, env.userID, env.userID)
		if err != nil {
			t.Fatalf("RemoveOwner: %v", err)
		}

		env.db.Table("pm_task_owners").Where("task_id = ? AND user_id = ?", story.Task.ID, env.userID).Count(&count)
		if count != 0 {
			t.Errorf("owner count after remove = %d, want 0", count)
		}
	})

	t.Run("add owner requires user_id", func(t *testing.T) {
		story := createTestTask(t, env, "Empty Owner")

		err := env.svc.AddOwner(ctx, story.Task.ID, "", env.userID)
		if err == nil {
			t.Fatal("expected error for empty user_id")
		}
	})
}

// ---------------------------------------------------------------------------
// 11. Follower management
// ---------------------------------------------------------------------------

func TestPMTaskService_Followers(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	t.Run("add and remove follower", func(t *testing.T) {
		story := createTestTask(t, env, "Follower Test")

		err := env.svc.AddFollower(ctx, story.Task.ID, env.userID, env.userID)
		if err != nil {
			t.Fatalf("AddFollower: %v", err)
		}

		var count int64
		env.db.Table("pm_task_followers").Where("task_id = ? AND user_id = ?", story.Task.ID, env.userID).Count(&count)
		if count < 1 {
			t.Error("expected follower to be added")
		}

		err = env.svc.RemoveFollower(ctx, story.Task.ID, env.userID, env.userID)
		if err != nil {
			t.Fatalf("RemoveFollower: %v", err)
		}
	})

	t.Run("add follower requires user_id", func(t *testing.T) {
		story := createTestTask(t, env, "No Follower ID")

		err := env.svc.AddFollower(ctx, story.Task.ID, "", env.userID)
		if err == nil {
			t.Fatal("expected error for empty user_id")
		}
	})
}

// ---------------------------------------------------------------------------
// 12. Label management
// ---------------------------------------------------------------------------

func TestPMTaskService_Labels(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	// Seed a workspace-level label.
	now := time.Now()
	mustExec(t, env.db, `INSERT INTO pm_labels (id, workspace_id, name, color, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"label-001", env.wsID, "Bug", "#ff0000", now, now)

	t.Run("add and remove label", func(t *testing.T) {
		story := createTestTask(t, env, "Label Test")

		err := env.svc.AddLabel(ctx, story.Task.ID, "label-001", env.userID)
		if err != nil {
			t.Fatalf("AddLabel: %v", err)
		}

		var count int64
		env.db.Table("pm_task_labels").Where("task_id = ? AND label_id = ?", story.Task.ID, "label-001").Count(&count)
		if count != 1 {
			t.Errorf("label count = %d, want 1", count)
		}

		err = env.svc.RemoveLabel(ctx, story.Task.ID, "label-001", env.userID)
		if err != nil {
			t.Fatalf("RemoveLabel: %v", err)
		}

		env.db.Table("pm_task_labels").Where("task_id = ? AND label_id = ?", story.Task.ID, "label-001").Count(&count)
		if count != 0 {
			t.Errorf("label count after remove = %d, want 0", count)
		}
	})
}

// ---------------------------------------------------------------------------
// 13. Archive and retrieve
// ---------------------------------------------------------------------------

func TestPMTaskService_ArchiveViaUpdate(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	story := createTestTask(t, env, "Archive Me")

	archived := true
	updated, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
		Archived: &archived,
	}, env.userID)
	if err != nil {
		t.Fatalf("Update archived: %v", err)
	}
	if !updated.Task.Archived {
		t.Error("expected archived = true")
	}
}

// ---------------------------------------------------------------------------
// 14. Estimate field
// ---------------------------------------------------------------------------

func TestPMTaskService_Estimate(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	est := 5
	story, err := env.svc.Create(ctx, model.CreateTaskRequest{
		WorkspaceID:     env.wsID,
		Name:            "Estimated Story",
		WorkflowID:      env.wfID,
		WorkflowStateID: env.stTodo,
		Estimate:        &est,
	}, env.userID)
	if err != nil {
		t.Fatalf("Create with estimate: %v", err)
	}
	if story.Task.Estimate == nil || *story.Task.Estimate != 5 {
		t.Errorf("estimate mismatch, got %v", story.Task.Estimate)
	}

	newEst := 13
	updated, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
		Estimate: &newEst,
	}, env.userID)
	if err != nil {
		t.Fatalf("Update estimate: %v", err)
	}
	if updated.Task.Estimate == nil || *updated.Task.Estimate != 13 {
		t.Errorf("updated estimate mismatch, got %v", updated.Task.Estimate)
	}
}

// ---------------------------------------------------------------------------
// 15. Manager role can create and update (but not delete)
// ---------------------------------------------------------------------------

func TestPMTaskService_ManagerPermissions(t *testing.T) {
	t.Parallel()
	env := newTaskTestEnv(t)
	ctx := context.Background()

	managerID := "user-mgr-001"
	seedUser(t, env.db, managerID, "manager@test.com", "Manager User", "hash")
	seedWorkspaceMember(t, env.db, "wm-mgr-001", env.wsID, managerID, "manager@test.com", "Manager User", "manager")

	t.Run("manager can create", func(t *testing.T) {
		story, err := env.svc.Create(ctx, model.CreateTaskRequest{
			WorkspaceID:     env.wsID,
			Name:            "Manager Created",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
		}, managerID)
		if err != nil {
			t.Fatalf("manager Create: %v", err)
		}
		if story.Task.Name != "Manager Created" {
			t.Errorf("name = %q, want %q", story.Task.Name, "Manager Created")
		}
	})

	t.Run("manager can update", func(t *testing.T) {
		story := createTestTask(t, env, "Manager Updates")

		newName := "Manager Updated"
		updated, err := env.svc.Update(ctx, story.Task.ID, model.UpdateTaskRequest{
			Name: &newName,
		}, managerID)
		if err != nil {
			t.Fatalf("manager Update: %v", err)
		}
		if updated.Task.Name != "Manager Updated" {
			t.Errorf("name = %q, want %q", updated.Task.Name, "Manager Updated")
		}
	})

	t.Run("manager cannot delete", func(t *testing.T) {
		story := createTestTask(t, env, "Manager Cant Delete")

		err := env.svc.Delete(ctx, story.Task.ID, managerID)
		if err == nil {
			t.Fatal("expected forbidden error for manager on Delete")
		}
	})
}
