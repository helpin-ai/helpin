// Package aimodel describes model identity and execution limits without pricing.
package aimodel

// Tier is a customer-facing model-size classification.
type Tier string

const (
	// TierSmall identifies efficient, high-volume models.
	TierSmall Tier = "small"
	// TierMedium identifies general-purpose models.
	TierMedium Tier = "medium"
	// TierLarge identifies advanced planning, coding, and review models.
	TierLarge Tier = "large"
	// TierFlagship identifies the highest-capability supported models.
	TierFlagship Tier = "flagship"
)

// ModelDefinition is one catalog model shown to customers.
type ModelDefinition struct {
	Provider       string   `json:"provider"`
	CanonicalModel string   `json:"canonical_model"`
	Label          string   `json:"label"`
	Aliases        []string `json:"aliases,omitempty"`
	Tier           Tier     `json:"tier"`
	Enabled        bool     `json:"enabled"`
}

// RouteDefinition is one exact provider route eligible for execution.
type RouteDefinition struct {
	Provider          string `json:"provider"`
	CanonicalModel    string `json:"canonical_model"`
	Route             string `json:"route"`
	ServiceTier       string `json:"service_tier"`
	Tier              Tier   `json:"tier"`
	CacheReadSupport  bool   `json:"cache_read_support"`
	CacheWriteSupport bool   `json:"cache_write_support"`
	ContextWindow     int64  `json:"context_window"`
	MaximumOutput     int64  `json:"maximum_output"`
	Enabled           bool   `json:"enabled"`
}

// PublicModel is the customer-safe representation of one catalog model.
type PublicModel struct {
	Provider       string `json:"provider"`
	CanonicalModel string `json:"canonical_model"`
	SelectionModel string `json:"selection_model"`
	Label          string `json:"label"`
	Tier           Tier   `json:"tier"`
	Enabled        bool   `json:"enabled"`
}

// TierDefinition preserves legacy size labels while agents migrate to profiles.
type TierDefinition struct {
	Key         Tier   `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// Catalog contains nonfinancial model metadata. Custom catalogs need no prices.
type Catalog struct {
	Models []ModelDefinition `json:"models"`
	Routes []RouteDefinition `json:"routes"`
	Tiers  []TierDefinition  `json:"tiers"`
}
