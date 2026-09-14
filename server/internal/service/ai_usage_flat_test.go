package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
)

func testFlatTariff(rate int64) *aiusage.FlatTokenTariff {
	return &aiusage.FlatTokenTariff{Version: "byok-2026-09", Currency: "USD",
		MicrousdPerMillion: &rate, AccountingVersion: aiusage.FlatTokenAccountingVersion}
}

func TestFlatBYOKRequiresCompleteTariffBeforeReservation(t *testing.T) {
	for _, name := range []string{"missing", "unset rate", "negative", "currency", "version", "accounting"} {
		t.Run(name, func(t *testing.T) {
			tariff := testFlatTariff(0)
			switch name {
			case "missing":
				tariff = nil
			case "unset rate":
				tariff.MicrousdPerMillion = nil
			case "negative":
				*tariff.MicrousdPerMillion = -1
			case "currency":
				tariff.Currency = ""
			case "version":
				tariff.Version = ""
			case "accounting":
				tariff.AccountingVersion = "unknown"
			}
			store := &fakeAIUsageStore{}
			svc := NewAIUsageService(nil, store, nil)
			_, err := svc.Preflight(context.Background(), PreflightRequest{Metering: MeteringRequest{
				Provider: "openai", Model: "private-model", FundingMode: aiusage.FundingCustomerFlat, FlatTariff: tariff}})
			if err == nil || store.reserveCalls != 0 {
				t.Fatalf("error = %v, reservations = %d", err, store.reserveCalls)
			}
		})
	}
}

func TestFlatBYOKCustomModelFreezesTokenAndToolTariffs(t *testing.T) {
	store := &fakeAIUsageStore{}
	catalog := &aiusage.Catalog{Tools: []aiusage.ToolRate{{Key: "search", CustomerMicrousd: 7500}}}
	svc := NewAIUsageService(catalog, store, nil)
	tariff := testFlatTariff(2_000_000)
	metering, err := svc.Preflight(context.Background(), PreflightRequest{Metering: MeteringRequest{
		WorkspaceID: "ws", Provider: "openai_chatgpt", Model: "private-unpriced-model",
		FundingMode: aiusage.FundingCustomerFlat, FlatTariff: tariff, MaximumOutputTokens: 1000,
		InputTokensEstimate: 5000, AllowedPaidTools: []string{"search"}, IdempotencyKey: "run-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if metering.MaxBillableMicrousd != 19_500 {
		t.Fatalf("bound = %d", metering.MaxBillableMicrousd)
	}
	*tariff.MicrousdPerMillion = 99_000_000
	catalog.Tools[0].CustomerMicrousd = 999_000
	// Persist/reload the exact admission context, as a restarted worker does.
	raw, err := json.Marshal(metering)
	if err != nil {
		t.Fatal(err)
	}
	var restored MeteringContext
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Reconcile(context.Background(), CompletionUsage{Context: restored,
		Telemetry: aiusage.TokenTelemetry{InputTokensTotal: 1000, CacheReadTokens: 300, CacheWriteTokens: 200,
			CompletionTokensTotal: 500, ReasoningTokens: 100, CompletionIncludesReasoning: true},
		PaidTools: []aiusage.PaidToolUsage{{Key: "search", Count: 2}}, MeasurementStatus: "actual"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ChargedMicrousd != 18_000 {
		t.Fatalf("charge = %d, want 3000 tokens fee + 15000 tools", result.ChargedMicrousd)
	}
	if store.reconcile.Entry.PublishedChargeMicrousd != 0 {
		t.Fatal("flat fee reported as published-equivalent price")
	}
	if !strings.Contains(string(store.reconcile.Entry.RateSnapshot), "normalized-tokens-v1") {
		t.Fatal("missing accounting snapshot")
	}
}

func TestFlatBYOKZeroRateAndEstimatesNeverUseLaunchCharge(t *testing.T) {
	for _, rate := range []int64{0, 2_000_000} {
		store := &fakeAIUsageStore{}
		svc := NewAIUsageService(nil, store, nil)
		metering, err := svc.Preflight(context.Background(), PreflightRequest{Metering: MeteringRequest{
			Provider: "openai", Model: "private-model", FundingMode: aiusage.FundingCustomerFlat,
			FlatTariff: testFlatTariff(rate), InputTokensEstimate: 1000, MaximumOutputTokens: 1000}})
		if err != nil {
			t.Fatal(err)
		}
		result, err := svc.Reconcile(context.Background(), CompletionUsage{Context: *metering,
			Telemetry: aiusage.TokenTelemetry{InputTokensTotal: 100}, MeasurementStatus: "estimated"})
		if err != nil {
			t.Fatal(err)
		}
		if result.ChargedMicrousd != rate/10_000 {
			t.Fatalf("rate %d: charge = %d", rate, result.ChargedMicrousd)
		}
		if store.reconcile.Entry.EstimationMethod != "estimated_tokens" {
			t.Fatal("wrong estimate basis")
		}
	}
}

func TestFlatBYOKCheckpointRoundingMatchesOneExecution(t *testing.T) {
	svc := NewAIUsageService(nil, &fakeAIUsageStore{}, nil)
	metering, err := svc.ResolveMeteringContext(MeteringRequest{Provider: "openai", Model: "private-model",
		FundingMode: aiusage.FundingCustomerFlat, FlatTariff: testFlatTariff(600_000)})
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for i := int64(1); i <= 10; i++ {
		previous := aiusage.TokenTelemetry{InputTokensTotal: i - 1}
		current := aiusage.TokenTelemetry{InputTokensTotal: i}
		_, charge, _, _, err := svc.prepareCompletion(context.Background(), CompletionUsage{Context: metering,
			Telemetry: aiusage.TokenTelemetry{InputTokensTotal: 1}, PreviousTelemetry: &previous,
			CumulativeTelemetry: &current, MeasurementStatus: "actual"})
		if err != nil {
			t.Fatal(err)
		}
		total += charge
	}
	if total != 6 {
		t.Fatalf("split execution charge = %d, want 6", total)
	}
}
