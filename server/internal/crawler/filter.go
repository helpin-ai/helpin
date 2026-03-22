package crawler

import (
	"net/url"
	"path"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// URLFilter determines whether a discovered URL should be crawled.
type URLFilter struct {
	baseDomain        string
	includeSubdomains bool
	includePatterns   []string
	excludePatterns   []string
}

// NewURLFilter creates a filter from the given content source settings.
func NewURLFilter(source model.SupportContentSource) *URLFilter {
	return &URLFilter{
		baseDomain:        extractDomain(source.StartURL),
		includeSubdomains: source.IncludeSubdomains,
		includePatterns:   []string(source.IncludePatterns),
		excludePatterns:   []string(source.ExcludePatterns),
	}
}

// Allowed returns true if the URL should be visited.
func (f *URLFilter) Allowed(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	// Must be http or https.
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	// Domain check.
	host := strings.ToLower(parsed.Hostname())
	if host != f.baseDomain {
		if !f.includeSubdomains || !strings.HasSuffix(host, "."+f.baseDomain) {
			return false
		}
	}

	// Include patterns: if set, URL path must match at least one.
	if len(f.includePatterns) > 0 {
		matched := false
		for _, pattern := range f.includePatterns {
			if matchPattern(parsed.Path, strings.TrimSpace(pattern)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Exclude patterns: URL path must not match any.
	for _, pattern := range f.excludePatterns {
		if matchPattern(parsed.Path, strings.TrimSpace(pattern)) {
			return false
		}
	}

	return true
}

// matchPattern performs glob-style matching. It uses path.Match for simple
// patterns and falls back to prefix/suffix matching for patterns with wildcards.
func matchPattern(urlPath, pattern string) bool {
	if pattern == "" {
		return false
	}

	// Try standard path.Match first.
	if matched, err := path.Match(pattern, urlPath); err == nil && matched {
		return true
	}

	// Handle common wildcard patterns like "/docs/*" or "*.html".
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(urlPath, prefix)
	}
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(urlPath, suffix)
	}

	return urlPath == pattern
}

// extractDomain returns the hostname without port from a URL string.
func extractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

// NormalizeURL strips the fragment and normalizes a URL for deduplication.
func NormalizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.Fragment = ""
	return parsed.String()
}
