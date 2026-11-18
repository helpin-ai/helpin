package service

import (
	"regexp"
	"slices"
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
	Outcome             string
	RepairDecision      string
	Reasons             []string
	UnsupportedClaims   []AIResponseClaim
	SupportedClaimCount int
	MaterialClaimCount  int
}

var (
	// _supportNumberPattern matches a number as written: 1,000 / 49.00 / 2.0.
	_supportNumberPattern = regexp.MustCompile(`\d+(?:[.,]\d+)*`)
	groupedNumberPattern  = regexp.MustCompile(`^\d{1,3}(?:,\d{3})+(?:\.\d+)?$`)
	orderedListPattern    = regexp.MustCompile(`(?m)^\s*(?:#{1,6}\s+)?(?:\*\*|__)?\d+[.)](?:\*\*|__)?\s+`)
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
	for _, claim := range response.Claims {
		if strings.TrimSpace(claim.Text) == "" {
			continue
		}
		validation.MaterialClaimCount++
		// Terra reviews the supplied chunks and returns strict evidence-ID
		// mappings; check only that the mapped evidence exists.
		if slices.ContainsFunc(claim.EvidenceIDs, func(id string) bool { _, ok := evidence[id]; return ok }) {
			validation.SupportedClaimCount++
			continue
		}
		validation.Outcome = supportValidationUngrounded
		validation.Reasons = append(validation.Reasons, "unsupported_claim")
		validation.UnsupportedClaims = append(validation.UnsupportedClaims, claim)
	}
	if strings.TrimSpace(response.Content) != "" && validation.MaterialClaimCount == 0 {
		validation.Outcome = supportValidationUngrounded
		validation.Reasons = append(validation.Reasons, "answer_without_claim_map")
	}
	// A number the reply states must appear in some retrieved evidence. Only
	// the digits are compared: units, currency, and wording vary too much
	// between a reply and a crawled page to match reliably, while an invented
	// number appears nowhere.
	var evidenceText strings.Builder
	for _, result := range results {
		evidenceText.WriteString(result.Title + "\n" + result.HeadingPath + "\n" + result.Content + "\n")
	}
	evidenceNumbers := supportNumbers(evidenceText.String())
	for number := range supportNumbers(response.Content) {
		if _, ok := evidenceNumbers[number]; !ok {
			validation.Outcome = supportValidationNumeric
			validation.Reasons = append(validation.Reasons, "unsupported_numeric:"+number)
		}
	}
	validation.Reasons = dedupeQueries(validation.Reasons)
	sort.Strings(validation.Reasons)
	return validation
}

// supportNumbers returns the numbers a text states as digits, so "$1,000.00"
// is "1000". List numbering is not a number the text states.
func supportNumbers(text string) map[string]struct{} {
	numbers := map[string]struct{}{}
	for _, number := range _supportNumberPattern.FindAllString(orderedListPattern.ReplaceAllString(text, ""), -1) {
		if groupedNumberPattern.MatchString(number) {
			number = strings.ReplaceAll(number, ",", "")
		}
		if whole, decimals, ok := strings.Cut(number, "."); ok && strings.Trim(decimals, "0") == "" {
			number = whole
		}
		numbers[number] = struct{}{}
	}
	return numbers
}
