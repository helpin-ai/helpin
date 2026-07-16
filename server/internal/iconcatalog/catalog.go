// Package iconcatalog resolves and validates Helpin's generated icon catalog.
package iconcatalog

import (
	"errors"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxDisplayTextBytes = 64
	maxDisplayTextRunes = 16
)

var errInvalidIcon = errors.New("invalid icon; use a canonical icon ID from search_icons or omit the icon")

type entry struct {
	ID    string
	Label string
}

type aliasEntry struct {
	Source string
	Target string
}

// SearchResult is one canonical icon returned to callers.
type SearchResult struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ResolutionKind describes how a persisted icon value should be rendered.
type ResolutionKind uint8

const (
	// ResolutionNone means the caller should use its default icon behavior.
	ResolutionNone ResolutionKind = iota
	// ResolutionAsset means Value is a canonical generated icon ID.
	ResolutionAsset
	// ResolutionDisplayText means Value is intentional legacy display text.
	ResolutionDisplayText
)

// Resolution is the normalized render result for one persisted icon value.
type Resolution struct {
	Kind  ResolutionKind
	Value string
}

// Version returns the pinned generated icon-catalog version.
func Version() string { return _catalogVersion }

// Hash returns the deterministic generated icon-catalog hash.
func Hash() string { return _catalogHash }

// ResolveStored resolves a persisted value without applying strict new-write rules.
func ResolveStored(value *string) Resolution {
	if value == nil {
		return Resolution{Kind: ResolutionNone}
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return Resolution{Kind: ResolutionNone}
	}
	if canonical, ok := resolveKnown(trimmed, true); ok {
		return Resolution{Kind: ResolutionAsset, Value: canonical}
	}
	if looksLikeRawIconReference(trimmed) {
		return Resolution{Kind: ResolutionNone}
	}
	if !isIdentifierShaped(trimmed) {
		return Resolution{Kind: ResolutionDisplayText, Value: trimmed}
	}
	return Resolution{Kind: ResolutionNone}
}

// ResolvePublicValue resolves a stored value to the existing nullable-string wire format.
// Unknown identifier-shaped values use fallbackID; empty values remain nil.
func ResolvePublicValue(value *string, fallbackID string) *string {
	resolved := ResolveStored(value)
	switch resolved.Kind {
	case ResolutionAsset, ResolutionDisplayText:
		result := resolved.Value
		return &result
	case ResolutionNone:
		if value != nil && strings.TrimSpace(*value) != "" && fallbackID != "" {
			result := fallbackID
			return &result
		}
		return nil
	default:
		return nil
	}
}

// NormalizeNew validates and canonicalizes a newly supplied icon value.
func NormalizeNew(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	if canonical, ok := resolveKnown(trimmed, false); ok {
		return &canonical, nil
	}
	if validDisplayText(trimmed) {
		return &trimmed, nil
	}
	return nil, errInvalidIcon
}

// NormalizeUpdate validates an explicitly submitted update while tolerating an
// unchanged historical value. The bool reports whether the caller should write
// the normalized value.
func NormalizeUpdate(value, current *string) (*string, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	submitted := strings.TrimSpace(*value)
	persisted := ""
	if current != nil {
		persisted = strings.TrimSpace(*current)
	}
	if submitted == persisted {
		return nil, false, nil
	}
	normalized, err := NormalizeNew(&submitted)
	if err != nil {
		return nil, false, err
	}
	return normalized, true, nil
}

