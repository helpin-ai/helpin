//go:build ee

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/ee/pricing"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// RolloverPeriod retires an expired allowance and opens its successor atomically.
// Readers never observe a gap and create an allowance with a new anniversary.
func (r *AIUsageRepository) RolloverPeriod(ctx context.Context, periodID string, schedule AIUsagePeriodSchedule, at time.Time) (*model.AIUsagePeriod, error) {
	var next *model.AIUsagePeriod
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := NewAIUsageRepository(tx)
		if _, err := repo.ClosePeriod(ctx, periodID, at); err != nil {
			return err
		}
		var err error
		next, err = repo.OpenNextPeriod(ctx, schedule)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("roll over AI usage period: %w", err)
	}
	return next, nil
}

// ClosePeriod retires an expired allowance without discarding unfinished usage.
// A closing period accepts reconciliation, but cannot accept new reservations.
// Overage is snapshotted only once all active reservations have drained.
func (r *AIUsageRepository) ClosePeriod(ctx context.Context, periodID string, at time.Time) (*model.AIUsageSettlement, error) {
	var settlement *model.AIUsageSettlement
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var period model.AIUsagePeriod
		query := tx.Where("id = ? AND period_end <= ?", periodID, at)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&period).Error; err != nil {
			return fmt.Errorf("load due AI usage period: %w", err)
		}
		if period.Status == model.AIUsagePeriodClosed {
			err := tx.Where("period_id = ?", period.ID).First(&settlement).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				settlement = nil
				return nil
			}
			return err
		}
		var active int64
		if err := tx.Model(&model.AIUsageReservation{}).Where("period_id = ? AND status = ?", period.ID, model.AIUsageReservationActive).Count(&active).Error; err != nil {
			return fmt.Errorf("count outstanding AI usage reservations: %w", err)
		}
		if period.ReservedMicrousd != 0 || active != 0 {
			return tx.Model(&period).Update("status", model.AIUsagePeriodClosing).Error
		}
		if err := tx.Model(&period).Update("status", model.AIUsagePeriodClosed).Error; err != nil {
			return fmt.Errorf("close AI usage period: %w", err)
		}
		if period.EnforcementMode != model.AIUsageEnforcementExtra || period.OverageMicrousd <= 0 {
			return nil
		}
		key := fmt.Sprintf("ai-usage:%s:%s:%s:v1", period.WorkspaceID, period.ID, period.PricingVersion)
		var existing model.AIUsageSettlement
		if err := tx.Where("idempotency_key = ?", key).First(&existing).Error; err == nil {
			settlement = &existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		cents, adjustment, err := pricing.RoundMicrousdToCents(period.OverageMicrousd)
		if err != nil {
			return err
		}
		created := model.AIUsageSettlement{
			ID: uuid.NewString(), WorkspaceID: period.WorkspaceID, PeriodID: period.ID,
			ExactOverageMicrousd: period.OverageMicrousd, RoundedInvoiceCents: cents,
			RoundingAdjustmentMicrousd: adjustment, IdempotencyKey: key, Status: model.AIUsageSettlementPending,
		}
		var billing model.WorkspaceBilling
		if err := tx.Select("stripe_customer_id", "stripe_subscription_id").Where("workspace_id = ?", period.WorkspaceID).First(&billing).Error; err == nil {
			created.StripeCustomerID = billing.StripeCustomerID
			created.StripeSubscriptionID = billing.StripeSubscriptionID
		}
		if err := tx.Create(&created).Error; err != nil {
			return fmt.Errorf("create AI usage settlement: %w", err)
		}
		settlement = &created
		return nil
	})
	return settlement, err
}

// OpenNextPeriod creates a zero-used period independently of prior settlement work.
func (r *AIUsageRepository) OpenNextPeriod(ctx context.Context, input AIUsagePeriodSchedule) (*model.AIUsagePeriod, error) {
	if input.WorkspaceID == "" || input.PricingVersion == "" || !input.End.After(input.Start) || input.AllowanceMicrousd < 0 {
		return nil, fmt.Errorf("invalid AI usage period schedule")
	}
	var period model.AIUsagePeriod
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Retrying a drained historical period must not create another successor.
		lookup := func() error {
			return tx.Where("workspace_id = ? AND (status = ? OR period_start >= ?)", input.WorkspaceID, model.AIUsagePeriodOpen, input.Start).
				Order("period_start DESC").First(&period).Error
		}
		if err := lookup(); err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		period = model.AIUsagePeriod{
			ID: uuid.NewString(), WorkspaceID: input.WorkspaceID, PeriodStart: input.Start, PeriodEnd: input.End,
			AllowanceMicrousd: input.AllowanceMicrousd, EnforcementMode: input.EnforcementMode,
			Status: model.AIUsagePeriodOpen, PricingVersion: input.PricingVersion,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&period)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return lookup()
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("open AI usage period: %w", err)
	}
	return &period, nil
}

// ListDuePeriods returns expired allowances, drained periods, and interrupted rollovers.
func (r *AIUsageRepository) ListDuePeriods(ctx context.Context, now time.Time, limit int) ([]model.AIUsagePeriod, error) {
	var periods []model.AIUsagePeriod
	err := r.db.WithContext(ctx).Where(`period_end <= ? AND (
  status = ? OR
  (status = ? AND reserved_microusd = 0 AND NOT EXISTS (
   SELECT 1 FROM billing_ai_usage_reservations reservation
   WHERE reservation.period_id = billing_ai_usage_periods.id AND reservation.status = ?
  )) OR
  (status IN ? AND NOT EXISTS (
   SELECT 1 FROM billing_ai_usage_periods successor
   WHERE successor.workspace_id = billing_ai_usage_periods.workspace_id
    AND successor.period_start >= billing_ai_usage_periods.period_end
  ))
 )`, now, model.AIUsagePeriodOpen, model.AIUsagePeriodClosing, model.AIUsageReservationActive,
		[]string{model.AIUsagePeriodClosing, model.AIUsagePeriodClosed}).
		Order("period_end, id").Limit(limit).Find(&periods).Error
	return periods, err
}
