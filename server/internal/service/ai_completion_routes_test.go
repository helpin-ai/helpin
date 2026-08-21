package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
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
	catalog, err := aiusage.LoadCatalog()
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

func TestAICompletionRouteRegistryKeepsMediaOnApprovedVisionRoute(t *testing.T) {
	policy, ok := DefaultAICompletionRouteRegistry().Policy(BillingFeatureAskChat, AIUsageOperationMediaEnrichment)
	if !ok {
		t.Fatal("media route policy missing")
	}
	if policy.Primary.Provider != "openrouter" || policy.Primary.Model != "google/gemini-3.7-flash" {
		t.Fatalf("media route = %#v", policy.Primary)
	}
	if len(policy.Fallbacks) != 0 {
		t.Fatalf("media fallbacks = %#v, want none", policy.Fallbacks)
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
