package model

import "time"

const (
	AIUsagePeriodOpen            = "open"
	AIUsagePeriodClosed          = "closed"
	AIUsageEnforcementStrict     = "enforced"
	AIUsageEnforcementExtra      = "extra_allowed"
	AIUsageEnforcementSoft       = "soft"
	AIUsageReservationActive     = "active"
	AIUsageReservationReconciled = "reconciled"
	AIUsageReservationReleased   = "released"
	AIUsageSettlementPending     = "pending"
)

type AIUsagePeriod struct {
	ID, WorkspaceID                                                    string
	PeriodStart, PeriodEnd                                             time.Time
	AllowanceMicrousd, UsedMicrousd, OverageMicrousd, ReservedMicrousd int64
	EnforcementMode, Status, PricingVersion                            string
	CreatedAt, UpdatedAt                                               time.Time
}

func (AIUsagePeriod) TableName() string { return "billing_ai_usage_periods" }

type AIUsageLedgerEntry struct {
	ID, WorkspaceID, PeriodID, EntryKind, FeatureKey, Category, ModelTier        string
	Provider, CanonicalModel, Route, ServiceTier, FundingMode, PricingVersion    string
	RateSnapshot, ToolUsage, Metadata                                            JSONBlob
	InputTokensTotal, UncachedInputTokens, CacheReadTokens, CacheWriteTokens     int64
	OutputTokens, ReasoningTokens, PublishedChargeMicrousd, FinalChargedMicrousd int64
	MeasurementStatus, EstimationMethod, IdempotencyKey                          string
	SourceSampleSize                                                             int
	CreatedAt                                                                    time.Time
}

func (AIUsageLedgerEntry) TableName() string { return "billing_ai_usage_ledger" }

type AIUsageReservation struct {
	ID, WorkspaceID, PeriodID, TaskNature, ModelTier, ExecutionID, IdempotencyKey string
	ReservedMicrousd, ConsumedMicrousd                                            int64
	Status                                                                        string
	EnforcementMode                                                               string `gorm:"-"`
	ExpiresAt, HeartbeatAt, CreatedAt, UpdatedAt                                  time.Time
}

func (AIUsageReservation) TableName() string { return "billing_ai_usage_reservations" }

type AIUsageSettlement struct {
	ID, WorkspaceID, PeriodID                                                    string
	ExactOverageMicrousd, RoundedInvoiceCents, RoundingAdjustmentMicrousd        int64
	StripeCustomerID, StripeSubscriptionID, StripeInvoiceItemID, StripeInvoiceID *string
	IdempotencyKey, Status                                                       string
	AttemptCount                                                                 int
	LastError                                                                    *string
	CreatedAt, UpdatedAt                                                         time.Time
	PricingVersion, BillingInterval                                              string    `gorm:"->;-:migration"`
	PeriodStart, PeriodEnd                                                       time.Time `gorm:"->;-:migration"`
}

func (AIUsageSettlement) TableName() string { return "billing_ai_usage_settlements" }

type AIUsageTaskEstimate struct {
	ID, TaskNature, ModelTier, FundingMode, PricingVersion      string
	SampleSize                                                  int
	AverageTokens                                               JSONBlob
	AverageChargeMicrousd, P50ChargeMicrousd, P90ChargeMicrousd int64
	CalculatedAt, CreatedAt, UpdatedAt                          time.Time
}

func (AIUsageTaskEstimate) TableName() string { return "billing_ai_usage_task_estimates" }
