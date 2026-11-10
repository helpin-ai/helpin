package service

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
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
	groupedNumberPattern = regexp.MustCompile(`^\d{1,3}(?:,\d{3})+(?:\.\d+)?$`)
	numericTokenPattern  = regexp.MustCompile(`\d+(?:[,.]\d+)*`)

	dateValuePattern      = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
	rangeValuePattern     = regexp.MustCompile(`(?i)([+−-]?\b\d+(?:\.\d+)?)\s*(?:[-–]|to)\s*([+−-]?\d+(?:\.\d+)?)\b`)
	qualifiedValuePattern = regexp.MustCompile(`(?i)\b(up to|at most|at least|more than|less than|over|under|exactly)\s+((?:[$€£]\s*)?[+−-]?\d+(?:[,.]\d+)*(?:\s*(?:thousand|million|billion|k)\b)?(?:\s*%|\s*(?:percentage points?|users?|seats?|events?|sessions?|months?|years?|days?|hours?|minutes?|seconds?|requests?|credits?|gb|mb)\b)?)`)

	currencyValuePattern      = regexp.MustCompile(`(?i)(?:[+−-]?[$€£]\s?[+−-]?\d[\d,.]*(?:\s*(?:thousand|million|billion|k)\b)?|\b(?:usd|eur|gbp)\s?[+−-]?\d[\d,.]*(?:\s*(?:thousand|million|billion|k)\b)?|[+−-]?\b\d[\d,.]*(?:\s*(?:thousand|million|billion|k)\b)?\s?(?:usd|eur|gbp|dollars?|euros?|pounds?)\b)`)
	monetaryScalePattern      = regexp.MustCompile(`(?i)\b(thousand|million|billion|k)\b`)
	adjacentBillingPattern    = regexp.MustCompile(`(?i)^\s*(?:/|per\s+|a\s+|each\s+)(months?|mo|years?|yr|days?|users?|seats?)\b`)
	numericValuePattern       = regexp.MustCompile(`(?i)[+−-]?\b\d+(?:[,.]\d+)*(?:\s*(?:thousand|million|billion|k)\b)?(?:\s*%|\s*(?:percentage points?|users?|seats?|events?|sessions?|months?|years?|days?|hours?|minutes?|seconds?|requests?|credits?|gb|mb)\b)?`)
	technicalValuePattern     = regexp.MustCompile(`(?i)\b(?:saml|tls|http|oauth|x)[ .-]*\d+(?:\.\d+)*\b`)
	billingPeriodPattern      = regexp.MustCompile(`(?i)\b(?:per|a|each)\s+(month|year|day|user|seat)\b`)
	technicalSeparatorPattern = regexp.MustCompile(`[ -]+`)
	orderedListPattern        = regexp.MustCompile(`(?m)^\s*(?:#{1,6}\s+)?(?:\*\*|__)?\d+[.)](?:\*\*|__)?\s+`)
	freePlanClaimPattern      = regexp.MustCompile(`(?i)\bfree\s+(?:plan|tier)\b|\bfree\s+forever\b`)
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
	evidenceValues := make(map[string]map[string]struct{}, len(evidence))
	for _, claim := range response.Claims {
		claim.Text = strings.TrimSpace(claim.Text)
		if claim.Text == "" {
			continue
		}
		validation.MaterialClaimCount++
		claimSupported := false
		claimValues := map[string]struct{}{}
		for _, evidenceID := range claim.EvidenceIDs {
			item, ok := evidence[evidenceID]
			if !ok {
				continue
			}
			// Terra reviews the supplied chunks and returns strict evidence-ID
			// mappings. Token overlap is not a reliable semantic verifier: valid
			// paraphrases, different languages, and crawler formatting such as
			// "YearlySave" routinely fail it. Validate that the mapped evidence
			// exists here, then enforce exact numeric grounding below.
			if _, parsed := evidenceValues[evidenceID]; !parsed {
				evidenceValues[evidenceID] = normalizedNumericValues(item.Title + "\n" + item.HeadingPath + "\n" + item.Content)
			}
			claimSupported = true
			citedEvidenceIDs[evidenceID] = struct{}{}
			for value := range evidenceValues[evidenceID] {
				claimValues[value] = struct{}{}
			}
		}
		if claimSupported {
			claimNumbers := normalizedNumericValues(claim.Text)
			for value := range claimNumbers {
				if _, ok := claimValues[value]; !ok {
					validation.Outcome = supportValidationNumeric
					validation.Reasons = append(validation.Reasons, "unsupported_numeric:"+value)
					validation.UnsupportedClaims = append(validation.UnsupportedClaims, claim)
				}
			}
			for value := range claimValues {
				if strings.HasPrefix(value, "qualified:") {
					quantity := value[strings.LastIndex(value, "|")+1:]
					_, mentioned := claimNumbers[quantity]
					if mentioned {
						if _, ok := claimNumbers[value]; !ok {
							validation.Outcome = supportValidationNumeric
							validation.Reasons = append(validation.Reasons, "numeric_qualifier_mismatch:"+value)
							validation.UnsupportedClaims = append(validation.UnsupportedClaims, claim)
						}
					}
				}
			}
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
	evidenceNumbers := map[string]struct{}{}
	for id := range citedEvidenceIDs {
		for value := range evidenceValues[id] {
			evidenceNumbers[value] = struct{}{}
		}
	}
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
	text = writtenQuantityPattern.ReplaceAllStringFunc(text, func(value string) string {
		if normalized, ok := normalizeWrittenQuantity(value); ok {
			return normalized
		}
		values["written:"+strings.ToLower(strings.Join(strings.Fields(value), " "))] = struct{}{}
		return value
	})
	for _, value := range dateValuePattern.FindAllString(text, -1) {
		values["date:"+value] = struct{}{}
	}
	text = dateValuePattern.ReplaceAllString(text, "")
	for _, match := range rangeValuePattern.FindAllStringSubmatch(text, -1) {
		values["range:"+normalizeNumericToken(match[1])+".."+normalizeNumericToken(match[2])] = struct{}{}
	}
	// Range dashes are separators, not negative signs on their second endpoint.
	text = rangeValuePattern.ReplaceAllStringFunc(text, func(value string) string {
		match := rangeValuePattern.FindStringSubmatch(value)
		return match[1] + " to " + match[2]
	})
	for _, match := range qualifiedValuePattern.FindAllStringSubmatch(text, -1) {
		qualifier := strings.ToLower(strings.Join(strings.Fields(match[1]), " "))
		for quantity := range normalizedNumericValues(match[2]) {
			values["qualified:"+qualifier+"|"+quantity] = struct{}{}
		}
	}
	for _, value := range technicalValuePattern.FindAllString(text, -1) {
		values["identifier:"+strings.Replace(technicalSeparatorPattern.ReplaceAllString(strings.ToLower(value), ""), "x.", "x", 1)] = struct{}{}
	}
	text = technicalValuePattern.ReplaceAllString(text, "")
	if currencyValuePattern.MatchString(text) {
		for _, match := range billingPeriodPattern.FindAllStringSubmatch(text, -1) {
			values["billing:"+strings.ToLower(match[1])] = struct{}{}
		}
	}
	for _, span := range currencyValuePattern.FindAllStringIndex(text, -1) {
		value := strings.ToLower(text[span[0]:span[1]])
		currency := "usd"
		if strings.ContainsAny(value, "€") || strings.Contains(value, "eur") {
			currency = "eur"
		}
		if strings.ContainsAny(value, "£") || strings.Contains(value, "gbp") || strings.Contains(value, "pound") {
			currency = "gbp"
		}
		number := normalizeNumericToken(numericTokenPattern.FindString(value))
		if strings.Contains(number, ".") && !strings.Contains(number, ",") {
			number = strings.TrimSuffix(strings.TrimRight(number, "0"), ".")
		}
		if strings.ContainsAny(value, "-−") {
			number = "-" + number
		}
		if strings.Contains(value, "+") {
			number = "+" + number
		}
		if scale := monetaryScalePattern.FindString(value); scale != "" {
			number += " " + scale
		}
		normalized := currency + ":" + number
		if match := adjacentBillingPattern.FindStringSubmatch(text[span[1]:]); match != nil {
			period := strings.TrimSuffix(strings.ToLower(match[1]), "s")
			if period == "mo" {
				period = "month"
			}
			if period == "yr" {
				period = "year"
			}
			values[normalized+"/"+period] = struct{}{}
		}
		if normalized != "" {
			values[normalized] = struct{}{}
		}
	}
	text = currencyValuePattern.ReplaceAllString(text, "")
	for _, value := range numericValuePattern.FindAllString(strings.ToLower(text), -1) {
		normalized := normalizeNumericToken(value)
		if normalized != "" {
			values[normalized] = struct{}{}
		}
	}
	return values
}

// Normalize only unambiguous English numeric formatting; comma decimals stay distinct.
func normalizeNumericToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(strings.TrimRightFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r == '.' || r == ',' })))
	value = numericTokenPattern.ReplaceAllStringFunc(value, func(number string) string {
		if groupedNumberPattern.MatchString(number) {
			number = strings.ReplaceAll(number, ",", "")
		}
		return number
	})
	value = strings.ReplaceAll(strings.Join(strings.Fields(value), " "), "−", "-")
	value = strings.ReplaceAll(value, " %", "%")
	for _, symbol := range []string{"$", "€", "£"} {
		value = strings.ReplaceAll(value, symbol+" ", symbol)
	}
	return value
}

