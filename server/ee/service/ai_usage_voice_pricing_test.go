//go:build ee

package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
)

func TestTranscriptionDurationPricing(t *testing.T) {
	request := MeteringRequest{WorkspaceID: "ws", IdempotencyKey: "voice", OperationKey: aiusage.OperationVoiceTranscription, Provider: "openrouter", Model: "openai/gpt-transcribe", FundingMode: aiusage.FundingHelpinHosted}
	ctx, err := resolveVoiceMetering(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ ms, want int64 }{{1000, 75}, {60000, 4500}, {120000, 9000}, {300000, 22500}, {1, 1}} {
		got, err := voiceChargeMicrousd(ctx, tt.ms)
		if err != nil || got != tt.want {
			t.Fatalf("%d ms: got %d, %v", tt.ms, got, err)
		}
	}
	request.Model = "another-model"
	if _, err := resolveVoiceMetering(request); err == nil {
		t.Fatal("accepted unpriced model")
	}
	if _, err := voiceChargeMicrousd(ctx, -1); err == nil {
		t.Fatal("accepted negative duration")
	}
}
