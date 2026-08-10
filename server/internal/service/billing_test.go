package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeBillingGateway struct {
	blocks           []BillingCreditBlockCharge
	blockDeadlineSet bool
	blockDeadline    time.Time
	immediateChanges []BillingSubscriptionChangeInput
	cancellations    []BillingSubscriptionCancelInput
	immediateCancels []BillingSubscriptionCancelInput
	resumes          []BillingSubscriptionCancelInput
	checkoutSession  *BillingCheckoutSession
	cancelErr        error
	blockErr         error
}

func (g *fakeBillingGateway) CreateCheckoutSession(ctx context.Context, input BillingCheckoutInput) (string, error) {
	return "https://checkout.stripe.test/session", nil
}

func (g *fakeBillingGateway) RetrieveCheckoutSession(ctx context.Context, sessionID string) (*BillingCheckoutSession, error) {
	if g.checkoutSession != nil {
		return g.checkoutSession, nil
	}
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
	deadline, ok := ctx.Deadline()
	g.blockDeadlineSet = ok
	g.blockDeadline = deadline
	if g.blockErr != nil {
		return g.blockErr
	}
	g.blocks = append(g.blocks, input)
	return nil
}

func (g *fakeBillingGateway) UpdateSubscriptionPrice(ctx context.Context, input BillingSubscriptionChangeInput) error {
	g.immediateChanges = append(g.immediateChanges, input)
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
		`CREATE TABLE organization_billing (
			id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL UNIQUE,
			stripe_customer_id TEXT,
			default_payment_method_id TEXT,
			founder_plan_enabled BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			owner_id TEXT NOT NULL,
			organization_id TEXT,
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

func configureBillingOutboxTest(t *testing.T, db *gorm.DB, svc *BillingService) *repository.CustomerIOLifecycleOutboxRepository {
	t.Helper()
	if err := db.Exec(`CREATE TABLE customer_io_outbox (
		id TEXT PRIMARY KEY, semantic_key TEXT NOT NULL UNIQUE, workspace_id TEXT,
		event_name TEXT NOT NULL, occurred_at DATETIME NOT NULL,
		attributes TEXT NOT NULL DEFAULT '{}', recipient_snapshot TEXT NOT NULL DEFAULT '[]',
		status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0,
		next_attempt_at DATETIME NOT NULL, claim_token TEXT, claimed_at DATETIME,
		lease_expires_at DATETIME, last_error TEXT, created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create Customer.io outbox: %v", err)
	}
	if err := db.Exec(`CREATE TABLE workspace_members (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		user_id TEXT,
		role TEXT NOT NULL,
		status TEXT NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create workspace_members: %v", err)
	}
	outboxRepo := repository.NewCustomerIOLifecycleOutboxRepository(db)
	svc.SetCustomerIOLifecycleOutboxRepository(outboxRepo)
	return outboxRepo
}

func seedBillingOutboxWorkspace(t *testing.T, db *gorm.DB, workspaceID string, now time.Time) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO workspaces (id, name, slug, owner_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		workspaceID, "Acme", workspaceID, "owner-1", now, now,
	).Error; err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO workspace_members (id, workspace_id, user_id, role, status) VALUES (?, ?, ?, ?, ?)`,
		"member-"+workspaceID, workspaceID, "owner-1", model.RoleOwner, model.WorkspaceMemberStatusActive,
	).Error; err != nil {
		t.Fatalf("seed workspace member: %v", err)
	}
}

func TestBillingServiceTrialInitializationOutbox(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	configureBillingOutboxTest(t, db, svc)
	seedBillingOutboxWorkspace(t, db, "workspace-trial-outbox", now)

	first, err := svc.EnsureTrialForWorkspace(context.Background(), "workspace-trial-outbox")
	if err != nil {
		t.Fatalf("first ensure trial: %v", err)
	}
	second, err := svc.EnsureTrialForWorkspace(context.Background(), "workspace-trial-outbox")
	if err != nil {
		t.Fatalf("second ensure trial: %v", err)
	}
	if first.TrialEndsAt == nil || second.TrialEndsAt == nil || !first.TrialEndsAt.Equal(*second.TrialEndsAt) {
		t.Fatalf("trial ends differ: first=%v second=%v", first.TrialEndsAt, second.TrialEndsAt)
	}

	var events []model.CustomerIOOutbox
	if err := db.Find(&events).Error; err != nil {
		t.Fatalf("list outbox: %v", err)
	}
	if len(events) != 1 || events[0].EventName != "trial_started" {
		t.Fatalf("outbox events = %#v, want one trial_started", events)
	}
}

