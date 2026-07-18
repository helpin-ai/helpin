package service

import (
	"regexp"
	"sort"
	"strings"
)

const (
	supportValidationPass       = "pass"
	supportValidationIncomplete = "incomplete_evidence"
	supportValidationUngrounded = "ungrounded_claim"
	supportValidationNumeric    = "numeric_mismatch"
)

type supportEvidenceCoverage struct {
	Found   map[string][]string
	Missing []string
}

type supportAnswerValidation struct {
	Outcome              string
	Reasons              []string
	SupportedClaimCount  int
	MaterialClaimCount   int
	NumericClaimsChecked int
}

var (
	currencyValuePattern  = regexp.MustCompile(`(?i)(?:[$€£]\s?\d[\d,.]*|\b\d[\d,.]*\s?(?:usd|eur|gbp|dollars?|euros?|pounds?)\b)`)
	numericValuePattern   = regexp.MustCompile(`(?i)\b\d[\d,.]*(?:\s?%|\s?(?:users?|events?|sessions?|months?|years?|days?|hours?))?\b`)
	urlValuePattern       = regexp.MustCompile(`https?://[^\s)\]>]+`)
	orderedListPattern    = regexp.MustCompile(`(?m)^\s*\d+[.)]\s+`)
	supportClaimStopWords = map[string]struct{}{
		"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "by": {},
		"for": {}, "from": {}, "has": {}, "have": {}, "in": {}, "is": {}, "it": {}, "of": {},
		"on": {}, "or": {}, "that": {}, "the": {}, "their": {}, "this": {}, "to": {}, "with": {},
	}
	supportGenericSubjectTerms = map[string]struct{}{
		"analytics": {}, "billing": {}, "cost": {}, "costs": {}, "plan": {}, "plans": {},
		"price": {}, "prices": {}, "pricing": {}, "product": {}, "products": {},
		"subscription": {}, "subscriptions": {}, "tool": {}, "tools": {}, "website": {}, "websites": {},
	}
)

func evaluateSupportEvidenceCoverage(plan SupportQueryPlanContract, results []KnowledgeSearchResult) supportEvidenceCoverage {
	coverage := supportEvidenceCoverage{Found: map[string][]string{}, Missing: []string{}}
	for _, fieldID := range plan.RequiredEvidence {
		if fieldID == "customer_needs" && supportRequestContainsNeeds(plan.StandaloneQuery) {
			coverage.Found[fieldID] = []string{"request"}
			continue
		}
		for _, result := range results {
			if plan.Risk == supportRiskCommercial && plan.Subject != "" && !supportEvidenceMatchesSubject(plan.Subject, result) {
				continue
			}
			if supportEvidenceContainsField(fieldID, result) {
				coverage.Found[fieldID] = append(coverage.Found[fieldID], result.ID)
			}
		}
		coverage.Found[fieldID] = dedupeQueries(coverage.Found[fieldID])
		if len(coverage.Found[fieldID]) == 0 {
			coverage.Missing = append(coverage.Missing, fieldID)
		}
	}
	return coverage
}

func supportEvidenceMatchesSubject(subject string, result KnowledgeSearchResult) bool {
	if result.SourceType == knowledgeSourceTypeGuidance {
		return true
	}
	subjectTerms := make([]string, 0)
	for _, term := range supportMaterialTerms(subject) {
		if _, generic := supportGenericSubjectTerms[term]; generic {
			continue
		}
		subjectTerms = append(subjectTerms, term)
	}
	if len(subjectTerms) == 0 {
		return false
	}
	primaryText := strings.ToLower(result.Title + "\n" + result.HeadingPath + "\n" + result.Content)
	for _, term := range subjectTerms {
		if !strings.Contains(primaryText, term) {
			return supportEvidenceSubjectAppearsInCanonicalURL(subjectTerms, result.URL)
		}
	}
	return true
}

func supportEvidenceSubjectAppearsInCanonicalURL(subjectTerms []string, rawURL string) bool {
	url := strings.ToLower(rawURL)
	if !(strings.Contains(url, "/pricing") || strings.Contains(url, "/plans") || strings.Contains(url, "/billing")) {
		return false
	}
	for _, term := range subjectTerms {
		if len(term) < 2 || !strings.Contains(url, term) {
			return false
		}
	}
	return true
}

