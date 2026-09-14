package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/ee/pricing"
	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	mediaEnrichmentProvider       = aiusage.MediaEnrichmentProvider
	mediaEnrichmentCanonicalModel = aiusage.MediaEnrichmentCanonicalModel
	mediaEnrichmentRoute          = aiusage.MediaEnrichmentRoute
)

// AIUsageStore is the transactional persistence needed by direct-call metering.
type AIUsageStore interface {
	Reserve(context.Context, eerepository.AIUsageReservationRequest) (*model.AIUsageReservation, error)
	Reconcile(context.Context, eerepository.AIUsageReconcileRequest) (*model.AIUsagePeriod, error)
	Checkpoint(context.Context, eerepository.AIUsageCheckpointRequest) (*model.AIUsagePeriod, error)
	Release(context.Context, string, string) error
	RecordUncharged(context.Context, model.AIUsageLedgerEntry) error
}

// AIUsageEstimateSource supplies an observed reservation P90 when reliable.
type AIUsageEstimateSource interface {
	P90Microusd(context.Context, string, aiusage.Tier, aiusage.FundingMode) (int64, bool, error)
}

// AIUsageService prices, reserves, and reconciles actual AI usage.
type AIUsageService struct {
	catalog   *pricing.Catalog
	store     AIUsageStore
	estimates AIUsageEstimateSource
}

// NewAIUsageService creates the token-priced AI usage service.
func NewAIUsageService(catalog *pricing.Catalog, store AIUsageStore, estimates AIUsageEstimateSource) *AIUsageService {
	return &AIUsageService{catalog: catalog, store: store, estimates: estimates}
}

// ResolveMeteringContext resolves an exact eligible route and enforces built-in task sizing.
func (s *AIUsageService) ResolveMeteringContext(input MeteringRequest) (MeteringContext, error) {
	if s == nil {
		return MeteringContext{}, model.ErrPricingConfigurationMissing
	}
	if input.FundingMode == aiusage.FundingCustomerFlat {
		return s.resolveFlatMeteringContext(input)
	}
	if s.catalog == nil {
		return MeteringContext{}, model.ErrPricingConfigurationMissing
	}
	var resolved aiusage.ResolvedRoute
	var err error
	if strings.TrimSpace(input.Route) == "" {
		resolved, err = s.catalog.ResolveDefault(input.Provider, input.Model, input.ServiceTier)
	} else {
		resolved, err = s.catalog.Resolve(input.Provider, input.Model, input.Route, input.ServiceTier)
	}
	if err != nil {
		switch {
		case errors.Is(err, pricing.ErrPricingConfigurationMissing):
			return MeteringContext{}, fmt.Errorf("%w: %v", model.ErrPricingConfigurationMissing, err)
		default:
			return MeteringContext{}, fmt.Errorf("%w: %v", model.ErrModelUnavailableUnderPricing, err)
		}
	}
	if err := validateAIUsageOperation(input.OperationKey, input.TaskNature, resolved); err != nil {
		return MeteringContext{}, err
	}
	funding := input.FundingMode
	if funding == "" {
		funding = aiusage.FundingHelpinHosted
	}
	return MeteringContext{
		Route: resolved, PricingVersion: s.catalog.PricingVersion, Promotional: input.Promotional,
		WorkspaceID: input.WorkspaceID, TaskNature: input.TaskNature, FeatureKey: input.FeatureKey, OperationKey: input.OperationKey,
		FundingMode: funding, IdempotencyKey: input.IdempotencyKey, ToolRates: s.snapshotToolRates(),
	}, nil
}

