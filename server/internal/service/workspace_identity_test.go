package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWorkspaceRepositoryPendingAndAssignableMembers(t *testing.T) {
	db := newWorkspaceIdentityTestDB(t)
	ctx := context.Background()

	userRepo := repository.NewUserRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)

	inviter := seedWorkspaceIdentityUser(t, db, "user-owner", "owner@example.com", "Owner User")
	memberUser := seedWorkspaceIdentityUser(t, db, "user-joined", "joined@example.com", "Joined User")
	seedWorkspaceIdentityWorkspace(t, db, "ws-1", inviter.ID)

	if _, err := workspaceRepo.AddMember(ctx, "ws-1", inviter.ID, model.RoleOwner); err != nil {
		t.Fatalf("add owner member: %v", err)
	}
	joined, err := workspaceRepo.AddMember(ctx, "ws-1", memberUser.ID, model.RoleMember)
	if err != nil {
		t.Fatalf("add joined member: %v", err)
	}

	pending, err := workspaceRepo.UpsertPendingMember(ctx, "ws-1", "pending@example.com", model.RoleMember, inviter.ID)
	if err != nil {
		t.Fatalf("upsert pending member: %v", err)
	}
	if pending.Status != model.WorkspaceMemberStatusPending {
		t.Fatalf("pending status = %s, want %s", pending.Status, model.WorkspaceMemberStatusPending)
	}

	assignable, err := workspaceRepo.ListAssignableMembers(ctx, "ws-1")
	if err != nil {
		t.Fatalf("list assignable members: %v", err)
	}
	if len(assignable) != 3 {
		t.Fatalf("expected 3 assignable members, got %d", len(assignable))
	}
	if assignable[0].Status != model.WorkspaceMemberStatusActive || assignable[2].Status != model.WorkspaceMemberStatusPending {
		t.Fatalf("unexpected assignable ordering/statuses: %#v", assignable)
	}

	activeOnly, err := workspaceRepo.ListMembers(ctx, "ws-1")
	if err != nil {
		t.Fatalf("list active members: %v", err)
	}
	if len(activeOnly) != 2 {
		t.Fatalf("expected 2 active joined members, got %d", len(activeOnly))
	}

	byUserID, err := workspaceRepo.ResolveMemberReference(ctx, "ws-1", memberUser.ID)
	if err != nil {
		t.Fatalf("resolve by user id: %v", err)
	}
	if byUserID == nil || byUserID.ID != joined.ID {
		t.Fatalf("resolve by user id returned %#v, want joined member %s", byUserID, joined.ID)
	}

	byMemberID, err := workspaceRepo.ResolveMemberReference(ctx, "ws-1", pending.ID)
	if err != nil {
		t.Fatalf("resolve by member id: %v", err)
	}
	if byMemberID == nil || byMemberID.Email != "pending@example.com" {
		t.Fatalf("resolve by member id returned %#v", byMemberID)
	}

	loadedUser, err := userRepo.GetByID(ctx, inviter.ID)
	if err != nil || loadedUser == nil {
		t.Fatalf("get seeded inviter: %v", err)
	}
}

