package aipolicy

import "strings"

type UsageFeatureDefinition struct {
	FeatureKey string
	Label      string
	Category   string
	Chargeable bool
}

func UsageFeatures(registry *Registry) map[string]UsageFeatureDefinition {
	features := make(map[string]UsageFeatureDefinition)
	for _, action := range registry.Actions() {
		_, exists := features[action.FeatureKey]
		// The feature-level action owns the customer-facing label/category.
		// Specialized sub-actions only supply execution policy and audit detail.
		if exists && !strings.HasPrefix(action.Key, "feature.") {
			continue
		}
		features[action.FeatureKey] = UsageFeatureDefinition{
			FeatureKey: action.FeatureKey,
			Label:      action.Label,
			Category:   string(action.Category),
			Chargeable: action.Chargeable,
		}
	}
	return features
}