// Preflight reserves the greater of the observed P90 and deterministic maximum call charge.
func (s *AIUsageService) Preflight(ctx context.Context, input PreflightRequest) (*MeteringContext, error) {
	metering, err := s.ResolveMeteringContext(input.Metering)
	if err != nil {
		return nil, err
	}
	bound, err := s.deterministicBound(input.Metering, metering)
	if err != nil {
		return nil, err
	}
	if s.estimates != nil {
		p90, available, estimateErr := s.estimates.P90Microusd(ctx, input.Metering.TaskNature, metering.Route.Tier, metering.FundingMode)
		if estimateErr != nil {
			return nil, fmt.Errorf("load AI usage estimate: %w", estimateErr)
		}
		if available && p90 > bound {
			bound = p90
		}
	}
	metering.MaxBillableMicrousd = bound
	if metering.Promotional {
		return &metering, nil
	}
	if s.store == nil {
		return nil, model.ErrPricingConfigurationMissing
	}
	reservation, err := s.store.Reserve(ctx, eerepository.AIUsageReservationRequest{
		WorkspaceID: input.Metering.WorkspaceID, TaskNature: input.Metering.TaskNature,
		ModelTier: string(metering.Route.Tier), ExecutionID: input.Metering.ExecutionID,
		IdempotencyKey: input.Metering.IdempotencyKey, ReservedMicrousd: bound,
	})
	if err != nil {
		return nil, err
	}
	metering.ReservationID = reservation.ID
	metering.EnforcementMode = reservation.EnforcementMode
	return &metering, nil
}

// Reconcile converts actual telemetry into one immutable, idempotent ledger entry.
func (s *AIUsageService) Reconcile(ctx context.Context, input CompletionUsage) (*UsageResult, error) {
	entry, charged, absorbed, done, err := s.prepareCompletion(ctx, input)
	if err != nil {
		return nil, err
	}
	if done {
		return &UsageResult{ChargedMicrousd: charged, AbsorbedMicrousd: absorbed}, nil
	}
	if _, err := s.store.Reconcile(ctx, eerepository.AIUsageReconcileRequest{
		ReservationID: input.Context.ReservationID, Entry: entry,
		ChargedMicrousd: charged, AbsorbedMicrousd: absorbed,
		AllowLateUsage: input.AllowLateUsage, RunID: input.RunID, RunOutputSummary: input.RunOutputSummary,
	}); err != nil {
		return nil, err
	}
	return &UsageResult{ChargedMicrousd: charged, AbsorbedMicrousd: absorbed}, nil
}

// Checkpoint posts one interactive turn while retaining its reservation for
// later turns in the same long-lived run.
func (s *AIUsageService) Checkpoint(ctx context.Context, input CompletionUsage) (*UsageResult, error) {
	entry, charged, absorbed, done, err := s.prepareCompletion(ctx, input)
	if err != nil {
		return nil, err
	}
	if done {
		return &UsageResult{ChargedMicrousd: charged, AbsorbedMicrousd: absorbed}, nil
	}
	if _, err := s.store.Checkpoint(ctx, eerepository.AIUsageCheckpointRequest{
		ReservationID: input.Context.ReservationID, Entry: entry,
		ChargedMicrousd: charged, AbsorbedMicrousd: absorbed,
		RunID: input.RunID, RunOutputSummary: input.RunOutputSummary,
	}); err != nil {
		return nil, err
	}
	return &UsageResult{ChargedMicrousd: charged, AbsorbedMicrousd: absorbed}, nil
}

