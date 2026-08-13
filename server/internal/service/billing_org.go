package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// orgBillingRoleResolver resolves a user's organization role. Implemented by
// OrganizationService.
type orgBillingRoleResolver interface {
	GetMemberRole(ctx context.Context, orgID, userID string) (string, error)
	ListOwners(ctx context.Context, orgID string) ([]model.MemberWithUser, error)
}

// SetOrgRoleResolver wires the organization role resolver used by the billing
// access predicates.
func (s *BillingService) SetOrgRoleResolver(resolver orgBillingRoleResolver) {
	s.orgRoles = resolver
}

// ErrBillingForbidden indicates the requester may not manage the requested
// billing scope.
var ErrBillingForbidden = errors.New("not authorized to manage billing")

// PaymentMethodRef is a compact card reference embedded in a workspace card.
type PaymentMethodRef struct {
	ID       string `json:"id"`
	Brand    string `json:"brand"`
	Last4    string `json:"last4"`
	ExpMonth int    `json:"exp_month"`
	ExpYear  int    `json:"exp_year"`
}

// BillingOwnerRef is a compact billing-owner reference.
type BillingOwnerRef struct {
	UserID string `json:"user_id"`
}

// WorkspaceBillingCard is a per-workspace entry in the org roll-up.
type WorkspaceBillingCard struct {
	WorkspaceID              string            `json:"workspace_id"`
	WorkspaceName            string            `json:"workspace_name"`
	WorkspaceSlug            string            `json:"workspace_slug"`
	Plan                     string            `json:"plan"`
	Status                   string            `json:"status"`
	Locked                   bool              `json:"locked"`
	Trialing                 bool              `json:"trialing"`
	TrialEndsAt              *time.Time        `json:"trial_ends_at,omitempty"`
	CurrentPeriodEnd         time.Time         `json:"current_period_end"`
	AIUsageAllowanceMicrousd int64             `json:"ai_usage_allowance_microusd"`
	AIUsageUsedMicrousd      int64             `json:"ai_usage_used_microusd"`
	AIUsageReservedMicrousd  int64             `json:"ai_usage_reserved_microusd"`
	AIUsagePercent           float64           `json:"ai_usage_percent"`
	ExtraAIUsageEnabled      bool              `json:"extra_ai_usage_enabled"`
	ExtraAIUsageAvailable    bool              `json:"extra_ai_usage_available"`
	PriceCents               int               `json:"price_cents"`
	BillingInterval          string            `json:"billing_interval"`
	PaymentMethod            *PaymentMethodRef `json:"payment_method"`
	BillingOwner             *BillingOwnerRef  `json:"billing_owner"`
	CanManage                bool              `json:"can_manage"`
}

// OrganizationBillingSummary is the org roll-up returned by GET /billing.
type OrganizationBillingSummary struct {
	OrganizationID           string                 `json:"organization_id"`
	TotalMonthlySpendCents   int                    `json:"total_monthly_spend_cents"`
	PaidCount                int                    `json:"paid_count"`
	TrialingCount            int                    `json:"trialing_count"`
	AIUsageUsedMicrousd      int64                  `json:"ai_usage_used_microusd"`
	AIUsageAllowanceMicrousd int64                  `json:"ai_usage_allowance_microusd"`
	AIUsagePercent           float64                `json:"ai_usage_percent"`
	SetupComplete            bool                   `json:"setup_complete"`
	Workspaces               []WorkspaceBillingCard `json:"workspaces"`
}

// UsageFeature is a per-feature usage table row.
type UsageFeature struct {
	FeatureKey       string   `json:"feature_key"`
	Label            string   `json:"label"`
	ModelTier        string   `json:"model_tier,omitempty"`
	ModelTiers       []string `json:"model_tiers"`
	ActionCount      int      `json:"action_count"`
	ActualCount      int      `json:"actual_count"`
	EstimatedCount   int      `json:"estimated_count"`
	ChargedMicrousd  int64    `json:"charged_microusd"`
	InputTokens      int64    `json:"input_tokens"`
	CacheReadTokens  int64    `json:"cache_read_tokens"`
	CacheWriteTokens int64    `json:"cache_write_tokens"`
	OutputTokens     int64    `json:"output_tokens"`
	ReasoningTokens  int64    `json:"reasoning_tokens"`
	Pct              float64  `json:"pct"`
}

