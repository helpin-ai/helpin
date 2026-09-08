package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMSituationLifecycleManualCustomerJourney(t *testing.T) {
	db, svc, ctx := situationServiceFixture(t)
	created, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	req := workCommand("update", 1)
	req.Changes = &model.CRMSituationChanges{
		Owner:    &model.CRMSituationOwnerChange{MemberID: f.Ptr(f.Success)},
		NextStep: f.Ptr("Confirm the agreed kickoff date"), Attention: f.Ptr("waiting_customer"),
		Checkpoint: &model.CRMSituationCheckpointChange{At: &checkpoint},
	}
	changed := mustWorkCommand(t, svc, ctx, created.ID, req)
	if changed.Change.After.NextActionOwnerMemberID == nil || *changed.Change.After.NextActionOwnerMemberID != f.Sales {
		t.Fatal("reassigning situation ownership silently reassigned the existing commitment")
	}
	pause := workCommand("pause", 2)
	pause.Reason = "Customer requested a hold"
	paused := mustWorkCommand(t, svc, ctx, created.ID, pause)
	if paused.Change.After.Lifecycle != "paused" || !paused.Change.After.NextCheckpointAt.Equal(checkpoint) {
		t.Fatalf("pause lost the customer commitment: %#v", paused)
	}
	resumed := mustWorkCommand(t, svc, ctx, created.ID, workCommand("resume", 3))
	if resumed.Change.After.Lifecycle != "open" {
		t.Fatal("resume did not restore open work")
	}
	closing := workCommand("close", 4)
	closing.Outcome = &model.CRMSituationOutcome{Kind: "achieved", Summary: "Customer confirmed kickoff for Monday"}
	closed := mustWorkCommand(t, svc, ctx, created.ID, closing)
	state := closed.Change.After
	if state.Lifecycle != "closed" || state.OutcomeBasis == nil || *state.OutcomeBasis != "human_assessment" ||
		state.ClosedByMemberID == nil || *state.ClosedByMemberID != f.Sales || state.ClosedAt == nil || state.NextCheckpointAt != nil {
		t.Fatalf("closure lacks explicit provenance: %#v", state)
	}
	item, err := svc.GetByID(ctx, f.Workspace, created.ID)
	if err != nil || item.Situation.Revision != 5 || item.Situation.Priority != 12.75 {
		t.Fatalf("work revision/priority: %#v %v", item, err)
	}
	history, err := svc.History(ctx, f.Workspace, created.ID, 0, 100)
	if err != nil || len(history.Data) != 5 {
		t.Fatalf("history: %#v %v", history, err)
	}
	for i, change := range history.Data {
		if change.Revision != int64(5-i) || change.ActorKind != "member" || change.ActorMemberID == nil || *change.ActorMemberID != f.Sales {
			t.Fatalf("incorrect ordered attribution: %#v", change)
		}
	}
	if history.Data[4].Operation != "created" || history.Data[4].Before != nil {
		t.Fatal("creation audit missing")
	}
	var owner string
	if err := db.Table("crm_companies").Select("owner_member_id").Where("id = ?", f.Company).Scan(&owner).Error; err != nil || owner != f.Sales {
		t.Fatalf("situation assignment changed account ownership: %q %v", owner, err)
	}
	// This fixture has no PM, Flow, agent, Support or Docs tables. Manual work is independent.
}

