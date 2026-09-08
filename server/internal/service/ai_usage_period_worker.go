package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AIUsagePeriodStore is the transaction boundary for allowance rollover.
type AIUsagePeriodStore interface {
	ListDuePeriodsAfter(context.Context, time.Time, *model.AIUsagePeriod, int) ([]model.AIUsagePeriod, error)
	RolloverPeriod(context.Context, string, repository.AIUsagePeriodSchedule) error
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

// CloseDuePeriods renews due allowances atomically and isolates workspace failures.
func (w *AIUsagePeriodWorker) CloseDuePeriods(ctx context.Context, now time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	processed := 0
	var failures []error
	var cursor *model.AIUsagePeriod
	for {
		if err := ctx.Err(); err != nil {
			return processed, errors.Join(append(failures, err)...)
		}
		periods, err := w.store.ListDuePeriodsAfter(ctx, now, cursor, limit)
		if err != nil {
			return processed, errors.Join(append(failures, err)...)
		}
		for _, period := range periods {
			schedule, err := currentAIUsageSchedule(ctx, w.schedule, period, now)
			if err == nil {
				err = w.store.RolloverPeriod(ctx, period.ID, schedule)
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("roll over AI usage period %s: %w", period.ID, err))
				continue
			}
			processed++
		}
		if len(periods) < limit {
			break
		}
		last := periods[len(periods)-1]
		cursor = &last
	}
	return processed, errors.Join(failures...)
}

// Catch up missed renewals without granting an obsolete allowance window.
func currentAIUsageSchedule(ctx context.Context, resolve AIUsagePeriodScheduleResolver, period model.AIUsagePeriod, now time.Time) (repository.AIUsagePeriodSchedule, error) {
	start := period.PeriodEnd
	for attempts := 0; attempts < 1200; attempts++ {
		schedule, err := resolve(ctx, period.WorkspaceID, start)
		if err != nil {
			return schedule, err
		}
		if schedule.WorkspaceID != period.WorkspaceID || !schedule.Start.Equal(start) || !schedule.End.After(start) {
			return schedule, fmt.Errorf("invalid AI usage renewal schedule")
		}
		if schedule.End.After(now) {
			return schedule, nil
		}
		start = schedule.End
	}
	return repository.AIUsagePeriodSchedule{}, fmt.Errorf("AI usage renewal exceeded catch-up limit")
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
