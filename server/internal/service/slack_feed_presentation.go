package service

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func feedOneLine(text string) string { return strings.Join(strings.Fields(text), " ") }

func feedExcerpt(subject, text string) string {
	text = feedOneLine(text)
	if strings.EqualFold(feedOneLine(subject), text) {
		return ""
	}
	if utf8.RuneCountInString(text) <= 100 {
		return text
	}
	runes := []rune(text)
	cut := 99
	if !unicode.IsSpace(runes[cut]) {
		if boundary := strings.LastIndex(string(runes[:cut]), " "); boundary >= 0 {
			return string(runes[:cut])[:boundary] + "…"
		}
	}
	return strings.TrimSpace(string(runes[:cut])) + "…"
}

func feedRichText(kind string, elements ...any) map[string]any {
	return map[string]any{"type": "rich_text", "elements": []any{map[string]any{"type": kind, "elements": elements}}}
}
