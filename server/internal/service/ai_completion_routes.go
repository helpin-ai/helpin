package service

import (
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
)

const defaultAICompletionServiceTier = "standard"

var (
	deepSeekV4FlashRoute = AICompletionRoute{
		Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731",
		ServiceTier: defaultAICompletionServiceTier,
	}
	openRouterLunaRoute = AICompletionRoute{
		Provider: "openrouter", Model: "openai/gpt-5.6-luna",
		ServiceTier: defaultAICompletionServiceTier,
	}
	openRouterGeminiFlashRoute = AICompletionRoute{
		Provider: "openrouter", Model: "google/gemini-3.7-flash",
		ServiceTier: defaultAICompletionServiceTier,
	}
)

// AICompletionRoute identifies one exact provider route used for a direct completion.
type AICompletionRoute struct {
	Provider    string
	Model       string
	ServiceTier string
}

// AICompletionRoutePolicy defines one feature's default and bounded fallback routes.
type AICompletionRoutePolicy struct {
	FeatureKey          string
	OperationKey        string
	Primary             AICompletionRoute
	Fallbacks           []AICompletionRoute
	PreferRequestRoute  bool
	MaximumOutputTokens int
}

// AICompletionRouteRegistry is the immutable product-owned direct-completion route matrix.
type AICompletionRouteRegistry struct {
	policies map[string]AICompletionRoutePolicy
}

// DefaultAICompletionRouteRegistry returns the reviewed production route matrix.
func DefaultAICompletionRouteRegistry() AICompletionRouteRegistry {
	common := func(featureKey string, maximumOutput int) AICompletionRoutePolicy {
		return AICompletionRoutePolicy{
			FeatureKey: featureKey, Primary: deepSeekV4FlashRoute,
			Fallbacks: []AICompletionRoute{openRouterLunaRoute}, MaximumOutputTokens: maximumOutput,
		}
	}
	policies := []AICompletionRoutePolicy{
		common(BillingFeatureAIRouting, 400),
		common(BillingFeatureCoverageGapAnalysis, 1800),
		common(BillingFeatureCRMSignalDetection, 4096),
		common(BillingFeatureSupportReplyRewrite, 900),
		common(BillingFeatureDealAutomationInference, 2048),
		{
			FeatureKey: BillingFeatureMeetingIntelligence, Primary: deepSeekV4FlashRoute,
			Fallbacks: []AICompletionRoute{openRouterGeminiFlashRoute}, MaximumOutputTokens: 8192,
		},
		common(BillingFeatureCRMSummary, 1400),
		common(BillingFeatureTaskStandingBrief, 4096),
		{
			FeatureKey: BillingFeatureSupportAIReply, Primary: deepSeekV4FlashRoute,
			Fallbacks: []AICompletionRoute{openRouterLunaRoute}, PreferRequestRoute: true,
			MaximumOutputTokens: 1024,
		},
		{
			FeatureKey: BillingFeatureSupportTaskDraft, Primary: deepSeekV4FlashRoute,
			Fallbacks: []AICompletionRoute{openRouterLunaRoute}, PreferRequestRoute: true,
			MaximumOutputTokens: 1200,
		},
		common(BillingFeatureDocsArticleTranslation, 4096),
		common(BillingFeatureDocsArticleGeneration, 2000),
		{
			FeatureKey: BillingFeatureDocsImportConversion, Primary: openRouterLunaRoute,
			Fallbacks: []AICompletionRoute{openRouterGeminiFlashRoute}, MaximumOutputTokens: 24_000,
		},
		common(BillingFeatureHelpcenterAnswer, 4096),
		common(BillingFeatureCustomAgentDraft, 1600),
		common(BillingFeatureCompanyProductContext, 2400),
		common(BillingFeatureDockChatTitle, 80),
		{
			FeatureKey: BillingFeatureAskChat, OperationKey: AIUsageOperationMediaEnrichment,
			Primary: openRouterGeminiFlashRoute, MaximumOutputTokens: 700,
		},
	}
	registry := AICompletionRouteRegistry{policies: make(map[string]AICompletionRoutePolicy, len(policies))}
	for _, policy := range policies {
		registry.policies[aiCompletionPolicyKey(policy.FeatureKey, policy.OperationKey)] = policy
	}
	return registry
}

// Policy returns one feature/operation route policy.
func (r AICompletionRouteRegistry) Policy(featureKey, operationKey string) (AICompletionRoutePolicy, bool) {
	policy, ok := r.policies[aiCompletionPolicyKey(featureKey, operationKey)]
	return policy, ok
}

// Validate returns all catalog and structural route-policy issues.
func (r AICompletionRouteRegistry) Validate(catalog *aiusage.Catalog) []error {
	var issues []error
	for key, policy := range r.policies {
		if strings.TrimSpace(policy.FeatureKey) == "" {
			issues = append(issues, fmt.Errorf("%s: feature key is required", key))
		}
		routes := append([]AICompletionRoute{policy.Primary}, policy.Fallbacks...)
		seen := make(map[string]struct{}, len(routes))
		for index, route := range routes {
			routeKey := aiCompletionRouteKey(route)
			if _, ok := seen[routeKey]; ok {
				issues = append(issues, fmt.Errorf("%s: duplicate route %s", key, routeKey))
				continue
			}
			seen[routeKey] = struct{}{}
			resolved, err := catalog.Resolve(route.Provider, route.Model, route.Model, route.ServiceTier)
			if err != nil {
				issues = append(issues, fmt.Errorf("%s route %d: %w", key, index, err))
				continue
			}
			if int64(policy.MaximumOutputTokens) > resolved.MaximumOutput {
				issues = append(issues, fmt.Errorf(
					"%s route %d: output ceiling %d exceeds catalog maximum %d",
					key, index, policy.MaximumOutputTokens, resolved.MaximumOutput,
				))
			}
		}
	}
	return issues
}

func aiCompletionPolicyKey(featureKey, operationKey string) string {
	return strings.ToLower(strings.TrimSpace(featureKey)) + ":" + strings.ToLower(strings.TrimSpace(operationKey))
}

func aiCompletionRouteKey(route AICompletionRoute) string {
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(route.Provider)),
		strings.TrimSpace(route.Model),
		strings.ToLower(strings.TrimSpace(route.ServiceTier)),
	}, ":")
}
