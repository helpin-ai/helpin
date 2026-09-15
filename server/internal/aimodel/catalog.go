package aimodel

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrModelUnavailable indicates that a model or exact route is not enabled.
var ErrModelUnavailable = errors.New("model unavailable")

//go:embed catalog.json
var catalogJSON []byte

// LoadCatalog loads the built-in metadata independently of commercial prices.
func LoadCatalog() (*Catalog, error) {
	var catalog Catalog
	if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
		return nil, fmt.Errorf("decode model catalog: %w", err)
	}
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	return &catalog, nil
}

// Validate checks identity and limits, including catalogs with unpriced models.
func (c *Catalog) Validate() error {
	if c == nil {
		return errors.New("model catalog is missing")
	}
	models := make(map[string]ModelDefinition)
	aliases := make(map[string]string)
	for _, model := range c.Models {
		identity := key(model.Provider, model.CanonicalModel)
		_, exists := models[identity]
		if normalize(model.Provider) == "" || normalize(model.CanonicalModel) == "" || exists {
			return fmt.Errorf("invalid or duplicate model %q", identity)
		}
		models[identity] = model
		for _, alias := range append([]string{model.CanonicalModel}, model.Aliases...) {
			aliasKey := key(model.Provider, alias)
			if normalize(alias) == "" || (aliases[aliasKey] != "" && aliases[aliasKey] != identity) {
				return fmt.Errorf("conflicting model alias %q", aliasKey)
			}
			aliases[aliasKey] = identity
		}
	}
	routes := make(map[string]bool)
	for _, route := range c.Routes {
		identity := key(route.Provider, route.CanonicalModel, route.Route, route.ServiceTier)
		model, exists := models[key(route.Provider, route.CanonicalModel)]
		if !exists || normalize(route.Route) == "" || normalize(route.ServiceTier) == "" || routes[identity] {
			return fmt.Errorf("invalid or duplicate model route %q", identity)
		}
		if route.Tier != model.Tier {
			return fmt.Errorf("model route tier differs from model for %q", identity)
		}
		if route.ContextWindow < 0 || route.MaximumOutput < 0 || (route.ContextWindow > 0 && route.MaximumOutput > route.ContextWindow) {
			return fmt.Errorf("invalid model limits for %q", identity)
		}
		routes[identity] = true
	}
	return nil
}

// Resolve returns enabled execution metadata for an exact route.
func (c *Catalog) Resolve(provider, model, route, serviceTier string) (RouteDefinition, error) {
	if c == nil {
		return RouteDefinition{}, ErrModelUnavailable
	}
	provider, model, route = normalize(provider), normalize(model), normalize(route)
	serviceTier = normalize(serviceTier)
	if serviceTier == "" {
		serviceTier = "standard"
	}
	canonical := ""
	for _, definition := range c.Models {
		if normalize(definition.Provider) != provider {
			continue
		}
		for _, alias := range append([]string{definition.CanonicalModel}, definition.Aliases...) {
			if normalize(alias) == model {
				if !definition.Enabled {
					return RouteDefinition{}, ErrModelUnavailable
				}
				canonical = normalize(definition.CanonicalModel)
				break
			}
		}
		if canonical != "" {
			break
		}
	}
	for _, candidate := range c.Routes {
		if canonical != "" && candidate.Enabled && normalize(candidate.Provider) == provider && normalize(candidate.CanonicalModel) == canonical && normalize(candidate.Route) == route && normalize(candidate.ServiceTier) == serviceTier {
			return candidate, nil
		}
	}
	return RouteDefinition{}, ErrModelUnavailable
}

// ResolveDefault prefers an exact route; ambiguous canonical-only models fail.
func (c *Catalog) ResolveDefault(provider, model, serviceTier string) (RouteDefinition, error) {
	if exact, err := c.Resolve(provider, model, model, serviceTier); err == nil {
		return exact, nil
	}
	if c == nil {
		return RouteDefinition{}, ErrModelUnavailable
	}
	var match *RouteDefinition
	for _, candidate := range c.Routes {
		resolved, err := c.Resolve(provider, model, candidate.Route, serviceTier)
		if err != nil {
			continue
		}
		if match != nil && match.Route != resolved.Route {
			return RouteDefinition{}, ErrModelUnavailable
		}
		match = &resolved
	}
	if match == nil {
		return RouteDefinition{}, ErrModelUnavailable
	}
	return *match, nil
}

// PublicModels returns selection metadata without any pricing dependency.
func (c *Catalog) PublicModels() []PublicModel {
	var models []PublicModel
	if c == nil {
		return models
	}
	for _, definition := range c.Models {
		selection := definition.CanonicalModel
		for _, route := range c.Routes {
			if route.Enabled && key(route.Provider, route.CanonicalModel) == key(definition.Provider, definition.CanonicalModel) {
				selection = route.Route
				break
			}
		}
		models = append(models, PublicModel{Provider: definition.Provider, CanonicalModel: definition.CanonicalModel, SelectionModel: selection, Label: definition.Label, Tier: definition.Tier, Enabled: definition.Enabled})
	}
	return models
}

func normalize(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func key(parts ...string) string {
	for i := range parts {
		parts[i] = normalize(parts[i])
	}
	return strings.Join(parts, ":")
}
