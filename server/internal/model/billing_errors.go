package model

import "errors"

var (
	ErrBillingWorkspaceLocked          = errors.New("workspace is locked; choose a plan to reactivate it")
	ErrAIAllowanceExhausted            = errors.New("AI allowance exhausted")
	ErrAIUsageExhausted                = ErrAIAllowanceExhausted
	ErrExtraAIUsageDisabled            = errors.New("extra AI usage disabled")
	ErrExtraAIUsageUnavailable         = errors.New("extra AI usage is not available")
	ErrExtraAIUsageBillingUnconfigured = errors.New("extra AI usage billing is not configured")
	ErrModelUnavailableUnderPricing    = errors.New("model unavailable under current pricing")
	ErrPricingConfigurationMissing     = errors.New("pricing configuration missing")
	ErrExecutionStoppedBeforeAllowance = errors.New("execution stopped before exceeding allowance")
	ErrUsageSettlementFailed           = errors.New("usage settlement pending or payment failed")
)
