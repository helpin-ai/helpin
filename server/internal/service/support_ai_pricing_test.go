package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aiusage"
 "github.com/helpin-ai/helpin/server/ee/pricing"
)

func TestDirectSupportModelsMatchSmallPricingTier(t *testing.T) {
	catalog, err := pricing.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	models := []struct {
		name, provider, model string
	}{
		{name: "rewrite", provider: supportRewriteProvider, model: supportRewriteModel},
		{name: "triage", provider: supportTriageProvider, model: supportTriageModel},
	}
	for _, candidate := range models {
		t.Run(candidate.name, func(t *testing.T) {
			resolved, err := catalog.ResolveDefault(candidate.provider, candidate.model, "standard")
			if err != nil {
				t.Fatalf("resolve support model: %v", err)
			}
			if resolved.Tier != aiusage.TierSmall {
				t.Fatalf("tier = %q, want %q", resolved.Tier, aiusage.TierSmall)
			}
		})
	}
}
