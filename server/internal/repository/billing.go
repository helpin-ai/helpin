package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type BillingRepository struct {
	db *gorm.DB
}

// StripeLifecycleMutation applies a billing change inside the webhook transaction
// and returns the lifecycle event to enqueue. A nil event marks the webhook
// processed without enqueueing (for example, when no billing row matches).
type StripeLifecycleMutation func(*BillingRepository) (*model.WorkspaceBilling, *CustomerIOLifecycleEventInput, error)

func NewBillingRepository(db *gorm.DB) *BillingRepository {
	return &BillingRepository{db: db}
}

// GetOpenAIUsagePeriod returns the current transactional allowance period.
func (r *BillingRepository) GetOpenAIUsagePeriod(ctx context.Context, workspaceID string) (*model.AIUsagePeriod, error) {
	if !r.db.Migrator().HasTable(&model.AIUsagePeriod{}) {
		return nil, nil
	}
	var period model.AIUsagePeriod
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND status = ?", workspaceID, model.AIUsagePeriodOpen).
		Order("period_start DESC").First(&period).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get open AI usage period: %w", err)
	}
	return &period, nil
}

// EnsureOpenAIUsagePeriod creates the first token-priced allowance for a
// workspace created after the common cutover. Existing periods are returned.
func (r *BillingRepository) EnsureOpenAIUsagePeriod(ctx context.Context, input AIUsagePeriodSchedule) (*model.AIUsagePeriod, error) {
	if !r.db.Migrator().HasTable(&model.AIUsagePeriod{}) {
		return nil, nil
	}
	var period model.AIUsagePeriod
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockAIUsageWorkspace(tx, input.WorkspaceID); err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND status = ?", input.WorkspaceID, model.AIUsagePeriodOpen).First(&period).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		period = model.AIUsagePeriod{
			ID: uuid.NewString(), WorkspaceID: input.WorkspaceID, PeriodStart: input.Start, PeriodEnd: input.End,
			AllowanceMicrousd: input.AllowanceMicrousd, EnforcementMode: input.EnforcementMode,
			Status: model.AIUsagePeriodOpen, PricingVersion: input.PricingVersion,
		}
		return tx.Create(&period).Error
	})
	if err != nil {
		return nil, fmt.Errorf("ensure open AI usage period: %w", err)
	}
	return &period, nil
}

// UpdateOpenAIUsageControls applies immediate upgrades and extra-usage toggle
// changes without ever reducing the current period allowance.
func (r *BillingRepository) UpdateOpenAIUsageControls(ctx context.Context, period *model.AIUsagePeriod, allowanceMicrousd int64, enforcementMode string) error {
	if period == nil {
		return nil
	}
	updates := map[string]any{"enforcement_mode": enforcementMode}
	if allowanceMicrousd > period.AllowanceMicrousd {
		updates["allowance_microusd"] = allowanceMicrousd
		period.AllowanceMicrousd = allowanceMicrousd
	}
	period.EnforcementMode = enforcementMode
	return r.db.WithContext(ctx).Model(&model.AIUsagePeriod{}).Where("id = ? AND status = ?", period.ID, model.AIUsagePeriodOpen).Updates(updates).Error
}

// ProcessStripeLifecycleEvent atomically deduplicates a Stripe webhook, applies
// its billing mutation, snapshots recipients, enqueues Customer.io delivery,
// and marks the webhook processed.
func (r *BillingRepository) ProcessStripeLifecycleEvent(
	ctx context.Context,
	eventID string,
	eventType string,
	mutate StripeLifecycleMutation,
) (*model.WorkspaceBilling, bool, error) {
	var billing *model.WorkspaceBilling
	processed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if eventID != "" {
			inserted := &model.StripeWebhookEvent{ID: eventID, Type: eventType}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).Create(inserted).Error; err != nil {
				return fmt.Errorf("insert stripe webhook event: %w", err)
			}
			query := tx.Where("id = ?", eventID)
			if tx.Dialector.Name() == "postgres" {
				query = query.Clauses(clause.Locking{Strength: "UPDATE"})
			}
			var webhook model.StripeWebhookEvent
			if err := query.First(&webhook).Error; err != nil {
				return fmt.Errorf("lock stripe webhook event: %w", err)
			}
			if webhook.Processed {
				return nil
			}
		}

		txRepo := &BillingRepository{db: tx}
		var lifecycle *CustomerIOLifecycleEventInput
		var err error
		billing, lifecycle, err = mutate(txRepo)
		if err != nil {
			return err
		}
		if lifecycle != nil {
			if _, err := enqueueCustomerIOLifecycleEventTx(ctx, tx, *lifecycle); err != nil {
				return err
			}
		}
		if eventID != "" {
			if err := tx.Model(&model.StripeWebhookEvent{}).Where("id = ?", eventID).Update("processed", true).Error; err != nil {
				return fmt.Errorf("mark stripe webhook processed: %w", err)
			}
		}
		processed = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return billing, processed, nil
}

