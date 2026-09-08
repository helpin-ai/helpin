package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// ClosePeriod snapshots exact overage for asynchronous settlement and closes the period.
func (r *AIUsageRepository) ClosePeriod(ctx context.Context, workspaceID string, at time.Time) (*model.AIUsageSettlement, error) {
	var settlement *model.AIUsageSettlement
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockAIUsageWorkspace(tx, workspaceID); err != nil {
			return err
		}
		var period model.AIUsagePeriod
		query := tx.Where("workspace_id = ? AND status = ? AND period_end <= ?", workspaceID, model.AIUsagePeriodOpen, at)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Order("period_end, id").First(&period).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("load due AI usage period: %w", err)
			}
			if err := tx.Where("workspace_id = ? AND status = ?", workspaceID, model.AIUsagePeriodClosed).
				Order("period_end DESC").First(&period).Error; err != nil {
				return fmt.Errorf("load closed AI usage period: %w", err)
			}
			var existing model.AIUsageSettlement
			if err := tx.Where("period_id = ?", period.ID).First(&existing).Error; err == nil {
				settlement = &existing
			}
			return nil
		}
		if period.ReservedMicrousd != 0 {
			return fmt.Errorf("AI usage period has active reservations")
		}
		if err := tx.Model(&period).Update("status", model.AIUsagePeriodClosed).Error; err != nil {
			return fmt.Errorf("close AI usage period: %w", err)
		}
		if period.EnforcementMode != model.AIUsageEnforcementExtra || period.OverageMicrousd <= 0 {
			return nil
		}
		key := fmt.Sprintf("ai-usage:%s:%s:%s:v1", workspaceID, period.ID, period.PricingVersion)
		var existing model.AIUsageSettlement
		if err := tx.Where("idempotency_key = ?", key).First(&existing).Error; err == nil {
			settlement = &existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		cents, adjustment, err := aiusage.RoundMicrousdToCents(period.OverageMicrousd)
		if err != nil {
			return err
		}
		created := model.AIUsageSettlement{
			ID: uuid.NewString(), WorkspaceID: workspaceID, PeriodID: period.ID,
			ExactOverageMicrousd: period.OverageMicrousd, RoundedInvoiceCents: cents,
			RoundingAdjustmentMicrousd: adjustment, IdempotencyKey: key, Status: model.AIUsageSettlementPending,
		}
		var billing model.WorkspaceBilling
		if err := tx.Select("stripe_customer_id", "stripe_subscription_id").Where("workspace_id = ?", workspaceID).First(&billing).Error; err == nil {
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
	period := model.AIUsagePeriod{
		ID: uuid.NewString(), WorkspaceID: input.WorkspaceID, PeriodStart: input.Start, PeriodEnd: input.End,
		AllowanceMicrousd: input.AllowanceMicrousd, EnforcementMode: input.EnforcementMode,
		Status: model.AIUsagePeriodOpen, PricingVersion: input.PricingVersion,
	}
	if err := r.db.WithContext(ctx).Create(&period).Error; err != nil {
		return nil, fmt.Errorf("open AI usage period: %w", err)
	}
	return &period, nil
}

// ListDuePeriods returns open periods ready to close in stable order.
func (r *AIUsageRepository) ListDuePeriods(ctx context.Context, now time.Time, limit int) ([]model.AIUsagePeriod, error) {
	return r.ListDuePeriodsAfter(ctx, now, nil, limit)
}
