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

	// Include patterns: if set, the URL must match at least one.
	if len(f.includePatterns) > 0 {
		matched := false
		for _, pattern := range f.includePatterns {
			if matchURLPattern(parsed, strings.TrimSpace(pattern)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Exclude patterns: the URL must not match any.
	for _, pattern := range f.excludePatterns {
		if matchURLPattern(parsed, strings.TrimSpace(pattern)) {
			return false
		}
	}

	return true
}

// matchURLPattern preserves path-only rules while supporting URL globs such as
// "*://blog.*/**" for hostname exclusions.
func matchURLPattern(parsed *url.URL, pattern string) bool {
	scheme, remainder, fullURL := strings.Cut(pattern, "://")
	if !fullURL {
		return matchPattern(parsed.Path, pattern)
	}
	host, urlPath, hasPath := strings.Cut(remainder, "/")
	if !matchPattern(parsed.Scheme, scheme) || !matchPattern(strings.ToLower(parsed.Host), strings.ToLower(host)) {
		return false
	}
	if !hasPath {
		return true
	}
	actualPath := parsed.Path
	if actualPath == "" {
		actualPath = "/"
	}
	return matchPattern(actualPath, "/"+urlPath)
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
		prefix := strings.TrimRight(pattern, "*")
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
