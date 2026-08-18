package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AIUsagePeriodStore is the transaction boundary for allowance rollover.
type AIUsagePeriodStore interface {
	ListDuePeriods(context.Context, time.Time, int) ([]model.AIUsagePeriod, error)
	ClosePeriod(context.Context, string, time.Time) (*model.AIUsageSettlement, error)
	OpenNextPeriod(context.Context, repository.AIUsagePeriodSchedule) (*model.AIUsagePeriod, error)
}

// AIUsagePeriodScheduleResolver derives the next plan allowance and anniversary window.
type AIUsagePeriodScheduleResolver func(context.Context, string, time.Time) (repository.AIUsagePeriodSchedule, error)

// AIUsagePeriodWorker rolls periods independently from Stripe settlement.
type AIUsagePeriodWorker struct {
	store    AIUsagePeriodStore
	schedule AIUsagePeriodScheduleResolver
}

// NewAIUsagePeriodWorker creates a rollover worker.
func NewAIUsagePeriodWorker(store AIUsagePeriodStore, schedule AIUsagePeriodScheduleResolver) *AIUsagePeriodWorker {
	return &AIUsagePeriodWorker{store: store, schedule: schedule}
}

// CloseDuePeriods closes due periods and then opens successors separately.
func (w *AIUsagePeriodWorker) CloseDuePeriods(ctx context.Context, now time.Time, limit int) (int, error) {
	periods, err := w.store.ListDuePeriods(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, period := range periods {
		if _, err := w.store.ClosePeriod(ctx, period.WorkspaceID, now); err != nil {
			return processed, fmt.Errorf("close AI usage period %s: %w", period.ID, err)
		}
		schedule, err := w.schedule(ctx, period.WorkspaceID, period.PeriodEnd)
		if err != nil {
			return processed, fmt.Errorf("schedule next AI usage period: %w", err)
		}
		if _, err := w.store.OpenNextPeriod(ctx, schedule); err != nil {
			return processed, fmt.Errorf("open next AI usage period: %w", err)
		}
		processed++
	}
	return processed, nil
}

// Run checks due periods until cancellation.
func (w *AIUsagePeriodWorker) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := w.CloseDuePeriods(ctx, time.Now().UTC(), 100); err != nil {
			slog.Error("AI usage period worker failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