// CreateTrialWithLifecycleEvent inserts a new trial and its lifecycle event atomically.
func (r *BillingRepository) CreateTrialWithLifecycleEvent(
	ctx context.Context,
	billing *model.WorkspaceBilling,
	event CustomerIOLifecycleEventInput,
) (*model.WorkspaceBilling, bool, error) {
	if billing.ID == "" {
		billing.ID = uuid.NewString()
	}
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "workspace_id"}},
			DoNothing: true,
		}).Create(billing)
		if result.Error != nil {
			return fmt.Errorf("create workspace trial: %w", result.Error)
		}
		created = result.RowsAffected == 1
		if !created {
			return nil
		}
		if _, err := enqueueCustomerIOLifecycleEventTx(ctx, tx, event); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if !created {
		existing, err := r.GetByWorkspaceID(ctx, billing.WorkspaceID)
		return existing, false, err
	}
	return billing, true, nil
}

// ExpireOverdueTrialsWithLifecycleEvents expires eligible trials and enqueues events atomically.
func (r *BillingRepository) ExpireOverdueTrialsWithLifecycleEvents(ctx context.Context, now time.Time) (int64, error) {
	var expired int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidates []model.WorkspaceBilling
		if err := tx.Where(
			"status = ? AND trial_ends_at IS NOT NULL AND trial_ends_at <= ? AND stripe_subscription_id IS NULL",
			model.BillingStatusTrialing, now,
		).Order("workspace_id ASC").Find(&candidates).Error; err != nil {
			return fmt.Errorf("list overdue billing trials: %w", err)
		}
		for i := range candidates {
			result := tx.Model(&model.WorkspaceBilling{}).
				Where("workspace_id = ? AND status = ? AND trial_ends_at <= ? AND stripe_subscription_id IS NULL",
					candidates[i].WorkspaceID, model.BillingStatusTrialing, now).
				Updates(map[string]any{
					"status": model.BillingStatusTrialExpired, "on_demand_enabled": false,
					"on_demand_blocks_invoiced": 0, "updated_at": now,
				})
			if result.Error != nil {
				return fmt.Errorf("expire workspace trial %q: %w", candidates[i].WorkspaceID, result.Error)
			}
			if result.RowsAffected == 0 {
				continue
			}
			eventAt := candidates[i].TrialEndsAt.UTC()
			if _, err := enqueueCustomerIOLifecycleEventTx(ctx, tx, CustomerIOLifecycleEventInput{
				SemanticKey: fmt.Sprintf("trial_expired:%s:%d", candidates[i].WorkspaceID, eventAt.Unix()),
				WorkspaceID: candidates[i].WorkspaceID,
				EventName:   "trial_expired",
				OccurredAt:  eventAt,
				Attributes:  map[string]any{"expired_at": now, "plan": candidates[i].Plan, "trial_ends_at": eventAt},
			}); err != nil {
				return err
			}
			expired++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("expire overdue trials with Customer.io events: %w", err)
	}
	return expired, nil
}

func (r *BillingRepository) GetByWorkspaceID(ctx context.Context, workspaceID string) (*model.WorkspaceBilling, error) {
	var billing model.WorkspaceBilling
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&billing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace billing: %w", err)
	}
	return &billing, nil
}

func (r *BillingRepository) WorkspaceHasFounderPlanOrganization(ctx context.Context, workspaceID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("workspaces AS w").
		Joins("JOIN organization_billing AS ob ON ob.organization_id = w.organization_id").
		Where("w.id = ? AND ob.founder_plan_enabled = ?", workspaceID, true).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check founder plan organization: %w", err)
	}
	return count > 0, nil
}

func (r *BillingRepository) GetByStripeSubscriptionID(ctx context.Context, subscriptionID string) (*model.WorkspaceBilling, error) {
	var billing model.WorkspaceBilling
	err := r.db.WithContext(ctx).Where("stripe_subscription_id = ?", subscriptionID).First(&billing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace billing by stripe subscription: %w", err)
	}
	return &billing, nil
}

