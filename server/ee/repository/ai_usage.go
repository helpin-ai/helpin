package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	corerepository "github.com/helpin-ai/helpin/server/internal/repository"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/ee/pricing"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// AIUsageRepository persists periods, reservations, and immutable usage entries.
type AIUsageRepository struct{ db *gorm.DB }

// AIUsageReservationRequest describes one idempotent allowance reservation.
type AIUsageReservationRequest struct {
	WorkspaceID, PeriodID, TaskNature, ModelTier, ExecutionID, IdempotencyKey string
	ReservedMicrousd                                                          int64
	ExpiresAt, HeartbeatAt                                                    time.Time
}

// AIUsageReconcileRequest describes an idempotent terminal usage posting.
type AIUsageReconcileRequest struct {
	ReservationID                     string
	Entry                             model.AIUsageLedgerEntry
	ChargedMicrousd, AbsorbedMicrousd int64
	AllowLateUsage                    bool
	RunID                             string
	RunOutputSummary                  model.JSONBlob
}

// ErrAIUsageWatermarkChanged requires reloading the run before retrying a delta.
var ErrAIUsageWatermarkChanged = corerepository.ErrAIUsageWatermarkChanged

// AIUsageCheckpointRequest describes an idempotent partial usage posting that
// keeps the execution reservation active for a later turn.
type AIUsageCheckpointRequest struct {
	ReservationID                     string
	Entry                             model.AIUsageLedgerEntry
	ChargedMicrousd, AbsorbedMicrousd int64
	RunID                             string
	RunOutputSummary                  model.JSONBlob
}

// AIUsagePeriodSchedule describes a new, empty allowance period.
type AIUsagePeriodSchedule struct {
	WorkspaceID, Plan, BillingInterval, PricingVersion, EnforcementMode string
	Anchor, Start, End                                                  time.Time
	AllowanceMicrousd                                                   int64
}

// NewAIUsageRepository creates an AI usage repository.
func NewAIUsageRepository(db *gorm.DB) *AIUsageRepository { return &AIUsageRepository{db: db} }

// Reserve atomically holds allowance for an execution.
func (r *AIUsageRepository) Reserve(ctx context.Context, input AIUsageReservationRequest) (*model.AIUsageReservation, error) {
	if input.WorkspaceID == "" || input.IdempotencyKey == "" || input.ReservedMicrousd <= 0 {
		return nil, fmt.Errorf("invalid AI usage reservation")
	}
	var reservation model.AIUsageReservation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("idempotency_key = ?", input.IdempotencyKey).First(&reservation).Error; err == nil {
			var period model.AIUsagePeriod
			if err := tx.Select("enforcement_mode").First(&period, "id = ?", reservation.PeriodID).Error; err != nil {
				return fmt.Errorf("load reserved AI usage period: %w", err)
			}
			reservation.EnforcementMode = period.EnforcementMode
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check AI usage reservation: %w", err)
		}
		query := tx.Where("workspace_id = ? AND status = ?", input.WorkspaceID, model.AIUsagePeriodOpen)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var period model.AIUsagePeriod
		if err := query.First(&period).Error; err != nil {
			return fmt.Errorf("load open AI usage period: %w", err)
		}
		if period.EnforcementMode == model.AIUsageEnforcementStrict && period.UsedMicrousd+period.ReservedMicrousd+input.ReservedMicrousd > period.AllowanceMicrousd {
			return model.ErrAIUsageExhausted
		}
		now := time.Now().UTC()
		if input.HeartbeatAt.IsZero() {
			input.HeartbeatAt = now
		}
		if input.ExpiresAt.IsZero() {
			input.ExpiresAt = now.Add(time.Hour)
		}
		reservation = model.AIUsageReservation{ID: uuid.NewString(), WorkspaceID: input.WorkspaceID, PeriodID: period.ID, TaskNature: input.TaskNature, ModelTier: input.ModelTier, ExecutionID: input.ExecutionID, IdempotencyKey: input.IdempotencyKey, ReservedMicrousd: input.ReservedMicrousd, Status: model.AIUsageReservationActive, HeartbeatAt: input.HeartbeatAt, ExpiresAt: input.ExpiresAt}
		reservation.EnforcementMode = period.EnforcementMode
		if err := tx.Create(&reservation).Error; err != nil {
			return fmt.Errorf("create AI usage reservation: %w", err)
		}
		if err := tx.Model(&period).Update("reserved_microusd", period.ReservedMicrousd+input.ReservedMicrousd).Error; err != nil {
			return fmt.Errorf("update AI usage period reservation: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

// RecordUncharged writes promotional or absorbed telemetry without consuming allowance.
func (r *AIUsageRepository) RecordUncharged(ctx context.Context, entry model.AIUsageLedgerEntry) error {
	if entry.WorkspaceID == "" || entry.IdempotencyKey == "" || entry.FinalChargedMicrousd != 0 {
		return fmt.Errorf("invalid uncharged AI usage entry")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.AIUsageLedgerEntry
		if err := tx.Where("idempotency_key = ?", entry.IdempotencyKey).First(&existing).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check uncharged AI usage entry: %w", err)
		}
		var period model.AIUsagePeriod
		if err := tx.Where("workspace_id = ? AND status = ?", entry.WorkspaceID, model.AIUsagePeriodOpen).First(&period).Error; err != nil {
			return fmt.Errorf("load open AI usage period: %w", err)
		}
		entry.ID = uuid.NewString()
		entry.PeriodID = period.ID
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("create uncharged AI usage entry: %w", err)
		}
		return nil
	})
}

