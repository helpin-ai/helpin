package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func scheduledEventFixture(t *testing.T, db *gorm.DB, due time.Time) model.AutomationScheduledEvent {
	t.Helper()
	event := model.AutomationScheduledEvent{
		WorkspaceID: f.Workspace, EventKey: uuid.NewString(), Kind: "test.checkpoint",
		TargetType: "test_record", TargetID: uuid.NewString(), ExpectedRevision: 1, DueAt: due,
	}
	if err := NewAutomationScheduledEventRepository(db).Enqueue(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	var stored model.AutomationScheduledEvent
	if err := db.Where("workspace_id = ? AND event_key = ?", event.WorkspaceID, event.EventKey).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	return stored
}

func readScheduledEvent(t *testing.T, db *gorm.DB, id string) model.AutomationScheduledEvent {
	t.Helper()
	var event model.AutomationScheduledEvent
	if err := db.Where("id = ?", id).Take(&event).Error; err != nil {
		t.Fatal(err)
	}
	return event
}

func TestAutomationScheduledEventIdentityAndDueTime(t *testing.T) {
	db := f.Open(t)
	repo := NewAutomationScheduledEventRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	event := scheduledEventFixture(t, db, now.Add(time.Hour))
	if err := repo.Enqueue(context.Background(), event); err != nil {
		t.Fatalf("identical retry: %v", err)
	}
	changed := event
	changed.DueAt = changed.DueAt.Add(time.Minute)
	if err := repo.Enqueue(context.Background(), changed); !errors.Is(err, ErrAutomationEventConflict) {
		t.Fatalf("changed event identity = %v", err)
	}
	var count int64
	if err := db.Model(&model.AutomationScheduledEvent{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("outbox count = %d, %v", count, err)
	}
	if claim, err := repo.ClaimNext(context.Background(), now, time.Minute); err != nil || claim != nil {
		t.Fatalf("future event claimed: %#v %v", claim, err)
	}
	claim, err := repo.ClaimNext(context.Background(), event.DueAt, time.Minute)
	if err != nil || claim == nil || claim.ID != event.ID {
		t.Fatalf("due event missing: %#v %v", claim, err)
	}
	if err := repo.Complete(context.Background(), *claim, "checked", event.DueAt); err != nil {
		t.Fatal(err)
	}
	if err := repo.Enqueue(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if stored := readScheduledEvent(t, db, event.ID); stored.Status != model.AutomationEventDelivered || stored.Attempts != 1 {
		t.Fatalf("retry resurrected delivered event: %#v", stored)
	}
}

func TestAutomationScheduledEventConcurrentClaimsAndLeaseFencing(t *testing.T) {
	db := f.Open(t)
	repo := NewAutomationScheduledEventRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	event := scheduledEventFixture(t, db, now)
	claims := make([]*model.AutomationScheduledEvent, 8)
	errs := make([]error, len(claims))
	var group sync.WaitGroup
	for i := range claims {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			claims[index], errs[index] = repo.ClaimNext(context.Background(), now, time.Minute)
		}(i)
	}
	group.Wait()
	var first *model.AutomationScheduledEvent
	for i, claim := range claims {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if claim != nil {
			if first != nil {
				t.Fatal("more than one worker claimed the event")
			}
			first = claim
		}
	}
	if first == nil {
		t.Fatal("event was not claimed")
	}
	second, err := repo.ClaimNext(context.Background(), now.Add(time.Minute), time.Minute)
	if err != nil || second == nil || second.ID != event.ID || second.Attempts != 2 || *second.LeaseToken == *first.LeaseToken {
		t.Fatalf("expired lease recovery = %#v %v", second, err)
	}
	if err := repo.Complete(context.Background(), *first, "checked", now.Add(time.Minute)); !errors.Is(err, ErrAutomationEventLeaseLost) {
		t.Fatalf("old claimant acknowledged replacement: %v", err)
	}
	if err := repo.Retry(context.Background(), *first, "error", false, now.Add(time.Minute)); !errors.Is(err, ErrAutomationEventLeaseLost) {
		t.Fatalf("old claimant changed replacement retry: %v", err)
	}
	if err := repo.Complete(context.Background(), *second, "checked", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if claim, err := repo.ClaimNext(context.Background(), now.Add(time.Hour), time.Minute); err != nil || claim != nil {
		t.Fatalf("delivered event repeated: %#v %v", claim, err)
	}
}

func TestAutomationScheduledEventRetryBudget(t *testing.T) {
	for _, crashed := range []bool{false, true} {
		name := "reported failure"
		if crashed {
			name = "worker crash"
		}
		t.Run(name, func(t *testing.T) {
			db := f.Open(t)
			repo := NewAutomationScheduledEventRepository(db)
			now := time.Now().UTC().Truncate(time.Microsecond)
			event := scheduledEventFixture(t, db, now)
			for attempt := 1; attempt <= event.MaxAttempts; attempt++ {
				claim, err := repo.ClaimNext(context.Background(), now, time.Minute)
				if err != nil || claim == nil || claim.Attempts != attempt {
					t.Fatalf("attempt %d: %#v %v", attempt, claim, err)
				}
				if crashed {
					now = *claim.LeaseUntil
					continue
				}
				if err := repo.Retry(context.Background(), *claim, "delivery_failed", false, now); err != nil {
					t.Fatal(err)
				}
				stored := readScheduledEvent(t, db, event.ID)
				if attempt < event.MaxAttempts && !stored.AvailableAt.After(now) {
					t.Fatal("retry has no backoff")
				}
				now = stored.AvailableAt
			}
			claim, err := repo.ClaimNext(context.Background(), now.Add(time.Hour), time.Minute)
			if err != nil || (claim != nil && claim.Status != model.AutomationEventFailed) {
				t.Fatalf("exhausted event reclaimed: %#v %v", claim, err)
			}
			stored := readScheduledEvent(t, db, event.ID)
			if stored.Status != model.AutomationEventFailed || stored.Attempts != 5 || stored.CompletedAt == nil || stored.LeaseToken != nil {
				t.Fatalf("retry budget lost: %#v", stored)
			}
		})
	}
}

func TestAutomationScheduledEventCancellationIsFencedAndScoped(t *testing.T) {
	db := f.Open(t)
	repo := NewAutomationScheduledEventRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	event := scheduledEventFixture(t, db, now)
	other := event
	other.WorkspaceID = f.ForeignWorkspace
	if err := repo.Enqueue(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	claim, err := repo.ClaimNext(context.Background(), now, time.Minute)
	if err != nil || claim == nil {
		t.Fatalf("claim: %#v %v", claim, err)
	}
	if err := repo.CancelTarget(context.Background(), claim.WorkspaceID, claim.Kind, claim.TargetType, claim.TargetID, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.Complete(context.Background(), *claim, "checked", now); !errors.Is(err, ErrAutomationEventLeaseLost) {
		t.Fatalf("cancelled lease accepted: %v", err)
	}
	next, err := repo.ClaimNext(context.Background(), now, time.Minute)
	if err != nil || next == nil || next.WorkspaceID == claim.WorkspaceID {
		t.Fatalf("cancellation crossed workspace boundary: %#v %v", next, err)
	}
}
