package model

import "time"

type BillingCreditConsumption struct {
	WorkspaceID    string
	FeatureKey     string
	Credits        int
	IdempotencyKey string
	Metadata       map[string]any
	AllowOverage   bool
}

type BillingCreditPreflight struct {
	WorkspaceID string
	FeatureKey  string
	Credits     int
}

type BillingManagerRef struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

type BillingSummary struct {
	WorkspaceID              string     `json:"workspace_id"`
	Plan                     string     `json:"plan"`
	Status                   string     `json:"status"`
	BillingInterval          string     `json:"billing_interval"`
	Trialing                 bool       `json:"trialing"`
	TrialEndsAt              *time.Time `json:"trial_ends_at,omitempty"`
	CurrentPeriodStart       time.Time  `json:"current_period_start"`
	CurrentPeriodEnd         time.Time  `json:"current_period_end"`
	IncludedCredits          int        `json:"included_credits"`
	CreditsUsed              int        `json:"credits_used"`
	CreditsRemaining         int        `json:"credits_remaining"`
	NextChargeCents          int        `json:"next_charge_cents"`
	OnDemandEnabled          bool       `json:"on_demand_enabled"`
	OnDemandAvailable        bool       `json:"on_demand_available"`
	StripeCustomerID         *string    `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID     *string    `json:"stripe_subscription_id,omitempty"`
	PendingPlan              *string    `json:"pending_plan,omitempty"`
	PendingBillingInterval   *string    `json:"pending_billing_interval,omitempty"`
	PendingChangeAt          *time.Time `json:"pending_change_at,omitempty"`
	CancelAtPeriodEnd        bool       `json:"cancel_at_period_end"`
	CanceledAt               *time.Time `json:"canceled_at,omitempty"`
	Locked                   bool       `json:"locked"`
	BillingNoticeType        string     `json:"billing_notice_type,omitempty"`
	BillingNoticeMessage     string     `json:"billing_notice_message,omitempty"`
	BillingNoticeAt          *time.Time `json:"billing_notice_at,omitempty"`
	PaymentFailedAt          *time.Time `json:"payment_failed_at,omitempty"`
	TrialWillEndAt           *time.Time `json:"trial_will_end_at,omitempty"`
	ManageBillingEnabled     bool       `json:"manage_billing_enabled"`
	Warning                  string     `json:"warning,omitempty"`
	SeatLimit                int        `json:"seat_limit,omitempty"`
	SeatUsage                int        `json:"seat_usage,omitempty"`
	SeatOverLimit            bool       `json:"seat_over_limit,omitempty"`
	EntitlementWarning       string     `json:"entitlement_warning,omitempty"`
	OnDemandBlocksInvoiced   int        `json:"on_demand_blocks_invoiced"`
	AIUsageAllowanceMicrousd int64      `json:"ai_usage_allowance_microusd"`
	AIUsageUsedMicrousd      int64      `json:"ai_usage_used_microusd"`
	AIUsageRemainingMicrousd int64      `json:"ai_usage_remaining_microusd"`
	AIUsageReservedMicrousd  int64      `json:"ai_usage_reserved_microusd"`
	AIUsageOverageMicrousd   int64      `json:"ai_usage_overage_microusd"`
	AIUsagePeriodStart       time.Time  `json:"ai_usage_period_start"`
	AIUsagePeriodEnd         time.Time  `json:"ai_usage_period_end"`
	AIUsageUnlimited         bool       `json:"ai_usage_unlimited"`
	ExtraAIUsageEnabled      bool       `json:"extra_ai_usage_enabled"`
	ExtraAIUsageAvailable    bool       `json:"extra_ai_usage_available"`
	PricingVersion           string     `json:"pricing_version"`
	// BillingManagers lists the workspace owner(s) and org owner who can manage
	// billing. Populated only on the workspace billing page, not hot paths.
	BillingManagers []BillingManagerRef `json:"billing_managers,omitempty"`
}
