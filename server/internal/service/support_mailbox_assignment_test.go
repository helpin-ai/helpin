package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func assignmentFixture(t *testing.T) *supportTriageTestFixture {
	t.Helper()
	f := newSupportTriageTestFixture(t, nil, func(s *model.SupportInboxSettings) { s.HandoffBehavior = "assign_to_team" })
	ensureSupportModuleGrantsTable(t, f.db)
	for _, id := range []string{"a", "b", "observer", "outsider", "viewer", "no-support"} {
		role := model.RoleMember
		if id == "viewer" {
			role = model.RoleViewer
		}
		seedUser(t, f.db, "user-"+id, id+"@example.com", id, "hash")
		seedWorkspaceMember(t, f.db, "wm-"+id, f.workspaceID, "user-"+id, id+"@example.com", id, role)
		if id != "no-support" {
			seedSupportModuleGrant(t, f.db, "grant-"+id, f.workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-"+id)
		}
	}
	return f
}

func assignmentRequest(t *testing.T, mode, pool string) model.CreateSupportMailboxRequest {
	t.Helper()
	var req model.CreateSupportMailboxRequest
	if err := json.Unmarshal([]byte(`{"name":"Support","handle":"support","workspace_member_ids":["wm-a","wm-b","wm-observer","wm-viewer","wm-no-support"],"assignment_mode":"`+mode+`","assignment_member_ids":`+pool+`}`), &req); err != nil {
		t.Fatal(err)
	}
	return req
}

func TestMailboxAssignmentValidatesSelectedMembers(t *testing.T) {
	for _, tt := range []struct {
		name, mode, pool string
		valid            bool
	}{
		{"specific member", "specific_member", `["wm-b"]`, true},
		{"round robin subset", "round_robin", `["wm-a","wm-b"]`, true},
		{"round robin needs selection", "round_robin", `[]`, false},
		{"round robin cannot implicitly select everyone", "round_robin", `null`, false},
		{"specific needs exactly one", "specific_member", `["wm-a","wm-b"]`, false},
		{"no inbox access", "round_robin", `["wm-outsider"]`, false},
		{"viewer cannot handle conversations", "round_robin", `["wm-viewer"]`, false},
		{"no support access", "round_robin", `["wm-no-support"]`, false},
		{"unknown member", "round_robin", `["wm-missing"]`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := assignmentFixture(t)
			_, err := f.supportSvc.CreateMailbox(f.ctx, f.workspaceID, assignmentRequest(t, tt.mode, tt.pool), f.actorID)
			if (err == nil) != tt.valid {
				t.Fatalf("CreateMailbox error = %v, valid = %v", err, tt.valid)
			}
		})
	}
}

func TestMailboxAssignmentUsesSelectedPoolForArrivalsAndMoves(t *testing.T) {
	for _, mode := range []string{"round_robin", "specific_member"} {
		t.Run(mode, func(t *testing.T) {
			f := assignmentFixture(t)
			box, err := f.supportSvc.CreateMailbox(f.ctx, f.workspaceID, assignmentRequest(t, mode, `["wm-b"]`), f.actorID)
			if err != nil {
				t.Fatal(err)
			}
			for _, previous := range []*string{nil, strPtr("user-observer")} {
				owner, state, err := f.supportSvc.determineMailboxOwner(f.ctx, f.workspaceID, box, previous)
				if err != nil || owner == nil || *owner != "user-b" || state != model.SupportConversationFlowStateAssignedToHuman {
					t.Fatalf("owner=%v state=%s err=%v", owner, state, err)
				}
			}
			// Losing inbox access must never expand the pool or retain an observer as the owner.
			if err := f.mailboxRepo.ReplaceMembers(f.ctx, box.ID, []string{"wm-observer"}); err != nil {
				t.Fatal(err)
			}
			owner, state, err := f.supportSvc.determineMailboxOwner(f.ctx, f.workspaceID, box, strPtr("user-observer"))
			if err != nil || owner != nil || state != model.SupportConversationFlowStateWaitingForHuman {
				t.Fatalf("expected unassigned, owner=%v state=%s err=%v", owner, state, err)
			}
		})
	}
}

