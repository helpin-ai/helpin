package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeBillingGateway struct {
	blocks []BillingCreditBlockCharge
}

func (g *fakeBillingGateway) CreateCheckoutSession(ctx context.Context, input BillingCheckoutInput) (string, error) {
	return "https://checkout.stripe.test/session", nil
}

func (g *fakeBillingGateway) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	return "https://billing.stripe.test/session", nil
}

func (g *fakeBillingGateway) BillCreditBlock(ctx context.Context, input BillingCreditBlockCharge) error {
	g.blocks = append(g.blocks, input)
	return nil
}

func (g *fakeBillingGateway) EnsureCustomer(ctx context.Context, orgID, email string) (string, error) {
	return "cus_test", nil
}

func (g *fakeBillingGateway) CreateSetupIntent(ctx context.Context, customerID string) (string, error) {
	return "seti_secret_test", nil
}

func (g *fakeBillingGateway) ListPaymentMethods(ctx context.Context, customerID string) ([]StripePaymentMethod, error) {
	return nil, nil
}

func (g *fakeBillingGateway) DetachPaymentMethod(ctx context.Context, paymentMethodID string) error {
	return nil
}

func (g *fakeBillingGateway) SetDefaultPaymentMethod(ctx context.Context, customerID, paymentMethodID string) error {
	return nil
}

func (g *fakeBillingGateway) ListInvoices(ctx context.Context, customerID string) ([]StripeInvoice, error) {
	return nil, nil
}

func newBillingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:billing_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE workspace_billing (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL UNIQUE,
			plan TEXT NOT NULL DEFAULT 'free',
			status TEXT NOT NULL DEFAULT 'active',
			stripe_customer_id TEXT,
			stripe_subscription_id TEXT,
			stripe_price_id TEXT,
			billing_interval TEXT NOT NULL DEFAULT 'monthly',
			included_credits INTEGER NOT NULL DEFAULT 1000,
			credits_used INTEGER NOT NULL DEFAULT 0,
			on_demand_enabled BOOLEAN NOT NULL DEFAULT 0,
			on_demand_blocks_invoiced INTEGER NOT NULL DEFAULT 0,
			current_period_start DATETIME NOT NULL,
			current_period_end DATETIME NOT NULL,
			trial_ends_at DATETIME,
			last_stripe_event_id TEXT,
			payment_method_id TEXT,
			billing_owner_user_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE billing_credit_ledger (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			feature_key TEXT NOT NULL DEFAULT '',
			credits INTEGER NOT NULL,
			idempotency_key TEXT NOT NULL UNIQUE,
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME
		)`,
		`CREATE TABLE stripe_webhook_events (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			processed BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create billing table: %v", err)
		}
	}
	return db
}

func TestBillingServiceEnsureTrialForWorkspaceStartsGrowthTrial(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	summary, err := svc.EnsureTrialForWorkspace(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("ensure trial: %v", err)
	}

	if summary.Plan != model.BillingPlanGrowth || summary.Status != model.BillingStatusTrialing {
		t.Fatalf("unexpected plan/status: %s/%s", summary.Plan, summary.Status)
	}
	if !summary.Trialing || summary.TrialEndsAt == nil {
		t.Fatalf("expected active trial, got %#v", summary)
	}
	if got := summary.TrialEndsAt.Sub(now); got != 14*24*time.Hour {
		t.Fatalf("trial duration = %s, want 336h", got)
	}
	if summary.IncludedCredits != 25000 || summary.CreditsRemaining != 25000 {
		t.Fatalf("credits = %d remaining %d, want 25000", summary.IncludedCredits, summary.CreditsRemaining)
	}
}

func TestBillingServiceDowngradesExpiredUnpaidTrialToFree(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	past := now.Add(-time.Hour)
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:        "workspace-1",
		Plan:               model.BillingPlanGrowth,
		Status:             model.BillingStatusTrialing,
		IncludedCredits:    25000,
		CreditsUsed:        125,
		CurrentPeriodStart: now.Add(-15 * 24 * time.Hour),
		CurrentPeriodEnd:   past,
		TrialEndsAt:        &past,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.GetWorkspaceBilling(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("get billing: %v", err)
	}

	if summary.Plan != model.BillingPlanFree || summary.Status != model.BillingStatusActive {
		t.Fatalf("unexpected plan/status after trial expiry: %s/%s", summary.Plan, summary.Status)
	}
	if summary.IncludedCredits != 1000 || summary.CreditsUsed != 0 {
		t.Fatalf("credits after downgrade = included %d used %d, want 1000/0", summary.IncludedCredits, summary.CreditsUsed)
	}
}

func TestBillingServiceConsumeCreditsRejectsWhenOnDemandDisabled(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:        "workspace-1",
		Plan:               model.BillingPlanStarter,
		Status:             model.BillingStatusActive,
		IncludedCredits:    5000,
		CreditsUsed:        4995,
		CurrentPeriodStart: now.Add(-24 * time.Hour),
		CurrentPeriodEnd:   now.Add(30 * 24 * time.Hour),
		OnDemandEnabled:    false,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	_, err := svc.ConsumeCredits(context.Background(), BillingCreditConsumption{
		WorkspaceID:    "workspace-1",
		FeatureKey:     BillingFeatureSupportAIReply,
		Credits:        10,
		IdempotencyKey: "reply-1",
	})
	if err == nil {
		t.Fatal("expected credit exhaustion error")
	}
}

func TestBillingServiceConsumeCreditsBillsOnDemandBlocks(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	gateway := &fakeBillingGateway{}
	svc := NewBillingService(repo, gateway, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:            "workspace-1",
		Plan:                   model.BillingPlanStarter,
		Status:                 model.BillingStatusActive,
		StripeCustomerID:       billingStringPtr("cus_123"),
		StripeSubscriptionID:   billingStringPtr("sub_123"),
		IncludedCredits:        5000,
		CreditsUsed:            4995,
		CurrentPeriodStart:     now.Add(-24 * time.Hour),
		CurrentPeriodEnd:       now.Add(30 * 24 * time.Hour),
		OnDemandEnabled:        true,
		OnDemandBlocksInvoiced: 0,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ConsumeCredits(context.Background(), BillingCreditConsumption{
		WorkspaceID:    "workspace-1",
		FeatureKey:     BillingFeatureSupportAIReply,
		Credits:        10,
		IdempotencyKey: "reply-1",
	})
	if err != nil {
		t.Fatalf("consume credits: %v", err)
	}

	if summary.CreditsUsed != 5005 || len(gateway.blocks) != 1 {
		t.Fatalf("credits used = %d, billed blocks = %d; want 5005/1", summary.CreditsUsed, len(gateway.blocks))
	}
	if gateway.blocks[0].WorkspaceID != "workspace-1" || gateway.blocks[0].Blocks != 1 {
		t.Fatalf("unexpected block charge: %#v", gateway.blocks[0])
	}
}

func billingStringPtr(v string) *string { return &v }
