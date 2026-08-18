package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPMSprintService_GetCloseout_IncludesCloseoutAndInboundRollover(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	ensurePMSprintCloseoutTables(t, db)

	const (
		workspaceID    = "ws-sprint-closeout"
		workspaceSlug  = "sprint-closeout"
		ownerID        = "owner-sprint-closeout"
		ownerMemberID  = "wm-sprint-closeout"
		teamID         = "team-sprint-closeout"
		sourceSprintID = "sprint-source"
		targetSprintID = "sprint-target"
		closeoutID     = "closeout-source"
	)

	seedWorkspace(t, db, workspaceID, "Sprint Closeout WS", workspaceSlug, ownerID)
	seedWorkspaceMember(t, db, ownerMemberID, workspaceID, ownerID, "owner@example.com", "Owner", model.RoleOwner)
	seedSprintAutomationTeam(t, db, teamID, workspaceID, "Growth")
	seedSprintAutomationSprint(t, db, sourceSprintID, workspaceID, teamID, "Sprint 12", time.Now().UTC().AddDate(0, 0, -14), time.Now().UTC().AddDate(0, 0, -7))
	seedSprintAutomationSprint(t, db, targetSprintID, workspaceID, teamID, "Sprint 13", time.Now().UTC().AddDate(0, 0, -1), time.Now().UTC().AddDate(0, 0, 7))
	teamIDValue := teamID
	targetSprintIDValue := targetSprintID

	closeoutRepo := repository.NewPMSprintCloseoutRepository(db)
	if _, err := closeoutRepo.CreateCloseout(context.Background(), &model.PMSprintCloseout{
		ID:               closeoutID,
		SprintID:         sourceSprintID,
		WorkspaceID:      workspaceID,
		TeamID:           &teamIDValue,
		RolledToSprintID: &targetSprintIDValue,
		CommittedCount:   5,
		CompletedCount:   3,
		UnfinishedCount:  2,
		RolledOverCount:  2,
		CommittedPoints:  13,
		CompletedPoints:  8,
		UnfinishedPoints: 5,
		RolledOverPoints: 5,
		ClosedAt:         time.Now().UTC().AddDate(0, 0, -7),
	}, []model.PMSprintCloseoutTask{
		{TaskID: "task-complete", Outcome: model.PMSprintCloseoutOutcomeCompleted, Estimate: 3},
		{TaskID: "task-rollover", Outcome: model.PMSprintCloseoutOutcomeRolledOver, Estimate: 5},
	}); err != nil {
		t.Fatalf("CreateCloseout: %v", err)
	}

	svc := NewPMSprintService(
		repository.NewPMSprintRepository(db),
		repository.NewPMTaskRepository(db),
		repository.NewPMLabelRepository(db),
		repository.NewPMAttachmentRepository(db),
		repository.NewWorkspaceRepository(db),
		repository.NewSettingsRepository(db),
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
		closeoutRepo,
	)

	ctx := authorization.WithActor(context.Background(), &authorization.Actor{
		UserID:            ownerID,
		WorkspaceID:       workspaceID,
		WorkspaceMemberID: ownerMemberID,
		Role:              model.RoleOwner,
		TeamMemberships:   []authorization.TeamRole{{TeamID: teamID, Role: "owner"}},
	})

	sourceResponse, err := svc.GetCloseout(ctx, sourceSprintID)
	if err != nil {
		t.Fatalf("GetCloseout(source): %v", err)
	}
	if sourceResponse.Closeout == nil {
		t.Fatal("expected source sprint closeout")
	}
	if sourceResponse.Closeout.ID != closeoutID {
		t.Fatalf("closeout.id = %q, want %q", sourceResponse.Closeout.ID, closeoutID)
	}

	targetResponse, err := svc.GetCloseout(ctx, targetSprintID)
	if err != nil {
		t.Fatalf("GetCloseout(target): %v", err)
	}
	if targetResponse.Closeout != nil {
		t.Fatal("expected target sprint to have no direct closeout")
	}
	if len(targetResponse.RolledInFrom) != 1 {
		t.Fatalf("rolled_in_from len = %d, want 1", len(targetResponse.RolledInFrom))
	}
	if targetResponse.RolledInFrom[0].SourceSprintID != sourceSprintID {
		t.Fatalf("rolled_in_from[0].source_sprint_id = %q, want %q", targetResponse.RolledInFrom[0].SourceSprintID, sourceSprintID)
	}
}

