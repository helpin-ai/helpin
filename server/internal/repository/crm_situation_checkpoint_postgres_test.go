package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAutomationScheduledEventPostgresBackfill(t *testing.T) {
	db := f.Open(t)
	if db.Dialector.Name() != "postgres" {
		t.Skip("requires disposable PostgreSQL")
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	open := f.Situation("retention")
	open.NextCheckpointAt = &now
	paused, closed, none := f.Situation("retention"), f.Situation("retention"), f.Situation("retention")
	paused.Lifecycle, paused.NextCheckpointAt = "paused", &now
	closed.Lifecycle, closed.OutcomeKind, closed.OutcomeSummary, closed.ClosedAt, closed.NextCheckpointAt = "closed", f.Ptr("achieved"), f.Ptr("Confirmed by customer"), &now, &now
	// Direct writes simulate records that predate the outbox migration.
	for _, situation := range []model.CRMSituation{open, paused, closed, none} {
		if err := db.Create(&situation).Error; err != nil {
			t.Fatal(err)
		}
	}
	_, path, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(path), "../dbmigrate/sql/202609060003_automation_scheduled_events.sql"))
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, string(source))
	var events []model.AutomationScheduledEvent
	if err := db.Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].TargetID != open.ID || events[0].ExpectedRevision != 1 || !events[0].DueAt.Equal(now) {
		t.Fatalf("backfilled more than explicit open commitments: %#v", events)
	}
	repo := NewAutomationScheduledEventRepository(db)
	claim, err := repo.ClaimNext(context.Background(), now, time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("claim: %#v %v", claim, err)
	}
	if err := repo.Complete(context.Background(), *claim, "follow_up_due", now); err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, string(source))
	if err := db.Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Status != model.AutomationEventDelivered || events[0].ID != claim.ID {
		t.Fatalf("migration replay reset a receipt: %#v", events)
	}
}

func TestAutomationScheduledEventPostgresSkipsLockedRows(t *testing.T) {
	db := f.Open(t)
	if db.Dialector.Name() != "postgres" {
		t.Skip("requires disposable PostgreSQL")
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	first := scheduledEventFixture(t, db, now.Add(-time.Minute))
	second := scheduledEventFixture(t, db, now)
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer func() {
		if err := tx.Rollback().Error; err != nil {
			t.Error(err)
		}
	}()
	f.Exec(t, tx, "SELECT id FROM automation_scheduled_events WHERE id = ? FOR UPDATE", first.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	claim, err := NewAutomationScheduledEventRepository(db).ClaimNext(ctx, now, time.Minute)
	if err != nil || claim == nil || claim.ID != second.ID {
		t.Fatalf("locked event blocked another worker: %#v %v", claim, err)
	}
}

func TestAutomationScheduledEventConcurrentEnqueue(t *testing.T) {
	db := f.Open(t)
	repo := NewAutomationScheduledEventRepository(db)
	event := model.AutomationScheduledEvent{WorkspaceID: f.Workspace, EventKey: uuid.NewString(), Kind: model.CRMCheckpointEvent,
		TargetType: "crm_situation", TargetID: uuid.NewString(), ExpectedRevision: 1, DueAt: time.Now()}
	var group sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		group.Add(1)
		go func(index int) { defer group.Done(); errs[index] = repo.Enqueue(context.Background(), event) }(i)
	}
	group.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Model(&model.AutomationScheduledEvent{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("duplicate scheduled intent: count=%d err=%v", count, err)
	}
}

func TestCRMSituationCheckpointPostgresLeaseExpiresBehindDomainLock(t *testing.T) {
	db := f.Open(t)
	if db.Dialector.Name() != "postgres" {
		t.Skip("requires disposable PostgreSQL")
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	checkpointSituation(t, db, now)
	claim, err := NewAutomationScheduledEventRepository(db).ClaimNext(context.Background(), now, 100*time.Millisecond)
	if err != nil || claim == nil {
		t.Fatalf("claim: %#v %v", claim, err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if err := lockSituationWorkspace(tx, f.Workspace); err != nil {
		t.Fatal(err)
	}
	// Release the domain lock after the claimed lease has expired. The consumer
	// starts with a live timestamp, but must not reuse it after waiting on a lock.
	unlock := make(chan error, 1)
	go func() { time.Sleep(200 * time.Millisecond); unlock <- tx.Rollback().Error }()
	called := false
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = NewCRMSituationRepository(db).DeliverCheckpoint(ctx, *claim, now, func(model.CRMSituationItem) string { called = true; return "follow_up_due" })
	if releaseErr := <-unlock; releaseErr != nil {
		t.Fatal(releaseErr)
	}
	if !errors.Is(err, ErrAutomationEventLeaseLost) || called {
		t.Fatalf("expired worker reevaluated after lock wait: evaluated=%v err=%v", called, err)
	}
}

func TestCRMSituationCheckpointCanonicalClaimAndAtomicReceipt(t *testing.T) {
	db := f.Open(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	situation := checkpointSituation(t, db, now)
	events := NewAutomationScheduledEventRepository(db)
	claim, err := events.ClaimNext(context.Background(), now, time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("claim: %#v %v", claim, err)
	}
	claim.TargetID, claim.ExpectedRevision, claim.Kind = uuid.NewString(), 999, "forged.kind"
	repo := NewCRMSituationRepository(db)
	err = repo.DeliverCheckpoint(context.Background(), *claim, now, func(item model.CRMSituationItem) string {
		if item.Situation.ID != situation.ID {
			t.Fatal("consumer trusted claim payload instead of canonical identity")
		}
		return "" // invalid receipt must roll back, not leave a delivered event
	})
	if err == nil {
		t.Fatal("invalid receipt was committed")
	}
	if event := readScheduledEvent(t, db, claim.ID); event.Status != model.AutomationEventProcessing {
		t.Fatalf("partial delivery persisted: %#v", event)
	}
	if err := repo.DeliverCheckpoint(context.Background(), *claim, now, func(model.CRMSituationItem) string { return "follow_up_due" }); err != nil {
		t.Fatal(err)
	}
}