// UsageSeriesPoint is one day in the usage chart, with per-feature credits.
type UsageSeriesPoint struct {
	Date     string           `json:"date"`
	Features map[string]int64 `json:"features"`
}

// WorkspaceUsage is the response for GET /billing/usage.
type WorkspaceUsage struct {
	Period                   string             `json:"period"`
	PeriodStart              time.Time          `json:"period_start"`
	PeriodEnd                time.Time          `json:"period_end"`
	Mode                     string             `json:"mode"`
	AIUsageAllowanceMicrousd int64              `json:"ai_usage_allowance_microusd"`
	AIUsageUsedMicrousd      int64              `json:"ai_usage_used_microusd"`
	AIUsageReservedMicrousd  int64              `json:"ai_usage_reserved_microusd"`
	AIUsageOverageMicrousd   int64              `json:"ai_usage_overage_microusd"`
	Series                   []UsageSeriesPoint `json:"series"`
	Features                 []UsageFeature     `json:"features"`
}

// PriceCentsForPlan returns the price (in cents) for a plan + interval.
func PriceCentsForPlan(plan, interval string) int {
	switch plan {
	case model.BillingPlanStarter:
		if interval == "annual" {
			return 94800
		}
		return 9900
	case model.BillingPlanGrowth:
		if interval == "annual" {
			return 286800
		}
		return 29900
	default:
		return 0
	}
}

func billingFeatureLabel(key string) string {
	if feature, ok := AIUsageFeature(key); ok {
		return feature.Label
	}
	if key == "" {
		return "Other"
	}
	return key
}

