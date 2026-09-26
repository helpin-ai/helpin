package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aimodel"
)

func TestAICompletionRouteRegistryCoversDirectFeatures(t *testing.T) {
	registry := DefaultAICompletionRouteRegistry()
	want := []string{
		BillingFeatureAIRouting,
		BillingFeatureCoverageGapAnalysis,
		BillingFeatureCRMSignalDetection,
		BillingFeatureSupportReplyRewrite,
		BillingFeatureDealAutomationInference,
		BillingFeatureMeetingIntelligence,
		BillingFeatureCRMSummary,
		BillingFeatureTaskStandingBrief,
		BillingFeatureSupportAIReply,
		BillingFeatureSupportTaskDraft,
		BillingFeatureDocsArticleTranslation,
		BillingFeatureDocsArticleGeneration,
		BillingFeatureDocsImportConversion,
		BillingFeatureHelpcenterAnswer,
		BillingFeatureCustomAgentDraft,
		BillingFeatureCompanyProductContext,
		BillingFeatureDockChatTitle,
	}
	for _, featureKey := range want {
		if _, ok := registry.Policy(featureKey, ""); !ok {
			t.Errorf("route policy missing for %q", featureKey)
		}
	}
	if _, ok := registry.Policy(BillingFeatureAskChat, AIUsageOperationMediaEnrichment); !ok {
		t.Errorf("media enrichment route policy missing")
	}
}

func TestAICompletionRouteRegistryResolvesEveryFixedRouteInPricingCatalog(t *testing.T) {
	catalog, err := aimodel.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	issues := DefaultAICompletionRouteRegistry().Validate(catalog)
	if len(issues) != 0 {
		t.Fatalf("route registry validation issues = %v", issues)
	}
}

func TestAICompletionRouteRegistryUsesLongOutputRoutesForDocsImport(t *testing.T) {
	policy, ok := DefaultAICompletionRouteRegistry().Policy(BillingFeatureDocsImportConversion, "")
	if !ok {
		t.Fatal("docs import route policy missing")
	}
	if policy.Primary.Model != "openai/gpt-5.6-luna" {
		t.Fatalf("docs import primary = %q, want OpenRouter Luna", policy.Primary.Model)
	}
	if policy.MaximumOutputTokens < 24_000 {
		t.Fatalf("docs import output ceiling = %d, want at least 24000", policy.MaximumOutputTokens)
	}
}

func TestGreetingUsesGPT6LunaWithProviderFailover(t *testing.T) {
	policy, ok := DefaultAICompletionRouteRegistry().Policy(BillingFeatureSupportAIReply, supportGreetingOperation)
	if !ok {
		t.Fatal("greeting route missing")
	}
	if policy.Primary.Provider != "openrouter" || policy.Primary.Model != "openai/gpt-6-luna" || policy.Primary.OpenRouterProvider != "" || policy.Primary.ProviderOptions != "" {
		t.Fatalf("greeting route = %+v", policy.Primary)
	}
}

func TestAICompletionRouteRegistryKeepsMediaOnApprovedVisionRoute(t *testing.T) {
	policy, ok := DefaultAICompletionRouteRegistry().Policy(BillingFeatureAskChat, AIUsageOperationMediaEnrichment)
	if !ok {
		t.Fatal("media route policy missing")
	}
	if policy.Primary.Provider != "openrouter" || policy.Primary.Model != "google/gemini-3.8-flash" {
		t.Fatalf("media route = %#v", policy.Primary)
	}
	if len(policy.Fallbacks) != 1 || policy.Fallbacks[0].Model != "qwen/qwen3.8-omni-flash" {
		t.Fatalf("media fallbacks = %#v, want Qwen Omni", policy.Fallbacks)
	}
}

func TestAICompletionRouteRegistryReportsMissingConfiguredProviders(t *testing.T) {
	issues := DefaultAICompletionRouteRegistry().ValidateProviders(func(provider string) bool {
		return provider == "anthropic"
	})
	if len(issues) != 1 || issues[0].Error() != `AI completion provider "openrouter" is not configured` {
		t.Fatalf("provider validation issues = %v", issues)
	}
}