func (r *BillingRepository) GetByStripeCustomerID(ctx context.Context, customerID string) (*model.WorkspaceBilling, error) {
	var rows []model.WorkspaceBilling
	err := r.db.WithContext(ctx).
		Where("stripe_customer_id = ?", customerID).
		Order("updated_at DESC").
		Limit(2).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("get workspace billing by stripe customer: %w", err)
	}
	if len(rows) != 1 {
		return nil, nil
	}
	return &rows[0], nil
}

func (r *BillingRepository) InsertStripeWebhookEvent(ctx context.Context, eventID, eventType string) (bool, error) {
	event := &model.StripeWebhookEvent{
		ID:   eventID,
		Type: eventType,
	}
	err := r.db.WithContext(ctx).Create(event).Error
	if err == nil {
		return true, nil
	}
	if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE constraint failed") {
		var existing model.StripeWebhookEvent
		if loadErr := r.db.WithContext(ctx).Where("id = ?", eventID).First(&existing).Error; loadErr != nil {
			return false, fmt.Errorf("load stripe webhook event: %w", loadErr)
		}
		return !existing.Processed, nil
	}
	return false, fmt.Errorf("insert stripe webhook event: %w", err)
}

func (r *BillingRepository) MarkStripeWebhookProcessed(ctx context.Context, eventID string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.StripeWebhookEvent{}).
		Where("id = ?", eventID).
		Update("processed", true).Error; err != nil {
		return fmt.Errorf("mark stripe webhook processed: %w", err)
	}
	return nil
}

func (r *BillingRepository) UpsertWorkspaceBilling(ctx context.Context, billing *model.WorkspaceBilling) error {
	if billing.ID == "" {
		billing.ID = uuid.NewString()
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "workspace_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"plan",
				"status",
				"stripe_customer_id",
				"stripe_subscription_id",
				"stripe_price_id",
				"billing_interval",
				"included_credits",
				"credits_used",
				"on_demand_enabled",
				"on_demand_blocks_invoiced",
				"current_period_start",
				"current_period_end",
				"trial_ends_at",
				"pending_plan",
				"pending_billing_interval",
				"pending_change_at",
				"cancel_at_period_end",
				"canceled_at",
				"billing_notice_type",
				"billing_notice_message",
				"billing_notice_at",
				"payment_failed_at",
				"trial_will_end_at",
				"last_stripe_event_id",
				"updated_at",
			}),
		}).
		Create(billing).Error; err != nil {
		return fmt.Errorf("upsert workspace billing: %w", err)
	}
	return nil
}

func (r *BillingRepository) UpdateWorkspaceBilling(ctx context.Context, billing *model.WorkspaceBilling) error {
	if err := r.db.WithContext(ctx).Save(billing).Error; err != nil {
		return fmt.Errorf("update workspace billing: %w", err)
	}
	return nil
}