// CanManageWorkspaceBilling reports whether the user is an owner for billing
// purposes. Billing management is owner-only; admins and delegated billing
// owners are intentionally excluded for now.
func (s *BillingService) CanManageWorkspaceBilling(ctx context.Context, userID, workspaceID string) (bool, error) {
	if s.workspaceRepo != nil {
		role, err := s.workspaceRepo.GetMemberRole(ctx, workspaceID, userID)
		if err != nil {
			return false, err
		}
		if role == model.RoleOwner {
			return true, nil
		}
	}
	orgID, err := s.repo.FindWorkspaceOrgID(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	if orgID == "" {
		return false, nil
	}
	return s.isOrgOwner(ctx, userID, orgID)
}

// WorkspaceBillingManagers returns the people who can manage this workspace's
// billing — the organization owner(s) and the workspace owner(s) — deduped by
// user, so non-owners viewing the read-only billing page know who to contact.
// Org owners take precedence over the workspace-owner label when a person holds
// both roles. Best-effort: partial data is returned rather than failing.
func (s *BillingService) WorkspaceBillingManagers(ctx context.Context, workspaceID string) ([]BillingManagerRef, error) {
	managers := make([]BillingManagerRef, 0, 2)
	seen := map[string]bool{}

	orgID, err := s.repo.FindWorkspaceOrgID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if orgID != "" && s.orgRoles != nil {
		owners, err := s.orgRoles.ListOwners(ctx, orgID)
		if err != nil {
			return nil, err
		}
		for _, o := range owners {
			if seen[o.UserID] {
				continue
			}
			seen[o.UserID] = true
			managers = append(managers, BillingManagerRef{
				UserID: o.UserID,
				Name:   o.FullName,
				Email:  o.Email,
				Role:   "Organization owner",
			})
		}
	}

	if s.workspaceRepo != nil {
		members, err := s.workspaceRepo.ListMembers(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			if m.Role != model.RoleOwner || seen[m.UserID] {
				continue
			}
			seen[m.UserID] = true
			managers = append(managers, BillingManagerRef{
				UserID: m.UserID,
				Name:   m.FullName,
				Email:  m.Email,
				Role:   "Workspace owner",
			})
		}
	}

	return managers, nil
}

// CanManageOrgBilling reports whether the user may manage org-level billing.
// This is owner-only; delegated workspace billing owners do not grant org
// billing access.
func (s *BillingService) CanManageOrgBilling(ctx context.Context, userID, orgID string) (bool, error) {
	return s.isOrgOwner(ctx, userID, orgID)
}

// IsOrgOwner reports whether the user is the organization owner.
func (s *BillingService) IsOrgOwner(ctx context.Context, userID, orgID string) (bool, error) {
	return s.isOrgOwner(ctx, userID, orgID)
}

// WorkspaceOrgID returns the organization id owning a workspace.
func (s *BillingService) WorkspaceOrgID(ctx context.Context, workspaceID string) (string, error) {
	return s.repo.FindWorkspaceOrgID(ctx, workspaceID)
}

func (s *BillingService) isOrgOwner(ctx context.Context, userID, orgID string) (bool, error) {
	if s.orgRoles == nil {
		return false, nil
	}
	role, err := s.orgRoles.GetMemberRole(ctx, orgID, userID)
	if err != nil {
		return false, err
	}
	return role == model.RoleOwner, nil
}

// GetOrganizationBilling builds the org roll-up plus per-workspace cards.
func (s *BillingService) GetOrganizationBilling(ctx context.Context, userID, orgID string) (*OrganizationBillingSummary, error) {
	rows, err := s.repo.ListWorkspaceBillingsForOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	orgOwner, err := s.isOrgOwner(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}
	ob, err := s.repo.GetOrganizationBilling(ctx, orgID)
	if err != nil {
		return nil, err
	}
	methods, err := s.repo.ListPaymentMethods(ctx, orgID)
	if err != nil {
		return nil, err
	}
	methodByID := make(map[string]model.BillingPaymentMethod, len(methods))
	for i := range methods {
		methodByID[methods[i].Method.ID] = methods[i].Method
	}

	summary := &OrganizationBillingSummary{
		OrganizationID: orgID,
		Workspaces:     make([]WorkspaceBillingCard, 0, len(rows)),
	}
	for i := range rows {
		b := rows[i].Billing
		plan := b.Plan
		if plan == "" {
			plan = model.BillingPlanGrowth
		}
		interval := b.BillingInterval
		if interval == "" {
			interval = "monthly"
		}
		price := PriceCentsForPlan(plan, interval)
		trialing := b.Status == model.BillingStatusTrialing
		card := WorkspaceBillingCard{
			WorkspaceID:           rows[i].WorkspaceID,
			WorkspaceName:         rows[i].WorkspaceName,
			WorkspaceSlug:         rows[i].WorkspaceSlug,
			Plan:                  plan,
			Status:                b.Status,
			Locked:                billingStatusLocked(b.Status),
			Trialing:              trialing,
			TrialEndsAt:           b.TrialEndsAt,
			CurrentPeriodEnd:      b.CurrentPeriodEnd,
			ExtraAIUsageEnabled:   b.OnDemandEnabled,
			ExtraAIUsageAvailable: billingCanUseOnDemand(&b),
			PriceCents:            price,
			BillingInterval:       interval,
		}
		period, periodErr := s.repo.GetOpenAIUsagePeriod(ctx, b.WorkspaceID)
		if periodErr != nil {
			return nil, periodErr
		}
		if period != nil {
			card.AIUsageAllowanceMicrousd = period.AllowanceMicrousd
			card.AIUsageUsedMicrousd = period.UsedMicrousd
			card.AIUsageReservedMicrousd = period.ReservedMicrousd
			if period.AllowanceMicrousd > 0 {
				card.AIUsagePercent = float64(period.UsedMicrousd) / float64(period.AllowanceMicrousd) * 100
			}
			summary.AIUsageUsedMicrousd += period.UsedMicrousd
			summary.AIUsageAllowanceMicrousd += period.AllowanceMicrousd
		}
		// Resolve linked card: explicit link, else org default.
		linkedID := b.PaymentMethodID
		if linkedID == nil && ob != nil {
			linkedID = ob.DefaultPaymentMethodID
		}
		if linkedID != nil {
			if pm, ok := methodByID[*linkedID]; ok {
				card.PaymentMethod = &PaymentMethodRef{
					ID:       pm.ID,
					Brand:    pm.Brand,
					Last4:    pm.Last4,
					ExpMonth: pm.ExpMonth,
					ExpYear:  pm.ExpYear,
				}
			}
		}
		if b.BillingOwnerUserID != nil {
			card.BillingOwner = &BillingOwnerRef{UserID: *b.BillingOwnerUserID}
		}
		card.CanManage = orgOwner

		if trialing {
			summary.TrialingCount++
		}
		if b.Status == model.BillingStatusActive {
			summary.PaidCount++
			if interval == "annual" {
				summary.TotalMonthlySpendCents += price / 12
			} else {
				summary.TotalMonthlySpendCents += price
			}
		}
		summary.Workspaces = append(summary.Workspaces, card)
	}
	if summary.AIUsageAllowanceMicrousd > 0 {
		summary.AIUsagePercent = float64(summary.AIUsageUsedMicrousd) / float64(summary.AIUsageAllowanceMicrousd) * 100
	}
	summary.SetupComplete = ob != nil && (ob.FounderPlanEnabled || (ob.StripeCustomerID != nil && len(methods) > 0))
	return summary, nil
}

// ListPaymentMethods returns saved cards with linked-workspace counts.
func (s *BillingService) ListPaymentMethods(ctx context.Context, orgID string) ([]model.PaymentMethodSummary, error) {
	rows, err := s.repo.ListPaymentMethods(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]model.PaymentMethodSummary, 0, len(rows))
	for i := range rows {
		pm := rows[i].Method
		out = append(out, model.PaymentMethodSummary{
			ID:               pm.ID,
			OrganizationID:   pm.OrganizationID,
			Brand:            pm.Brand,
			Last4:            pm.Last4,
			ExpMonth:         pm.ExpMonth,
			ExpYear:          pm.ExpYear,
			Cardholder:       pm.Cardholder,
			IsOrgDefault:     pm.IsOrgDefault,
			LinkedWorkspaces: rows[i].Linked,
			StripePaymentID:  pm.StripePaymentMethodID,
		})
	}
	return out, nil
}

