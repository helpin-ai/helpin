package service

import (
	"strings"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// standardTierOrder lists public sizes from cheapest to most capable.
var standardTierOrder = []aimodel.Tier{aimodel.TierSmall, aimodel.TierMedium, aimodel.TierLarge, aimodel.TierFlagship}

// standardProviderPreference breaks ties after a tier's fixed provider.
var standardProviderPreference = []string{"openrouter", "openai", "anthropic"}

// standardRouteForTier returns the fixed route for tier when its provider is
// connected. Otherwise it selects a deterministic catalog route from a connected
// provider: the exact tier first, then the nearest tier (larger before smaller),
// with providers ordered by standardProviderPreference. Within a provider the
// fixed route's model is preferred, then the first enabled catalog route. When
// no provider is connected, or the catalog has no candidate, the fixed route is
// returned unchanged.
func standardRouteForTier(tier string, connected map[string]bool) (sdk.RunModel, error) {
	fixed, err := standardModelForTier(tier)
	if err != nil {
		return sdk.RunModel{}, err
	}
	if len(connected) == 0 || connected[fixed.Provider] {
		return fixed, nil
	}
	resolver := loadDefaultAgentModelTierResolver()
	if resolver == nil || resolver.catalog == nil {
		return fixed, nil
	}
	catalog := resolver.catalog
	preferredCanonical := ""
	if resolved, err := catalog.Resolve(fixed.Provider, fixed.Model, fixed.Model, defaultAICompletionServiceTier); err == nil {
		preferredCanonical = resolved.CanonicalModel
	}
	providers := orderedConnectedProviders(fixed.Provider, connected)
	for _, candidateTier := range tiersByDistance(aimodel.Tier(strings.TrimSpace(tier))) {
		for _, provider := range providers {
			if route, ok := catalogRouteForTier(catalog, provider, candidateTier, preferredCanonical); ok {
				return standardRunModel(provider, route.Route), nil
			}
		}
	}
	return fixed, nil
}

func orderedConnectedProviders(first string, connected map[string]bool) []string {
	var providers []string
	seen := map[string]bool{}
	for _, provider := range append([]string{first}, standardProviderPreference...) {
		if connected[provider] && !seen[provider] {
			providers = append(providers, provider)
			seen[provider] = true
		}
	}
	return providers
}

// tiersByDistance orders all tiers by distance from tier, preferring the
// larger tier at equal distance so a substitute is never less capable when
// both directions are available.
func tiersByDistance(tier aimodel.Tier) []aimodel.Tier {
	index := -1
	for i, candidate := range standardTierOrder {
		if candidate == tier {
			index = i
		}
	}
	if index < 0 {
		return nil
	}
	out := []aimodel.Tier{tier}
	for distance := 1; distance < len(standardTierOrder); distance++ {
		if up := index + distance; up < len(standardTierOrder) {
			out = append(out, standardTierOrder[up])
		}
		if down := index - distance; down >= 0 {
			out = append(out, standardTierOrder[down])
		}
	}
	return out
}

func catalogRouteForTier(catalog *aimodel.Catalog, provider string, tier aimodel.Tier, preferredCanonical string) (aimodel.RouteDefinition, bool) {
	var first *aimodel.RouteDefinition
	for _, candidate := range catalog.Routes {
		if !strings.EqualFold(candidate.Provider, provider) || candidate.Tier != tier {
			continue
		}
		resolved, err := catalog.Resolve(provider, candidate.CanonicalModel, candidate.Route, defaultAICompletionServiceTier)
		if err != nil {
			continue
		}
		if preferredCanonical != "" && strings.EqualFold(resolved.CanonicalModel, preferredCanonical) {
			return resolved, true
		}
		if first == nil {
			first = &resolved
		}
	}
	if first == nil {
		return aimodel.RouteDefinition{}, false
	}
	return *first, true
}

func standardRunModel(provider, route string) sdk.RunModel {
	controls := &sdk.ModelControls{}
	if q := providerQuantizationsForAgentRoute(AICompletionRoute{Provider: provider, Model: route}); len(q) > 0 {
		controls.OpenRouter = &sdk.OpenRouterModelControls{Provider: &sdk.OpenRouterProviderPreferences{Quantizations: q}}
	}
	return sdk.RunModel{Provider: provider, Model: route, Controls: controls}
}

// smallestCatalogRoute returns the provider's cheapest enabled standard route.
func smallestCatalogRoute(catalog *aimodel.Catalog, provider string) (aimodel.RouteDefinition, bool) {
	if catalog == nil {
		return aimodel.RouteDefinition{}, false
	}
	for _, tier := range standardTierOrder {
		if route, ok := catalogRouteForTier(catalog, provider, tier, ""); ok {
			return route, true
		}
	}
	return aimodel.RouteDefinition{}, false
}

// usableStandardConnection reports whether a shared connection can serve runs.
func usableStandardConnection(c *model.AIConnection) bool {
	return c != nil && c.Status == "connected" && len(c.EncryptedSecret) > 0 && c.SupersededBy == nil &&
		c.Scope == "workspace" && c.UserID == nil
}
