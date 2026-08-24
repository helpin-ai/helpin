package service

import (
	"fmt"
	"strings"
	"unicode"
)

// validateEnglishCRMNarrative catches the failure mode where an otherwise
// valid JSON response is written primarily in another script rather than the
// product's English CRM language. A small amount is allowed for names and
// quoted terms.
func validateEnglishCRMNarrative(values ...string) error {
	var latin, nonLatin int
	for _, value := range values {
		for _, r := range strings.Join(strings.Fields(value), " ") {
			switch {
			case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul):
				nonLatin++
			case unicode.Is(unicode.Latin, r):
				latin++
			case unicode.IsLetter(r):
				nonLatin++
			}
		}
	}
	if nonLatin >= 4 && nonLatin*5 > latin {
		return fmt.Errorf("CRM narrative must be written in English")
	}
	return nil
}
