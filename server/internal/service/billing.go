package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

	billingTrialDays      = 14
	billingFounderCredits = 100000
)

type BillingStripeGateway interface {
	CreateCheckoutSession(ctx context.Context, input BillingCheckoutInput) (string, error)
	RetrieveCheckoutSession(ctx context.Context, sessionID string) (*BillingCheckoutSession, error)
	CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error)
	PreviewSubscriptionPriceChange(ctx context.Context, input BillingSubscriptionChangeInput) (*BillingStripeInvoicePreview, error)
	UpdateSubscriptionPrice(ctx context.Context, input BillingSubscriptionChangeInput) error
	CancelSubscriptionAtPeriodEnd(ctx context.Context, input BillingSubscriptionCancelInput) error
	CancelSubscriptionImmediately(ctx context.Context, input BillingSubscriptionCancelInput) error
	ResumeSubscription(ctx context.Context, input BillingSubscriptionCancelInput) error
	EnsureCustomer(ctx context.Context, orgID, email string) (string, error)
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
	UserID               string
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
	OccurredAt           time.Time
	CanceledAt           *time.Time
}

type billingAnalyticsState struct {
	Plan              string
	Interval          string
	Status            string
	SubscriptionID    string
	CancelAtPeriodEnd bool
}

type BillingStripeInvoiceEvent struct {
	EventID        string
	EventType      string
	SubscriptionID string
	CustomerID     string
	InvoiceID      string
	OccurredAt     time.Time
}

