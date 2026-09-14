package service

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const minimumObservedEstimateSamples = 10

type averageTokenBreakdown struct {
	InputTokensTotal    int64 `json:"input_tokens_total"`
	UncachedInputTokens int64 `json:"uncached_input_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	CacheWriteTokens    int64 `json:"cache_write_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	ReasoningTokens     int64 `json:"reasoning_tokens"`
}

func calculateTaskEstimate(entries []model.AIUsageLedgerEntry, calculatedAt time.Time) (model.AIUsageTaskEstimate, bool) {
	if len(entries) < minimumObservedEstimateSamples {
		return model.AIUsageTaskEstimate{}, false
	}
	sorted := append([]model.AIUsageLedgerEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].FinalChargedMicrousd < sorted[j].FinalChargedMicrousd })
	trim := len(sorted) / 10
	trimmed := sorted[trim : len(sorted)-trim]
	var charge int64
	var tokens averageTokenBreakdown
	for _, entry := range trimmed {
		charge += entry.FinalChargedMicrousd
		tokens.InputTokensTotal += entry.InputTokensTotal
		tokens.UncachedInputTokens += entry.UncachedInputTokens
		tokens.CacheReadTokens += entry.CacheReadTokens
		tokens.CacheWriteTokens += entry.CacheWriteTokens
		tokens.OutputTokens += entry.OutputTokens
		tokens.ReasoningTokens += entry.ReasoningTokens
	}
	count := int64(len(trimmed))
	tokens.InputTokensTotal /= count
	tokens.UncachedInputTokens /= count
	tokens.CacheReadTokens /= count
	tokens.CacheWriteTokens /= count
	tokens.OutputTokens /= count
	tokens.ReasoningTokens /= count
	encoded, _ := json.Marshal(tokens)
	return model.AIUsageTaskEstimate{
		SampleSize: len(entries), AverageTokens: model.JSONBlob(encoded), AverageChargeMicrousd: charge / count,
		P50ChargeMicrousd: nearestRank(sorted, 50), P90ChargeMicrousd: nearestRank(sorted, 90), CalculatedAt: calculatedAt,
	}, true
}

func nearestRank(entries []model.AIUsageLedgerEntry, percentile int) int64 {
	index := (percentile*len(entries)+99)/100 - 1
	if index < 0 {
		index = 0
	}
	return entries[index].FinalChargedMicrousd
}

func launchEstimateMicrousd(taskNature, tier string) int64 {
	base := map[string]int64{"small": 25_000, "medium": 75_000, "large": 250_000, "flagship": 750_000}
	value := base[strings.ToLower(tier)]
	if value == 0 {
		value = base["large"]
	}
	switch strings.ToLower(taskNature) {
	case "planning", "coding", "review":
		return value * 4
	default:
		return value
	}
}
