package service

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/decision"
	"testing"
)

func TestIconSuggestionSelectionAndCache(t *testing.T) {
	svc, provider, usage, _ := setupJevDecisionTest(t, "primary")
	svc.policies[JevIconSelection] = decision.Policy{Mode: "primary", Threshold: .95, DailyLimit: 10}
	provider.choices["icon"] = "rocket"
	icons := NewIconSuggestionService(svc)
	for range 2 {
		icon, err := icons.Suggest(context.Background(), "workspace", "collection", "Getting Started")
		if err != nil || icon != "rocket" {
			t.Fatalf("icon=%q err=%v", icon, err)
		}
	}
	if provider.calls != 1 || len(usage.entries) != 1 {
		t.Fatalf("calls=%d usage=%d", provider.calls, len(usage.entries))
	}
}

func TestIconSuggestionFallbacks(t *testing.T) {
	for _, scenario := range []string{"uncertain", "none", "off", "shadow", "invalid", "blank"} {
		t.Run(scenario, func(t *testing.T) {
			svc, provider, _, _ := setupJevDecisionTest(t, "primary")
			policy := decision.Policy{Mode: "primary", Threshold: .95, DailyLimit: 10}
			provider.choices["icon"] = "rocket"
			name := "Getting Started"
			if scenario == "uncertain" {
				provider.probability = .6
			}
			if scenario == "none" {
				provider.choices["icon"] = "none"
			}
			if scenario == "off" || scenario == "shadow" {
				policy.Mode = scenario
			}
			if scenario == "invalid" {
				provider.choices["icon"] = "not-a-real-icon"
			}
			if scenario == "blank" {
				name = " "
			}
			svc.policies[JevIconSelection] = policy
			icon, _ := NewIconSuggestionService(svc).Suggest(context.Background(), "workspace", "collection", name)
			if icon != "" {
				t.Fatalf("unexpected icon %q", icon)
			}
			if (scenario == "off" || scenario == "blank") && provider.calls != 0 {
				t.Fatal("unexpected provider call")
			}
		})
	}
}

func TestIconSuggestionPolicySupported(t *testing.T) {
	svc, provider, usage, _ := setupJevDecisionTest(t, "primary")
	_, err := NewJevDecisionService(provider, svc.store, usage, map[string]decision.Policy{JevIconSelection: {Mode: "primary", Threshold: .95, DailyLimit: 10}}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestIconSuggestionCandidateCoverage(t *testing.T) {
	for _, name := range []string{"Billing & payments", "安全", "Getting Started", "SoMeThing unusual"} {
		choices := iconSuggestionChoices(name)
		for _, required := range []string{"none", "rocket", "credit-card", "shield", "customer-support", "code", "key01", "mail01"} {
			if choices[required] == "" {
				t.Fatalf("%q missing baseline candidate %q", name, required)
			}
		}
		if len(choices) > 48 {
			t.Fatal("unbounded shortlist")
		}
	}
}
