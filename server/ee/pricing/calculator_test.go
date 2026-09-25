//go:build ee

package pricing

import (
	"errors"
	"math"
	"testing"
)

func TestNormalizeTokensSeparatesReasoningIncludedInCompletion(t *testing.T) {
	got, err := NormalizeTokens(TokenTelemetry{
		InputTokensTotal:            100,
		CacheReadTokens:             30,
		CacheWriteTokens:            10,
		CompletionTokensTotal:       50,
		ReasoningTokens:             20,
		CompletionIncludesReasoning: true,
	})
	if err != nil {
		t.Fatalf("NormalizeTokens() error = %v", err)
	}
	want := NormalizedTokens{
		InputTokensTotal: 100, UncachedInputTokens: 60,
		CacheReadTokens: 30, CacheWriteTokens: 10,
		OutputTokens: 30, ReasoningTokens: 20,
	}
	if got != want {
		t.Fatalf("NormalizeTokens() = %#v, want %#v", got, want)
	}
}

func TestNormalizeTokensClampsOverreportedCacheToZeroUncached(t *testing.T) {
	got, err := NormalizeTokens(TokenTelemetry{
		InputTokensTotal: 20, CacheReadTokens: 15, CacheWriteTokens: 10,
		OutputTokens: 5,
	})
	if err != nil {
		t.Fatalf("NormalizeTokens() error = %v", err)
	}
	if got.UncachedInputTokens != 0 {
		t.Fatalf("uncached input = %d, want 0", got.UncachedInputTokens)
	}
}

func TestNormalizeTokensRejectsNegativeTelemetry(t *testing.T) {
	_, err := NormalizeTokens(TokenTelemetry{InputTokensTotal: -1})
	if !errors.Is(err, ErrInvalidTokenTelemetry) {
		t.Fatalf("NormalizeTokens() error = %v, want ErrInvalidTokenTelemetry", err)
	}
}

func TestCalculateChargeRoundsCompleteHostedEventOnce(t *testing.T) {
	got, err := CalculateCharge(ChargeInput{
		FundingMode: FundingHelpinHosted,
		Tokens: NormalizedTokens{
			UncachedInputTokens: 3, CacheReadTokens: 2,
			CacheWriteTokens: 1, OutputTokens: 2, ReasoningTokens: 1,
		},
		Rates: TokenRates{
			InputMicrousdPerMillion: 220_000, CacheReadMicrousdPerMillion: 22_000,
			CacheWriteMicrousdPerMillion: 275_000, OutputMicrousdPerMillion: 1_320_000,
		},
		PaidToolMicrousd: 7,
	})
	if err != nil {
		t.Fatalf("CalculateCharge() error = %v", err)
	}
	if got.PublishedEquivalentMicrousd != 12 || got.HostedMicrousd != 12 || got.FinalMicrousd != 12 {
		t.Fatalf("CalculateCharge() = %#v, want published/hosted/final 12", got)
	}
}

func TestCalculateChargeCustomerFundedChargesTenPercent(t *testing.T) {
	got, err := CalculateCharge(ChargeInput{
		FundingMode: FundingCustomer,
		Tokens:      NormalizedTokens{UncachedInputTokens: 1_000_000},
		Rates:       TokenRates{InputMicrousdPerMillion: 220_000},
	})
	if err != nil {
		t.Fatalf("CalculateCharge() error = %v", err)
	}
	if got.PublishedEquivalentMicrousd != 220_000 || got.OrchestrationMicrousd != 22_000 || got.FinalMicrousd != 22_000 {
		t.Fatalf("CalculateCharge() = %#v", got)
	}
	if got.HostedMicrousd != 0 {
		t.Fatalf("hosted charge = %d, want 0", got.HostedMicrousd)
	}
}

func TestCalculateChargeRejectsOverflow(t *testing.T) {
	_, err := CalculateCharge(ChargeInput{
		FundingMode: FundingHelpinHosted,
		Tokens:      NormalizedTokens{UncachedInputTokens: math.MaxInt64},
		Rates:       TokenRates{InputMicrousdPerMillion: math.MaxInt64},
	})
	if !errors.Is(err, ErrChargeOverflow) {
		t.Fatalf("CalculateCharge() error = %v, want ErrChargeOverflow", err)
	}
}

func TestRoundMicrousdToCentsUsesHalfUpRounding(t *testing.T) {
	tests := []struct {
		name           string
		microusd       int64
		wantCents      int64
		wantAdjustment int64
	}{
		{name: "below half", microusd: 4_999, wantCents: 0, wantAdjustment: -4_999},
		{name: "at half", microusd: 5_000, wantCents: 1, wantAdjustment: 5_000},
		{name: "ordinary", microusd: 33_700_001, wantCents: 3_370, wantAdjustment: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cents, adjustment, err := RoundMicrousdToCents(tt.microusd)
			if err != nil {
				t.Fatalf("RoundMicrousdToCents() error = %v", err)
			}
			if cents != tt.wantCents || adjustment != tt.wantAdjustment {
				t.Fatalf("RoundMicrousdToCents() = %d/%d, want %d/%d",
					cents, adjustment, tt.wantCents, tt.wantAdjustment)
			}
		})
	}
}

func TestPersonalConnectionKeepsFullPlatformCharge(t *testing.T) {
	charge, err := CalculateCharge(ChargeInput{FundingMode: FundingCustomerPlatform, Tokens: NormalizedTokens{UncachedInputTokens: 1_000_000}, Rates: TokenRates{InputMicrousdPerMillion: 220_000}})
	if err != nil || charge.FinalMicrousd != 220_000 || charge.OrchestrationMicrousd != 220_000 || charge.HostedMicrousd != 0 {
		t.Fatalf("personal connection charge: %+v %v", charge, err)
	}
}
