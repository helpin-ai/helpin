package model

import "errors"

var (
	ErrBillingWorkspaceLocked          = errors.New("workspace is locked; choose a plan to reactivate it")
	ErrAIUsageExhausted                = errors.New("AI usage exhausted")
	ErrExtraAIUsageUnavailable         = errors.New("extra AI usage is not available")
	ErrExtraAIUsageBillingUnconfigured = errors.New("extra AI usage billing is not configured")
)
