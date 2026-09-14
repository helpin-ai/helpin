package service

import (
	"slices"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestAgentModelTierResolverResolvesSelectableTiers(t *testing.T) {
	catalog, err := aimodel.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewAgentModelTierResolver(catalog, func(provider string) bool {
		return provider == "openrouter" || provider == "openai" || provider == "anthropic"
	})
	want := map[aimodel.Tier]struct{ provider, model string }{
		aimodel.TierSmall:    {"openrouter", "deepseek/deepseek-v4.1-flash:nitro"},
		aimodel.TierMedium:   {"openrouter", "google/gemini-3.8-flash"},
		aimodel.TierLarge:    {"openai", "gpt-5.6-terra"},
		aimodel.TierFlagship: {"anthropic", "claude-sonnet-5"},
	}
	for tier, route := range want {
		snapshot, err := resolver.ResolveCustom(tier)
		if err != nil {
			t.Fatalf("ResolveCustom(%q): %v", tier, err)
		}
		if snapshot.ModelTier != string(tier) || snapshot.Provider != route.provider || snapshot.Model != route.model {
			t.Errorf("ResolveCustom(%q) = %#v", tier, snapshot)
		}
		if tier == aimodel.TierSmall {
			if got := snapshot.ProviderQuantizations; !slices.Equal(got, defaultFastOpenRouterQuantizations) {
				t.Errorf("ResolveCustom(%q) quantizations = %v, want %v", tier, got, defaultFastOpenRouterQuantizations)
			}
		} else if len(snapshot.ProviderQuantizations) != 0 {
			t.Errorf("ResolveCustom(%q) unexpectedly has quantizations %v", tier, snapshot.ProviderQuantizations)
		}
	}
}

func TestAgentModelTierResolverRequiresDirectProviderForLargeAndFlagship(t *testing.T) {
	catalog, err := aimodel.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewAgentModelTierResolver(catalog, func(provider string) bool { return provider == "openrouter" })
	for _, tier := range []aimodel.Tier{aimodel.TierLarge, aimodel.TierFlagship} {
		if _, err := resolver.ResolveCustom(tier); err == nil {
			t.Fatalf("%s silently used OpenRouter without its direct provider", tier)
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
		ModelTier:             string(aimodel.TierSmall),
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
	catalog, err := aimodel.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewAgentModelTierResolver(catalog, func(string) bool { return false })
	_, err = resolver.ResolveCustom(aimodel.TierSmall)
	if err == nil || err.Error() != "model size temporarily unavailable" {
		t.Fatalf("error = %v", err)
	}
}

func TestAgentModelTierResolverStartupValidationRejectsMissingProvider(t *testing.T) {
	catalog, err := aimodel.LoadCatalog()
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
	catalog, err := aimodel.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewAgentModelTierResolver(catalog, nil)
	tier, err := resolver.Derive("openrouter", "openai/gpt-5.6-terra", "standard")
	if err != nil || tier != aimodel.TierLarge {
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
	catalog, err := aimodel.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolver := NewAgentModelTierResolver(catalog, nil)
	for _, previous := range []string{"", "codex", "opencode", "native_sdk"} {
		snapshot, err := resolver.ResolveCustom(aimodel.TierLarge)
		agent := &model.Agent{RuntimeKind: previous, AllowedTargets: []byte(`["repository"]`), AllowedTools: []byte(`["write_file","apply_patch"]`)}
		applyModelTierSnapshotToAgent(agent, snapshot)
		if err != nil || agent.RuntimeKind != "native_sdk" {
			t.Fatalf("previous=%q: runtime=%q err=%v", previous, snapshot.RuntimeKind, err)
		}
	}
}

func TestTierChangePreservesIndependentExecutionSettings(t *testing.T) {
	original := model.JSONBlob(`{"reasoning_effort":"high","max_tool_steps":42,"native_context":{"enabled":true},"service_tier":"priority","openrouter":{"provider":{"quantizations":["int4"]}}}`)
	snapshot := AgentModelTierSnapshot{ModelTier: "large", Provider: "openrouter", Model: "openai/gpt-5.6-terra", ServiceTier: "standard", RuntimeKind: "native_sdk"}
	agent := &model.Agent{ExecutionConfig: original}
	version := &model.AgentVersion{ExecutionConfig: original}
	applyModelTierSnapshotToAgent(agent, snapshot)
	applyModelTierSnapshotToVersion(version, snapshot)
	for _, raw := range []model.JSONBlob{agent.ExecutionConfig, version.ExecutionConfig} {
		cfg, err := model.ParseAgentExecutionConfig(raw)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ReasoningEffort == nil || *cfg.ReasoningEffort != "high" || cfg.MaxToolSteps == nil || *cfg.MaxToolSteps != 42 || cfg.NativeContext == nil || !cfg.NativeContext.Enabled {
			t.Fatalf("tier erased independent controls: %s", raw)
		}
		if cfg.ServiceTier != nil || cfg.OpenRouter != nil {
			t.Fatalf("tier retained old route controls: %s", raw)
		}
	}
}
