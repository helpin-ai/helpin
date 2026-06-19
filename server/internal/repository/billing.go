package repository

import (
	"context"
	"errors"
	"fmt"
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

func NewBillingRepository(db *gorm.DB) *BillingRepository {
	return &BillingRepository{db: db}
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
		return false, nil
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

type BillingConsumeResult struct {
	Billing     *model.WorkspaceBilling
	AlreadyUsed bool
}

func (r *BillingRepository) ConsumeCredits(ctx context.Context, workspaceID string, credits int, entry model.BillingCreditLedgerEntry) (*BillingConsumeResult, error) {
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

		billing.CreditsUsed += credits
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

// SetWorkspaceBillingOwner sets (or clears) the delegated billing owner.
func (r *BillingRepository) SetWorkspaceBillingOwner(ctx context.Context, workspaceID string, userID *string) error {
	if err := r.db.WithContext(ctx).
		Model(&model.WorkspaceBilling{}).
		Where("workspace_id = ?", workspaceID).
		Update("billing_owner_user_id", userID).Error; err != nil {
		return fmt.Errorf("set workspace billing owner: %w", err)
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
	Day        string
	FeatureKey string
	Entries    int
	Credits    int
}

// UsageByDayFeature groups usage ledger entries by day and feature for a
// workspace within [start, end).
func (r *BillingRepository) UsageByDayFeature(ctx context.Context, workspaceID string, start, end time.Time) ([]UsageLedgerRow, error) {
	var rows []UsageLedgerRow
	if err := r.db.WithContext(ctx).
		Model(&model.BillingCreditLedgerEntry{}).
		Select("to_char(created_at, 'YYYY-MM-DD') AS day, feature_key AS feature_key, COUNT(*) AS entries, COALESCE(SUM(credits),0) AS credits").
		Where("workspace_id = ? AND kind = ? AND created_at >= ? AND created_at < ?",
			workspaceID, model.BillingLedgerKindUsage, start, end).
		Group("day, feature_key").
		Order("day ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("aggregate usage by day feature: %w", err)
	}
	return rows, nil
}
