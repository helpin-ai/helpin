package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"testing"
)

type playbookReceivingAuthority struct {
	playbookLiveTestAuthority
	successUser string
}

func (a *playbookReceivingAuthority) ResolveActor(_ context.Context, ws, user string) (*authorization.Actor, error) {
	actor := f.Actor("admin")
	if user == a.successUser {
		actor.WorkspaceMemberID, actor.UserID = f.Success, a.successUser
	}
	if ws != f.Workspace || user != f.SalesUser && user != a.successUser {
		return nil, ErrCRMPlaybookForbidden
	}
	return actor, nil
}
func (a *playbookReceivingAuthority) GetMembershipByID(_ context.Context, ws, id string) (*model.WorkspaceMember, error) {
	if ws != f.Workspace || id != f.Sales && id != f.Success {
		return nil, ErrCRMPlaybookForbidden
	}
	user := f.SalesUser
	if id == f.Success {
		user = a.successUser
	}
	return &model.WorkspaceMember{ID: id, WorkspaceID: ws, UserID: &user, Status: model.WorkspaceMemberStatusActive}, nil
}

func TestCRMPlaybookRealModuleAdaptersRecordOnlyHumanConfirmedProgress(t *testing.T) {
	for index, name := range []string{"buying_intent", "sales_handoff", "renewal_recovery"} {
		t.Run(name, func(t *testing.T) {
			db, svc, binding, _, ctx := playbookActionFixture(t, index)
			store := repository.NewCRMPlaybookExecutionRepository(db)
			svc.executor = NewCRMPlaybookModuleExecutor(svc.execution, store, nil, nil, nil, repository.NewWorkspaceRepository(db), repository.NewAgentRepository(db), repository.NewCRMDealRepository(db))
			proposal := proposalForJourney(index)
			if index == 1 {
				authority := &playbookReceivingAuthority{successUser: uuid.NewString()}
				f.Exec(t, db, "UPDATE workspace_members SET user_id=? WHERE id=?", authority.successUser, f.Success)
				svc.execution.authz, svc.execution.members = authority, authority
				proposal = model.CRMPlaybookAction{Version: 1, Kind: "handoff", Title: "Accept the sales handoff", Reason: "Sold scope and promises are ready for the receiving owner", Handoff: &model.CRMPlaybookHandoffAction{ReceivingMemberID: f.Success, Summary: "The customer bought an annual plan and confirmed onboarding scope and the kickoff date."}}
				actor, _ := authority.ResolveActor(ctx, f.Workspace, authority.successUser)
				ctx = authorization.WithActor(ctx, actor)
			}
			action := proposeActionForTest(t, svc, authorization.WithActor(ctx, f.Actor("admin")), binding, proposal)
			if index == 1 {
				if _, err := svc.AcceptSuggestionRevision(authorization.WithActor(ctx, f.Actor("admin")), f.Workspace, action.ID, action.Revision, nil); err == nil {
					t.Fatal("Sales accepted on behalf of Success")
				}
			}
			result, err := svc.AcceptSuggestionRevision(ctx, f.Workspace, action.ID, action.Revision, nil)
			if err != nil || result.ExecutionStatus != "succeeded" {
				t.Fatalf("real adapter did not complete: %#v %v", result, err)
			}
			if _, err := svc.AcceptSuggestionRevision(ctx, f.Workspace, action.ID, action.Revision, nil); err != nil {
				t.Fatalf("original approval could not be replayed: %v", err)
			}
			item, err := repository.NewCRMSituationRepository(db).GetByID(ctx, f.Workspace, binding.SituationID)
			if err != nil || item.Situation.Lifecycle != "open" || item.Situation.OutcomeKind != nil {
				t.Fatal("module operation falsely closed customer objective")
			}
			achieved := 0
			for _, m := range item.Situation.PlaybookMilestones {
				if m.Status == "achieved" {
					achieved++
					if m.Basis == nil || *m.Basis != "human_assessment" {
						t.Fatal("missing human assessment")
					}
				}
			}
			if achieved != 1 {
				t.Fatalf("incorrect milestones: %#v", item.Situation.PlaybookMilestones)
			}
			if index == 1 && (item.Situation.OwnerMemberID == nil || *item.Situation.OwnerMemberID != f.Success) {
				t.Fatal("accepted handoff did not transfer responsibility")
			}
		})
	}
}
