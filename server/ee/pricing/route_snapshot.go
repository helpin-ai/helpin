//go:build ee

package pricing

import "github.com/helpin-ai/helpin/server/internal/aimodel"

// routeSnapshot preserves the historical persisted shape while price records
// and model capabilities have independent sources of truth.
type routeSnapshot struct {
	Provider          string     `json:"provider"`
	CanonicalModel    string     `json:"canonical_model"`
	Route             string     `json:"route"`
	ServiceTier       string     `json:"service_tier"`
	Tier              Tier       `json:"tier"`
	MaximumCostRates  TokenRates `json:"maximum_cost_rates"`
	CacheReadSupport  bool       `json:"cache_read_support"`
	CacheWriteSupport bool       `json:"cache_write_support"`
	ContextWindow     int64      `json:"context_window"`
	MaximumOutput     int64      `json:"maximum_output"`
	Enabled           bool       `json:"enabled"`
}

func snapshotRoute(price RouteDefinition, metadata aimodel.RouteDefinition) routeSnapshot {
	return routeSnapshot{Provider: price.Provider, CanonicalModel: price.CanonicalModel, Route: price.Route,
		ServiceTier: price.ServiceTier, Tier: price.Tier, MaximumCostRates: price.MaximumCostRates,
		CacheReadSupport: metadata.CacheReadSupport, CacheWriteSupport: metadata.CacheWriteSupport,
		ContextWindow: metadata.ContextWindow, MaximumOutput: metadata.MaximumOutput, Enabled: price.Enabled}
}
