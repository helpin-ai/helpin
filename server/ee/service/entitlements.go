//go:build ee

package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type EntitlementService struct {
	billing *BillingService
}

func NewEntitlementService(billing *BillingService) *EntitlementService {
	return &EntitlementService{billing: billing}
}

func (s *EntitlementService) RequireFeature(ctx context.Context, workspaceID string, feature EntitlementFeature) error {
	if s == nil || s.billing == nil {
		return nil
	}
	summary, err := s.billing.GetWorkspaceBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	if summary.Locked {
		return &EntitlementError{
			Feature: string(feature),
			Message: "workspace is locked; choose a plan to reactivate it",
		}
	}
	def := featureDefinition(feature)
	if def.requiredPlan == "" {
		return nil
	}
	if feature == EntitlementFeatureRemoveBranding && summary.Plan == model.BillingPlanFounder {
		return &EntitlementError{
			Feature:      string(feature),
			RequiredPlan: def.requiredPlan,
			CurrentPlan:  summary.Plan,
			Message:      fmt.Sprintf("%s requires the Growth plan", def.label),
		}
	}
	if billingPlanRank(summary.Plan) < billingPlanRank(def.requiredPlan) {
		return &EntitlementError{
			Feature:      string(feature),
			RequiredPlan: def.requiredPlan,
			CurrentPlan:  summary.Plan,
			Message:      fmt.Sprintf("%s requires the Growth plan", def.label),
		}
	}
	return nil
}

func (s *EntitlementService) RequireLimitUsage(ctx context.Context, workspaceID string, limit EntitlementLimit, current, delta int64) error {
	if s == nil || s.billing == nil {
		return nil
	}
	summary, err := s.billing.GetWorkspaceBilling(ctx, workspaceID)
	if err != nil {
		return err
	}
	if summary.Locked {
		return &EntitlementError{
			Limit:   string(limit),
			Message: "workspace is locked; choose a plan to reactivate it",
		}
	}
	def := limitDefinition(limit)
	allowed, ok := def.byPlan[summary.Plan]
	if !ok || allowed < 0 {
		return nil
	}
	if current+delta > allowed {
		return &EntitlementError{
			Limit:       string(limit),
			CurrentPlan: summary.Plan,
			Message:     fmt.Sprintf("Starter includes up to %s %s", def.displayLimit, def.label),
		}
	}
	return nil
}

type entitlementFeatureDefinition struct {
	label        string
	requiredPlan string
}

func featureDefinition(feature EntitlementFeature) entitlementFeatureDefinition {
	defs := map[EntitlementFeature]entitlementFeatureDefinition{
		EntitlementFeatureCustomAgents:           {label: "Custom AI agents", requiredPlan: model.BillingPlanGrowth},
		EntitlementFeatureAutomationFlows:        {label: "Automation flows", requiredPlan: model.BillingPlanGrowth},
		EntitlementFeatureAgentScheduling:        {label: "Scheduled agents and cron", requiredPlan: model.BillingPlanGrowth},
		EntitlementFeatureAIConversationRouting:  {label: "AI conversation routing", requiredPlan: model.BillingPlanGrowth},
		EntitlementFeatureRoundRobinAssignment:   {label: "Round robin assignment", requiredPlan: model.BillingPlanGrowth},
		EntitlementFeatureSLAPolicies:            {label: "SLA policies", requiredPlan: model.BillingPlanGrowth},
		EntitlementFeatureMultilingualHelpCenter: {label: "Multilingual help center", requiredPlan: model.BillingPlanGrowth},
		EntitlementFeatureAIArticleTranslation:   {label: "AI article translation", requiredPlan: model.BillingPlanGrowth},
		EntitlementFeatureDealAutomation:         {label: "Deal automation", requiredPlan: model.BillingPlanGrowth},
		EntitlementFeatureRemoveBranding:         {label: "Removing Helpin branding", requiredPlan: model.BillingPlanGrowth},
	}
	return defs[feature]
}

type entitlementLimitDefinition struct {
	label        string
	displayLimit string
	byPlan       map[string]int64
}

func limitDefinition(limit EntitlementLimit) entitlementLimitDefinition {
	defs := map[EntitlementLimit]entitlementLimitDefinition{
		EntitlementLimitTeams: {
			label:        "teams",
			displayLimit: "10",
			byPlan: map[string]int64{
				model.BillingPlanStarter: 10,
				model.BillingPlanGrowth:  -1,
				model.BillingPlanFounder: -1,
			},
		},
		EntitlementLimitDocuments: {
			label:        "documents",
			displayLimit: "500",
			byPlan: map[string]int64{
				model.BillingPlanStarter: 500,
				model.BillingPlanGrowth:  -1,
				model.BillingPlanFounder: -1,
			},
		},
		EntitlementLimitContacts: {
			label:        "contacts",
			displayLimit: "5,000",
			byPlan: map[string]int64{
				model.BillingPlanStarter: 5000,
				model.BillingPlanGrowth:  -1,
				model.BillingPlanFounder: -1,
			},
		},
	}
	return defs[limit]
}
