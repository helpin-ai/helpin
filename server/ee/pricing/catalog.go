package pricing

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/aimodel"
)

var (
	// ErrModelUnavailable indicates that a model or exact route is not eligible.
	ErrModelUnavailable = errors.New("model unavailable under current pricing")
	// ErrPricingConfigurationMissing indicates that catalog data is incomplete.
	ErrPricingConfigurationMissing = errors.New("pricing configuration missing")
)

//go:embed catalog.json
var catalogJSON []byte

// LoadCatalog loads the embedded immutable pricing catalog.
func LoadCatalog() (*Catalog, error) {
	var catalog Catalog
	if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
		return nil, fmt.Errorf("decode AI pricing catalog: %w", err)
	}
	models, err := aimodel.LoadCatalog()
	if err != nil {
		return nil, err
	}
	catalog.ModelCatalog = models
	catalog.Models = models.Models
	if issues := catalog.Validate(); len(issues) != 0 {
		return nil, fmt.Errorf("%w: %s at %s", ErrPricingConfigurationMissing,
			issues[0].Message, issues[0].Path)
	}
	return &catalog, nil
}

// Resolve returns the exact enabled route and immutable customer rates.
func (c *Catalog) Resolve(provider, model, route, serviceTier string) (ResolvedRoute, error) {
	if c == nil {
		return ResolvedRoute{}, ErrPricingConfigurationMissing
	}
	provider = canonicalKey(provider)
	model = canonicalKey(model)
	route = canonicalKey(route)
	serviceTier = canonicalKey(serviceTier)
	if serviceTier == "" {
		serviceTier = "standard"
	}

	canonicalModel := ""
	for _, definition := range c.Models {
		if canonicalKey(definition.Provider) != provider {
			continue
		}
		if canonicalKey(definition.CanonicalModel) == model || containsKey(definition.Aliases, model) {
			canonicalModel = canonicalKey(definition.CanonicalModel)
			if !definition.Enabled {
				return ResolvedRoute{}, ErrModelUnavailable
			}
			break
		}
	}
	if canonicalModel == "" {
		return ResolvedRoute{}, ErrModelUnavailable
	}

	for _, candidate := range c.Routes {
		if !candidate.Enabled || canonicalKey(candidate.Provider) != provider ||
			canonicalKey(candidate.CanonicalModel) != canonicalModel ||
			canonicalKey(candidate.Route) != route ||
			canonicalKey(candidate.ServiceTier) != serviceTier {
			continue
		}
		tier, ok := c.tier(candidate.Tier)
		if !ok {
			return ResolvedRoute{}, ErrPricingConfigurationMissing
		}
		metadata, err := c.ModelCatalog.Resolve(provider, canonicalModel, candidate.Route, serviceTier)
		if err != nil {
			return ResolvedRoute{}, ErrModelUnavailable
		}
		snapshot, err := json.Marshal(struct {
			PricingVersion string        `json:"pricing_version"`
			Route          routeSnapshot `json:"route"`
			Rates          TokenRates    `json:"rates"`
		}{PricingVersion: c.PricingVersion, Route: snapshotRoute(candidate, metadata), Rates: tier.CustomerRates})
		if err != nil {
			return ResolvedRoute{}, fmt.Errorf("snapshot AI pricing route: %w", err)
		}
		return ResolvedRoute{
			Provider: provider, CanonicalModel: canonicalModel, Route: candidate.Route,
			ServiceTier: candidate.ServiceTier, Tier: candidate.Tier,
			Rates: tier.CustomerRates, RateSnapshot: snapshot,
			ContextWindow: metadata.ContextWindow, MaximumOutput: metadata.MaximumOutput,
		}, nil
	}
	return ResolvedRoute{}, ErrModelUnavailable
}

// ResolveDefault resolves an exact route when the stored model is already a
// route ID, then falls back to the single approved route for older canonical
// model records. Ambiguous canonical-only records fail closed.
func (c *Catalog) ResolveDefault(provider, model, serviceTier string) (ResolvedRoute, error) {
	provider = canonicalKey(provider)
	model = canonicalKey(model)
	serviceTier = canonicalKey(serviceTier)
	if serviceTier == "" {
		serviceTier = "standard"
	}
	if exact, err := c.Resolve(provider, model, model, serviceTier); err == nil {
		return exact, nil
	}
	var match *ResolvedRoute
	for _, route := range c.Routes {
		if !route.Enabled || canonicalKey(route.Provider) != provider || canonicalKey(route.ServiceTier) != serviceTier {
			continue
		}
		resolved, err := c.Resolve(provider, model, route.Route, serviceTier)
		if err != nil {
			continue
		}
		if match != nil && match.Route != resolved.Route {
			return ResolvedRoute{}, ErrModelUnavailable
		}
		copy := resolved
		match = &copy
	}
	if match == nil {
		return ResolvedRoute{}, ErrModelUnavailable
	}
	return *match, nil
}

