//go:build ee

package bootstrap

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/helpin-ai/helpin/server/ee/billingstripe"
	"github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/ee/service"
)

func workers(usage *repository.AIUsageRepository, billing *service.BillingService, gateway *billingstripe.Gateway) func(context.Context) {
	return func(ctx context.Context) {
		var group sync.WaitGroup
		start := func(run func()) { group.Add(1); go func() { defer group.Done(); run() }() }
		if gateway != nil {
			start(func() { service.NewAIUsageSettlementWorker(usage, gateway).Run(ctx, time.Minute) })
		}
		start(func() { service.NewAIUsagePeriodWorker(usage, billing.NextAIUsagePeriodSchedule).Run(ctx, time.Minute) })
		start(func() { service.NewAIUsageReservationSweeper(usage).Run(ctx, time.Minute, 15*time.Minute) })
		start(func() { expireTrials(ctx, billing) })
		group.Wait()
	}
}
func expireTrials(ctx context.Context, billing *service.BillingService) {
	sweep := func() {
		count, err := billing.ExpireOverdueTrials(ctx)
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("billing trial expiry sweep failed", "error", err)
			}
			return
		}
		if count > 0 {
			slog.Info("billing trial expiry sweep complete", "expired_count", count)
		}
	}
	sweep()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweep()
		}
	}
}
