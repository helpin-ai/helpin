package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// newEpicTestEnv sets up a test environment for PMEpicService tests:
// workspace, user, workspace member (admin role), and all required repos/services.
func newEpicTestEnv(t *testing.T) (svc *PMEpicService, wsID, userID string) {
	t.Helper()
	db := newTestDB(t)

	wsID = "ws-epic-001"
	userID = "user-epic-001"
	memberID := "member-epic-001"

	seedUser(t, db, userID, "epicadmin@test.com", "Epic Admin", "hash")
	seedWorkspace(t, db, wsID, "Epic Workspace", "epic-ws", userID)
	seedWorkspaceMember(t, db, memberID, wsID, userID, "epicadmin@test.com", "Epic Admin", model.RoleAdmin)

	epicRepo := repository.NewPMEpicRepository(db)
	storyRepo := repository.NewPMStoryRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	gitRepo := repository.NewGitRepositoryRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)
	activityService := NewPMActivityService(activityRepo)

	svc = NewPMEpicService(epicRepo, storyRepo, labelRepo, gitRepo, workspaceRepo, activityService, nil, nil)
	return svc, wsID, userID
}

// helper to create a basic epic for reuse across tests.
func createTestEpic(t *testing.T, svc *PMEpicService, wsID, userID, name string) *model.EpicWithStats {
	t.Helper()
	epic, err := svc.Create(context.Background(), model.CreateEpicRequest{
		WorkspaceID: wsID,
		Name:        name,
	}, userID)
	if err != nil {
		t.Fatalf("createTestEpic(%q): %v", name, err)
	}
	return epic
}

