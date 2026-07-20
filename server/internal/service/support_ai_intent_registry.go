package service

import "strings"

const (
	supportIntentRegistryVersion = 1

	supportIntentPricingGeneral     = "pricing_general"
	supportIntentPlanRecommendation = "plan_recommendation"
	supportIntentBillingTax         = "billing_tax"
	supportIntentUnknown            = "unknown"

	supportEvidenceModeSlots       = "slots"
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
		ID:           supportIntentPricingGeneral,
		Version:      supportIntentRegistryVersion,
		EvidenceMode: supportEvidenceModeSlots,
		Risk:         supportRiskCommercial,
		RequiredEvidence: []string{
			"plan_names",
			"starting_prices",
			"billing_cadence",
			"enterprise_status",
			"canonical_url",
		},
	},
	supportIntentPlanRecommendation: {
		ID:           supportIntentPlanRecommendation,
		Version:      supportIntentRegistryVersion,
		EvidenceMode: supportEvidenceModeSlots,
		Risk:         supportRiskCommercial,
		RequiredEvidence: []string{
			"customer_needs",
			"recommended_plan",
			"recommendation_basis",
			"applicable_limits",
			"canonical_url",
		},
	},
	supportIntentBillingTax: {
		ID:           supportIntentBillingTax,
		Version:      supportIntentRegistryVersion,
		EvidenceMode: supportEvidenceModeSlots,
		Risk:         supportRiskCommercial,
		RequiredEvidence: []string{
			"advertised_price_tax_status",
			"tax_location_basis",
			"checkout_total_qualifier",
			"canonical_url",
		},
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

func normalizeSupportIntent(intent string, proposedEvidence []string) SupportIntentDefinition {
	definition := supportIntentDefinition(intent)
	if definition.ID == supportIntentUnknown {
		return definition
	}

	allowed := make(map[string]struct{}, len(definition.RequiredEvidence))
	for _, fieldID := range definition.RequiredEvidence {
		allowed[fieldID] = struct{}{}
	}
	for _, fieldID := range proposedEvidence {
		fieldID = strings.ToLower(strings.TrimSpace(fieldID))
		if fieldID == "" {
			continue
		}
		if _, ok := allowed[fieldID]; !ok {
			return supportIntentDefinition(supportIntentUnknown)
		}
	}
	return definition
}

func supportIntentIDs() []string {
	return []string{
		supportIntentPricingGeneral,
		supportIntentPlanRecommendation,
		supportIntentBillingTax,
		supportIntentUnknown,
	}
}

func supportEvidenceFieldIDs() []string {
	seen := map[string]struct{}{}
	fields := []string{}
	for _, intent := range supportIntentIDs() {
		for _, fieldID := range supportIntentRegistryV1[intent].RequiredEvidence {
			if _, ok := seen[fieldID]; ok {
				continue
			}
			seen[fieldID] = struct{}{}
			fields = append(fields, fieldID)
		}
	}
	return fields
}
