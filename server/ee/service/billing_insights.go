package service

import (
	"context"
	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"strings"
)

// CustomerIOBillingReader projects commercial metadata without granting entitlements.
type CustomerIOBillingReader struct {
	billingRepo   *eerepository.BillingRepository
	workspaceRepo *repository.WorkspaceRepository
}

func NewCustomerIOBillingReader(billing *eerepository.BillingRepository, workspace *repository.WorkspaceRepository) *CustomerIOBillingReader {
	return &CustomerIOBillingReader{billingRepo: billing, workspaceRepo: workspace}
}

func (s *CustomerIOBillingReader) WorkspaceSummary(ctx context.Context, workspaceID string) (*BillingSummary, error) {
	if s.billingRepo == nil {
		return nil, nil
	}
	billing, err := s.billingRepo.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if billing == nil {
		return nil, nil
	}
	summary := customerIOBillingSummary(ctx, billing, s.workspaceRepo)
	if period, periodErr := s.billingRepo.GetOpenAIUsagePeriod(ctx, workspaceID); periodErr == nil && period != nil {
		summary.AIUsageAllowanceMicrousd = period.AllowanceMicrousd
		summary.AIUsageUsedMicrousd = period.UsedMicrousd
		summary.AIUsageReservedMicrousd = period.ReservedMicrousd
		summary.AIUsageOverageMicrousd = period.OverageMicrousd
		summary.AIUsageRemainingMicrousd = max(period.AllowanceMicrousd-period.UsedMicrousd-period.ReservedMicrousd, 0)
		summary.ExtraAIUsageEnabled = period.EnforcementMode == model.AIUsageEnforcementExtra
		summary.ExtraAIUsageAvailable = billingCanUseOnDemand(billing)
		summary.PricingVersion = period.PricingVersion
	}
	return summary, nil
}

func (s *CustomerIOBillingReader) OrganizationSummary(ctx context.Context, orgID string) (model.CustomerIOOrganizationSummary, error) {
	var summary model.CustomerIOOrganizationSummary
	if s.billingRepo == nil {
		return summary, nil
	}
	rows, err := s.billingRepo.ListWorkspaceBillingsForOrg(ctx, orgID)
	if err != nil {
		return summary, err
	}
	summary.WorkspaceCount = len(rows)
	for _, row := range rows {
		billing := row.Billing
		if billing.Status == model.BillingStatusTrialing {
			summary.TrialingWorkspaces++
			summary.HasTrialWorkspace = true
		}
		if billing.Status == model.BillingStatusActive {
			summary.ActiveWorkspaces++
			if billing.StripeSubscriptionID != nil || billing.Plan == model.BillingPlanFounder {
				summary.HasPaidWorkspace = true
				summary.PaidWorkspaceCount++
			}
		}
		if billingStatusLocked(billing.Status) || billing.Status == model.BillingStatusPastDue || billing.Status == model.BillingStatusUnpaid {
			summary.LockedWorkspaces++
		}
		if billingPlanRank(billing.Plan) > billingPlanRank(summary.HighestPlan) {
			summary.HighestPlan = billing.Plan
		}
		if billing.Status == model.BillingStatusActive {
			summary.MonthlyDueCents += monthlyDueCentsForCustomerIO(billing.Plan, billing.BillingInterval)
		}
	}
	return summary, nil
}

func customerIOBillingSummary(ctx context.Context, billing *model.WorkspaceBilling, workspaceRepo *repository.WorkspaceRepository) *BillingSummary {
	if billing == nil {
		return nil
	}
	includedCredits := includedCreditsForPlan(billing.Plan)
	if billing.IncludedCredits > 0 {
		includedCredits = billing.IncludedCredits
	}
	remaining := includedCredits - billing.CreditsUsed
	if remaining < 0 {
		remaining = 0
	}
	summary := &BillingSummary{
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
		ManageBillingEnabled:   billing.StripeCustomerID != nil,
		OnDemandBlocksInvoiced: billing.OnDemandBlocksInvoiced,
	}
	if workspaceRepo != nil {
		if count, err := workspaceRepo.CountBillableSeats(ctx, billing.WorkspaceID); err == nil {
			summary.SeatUsage = int(count)
		}
	}
	return summary
}

func monthlyDueCentsForCustomerIO(plan, interval string) int {
	if plan == model.BillingPlanFounder {
		return 0
	}
	cents := PriceCentsForPlan(plan, interval)
	if strings.EqualFold(interval, "annual") {
		return cents / 12
	}
	return cents
}