// ResizeReservation changes an active hold while enforcing the period allowance.
func (r *AIUsageRepository) ResizeReservation(ctx context.Context, id string, targetMicrousd int64, heartbeat time.Time) error {
	if id == "" || targetMicrousd < 0 {
		return fmt.Errorf("invalid AI usage reservation resize")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var reservation model.AIUsageReservation
		query := tx.Where("id = ?", id)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&reservation).Error; err != nil {
			return fmt.Errorf("load AI usage reservation: %w", err)
		}
		if reservation.Status != model.AIUsageReservationActive {
			return nil
		}
		var period model.AIUsagePeriod
		periodQuery := tx.Where("id = ?", reservation.PeriodID)
		if tx.Dialector.Name() == "postgres" {
			periodQuery = periodQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := periodQuery.First(&period).Error; err != nil {
			return fmt.Errorf("load AI usage period: %w", err)
		}
		delta := targetMicrousd - reservation.ReservedMicrousd
		if period.EnforcementMode == model.AIUsageEnforcementStrict && period.UsedMicrousd+period.ReservedMicrousd+delta > period.AllowanceMicrousd {
			return model.ErrAIUsageExhausted
		}
		if heartbeat.IsZero() {
			heartbeat = time.Now().UTC()
		}
		if err := tx.Model(&reservation).Updates(map[string]any{"reserved_microusd": targetMicrousd, "heartbeat_at": heartbeat}).Error; err != nil {
			return fmt.Errorf("resize AI usage reservation: %w", err)
		}
		if err := tx.Model(&period).Update("reserved_microusd", period.ReservedMicrousd+delta).Error; err != nil {
			return fmt.Errorf("update AI usage period reservation: %w", err)
		}
		return nil
	})
}