func TestMailboxAssignmentUpdateValidatesProspectiveAccess(t *testing.T) {
	f := assignmentFixture(t)
	box, err := f.supportSvc.CreateMailbox(f.ctx, f.workspaceID, assignmentRequest(t, "round_robin", `["wm-b"]`), f.actorID)
	if err != nil {
		t.Fatal(err)
	}
	var req model.UpdateSupportMailboxRequest
	if err := json.Unmarshal([]byte(`{"workspace_member_ids":["wm-observer"],"assignment_member_ids":["wm-b"]}`), &req); err != nil {
		t.Fatal(err)
	}
	if _, err := f.supportSvc.UpdateMailbox(f.ctx, f.workspaceID, box.ID, req); err == nil {
		t.Fatal("expected removed inbox member to be rejected as an assignee")
	}
	members, err := f.mailboxRepo.ListMembers(f.ctx, box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 5 {
		t.Fatal("invalid update changed inbox access")
	}
}

func TestMailboxAssignmentHandoffUsesSelectedPool(t *testing.T) {
	for _, mode := range []string{"round_robin", "specific_member", "manual"} {
		t.Run(mode, func(t *testing.T) {
			f := assignmentFixture(t)
			pool := `["wm-b"]`
			if mode == "manual" {
				pool = `[]`
			}
			box, err := f.supportSvc.CreateMailbox(f.ctx, f.workspaceID, assignmentRequest(t, mode, pool), f.actorID)
			if err != nil {
				t.Fatal(err)
			}
			// Exercise the actual escalation path, including an observer as the previous owner.
			conv := f.createConversation(t, "question", "customer@example.com", nil)
			if err := f.conversationRepo.UpdateFields(f.ctx, f.workspaceID, conv.ID, map[string]any{"mailbox_id": box.ID, "assigned_user_id": "user-observer"}); err != nil {
				t.Fatal(err)
			}
			ai := &SupportAIService{handoffRepo: repository.NewAgentHandoffRepository(f.db), conversationRepo: f.conversationRepo, messageRepo: f.messageRepo, mailboxRepo: f.mailboxRepo, workspaceRepo: repository.NewWorkspaceRepository(f.db), installationRepo: repository.NewSupportInboxInstallationRepository(f.db)}
			if err := ai.EscalateToHuman(f.ctx, f.workspaceID, conv.ID, "requested"); err != nil {
				t.Fatal(err)
			}
			got, err := f.conversationRepo.GetByID(f.ctx, f.workspaceID, conv.ID, "", model.RoleOwner)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "manual" {
				if got.AssignedUserID != nil {
					t.Fatalf("manual handoff assigned %s", *got.AssignedUserID)
				}
			} else if got.AssignedUserID == nil || *got.AssignedUserID != "user-b" {
				t.Fatalf("handoff ignored selected member: %v", got.AssignedUserID)
			}
		})
	}
}

func TestMailboxAssignmentRoundRobinRotates(t *testing.T) {
	f := assignmentFixture(t)
	box, err := f.supportSvc.CreateMailbox(f.ctx, f.workspaceID, assignmentRequest(t, "round_robin", `["wm-a","wm-b"]`), f.actorID)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, err := f.supportSvc.determineMailboxOwner(f.ctx, f.workspaceID, box, nil)
	if err != nil || owner == nil || *owner != "user-a" {
		t.Fatalf("first owner=%v err=%v", owner, err)
	}
	conv := f.createConversation(t, "first", "customer@example.com", nil)
	if err := f.conversationRepo.UpdateFields(f.ctx, f.workspaceID, conv.ID, map[string]any{"mailbox_id": box.ID, "assigned_user_id": *owner, "created_at": time.Now()}); err != nil {
		t.Fatal(err)
	}
	owner, _, err = f.supportSvc.determineMailboxOwner(f.ctx, f.workspaceID, box, nil)
	if err != nil || owner == nil || *owner != "user-b" {
		t.Fatalf("next owner=%v err=%v", owner, err)
	}
}

func TestMailboxAssignmentLegacyPoolSurvivesUnrelatedUpdate(t *testing.T) {
	f := assignmentFixture(t)
	box := f.createMailbox(t, "Legacy", "legacy", true)
	box.AssignmentMode = "round_robin"
	if err := f.mailboxRepo.Update(f.ctx, box); err != nil {
		t.Fatal(err)
	}
	if err := f.mailboxRepo.AddMembers(f.ctx, box.ID, []string{"wm-a"}); err != nil {
		t.Fatal(err)
	}
	updated, err := f.supportSvc.UpdateMailbox(f.ctx, f.workspaceID, box.ID, model.UpdateSupportMailboxRequest{Name: strPtr("Renamed")})
	if err != nil {
		t.Fatal(err)
	}
	if updated.AssignmentMemberIDs != nil {
		t.Fatal("unrelated edit changed legacy assignment pool")
	}
	owner, _, err := f.supportSvc.determineMailboxOwner(f.ctx, f.workspaceID, updated, nil)
	if err != nil || owner == nil || *owner != "user-a" {
		t.Fatalf("legacy owner=%v err=%v", owner, err)
	}
	if _, err := f.supportSvc.UpdateMailbox(f.ctx, f.workspaceID, box.ID, model.UpdateSupportMailboxRequest{AssignmentMemberIDs: []string{}}); err == nil {
		t.Fatal("explicit empty round-robin pool should be rejected")
	}
}

func TestMailboxAssignmentManualOverrideStillAllowsObservers(t *testing.T) {
	f := assignmentFixture(t)
	f.supportSvc.SetWorkspaceRepo(repository.NewWorkspaceRepository(f.db))
	box, err := f.supportSvc.CreateMailbox(f.ctx, f.workspaceID, assignmentRequest(t, "round_robin", `["wm-b"]`), f.actorID)
	if err != nil {
		t.Fatal(err)
	}
	conv := f.createConversation(t, "Manual override", "customer@example.com", &box.ID)
	if err := f.supportSvc.AssignConversationUser(f.ctx, f.workspaceID, conv.ID, strPtr("user-observer"), f.actorID); err != nil {
		t.Fatal(err)
	}
	got, err := f.conversationRepo.GetByID(f.ctx, f.workspaceID, conv.ID, "", model.RoleOwner)
	if err != nil || got == nil || got.AssignedUserID == nil || *got.AssignedUserID != "user-observer" {
		t.Fatalf("manual assignment lost: conversation=%v err=%v", got, err)
	}
}

func TestMailboxAssignmentFollowUpHandoff(t *testing.T) {
	for _, mode := range []string{"round_robin", "specific_member", "manual"} {
		t.Run(mode, func(t *testing.T) {
			f := assignmentFixture(t)
			pool := `["wm-b"]`
			if mode == "manual" {
				pool = `[]`
			}
			box, err := f.supportSvc.CreateMailbox(f.ctx, f.workspaceID, assignmentRequest(t, mode, pool), f.actorID)
			if err != nil {
				t.Fatal(err)
			}
			conv := f.createConversation(t, "Follow-up handoff", "customer@example.com", &box.ID)
			svc := &SupportFollowUpService{chat: &SupportChatService{supportAIService: &SupportAIService{workspaceRepo: repository.NewWorkspaceRepository(f.db), mailboxRepo: f.mailboxRepo, installationRepo: repository.NewSupportInboxInstallationRepository(f.db)}}}
			settings := model.DefaultSupportInboxSettings()
			settings.HandoffBehavior = "assign_to_team"
			if err := svc.handoff(f.ctx, f.db, conv, settings, time.Now()); err != nil {
				t.Fatal(err)
			}
			got, err := f.conversationRepo.GetByID(f.ctx, f.workspaceID, conv.ID, "", model.RoleOwner)
			if err != nil || got == nil {
				t.Fatalf("get handoff: %v", err)
			}
			if mode == "manual" {
				if got.AssignedUserID != nil {
					t.Fatal("manual inbox received automatic assignment")
				}
			} else if got.AssignedUserID == nil || *got.AssignedUserID != "user-b" {
				t.Fatalf("handoff selected %v", got.AssignedUserID)
			}
		})
	}
}

func TestMailboxAssignmentHandoffRotationDoesNotRestrictNotifications(t *testing.T) {
	f := assignmentFixture(t)
	box, err := f.supportSvc.CreateMailbox(f.ctx, f.workspaceID, assignmentRequest(t, "round_robin", `["wm-a","wm-b"]`), f.actorID)
	if err != nil {
		t.Fatal(err)
	}
	conv := f.createConversation(t, "Already assigned", "customer@example.com", &box.ID)
	if err := f.conversationRepo.UpdateFields(f.ctx, f.workspaceID, conv.ID, map[string]any{"assigned_user_id": "user-a"}); err != nil {
		t.Fatal(err)
	}
	workspaceRepo := repository.NewWorkspaceRepository(f.db)
	instRepo := repository.NewSupportInboxInstallationRepository(f.db)
	input := supportRecipientSelectorInput{WorkspaceID: f.workspaceID, MailboxID: &box.ID, OwnerUserID: strPtr("user-observer"), UseMailboxAssignment: true}
	selected, err := selectSupportConversationRecipient(f.ctx, workspaceRepo, f.mailboxRepo, instRepo, nil, nil, nil, input)
	if err != nil || selected == nil || selected.UserID != "user-b" {
		t.Fatalf("round-robin handoff selection=%v err=%v", selected, err)
	}
	input.UseMailboxAssignment = false
	selected, err = selectSupportConversationRecipient(f.ctx, workspaceRepo, f.mailboxRepo, instRepo, nil, nil, nil, input)
	if err != nil || selected == nil || selected.UserID != "user-observer" {
		t.Fatalf("notification selection changed: %v err=%v", selected, err)
	}
	// Revoking Support access must not turn a single-person pool into an all-members pool.
	box.AssignmentMemberIDs = []string{"wm-b"}
	if err := f.mailboxRepo.Update(f.ctx, box); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.db, `DELETE FROM workspace_module_grants WHERE subject_id = ?`, "wm-b")
	input.UseMailboxAssignment = true
	selected, err = selectSupportConversationRecipient(f.ctx, workspaceRepo, f.mailboxRepo, instRepo, nil, nil, nil, input)
	if err != nil || selected != nil {
		t.Fatalf("revoked pool fell back: %v err=%v", selected, err)
	}
}

