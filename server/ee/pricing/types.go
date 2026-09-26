//go:build ee

package pricing

import (
	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
)

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

// ModelDefinition is the nonfinancial model identity retained in public pricing.
type ModelDefinition = aimodel.ModelDefinition

// RouteDefinition is one exact provider route eligible for execution.
type RouteDefinition struct {
	Provider         string     `json:"provider"`
	CanonicalModel   string     `json:"canonical_model"`
	Route            string     `json:"route"`
	ServiceTier      string     `json:"service_tier"`
	Tier             Tier       `json:"tier"`
	MaximumCostRates TokenRates `json:"maximum_cost_rates"`
	Enabled          bool       `json:"enabled"`
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

// Catalog is the immutable AI pricing and eligibility catalog.
type Catalog struct {
	ModelCatalog   *aimodel.Catalog  `json:"-"`
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

// PublicModel is a nonfinancial model option.
type PublicModel = aimodel.PublicModel

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
type Tier = aiusage.Tier
type FundingMode = aiusage.FundingMode
type TokenRates = aiusage.TokenRates
type ResolvedRoute = aiusage.ResolvedRoute
type NormalizedTokens = aiusage.NormalizedTokens
type TokenTelemetry = aiusage.TokenTelemetry
type FlatTokenTariff = aiusage.FlatTokenTariff

const (
	TierSmall                  = aiusage.TierSmall
	TierMedium                 = aiusage.TierMedium
	TierLarge                  = aiusage.TierLarge
	TierFlagship               = aiusage.TierFlagship
	FundingHelpinHosted        = aiusage.FundingHelpinHosted
	FundingCustomer            = aiusage.FundingCustomer
	FundingCustomerPlatform    = aiusage.FundingCustomerPlatform
	FundingCustomerFlat        = aiusage.FundingCustomerFlat
	FlatTokenAccountingVersion = aiusage.FlatTokenAccountingVersion
)

var NormalizeTokens = aiusage.NormalizeTokens
var ErrInvalidTokenTelemetry = aiusage.ErrInvalidTokenTelemetry
