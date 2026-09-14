package service

import "github.com/helpin-ai/helpin/server/internal/model"

type BillingCreditConsumption = model.BillingCreditConsumption
type BillingCreditPreflight = model.BillingCreditPreflight
type BillingManagerRef = model.BillingManagerRef
type BillingSummary = model.BillingSummary

const (
	BillingFeatureSupportAIReply = "support_ai_reply"
	BillingFeatureCRMAction      = "crm_action"
	BillingFeatureDocsGeneration = "docs_generation"
	BillingFeaturePlanningRun    = "planning_run"
	BillingFeatureCodingRun      = "coding_run"
)
