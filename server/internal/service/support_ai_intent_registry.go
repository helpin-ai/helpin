package service

import "strings"

const (
	supportIntentRegistryVersion = 1

	supportIntentPricingGeneral     = "pricing_general"
	supportIntentPlanRecommendation = "plan_recommendation"
	supportIntentBillingTax         = "billing_tax"
	supportIntentUnknown            = "unknown"

	supportEvidenceModeSufficiency = "sufficiency"

	supportRiskCommercial = "time_sensitive_commercial"
	supportRiskGeneral    = "general"
)

// SupportIntentDefinition is the server-owned evidence contract for one
// support intent. The LLM selects an intent; it cannot invent field IDs.
type SupportIntentDefinition struct {
	ID               string
	Version          int
	EvidenceMode     string
	Risk             string
	RequiredEvidence []string
}

var supportIntentRegistryV1 = map[string]SupportIntentDefinition{
	supportIntentPricingGeneral: {
		ID: supportIntentPricingGeneral, Version: supportIntentRegistryVersion,
		EvidenceMode: supportEvidenceModeSufficiency, Risk: supportRiskCommercial,
	},
	supportIntentPlanRecommendation: {
		ID: supportIntentPlanRecommendation, Version: supportIntentRegistryVersion,
		EvidenceMode: supportEvidenceModeSufficiency, Risk: supportRiskCommercial,
	},
	supportIntentBillingTax: {
		ID: supportIntentBillingTax, Version: supportIntentRegistryVersion,
		EvidenceMode: supportEvidenceModeSufficiency, Risk: supportRiskCommercial,
	},
	supportIntentUnknown: {
		ID:               supportIntentUnknown,
		Version:          supportIntentRegistryVersion,
		EvidenceMode:     supportEvidenceModeSufficiency,
		Risk:             supportRiskGeneral,
		RequiredEvidence: []string{},
	},
}

func supportIntentDefinition(intent string) SupportIntentDefinition {
	normalized := strings.ToLower(strings.TrimSpace(intent))
	definition, ok := supportIntentRegistryV1[normalized]
	if !ok {
		definition = supportIntentRegistryV1[supportIntentUnknown]
	}
	definition.RequiredEvidence = cloneStringSlice(definition.RequiredEvidence)
	return definition
}
