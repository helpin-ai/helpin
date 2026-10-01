//go:build ee

package service

import (
	"fmt"
	"math"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
)

// resolveVoiceMetering pins a duration tariff independently of chat tiers.
// Source: https://openrouter.ai/openai/gpt-transcribe (2026-10-01).
func resolveVoiceMetering(input MeteringRequest) (MeteringContext, error) {
	if input.OperationKey != aiusage.OperationVoiceTranscription || input.Provider != "openrouter" || input.Model != "openai/gpt-transcribe" || (input.Route != "" && input.Route != "openai/gpt-transcribe") || input.ServiceTier != "" || input.WorkspaceID == "" || input.IdempotencyKey == "" || (input.FundingMode != "" && input.FundingMode != aiusage.FundingHelpinHosted) || input.Promotional {
		return MeteringContext{}, fmt.Errorf("invalid transcription metering request")
	}
	return MeteringContext{
		WorkspaceID: input.WorkspaceID, TaskNature: input.TaskNature, FeatureKey: input.FeatureKey, OperationKey: input.OperationKey,
		IdempotencyKey: input.IdempotencyKey, FundingMode: aiusage.FundingHelpinHosted, PricingVersion: "voice-openrouter-2026-10-01",
		Route: aiusage.ResolvedRoute{Provider: "openrouter", CanonicalModel: "gpt-transcribe", Route: "openai/gpt-transcribe", Tier: aiusage.TierSmall,
			AudioMicrousdPerMinute: 4500, RateSnapshot: []byte(`{"audio_microusd_per_minute":4500,"unit":"audio_millisecond"}`)},
		ToolRates: map[string]int64{},
	}, nil
}

// voiceChargeMicrousd rounds up once per recording without inventing token counts.
func voiceChargeMicrousd(ctx MeteringContext, milliseconds int64) (int64, error) {
	rate := ctx.Route.AudioMicrousdPerMinute
	if ctx.OperationKey != aiusage.OperationVoiceTranscription || rate <= 0 || milliseconds < 0 || milliseconds > (math.MaxInt64-59999)/rate {
		return 0, fmt.Errorf("invalid audio usage")
	}
	return (milliseconds*rate + 59999) / 60000, nil
}