const writtenNumberWords = "zero|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty|thirty|forty|fifty|sixty|seventy|eighty|ninety|hundred|thousand|million|dozen"

var writtenQuantityPattern = regexp.MustCompile(`(?i)\b((?:` + writtenNumberWords + `)[ -]+(?:(?:` + writtenNumberWords + `|and)[ -]+)*)(users?|seats?|events?|sessions?|months?|years?|days?|hours?|minutes?|seconds?|requests?|credits?|dollars?|euros?|pounds?|percent)\b`)

// Larger or ambiguous written quantities require literal source support rather
// than an inferred arithmetic conversion.
func normalizeWrittenQuantity(value string) (string, bool) {
	parts := strings.Fields(strings.ReplaceAll(strings.ToLower(value), "-", " "))
	numbers := map[string]int{"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19, "twenty": 20, "thirty": 30, "forty": 40, "fifty": 50, "sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90}
	if len(parts) < 2 {
		return value, false
	}
	number, ok := numbers[parts[0]]
	if !ok {
		return value, false
	}
	if len(parts) == 3 {
		ones, exists := numbers[parts[1]]
		if !exists || number < 20 || number%10 != 0 || ones < 1 || ones > 9 {
			return value, false
		}
		number += ones
	} else if len(parts) != 2 {
		return value, false
	}
	unit := parts[len(parts)-1]
	if unit == "percent" {
		unit = "%"
	}
	return strconv.Itoa(number) + " " + unit, true
}
