package config

import "testing"

func TestJevProductPoliciesDefaultToIndependentShadow(t *testing.T) {
	for _, prefix := range []string{"JEV_MEETING_ROUTING", "JEV_COVERAGE_CLASSIFICATION", "JEV_COVERAGE_TOPIC_MATCHING", "JEV_AUTOMATION_CONDITION", "JEV_ANSWER_EVIDENCE"} {
		t.Setenv(prefix+"_MODE", "")
		t.Setenv(prefix+"_THRESHOLD", "")
		t.Setenv(prefix+"_DAILY_LIMIT", "")
	}
	t.Setenv("JEV_MEETING_ROUTING_MODE", "primary")
	t.Setenv("JEV_MEETING_ROUTING_THRESHOLD", "0.98")
	t.Setenv("JEV_MEETING_ROUTING_DAILY_LIMIT", "75")
	policies := jevProductPolicies()
	if len(policies) != 5 {
		t.Fatal("missing feature policy")
	}
	for name, policy := range policies {
		if err := policy.Validate(); err != nil {
			t.Fatal(err)
		}
		if name == "meeting_routing" {
			if policy.Mode != "primary" || policy.Threshold != .98 || policy.DailyLimit != 75 {
				t.Fatalf("meeting override lost: %+v", policy)
			}
		} else if policy.Mode != "shadow" || policy.Threshold != .95 || policy.DailyLimit != 1000 {
			t.Fatalf("independent default changed: %+v", policy)
		}
	}
}
