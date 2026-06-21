package model

import "time"

const (
	BillingPlanFree    = "free"
	BillingPlanStarter = "starter"
	BillingPlanGrowth  = "growth"

	BillingStatusTrialing = "trialing"
	BillingStatusActive   = "active"
	BillingStatusPastDue  = "past_due"
	BillingStatusCanceled = "canceled"

	BillingLedgerKindUsage        = "usage"
	BillingLedgerKindReset        = "reset"
	BillingLedgerKindPlanChange   = "plan_change"
	BillingLedgerKindOnDemandBill = "on_demand_bill"

	// BillingRelationBillingManager is the authorization_relations relation used
	// to delegate per-workspace billing management to a non-owner member.
	BillingRelationBillingManager = "billing_manager"
)

type WorkspaceBilling struct {
	ID                     string     `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID            string     `json:"workspace_id" gorm:"type:uuid;not null;unique"`
	Plan                   string     `json:"plan" gorm:"not null;default:'free';index:idx_workspace_billing_plan"`
	Status                 string     `json:"status" gorm:"not null;default:'active';index:idx_workspace_billing_status"`
	StripeCustomerID       *string    `json:"stripe_customer_id,omitempty" gorm:"index:idx_workspace_billing_stripe_customer"`
	StripeSubscriptionID   *string    `json:"stripe_subscription_id,omitempty" gorm:"index:idx_workspace_billing_stripe_subscription"`
	StripePriceID          *string    `json:"stripe_price_id,omitempty"`
	BillingInterval        string     `json:"billing_interval" gorm:"not null;default:'monthly'"`
	IncludedCredits        int        `json:"included_credits" gorm:"not null;default:1000"`
	CreditsUsed            int        `json:"credits_used" gorm:"not null;default:0"`
	OnDemandEnabled        bool       `json:"on_demand_enabled" gorm:"not null;default:false"`
	OnDemandBlocksInvoiced int        `json:"on_demand_blocks_invoiced" gorm:"not null;default:0"`
	CurrentPeriodStart     time.Time  `json:"current_period_start" gorm:"not null"`
	CurrentPeriodEnd       time.Time  `json:"current_period_end" gorm:"not null"`
	TrialEndsAt            *time.Time `json:"trial_ends_at,omitempty"`
	PendingPlan            *string    `json:"pending_plan,omitempty"`
	PendingBillingInterval *string    `json:"pending_billing_interval,omitempty"`
	PendingChangeAt        *time.Time `json:"pending_change_at,omitempty"`
	CancelAtPeriodEnd      bool       `json:"cancel_at_period_end" gorm:"not null;default:false"`
	CanceledAt             *time.Time `json:"canceled_at,omitempty"`
	BillingNoticeType      *string    `json:"billing_notice_type,omitempty"`
	BillingNoticeMessage   *string    `json:"billing_notice_message,omitempty"`
	BillingNoticeAt        *time.Time `json:"billing_notice_at,omitempty"`
	PaymentFailedAt        *time.Time `json:"payment_failed_at,omitempty"`
	TrialWillEndAt         *time.Time `json:"trial_will_end_at,omitempty"`
	LastStripeEventID      *string    `json:"last_stripe_event_id,omitempty"`
	// PaymentMethodID links this workspace to a saved org card. Null means the
	// org default card is used.
	PaymentMethodID *string `json:"payment_method_id,omitempty" gorm:"type:uuid;index:idx_workspace_billing_payment_method"`
	// BillingOwnerUserID is the delegated billing owner for this workspace, if any.
	BillingOwnerUserID *string   `json:"billing_owner_user_id,omitempty" gorm:"type:uuid"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (WorkspaceBilling) TableName() string { return "workspace_billing" }

// OrganizationBilling holds the single Stripe Customer per organization that
// owns saved cards and workspace subscriptions.
type OrganizationBilling struct {
	ID                     string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrganizationID         string    `json:"organization_id" gorm:"type:uuid;not null;unique"`
	StripeCustomerID       *string   `json:"stripe_customer_id,omitempty"`
	DefaultPaymentMethodID *string   `json:"default_payment_method_id,omitempty" gorm:"type:uuid"`
	CreatedAt              time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt              time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (OrganizationBilling) TableName() string { return "organization_billing" }

// BillingPaymentMethod is a first-class saved card linked to an organization.
type BillingPaymentMethod struct {
	ID                    string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrganizationID        string    `json:"organization_id" gorm:"type:uuid;not null;index:idx_billing_payment_method_org"`
	StripePaymentMethodID string    `json:"stripe_payment_method_id" gorm:"not null"`
	Brand                 string    `json:"brand"`
	Last4                 string    `json:"last4"`
	ExpMonth              int       `json:"exp_month"`
	ExpYear               int       `json:"exp_year"`
	Cardholder            string    `json:"cardholder"`
	IsOrgDefault          bool      `json:"is_org_default" gorm:"not null;default:false"`
	BillingDetails        JSONBlob  `json:"billing_details" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt             time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (BillingPaymentMethod) TableName() string { return "billing_payment_method" }

// PaymentMethodSummary is a card representation returned to clients, including
// the count of workspaces linked to it.
type PaymentMethodSummary struct {
	ID               string `json:"id"`
	OrganizationID   string `json:"organization_id"`
	Brand            string `json:"brand"`
	Last4            string `json:"last4"`
	ExpMonth         int    `json:"exp_month"`
	ExpYear          int    `json:"exp_year"`
	Cardholder       string `json:"cardholder"`
	IsOrgDefault     bool   `json:"is_org_default"`
	LinkedWorkspaces int    `json:"linked_workspaces"`
	StripePaymentID  string `json:"stripe_payment_method_id"`
}

// UpdatePaymentMethodRequest is the payload for PUT cards/{cardId}.
type UpdatePaymentMethodRequest struct {
	Cardholder   *string `json:"cardholder"`
	ExpMonth     *int    `json:"exp_month"`
	ExpYear      *int    `json:"exp_year"`
	SetAsDefault *bool   `json:"set_as_default"`
}

// LinkPaymentMethodRequest is the payload for PUT workspaces/{id}/billing/payment-method.
type LinkPaymentMethodRequest struct {
	PaymentMethodID *string `json:"payment_method_id"`
}

// SetBillingOwnerRequest is the payload for PUT workspaces/{id}/billing/owner.
type SetBillingOwnerRequest struct {
	UserID *string `json:"user_id"`
}

type BillingCreditLedgerEntry struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	WorkspaceID    string    `json:"workspace_id" gorm:"type:uuid;not null;index"`
	Kind           string    `json:"kind" gorm:"not null;index"`
	FeatureKey     string    `json:"feature_key" gorm:"not null;default:'';index"`
	Credits        int       `json:"credits" gorm:"not null"`
	IdempotencyKey string    `json:"idempotency_key" gorm:"not null;unique"`
	Metadata       JSONBlob  `json:"metadata" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (BillingCreditLedgerEntry) TableName() string { return "billing_credit_ledger" }

type StripeWebhookEvent struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Type      string    `json:"type" gorm:"not null;index"`
	Processed bool      `json:"processed" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (StripeWebhookEvent) TableName() string { return "stripe_webhook_events" }