func TestPMEpicService_Create(t *testing.T) {
	t.Parallel()
	svc, wsID, userID := newEpicTestEnv(t)
	ctx := context.Background()

	t.Run("basic create", func(t *testing.T) {
		epic, err := svc.Create(ctx, model.CreateEpicRequest{
			WorkspaceID: wsID,
			Name:        "My Epic",
		}, userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if epic == nil {
			t.Fatal("expected non-nil epic")
		}
		if epic.Epic.Name != "My Epic" {
			t.Errorf("name = %q, want %q", epic.Epic.Name, "My Epic")
		}
		if epic.Epic.WorkspaceID != wsID {
			t.Errorf("workspace_id = %q, want %q", epic.Epic.WorkspaceID, wsID)
		}
		if epic.Epic.ID == "" {
			t.Error("expected non-empty ID")
		}
		if epic.Epic.Health != model.PMEpicHealthOnTrack {
			t.Errorf("health = %q, want %q", epic.Epic.Health, model.PMEpicHealthOnTrack)
		}
		if epic.Epic.Archived {
			t.Error("expected archived = false")
		}
	})

	t.Run("create trims whitespace", func(t *testing.T) {
		epic, err := svc.Create(ctx, model.CreateEpicRequest{
			WorkspaceID: wsID,
			Name:        "  Padded Name  ",
		}, userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if epic.Epic.Name != "Padded Name" {
			t.Errorf("name = %q, want %q", epic.Epic.Name, "Padded Name")
		}
	})

	t.Run("create with description and color", func(t *testing.T) {
		desc := "Epic description"
		color := "#ff0000"
		epic, err := svc.Create(ctx, model.CreateEpicRequest{
			WorkspaceID: wsID,
			Name:        "Colored Epic",
			Description: &desc,
			Color:       &color,
		}, userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if epic.Epic.Description == nil || *epic.Epic.Description != desc {
			t.Errorf("description = %v, want %q", epic.Epic.Description, desc)
		}
		if epic.Epic.Color == nil || *epic.Epic.Color != color {
			t.Errorf("color = %v, want %q", epic.Epic.Color, color)
		}
	})

	t.Run("create with custom health", func(t *testing.T) {
		health := model.PMEpicHealthAtRisk
		epic, err := svc.Create(ctx, model.CreateEpicRequest{
			WorkspaceID: wsID,
			Name:        "At Risk Epic",
			Health:      &health,
		}, userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if epic.Epic.Health != model.PMEpicHealthAtRisk {
			t.Errorf("health = %q, want %q", epic.Epic.Health, model.PMEpicHealthAtRisk)
		}
	})

	t.Run("create with position", func(t *testing.T) {
		pos := 42
		epic, err := svc.Create(ctx, model.CreateEpicRequest{
			WorkspaceID: wsID,
			Name:        "Positioned Epic",
			Position:    &pos,
		}, userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if epic.Epic.Position != 42 {
			t.Errorf("position = %d, want 42", epic.Epic.Position)
		}
	})

	t.Run("create sets created_by", func(t *testing.T) {
		epic, err := svc.Create(ctx, model.CreateEpicRequest{
			WorkspaceID: wsID,
			Name:        "Created By Epic",
		}, userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if epic.Epic.CreatedBy == nil || *epic.Epic.CreatedBy != userID {
			t.Errorf("created_by = %v, want %q", epic.Epic.CreatedBy, userID)
		}
	})

	t.Run("error on missing workspace_id", func(t *testing.T) {
		_, err := svc.Create(ctx, model.CreateEpicRequest{
			Name: "No Workspace",
		}, userID)
		if err == nil {
			t.Fatal("expected error for missing workspace_id")
		}
	})

	t.Run("error on empty name", func(t *testing.T) {
		_, err := svc.Create(ctx, model.CreateEpicRequest{
			WorkspaceID: wsID,
			Name:        "   ",
		}, userID)
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("error on invalid health", func(t *testing.T) {
		badHealth := "terrible"
		_, err := svc.Create(ctx, model.CreateEpicRequest{
			WorkspaceID: wsID,
			Name:        "Bad Health Epic",
			Health:      &badHealth,
		}, userID)
		if err == nil {
			t.Fatal("expected error for invalid health value")
		}
	})
}

func TestPMEpicService_Create_Forbidden(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	wsID := "ws-epic-forbid"
	userID := "user-epic-viewer"
	memberID := "member-epic-viewer"

	seedUser(t, db, userID, "viewer@test.com", "Viewer", "hash")
	seedWorkspace(t, db, wsID, "Forbidden WS", "forbid-ws", "some-owner")
	seedWorkspaceMember(t, db, memberID, wsID, userID, "viewer@test.com", "Viewer", model.RoleMember)

	epicRepo := repository.NewPMEpicRepository(db)
	storyRepo := repository.NewPMStoryRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	gitRepo := repository.NewGitRepositoryRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)
	activityService := NewPMActivityService(activityRepo)
	svc := NewPMEpicService(epicRepo, storyRepo, labelRepo, gitRepo, workspaceRepo, activityService, nil, nil)

	_, err := svc.Create(context.Background(), model.CreateEpicRequest{
		WorkspaceID: wsID,
		Name:        "Should Fail",
	}, userID)
	if err == nil {
		t.Fatal("expected forbidden error for member role")
	}
	if _, ok := err.(*model.ErrForbidden); !ok {
		t.Fatalf("expected *model.ErrForbidden, got %T: %v", err, err)
	}
}

func TestPMEpicService_List(t *testing.T) {
	t.Parallel()
	svc, wsID, userID := newEpicTestEnv(t)
	ctx := context.Background()

	t.Run("empty list", func(t *testing.T) {
		epics, err := svc.List(ctx, wsID, model.PMEpicListFilters{})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(epics) != 0 {
			t.Errorf("len = %d, want 0", len(epics))
		}
	})

	// Create two epics
	createTestEpic(t, svc, wsID, userID, "Epic Alpha")
	createTestEpic(t, svc, wsID, userID, "Epic Beta")

	t.Run("returns all epics", func(t *testing.T) {
		epics, err := svc.List(ctx, wsID, model.PMEpicListFilters{})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(epics) != 2 {
			t.Errorf("len = %d, want 2", len(epics))
		}
	})

	t.Run("filter by archived false", func(t *testing.T) {
		notArchived := false
		epics, err := svc.List(ctx, wsID, model.PMEpicListFilters{Archived: &notArchived})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(epics) != 2 {
			t.Errorf("len = %d, want 2", len(epics))
		}
	})

	t.Run("error on empty workspace_id", func(t *testing.T) {
		_, err := svc.List(ctx, "", model.PMEpicListFilters{})
		if err == nil {
			t.Fatal("expected error for empty workspace_id")
		}
	})
}

func TestPMEpicService_GetByID(t *testing.T) {
	t.Parallel()
	svc, wsID, userID := newEpicTestEnv(t)
	ctx := context.Background()

	created := createTestEpic(t, svc, wsID, userID, "Findable Epic")

	t.Run("found", func(t *testing.T) {
		epic, err := svc.GetByID(ctx, created.Epic.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if epic == nil {
			t.Fatal("expected non-nil epic")
		}
		if epic.Epic.ID != created.Epic.ID {
			t.Errorf("id = %q, want %q", epic.Epic.ID, created.Epic.ID)
		}
		if epic.Epic.Name != "Findable Epic" {
			t.Errorf("name = %q, want %q", epic.Epic.Name, "Findable Epic")
		}
		if epic.SuggestedHealth == "" {
			t.Error("expected non-empty suggested_health")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetByID(ctx, "nonexistent-id")
		if err == nil {
			t.Fatal("expected error for nonexistent epic")
		}
	})
}

func TestPMEpicService_Update(t *testing.T) {
	t.Parallel()
	svc, wsID, userID := newEpicTestEnv(t)
	ctx := context.Background()

	created := createTestEpic(t, svc, wsID, userID, "Original Name")

	t.Run("update name", func(t *testing.T) {
		newName := "Updated Name"
		epic, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
			Name: &newName,
		}, userID)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if epic.Epic.Name != "Updated Name" {
			t.Errorf("name = %q, want %q", epic.Epic.Name, "Updated Name")
		}
	})

	t.Run("update description", func(t *testing.T) {
		desc := "New description"
		epic, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
			Description: &desc,
		}, userID)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if epic.Epic.Description == nil || *epic.Epic.Description != desc {
			t.Errorf("description = %v, want %q", epic.Epic.Description, desc)
		}
	})

	t.Run("update color", func(t *testing.T) {
		color := "#00ff00"
		epic, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
			Color: &color,
		}, userID)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if epic.Epic.Color == nil || *epic.Epic.Color != color {
			t.Errorf("color = %v, want %q", epic.Epic.Color, color)
		}
	})

	t.Run("update health", func(t *testing.T) {
		health := model.PMEpicHealthOffTrack
		epic, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
			Health: &health,
		}, userID)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if epic.Epic.Health != model.PMEpicHealthOffTrack {
			t.Errorf("health = %q, want %q", epic.Epic.Health, model.PMEpicHealthOffTrack)
		}
	})

	t.Run("update position", func(t *testing.T) {
		pos := 99
		epic, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
			Position: &pos,
		}, userID)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if epic.Epic.Position != 99 {
			t.Errorf("position = %d, want 99", epic.Epic.Position)
		}
	})

	t.Run("error on empty name", func(t *testing.T) {
		empty := "   "
		_, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
			Name: &empty,
		}, userID)
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("error on invalid health", func(t *testing.T) {
		bad := "invalid_health"
		_, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
			Health: &bad,
		}, userID)
		if err == nil {
			t.Fatal("expected error for invalid health value")
		}
	})

	t.Run("error on nonexistent epic", func(t *testing.T) {
		name := "whatever"
		_, err := svc.Update(ctx, "nonexistent-id", model.UpdateEpicRequest{
			Name: &name,
		}, userID)
		if err == nil {
			t.Fatal("expected error for nonexistent epic")
		}
	})
}