func TestBillingServiceExpireOverdueTrialsOutboxIsIdempotent(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	configureBillingOutboxTest(t, db, svc)
	seedBillingOutboxWorkspace(t, db, "workspace-expiry-outbox", now)
	past := now.Add(-time.Hour)
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID: "workspace-expiry-outbox", Plan: model.BillingPlanGrowth,
		Status: model.BillingStatusTrialing, BillingInterval: "monthly",
		CurrentPeriodStart: now.AddDate(0, 0, -14), CurrentPeriodEnd: past, TrialEndsAt: &past,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	first, err := svc.ExpireOverdueTrials(context.Background())
	if err != nil {
		t.Fatalf("first expiry: %v", err)
	}
	second, err := svc.ExpireOverdueTrials(context.Background())
	if err != nil {
		t.Fatalf("second expiry: %v", err)
	}
	if first != 1 || second != 0 {
		t.Fatalf("expiry counts = %d, %d; want 1, 0", first, second)
	}
	var count int64
	if err := db.Model(&model.CustomerIOOutbox{}).Where("event_name = ?", "trial_expired").Count(&count).Error; err != nil {
		t.Fatalf("count expiry events: %v", err)
	}
	if count != 1 {
		t.Fatalf("trial_expired outbox count = %d, want 1", count)
	}
}

func TestBillingServiceExpireOverdueTrialsOutboxRollback(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	configureBillingOutboxTest(t, db, svc)
	seedBillingOutboxWorkspace(t, db, "workspace-expiry-rollback", now)
	past := now.Add(-time.Hour)
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID: "workspace-expiry-rollback", Plan: model.BillingPlanGrowth,
		Status: model.BillingStatusTrialing, BillingInterval: "monthly",
		CurrentPeriodStart: now.AddDate(0, 0, -14), CurrentPeriodEnd: past, TrialEndsAt: &past,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}
	if err := db.Exec("DROP TABLE customer_io_outbox").Error; err != nil {
		t.Fatalf("drop outbox: %v", err)
	}

	if _, err := svc.ExpireOverdueTrials(context.Background()); err == nil {
		t.Fatal("expiry succeeded without outbox table")
	}
	billing, err := repo.GetByWorkspaceID(context.Background(), "workspace-expiry-rollback")
	if err != nil {
		t.Fatalf("reload billing: %v", err)
	}
	if billing.Status != model.BillingStatusTrialing {
		t.Fatalf("billing status = %q, want rollback to trialing", billing.Status)
	}
}

func TestBillingServiceReadExpiredTrialUsesOutbox(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	configureBillingOutboxTest(t, db, svc)
	seedBillingOutboxWorkspace(t, db, "workspace-read-expired", now)
	past := now.Add(-time.Hour)
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID: "workspace-read-expired", Plan: model.BillingPlanGrowth,
		Status: model.BillingStatusTrialing, BillingInterval: "monthly",
		CurrentPeriodStart: now.AddDate(0, 0, -14), CurrentPeriodEnd: past, TrialEndsAt: &past,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.GetWorkspaceBilling(context.Background(), "workspace-read-expired")
	if err != nil {
		t.Fatalf("get expired billing: %v", err)
	}
	if summary.Status != model.BillingStatusTrialExpired {
		t.Fatalf("status = %q, want trial_expired", summary.Status)
	}
	var count int64
	if err := db.Model(&model.CustomerIOOutbox{}).Where("event_name = ?", "trial_expired").Count(&count).Error; err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if count != 1 {
		t.Fatalf("trial_expired outbox count = %d, want 1", count)
	}
}

