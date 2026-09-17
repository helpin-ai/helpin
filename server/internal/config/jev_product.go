package config

import (
	"os"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/decision"
)

func jevProductPolicies() map[string]decision.Policy {
	policies := map[string]decision.Policy{}
	for _, feature := range []string{"meeting_routing", "coverage_classification", "coverage_topic_matching", "automation_condition", "answer_evidence"} {
		prefix := "JEV_" + strings.ToUpper(feature)
		mode := strings.TrimSpace(os.Getenv(prefix + "_MODE"))
		if mode == "" {
			mode = "shadow"
		}
		policies[feature] = decision.Policy{Mode: mode, Threshold: parseJevProbability(os.Getenv(prefix+"_THRESHOLD"), .95), DailyLimit: parsePositiveIntEnv(os.Getenv(prefix+"_DAILY_LIMIT"), 1000)}
	}
	return policies
}