func (s *BillingService) ensureOrgCustomer(ctx context.Context, orgID, email string) (string, error) {
	ob, err := s.repo.GetOrganizationBilling(ctx, orgID)
	if err != nil {
		return "", err
	}
	if ob != nil && ob.StripeCustomerID != nil && *ob.StripeCustomerID != "" {
		return *ob.StripeCustomerID, nil
	}
	customerID, err := s.gateway.EnsureCustomer(ctx, orgID, email)
	if err != nil {
		return "", err
	}
	row := &model.OrganizationBilling{OrganizationID: orgID, StripeCustomerID: &customerID}
	if ob != nil {
		row.ID = ob.ID
		row.DefaultPaymentMethodID = ob.DefaultPaymentMethodID
	}
	if err := s.repo.UpsertOrganizationBilling(ctx, row); err != nil {
		return "", err
	}
	return customerID, nil
}

// UpdatePaymentMethod edits card details and/or sets it as the org default.
func (s *BillingService) UpdatePaymentMethod(ctx context.Context, orgID, cardID string, req model.UpdatePaymentMethodRequest) (*model.BillingPaymentMethod, error) {
	pm, err := s.repo.GetPaymentMethod(ctx, orgID, cardID)
	if err != nil {
		return nil, err
	}
	if pm == nil {
		return nil, fmt.Errorf("payment method not found")
	}
	if req.Cardholder != nil {
		pm.Cardholder = strings.TrimSpace(*req.Cardholder)
	}
	if req.ExpMonth != nil {
		pm.ExpMonth = *req.ExpMonth
	}
	if req.ExpYear != nil {
		pm.ExpYear = *req.ExpYear
	}
	if err := s.repo.UpdatePaymentMethod(ctx, pm); err != nil {
		return nil, err
	}
	if req.SetAsDefault != nil && *req.SetAsDefault {
		if err := s.repo.SetDefaultPaymentMethod(ctx, orgID, cardID); err != nil {
			return nil, err
		}
		pm.IsOrgDefault = true
		if s.gateway != nil {
			ob, err := s.repo.GetOrganizationBilling(ctx, orgID)
			if err == nil && ob != nil && ob.StripeCustomerID != nil {
				if derr := s.gateway.SetDefaultPaymentMethod(ctx, *ob.StripeCustomerID, pm.StripePaymentMethodID); derr != nil {
					return nil, derr
				}
			}
		}
	}
	return pm, nil
}