func TestBillingServiceEnsureTrialForFounderOrgStartsFounderPlan(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := db.Exec(`INSERT INTO organization_billing (id, organization_id, founder_plan_enabled, created_at, updated_at) VALUES ('ob-1', 'org-1', 1, ?, ?)`, now, now).Error; err != nil {
		t.Fatalf("seed org billing: %v", err)
	}
	if err := db.Exec(`INSERT INTO workspaces (id, name, slug, owner_id, organization_id, created_at, updated_at) VALUES ('workspace-founder', 'Founder Workspace', 'founder-workspace', 'owner-1', 'org-1', ?, ?)`, now, now).Error; err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	summary, err := svc.EnsureTrialForWorkspace(context.Background(), "workspace-founder")
	if err != nil {
		t.Fatalf("ensure founder billing: %v", err)
	}
	if summary.Plan != model.BillingPlanFounder || summary.Status != model.BillingStatusActive {
		t.Fatalf("plan/status = %s/%s, want founder/active", summary.Plan, summary.Status)
	}
	if summary.Trialing || summary.TrialEndsAt != nil {
		t.Fatalf("founder workspace should not be trialing: %#v", summary)
	}
	if summary.IncludedCredits != 100000 || summary.CreditsRemaining != 100000 {
		t.Fatalf("credits = %d remaining %d, want 100000", summary.IncludedCredits, summary.CreditsRemaining)
	}
	if summary.ManageBillingEnabled || summary.OnDemandAvailable {
		t.Fatalf("founder should not expose Stripe-managed actions: manage=%v on_demand=%v", summary.ManageBillingEnabled, summary.OnDemandAvailable)
	}
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

func TestBillingServiceRejectsStripeActionsForFounderPlan(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	gateway := &fakeBillingGateway{}
	svc := NewBillingService(repo, gateway, func() time.Time { return now })
	svc.SetPriceConfig(BillingPriceConfig{
		StarterMonthly: "price_starter_monthly",
		GrowthMonthly:  "price_growth_monthly",
	})
	customerID := "cus_founder"
	subscriptionID := "sub_founder"
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-founder",
		Plan:                 model.BillingPlanFounder,
		Status:               model.BillingStatusActive,
		BillingInterval:      "monthly",
		IncludedCredits:      100000,
		StripeCustomerID:     &customerID,
		StripeSubscriptionID: &subscriptionID,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.AddDate(0, 1, 0),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	if _, err := svc.CreatePortalSession(context.Background(), "workspace-founder", "https://app.test/billing"); err == nil || !strings.Contains(err.Error(), "Founder") {
		t.Fatalf("CreatePortalSession error = %v, want Founder rejection", err)
	}
	if _, err := svc.CreateCheckoutSession(context.Background(), BillingCheckoutRequest{
		WorkspaceID: "workspace-founder",
		Plan:        model.BillingPlanGrowth,
		Interval:    "monthly",
		ReturnURL:   "https://app.test/billing",
	}); err == nil || !strings.Contains(err.Error(), "Founder") {
		t.Fatalf("CreateCheckoutSession error = %v, want Founder rejection", err)
	}
	if _, err := svc.SetOnDemandEnabled(context.Background(), "workspace-founder", true); err == nil || !strings.Contains(err.Error(), "Founder") {
		t.Fatalf("SetOnDemandEnabled error = %v, want Founder rejection", err)
	}
}

func TestBillingServiceResetsFounderCreditsMonthly(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:        "workspace-founder",
		Plan:               model.BillingPlanFounder,
		Status:             model.BillingStatusActive,
		BillingInterval:    "monthly",
		IncludedCredits:    100000,
		CreditsUsed:        72000,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.GetWorkspaceBilling(context.Background(), "workspace-founder")
	if err != nil {
		t.Fatalf("get billing: %v", err)
	}
	if summary.CreditsUsed != 0 || summary.CreditsRemaining != 100000 {
		t.Fatalf("credits after founder reset = used %d remaining %d, want 0/100000", summary.CreditsUsed, summary.CreditsRemaining)
	}
	if !summary.CurrentPeriodStart.Equal(now) {
		t.Fatalf("period start = %s, want %s", summary.CurrentPeriodStart, now)
	}
	if !summary.CurrentPeriodEnd.Equal(now.AddDate(0, 1, 0)) {
		t.Fatalf("period end = %s, want %s", summary.CurrentPeriodEnd, now.AddDate(0, 1, 0))
	}
}

func TestBillingServiceLocksExpiredUnpaidTrial(t *testing.T) {
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

	if summary.Plan != model.BillingPlanGrowth || summary.Status != model.BillingStatusTrialExpired {
		t.Fatalf("unexpected plan/status after trial expiry: %s/%s", summary.Plan, summary.Status)
	}
	if summary.IncludedCredits != 25000 || summary.CreditsUsed != 125 {
		t.Fatalf("credits after lock = included %d used %d, want 25000/125", summary.IncludedCredits, summary.CreditsUsed)
	}
	if !summary.Locked {
		t.Fatalf("expected locked summary after trial expiry")
	}
}

func TestBillingServiceExpireOverdueTrialsLocksInternalTrials(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:        "expired-trial",
		Plan:               model.BillingPlanGrowth,
		Status:             model.BillingStatusTrialing,
		IncludedCredits:    25000,
		CreditsUsed:        250,
		CurrentPeriodStart: now.Add(-15 * 24 * time.Hour),
		CurrentPeriodEnd:   past,
		TrialEndsAt:        &past,
	}); err != nil {
		t.Fatalf("seed expired trial: %v", err)
	}
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:        "active-trial",
		Plan:               model.BillingPlanGrowth,
		Status:             model.BillingStatusTrialing,
		IncludedCredits:    25000,
		CreditsUsed:        125,
		CurrentPeriodStart: now.Add(-24 * time.Hour),
		CurrentPeriodEnd:   future,
		TrialEndsAt:        &future,
	}); err != nil {
		t.Fatalf("seed active trial: %v", err)
	}
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "stripe-trial",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusTrialing,
		StripeSubscriptionID: billingStringPtr("sub_trial"),
		IncludedCredits:      25000,
		CurrentPeriodStart:   now.Add(-15 * 24 * time.Hour),
		CurrentPeriodEnd:     past,
		TrialEndsAt:          &past,
	}); err != nil {
		t.Fatalf("seed stripe trial: %v", err)
	}

	expired, err := svc.ExpireOverdueTrials(context.Background())
	if err != nil {
		t.Fatalf("expire overdue trials: %v", err)
	}
	if expired != 1 {
		t.Fatalf("expired count = %d, want 1", expired)
	}

	for _, tc := range []struct {
		workspaceID string
		wantStatus  string
	}{
		{"expired-trial", model.BillingStatusTrialExpired},
		{"active-trial", model.BillingStatusTrialing},
		{"stripe-trial", model.BillingStatusTrialing},
	} {
		billing, err := repo.GetByWorkspaceID(context.Background(), tc.workspaceID)
		if err != nil {
			t.Fatalf("load %s: %v", tc.workspaceID, err)
		}
		if billing.Status != tc.wantStatus {
			t.Fatalf("%s status = %s, want %s", tc.workspaceID, billing.Status, tc.wantStatus)
		}
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

func TestBillingServiceConsumeCreditsRejectsWhenWorkspaceLocked(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:        "workspace-1",
		Plan:               model.BillingPlanGrowth,
		Status:             model.BillingStatusTrialExpired,
		IncludedCredits:    25000,
		CreditsUsed:        100,
		CurrentPeriodStart: now.Add(-24 * time.Hour),
		CurrentPeriodEnd:   now.Add(30 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	_, err := svc.ConsumeCredits(context.Background(), BillingCreditConsumption{
		WorkspaceID:    "workspace-1",
		FeatureKey:     BillingFeatureSupportAIReply,
		Credits:        10,
		IdempotencyKey: "reply-1",
	})
	if err == nil || !strings.Contains(err.Error(), "workspace is locked") {
		t.Fatalf("ConsumeCredits() error = %v, want locked workspace error", err)
	}
}

func TestBillingServicePreflightCreditsRejectsWhenUsageExhausted(t *testing.T) {
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

	err := svc.PreflightCredits(context.Background(), BillingCreditPreflight{
		WorkspaceID: "workspace-1",
		FeatureKey:  BillingFeatureSupportAIReply,
		Credits:     8,
	})
	if err == nil || !strings.Contains(err.Error(), "AI usage exhausted") {
		t.Fatalf("PreflightCredits() error = %v, want AI usage exhausted", err)
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

func TestBillingServiceBoundsOnDemandStripeChargeContext(t *testing.T) {
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

	started := time.Now()
	if _, err := svc.ConsumeCredits(context.Background(), BillingCreditConsumption{
		WorkspaceID:    "workspace-1",
		FeatureKey:     BillingFeatureSupportAIReply,
		Credits:        10,
		IdempotencyKey: "reply-1",
	}); err != nil {
		t.Fatalf("consume credits: %v", err)
	}

	if !gateway.blockDeadlineSet {
		t.Fatal("BillCreditBlock context has no deadline")
	}
	elapsed := gateway.blockDeadline.Sub(started)
	if elapsed <= 0 || elapsed > billingStripeChargeTimeout+100*time.Millisecond {
		t.Fatalf("BillCreditBlock deadline in %s, want within %s", elapsed, billingStripeChargeTimeout)
	}
}

func TestBillingServiceRetriesOnDemandBillingAfterFailedCharge(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	gateway := &fakeBillingGateway{blockErr: fmt.Errorf("stripe unavailable")}
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

	input := BillingCreditConsumption{
		WorkspaceID:    "workspace-1",
		FeatureKey:     BillingFeatureSupportAIReply,
		Credits:        10,
		IdempotencyKey: "reply-1",
	}
	if _, err := svc.ConsumeCredits(context.Background(), input); err == nil {
		t.Fatal("expected first consume to return stripe charge error")
	}
	billing, err := repo.GetByWorkspaceID(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("reload billing: %v", err)
	}
	if billing.CreditsUsed != 4995 || billing.OnDemandBlocksInvoiced != 0 || len(gateway.blocks) != 0 {
		t.Fatalf("after failed charge used=%d blocks_invoiced=%d charges=%d, want 4995/0/0", billing.CreditsUsed, billing.OnDemandBlocksInvoiced, len(gateway.blocks))
	}
	var ledgerCount int64
	if err := db.Table("billing_credit_ledger").Where("workspace_id = ?", "workspace-1").Count(&ledgerCount).Error; err != nil {
		t.Fatalf("count ledger rows: %v", err)
	}
	if ledgerCount != 0 {
		t.Fatalf("ledger rows after failed charge = %d, want 0", ledgerCount)
	}

	gateway.blockErr = nil
	summary, err := svc.ConsumeCredits(context.Background(), input)
	if err != nil {
		t.Fatalf("retry consume: %v", err)
	}
	if summary.CreditsUsed != 5005 || summary.OnDemandBlocksInvoiced != 1 || len(gateway.blocks) != 1 {
		t.Fatalf("retry used=%d blocks_invoiced=%d charges=%d, want 5005/1/1", summary.CreditsUsed, summary.OnDemandBlocksInvoiced, len(gateway.blocks))
	}
}

func TestBillingRepositoryConsumeCreditsRejectsOverLimitInsideLock(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)

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

	_, err := repo.ConsumeCredits(context.Background(), "workspace-1", 10, model.BillingCreditLedgerEntry{
		WorkspaceID:    "workspace-1",
		Kind:           model.BillingLedgerKindUsage,
		FeatureKey:     BillingFeatureSupportAIReply,
		Credits:        10,
		IdempotencyKey: "reply-1",
		Metadata:       model.JSONBlob(`{}`),
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "AI usage exhausted") {
		t.Fatalf("ConsumeCredits() error = %v, want AI usage exhausted", err)
	}
	billing, err := repo.GetByWorkspaceID(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("reload billing: %v", err)
	}
	if billing.CreditsUsed != 4995 {
		t.Fatalf("credits_used = %d, want unchanged 4995", billing.CreditsUsed)
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

func TestBillingServiceConfirmCheckoutReactivatesExpiredTrialWithFallbackPeriodStart(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	gateway := &fakeBillingGateway{
		checkoutSession: &BillingCheckoutSession{
			ID:                   "cs_paid",
			WorkspaceID:          "workspace-1",
			Plan:                 model.BillingPlanStarter,
			Interval:             "monthly",
			StripeCustomerID:     "cus_paid",
			StripeSubscriptionID: "sub_paid",
			StripePriceID:        "price_starter_monthly",
			SubscriptionStatus:   model.BillingStatusActive,
			CurrentPeriodEnd:     now.AddDate(0, 1, 0),
		},
	}
	svc := NewBillingService(repo, gateway, func() time.Time { return now })
	svc.SetPriceConfig(BillingPriceConfig{StarterMonthly: "price_starter_monthly"})

	past := now.Add(-time.Hour)
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:        "workspace-1",
		Plan:               model.BillingPlanGrowth,
		Status:             model.BillingStatusTrialExpired,
		IncludedCredits:    25000,
		CreditsUsed:        25000,
		CurrentPeriodStart: now.Add(-15 * 24 * time.Hour),
		CurrentPeriodEnd:   past,
		TrialEndsAt:        &past,
	}); err != nil {
		t.Fatalf("seed expired trial: %v", err)
	}

	summary, err := svc.ConfirmCheckoutSession(context.Background(), "workspace-1", "cs_paid")
	if err != nil {
		t.Fatalf("confirm checkout: %v", err)
	}

	if summary.Status != model.BillingStatusActive || summary.Plan != model.BillingPlanStarter {
		t.Fatalf("plan/status = %s/%s, want starter/active", summary.Plan, summary.Status)
	}
	if !summary.CurrentPeriodStart.Equal(now) {
		t.Fatalf("period start = %s, want fallback now %s", summary.CurrentPeriodStart, now)
	}
	if summary.CreditsUsed != 0 || summary.CreditsRemaining != 5000 {
		t.Fatalf("credits after reactivation = used %d remaining %d, want 0/5000", summary.CreditsUsed, summary.CreditsRemaining)
	}
	if summary.TrialEndsAt != nil || summary.Locked {
		t.Fatalf("trial/lock state after checkout = trial %v locked %v, want nil/false", summary.TrialEndsAt, summary.Locked)
	}
}

func TestBillingServiceCanManageWorkspaceBillingRejectsDelegatedBillingOwner(t *testing.T) {
	db := newBillingTestDB(t)
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS workspaces (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		slug TEXT NOT NULL,
		organization_id TEXT
	)`).Error; err != nil {
		t.Fatalf("create workspaces table: %v", err)
	}
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	svc.SetOrgRoleResolver(testOrgRoleResolver{roles: map[string]string{
		"delegated-user": model.RoleAdmin,
	}})

	if err := db.Exec(`INSERT INTO workspaces (id, name, slug, owner_id, organization_id) VALUES (?, ?, ?, ?, ?)`, "workspace-1", "Workspace", "workspace", "owner-1", "org-1").Error; err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		BillingOwnerUserID:   billingStringPtr("delegated-user"),
		BillingInterval:      "monthly",
		IncludedCredits:      25000,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(30 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	ok, err := svc.CanManageWorkspaceBilling(context.Background(), "delegated-user", "workspace-1")
	if err != nil {
		t.Fatalf("CanManageWorkspaceBilling() error = %v", err)
	}
	if ok {
		t.Fatal("delegated billing owner can manage workspace billing, want owner-only")
	}
}

func TestBillingServiceCanManageOrgBillingRejectsDelegatedWorkspaceBillingOwner(t *testing.T) {
	db := newBillingTestDB(t)
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS workspaces (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		slug TEXT NOT NULL,
		organization_id TEXT
	)`).Error; err != nil {
		t.Fatalf("create workspaces table: %v", err)
	}
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	svc.SetOrgRoleResolver(testOrgRoleResolver{roles: map[string]string{
		"delegated-user": model.RoleAdmin,
	}})

	if err := db.Exec(`INSERT INTO workspaces (id, name, slug, owner_id, organization_id) VALUES (?, ?, ?, ?, ?)`, "workspace-1", "Workspace", "workspace", "owner-1", "org-1").Error; err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		BillingOwnerUserID:   billingStringPtr("delegated-user"),
		BillingInterval:      "monthly",
		IncludedCredits:      25000,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(30 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	ok, err := svc.CanManageOrgBilling(context.Background(), "delegated-user", "org-1")
	if err != nil {
		t.Fatalf("CanManageOrgBilling() error = %v", err)
	}
	if ok {
		t.Fatal("delegated billing owner can manage org billing, want owner-only")
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

	if len(gateway.immediateChanges) != 1 {
		t.Fatalf("immediate=%d, want 1", len(gateway.immediateChanges))
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

	if len(gateway.immediateChanges) != 1 {
		t.Fatalf("immediate=%d, want 1", len(gateway.immediateChanges))
	}
	if got := gateway.immediateChanges[0]; got.SubscriptionID != "sub_123" || got.PriceID != "price_growth_monthly" {
		t.Fatalf("unexpected change input: %#v", got)
	}
	if summary.BillingInterval != "monthly" || summary.PendingBillingInterval != nil {
		t.Fatalf("unexpected interval state: current=%s pending=%v", summary.BillingInterval, summary.PendingBillingInterval)
	}
}

func TestBillingServiceChangePlanRejectsFreePlan(t *testing.T) {
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
		Plan:        "free",
		Interval:    "monthly",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported billing plan or interval") {
		t.Fatalf("ChangeWorkspacePlan() error = %v, want unsupported billing plan or interval", err)
	}

	if summary != nil {
		t.Fatalf("summary = %#v, want nil", summary)
	}
	if len(gateway.cancellations) != 0 {
		t.Fatalf("unexpected cancellations: %#v", gateway.cancellations)
	}
}

func TestBillingServiceCanceledSubscriptionKeepsPaidPlanAndLocksWorkspace(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		StripePriceID:        billingStringPtr("price_growth_monthly"),
		BillingInterval:      "monthly",
		IncludedCredits:      25000,
		CreditsUsed:          200,
		OnDemandEnabled:      true,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now,
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	canceledAt := now
	summary, err := svc.ApplyStripeSubscriptionUpdate(context.Background(), BillingStripeSubscriptionUpdate{
		EventID:              "evt_cancel",
		EventType:            "customer.subscription.deleted",
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		BillingInterval:      "monthly",
		Status:               model.BillingStatusCanceled,
		StripeCustomerID:     "cus_123",
		StripeSubscriptionID: "sub_123",
		StripePriceID:        "price_growth_monthly",
		CurrentPeriodStart:   now.Add(-30 * 24 * time.Hour),
		CurrentPeriodEnd:     now,
		CanceledAt:           &canceledAt,
	})
	if err != nil {
		t.Fatalf("apply canceled subscription: %v", err)
	}

	if summary.Plan != model.BillingPlanGrowth || summary.Status != model.BillingStatusCanceled {
		t.Fatalf("plan/status = %s/%s, want growth/canceled", summary.Plan, summary.Status)
	}
	if !summary.Locked {
		t.Fatalf("expected canceled workspace to be locked")
	}
	if summary.IncludedCredits != 25000 {
		t.Fatalf("included credits = %d, want 25000", summary.IncludedCredits)
	}
	if summary.OnDemandEnabled {
		t.Fatalf("on-demand should be disabled after cancellation")
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
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		StripePriceID:        billingStringPtr("price_growth_monthly"),
		BillingInterval:      "monthly",
		IncludedCredits:      25000,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     renewal,
		PendingChangeAt:      &renewal,
		CancelAtPeriodEnd:    true,
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

func TestBillingServicePaymentFailedAtomicallyEnqueuesLifecycleEvent(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	receivedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	occurredAt := receivedAt.Add(-2 * time.Minute)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return receivedAt })
	configureBillingOutboxTest(t, db, svc)
	if err := db.Exec(`INSERT INTO workspace_members (id, workspace_id, user_id, role, status) VALUES (?, ?, ?, ?, ?)`, "member-1", "workspace-1", "user-1", model.RoleOwner, model.WorkspaceMemberStatusActive).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{WorkspaceID: "workspace-1", Plan: model.BillingPlanGrowth, Status: model.BillingStatusActive, StripeCustomerID: billingStringPtr("cus_123"), StripeSubscriptionID: billingStringPtr("sub_123"), BillingInterval: "monthly", IncludedCredits: 25000, CurrentPeriodStart: receivedAt.Add(-24 * time.Hour), CurrentPeriodEnd: receivedAt.Add(29 * 24 * time.Hour)}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}
	event := BillingStripeInvoiceEvent{EventID: "evt_atomic_failed", EventType: "invoice.payment_failed", SubscriptionID: "sub_123", CustomerID: "cus_123", OccurredAt: occurredAt}
	if _, err := svc.ApplyStripeInvoicePaymentFailed(context.Background(), event); err != nil {
		t.Fatalf("apply event: %v", err)
	}
	if _, err := svc.ApplyStripeInvoicePaymentFailed(context.Background(), event); err != nil {
		t.Fatalf("replay event: %v", err)
	}
	var row model.CustomerIOOutbox
	if err := db.Where("semantic_key = ?", "stripe:"+event.EventID).First(&row).Error; err != nil {
		t.Fatalf("load outbox: %v", err)
	}
	if row.EventName != "payment_failed" || !row.OccurredAt.Equal(occurredAt) {
		t.Fatalf("unexpected outbox row: %#v", row)
	}
	var count int64
	if err := db.Model(&model.CustomerIOOutbox{}).Where("semantic_key = ?", "stripe:"+event.EventID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("outbox count = %d, err = %v", count, err)
	}
	var webhook model.StripeWebhookEvent
	if err := db.First(&webhook, "id = ?", event.EventID).Error; err != nil || !webhook.Processed {
		t.Fatalf("webhook not processed atomically: %#v err=%v", webhook, err)
	}
}

func TestBillingServicePaymentFailedRollsBackWhenOutboxInsertFails(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })
	configureBillingOutboxTest(t, db, svc)
	if err := db.Exec("DROP TABLE customer_io_outbox").Error; err != nil {
		t.Fatalf("drop outbox: %v", err)
	}
	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{WorkspaceID: "workspace-1", Plan: model.BillingPlanGrowth, Status: model.BillingStatusActive, StripeSubscriptionID: billingStringPtr("sub_rollback"), BillingInterval: "monthly", CurrentPeriodStart: now.Add(-time.Hour), CurrentPeriodEnd: now.Add(30 * 24 * time.Hour)}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}
	_, err := svc.ApplyStripeInvoicePaymentFailed(context.Background(), BillingStripeInvoiceEvent{EventID: "evt_rollback", EventType: "invoice.payment_failed", SubscriptionID: "sub_rollback", OccurredAt: now})
	if err == nil {
		t.Fatal("expected outbox failure")
	}
	billing, err := repo.GetByWorkspaceID(context.Background(), "workspace-1")
	if err != nil || billing.Status != model.BillingStatusActive {
		t.Fatalf("billing mutation was not rolled back: %#v err=%v", billing, err)
	}
	var count int64
	if err := db.Model(&model.StripeWebhookEvent{}).Where("id = ?", "evt_rollback").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("webhook insert was not rolled back: count=%d err=%v", count, err)
	}
}

