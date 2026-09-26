//go:build ee

package pricing

import (
	"errors"
	"testing"
)

func TestLaunchCatalogValidates(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}

	if issues := catalog.Validate(); len(issues) != 0 {
		t.Fatalf("Validate() issues = %#v", issues)
	}
}

func TestCatalogResolveCanonicalizesRecognizedAlias(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}

	resolved, err := catalog.Resolve(
		"openrouter",
		"gpt-5.6-luna",
		"openai/gpt-5.6-luna",
		"standard",
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Tier != TierSmall {
		t.Errorf("Resolve() tier = %q, want %q", resolved.Tier, TierSmall)
	}
	if resolved.CanonicalModel != "gpt-5.6-luna" {
		t.Errorf("Resolve() canonical model = %q, want gpt-5.6-luna", resolved.CanonicalModel)
	}
	if resolved.Route != "openai/gpt-5.6-luna" {
		t.Errorf("Resolve() route = %q, want openai/gpt-5.6-luna", resolved.Route)
	}
}

func TestCatalogPricesGPT6LunaOpenRouterRoute(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	route, err := catalog.Resolve("openrouter", "gpt-6-luna", "openai/gpt-6-luna", "standard")
	if err != nil || route.Tier != TierSmall {
		t.Fatalf("GPT-6 Luna pricing route = %+v, %v", route, err)
	}
}

func TestCatalogResolvesAskMediaReaderRoute(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}

	resolved, err := catalog.Resolve(
		"openrouter",
		"google/gemini-3.8-flash",
		"google/gemini-3.8-flash",
		"standard",
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Tier != TierMedium {
		t.Errorf("Resolve() tier = %q, want %q", resolved.Tier, TierMedium)
	}
}

func TestCatalogResolvesGLM53FlashExactoRoute(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}

	resolved, err := catalog.Resolve(
		"openrouter",
		"z-ai/glm-5.3-flash:exacto",
		"z-ai/glm-5.3-flash:exacto",
		"standard",
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.CanonicalModel != "glm-5.3-flash" || resolved.Route != "z-ai/glm-5.3-flash:exacto" {
		t.Fatalf("Resolve() = %#v, want GLM 5.3 Flash Exacto", resolved)
	}
	if resolved.Tier != TierSmall || resolved.ContextWindow != 1_048_576 || resolved.MaximumOutput != 131_072 {
		t.Fatalf("Resolve() pricing identity = %#v", resolved)
	}
}

func TestCatalogResolveDefaultPrefersExactDeepSeekNitroRoute(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}

	resolved, err := catalog.ResolveDefault(
		"openrouter",
		"deepseek/deepseek-v4-flash-0731:nitro",
		"standard",
	)
	if err != nil {
		t.Fatalf("ResolveDefault() error = %v", err)
	}
	if resolved.CanonicalModel != "deepseek-v4-flash-0731" ||
		resolved.Route != "deepseek/deepseek-v4-flash-0731:nitro" {
		t.Fatalf("ResolveDefault() = %#v, want exact Nitro route", resolved)
	}
}

func TestCatalogResolvesDeepSeek41FlashRoutes(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"deepseek/deepseek-v4.1-flash", "deepseek/deepseek-v4.1-flash:nitro"} {
		resolved, err := catalog.ResolveDefault("openrouter", route, "standard")
		if err != nil {
			t.Fatal(err)
		}
		if resolved.CanonicalModel != "deepseek-v4.1-flash" || resolved.Route != route || resolved.Tier != TierSmall {
			t.Fatalf("unexpected routing or metering identity: %+v", resolved)
		}
		if resolved.Rates.InputMicrousdPerMillion != 330_000 || resolved.Rates.CacheReadMicrousdPerMillion != 33_000 || resolved.Rates.OutputMicrousdPerMillion != 1_320_000 {
			t.Fatalf("unexpected Small tariff: %+v", resolved.Rates)
		}
		if resolved.ContextWindow != 1_048_576 || resolved.MaximumOutput != 384_000 {
			t.Fatalf("unexpected model limits: %+v", resolved)
		}
	}
}

func TestCatalogResolvesClaudeSonnet5Route(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}

	resolved, err := catalog.Resolve("anthropic", "claude-sonnet-5", "claude-sonnet-5", "standard")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.CanonicalModel != "claude-sonnet-5" || resolved.Route != "claude-sonnet-5" {
		t.Fatalf("Resolve() = %#v, want Claude Sonnet 5", resolved)
	}
}

func TestCatalogResolvesClaudeSonnet5ThroughOpenRouter(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}

	resolved, err := catalog.Resolve("openrouter", "anthropic/claude-sonnet-5", "anthropic/claude-sonnet-5", "standard")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.CanonicalModel != "claude-sonnet-5" || resolved.Route != "anthropic/claude-sonnet-5" {
		t.Fatalf("Resolve() = %#v, want OpenRouter Claude Sonnet 5", resolved)
	}
}

func TestCatalogResolveRejectsUnapprovedRoute(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}

	_, err = catalog.Resolve(
		"openrouter",
		"gpt-5.6-luna",
		"fallback/unknown",
		"standard",
	)
	if !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("Resolve() error = %v, want ErrModelUnavailable", err)
	}
}

func TestPublicPricingUsesPercentageAllowanceDenominators(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}

	pricing := catalog.PublicSnapshot()
	if pricing.PricingVersion != "2026-09-26" {
		t.Errorf("pricing version = %q, want 2026-09-26", pricing.PricingVersion)
	}

	wantAllowances := map[string]int64{
		"starter:monthly": 99_000_000,
		"starter:annual":  79_000_000,
		"growth:monthly":  299_000_000,
		"growth:annual":   239_000_000,
		"growth:trial":    140_000_000,
		"founder:monthly": 150_000_000,
	}
	for _, plan := range pricing.Plans {
		key := plan.Plan + ":" + plan.BillingInterval
		want, ok := wantAllowances[key]
		if !ok {
			t.Errorf("unexpected public plan %q", key)
			continue
		}
		if plan.AllowanceMicrousd != want {
			t.Errorf("plan %q allowance = %d, want %d", key, plan.AllowanceMicrousd, want)
		}
		delete(wantAllowances, key)
	}
	if len(wantAllowances) != 0 {
		t.Errorf("missing public plans = %#v", wantAllowances)
	}
}

func TestCatalogValidateRejectsDuplicateExactRoute(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}
	catalog.Routes = append(catalog.Routes, catalog.Routes[0])

	issues := catalog.Validate()
	if !hasValidationCode(issues, "duplicate_route") {
		t.Fatalf("Validate() issues = %#v, want duplicate_route", issues)
	}
}

func hasValidationCode(issues []ValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