// DeletePaymentMethod detaches the card in Stripe and unlinks workspaces using it.
func (s *BillingService) DeletePaymentMethod(ctx context.Context, orgID, cardID string) error {
	pm, err := s.repo.GetPaymentMethod(ctx, orgID, cardID)
	if err != nil {
		return err
	}
	if pm == nil {
		return fmt.Errorf("payment method not found")
	}
	if s.gateway != nil && pm.StripePaymentMethodID != "" {
		if err := s.gateway.DetachPaymentMethod(ctx, pm.StripePaymentMethodID); err != nil {
			return err
		}
	}
	return s.repo.DeletePaymentMethod(ctx, orgID, cardID)
}

// LinkWorkspacePaymentMethod links (or clears with nil) a workspace's card. A
// non-nil card id must belong to the workspace's organization.
func (s *BillingService) LinkWorkspacePaymentMethod(ctx context.Context, workspaceID string, paymentMethodID *string) error {
	if paymentMethodID != nil {
		orgID, err := s.repo.FindWorkspaceOrgID(ctx, workspaceID)
		if err != nil {
			return err
		}
		pm, err := s.repo.GetPaymentMethod(ctx, orgID, *paymentMethodID)
		if err != nil {
			return err
		}
		if pm == nil {
			return fmt.Errorf("payment method not found for this organization")
		}
	}
	return s.repo.LinkWorkspacePaymentMethod(ctx, workspaceID, paymentMethodID)
}