type BillingStripeTrialWillEndEvent struct {
	EventID        string
	EventType      string
	SubscriptionID string
	CustomerID     string
	TrialEndsAt    time.Time
	OccurredAt     time.Time
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

type BillingCreditConsumption = model.BillingCreditConsumption

type BillingCreditPreflight = model.BillingCreditPreflight

// BillingManagerRef identifies a person who can manage this workspace's
// billing (a workspace owner or the organization owner), for display so
// non-owners know who to contact.
type BillingManagerRef = model.BillingManagerRef

type BillingSummary = model.BillingSummary

type BillingService struct {
	repo             *repository.BillingRepository
	gateway          BillingStripeGateway
	now              func() time.Time
	priceConf        BillingPriceConfig
	orgRoles         orgBillingRoleResolver
	workspaceRepo    *repository.WorkspaceRepository
	customerIO       billingIdentity
	customerIOOutbox *repository.CustomerIOLifecycleOutboxRepository
	productAnalytics billingAnalytics
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

func (s *BillingService) SetCustomerIOIdentityService(identity billingIdentity) {
	s.customerIO = identity
}

// SetCustomerIOLifecycleOutboxRepository enables durable lifecycle delivery.
func (s *BillingService) SetCustomerIOLifecycleOutboxRepository(repo *repository.CustomerIOLifecycleOutboxRepository) {
	s.customerIOOutbox = repo
}

// SetProductAnalyticsService enables canonical backend product events.
func (s *BillingService) SetProductAnalyticsService(analytics billingAnalytics) {
	s.productAnalytics = analytics
}

func (s *BillingService) lifecycleEvent(input repository.CustomerIOLifecycleEventInput) *repository.CustomerIOLifecycleEventInput {
	if s.customerIOOutbox == nil {
		return nil
	}
	return &input
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
	founderOrg, err := s.repo.WorkspaceHasFounderPlanOrganization(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if founderOrg {
		periodEnd := now.AddDate(0, 1, 0)
		billing := &model.WorkspaceBilling{
			WorkspaceID:        workspaceID,
			Plan:               model.BillingPlanFounder,
			Status:             model.BillingStatusActive,
			BillingInterval:    "monthly",
			IncludedCredits:    includedCreditsForPlan(model.BillingPlanFounder),
			CreditsUsed:        0,
			OnDemandEnabled:    false,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   periodEnd,
		}
		if err := s.repo.UpsertWorkspaceBilling(ctx, billing); err != nil {
			return nil, err
		}
		summary := s.summary(billing)
		if err := s.addAIUsage(ctx, summary); err != nil {
			return nil, err
		}
		if err := s.addSeatEntitlements(ctx, summary); err != nil {
			return nil, err
		}
		s.syncCustomerIOWorkspace(ctx, workspaceID)
		return summary, nil
	}

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
	if s.customerIOOutbox != nil {
		persisted, _, err := s.repo.CreateTrialWithLifecycleEvent(ctx, billing, repository.CustomerIOLifecycleEventInput{
			SemanticKey: fmt.Sprintf("trial_started:%s:%d", workspaceID, trialEnds.Unix()),
			WorkspaceID: workspaceID,
			EventName:   "trial_started",
			OccurredAt:  now,
			Attributes:  map[string]any{"trial_ends_at": trialEnds},
		})
		if err != nil {
			return nil, err
		}
		billing = persisted
	} else if err := s.repo.UpsertWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	s.trackProductAnalytics(ctx, ProductAnalyticsEvent{
		SemanticKey: fmt.Sprintf("trial_started:%s:%d", workspaceID, trialEnds.Unix()),
		WorkspaceID: workspaceID,
		Name:        "trial_started", Source: "system", OccurredAt: now,
		Attributes: map[string]any{"trial_ends_at": trialEnds, "plan": billing.Plan},
	})
	summary := s.summary(billing)
	if err := s.addAIUsage(ctx, summary); err != nil {
		return nil, err
	}
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

// NextAIUsagePeriodSchedule derives the next renewal-anniversary allowance from subscription state.
func (s *BillingService) NextAIUsagePeriodSchedule(ctx context.Context, workspaceID string, start time.Time) (repository.AIUsagePeriodSchedule, error) {
	billing, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return repository.AIUsagePeriodSchedule{}, err
	}
	if billing == nil {
		return repository.AIUsagePeriodSchedule{}, fmt.Errorf("workspace billing is missing")
	}
	mode := model.AIUsageEnforcementStrict
	if billing.Plan == model.BillingPlanFounder {
		mode = model.AIUsageEnforcementSoft
	} else if billing.OnDemandEnabled {
		mode = model.AIUsageEnforcementExtra
	}
	allowance := aiUsageAllowanceMicrousd(billing.Plan, billing.BillingInterval, billing.Status)
	end := NextAIUsageBoundary(billing.CurrentPeriodStart, start)
	if billing.Status == model.BillingStatusTrialing && billing.TrialEndsAt != nil && billing.TrialEndsAt.After(start) {
		end = *billing.TrialEndsAt
	}
	return repository.AIUsagePeriodSchedule{
		WorkspaceID: workspaceID, Plan: billing.Plan, BillingInterval: billing.BillingInterval,
		PricingVersion: "2026-08-13", EnforcementMode: mode, Anchor: billing.CurrentPeriodStart,
		Start: start, End: end, AllowanceMicrousd: allowance,
	}, nil
}

func aiUsageAllowanceMicrousd(plan, interval, status string) int64 {
	if plan == model.BillingPlanFounder {
		return 150_000_000
	}
	if status == model.BillingStatusTrialing {
		return 140_000_000
	}
	if plan == model.BillingPlanStarter {
		if interval == "annual" {
			return 79_000_000
		}
		return 99_000_000
	}
	if interval == "annual" {
		return 239_000_000
	}
	return 299_000_000
}

func (s *BillingService) CanReserveWorkspaceSeat(ctx context.Context, workspaceID string) error {
	billing, err := s.repo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if billing != nil && billingStatusLocked(billing.Status) {
		return model.ErrBillingWorkspaceLocked
	}
	return nil
}

func (s *BillingService) PreflightCredits(ctx context.Context, input BillingCreditPreflight) error {
	if input.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if input.Credits <= 0 {
		return nil
	}
	summary, err := s.GetWorkspaceBilling(ctx, input.WorkspaceID)
	if err != nil {
		return err
	}
	if summary.Locked {
		return model.ErrBillingWorkspaceLocked
	}
	nextUsed := summary.CreditsUsed + input.Credits
	if nextUsed > summary.IncludedCredits && !summary.OnDemandEnabled {
		return model.ErrAIUsageExhausted
	}
	if nextUsed > summary.IncludedCredits && !summary.OnDemandAvailable {
		return model.ErrExtraAIUsageUnavailable
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
	if billing.Plan == model.BillingPlanFounder && enabled {
		return nil, fmt.Errorf("Founder plan includes monthly AI usage and does not support extra AI usage billing")
	}
	if enabled && !billingCanUseOnDemand(billing) {
		return nil, fmt.Errorf("extra AI usage is available only on active paid workspaces")
	}
	changed := billing.OnDemandEnabled != enabled
	billing.OnDemandEnabled = enabled
	if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	s.syncCustomerIOWorkspace(ctx, workspaceID)
	if changed {
		s.trackProductAnalytics(ctx, ProductAnalyticsEvent{
			SemanticKey: fmt.Sprintf("on_demand_billing_changed:%s:%d", workspaceID, s.now().UTC().UnixNano()),
			WorkspaceID: workspaceID, Name: "on_demand_billing_changed", Source: "api",
			Attributes: map[string]any{"on_demand_enabled": enabled, "plan": billing.Plan},
		})
	}
	summary := s.summary(billing)
	if err := s.addAIUsage(ctx, summary); err != nil {
		return nil, err
	}
	if err := s.addSeatEntitlements(ctx, summary); err != nil {
		return nil, err
	}
	return summary, nil
}

func (s *BillingService) CreateCheckoutSession(ctx context.Context, input BillingCheckoutRequest) (string, error) {
	if s.gateway == nil {
		return "", fmt.Errorf("stripe billing is not configured")
	}
	billing, err := s.repo.GetByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return "", err
	}
	if input.Plan == model.BillingPlanFounder || (billing != nil && billing.Plan == model.BillingPlanFounder) {
		return "", fmt.Errorf("Founder plan is managed by Helpin and cannot be changed in Stripe")
	}
	priceID, err := s.priceIDFor(input.Plan, input.Interval)
	if err != nil {
		return "", err
	}
	if billing != nil && billing.StripeSubscriptionID != nil {
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
	if session.CurrentPeriodStart.IsZero() {
		session.CurrentPeriodStart = s.now().UTC()
	}
	if periodEnd.IsZero() {
		now := session.CurrentPeriodStart
		if interval == "annual" {
			periodEnd = now.AddDate(1, 0, 0)
		} else {
			periodEnd = now.AddDate(0, 1, 0)
		}
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

func (s *BillingService) ExpireOverdueTrials(ctx context.Context) (int64, error) {
	if s.repo == nil {
		return 0, nil
	}
	now := s.now().UTC()
	if s.customerIOOutbox != nil {
		return s.repo.ExpireOverdueTrialsWithLifecycleEvents(ctx, now)
	}
	workspaceIDs, err := s.repo.ListOverdueTrialWorkspaceIDs(ctx, now)
	if err != nil {
		return 0, err
	}
	if len(workspaceIDs) == 0 {
		return 0, nil
	}

	expired, err := s.repo.ExpireOverdueTrials(ctx, now)
	if err != nil {
		return 0, err
	}
	for _, workspaceID := range workspaceIDs {
		billing, getErr := s.repo.GetByWorkspaceID(ctx, workspaceID)
		if getErr != nil {
			slog.ErrorContext(ctx, "failed to load expired workspace billing for Customer.io", "error", getErr, "workspace_id", workspaceID)
			continue
		}
		attributes := map[string]any{"expired_at": now}
		if billing != nil {
			attributes["plan"] = billing.Plan
			if billing.TrialEndsAt != nil {
				attributes["trial_ends_at"] = billing.TrialEndsAt
			}
		}
		s.trackCustomerIOWorkspaceEvent(ctx, workspaceID, "trial_expired", now, attributes)
	}
	return expired, nil
}

func (s *BillingService) PreviewWorkspacePlanChange(ctx context.Context, input BillingPlanChangeRequest) (*BillingPlanChangePreview, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	billing, err := s.repo.GetByWorkspaceID(ctx, input.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if billing == nil {
		return nil, fmt.Errorf("workspace billing not found")
	}
	if input.Plan == model.BillingPlanFounder || billing.Plan == model.BillingPlanFounder {
		return nil, fmt.Errorf("Founder plan is managed by Helpin and cannot be changed in Stripe")
	}
	if s.gateway == nil {
		return nil, fmt.Errorf("stripe billing is not configured")
	}
	priceID, err := s.priceIDFor(input.Plan, input.Interval)
	if err != nil {
		return nil, err
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
	if input.Plan == model.BillingPlanFounder || billing.Plan == model.BillingPlanFounder {
		return nil, fmt.Errorf("Founder plan is managed by Helpin and cannot be changed in Stripe")
	}
	if _, err := s.priceIDFor(input.Plan, input.Interval); err != nil {
		return nil, err
	}
	if billing.StripeSubscriptionID == nil || *billing.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("workspace has no active Stripe subscription")
	}
	if billing.Plan == input.Plan && billing.BillingInterval == input.Interval && !billing.CancelAtPeriodEnd && billing.PendingPlan == nil {
		return s.summaryWithEntitlements(ctx, billing)
	}

	now := s.now().UTC()
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
	if err := s.repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
		return nil, err
	}
	s.syncCustomerIOWorkspace(ctx, input.WorkspaceID)
	return s.summaryWithEntitlements(ctx, billing)
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
	if billing.Plan == model.BillingPlanFounder {
		return nil, fmt.Errorf("Founder plan is managed by Helpin and cannot be changed in Stripe")
	}
	if billing.Status == model.BillingStatusCanceled {
		return nil, fmt.Errorf("subscription is already canceled")
	}
	if billing.StripeSubscriptionID == nil || *billing.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("workspace has no active Stripe subscription")
	}
	if !billing.CancelAtPeriodEnd {
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
	s.syncCustomerIOWorkspace(ctx, workspaceID)
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
	if billing.Plan == model.BillingPlanFounder {
		return "", fmt.Errorf("Founder plan is managed by Helpin and does not use the Stripe billing portal")
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
	if billing.Plan == model.BillingPlanFounder {
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
	if billing.Plan == "" {
		billing.Plan = model.BillingPlanGrowth
	}
	billing.Status = model.BillingStatusCanceled
	billing.IncludedCredits = includedCreditsForPlan(billing.Plan)
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
	s.syncCustomerIOWorkspace(ctx, workspaceID)
	return nil
}

func (s *BillingService) ApplyStripeSubscriptionUpdate(ctx context.Context, update BillingStripeSubscriptionUpdate) (*BillingSummary, error) {
	var previousState billingAnalyticsState
	billing, processed, err := s.repo.ProcessStripeLifecycleEvent(ctx, update.EventID, update.EventType, func(repo *repository.BillingRepository) (*model.WorkspaceBilling, *repository.CustomerIOLifecycleEventInput, error) {
		billing, previous, err := s.applyStripeSubscriptionMutation(ctx, repo, &update)
		previousState = previous
		return billing, nil, err
	})
	if err != nil {
		return nil, err
	}
	if !processed {
		if update.WorkspaceID != "" {
			return s.GetWorkspaceBilling(ctx, update.WorkspaceID)
		}
		return nil, nil
	}
	if billing.Plan != model.BillingPlanFounder {
		s.trackStripeSubscriptionEvents(ctx, previousState, update, billing)
		s.syncCustomerIOWorkspace(ctx, update.WorkspaceID)
	}
	return s.summaryWithEntitlements(ctx, billing)
}

// Both the business write and webhook receipt commit together. Failed historical
// events whose processed flag is false remain eligible for a successful retry.
func (s *BillingService) applyStripeSubscriptionMutation(ctx context.Context, repo *repository.BillingRepository, update *BillingStripeSubscriptionUpdate) (*model.WorkspaceBilling, billingAnalyticsState, error) {
	billing, err := repo.GetByWorkspaceIDForUpdate(ctx, update.WorkspaceID)
	if err != nil {
		return nil, billingAnalyticsState{}, err
	}
	if billing == nil {
		billing = &model.WorkspaceBilling{WorkspaceID: update.WorkspaceID}
	}
	if billing.Plan == model.BillingPlanFounder {
		return billing, billingAnalyticsState{}, nil
	}

	previousState := billingAnalyticsState{
		Plan: billing.Plan, Interval: billing.BillingInterval, Status: billing.Status,
		CancelAtPeriodEnd: billing.CancelAtPeriodEnd,
	}
	if billing.StripeSubscriptionID != nil {
		previousState.SubscriptionID = *billing.StripeSubscriptionID
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
		if update.Plan == "" {
			update.Plan = billing.Plan
		}
		if update.BillingInterval == "" {
			update.BillingInterval = billing.BillingInterval
		}
	}
	periodAdvanced := previousPeriodStart.IsZero() || (!update.CurrentPeriodStart.IsZero() && update.CurrentPeriodStart.After(previousPeriodStart))
	becameLockedOrCanceled := billingStatusLocked(status)

	billing.Plan = update.Plan
	billing.Status = status
	billing.BillingInterval = update.BillingInterval
	billing.IncludedCredits = includedCreditsForPlan(update.Plan)
	if periodAdvanced {
		billing.CreditsUsed = 0
		billing.OnDemandBlocksInvoiced = 0
	}
	if becameLockedOrCanceled {
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
	billing.CurrentPeriodStart = update.CurrentPeriodStart
	billing.CurrentPeriodEnd = update.CurrentPeriodEnd
	billing.StripeCustomerID = optionalBillingString(update.StripeCustomerID)
	billing.StripeSubscriptionID = optionalBillingString(update.StripeSubscriptionID)
	billing.StripePriceID = optionalBillingString(update.StripePriceID)
	billing.LastStripeEventID = optionalBillingString(update.EventID)
	if err := repo.UpsertWorkspaceBilling(ctx, billing); err != nil {
		return nil, billingAnalyticsState{}, err
	}
	return billing, previousState, nil
}

func (s *BillingService) ApplyStripeInvoicePaymentFailed(ctx context.Context, event BillingStripeInvoiceEvent) (*BillingSummary, error) {
	occurredAt := event.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = s.now().UTC()
	}
	billing, processed, err := s.repo.ProcessStripeLifecycleEvent(ctx, event.EventID, event.EventType, func(repo *repository.BillingRepository) (*model.WorkspaceBilling, *repository.CustomerIOLifecycleEventInput, error) {
		billing, err := billingForStripeInvoiceEvent(ctx, repo, event)
		if err != nil || billing == nil {
			return billing, nil, err
		}
		billing.Status = model.BillingStatusPastDue
		billing.BillingNoticeType = optionalBillingString("payment_failed")
		billing.BillingNoticeMessage = optionalBillingString("Payment failed. Update your payment method to keep this workspace active.")
		billing.BillingNoticeAt, billing.PaymentFailedAt = &occurredAt, &occurredAt
		if event.CustomerID != "" {
			billing.StripeCustomerID = optionalBillingString(event.CustomerID)
		}
		if event.SubscriptionID != "" {
			billing.StripeSubscriptionID = optionalBillingString(event.SubscriptionID)
		}
		if err := repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
			return nil, nil, err
		}
		return billing, s.lifecycleEvent(repository.CustomerIOLifecycleEventInput{SemanticKey: "stripe:" + event.EventID, WorkspaceID: billing.WorkspaceID, EventName: "payment_failed", OccurredAt: occurredAt, Attributes: map[string]any{"payment_failed_at": occurredAt}}), nil
	})
	if err != nil || !processed || billing == nil {
		return nil, err
	}
	s.trackProductAnalytics(ctx, ProductAnalyticsEvent{
		SemanticKey: "stripe:" + event.EventID, WorkspaceID: billing.WorkspaceID,
		Name: "payment_failed", Source: "stripe", OccurredAt: occurredAt,
		Attributes: map[string]any{"invoice_id": event.InvoiceID, "subscription_id": event.SubscriptionID},
	})
	return s.summaryWithEntitlements(ctx, billing)
}

func (s *BillingService) ApplyStripeInvoicePaymentSucceeded(ctx context.Context, event BillingStripeInvoiceEvent) (*BillingSummary, error) {
	occurredAt := event.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = s.now().UTC()
	}
	billing, processed, err := s.repo.ProcessStripeLifecycleEvent(ctx, event.EventID, event.EventType, func(repo *repository.BillingRepository) (*model.WorkspaceBilling, *repository.CustomerIOLifecycleEventInput, error) {
		billing, err := billingForStripeInvoiceEvent(ctx, repo, event)
		if err != nil || billing == nil {
			return billing, nil, err
		}
		if billing.Status == model.BillingStatusPastDue {
			billing.Status = model.BillingStatusActive
		}
		billing.BillingNoticeType, billing.BillingNoticeMessage, billing.BillingNoticeAt, billing.PaymentFailedAt = nil, nil, nil, nil
		if event.CustomerID != "" {
			billing.StripeCustomerID = optionalBillingString(event.CustomerID)
		}
		if event.SubscriptionID != "" {
			billing.StripeSubscriptionID = optionalBillingString(event.SubscriptionID)
		}
		if err := repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
			return nil, nil, err
		}
		return billing, s.lifecycleEvent(repository.CustomerIOLifecycleEventInput{SemanticKey: "stripe:" + event.EventID, WorkspaceID: billing.WorkspaceID, EventName: "payment_succeeded", OccurredAt: occurredAt}), nil
	})
	if err != nil || !processed || billing == nil {
		return nil, err
	}
	s.trackProductAnalytics(ctx, ProductAnalyticsEvent{
		SemanticKey: "stripe:" + event.EventID, WorkspaceID: billing.WorkspaceID,
		Name: "payment_succeeded", Source: "stripe", OccurredAt: occurredAt,
		Attributes: map[string]any{"invoice_id": event.InvoiceID, "subscription_id": event.SubscriptionID},
	})
	return s.summaryWithEntitlements(ctx, billing)
}

func (s *BillingService) ApplyStripeTrialWillEnd(ctx context.Context, event BillingStripeTrialWillEndEvent) (*BillingSummary, error) {
	occurredAt := event.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = s.now().UTC()
	}
	billing, processed, err := s.repo.ProcessStripeLifecycleEvent(ctx, event.EventID, event.EventType, func(repo *repository.BillingRepository) (*model.WorkspaceBilling, *repository.CustomerIOLifecycleEventInput, error) {
		billing, err := billingForStripeInvoiceEvent(ctx, repo, BillingStripeInvoiceEvent{SubscriptionID: event.SubscriptionID, CustomerID: event.CustomerID})
		if err != nil || billing == nil {
			return billing, nil, err
		}
		trialEnd := event.TrialEndsAt
		if trialEnd.IsZero() && billing.TrialEndsAt != nil {
			trialEnd = *billing.TrialEndsAt
		}
		billing.BillingNoticeType = optionalBillingString("trial_will_end")
		billing.BillingNoticeMessage = optionalBillingString("Your trial is ending soon. Choose a plan to keep paid features active.")
		billing.BillingNoticeAt = &occurredAt
		if !trialEnd.IsZero() {
			billing.TrialWillEndAt, billing.TrialEndsAt = &trialEnd, &trialEnd
		}
		if event.CustomerID != "" {
			billing.StripeCustomerID = optionalBillingString(event.CustomerID)
		}
		if event.SubscriptionID != "" {
			billing.StripeSubscriptionID = optionalBillingString(event.SubscriptionID)
		}
		if err := repo.UpdateWorkspaceBilling(ctx, billing); err != nil {
			return nil, nil, err
		}
		return billing, s.lifecycleEvent(repository.CustomerIOLifecycleEventInput{SemanticKey: "stripe:" + event.EventID, WorkspaceID: billing.WorkspaceID, EventName: "trial_will_end", OccurredAt: occurredAt, Attributes: map[string]any{"trial_ends_at": trialEnd}}), nil
	})
	if err != nil || !processed || billing == nil {
		return nil, err
	}
	s.trackProductAnalytics(ctx, ProductAnalyticsEvent{
		SemanticKey: "stripe:" + event.EventID, WorkspaceID: billing.WorkspaceID,
		Name: "trial_will_end", Source: "stripe", OccurredAt: occurredAt,
		Attributes: map[string]any{"trial_ends_at": event.TrialEndsAt, "subscription_id": event.SubscriptionID},
	})
	return s.summaryWithEntitlements(ctx, billing)
}

func (s *BillingService) trackProductAnalytics(ctx context.Context, event ProductAnalyticsEvent) {
	if s.productAnalytics != nil {
		s.productAnalytics.Track(ctx, event)
	}
}

func (s *BillingService) syncCustomerIOWorkspace(ctx context.Context, workspaceID string) {
	if s.customerIO == nil || strings.TrimSpace(workspaceID) == "" {
		return
	}
	s.customerIO.SyncWorkspace(ctx, workspaceID, "")
}

func (s *BillingService) trackCustomerIOWorkspaceEvent(ctx context.Context, workspaceID, name string, occurredAt time.Time, attributes map[string]any) {
	if s.customerIO == nil || strings.TrimSpace(workspaceID) == "" {
		return
	}
	s.customerIO.TrackWorkspaceEvent(ctx, workspaceID, name, occurredAt, attributes)
}

func (s *BillingService) billingForStripeInvoiceEvent(ctx context.Context, event BillingStripeInvoiceEvent) (*model.WorkspaceBilling, error) {
	return billingForStripeInvoiceEvent(ctx, s.repo, event)
}

func billingForStripeInvoiceEvent(ctx context.Context, repo *repository.BillingRepository, event BillingStripeInvoiceEvent) (*model.WorkspaceBilling, error) {
	if event.SubscriptionID != "" {
		billing, err := repo.GetByStripeSubscriptionID(ctx, event.SubscriptionID)
		if err != nil || billing != nil {
			return billing, err
		}
	}
	if event.CustomerID != "" {
		return repo.GetByStripeCustomerID(ctx, event.CustomerID)
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

func billingPlanRank(plan string) int {
	switch plan {
	case model.BillingPlanFounder:
		return 3
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
		return nil, fmt.Errorf("AI usage units must be positive")
	}
	if input.IdempotencyKey == "" {
		return nil, fmt.Errorf("idempotency_key is required")
	}

	summary, err := s.GetWorkspaceBilling(ctx, input.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if summary.Locked {
		return nil, model.ErrBillingWorkspaceLocked
	}
	nextUsed := summary.CreditsUsed + input.Credits
	if nextUsed > summary.IncludedCredits && !summary.OnDemandEnabled && !input.AllowOverage {
		return nil, model.ErrAIUsageExhausted
	}
	if nextUsed > summary.IncludedCredits && !summary.OnDemandAvailable && !input.AllowOverage {
		return nil, model.ErrExtraAIUsageUnavailable
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
	// The credit ledger is retained only for pre-cutover compatibility. It must
	// never create a legacy fixed-block Stripe charge; active AI usage settles
	// exact overage through AIUsageSettlementWorker.
	result, err := s.repo.ConsumeCredits(ctx, input.WorkspaceID, input.Credits, entry, nil)
	if err != nil {
		return nil, err
	}

	return s.summaryWithEntitlements(ctx, result.Billing)
}

func (s *BillingService) summarizeAndNormalize(ctx context.Context, billing *model.WorkspaceBilling) (*BillingSummary, error) {
	now := s.now().UTC()
	changed := false
	if billing.Status == model.BillingStatusTrialing && billing.TrialEndsAt != nil && !billing.TrialEndsAt.After(now) && billing.StripeSubscriptionID == nil {
		if s.customerIOOutbox != nil {
			if _, err := s.repo.ExpireOverdueTrialsWithLifecycleEvents(ctx, now); err != nil {
				return nil, err
			}
			reloaded, err := s.repo.GetByWorkspaceID(ctx, billing.WorkspaceID)
			if err != nil {
				return nil, err
			}
			if reloaded != nil {
				billing = reloaded
			}
		} else {
			billing.Status = model.BillingStatusTrialExpired
			billing.IncludedCredits = includedCreditsForPlan(billing.Plan)
			billing.OnDemandEnabled = false
			billing.OnDemandBlocksInvoiced = 0
			changed = true
		}
	}
	if billing.Plan == model.BillingPlanFounder && billing.Status == model.BillingStatusActive && !billing.CurrentPeriodEnd.After(now) {
		billing.CurrentPeriodStart = now
		billing.CurrentPeriodEnd = now.AddDate(0, 1, 0)
		billing.CreditsUsed = 0
		billing.OnDemandEnabled = false
		billing.OnDemandBlocksInvoiced = 0
		billing.TrialEndsAt = nil
		billing.PendingPlan = nil
		billing.PendingBillingInterval = nil
		billing.PendingChangeAt = nil
		billing.CancelAtPeriodEnd = false
		billing.CanceledAt = nil
		billing.BillingNoticeType = nil
		billing.BillingNoticeMessage = nil
		billing.BillingNoticeAt = nil
		billing.PaymentFailedAt = nil
		billing.TrialWillEndAt = nil
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
	if err := s.addAIUsage(ctx, summary); err != nil {
		return nil, err
	}
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
		Locked:                 billingStatusLocked(billing.Status),
		BillingNoticeType:      billingStringValue(billing.BillingNoticeType),
		BillingNoticeMessage:   billingStringValue(billing.BillingNoticeMessage),
		BillingNoticeAt:        billing.BillingNoticeAt,
		PaymentFailedAt:        billing.PaymentFailedAt,
		TrialWillEndAt:         billing.TrialWillEndAt,
		ManageBillingEnabled:   billing.Plan != model.BillingPlanFounder && billing.StripeCustomerID != nil,
		OnDemandBlocksInvoiced: billing.OnDemandBlocksInvoiced,
	}
}

func (s *BillingService) summaryWithEntitlements(ctx context.Context, billing *model.WorkspaceBilling) (*BillingSummary, error) {
	summary := s.summary(billing)
	if err := s.addAIUsage(ctx, summary); err != nil {
		return nil, err
	}
	if err := s.addSeatEntitlements(ctx, summary); err != nil {
		return nil, err
	}
	return summary, nil
}

func (s *BillingService) addAIUsage(ctx context.Context, summary *BillingSummary) error {
	if summary == nil || s.repo == nil {
		return nil
	}
	period, err := s.repo.GetOpenAIUsagePeriod(ctx, summary.WorkspaceID)
	if err != nil {
		return err
	}
	if period == nil {
		schedule, scheduleErr := s.NextAIUsagePeriodSchedule(ctx, summary.WorkspaceID, s.now().UTC())
		if scheduleErr != nil {
			return scheduleErr
		}
		period, err = s.repo.EnsureOpenAIUsagePeriod(ctx, schedule)
		if err != nil || period == nil {
			return err
		}
	}
	desiredMode := model.AIUsageEnforcementStrict
	if summary.Plan == model.BillingPlanFounder {
		desiredMode = model.AIUsageEnforcementSoft
	} else if summary.OnDemandEnabled {
		desiredMode = model.AIUsageEnforcementExtra
	}
	desiredAllowance := aiUsageAllowanceMicrousd(summary.Plan, summary.BillingInterval, summary.Status)
	if err := s.repo.UpdateOpenAIUsageControls(ctx, period, desiredAllowance, desiredMode); err != nil {
		return err
	}
	remaining := period.AllowanceMicrousd - period.UsedMicrousd - period.ReservedMicrousd
	if remaining < 0 {
		remaining = 0
	}
	summary.AIUsageAllowanceMicrousd = period.AllowanceMicrousd
	summary.AIUsageUsedMicrousd = period.UsedMicrousd
	summary.AIUsageRemainingMicrousd = remaining
	summary.AIUsageReservedMicrousd = period.ReservedMicrousd
	summary.AIUsageOverageMicrousd = period.OverageMicrousd
	summary.AIUsagePeriodStart = period.PeriodStart
	summary.AIUsagePeriodEnd = period.PeriodEnd
	summary.AIUsageUnlimited = period.EnforcementMode == model.AIUsageEnforcementSoft
	summary.ExtraAIUsageEnabled = period.EnforcementMode == model.AIUsageEnforcementExtra
	summary.ExtraAIUsageAvailable = summary.OnDemandAvailable
	summary.PricingVersion = period.PricingVersion
	return nil
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
	return nil
}

func billingCanUseOnDemand(billing *model.WorkspaceBilling) bool {
	if billing == nil {
		return false
	}
	if billing.Plan == model.BillingPlanFounder {
		return false
	}
	return billing.Status == model.BillingStatusActive &&
		billing.StripeCustomerID != nil &&
		billing.StripeSubscriptionID != nil
}

func includedCreditsForPlan(plan string) int {
	switch plan {
	case model.BillingPlanStarter:
		return 5000
	case model.BillingPlanGrowth:
		return 25000
	case model.BillingPlanFounder:
		return billingFounderCredits
	default:
		return 0
	}
}

func nextChargeCentsForBilling(billing *model.WorkspaceBilling) int {
	if billing == nil {
		return 0
	}
	if billing.Status == model.BillingStatusTrialing || billingStatusLocked(billing.Status) {
		return 0
	}
	if billing.CancelAtPeriodEnd {
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
	case "trial_expired":
		return model.BillingStatusTrialExpired
	case "past_due", "incomplete", "incomplete_expired":
		return model.BillingStatusPastDue
	case "unpaid":
		return model.BillingStatusUnpaid
	case "canceled":
		return model.BillingStatusCanceled
	default:
		return model.BillingStatusActive
	}
}

func billingStatusLocked(status string) bool {
	return status == model.BillingStatusTrialExpired || status == model.BillingStatusUnpaid || status == model.BillingStatusCanceled
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
	if feature, ok := AIUsageFeature(featureKey); ok && feature.Chargeable {
		return feature.FloorUnits
	}
	return 0
}

// WorkspaceCreated supplies the commercial lifecycle policy to core workspace creation.
func (s *BillingService) WorkspaceCreated(ctx context.Context, workspaceID string) error {
	_, err := s.EnsureTrialForWorkspace(ctx, workspaceID)
	return err
}

// WorkspaceDeleting cancels a paid subscription before its workspace is removed.
func (s *BillingService) WorkspaceDeleting(ctx context.Context, workspaceID string) error {
	return s.CancelWorkspaceSubscriptionImmediately(ctx, workspaceID)
}
