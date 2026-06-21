package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
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
	billingFreeSeatLimit   = 2
)

type BillingStripeGateway interface {
	CreateCheckoutSession(ctx context.Context, input BillingCheckoutInput) (string, error)
	RetrieveCheckoutSession(ctx context.Context, sessionID string) (*BillingCheckoutSession, error)
	CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error)
	PreviewSubscriptionPriceChange(ctx context.Context, input BillingSubscriptionChangeInput) (*BillingStripeInvoicePreview, error)
	UpdateSubscriptionPrice(ctx context.Context, input BillingSubscriptionChangeInput) error
	ScheduleSubscriptionPriceChange(ctx context.Context, input BillingSubscriptionChangeInput) error
	CancelSubscriptionAtPeriodEnd(ctx context.Context, input BillingSubscriptionCancelInput) error
	CancelSubscriptionImmediately(ctx context.Context, input BillingSubscriptionCancelInput) error
	ResumeSubscription(ctx context.Context, input BillingSubscriptionCancelInput) error
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

type BillingCheckoutSession struct {
	ID                   string
	WorkspaceID          string
	Plan                 string
	Interval             string
	Status               string
	PaymentStatus        string
	StripeCustomerID     string
	StripeSubscriptionID string
	StripePriceID        string
	SubscriptionStatus   string
	CurrentPeriodStart   time.Time
	CurrentPeriodEnd     time.Time
	CancelAtPeriodEnd    bool
}

type BillingPlanChangeRequest struct {
	WorkspaceID   string
	Plan          string
	Interval      string
	ProrationDate int64
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
	CancelAtPeriodEnd    bool
	CanceledAt           *time.Time
}

type BillingStripeInvoiceEvent struct {
	EventID        string
	EventType      string
	SubscriptionID string
	CustomerID     string
	InvoiceID      string
}

type BillingStripeTrialWillEndEvent struct {
	EventID        string
	EventType      string
	SubscriptionID string
	CustomerID     string
	TrialEndsAt    time.Time
}

type BillingSubscriptionChangeInput struct {
	WorkspaceID        string
	SubscriptionID     string
	CurrentPriceID     string
	PriceID            string
	Plan               string
	Interval           string
	CurrentPlan        string
	CurrentInterval    string
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	EffectiveAt        time.Time
	ProrationDate      int64
}

type BillingStripeInvoicePreview struct {
	AmountDueCents     int64
	SubtotalCents      int64
	TotalCents         int64
	Currency           string
	NextPaymentAttempt int64
	PeriodStart        int64
	PeriodEnd          int64
	Lines              []BillingStripeInvoicePreviewLine
}

type BillingStripeInvoicePreviewLine struct {
	Description string
	AmountCents int64
	Proration   bool
}

type BillingPlanChangePreview struct {
	WorkspaceID            string                         `json:"workspace_id"`
	CurrentPlan            string                         `json:"current_plan"`
	CurrentInterval        string                         `json:"current_interval"`
	TargetPlan             string                         `json:"target_plan"`
	TargetInterval         string                         `json:"target_interval"`
	Effective              string                         `json:"effective"`
	ProrationDate          int64                          `json:"proration_date"`
	AmountDueCents         int64                          `json:"amount_due_cents"`
	SubtotalCents          int64                          `json:"subtotal_cents"`
	TotalCents             int64                          `json:"total_cents"`
	Currency               string                         `json:"currency"`
	NextPaymentAttempt     *time.Time                     `json:"next_payment_attempt,omitempty"`
	CurrentPeriodEnd       time.Time                      `json:"current_period_end"`
	CurrentIncludedCredits int                            `json:"current_included_credits"`
	TargetIncludedCredits  int                            `json:"target_included_credits"`
	CreditsUsed            int                            `json:"credits_used"`
	CreditsRemainingAfter  int                            `json:"credits_remaining_after"`
	Lines                  []BillingPlanChangePreviewLine `json:"lines"`
}

type BillingPlanChangePreviewLine struct {
	Description string `json:"description"`
	AmountCents int64  `json:"amount_cents"`
	Proration   bool   `json:"proration"`
}

