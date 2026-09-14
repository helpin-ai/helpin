//go:build ee

package service

import (
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/ee/pricing"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *AIUsageService) resolveFlatMeteringContext(input MeteringRequest) (MeteringContext, error) {
	if err := pricing.ValidateFlatTokenTariff(input.FlatTariff); err != nil {
		return MeteringContext{}, fmt.Errorf("%w: %v", model.ErrPricingConfigurationMissing, err)
	}
	if err := sdk.ValidateRunModel(&sdk.RunModel{Provider: input.Provider, Model: input.Model, Endpoint: input.Endpoint}); err != nil {
		return MeteringContext{}, err
	}
	if input.OperationKey != "" {
		return MeteringContext{}, fmt.Errorf("%w: BYOK cannot replace a governed media operation", model.ErrModelUnavailableUnderPricing)
	}
	// Copy the rate as well as the struct so changes to configuration cannot
	// mutate an already accepted context through its pointer.
	tariff := *input.FlatTariff
	rate := *tariff.MicrousdPerMillion
	tariff.MicrousdPerMillion = &rate
	tools := s.snapshotToolRates()
	snapshot, err := json.Marshal(struct {
		Basis     string                  `json:"basis"`
		Tariff    aiusage.FlatTokenTariff `json:"tariff"`
		ToolRates map[string]int64        `json:"tool_rates_microusd"`
	}{Basis: "per_million_normalized_tokens", Tariff: tariff, ToolRates: tools})
	if err != nil {
		return MeteringContext{}, err
	}
	route := strings.TrimSpace(input.Route)
	if route == "" {
		route = input.Model
	}
	return MeteringContext{
		Route: aiusage.ResolvedRoute{Provider: input.Provider, CanonicalModel: input.Model,
			Route: route, ServiceTier: input.ServiceTier, Rates: pricing.FlatTokenRates(tariff), RateSnapshot: snapshot},
		PricingVersion: tariff.Version, FlatTariff: &tariff, ToolRates: tools,
		WorkspaceID: input.WorkspaceID, TaskNature: input.TaskNature, FeatureKey: input.FeatureKey,
		FundingMode: aiusage.FundingCustomerFlat, IdempotencyKey: input.IdempotencyKey,
		Promotional: input.Promotional,
	}, nil
}

func (s *AIUsageService) snapshotToolRates() map[string]int64 {
	rates := make(map[string]int64)
	if s.catalog != nil {
		for _, tool := range s.catalog.Tools {
			rates[strings.ToLower(strings.TrimSpace(tool.Key))] = tool.CustomerMicrousd
		}
	}
	return rates
}

func flatCheckpointCharge(input CompletionUsage, tools int64) (pricing.Charge, error) {
	if input.PreviousTelemetry == nil {
		return pricing.Charge{}, aiusage.ErrInvalidTokenTelemetry
	}
	current, err := aiusage.NormalizeTokens(*input.CumulativeTelemetry)
	if err != nil {
		return pricing.Charge{}, err
	}
	previous, err := aiusage.NormalizeTokens(*input.PreviousTelemetry)
	if err != nil {
		return pricing.Charge{}, err
	}
	charge, err := pricing.CalculateCharge(pricing.ChargeInput{FundingMode: aiusage.FundingCustomerFlat,
		Tokens: current, Rates: input.Context.Route.Rates, PaidToolMicrousd: tools})
	if err != nil {
		return pricing.Charge{}, err
	}
	prior, err := pricing.CalculateCharge(pricing.ChargeInput{FundingMode: aiusage.FundingCustomerFlat,
		Tokens: previous, Rates: input.Context.Route.Rates})
	if err != nil {
		return pricing.Charge{}, err
	}
	if charge.OrchestrationMicrousd < prior.OrchestrationMicrousd {
		return pricing.Charge{}, aiusage.ErrInvalidTokenTelemetry
	}
	charge.OrchestrationMicrousd -= prior.OrchestrationMicrousd
	charge.FinalMicrousd -= prior.FinalMicrousd
	return charge, nil
}
