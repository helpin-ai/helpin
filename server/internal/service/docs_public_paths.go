package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const docsHelpcenterPublicIDLength = 8

func buildDocsHelpcenterArticleKey(slug, publicID string) string {
	trimmedSlug := strings.Trim(strings.TrimSpace(slug), "/")
	trimmedPublicID := normalizeDocsHelpcenterPublicID(publicID)
	if trimmedSlug == "" {
		return trimmedPublicID
	}
	if trimmedPublicID == "" {
		return trimmedSlug
	}
	return trimmedSlug + "-" + trimmedPublicID
}

func parseDocsHelpcenterArticleKey(key string) (slug string, publicID string, ok bool) {
	trimmed := strings.Trim(strings.TrimSpace(key), "/")
	if trimmed == "" {
		return "", "", false
	}

	lastDash := strings.LastIndex(trimmed, "-")
	if lastDash <= 0 || lastDash >= len(trimmed)-1 {
		return "", "", false
	}

	candidatePublicID := normalizeDocsHelpcenterPublicID(trimmed[lastDash+1:])
	if len(candidatePublicID) != docsHelpcenterPublicIDLength {
		return "", "", false
	}

	candidateSlug := strings.TrimSpace(trimmed[:lastDash])
	if candidateSlug == "" {
		return "", "", false
	}

	return candidateSlug, candidatePublicID, true
}

func buildDocsHelpcenterCollectionKey(slug, publicID string) string {
	trimmedSlug := strings.Trim(strings.TrimSpace(slug), "/")
	trimmedPublicID := normalizeDocsHelpcenterPublicID(publicID)
	if trimmedSlug == "" {
		return trimmedPublicID
	}
	if trimmedPublicID == "" {
		return trimmedSlug
	}
	return trimmedSlug + "-" + trimmedPublicID
}

func parseDocsHelpcenterCollectionKey(key string) (slug string, publicID string, ok bool) {
	trimmed := strings.Trim(strings.TrimSpace(key), "/")
	if trimmed == "" {
		return "", "", false
	}

	lastDash := strings.LastIndex(trimmed, "-")
	if lastDash <= 0 || lastDash >= len(trimmed)-1 {
		return "", "", false
	}

	candidatePublicID := normalizeDocsHelpcenterPublicID(trimmed[lastDash+1:])
	if len(candidatePublicID) != docsHelpcenterPublicIDLength {
		return "", "", false
	}

	candidateSlug := strings.TrimSpace(trimmed[:lastDash])
	if candidateSlug == "" {
		return "", "", false
	}

	return candidateSlug, candidatePublicID, true
}

func normalizeDocsHelpcenterPublicID(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if len(trimmed) != docsHelpcenterPublicIDLength {
		return trimmed
	}

	for _, r := range trimmed {
		isDigit := r >= '0' && r <= '9'
		isHexLetter := r >= 'a' && r <= 'f'
		if !isDigit && !isHexLetter {
			return ""
		}
	}

	return trimmed
}

func generateDocsHelpcenterPublicID() (string, error) {
	buf := make([]byte, docsHelpcenterPublicIDLength/2)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate docs helpcenter public id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func buildDocsHelpcenterCollectionCanonicalPath(cfg *model.DocsHelpcenterConfig, locale, collectionSlug, publicID string) string {
	collectionKey := buildDocsHelpcenterCollectionKey(collectionSlug, publicID)
	if collectionKey == "" {
		return "/"
	}

	if docsHelpcenterMultilingualEnabled(cfg) {
		return "/" + defaultDocsHelpcenterRouteLocale(cfg, locale) + "/c/" + collectionKey
	}

	return "/c/" + collectionKey
}

func buildDocsHelpcenterArticleCanonicalPath(cfg *model.DocsHelpcenterConfig, locale, slug, publicID string) string {
	articleKey := buildDocsHelpcenterArticleKey(slug, publicID)
	if articleKey == "" {
		return "/"
	}

	if docsHelpcenterMultilingualEnabled(cfg) {
		return "/" + defaultDocsHelpcenterRouteLocale(cfg, locale) + "/articles/" + articleKey
	}

	return "/articles/" + articleKey
}

func defaultDocsHelpcenterRouteLocale(cfg *model.DocsHelpcenterConfig, locale string) string {
	trimmed := strings.TrimSpace(strings.ToLower(locale))
	if trimmed != "" {
		return trimmed
	}
	return defaultHelpcenterLocale(cfg)
}

func docsHelpcenterMultilingualEnabled(cfg *model.DocsHelpcenterConfig) bool {
	if cfg == nil {
		return false
	}
	seen := make(map[string]struct{}, len(cfg.EnabledLocales))
	for _, locale := range cfg.EnabledLocales {
		normalized := strings.TrimSpace(strings.ToLower(locale))
		if normalized == "" {
			continue
		}
		seen[normalized] = struct{}{}
	}
	return len(seen) > 1
}
