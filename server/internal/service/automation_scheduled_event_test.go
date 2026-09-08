package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type scheduledEventHandlerFunc func(context.Context, model.AutomationScheduledEvent, time.Time) error

func (fn scheduledEventHandlerFunc) HandleScheduledEvent(ctx context.Context, event model.AutomationScheduledEvent, now time.Time) error {
	return fn(ctx, event, now)
}

func scheduledServiceEvent(t *testing.T, db *gorm.DB, kind string, due time.Time) model.AutomationScheduledEvent {
	t.Helper()
	event := model.AutomationScheduledEvent{WorkspaceID: f.Workspace, EventKey: uuid.NewString(), Kind: kind,
		TargetType: "test_record", TargetID: uuid.NewString(), ExpectedRevision: 1, DueAt: due}
	if err := repository.NewAutomationScheduledEventRepository(db).Enqueue(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("event_key = ?", event.EventKey).Take(&event).Error; err != nil {
		t.Fatal(err)
	}
	return event
}

func TestAutomationScheduledEventDispatchRetriesAndFailsClosed(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		handler  ScheduledEventHandler
		code     string
		attempts int
	}{
		{name: "unknown kind", code: "unsupported_event", attempts: 1},
		{name: "unacknowledged success", handler: scheduledEventHandlerFunc(func(context.Context, model.AutomationScheduledEvent, time.Time) error { return nil }), code: "delivery_incomplete", attempts: 5},
		{name: "consumer failure", handler: scheduledEventHandlerFunc(func(context.Context, model.AutomationScheduledEvent, time.Time) error {
			return errors.New("private downstream failure")
		}), code: "delivery_failed", attempts: 5},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := f.Open(t)
			now := time.Now().UTC().Truncate(time.Microsecond)
			event := scheduledServiceEvent(t, db, "test.dispatch", now)
			svc := NewAutomationScheduledEventService(repository.NewAutomationScheduledEventRepository(db), map[string]ScheduledEventHandler{event.Kind: scenario.handler})
			svc.now = func() time.Time { return now }
			for attempt := 1; attempt <= scenario.attempts; attempt++ {
				if err := svc.DispatchDue(context.Background()); err != nil {
					t.Fatal(err)
				}
				if err := db.Where("id = ?", event.ID).Take(&event).Error; err != nil {
					t.Fatal(err)
				}
				if event.Attempts != attempt || event.ResultCode != scenario.code || event.LeaseToken != nil {
					t.Fatalf("invalid retry receipt: %#v", event)
				}
				if attempt < scenario.attempts {
					if event.Status != model.AutomationEventScheduled || !event.AvailableAt.After(now) {
						t.Fatalf("no durable backoff: %#v", event)
					}
					if err := svc.DispatchDue(context.Background()); err != nil {
						t.Fatal(err)
					}
					var stored model.AutomationScheduledEvent
					if err := db.Where("id = ?", event.ID).Take(&stored).Error; err != nil {
						t.Fatal(err)
					}
					if stored.Attempts != attempt {
						t.Fatal("retried before backoff elapsed")
					}
					now = event.AvailableAt
				}
			}
			if event.Status != model.AutomationEventFailed || event.CompletedAt == nil {
				t.Fatalf("failure not terminal: %#v", event)
			}
			now = now.Add(time.Hour)
			if err := svc.DispatchDue(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := db.Where("id = ?", event.ID).Take(&event).Error; err != nil {
				t.Fatal(err)
			}
			if event.Attempts != scenario.attempts {
				t.Fatal("failed event restarted")
			}
		})
	}
}

func TestAutomationScheduledEventDispatchDrainsPastExhaustedClaims(t *testing.T) {
	db := f.Open(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	store := repository.NewAutomationScheduledEventRepository(db)
	first := scheduledServiceEvent(t, db, "test.dispatch", now.Add(-time.Minute))
	claim, err := store.ClaimNext(context.Background(), now.Add(-time.Second), time.Millisecond)
	if err != nil || claim == nil {
		t.Fatalf("claim: %#v %v", claim, err)
	}
	f.Exec(t, db, "UPDATE automation_scheduled_events SET attempts = max_attempts WHERE id = ?", first.ID)
	second := scheduledServiceEvent(t, db, "test.dispatch", now)
	svc := NewAutomationScheduledEventService(store, map[string]ScheduledEventHandler{"test.dispatch": scheduledEventHandlerFunc(func(ctx context.Context, event model.AutomationScheduledEvent, at time.Time) error {
		return store.Complete(ctx, event, "checked", at)
	})})
	svc.now = func() time.Time { return now }
	if err := svc.DispatchDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", first.ID).Take(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", second.ID).Take(&second).Error; err != nil {
		t.Fatal(err)
	}
	if first.Status != model.AutomationEventFailed || second.Status != model.AutomationEventDelivered {
		t.Fatalf("dead head starved due work: first=%s second=%s", first.Status, second.Status)
	}
}

func TestAutomationScheduledEventDispatchDoesNotReviveCancelledClaim(t *testing.T) {
	db := f.Open(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	event := scheduledServiceEvent(t, db, "test.dispatch", now)
	store := repository.NewAutomationScheduledEventRepository(db)
	svc := NewAutomationScheduledEventService(store, map[string]ScheduledEventHandler{event.Kind: scheduledEventHandlerFunc(func(ctx context.Context, claim model.AutomationScheduledEvent, at time.Time) error {
		if err := store.CancelTarget(ctx, claim.WorkspaceID, claim.Kind, claim.TargetType, claim.TargetID, at); err != nil {
			return err
		}
		return errors.New("delivery interrupted by cancellation")
	})})
	svc.now = func() time.Time { return now }
	if err := svc.DispatchDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", event.ID).Take(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.Status != model.AutomationEventCancelled || event.Attempts != 1 {
		t.Fatalf("cancelled claim revived: %#v", event)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := svc.DispatchDue(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled dispatcher: %v", err)
	}
}

type scheduledEventBatchStore struct{ claims int }

func (s *scheduledEventBatchStore) ClaimNext(context.Context, time.Time, time.Duration) (*model.AutomationScheduledEvent, error) {
	s.claims++
	return &model.AutomationScheduledEvent{Kind: "test.batch", Status: model.AutomationEventProcessing}, nil
}

func (s *scheduledEventBatchStore) DeliveryFinished(context.Context, model.AutomationScheduledEvent) (bool, error) {
	return true, nil
}

func (s *scheduledEventBatchStore) Retry(context.Context, model.AutomationScheduledEvent, string, bool, time.Time) error {
	return errors.New("unexpected retry")
}

func TestAutomationScheduledEventDispatchBoundsBatch(t *testing.T) {
	store := &scheduledEventBatchStore{}
	handlers := map[string]ScheduledEventHandler{"test.batch": scheduledEventHandlerFunc(func(context.Context, model.AutomationScheduledEvent, time.Time) error { return nil })}
	svc := NewAutomationScheduledEventService(store, handlers)
	delete(handlers, "test.batch") // Constructor registrations cannot change at runtime.
	if err := svc.DispatchDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.claims != 100 {
		t.Fatalf("unbounded batch: %d", store.claims)
	}
}
