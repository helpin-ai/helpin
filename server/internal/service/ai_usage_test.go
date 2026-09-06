package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestAIUsageServiceResolvesBuiltInTaskTiers(t *testing.T) {
	service := newTestAIUsageService(t, &fakeAIUsageStore{})
	tests := []struct {
		name, task, provider, model, route string
		wantTier                           aiusage.Tier
	}{
		{name: "support is small", task: "support", provider: "openai", model: "gpt-5.6-luna", route: "gpt-5.6-luna", wantTier: aiusage.TierSmall},
		{name: "planning is large", task: "planning", provider: "openai", model: "gpt-5.6-terra", route: "gpt-5.6-terra", wantTier: aiusage.TierLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := service.ResolveMeteringContext(MeteringRequest{
				WorkspaceID: "ws", TaskNature: test.task, Provider: test.provider, Model: test.model,
				Route: test.route, FundingMode: aiusage.FundingHelpinHosted,
			})
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Route.Tier != test.wantTier {
				t.Fatalf("tier = %q, want %q", resolved.Route.Tier, test.wantTier)
			}
		})
	}
}

func TestAIUsageServiceResolvesCompanyContextRoute(t *testing.T) {
	usageService := newTestAIUsageService(t, &fakeAIUsageStore{})
	policy, ok := DefaultAICompletionRouteRegistry().Policy(BillingFeatureCompanyProductContext, "")
	if !ok {
		t.Fatal("company context route policy missing")
	}
	resolved, err := usageService.ResolveMeteringContext(MeteringRequest{
		WorkspaceID: "ws", TaskNature: taskNatureForFeature(BillingFeatureCompanyProductContext),
		FeatureKey: BillingFeatureCompanyProductContext, Provider: policy.Primary.Provider,
		Model: policy.Primary.Model, Route: policy.Primary.Model, FundingMode: aiusage.FundingHelpinHosted,
		Promotional: true,
	})
	if err != nil {
		t.Fatalf("ResolveMeteringContext() error = %v", err)
	}
	if resolved.Route.Tier != aiusage.TierSmall {
		t.Fatalf("company context tier = %q, want %q", resolved.Route.Tier, aiusage.TierSmall)
	}
}

func TestAIUsageServiceResolvesCataloguedRouteRegardlessOfTaskNature(t *testing.T) {
	service := newTestAIUsageService(t, &fakeAIUsageStore{})
	for _, request := range []MeteringRequest{
		{TaskNature: "planning", Provider: "openai", Model: "gpt-5.6-luna", Route: "gpt-5.6-luna"},
		{TaskNature: "support", Provider: "openai", Model: "gpt-5-mini", Route: "gpt-5-mini"},
		{TaskNature: "support", Provider: "openai", Model: "gpt-5.6-terra", Route: "gpt-5.6-terra"},
		{TaskNature: "support", Provider: "openai", Model: "gpt-5.5", Route: "gpt-5.5"},
	} {
		if _, err := service.ResolveMeteringContext(request); err != nil {
			t.Fatalf("ResolveMeteringContext(%s/%s) error = %v", request.Provider, request.Model, err)
		}
	}
}

func TestAIUsageServiceRejectsUnknownRoute(t *testing.T) {
	service := newTestAIUsageService(t, &fakeAIUsageStore{})
	_, err := service.ResolveMeteringContext(MeteringRequest{
		TaskNature: "custom", Provider: "openai", Model: "unknown", Route: "unknown",
	})
	if !errors.Is(err, model.ErrModelUnavailableUnderPricing) {
		t.Fatalf("ResolveMeteringContext() error = %v, want model unavailable", err)
	}
}

