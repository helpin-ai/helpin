package aiusage

import (
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/aimodel"
)

// Tier is the legacy model-size classification.
type Tier = aimodel.Tier

const (
	TierSmall    = aimodel.TierSmall
	TierMedium   = aimodel.TierMedium
	TierLarge    = aimodel.TierLarge
	TierFlagship = aimodel.TierFlagship
)

// FundingMode identifies who pays the inference provider.
type FundingMode string

const (
	// FundingHelpinHosted means Helpin pays the inference provider.
	FundingHelpinHosted FundingMode = "helpin_hosted"
	// FundingCustomer means the customer pays the inference provider.
	FundingCustomer FundingMode = "customer_funded"
	// FundingCustomerPlatform applies the full Helpin platform charge to personal connections.
	FundingCustomerPlatform FundingMode = "customer_funded_platform"
	// FundingCustomerFlat charges normalized tokens at the accepted flat tariff;
	// paid tools retain their own prices and receive no token surcharge.
	FundingCustomerFlat FundingMode = "customer_funded_flat"
	// FundingCustomerUnbilled records community usage without a Helpin charge.
	FundingCustomerUnbilled FundingMode = "customer_unbilled"
)

// TokenRates holds integer micro-USD prices per one million actual tokens.
type TokenRates struct {
	InputMicrousdPerMillion      int64 `json:"input_microusd_per_million"`
	CacheReadMicrousdPerMillion  int64 `json:"cache_read_microusd_per_million"`
	CacheWriteMicrousdPerMillion int64 `json:"cache_write_microusd_per_million"`
	OutputMicrousdPerMillion     int64 `json:"output_microusd_per_million"`
}

// PaidToolUsage is the observed call count for a priced provider-native tool.
type PaidToolUsage struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// ResolvedRoute is the immutable pricing identity attached to one execution.
type ResolvedRoute struct {
	AudioMicrousdPerMinute int64
	Provider               string
	CanonicalModel         string
	Route                  string
	ServiceTier            string
	Tier                   Tier
	Rates                  TokenRates
	RateSnapshot           json.RawMessage
	ContextWindow          int64
	MaximumOutput          int64
}

// PublicPricing is the public, read-only pricing response.
