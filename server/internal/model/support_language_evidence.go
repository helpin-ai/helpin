package model

import (
	"regexp"
	"strings"
	"unicode"
)

var nonLanguageContent = regexp.MustCompile("(?s)```.*?```|`[^`]*`|https?://[^\\s<>]+|[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}|\\b[A-Z0-9_-]*[0-9][A-Z0-9_-]*\\b")

// MeaningfulSupportLanguageText is conservative about acknowledgments and IDs,
// while allowing short sentences in scripts that do not separate words by spaces.
func MeaningfulSupportLanguageText(text string) bool {
	text = strings.TrimSpace(nonLanguageContent.ReplaceAllString(text, ""))
	switch strings.ToLower(text) {
	case "ok", "okay", "yes", "no", "thanks", "thank you", "hi", "hello":
		return false
	}
	fields := strings.Fields(text)
	if len(fields) >= 2 && len(fields) <= 3 {
		names := true
		for _, f := range fields {
			rs := []rune(f)
			if !unicode.IsUpper(rs[0]) {
				names = false
				break
			}
			for _, r := range rs[1:] {
				if !unicode.IsLower(r) {
					names = false
					break
				}
			}
		}
		if names {
			return false
		}
	}
	letters, scriptLetters := 0, 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
		}
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Thai) {
			scriptLetters++
		}
	}
	return scriptLetters >= 3 || (letters >= 8 && len(strings.Fields(text)) >= 2)
}
