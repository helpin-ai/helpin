package aiusage

import "encoding/json"

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

// FundingMode identifies who pays the inference provider.
type FundingMode string

const (
	// FundingHelpinHosted means Helpin pays the inference provider.
	FundingHelpinHosted FundingMode = "helpin_hosted"
	// FundingCustomer means the customer pays the inference provider.
	FundingCustomer FundingMode = "customer_funded"
)

// TokenRates holds integer micro-USD prices per one million actual tokens.
type TokenRates struct {
	InputMicrousdPerMillion      int64 `json:"input_microusd_per_million"`
	CacheReadMicrousdPerMillion  int64 `json:"cache_read_microusd_per_million"`
	CacheWriteMicrousdPerMillion int64 `json:"cache_write_microusd_per_million"`
	OutputMicrousdPerMillion     int64 `json:"output_microusd_per_million"`
}

// TierDefinition describes one model-size tier and its prices.
type TierDefinition struct {
	Key           Tier       `json:"key"`
	Label         string     `json:"label"`
	Description   string     `json:"description"`
	CeilingRates  TokenRates `json:"ceiling_rates"`
	CustomerRates TokenRates `json:"customer_rates"`
}

// PlanAllowance defines one period's internal percentage denominator.
type PlanAllowance struct {
	Plan              string `json:"plan"`
	BillingInterval   string `json:"billing_interval"`
	AllowanceMicrousd int64  `json:"allowance_microusd"`
	Soft              bool   `json:"soft,omitempty"`
}

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

// ToolRate defines one approved provider-native paid tool.
type ToolRate struct {
	Key              string `json:"key"`
	Label            string `json:"label"`
	Provider         string `json:"provider"`
	CeilingMicrousd  int64  `json:"ceiling_microusd_per_call"`
	CustomerMicrousd int64  `json:"customer_microusd_per_call"`
	SafeToExpose     bool   `json:"safe_to_expose"`
}

// PaidToolUsage is the observed call count for a priced provider-native tool.
type PaidToolUsage struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// Catalog is the immutable AI pricing and eligibility catalog.
type Catalog struct {
	PricingVersion string            `json:"pricing_version"`
	EffectiveDate  string            `json:"effective_date"`
	Tiers          []TierDefinition  `json:"tiers"`
	Plans          []PlanAllowance   `json:"plans"`
	Models         []ModelDefinition `json:"models"`
	Routes         []RouteDefinition `json:"routes"`
	Tools          []ToolRate        `json:"tools"`
	SourceURLs     []string          `json:"source_urls"`
	ReviewedDate   string            `json:"reviewed_date"`
}

// ResolvedRoute is the immutable pricing identity attached to one execution.
type ResolvedRoute struct {
	Provider       string
	CanonicalModel string
	Route          string
	ServiceTier    string
	Tier           Tier
	Rates          TokenRates
	RateSnapshot   json.RawMessage
	ContextWindow  int64
	MaximumOutput  int64
}

// ValidationIssue identifies an invalid catalog field.
type ValidationIssue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

// PublicTier is the customer-safe representation of one model-size tier.
type PublicTier struct {
	Key         Tier       `json:"key"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Rates       TokenRates `json:"rates"`
}

// PublicPlan is the machine-readable allowance for one plan schedule.
type PublicPlan struct {
	Plan              string `json:"plan"`
	BillingInterval   string `json:"billing_interval"`
	AllowanceMicrousd int64  `json:"allowance_microusd"`
	Soft              bool   `json:"soft"`
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

// PublicTool is a provider-native paid-tool price safe for customer display.
type PublicTool struct {
	Key             string `json:"key"`
	Label           string `json:"label"`
	MicrousdPerCall int64  `json:"microusd_per_call"`
}

// PublicPricing is the public, read-only pricing response.
type PublicPricing struct {
	PricingVersion string        `json:"pricing_version"`
	EffectiveDate  string        `json:"effective_date"`
	Tiers          []PublicTier  `json:"tiers"`
	Plans          []PublicPlan  `json:"plans"`
	Models         []PublicModel `json:"models"`
	Tools          []PublicTool  `json:"tools"`
}
