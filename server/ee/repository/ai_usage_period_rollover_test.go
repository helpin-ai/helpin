//go:build ee

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAIUsageRolloverPreservesOutstandingReservations(t *testing.T) {
	for _, mode := range []string{model.AIUsageEnforcementSoft, model.AIUsageEnforcementStrict, model.AIUsageEnforcementExtra} {
		t.Run(mode, func(t *testing.T) {
			repo := setupAIUsageRepository(t, mode, 1_000_000)
			ctx := context.Background()
			reservation, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "old-run", ReservedMicrousd: 500_000})
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			if err := repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Update("period_end", now).Error; err != nil {
				t.Fatal(err)
			}
			settlement, err := repo.ClosePeriod(ctx, "period", now)
			if err != nil {
				t.Fatalf("rollover with outstanding reservation: %v", err)
			}
			if settlement != nil {
				t.Fatal("settled before outstanding usage was reconciled")
			}
			var old model.AIUsagePeriod
			if err := repo.db.First(&old, "id = ?", "period").Error; err != nil {
				t.Fatal(err)
			}
			if old.Status != "closing" || old.ReservedMicrousd != 500_000 {
				t.Fatalf("old period = %+v", old)
			}
			schedule := AIUsagePeriodSchedule{WorkspaceID: "ws", Start: now, End: now.AddDate(0, 1, 0), AllowanceMicrousd: 1_000_000, EnforcementMode: mode, PricingVersion: "2026-08-13"}
			next, err := repo.OpenNextPeriod(ctx, schedule)
			if err != nil {
				t.Fatal(err)
			}
			again, err := repo.OpenNextPeriod(ctx, schedule)
			if err != nil || again.ID != next.ID {
				t.Fatalf("retry created a different period: %+v, %v", again, err)
			}
			fresh, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "new-run", ReservedMicrousd: 100_000})
			if err != nil {
				t.Fatal(err)
			}
			if fresh.PeriodID != next.ID {
				t.Fatal("new usage reserved against expired period")
			}
			due, err := repo.ListDuePeriods(ctx, now, 10)
			if err != nil || len(due) != 0 {
				t.Fatalf("pending old reservation should await reconciliation: %+v %v", due, err)
			}
			reconciled, err := repo.Reconcile(ctx, AIUsageReconcileRequest{ReservationID: reservation.ID, ChargedMicrousd: 1_200_000, Entry: model.AIUsageLedgerEntry{IdempotencyKey: "old-run-final"}})
			if err != nil {
				t.Fatal(err)
			}
			if reconciled.ID != "period" || reconciled.ReservedMicrousd != 0 {
				t.Fatalf("reconciliation moved or lost old usage: %+v", reconciled)
			}
			due, err = repo.ListDuePeriods(ctx, now, 10)
			if err != nil || len(due) != 1 || due[0].ID != "period" {
				t.Fatalf("drained period not ready for settlement: %+v %v", due, err)
			}
			settlement, err = repo.ClosePeriod(ctx, "period", now)
			if err != nil {
				t.Fatal(err)
			}
			if mode == model.AIUsageEnforcementExtra {
				if settlement == nil || settlement.ExactOverageMicrousd != 200_000 {
					t.Fatalf("settlement = %+v", settlement)
				}
			} else if settlement != nil {
				t.Fatalf("unexpected settlement: %+v", settlement)
			}
			if err := repo.db.First(&old, "id = ?", "period").Error; err != nil {
				t.Fatal(err)
			}
			if old.Status != model.AIUsagePeriodClosed {
				t.Fatalf("drained period status = %s", old.Status)
			}
			if err := repo.db.First(next, "id = ?", next.ID).Error; err != nil {
				t.Fatal(err)
			}
			if next.UsedMicrousd != 0 || next.ReservedMicrousd != 100_000 {
				t.Fatalf("old reconciliation affected new allowance: %+v", next)
			}
		})
	}
}