func TestCRMSituationLifecycleReplayPreservesOriginalReceipt(t *testing.T) {
	_, svc, ctx := situationServiceFixture(t)
	stored, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	pause := workCommand("pause", 1)
	pause.Reason = "  Customer hold  "
	first := mustWorkCommand(t, svc, ctx, stored.ID, pause)
	mustWorkCommand(t, svc, ctx, stored.ID, workCommand("resume", 2))
	pause.Reason = "Customer hold"
	replay := mustWorkCommand(t, svc, ctx, stored.ID, pause)
	if !replay.Replayed || !reflect.DeepEqual(first.Change, replay.Change) {
		t.Fatalf("retry did not return original receipt: first=%#v replay=%#v", first, replay)
	}
	item, err := svc.GetByID(ctx, f.Workspace, stored.ID)
	if err != nil || item.Situation.Lifecycle != "open" || item.Situation.Revision != 3 {
		t.Fatalf("old retry rewrote newer work: %#v %v", item, err)
	}
	pause.Reason = "A different reason"
	if _, err := svc.Command(ctx, f.Workspace, stored.ID, pause); !errors.Is(err, repository.ErrCRMSituationCommandConflict) {
		t.Fatalf("retry key reused for different intent: %v", err)
	}
	pause.CommandKey = uuid.NewString()
	if _, err := svc.Command(ctx, f.Workspace, stored.ID, pause); !errors.Is(err, repository.ErrCRMSituationStale) {
		t.Fatalf("stale edit allowed: %v", err)
	}
}

func TestCRMSituationLifecycleConcurrentEdits(t *testing.T) {
	_, svc, ctx := situationServiceFixture(t)
	stored, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	results := make([]error, 8)
	for i := range results {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			req := workCommand("pause", 1)
			req.Reason = "Concurrent hold"
			_, results[index] = svc.Command(ctx, f.Workspace, stored.ID, req)
		}(i)
	}
	workers.Wait()
	winners := 0
	for _, err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, repository.ErrCRMSituationStale) {
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent winners = %d", winners)
	}
	history, err := svc.History(ctx, f.Workspace, stored.ID, 0, 100)
	if err != nil || len(history.Data) != 2 {
		t.Fatalf("duplicate history after concurrent edits: %#v %v", history, err)
	}
}

func TestCRMSituationLifecycleWaitRequiresCommitment(t *testing.T) {
	db, svc, ctx := situationServiceFixture(t)
	stored, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().UTC().Add(-time.Hour)
	for _, tt := range []struct {
		name   string
		change model.CRMSituationChanges
	}{
		{name: "missing step and deadline", change: model.CRMSituationChanges{Attention: f.Ptr("waiting_customer")}},
		{name: "missing deadline", change: model.CRMSituationChanges{Attention: f.Ptr("waiting_work"), NextStep: f.Ptr("Check progress")}},
		{name: "missing owner", change: model.CRMSituationChanges{Attention: f.Ptr("waiting_customer"), NextStep: f.Ptr("Check response"), Checkpoint: &model.CRMSituationCheckpointChange{At: &due}, Owner: &model.CRMSituationOwnerChange{}, NextActionOwner: &model.CRMSituationOwnerChange{}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := workCommand("update", 1)
			req.Changes = &tt.change
			if _, err := svc.Command(ctx, f.Workspace, stored.ID, req); !errors.Is(err, ErrCRMSituationInput) {
				t.Fatalf("invalid wait accepted: %v", err)
			}
		})
	}
	req := workCommand("update", 1)
	req.Changes = &model.CRMSituationChanges{Attention: f.Ptr("waiting_customer"), NextStep: f.Ptr("Check response"), Checkpoint: &model.CRMSituationCheckpointChange{At: &due}}
	mustWorkCommand(t, svc, ctx, stored.ID, req)
	list, err := svc.List(ctx, f.Workspace, model.CRMSituationListFilters{})
	if err != nil || list.Total != 1 || list.Data[0].EffectiveAttention != "follow_up_due" {
		t.Fatalf("due commitment did not surface: %#v %v", list, err)
	}
	pause := workCommand("pause", 2)
	pause.Reason = "Hold"
	mustWorkCommand(t, svc, ctx, stored.ID, pause)
	f.Exec(t, db, "UPDATE workspace_members SET status = 'inactive' WHERE id = ?", f.Sales)
	actor := f.Actor("member")
	actor.WorkspaceMemberID = f.Success
	successCtx := authorization.WithActor(context.Background(), actor)
	if _, err := svc.Command(successCtx, f.Workspace, stored.ID, workCommand("resume", 3)); !errors.Is(err, repository.ErrCRMSituationInvalidReference) {
		t.Fatalf("resumed wait with inactive escalation owner: %v", err)
	}
	req = workCommand("update", 3)
	req.Changes = &model.CRMSituationChanges{NextActionOwner: &model.CRMSituationOwnerChange{MemberID: f.Ptr(f.Success)}}
	mustWorkCommand(t, svc, successCtx, stored.ID, req)
	mustWorkCommand(t, svc, successCtx, stored.ID, workCommand("resume", 4))
}