// Validate returns every catalog integrity issue in stable traversal order.
func (c *Catalog) Validate() []ValidationIssue {
	if c == nil {
		return []ValidationIssue{{Code: "missing_catalog", Path: "catalog", Message: "catalog is nil"}}
	}
	var issues []ValidationIssue
	if strings.TrimSpace(c.PricingVersion) == "" {
		issues = append(issues, ValidationIssue{Code: "missing_version", Path: "pricing_version", Message: "pricing version is required"})
	}
	tiers := make(map[Tier]TierDefinition, len(c.Tiers))
	for index, tier := range c.Tiers {
		path := fmt.Sprintf("tiers[%d]", index)
		if _, exists := tiers[tier.Key]; exists {
			issues = append(issues, ValidationIssue{Code: "duplicate_tier", Path: path, Message: "tier key conflicts"})
			continue
		}
		tiers[tier.Key] = tier
		issues = append(issues, validateMarkup(path, tier.CeilingRates, tier.CustomerRates)...)
	}

	aliases := make(map[string]string)
	models := make(map[string]ModelDefinition)
	for index, definition := range c.Models {
		path := fmt.Sprintf("models[%d]", index)
		key := modelKey(definition.Provider, definition.CanonicalModel)
		if _, exists := models[key]; exists {
			issues = append(issues, ValidationIssue{Code: "duplicate_model", Path: path, Message: "canonical model conflicts"})
		}
		models[key] = definition
		if _, ok := tiers[definition.Tier]; !ok {
			issues = append(issues, ValidationIssue{Code: "unknown_tier", Path: path + ".tier", Message: "model tier is not defined"})
		}
		for _, alias := range append([]string{definition.CanonicalModel}, definition.Aliases...) {
			aliasKey := modelKey(definition.Provider, alias)
			if existing, exists := aliases[aliasKey]; exists && existing != key {
				issues = append(issues, ValidationIssue{Code: "alias_conflict", Path: path + ".aliases", Message: "model alias conflicts"})
				continue
			}
			aliases[aliasKey] = key
		}
	}

	routes := make(map[string]struct{}, len(c.Routes))
	for index, route := range c.Routes {
		path := fmt.Sprintf("routes[%d]", index)
		routeKey := strings.Join([]string{
			canonicalKey(route.Provider), canonicalKey(route.CanonicalModel),
			canonicalKey(route.Route), canonicalKey(route.ServiceTier),
		}, ":")
		if _, exists := routes[routeKey]; exists {
			issues = append(issues, ValidationIssue{Code: "duplicate_route", Path: path, Message: "exact route conflicts"})
			continue
		}
		routes[routeKey] = struct{}{}
		model, ok := models[modelKey(route.Provider, route.CanonicalModel)]
		if !ok {
			issues = append(issues, ValidationIssue{Code: "unknown_model", Path: path + ".canonical_model", Message: "route model is not defined"})
			continue
		}
		if model.Tier != route.Tier {
			issues = append(issues, ValidationIssue{Code: "tier_mismatch", Path: path + ".tier", Message: "route and model tiers differ"})
		}
		tier, ok := tiers[route.Tier]
		if !ok {
			continue
		}
		metadata, err := c.ModelCatalog.Resolve(route.Provider, route.CanonicalModel, route.Route, route.ServiceTier)
		if route.Enabled && err != nil {
			issues = append(issues, ValidationIssue{Code: "missing_model_metadata", Path: path, Message: "priced route requires enabled model metadata"})
		}
		issues = append(issues, validateRouteCosts(path, route, metadata, tier.CeilingRates)...)
	}
	for index, tool := range c.Tools {
		if tool.CustomerMicrousd != markup(tool.CeilingMicrousd) {
			issues = append(issues, ValidationIssue{Code: "tool_markup", Path: fmt.Sprintf("tools[%d]", index), Message: "tool customer rate must equal ceiling x 1.10"})
		}
	}
	return issues
}