func TestBillingServiceReprocessesUnprocessedWebhookEvent(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanStarter,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		BillingInterval:      "monthly",
		IncludedCredits:      5000,
		CreditsUsed:          250,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(29 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}
	if err := db.Exec(`INSERT INTO stripe_webhook_events (id, type, processed) VALUES (?, ?, ?)`, "evt_retry", "customer.subscription.updated", false).Error; err != nil {
		t.Fatalf("seed webhook event: %v", err)
	}

	summary, err := svc.ApplyStripeSubscriptionUpdate(context.Background(), BillingStripeSubscriptionUpdate{
		EventID:              "evt_retry",
		EventType:            "customer.subscription.updated",
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusActive,
		StripeCustomerID:     "cus_123",
		StripeSubscriptionID: "sub_123",
		BillingInterval:      "monthly",
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(29 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("apply subscription update: %v", err)
	}
	if summary.Plan != model.BillingPlanGrowth || summary.IncludedCredits != 25000 {
		t.Fatalf("plan/credits = %s/%d, want growth/25000", summary.Plan, summary.IncludedCredits)
	}
	var processed bool
	if err := db.Raw(`SELECT processed FROM stripe_webhook_events WHERE id = ?`, "evt_retry").Scan(&processed).Error; err != nil {
		t.Fatalf("load webhook event: %v", err)
	}
	if !processed {
		t.Fatal("expected webhook event to be marked processed")
	}
}

func TestBillingServiceDoesNotApplyCustomerOnlyInvoiceToAmbiguousWorkspace(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	for _, workspaceID := range []string{"workspace-1", "workspace-2"} {
		if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
			WorkspaceID:          workspaceID,
			Plan:                 model.BillingPlanGrowth,
			Status:               model.BillingStatusActive,
			StripeCustomerID:     billingStringPtr("cus_shared"),
			StripeSubscriptionID: billingStringPtr("sub_" + workspaceID),
			BillingInterval:      "monthly",
			IncludedCredits:      25000,
			CurrentPeriodStart:   now.Add(-24 * time.Hour),
			CurrentPeriodEnd:     now.Add(29 * 24 * time.Hour),
		}); err != nil {
			t.Fatalf("seed billing %s: %v", workspaceID, err)
		}
	}

	summary, err := svc.ApplyStripeInvoicePaymentFailed(context.Background(), BillingStripeInvoiceEvent{
		EventID:    "evt_customer_only",
		EventType:  "invoice.payment_failed",
		CustomerID: "cus_shared",
		InvoiceID:  "in_123",
	})
	if err != nil {
		t.Fatalf("apply failed payment: %v", err)
	}
	if summary != nil {
		t.Fatalf("summary = %#v, want nil for ambiguous customer-only invoice", summary)
	}
	for _, workspaceID := range []string{"workspace-1", "workspace-2"} {
		billing, err := repo.GetByWorkspaceID(context.Background(), workspaceID)
		if err != nil {
			t.Fatalf("reload billing %s: %v", workspaceID, err)
		}
		if billing.Status != model.BillingStatusActive || billing.BillingNoticeType != nil {
			t.Fatalf("%s was mutated by ambiguous customer invoice: status=%s notice=%v", workspaceID, billing.Status, billing.BillingNoticeType)
		}
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
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusCanceled,
		StripeCustomerID:     "cus_123",
		StripeSubscriptionID: "sub_123",
		BillingInterval:      "annual",
		CurrentPeriodStart:   now,
		CurrentPeriodEnd:     now.AddDate(0, 1, 0),
		CanceledAt:           &canceledAt,
	})
	if err != nil {
		t.Fatalf("apply subscription update: %v", err)
	}

	if summary.Plan != model.BillingPlanGrowth || !summary.Locked || summary.OnDemandEnabled {
		t.Fatalf("expected locked growth with on-demand disabled, got plan=%s locked=%v on_demand=%v", summary.Plan, summary.Locked, summary.OnDemandEnabled)
	}
	if summary.CanceledAt == nil || !summary.CanceledAt.Equal(canceledAt) {
		t.Fatalf("canceled_at = %v, want %s", summary.CanceledAt, canceledAt)
	}
}

