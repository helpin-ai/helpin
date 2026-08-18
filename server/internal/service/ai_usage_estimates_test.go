package service

import (
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestTaskEstimateTrimsTenPercentAndCalculatesPercentiles(t *testing.T) {
	entries := make([]model.AIUsageLedgerEntry, 20)
	for index := range entries {
		value := int64(index + 1)
		entries[index] = model.AIUsageLedgerEntry{
			FinalChargedMicrousd: value, InputTokensTotal: value * 10,
			UncachedInputTokens: value * 8, CacheReadTokens: value,
			CacheWriteTokens: value, OutputTokens: value * 2, ReasoningTokens: value,
		}
	}
	estimate, ok := calculateTaskEstimate(entries, time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("calculateTaskEstimate() unavailable")
	}
	if estimate.SampleSize != 20 || estimate.AverageChargeMicrousd != 10 || estimate.P50ChargeMicrousd != 10 || estimate.P90ChargeMicrousd != 18 {
		t.Fatalf("estimate = %#v", estimate)
	}
	if string(estimate.AverageTokens) != `{"input_tokens_total":105,"uncached_input_tokens":84,"cache_read_tokens":10,"cache_write_tokens":10,"output_tokens":21,"reasoning_tokens":10}` {
		t.Fatalf("average tokens = %s", estimate.AverageTokens)
	}
}

func TestTaskEstimateRequiresTenActualObservations(t *testing.T) {
	entries := make([]model.AIUsageLedgerEntry, 9)
	if _, ok := calculateTaskEstimate(entries, time.Now()); ok {
		t.Fatal("calculateTaskEstimate() available with 9 observations")
	}
	if fallback := launchEstimateMicrousd("planning", "large"); fallback <= 0 {
		t.Fatalf("launch estimate = %d, want positive", fallback)
	}
}