func TestInviteServiceCreateAndAcceptInvitationUsesWorkspaceMemberIdentity(t *testing.T) {
	db := newWorkspaceIdentityTestDB(t)
	ctx := context.Background()

	workspaceRepo := repository.NewWorkspaceRepository(db)
	invitationRepo := repository.NewInvitationRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	inviter := seedWorkspaceIdentityUser(t, db, "user-owner", "owner@example.com", "Owner User")
	invitee := seedWorkspaceIdentityUser(t, db, "user-invitee", "pending@example.com", "Pending Invitee")
	seedWorkspaceIdentityWorkspace(t, db, "ws-1", inviter.ID)
	if _, err := workspaceRepo.AddMember(ctx, "ws-1", inviter.ID, model.RoleOwner); err != nil {
		t.Fatalf("add owner member: %v", err)
	}

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "https://app.example.com", nil)

	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: "ws-1",
		Email:       invitee.Email,
		Role:        model.RoleMember,
	}, inviter.ID)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	if resp.WorkspaceMemberID == nil || *resp.WorkspaceMemberID == "" {
		t.Fatal("expected invitation to be linked to a pending workspace member")
	}

	assignable, err := workspaceRepo.ListAssignableMembers(ctx, "ws-1")
	if err != nil {
		t.Fatalf("list assignable members: %v", err)
	}
	if len(assignable) != 2 {
		t.Fatalf("expected 2 assignable members after invite, got %d", len(assignable))
	}

	inv, err := invitationRepo.GetByID(ctx, resp.ID)
	if err != nil {
		t.Fatalf("get invitation: %v", err)
	}
	if inv == nil || inv.WorkspaceMemberID == nil || *inv.WorkspaceMemberID != *resp.WorkspaceMemberID {
		t.Fatalf("invitation member link mismatch: %#v", inv)
	}

	if err := svc.AcceptInvitation(ctx, inv.Token, invitee.ID); err != nil {
		t.Fatalf("accept invitation: %v", err)
	}

	member, err := workspaceRepo.GetMembership(ctx, "ws-1", invitee.ID)
	if err != nil {
		t.Fatalf("get accepted membership: %v", err)
	}
	if member == nil {
		t.Fatal("expected accepted workspace membership")
	}
	if member.ID != *resp.WorkspaceMemberID {
		t.Fatalf("accepted membership id = %s, want pending id %s", member.ID, *resp.WorkspaceMemberID)
	}
	if member.Status != model.WorkspaceMemberStatusActive {
		t.Fatalf("accepted membership status = %s, want active", member.Status)
	}

	acceptedInvitation, err := invitationRepo.GetByID(ctx, resp.ID)
	if err != nil {
		t.Fatalf("reload invitation: %v", err)
	}
	if acceptedInvitation.Status != "accepted" {
		t.Fatalf("invitation status = %s, want accepted", acceptedInvitation.Status)
	}

	if db.Migrator().HasTable("workspace_people") {
		t.Fatal("workspace_people table should not exist in the identity test schema")
	}
}

func TestPMStoryServiceCreateSupportsPendingOwnerMember(t *testing.T) {
	db := newWorkspaceIdentityTestDB(t)
	ctx := context.Background()

	workspaceRepo := repository.NewWorkspaceRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	storyRepo := repository.NewPMStoryRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)

	actor := seedWorkspaceIdentityUser(t, db, "user-actor", "actor@example.com", "Actor User")
	seedWorkspaceIdentityWorkspace(t, db, "ws-1", actor.ID)

	actorMember, err := workspaceRepo.AddMember(ctx, "ws-1", actor.ID, model.RoleOwner)
	if err != nil {
		t.Fatalf("add actor member: %v", err)
	}
	pendingOwner, err := workspaceRepo.UpsertPendingMember(ctx, "ws-1", "pending-owner@example.com", model.RoleMember, actor.ID)
	if err != nil {
		t.Fatalf("add pending owner member: %v", err)
	}

	seedWorkflowForStoryTest(t, db, "ws-1", "wf-1", "state-1")

	svc := NewPMStoryService(
		storyRepo,
		workspaceRepo,
		workflowRepo,
		repository.NewPMEpicRepository(db),
		repository.NewPMSprintRepository(db),
		nil,
		nil,
		nil,
		repository.NewPMAttachmentRepository(db),
		NewPMActivityService(activityRepo),
		nil,
		nil,
		nil,
		nil,
	)

	story, err := svc.Create(ctx, model.CreateStoryRequest{
		WorkspaceID:     "ws-1",
		Name:            "Pending assignee story",
		WorkflowID:      "wf-1",
		WorkflowStateID: "state-1",
		OwnerMemberID:   &pendingOwner.ID,
	}, actor.ID)
	if err != nil {
		t.Fatalf("create story with pending owner: %v", err)
	}

	if story.Story.OwnerMemberID == nil || *story.Story.OwnerMemberID != pendingOwner.ID {
		t.Fatalf("story owner_member_id = %#v, want %s", story.Story.OwnerMemberID, pendingOwner.ID)
	}
	if story.Story.OwnerID != nil {
		t.Fatalf("story owner_id = %#v, want nil for pending member", story.Story.OwnerID)
	}
	if story.OwnerMember == nil || story.OwnerMember.Email != "pending-owner@example.com" {
		t.Fatalf("story owner member = %#v", story.OwnerMember)
	}
	if story.Story.RequesterMemberID == nil || *story.Story.RequesterMemberID != actorMember.ID {
		t.Fatalf("story requester_member_id = %#v, want %s", story.Story.RequesterMemberID, actorMember.ID)
	}
	if story.Story.RequesterID == nil || *story.Story.RequesterID != actor.ID {
		t.Fatalf("story requester_id = %#v, want %s", story.Story.RequesterID, actor.ID)
	}

	var ownerLinks int64
	if err := db.Table("pm_story_owners").Where("story_id = ?", story.Story.ID).Count(&ownerLinks).Error; err != nil {
		t.Fatalf("count story owners: %v", err)
	}
	if ownerLinks != 0 {
		t.Fatalf("expected no legacy owner links for pending assignee, got %d", ownerLinks)
	}

	var followerLinks int64
	if err := db.Table("pm_story_followers").Where("story_id = ?", story.Story.ID).Count(&followerLinks).Error; err != nil {
		t.Fatalf("count story followers: %v", err)
	}
	if followerLinks != 1 {
		t.Fatalf("expected requester to auto-follow, got %d follower links", followerLinks)
	}
}

