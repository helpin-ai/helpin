package service

func supportPlannerJSONSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"route", "reply", "intent", "subject", "language", "risk",
			"context_action", "issue_key", "issue_summary", "progress_signal",
			"standalone_query", "search_queries", "reason",
		},
		"properties": map[string]any{
			"route": map[string]any{
				"type": "string",
				"enum": []string{supportRouteConversational, supportDecisionAnswer, supportDecisionClarify, supportDecisionHandoff, supportDecisionConfirm},
			},
			"reply": map[string]any{"type": "string"},
			"intent": map[string]any{
				"type": "string",
				"enum": supportIntentIDs(),
			},
			"subject":  map[string]any{"type": "string", "maxLength": 120},
			"language": map[string]any{"type": "string", "maxLength": 16},
			"risk": map[string]any{
				"type": "string",
				"enum": []string{supportRiskCommercial, supportRiskGeneral},
			},
			"context_action": map[string]any{
				"type": "string",
				"enum": []string{"new_issue", "continue", "confirm_previous"},
			},
			"issue_key":        map[string]any{"type": "string"},
			"issue_summary":    map[string]any{"type": "string"},
			"progress_signal":  map[string]any{"type": "string", "enum": []string{supportProgressNewIssue, supportProgressSameNewInfo, supportProgressSameRepeat, supportProgressSameUnclear}},
			"standalone_query": map[string]any{"type": "string"},
			"search_queries": map[string]any{
				"type":     "array",
				"maxItems": 4,
				"items":    map[string]any{"type": "string"},
			},
			"reason": map[string]any{"type": "string"},
		},
	}
}