// Reconcile atomically posts a ledger entry, consumes the hold, and updates its period.
func (r *AIUsageRepository) Reconcile(ctx context.Context, input AIUsageReconcileRequest) (*model.AIUsagePeriod, error) {
	if input.ReservationID == "" || input.Entry.IdempotencyKey == "" || input.ChargedMicrousd < 0 || input.AbsorbedMicrousd < 0 {
		return nil, fmt.Errorf("invalid AI usage reconciliation")
	}
	var period model.AIUsagePeriod
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.AIUsageLedgerEntry
		if err := tx.Where("idempotency_key = ?", input.Entry.IdempotencyKey).First(&existing).Error; err == nil {
			if input.AllowLateUsage && input.RunID != "" && (existing.InputTokensTotal != input.Entry.InputTokensTotal || existing.OutputTokens != input.Entry.OutputTokens || existing.ReasoningTokens != input.Entry.ReasoningTokens || existing.CacheReadTokens != input.Entry.CacheReadTokens) {
				return ErrAIUsageWatermarkChanged
			}
			if input.AllowLateUsage && input.RunID != "" {
				current, err := corerepository.LoadRunUsageWatermark(tx, existing.WorkspaceID, input.RunID)
				if err != nil {
					return err
				}
				requested, err := corerepository.ParseRunUsageWatermark(json.RawMessage(input.RunOutputSummary))
				if err != nil {
					return err
				}
				if current != requested {
					return ErrAIUsageWatermarkChanged
				}
			}
			return tx.First(&period, "id = ?", existing.PeriodID).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check AI usage ledger: %w", err)
		}

		var reservation model.AIUsageReservation
		reservationQuery := tx.Where("id = ?", input.ReservationID)
		if tx.Dialector.Name() == "postgres" {
			reservationQuery = reservationQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := reservationQuery.First(&reservation).Error; err != nil {
			return fmt.Errorf("load AI usage reservation: %w", err)
		}
		if reservation.Status != model.AIUsageReservationActive && !(input.AllowLateUsage &&
			(reservation.Status == model.AIUsageReservationReconciled || reservation.Status == model.AIUsageReservationReleased)) {
			return fmt.Errorf("AI usage reservation is %s", reservation.Status)
		}
		periodQuery := tx.Where("id = ?", reservation.PeriodID)
		if tx.Dialector.Name() == "postgres" {
			periodQuery = periodQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := periodQuery.First(&period).Error; err != nil {
			return fmt.Errorf("load AI usage period: %w", err)
		}

		entry := input.Entry
		entry.ID = uuid.NewString()
		entry.WorkspaceID = reservation.WorkspaceID
		entry.PeriodID = reservation.PeriodID
		entry.FinalChargedMicrousd = input.ChargedMicrousd
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("create AI usage ledger entry: %w", err)
		}
		period.UsedMicrousd += input.ChargedMicrousd
		if reservation.Status == model.AIUsageReservationActive {
			period.ReservedMicrousd -= reservation.ReservedMicrousd
		}
		if period.ReservedMicrousd < 0 {
			period.ReservedMicrousd = 0
		}
		if period.EnforcementMode == model.AIUsageEnforcementExtra && period.UsedMicrousd > period.AllowanceMicrousd {
			period.OverageMicrousd = period.UsedMicrousd - period.AllowanceMicrousd
		}
		if err := tx.Model(&period).Updates(map[string]any{
			"used_microusd": period.UsedMicrousd, "reserved_microusd": period.ReservedMicrousd,
			"overage_microusd": period.OverageMicrousd,
		}).Error; err != nil {
			return fmt.Errorf("update AI usage period: %w", err)
		}
		if err := tx.Model(&reservation).Updates(map[string]any{
			"consumed_microusd": reservation.ConsumedMicrousd + input.ChargedMicrousd,
			"status":            model.AIUsageReservationReconciled,
		}).Error; err != nil {
			return fmt.Errorf("reconcile AI usage reservation: %w", err)
		}
		if input.RunID != "" {
			result := tx.Model(&model.AgentRun{}).Where("id = ? AND workspace_id = ?", input.RunID, reservation.WorkspaceID).Update("output_summary", input.RunOutputSummary)
			if result.Error != nil {
				return fmt.Errorf("persist terminal usage watermark: %w", result.Error)
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("terminal usage run not found")
			}
		}
		return nil
	})
	if err != nil {
		// Another process may have posted this turn after our initial lookup.
		// Do not let its caller persist an older usage watermark over that commit.
		if input.AllowLateUsage && input.RunID != "" {
			var existing model.AIUsageLedgerEntry
			if lookupErr := r.db.WithContext(ctx).Where("idempotency_key = ?", input.Entry.IdempotencyKey).First(&existing).Error; lookupErr == nil {
				return nil, ErrAIUsageWatermarkChanged
			}
		}
		return nil, err
	}
	return &period, nil
}

