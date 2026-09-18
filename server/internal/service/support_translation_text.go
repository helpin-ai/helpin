package service

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var translationProtected = regexp.MustCompile("(?s)HELPIN_KEEP_[A-Za-z0-9_]*|```.*?```|`[^`\\n]+`|https?://[^\\s<>\\)]+|[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}|\\b[A-Z][A-Z0-9_-]*[0-9][A-Z0-9_-]*\\b|\\b[0-9]+(?:[.,:/-][0-9]+)*\\b")

func protectTranslationText(text string) (string, []string) {
	values := []string{}
	masked := translationProtected.ReplaceAllStringFunc(text, func(value string) string {
		values = append(values, value)
		return fmt.Sprintf("HELPIN_KEEP_%d_END", len(values)-1)
	})
	return masked, values
}
func restoreTranslationText(text string, values []string) (string, error) {
	replacements := make(map[string]string, len(values))
	for i, value := range values {
		token := fmt.Sprintf("HELPIN_KEEP_%d_END", i)
		if strings.Count(text, token) != 1 {
			return "", ErrSupportTranslation
		}
		replacements[token] = value
	}
	valid := true
	restored := translationPlaceholder.ReplaceAllStringFunc(text, func(token string) string {
		value, ok := replacements[token]
		if !ok {
			valid = false
		}
		return value
	})
	if !valid {
		return "", ErrSupportTranslation
	}
	return restored, nil
}

var translationPlaceholder = regexp.MustCompile(`HELPIN_KEEP_[A-Za-z0-9_]*`)

func translationLanguageCodes() string {
	codes := make([]string, 0, len(supportTranslationLanguages))
	for code := range supportTranslationLanguages {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return strings.Join(codes, ", ")
}
