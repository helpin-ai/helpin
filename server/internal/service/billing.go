package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	BillingFeatureSupportAIReply = "support_ai_reply"
	BillingFeatureCRMAction      = "crm_action"
	BillingFeatureDocsGeneration = "docs_generation"
	BillingFeaturePlanningRun    = "planning_run"
	BillingFeatureCodingRun      = "coding_run"

	billingTrialDays       = 14
	billingCreditBlockSize = 5000
	billingCreditBlockCost = 5000
)

type BillingStripeGateway interface {
	CreateCheckoutSession(ctx context.Context, input BillingCheckoutInput) (string, error)
	CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error)
	BillCreditBlock(ctx context.Context, input BillingCreditBlockCharge) error
	EnsureCustomer(ctx context.Context, orgID, email string) (string, error)
	CreateSetupIntent(ctx context.Context, customerID string) (string, error)
	ListPaymentMethods(ctx context.Context, customerID string) ([]StripePaymentMethod, error)
	DetachPaymentMethod(ctx context.Context, paymentMethodID string) error
	SetDefaultPaymentMethod(ctx context.Context, customerID, paymentMethodID string) error
	ListInvoices(ctx context.Context, customerID string) ([]StripeInvoice, error)
}

// StripePaymentMethod is a normalized saved card from Stripe.
type StripePaymentMethod struct {
	ID         string `json:"id"`
	Brand      string `json:"brand"`
	Last4      string `json:"last4"`
	ExpMonth   int    `json:"exp_month"`
	ExpYear    int    `json:"exp_year"`
	Cardholder string `json:"cardholder"`
}

// StripeInvoice is a normalized invoice from Stripe.
type StripeInvoice struct {
	ID          string `json:"id"`
	Number      string `json:"number"`
	Status      string `json:"status"`
	AmountDue   int64  `json:"amount_due_cents"`
	AmountPaid  int64  `json:"amount_paid_cents"`
	Currency    string `json:"currency"`
	Created     int64  `json:"created"`
	HostedURL   string `json:"hosted_invoice_url"`
	PDFURL      string `json:"invoice_pdf_url"`
	PeriodStart int64  `json:"period_start"`
	PeriodEnd   int64  `json:"period_end"`
}

type BillingCheckoutInput struct {
	WorkspaceID string
	UserID      string
	Email       string
	Plan        string
	Interval    string
	CustomerID  string
	ReturnURL   string
	PriceID     string
}

type BillingPriceConfig struct {
	StarterMonthly string
	StarterAnnual  string
	GrowthMonthly  string
	GrowthAnnual   string
}

type BillingCheckoutRequest struct {
	WorkspaceID string
	UserID      string
	Email       string
	Plan        string
	Interval    string
	ReturnURL   string
}

type BillingStripeSubscriptionUpdate struct {
	EventID              string
	EventType            string
	WorkspaceID          string
	Plan                 string
	Status               string
	StripeCustomerID     string
	StripeSubscriptionID string
	StripePriceID        string
	BillingInterval      string
	CurrentPeriodStart   time.Time
	CurrentPeriodEnd     time.Time
}

type BillingCreditBlockCharge struct {
	WorkspaceID    string
	CustomerID     string
	SubscriptionID string
	Blocks         int
	AmountCents    int
	IdempotencyKey string
}

type BillingCreditConsumption struct {
	WorkspaceID    string
	FeatureKey     string
	Credits        int
	IdempotencyKey string
	Metadata       map[string]any
}