func (s *AIUsageService) prepareCompletion(ctx context.Context, input CompletionUsage) (model.AIUsageLedgerEntry, int64, int64, bool, error) {
	normalized, err := aiusage.NormalizeTokens(input.Telemetry)
	if err != nil {
		return model.AIUsageLedgerEntry{}, 0, 0, false, err
	}
	toolMicrousd, toolSnapshot, err := s.priceObservedTools(input.Context, input.PaidTools)
	if err != nil {
		return model.AIUsageLedgerEntry{}, 0, 0, false, err
	}
	charge, err := pricing.CalculateCharge(pricing.ChargeInput{
		FundingMode: input.Context.FundingMode, Tokens: normalized, Rates: input.Context.Route.Rates,
		PaidToolMicrousd: toolMicrousd,
	})
	if err != nil {
		return model.AIUsageLedgerEntry{}, 0, 0, false, err
	}
	if input.Context.FundingMode == aiusage.FundingCustomerFlat && input.CumulativeTelemetry != nil {
		charge, err = flatCheckpointCharge(input, toolMicrousd)
		if err != nil {
			return model.AIUsageLedgerEntry{}, 0, 0, false, err
		}
	}
	if input.MeasurementStatus == "estimated" && input.Context.FundingMode != aiusage.FundingCustomerFlat {
		fallback := launchEstimateMicrousd(input.Context.TaskNature, string(input.Context.Route.Tier))
		charge.PublishedEquivalentMicrousd = fallback
		charge.FinalMicrousd = fallback
		if input.Context.FundingMode == aiusage.FundingCustomer {
			charge.OrchestrationMicrousd = (fallback + 5) / 10
			charge.FinalMicrousd = charge.OrchestrationMicrousd
		}
	}
	entry := model.AIUsageLedgerEntry{
		WorkspaceID: input.Context.WorkspaceID, EntryKind: "usage", FeatureKey: input.Context.FeatureKey,
		Category: input.Context.TaskNature, ModelTier: string(input.Context.Route.Tier), Provider: input.Context.Route.Provider,
		CanonicalModel: input.Context.Route.CanonicalModel, Route: input.Context.Route.Route,
		ServiceTier: input.Context.Route.ServiceTier, FundingMode: string(input.Context.FundingMode),
		PricingVersion: input.Context.PricingVersion, RateSnapshot: model.JSONBlob(input.Context.Route.RateSnapshot),
		ToolUsage: model.JSONBlob(toolSnapshot), InputTokensTotal: normalized.InputTokensTotal,
		UncachedInputTokens: normalized.UncachedInputTokens, CacheReadTokens: normalized.CacheReadTokens,
		CacheWriteTokens: normalized.CacheWriteTokens, OutputTokens: normalized.OutputTokens,
		ReasoningTokens: normalized.ReasoningTokens, PublishedChargeMicrousd: charge.PublishedEquivalentMicrousd,
		MeasurementStatus: input.MeasurementStatus, IdempotencyKey: input.Context.IdempotencyKey + ":usage",
	}
	if input.MeasurementStatus == "estimated" {
		entry.EntryKind = "estimate"
		entry.EstimationMethod = "launch_fallback"
		if input.Context.FundingMode == aiusage.FundingCustomerFlat {
			entry.EstimationMethod = "estimated_tokens"
		}
	}
	if input.Context.Promotional {
		entry.EntryKind = "promotional"
		entry.FinalChargedMicrousd = 0
		if err := s.store.RecordUncharged(ctx, entry); err != nil {
			return model.AIUsageLedgerEntry{}, 0, 0, false, err
		}
		return entry, 0, 0, true, nil
	}

	charged := charge.FinalMicrousd
	absorbed := int64(0)
	if input.Context.EnforcementMode == model.AIUsageEnforcementStrict && charged > input.Context.MaxBillableMicrousd {
		absorbed = charged - input.Context.MaxBillableMicrousd
		charged = input.Context.MaxBillableMicrousd
	}
	entry.FinalChargedMicrousd = charged
	metadata, _ := json.Marshal(map[string]int64{"absorbed_microusd": absorbed})
	entry.Metadata = model.JSONBlob(metadata)
	return entry, charged, absorbed, false, nil
}

// Fail releases a reservation after provider failure.
func (s *AIUsageService) Fail(ctx context.Context, reservationID string) error {
	return s.store.Release(ctx, reservationID, "provider_failure")
}

// Release returns an active reservation without recording additional usage.
func (s *AIUsageService) Release(ctx context.Context, reservationID, reason string) error {
	if reservationID == "" {
		return nil
	}
	return s.store.Release(ctx, reservationID, reason)
}

// Heartbeat keeps a long-running reservation live without changing its bound.
func (s *AIUsageService) Heartbeat(ctx context.Context, metering MeteringContext) error {
	if metering.ReservationID == "" {
		return nil
	}
	resizer, ok := s.store.(interface {
		ResizeReservation(context.Context, string, int64, time.Time) error
	})
	if !ok {
		return nil
	}
	return resizer.ResizeReservation(ctx, metering.ReservationID, metering.MaxBillableMicrousd, time.Now().UTC())
}

// SuspendReservation releases the unused hold while an interactive run is
// waiting for its next human message. The reservation remains active so
// Heartbeat can restore its conservative bound before the next turn resumes.
func (s *AIUsageService) SuspendReservation(ctx context.Context, metering MeteringContext) error {
	if metering.ReservationID == "" {
		return nil
	}
	resizer, ok := s.store.(interface {
		ResizeReservation(context.Context, string, int64, time.Time) error
	})
	if !ok {
		return nil
	}
	return resizer.ResizeReservation(ctx, metering.ReservationID, 0, time.Now().UTC())
}

