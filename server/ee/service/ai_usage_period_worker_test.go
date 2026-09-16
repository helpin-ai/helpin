//go:build ee

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAIUsagePeriodWorkerClosesAndOpensAnniversaryPeriod(t *testing.T) {
	now := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)
	store := &fakePeriodWorkerStore{due: []model.AIUsagePeriod{{ID: "old", WorkspaceID: "ws", PeriodEnd: now}}}
	worker := NewAIUsagePeriodWorker(store, func(context.Context, string, time.Time) (eerepository.AIUsagePeriodSchedule, error) {
		return eerepository.AIUsagePeriodSchedule{
			WorkspaceID: "ws", Start: now, End: now.AddDate(0, 1, 0), AllowanceMicrousd: 99_000_000,
			PricingVersion: "2026-08-13", EnforcementMode: model.AIUsageEnforcementStrict,
		}, nil
	})
	count, err := worker.CloseDuePeriods(context.Background(), now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || store.closed != "old" || store.opened.AllowanceMicrousd != 99_000_000 {
		t.Fatalf("count/store = %d/%#v", count, store)
	}
}

type fakePeriodWorkerStore struct {
	openErr  map[string]error
	closeErr map[string]error
	due      []model.AIUsagePeriod
	closed   string
	opened   eerepository.AIUsagePeriodSchedule
}

func (f *fakePeriodWorkerStore) ListDuePeriods(context.Context, time.Time, int) ([]model.AIUsagePeriod, error) {
	return f.due, nil
}

func (f *fakePeriodWorkerStore) RolloverPeriod(ctx context.Context, periodID string, schedule eerepository.AIUsagePeriodSchedule, at time.Time) (*model.AIUsagePeriod, error) {
	if _, err := f.ClosePeriod(ctx, periodID, at); err != nil {
		return nil, err
	}
	return f.OpenNextPeriod(ctx, schedule)
}

func (f *fakePeriodWorkerStore) ClosePeriod(_ context.Context, periodID string, _ time.Time) (*model.AIUsageSettlement, error) {
	if err := f.closeErr[periodID]; err != nil {
		return nil, err
	}
	f.closed = periodID
	return nil, nil
}

func (f *fakePeriodWorkerStore) OpenNextPeriod(_ context.Context, schedule eerepository.AIUsagePeriodSchedule) (*model.AIUsagePeriod, error) {
	if err := f.openErr[schedule.WorkspaceID]; err != nil {
		return nil, err
	}
	f.opened = schedule
	return &model.AIUsagePeriod{}, nil
}

func TestAIUsagePeriodWorkerContinuesAfterWorkspaceFailure(t *testing.T) {
	for _, stage := range []string{"close", "schedule", "open"} {
		t.Run(stage, func(t *testing.T) {
			now := time.Now().UTC()
			failure := errors.New("workspace unavailable")
			store := &fakePeriodWorkerStore{due: []model.AIUsagePeriod{
				{ID: "bad", WorkspaceID: "bad", PeriodEnd: now},
				{ID: "good", WorkspaceID: "good", PeriodEnd: now},
			}, closeErr: map[string]error{}, openErr: map[string]error{}}
			if stage == "close" {
				store.closeErr["bad"] = failure
			}
			if stage == "open" {
				store.openErr["bad"] = failure
			}
			worker := NewAIUsagePeriodWorker(store, func(_ context.Context, workspaceID string, start time.Time) (eerepository.AIUsagePeriodSchedule, error) {
				if stage == "schedule" && workspaceID == "bad" {
					return eerepository.AIUsagePeriodSchedule{}, failure
				}
				return eerepository.AIUsagePeriodSchedule{WorkspaceID: workspaceID, Start: start, End: start.AddDate(0, 1, 0)}, nil
			})
			count, err := worker.CloseDuePeriods(context.Background(), now, 10)
			if !errors.Is(err, failure) {
				t.Fatalf("lost rollover error: %v", err)
			}
			if count != 1 || store.opened.WorkspaceID != "good" {
				t.Fatalf("one workspace blocked another: count=%d opened=%+v", count, store.opened)
			}
		})
	}
}
