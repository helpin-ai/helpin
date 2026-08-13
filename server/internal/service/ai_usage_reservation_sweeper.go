package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AIUsageReservationRecoveryStore provides execution-aware stale-hold recovery.
type AIUsageReservationRecoveryStore interface {
	ListStaleReservations(context.Context, time.Time, int) ([]model.AIUsageReservation, error)
	IsExecutionActive(context.Context, string) (bool, error)
	Release(context.Context, string, string) error
}

type AIUsageReservationSweeper struct {
	store AIUsageReservationRecoveryStore
}

func NewAIUsageReservationSweeper(store AIUsageReservationRecoveryStore) *AIUsageReservationSweeper {
	return &AIUsageReservationSweeper{store: store}
}

// Sweep releases a stale hold only after its durable execution is terminal or absent.
func (w *AIUsageReservationSweeper) Sweep(ctx context.Context, before time.Time, limit int) (int, error) {
	reservations, err := w.store.ListStaleReservations(ctx, before, limit)
	if err != nil {
		return 0, err
	}
	released := 0
	for _, reservation := range reservations {
		active, err := w.store.IsExecutionActive(ctx, reservation.ExecutionID)
		if err != nil {
			return released, fmt.Errorf("verify reservation %s execution: %w", reservation.ID, err)
		}
		if active {
			continue
		}
		if err := w.store.Release(ctx, reservation.ID, "stale_terminal_or_absent_execution"); err != nil {
			return released, fmt.Errorf("release stale reservation %s: %w", reservation.ID, err)
		}
		released++
	}
	return released, nil
}

func (w *AIUsageReservationSweeper) Run(ctx context.Context, interval, staleAfter time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	if staleAfter <= 0 {
		staleAfter = 15 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := w.Sweep(ctx, time.Now().UTC().Add(-staleAfter), 100); err != nil {
			slog.Error("AI usage reservation sweeper failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