func TestPMSprintService_ListCloseouts_FiltersByAccessibleTeams(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	ensurePMSprintCloseoutTables(t, db)

	const (
		workspaceID   = "ws-sprint-closeout-list"
		workspaceSlug = "sprint-closeout-list"
		adminUserID   = "admin-closeout-list"
		adminMemberID = "wm-admin-closeout-list"
		memberUserID  = "member-closeout-list"
		memberID      = "wm-member-closeout-list"
		teamAID       = "team-closeout-a"
		teamBID       = "team-closeout-b"
	)

	seedWorkspace(t, db, workspaceID, "Sprint Closeout List", workspaceSlug, adminUserID)
	seedWorkspaceMember(t, db, adminMemberID, workspaceID, adminUserID, "admin@example.com", "Admin", model.RoleAdmin)
	seedWorkspaceMember(t, db, memberID, workspaceID, memberUserID, "member@example.com", "Member", model.RoleMember)
	seedSprintAutomationTeam(t, db, teamAID, workspaceID, "Alpha")
	seedSprintAutomationTeam(t, db, teamBID, workspaceID, "Beta")
	seedSprintAutomationSprint(t, db, "sprint-alpha", workspaceID, teamAID, "Sprint Alpha", time.Now().UTC().AddDate(0, 0, -10), time.Now().UTC().AddDate(0, 0, -3))
	seedSprintAutomationSprint(t, db, "sprint-beta", workspaceID, teamBID, "Sprint Beta", time.Now().UTC().AddDate(0, 0, -20), time.Now().UTC().AddDate(0, 0, -13))
	teamAIDValue := teamAID
	teamBIDValue := teamBID

	closeoutRepo := repository.NewPMSprintCloseoutRepository(db)
	if _, err := closeoutRepo.CreateCloseout(context.Background(), &model.PMSprintCloseout{
		ID:               "closeout-alpha",
		SprintID:         "sprint-alpha",
		WorkspaceID:      workspaceID,
		TeamID:           &teamAIDValue,
		CommittedCount:   4,
		CompletedCount:   3,
		UnfinishedCount:  1,
		CommittedPoints:  9,
		CompletedPoints:  7,
		UnfinishedPoints: 2,
		ClosedAt:         time.Now().UTC().AddDate(0, 0, -3),
	}, nil); err != nil {
		t.Fatalf("CreateCloseout(alpha): %v", err)
	}
	if _, err := closeoutRepo.CreateCloseout(context.Background(), &model.PMSprintCloseout{
		ID:               "closeout-beta",
		SprintID:         "sprint-beta",
		WorkspaceID:      workspaceID,
		TeamID:           &teamBIDValue,
		CommittedCount:   6,
		CompletedCount:   2,
		UnfinishedCount:  4,
		CommittedPoints:  12,
		CompletedPoints:  4,
		UnfinishedPoints: 8,
		ClosedAt:         time.Now().UTC().AddDate(0, 0, -13),
	}, nil); err != nil {
		t.Fatalf("CreateCloseout(beta): %v", err)
	}

	svc := NewPMSprintService(
		repository.NewPMSprintRepository(db),
		repository.NewPMTaskRepository(db),
		repository.NewPMLabelRepository(db),
		repository.NewPMAttachmentRepository(db),
		repository.NewWorkspaceRepository(db),
		repository.NewSettingsRepository(db),
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
		closeoutRepo,
	)

	adminCtx := authorization.WithActor(context.Background(), &authorization.Actor{
		UserID:            adminUserID,
		WorkspaceID:       workspaceID,
		WorkspaceMemberID: adminMemberID,
		Role:              model.RoleAdmin,
	})
	memberCtx := authorization.WithActor(context.Background(), &authorization.Actor{
		UserID:            memberUserID,
		WorkspaceID:       workspaceID,
		WorkspaceMemberID: memberID,
		Role:              model.RoleMember,
		TeamMemberships:   []authorization.TeamRole{{TeamID: teamAID, Role: "member"}},
	})

	adminItems, err := svc.ListCloseouts(adminCtx, workspaceID, nil)
	if err != nil {
		t.Fatalf("ListCloseouts(admin): %v", err)
	}
	if len(adminItems) != 2 {
		t.Fatalf("admin closeouts len = %d, want 2", len(adminItems))
	}

	memberItems, err := svc.ListCloseouts(memberCtx, workspaceID, nil)
	if err != nil {
		t.Fatalf("ListCloseouts(member): %v", err)
	}
	if len(memberItems) != 1 {
		t.Fatalf("member closeouts len = %d, want 1", len(memberItems))
	}
	if memberItems[0].TeamID == nil || *memberItems[0].TeamID != teamAID {
		t.Fatalf("member closeout team_id = %v, want %q", memberItems[0].TeamID, teamAID)
	}
}
