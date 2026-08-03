package service

// SupportQueryPlanContract defaults and normalizers. The LLM planner is
// gone; these keep deterministic plan construction for preview and the
// retained admin helpers.

import (
	"strings"
)

func defaultSupportQueryPlan(customerMessage string) SupportQueryPlanContract {
	current := strings.TrimSpace(customerMessage)
	definition := supportIntentDefinition(supportIntentUnknown)
	queries := []string{}
	if current != "" {
		queries = []string{current}
	}
	return SupportQueryPlanContract{
		Route:            supportDecisionAnswer,
		Decision:         supportDecisionAnswer,
		Intent:           definition.ID,
		Subject:          "",
		Language:         "",
		Risk:             definition.Risk,
		RequiredEvidence: cloneStringSlice(definition.RequiredEvidence),
		EvidenceMode:     definition.EvidenceMode,
		RegistryVersion:  definition.Version,
		ContextAction:    "new_issue",
		IssueKey:         normalizeSupportIssueKey("", current),
		IssueSummary:     normalizeSupportIssueSummary("", current),
		ProgressSignal:   supportProgressNewIssue,
		StandaloneQuery:  current,
		SearchQueries:    queries,
		Reason:           "planner_unavailable",
	}
}

func normalizeSupportLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	language = strings.ReplaceAll(language, "_", "-")
	if language == "" {
		return ""
	}
	parts := strings.Split(language, "-")
	if len(language) > 16 || len(parts[0]) < 2 || len(parts[0]) > 3 {
		return ""
	}
	for partIndex, part := range parts {
		if part == "" || len(part) > 8 {
			return ""
		}
		for _, value := range part {
			isAlpha := value >= 'a' && value <= 'z'
			isDigit := value >= '0' && value <= '9'
			if (!isAlpha && partIndex == 0) || (!isAlpha && !isDigit && partIndex > 0) {
				return ""
			}
		}
	}
	return language
}

func normalizeSupportContextAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "continue", "confirm_previous":
		return strings.ToLower(strings.TrimSpace(action))
	default:
		return "new_issue"
	}
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func dedupeQueries(queries []string) []string {
	if len(queries) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	deduped := make([]string, 0, len(queries))
	for _, query := range queries {
		trimmed := strings.TrimSpace(query)
		if trimmed == "" {
			continue
		}
		key := normalizeQueryKey(trimmed)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, trimmed)
	}
	return deduped
}

