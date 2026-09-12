package service

import (
	"slices"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAgentModelTierResolverResolvesSelectableTiers(t *testing.T) {
	catalog, err := aiusage.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewAgentModelTierResolver(catalog, func(provider string) bool { return provider == "openrouter" })
	want := map[aiusage.Tier]string{
		aiusage.TierSmall:    defaultFastOpenRouterAgentModel,
		aiusage.TierMedium:   "google/gemini-3.7-flash",
		aiusage.TierLarge:    "openai/gpt-5.6-terra",
		aiusage.TierFlagship: "anthropic/claude-sonnet-5",
	}
	for tier, model := range want {
		snapshot, err := resolver.ResolveCustom(tier)
		if err != nil {
			t.Fatalf("ResolveCustom(%q): %v", tier, err)
		}
		if snapshot.ModelTier != string(tier) || snapshot.Provider != "openrouter" || snapshot.Model != model {
			t.Errorf("ResolveCustom(%q) = %#v", tier, snapshot)
		}
		if tier == aiusage.TierSmall {
			if got := snapshot.ProviderQuantizations; !slices.Equal(got, defaultFastOpenRouterQuantizations) {
				t.Errorf("ResolveCustom(%q) quantizations = %v, want %v", tier, got, defaultFastOpenRouterQuantizations)
			}
		} else if len(snapshot.ProviderQuantizations) != 0 {
			t.Errorf("ResolveCustom(%q) unexpectedly has quantizations %v", tier, snapshot.ProviderQuantizations)
		}
	}
}

func TestAgentVersionActivationProjectsModelTier(t *testing.T) {
	agent := &model.Agent{ModelTier: "small"}
	version := &model.AgentVersion{ID: "version-large", ModelTier: "large", RuntimeKind: "codex"}
	applyAgentVersionToAgent(agent, version)
	if agent.ModelTier != "large" || agent.ActiveVersionID == nil || *agent.ActiveVersionID != version.ID {
		t.Fatalf("activated agent = %#v", agent)
	}
}

func TestSmallModelTierSnapshotPersistsOpenRouterQuantizations(t *testing.T) {
	snapshot := AgentModelTierSnapshot{
		ModelTier:             string(aiusage.TierSmall),
		Provider:              model.AgentModelProviderOpenRouter,
		Model:                 defaultFastOpenRouterAgentModel,
		ServiceTier:           defaultAICompletionServiceTier,
		RuntimeKind:           "native_sdk",
		ProviderQuantizations: slices.Clone(defaultFastOpenRouterQuantizations),
	}
	agent := &model.Agent{}
	applyModelTierSnapshotToAgent(agent, snapshot)
	config, err := model.ParseAgentExecutionConfig(agent.ExecutionConfig)
	if err != nil {
		t.Fatalf("ParseAgentExecutionConfig: %v", err)
	}
	if config.OpenRouter == nil || config.OpenRouter.Provider == nil ||
		!slices.Equal(config.OpenRouter.Provider.Quantizations, defaultFastOpenRouterQuantizations) {
		t.Fatalf("execution config = %s, want quantizations %v", agent.ExecutionConfig, defaultFastOpenRouterQuantizations)
	}
}

func TestAgentModelTierResolverRejectsUnavailableTierWithoutLeakingRoute(t *testing.T) {
	catalog, err := aiusage.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewAgentModelTierResolver(catalog, func(string) bool { return false })
	_, err = resolver.ResolveCustom(aiusage.TierSmall)
	if err == nil || err.Error() != "model size temporarily unavailable" {
		t.Fatalf("error = %v", err)
	}
}

func TestAgentModelTierResolverStartupValidationRejectsMissingProvider(t *testing.T) {
	catalog, err := aiusage.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewAgentModelTierResolver(catalog, func(string) bool { return false })
	issues := resolver.ValidateSelectable()
	if len(issues) != 4 {
		t.Fatalf("ValidateSelectable() returned %d issues, want one per public tier: %v", len(issues), issues)
	}
}

func TestAgentModelTierResolverDerivesTierFromStoredRoute(t *testing.T) {
	catalog, err := aiusage.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewAgentModelTierResolver(catalog, nil)
	tier, err := resolver.Derive("openrouter", "openai/gpt-5.6-terra", "standard")
	if err != nil || tier != aiusage.TierLarge {
		t.Fatalf("Derive() = %q, %v", tier, err)
	}
}

func TestBuiltInAgentPresetPricingRoutesHaveReadOnlyTier(t *testing.T) {
	for _, preset := range ListAgentPresets() {
		if preset.ModelTier == "" {
			t.Errorf("preset %q route %s/%s has no model tier", preset.Key, tierTestStringValue(preset.Provider), tierTestStringValue(preset.Model))
		}
	}
}

func tierTestStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func TestCustomCodingModelTiersUseNativeIncludingRetiredPins(t *testing.T) {
	catalog, err := aiusage.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewAgentModelTierResolver(catalog, nil)
	for _, previous := range []string{"", "codex", "opencode", "native_sdk"} {
		snapshot, err := resolver.ResolveCustom(aiusage.TierLarge)
		agent := &model.Agent{RuntimeKind: previous, AllowedTargets: []byte(`["repository"]`), AllowedTools: []byte(`["write_file","apply_patch"]`)}
		applyModelTierSnapshotToAgent(agent, snapshot)
		if err != nil || agent.RuntimeKind != "native_sdk" {
			t.Fatalf("previous=%q: runtime=%q err=%v", previous, snapshot.RuntimeKind, err)
		}
	}
}