func supportEvidenceContainsField(fieldID string, result KnowledgeSearchResult) bool {
	text := strings.ToLower(result.Title + "\n" + result.HeadingPath + "\n" + result.Content)
	hasAny := func(values ...string) bool {
		for _, value := range values {
			if strings.Contains(text, value) {
				return true
			}
		}
		return false
	}
	switch fieldID {
	case "plan_names":
		return hasAny(" plan", "free", "starter", "basic", "pro", "growth", "scale", "business", "enterprise")
	case "starting_prices":
		return currencyValuePattern.MatchString(text)
	case "billing_cadence":
		return hasAny("monthly", "annual", "annually", "per month", "per year", "/month", "/year")
	case "enterprise_status":
		return hasAny("enterprise", "custom pricing", "contact sales", "talk to sales")
	case "canonical_url":
		return strings.TrimSpace(result.URL) != "" || urlValuePattern.MatchString(result.Content) || result.SourceType == knowledgeSourceTypeGuidance
	case "recommended_plan":
		return hasAny("recommend", "recommended", "best plan", "suitable", "ideal for", " plan")
	case "recommendation_basis":
		return hasAny("because", "based on", "for teams", "for businesses", "includes", "supports", "best for", "ideal for")
	case "applicable_limits":
		return numericValuePattern.MatchString(text) || hasAny("limit", "usage", "tracked", "quota", "volume")
	case "advertised_price_tax_status":
		return hasAny("excluding tax", "excluding vat", "include tax", "includes vat", "exclusive of", "before tax", "plus tax")
	case "tax_location_basis":
		return hasAny("billing location", "billing country", "country", "state", "province", "jurisdiction", "location")
	case "checkout_total_qualifier":
		return hasAny("checkout", "final total", "added automatically", "calculated", "charged", "invoice")
	default:
		return false
	}
}

func supportRequestContainsNeeds(query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return false
	}
	return strings.Contains(query, " for ") || numericValuePattern.MatchString(query) ||
		strings.Contains(query, "need") || strings.Contains(query, "website") ||
		strings.Contains(query, "product analytics") || strings.Contains(query, "team")
}

func targetedEvidenceQueries(plan SupportQueryPlanContract, missing []string) []string {
	queries := []string{}
	for _, fieldID := range missing {
		label := strings.ReplaceAll(fieldID, "_", " ")
		queryParts := []string{}
		if plan.Subject != "" && !strings.Contains(strings.ToLower(plan.StandaloneQuery), strings.ToLower(plan.Subject)) {
			queryParts = append(queryParts, plan.Subject)
		}
		queryParts = append(queryParts, plan.StandaloneQuery, label)
		queries = append(queries, strings.Join(nonEmptyStrings(queryParts), " "))
	}
	return dedupeQueries(queries)
}

func mergeKnowledgeResults(query string, groups ...[]KnowledgeSearchResult) []KnowledgeSearchResult {
	byID := map[string]KnowledgeSearchResult{}
	for _, group := range groups {
		for _, result := range group {
			current, ok := byID[result.ID]
			if !ok || result.CombinedScore > current.CombinedScore {
				byID[result.ID] = result
			}
		}
	}
	merged := make([]KnowledgeSearchResult, 0, len(byID))
	for _, result := range byID {
		merged = append(merged, result)
	}
	merged = rerankKnowledgeResults(query, merged)
	if len(merged) > 12 {
		merged = merged[:12]
	}
	return merged
}

func selectSupportEvidenceContext(
	plan SupportQueryPlanContract,
	coverage supportEvidenceCoverage,
	results []KnowledgeSearchResult,
	limit int,
) []KnowledgeSearchResult {
	if limit <= 0 || len(results) == 0 {
		return nil
	}
	byID := make(map[string]KnowledgeSearchResult, len(results))
	for _, result := range results {
		byID[result.ID] = result
	}
	selected := make([]KnowledgeSearchResult, 0, min(limit, len(results)))
	seen := map[string]struct{}{}
	add := func(result KnowledgeSearchResult) {
		if len(selected) >= limit {
			return
		}
		if _, ok := seen[result.ID]; ok {
			return
		}
		seen[result.ID] = struct{}{}
		selected = append(selected, result)
	}
	for _, fieldID := range plan.RequiredEvidence {
		for _, evidenceID := range coverage.Found[fieldID] {
			if result, ok := byID[evidenceID]; ok {
				add(result)
				break
			}
		}
	}
	for _, result := range results {
		add(result)
	}
	return selected
}

