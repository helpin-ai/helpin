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
	blocks           []BillingCreditBlockCharge
	immediateChanges []BillingSubscriptionChangeInput
	scheduledChanges []BillingSubscriptionChangeInput
	cancellations    []BillingSubscriptionCancelInput
	immediateCancels []BillingSubscriptionCancelInput
	resumes          []BillingSubscriptionCancelInput
	cancelErr        error
}

func (g *fakeBillingGateway) CreateCheckoutSession(ctx context.Context, input BillingCheckoutInput) (string, error) {
	return "https://checkout.stripe.test/session", nil
}

func (g *fakeBillingGateway) RetrieveCheckoutSession(ctx context.Context, sessionID string) (*BillingCheckoutSession, error) {
	return &BillingCheckoutSession{ID: sessionID}, nil
}

func (g *fakeBillingGateway) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	return "https://billing.stripe.test/session", nil
}

func (g *fakeBillingGateway) PreviewSubscriptionPriceChange(ctx context.Context, input BillingSubscriptionChangeInput) (*BillingStripeInvoicePreview, error) {
	return &BillingStripeInvoicePreview{
		AmountDueCents: 4200,
		SubtotalCents:  4200,
		TotalCents:     4200,
		Currency:       "usd",
		Lines: []BillingStripeInvoicePreviewLine{{
			Description: "Proration",
			AmountCents: 4200,
			Proration:   true,
		}},
	}, nil
}

func (g *fakeBillingGateway) BillCreditBlock(ctx context.Context, input BillingCreditBlockCharge) error {
	g.blocks = append(g.blocks, input)
	return nil
}

func (g *fakeBillingGateway) UpdateSubscriptionPrice(ctx context.Context, input BillingSubscriptionChangeInput) error {
	g.immediateChanges = append(g.immediateChanges, input)
	return nil
}

func (g *fakeBillingGateway) ScheduleSubscriptionPriceChange(ctx context.Context, input BillingSubscriptionChangeInput) error {
	g.scheduledChanges = append(g.scheduledChanges, input)
	return nil
}

func (g *fakeBillingGateway) CancelSubscriptionAtPeriodEnd(ctx context.Context, input BillingSubscriptionCancelInput) error {
	if g.cancelErr != nil {
		return g.cancelErr
	}
	g.cancellations = append(g.cancellations, input)
	return nil
}

func (g *fakeBillingGateway) CancelSubscriptionImmediately(ctx context.Context, input BillingSubscriptionCancelInput) error {
	if g.cancelErr != nil {
		return g.cancelErr
	}
	g.immediateCancels = append(g.immediateCancels, input)
	return nil
}