// PublicSnapshot returns customer-safe catalog data without provider ceilings.
func (c *Catalog) PublicSnapshot() PublicPricing {
	pricing := PublicPricing{PricingVersion: c.PricingVersion, EffectiveDate: c.EffectiveDate}
	for _, tier := range c.Tiers {
		pricing.Tiers = append(pricing.Tiers, PublicTier{
			Key: tier.Key, Label: tier.Label, Description: tier.Description, Rates: tier.CustomerRates,
		})
	}
	for _, plan := range c.Plans {
		pricing.Plans = append(pricing.Plans, PublicPlan(plan))
	}
	for _, definition := range c.Models {
		selectionModel := definition.CanonicalModel
		for _, route := range c.Routes {
			if route.Enabled && canonicalKey(route.Provider) == canonicalKey(definition.Provider) && canonicalKey(route.CanonicalModel) == canonicalKey(definition.CanonicalModel) {
				selectionModel = route.Route
				break
			}
		}
		pricing.Models = append(pricing.Models, PublicModel{
			Provider: definition.Provider, CanonicalModel: definition.CanonicalModel,
			SelectionModel: selectionModel, Label: definition.Label, Tier: definition.Tier, Enabled: definition.Enabled,
		})
	}
	for _, tool := range c.Tools {
		if tool.SafeToExpose {
			pricing.Tools = append(pricing.Tools, PublicTool{
				Key: tool.Key, Label: tool.Label, MicrousdPerCall: tool.CustomerMicrousd,
			})
		}
	}
	return pricing
}

func (c *Catalog) tier(key Tier) (TierDefinition, bool) {
	for _, tier := range c.Tiers {
		if tier.Key == key {
			return tier, true
		}
	}
	return TierDefinition{}, false
}

func validateMarkup(path string, ceiling, customer TokenRates) []ValidationIssue {
	checks := []struct {
		name string
		cost int64
		rate int64
	}{
		{name: "input", cost: ceiling.InputMicrousdPerMillion, rate: customer.InputMicrousdPerMillion},
		{name: "cache_read", cost: ceiling.CacheReadMicrousdPerMillion, rate: customer.CacheReadMicrousdPerMillion},
		{name: "cache_write", cost: ceiling.CacheWriteMicrousdPerMillion, rate: customer.CacheWriteMicrousdPerMillion},
		{name: "output", cost: ceiling.OutputMicrousdPerMillion, rate: customer.OutputMicrousdPerMillion},
	}
	var issues []ValidationIssue
	for _, check := range checks {
		if check.cost <= 0 || check.rate != markup(check.cost) {
			issues = append(issues, ValidationIssue{Code: "tier_markup", Path: path + "." + check.name, Message: "customer rate must equal ceiling x 1.10"})
		}
	}
	return issues
}

func validateRouteCosts(path string, route RouteDefinition, metadata aimodel.RouteDefinition, ceiling TokenRates) []ValidationIssue {
	var issues []ValidationIssue
	if route.MaximumCostRates.InputMicrousdPerMillion <= 0 || route.MaximumCostRates.OutputMicrousdPerMillion <= 0 {
		issues = append(issues, ValidationIssue{Code: "missing_route_rate", Path: path, Message: "enabled route requires input and output costs"})
	}
	if metadata.CacheReadSupport && route.MaximumCostRates.CacheReadMicrousdPerMillion <= 0 {
		issues = append(issues, ValidationIssue{Code: "missing_cache_read_rate", Path: path, Message: "cache-read route requires a cost"})
	}
	if metadata.CacheWriteSupport && route.MaximumCostRates.CacheWriteMicrousdPerMillion <= 0 {
		issues = append(issues, ValidationIssue{Code: "missing_cache_write_rate", Path: path, Message: "cache-write route requires a cost"})
	}
	checks := []struct{ actual, maximum int64 }{
		{route.MaximumCostRates.InputMicrousdPerMillion, ceiling.InputMicrousdPerMillion},
		{route.MaximumCostRates.CacheReadMicrousdPerMillion, ceiling.CacheReadMicrousdPerMillion},
		{route.MaximumCostRates.CacheWriteMicrousdPerMillion, ceiling.CacheWriteMicrousdPerMillion},
		{route.MaximumCostRates.OutputMicrousdPerMillion, ceiling.OutputMicrousdPerMillion},
	}
	for _, check := range checks {
		if check.actual > check.maximum {
			issues = append(issues, ValidationIssue{Code: "tier_ceiling", Path: path, Message: "route exceeds its tier ceiling"})
			break
		}
	}
	return issues
}

func markup(value int64) int64 { return value * 110 / 100 }

func canonicalKey(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func modelKey(provider, model string) string {
	return canonicalKey(provider) + ":" + canonicalKey(model)
}

func containsKey(values []string, target string) bool {
	for _, value := range values {
		if canonicalKey(value) == target {
			return true
		}
	}
	return false
}
