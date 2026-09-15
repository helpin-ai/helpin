package aiusage

import "testing"

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