func TestPMEpicService_Delete(t *testing.T) {
	t.Parallel()
	svc, wsID, userID := newEpicTestEnv(t)
	ctx := context.Background()

	created := createTestEpic(t, svc, wsID, userID, "To Delete")

	t.Run("delete sets archived", func(t *testing.T) {
		err := svc.Delete(ctx, created.Epic.ID, userID)
		if err != nil {
			t.Fatalf("Delete: %v", err)
		}

		// After delete (archive), GetByID should still return it but archived
		epic, err := svc.GetByID(ctx, created.Epic.ID)
		if err != nil {
			t.Fatalf("GetByID after delete: %v", err)
		}
		if !epic.Epic.Archived {
			t.Error("expected archived = true after delete")
		}
	})

	t.Run("error on nonexistent epic", func(t *testing.T) {
		err := svc.Delete(ctx, "nonexistent-id", userID)
		if err == nil {
			t.Fatal("expected error for nonexistent epic")
		}
	})
}

func TestPMEpicService_Delete_Forbidden(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	wsID := "ws-epic-del-forbid"
	adminUserID := "user-del-admin"
	managerUserID := "user-del-manager"

	seedUser(t, db, adminUserID, "deladmin@test.com", "Admin", "hash")
	seedUser(t, db, managerUserID, "delmanager@test.com", "Manager", "hash")
	seedWorkspace(t, db, wsID, "Del WS", "del-ws", adminUserID)
	seedWorkspaceMember(t, db, "member-del-admin", wsID, adminUserID, "deladmin@test.com", "Admin", model.RoleAdmin)
	seedWorkspaceMember(t, db, "member-del-manager", wsID, managerUserID, "delmanager@test.com", "Manager", model.RoleManager)

	epicRepo := repository.NewPMEpicRepository(db)
	storyRepo := repository.NewPMStoryRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	gitRepo := repository.NewGitRepositoryRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)
	activityService := NewPMActivityService(activityRepo)
	svc := NewPMEpicService(epicRepo, storyRepo, labelRepo, gitRepo, workspaceRepo, activityService, nil, nil)

	// Manager can create (requireCanEdit) but cannot delete (requireAdmin)
	created := createTestEpic(t, svc, wsID, managerUserID, "Manager Epic")

	err := svc.Delete(context.Background(), created.Epic.ID, managerUserID)
	if err == nil {
		t.Fatal("expected forbidden error for manager role on delete")
	}
	if _, ok := err.(*model.ErrForbidden); !ok {
		t.Fatalf("expected *model.ErrForbidden, got %T: %v", err, err)
	}
}

