package decision

import (
	"math"
	"testing"
)

func TestProductDecisionPolicyValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy Policy
		valid  bool
	}{
		{name: "primary", policy: Policy{Mode: "primary", Threshold: .95, DailyLimit: 10}, valid: true},
		{name: "unknown mode", policy: Policy{Mode: "enabled", Threshold: .95, DailyLimit: 10}},
		{name: "invalid threshold", policy: Policy{Mode: "shadow", Threshold: math.NaN(), DailyLimit: 10}},
		{name: "unbounded budget", policy: Policy{Mode: "primary", Threshold: .95, DailyLimit: 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if (tc.policy.Validate() == nil) != tc.valid {
				t.Fatal("unexpected validation result")
			}
		})
	}
}
func TestProductDecisionRejectsMalformedAnswers(t *testing.T) {
	questions := map[string]Question{"supported": {Instructions: "Assess evidence", Choices: map[string]string{"yes": "Supported", "no": "Unsupported"}}}
	for _, scenario := range []string{"valid", "missing question", "unknown choice", "nonfinite probability", "invalid sum", "not maximum", "negative usage"} {
		t.Run(scenario, func(t *testing.T) {
			result := &Result{Model: Model, Answers: map[string]Answer{"supported": {Choice: "yes", Probabilities: map[string]float64{"yes": .99, "no": .01}}}}
			answer := result.Answers["supported"]
			switch scenario {
			case "missing question":
				delete(result.Answers, "supported")
			case "unknown choice":
				answer.Choice = "invented"
				result.Answers["supported"] = answer
			case "nonfinite probability":
				answer.Probabilities["yes"] = math.NaN()
			case "invalid sum":
				answer.Probabilities["no"] = .5
			case "not maximum":
				answer.Probabilities["no"] = .9
				answer.Probabilities["yes"] = .1
			case "negative usage":
				result.InputTokens = -1
			}
			if (ValidateResult(result, questions) == nil) != (scenario == "valid") {
				t.Fatal("malformed result was accepted or valid result rejected")
			}
		})
	}
}
