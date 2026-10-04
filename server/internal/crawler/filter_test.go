package crawler

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestURLFilter_Allowed(t *testing.T) {
	tests := []struct {
		name              string
		startURL          string
		includeSubdomains bool
		includePatterns   []string
		excludePatterns   []string
		testURL           string
		want              bool
	}{
		{
			name:     "matching base domain is allowed",
			startURL: "https://example.com/docs",
			testURL:  "https://example.com/about",
			want:     true,
		},
		{
			name:     "different domain is blocked",
			startURL: "https://example.com/docs",
			testURL:  "https://other.com/about",
			want:     false,
		},
		{
			name:              "subdomain allowed when includeSubdomains true",
			startURL:          "https://example.com/docs",
			includeSubdomains: true,
			testURL:           "https://blog.example.com/post",
			want:              true,
		},
		{
			name:              "subdomain blocked when includeSubdomains false",
			startURL:          "https://example.com/docs",
			includeSubdomains: false,
			testURL:           "https://blog.example.com/post",
			want:              false,
		},
		{
			name:            "URL matching include pattern is allowed",
			startURL:        "https://example.com",
			includePatterns: []string{"/docs/*"},
			testURL:         "https://example.com/docs/intro",
			want:            true,
		},
		{
			name:            "URL not matching include pattern is blocked",
			startURL:        "https://example.com",
			includePatterns: []string{"/docs/*"},
			testURL:         "https://example.com/blog/post",
			want:            false,
		},
		{
			name:            "URL matching exclude pattern is blocked",
			startURL:        "https://example.com",
			excludePatterns: []string{"/private/*"},
			testURL:         "https://example.com/private/secret",
			want:            false,
		},
		{
			name:            "URL not matching exclude pattern is allowed",
			startURL:        "https://example.com",
			excludePatterns: []string{"/private/*"},
			testURL:         "https://example.com/public/page",
			want:            true,
		},
		{
			name:     "non-http scheme ftp is blocked",
			startURL: "https://example.com",
			testURL:  "ftp://example.com/file.txt",
			want:     false,
		},
		{
			name:     "mailto scheme is blocked",
			startURL: "https://example.com",
			testURL:  "mailto:user@example.com",
			want:     false,
		},
		{
			name:     "empty URL is blocked",
			startURL: "https://example.com",
			testURL:  "",
			want:     false,
		},
		{
			name:     "invalid URL is blocked",
			startURL: "https://example.com",
			testURL:  "://broken",
			want:     false,
		},
		{
			name:     "http scheme is allowed",
			startURL: "https://example.com",
			testURL:  "http://example.com/page",
			want:     true,
		},
		{
			name:            "include and exclude patterns both applied",
			startURL:        "https://example.com",
			includePatterns: []string{"/docs/*"},
			excludePatterns: []string{"/docs/internal/*"},
			testURL:         "https://example.com/docs/internal/secret",
			want:            false,
		},
		{
			name:              "deeply nested subdomain allowed when includeSubdomains true",
			startURL:          "https://example.com",
			includeSubdomains: true,
			testURL:           "https://a.b.example.com/page",
			want:              true,
		},
		{
			name:     "domain with port is matched correctly",
			startURL: "https://example.com:8080/docs",
			testURL:  "https://example.com/page",
			want:     true,
		},
		{
			name:     "case insensitive domain matching",
			startURL: "https://Example.COM/docs",
			testURL:  "https://example.com/page",
			want:     true,
		},
		{
			name:            "whitespace in include pattern is trimmed",
			startURL:        "https://example.com",
			includePatterns: []string{"  /docs/*  "},
			testURL:         "https://example.com/docs/intro",
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := model.SupportContentSource{
				StartURL:          tt.startURL,
				IncludeSubdomains: tt.includeSubdomains,
				IncludePatterns:   model.DocsStringArray(tt.includePatterns),
				ExcludePatterns:   model.DocsStringArray(tt.excludePatterns),
			}
			filter := NewURLFilter(source)
			got := filter.Allowed(tt.testURL)
			if got != tt.want {
				t.Errorf("Allowed(%q) = %v, want %v", tt.testURL, got, tt.want)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "strips fragment",
			url:  "https://example.com/page#section",
			want: "https://example.com/page",
		},
		{
			name: "preserves path and query",
			url:  "https://example.com/docs/intro?lang=en",
			want: "https://example.com/docs/intro?lang=en",
		},
		{
			name: "preserves URL without fragment",
			url:  "https://example.com/page",
			want: "https://example.com/page",
		},
		{
			name: "strips fragment but keeps query",
			url:  "https://example.com/page?q=test#frag",
			want: "https://example.com/page?q=test",
		},
		{
			name: "returns invalid URL unchanged",
			url:  "://broken",
			want: "://broken",
		},
		{
			name: "empty string returns empty",
			url:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeURL(tt.url)
			if got != tt.want {
				t.Errorf("NormalizeURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestURLFilterBlogExclusions(t *testing.T) {
	filter := NewURLFilter(model.SupportContentSource{
		StartURL: "https://example.com", IncludeSubdomains: true,
		ExcludePatterns: model.DocsStringArray{"/blog", "/blog/*", "*://blog.*/**"},
	})
	for _, tc := range []struct {
		url     string
		allowed bool
	}{
		{"https://example.com/blog", false},
		{"https://example.com/blog?category=news", false},
		{"https://example.com/blog/guide/intro", false},
		{"https://blog.example.com", false},
		{"https://blog.example.com/", false},
		{"https://BLOG.example.com/news/launch", false},
		{"http://blog.example.com/news/launch", false},
		{"https://example.com/blogging-guide", true},
		{"https://example.com/docs/guide", true},
		{"https://help.example.com/guide", true},
	} {
		t.Run(tc.url, func(t *testing.T) {
			if got := filter.Allowed(tc.url); got != tc.allowed {
				t.Fatalf("Allowed(%s) = %v, want %v", tc.url, got, tc.allowed)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		urlPath string
		pattern string
		want    bool
	}{
		{
			name:    "wildcard suffix matches",
			urlPath: "/docs/intro",
			pattern: "/docs/*",
			want:    true,
		},
		{
			name:    "wildcard suffix does not match different prefix",
			urlPath: "/blog/post",
			pattern: "/docs/*",
			want:    false,
		},
		{
			name:    "wildcard prefix matches",
			urlPath: "/page.html",
			pattern: "*.html",
			want:    true,
		},
		{
			name:    "wildcard prefix does not match different suffix",
			urlPath: "/page.css",
			pattern: "*.html",
			want:    false,
		},
		{
			name:    "exact match",
			urlPath: "/about",
			pattern: "/about",
			want:    true,
		},
		{
			name:    "exact non-match",
			urlPath: "/about",
			pattern: "/contact",
			want:    false,
		},
		{
			name:    "empty pattern never matches",
			urlPath: "/page",
			pattern: "",
			want:    false,
		},
		{
			name:    "path.Match single segment wildcard",
			urlPath: "/docs/intro",
			pattern: "/docs/intro",
			want:    true,
		},
		{
			name:    "wildcard suffix matches nested path",
			urlPath: "/docs/guide/start",
			pattern: "/docs/*",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPattern(tt.urlPath, tt.pattern)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.urlPath, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name:   "standard URL",
			rawURL: "https://example.com/docs",
			want:   "example.com",
		},
		{
			name:   "URL with port",
			rawURL: "https://example.com:8080/docs",
			want:   "example.com",
		},
		{
			name:   "uppercase domain",
			rawURL: "https://EXAMPLE.COM/docs",
			want:   "example.com",
		},
		{
			name:   "invalid URL returns empty",
			rawURL: "://broken",
			want:   "",
		},
		{
			name:   "empty string returns empty",
			rawURL: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDomain(tt.rawURL)
			if got != tt.want {
				t.Errorf("extractDomain(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}
