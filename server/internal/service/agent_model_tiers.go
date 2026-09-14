package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/model"
)

var selectableAgentTierRoutes = map[aimodel.Tier]AICompletionRoute{
	aimodel.TierSmall: {
		Provider: "openrouter", Model: defaultFastOpenRouterAgentModel, ServiceTier: defaultAICompletionServiceTier,
	},
	aimodel.TierMedium: {
		Provider: "openrouter", Model: "google/gemini-3.7-flash", ServiceTier: defaultAICompletionServiceTier,
	},
	aimodel.TierLarge: {
		Provider: "openrouter", Model: "openai/gpt-5.6-terra", ServiceTier: defaultAICompletionServiceTier,
	},
	aimodel.TierFlagship: {
		Provider: "openrouter", Model: "anthropic/claude-sonnet-5", ServiceTier: defaultAICompletionServiceTier,
	},
}

// AgentModelTierSnapshot is the internal execution identity resolved from a public model size.
type AgentModelTierSnapshot struct {
	ModelTier             string
	Provider              string
	Model                 string
	ServiceTier           string
	RuntimeKind           string
	ProviderQuantizations []string
}

// AgentModelTierResolver owns the public-tier to internal-route policy.
type AgentModelTierResolver struct {
	catalog     *aimodel.Catalog
	hasProvider func(string) bool
}

func NewAgentModelTierResolver(catalog *aimodel.Catalog, hasProvider func(string) bool) *AgentModelTierResolver {
	return &AgentModelTierResolver{catalog: catalog, hasProvider: hasProvider}
}

// ValidateSelectable verifies every public tier before the process starts serving traffic.
func (r *AgentModelTierResolver) ValidateSelectable() []error {
	ordered := []aimodel.Tier{aimodel.TierSmall, aimodel.TierMedium, aimodel.TierLarge, aimodel.TierFlagship}
	var issues []error
	for _, tier := range ordered {
		route := selectableAgentTierRoutes[tier]
		if r == nil || r.catalog == nil {
			issues = append(issues, fmt.Errorf("agent model tier %q has no model catalog", tier))
			continue
		}
		if r.hasProvider != nil && !r.hasProvider(route.Provider) {
			issues = append(issues, fmt.Errorf("agent model tier %q provider %q is not configured", tier, route.Provider))
			continue
		}
		resolved, err := r.catalog.Resolve(route.Provider, route.Model, route.Model, route.ServiceTier)
		if err != nil {
			issues = append(issues, fmt.Errorf("agent model tier %q: %w", tier, err))
			continue
		}
		if resolved.Tier != tier {
			issues = append(issues, fmt.Errorf("agent model tier %q resolves as %q", tier, resolved.Tier))
		}
	}
	return issues
}

// ResolveCustom validates a selectable tier and returns its immutable execution snapshot.
func (r *AgentModelTierResolver) ResolveCustom(tier aimodel.Tier) (AgentModelTierSnapshot, error) {
	route, ok := selectableAgentTierRoutes[tier]
	if !ok || r == nil || r.catalog == nil {
		return AgentModelTierSnapshot{}, fmt.Errorf("model size temporarily unavailable")
	}
	if r.hasProvider != nil && !r.hasProvider(route.Provider) {
		return AgentModelTierSnapshot{}, fmt.Errorf("model size temporarily unavailable")
	}
	resolved, err := r.catalog.Resolve(route.Provider, route.Model, route.Model, route.ServiceTier)
	if err != nil || resolved.Tier != tier {
		return AgentModelTierSnapshot{}, fmt.Errorf("model size temporarily unavailable")
	}
	return AgentModelTierSnapshot{
		ModelTier: string(tier), Provider: route.Provider, Model: route.Model,
		ServiceTier: route.ServiceTier, RuntimeKind: "native_sdk",
		ProviderQuantizations: providerQuantizationsForAgentRoute(route),
	}, nil
}

func providerQuantizationsForAgentRoute(route AICompletionRoute) []string {
	if normalizeModelProvider(route.Provider) != model.AgentModelProviderOpenRouter ||
		strings.TrimSpace(route.Model) != defaultFastOpenRouterAgentModel {
		return nil
	}
	return append([]string(nil), defaultFastOpenRouterQuantizations...)
}

// Derive returns the public tier for an existing exact execution route.
func (r *AgentModelTierResolver) Derive(provider, model, serviceTier string) (aimodel.Tier, error) {
	if r == nil || r.catalog == nil {
		return "", fmt.Errorf("model size temporarily unavailable")
	}
	resolved, err := r.catalog.Resolve(provider, model, model, serviceTier)
	if err != nil {
		resolved, err = r.catalog.ResolveDefault(provider, model, serviceTier)
	}
	if err != nil {
		return "", err
	}
	return resolved.Tier, nil
}

var (
	defaultAgentTierResolverOnce sync.Once
	defaultAgentTierResolver     *AgentModelTierResolver
)

func loadDefaultAgentModelTierResolver() *AgentModelTierResolver {
	defaultAgentTierResolverOnce.Do(func() {
		catalog, err := aimodel.LoadCatalog()
		if err == nil {
			defaultAgentTierResolver = NewAgentModelTierResolver(catalog, nil)
		}
	})
	return defaultAgentTierResolver
}

func deriveAgentModelTier(provider, modelName *string, executionConfig model.JSONBlob) string {
	resolver := loadDefaultAgentModelTierResolver()
	if resolver == nil || provider == nil || modelName == nil {
		return ""
	}
	serviceTier := defaultAICompletionServiceTier
	var config model.AgentExecutionConfig
	if len(executionConfig) > 0 && json.Unmarshal(executionConfig, &config) == nil && config.ServiceTier != nil {
		if value := strings.TrimSpace(*config.ServiceTier); value != "" {
			serviceTier = value
		}
	}
	tier, err := resolver.Derive(*provider, *modelName, serviceTier)
	if err != nil {
		return ""
	}
	return string(tier)
}