func TestPMStoryRepositoryDerivesShortcutStyleBlockingSemantics(t *testing.T) {
	db := newWorkspaceIdentityTestDB(t)
	ctx := context.Background()

	seedWorkspaceIdentityWorkspace(t, db, "ws-1", "user-owner")
	seedWorkflowForStoryTest(t, db, "ws-1", "wf-1", "state-backlog")

	doneState := model.PMWorkflowState{
		ID:         "state-done",
		WorkflowID: "wf-1",
		Name:       "Done",
		StateType:  model.PMStateTypeDone,
		Position:   1,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := db.Create(&doneState).Error; err != nil {
		t.Fatalf("seed done state: %v", err)
	}

	stories := []model.PMStory{
		{ID: "source-active", WorkspaceID: "ws-1", DisplayID: 1, Name: "Source active", StoryType: model.PMStoryTypeFeature, WorkflowID: "wf-1", WorkflowStateID: "state-backlog", Priority: model.PMStoryPriorityNone, Severity: model.PMStorySeverityNone, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		{ID: "target-active", WorkspaceID: "ws-1", DisplayID: 2, Name: "Target active", StoryType: model.PMStoryTypeFeature, WorkflowID: "wf-1", WorkflowStateID: "state-backlog", Priority: model.PMStoryPriorityNone, Severity: model.PMStorySeverityNone, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		{ID: "source-done", WorkspaceID: "ws-1", DisplayID: 3, Name: "Source done", StoryType: model.PMStoryTypeFeature, WorkflowID: "wf-1", WorkflowStateID: "state-done", Priority: model.PMStoryPriorityNone, Severity: model.PMStorySeverityNone, Completed: true, CompletedAt: timePtr(time.Now().UTC()), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		{ID: "target-cleared", WorkspaceID: "ws-1", DisplayID: 4, Name: "Target cleared", StoryType: model.PMStoryTypeFeature, WorkflowID: "wf-1", WorkflowStateID: "state-backlog", Priority: model.PMStoryPriorityNone, Severity: model.PMStorySeverityNone, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		{ID: "legacy-external", WorkspaceID: "ws-1", DisplayID: 5, Name: "Legacy external", StoryType: model.PMStoryTypeFeature, WorkflowID: "wf-1", WorkflowStateID: "state-backlog", Priority: model.PMStoryPriorityNone, Severity: model.PMStorySeverityNone, Blocked: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		{ID: "external-note", WorkspaceID: "ws-1", DisplayID: 6, Name: "External note", StoryType: model.PMStoryTypeFeature, WorkflowID: "wf-1", WorkflowStateID: "state-backlog", Priority: model.PMStoryPriorityNone, Severity: model.PMStorySeverityNone, Blocker: strPtr("Waiting on vendor"), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}
	for _, story := range stories {
		if err := db.Create(&story).Error; err != nil {
			t.Fatalf("seed story %s: %v", story.ID, err)
		}
	}

	links := []model.PMStoryLink{
		{ID: "link-1", WorkspaceID: "ws-1", SourceStoryID: "source-active", TargetStoryID: "target-active", LinkType: model.PMStoryLinkTypeBlocks, CreatedBy: "user-owner", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		{ID: "link-2", WorkspaceID: "ws-1", SourceStoryID: "source-done", TargetStoryID: "target-cleared", LinkType: model.PMStoryLinkTypeBlocks, CreatedBy: "user-owner", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}
	for _, link := range links {
		if err := db.Create(&link).Error; err != nil {
			t.Fatalf("seed story link %s: %v", link.ID, err)
		}
	}

	repo := repository.NewPMStoryRepository(db)

	active, err := repo.GetByID(ctx, "target-active")
	if err != nil {
		t.Fatalf("get active blocked story: %v", err)
	}
	if !active.Story.Blocked || !active.Story.IsBlockedByTask || active.Story.BlockedByCount != 1 {
		t.Fatalf("active dependency state mismatch: %#v", active.Story)
	}
	if len(active.Story.BlockedByTasks) != 1 || active.Story.BlockedByTasks[0].ID != "source-active" {
		t.Fatalf("expected active blocker to be visible in detail: %#v", active.Story.BlockedByTasks)
	}

	cleared, err := repo.GetByID(ctx, "target-cleared")
	if err != nil {
		t.Fatalf("get cleared blocked story: %v", err)
	}
	if cleared.Story.Blocked || cleared.Story.IsBlockedByTask || cleared.Story.BlockedByCount != 0 {
		t.Fatalf("completed blocker should not keep task blocked: %#v", cleared.Story)
	}
	if len(cleared.Story.BlockedByTasks) != 1 || !cleared.Story.BlockedByTasks[0].Completed {
		t.Fatalf("completed blocker should remain visible in detail: %#v", cleared.Story.BlockedByTasks)
	}

	legacy, err := repo.GetByID(ctx, "legacy-external")
	if err != nil {
		t.Fatalf("get legacy blocked story: %v", err)
	}
	if !legacy.Story.Blocked || legacy.Story.BlockedByCount != 0 {
		t.Fatalf("legacy blocked fallback mismatch: %#v", legacy.Story)
	}

	_, total, err := repo.List(ctx, "ws-1", model.PMStoryFilters{Blocked: strPtr("true")}, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("list blocked stories: %v", err)
	}
	if total != 3 {
		t.Fatalf("blocked story total = %d, want 3", total)
	}

	blockingStories, total, err := repo.List(ctx, "ws-1", model.PMStoryFilters{Blocking: strPtr("true")}, model.PMPagination{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("list blocking stories: %v", err)
	}
	if total != 2 {
		t.Fatalf("blocking story total = %d, want 2", total)
	}
	foundActiveBlocker := false
	for _, story := range blockingStories {
		if story.ID == "source-active" && story.IsBlockingOtherTask {
			foundActiveBlocker = true
			break
		}
	}
	if !foundActiveBlocker {
		t.Fatalf("expected source-active to be marked as blocking others: %#v", blockingStories)
	}
}

func TestPMEpicServiceCreateSupportsWorkspaceMemberOwners(t *testing.T) {
	db := newWorkspaceIdentityTestDB(t)
	createPMEpicObjectiveTables(t, db)
	ctx := context.Background()

	workspaceRepo := repository.NewWorkspaceRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)

	actor := seedWorkspaceIdentityUser(t, db, "user-epic-actor", "actor@example.com", "Actor User")
	joinedOwner := seedWorkspaceIdentityUser(t, db, "user-epic-joined", "joined@example.com", "Joined Owner")
	seedWorkspaceIdentityWorkspace(t, db, "ws-1", actor.ID)

	if _, err := workspaceRepo.AddMember(ctx, "ws-1", actor.ID, model.RoleOwner); err != nil {
		t.Fatalf("add actor member: %v", err)
	}
	joinedMember, err := workspaceRepo.AddMember(ctx, "ws-1", joinedOwner.ID, model.RoleMember)
	if err != nil {
		t.Fatalf("add joined member: %v", err)
	}
	pendingOwner, err := workspaceRepo.UpsertPendingMember(ctx, "ws-1", "pending-epic@example.com", model.RoleMember, actor.ID)
	if err != nil {
		t.Fatalf("add pending epic owner: %v", err)
	}

	svc := NewPMEpicService(
		epicRepo,
		nil,
		nil,
		repository.NewGitRepositoryRepository(db),
		repository.NewPMAttachmentRepository(db),
		workspaceRepo,
		NewPMActivityService(activityRepo),
		nil,
		nil,
	)

	epic, err := svc.Create(ctx, model.CreateEpicRequest{
		WorkspaceID:   "ws-1",
		Name:          "Epic with pending owner",
		OwnerMemberID: &pendingOwner.ID,
	}, actor.ID)
	if err != nil {
		t.Fatalf("create epic with pending owner: %v", err)
	}
	if epic.Epic.OwnerMemberID == nil || *epic.Epic.OwnerMemberID != pendingOwner.ID {
		t.Fatalf("epic owner_member_id = %#v, want %s", epic.Epic.OwnerMemberID, pendingOwner.ID)
	}
	if epic.Epic.OwnerID != nil {
		t.Fatalf("epic owner_id = %#v, want nil for pending owner", epic.Epic.OwnerID)
	}

	updated, err := svc.Update(ctx, epic.Epic.ID, model.UpdateEpicRequest{
		OwnerID: &joinedOwner.ID,
	}, actor.ID)
	if err != nil {
		t.Fatalf("update epic with joined owner via user id: %v", err)
	}
	if updated.Epic.OwnerMemberID == nil || *updated.Epic.OwnerMemberID != joinedMember.ID {
		t.Fatalf("updated epic owner_member_id = %#v, want %s", updated.Epic.OwnerMemberID, joinedMember.ID)
	}
	if updated.Epic.OwnerID == nil || *updated.Epic.OwnerID != joinedOwner.ID {
		t.Fatalf("updated epic owner_id = %#v, want %s", updated.Epic.OwnerID, joinedOwner.ID)
	}

	var stored model.PMEpic
	if err := db.Where("id = ?", epic.Epic.ID).First(&stored).Error; err != nil {
		t.Fatalf("load stored epic: %v", err)
	}
	if stored.OwnerMemberID == nil || *stored.OwnerMemberID != joinedMember.ID {
		t.Fatalf("stored epic owner_member_id = %#v, want %s", stored.OwnerMemberID, joinedMember.ID)
	}
	if stored.OwnerID == nil || *stored.OwnerID != joinedOwner.ID {
		t.Fatalf("stored epic owner_id = %#v, want %s", stored.OwnerID, joinedOwner.ID)
	}
}

func TestPMObjectiveServiceUsesWorkspaceMemberOwners(t *testing.T) {
	db := newWorkspaceIdentityTestDB(t)
	createPMEpicObjectiveTables(t, db)
	ctx := context.Background()

	workspaceRepo := repository.NewWorkspaceRepository(db)
	objectiveRepo := repository.NewPMObjectiveRepository(db)
	activityRepo := repository.NewPMActivityRepository(db)

	actor := seedWorkspaceIdentityUser(t, db, "user-obj-actor", "actor@example.com", "Actor User")
	joinedOwner := seedWorkspaceIdentityUser(t, db, "user-obj-joined", "joined@example.com", "Joined Owner")
	seedWorkspaceIdentityWorkspace(t, db, "ws-1", actor.ID)

	actorMember, err := workspaceRepo.AddMember(ctx, "ws-1", actor.ID, model.RoleOwner)
	if err != nil {
		t.Fatalf("add actor member: %v", err)
	}
	joinedMember, err := workspaceRepo.AddMember(ctx, "ws-1", joinedOwner.ID, model.RoleMember)
	if err != nil {
		t.Fatalf("add joined member: %v", err)
	}
	pendingOwner, err := workspaceRepo.UpsertPendingMember(ctx, "ws-1", "pending-objective@example.com", model.RoleMember, actor.ID)
	if err != nil {
		t.Fatalf("add pending objective owner: %v", err)
	}

	svc := NewPMObjectiveService(
		objectiveRepo,
		nil,
		nil,
		repository.NewPMAttachmentRepository(db),
		workspaceRepo,
		NewPMActivityService(activityRepo),
		nil,
		nil,
	)

	obj, err := svc.Create(ctx, model.CreateObjectiveRequest{
		WorkspaceID:    "ws-1",
		Name:           "Objective with mixed owners",
		ObjectiveType:  model.PMObjectiveTypeTactical,
		OwnerIDs:       []string{joinedOwner.ID},
		OwnerMemberIDs: []string{pendingOwner.ID},
	}, actor.ID)
	if err != nil {
		t.Fatalf("create objective with workspace member owners: %v", err)
	}
	if len(obj.OwnerMemberIDs) != 2 {
		t.Fatalf("objective owner_member_ids length = %d, want 2", len(obj.OwnerMemberIDs))
	}
	assertContainsString(t, obj.OwnerMemberIDs, joinedMember.ID)
	assertContainsString(t, obj.OwnerMemberIDs, pendingOwner.ID)
	assertContainsString(t, obj.Owners, joinedOwner.ID)
	assertContainsString(t, obj.Owners, pendingOwner.ID)

	var storedOwnerIDs []string
	if err := db.Table("pm_objective_owners").Where("objective_id = ?", obj.Objective.ID).Order("workspace_member_id").Pluck("workspace_member_id", &storedOwnerIDs).Error; err != nil {
		t.Fatalf("load stored objective owners: %v", err)
	}
	if len(storedOwnerIDs) != 2 {
		t.Fatalf("stored objective owners length = %d, want 2", len(storedOwnerIDs))
	}
	assertContainsString(t, storedOwnerIDs, joinedMember.ID)
	assertContainsString(t, storedOwnerIDs, pendingOwner.ID)

	if err := svc.AddOwner(ctx, obj.Objective.ID, actor.ID, actor.ID); err != nil {
		t.Fatalf("add objective owner via user id: %v", err)
	}
	if err := svc.RemoveOwner(ctx, obj.Objective.ID, joinedOwner.ID, actor.ID); err != nil {
		t.Fatalf("remove objective owner via user id: %v", err)
	}

	reloaded, err := objectiveRepo.GetByID(ctx, obj.Objective.ID)
	if err != nil {
		t.Fatalf("reload objective: %v", err)
	}
	if reloaded == nil {
		t.Fatal("expected reloaded objective")
	}
	assertContainsString(t, reloaded.OwnerMemberIDs, actorMember.ID)
	assertContainsString(t, reloaded.OwnerMemberIDs, pendingOwner.ID)
	if containsTestString(reloaded.OwnerMemberIDs, joinedMember.ID) {
		t.Fatalf("expected joined owner member %s to be removed", joinedMember.ID)
	}
	assertContainsString(t, reloaded.Owners, actor.ID)
	assertContainsString(t, reloaded.Owners, pendingOwner.ID)
}

func TestSettingsRepositoryPersonLifecycleWithoutWorkspacePeopleTable(t *testing.T) {
	db := newWorkspaceIdentityTestDB(t)
	ctx := context.Background()

	createSettingsIdentityTables(t, db)

	workspaceRepo := repository.NewWorkspaceRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	owner := seedWorkspaceIdentityUser(t, db, "user-owner", "owner@example.com", "Owner User")
	seedWorkspaceIdentityWorkspace(t, db, "ws-1", owner.ID)
	ownerMember, err := workspaceRepo.AddMember(ctx, "ws-1", owner.ID, model.RoleOwner)
	if err != nil {
		t.Fatalf("add owner member: %v", err)
	}

	team, err := settingsRepo.CreateTeam(ctx, model.CreateTeamRequest{
		WorkspaceID: "ws-1",
		Name:        "Operations",
		ManagerID:   &ownerMember.ID,
	})
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	person, err := settingsRepo.CreatePerson(ctx, model.CreatePersonRequest{
		WorkspaceID:         "ws-1",
		Name:                "Pending Assignee",
		Email:               "pending@example.com",
		Role:                "employee",
		JobRole:             "Support",
		ManagerID:           &ownerMember.ID,
		HireDate:            "2026-03-07",
		BaseSalary:          95000,
		ActiveForBonus:      true,
		ActiveForEvaluation: true,
		IsAccountOwner:      false,
		TeamIDs:             []string{team.ID},
	})
	if err != nil {
		t.Fatalf("create person without workspace_people table: %v", err)
	}
	if person.ID == "" {
		t.Fatal("expected synthesized person ID from workspace_members")
	}
	if person.ManagerID == nil || *person.ManagerID != ownerMember.ID {
		t.Fatalf("person manager_id = %#v, want %s", person.ManagerID, ownerMember.ID)
	}

	var member model.WorkspaceMember
	if err := db.Where("id = ?", person.ID).First(&member).Error; err != nil {
		t.Fatalf("load workspace member: %v", err)
	}
	if member.Email != "pending@example.com" || member.DisplayName != "Pending Assignee" {
		t.Fatalf("workspace member mismatch: %#v", member)
	}

	var profile model.RewardProfile
	if err := db.Where("workspace_member_id = ?", person.ID).First(&profile).Error; err != nil {
		t.Fatalf("load reward profile: %v", err)
	}
	if profile.JobRole != "Support" || profile.ManagerMemberID == nil || *profile.ManagerMemberID != ownerMember.ID {
		t.Fatalf("reward profile mismatch: %#v", profile)
	}

	var teamMembership model.TeamWorkspaceMembership
	if err := db.Where("team_id = ? AND workspace_member_id = ?", team.ID, person.ID).First(&teamMembership).Error; err != nil {
		t.Fatalf("load team workspace membership: %v", err)
	}

	updatedName := "Joined Assignee"
	updatedStatus := model.WorkspaceMemberStatusPending
	updatedJobRole := "Customer Success"
	updatedSalary := 101000.0
	updatedTeamIDs := []string{}
	updated, err := settingsRepo.UpdatePerson(ctx, person.ID, model.UpdatePersonRequest{
		Name:       &updatedName,
		Status:     &updatedStatus,
		JobRole:    &updatedJobRole,
		BaseSalary: &updatedSalary,
		TeamIDs:    updatedTeamIDs,
	})
	if err != nil {
		t.Fatalf("update person without workspace_people table: %v", err)
	}
	if updated.Name != updatedName || updated.Status != updatedStatus || updated.JobRole != updatedJobRole {
		t.Fatalf("updated person mismatch: %#v", updated)
	}

	if err := db.Where("workspace_member_id = ?", person.ID).First(&profile).Error; err != nil {
		t.Fatalf("reload reward profile: %v", err)
	}
	if profile.JobRole != updatedJobRole || profile.BaseSalary != updatedSalary {
		t.Fatalf("updated reward profile mismatch: %#v", profile)
	}

	var membershipCount int64
	if err := db.Table("team_workspace_memberships").Where("workspace_member_id = ?", person.ID).Count(&membershipCount).Error; err != nil {
		t.Fatalf("count team workspace memberships: %v", err)
	}
	if membershipCount != 0 {
		t.Fatalf("expected updated person to have no team memberships, got %d", membershipCount)
	}
}

func newWorkspaceIdentityTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	registerTestUUIDCallback(t, db)

	statements := []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			full_name TEXT NOT NULL,
			avatar_url TEXT,
			default_workspace_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			owner_id TEXT NOT NULL,
			organization_id TEXT,
			description TEXT,
			website_url TEXT,
			logo_url TEXT,
			timezone TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_members (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			user_id TEXT,
			email TEXT NOT NULL,
			display_name TEXT NOT NULL,
			role TEXT NOT NULL,
			status TEXT NOT NULL,
			invited_by TEXT,
			invited_at DATETIME,
			accepted_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspace_invitations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			workspace_member_id TEXT,
			email TEXT NOT NULL,
			role TEXT NOT NULL,
			token TEXT NOT NULL,
			invited_by TEXT NOT NULL,
			status TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			accepted_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE invitation_team_preassignments (
			id TEXT PRIMARY KEY,
			invitation_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE pm_workflows (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			team_id TEXT,
			default_state_id TEXT,
			auto_assign_owner BOOLEAN NOT NULL DEFAULT 0,
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
			description TEXT,
			w_ip_limit INTEGER,
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
			story_type TEXT NOT NULL,
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
			priority TEXT NOT NULL,
			severity TEXT NOT NULL,
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
		`CREATE TABLE pm_story_owners (
			story_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (story_id, user_id)
		)`,
		`CREATE TABLE pm_story_followers (
			story_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (story_id, user_id)
		)`,
		`CREATE TABLE pm_story_labels (
			story_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (story_id, label_id)
		)`,
		`CREATE TABLE pm_story_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			source_story_id TEXT NOT NULL,
			target_story_id TEXT NOT NULL,
			link_type TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_labels (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			color TEXT,
			team_id TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_activity_log (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			actor_id TEXT,
			action TEXT NOT NULL,
			field_name TEXT,
			old_value TEXT,
			new_value TEXT,
			metadata TEXT,
			created_at DATETIME
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}

	return db
}

func seedWorkspaceIdentityUser(t *testing.T, db *gorm.DB, id, email, fullName string) model.User {
	t.Helper()
	user := model.User{
		ID:           id,
		Email:        email,
		PasswordHash: "x",
		FullName:     fullName,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user %s: %v", id, err)
	}
	return user
}

func seedWorkspaceIdentityWorkspace(t *testing.T, db *gorm.DB, id, ownerID string) {
	t.Helper()
	workspace := model.Workspace{
		ID:        id,
		Name:      "Workspace " + id,
		Slug:      "workspace-" + id,
		OwnerID:   ownerID,
		Timezone:  "UTC",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.Create(&workspace).Error; err != nil {
		t.Fatalf("seed workspace %s: %v", id, err)
	}
}

func seedWorkflowForStoryTest(t *testing.T, db *gorm.DB, workspaceID, workflowID, stateID string) {
	t.Helper()
	workflow := model.PMWorkflow{
		ID:             workflowID,
		WorkspaceID:    workspaceID,
		Name:           "Default Workflow",
		DefaultStateID: &stateID,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := db.Create(&workflow).Error; err != nil {
		t.Fatalf("seed workflow: %v", err)
	}
	state := model.PMWorkflowState{
		ID:         stateID,
		WorkflowID: workflowID,
		Name:       "Backlog",
		StateType:  model.PMStateTypeBacklog,
		Position:   0,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := db.Create(&state).Error; err != nil {
		t.Fatalf("seed workflow state: %v", err)
	}
}

func createPMEpicObjectiveTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	statements := []string{
		`CREATE TABLE pm_epics (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			epic_state_id TEXT,
			owner_id TEXT,
			owner_member_id TEXT,
			team_id TEXT,
			planned_start_date DATETIME,
			deadline DATETIME,
			started BOOLEAN NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed BOOLEAN NOT NULL DEFAULT 0,
			completed_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			color TEXT,
			health TEXT NOT NULL DEFAULT 'no_health',
			health_comment TEXT,
			archived BOOLEAN NOT NULL DEFAULT 0,
			orchestrator_agent_id TEXT,
			spec_document_id TEXT,
			planning_repository_id TEXT,
			planning_state TEXT NOT NULL DEFAULT 'not_started',
			spec_clarifications BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			spec_clarified_at DATETIME,
			spec_clarified_by TEXT,
			approved_spec_version_id TEXT,
			active_planning_session_id TEXT,
			active_flow_run_id TEXT,
			last_planning_run_id TEXT,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_labels (
			epic_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (epic_id, label_id)
		)`,
		`CREATE TABLE pm_objectives (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			external_id TEXT,
			objective_type TEXT NOT NULL,
			state TEXT NOT NULL,
			planned_start_date DATETIME,
			deadline DATETIME,
			health TEXT NOT NULL,
			health_comment TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			archived BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_objective_teams (
			objective_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (objective_id, team_id)
		)`,
		`CREATE TABLE pm_objective_owners (
			objective_id TEXT NOT NULL,
			workspace_member_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (objective_id, workspace_member_id)
		)`,
		`CREATE TABLE pm_objective_labels (
			objective_id TEXT NOT NULL,
			label_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (objective_id, label_id)
		)`,
		`CREATE TABLE pm_key_results (
			id TEXT PRIMARY KEY,
			objective_id TEXT NOT NULL,
			name TEXT NOT NULL,
			result_type TEXT NOT NULL,
			initial_value REAL NOT NULL DEFAULT 0,
			current_value REAL NOT NULL DEFAULT 0,
			target_value REAL NOT NULL DEFAULT 100,
			progress REAL NOT NULL DEFAULT 0,
			note TEXT,
			note_updated_by TEXT,
			note_updated_at DATETIME,
			position INTEGER NOT NULL DEFAULT 0,
			updated_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE pm_epic_objectives (
			epic_id TEXT NOT NULL,
			objective_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (epic_id, objective_id)
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create pm epic/objective test table: %v", err)
		}
	}
}

func createSettingsIdentityTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	statements := []string{
		`CREATE TABLE workspace_teams (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			handle TEXT,
			description TEXT,
			manager_id TEXT,
			team_type TEXT NOT NULL DEFAULT 'engineering',
			default_story_type TEXT NOT NULL DEFAULT 'feature',
			docs_publisher_enabled BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE reward_profiles (
			id TEXT PRIMARY KEY,
			workspace_member_id TEXT NOT NULL,
			manager_member_id TEXT,
			role TEXT NOT NULL DEFAULT 'employee',
			job_role TEXT NOT NULL DEFAULT '',
			hire_date TEXT,
			base_salary REAL NOT NULL DEFAULT 0,
			active_for_bonus BOOLEAN NOT NULL DEFAULT 1,
			active_for_evaluation BOOLEAN NOT NULL DEFAULT 1,
			is_account_owner BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE team_workspace_memberships (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL,
			workspace_member_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE UNIQUE INDEX idx_team_workspace_member_unique_test
			ON team_workspace_memberships (team_id, workspace_member_id)`,
		`CREATE TABLE workspace_managers (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			person_id TEXT NOT NULL,
			can_create_goals BOOLEAN NOT NULL DEFAULT 0,
			can_score_performance BOOLEAN NOT NULL DEFAULT 0,
			reporting_to TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create settings identity table: %v", err)
		}
	}
}

func assertContainsString(t *testing.T, values []string, target string) {
	t.Helper()
	if !containsTestString(values, target) {
		t.Fatalf("expected %q in %#v", target, values)
	}
}

func containsTestString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