func (r *BillingRepository) ExpireOverdueTrials(ctx context.Context, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&model.WorkspaceBilling{}).
		Where("status = ? AND trial_ends_at IS NOT NULL AND trial_ends_at <= ? AND stripe_subscription_id IS NULL", model.BillingStatusTrialing, now).
		Updates(map[string]interface{}{
			"status":                    model.BillingStatusTrialExpired,
			"on_demand_enabled":         false,
			"on_demand_blocks_invoiced": 0,
			"updated_at":                now,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("expire overdue billing trials: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *BillingRepository) ListOverdueTrialWorkspaceIDs(ctx context.Context, now time.Time) ([]string, error) {
	var workspaceIDs []string
	err := r.db.WithContext(ctx).
		Model(&model.WorkspaceBilling{}).
		Where("status = ? AND trial_ends_at IS NOT NULL AND trial_ends_at <= ? AND stripe_subscription_id IS NULL", model.BillingStatusTrialing, now).
		Pluck("workspace_id", &workspaceIDs).Error
	if err != nil {
		return nil, fmt.Errorf("list overdue billing trial workspaces: %w", err)
	}
	return workspaceIDs, nil
}

type BillingConsumeResult struct {
	Billing     *model.WorkspaceBilling
	AlreadyUsed bool
}

type BillingOnDemandChargeFunc func(billing *model.WorkspaceBilling, requiredBlocks, newBlocks int) error

func (r *BillingRepository) ConsumeCredits(ctx context.Context, workspaceID string, credits int, entry model.BillingCreditLedgerEntry, chargeOnDemand BillingOnDemandChargeFunc) (*BillingConsumeResult, error) {
	var result BillingConsumeResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.BillingCreditLedgerEntry
		err := tx.Where("idempotency_key = ?", entry.IdempotencyKey).First(&existing).Error
		if err == nil {
			var billing model.WorkspaceBilling
			if err := tx.Where("workspace_id = ?", workspaceID).First(&billing).Error; err != nil {
				return fmt.Errorf("reload billing after idempotent usage: %w", err)
			}
			result.Billing = &billing
			result.AlreadyUsed = true
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check billing ledger idempotency: %w", err)
		}

		var billing model.WorkspaceBilling
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ?", workspaceID).First(&billing).Error; err != nil {
			return fmt.Errorf("load workspace billing for usage: %w", err)
		}

		if billing.Status == model.BillingStatusTrialExpired || billing.Status == model.BillingStatusUnpaid || billing.Status == model.BillingStatusCanceled {
			return model.ErrBillingWorkspaceLocked
		}
		nextUsed := billing.CreditsUsed + credits
		onDemandAvailable := billing.Status == model.BillingStatusActive &&
			billing.StripeCustomerID != nil &&
			billing.StripeSubscriptionID != nil
		if nextUsed > billing.IncludedCredits && !billing.OnDemandEnabled {
			return model.ErrAIUsageExhausted
		}
		if nextUsed > billing.IncludedCredits && !onDemandAvailable {
			return model.ErrExtraAIUsageUnavailable
		}

		if nextUsed > billing.IncludedCredits {
			requiredBlocks := int(math.Ceil(float64(nextUsed-billing.IncludedCredits) / float64(5000)))
			newBlocks := requiredBlocks - billing.OnDemandBlocksInvoiced
			if newBlocks > 0 {
				if chargeOnDemand == nil {
					return model.ErrExtraAIUsageBillingUnconfigured
				}
				if err := chargeOnDemand(&billing, requiredBlocks, newBlocks); err != nil {
					return err
				}
				billing.OnDemandBlocksInvoiced += newBlocks
			}
		}

		billing.CreditsUsed = nextUsed
		if err := tx.Save(&billing).Error; err != nil {
			return fmt.Errorf("increment billing credits used: %w", err)
		}
		if entry.ID == "" {
			entry.ID = uuid.NewString()
		}
		if entry.WorkspaceID == "" {
			entry.WorkspaceID = workspaceID
		}
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("create billing ledger entry: %w", err)
		}
		result.Billing = &billing
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *BillingRepository) InsertLedgerEntry(ctx context.Context, entry *model.BillingCreditLedgerEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	if err := r.db.WithContext(ctx).Create(entry).Error; err != nil {
		return fmt.Errorf("insert billing ledger entry: %w", err)
	}
	return nil
}

// GetOrganizationBilling returns the org billing row, or nil if not present.
func (r *BillingRepository) GetOrganizationBilling(ctx context.Context, orgID string) (*model.OrganizationBilling, error) {
	var ob model.OrganizationBilling
	err := r.db.WithContext(ctx).Where("organization_id = ?", orgID).First(&ob).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get organization billing: %w", err)
	}
	return &ob, nil
}

// UpsertOrganizationBilling inserts or updates the org billing row keyed on
// organization_id.
func (r *BillingRepository) UpsertOrganizationBilling(ctx context.Context, ob *model.OrganizationBilling) error {
	if ob.ID == "" {
		ob.ID = uuid.NewString()
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "organization_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"stripe_customer_id",
				"default_payment_method_id",
				"updated_at",
			}),
		}).
		Create(ob).Error; err != nil {
		return fmt.Errorf("upsert organization billing: %w", err)
	}
	return nil
}

// PaymentMethodWithCount pairs a saved card with the number of workspaces
// linked to it.
type PaymentMethodWithCount struct {
	Method model.BillingPaymentMethod
	Linked int
}

// ListPaymentMethods returns all saved cards for an org with linked-workspace
// counts.
func (r *BillingRepository) ListPaymentMethods(ctx context.Context, orgID string) ([]PaymentMethodWithCount, error) {
	var methods []model.BillingPaymentMethod
	if err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Order("is_org_default DESC, created_at ASC").
		Find(&methods).Error; err != nil {
		return nil, fmt.Errorf("list payment methods: %w", err)
	}
	out := make([]PaymentMethodWithCount, 0, len(methods))
	for i := range methods {
		var count int64
		if err := r.db.WithContext(ctx).
			Model(&model.WorkspaceBilling{}).
			Where("payment_method_id = ?", methods[i].ID).
			Count(&count).Error; err != nil {
			return nil, fmt.Errorf("count linked workspaces: %w", err)
		}
		out = append(out, PaymentMethodWithCount{Method: methods[i], Linked: int(count)})
	}
	return out, nil
}