// ListInvoices returns the org's Stripe invoices. Degrades to an empty list
// when the gateway or customer is unavailable.
func (s *BillingService) ListInvoices(ctx context.Context, orgID string) ([]StripeInvoice, error) {
	if s.gateway == nil {
		return []StripeInvoice{}, nil
	}
	ob, err := s.repo.GetOrganizationBilling(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if ob == nil || ob.StripeCustomerID == nil || *ob.StripeCustomerID == "" {
		return []StripeInvoice{}, nil
	}
	return s.gateway.ListInvoices(ctx, *ob.StripeCustomerID)
}

// GetWorkspaceUsage returns daily/cumulative usage for a workspace in a period.
func (s *BillingService) GetWorkspaceUsage(ctx context.Context, workspaceID, period, mode, startParam, endParam string) (*WorkspaceUsage, error) {
	if mode != "cumulative" {
		mode = "daily"
	}
	start, end, err := parseUsageWindow(period, startParam, endParam, s.now().UTC())
	if err != nil {
		return nil, err
	}
	summary, err := s.GetWorkspaceBilling(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.UsageByDayFeature(ctx, workspaceID, start, end)
	if err != nil {
		return nil, err
	}

	featureRows := map[string]UsageFeature{}
	featureTiers := map[string]map[string]struct{}{}
	var totalCharged int64
	dayIndex := map[string]int{}
	series := make([]UsageSeriesPoint, 0)
	for _, row := range rows {
		key := row.FeatureKey
		feature := featureRows[key]
		feature.FeatureKey, feature.Label = row.FeatureKey, billingFeatureLabel(row.FeatureKey)
		if featureTiers[key] == nil {
			featureTiers[key] = map[string]struct{}{}
		}
		if row.ModelTier != "" {
			featureTiers[key][row.ModelTier] = struct{}{}
		}
		feature.ActionCount += row.Entries
		feature.ActualCount += row.ActualEntries
		feature.EstimatedCount += row.EstimatedEntries
		feature.ChargedMicrousd += row.ChargedMicrousd
		feature.InputTokens += row.InputTokens
		feature.CacheReadTokens += row.CacheReadTokens
		feature.CacheWriteTokens += row.CacheWriteTokens
		feature.OutputTokens += row.OutputTokens
		feature.ReasoningTokens += row.ReasoningTokens
		featureRows[key] = feature
		totalCharged += row.ChargedMicrousd
		idx, ok := dayIndex[row.Day]
		if !ok {
			idx = len(series)
			dayIndex[row.Day] = idx
			series = append(series, UsageSeriesPoint{Date: row.Day, Features: map[string]int64{}})
		}
		series[idx].Features[row.FeatureKey] += row.ChargedMicrousd
	}

	if mode == "cumulative" {
		running := map[string]int64{}
		for i := range series {
			for f, c := range series[i].Features {
				running[f] += c
			}
			cum := make(map[string]int64, len(running))
			for f, c := range running {
				cum[f] = c
			}
			series[i].Features = cum
		}
	}

	features := make([]UsageFeature, 0, len(featureRows))
	for key, feature := range featureRows {
		feature.ModelTiers = orderedModelTiers(featureTiers[key])
		if len(feature.ModelTiers) == 1 {
			feature.ModelTier = feature.ModelTiers[0]
		}
		if totalCharged > 0 {
			feature.Pct = float64(feature.ChargedMicrousd) / float64(totalCharged) * 100
		}
		features = append(features, feature)
	}
	sort.Slice(features, func(i, j int) bool {
		if features[i].ChargedMicrousd != features[j].ChargedMicrousd {
			return features[i].ChargedMicrousd > features[j].ChargedMicrousd
		}
		return features[i].FeatureKey < features[j].FeatureKey
	})

	return &WorkspaceUsage{
		Period: period, PeriodStart: start, PeriodEnd: end, Mode: mode,
		AIUsageAllowanceMicrousd: summary.AIUsageAllowanceMicrousd,
		AIUsageUsedMicrousd:      summary.AIUsageUsedMicrousd,
		AIUsageReservedMicrousd:  summary.AIUsageReservedMicrousd,
		AIUsageOverageMicrousd:   summary.AIUsageOverageMicrousd,
		Series:                   series, Features: features,
	}, nil
}

func orderedModelTiers(values map[string]struct{}) []string {
	order := []string{"small", "medium", "large", "flagship"}
	result := make([]string, 0, len(values))
	for _, tier := range order {
		if _, ok := values[tier]; ok {
			result = append(result, tier)
		}
	}
	unknown := make([]string, 0, len(values))
	for tier := range values {
		known := false
		for _, standard := range order {
			if tier == standard {
				known = true
				break
			}
		}
		if !known {
			unknown = append(unknown, tier)
		}
	}
	sort.Strings(unknown)
	return append(result, unknown...)
}

// parseBillingPeriod resolves a "YYYY-MM" period into [start, end). An empty
// period defaults to the current month.
func parseBillingPeriod(period string, now time.Time) (time.Time, time.Time, error) {
	if strings.TrimSpace(period) == "" {
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0), nil
	}
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid period %q (want YYYY-MM)", period)
	}
	start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0), nil
}

func parseUsageWindow(period, startParam, endParam string, now time.Time) (time.Time, time.Time, error) {
	startParam = strings.TrimSpace(startParam)
	endParam = strings.TrimSpace(endParam)
	if startParam == "" && endParam == "" {
		return parseBillingPeriod(period, now)
	}
	if startParam == "" || endParam == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("start and end must be provided together")
	}
	start, err := time.Parse(time.RFC3339, startParam)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start %q (want RFC3339)", startParam)
	}
	end, err := time.Parse(time.RFC3339, endParam)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end %q (want RFC3339)", endParam)
	}
	start = start.UTC()
	end = end.UTC()
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be after start")
	}
	return start, end, nil
}
