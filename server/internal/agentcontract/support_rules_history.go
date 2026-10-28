package agentcontract

import (
	"embed"
	"io/fs"
	"strings"
)

// supportRulesHistory holds each shipped earlier version of supportRequiredRules,
// named by the date it shipped. Add the outgoing version here whenever the
// rules change: saved prompts copied from an older default still contain it.
//
//go:embed support_rules_history/*.md
var supportRulesHistory embed.FS

// replacePreviousSupportRules swaps an earlier shipped copy of the support rules
// in a saved prompt for the current rules, so the agent never receives two
// conflicting versions. Text outside the rules is kept as it is.
func replacePreviousSupportRules(prompt string) string {
	entries, err := fs.ReadDir(supportRulesHistory, "support_rules_history")
	if err != nil {
		return prompt
	}
	for _, entry := range entries {
		raw, err := supportRulesHistory.ReadFile("support_rules_history/" + entry.Name())
		if err != nil {
			continue
		}
		previous := strings.TrimSpace(string(raw))
		if previous == "" || previous == supportRequiredRules || !strings.Contains(prompt, previous) {
			continue
		}
		replacement := supportRequiredRules
		if strings.Contains(prompt, supportRequiredRules) {
			replacement = ""
		}
		prompt = strings.Replace(prompt, previous, replacement, 1)
		prompt = strings.ReplaceAll(prompt, previous, "")
	}
	return prompt
}
