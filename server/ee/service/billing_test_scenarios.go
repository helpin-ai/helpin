//go:build ee

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	BillingTestScenarioResetStarter        = "reset_starter"
	BillingTestScenarioTrialCap            = "trial_cap"
	BillingTestScenarioPastDueGrace        = "past_due_grace"
	BillingTestScenarioUnpaidLocked        = "unpaid_locked"
	BillingTestScenarioStarterAICap        = "starter_ai_cap"
	BillingTestScenarioStarterOnDemand     = "starter_on_demand"
	BillingTestScenarioStarterDocsLimit    = "starter_docs_limit"
	BillingTestScenarioStarterContactsOver = "starter_contacts_over_limit"

	billingTestSource       = "billing_test_scenario"
	billingTestActorID      = "00000000-0000-4000-8000-000000000001"
	billingTestDocsSlug     = "billing-test-scenarios"
	billingTestDocsTitle    = "[Billing test]"
	billingTestContactsGoal = 5001
	billingTestDocsGoal     = 500
)

type BillingTestScenarioService struct {
	db      *gorm.DB
	billing *BillingService
	now     func() time.Time
}

func NewBillingTestScenarioService(db *gorm.DB, billing *BillingService, now func() time.Time) *BillingTestScenarioService {
	if now == nil {
		now = time.Now
	}
	return &BillingTestScenarioService{db: db, billing: billing, now: now}
}

func (s *BillingTestScenarioService) Apply(ctx context.Context, workspaceID, scenario string) (*BillingSummary, error) {
	if s == nil || s.db == nil || s.billing == nil {
		return nil, fmt.Errorf("billing test scenarios are not configured")
	}
	scenario = strings.TrimSpace(scenario)
	if scenario == "" {
		return nil, fmt.Errorf("scenario is required")
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch scenario {
		case BillingTestScenarioResetStarter:
			if err := s.clearTestRows(ctx, tx, workspaceID); err != nil {
				return err
			}
			return s.applyBilling(ctx, tx, workspaceID, model.BillingPlanStarter, model.BillingStatusActive, 0, false, false, nil)
		case BillingTestScenarioTrialCap:
			trialEnds := s.now().UTC().Add(7 * 24 * time.Hour)
			return s.applyBilling(ctx, tx, workspaceID, model.BillingPlanGrowth, model.BillingStatusTrialing, 25000, false, false, &trialEnds)
		case BillingTestScenarioPastDueGrace:
			failedAt := s.now().UTC()
			return s.applyBilling(ctx, tx, workspaceID, model.BillingPlanGrowth, model.BillingStatusPastDue, 3000, false, true, nil, withBillingNotice("payment_failed", "Payment failed. Update your payment method to keep this workspace active.", &failedAt))
		case BillingTestScenarioUnpaidLocked:
			failedAt := s.now().UTC().Add(-3 * 24 * time.Hour)
			return s.applyBilling(ctx, tx, workspaceID, model.BillingPlanGrowth, model.BillingStatusUnpaid, 3000, false, true, nil, withBillingNotice("payment_failed", "Your subscription is unpaid. Update your payment method to reactivate this workspace.", &failedAt))
		case BillingTestScenarioStarterAICap:
			return s.applyBilling(ctx, tx, workspaceID, model.BillingPlanStarter, model.BillingStatusActive, 5000, false, false, nil)
		case BillingTestScenarioStarterOnDemand:
			return s.applyBilling(ctx, tx, workspaceID, model.BillingPlanStarter, model.BillingStatusActive, 4995, true, true, nil)
		case BillingTestScenarioStarterDocsLimit:
			if err := s.applyBilling(ctx, tx, workspaceID, model.BillingPlanStarter, model.BillingStatusActive, 0, false, false, nil); err != nil {
				return err
			}
			return s.seedDocs(ctx, tx, workspaceID, billingTestDocsGoal)
		case BillingTestScenarioStarterContactsOver:
			if err := s.applyBilling(ctx, tx, workspaceID, model.BillingPlanStarter, model.BillingStatusActive, 0, false, false, nil); err != nil {
				return err
			}
			return s.seedContacts(ctx, tx, workspaceID, billingTestContactsGoal)
		default:
			return fmt.Errorf("unknown billing test scenario %q", scenario)
		}
	}); err != nil {
		return nil, err
	}

	return s.billing.GetWorkspaceBilling(ctx, workspaceID)
}

type billingScenarioOption func(*model.WorkspaceBilling)

func withBillingNotice(noticeType, message string, at *time.Time) billingScenarioOption {
	return func(b *model.WorkspaceBilling) {
		b.BillingNoticeType = &noticeType
		b.BillingNoticeMessage = &message
		b.BillingNoticeAt = at
		b.PaymentFailedAt = at
	}
}

