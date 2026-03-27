package docsi18n

import (
	"fmt"
	"sort"
	"strings"
)

type protectedTermSet struct {
	terms        []string
	termToToken  map[string]string
	sortedBySize []string
}

func newProtectedTermSet(terms []string) protectedTermSet {
	seen := make(map[string]struct{}, len(terms))
	normalized := make([]string, 0, len(terms))
	for _, term := range terms {
		trimmed := strings.TrimSpace(term)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	termToToken := make(map[string]string, len(normalized))
	for i, term := range normalized {
		termToToken[term] = fmt.Sprintf("__TERM_%03d__", i+1)
	}

	sorted := append([]string(nil), normalized...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if len(sorted[i]) == len(sorted[j]) {
			return sorted[i] < sorted[j]
		}
		return len(sorted[i]) > len(sorted[j])
	})

	return protectedTermSet{
		terms:        normalized,
		termToToken:  termToToken,
		sortedBySize: sorted,
	}
}

func (p protectedTermSet) Terms() []string {
	return append([]string(nil), p.terms...)
}

func (p protectedTermSet) Protect(text string) (string, segmentProtection) {
	protected := segmentProtection{
		placeholderToTerm: map[string]string{},
		placeholderCounts: map[string]int{},
	}
	replaced := text
	for _, term := range p.sortedBySize {
		token := p.termToToken[term]
		count := strings.Count(replaced, term)
		if count == 0 {
			continue
		}
		replaced = strings.ReplaceAll(replaced, term, token)
		protected.placeholderToTerm[token] = term
		protected.placeholderCounts[token] = count
	}
	return replaced, protected
}

func restoreProtectedTerms(text string, protection segmentProtection) (string, error) {
	restored := text
	for token, term := range protection.placeholderToTerm {
		expected := protection.placeholderCounts[token]
		if count := strings.Count(restored, token); count != expected {
			return "", fmt.Errorf("protected term placeholder %s count = %d, want %d", token, count, expected)
		}
		restored = strings.ReplaceAll(restored, token, term)
	}
	return restored, nil
}