// Search returns a bounded set of canonical icons matching ID or label.
func Search(query string, limit int) []SearchResult {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	type rankedResult struct {
		entry entry
		rank  int
	}
	ranked := make([]rankedResult, 0, limit)
	for _, item := range _entries {
		id := strings.ToLower(item.ID)
		label := strings.ToLower(item.Label)
		rank := 4
		switch {
		case normalizedQuery == "":
			if !isCommonIcon(item.ID) {
				continue
			}
			rank = 0
		case id == normalizedQuery || label == normalizedQuery:
			rank = 0
		case strings.HasPrefix(id, normalizedQuery) || strings.HasPrefix(label, normalizedQuery):
			rank = 1
		case strings.Contains(id, normalizedQuery) || strings.Contains(label, normalizedQuery):
			rank = 2
		default:
			continue
		}
		ranked = append(ranked, rankedResult{entry: item, rank: rank})
	}
	sort.Slice(ranked, func(left, right int) bool {
		if ranked[left].rank != ranked[right].rank {
			return ranked[left].rank < ranked[right].rank
		}
		if ranked[left].entry.Label != ranked[right].entry.Label {
			return ranked[left].entry.Label < ranked[right].entry.Label
		}
		return ranked[left].entry.ID < ranked[right].entry.ID
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	result := make([]SearchResult, 0, len(ranked))
	for _, item := range ranked {
		result = append(result, SearchResult{ID: item.entry.ID, Label: item.entry.Label})
	}
	return result
}

func resolveKnown(value string, includeHeuristics bool) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if hasCanonicalID(normalized) {
		return normalized, true
	}
	if target, ok := findAlias(_aliases[:], normalized); ok {
		return target, true
	}
	if !includeHeuristics {
		return "", false
	}
	heuristicName := normalizeLegacyName(value)
	if heuristicName == "" {
		return "", false
	}
	if hasCanonicalID(heuristicName) {
		return heuristicName, true
	}
	if target, ok := findAlias(_aliases[:], heuristicName); ok {
		return target, true
	}
	if target, ok := findAlias(_normalizedAliases[:], heuristicName); ok {
		return target, true
	}
	for _, token := range strings.Split(heuristicName, "-") {
		if target, ok := findAlias(_tokenAliases[:], token); ok {
			return target, true
		}
	}
	return "", false
}

func hasCanonicalID(id string) bool {
	index := sort.Search(len(_entries), func(index int) bool { return _entries[index].ID >= id })
	return index < len(_entries) && _entries[index].ID == id
}

func findAlias(entries []aliasEntry, source string) (string, bool) {
	index := sort.Search(len(entries), func(index int) bool { return entries[index].Source >= source })
	if index >= len(entries) || entries[index].Source != source {
		return "", false
	}
	return entries[index].Target, true
}

func normalizeLegacyName(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("icon") && strings.EqualFold(value[len(value)-len("icon"):], "icon") {
		value = value[:len(value)-len("icon")]
	}
	var builder strings.Builder
	lastHyphen := false
	var previous rune
	for index, r := range value {
		switch {
		case r >= '0' && r <= '9':
			continue
		case r >= 'A' && r <= 'Z':
			if index > 0 && ((previous >= 'a' && previous <= 'z') || (previous >= '0' && previous <= '9')) && !lastHyphen {
				builder.WriteByte('-')
			}
			builder.WriteRune(unicode.ToLower(r))
			lastHyphen = false
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastHyphen = false
		default:
			if builder.Len() > 0 && !lastHyphen {
				builder.WriteByte('-')
				lastHyphen = true
			}
		}
		previous = r
	}
	return strings.Trim(builder.String(), "-")
}

func isIdentifierShaped(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func validDisplayText(value string) bool {
	if !utf8.ValidString(value) || len(value) > maxDisplayTextBytes || utf8.RuneCountInString(value) > maxDisplayTextRunes {
		return false
	}
	hasNonASCII := false
	for _, r := range value {
		if r > unicode.MaxASCII {
			hasNonASCII = true
		}
		if unicode.IsControl(r) || r == '\n' || r == '\r' {
			return false
		}
	}
	return hasNonASCII && !isIdentifierShaped(value)
}

func looksLikeRawIconReference(value string) bool {
	if len(value) >= len("icon") && strings.EqualFold(value[len(value)-len("icon"):], "icon") {
		return true
	}
	var previous rune
	for _, r := range value {
		if r >= 'A' && r <= 'Z' && ((previous >= 'a' && previous <= 'z') || (previous >= '0' && previous <= '9')) {
			return true
		}
		previous = r
	}
	return false
}

func isCommonIcon(id string) bool {
	switch id {
	case "book-open01", "code", "file01", "folder", "globe", "help-circle", "home", "information-circle", "rocket", "search", "settings", "user":
		return true
	default:
		return false
	}
}
