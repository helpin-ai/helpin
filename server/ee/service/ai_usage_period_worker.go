//go:build ee

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// AIUsagePeriodStore is the transaction boundary for allowance rollover.
type AIUsagePeriodStore interface {
	ListDuePeriods(context.Context, time.Time, int) ([]model.AIUsagePeriod, error)
	RolloverPeriod(context.Context, string, eerepository.AIUsagePeriodSchedule, time.Time) (*model.AIUsagePeriod, error)
}

// AIUsagePeriodScheduleResolver derives the next plan allowance and anniversary window.
type AIUsagePeriodScheduleResolver func(context.Context, string, time.Time) (eerepository.AIUsagePeriodSchedule, error)

// AIUsagePeriodWorker rolls periods independently from Stripe settlement.
type AIUsagePeriodWorker struct {
	store    AIUsagePeriodStore
	schedule AIUsagePeriodScheduleResolver
}

// NewAIUsagePeriodWorker creates a rollover worker.
func NewAIUsagePeriodWorker(store AIUsagePeriodStore, schedule AIUsagePeriodScheduleResolver) *AIUsagePeriodWorker {
	return &AIUsagePeriodWorker{store: store, schedule: schedule}
}

// CloseDuePeriods advances due allowances and finalizes drained periods.
func (w *AIUsagePeriodWorker) CloseDuePeriods(ctx context.Context, now time.Time, limit int) (int, error) {
	periods, err := w.store.ListDuePeriods(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	var failures []error
	for _, period := range periods {
		schedule, err := w.schedule(ctx, period.WorkspaceID, period.PeriodEnd)
		if err != nil {
			failures = append(failures, fmt.Errorf("schedule next AI usage period %s: %w", period.ID, err))
			continue
		}
		if _, err := w.store.RolloverPeriod(ctx, period.ID, schedule, now); err != nil {
			failures = append(failures, fmt.Errorf("roll over AI usage period %s: %w", period.ID, err))
			continue
		}
		processed++
	}
	return processed, errors.Join(failures...)
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