type BillingSummary struct {
	WorkspaceID            string     `json:"workspace_id"`
	Plan                   string     `json:"plan"`
	Status                 string     `json:"status"`
	BillingInterval        string     `json:"billing_interval"`
	Trialing               bool       `json:"trialing"`
	TrialEndsAt            *time.Time `json:"trial_ends_at,omitempty"`
	CurrentPeriodStart     time.Time  `json:"current_period_start"`
	CurrentPeriodEnd       time.Time  `json:"current_period_end"`
	IncludedCredits        int        `json:"included_credits"`
	CreditsUsed            int        `json:"credits_used"`
	CreditsRemaining       int        `json:"credits_remaining"`
	OnDemandEnabled        bool       `json:"on_demand_enabled"`
	OnDemandAvailable      bool       `json:"on_demand_available"`
	StripeCustomerID       *string    `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID   *string    `json:"stripe_subscription_id,omitempty"`
	ManageBillingEnabled   bool       `json:"manage_billing_enabled"`
	Warning                string     `json:"warning,omitempty"`
	OnDemandBlocksInvoiced int        `json:"on_demand_blocks_invoiced"`
}

type BillingService struct {
	repo      *repository.BillingRepository
	gateway   BillingStripeGateway
	now       func() time.Time
	priceConf BillingPriceConfig
	orgRoles  orgBillingRoleResolver
}

func NewBillingService(repo *repository.BillingRepository, gateway BillingStripeGateway, now func() time.Time) *BillingService {
	if now == nil {
		now = time.Now
	}
	return &BillingService{repo: repo, gateway: gateway, now: now}
}

func (s *BillingService) SetPriceConfig(config BillingPriceConfig) {
	s.priceConf = config
}

func (s *BillingService) EnsureTrialForWorkspace(ctx context.Context, workspaceID string) (*BillingSummary, error) {
	existing, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return s.summarizeAndNormalize(ctx, existing)
	}

	now := s.now().UTC()
	trialEnds := now.AddDate(0, 0, billingTrialDays)
	billing := &model.WorkspaceBilling{
		WorkspaceID:        workspaceID,
		Plan:               model.BillingPlanGrowth,
		Status:             model.BillingStatusTrialing,
		BillingInterval:    "monthly",
		IncludedCredits:    includedCreditsForPlan(model.BillingPlanGrowth),
		CreditsUsed:        0,
		OnDemandEnabled:    false,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   trialEnds,
		TrialEndsAt:        &trialEnds,
	}
	if err := s.repo.UpsertWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	return s.summary(billing), nil
}

func (s *BillingService) GetWorkspaceBilling(ctx context.Context, workspaceID string) (*BillingSummary, error) {
	billing, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if billing == nil {
		return s.EnsureTrialForWorkspace(ctx, workspaceID)
	}
	return s.summarizeAndNormalize(ctx, billing)
}

func (s *BillingService) SetOnDemandEnabled(ctx context.Context, workspaceID string, enabled bool) (*BillingSummary, error) {
	billing, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if billing == nil {
		return nil, fmt.Errorf("workspace billing not found")
	}
	if billing.Plan == model.BillingPlanFree && enabled {
		return nil, fmt.Errorf("on-demand credits are not available on the free plan")
	}
	billing.OnDemandEnabled = enabled
	if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	return s.summary(billing), nil
}

func (s *BillingService) CreateCheckoutSession(ctx context.Context, input BillingCheckoutRequest) (string, error) {
	if s.gateway == nil {
		return "", fmt.Errorf("stripe billing is not configured")
	}
	priceID, err := s.priceIDFor(input.Plan, input.Interval)
	if err != nil {
		return "", err
	}
	billing, err := s.repo.GetByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return "", err
	}
	var customerID string
	if billing != nil && billing.StripeCustomerID != nil {
		customerID = *billing.StripeCustomerID
	}
	return s.gateway.CreateCheckoutSession(ctx, BillingCheckoutInput{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		Email:       input.Email,
		Plan:        input.Plan,
		Interval:    input.Interval,
		CustomerID:  customerID,
		ReturnURL:   input.ReturnURL,
		PriceID:     priceID,
	})
}

func (s *BillingService) CreatePortalSession(ctx context.Context, workspaceID, returnURL string) (string, error) {
	if s.gateway == nil {
		return "", fmt.Errorf("stripe billing is not configured")
	}
	billing, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	if billing == nil || billing.StripeCustomerID == nil {
		return "", fmt.Errorf("workspace has no Stripe customer")
	}
	return s.gateway.CreatePortalSession(ctx, *billing.StripeCustomerID, returnURL)
}

func (s *BillingService) ApplyStripeSubscriptionUpdate(ctx context.Context, update BillingStripeSubscriptionUpdate) (*BillingSummary, error) {
	if update.EventID != "" {
		isNew, err := s.repo.InsertStripeWebhookEvent(ctx, update.EventID, update.EventType)
		if err != nil {
			return nil, err
		}
		if !isNew {
			if update.WorkspaceID != "" {
				return s.GetWorkspaceBilling(ctx, update.WorkspaceID)
			}
			return nil, nil
		}
	}

	billing, err := s.repo.GetByWorkspaceID(ctx, update.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if billing == nil {
		billing = &model.WorkspaceBilling{WorkspaceID: update.WorkspaceID}
	}
	billing.Plan = update.Plan
	billing.Status = normalizeBillingStatus(update.Status)
	billing.BillingInterval = update.BillingInterval
	billing.IncludedCredits = includedCreditsForPlan(update.Plan)
	billing.CreditsUsed = 0
	billing.OnDemandBlocksInvoiced = 0
	billing.TrialEndsAt = nil
	billing.CurrentPeriodStart = update.CurrentPeriodStart
	billing.CurrentPeriodEnd = update.CurrentPeriodEnd
	billing.StripeCustomerID = optionalBillingString(update.StripeCustomerID)
	billing.StripeSubscriptionID = optionalBillingString(update.StripeSubscriptionID)
	billing.StripePriceID = optionalBillingString(update.StripePriceID)
	billing.LastStripeEventID = optionalBillingString(update.EventID)
	if err := s.repo.UpsertWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	if update.EventID != "" {
		if err := s.repo.MarkStripeWebhookProcessed(ctx, update.EventID); err != nil {
			return nil, err
		}
	}
	return s.summary(billing), nil
}

func (s *BillingService) priceIDFor(plan, interval string) (string, error) {
	switch {
	case plan == model.BillingPlanStarter && interval == "monthly":
		return requirePriceID(s.priceConf.StarterMonthly, "starter monthly")
	case plan == model.BillingPlanStarter && interval == "annual":
		return requirePriceID(s.priceConf.StarterAnnual, "starter annual")
	case plan == model.BillingPlanGrowth && interval == "monthly":
		return requirePriceID(s.priceConf.GrowthMonthly, "growth monthly")
	case plan == model.BillingPlanGrowth && interval == "annual":
		return requirePriceID(s.priceConf.GrowthAnnual, "growth annual")
	default:
		return "", fmt.Errorf("unsupported billing plan or interval")
	}
}

func (s *BillingService) ConsumeCredits(ctx context.Context, input BillingCreditConsumption) (*BillingSummary, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if input.Credits <= 0 {
		return nil, fmt.Errorf("credits must be positive")
	}
	if input.IdempotencyKey == "" {
		return nil, fmt.Errorf("idempotency_key is required")
	}

	summary, err := s.GetWorkspaceBilling(ctx, input.WorkspaceID)
	if err != nil {
		return nil, err
	}
	nextUsed := summary.CreditsUsed + input.Credits
	if nextUsed > summary.IncludedCredits && !summary.OnDemandEnabled {
		return nil, errors.New("billing credits exhausted")
	}
	if nextUsed > summary.IncludedCredits && !summary.OnDemandAvailable {
		return nil, errors.New("on-demand credits are not available")
	}

	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("encode billing usage metadata: %w", err)
	}
	entry := model.BillingCreditLedgerEntry{
		WorkspaceID:    input.WorkspaceID,
		Kind:           model.BillingLedgerKindUsage,
		FeatureKey:     input.FeatureKey,
		Credits:        input.Credits,
		IdempotencyKey: input.IdempotencyKey,
		Metadata:       model.JSONBlob(metadata),
	}
	result, err := s.repo.ConsumeCredits(ctx, input.WorkspaceID, input.Credits, entry)
	if err != nil {
		return nil, err
	}

	billing := result.Billing
	if !result.AlreadyUsed {
		if err := s.billOnDemandBlocksIfNeeded(ctx, billing, input.IdempotencyKey); err != nil {
			return nil, err
		}
	}
	return s.summary(billing), nil
}

func (s *BillingService) billOnDemandBlocksIfNeeded(ctx context.Context, billing *model.WorkspaceBilling, idempotencyKey string) error {
	if billing == nil || !billing.OnDemandEnabled || billing.CreditsUsed <= billing.IncludedCredits {
		return nil
	}
	if billing.StripeCustomerID == nil || billing.StripeSubscriptionID == nil {
		return errors.New("stripe customer and subscription are required for on-demand credits")
	}
	requiredBlocks := int(math.Ceil(float64(billing.CreditsUsed-billing.IncludedCredits) / float64(billingCreditBlockSize)))
	newBlocks := requiredBlocks - billing.OnDemandBlocksInvoiced
	if newBlocks <= 0 {
		return nil
	}
	if s.gateway == nil {
		return errors.New("billing gateway is not configured")
	}
	if err := s.gateway.BillCreditBlock(ctx, BillingCreditBlockCharge{
		WorkspaceID:    billing.WorkspaceID,
		CustomerID:     *billing.StripeCustomerID,
		SubscriptionID: *billing.StripeSubscriptionID,
		Blocks:         newBlocks,
		AmountCents:    newBlocks * billingCreditBlockCost,
		IdempotencyKey: fmt.Sprintf("%s:on_demand:%d", idempotencyKey, requiredBlocks),
	}); err != nil {
		return err
	}
	billing.OnDemandBlocksInvoiced += newBlocks
	if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
		return err
	}
	return nil
}

func (s *BillingService) summarizeAndNormalize(ctx context.Context, billing *model.WorkspaceBilling) (*BillingSummary, error) {
	now := s.now().UTC()
	if billing.Status == model.BillingStatusTrialing && billing.TrialEndsAt != nil && !billing.TrialEndsAt.After(now) && billing.StripeSubscriptionID == nil {
		billing.Plan = model.BillingPlanFree
		billing.Status = model.BillingStatusActive
		billing.BillingInterval = "monthly"
		billing.IncludedCredits = includedCreditsForPlan(model.BillingPlanFree)
		billing.CreditsUsed = 0
		billing.OnDemandEnabled = false
		billing.OnDemandBlocksInvoiced = 0
		billing.CurrentPeriodStart = now
		billing.CurrentPeriodEnd = now.AddDate(0, 1, 0)
		billing.TrialEndsAt = nil
		if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
			return nil, err
		}
	}
	return s.summary(billing), nil
}

func (s *BillingService) summary(billing *model.WorkspaceBilling) *BillingSummary {
	remaining := billing.IncludedCredits - billing.CreditsUsed
	if remaining < 0 {
		remaining = 0
	}
	return &BillingSummary{
		WorkspaceID:            billing.WorkspaceID,
		Plan:                   billing.Plan,
		Status:                 billing.Status,
		BillingInterval:        billing.BillingInterval,
		Trialing:               billing.Status == model.BillingStatusTrialing,
		TrialEndsAt:            billing.TrialEndsAt,
		CurrentPeriodStart:     billing.CurrentPeriodStart,
		CurrentPeriodEnd:       billing.CurrentPeriodEnd,
		IncludedCredits:        billing.IncludedCredits,
		CreditsUsed:            billing.CreditsUsed,
		CreditsRemaining:       remaining,
		OnDemandEnabled:        billing.OnDemandEnabled,
		OnDemandAvailable:      billing.Plan != model.BillingPlanFree && billing.StripeCustomerID != nil && billing.StripeSubscriptionID != nil,
		StripeCustomerID:       billing.StripeCustomerID,
		StripeSubscriptionID:   billing.StripeSubscriptionID,
		ManageBillingEnabled:   billing.StripeCustomerID != nil,
		OnDemandBlocksInvoiced: billing.OnDemandBlocksInvoiced,
	}
}

func includedCreditsForPlan(plan string) int {
	switch plan {
	case model.BillingPlanStarter:
		return 5000
	case model.BillingPlanGrowth:
		return 25000
	default:
		return 1000
	}
}

func requirePriceID(value, label string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("missing Stripe price ID for %s", label)
	}
	return value, nil
}

func normalizeBillingStatus(status string) string {
	switch status {
	case "trialing":
		return model.BillingStatusTrialing
	case "past_due", "unpaid", "incomplete", "incomplete_expired":
		return model.BillingStatusPastDue
	case "canceled":
		return model.BillingStatusCanceled
	default:
		return model.BillingStatusActive
	}
}

func optionalBillingString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func BillingCreditsForFeature(featureKey string) int {
	switch featureKey {
	case BillingFeatureSupportAIReply:
		return 5
	case BillingFeatureCRMAction:
		return 10
	case BillingFeatureDocsGeneration:
		return 20
	case BillingFeaturePlanningRun:
		return 50
	case BillingFeatureCodingRun:
		return 100
	default:
		return 0
	}
}
