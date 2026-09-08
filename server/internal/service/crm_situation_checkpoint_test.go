package service

import (
	"context"
	"errors"
	"testing"
	"time"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCRMSituationCheckpointDispatchUsesCurrentFacts(t *testing.T) {
	db, crm, ctx := situationServiceFixture(t)
	created, _, err := crm.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	req := workCommand("update", 1)
	req.Changes = &model.CRMSituationChanges{NextStep: f.Ptr("Ask whether the proposal was reviewed"),
		Attention: f.Ptr("waiting_customer"), Checkpoint: &model.CRMSituationCheckpointChange{At: &now}}
	mustWorkCommand(t, crm, ctx, created.ID, req)
	// A membership change does not increment the Signal revision. The worker
	// must load current ownership instead of trusting the original assignment.
	f.Exec(t, db, "UPDATE workspace_members SET status = 'inactive' WHERE id = ?", f.Sales)
	repo := repository.NewCRMSituationRepository(db)
	svc := NewAutomationScheduledEventService(repository.NewAutomationScheduledEventRepository(db), map[string]ScheduledEventHandler{
		model.CRMCheckpointEvent: NewCRMSituationCheckpointService(repo),
	})
	svc.now = func() time.Time { return now }
	for range 2 {
		if err := svc.DispatchDue(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	item, err := repo.GetByID(context.Background(), f.Workspace, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.CheckpointResult != "needs_owner" || item.CheckpointAttempts != 1 || item.CheckpointStatus != "delivered" || item.Situation.Revision != 2 || item.Situation.OutcomeKind != nil {
		t.Fatalf("stale context or duplicate lifecycle effect: %#v", item)
	}
	list, err := repo.List(context.Background(), f.Workspace, f.Success, model.CRMSituationListFilters{Scope: "unassigned", State: "needs_attention", Category: "all", Page: 1, PageSize: 20})
	if err != nil || list.Total != 1 {
		t.Fatalf("missing owner not discoverable: %#v %v", list, err)
	}
	// No separate task, run, or outcome is manufactured by a scheduled check.
	var history, suggestions int64
	if err := db.Table("crm_situation_changes").Where("situation_id = ?", created.ID).Count(&history).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("crm_suggestions").Count(&suggestions).Error; err != nil {
		t.Fatal(err)
	}
	if history != 2 || suggestions != 1 {
		t.Fatalf("checkpoint created unrelated work: history=%d suggestions=%d", history, suggestions)
	}
}

func TestCRMSituationCheckpointFailureIsVisibleAndRevisionScoped(t *testing.T) {
	db, crm, ctx := situationServiceFixture(t)
	created, _, err := crm.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	req := workCommand("update", 1)
	req.Changes = &model.CRMSituationChanges{NextStep: f.Ptr("Confirm the proposal"),
		Attention: f.Ptr("waiting_customer"), Checkpoint: &model.CRMSituationCheckpointChange{At: &now}}
	mustWorkCommand(t, crm, ctx, created.ID, req)
	svc := NewAutomationScheduledEventService(repository.NewAutomationScheduledEventRepository(db), map[string]ScheduledEventHandler{
		model.CRMCheckpointEvent: scheduledEventHandlerFunc(func(context.Context, model.AutomationScheduledEvent, time.Time) error {
			return errors.New("temporary dependency unavailable")
		}),
	})
	svc.now = func() time.Time { return now }
	for range 5 {
		if err := svc.DispatchDue(context.Background()); err != nil {
			t.Fatal(err)
		}
		now = now.Add(20 * time.Minute)
	}
	item, err := crm.GetByID(ctx, f.Workspace, created.ID)
	if err != nil || item.EffectiveAttention != "automation_failed" || item.CheckpointAttempts != 5 {
		t.Fatalf("invisible failed checkpoint: %#v %v", item, err)
	}
	list, err := crm.List(ctx, f.Workspace, model.CRMSituationListFilters{Scope: "mine", State: "needs_attention"})
	if err != nil || list.Total != 1 {
		t.Fatalf("failure missing from responsible member's inbox: %#v %v", list, err)
	}
	// A colleague's pending approval must not hide this member's failed check.
	f.Exec(t, db, "INSERT INTO crm_situation_references (workspace_id, situation_id, kind, source_id) VALUES (?,?,'suggestion',?)", f.Workspace, created.ID, f.Suggestion)
	f.Exec(t, db, "UPDATE crm_suggestions SET user_id = (SELECT user_id FROM workspace_members WHERE id = ?) WHERE id = ?", f.Success, f.Suggestion)
	list, err = crm.List(ctx, f.Workspace, model.CRMSituationListFilters{Scope: "mine", State: "needs_attention"})
	if err != nil || list.Total != 1 {
		t.Fatalf("colleague's approval hid checkpoint failure: %#v %v", list, err)
	}
	// Resolve the fixture proposal before checking the next waiting commitment.
	f.Exec(t, db, "UPDATE crm_suggestions SET status = 'dismissed' WHERE id = ?", f.Suggestion)
	future := now.Add(time.Hour)
	req = workCommand("update", 2)
	req.Changes = &model.CRMSituationChanges{Checkpoint: &model.CRMSituationCheckpointChange{At: &future}}
	mustWorkCommand(t, crm, ctx, created.ID, req)
	item, err = crm.GetByID(ctx, f.Workspace, created.ID)
	if err != nil || item.CheckpointStatus != "scheduled" || item.CheckpointResult != "" || item.EffectiveAttention != "waiting_customer" {
		t.Fatalf("old failure leaked into new commitment: %#v %v", item, err)
	}
}

func TestCRMSituationCheckpointLifecycleRevokesClaimedChecks(t *testing.T) {
	for _, operation := range []string{"reschedule", "clear", "reassign", "close"} {
		t.Run(operation, func(t *testing.T) {
			db, crm, ctx := situationServiceFixture(t)
			created, _, err := crm.Create(ctx, f.Workspace, situationCreateRequest())
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Truncate(time.Microsecond)
			req := workCommand("update", 1)
			req.Changes = &model.CRMSituationChanges{NextStep: f.Ptr("Confirm the next milestone"), Checkpoint: &model.CRMSituationCheckpointChange{At: &now}}
			mustWorkCommand(t, crm, ctx, created.ID, req)
			events := repository.NewAutomationScheduledEventRepository(db)
			claim, err := events.ClaimNext(ctx, now, time.Minute)
			if err != nil || claim == nil {
				t.Fatalf("claim: %#v %v", claim, err)
			}
			req = workCommand("update", 2)
			future := now.Add(time.Hour)
			switch operation {
			case "reschedule":
				req.Changes = &model.CRMSituationChanges{Checkpoint: &model.CRMSituationCheckpointChange{At: &future}}
			case "clear":
				req.Changes = &model.CRMSituationChanges{Checkpoint: &model.CRMSituationCheckpointChange{At: nil}}
			case "reassign":
				req.Changes = &model.CRMSituationChanges{NextActionOwner: &model.CRMSituationOwnerChange{MemberID: f.Ptr(f.Success)}}
			case "close":
				req.Operation = "close"
				req.Outcome = &model.CRMSituationOutcome{Kind: "achieved", Summary: "Customer confirmed the milestone"}
			}
			mustWorkCommand(t, crm, ctx, created.ID, req)
			replay := mustWorkCommand(t, crm, ctx, created.ID, req)
			if !replay.Replayed {
				t.Fatal("lifecycle retry lost its receipt")
			}
			consumer := NewCRMSituationCheckpointService(repository.NewCRMSituationRepository(db))
			if err := consumer.HandleScheduledEvent(ctx, *claim, now); !errors.Is(err, repository.ErrAutomationEventLeaseLost) {
				t.Fatalf("old check survived %s: %v", operation, err)
			}
			var stored []model.AutomationScheduledEvent
			if err := db.Order("expected_revision").Find(&stored).Error; err != nil {
				t.Fatal(err)
			}
			want := 1
			if operation == "reschedule" || operation == "reassign" {
				want = 2
			}
			if len(stored) != want || stored[0].Status != "cancelled" {
				t.Fatalf("invalid replacement schedule: %#v", stored)
			}
			if want == 2 && (stored[1].ExpectedRevision != 3 || stored[1].Status != "scheduled") {
				t.Fatalf("replacement not bound to new commitment: %#v", stored[1])
			}
		})
	}
}

func TestCRMSituationCheckpointEvaluationPriority(t *testing.T) {
	base := model.CRMSituationItem{OwnerAvailable: true, NextActionOwnerAvailable: true, Situation: model.CRMSituation{NextStep: "Review the reply"}}
	for _, tc := range []struct {
		name, want string
		change     func(*model.CRMSituationItem)
	}{
		{"follow-up", "follow_up_due", func(*model.CRMSituationItem) {}},
		{"missing owner", "needs_owner", func(i *model.CRMSituationItem) { i.OwnerAvailable = false; i.PendingActionCount = 1 }},
		{"explicit inactive owner", "needs_owner", func(i *model.CRMSituationItem) {
			i.Situation.NextActionOwnerMemberID = f.Ptr(f.Inactive)
			i.NextActionOwnerAvailable = false
		}},
		{"failure before approval", "automation_failed", func(i *model.CRMSituationItem) { i.FailedActionCount = 1; i.PendingActionCount = 1 }},
		{"uncertain execution", "automation_failed", func(i *model.CRMSituationItem) { i.UncertainActionCount = 1 }},
		{"approval", "needs_approval", func(i *model.CRMSituationItem) { i.PendingActionCount = 1 }},
		{"manual action", "needs_context", func(i *model.CRMSituationItem) { i.ManualActionCount = 1 }},
		{"missing next step", "needs_context", func(i *model.CRMSituationItem) { i.Situation.NextStep = "" }},
		{"executing", "waiting_work", func(i *model.CRMSituationItem) { i.ExecutingActionCount = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := base
			tc.change(&item)
			if got := evaluateCRMCheckpoint(item); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestCRMSituationCheckpointOwnerRecoveryUsesLiveMembership(t *testing.T) {
	db, crm, ctx := situationServiceFixture(t)
	created, _, err := crm.Create(ctx, f.Workspace, situationCreateRequest())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	req := workCommand("update", 1)
	req.Changes = &model.CRMSituationChanges{NextStep: f.Ptr("Confirm onboarding progress"),
		NextActionOwner: &model.CRMSituationOwnerChange{MemberID: f.Ptr(f.Success)},
		Attention:       f.Ptr("waiting_work"), Checkpoint: &model.CRMSituationCheckpointChange{At: &now}}
	mustWorkCommand(t, crm, ctx, created.ID, req)
	f.Exec(t, db, "UPDATE workspace_members SET status = 'inactive' WHERE id = ?", f.Success)
	f.Exec(t, db, "INSERT INTO crm_situation_references (workspace_id, situation_id, kind, source_id) VALUES (?,?,'suggestion',?)", f.Workspace, created.ID, f.Suggestion)
	f.Exec(t, db, "UPDATE crm_suggestions SET status = 'accepted', execution_status = 'in_progress', updated_at = ? WHERE id = ?", time.Now().UTC(), f.Suggestion)
	repo := repository.NewCRMSituationRepository(db)
	svc := NewAutomationScheduledEventService(repository.NewAutomationScheduledEventRepository(db), map[string]ScheduledEventHandler{
		model.CRMCheckpointEvent: NewCRMSituationCheckpointService(repo),
	})
	svc.now = func() time.Time { return now }
	if err := svc.DispatchDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	filters := model.CRMSituationListFilters{Scope: "unassigned", State: "needs_attention", Category: "all", Page: 1, PageSize: 20}
	list, err := repo.List(context.Background(), f.Workspace, f.Sales, filters)
	if err != nil || list.Total != 1 {
		t.Fatalf("active execution hid a missing commitment owner: %#v %v", list, err)
	}
	f.Exec(t, db, "UPDATE workspace_members SET status = 'active' WHERE id = ?", f.Success)
	list, err = repo.List(context.Background(), f.Workspace, f.Sales, filters)
	if err != nil || list.Total != 0 {
		t.Fatalf("old needs-owner receipt overruled restored membership: %#v %v", list, err)
	}
	filters.State = "open"
	list, err = repo.List(context.Background(), f.Workspace, f.Sales, filters)
	if err != nil || list.Total != 0 {
		t.Fatalf("restored owner is still marked unassigned: %#v %v", list, err)
	}
}
