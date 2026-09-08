package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func checkpointSituation(t *testing.T, db *gorm.DB, due time.Time) *model.CRMSituation {
	t.Helper()
	situation := f.Situation("retention")
	situation.NextCheckpointAt, situation.NextStep = &due, "Review the renewal risk"
	situation.OwnerMemberID, situation.NextActionOwnerMemberID = checkpointPtr(f.Sales), checkpointPtr(f.Sales)
	stored, _, err := NewCRMSituationRepository(db).Create(context.Background(), situation, nil)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func checkpointCommand(t *testing.T, repo *CRMSituationRepository, current model.CRMSituation, operation string, state model.CRMSituationWorkState) *model.CRMSituationCommandResult {
	t.Helper()
	req := model.CRMSituationCommandRequest{CommandKey: uuid.NewString(), ExpectedRevision: current.Revision, Operation: operation, Reason: "Explicit change"}
	result, err := repo.ApplyCommand(context.Background(), current.WorkspaceID, current.ID, f.Sales, req, req.CommandKey,
		func(model.CRMSituation) (model.CRMSituationWorkState, error) { return state, nil })
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCRMSituationCheckpointChangeAndCancellationAreAtomic(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	events := NewAutomationScheduledEventRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	current := checkpointSituation(t, db, now)
	claim, err := events.ClaimNext(context.Background(), now, time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("checkpoint not scheduled: %#v %v", claim, err)
	}
	paused := model.CRMSituationState(*current)
	paused.Lifecycle = model.CRMSituationPaused
	checkpointCommand(t, repo, *current, "pause", paused)
	called := false
	err = repo.DeliverCheckpoint(context.Background(), *claim, now, func(model.CRMSituationItem) string {
		called = true
		return "follow_up_due"
	})
	if !errors.Is(err, ErrAutomationEventLeaseLost) || called {
		t.Fatalf("paused work reevaluated: called=%v err=%v", called, err)
	}
	if event := readScheduledEvent(t, db, claim.ID); event.Status != model.AutomationEventCancelled {
		t.Fatalf("pause did not revoke claimed event: %#v", event)
	}
	current.Lifecycle, current.Revision = model.CRMSituationPaused, 2
	resumed := model.CRMSituationState(*current)
	resumed.Lifecycle = model.CRMSituationOpen
	checkpointCommand(t, repo, *current, "resume", resumed)
	fresh, err := events.ClaimNext(context.Background(), now.Add(time.Second), time.Minute)
	if err != nil || fresh == nil || fresh.ExpectedRevision != 3 || fresh.ID == claim.ID {
		t.Fatalf("resume did not bind a new checkpoint: %#v %v", fresh, err)
	}
	if err := repo.DeliverCheckpoint(context.Background(), *fresh, now.Add(time.Second), func(model.CRMSituationItem) string { return "follow_up_due" }); err != nil {
		t.Fatal(err)
	}
	item, err := repo.GetByID(context.Background(), f.Workspace, current.ID)
	if err != nil || item.CheckpointStatus != model.AutomationEventDelivered || item.CheckpointResult != "follow_up_due" || item.CheckpointCompletedAt == nil {
		t.Fatalf("checkpoint receipt not exposed: %#v %v", item, err)
	}
	if item.Situation.Revision != 3 || item.Situation.OutcomeKind != nil {
		t.Fatalf("event changed customer lifecycle or outcome: %#v", item.Situation)
	}
}

func TestCRMSituationCheckpointOutboxFailureRollsBackChange(t *testing.T) {
	db := f.Open(t)
	repo := NewCRMSituationRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	current := checkpointSituation(t, db, now)
	injected := errors.New("injected outbox failure")
	const callback = "test:reject_scheduled_event"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "automation_scheduled_events" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Callback().Create().Remove(callback); err != nil {
			t.Error(err)
		}
	})
	req := model.CRMSituationCommandRequest{CommandKey: uuid.NewString(), ExpectedRevision: 1, Operation: "update"}
	_, err := repo.ApplyCommand(context.Background(), f.Workspace, current.ID, f.Sales, req, req.CommandKey,
		func(s model.CRMSituation) (model.CRMSituationWorkState, error) {
			state := model.CRMSituationState(s)
			later := now.Add(time.Hour)
			state.NextCheckpointAt = &later
			return state, nil
		})
	if !errors.Is(err, injected) {
		t.Fatalf("outbox failure not returned: %v", err)
	}
	item, err := repo.GetByID(context.Background(), f.Workspace, current.ID)
	if err != nil || item.Situation.Revision != 1 || !item.Situation.NextCheckpointAt.Equal(now) || item.CheckpointStatus != model.AutomationEventScheduled {
		t.Fatalf("partial checkpoint change committed: %#v %v", item, err)
	}
	history, err := repo.History(context.Background(), f.Workspace, current.ID, 0, 50)
	if err != nil || len(history.Data) != 1 {
		t.Fatalf("uncommitted history survived: %#v %v", history, err)
	}
}

func TestCRMSituationCheckpointRejectsStaleAndForeignEvents(t *testing.T) {
	for _, change := range []string{"revision", "due_time", "closed", "foreign_workspace", "missing"} {
		t.Run(change, func(t *testing.T) {
			db := f.Open(t)
			repo := NewCRMSituationRepository(db)
			events := NewAutomationScheduledEventRepository(db)
			now := time.Now().UTC().Truncate(time.Microsecond)
			current := checkpointSituation(t, db, now)
			spec := model.AutomationScheduledEvent{
				WorkspaceID: f.Workspace, EventKey: uuid.NewString(), Kind: model.CRMCheckpointEvent,
				TargetType: "crm_situation", TargetID: current.ID, ExpectedRevision: current.Revision, DueAt: now,
			}
			switch change {
			case "revision":
				spec.ExpectedRevision = 99
			case "due_time":
				spec.DueAt = now.Add(-time.Hour)
			case "closed":
				state := model.CRMSituationState(*current)
				state.Lifecycle, state.NextCheckpointAt = model.CRMSituationClosed, nil
				state.OutcomeKind, state.OutcomeSummary, state.OutcomeBasis = checkpointPtr("not_pursued"), checkpointPtr("Explicit close"), checkpointPtr("human_assessment")
				state.ClosedAt, state.ClosedByMemberID = &now, checkpointPtr(f.Sales)
				checkpointCommand(t, repo, *current, "close", state)
				spec.ExpectedRevision = 2
			case "foreign_workspace":
				spec.WorkspaceID = f.ForeignWorkspace
			case "missing":
				spec.TargetID = uuid.NewString()
			}
			// Cancel the normal event so this explicit stale-delivery fixture is next.
			if err := events.CancelTarget(context.Background(), f.Workspace, model.CRMCheckpointEvent, "crm_situation", current.ID, now); err != nil {
				t.Fatal(err)
			}
			if err := events.Enqueue(context.Background(), spec); err != nil {
				t.Fatal(err)
			}
			claim, err := events.ClaimNext(context.Background(), now, time.Minute)
			if err != nil || claim == nil {
				t.Fatalf("claim: %#v %v", claim, err)
			}
			called := false
			if err := repo.DeliverCheckpoint(context.Background(), *claim, now, func(model.CRMSituationItem) string { called = true; return "follow_up_due" }); err != nil {
				t.Fatal(err)
			}
			if stored := readScheduledEvent(t, db, claim.ID); called || stored.ResultCode != "stale" || stored.Status != model.AutomationEventDelivered {
				t.Fatalf("stale event reevaluated: called=%v event=%#v", called, stored)
			}
		})
	}
}

func checkpointPtr(value string) *string { return &value }
