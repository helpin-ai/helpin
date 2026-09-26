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
	UnsupportedClaims    []AIResponseClaim
	SupportedClaimCount  int
	MaterialClaimCount   int
	NumericClaimsChecked int
}

var (
	currencyValuePattern = regexp.MustCompile(`(?i)(?:[$€£]\s?\d[\d,.]*|\b\d[\d,.]*\s?(?:usd|eur|gbp|dollars?|euros?|pounds?)\b)`)
	numericValuePattern  = regexp.MustCompile(`(?i)\b\d[\d,.]*(?:\s?%|\s?(?:users?|events?|sessions?|months?|years?|days?|hours?))?\b`)
	orderedListPattern   = regexp.MustCompile(`(?m)^\s*\d+[.)]\s+`)
	freePlanClaimPattern = regexp.MustCompile(`(?i)\bfree\s+(?:plan|tier)\b|\bfree\s+forever\b`)
)

func validateSupportAnswer(
	_ SupportQueryPlanContract,
	_ supportEvidenceCoverage,
	results []KnowledgeSearchResult,
	response *AIResponseContract,
) supportAnswerValidation {
	validation := supportAnswerValidation{Outcome: supportValidationPass, Reasons: []string{}}
	if response == nil {
		validation.Outcome = supportValidationUngrounded
		validation.Reasons = append(validation.Reasons, "missing_response")
		return validation
	}
	evidence := make(map[string]KnowledgeSearchResult, len(results))
	for _, result := range results {
		evidence[result.ID] = result
	}
	citedEvidenceIDs := map[string]struct{}{}
	for _, claim := range response.Claims {
		claim.Text = strings.TrimSpace(claim.Text)
		if claim.Text == "" {
			continue
		}
		validation.MaterialClaimCount++
		claimSupported := false
		for _, evidenceID := range claim.EvidenceIDs {
			_, ok := evidence[evidenceID]
			if !ok {
				continue
			}
			// Terra reviews the supplied chunks and returns strict evidence-ID
			// mappings. Token overlap is not a reliable semantic verifier: valid
			// paraphrases, different languages, and crawler formatting such as
			// "YearlySave" routinely fail it. Validate that the mapped evidence
			// exists here, then enforce exact numeric grounding below.
			claimSupported = true
			citedEvidenceIDs[evidenceID] = struct{}{}
		}
		if claimSupported {
			validation.SupportedClaimCount++
		} else {
			validation.Outcome = supportValidationUngrounded
			validation.Reasons = append(validation.Reasons, "unsupported_claim")
			validation.UnsupportedClaims = append(validation.UnsupportedClaims, claim)
		}
	}
	if strings.TrimSpace(response.Content) != "" && validation.MaterialClaimCount == 0 {
		validation.Outcome = supportValidationUngrounded
		validation.Reasons = append(validation.Reasons, "answer_without_claim_map")
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
	if freePlanClaimPattern.MatchString(response.Content) && !freePlanClaimPattern.MatchString(citedEvidenceText.String()) {
		validation.Outcome = supportValidationUngrounded
		validation.Reasons = append(validation.Reasons, "unsupported_free_plan_claim")
	}
	validation.Reasons = dedupeQueries(validation.Reasons)
	sort.Strings(validation.Reasons)
	return validation
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
