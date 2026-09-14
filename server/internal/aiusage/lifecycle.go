package aiusage

import "context"

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
	FundingMode                                                                            FundingMode
	InputTokensEstimate, MaximumOutputTokens                                               int64
	AllowedPaidTools                                                                       []string
	ExecutionID, IdempotencyKey                                                            string
	Promotional                                                                            bool
	FlatTariff                                                                             *FlatTokenTariff
}

// MeteringContext carries the immutable usage and optional financial policy.
type MeteringContext struct {
	PolicyMode                                        string
	Route                                             ResolvedRoute
	ReservationID, PricingVersion                     string
	MaxBillableMicrousd                               int64
	Promotional                                       bool
	WorkspaceID, TaskNature, FeatureKey, OperationKey string
	FundingMode                                       FundingMode
	IdempotencyKey, EnforcementMode                   string
	FlatTariff                                        *FlatTokenTariff
	// Nil identifies a historical context; new contexts freeze even an empty map.
	ToolRates map[string]int64
}

// AIUsageOperationMediaEnrichment identifies the separately governed media pass.
const AIUsageOperationMediaEnrichment = "media_enrichment"

// PreflightRequest contains one execution's metering request.
type PreflightRequest struct{ Metering MeteringRequest }

// CompletionUsage contains terminal provider telemetry.
type CompletionUsage struct {
	Context           MeteringContext
	Telemetry         TokenTelemetry
	PaidTools         []PaidToolUsage
	MeasurementStatus string
	RunID             string
	RunOutputSummary  []byte
	AllowLateUsage    bool
	// Durable checkpoints supply both to round cumulative flat fees once,
	// independent of how many interruptions split the execution.
	PreviousTelemetry   *TokenTelemetry
	CumulativeTelemetry *TokenTelemetry
}

// UsageResult reports customer-charged and internally absorbed value.
type UsageResult struct {
	ChargedMicrousd, AbsorbedMicrousd int64
}
