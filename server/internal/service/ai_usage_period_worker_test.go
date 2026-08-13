package service

import (
	"context"
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

func (f *fakePeriodWorkerStore) ListDuePeriods(context.Context, time.Time, int) ([]model.AIUsagePeriod, error) {
	return f.due, nil
}

func (f *fakePeriodWorkerStore) ClosePeriod(_ context.Context, workspaceID string, _ time.Time) (*model.AIUsageSettlement, error) {
	f.closed = workspaceID
	return nil, nil
}

func (f *fakePeriodWorkerStore) OpenNextPeriod(_ context.Context, schedule repository.AIUsagePeriodSchedule) (*model.AIUsagePeriod, error) {
	f.opened = schedule
	return &model.AIUsagePeriod{}, nil
}