func normalizeSupportQueryPlan(plan SupportQueryPlanContract, customerMessage string) SupportQueryPlanContract {
	current := strings.TrimSpace(customerMessage)
	normalized := defaultSupportQueryPlan(current)
	route := strings.ToLower(strings.TrimSpace(plan.Route))
	if route == "" {
		route = strings.ToLower(strings.TrimSpace(plan.Decision))
	}
	if route == supportDecisionGreet {
		route = supportRouteConversational
	}
	definition := normalizeSupportIntent(plan.Intent, plan.RequiredEvidence)
	issueKey := normalizeSupportIssueKey(plan.IssueKey, current)
	issueSummary := normalizeSupportIssueSummary(plan.IssueSummary, current)
	progressSignal := normalizeSupportProgressSignal(plan.ProgressSignal)
	contextAction := normalizeSupportContextAction(plan.ContextAction)
	subject := strings.Join(strings.Fields(strings.TrimSpace(plan.Subject)), " ")
	if len(subject) > 120 {
		subject = strings.TrimSpace(subject[:120])
	}
	language := normalizeSupportLanguage(plan.Language)
	applyRegistry := func(result *SupportQueryPlanContract) {
		result.Intent = definition.ID
		result.Subject = subject
		result.Language = language
		result.Risk = definition.Risk
		result.RequiredEvidence = cloneStringSlice(definition.RequiredEvidence)
		result.EvidenceMode = definition.EvidenceMode
		result.RegistryVersion = definition.Version
		result.ContextAction = contextAction
	}

	switch route {
	case supportRouteConversational:
		reply := strings.TrimSpace(plan.Reply)
		if reply == "" {
			reply = strings.TrimSpace(plan.GreetingReply)
		}
		if reply == "" {
			return normalized
		}
		result := SupportQueryPlanContract{
			Route:          supportRouteConversational,
			Decision:       supportDecisionGreet,
			Reply:          reply,
			ProgressSignal: defaultPlannerProgressSignal(progressSignal, supportProgressNewIssue),
			SearchQueries:  []string{},
			GreetingReply:  reply,
			Reason:         normalizedPlannerReason(plan.Reason, "conversational"),
		}
		applyRegistry(&result)
		return result
	case supportDecisionConfirm:
		reply := strings.TrimSpace(plan.Reply)
		if reply == "" {
			return normalized
		}
		result := SupportQueryPlanContract{
			Route:          supportDecisionConfirm,
			Decision:       supportDecisionConfirm,
			Reply:          reply,
			ProgressSignal: defaultPlannerProgressSignal(progressSignal, supportProgressSameNewInfo),
			SearchQueries:  []string{},
			Reason:         normalizedPlannerReason(plan.Reason, "confirmation"),
		}
		result.ContextAction = "confirm_previous"
		applyRegistry(&result)
		result.ContextAction = "confirm_previous"
		return result
	case supportDecisionClarify:
		question := strings.TrimSpace(plan.Reply)
		if question == "" {
			question = strings.TrimSpace(plan.ClarifyingQuestion)
		}
		if question == "" {
			normalized.IssueKey = issueKey
			normalized.IssueSummary = issueSummary
			normalized.ProgressSignal = defaultPlannerProgressSignal(progressSignal, supportProgressSameUnclear)
			return normalized
		}
		result := SupportQueryPlanContract{
			Route:              supportDecisionClarify,
			Decision:           supportDecisionClarify,
			Reply:              question,
			IssueKey:           issueKey,
			IssueSummary:       issueSummary,
			ProgressSignal:     defaultPlannerProgressSignal(progressSignal, supportProgressSameUnclear),
			SearchQueries:      []string{},
			ClarifyingQuestion: question,
			Reason:             normalizedPlannerReason(plan.Reason, "needs_clarification"),
		}
		applyRegistry(&result)
		return result
	case supportDecisionHandoff:
		result := SupportQueryPlanContract{
			Route:          supportDecisionHandoff,
			Decision:       supportDecisionHandoff,
			Reply:          strings.TrimSpace(plan.Reply),
			IssueKey:       issueKey,
			IssueSummary:   issueSummary,
			ProgressSignal: defaultPlannerProgressSignal(progressSignal, supportProgressSameRepeat),
			SearchQueries:  []string{},
			Reason:         normalizedPlannerReason(plan.Reason, "planner_handoff"),
		}
		applyRegistry(&result)
		return result
	default:
		standalone := strings.TrimSpace(plan.StandaloneQuery)
		if standalone == "" {
			standalone = current
		}
		searchQueries := dedupeQueries(append([]string{standalone}, plan.SearchQueries...))
		if len(searchQueries) > 4 {
			searchQueries = searchQueries[:4]
		}
		if len(searchQueries) == 0 && standalone != "" {
			searchQueries = []string{standalone}
		}
		if len(searchQueries) == 0 && current != "" {
			searchQueries = []string{current}
		}
		result := SupportQueryPlanContract{
			Route:           supportDecisionAnswer,
			Decision:        supportDecisionAnswer,
			IssueKey:        issueKey,
			IssueSummary:    issueSummary,
			ProgressSignal:  defaultPlannerProgressSignal(progressSignal, supportProgressNewIssue),
			StandaloneQuery: standalone,
			SearchQueries:   searchQueries,
			Reason:          normalizedPlannerReason(plan.Reason, "resolved_from_context"),
		}
		applyRegistry(&result)
		return result
	}
}
func defaultPlannerProgressSignal(candidate, fallback string) string {
	if candidate != "" {
		return candidate
	}
	return fallback
}

// normalizedPlannerReason lowercases free text into a snake_case token.
func normalizedPlannerReason(raw, fallback string) string {
	candidate := strings.ToLower(strings.TrimSpace(raw))
	if candidate == "" {
		return fallback
	}
	var sb strings.Builder
	lastUnderscore := false
	for _, r := range candidate {
		switch {
		case r >= 'a' && r <= 'z':
			sb.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			sb.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore && sb.Len() > 0 {
				sb.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	reason := strings.Trim(sb.String(), "_")
	if reason == "" {
		return fallback
	}
	return reason
}

func normalizeSupportIssueKey(raw, fallback string) string {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		candidate = strings.TrimSpace(fallback)
	}
	if candidate == "" {
		return ""
	}
	tokens := tokenizeWords(candidate)
	if len(tokens) == 0 {
		return normalizedPlannerReason(candidate, "")
	}
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		switch token {
		case "a", "an", "and", "are", "do", "for", "help", "i", "is", "it", "me", "my", "of", "on", "please", "the", "to", "we", "with", "you":
			continue
		default:
			filtered = append(filtered, token)
		}
		if len(filtered) >= 6 {
			break
		}
	}
	if len(filtered) == 0 {
		filtered = tokens
		if len(filtered) > 6 {
			filtered = filtered[:6]
		}
	}
	return strings.Join(filtered, "_")
}

func normalizeSupportIssueSummary(raw, fallback string) string {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		candidate = strings.TrimSpace(fallback)
	}
	if candidate == "" {
		return ""
	}
	if len(candidate) <= 140 {
		return candidate
	}
	return strings.TrimSpace(candidate[:140])
}

func normalizeSupportProgressSignal(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case supportProgressNewIssue:
		return supportProgressNewIssue
	case supportProgressSameNewInfo:
		return supportProgressSameNewInfo
	case supportProgressSameRepeat:
		return supportProgressSameRepeat
	case supportProgressSameUnclear:
		return supportProgressSameUnclear
	default:
		return ""
	}
}
