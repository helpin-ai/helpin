package prcontent

import "strings"

const (
	StartMarker = "<!-- helpin:delivery:start -->"
	EndMarker   = "<!-- helpin:delivery:end -->"
)

// Wrap marks content that Helpin may safely refresh on a later delivery run.
func Wrap(body string) string {
	return StartMarker + "\n" + strings.TrimSpace(body) + "\n" + EndMarker
}

// Merge replaces only Helpin's managed section. Human-authored content outside
// the markers is preserved. Older PRs without markers keep their current body
// and receive the managed section at the end.
func Merge(existing, generated string) string {
	existing = strings.TrimSpace(existing)
	generated = strings.TrimSpace(generated)
	if generated == "" {
		return existing
	}

	start := strings.Index(existing, StartMarker)
	if start >= 0 {
		endOffset := strings.Index(existing[start+len(StartMarker):], EndMarker)
		if endOffset >= 0 {
			end := start + len(StartMarker) + endOffset + len(EndMarker)
			return strings.TrimSpace(existing[:start] + generated + existing[end:])
		}
	}
	if existing == "" {
		return generated
	}
	return existing + "\n\n" + generated
}