// GetPaymentMethod returns a saved card by id scoped to an org, or nil.
func (r *BillingRepository) GetPaymentMethod(ctx context.Context, orgID, cardID string) (*model.BillingPaymentMethod, error) {
	var pm model.BillingPaymentMethod
	err := r.db.WithContext(ctx).
		Where("id = ? AND organization_id = ?", cardID, orgID).
		First(&pm).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get payment method: %w", err)
	}
	return &pm, nil
}

// InsertPaymentMethod creates a saved card.
func (r *BillingRepository) InsertPaymentMethod(ctx context.Context, pm *model.BillingPaymentMethod) error {
	if pm.ID == "" {
		pm.ID = uuid.NewString()
	}
	if err := r.db.WithContext(ctx).Create(pm).Error; err != nil {
		return fmt.Errorf("insert payment method: %w", err)
	}
	return nil
}

// UpdatePaymentMethod persists editable card fields.
func (r *BillingRepository) UpdatePaymentMethod(ctx context.Context, pm *model.BillingPaymentMethod) error {
	if err := r.db.WithContext(ctx).Save(pm).Error; err != nil {
		return fmt.Errorf("update payment method: %w", err)
	}
	return nil
}

// SetDefaultPaymentMethod marks one card as the org default and clears the flag
// on all others, updating the org billing default pointer in one transaction.
func (r *BillingRepository) SetDefaultPaymentMethod(ctx context.Context, orgID, cardID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.BillingPaymentMethod{}).
			Where("organization_id = ?", orgID).
			Update("is_org_default", false).Error; err != nil {
			return fmt.Errorf("clear default cards: %w", err)
		}
		if err := tx.Model(&model.BillingPaymentMethod{}).
			Where("id = ? AND organization_id = ?", cardID, orgID).
			Update("is_org_default", true).Error; err != nil {
			return fmt.Errorf("set default card: %w", err)
		}
		if err := tx.Model(&model.OrganizationBilling{}).
			Where("organization_id = ?", orgID).
			Update("default_payment_method_id", cardID).Error; err != nil {
			return fmt.Errorf("set org default pointer: %w", err)
		}
		return nil
	})
}

// DeletePaymentMethod removes a saved card and unlinks any workspaces that
// pointed at it.
func (r *BillingRepository) DeletePaymentMethod(ctx context.Context, orgID, cardID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.WorkspaceBilling{}).
			Where("payment_method_id = ?", cardID).
			Update("payment_method_id", nil).Error; err != nil {
			return fmt.Errorf("unlink workspaces from card: %w", err)
		}
		if err := tx.Model(&model.OrganizationBilling{}).
			Where("organization_id = ? AND default_payment_method_id = ?", orgID, cardID).
			Update("default_payment_method_id", nil).Error; err != nil {
			return fmt.Errorf("clear org default on delete: %w", err)
		}
		if err := tx.Where("id = ? AND organization_id = ?", cardID, orgID).
			Delete(&model.BillingPaymentMethod{}).Error; err != nil {
			return fmt.Errorf("delete payment method: %w", err)
		}
		return nil
	})
}

// LinkWorkspacePaymentMethod sets (or clears) the card linked to a workspace.
func (r *BillingRepository) LinkWorkspacePaymentMethod(ctx context.Context, workspaceID string, paymentMethodID *string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.WorkspaceBilling{}).
		Where("workspace_id = ?", workspaceID).
		Update("payment_method_id", paymentMethodID).Error; err != nil {
		return fmt.Errorf("link workspace payment method: %w", err)
	}
	return nil
}

// OrgWorkspaceBilling pairs a workspace billing row with its workspace identity.
type OrgWorkspaceBilling struct {
	Billing       model.WorkspaceBilling
	WorkspaceID   string
	WorkspaceName string
	WorkspaceSlug string
}