func (s *AIUsageService) deterministicBound(input MeteringRequest, metering MeteringContext) (int64, error) {
	if input.InputTokensEstimate < 0 || input.MaximumOutputTokens < 0 {
		return 0, fmt.Errorf("invalid AI usage token bound")
	}
	maximumOutput := input.MaximumOutputTokens
	if metering.Route.MaximumOutput > 0 && maximumOutput > metering.Route.MaximumOutput {
		maximumOutput = metering.Route.MaximumOutput
	}
	toolMicrousd, err := s.priceAllowedTools(metering, input.AllowedPaidTools)
	if err != nil {
		return 0, err
	}
	charge, err := pricing.CalculateCharge(pricing.ChargeInput{
		FundingMode: metering.FundingMode,
		Tokens:      aiusage.NormalizedTokens{InputTokensTotal: input.InputTokensEstimate, UncachedInputTokens: input.InputTokensEstimate, OutputTokens: maximumOutput},
		Rates:       metering.Route.Rates, PaidToolMicrousd: toolMicrousd,
	})
	if err != nil {
		return 0, err
	}
	return charge.FinalMicrousd, nil
}

func (s *AIUsageService) priceAllowedTools(metering MeteringContext, keys []string) (int64, error) {
	var total int64
	for _, key := range keys {
		rate, ok := s.acceptedToolRate(metering, key)
		if !ok || rate < 0 {
			return 0, fmt.Errorf("%w: paid tool %q", model.ErrPricingConfigurationMissing, key)
		}
		if total > math.MaxInt64-rate {
			return 0, pricing.ErrChargeOverflow
		}
		total += rate
	}
	return total, nil
}

func (s *AIUsageService) priceObservedTools(metering MeteringContext, tools []aiusage.PaidToolUsage) (int64, []byte, error) {
	var total int64
	for _, usage := range tools {
		rate, ok := s.acceptedToolRate(metering, usage.Key)
		if !ok || rate < 0 || usage.Count < 0 {
			return 0, nil, fmt.Errorf("%w: paid tool %q", model.ErrPricingConfigurationMissing, usage.Key)
		}
		if usage.Count != 0 && rate > math.MaxInt64/usage.Count {
			return 0, nil, pricing.ErrChargeOverflow
		}
		component := rate * usage.Count
		if total > math.MaxInt64-component {
			return 0, nil, pricing.ErrChargeOverflow
		}
		total += component
	}
	snapshot, err := json.Marshal(tools)
	return total, snapshot, err
}

func (s *AIUsageService) acceptedToolRate(metering MeteringContext, key string) (int64, bool) {
	if metering.ToolRates != nil {
		rate, ok := metering.ToolRates[strings.ToLower(strings.TrimSpace(key))]
		return rate, ok
	}
	// Only historical contexts predate immutable tool tariffs.
	if s.catalog == nil {
		return 0, false
	}
	for _, tool := range s.catalog.Tools {
		if strings.EqualFold(strings.TrimSpace(tool.Key), strings.TrimSpace(key)) {
			return tool.CustomerMicrousd, true
		}
	}
	return 0, false
}

func validateAIUsageOperation(operationKey, taskNature string, resolved aiusage.ResolvedRoute) error {
	switch strings.ToLower(strings.TrimSpace(operationKey)) {
	case "":
		return nil
	case AIUsageOperationMediaEnrichment:
		if resolved.Provider != mediaEnrichmentProvider || resolved.CanonicalModel != mediaEnrichmentCanonicalModel || resolved.Route != mediaEnrichmentRoute || resolved.Tier != aiusage.TierMedium {
			return fmt.Errorf("%w: %s requires the approved multimodal reader", model.ErrModelUnavailableUnderPricing, operationKey)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown AI operation %q", model.ErrPricingConfigurationMissing, operationKey)
	}
}

var _ AIUsageLifecycle = (*AIUsageService)(nil)

func (s *AIUsageService) ChargeForTokens(metering MeteringContext, tokens aiusage.NormalizedTokens) (int64, error) {
	charge, err := pricing.CalculateCharge(pricing.ChargeInput{FundingMode: metering.FundingMode, Tokens: tokens, Rates: metering.Route.Rates})
	return charge.FinalMicrousd, err
}