func TestAIUsageServiceAllowsConfiguredMediaEnrichmentRouteForSupportWork(t *testing.T) {
	service := newTestAIUsageService(t, &fakeAIUsageStore{})

	resolved, err := service.ResolveMeteringContext(MeteringRequest{
		WorkspaceID: "ws", TaskNature: "support", FeatureKey: BillingFeatureAskChat,
		OperationKey: AIUsageOperationMediaEnrichment,
		Provider:     "openrouter", Model: "google/gemini-3.8-flash", Route: "google/gemini-3.8-flash",
		FundingMode: aiusage.FundingHelpinHosted,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Route.Tier != aiusage.TierMedium {
		t.Fatalf("tier = %q, want %q", resolved.Route.Tier, aiusage.TierMedium)
	}
}

func TestAIUsageServiceRejectsMediaEnrichmentOperationForOtherModels(t *testing.T) {
	service := newTestAIUsageService(t, &fakeAIUsageStore{})

	_, err := service.ResolveMeteringContext(MeteringRequest{
		WorkspaceID: "ws", TaskNature: "support", FeatureKey: BillingFeatureAskChat,
		OperationKey: AIUsageOperationMediaEnrichment,
		Provider:     "openai", Model: "gpt-5.6-terra", Route: "gpt-5.6-terra",
		FundingMode: aiusage.FundingHelpinHosted,
	})
	if !errors.Is(err, model.ErrModelUnavailableUnderPricing) {
		t.Fatalf("ResolveMeteringContext() error = %v, want model unavailable", err)
	}
}

func TestAIUsageServicePreflightUsesGreaterP90AndDeterministicBound(t *testing.T) {
	store := &fakeAIUsageStore{}
	service := newTestAIUsageService(t, store)
	service.estimates = fixedAIUsageEstimates{p90: 900_000}
	context, err := service.Preflight(context.Background(), PreflightRequest{Metering: MeteringRequest{
		WorkspaceID: "ws", TaskNature: "support", FeatureKey: "support_reply", Provider: "openai",
		Model: "gpt-5.6-luna", Route: "gpt-5.6-luna", FundingMode: aiusage.FundingHelpinHosted,
		InputTokensEstimate: 1_000, MaximumOutputTokens: 100, ExecutionID: "run", IdempotencyKey: "run:1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if store.reservation.ReservedMicrousd != 900_000 {
		t.Fatalf("reserved = %d, want P90 900000", store.reservation.ReservedMicrousd)
	}
	if context.MaxBillableMicrousd != 900_000 || context.ReservationID != "reservation" {
		t.Fatalf("metering context = %#v", context)
	}
}

func TestAIUsageServicePaidToolsIncreaseReservationBound(t *testing.T) {
	catalog, err := aiusage.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	catalog.Tools = append(catalog.Tools, aiusage.ToolRate{Key: "web_search", Provider: "openai", CustomerMicrousd: 250_000})
	store := &fakeAIUsageStore{}
	service := NewAIUsageService(catalog, store, fixedAIUsageEstimates{})
	_, err = service.Preflight(context.Background(), PreflightRequest{Metering: MeteringRequest{
		WorkspaceID: "ws", TaskNature: "support", Provider: "openai", Model: "gpt-5.6-luna", Route: "gpt-5.6-luna",
		FundingMode: aiusage.FundingHelpinHosted, InputTokensEstimate: 1, MaximumOutputTokens: 1,
		AllowedPaidTools: []string{"web_search"}, IdempotencyKey: "run:tool",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if store.reservation.ReservedMicrousd < 250_000 {
		t.Fatalf("reserved = %d, want paid-tool exposure included", store.reservation.ReservedMicrousd)
	}
}

func TestAIUsageServicePromotionalUsageDoesNotReserveOrCharge(t *testing.T) {
	store := &fakeAIUsageStore{}
	service := newTestAIUsageService(t, store)
	metering, err := service.Preflight(context.Background(), PreflightRequest{Metering: MeteringRequest{
		WorkspaceID: "ws", TaskNature: "setup", FeatureKey: "automation_setup", Provider: "openai",
		Model: "gpt-5.6-luna", Route: "gpt-5.6-luna", FundingMode: aiusage.FundingHelpinHosted,
		InputTokensEstimate: 1_000, MaximumOutputTokens: 100, IdempotencyKey: "setup:1", Promotional: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if store.reserveCalls != 0 || !metering.Promotional {
		t.Fatalf("reserve calls/context = %d/%#v", store.reserveCalls, metering)
	}
	result, err := service.Reconcile(context.Background(), CompletionUsage{
		Context: *metering, Telemetry: aiusage.TokenTelemetry{InputTokensTotal: 1_000, OutputTokens: 100}, MeasurementStatus: "actual",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ChargedMicrousd != 0 || store.uncharged.EntryKind != "promotional" || store.uncharged.PublishedChargeMicrousd == 0 {
		t.Fatalf("result/ledger = %#v/%#v", result, store.uncharged)
	}
}

func TestAIUsageServiceCapsStrictDefectAndFounderNeverCaps(t *testing.T) {
	for _, test := range []struct {
		name, mode string
		wantCharge int64
	}{
		{name: "strict", mode: model.AIUsageEnforcementStrict, wantCharge: 2},
		{name: "Founder soft", mode: model.AIUsageEnforcementSoft, wantCharge: 1_320_000},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeAIUsageStore{mode: test.mode}
			service := newTestAIUsageService(t, store)
			metering, err := service.Preflight(context.Background(), PreflightRequest{Metering: MeteringRequest{
				WorkspaceID: "ws", TaskNature: "support", Provider: "openai", Model: "gpt-5.6-luna", Route: "gpt-5.6-luna",
				FundingMode: aiusage.FundingHelpinHosted, InputTokensEstimate: 1, MaximumOutputTokens: 1, IdempotencyKey: "run:1",
			}})
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.Reconcile(context.Background(), CompletionUsage{
				Context: *metering, Telemetry: aiusage.TokenTelemetry{OutputTokens: 1_000_000}, MeasurementStatus: "actual",
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.ChargedMicrousd != test.wantCharge {
				t.Fatalf("charged = %d, want %d", result.ChargedMicrousd, test.wantCharge)
			}
			if test.mode == model.AIUsageEnforcementStrict && result.AbsorbedMicrousd != 1_319_998 {
				t.Fatalf("absorbed = %d, want 1319998", result.AbsorbedMicrousd)
			}
		})
	}
}

func TestAIUsageServiceFailReleasesReservation(t *testing.T) {
	store := &fakeAIUsageStore{}
	service := newTestAIUsageService(t, store)
	if err := service.Fail(context.Background(), "reservation"); err != nil {
		t.Fatal(err)
	}
	if store.releasedID != "reservation" {
		t.Fatalf("released ID = %q", store.releasedID)
	}
}

func TestAIUsageServiceMissingTelemetryUsesLaunchEstimate(t *testing.T) {
	store := &fakeAIUsageStore{mode: model.AIUsageEnforcementExtra}
	service := newTestAIUsageService(t, store)
	result, err := service.Reconcile(context.Background(), CompletionUsage{
		Context: MeteringContext{
			Route:         aiusage.ResolvedRoute{Tier: aiusage.TierSmall, Rates: aiusage.TokenRates{}},
			ReservationID: "reservation", WorkspaceID: "ws", TaskNature: "support", FeatureKey: "support_reply",
			FundingMode: aiusage.FundingHelpinHosted, IdempotencyKey: "missing:1", PricingVersion: "2026-08-13",
			EnforcementMode: model.AIUsageEnforcementExtra,
		},
		MeasurementStatus: "estimated",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ChargedMicrousd != 25_000 || store.reconcile.Entry.EntryKind != "estimate" || store.reconcile.Entry.EstimationMethod != "launch_fallback" {
		t.Fatalf("result/reconcile = %#v/%#v", result, store.reconcile)
	}
}

func newTestAIUsageService(t *testing.T, store *fakeAIUsageStore) *AIUsageService {
	t.Helper()
	catalog, err := aiusage.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	return NewAIUsageService(catalog, store, fixedAIUsageEstimates{})
}

type fakeAIUsageStore struct {
	mode         string
	reserveErr   error
	reserveCalls int
	reservation  repository.AIUsageReservationRequest
	reconcile    repository.AIUsageReconcileRequest
	checkpoint   repository.AIUsageCheckpointRequest
	checkpoints  int
	uncharged    model.AIUsageLedgerEntry
	releasedID   string
	resizeCalls  int
	resizedID    string
	resizedTo    int64
}

func (f *fakeAIUsageStore) Reserve(_ context.Context, input repository.AIUsageReservationRequest) (*model.AIUsageReservation, error) {
	f.reserveCalls++
	f.reservation = input
	if f.reserveErr != nil {
		return nil, f.reserveErr
	}
	mode := f.mode
	if mode == "" {
		mode = model.AIUsageEnforcementStrict
	}
	return &model.AIUsageReservation{ID: "reservation", ReservedMicrousd: input.ReservedMicrousd, EnforcementMode: mode}, nil
}

func (f *fakeAIUsageStore) Reconcile(_ context.Context, input repository.AIUsageReconcileRequest) (*model.AIUsagePeriod, error) {
	f.reconcile = input
	return &model.AIUsagePeriod{}, nil
}

func (f *fakeAIUsageStore) Checkpoint(_ context.Context, input repository.AIUsageCheckpointRequest) (*model.AIUsagePeriod, error) {
	f.checkpoints++
	f.checkpoint = input
	return &model.AIUsagePeriod{}, nil
}

func (f *fakeAIUsageStore) Release(_ context.Context, id, _ string) error {
	f.releasedID = id
	return nil
}

func (f *fakeAIUsageStore) ResizeReservation(_ context.Context, id string, target int64, _ time.Time) error {
	f.resizeCalls++
	f.resizedID = id
	f.resizedTo = target
	return nil
}

func (f *fakeAIUsageStore) RecordUncharged(_ context.Context, input model.AIUsageLedgerEntry) error {
	f.uncharged = input
	return nil
}

type fixedAIUsageEstimates struct{ p90 int64 }

func (f fixedAIUsageEstimates) P90Microusd(context.Context, string, aiusage.Tier, aiusage.FundingMode) (int64, bool, error) {
	return f.p90, f.p90 > 0, nil
}

func TestAIUsageServiceSuspendsAndRestoresInteractiveReservation(t *testing.T) {
	store := &fakeAIUsageStore{}
	usageService := newTestAIUsageService(t, store)
	metering := MeteringContext{ReservationID: "reservation", MaxBillableMicrousd: 42_000}

	if err := usageService.SuspendReservation(context.Background(), metering); err != nil {
		t.Fatal(err)
	}
	if store.resizeCalls != 1 || store.resizedID != "reservation" || store.resizedTo != 0 {
		t.Fatalf("suspended reservation = id %q target %d calls %d", store.resizedID, store.resizedTo, store.resizeCalls)
	}
	if err := usageService.Heartbeat(context.Background(), metering); err != nil {
		t.Fatal(err)
	}
	if store.resizeCalls != 2 || store.resizedTo != 42_000 {
		t.Fatalf("restored reservation target = %d calls %d", store.resizedTo, store.resizeCalls)
	}
}