func (g *fakeBillingGateway) ResumeSubscription(ctx context.Context, input BillingSubscriptionCancelInput) error {
	g.resumes = append(g.resumes, input)
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
			pending_plan TEXT,
			pending_billing_interval TEXT,
			pending_change_at DATETIME,
			cancel_at_period_end BOOLEAN NOT NULL DEFAULT 0,
			canceled_at DATETIME,
			billing_notice_type TEXT,
			billing_notice_message TEXT,
			billing_notice_at DATETIME,
			payment_failed_at DATETIME,
			trial_will_end_at DATETIME,
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

func TestBillingServiceConsumeCreditsRejectsWhenFreePlanOverSeatLimit(t *testing.T) {
	db := newBillingTestDB(t)
	if err := db.Exec(`CREATE TABLE workspace_members (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		user_id TEXT,
		email TEXT NOT NULL,
		display_name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'member',
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create workspace_members: %v", err)
	}
	repo := repository.NewBillingRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	svc.SetWorkspaceRepository(workspaceRepo)

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:        "workspace-1",
		Plan:               model.BillingPlanFree,
		Status:             model.BillingStatusActive,
		IncludedCredits:    1000,
		CreditsUsed:        100,
		CurrentPeriodStart: now.Add(-24 * time.Hour),
		CurrentPeriodEnd:   now.Add(30 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if err := db.Exec(`INSERT INTO workspace_members (id, workspace_id, user_id, email, display_name, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("wm-%d", i),
			"workspace-1",
			fmt.Sprintf("user-%d", i),
			fmt.Sprintf("user%d@example.com", i),
			fmt.Sprintf("User %d", i),
			model.RoleMember,
			model.WorkspaceMemberStatusActive,
			now,
			now,
		).Error; err != nil {
			t.Fatalf("seed member %d: %v", i, err)
		}
	}

	_, err := svc.ConsumeCredits(context.Background(), BillingCreditConsumption{
		WorkspaceID:    "workspace-1",
		FeatureKey:     BillingFeatureSupportAIReply,
		Credits:        10,
		IdempotencyKey: "reply-1",
	})
	if err == nil || err.Error() == "" {
		t.Fatal("expected over seat limit error")
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

func TestBillingServiceRejectsCheckoutForExistingPaidSubscription(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	svc.SetPriceConfig(BillingPriceConfig{GrowthMonthly: "price_growth_monthly"})

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		BillingInterval:      "monthly",
		IncludedCredits:      25000,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(30 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	_, err := svc.CreateCheckoutSession(context.Background(), BillingCheckoutRequest{
		WorkspaceID: "workspace-1",
		UserID:      "user-1",
		Email:       "owner@example.com",
		Plan:        model.BillingPlanGrowth,
		Interval:    "monthly",
	})

	if err == nil {
		t.Fatal("expected checkout to be rejected for existing subscription")
	}
}

func TestBillingServiceChangePlanUpgradesExistingSubscriptionImmediately(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	gateway := &fakeBillingGateway{}
	svc := NewBillingService(repo, gateway, func() time.Time { return now })
	svc.SetPriceConfig(BillingPriceConfig{GrowthMonthly: "price_growth_monthly"})

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanStarter,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		StripePriceID:        billingStringPtr("price_starter_monthly"),
		BillingInterval:      "monthly",
		IncludedCredits:      5000,
		CreditsUsed:          1000,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(30 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ChangeWorkspacePlan(context.Background(), BillingPlanChangeRequest{
		WorkspaceID: "workspace-1",
		Plan:        model.BillingPlanGrowth,
		Interval:    "monthly",
	})
	if err != nil {
		t.Fatalf("change plan: %v", err)
	}

	if len(gateway.immediateChanges) != 1 {
		t.Fatalf("immediate changes = %d, want 1", len(gateway.immediateChanges))
	}
	if got := gateway.immediateChanges[0]; got.SubscriptionID != "sub_123" || got.PriceID != "price_growth_monthly" {
		t.Fatalf("unexpected change input: %#v", got)
	}
	if summary.Plan != model.BillingPlanGrowth || summary.PendingPlan != nil {
		t.Fatalf("unexpected summary after upgrade: plan=%s pending=%v", summary.Plan, summary.PendingPlan)
	}
	if summary.IncludedCredits != 25000 || summary.CreditsUsed != 1000 {
		t.Fatalf("credits after upgrade = included %d used %d", summary.IncludedCredits, summary.CreditsUsed)
	}
}

func TestBillingServiceChangePlanDowngradesExistingSubscriptionImmediately(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	renewal := now.Add(30 * 24 * time.Hour)
	gateway := &fakeBillingGateway{}
	svc := NewBillingService(repo, gateway, func() time.Time { return now })
	svc.SetPriceConfig(BillingPriceConfig{StarterMonthly: "price_starter_monthly"})

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		StripePriceID:        billingStringPtr("price_growth_monthly"),
		BillingInterval:      "monthly",
		IncludedCredits:      25000,
		CreditsUsed:          12000,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     renewal,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ChangeWorkspacePlan(context.Background(), BillingPlanChangeRequest{
		WorkspaceID: "workspace-1",
		Plan:        model.BillingPlanStarter,
		Interval:    "monthly",
	})
	if err != nil {
		t.Fatalf("change plan: %v", err)
	}

	if len(gateway.immediateChanges) != 1 || len(gateway.scheduledChanges) != 0 {
		t.Fatalf("immediate=%d scheduled=%d, want 1/0", len(gateway.immediateChanges), len(gateway.scheduledChanges))
	}
	if got := gateway.immediateChanges[0]; got.SubscriptionID != "sub_123" || got.PriceID != "price_starter_monthly" {
		t.Fatalf("unexpected change input: %#v", got)
	}
	if summary.Plan != model.BillingPlanStarter || summary.PendingPlan != nil {
		t.Fatalf("unexpected summary after downgrade: plan=%s pending=%v", summary.Plan, summary.PendingPlan)
	}
	if summary.IncludedCredits != 5000 || summary.CreditsUsed != 12000 || summary.CreditsRemaining != 0 {
		t.Fatalf("credits after downgrade = included %d used %d remaining %d", summary.IncludedCredits, summary.CreditsUsed, summary.CreditsRemaining)
	}
}

func TestBillingServiceChangePlanSwitchesAnnualToMonthlyImmediately(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	gateway := &fakeBillingGateway{}
	svc := NewBillingService(repo, gateway, func() time.Time { return now })
	svc.SetPriceConfig(BillingPriceConfig{GrowthMonthly: "price_growth_monthly"})

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		StripePriceID:        billingStringPtr("price_growth_annual"),
		BillingInterval:      "annual",
		IncludedCredits:      25000,
		CreditsUsed:          12000,
		CurrentPeriodStart:   now.AddDate(0, -1, 0),
		CurrentPeriodEnd:     now.AddDate(0, 11, 0),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ChangeWorkspacePlan(context.Background(), BillingPlanChangeRequest{
		WorkspaceID: "workspace-1",
		Plan:        model.BillingPlanGrowth,
		Interval:    "monthly",
	})
	if err != nil {
		t.Fatalf("change plan: %v", err)
	}

	if len(gateway.immediateChanges) != 1 || len(gateway.scheduledChanges) != 0 {
		t.Fatalf("immediate=%d scheduled=%d, want 1/0", len(gateway.immediateChanges), len(gateway.scheduledChanges))
	}
	if got := gateway.immediateChanges[0]; got.SubscriptionID != "sub_123" || got.PriceID != "price_growth_monthly" {
		t.Fatalf("unexpected change input: %#v", got)
	}
	if summary.BillingInterval != "monthly" || summary.PendingBillingInterval != nil {
		t.Fatalf("unexpected interval state: current=%s pending=%v", summary.BillingInterval, summary.PendingBillingInterval)
	}
}

func TestBillingServiceChangePlanCancelsPaidSubscriptionAtRenewal(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	renewal := now.Add(365 * 24 * time.Hour)
	gateway := &fakeBillingGateway{}
	svc := NewBillingService(repo, gateway, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		StripePriceID:        billingStringPtr("price_growth_annual"),
		BillingInterval:      "annual",
		IncludedCredits:      25000,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     renewal,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ChangeWorkspacePlan(context.Background(), BillingPlanChangeRequest{
		WorkspaceID: "workspace-1",
		Plan:        model.BillingPlanFree,
		Interval:    "monthly",
	})
	if err != nil {
		t.Fatalf("change plan: %v", err)
	}

	if len(gateway.cancellations) != 1 || gateway.cancellations[0].SubscriptionID != "sub_123" {
		t.Fatalf("unexpected cancellations: %#v", gateway.cancellations)
	}
	if !summary.CancelAtPeriodEnd || summary.PendingPlan == nil || *summary.PendingPlan != model.BillingPlanFree {
		t.Fatalf("unexpected cancel summary: cancel=%v pending=%v", summary.CancelAtPeriodEnd, summary.PendingPlan)
	}
}

func TestBillingServiceChangePlanRejectsFreeWhenWorkspaceExceedsSeatLimit(t *testing.T) {
	db := newBillingTestDB(t)
	if err := db.Exec(`CREATE TABLE workspace_members (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		user_id TEXT,
		email TEXT NOT NULL,
		display_name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'member',
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create workspace_members: %v", err)
	}
	repo := repository.NewBillingRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	gateway := &fakeBillingGateway{}
	svc := NewBillingService(repo, gateway, func() time.Time { return now })
	svc.SetWorkspaceRepository(workspaceRepo)

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanStarter,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		StripePriceID:        billingStringPtr("price_starter_monthly"),
		BillingInterval:      "monthly",
		IncludedCredits:      5000,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(30 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if err := db.Exec(`INSERT INTO workspace_members (id, workspace_id, user_id, email, display_name, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("wm-%d", i),
			"workspace-1",
			fmt.Sprintf("user-%d", i),
			fmt.Sprintf("user%d@example.com", i),
			fmt.Sprintf("User %d", i),
			model.RoleMember,
			model.WorkspaceMemberStatusActive,
			now,
			now,
		).Error; err != nil {
			t.Fatalf("seed member %d: %v", i, err)
		}
	}

	_, err := svc.ChangeWorkspacePlan(context.Background(), BillingPlanChangeRequest{
		WorkspaceID: "workspace-1",
		Plan:        model.BillingPlanFree,
		Interval:    "monthly",
	})
	if err == nil {
		t.Fatal("expected free plan seat limit error")
	}
	if len(gateway.cancellations) != 0 {
		t.Fatalf("expected no Stripe cancellation, got %#v", gateway.cancellations)
	}
}

func TestBillingServiceResumeSubscriptionClearsScheduledCancellation(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	renewal := now.Add(30 * 24 * time.Hour)
	gateway := &fakeBillingGateway{}
	svc := NewBillingService(repo, gateway, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:            "workspace-1",
		Plan:                   model.BillingPlanGrowth,
		Status:                 model.BillingStatusActive,
		StripeCustomerID:       billingStringPtr("cus_123"),
		StripeSubscriptionID:   billingStringPtr("sub_123"),
		StripePriceID:          billingStringPtr("price_growth_monthly"),
		BillingInterval:        "monthly",
		IncludedCredits:        25000,
		CurrentPeriodStart:     now.Add(-24 * time.Hour),
		CurrentPeriodEnd:       renewal,
		PendingPlan:            billingStringPtr(model.BillingPlanFree),
		PendingBillingInterval: billingStringPtr("monthly"),
		PendingChangeAt:        &renewal,
		CancelAtPeriodEnd:      true,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ResumeWorkspaceSubscription(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("resume subscription: %v", err)
	}

	if len(gateway.resumes) != 1 || gateway.resumes[0].SubscriptionID != "sub_123" {
		t.Fatalf("unexpected resumes: %#v", gateway.resumes)
	}
	if summary.CancelAtPeriodEnd || summary.PendingPlan != nil || summary.PendingChangeAt != nil {
		t.Fatalf("unexpected resumed summary: cancel=%v pending=%v at=%v", summary.CancelAtPeriodEnd, summary.PendingPlan, summary.PendingChangeAt)
	}
	if summary.Plan != model.BillingPlanGrowth || summary.Status != model.BillingStatusActive {
		t.Fatalf("plan/status = %s/%s, want growth/active", summary.Plan, summary.Status)
	}
}

func TestBillingServiceRecordsPaymentFailedNotice(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		BillingInterval:      "monthly",
		IncludedCredits:      25000,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(29 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ApplyStripeInvoicePaymentFailed(context.Background(), BillingStripeInvoiceEvent{
		EventID:        "evt_failed",
		EventType:      "invoice.payment_failed",
		SubscriptionID: "sub_123",
		CustomerID:     "cus_123",
		InvoiceID:      "in_123",
	})
	if err != nil {
		t.Fatalf("apply failed payment: %v", err)
	}

	if summary.Status != model.BillingStatusPastDue {
		t.Fatalf("status = %s, want past_due", summary.Status)
	}
	if summary.BillingNoticeType != "payment_failed" || summary.BillingNoticeAt == nil || summary.PaymentFailedAt == nil {
		t.Fatalf("unexpected billing notice: %#v", summary)
	}
}

func TestBillingServiceClearsPaymentNoticeOnSucceededInvoice(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	failedAt := now.Add(-time.Hour)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanStarter,
		Status:               model.BillingStatusPastDue,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		BillingInterval:      "monthly",
		IncludedCredits:      5000,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(29 * 24 * time.Hour),
		BillingNoticeType:    billingStringPtr("payment_failed"),
		BillingNoticeMessage: billingStringPtr("Payment failed."),
		BillingNoticeAt:      &failedAt,
		PaymentFailedAt:      &failedAt,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ApplyStripeInvoicePaymentSucceeded(context.Background(), BillingStripeInvoiceEvent{
		EventID:        "evt_succeeded",
		EventType:      "invoice.payment_succeeded",
		SubscriptionID: "sub_123",
		CustomerID:     "cus_123",
		InvoiceID:      "in_123",
	})
	if err != nil {
		t.Fatalf("apply succeeded payment: %v", err)
	}

	if summary.Status != model.BillingStatusActive {
		t.Fatalf("status = %s, want active", summary.Status)
	}
	if summary.BillingNoticeType != "" || summary.PaymentFailedAt != nil {
		t.Fatalf("payment notice was not cleared: %#v", summary)
	}
}

func TestBillingServiceRecordsTrialWillEndNotice(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	trialEnd := now.Add(48 * time.Hour)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusTrialing,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		BillingInterval:      "monthly",
		IncludedCredits:      25000,
		CurrentPeriodStart:   now.Add(-12 * 24 * time.Hour),
		CurrentPeriodEnd:     trialEnd,
		TrialEndsAt:          &trialEnd,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ApplyStripeTrialWillEnd(context.Background(), BillingStripeTrialWillEndEvent{
		EventID:        "evt_trial_end",
		EventType:      "customer.subscription.trial_will_end",
		SubscriptionID: "sub_123",
		CustomerID:     "cus_123",
		TrialEndsAt:    trialEnd,
	})
	if err != nil {
		t.Fatalf("apply trial notice: %v", err)
	}

	if summary.BillingNoticeType != "trial_will_end" || summary.TrialWillEndAt == nil || !summary.TrialWillEndAt.Equal(trialEnd) {
		t.Fatalf("unexpected trial notice: %#v", summary)
	}
	if summary.Status != model.BillingStatusTrialing || summary.Plan != model.BillingPlanGrowth {
		t.Fatalf("trial notice changed billing state: %s/%s", summary.Plan, summary.Status)
	}
}

func TestBillingServiceSubscriptionUpdateKeepsCreditsWithinSamePeriod(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	periodStart := now.Add(-24 * time.Hour)
	periodEnd := now.Add(29 * 24 * time.Hour)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanStarter,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		BillingInterval:      "monthly",
		IncludedCredits:      5000,
		CreditsUsed:          4200,
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     periodEnd,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ApplyStripeSubscriptionUpdate(context.Background(), BillingStripeSubscriptionUpdate{
		EventID:              "evt_update_same_period",
		EventType:            "customer.subscription.updated",
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     "cus_123",
		StripeSubscriptionID: "sub_123",
		BillingInterval:      "monthly",
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     periodEnd,
	})
	if err != nil {
		t.Fatalf("apply subscription update: %v", err)
	}

	if summary.Plan != model.BillingPlanGrowth || summary.IncludedCredits != 25000 {
		t.Fatalf("plan/credits = %s/%d, want growth/25000", summary.Plan, summary.IncludedCredits)
	}
	if summary.CreditsUsed != 4200 {
		t.Fatalf("credits used = %d, want 4200", summary.CreditsUsed)
	}
}

func TestBillingServiceSubscriptionUpdateResetsCreditsWhenPeriodAdvances(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	oldStart := now.AddDate(0, -1, 0)
	newStart := now
	newEnd := now.AddDate(0, 1, 0)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:            "workspace-1",
		Plan:                   model.BillingPlanGrowth,
		Status:                 model.BillingStatusActive,
		StripeCustomerID:       billingStringPtr("cus_123"),
		StripeSubscriptionID:   billingStringPtr("sub_123"),
		BillingInterval:        "monthly",
		IncludedCredits:        25000,
		CreditsUsed:            18000,
		OnDemandEnabled:        true,
		OnDemandBlocksInvoiced: 2,
		CurrentPeriodStart:     oldStart,
		CurrentPeriodEnd:       newStart,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ApplyStripeSubscriptionUpdate(context.Background(), BillingStripeSubscriptionUpdate{
		EventID:              "evt_update_new_period",
		EventType:            "customer.subscription.updated",
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanStarter,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     "cus_123",
		StripeSubscriptionID: "sub_123",
		BillingInterval:      "monthly",
		CurrentPeriodStart:   newStart,
		CurrentPeriodEnd:     newEnd,
	})
	if err != nil {
		t.Fatalf("apply subscription update: %v", err)
	}

	if summary.Plan != model.BillingPlanStarter || summary.IncludedCredits != 5000 {
		t.Fatalf("plan/credits = %s/%d, want starter/5000", summary.Plan, summary.IncludedCredits)
	}
	if summary.CreditsUsed != 0 || summary.OnDemandBlocksInvoiced != 0 {
		t.Fatalf("credits reset = used %d blocks %d, want 0/0", summary.CreditsUsed, summary.OnDemandBlocksInvoiced)
	}
}

func TestBillingServiceCanceledSubscriptionDisablesOnDemandAndRecordsCanceledAt(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	canceledAt := now.Add(-time.Minute)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		BillingInterval:      "annual",
		IncludedCredits:      25000,
		OnDemandEnabled:      true,
		CurrentPeriodStart:   now.AddDate(0, -1, 0),
		CurrentPeriodEnd:     now.AddDate(0, 11, 0),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ApplyStripeSubscriptionUpdate(context.Background(), BillingStripeSubscriptionUpdate{
		EventID:              "evt_deleted",
		EventType:            "customer.subscription.deleted",
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanFree,
		Status:               model.BillingStatusCanceled,
		StripeCustomerID:     "cus_123",
		StripeSubscriptionID: "sub_123",
		BillingInterval:      "monthly",
		CurrentPeriodStart:   now,
		CurrentPeriodEnd:     now.AddDate(0, 1, 0),
		CanceledAt:           &canceledAt,
	})
	if err != nil {
		t.Fatalf("apply subscription update: %v", err)
	}

	if summary.Plan != model.BillingPlanFree || summary.OnDemandEnabled {
		t.Fatalf("expected free with on-demand disabled, got plan=%s on_demand=%v", summary.Plan, summary.OnDemandEnabled)
	}
	if summary.CanceledAt == nil || !summary.CanceledAt.Equal(canceledAt) {
		t.Fatalf("canceled_at = %v, want %s", summary.CanceledAt, canceledAt)
	}
}

func billingStringPtr(v string) *string { return &v }
