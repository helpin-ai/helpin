package pricing

import (
	"errors"
	"math"
	"testing"
)

func TestFlatTariffChargesEveryTokenOnceAndToolsSeparately(t *testing.T) {
	tokens, err := NormalizeTokens(TokenTelemetry{InputTokensTotal: 1000, CacheReadTokens: 400,
		CacheWriteTokens: 200, CompletionTokensTotal: 500, ReasoningTokens: 100, CompletionIncludesReasoning: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, rate := range []int64{0, 1, 1_000_000, 20_000_000} {
		tariff := FlatTokenTariff{MicrousdPerMillion: &rate}
		charge, err := CalculateCharge(ChargeInput{FundingMode: FundingCustomerFlat, Tokens: tokens,
			Rates: FlatTokenRates(tariff), PaidToolMicrousd: 30_000})
		if err != nil {
			t.Fatal(err)
		}
		want := (1500*rate + 500_000) / 1_000_000
		if charge.FinalMicrousd != want+30_000 || charge.OrchestrationMicrousd != want || charge.PublishedEquivalentMicrousd != 0 {
			t.Fatalf("rate %d: charge = %#v", rate, charge)
		}
	}
}

func TestFlatTariffRejectsOverflowAndUnequalRates(t *testing.T) {
	rate := int64(math.MaxInt64)
	tariff := FlatTokenTariff{MicrousdPerMillion: &rate}
	_, err := CalculateCharge(ChargeInput{FundingMode: FundingCustomerFlat,
		Tokens: NormalizedTokens{UncachedInputTokens: math.MaxInt64}, Rates: FlatTokenRates(tariff)})
	if !errors.Is(err, ErrChargeOverflow) {
		t.Fatalf("overflow error = %v", err)
	}
	_, err = CalculateCharge(ChargeInput{FundingMode: FundingCustomerFlat,
		Rates: TokenRates{InputMicrousdPerMillion: 1}})
	if !errors.Is(err, ErrInvalidTokenTelemetry) {
		t.Fatalf("unequal rates error = %v", err)
	}
}
