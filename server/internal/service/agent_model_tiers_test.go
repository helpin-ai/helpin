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

func TestStandardRouteForTierFollowsConnectedProviders(t *testing.T) {
	type route struct{ provider, model string }
	fixed := map[string]route{
		"small":    {"openrouter", "deepseek/deepseek-v4.1-flash:nitro"},
		"medium":   {"openrouter", "google/gemini-3.8-flash"},
		"large":    {"openai", "gpt-5.6-terra"},
		"flagship": {"anthropic", "claude-sonnet-5"},
	}
	tests := []struct {
		name      string
		connected []string
		want      map[string]route
	}{
		{name: "no provider keeps fixed routes", want: fixed},
		{name: "all providers keep fixed routes", connected: []string{"openrouter", "openai", "anthropic"}, want: fixed},
		{name: "only openrouter", connected: []string{"openrouter"}, want: map[string]route{
			"small":    fixed["small"],
			"medium":   fixed["medium"],
			"large":    {"openrouter", "openai/gpt-5.6-terra"},
			"flagship": {"openrouter", "anthropic/claude-sonnet-5"},
		}},
		{name: "only openai", connected: []string{"openai"}, want: map[string]route{
			"small":    {"openai", "gpt-5.6-luna"},
			"medium":   {"openai", "gpt-5-mini"},
			"large":    fixed["large"],
			"flagship": {"openai", "gpt-5.5"},
		}},
		{name: "only anthropic uses the nearest available size", connected: []string{"anthropic"}, want: map[string]route{
			"small":    {"anthropic", "claude-haiku-4-5"},
			"medium":   {"anthropic", "claude-haiku-4-5"},
			"large":    {"anthropic", "claude-haiku-4-5"},
			"flagship": fixed["flagship"],
		}},
		{name: "openai and anthropic", connected: []string{"openai", "anthropic"}, want: map[string]route{
			"small":    {"openai", "gpt-5.6-luna"},
			"medium":   {"openai", "gpt-5-mini"},
			"large":    fixed["large"],
			"flagship": fixed["flagship"],
		}},
		{name: "openrouter and anthropic prefer openrouter for large", connected: []string{"openrouter", "anthropic"}, want: map[string]route{
			"small":    fixed["small"],
			"medium":   fixed["medium"],
			"large":    {"openrouter", "openai/gpt-5.6-terra"},
			"flagship": fixed["flagship"],
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connected := map[string]bool{}
			for _, provider := range tt.connected {
				connected[provider] = true
			}
			for tier, want := range tt.want {
				got, err := standardRouteForTier(tier, connected)
				if err != nil {
					t.Fatalf("standardRouteForTier(%q): %v", tier, err)
				}
				if got.Provider != want.provider || got.Model != want.model {
					t.Errorf("%s = %s/%s, want %s/%s", tier, got.Provider, got.Model, want.provider, want.model)
				}
				if got.Controls == nil {
					t.Errorf("%s has nil controls", tier)
				}
			}
		})
	}
}

func TestStandardRouteForTierRejectsUnknownTier(t *testing.T) {
	if _, err := standardRouteForTier("huge", map[string]bool{"openai": true}); err == nil {
		t.Fatal("unknown tier accepted")
	}
}