func TestMailboxAssignmentAcceptsLinkedTeamAccess(t *testing.T) {
	f := assignmentFixture(t)
	mustExec(t, f.db, `INSERT INTO workspace_teams (id, workspace_id, name) VALUES (?, ?, ?)`, "team-support", f.workspaceID, "Support")
	mustExec(t, f.db, `INSERT INTO team_workspace_memberships (id, workspace_member_id, team_id) VALUES (?, ?, ?)`, "team-membership", "wm-outsider", "team-support")
	req := assignmentRequest(t, "specific_member", `["wm-outsider"]`)
	req.LinkedTeamID = strPtr("team-support")
	box, err := f.supportSvc.CreateMailbox(f.ctx, f.workspaceID, req, f.actorID)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, err := f.supportSvc.determineMailboxOwner(f.ctx, f.workspaceID, box, nil)
	if err != nil || owner == nil || *owner != "user-outsider" {
		t.Fatalf("team assignment=%v err=%v", owner, err)
	}
}

func TestMailboxAssignmentViewerIsDisabledForManualAssignment(t *testing.T) {
	f := assignmentFixture(t)
	f.supportSvc.SetWorkspaceRepo(repository.NewWorkspaceRepository(f.db))
	box, err := f.supportSvc.CreateMailbox(f.ctx, f.workspaceID, assignmentRequest(t, "manual", `[]`), f.actorID)
	if err != nil {
		t.Fatal(err)
	}
	conv := f.createConversation(t, "Read-only member", "customer@example.com", &box.ID)
	members, err := f.supportSvc.ListConversationAssignableUsers(f.ctx, f.workspaceID, conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, member := range members {
		if member.ID == "wm-viewer" {
			found = true
			if member.AssignmentDisabledReason == nil {
				t.Fatal("viewer was selectable")
			}
		}
	}
	if !found {
		t.Fatal("viewer should be visible with a disabled reason")
	}
	if err := f.supportSvc.AssignConversationUser(f.ctx, f.workspaceID, conv.ID, strPtr("user-viewer"), f.actorID); err == nil {
		t.Fatal("viewer could receive a conversation they cannot handle")
	}
}
