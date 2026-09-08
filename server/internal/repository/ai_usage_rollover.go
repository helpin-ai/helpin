package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrAIUsagePeriodExpired asks callers to retry after allowance renewal.
var ErrAIUsagePeriodExpired = errors.New("AI usage allowance is renewing; please retry")

// Serialize reservation changes and rollover before acquiring individual row
// locks. Both API replicas and the worker use this workspace-scoped lock.
func lockAIUsageWorkspace(tx *gorm.DB, workspaceID string) error {
	if tx.Dialector.Name() != "postgres" {
		return nil
	}
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "ai-usage:"+workspaceID).Error
}

func lockAIUsageReservationWorkspace(tx *gorm.DB, reservationID string) error {
	var reservation model.AIUsageReservation
	if err := tx.Select("workspace_id").First(&reservation, "id = ?", reservationID).Error; err != nil {
		return err
	}
	return lockAIUsageWorkspace(tx, reservation.WorkspaceID)
}

// Rebinding changes only future usage attribution. Previously posted ledger
// entries and settled invoices retain their original period and pricing.
func rebindAIUsageReservation(tx *gorm.DB, reservation *model.AIUsageReservation) error {
	var previous model.AIUsagePeriod
	if err := tx.First(&previous, "id = ?", reservation.PeriodID).Error; err != nil {
		return err
	}
	if previous.Status == model.AIUsagePeriodOpen {
		return nil
	}
	var current model.AIUsagePeriod
	if err := tx.Where("workspace_id = ? AND status = ?", reservation.WorkspaceID, model.AIUsagePeriodOpen).First(&current).Error; err != nil {
		return err
	}
	if reservation.Status == model.AIUsageReservationActive && reservation.ReservedMicrousd != 0 {
		return fmt.Errorf("closed AI usage period retains a reservation hold")
	}
	if err := tx.Model(reservation).Update("period_id", current.ID).Error; err != nil {
		return err
	}
	reservation.PeriodID = current.ID
	return nil
}

// RolloverPeriod atomically closes a due allowance, opens its successor, and
// transfers outstanding holds. Agent execution never delays monthly renewal.
func (r *AIUsageRepository) RolloverPeriod(ctx context.Context, periodID string, schedule AIUsagePeriodSchedule) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockAIUsageWorkspace(tx, schedule.WorkspaceID); err != nil {
			return err
		}
		var previous model.AIUsagePeriod
		if err := tx.First(&previous, "id = ? AND workspace_id = ?", periodID, schedule.WorkspaceID).Error; err != nil {
			return err
		}
		var current model.AIUsagePeriod
		err := tx.Where("workspace_id = ? AND status = ?", schedule.WorkspaceID, model.AIUsagePeriodOpen).First(&current).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && current.ID != previous.ID {
			return nil
		} // another replica won
		if previous.PeriodEnd.After(time.Now().UTC()) {
			return fmt.Errorf("AI usage period is not due for renewal")
		}
		if schedule.Start.Before(previous.PeriodEnd) {
			return fmt.Errorf("successor overlaps previous AI usage period")
		}
		var holds int64
		if err := tx.Model(&model.AIUsageReservation{}).Where("period_id = ? AND status = ?", previous.ID, model.AIUsageReservationActive).
			Select("COALESCE(SUM(reserved_microusd), 0)").Scan(&holds).Error; err != nil {
			return err
		}
		if previous.Status == model.AIUsagePeriodOpen {
			if holds != previous.ReservedMicrousd {
				return fmt.Errorf("AI usage reservation counter differs from active holds")
			}
			if err := tx.Model(&previous).Update("reserved_microusd", 0).Error; err != nil {
				return err
			}
			if _, err := NewAIUsageRepository(tx).ClosePeriod(ctx, previous.WorkspaceID, previous.PeriodEnd); err != nil {
				return err
			}
		}
		next, err := NewAIUsageRepository(tx).OpenNextPeriod(ctx, schedule)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.AIUsageReservation{}).Where("period_id = ? AND status = ?", previous.ID, model.AIUsageReservationActive).
			Update("period_id", next.ID).Error; err != nil {
			return err
		}
		return tx.Model(next).Update("reserved_microusd", holds).Error
	})
}

// ListDuePeriodsAfter uses a stable cursor so failing workspaces cannot starve
// later batches. Only the latest orphaned closed period needs a successor.
func (r *AIUsageRepository) ListDuePeriodsAfter(ctx context.Context, now time.Time, after *model.AIUsagePeriod, limit int) ([]model.AIUsagePeriod, error) {
	query := r.db.WithContext(ctx).Where("(status = ? AND period_end <= ?) OR (status = ? AND NOT EXISTS (SELECT 1 FROM billing_ai_usage_periods successor WHERE successor.workspace_id = billing_ai_usage_periods.workspace_id AND (successor.status = ? OR successor.period_end > billing_ai_usage_periods.period_end)))",
		model.AIUsagePeriodOpen, now, model.AIUsagePeriodClosed, model.AIUsagePeriodOpen)
	if after != nil {
		query = query.Where("period_end > ? OR (period_end = ? AND id > ?)", after.PeriodEnd, after.PeriodEnd, after.ID)
	}
	var periods []model.AIUsagePeriod
	err := query.Order("period_end, id").Limit(limit).Find(&periods).Error
	return periods, err
}
