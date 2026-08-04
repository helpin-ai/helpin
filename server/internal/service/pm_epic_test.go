package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

// newEpicTestEnv sets up a test environment for PMEpicService tests:
// workspace, user, workspace member (admin role), and all required repos/services.
func newEpicTestEnv(t *testing.T) (svc *PMEpicService, wsID, userID string) {
	t.Helper()
	svc, _, wsID, userID = newEpicTestEnvWithDB(t)
	return svc, wsID, userID
}

func newEpicTestEnvWithDB(t *testing.T) (svc *PMEpicService, db *gorm.DB, wsID, userID string) {
	t.Helper()
	db = newTestDB(t)

	wsID = "ws-epic-001"
	userID = "user-epic-001"
	memberID := "member-epic-001"

	seedUser(t, db, userID, "epicadmin@test.com", "Epic Admin", "hash")
	seedWorkspace(t, db, wsID, "Epic Workspace", "epic-ws", userID)
	seedWorkspaceMember(t, db, memberID, wsID, userID, "epicadmin@test.com", "Epic Admin", model.RoleAdmin)

	epicRepo := repository.NewPMEpicRepository(db)
	storyRepo := repository.NewPMTaskRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	gitRepo := repository.NewGitRepositoryRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)
	activityService := NewPMActivityService(activityRepo)

	svc = NewPMEpicService(epicRepo, storyRepo, labelRepo, gitRepo, repository.NewPMAttachmentRepository(db), workspaceRepo, activityService, nil, nil)
	return svc, db, wsID, userID
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
		if epic.Epic.Health != model.PMEpicHealthNone {
			t.Errorf("health = %q, want %q", epic.Epic.Health, model.PMEpicHealthNone)
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

	t.Run("create stores assigned agent", func(t *testing.T) {
		agentID := "agent-epic-planner-001"
		epic, err := svc.Create(ctx, model.CreateEpicRequest{
			WorkspaceID:     wsID,
			Name:            "Agent-backed Epic",
			AssignedAgentID: &agentID,
		}, userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if epic.Epic.AssignedAgentID == nil || *epic.Epic.AssignedAgentID != agentID {
			t.Fatalf("assigned_agent_id = %v, want %q", epic.Epic.AssignedAgentID, agentID)
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

	t.Run("create reassigns temporary attachment ids", func(t *testing.T) {
		svc, db, wsID, userID := newEpicTestEnvWithDB(t)
		seedTemporaryAttachment(t, db, "attachment-epic-1", wsID, wsID, userID)

		epic, err := svc.Create(ctx, model.CreateEpicRequest{
			WorkspaceID:   wsID,
			Name:          "Epic With Image",
			AttachmentIDs: []string{"attachment-epic-1"},
		}, userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		attachmentRepo := repository.NewPMAttachmentRepository(db)
		attachment, err := attachmentRepo.GetByID(ctx, "attachment-epic-1")
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if attachment == nil {
			t.Fatal("expected attachment")
		}
		if attachment.EntityType != "epic" {
			t.Fatalf("entity_type = %q, want %q", attachment.EntityType, "epic")
		}
		if attachment.EntityID != epic.Epic.ID {
			t.Fatalf("entity_id = %q, want %q", attachment.EntityID, epic.Epic.ID)
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

func TestPMEpicServiceCreateAndUpdateReturnSuggestedHealth(t *testing.T) {
	svc, wsID, userID := newEpicTestEnv(t)
	created, err := svc.Create(context.Background(), model.CreateEpicRequest{
		WorkspaceID: wsID,
		Name:        "Suggested Health Epic",
	}, userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.SuggestedHealth != model.PMEpicHealthNone {
		t.Fatalf("create suggested_health = %q, want %q", created.SuggestedHealth, model.PMEpicHealthNone)
	}

	name := "Suggested Health Epic Updated"
	updated, err := svc.Update(context.Background(), created.Epic.ID, model.UpdateEpicRequest{Name: &name}, userID)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.SuggestedHealth != model.PMEpicHealthNone {
		t.Fatalf("update suggested_health = %q, want %q", updated.SuggestedHealth, model.PMEpicHealthNone)
	}
}

func TestPMEpicServiceListTasksReturnsTableEnrichment(t *testing.T) {
	t.Parallel()
	svc, db, wsID, userID := newEpicTestEnvWithDB(t)
	ctx := context.Background()

	const (
		ownerUserID   = "user-epic-task-owner"
		ownerMemberID = "member-epic-task-owner"
		workflowID    = "workflow-epic-task-list"
		stateID       = "state-epic-task-list"
		taskID        = "task-epic-task-list"
		labelID       = "label-epic-task-list"
	)

	seedUser(t, db, ownerUserID, "owner-list@test.com", "Owner List", "hash")
	seedWorkspaceMember(t, db, ownerMemberID, wsID, ownerUserID, "owner-list@test.com", "Owner List", model.RoleMember)
	seedWorkflowForStoryTest(t, db, wsID, workflowID, stateID)
	epic := createTestEpic(t, svc, wsID, userID, "Enriched task list")

	task := model.PMTask{
		ID:              taskID,
		WorkspaceID:     wsID,
		DisplayID:       42,
		Name:            "Owned task",
		TaskType:        model.PMTaskTypeFeature,
		WorkflowID:      workflowID,
		WorkflowStateID: stateID,
		EpicID:          &epic.Epic.ID,
		Priority:        model.PMTaskPriorityMedium,
		Severity:        model.PMTaskSeverityNone,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := db.Create(&model.PMTaskOwner{TaskID: taskID, UserID: ownerUserID}).Error; err != nil {
		t.Fatalf("create task owner: %v", err)
	}
	label := model.PMLabel{
		ID:          labelID,
		WorkspaceID: wsID,
		Name:        "Important",
		Color:       stringPtr("#2563eb"),
	}
	if err := db.Create(&label).Error; err != nil {
		t.Fatalf("create label: %v", err)
	}
	if err := db.Create(&model.PMTaskLabel{TaskID: taskID, LabelID: labelID}).Error; err != nil {
		t.Fatalf("create task label: %v", err)
	}

	tasks, err := svc.ListTasks(ctx, epic.Epic.ID)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks len = %d, want 1", len(tasks))
	}
	if got := tasks[0].OwnerMemberIDs; len(got) != 1 || got[0] != ownerMemberID {
		t.Fatalf("OwnerMemberIDs = %#v, want [%s]", got, ownerMemberID)
	}
	if len(tasks[0].Labels) != 1 || tasks[0].Labels[0].ID != labelID {
		t.Fatalf("Labels = %#v, want label %s", tasks[0].Labels, labelID)
	}
	if tasks[0].StateName == nil || *tasks[0].StateName != "Backlog" {
		t.Fatalf("StateName = %v, want Backlog", tasks[0].StateName)
	}
	if tasks[0].TaskKey == "" {
		t.Fatal("TaskKey is empty")
	}
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
	storyRepo := repository.NewPMTaskRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	gitRepo := repository.NewGitRepositoryRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)
	activityService := NewPMActivityService(activityRepo)
	svc := NewPMEpicService(epicRepo, storyRepo, labelRepo, gitRepo, repository.NewPMAttachmentRepository(db), workspaceRepo, activityService, nil, nil)

	// Inject a member actor (not a team owner) — members cannot create epics
	ctx := authorization.WithActor(context.Background(), &authorization.Actor{
		UserID:            userID,
		WorkspaceID:       wsID,
		WorkspaceMemberID: memberID,
		Role:              model.RoleMember,
	})
	_, err := svc.Create(ctx, model.CreateEpicRequest{
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

	t.Run("update assigned agent", func(t *testing.T) {
		agentID := "agent-epic-planner-002"
		epic, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
			AssignedAgentID: &agentID,
		}, userID)
		if err != nil {
			t.Fatalf("Update assigned agent: %v", err)
		}
		if epic.Epic.AssignedAgentID == nil || *epic.Epic.AssignedAgentID != agentID {
			t.Fatalf("assigned_agent_id = %v, want %q", epic.Epic.AssignedAgentID, agentID)
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

func TestPMEpicServiceUpdateClearsExplicitlySuppliedNullableFields(t *testing.T) {
	t.Parallel()
	svc, db, wsID, userID := newEpicTestEnvWithDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	teamID := "team-epic-clear"
	stateID := "state-epic-clear"
	repoID := "repo-epic-clear"
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		teamID, wsID, "Clear Team", now, now)
	mustExec(t, db, `INSERT INTO pm_epic_workflow_states (id, workspace_id, name, state_type, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		stateID, wsID, "To Do", model.PMStateTypeUnstarted, 0, now, now)
	if err := db.Create(&model.GitRepository{
		ID: repoID, WorkspaceID: wsID, IntegrationID: "integration-clear", Provider: "github",
		ExternalID: "repo-clear", FullName: "helpin/clear", Permissions: json.RawMessage(`{}`),
		Active: true, Selected: true,
	}).Error; err != nil {
		t.Fatalf("create git repository: %v", err)
	}

	start := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	created, err := svc.Create(ctx, model.CreateEpicRequest{
		WorkspaceID:          wsID,
		Name:                 "Clearable Epic",
		TeamID:               &teamID,
		EpicStateID:          &stateID,
		OwnerMemberID:        stringPtr("member-epic-001"),
		PlannedStartDate:     &start,
		Deadline:             &deadline,
		PlanningRepositoryID: &repoID,
	}, userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
		EpicStateIDSet:          true,
		OwnerSet:                true,
		TeamIDSet:               true,
		PlannedStartDateSet:     true,
		DeadlineSet:             true,
		PlanningRepositoryIDSet: true,
	}, userID)
	if err != nil {
		t.Fatalf("Update clears: %v", err)
	}
	if updated.Epic.EpicStateID != nil || updated.Epic.OwnerID != nil || updated.Epic.OwnerMemberID != nil ||
		updated.Epic.TeamID != nil || updated.Epic.PlannedStartDate != nil || updated.Epic.Deadline != nil ||
		updated.Epic.PlanningRepositoryID != nil {
		t.Fatalf("nullable fields were not cleared: %#v", updated.Epic)
	}
}

func TestPMEpicServiceUpdateRejectsDestinationTeamOutsideActorScope(t *testing.T) {
	t.Parallel()
	svc, db, wsID, adminUserID := newEpicTestEnvWithDB(t)
	now := time.Now().UTC()
	teamA := "team-epic-destination-a"
	teamB := "team-epic-destination-b"
	memberUserID := "user-epic-destination-member"
	memberID := "member-epic-destination-member"

	seedUser(t, db, memberUserID, "destination@test.com", "Destination Member", "hash")
	seedWorkspaceMember(t, db, memberID, wsID, memberUserID, "destination@test.com", "Destination Member", model.RoleMember)
	for _, team := range []struct{ id, name string }{{teamA, "Team A"}, {teamB, "Team B"}} {
		mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, team.id, wsID, team.name, now, now)
	}
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"membership-epic-destination", teamA, memberID, "member", now, now)

	created, err := svc.Create(context.Background(), model.CreateEpicRequest{WorkspaceID: wsID, Name: "Scoped Epic", TeamID: &teamA}, adminUserID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	memberCtx := authorization.WithActor(context.Background(), &authorization.Actor{
		UserID: memberUserID, WorkspaceID: wsID, WorkspaceMemberID: memberID, Role: model.RoleMember,
		TeamMemberships: []authorization.TeamRole{{TeamID: teamA, Role: "member"}},
	})

	_, err = svc.Update(memberCtx, created.Epic.ID, model.UpdateEpicRequest{TeamID: &teamB}, memberUserID)
	if err == nil {
		t.Fatal("expected update to an inaccessible destination team to fail")
	}
	reloaded, loadErr := svc.GetByID(context.Background(), created.Epic.ID)
	if loadErr != nil {
		t.Fatalf("GetByID: %v", loadErr)
	}
	if reloaded.Epic.TeamID == nil || *reloaded.Epic.TeamID != teamA {
		t.Fatalf("team_id = %v, want %s", reloaded.Epic.TeamID, teamA)
	}
}

func TestPMEpicServiceUpdateRejectsTeamMoveIncompatibleWithAssignedAgent(t *testing.T) {
	svc, db, wsID, userID := newEpicTestEnvWithDB(t)
	now := time.Now().UTC()
	teamA := "team-epic-agent-a"
	teamB := "team-epic-agent-b"
	agentID := "agent-epic-team-a"
	for _, team := range []struct{ id, name string }{{teamA, "Agent Team A"}, {teamB, "Agent Team B"}} {
		mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, team.id, wsID, team.name, now, now)
	}
	seedTeamScopedEpicAgent(t, db, svc, wsID, agentID, teamA)
	created, err := svc.Create(context.Background(), model.CreateEpicRequest{
		WorkspaceID: wsID, Name: "Agent Scoped Epic", TeamID: &teamA, AssignedAgentID: &agentID,
	}, userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svc.Update(context.Background(), created.Epic.ID, model.UpdateEpicRequest{TeamID: &teamB}, userID)
	if err == nil || !strings.Contains(err.Error(), "restricted to team") {
		t.Fatalf("team move error = %v, want assigned-agent team restriction", err)
	}
	reloaded, err := svc.GetByID(context.Background(), created.Epic.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if reloaded.Epic.TeamID == nil || *reloaded.Epic.TeamID != teamA || reloaded.Epic.AssignedAgentID == nil || *reloaded.Epic.AssignedAgentID != agentID {
		t.Fatalf("epic mutated after rejected team move: %#v", reloaded.Epic)
	}
}

func seedTeamScopedEpicAgent(t *testing.T, db *gorm.DB, svc *PMEpicService, workspaceID, agentID, teamID string) {
	t.Helper()
	mustExec(t, db, `CREATE TABLE agents (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, is_system BOOLEAN NOT NULL DEFAULT 0,
		name TEXT NOT NULL, preset_key TEXT, preset_version_key TEXT, status TEXT,
		runtime_kind TEXT, skills TEXT, allowed_tools TEXT, allowed_commands TEXT,
		allowed_targets TEXT, approval_mode TEXT, default_invocation_mode TEXT, team_id TEXT
	)`)
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, preset_version_key, status,
		runtime_kind, skills, allowed_tools, allowed_commands, allowed_targets,
		approval_mode, default_invocation_mode, team_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agentID, workspaceID, true, "Team A Epic Planner", model.AgentPresetEpicPlanner, "epic_planner_default", "idle",
		"native_sdk", `[]`, []byte(`[]`), []byte(`[]`), []byte(`["epic"]`), "never", "autonomous", teamID)
	svc.SetAgentService(&AgentService{agentRepo: repository.NewAgentRepository(db)})
}

func TestPMEpicServiceCreateValidatesLabelsBeforePersisting(t *testing.T) {
	t.Parallel()
	svc, db, wsID, userID := newEpicTestEnvWithDB(t)
	foreignWorkspaceID := "ws-epic-label-foreign"
	seedWorkspace(t, db, foreignWorkspaceID, "Foreign", "foreign-epic-label", userID)
	if err := db.Create(&model.PMLabel{ID: "label-epic-foreign", WorkspaceID: foreignWorkspaceID, Name: "Foreign"}).Error; err != nil {
		t.Fatalf("create foreign label: %v", err)
	}

	_, err := svc.Create(context.Background(), model.CreateEpicRequest{
		WorkspaceID: wsID,
		Name:        "Must Not Persist",
		LabelIDs:    []string{"label-epic-foreign"},
	}, userID)
	if err == nil {
		t.Fatal("expected foreign label validation error")
	}
	var count int64
	if err := db.Model(&model.PMEpic{}).Where("workspace_id = ? AND name = ?", wsID, "Must Not Persist").Count(&count).Error; err != nil {
		t.Fatalf("count epics: %v", err)
	}
	if count != 0 {
		t.Fatalf("persisted epics = %d, want 0", count)
	}
}

func TestPMEpicServiceCreateRollsBackWhenLabelInsertFails(t *testing.T) {
	svc, db, wsID, userID := newEpicTestEnvWithDB(t)
	ctx := context.Background()
	labelID := "label-epic-create-failure"
	attachmentID := "attachment-epic-create-failure"
	if err := db.Create(&model.PMLabel{ID: labelID, WorkspaceID: wsID, Name: "Fails on insert"}).Error; err != nil {
		t.Fatalf("create label: %v", err)
	}
	seedTemporaryAttachment(t, db, attachmentID, wsID, wsID, userID)
	mustExec(t, db, `CREATE TRIGGER fail_epic_label_create
		BEFORE INSERT ON pm_epic_labels
		WHEN NEW.label_id = 'label-epic-create-failure'
		BEGIN
			SELECT RAISE(FAIL, 'forced epic label insert failure');
		END`)

	_, err := svc.Create(ctx, model.CreateEpicRequest{
		WorkspaceID:   wsID,
		Name:          "Rolled Back Epic",
		LabelIDs:      []string{labelID},
		AttachmentIDs: []string{attachmentID},
	}, userID)
	if err == nil {
		t.Fatal("expected label insert failure")
	}

	var epicCount int64
	if err := db.Model(&model.PMEpic{}).
		Where("workspace_id = ? AND name = ?", wsID, "Rolled Back Epic").
		Count(&epicCount).Error; err != nil {
		t.Fatalf("count epics: %v", err)
	}
	if epicCount != 0 {
		t.Fatalf("persisted epics = %d, want 0", epicCount)
	}
	attachment, err := repository.NewPMAttachmentRepository(db).GetByID(ctx, attachmentID)
	if err != nil {
		t.Fatalf("get attachment: %v", err)
	}
	if attachment == nil || attachment.EntityType != "temporary" || attachment.EntityID != wsID {
		t.Fatalf("attachment was reassigned before epic commit: %#v", attachment)
	}
}

func TestPMEpicServiceUpdateRollsBackScalarAndLabelsWhenLabelInsertFails(t *testing.T) {
	svc, db, wsID, userID := newEpicTestEnvWithDB(t)
	ctx := context.Background()
	originalLabelID := "label-epic-update-original"
	failingLabelID := "label-epic-update-failure"
	for _, label := range []model.PMLabel{
		{ID: originalLabelID, WorkspaceID: wsID, Name: "Original"},
		{ID: failingLabelID, WorkspaceID: wsID, Name: "Fails on insert"},
	} {
		if err := db.Create(&label).Error; err != nil {
			t.Fatalf("create label %s: %v", label.ID, err)
		}
	}
	created, err := svc.Create(ctx, model.CreateEpicRequest{
		WorkspaceID: wsID,
		Name:        "Original Epic Name",
		LabelIDs:    []string{originalLabelID},
	}, userID)
	if err != nil {
		t.Fatalf("create epic: %v", err)
	}
	mustExec(t, db, `CREATE TRIGGER fail_epic_label_update
		BEFORE INSERT ON pm_epic_labels
		WHEN NEW.label_id = 'label-epic-update-failure'
		BEGIN
			SELECT RAISE(FAIL, 'forced epic label insert failure');
		END`)

	updatedName := "Must Roll Back"
	_, err = svc.Update(ctx, created.Epic.ID, model.UpdateEpicRequest{
		Name:     &updatedName,
		LabelIDs: []string{failingLabelID},
	}, userID)
	if err == nil {
		t.Fatal("expected label insert failure")
	}

	var epic model.PMEpic
	if err := db.Where("id = ?", created.Epic.ID).First(&epic).Error; err != nil {
		t.Fatalf("reload epic: %v", err)
	}
	if epic.Name != "Original Epic Name" {
		t.Fatalf("epic name = %q, want original value", epic.Name)
	}
	var links []model.PMEpicLabel
	if err := db.Where("epic_id = ?", created.Epic.ID).Find(&links).Error; err != nil {
		t.Fatalf("reload epic labels: %v", err)
	}
	if len(links) != 1 || links[0].LabelID != originalLabelID {
		t.Fatalf("epic labels = %#v, want only original label", links)
	}
}

func TestPMEpicServiceCreateDeduplicatesLabelIDs(t *testing.T) {
	svc, db, wsID, userID := newEpicTestEnvWithDB(t)
	labelID := "label-epic-duplicate"
	if err := db.Create(&model.PMLabel{ID: labelID, WorkspaceID: wsID, Name: "Duplicate"}).Error; err != nil {
		t.Fatalf("create label: %v", err)
	}

	created, err := svc.Create(context.Background(), model.CreateEpicRequest{
		WorkspaceID: wsID,
		Name:        "Deduplicated Labels",
		LabelIDs:    []string{labelID, " " + labelID + " ", labelID},
	}, userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	var linkCount int64
	if err := db.Model(&model.PMEpicLabel{}).Where("epic_id = ?", created.Epic.ID).Count(&linkCount).Error; err != nil {
		t.Fatalf("count epic labels: %v", err)
	}
	if linkCount != 1 {
		t.Fatalf("epic label links = %d, want 1", linkCount)
	}
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

func TestPMEpicService_Delete_TeamMemberAccess(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)

	wsID := "ws-epic-del-team"
	adminUserID := "user-del-team-admin"
	memberUserID := "user-del-team-member"
	otherUserID := "user-del-team-other"
	memberID := "member-del-team-member"
	otherMemberID := "member-del-team-other"
	teamID := "team-del-alpha"
	otherTeamID := "team-del-beta"

	seedUser(t, db, adminUserID, "delteamadmin@test.com", "Admin", "hash")
	seedUser(t, db, memberUserID, "delteammember@test.com", "Member", "hash")
	seedUser(t, db, otherUserID, "delteamother@test.com", "Other", "hash")
	seedWorkspace(t, db, wsID, "Del Team WS", "del-team-ws", adminUserID)
	seedWorkspaceMember(t, db, "member-del-team-admin", wsID, adminUserID, "delteamadmin@test.com", "Admin", model.RoleAdmin)
	seedWorkspaceMember(t, db, memberID, wsID, memberUserID, "delteammember@test.com", "Member", model.RoleMember)
	seedWorkspaceMember(t, db, otherMemberID, wsID, otherUserID, "delteamother@test.com", "Other", model.RoleMember)
	now := time.Now()
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		teamID, wsID, "Alpha", now, now)
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		otherTeamID, wsID, "Beta", now, now)
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"twm-del-team-member", teamID, memberID, "member", now, now)
	mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"twm-del-team-other", otherTeamID, otherMemberID, "member", now, now)

	epicRepo := repository.NewPMEpicRepository(db)
	storyRepo := repository.NewPMTaskRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	gitRepo := repository.NewGitRepositoryRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)
	activityService := NewPMActivityService(activityRepo)
	svc := NewPMEpicService(epicRepo, storyRepo, labelRepo, gitRepo, repository.NewPMAttachmentRepository(db), workspaceRepo, activityService, nil, nil)

	memberCtx := authorization.WithActor(context.Background(), &authorization.Actor{
		UserID:            memberUserID,
		WorkspaceID:       wsID,
		WorkspaceMemberID: memberID,
		Role:              model.RoleMember,
		TeamMemberships:   []authorization.TeamRole{{TeamID: teamID, Role: "member"}},
	})
	otherCtx := authorization.WithActor(context.Background(), &authorization.Actor{
		UserID:            otherUserID,
		WorkspaceID:       wsID,
		WorkspaceMemberID: otherMemberID,
		Role:              model.RoleMember,
		TeamMemberships:   []authorization.TeamRole{{TeamID: otherTeamID, Role: "member"}},
	})

	created, err := svc.Create(memberCtx, model.CreateEpicRequest{
		WorkspaceID: wsID,
		Name:        "Member Team Epic",
		TeamID:      &teamID,
	}, memberUserID)
	if err != nil {
		t.Fatalf("Create member team epic: %v", err)
	}

	if err := svc.Delete(memberCtx, created.Epic.ID, memberUserID); err != nil {
		t.Fatalf("Delete by member of epic team: %v", err)
	}

	otherEpic, err := svc.Create(memberCtx, model.CreateEpicRequest{
		WorkspaceID: wsID,
		Name:        "Other Member Team Epic",
		TeamID:      &teamID,
	}, memberUserID)
	if err != nil {
		t.Fatalf("Create other member team epic: %v", err)
	}

	err = svc.Delete(otherCtx, otherEpic.Epic.ID, otherUserID)
	if err == nil {
		t.Fatal("expected forbidden error for member outside epic team")
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
		{name: "no_health", input: model.PMEpicHealthNone, want: true},
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

	makeEpic := func(start, end time.Time, storyCount, doneCount int) *model.EpicWithStats {
		return &model.EpicWithStats{
			Epic: model.PMEpic{
				PlannedStartDate: &start,
				Deadline:         &end,
			},
			Stats: model.PMEpicStats{
				TaskCount:     storyCount,
				DoneTaskCount: doneCount,
			},
		}
	}

	t.Run("no dates returns no_health", func(t *testing.T) {
		epic := &model.EpicWithStats{
			Stats: model.PMEpicStats{TaskCount: 5, DoneTaskCount: 0},
		}
		result := computeEpicSuggestedHealthAt(epic, time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC))
		if result != model.PMEpicHealthNone {
			t.Errorf("got %q, want %q", result, model.PMEpicHealthNone)
		}
	})

	t.Run("no stories returns no_health", func(t *testing.T) {
		epic := &model.EpicWithStats{
			Stats: model.PMEpicStats{TaskCount: 0},
		}
		result := computeEpicSuggestedHealthAt(epic, time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC))
		if result != model.PMEpicHealthNone {
			t.Errorf("got %q, want %q", result, model.PMEpicHealthNone)
		}
	})

	t.Run("before start returns no_health", func(t *testing.T) {
		start := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 27, 0, 0, 0, 0, time.UTC)
		epic := makeEpic(start, end, 4, 0)
		result := computeEpicSuggestedHealthAt(epic, time.Date(2026, time.March, 13, 12, 0, 0, 0, time.UTC))
		if result != model.PMEpicHealthNone {
			t.Errorf("got %q, want %q", result, model.PMEpicHealthNone)
		}
	})

	t.Run("start day with no progress remains on_track", func(t *testing.T) {
		start := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC)
		epic := makeEpic(start, end, 2, 0)
		result := computeEpicSuggestedHealthAt(epic, time.Date(2026, time.March, 13, 18, 0, 0, 0, time.UTC))
		if result != model.PMEpicHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMEpicHealthOnTrack)
		}
	})

	t.Run("deadline day is not automatically overdue", func(t *testing.T) {
		start := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC)
		epic := makeEpic(start, end, 2, 1)
		result := computeEpicSuggestedHealthAt(epic, time.Date(2026, time.March, 14, 9, 0, 0, 0, time.UTC))
		if result != model.PMEpicHealthOnTrack {
			t.Errorf("got %q, want %q", result, model.PMEpicHealthOnTrack)
		}
	})

	t.Run("after deadline with incomplete work returns off_track", func(t *testing.T) {
		start := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 12, 0, 0, 0, 0, time.UTC)
		epic := makeEpic(start, end, 4, 3)
		result := computeEpicSuggestedHealthAt(epic, time.Date(2026, time.March, 13, 8, 0, 0, 0, time.UTC))
		if result != model.PMEpicHealthOffTrack {
			t.Errorf("got %q, want %q", result, model.PMEpicHealthOffTrack)
		}
	})

	t.Run("gap thresholds map to on_track at_risk and off_track", func(t *testing.T) {
		start := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, time.March, 19, 0, 0, 0, 0, time.UTC)
		now := time.Date(2026, time.March, 15, 12, 0, 0, 0, time.UTC)

		onTrack := makeEpic(start, end, 10, 4) // expected 50%, actual 40%, gap 10
		if result := computeEpicSuggestedHealthAt(onTrack, now); result != model.PMEpicHealthOnTrack {
			t.Errorf("on_track got %q, want %q", result, model.PMEpicHealthOnTrack)
		}

		atRisk := makeEpic(start, end, 10, 3) // expected 50%, actual 30%, gap 20
		if result := computeEpicSuggestedHealthAt(atRisk, now); result != model.PMEpicHealthAtRisk {
			t.Errorf("at_risk got %q, want %q", result, model.PMEpicHealthAtRisk)
		}

		offTrack := makeEpic(start, end, 10, 2) // expected 50%, actual 20%, gap 30
		if result := computeEpicSuggestedHealthAt(offTrack, now); result != model.PMEpicHealthOffTrack {
			t.Errorf("off_track got %q, want %q", result, model.PMEpicHealthOffTrack)
		}
	})
}
