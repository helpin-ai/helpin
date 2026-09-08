package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestAIUsagePeriodWorkerClosesAndOpensAnniversaryPeriod(t *testing.T) {
	now := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)
	store := &fakePeriodWorkerStore{due: []model.AIUsagePeriod{{ID: "old", WorkspaceID: "ws", PeriodEnd: now}}}
	worker := NewAIUsagePeriodWorker(store, func(context.Context, string, time.Time) (repository.AIUsagePeriodSchedule, error) {
		return repository.AIUsagePeriodSchedule{
			WorkspaceID: "ws", Start: now, End: now.AddDate(0, 1, 0), AllowanceMicrousd: 99_000_000,
			PricingVersion: "2026-08-13", EnforcementMode: model.AIUsageEnforcementStrict,
		}, nil
	})
	count, err := worker.CloseDuePeriods(context.Background(), now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || store.closed != "ws" || store.opened.AllowanceMicrousd != 99_000_000 {
		t.Fatalf("count/store = %d/%#v", count, store)
	}
}

type fakePeriodWorkerStore struct {
	due    []model.AIUsagePeriod
	closed string
	opened repository.AIUsagePeriodSchedule
}

func (f *fakePeriodWorkerStore) ListDuePeriodsAfter(_ context.Context, _ time.Time, after *model.AIUsagePeriod, limit int) ([]model.AIUsagePeriod, error) {
	start := 0
	if after != nil {
		for i, p := range f.due {
			if p.ID == after.ID {
				start = i + 1
				break
			}
		}
	}
	end := min(start+limit, len(f.due))
	return f.due[start:end], nil
}
func (f *fakePeriodWorkerStore) RolloverPeriod(_ context.Context, id string, schedule repository.AIUsagePeriodSchedule) error {
	if id == "blocked" {
		return errors.New("storage unavailable")
	}
	f.closed = schedule.WorkspaceID
	f.opened = schedule
	return nil
}

func TestAIUsagePeriodWorkerContinuesBeyondFailedBatch(t *testing.T) {
	now := time.Now().UTC()
	store := &fakePeriodWorkerStore{due: []model.AIUsagePeriod{
		{ID: "blocked", WorkspaceID: "first", PeriodEnd: now},
		{ID: "healthy", WorkspaceID: "second", PeriodEnd: now},
	}}
	worker := NewAIUsagePeriodWorker(store, func(_ context.Context, ws string, start time.Time) (repository.AIUsagePeriodSchedule, error) {
		return repository.AIUsagePeriodSchedule{WorkspaceID: ws, Start: start, End: start.AddDate(0, 1, 0)}, nil
	})
	count, err := worker.CloseDuePeriods(context.Background(), now, 1)
	if err == nil || count != 1 || store.closed != "second" {
		t.Fatalf("count=%d err=%v closed=%s", count, err, store.closed)
	}
}

func TestAIUsagePeriodWorkerCatchesUpMissedRenewals(t *testing.T) {
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	store := &fakePeriodWorkerStore{due: []model.AIUsagePeriod{{ID: "old", WorkspaceID: "ws", PeriodEnd: now.AddDate(0, -3, 0)}}}
	worker := NewAIUsagePeriodWorker(store, func(_ context.Context, ws string, start time.Time) (repository.AIUsagePeriodSchedule, error) {
		return repository.AIUsagePeriodSchedule{WorkspaceID: ws, Start: start, End: start.AddDate(0, 1, 0)}, nil
	})
	count, err := worker.CloseDuePeriods(context.Background(), now, 10)
	if err != nil || count != 1 || !store.opened.Start.Equal(now) {
		t.Fatalf("count=%d err=%v schedule=%+v", count, err, store.opened)
	}
}