func TestAIUsageListDuePeriodsRecoversMissingSuccessor(t *testing.T) {
	for _, status := range []string{"closing", model.AIUsagePeriodClosed} {
		t.Run(status, func(t *testing.T) {
			repo := setupAIUsageRepository(t, model.AIUsageEnforcementSoft, 1_000_000)
			now := time.Now().UTC()
			if err := repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Updates(map[string]any{"period_end": now, "status": status, "reserved_microusd": 100}).Error; err != nil {
				t.Fatal(err)
			}
			due, err := repo.ListDuePeriods(context.Background(), now, 10)
			if err != nil || len(due) != 1 {
				t.Fatalf("missing successor not retried: %+v %v", due, err)
			}
		})
	}
}

func TestAIUsageRolloverWaitsForZeroHoldExecutionBeforeSettlement(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementExtra, 1_000_000)
	ctx := context.Background()
	hold, err := repo.Reserve(ctx, AIUsageReservationRequest{WorkspaceID: "ws", IdempotencyKey: "zero-hold"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Updates(map[string]any{
		"period_end": now, "used_microusd": 1_200_000, "overage_microusd": 200_000,
	}).Error; err != nil {
		t.Fatal(err)
	}
	settlement, err := repo.ClosePeriod(ctx, "period", now)
	if err != nil || settlement != nil {
		t.Fatalf("settled while execution is still active: %+v %v", settlement, err)
	}
	next, err := repo.OpenNextPeriod(ctx, AIUsagePeriodSchedule{WorkspaceID: "ws", Start: now, End: now.AddDate(0, 1, 0), AllowanceMicrousd: 1_000_000, EnforcementMode: model.AIUsageEnforcementExtra, PricingVersion: "2026-08-13"})
	if err != nil {
		t.Fatal(err)
	}
	due, err := repo.ListDuePeriods(ctx, now, 10)
	if err != nil || len(due) != 0 {
		t.Fatalf("active zero hold keeps retrying: %+v %v", due, err)
	}
	if err := repo.Release(ctx, hold.ID, "execution_finished"); err != nil {
		t.Fatal(err)
	}
	settlement, err = repo.ClosePeriod(ctx, "period", now)
	if err != nil || settlement == nil || settlement.ExactOverageMicrousd != 200_000 {
		t.Fatalf("settlement = %+v %v", settlement, err)
	}
	retry, err := repo.ClosePeriod(ctx, "period", now)
	if err != nil || retry == nil || retry.ID != settlement.ID {
		t.Fatalf("settlement retry = %+v %v", retry, err)
	}
	var count int64
	if err := repo.db.Model(&model.AIUsageSettlement{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("settlement count = %d", count)
	}
	// A delayed rollover retry after its successor has itself closed must not
	// recreate the old month, even while there is no open period.
	if err := repo.db.Model(next).Update("status", model.AIUsagePeriodClosed).Error; err != nil {
		t.Fatal(err)
	}
	retryPeriod, err := repo.OpenNextPeriod(ctx, AIUsagePeriodSchedule{WorkspaceID: "ws", Start: now, End: now.AddDate(0, 1, 0), PricingVersion: "2026-08-13"})
	if err != nil || retryPeriod.ID != next.ID {
		t.Fatalf("historical successor recreated: %+v %v", retryPeriod, err)
	}
}

func TestAIUsageRolloverRollsBackWhenSuccessorCannotOpen(t *testing.T) {
	repo := setupAIUsageRepository(t, model.AIUsageEnforcementSoft, 1_000_000)
	now := time.Now().UTC()
	if err := repo.db.Model(&model.AIUsagePeriod{}).Where("id = ?", "period").Update("period_end", now).Error; err != nil {
		t.Fatal(err)
	}
	_, err := repo.RolloverPeriod(context.Background(), "period", AIUsagePeriodSchedule{WorkspaceID: "ws", Start: now, End: now}, now)
	if err == nil {
		t.Fatal("invalid successor accepted")
	}
	var period model.AIUsagePeriod
	if err := repo.db.First(&period, "id = ?", "period").Error; err != nil {
		t.Fatal(err)
	}
	if period.Status != model.AIUsagePeriodOpen {
		t.Fatalf("failed rollover left no open allowance: %s", period.Status)
	}
}
