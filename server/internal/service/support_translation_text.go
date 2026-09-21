package service

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
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

// estimatedTranslationTokens is a conservative multilingual estimate, not a
// provider-specific tokenizer. Non-ASCII characters receive a larger budget.
func estimatedTranslationTokens(text string) int {
	ascii, other := 0, 0
	for _, r := range text {
		if r < utf8.RuneSelf {
			ascii++
		} else {
			other++
		}
	}
	return (ascii+3)/4 + other*2
}

// translationChunks preserves every byte and prefers paragraph/word boundaries.
// Small chunks also keep original + translation within Jev's review budget.
func translationChunks(text string) []string {
	var chunks []string
	for len(text) > 0 {
		end, lastSpace, lastLine, ascii, other := 0, 0, 0, 0, 0
		for i, r := range text {
			next := i + utf8.RuneLen(r)
			if r < utf8.RuneSelf {
				ascii++
			} else {
				other++
			}
			if next > 3200 || (ascii+3)/4+other*2 > 1000 {
				break
			}
			end = next
			if r == '\n' || r == ' ' {
				lastSpace = next
			}
			if r == '\n' {
				lastLine = next
			}
		}
		if end < len(text) {
			if lastLine > end/2 {
				end = lastLine
			} else if lastSpace > end/2 {
				end = lastSpace
			}
		}
		// Never sever a URL, code block, identifier or protected literal.
		for _, span := range translationProtected.FindAllStringIndex(text, -1) {
			if span[0] >= end {
				break
			}
			if span[1] > end {
				if span[0] > 0 {
					end = span[0]
				} else {
					end = span[1]
				}
				break
			}
		}
		if end == 0 {
			_, end = utf8.DecodeRuneInString(text)
		}
		chunks = append(chunks, text[:end])
		text = text[end:]
	}
	return chunks
}