func TestBillingServiceUnpaidSubscriptionLocksWorkspace(t *testing.T) {
	db := newBillingTestDB(t)
	repo := repository.NewBillingRepository(db)
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	svc := NewBillingService(repo, &fakeBillingGateway{}, func() time.Time { return now })

	if err := repo.UpsertWorkspaceBilling(context.Background(), &model.WorkspaceBilling{
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               model.BillingStatusPastDue,
		StripeCustomerID:     billingStringPtr("cus_123"),
		StripeSubscriptionID: billingStringPtr("sub_123"),
		BillingInterval:      "monthly",
		IncludedCredits:      25000,
		OnDemandEnabled:      true,
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(29 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed billing: %v", err)
	}

	summary, err := svc.ApplyStripeSubscriptionUpdate(context.Background(), BillingStripeSubscriptionUpdate{
		EventID:              "evt_unpaid",
		EventType:            "customer.subscription.updated",
		WorkspaceID:          "workspace-1",
		Plan:                 model.BillingPlanGrowth,
		Status:               "unpaid",
		StripeCustomerID:     "cus_123",
		StripeSubscriptionID: "sub_123",
		BillingInterval:      "monthly",
		CurrentPeriodStart:   now.Add(-24 * time.Hour),
		CurrentPeriodEnd:     now.Add(29 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("apply subscription update: %v", err)
	}
	if summary.Status != model.BillingStatusUnpaid || !summary.Locked || summary.OnDemandEnabled {
		t.Fatalf("status=%s locked=%v on_demand=%v, want unpaid locked with on-demand disabled", summary.Status, summary.Locked, summary.OnDemandEnabled)
	}
}

func billingStringPtr(v string) *string { return &v }

type testOrgRoleResolver struct {
	roles map[string]string
}

func (r testOrgRoleResolver) GetMemberRole(_ context.Context, _ string, userID string) (string, error) {
	return r.roles[userID], nil
}

func (r testOrgRoleResolver) ListOwners(_ context.Context, _ string) ([]model.MemberWithUser, error) {
	owners := make([]model.MemberWithUser, 0)
	for userID, role := range r.roles {
		if role == model.RoleOwner {
			owners = append(owners, model.MemberWithUser{UserID: userID, Role: role})
		}
	}
	return owners, nil
}