func (s *BillingTestScenarioService) applyBilling(ctx context.Context, tx *gorm.DB, workspaceID, plan, status string, creditsUsed int, onDemand bool, fakeStripe bool, trialEnds *time.Time, opts ...billingScenarioOption) error {
	now := s.now().UTC()
	periodEnd := now.AddDate(0, 1, 0)
	billing := &model.WorkspaceBilling{
		ID:                     uuid.NewString(),
		WorkspaceID:            workspaceID,
		Plan:                   plan,
		Status:                 status,
		BillingInterval:        "monthly",
		IncludedCredits:        includedCreditsForPlan(plan),
		CreditsUsed:            creditsUsed,
		OnDemandEnabled:        onDemand,
		OnDemandBlocksInvoiced: 0,
		CurrentPeriodStart:     now,
		CurrentPeriodEnd:       periodEnd,
		TrialEndsAt:            trialEnds,
	}
	if fakeStripe {
		customerID := "cus_billing_test_" + strings.ReplaceAll(workspaceID, "-", "")
		subscriptionID := "sub_billing_test_" + strings.ReplaceAll(workspaceID, "-", "")
		priceID := "price_billing_test"
		billing.StripeCustomerID = &customerID
		billing.StripeSubscriptionID = &subscriptionID
		billing.StripePriceID = &priceID
	}
	for _, opt := range opts {
		opt(billing)
	}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workspace_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"plan", "status", "stripe_customer_id", "stripe_subscription_id", "stripe_price_id",
			"billing_interval", "included_credits", "credits_used", "on_demand_enabled",
			"on_demand_blocks_invoiced", "current_period_start", "current_period_end", "trial_ends_at",
			"pending_plan", "pending_billing_interval", "pending_change_at", "cancel_at_period_end",
			"canceled_at", "billing_notice_type", "billing_notice_message", "billing_notice_at",
			"payment_failed_at", "trial_will_end_at", "updated_at",
		}),
	}).Create(billing).Error; err != nil {
		return fmt.Errorf("apply billing test scenario: %w", err)
	}
	return nil
}

func (s *BillingTestScenarioService) clearTestRows(ctx context.Context, tx *gorm.DB, workspaceID string) error {
	if err := tx.WithContext(ctx).Unscoped().Where("workspace_id = ? AND source = ?", workspaceID, billingTestSource).Delete(&model.CRMContact{}).Error; err != nil {
		return fmt.Errorf("clear billing test contacts: %w", err)
	}
	if err := tx.WithContext(ctx).Unscoped().Where("workspace_id = ? AND title LIKE ?", workspaceID, billingTestDocsTitle+"%").Delete(&model.DocsDocument{}).Error; err != nil {
		return fmt.Errorf("clear billing test documents: %w", err)
	}
	if err := tx.WithContext(ctx).Unscoped().Where("workspace_id = ? AND slug = ?", workspaceID, billingTestDocsSlug).Delete(&model.DocsSpace{}).Error; err != nil {
		return fmt.Errorf("clear billing test docs space: %w", err)
	}
	return nil
}

func (s *BillingTestScenarioService) seedDocs(ctx context.Context, tx *gorm.DB, workspaceID string, target int) error {
	space := model.DocsSpace{
		ID:          billingTestUUID(workspaceID, "docs-space"),
		WorkspaceID: workspaceID,
		Name:        "Billing test scenarios",
		Slug:        billingTestDocsSlug,
		Visibility:  "workspace_wide",
		Type:        "internal",
		IsSystem:    true,
		CreatedBy:   billingTestActorID,
	}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"workspace_id", "name", "slug", "visibility", "type", "is_system", "updated_at"}),
	}).Create(&space).Error; err != nil {
		return fmt.Errorf("seed billing test docs space: %w", err)
	}

	var existing int64
	if err := tx.WithContext(ctx).Model(&model.DocsDocument{}).
		Where("workspace_id = ? AND title LIKE ?", workspaceID, billingTestDocsTitle+"%").
		Count(&existing).Error; err != nil {
		return fmt.Errorf("count billing test documents: %w", err)
	}
	if int(existing) >= target {
		return nil
	}

	docs := make([]model.DocsDocument, 0, target-int(existing))
	for i := int(existing) + 1; i <= target; i++ {
		docs = append(docs, model.DocsDocument{
			ID:          billingTestUUID(workspaceID, fmt.Sprintf("doc-%d", i)),
			WorkspaceID: workspaceID,
			SpaceID:     space.ID,
			Title:       fmt.Sprintf("%s document %03d", billingTestDocsTitle, i),
			Status:      "draft",
			Visibility:  "workspace_wide",
			CreatedBy:   billingTestActorID,
		})
	}
	if err := tx.WithContext(ctx).CreateInBatches(docs, 250).Error; err != nil {
		return fmt.Errorf("seed billing test documents: %w", err)
	}
	return nil
}

func (s *BillingTestScenarioService) seedContacts(ctx context.Context, tx *gorm.DB, workspaceID string, target int) error {
	var existing int64
	if err := tx.WithContext(ctx).Model(&model.CRMContact{}).
		Where("workspace_id = ? AND source = ?", workspaceID, billingTestSource).
		Count(&existing).Error; err != nil {
		return fmt.Errorf("count billing test contacts: %w", err)
	}
	if int(existing) >= target {
		return nil
	}

	source := billingTestSource
	contacts := make([]model.CRMContact, 0, target-int(existing))
	for i := int(existing) + 1; i <= target; i++ {
		email := fmt.Sprintf("billing-test-%05d@example.invalid", i)
		contacts = append(contacts, model.CRMContact{
			ID:               billingTestUUID(workspaceID, fmt.Sprintf("contact-%d", i)),
			WorkspaceID:      workspaceID,
			DisplayID:        fmt.Sprintf("BILL-TEST-%05d", i),
			FirstName:        fmt.Sprintf("Billing Test %05d", i),
			Email:            &email,
			LifecycleStage:   model.CRMLifecycleSubscriber,
			LeadStatus:       model.CRMLeadStatusNew,
			Source:           &source,
			CustomProperties: model.JSONB{"billing_test": true},
			EmailStatus:      model.CRMContactEmailStatusValid,
		})
	}
	if err := tx.WithContext(ctx).CreateInBatches(contacts, 500).Error; err != nil {
		return fmt.Errorf("seed billing test contacts: %w", err)
	}
	return nil
}

func billingTestUUID(workspaceID, key string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("billing-test:"+workspaceID+":"+key)).String()
}
