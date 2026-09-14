package aiusage

import (
	"errors"
	"fmt"
	"math"
	"math/big"
)

const tokensPerMillion int64 = 1_000_000

var (
	// ErrInvalidTokenTelemetry indicates negative or contradictory token counts.
	ErrInvalidTokenTelemetry = errors.New("invalid token telemetry")
	// ErrChargeOverflow indicates that an exact charge cannot fit in int64 micro-USD.
	ErrChargeOverflow = errors.New("AI usage charge overflow")
)

// TokenTelemetry is provider/runtime token data before normalization.
type TokenTelemetry struct {
	InputTokensTotal            int64
	CacheReadTokens             int64
	CacheWriteTokens            int64
	CompletionTokensTotal       int64
	OutputTokens                int64
	ReasoningTokens             int64
	CompletionIncludesReasoning bool
}

// NormalizedTokens separates every billable token class.
type NormalizedTokens struct {
	InputTokensTotal    int64
	UncachedInputTokens int64
	CacheReadTokens     int64
	CacheWriteTokens    int64
	OutputTokens        int64
	ReasoningTokens     int64
}

// ChargeInput contains one complete event's normalized usage and prices.
type ChargeInput struct {
	FundingMode      FundingMode
	Tokens           NormalizedTokens
	Rates            TokenRates
	PaidToolMicrousd int64
}

// Charge is the exact value result for one complete event.
type Charge struct {
	PublishedEquivalentMicrousd int64
	HostedMicrousd              int64
	OrchestrationMicrousd       int64
	FinalMicrousd               int64
}

// NormalizeTokens produces mutually exclusive billable token classes.
func NormalizeTokens(input TokenTelemetry) (NormalizedTokens, error) {
	values := []int64{
		input.InputTokensTotal, input.CacheReadTokens, input.CacheWriteTokens,
		input.CompletionTokensTotal, input.OutputTokens, input.ReasoningTokens,
	}
	for _, value := range values {
		if value < 0 {
			return NormalizedTokens{}, ErrInvalidTokenTelemetry
		}
	}

	outputTokens := input.OutputTokens
	if input.CompletionTokensTotal > 0 {
		outputTokens = input.CompletionTokensTotal
		if input.CompletionIncludesReasoning {
			outputTokens -= input.ReasoningTokens
			if outputTokens < 0 {
				outputTokens = 0
			}
		}
	}

	uncachedInput := input.InputTokensTotal - input.CacheReadTokens
	if uncachedInput < 0 {
		uncachedInput = 0
	}
	uncachedInput -= input.CacheWriteTokens
	if uncachedInput < 0 {
		uncachedInput = 0
	}

	return NormalizedTokens{
		InputTokensTotal: input.InputTokensTotal, UncachedInputTokens: uncachedInput,
		CacheReadTokens: input.CacheReadTokens, CacheWriteTokens: input.CacheWriteTokens,
		OutputTokens: outputTokens, ReasoningTokens: input.ReasoningTokens,
	}, nil
}

// CalculateCharge calculates and rounds one complete event in micro-USD.
func CalculateCharge(input ChargeInput) (Charge, error) {
	if err := validateChargeInput(input); err != nil {
		return Charge{}, err
	}

	numerator := new(big.Int)
	components := []struct {
		tokens int64
		rate   int64
	}{
		{tokens: input.Tokens.UncachedInputTokens, rate: input.Rates.InputMicrousdPerMillion},
		{tokens: input.Tokens.CacheReadTokens, rate: input.Rates.CacheReadMicrousdPerMillion},
		{tokens: input.Tokens.CacheWriteTokens, rate: input.Rates.CacheWriteMicrousdPerMillion},
		{tokens: input.Tokens.OutputTokens, rate: input.Rates.OutputMicrousdPerMillion},
		{tokens: input.Tokens.ReasoningTokens, rate: input.Rates.OutputMicrousdPerMillion},
	}
	for _, component := range components {
		term := new(big.Int).Mul(big.NewInt(component.tokens), big.NewInt(component.rate))
		numerator.Add(numerator, term)
	}
	toolTerm := new(big.Int).Mul(big.NewInt(input.PaidToolMicrousd), big.NewInt(tokensPerMillion))
	numerator.Add(numerator, toolTerm)

	published, err := roundPositiveRational(numerator, big.NewInt(tokensPerMillion))
	if err != nil {
		return Charge{}, err
	}
	result := Charge{PublishedEquivalentMicrousd: published}
	switch input.FundingMode {
	case FundingCustomerFlat:
		// The route rates are the flat tariff, not a provider-equivalent price.
		result.PublishedEquivalentMicrousd = 0
		result.OrchestrationMicrousd = published - input.PaidToolMicrousd
		result.FinalMicrousd = published
	case FundingHelpinHosted:
		result.HostedMicrousd = published
		result.FinalMicrousd = published
	case FundingCustomerPlatform:
		result.OrchestrationMicrousd = published
		result.FinalMicrousd = published
	case FundingCustomer:
		orchestration, err := roundPositiveRational(big.NewInt(published), big.NewInt(10))
		if err != nil {
			return Charge{}, err
		}
		result.OrchestrationMicrousd = orchestration
		result.FinalMicrousd = orchestration
	default:
		return Charge{}, fmt.Errorf("%w: unknown funding mode %q", ErrInvalidTokenTelemetry, input.FundingMode)
	}
	return result, nil
}

// RoundMicrousdToCents applies half-up rounding and returns the exact adjustment.
func RoundMicrousdToCents(microusd int64) (int64, int64, error) {
	if microusd < 0 {
		return 0, 0, ErrInvalidTokenTelemetry
	}
	cents := microusd / 10_000
	if microusd%10_000 >= 5_000 {
		if cents == math.MaxInt64 {
			return 0, 0, ErrChargeOverflow
		}
		cents++
	}
	if cents > math.MaxInt64/10_000 {
		return 0, 0, ErrChargeOverflow
	}
	adjustment := cents*10_000 - microusd
	return cents, adjustment, nil
}

func validateChargeInput(input ChargeInput) error {
	if input.FundingMode == FundingCustomerFlat &&
		(input.Rates.InputMicrousdPerMillion != input.Rates.OutputMicrousdPerMillion ||
			input.Rates.InputMicrousdPerMillion != input.Rates.CacheReadMicrousdPerMillion ||
			input.Rates.InputMicrousdPerMillion != input.Rates.CacheWriteMicrousdPerMillion) {
		return ErrInvalidTokenTelemetry
	}
	values := []int64{
		input.Tokens.InputTokensTotal, input.Tokens.UncachedInputTokens,
		input.Tokens.CacheReadTokens, input.Tokens.CacheWriteTokens,
		input.Tokens.OutputTokens, input.Tokens.ReasoningTokens,
		input.Rates.InputMicrousdPerMillion, input.Rates.CacheReadMicrousdPerMillion,
		input.Rates.CacheWriteMicrousdPerMillion, input.Rates.OutputMicrousdPerMillion,
		input.PaidToolMicrousd,
	}
	for _, value := range values {
		if value < 0 {
			return ErrInvalidTokenTelemetry
		}
	}
	return nil
}

func roundPositiveRational(numerator, denominator *big.Int) (int64, error) {
	if denominator.Sign() <= 0 || numerator.Sign() < 0 {
		return 0, ErrInvalidTokenTelemetry
	}
	adjusted := new(big.Int).Add(numerator, new(big.Int).Quo(denominator, big.NewInt(2)))
	result := new(big.Int).Quo(adjusted, denominator)
	if !result.IsInt64() {
		return 0, ErrChargeOverflow
	}
	return result.Int64(), nil
}