type BillingSubscriptionCancelInput struct {
	WorkspaceID    string
	SubscriptionID string
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
	NextChargeCents        int        `json:"next_charge_cents"`
	OnDemandEnabled        bool       `json:"on_demand_enabled"`
	OnDemandAvailable      bool       `json:"on_demand_available"`
	StripeCustomerID       *string    `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID   *string    `json:"stripe_subscription_id,omitempty"`
	PendingPlan            *string    `json:"pending_plan,omitempty"`
	PendingBillingInterval *string    `json:"pending_billing_interval,omitempty"`
	PendingChangeAt        *time.Time `json:"pending_change_at,omitempty"`
	CancelAtPeriodEnd      bool       `json:"cancel_at_period_end"`
	CanceledAt             *time.Time `json:"canceled_at,omitempty"`
	BillingNoticeType      string     `json:"billing_notice_type,omitempty"`
	BillingNoticeMessage   string     `json:"billing_notice_message,omitempty"`
	BillingNoticeAt        *time.Time `json:"billing_notice_at,omitempty"`
	PaymentFailedAt        *time.Time `json:"payment_failed_at,omitempty"`
	TrialWillEndAt         *time.Time `json:"trial_will_end_at,omitempty"`
	ManageBillingEnabled   bool       `json:"manage_billing_enabled"`
	Warning                string     `json:"warning,omitempty"`
	SeatLimit              int        `json:"seat_limit,omitempty"`
	SeatUsage              int        `json:"seat_usage,omitempty"`
	SeatOverLimit          bool       `json:"seat_over_limit,omitempty"`
	EntitlementWarning     string     `json:"entitlement_warning,omitempty"`
	OnDemandBlocksInvoiced int        `json:"on_demand_blocks_invoiced"`
}

type BillingService struct {
	repo          *repository.BillingRepository
	gateway       BillingStripeGateway
	now           func() time.Time
	priceConf     BillingPriceConfig
	orgRoles      orgBillingRoleResolver
	workspaceRepo *repository.WorkspaceRepository
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

func (s *BillingService) SetWorkspaceRepository(repo *repository.WorkspaceRepository) {
	s.workspaceRepo = repo
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
	summary := s.summary(billing)
	if err := s.addSeatEntitlements(ctx, summary); err != nil {
		return nil, err
	}
	return summary, nil
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

func (s *BillingService) CanReserveWorkspaceSeat(ctx context.Context, workspaceID string) error {
	billing, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if billing == nil || billing.Plan != model.BillingPlanFree {
		return nil
	}
	if s.workspaceRepo == nil {
		return nil
	}
	count, err := s.workspaceRepo.CountBillableSeats(ctx, workspaceID)
	if err != nil {
		return err
	}
	if int(count) >= billingFreeSeatLimit {
		return fmt.Errorf("free plan includes %d seats; upgrade to invite more people", billingFreeSeatLimit)
	}
	return nil
}

func (s *BillingService) SetOnDemandEnabled(ctx context.Context, workspaceID string, enabled bool) (*BillingSummary, error) {
	billing, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if billing == nil {
		return nil, fmt.Errorf("workspace billing not found")
	}
	if enabled && !billingCanUseOnDemand(billing) {
		return nil, fmt.Errorf("on-demand credits are available only on active paid workspaces")
	}
	billing.OnDemandEnabled = enabled
	if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	summary := s.summary(billing)
	if err := s.addSeatEntitlements(ctx, summary); err != nil {
		return nil, err
	}
	return summary, nil
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
	if billing != nil && billing.StripeSubscriptionID != nil && billing.Plan != model.BillingPlanFree {
		return "", fmt.Errorf("workspace already has an active Stripe subscription; use plan change instead")
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

func (s *BillingService) ConfirmCheckoutSession(ctx context.Context, workspaceID, sessionID string) (*BillingSummary, error) {
	if s.gateway == nil {
		return nil, fmt.Errorf("stripe billing is not configured")
	}
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("checkout session ID is required")
	}
	session, err := s.gateway.RetrieveCheckoutSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("checkout session not found")
	}
	if session.WorkspaceID != "" && session.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("checkout session does not belong to this workspace")
	}
	if session.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("checkout session has no subscription yet")
	}
	plan := session.Plan
	interval := session.Interval
	if plan == "" || interval == "" {
		plan, interval = s.planIntervalForPriceID(session.StripePriceID)
	}
	if plan == "" || interval == "" {
		return nil, fmt.Errorf("checkout session price is not configured for Helpin billing")
	}
	periodEnd := session.CurrentPeriodEnd
	if periodEnd.IsZero() {
		now := s.now()
		if interval == "annual" {
			periodEnd = now.AddDate(1, 0, 0)
		} else {
			periodEnd = now.AddDate(0, 1, 0)
		}
		session.CurrentPeriodStart = now
	}
	return s.ApplyStripeSubscriptionUpdate(ctx, BillingStripeSubscriptionUpdate{
		EventID:              "checkout.session.sync:" + session.ID,
		EventType:            "checkout.session.sync",
		WorkspaceID:          workspaceID,
		Plan:                 plan,
		Status:               session.SubscriptionStatus,
		StripeCustomerID:     session.StripeCustomerID,
		StripeSubscriptionID: session.StripeSubscriptionID,
		StripePriceID:        session.StripePriceID,
		BillingInterval:      interval,
		CurrentPeriodStart:   session.CurrentPeriodStart,
		CurrentPeriodEnd:     periodEnd,
		CancelAtPeriodEnd:    session.CancelAtPeriodEnd,
	})
}

func (s *BillingService) PreviewWorkspacePlanChange(ctx context.Context, input BillingPlanChangeRequest) (*BillingPlanChangePreview, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if input.Plan == model.BillingPlanFree {
		return nil, fmt.Errorf("free plan changes are scheduled at renewal and do not need an invoice preview")
	}
	if s.gateway == nil {
		return nil, fmt.Errorf("stripe billing is not configured")
	}
	priceID, err := s.priceIDFor(input.Plan, input.Interval)
	if err != nil {
		return nil, err
	}
	billing, err := s.repo.GetByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if billing == nil {
		return nil, fmt.Errorf("workspace billing not found")
	}
	if billing.StripeSubscriptionID == nil || *billing.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("workspace has no active Stripe subscription")
	}
	if billing.Plan == input.Plan && billing.BillingInterval == input.Interval && !billing.CancelAtPeriodEnd && billing.PendingPlan == nil {
		return nil, fmt.Errorf("workspace is already on this plan")
	}

	prorationDate := s.now().UTC().Unix()
	if input.ProrationDate > 0 {
		prorationDate = input.ProrationDate
	}
	change := BillingSubscriptionChangeInput{
		WorkspaceID:        input.WorkspaceID,
		SubscriptionID:     *billing.StripeSubscriptionID,
		PriceID:            priceID,
		Plan:               input.Plan,
		Interval:           input.Interval,
		CurrentPlan:        billing.Plan,
		CurrentInterval:    billing.BillingInterval,
		CurrentPeriodStart: billing.CurrentPeriodStart,
		CurrentPeriodEnd:   billing.CurrentPeriodEnd,
		EffectiveAt:        billing.CurrentPeriodEnd,
		ProrationDate:      prorationDate,
	}
	if billing.StripePriceID != nil {
		change.CurrentPriceID = *billing.StripePriceID
	}
	preview, err := s.gateway.PreviewSubscriptionPriceChange(ctx, change)
	if err != nil {
		return nil, err
	}
	targetCredits := includedCreditsForPlan(input.Plan)
	remainingAfter := targetCredits - billing.CreditsUsed
	if remainingAfter < 0 {
		remainingAfter = 0
	}
	out := &BillingPlanChangePreview{
		WorkspaceID:            input.WorkspaceID,
		CurrentPlan:            billing.Plan,
		CurrentInterval:        billing.BillingInterval,
		TargetPlan:             input.Plan,
		TargetInterval:         input.Interval,
		Effective:              "immediate",
		ProrationDate:          prorationDate,
		AmountDueCents:         preview.AmountDueCents,
		SubtotalCents:          preview.SubtotalCents,
		TotalCents:             preview.TotalCents,
		Currency:               preview.Currency,
		CurrentPeriodEnd:       billing.CurrentPeriodEnd,
		CurrentIncludedCredits: billing.IncludedCredits,
		TargetIncludedCredits:  targetCredits,
		CreditsUsed:            billing.CreditsUsed,
		CreditsRemainingAfter:  remainingAfter,
	}
	if preview.NextPaymentAttempt > 0 {
		t := time.Unix(preview.NextPaymentAttempt, 0).UTC()
		out.NextPaymentAttempt = &t
	}
	for _, line := range preview.Lines {
		out.Lines = append(out.Lines, BillingPlanChangePreviewLine{
			Description: line.Description,
			AmountCents: line.AmountCents,
			Proration:   line.Proration,
		})
	}
	return out, nil
}

func (s *BillingService) ChangeWorkspacePlan(ctx context.Context, input BillingPlanChangeRequest) (*BillingSummary, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if input.Plan != model.BillingPlanFree {
		if _, err := s.priceIDFor(input.Plan, input.Interval); err != nil {
			return nil, err
		}
	} else if input.Interval == "" {
		input.Interval = "monthly"
	}
	if s.gateway == nil {
		return nil, fmt.Errorf("stripe billing is not configured")
	}

	billing, err := s.repo.GetByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if billing == nil {
		return nil, fmt.Errorf("workspace billing not found")
	}
	if billing.StripeSubscriptionID == nil || *billing.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("workspace has no active Stripe subscription")
	}
	if billing.Plan == input.Plan && billing.BillingInterval == input.Interval && !billing.CancelAtPeriodEnd && billing.PendingPlan == nil {
		return s.summaryWithEntitlements(ctx, billing)
	}

	now := s.now().UTC()
	if input.Plan == model.BillingPlanFree {
		if err := s.ensureCanMoveToFree(ctx, input.WorkspaceID); err != nil {
			return nil, err
		}
		if err := s.gateway.CancelSubscriptionAtPeriodEnd(ctx, BillingSubscriptionCancelInput{
			WorkspaceID:    input.WorkspaceID,
			SubscriptionID: *billing.StripeSubscriptionID,
		}); err != nil {
			return nil, err
		}
		billing.PendingPlan = optionalBillingString(model.BillingPlanFree)
		billing.PendingBillingInterval = optionalBillingString("monthly")
		billing.PendingChangeAt = &billing.CurrentPeriodEnd
		billing.CancelAtPeriodEnd = true
		if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
			return nil, err
		}
		return s.summaryWithEntitlements(ctx, billing)
	}

	priceID, err := s.priceIDFor(input.Plan, input.Interval)
	if err != nil {
		return nil, err
	}
	change := BillingSubscriptionChangeInput{
		WorkspaceID:        input.WorkspaceID,
		SubscriptionID:     *billing.StripeSubscriptionID,
		PriceID:            priceID,
		Plan:               input.Plan,
		Interval:           input.Interval,
		CurrentPlan:        billing.Plan,
		CurrentInterval:    billing.BillingInterval,
		CurrentPeriodStart: billing.CurrentPeriodStart,
		CurrentPeriodEnd:   billing.CurrentPeriodEnd,
		EffectiveAt:        billing.CurrentPeriodEnd,
		ProrationDate:      input.ProrationDate,
	}
	if billing.StripePriceID != nil {
		change.CurrentPriceID = *billing.StripePriceID
	}

	if billingPlanChangeIsDeferred(billing.Plan, billing.BillingInterval, input.Plan, input.Interval) {
		if err := s.gateway.ScheduleSubscriptionPriceChange(ctx, change); err != nil {
			return nil, err
		}
		billing.PendingPlan = optionalBillingString(input.Plan)
		billing.PendingBillingInterval = optionalBillingString(input.Interval)
		billing.PendingChangeAt = &billing.CurrentPeriodEnd
		billing.CancelAtPeriodEnd = false
	} else {
		if err := s.gateway.UpdateSubscriptionPrice(ctx, change); err != nil {
			return nil, err
		}
		billing.Plan = input.Plan
		billing.BillingInterval = input.Interval
		billing.StripePriceID = &priceID
		billing.IncludedCredits = includedCreditsForPlan(input.Plan)
		billing.TrialEndsAt = nil
		billing.PendingPlan = nil
		billing.PendingBillingInterval = nil
		billing.PendingChangeAt = nil
		billing.CancelAtPeriodEnd = false
		if billing.Status == model.BillingStatusTrialing {
			billing.Status = model.BillingStatusActive
		}
		if billing.CurrentPeriodStart.IsZero() {
			billing.CurrentPeriodStart = now
		}
	}
	if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	return s.summaryWithEntitlements(ctx, billing)
}

func (s *BillingService) ensureCanMoveToFree(ctx context.Context, workspaceID string) error {
	if s.workspaceRepo == nil {
		return nil
	}
	count, err := s.workspaceRepo.CountBillableSeats(ctx, workspaceID)
	if err != nil {
		return err
	}
	if int(count) > billingFreeSeatLimit {
		return fmt.Errorf("Free includes %d seats. This workspace has %d seats. Remove %d members or stay on a paid plan.",
			billingFreeSeatLimit,
			count,
			int(count)-billingFreeSeatLimit,
		)
	}
	return nil
}

func (s *BillingService) ResumeWorkspaceSubscription(ctx context.Context, workspaceID string) (*BillingSummary, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if s.gateway == nil {
		return nil, fmt.Errorf("stripe billing is not configured")
	}
	billing, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if billing == nil {
		return nil, fmt.Errorf("workspace billing not found")
	}
	if billing.Status == model.BillingStatusCanceled {
		return nil, fmt.Errorf("subscription is already canceled")
	}
	if billing.StripeSubscriptionID == nil || *billing.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("workspace has no active Stripe subscription")
	}
	if !billing.CancelAtPeriodEnd && !billingPendingFree(billing) {
		return s.summaryWithEntitlements(ctx, billing)
	}
	if err := s.gateway.ResumeSubscription(ctx, BillingSubscriptionCancelInput{
		WorkspaceID:    workspaceID,
		SubscriptionID: *billing.StripeSubscriptionID,
	}); err != nil {
		return nil, err
	}
	billing.PendingPlan = nil
	billing.PendingBillingInterval = nil
	billing.PendingChangeAt = nil
	billing.CancelAtPeriodEnd = false
	billing.CanceledAt = nil
	if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	return s.summaryWithEntitlements(ctx, billing)
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

func (s *BillingService) CancelWorkspaceSubscriptionImmediately(ctx context.Context, workspaceID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if s.gateway == nil {
		return nil
	}
	billing, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if billing == nil || billing.StripeSubscriptionID == nil || strings.TrimSpace(*billing.StripeSubscriptionID) == "" {
		return nil
	}
	if billing.Status == model.BillingStatusCanceled {
		return nil
	}
	if err := s.gateway.CancelSubscriptionImmediately(ctx, BillingSubscriptionCancelInput{
		WorkspaceID:    workspaceID,
		SubscriptionID: *billing.StripeSubscriptionID,
	}); err != nil {
		return err
	}
	now := s.now().UTC()
	billing.Plan = model.BillingPlanFree
	billing.Status = model.BillingStatusCanceled
	billing.BillingInterval = "monthly"
	billing.IncludedCredits = includedCreditsForPlan(model.BillingPlanFree)
	billing.CreditsUsed = 0
	billing.OnDemandEnabled = false
	billing.OnDemandBlocksInvoiced = 0
	billing.PendingPlan = nil
	billing.PendingBillingInterval = nil
	billing.PendingChangeAt = nil
	billing.CancelAtPeriodEnd = false
	billing.CanceledAt = &now
	if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
		return err
	}
	return nil
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
	previousPeriodStart := billing.CurrentPeriodStart
	if update.Plan == "" || update.BillingInterval == "" {
		plan, interval := s.planIntervalForPriceID(update.StripePriceID)
		if update.Plan == "" {
			update.Plan = plan
		}
		if update.BillingInterval == "" {
			update.BillingInterval = interval
		}
	}
	if update.Plan == "" {
		update.Plan = billing.Plan
	}
	if update.BillingInterval == "" {
		update.BillingInterval = billing.BillingInterval
	}
	status := normalizeBillingStatus(update.Status)
	if status == model.BillingStatusCanceled {
		update.Plan = model.BillingPlanFree
		update.BillingInterval = "monthly"
	}
	periodAdvanced := previousPeriodStart.IsZero() || (!update.CurrentPeriodStart.IsZero() && update.CurrentPeriodStart.After(previousPeriodStart))
	becameFreeOrCanceled := update.Plan == model.BillingPlanFree || status == model.BillingStatusCanceled

	billing.Plan = update.Plan
	billing.Status = status
	billing.BillingInterval = update.BillingInterval
	billing.IncludedCredits = includedCreditsForPlan(update.Plan)
	if periodAdvanced || becameFreeOrCanceled {
		billing.CreditsUsed = 0
		billing.OnDemandBlocksInvoiced = 0
	}
	if becameFreeOrCanceled {
		billing.OnDemandEnabled = false
	}
	billing.TrialEndsAt = nil
	billing.CancelAtPeriodEnd = update.CancelAtPeriodEnd
	billing.CanceledAt = update.CanceledAt
	if billing.Status != model.BillingStatusCanceled && update.CanceledAt == nil {
		billing.CanceledAt = nil
	}
	if billing.BillingNoticeType != nil && *billing.BillingNoticeType == "trial_will_end" && status == model.BillingStatusActive {
		billing.BillingNoticeType = nil
		billing.BillingNoticeMessage = nil
		billing.BillingNoticeAt = nil
		billing.TrialWillEndAt = nil
	}
	if billing.PendingPlan != nil && *billing.PendingPlan == update.Plan {
		billing.PendingPlan = nil
		billing.PendingBillingInterval = nil
		billing.PendingChangeAt = nil
	}
	if !update.CancelAtPeriodEnd && billingPendingFree(billing) {
		billing.PendingPlan = nil
		billing.PendingBillingInterval = nil
		billing.PendingChangeAt = nil
	}
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
	return s.summaryWithEntitlements(ctx, billing)
}

func (s *BillingService) ApplyStripeInvoicePaymentFailed(ctx context.Context, event BillingStripeInvoiceEvent) (*BillingSummary, error) {
	if event.EventID != "" {
		isNew, err := s.repo.InsertStripeWebhookEvent(ctx, event.EventID, event.EventType)
		if err != nil {
			return nil, err
		}
		if !isNew {
			return nil, nil
		}
	}
	billing, err := s.billingForStripeInvoiceEvent(ctx, event)
	if err != nil {
		return nil, err
	}
	if billing == nil {
		return nil, nil
	}
	now := s.now().UTC()
	billing.Status = model.BillingStatusPastDue
	billing.BillingNoticeType = optionalBillingString("payment_failed")
	billing.BillingNoticeMessage = optionalBillingString("Payment failed. Update your payment method to keep this workspace active.")
	billing.BillingNoticeAt = &now
	billing.PaymentFailedAt = &now
	if event.CustomerID != "" {
		billing.StripeCustomerID = optionalBillingString(event.CustomerID)
	}
	if event.SubscriptionID != "" {
		billing.StripeSubscriptionID = optionalBillingString(event.SubscriptionID)
	}
	if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	if event.EventID != "" {
		if err := s.repo.MarkStripeWebhookProcessed(ctx, event.EventID); err != nil {
			return nil, err
		}
	}
	return s.summaryWithEntitlements(ctx, billing)
}

func (s *BillingService) ApplyStripeInvoicePaymentSucceeded(ctx context.Context, event BillingStripeInvoiceEvent) (*BillingSummary, error) {
	if event.EventID != "" {
		isNew, err := s.repo.InsertStripeWebhookEvent(ctx, event.EventID, event.EventType)
		if err != nil {
			return nil, err
		}
		if !isNew {
			return nil, nil
		}
	}
	billing, err := s.billingForStripeInvoiceEvent(ctx, event)
	if err != nil {
		return nil, err
	}
	if billing == nil {
		return nil, nil
	}
	if billing.Status == model.BillingStatusPastDue {
		billing.Status = model.BillingStatusActive
	}
	billing.BillingNoticeType = nil
	billing.BillingNoticeMessage = nil
	billing.BillingNoticeAt = nil
	billing.PaymentFailedAt = nil
	if event.CustomerID != "" {
		billing.StripeCustomerID = optionalBillingString(event.CustomerID)
	}
	if event.SubscriptionID != "" {
		billing.StripeSubscriptionID = optionalBillingString(event.SubscriptionID)
	}
	if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	if event.EventID != "" {
		if err := s.repo.MarkStripeWebhookProcessed(ctx, event.EventID); err != nil {
			return nil, err
		}
	}
	return s.summaryWithEntitlements(ctx, billing)
}

func (s *BillingService) ApplyStripeTrialWillEnd(ctx context.Context, event BillingStripeTrialWillEndEvent) (*BillingSummary, error) {
	if event.EventID != "" {
		isNew, err := s.repo.InsertStripeWebhookEvent(ctx, event.EventID, event.EventType)
		if err != nil {
			return nil, err
		}
		if !isNew {
			return nil, nil
		}
	}
	billing, err := s.billingForStripeInvoiceEvent(ctx, BillingStripeInvoiceEvent{
		SubscriptionID: event.SubscriptionID,
		CustomerID:     event.CustomerID,
	})
	if err != nil {
		return nil, err
	}
	if billing == nil {
		return nil, nil
	}
	now := s.now().UTC()
	trialEnd := event.TrialEndsAt
	if trialEnd.IsZero() && billing.TrialEndsAt != nil {
		trialEnd = *billing.TrialEndsAt
	}
	billing.BillingNoticeType = optionalBillingString("trial_will_end")
	billing.BillingNoticeMessage = optionalBillingString("Your trial is ending soon. Choose a plan to keep paid features active.")
	billing.BillingNoticeAt = &now
	if !trialEnd.IsZero() {
		billing.TrialWillEndAt = &trialEnd
		billing.TrialEndsAt = &trialEnd
	}
	if event.CustomerID != "" {
		billing.StripeCustomerID = optionalBillingString(event.CustomerID)
	}
	if event.SubscriptionID != "" {
		billing.StripeSubscriptionID = optionalBillingString(event.SubscriptionID)
	}
	if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	if event.EventID != "" {
		if err := s.repo.MarkStripeWebhookProcessed(ctx, event.EventID); err != nil {
			return nil, err
		}
	}
	return s.summaryWithEntitlements(ctx, billing)
}

func (s *BillingService) billingForStripeInvoiceEvent(ctx context.Context, event BillingStripeInvoiceEvent) (*model.WorkspaceBilling, error) {
	if event.SubscriptionID != "" {
		billing, err := s.repo.GetByStripeSubscriptionID(ctx, event.SubscriptionID)
		if err != nil || billing != nil {
			return billing, err
		}
	}
	if event.CustomerID != "" {
		return s.repo.GetByStripeCustomerID(ctx, event.CustomerID)
	}
	return nil, nil
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

func (s *BillingService) planIntervalForPriceID(priceID string) (string, string) {
	switch priceID {
	case s.priceConf.StarterMonthly:
		return model.BillingPlanStarter, "monthly"
	case s.priceConf.StarterAnnual:
		return model.BillingPlanStarter, "annual"
	case s.priceConf.GrowthMonthly:
		return model.BillingPlanGrowth, "monthly"
	case s.priceConf.GrowthAnnual:
		return model.BillingPlanGrowth, "annual"
	default:
		return "", ""
	}
}

func billingPlanChangeIsDeferred(currentPlan, currentInterval, targetPlan, targetInterval string) bool {
	if targetPlan == model.BillingPlanFree {
		return true
	}
	return false
}

func billingPlanRank(plan string) int {
	switch plan {
	case model.BillingPlanGrowth:
		return 2
	case model.BillingPlanStarter:
		return 1
	default:
		return 0
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
	if summary.SeatOverLimit {
		return nil, errors.New(summary.EntitlementWarning)
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
	return s.summaryWithEntitlements(ctx, billing)
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
	changed := false
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
		changed = true
	}
	expectedCredits := includedCreditsForPlan(billing.Plan)
	if billing.IncludedCredits != expectedCredits {
		billing.IncludedCredits = expectedCredits
		changed = true
	}
	if changed {
		if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
			return nil, err
		}
	}
	summary := s.summary(billing)
	if err := s.addSeatEntitlements(ctx, summary); err != nil {
		return nil, err
	}
	return summary, nil
}

func (s *BillingService) summary(billing *model.WorkspaceBilling) *BillingSummary {
	includedCredits := includedCreditsForPlan(billing.Plan)
	remaining := includedCredits - billing.CreditsUsed
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
		IncludedCredits:        includedCredits,
		CreditsUsed:            billing.CreditsUsed,
		CreditsRemaining:       remaining,
		NextChargeCents:        nextChargeCentsForBilling(billing),
		OnDemandEnabled:        billing.OnDemandEnabled,
		OnDemandAvailable:      billingCanUseOnDemand(billing),
		StripeCustomerID:       billing.StripeCustomerID,
		StripeSubscriptionID:   billing.StripeSubscriptionID,
		PendingPlan:            billing.PendingPlan,
		PendingBillingInterval: billing.PendingBillingInterval,
		PendingChangeAt:        billing.PendingChangeAt,
		CancelAtPeriodEnd:      billing.CancelAtPeriodEnd,
		CanceledAt:             billing.CanceledAt,
		BillingNoticeType:      billingStringValue(billing.BillingNoticeType),
		BillingNoticeMessage:   billingStringValue(billing.BillingNoticeMessage),
		BillingNoticeAt:        billing.BillingNoticeAt,
		PaymentFailedAt:        billing.PaymentFailedAt,
		TrialWillEndAt:         billing.TrialWillEndAt,
		ManageBillingEnabled:   billing.StripeCustomerID != nil,
		OnDemandBlocksInvoiced: billing.OnDemandBlocksInvoiced,
	}
}

func (s *BillingService) summaryWithEntitlements(ctx context.Context, billing *model.WorkspaceBilling) (*BillingSummary, error) {
	summary := s.summary(billing)
	if err := s.addSeatEntitlements(ctx, summary); err != nil {
		return nil, err
	}
	return summary, nil
}

func (s *BillingService) addSeatEntitlements(ctx context.Context, summary *BillingSummary) error {
	if summary == nil || s.workspaceRepo == nil {
		return nil
	}
	count, err := s.workspaceRepo.CountBillableSeats(ctx, summary.WorkspaceID)
	if err != nil {
		return err
	}
	summary.SeatUsage = int(count)
	if summary.Plan != model.BillingPlanFree {
		return nil
	}
	summary.SeatLimit = billingFreeSeatLimit
	if summary.SeatUsage > summary.SeatLimit {
		summary.SeatOverLimit = true
		summary.EntitlementWarning = fmt.Sprintf(
			"This workspace is over the Free plan seat limit: %d seats used, %d included. Remove members or upgrade to invite more people.",
			summary.SeatUsage,
			summary.SeatLimit,
		)
	}
	return nil
}

func billingCanUseOnDemand(billing *model.WorkspaceBilling) bool {
	if billing == nil {
		return false
	}
	return billing.Plan != model.BillingPlanFree &&
		billing.Status != model.BillingStatusCanceled &&
		billing.StripeCustomerID != nil &&
		billing.StripeSubscriptionID != nil
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

func billingPendingFree(billing *model.WorkspaceBilling) bool {
	return billing != nil && billing.PendingPlan != nil && *billing.PendingPlan == model.BillingPlanFree
}

func nextChargeCentsForBilling(billing *model.WorkspaceBilling) int {
	if billing == nil {
		return 0
	}
	if billing.Plan == model.BillingPlanFree || billing.Status == model.BillingStatusCanceled || billing.Status == model.BillingStatusTrialing {
		return 0
	}
	if billing.CancelAtPeriodEnd || billingPendingFree(billing) {
		return 0
	}
	return PriceCentsForPlan(billing.Plan, billing.BillingInterval)
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

func billingStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
