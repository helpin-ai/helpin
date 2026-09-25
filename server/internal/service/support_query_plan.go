package service

// Shared normalization helpers for support knowledge retrieval and guidance.

import (
	"strings"
)

func normalizeSupportLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	language = strings.ReplaceAll(language, "_", "-")
	if language == "" {
		return ""
	}
	parts := strings.Split(language, "-")
	if len(language) > 16 || len(parts[0]) < 2 || len(parts[0]) > 3 {
		return ""
	}
	for partIndex, part := range parts {
		if part == "" || len(part) > 8 {
			return ""
		}
		for _, value := range part {
			isAlpha := value >= 'a' && value <= 'z'
			isDigit := value >= '0' && value <= '9'
			if (!isAlpha && partIndex == 0) || (!isAlpha && !isDigit && partIndex > 0) {
				return ""
			}
		}
	}
	return language
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func dedupeQueries(queries []string) []string {
	if len(queries) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	deduped := make([]string, 0, len(queries))
	for _, query := range queries {
		trimmed := strings.TrimSpace(query)
		if trimmed == "" {
			continue
		}
		key := normalizeQueryKey(trimmed)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, trimmed)
	}
	return deduped
}
