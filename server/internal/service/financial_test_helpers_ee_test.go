//go:build ee

package service

import (
	"context"
	"fmt"
	"github.com/helpin-ai/helpin/server/ee/pricing"
	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	eeservice "github.com/helpin-ai/helpin/server/ee/service"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

type AIUsageEstimateSource = eeservice.AIUsageEstimateSource

type AIUsagePeriodScheduleResolver = eeservice.AIUsagePeriodScheduleResolver

type AIUsagePeriodStore = eeservice.AIUsagePeriodStore

type AIUsagePeriodWorker = eeservice.AIUsagePeriodWorker

type AIUsageReservationRecoveryStore = eeservice.AIUsageReservationRecoveryStore

type AIUsageReservationSweeper = eeservice.AIUsageReservationSweeper

type AIUsageService = eeservice.AIUsageService

type AIUsageSettlementCharge = eeservice.AIUsageSettlementCharge

type AIUsageSettlementGateway = eeservice.AIUsageSettlementGateway

type AIUsageSettlementStore = eeservice.AIUsageSettlementStore

type AIUsageSettlementWorker = eeservice.AIUsageSettlementWorker

type AIUsageStore = eeservice.AIUsageStore

type BillingCheckoutInput = eeservice.BillingCheckoutInput

type BillingCheckoutRequest = eeservice.BillingCheckoutRequest

type BillingCheckoutSession = eeservice.BillingCheckoutSession

type BillingOwnerRef = eeservice.BillingOwnerRef

type BillingPlanChangePreview = eeservice.BillingPlanChangePreview

type BillingPlanChangePreviewLine = eeservice.BillingPlanChangePreviewLine

type BillingPlanChangeRequest = eeservice.BillingPlanChangeRequest

type BillingPriceConfig = eeservice.BillingPriceConfig

type BillingService = eeservice.BillingService

type BillingStripeGateway = eeservice.BillingStripeGateway

type BillingStripeInvoiceEvent = eeservice.BillingStripeInvoiceEvent

type BillingStripeInvoicePreview = eeservice.BillingStripeInvoicePreview

type BillingStripeInvoicePreviewLine = eeservice.BillingStripeInvoicePreviewLine

type BillingStripeSubscriptionUpdate = eeservice.BillingStripeSubscriptionUpdate

type BillingStripeTrialWillEndEvent = eeservice.BillingStripeTrialWillEndEvent

type BillingSubscriptionCancelInput = eeservice.BillingSubscriptionCancelInput

type BillingSubscriptionChangeInput = eeservice.BillingSubscriptionChangeInput

type BillingTestScenarioService = eeservice.BillingTestScenarioService

type CustomerIOBillingReader = eeservice.CustomerIOBillingReader

type EntitlementService = eeservice.EntitlementService

type OrganizationBillingSummary = eeservice.OrganizationBillingSummary

type PaymentMethodRef = eeservice.PaymentMethodRef

type StripeInvoice = eeservice.StripeInvoice

type StripePaymentMethod = eeservice.StripePaymentMethod

type StripeSettlementResult = eeservice.StripeSettlementResult

type UsageFeature = eeservice.UsageFeature

type UsageSeriesPoint = eeservice.UsageSeriesPoint

type WorkspaceBillingCard = eeservice.WorkspaceBillingCard

type WorkspaceUsage = eeservice.WorkspaceUsage

var NewAIUsagePeriodWorker = eeservice.NewAIUsagePeriodWorker

var NewAIUsageReservationSweeper = eeservice.NewAIUsageReservationSweeper

var NewAIUsageService = eeservice.NewAIUsageService

var NewAIUsageSettlementWorker = eeservice.NewAIUsageSettlementWorker

var NewBillingService = eeservice.NewBillingService

var NewBillingTestScenarioService = eeservice.NewBillingTestScenarioService

var NewCustomerIOBillingReader = eeservice.NewCustomerIOBillingReader

var NewEntitlementService = eeservice.NewEntitlementService

var PriceCentsForPlan = eeservice.PriceCentsForPlan

type fakeBillingGateway struct {
	immediateChanges []BillingSubscriptionChangeInput
	cancellations    []BillingSubscriptionCancelInput
	immediateCancels []BillingSubscriptionCancelInput
	resumes          []BillingSubscriptionCancelInput
	checkoutSession  *BillingCheckoutSession
	cancelErr        error
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

func newTestAIUsageService(t *testing.T, store *fakeAIUsageStore) *AIUsageService {
	t.Helper()
	catalog, err := pricing.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	return NewAIUsageService(catalog, store, fixedAIUsageEstimates{})
}

type fakeAIUsageStore struct {
	reconciles    []eerepository.AIUsageReconcileRequest
	reconcileErr  error
	mode          string
	reserveErr    error
	reserveCalls  int
	reservation   eerepository.AIUsageReservationRequest
	reconcile     eerepository.AIUsageReconcileRequest
	checkpoint    eerepository.AIUsageCheckpointRequest
	checkpointErr error
	checkpoints   int
	uncharged     model.AIUsageLedgerEntry
	releasedID    string
	resizeCalls   int
	resizedID     string
	resizedTo     int64
}

func (f *fakeAIUsageStore) Reserve(_ context.Context, input eerepository.AIUsageReservationRequest) (*model.AIUsageReservation, error) {
	f.reserveCalls++
	f.reservation = input
	if f.reserveErr != nil {
		return nil, f.reserveErr
	}
	mode := f.mode
	if mode == "" {
		mode = model.AIUsageEnforcementStrict
	}
	return &model.AIUsageReservation{ID: "reservation", ReservedMicrousd: input.ReservedMicrousd, EnforcementMode: mode}, nil
}

func (f *fakeAIUsageStore) Reconcile(_ context.Context, input eerepository.AIUsageReconcileRequest) (*model.AIUsagePeriod, error) {
	f.reconcile = input
	f.reconciles = append(f.reconciles, input)
	if f.reconcileErr != nil {
		return nil, f.reconcileErr
	}
	return &model.AIUsagePeriod{}, nil
}

func (f *fakeAIUsageStore) Checkpoint(_ context.Context, input eerepository.AIUsageCheckpointRequest) (*model.AIUsagePeriod, error) {
	f.checkpoints++
	f.checkpoint = input
	if f.checkpointErr != nil {
		return nil, f.checkpointErr
	}
	return &model.AIUsagePeriod{}, nil
}

func (f *fakeAIUsageStore) Release(_ context.Context, id, _ string) error {
	f.releasedID = id
	return nil
}

func (f *fakeAIUsageStore) ResizeReservation(_ context.Context, id string, target int64, _ time.Time) error {
	f.resizeCalls++
	f.resizedID = id
	f.resizedTo = target
	return nil
}

func (f *fakeAIUsageStore) RecordUncharged(_ context.Context, input model.AIUsageLedgerEntry) error {
	f.uncharged = input
	return nil
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

type fixedAIUsageEstimates struct{ p90 int64 }

func (f fixedAIUsageEstimates) P90Microusd(context.Context, string, aiusage.Tier, aiusage.FundingMode) (int64, bool, error) {
	return f.p90, f.p90 > 0, nil
}