func validateSupportAnswer(
	plan SupportQueryPlanContract,
	coverage supportEvidenceCoverage,
	results []KnowledgeSearchResult,
	response *AIResponseContract,
) supportAnswerValidation {
	validation := supportAnswerValidation{Outcome: supportValidationPass, Reasons: []string{}}
	if response == nil {
		validation.Outcome = supportValidationUngrounded
		validation.Reasons = append(validation.Reasons, "missing_response")
		return validation
	}
	if len(coverage.Missing) > 0 {
		validation.Outcome = supportValidationIncomplete
		validation.Reasons = append(validation.Reasons, "missing:"+strings.Join(coverage.Missing, ","))
	}

	evidence := make(map[string]KnowledgeSearchResult, len(results))
	for _, result := range results {
		evidence[result.ID] = result
	}
	citedEvidenceIDs := map[string]struct{}{}
	requiredFields := make(map[string]struct{}, len(plan.RequiredEvidence))
	for _, fieldID := range plan.RequiredEvidence {
		requiredFields[fieldID] = struct{}{}
		mapped := false
		for _, evidenceID := range response.EvidenceCoverage[fieldID] {
			if !supportStringListContains(coverage.Found[fieldID], evidenceID) {
				continue
			}
			mapped = true
			if _, ok := evidence[evidenceID]; ok {
				citedEvidenceIDs[evidenceID] = struct{}{}
			}
		}
		if !mapped {
			validation.Outcome = supportValidationIncomplete
			validation.Reasons = append(validation.Reasons, "unmapped_required_evidence:"+fieldID)
		}
	}
	for fieldID := range response.EvidenceCoverage {
		if _, ok := requiredFields[fieldID]; !ok && plan.EvidenceMode == supportEvidenceModeSlots {
			validation.Outcome = supportValidationUngrounded
			validation.Reasons = append(validation.Reasons, "unexpected_evidence_field:"+fieldID)
		}
	}
	for _, claim := range response.Claims {
		claim.Text = strings.TrimSpace(claim.Text)
		if claim.Text == "" {
			continue
		}
		validation.MaterialClaimCount++
		claimSupported := false
		for _, evidenceID := range claim.EvidenceIDs {
			result, ok := evidence[evidenceID]
			if !ok {
				continue
			}
			if supportClaimOverlap(claim.Text, result.Title+"\n"+result.HeadingPath+"\n"+result.Content) >= 0.5 {
				claimSupported = true
				citedEvidenceIDs[evidenceID] = struct{}{}
				break
			}
		}
		if claimSupported {
			validation.SupportedClaimCount++
		} else {
			validation.Outcome = supportValidationUngrounded
			validation.Reasons = append(validation.Reasons, "unsupported_claim")
		}
	}
	if strings.TrimSpace(response.Content) != "" && validation.MaterialClaimCount == 0 {
		validation.Outcome = supportValidationUngrounded
		validation.Reasons = append(validation.Reasons, "answer_without_claim_map")
	}
	if validation.MaterialClaimCount > 0 {
		claimTexts := make([]string, 0, len(response.Claims))
		for _, claim := range response.Claims {
			if strings.TrimSpace(claim.Text) != "" {
				claimTexts = append(claimTexts, claim.Text)
			}
		}
		if supportClaimOverlap(response.Content, strings.Join(claimTexts, "\n")) < 0.7 {
			validation.Outcome = supportValidationUngrounded
			validation.Reasons = append(validation.Reasons, "response_claims_incomplete")
		}
	}

	var citedEvidenceText strings.Builder
	for evidenceID := range citedEvidenceIDs {
		result := evidence[evidenceID]
		citedEvidenceText.WriteString("\n")
		citedEvidenceText.WriteString(result.Title)
		citedEvidenceText.WriteString("\n")
		citedEvidenceText.WriteString(result.HeadingPath)
		citedEvidenceText.WriteString("\n")
		citedEvidenceText.WriteString(result.Content)
	}
	evidenceNumbers := normalizedNumericValues(citedEvidenceText.String())
	for value := range normalizedNumericValues(response.Content) {
		validation.NumericClaimsChecked++
		if _, ok := evidenceNumbers[value]; !ok {
			validation.Outcome = supportValidationNumeric
			validation.Reasons = append(validation.Reasons, "unsupported_numeric:"+value)
		}
	}
	validation.Reasons = dedupeQueries(validation.Reasons)
	sort.Strings(validation.Reasons)
	return validation
}

func supportStringListContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func supportClaimOverlap(claim, evidence string) float64 {
	claimTerms := supportMaterialTerms(claim)
	if len(claimTerms) == 0 {
		return 0
	}
	evidenceTerms := supportMaterialTerms(evidence)
	if len(evidenceTerms) == 0 {
		return 0
	}
	evidenceSet := make(map[string]struct{}, len(evidenceTerms))
	for _, term := range evidenceTerms {
		evidenceSet[term] = struct{}{}
	}
	matched := 0
	for _, term := range claimTerms {
		if _, ok := evidenceSet[term]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(claimTerms))
}

func supportMaterialTerms(value string) []string {
	terms := normalizedTerms(value)
	material := make([]string, 0, len(terms))
	for _, term := range terms {
		if _, stopWord := supportClaimStopWords[term]; stopWord {
			continue
		}
		material = append(material, term)
	}
	return material
}

func normalizedNumericValues(text string) map[string]struct{} {
	text = orderedListPattern.ReplaceAllString(text, "")
	values := map[string]struct{}{}
	for _, value := range numericValuePattern.FindAllString(strings.ToLower(text), -1) {
		normalized := strings.Join(strings.Fields(strings.ReplaceAll(value, ",", "")), " ")
		if normalized != "" {
			values[normalized] = struct{}{}
		}
	}
	for _, value := range currencyValuePattern.FindAllString(strings.ToLower(text), -1) {
		normalized := strings.Join(strings.Fields(strings.ReplaceAll(value, ",", "")), " ")
		if normalized != "" {
			values[normalized] = struct{}{}
		}
	}
	return values
}