func TestPMEpicService_Archive(t *testing.T) {
	t.Parallel()
	svc, wsID, userID := newEpicTestEnv(t)
	ctx := context.Background()

	created := createTestEpic(t, svc, wsID, userID, "To Archive")

	t.Run("archive via update", func(t *testing.T) {
		archived := true
		epic, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
			Archived: &archived,
		}, userID)
		if err != nil {
			t.Fatalf("Update(archived=true): %v", err)
		}
		if !epic.Epic.Archived {
			t.Error("expected archived = true")
		}
	})

	t.Run("unarchive via update", func(t *testing.T) {
		unarchived := false
		epic, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
			Archived: &unarchived,
		}, userID)
		if err != nil {
			t.Fatalf("Update(archived=false): %v", err)
		}
		if epic.Epic.Archived {
			t.Error("expected archived = false after unarchive")
		}
	})

	t.Run("archived epics filtered from list", func(t *testing.T) {
		archived := true
		_, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
			Archived: &archived,
		}, userID)
		if err != nil {
			t.Fatalf("Update(archived=true): %v", err)
		}

		notArchived := false
		epics, err := svc.List(ctx, wsID, model.PMEpicListFilters{Archived: &notArchived})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, e := range epics {
			if e.Epic.ID == created.Epic.ID {
				t.Error("archived epic should not appear in non-archived list")
			}
		}
	})
}

func TestPMEpicService_UpdateHealth(t *testing.T) {
	t.Parallel()
	svc, wsID, userID := newEpicTestEnv(t)
	ctx := context.Background()

	created := createTestEpic(t, svc, wsID, userID, "Health Epic")

	t.Run("update health to at_risk", func(t *testing.T) {
		comment := "falling behind"
		err := svc.UpdateHealth(ctx, created.Epic.ID, model.UpdateEpicHealthRequest{
			Health:  model.PMEpicHealthAtRisk,
			Comment: &comment,
		}, userID)
		if err != nil {
			t.Fatalf("UpdateHealth: %v", err)
		}

		epic, err := svc.GetByID(ctx, created.Epic.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if epic.Epic.Health != model.PMEpicHealthAtRisk {
			t.Errorf("health = %q, want %q", epic.Epic.Health, model.PMEpicHealthAtRisk)
		}
		if epic.Epic.HealthComment == nil || *epic.Epic.HealthComment != comment {
			t.Errorf("health_comment = %v, want %q", epic.Epic.HealthComment, comment)
		}
	})

	t.Run("error on invalid health", func(t *testing.T) {
		err := svc.UpdateHealth(ctx, created.Epic.ID, model.UpdateEpicHealthRequest{
			Health: "garbage",
		}, userID)
		if err == nil {
			t.Fatal("expected error for invalid health value")
		}
	})

	t.Run("error on nonexistent epic", func(t *testing.T) {
		err := svc.UpdateHealth(ctx, "nonexistent-id", model.UpdateEpicHealthRequest{
			Health: model.PMEpicHealthOnTrack,
		}, userID)
		if err == nil {
			t.Fatal("expected error for nonexistent epic")
		}
	})
}

func TestIsValidEpicHealth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "on_track", input: model.PMEpicHealthOnTrack, want: true},
		{name: "at_risk", input: model.PMEpicHealthAtRisk, want: true},
		{name: "off_track", input: model.PMEpicHealthOffTrack, want: true},
		{name: "empty", input: "", want: false},
		{name: "invalid", input: "critical", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isValidEpicHealth(tt.input)
			if got != tt.want {
				t.Errorf("isValidEpicHealth(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestComputeEpicSuggestedHealth(t *testing.T) {
	t.Parallel()

	t.Run("no dates returns on_track", func(t *testing.T) {
		epic := &model.EpicWithStats{
			Stats: model.PMEpicStats{StoryCount: 5, DoneStoryCount: 0},
		}
		result := computeEpicSuggestedHealth(epic)
		if result != model.PMEpicHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMEpicHealthOnTrack)
		}
	})

	t.Run("no stories returns on_track", func(t *testing.T) {
		epic := &model.EpicWithStats{
			Stats: model.PMEpicStats{StoryCount: 0},
		}
		result := computeEpicSuggestedHealth(epic)
		if result != model.PMEpicHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMEpicHealthOnTrack)
		}
	})
}
