//go:build ee

package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
)

func TestVoiceDurationReservationAndReconciliation(t *testing.T) {
	store := &fakeAIUsageStore{}
	svc := newTestAIUsageService(t, store)
	metering, err := svc.Preflight(context.Background(), PreflightRequest{Metering: MeteringRequest{
		WorkspaceID: "ws", TaskNature: "voice", FeatureKey: "voice_transcription", OperationKey: aiusage.OperationVoiceTranscription,
		Provider: "openrouter", Model: "openai/gpt-transcribe", IdempotencyKey: "voice", AudioMillisecondsEstimate: 300000,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if store.reservation.ReservedMicrousd != 22500 {
		t.Fatalf("hold %d", store.reservation.ReservedMicrousd)
	}
	_, err = svc.Reconcile(context.Background(), CompletionUsage{Context: *metering, AudioMilliseconds: 30000, MeasurementStatus: "actual"})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.reconciles) != 1 || store.reconciles[0].Entry.PublishedChargeMicrousd != 2250 {
		t.Fatalf("reconcile: %+v", store.reconciles)
	}
}
