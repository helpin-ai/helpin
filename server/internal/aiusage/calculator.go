package aiusage

import "errors"

// ErrInvalidTokenTelemetry indicates negative or contradictory token counts.
var ErrInvalidTokenTelemetry = errors.New("invalid token telemetry")

type TokenTelemetry struct {
	InputTokensTotal            int64
	CacheReadTokens             int64
	CacheWriteTokens            int64
	CompletionTokensTotal       int64
	OutputTokens                int64
	ReasoningTokens             int64
	CompletionIncludesReasoning bool
}

type NormalizedTokens struct {
	InputTokensTotal    int64
	UncachedInputTokens int64
	CacheReadTokens     int64
	CacheWriteTokens    int64
	OutputTokens        int64
	ReasoningTokens     int64
}

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