// Checkpoint atomically posts partial usage while retaining the remainder of
// the reservation for a subsequent interactive turn.
func (r *AIUsageRepository) Checkpoint(ctx context.Context, input AIUsageCheckpointRequest) (*model.AIUsagePeriod, error) {
	if input.ReservationID == "" || input.Entry.IdempotencyKey == "" || input.ChargedMicrousd < 0 || input.AbsorbedMicrousd < 0 {
		return nil, fmt.Errorf("invalid AI usage checkpoint")
	}
	var period model.AIUsagePeriod
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.AIUsageLedgerEntry
		if err := tx.Where("idempotency_key = ?", input.Entry.IdempotencyKey).First(&existing).Error; err == nil {
			if input.RunID != "" {
				if existing.InputTokensTotal != input.Entry.InputTokensTotal || existing.OutputTokens != input.Entry.OutputTokens || existing.ReasoningTokens != input.Entry.ReasoningTokens || existing.CacheReadTokens != input.Entry.CacheReadTokens {
					return ErrAIUsageWatermarkChanged
				}
				current, err := corerepository.LoadRunUsageWatermark(tx, existing.WorkspaceID, input.RunID)
				if err != nil {
					return err
				}
				requested, err := corerepository.ParseRunUsageWatermark(json.RawMessage(input.RunOutputSummary))
				if err != nil {
					return err
				}
				if current != requested {
					return ErrAIUsageWatermarkChanged
				}
			}
			return tx.First(&period, "id = ?", existing.PeriodID).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check AI usage checkpoint ledger: %w", err)
		}

		var reservation model.AIUsageReservation
		reservationQuery := tx.Where("id = ?", input.ReservationID)
		if tx.Dialector.Name() == "postgres" {
			reservationQuery = reservationQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := reservationQuery.First(&reservation).Error; err != nil {
			return fmt.Errorf("load AI usage checkpoint reservation: %w", err)
		}
		if reservation.Status != model.AIUsageReservationActive {
			return fmt.Errorf("AI usage reservation is %s", reservation.Status)
		}

		periodQuery := tx.Where("id = ?", reservation.PeriodID)
		if tx.Dialector.Name() == "postgres" {
			periodQuery = periodQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := periodQuery.First(&period).Error; err != nil {
			return fmt.Errorf("load AI usage checkpoint period: %w", err)
		}
		if period.EnforcementMode == model.AIUsageEnforcementStrict && input.ChargedMicrousd > reservation.ReservedMicrousd {
			return model.ErrAIUsageExhausted
		}

		entry := input.Entry
		entry.ID = uuid.NewString()
		entry.WorkspaceID = reservation.WorkspaceID
		entry.PeriodID = reservation.PeriodID
		entry.FinalChargedMicrousd = input.ChargedMicrousd
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("create AI usage checkpoint ledger entry: %w", err)
		}

		releasedHold := min(input.ChargedMicrousd, reservation.ReservedMicrousd)
		remainingReservation := reservation.ReservedMicrousd - releasedHold
		if remainingReservation < 0 {
			remainingReservation = 0
		}
		period.UsedMicrousd += input.ChargedMicrousd
		period.ReservedMicrousd -= releasedHold
		if period.ReservedMicrousd < 0 {
			period.ReservedMicrousd = 0
		}
		if period.EnforcementMode == model.AIUsageEnforcementExtra && period.UsedMicrousd > period.AllowanceMicrousd {
			period.OverageMicrousd = period.UsedMicrousd - period.AllowanceMicrousd
		}
		if err := tx.Model(&period).Updates(map[string]any{
			"used_microusd": period.UsedMicrousd, "reserved_microusd": period.ReservedMicrousd,
			"overage_microusd": period.OverageMicrousd,
		}).Error; err != nil {
			return fmt.Errorf("update AI usage checkpoint period: %w", err)
		}
		if err := tx.Model(&reservation).Updates(map[string]any{
			"reserved_microusd": remainingReservation,
			"consumed_microusd": reservation.ConsumedMicrousd + input.ChargedMicrousd,
			"heartbeat_at":      time.Now().UTC(),
		}).Error; err != nil {
			return fmt.Errorf("update AI usage checkpoint reservation: %w", err)
		}
		if input.RunID != "" {
			if err := tx.Model(&model.AgentRun{}).Where("id = ?", input.RunID).
				Update("output_summary", input.RunOutputSummary).Error; err != nil {
				return fmt.Errorf("update AI usage checkpoint run summary: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &period, nil
}

// Release returns an active reservation to the period without posting usage.
func (r *AIUsageRepository) Release(ctx context.Context, id, reason string) error {
	if id == "" {
		return fmt.Errorf("invalid AI usage reservation release")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var reservation model.AIUsageReservation
		query := tx.Where("id = ?", id)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&reservation).Error; err != nil {
			return fmt.Errorf("load AI usage reservation: %w", err)
		}
		if reservation.Status != model.AIUsageReservationActive {
			return nil
		}
		var period model.AIUsagePeriod
		if err := tx.First(&period, "id = ?", reservation.PeriodID).Error; err != nil {
			return fmt.Errorf("load AI usage period: %w", err)
		}
		remaining := period.ReservedMicrousd - reservation.ReservedMicrousd
		if remaining < 0 {
			remaining = 0
		}
		if err := tx.Model(&period).Update("reserved_microusd", remaining).Error; err != nil {
			return fmt.Errorf("release AI usage period reservation: %w", err)
		}
		if err := tx.Model(&reservation).Update("status", model.AIUsageReservationReleased).Error; err != nil {
			return fmt.Errorf("release AI usage reservation (%s): %w", reason, err)
		}
		return nil
	})
}

// ClosePeriod snapshots exact overage for asynchronous settlement and closes the period.
func (r *AIUsageRepository) ClosePeriod(ctx context.Context, workspaceID string, at time.Time) (*model.AIUsageSettlement, error) {
	var settlement *model.AIUsageSettlement
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		cents, adjustment, err := pricing.RoundMicrousdToCents(period.OverageMicrousd)
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
	var periods []model.AIUsagePeriod
	err := r.db.WithContext(ctx).Where("(status = ? AND period_end <= ?) OR (status = ? AND NOT EXISTS (SELECT 1 FROM billing_ai_usage_periods successor WHERE successor.workspace_id = billing_ai_usage_periods.workspace_id AND successor.status = ?))",
		model.AIUsagePeriodOpen, now, model.AIUsagePeriodClosed, model.AIUsagePeriodOpen).
		Order("period_end, id").Limit(limit).Find(&periods).Error
	return periods, err
}

// ListStaleReservations identifies candidates for execution-aware recovery without releasing them.
func (r *AIUsageRepository) ListStaleReservations(ctx context.Context, before time.Time, limit int) ([]model.AIUsageReservation, error) {
	var reservations []model.AIUsageReservation
	err := r.db.WithContext(ctx).Where("status = ? AND heartbeat_at < ?", model.AIUsageReservationActive, before).
		Order("heartbeat_at, id").Limit(limit).Find(&reservations).Error
	return reservations, err
}

// IsExecutionActive confirms whether a reservation's durable agent run can
// still produce usage. A missing execution is safe for stale-hold recovery.
func (r *AIUsageRepository) IsExecutionActive(ctx context.Context, executionID string) (bool, error) {
	if executionID == "" {
		return false, nil
	}
	var run model.AgentRun
	err := r.db.WithContext(ctx).Select("id", "status").First(&run, "id = ?", executionID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check AI usage reservation execution: %w", err)
	}
	switch run.Status {
	case model.AgentRunStatusCompleted, model.AgentRunStatusFailed, model.AgentRunStatusCancelled:
		return false, nil
	default:
		return true, nil
	}
}

// ListPendingSettlements returns exact-overage work in stable creation order.
func (r *AIUsageRepository) ListPendingSettlements(ctx context.Context, limit int) ([]model.AIUsageSettlement, error) {
	var settlements []model.AIUsageSettlement
	err := r.db.WithContext(ctx).Table("billing_ai_usage_settlements s").
		Select("s.*, p.pricing_version, p.period_start, p.period_end, wb.billing_interval").
		Joins("JOIN billing_ai_usage_periods p ON p.id = s.period_id").
		Joins("JOIN workspace_billing wb ON wb.workspace_id = s.workspace_id").
		Where("s.status = ?", model.AIUsageSettlementPending).
		Order("s.created_at, s.id").Limit(limit).Scan(&settlements).Error
	return settlements, err
}

// CompleteSettlement records Stripe identities after an idempotent charge.
func (r *AIUsageRepository) CompleteSettlement(ctx context.Context, id, invoiceItemID, invoiceID string) error {
	updates := map[string]any{"status": "settled", "last_error": nil}
	if invoiceItemID != "" {
		updates["stripe_invoice_item_id"] = invoiceItemID
	}
	if invoiceID != "" {
		updates["stripe_invoice_id"] = invoiceID
	}
	return r.db.WithContext(ctx).Model(&model.AIUsageSettlement{}).Where("id = ? AND status = ?", id, model.AIUsageSettlementPending).Updates(updates).Error
}

// FailSettlement retains a pending row for retry with a sanitized failure summary.
func (r *AIUsageRepository) FailSettlement(ctx context.Context, id, message string) error {
	if len(message) > 500 {
		message = message[:500]
	}
	return r.db.WithContext(ctx).Model(&model.AIUsageSettlement{}).Where("id = ?", id).Updates(map[string]any{
		"attempt_count": gorm.Expr("attempt_count + 1"), "last_error": message,
	}).Error
}
