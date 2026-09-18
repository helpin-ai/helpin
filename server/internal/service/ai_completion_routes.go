package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/aimodel"
)

const defaultAICompletionServiceTier = "standard"

var (
	glm53FlashExactoRoute = AICompletionRoute{
		Provider: "openrouter", Model: "z-ai/glm-5.3-flash:exacto",
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
	Provider           string
	Model              string
	ServiceTier        string
	OpenRouterProvider string
}

// CRMCompletionRouteConfig optionally overrides the reviewed CRM defaults.
// Zero-value routes preserve the built-in primary and fallback routes.
type CRMCompletionRouteConfig struct {
	Primary         AICompletionRoute
	Fallback        AICompletionRoute
	MeetingFallback AICompletionRoute
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
	return NewAICompletionRouteRegistry(CRMCompletionRouteConfig{})
}

// NewAICompletionRouteRegistry applies deployment-owned CRM route overrides
// without changing routes for other product features.
func NewAICompletionRouteRegistry(crmConfig CRMCompletionRouteConfig) AICompletionRouteRegistry {
	crmPrimary := completionRouteWithDefault(crmConfig.Primary, glm53FlashExactoRoute)
	crmFallback := completionRouteWithDefault(crmConfig.Fallback, openRouterLunaRoute)
	crmMeetingFallback := completionRouteWithDefault(crmConfig.MeetingFallback, openRouterGeminiFlashRoute)
	common := func(featureKey string, maximumOutput int) AICompletionRoutePolicy {
		return AICompletionRoutePolicy{
			FeatureKey: featureKey, Primary: glm53FlashExactoRoute,
			Fallbacks: []AICompletionRoute{openRouterLunaRoute}, MaximumOutputTokens: maximumOutput,
		}
	}
	crm := func(featureKey string, maximumOutput int) AICompletionRoutePolicy {
		return AICompletionRoutePolicy{
			FeatureKey: featureKey, Primary: crmPrimary,
			Fallbacks: []AICompletionRoute{crmFallback}, MaximumOutputTokens: maximumOutput,
		}
	}
	policies := []AICompletionRoutePolicy{
		{FeatureKey: BillingFeatureSupportTranslation, Primary: AICompletionRoute{Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731", ServiceTier: defaultAICompletionServiceTier}, PreferRequestRoute: true, MaximumOutputTokens: 4000},
		common(BillingFeatureAIRouting, 400),
		common(BillingFeatureCoverageGapAnalysis, 1800),
		crm(BillingFeatureCRMSignalDetection, 4096),
		{
			FeatureKey: BillingFeatureSupportReplyRewrite,
			Primary: AICompletionRoute{
				Provider: "openrouter", Model: "deepseek/deepseek-v4-flash-0731",
				ServiceTier: defaultAICompletionServiceTier,
			},
			Fallbacks: []AICompletionRoute{openRouterLunaRoute}, MaximumOutputTokens: 900,
		},
		common(BillingFeaturePMCommentRewrite, 900),
		crm(BillingFeatureCRMEmailRewrite, 900),
		crm(BillingFeatureDealAutomationInference, 2048),
		{
			FeatureKey: BillingFeatureMeetingIntelligence, Primary: crmPrimary,
			Fallbacks: []AICompletionRoute{crmMeetingFallback}, MaximumOutputTokens: 8192,
		},
		crm(BillingFeatureCRMSummary, 1400),
		common(BillingFeatureTaskStandingBrief, 4096),
		{
			FeatureKey: BillingFeatureSupportAIReply, Primary: glm53FlashExactoRoute,
			Fallbacks: []AICompletionRoute{openRouterLunaRoute}, PreferRequestRoute: true,
			MaximumOutputTokens: 1024,
		},
		{
			FeatureKey: BillingFeatureSupportAIReply, OperationKey: supportGreetingOperation,
			Primary: openRouterLunaRoute, MaximumOutputTokens: 256,
		},
		{
			FeatureKey: BillingFeatureSupportTaskDraft, Primary: glm53FlashExactoRoute,
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
			Primary: AICompletionRoute{
				Provider: mediaEnrichmentProvider, Model: mediaEnrichmentRoute,
				ServiceTier: defaultAICompletionServiceTier,
			}, MaximumOutputTokens: 700,
		},
	}
	registry := AICompletionRouteRegistry{policies: make(map[string]AICompletionRoutePolicy, len(policies))}
	for _, policy := range policies {
		registry.policies[aiCompletionPolicyKey(policy.FeatureKey, policy.OperationKey)] = policy
	}
	return registry
}

func completionRouteWithDefault(route, fallback AICompletionRoute) AICompletionRoute {
	if strings.TrimSpace(route.Provider) == "" {
		route.Provider = fallback.Provider
	}
	if strings.TrimSpace(route.Model) == "" {
		route.Model = fallback.Model
	}
	if strings.TrimSpace(route.ServiceTier) == "" {
		route.ServiceTier = fallback.ServiceTier
	}
	route.Provider = normalizeCompletionRouteProvider(route.Provider)
	route.Model = strings.TrimSpace(route.Model)
	route.OpenRouterProvider = strings.TrimSpace(route.OpenRouterProvider)
	return route
}

func normalizeCompletionRouteProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude", "claude_direct":
		return "anthropic"
	case "openai_direct":
		return "openai"
	case "openrouter-responses":
		return "openrouter"
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

// Policy returns one feature/operation route policy.
func (r AICompletionRouteRegistry) Policy(featureKey, operationKey string) (AICompletionRoutePolicy, bool) {
	policy, ok := r.policies[aiCompletionPolicyKey(featureKey, operationKey)]
	return policy, ok
}

// Validate returns all catalog and structural route-policy issues.
func (r AICompletionRouteRegistry) Validate(catalog *aimodel.Catalog) []error {
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
			if route.OpenRouterProvider != "" && normalizeCompletionRouteProvider(route.Provider) != "openrouter" {
				issues = append(issues, fmt.Errorf("%s route %d: OpenRouter provider selection requires provider openrouter", key, index))
				continue
			}
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

// ValidateProviders reports route providers that are absent from the runtime router.
func (r AICompletionRouteRegistry) ValidateProviders(hasProvider func(string) bool) []error {
	providers := make(map[string]struct{})
	for _, policy := range r.policies {
		routes := append([]AICompletionRoute{policy.Primary}, policy.Fallbacks...)
		for _, route := range routes {
			providers[strings.ToLower(strings.TrimSpace(route.Provider))] = struct{}{}
		}
	}
	names := make([]string, 0, len(providers))
	for provider := range providers {
		names = append(names, provider)
	}
	sort.Strings(names)
	var issues []error
	for _, provider := range names {
		if hasProvider == nil || !hasProvider(provider) {
			issues = append(issues, fmt.Errorf("AI completion provider %q is not configured", provider))
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
		strings.ToLower(strings.TrimSpace(route.OpenRouterProvider)),
	}, ":")
}

// ValidateAvailability lets an edition add readiness requirements without making
// the product route registry depend on prices or subscriptions.
func (r AICompletionRouteRegistry) ValidateAvailability(validate func(AICompletionRoute) error) []error {
	if validate == nil {
		return nil
	}
	var issues []error
	for key, policy := range r.policies {
		for _, route := range append([]AICompletionRoute{policy.Primary}, policy.Fallbacks...) {
			if err := validate(route); err != nil {
				issues = append(issues, fmt.Errorf("%s: %w", key, err))
			}
		}
	}
	return issues
}