func TestAICompletionRouteRegistryAppliesCRMOverridesOnlyToCRMFeatures(t *testing.T) {
	registry := NewAICompletionRouteRegistry(CRMCompletionRouteConfig{
		Primary: AICompletionRoute{
			Provider: "openrouter", Model: "anthropic/claude-sonnet-5", OpenRouterProvider: "anthropic",
		},
		Fallback: AICompletionRoute{
			Provider: "openrouter", Model: "openai/gpt-5.6-luna", OpenRouterProvider: "openai",
		},
		MeetingFallback: AICompletionRoute{
			Provider: "openrouter", Model: "google/gemini-3.7-flash", OpenRouterProvider: "google-vertex/global",
		},
	})

	for _, feature := range []string{
		BillingFeatureCRMSignalDetection,
		BillingFeatureDealAutomationInference,
		BillingFeatureCRMSummary,
		BillingFeatureMeetingIntelligence,
	} {
		policy, ok := registry.Policy(feature, "")
		if !ok {
			t.Fatalf("CRM route %q missing", feature)
		}
		if policy.Primary.Model != "anthropic/claude-sonnet-5" || policy.Primary.OpenRouterProvider != "anthropic" {
			t.Fatalf("CRM primary for %q = %#v", feature, policy.Primary)
		}
	}
	meeting, _ := registry.Policy(BillingFeatureMeetingIntelligence, "")
	if len(meeting.Fallbacks) != 1 || meeting.Fallbacks[0].OpenRouterProvider != "google-vertex/global" {
		t.Fatalf("meeting fallback = %#v", meeting.Fallbacks)
	}
	summary, _ := registry.Policy(BillingFeatureCRMSummary, "")
	if len(summary.Fallbacks) != 1 || summary.Fallbacks[0].OpenRouterProvider != "openai" {
		t.Fatalf("CRM fallback = %#v", summary.Fallbacks)
	}
	nonCRM, _ := registry.Policy(BillingFeatureAIRouting, "")
	if nonCRM.Primary.Model != "z-ai/glm-5.3-flash:exacto" || nonCRM.Primary.OpenRouterProvider != "" {
		t.Fatalf("non-CRM route changed = %#v", nonCRM.Primary)
	}

	catalog, err := aimodel.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if issues := registry.Validate(catalog); len(issues) != 0 {
		t.Fatalf("configured route validation issues = %v", issues)
	}
}

func TestAICompletionRouteRegistryRejectsOpenRouterSelectionOnDirectRoute(t *testing.T) {
	registry := NewAICompletionRouteRegistry(CRMCompletionRouteConfig{
		Primary: AICompletionRoute{
			Provider: "anthropic", Model: "claude-sonnet-5", OpenRouterProvider: "anthropic",
		},
	})
	catalog, err := aimodel.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	issues := registry.Validate(catalog)
	if len(issues) == 0 || !strings.Contains(issues[0].Error(), "OpenRouter provider selection requires provider openrouter") {
		t.Fatalf("route validation issues = %v", issues)
	}
}

func TestSupportRewriteRoutesUseOnlySmallTierModels(t *testing.T) {
	policy, ok := DefaultAICompletionRouteRegistry().Policy(BillingFeatureSupportReplyRewrite, "")
	if !ok {
		t.Fatal("support rewrite route policy missing")
	}
	if policy.Primary.Provider != "openrouter" || policy.Primary.Model != "deepseek/deepseek-v4-flash-0731" {
		t.Errorf("primary = %+v, want DeepSeek V4 Flash through OpenRouter", policy.Primary)
	}
	catalog, err := aimodel.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range append([]AICompletionRoute{policy.Primary}, policy.Fallbacks...) {
		resolved, err := catalog.Resolve(route.Provider, route.Model, route.Model, route.ServiceTier)
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Tier != "small" {
			t.Errorf("route %s tier = %s, want small", route.Model, resolved.Tier)
		}
	}
}

func TestLiveTranslatePinsPrimaryAndIndependentFallback(t *testing.T) {
	p, ok := DefaultAICompletionRouteRegistry().Policy(BillingFeatureSupportTranslation, "")
	if !ok || p.Primary.Model != "openai/gpt-oss-120b" || p.Primary.OpenRouterProvider != "cerebras/fp16" || p.PreferRequestRoute {
		t.Fatalf("unsafe translation primary: %+v", p)
	}
	if len(p.Fallbacks) != 1 || p.Fallbacks[0].Model != "deepseek/deepseek-v4.1-flash" || p.Fallbacks[0].OpenRouterProvider != "coreweave/fp8" {
		t.Fatalf("fallback: %+v", p.Fallbacks)
	}
}
