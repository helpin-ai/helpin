package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

// storyTestEnv bundles the service, DB, and common IDs used across story tests.
type storyTestEnv struct {
	svc    *PMStoryService
	db     *gorm.DB
	wsID   string
	userID string
	wfID   string
	stTodo string // default "unstarted" state
}

// newStoryTestEnv creates a fresh test environment for PMStoryService tests:
// workspace, user, workspace member (admin role), workflow with three states.
func newStoryTestEnv(t *testing.T) storyTestEnv {
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
	db.Exec(`CREATE TABLE IF NOT EXISTS pm_story_links (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		source_story_id TEXT NOT NULL,
		target_story_id TEXT NOT NULL,
		link_type TEXT NOT NULL,
		created_by TEXT,
		created_at DATETIME,
		updated_at DATETIME
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

	storyRepo := repository.NewPMStoryRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)
	activityService := NewPMActivityService(activityRepo)

	// wsPublisher is nil-safe (Publish is a no-op on nil receiver).
	svc := NewPMStoryService(storyRepo, workspaceRepo, workflowRepo, labelRepo, activityService, nil, nil, nil, nil)

	return storyTestEnv{
		svc:    svc,
		db:     db,
		wsID:   wsID,
		userID: userID,
		wfID:   wfID,
		stTodo: stTodo,
	}
}

// createTestStory is a helper that creates a story with minimal fields.
func createTestStory(t *testing.T, env storyTestEnv, name string) *model.StoryDetail {
	t.Helper()
	story, err := env.svc.Create(context.Background(), model.CreateStoryRequest{
		WorkspaceID:     env.wsID,
		Name:            name,
		WorkflowID:      env.wfID,
		WorkflowStateID: env.stTodo,
	}, env.userID)
	if err != nil {
		t.Fatalf("createTestStory(%q): %v", name, err)
	}
	return story
}

// ---------------------------------------------------------------------------
// 1. Create story
// ---------------------------------------------------------------------------

func TestPMStoryService_Create(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	t.Run("basic create", func(t *testing.T) {
		story, err := env.svc.Create(ctx, model.CreateStoryRequest{
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
		if story.Story.Name != "My Story" {
			t.Errorf("name = %q, want %q", story.Story.Name, "My Story")
		}
		if story.Story.WorkspaceID != env.wsID {
			t.Errorf("workspace_id = %q, want %q", story.Story.WorkspaceID, env.wsID)
		}
		if story.Story.ID == "" {
			t.Error("expected non-empty ID")
		}
		if story.Story.DisplayID < 1 {
			t.Errorf("display_id = %d, want >= 1", story.Story.DisplayID)
		}
		if story.Story.WorkflowStateID != env.stTodo {
			t.Errorf("workflow_state_id = %q, want %q", story.Story.WorkflowStateID, env.stTodo)
		}
		if story.Story.StoryType != model.PMStoryTypeFeature {
			t.Errorf("story_type = %q, want %q", story.Story.StoryType, model.PMStoryTypeFeature)
		}
		if story.Story.Priority != model.PMStoryPriorityNone {
			t.Errorf("priority = %q, want %q", story.Story.Priority, model.PMStoryPriorityNone)
		}
		if story.Story.Severity != model.PMStorySeverityNone {
			t.Errorf("severity = %q, want %q", story.Story.Severity, model.PMStorySeverityNone)
		}
		if story.Story.Archived {
			t.Error("expected archived = false")
		}
	})

	t.Run("create with explicit type bug", func(t *testing.T) {
		story, err := env.svc.Create(ctx, model.CreateStoryRequest{
			WorkspaceID:     env.wsID,
			Name:            "Bug Report",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
			StoryType:       model.PMStoryTypeBug,
		}, env.userID)
		if err != nil {
			t.Fatalf("Create bug: %v", err)
		}
		if story.Story.StoryType != model.PMStoryTypeBug {
			t.Errorf("story_type = %q, want %q", story.Story.StoryType, model.PMStoryTypeBug)
		}
	})

	t.Run("create with priority and severity", func(t *testing.T) {
		prio := model.PMStoryPriorityHigh
		sev := model.PMStorySeverityMajor
		story, err := env.svc.Create(ctx, model.CreateStoryRequest{
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
		if story.Story.Priority != model.PMStoryPriorityHigh {
			t.Errorf("priority = %q, want %q", story.Story.Priority, model.PMStoryPriorityHigh)
		}
		if story.Story.Severity != model.PMStorySeverityMajor {
			t.Errorf("severity = %q, want %q", story.Story.Severity, model.PMStorySeverityMajor)
		}
	})

	t.Run("display_id increments per workspace", func(t *testing.T) {
		s1 := createTestStory(t, env, "First")
		s2 := createTestStory(t, env, "Second")
		if s2.Story.DisplayID != s1.Story.DisplayID+1 {
			t.Errorf("display_id: first=%d, second=%d; expected consecutive", s1.Story.DisplayID, s2.Story.DisplayID)
		}
	})

	t.Run("create defaults workflow and state if omitted", func(t *testing.T) {
		// When workflowID is empty, Create should use GetDefaultWorkflow / SeedDefaultWorkflow.
		// Our seeded workflow is already the default for the workspace.
		story, err := env.svc.Create(ctx, model.CreateStoryRequest{
			WorkspaceID: env.wsID,
			Name:        "Auto Workflow Story",
		}, env.userID)
		if err != nil {
			t.Fatalf("Create without workflow: %v", err)
		}
		if story.Story.WorkflowID == "" {
			t.Error("expected non-empty workflow_id")
		}
		if story.Story.WorkflowStateID == "" {
			t.Error("expected non-empty workflow_state_id")
		}
	})
}

// ---------------------------------------------------------------------------
// 2. Create story validation
// ---------------------------------------------------------------------------

func TestPMStoryService_CreateValidation(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	t.Run("missing name", func(t *testing.T) {
		_, err := env.svc.Create(ctx, model.CreateStoryRequest{
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
		_, err := env.svc.Create(ctx, model.CreateStoryRequest{
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
		_, err := env.svc.Create(ctx, model.CreateStoryRequest{
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
		_, err := env.svc.Create(ctx, model.CreateStoryRequest{
			WorkspaceID:     env.wsID,
			Name:            "Bad Type",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
			StoryType:       "invalid_type",
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for invalid story_type")
		}
	})

	t.Run("invalid priority", func(t *testing.T) {
		badPrio := "super_urgent"
		_, err := env.svc.Create(ctx, model.CreateStoryRequest{
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
		_, err := env.svc.Create(ctx, model.CreateStoryRequest{
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
		_, err := env.svc.Create(ctx, model.CreateStoryRequest{
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

		_, err := env.svc.Create(ctx, model.CreateStoryRequest{
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

func TestPMStoryService_List(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	createTestStory(t, env, "Story Alpha")
	createTestStory(t, env, "Story Beta")

	stories, total, err := env.svc.List(ctx, env.wsID, model.PMStoryFilters{}, model.PMPagination{Page: 1, PerPage: 50})
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

func TestPMStoryService_ListEmptyWorkspace(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	stories, total, err := env.svc.List(ctx, env.wsID, model.PMStoryFilters{}, model.PMPagination{Page: 1, PerPage: 50})
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

func TestPMStoryService_ListRequiresWorkspaceID(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	_, _, err := env.svc.List(ctx, "", model.PMStoryFilters{}, model.PMPagination{})
	if err == nil {
		t.Fatal("expected error for empty workspace_id")
	}
}

// ---------------------------------------------------------------------------
// 4. GetByID
// ---------------------------------------------------------------------------

func TestPMStoryService_GetByID(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	created := createTestStory(t, env, "Get Me")

	got, err := env.svc.GetByID(ctx, created.Story.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil story")
	}
	if got.Story.ID != created.Story.ID {
		t.Errorf("id = %q, want %q", got.Story.ID, created.Story.ID)
	}
	if got.Story.Name != "Get Me" {
		t.Errorf("name = %q, want %q", got.Story.Name, "Get Me")
	}
	if got.Story.WorkspaceID != env.wsID {
		t.Errorf("workspace_id = %q, want %q", got.Story.WorkspaceID, env.wsID)
	}
	if got.Story.DisplayID != created.Story.DisplayID {
		t.Errorf("display_id = %d, want %d", got.Story.DisplayID, created.Story.DisplayID)
	}
}

func TestPMStoryService_GetByDisplayID(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	created := createTestStory(t, env, "Display ID Story")

	got, err := env.svc.GetByDisplayID(ctx, env.wsID, created.Story.DisplayID)
	if err != nil {
		t.Fatalf("GetByDisplayID: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil story")
	}
	if got.Story.ID != created.Story.ID {
		t.Errorf("id = %q, want %q", got.Story.ID, created.Story.ID)
	}
}

// ---------------------------------------------------------------------------
// 5. GetByID not found
// ---------------------------------------------------------------------------

func TestPMStoryService_GetByIDNotFound(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	_, err := env.svc.GetByID(ctx, "nonexistent-story-id")
	if err == nil {
		t.Fatal("expected error for unknown story ID")
	}
}

func TestPMStoryService_GetByDisplayIDNotFound(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	_, err := env.svc.GetByDisplayID(ctx, env.wsID, 99999)
	if err == nil {
		t.Fatal("expected error for unknown display ID")
	}
}

// ---------------------------------------------------------------------------
// 6. Update story
// ---------------------------------------------------------------------------

func TestPMStoryService_Update(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	created := createTestStory(t, env, "Original Name")

	t.Run("update name", func(t *testing.T) {
		newName := "Updated Name"
		updated, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			Name: &newName,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update name: %v", err)
		}
		if updated.Story.Name != "Updated Name" {
			t.Errorf("name = %q, want %q", updated.Story.Name, "Updated Name")
		}
	})

	t.Run("update priority", func(t *testing.T) {
		prio := model.PMStoryPriorityUrgent
		updated, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			Priority: &prio,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update priority: %v", err)
		}
		if updated.Story.Priority != model.PMStoryPriorityUrgent {
			t.Errorf("priority = %q, want %q", updated.Story.Priority, model.PMStoryPriorityUrgent)
		}
	})

	t.Run("update severity", func(t *testing.T) {
		sev := model.PMStorySeverityCritical
		updated, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			Severity: &sev,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update severity: %v", err)
		}
		if updated.Story.Severity != model.PMStorySeverityCritical {
			t.Errorf("severity = %q, want %q", updated.Story.Severity, model.PMStorySeverityCritical)
		}
	})

	t.Run("update story_type", func(t *testing.T) {
		st := model.PMStoryTypeChore
		updated, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			StoryType: &st,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update story_type: %v", err)
		}
		if updated.Story.StoryType != model.PMStoryTypeChore {
			t.Errorf("story_type = %q, want %q", updated.Story.StoryType, model.PMStoryTypeChore)
		}
	})

	t.Run("update description", func(t *testing.T) {
		desc := "Updated description text"
		updated, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			Description: &desc,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update description: %v", err)
		}
		if updated.Story.Description == nil || *updated.Story.Description != desc {
			t.Errorf("description mismatch")
		}
	})

	t.Run("update blocked", func(t *testing.T) {
		blocked := true
		updated, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			Blocked: &blocked,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update blocked: %v", err)
		}
		if !updated.Story.Blocked {
			t.Error("expected blocked = true")
		}

		// Unblock.
		unblocked := false
		updated2, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			Blocked: &unblocked,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update unblocked: %v", err)
		}
		if updated2.Story.Blocked {
			t.Error("expected blocked = false")
		}
	})

	t.Run("update blocker sets blocked", func(t *testing.T) {
		blocker := "waiting on backend API"
		updated, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			Blocker: &blocker,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update blocker: %v", err)
		}
		if !updated.Story.Blocked {
			t.Error("expected blocked = true when blocker is set")
		}
		if updated.Story.Blocker == nil || *updated.Story.Blocker != blocker {
			t.Errorf("blocker mismatch")
		}
	})

	t.Run("empty name rejected", func(t *testing.T) {
		emptyName := ""
		_, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			Name: &emptyName,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("invalid priority rejected", func(t *testing.T) {
		badPrio := "extreme"
		_, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			Priority: &badPrio,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for invalid priority")
		}
	})

	t.Run("invalid severity rejected", func(t *testing.T) {
		badSev := "apocalyptic"
		_, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			Severity: &badSev,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for invalid severity")
		}
	})

	t.Run("invalid story_type rejected", func(t *testing.T) {
		badType := "epic_story"
		_, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			StoryType: &badType,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for invalid story_type")
		}
	})

	t.Run("update nonexistent story", func(t *testing.T) {
		newName := "Nope"
		_, err := env.svc.Update(ctx, "nonexistent-story-id", model.UpdateStoryRequest{
			Name: &newName,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for nonexistent story")
		}
	})

	t.Run("forbidden for member role", func(t *testing.T) {
		memberID := "user-member-upd-001"
		seedUser(t, env.db, memberID, "member-upd@test.com", "Member User", "hash")
		seedWorkspaceMember(t, env.db, "wm-member-upd-001", env.wsID, memberID, "member-upd@test.com", "Member User", "member")

		newName := "Forbidden Update"
		_, err := env.svc.Update(ctx, created.Story.ID, model.UpdateStoryRequest{
			Name: &newName,
		}, memberID)
		if err == nil {
			t.Fatal("expected forbidden error for member role")
		}
	})
}

// ---------------------------------------------------------------------------
// 7. Update story state (via Update and MoveToState)
// ---------------------------------------------------------------------------

func TestPMStoryService_UpdateState(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	stInProgress := "state-inprogress-001"
	stDone := "state-done-001"

	t.Run("move to in_progress via Update", func(t *testing.T) {
		story := createTestStory(t, env, "State Change Story")

		updated, err := env.svc.Update(ctx, story.Story.ID, model.UpdateStoryRequest{
			WorkflowStateID: &stInProgress,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update state: %v", err)
		}
		if updated.Story.WorkflowStateID != stInProgress {
			t.Errorf("workflow_state_id = %q, want %q", updated.Story.WorkflowStateID, stInProgress)
		}
		// Verify raw row to check started/completed flags.
		var raw model.PMStory
		if err := env.db.Where("id = ?", story.Story.ID).First(&raw).Error; err != nil {
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
		story := createTestStory(t, env, "Done Story")

		updated, err := env.svc.Update(ctx, story.Story.ID, model.UpdateStoryRequest{
			WorkflowStateID: &stDone,
		}, env.userID)
		if err != nil {
			t.Fatalf("Update state to done: %v", err)
		}
		if updated.Story.WorkflowStateID != stDone {
			t.Errorf("workflow_state_id = %q, want %q", updated.Story.WorkflowStateID, stDone)
		}
		var raw model.PMStory
		if err := env.db.Where("id = ?", story.Story.ID).First(&raw).Error; err != nil {
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
		story := createTestStory(t, env, "Back To Todo")

		// First move to done.
		_, err := env.svc.Update(ctx, story.Story.ID, model.UpdateStoryRequest{
			WorkflowStateID: &stDone,
		}, env.userID)
		if err != nil {
			t.Fatalf("Move to done: %v", err)
		}

		// Move back to todo.
		todoState := env.stTodo
		_, err = env.svc.Update(ctx, story.Story.ID, model.UpdateStoryRequest{
			WorkflowStateID: &todoState,
		}, env.userID)
		if err != nil {
			t.Fatalf("Move back to todo: %v", err)
		}

		var raw model.PMStory
		if err := env.db.Where("id = ?", story.Story.ID).First(&raw).Error; err != nil {
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
		story := createTestStory(t, env, "Bad State Move")
		badState := "nonexistent-state-id"
		_, err := env.svc.Update(ctx, story.Story.ID, model.UpdateStoryRequest{
			WorkflowStateID: &badState,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for state not in workflow")
		}
	})
}

func TestPMStoryService_MoveToState(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	stInProgress := "state-inprogress-001"
	stDone := "state-done-001"

	t.Run("move to in_progress", func(t *testing.T) {
		story := createTestStory(t, env, "Move Via MoveToState")

		moved, err := env.svc.MoveToState(ctx, story.Story.ID, model.MoveStoryRequest{
			StateID: stInProgress,
		}, env.userID)
		if err != nil {
			t.Fatalf("MoveToState: %v", err)
		}
		if moved.Story.WorkflowStateID != stInProgress {
			t.Errorf("workflow_state_id = %q, want %q", moved.Story.WorkflowStateID, stInProgress)
		}
	})

	t.Run("move to done", func(t *testing.T) {
		story := createTestStory(t, env, "Move To Done")

		moved, err := env.svc.MoveToState(ctx, story.Story.ID, model.MoveStoryRequest{
			StateID: stDone,
		}, env.userID)
		if err != nil {
			t.Fatalf("MoveToState: %v", err)
		}
		if moved.Story.WorkflowStateID != stDone {
			t.Errorf("workflow_state_id = %q, want %q", moved.Story.WorkflowStateID, stDone)
		}

		var raw model.PMStory
		if err := env.db.Where("id = ?", story.Story.ID).First(&raw).Error; err != nil {
			t.Fatalf("raw query: %v", err)
		}
		if !raw.Completed {
			t.Error("expected completed = true")
		}
	})

	t.Run("move with position", func(t *testing.T) {
		story := createTestStory(t, env, "Move With Position")
		pos := 42

		moved, err := env.svc.MoveToState(ctx, story.Story.ID, model.MoveStoryRequest{
			StateID:  stInProgress,
			Position: &pos,
		}, env.userID)
		if err != nil {
			t.Fatalf("MoveToState with position: %v", err)
		}
		if moved.Story.WorkflowStateID != stInProgress {
			t.Errorf("workflow_state_id = %q, want %q", moved.Story.WorkflowStateID, stInProgress)
		}
		var raw model.PMStory
		if err := env.db.Where("id = ?", story.Story.ID).First(&raw).Error; err != nil {
			t.Fatalf("raw query: %v", err)
		}
		if raw.Position != 42 {
			t.Errorf("position = %d, want 42", raw.Position)
		}
	})

	t.Run("empty state_id rejected", func(t *testing.T) {
		story := createTestStory(t, env, "No State ID")

		_, err := env.svc.MoveToState(ctx, story.Story.ID, model.MoveStoryRequest{
			StateID: "",
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for empty state_id")
		}
	})

	t.Run("state not in workflow rejected", func(t *testing.T) {
		story := createTestStory(t, env, "Bad State")

		_, err := env.svc.MoveToState(ctx, story.Story.ID, model.MoveStoryRequest{
			StateID: "nonexistent-state",
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for state not in workflow")
		}
	})

	t.Run("nonexistent story", func(t *testing.T) {
		_, err := env.svc.MoveToState(ctx, "nonexistent-story", model.MoveStoryRequest{
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

func TestPMStoryService_Delete(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	t.Run("delete archives story", func(t *testing.T) {
		story := createTestStory(t, env, "To Be Deleted")

		err := env.svc.Delete(ctx, story.Story.ID, env.userID)
		if err != nil {
			t.Fatalf("Delete: %v", err)
		}

		// Verify archived flag is set.
		var raw model.PMStory
		if err := env.db.Where("id = ?", story.Story.ID).First(&raw).Error; err != nil {
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
		story := createTestStory(t, env, "Admin Only Delete")

		managerID := "user-mgr-del-001"
		seedUser(t, env.db, managerID, "mgr-del@test.com", "Manager Del", "hash")
		seedWorkspaceMember(t, env.db, "wm-mgr-del-001", env.wsID, managerID, "mgr-del@test.com", "Manager Del", "manager")

		err := env.svc.Delete(ctx, story.Story.ID, managerID)
		if err == nil {
			t.Fatal("expected forbidden error for manager role on Delete")
		}
	})

	t.Run("delete allowed for admin", func(t *testing.T) {
		story := createTestStory(t, env, "Admin Can Delete")

		err := env.svc.Delete(ctx, story.Story.ID, env.userID)
		if err != nil {
			t.Fatalf("Delete by admin: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 9. Reorder story
// ---------------------------------------------------------------------------

func TestPMStoryService_Reorder(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	t.Run("reorder changes position", func(t *testing.T) {
		story := createTestStory(t, env, "Reorder Me")

		err := env.svc.Reorder(ctx, story.Story.ID, model.ReorderStoryRequest{
			Position: 10,
		}, env.userID)
		if err != nil {
			t.Fatalf("Reorder: %v", err)
		}

		var raw model.PMStory
		if err := env.db.Where("id = ?", story.Story.ID).First(&raw).Error; err != nil {
			t.Fatalf("raw query: %v", err)
		}
		if raw.Position != 10 {
			t.Errorf("position = %d, want 10", raw.Position)
		}
	})

	t.Run("negative position rejected", func(t *testing.T) {
		story := createTestStory(t, env, "Negative Pos")

		err := env.svc.Reorder(ctx, story.Story.ID, model.ReorderStoryRequest{
			Position: -1,
		}, env.userID)
		if err == nil {
			t.Fatal("expected error for negative position")
		}
	})

	t.Run("nonexistent story", func(t *testing.T) {
		err := env.svc.Reorder(ctx, "nonexistent-story", model.ReorderStoryRequest{
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

func TestPMStoryService_Owners(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	t.Run("add and remove owner", func(t *testing.T) {
		story := createTestStory(t, env, "Owner Test")

		// Add the actor user as owner.
		err := env.svc.AddOwner(ctx, story.Story.ID, env.userID, env.userID)
		if err != nil {
			t.Fatalf("AddOwner: %v", err)
		}

		// Verify owner exists in pivot table.
		var count int64
		env.db.Table("pm_story_owners").Where("story_id = ? AND user_id = ?", story.Story.ID, env.userID).Count(&count)
		if count != 1 {
			t.Errorf("owner count = %d, want 1", count)
		}

		// Also verify auto-follow.
		var followerCount int64
		env.db.Table("pm_story_followers").Where("story_id = ? AND user_id = ?", story.Story.ID, env.userID).Count(&followerCount)
		if followerCount < 1 {
			t.Error("expected user to be auto-followed when added as owner")
		}

		// Remove owner.
		err = env.svc.RemoveOwner(ctx, story.Story.ID, env.userID, env.userID)
		if err != nil {
			t.Fatalf("RemoveOwner: %v", err)
		}

		env.db.Table("pm_story_owners").Where("story_id = ? AND user_id = ?", story.Story.ID, env.userID).Count(&count)
		if count != 0 {
			t.Errorf("owner count after remove = %d, want 0", count)
		}
	})

	t.Run("add owner requires user_id", func(t *testing.T) {
		story := createTestStory(t, env, "Empty Owner")

		err := env.svc.AddOwner(ctx, story.Story.ID, "", env.userID)
		if err == nil {
			t.Fatal("expected error for empty user_id")
		}
	})
}

// ---------------------------------------------------------------------------
// 11. Follower management
// ---------------------------------------------------------------------------

func TestPMStoryService_Followers(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	t.Run("add and remove follower", func(t *testing.T) {
		story := createTestStory(t, env, "Follower Test")

		err := env.svc.AddFollower(ctx, story.Story.ID, env.userID, env.userID)
		if err != nil {
			t.Fatalf("AddFollower: %v", err)
		}

		var count int64
		env.db.Table("pm_story_followers").Where("story_id = ? AND user_id = ?", story.Story.ID, env.userID).Count(&count)
		if count < 1 {
			t.Error("expected follower to be added")
		}

		err = env.svc.RemoveFollower(ctx, story.Story.ID, env.userID, env.userID)
		if err != nil {
			t.Fatalf("RemoveFollower: %v", err)
		}
	})

	t.Run("add follower requires user_id", func(t *testing.T) {
		story := createTestStory(t, env, "No Follower ID")

		err := env.svc.AddFollower(ctx, story.Story.ID, "", env.userID)
		if err == nil {
			t.Fatal("expected error for empty user_id")
		}
	})
}

// ---------------------------------------------------------------------------
// 12. Label management
// ---------------------------------------------------------------------------

func TestPMStoryService_Labels(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	// Seed a workspace-level label.
	now := time.Now()
	mustExec(t, env.db, `INSERT INTO pm_labels (id, workspace_id, name, color, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"label-001", env.wsID, "Bug", "#ff0000", now, now)

	t.Run("add and remove label", func(t *testing.T) {
		story := createTestStory(t, env, "Label Test")

		err := env.svc.AddLabel(ctx, story.Story.ID, "label-001", env.userID)
		if err != nil {
			t.Fatalf("AddLabel: %v", err)
		}

		var count int64
		env.db.Table("pm_story_labels").Where("story_id = ? AND label_id = ?", story.Story.ID, "label-001").Count(&count)
		if count != 1 {
			t.Errorf("label count = %d, want 1", count)
		}

		err = env.svc.RemoveLabel(ctx, story.Story.ID, "label-001", env.userID)
		if err != nil {
			t.Fatalf("RemoveLabel: %v", err)
		}

		env.db.Table("pm_story_labels").Where("story_id = ? AND label_id = ?", story.Story.ID, "label-001").Count(&count)
		if count != 0 {
			t.Errorf("label count after remove = %d, want 0", count)
		}
	})
}

// ---------------------------------------------------------------------------
// 13. Archive and retrieve
// ---------------------------------------------------------------------------

func TestPMStoryService_ArchiveViaUpdate(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	story := createTestStory(t, env, "Archive Me")

	archived := true
	updated, err := env.svc.Update(ctx, story.Story.ID, model.UpdateStoryRequest{
		Archived: &archived,
	}, env.userID)
	if err != nil {
		t.Fatalf("Update archived: %v", err)
	}
	if !updated.Story.Archived {
		t.Error("expected archived = true")
	}
}

// ---------------------------------------------------------------------------
// 14. Estimate field
// ---------------------------------------------------------------------------

func TestPMStoryService_Estimate(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	est := 5
	story, err := env.svc.Create(ctx, model.CreateStoryRequest{
		WorkspaceID:     env.wsID,
		Name:            "Estimated Story",
		WorkflowID:      env.wfID,
		WorkflowStateID: env.stTodo,
		Estimate:        &est,
	}, env.userID)
	if err != nil {
		t.Fatalf("Create with estimate: %v", err)
	}
	if story.Story.Estimate == nil || *story.Story.Estimate != 5 {
		t.Errorf("estimate mismatch, got %v", story.Story.Estimate)
	}

	newEst := 13
	updated, err := env.svc.Update(ctx, story.Story.ID, model.UpdateStoryRequest{
		Estimate: &newEst,
	}, env.userID)
	if err != nil {
		t.Fatalf("Update estimate: %v", err)
	}
	if updated.Story.Estimate == nil || *updated.Story.Estimate != 13 {
		t.Errorf("updated estimate mismatch, got %v", updated.Story.Estimate)
	}
}

// ---------------------------------------------------------------------------
// 15. Manager role can create and update (but not delete)
// ---------------------------------------------------------------------------

func TestPMStoryService_ManagerPermissions(t *testing.T) {
	t.Parallel()
	env := newStoryTestEnv(t)
	ctx := context.Background()

	managerID := "user-mgr-001"
	seedUser(t, env.db, managerID, "manager@test.com", "Manager User", "hash")
	seedWorkspaceMember(t, env.db, "wm-mgr-001", env.wsID, managerID, "manager@test.com", "Manager User", "manager")

	t.Run("manager can create", func(t *testing.T) {
		story, err := env.svc.Create(ctx, model.CreateStoryRequest{
			WorkspaceID:     env.wsID,
			Name:            "Manager Created",
			WorkflowID:      env.wfID,
			WorkflowStateID: env.stTodo,
		}, managerID)
		if err != nil {
			t.Fatalf("manager Create: %v", err)
		}
		if story.Story.Name != "Manager Created" {
			t.Errorf("name = %q, want %q", story.Story.Name, "Manager Created")
		}
	})

	t.Run("manager can update", func(t *testing.T) {
		story := createTestStory(t, env, "Manager Updates")

		newName := "Manager Updated"
		updated, err := env.svc.Update(ctx, story.Story.ID, model.UpdateStoryRequest{
			Name: &newName,
		}, managerID)
		if err != nil {
			t.Fatalf("manager Update: %v", err)
		}
		if updated.Story.Name != "Manager Updated" {
			t.Errorf("name = %q, want %q", updated.Story.Name, "Manager Updated")
		}
	})

	t.Run("manager cannot delete", func(t *testing.T) {
		story := createTestStory(t, env, "Manager Cant Delete")

		err := env.svc.Delete(ctx, story.Story.ID, managerID)
		if err == nil {
			t.Fatal("expected forbidden error for manager on Delete")
		}
	})
}