// ListWorkspaceBillingsForOrg returns all workspace billing rows for the
// workspaces belonging to an organization, joined with workspace identity.
func (r *BillingRepository) ListWorkspaceBillingsForOrg(ctx context.Context, orgID string) ([]OrgWorkspaceBilling, error) {
	var workspaces []model.Workspace
	if err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Order("name ASC").
		Find(&workspaces).Error; err != nil {
		return nil, fmt.Errorf("list org workspaces: %w", err)
	}
	out := make([]OrgWorkspaceBilling, 0, len(workspaces))
	for i := range workspaces {
		row := OrgWorkspaceBilling{
			WorkspaceID:   workspaces[i].ID,
			WorkspaceName: workspaces[i].Name,
			WorkspaceSlug: workspaces[i].Slug,
		}
		var billing model.WorkspaceBilling
		err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaces[i].ID).First(&billing).Error
		if err == nil {
			row.Billing = billing
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("get billing for workspace %q: %w", workspaces[i].ID, err)
		}
		out = append(out, row)
	}
	return out, nil
}

// FindWorkspaceOrgID returns the organization id for a workspace, or empty
// string if unset.
func (r *BillingRepository) FindWorkspaceOrgID(ctx context.Context, workspaceID string) (string, error) {
	var ws model.Workspace
	err := r.db.WithContext(ctx).Select("organization_id").Where("id = ?", workspaceID).First(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find workspace org id: %w", err)
	}
	if ws.OrganizationID == nil {
		return "", nil
	}
	return *ws.OrganizationID, nil
}

// UsageLedgerRow is one aggregated ledger group by day + feature.
type UsageLedgerRow struct {
	Day                                                          string
	FeatureKey, ModelTier                                        string
	Entries, ActualEntries, EstimatedEntries                     int
	ChargedMicrousd                                              int64
	InputTokens, CacheReadTokens, CacheWriteTokens, OutputTokens int64
	ReasoningTokens                                              int64
}

// UsageByDayFeature groups usage ledger entries by day and feature for a
// allowance window, including late postings. Arbitrary windows use [start, end).
func (r *BillingRepository) UsageByDayFeature(ctx context.Context, workspaceID string, start, end time.Time) ([]UsageLedgerRow, error) {
	period, err := r.GetAIUsagePeriodByWindow(ctx, workspaceID, start, end)
	if err != nil {
		return nil, err
	}
	query := r.db.WithContext(ctx).Model(&model.AIUsageLedgerEntry{}).
		Where("workspace_id = ? AND entry_kind IN ?", workspaceID, []string{"usage", "estimate"})
	if period != nil {
		query = query.Where("period_id = ?", period.ID)
	} else {
		query = query.Where("created_at >= ? AND created_at < ?", start, end)
	}

	var rows []UsageLedgerRow
	if err := query.Select(`DATE(created_at) AS day, feature_key, model_tier, COUNT(*) AS entries,
			SUM(CASE WHEN measurement_status = 'actual' THEN 1 ELSE 0 END) AS actual_entries,
			SUM(CASE WHEN measurement_status = 'estimated' THEN 1 ELSE 0 END) AS estimated_entries,
			COALESCE(SUM(final_charged_microusd),0) AS charged_microusd,
			COALESCE(SUM(input_tokens_total),0) AS input_tokens,
			COALESCE(SUM(cache_read_tokens),0) AS cache_read_tokens,
			COALESCE(SUM(cache_write_tokens),0) AS cache_write_tokens,
			COALESCE(SUM(output_tokens),0) AS output_tokens,
			COALESCE(SUM(reasoning_tokens),0) AS reasoning_tokens`).
		Group("DATE(created_at), feature_key, model_tier").
		Order("day ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("aggregate usage by day feature: %w", err)
	}
	return rows, nil
}

// GetAIUsagePeriodByWindow resolves a billing-period report by its exact bounds.
func (r *BillingRepository) GetAIUsagePeriodByWindow(ctx context.Context, workspaceID string, start, end time.Time) (*model.AIUsagePeriod, error) {
	var period model.AIUsagePeriod
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND period_start = ? AND period_end = ?", workspaceID, start, end).First(&period).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &period, err
}

// RolloverAIUsagePeriod advances an allowance under the shared workspace lock.
func (r *BillingRepository) RolloverAIUsagePeriod(ctx context.Context, periodID string, schedule AIUsagePeriodSchedule) error {
	return NewAIUsageRepository(r.db).RolloverPeriod(ctx, periodID, schedule)
}

// ListAIUsagePeriods returns recent allowance windows for billing history.
func (r *BillingRepository) ListAIUsagePeriods(ctx context.Context, workspaceID string) ([]model.AIUsagePeriod, error) {
	var periods []model.AIUsagePeriod
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).
		Order("period_start DESC, id DESC").Limit(24).Find(&periods).Error
	return periods, err
}