func TestCRMSituationLifecycleInvalidCommands(t *testing.T) {
	_, svc, ctx := situationServiceFixture(t)
	stored, _, err := svc.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*model.CRMSituationCommandRequest){
		"missing retry key":  func(r *model.CRMSituationCommandRequest) { r.CommandKey = "" },
		"reserved retry key": func(r *model.CRMSituationCommandRequest) { r.CommandKey = "system:created" },
		"no revision":        func(r *model.CRMSituationCommandRequest) { r.ExpectedRevision = 0 },
		"no pause reason":    func(r *model.CRMSituationCommandRequest) { r.Reason = " " },
		"unknown operation":  func(r *model.CRMSituationCommandRequest) { r.Operation = "reopen" },
		"outcome on pause":   func(r *model.CRMSituationCommandRequest) { r.Outcome = &model.CRMSituationOutcome{Kind: "achieved"} },
		"empty update": func(r *model.CRMSituationCommandRequest) {
			r.Operation = "update"
			r.Changes = &model.CRMSituationChanges{}
		},
		"fake approval": func(r *model.CRMSituationCommandRequest) {
			r.Operation = "update"
			r.Changes = &model.CRMSituationChanges{Attention: f.Ptr("needs_approval")}
		},
		"fake failure": func(r *model.CRMSituationCommandRequest) {
			r.Operation = "update"
			r.Changes = &model.CRMSituationChanges{Attention: f.Ptr("automation_failed")}
		},
		"invalid owner ID": func(r *model.CRMSituationCommandRequest) {
			r.Operation = "update"
			r.Changes = &model.CRMSituationChanges{Owner: &model.CRMSituationOwnerChange{MemberID: f.Ptr("bad")}}
		},
		"oversized step": func(r *model.CRMSituationCommandRequest) {
			r.Operation = "update"
			r.Changes = &model.CRMSituationChanges{NextStep: f.Ptr(strings.Repeat("x", 1001))}
		},
		"close without assessment": func(r *model.CRMSituationCommandRequest) {
			r.Operation = "close"
			r.Outcome = &model.CRMSituationOutcome{Kind: "achieved", Summary: " "}
		},
		"duplicate without destination": func(r *model.CRMSituationCommandRequest) {
			r.Operation = "close"
			r.Outcome = &model.CRMSituationOutcome{Kind: "duplicate", Summary: "Same objective"}
		},
		"unknown outcome": func(r *model.CRMSituationCommandRequest) {
			r.Operation = "close"
			r.Outcome = &model.CRMSituationOutcome{Kind: "sent", Summary: "An email was sent"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			req := workCommand("pause", 1)
			req.Reason = "Hold"
			mutate(&req)
			if _, err := svc.Command(ctx, f.Workspace, stored.ID, req); !errors.Is(err, ErrCRMSituationInput) {
				t.Fatalf("invalid command: %v", err)
			}
		})
	}
	history, err := svc.History(ctx, f.Workspace, stored.ID, 0, 100)
	if err != nil || len(history.Data) != 1 {
		t.Fatalf("invalid commands changed history: %#v %v", history, err)
	}
}

func workCommand(operation string, revision int64) model.CRMSituationCommandRequest {
	return model.CRMSituationCommandRequest{CommandKey: uuid.NewString(), ExpectedRevision: revision, Operation: operation}
}

func mustWorkCommand(t *testing.T, svc *CRMSituationService, ctx context.Context, id string, req model.CRMSituationCommandRequest) *model.CRMSituationCommandResult {
	t.Helper()
	result, err := svc.Command(ctx, f.Workspace, id, req)
	if err != nil {
		t.Fatalf("%s: %v", req.Operation, err)
	}
	return result
}
