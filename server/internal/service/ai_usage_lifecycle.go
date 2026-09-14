package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// AIUsageLifecycle preserves admission, durable usage accounting, and recovery
// independently of the edition's financial policy. A community implementation
// records telemetry without financial reservations or settlement.
type AIUsageLifecycle interface {
	ResolveMeteringContext(MeteringRequest) (MeteringContext, error)
	Preflight(context.Context, PreflightRequest) (*MeteringContext, error)
	Checkpoint(context.Context, CompletionUsage) (*UsageResult, error)
	Reconcile(context.Context, CompletionUsage) (*UsageResult, error)
	Heartbeat(context.Context, MeteringContext) error
	SuspendReservation(context.Context, MeteringContext) error
	Fail(context.Context, string) error
	Release(context.Context, string, string) error
}

// MeteringRequest identifies and bounds one model execution.
type MeteringRequest struct {
	WorkspaceID, TaskNature, FeatureKey, OperationKey, Provider, Model, Route, ServiceTier string
	FundingMode                                                                            aiusage.FundingMode
	InputTokensEstimate, MaximumOutputTokens                                               int64
	AllowedPaidTools                                                                       []string
	ExecutionID, IdempotencyKey                                                            string
	Promotional                                                                            bool
}

// MeteringContext carries the immutable usage and optional financial policy.
type MeteringContext struct {
	PolicyMode                                        string
	Route                                             aiusage.ResolvedRoute
	ReservationID, PricingVersion                     string
	MaxBillableMicrousd                               int64
	Promotional                                       bool
	WorkspaceID, TaskNature, FeatureKey, OperationKey string
	FundingMode                                       aiusage.FundingMode
	IdempotencyKey, EnforcementMode                   string
}

// AIUsageOperationMediaEnrichment identifies the separately governed media pass.
const AIUsageOperationMediaEnrichment = "media_enrichment"

// PreflightRequest contains one execution's metering request.
type PreflightRequest struct{ Metering MeteringRequest }

// CompletionUsage contains terminal provider telemetry.
type CompletionUsage struct {
	Context           MeteringContext
	Telemetry         aiusage.TokenTelemetry
	PaidTools         []aiusage.PaidToolUsage
	MeasurementStatus string
	RunID             string
	RunOutputSummary  model.JSONBlob
	AllowLateUsage    bool
}

// UsageResult reports customer-charged and internally absorbed value.
type UsageResult struct {
	ChargedMicrousd, AbsorbedMicrousd int64
}
