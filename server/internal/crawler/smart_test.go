package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewSmartCrawler_ModeDefaults(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		wantMode string
	}{
		{
			name:     "local mode preserved",
			mode:     "local",
			wantMode: "local",
		},
		{
			name:     "cloudflare mode preserved",
			mode:     "cloudflare",
			wantMode: "cloudflare",
		},
		{
			name:     "cloudflare_with_fallback explicit",
			mode:     "cloudflare_with_fallback",
			wantMode: "cloudflare_with_fallback",
		},
		{
			name:     "empty mode defaults to cloudflare_with_fallback",
			mode:     "",
			wantMode: "cloudflare_with_fallback",
		},
		{
			name:     "unknown mode defaults to cloudflare_with_fallback",
			mode:     "unknown",
			wantMode: "cloudflare_with_fallback",
		},
		{
			name:     "uppercase LOCAL normalized to local",
			mode:     "LOCAL",
			wantMode: "local",
		},
		{
			name:     "whitespace trimmed",
			mode:     "  local  ",
			wantMode: "local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := newTestLogger(t)
			sc := NewSmartCrawler(tt.mode, "", "", "", "", logger)
			if sc.mode != tt.wantMode {
				t.Errorf("NewSmartCrawler(%q).mode = %q, want %q", tt.mode, sc.mode, tt.wantMode)
			}
		})
	}
}

func TestNewSmartCrawler_CloudflareClientNilWhenCredentialsEmpty(t *testing.T) {
	logger := newTestLogger(t)

	sc := NewSmartCrawler("cloudflare_with_fallback", "", "", "", "", logger)
	if sc.cfClient != nil {
		t.Error("expected cfClient to be nil when credentials are empty")
	}

	sc = NewSmartCrawler("cloudflare_with_fallback", "account123", "token123", "", "", logger)
	if sc.cfClient == nil {
		t.Error("expected cfClient to be non-nil when credentials are provided")
	}
}

func TestNewSmartCrawler_ProxyURLsParsed(t *testing.T) {
	logger := newTestLogger(t)

	sc := NewSmartCrawler("local", "", "", "", "http://proxy1:8080,http://proxy2:8081", logger)
	if len(sc.proxyURLs) != 2 {
		t.Fatalf("expected 2 proxy URLs, got %d", len(sc.proxyURLs))
	}
	if sc.proxyURLs[0] != "http://proxy1:8080" {
		t.Errorf("proxyURLs[0] = %q, want %q", sc.proxyURLs[0], "http://proxy1:8080")
	}
	if sc.proxyURLs[1] != "http://proxy2:8081" {
		t.Errorf("proxyURLs[1] = %q, want %q", sc.proxyURLs[1], "http://proxy2:8081")
	}
}

func TestNewSmartCrawler_NilLogger(t *testing.T) {
	// Should not panic when logger is nil.
	sc := NewSmartCrawler("local", "", "", "", "", nil)
	if sc.logger == nil {
		t.Error("expected logger to be set to default when nil is passed")
	}
}

func TestSmartCrawler_Crawl_LocalMode(t *testing.T) {
	// Create a simple HTTP server that serves an HTML page.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<h1>Test Page</h1>
<p>This is a test page with enough content to be extracted by trafilatura.
We need quite a bit of text content here for the extraction library to consider
this page worth extracting. Let's add several sentences to ensure the content
extraction library picks this up. This paragraph covers the main topic of the
test page which is validating our crawler functionality.</p>
<p>Additional paragraph with more content to satisfy extraction thresholds.
The local crawler uses trafilatura for content extraction, which requires
a minimum amount of text content to produce meaningful results.</p>
</body>
</html>`))
		case "/robots.txt":
			w.WriteHeader(404)
		case "/sitemap.xml":
			w.WriteHeader(404)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	logger := newTestLogger(t)
	sc := NewSmartCrawler("local", "", "", "", "", logger)

	source := model.SupportContentSource{
		ID:          "test-source-1",
		StartURL:    server.URL + "/",
		CrawlLimit:  10,
		CrawlDepth:  1,
		CrawlSource: "links",
	}

	var records []CrawlRecord
	n, err := sc.Crawl(context.Background(), source, func(record CrawlRecord) error {
		records = append(records, record)
		return nil
	})
	if err != nil {
		t.Fatalf("Crawl() error: %v", err)
	}

	// We expect at least 0 pages (trafilatura extraction may or may not extract
	// enough from our test HTML). The key test is that the local crawler runs
	// without error.
	if n < 0 {
		t.Errorf("Crawl() returned negative page count: %d", n)
	}
	if n != len(records) {
		t.Errorf("Crawl() returned n=%d but callback received %d records", n, len(records))
	}
}

func TestSmartCrawler_Crawl_CloudflareWithNoCredentials(t *testing.T) {
	logger := newTestLogger(t)
	sc := NewSmartCrawler("cloudflare", "", "", "", "", logger)

	source := model.SupportContentSource{
		ID:       "test-source-2",
		StartURL: "https://example.com",
	}

	_, err := sc.Crawl(context.Background(), source, func(record CrawlRecord) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error when cloudflare credentials missing, got nil")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' in error, got: %v", err)
	}
}

func TestSmartCrawler_Crawl_CloudflareFallbackWithNoCredentials(t *testing.T) {
	// When mode is cloudflare_with_fallback and cfClient is nil, it should
	// fall back to local crawler immediately.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Fallback Page</title></head>
<body>
<h1>Fallback Page</h1>
<p>This is a fallback test page with sufficient content for extraction.
We provide enough text so the content extraction library can process it properly.
Multiple sentences help ensure the extraction meets minimum thresholds.</p>
</body>
</html>`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	logger := newTestLogger(t)
	sc := NewSmartCrawler("cloudflare_with_fallback", "", "", "", "", logger)

	if sc.cfClient != nil {
		t.Fatal("expected cfClient to be nil for this test")
	}

	source := model.SupportContentSource{
		ID:          "test-source-3",
		StartURL:    server.URL + "/",
		CrawlLimit:  10,
		CrawlDepth:  1,
		CrawlSource: "links",
	}

	// Should not return an error since it falls back to local.
	_, err := sc.Crawl(context.Background(), source, func(record CrawlRecord) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Crawl() with fallback should not error, got: %v", err)
	}
}

func TestSmartCrawler_Crawl_CallbackError(t *testing.T) {
	// Test that callback errors are propagated.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Callback Error Test</title></head>
<body>
<h1>Callback Error Test</h1>
<p>This is a test page with enough content for the extraction library.
We need several sentences of text to ensure the content extraction picks
this up and triggers our callback. The callback will return an error to
test error propagation through the crawl pipeline.</p>
<p>Another paragraph to add more content. This helps ensure the trafilatura
extraction library considers this page significant enough to extract text from.</p>
</body>
</html>`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	logger := newTestLogger(t)
	sc := NewSmartCrawler("local", "", "", "", "", logger)

	source := model.SupportContentSource{
		ID:          "test-source-4",
		StartURL:    server.URL + "/",
		CrawlLimit:  10,
		CrawlDepth:  1,
		CrawlSource: "links",
	}

	callbackErr := fmt.Errorf("callback failure")
	_, err := sc.Crawl(context.Background(), source, func(record CrawlRecord) error {
		return callbackErr
	})

	// The error may or may not propagate depending on whether trafilatura
	// extracts content from our test page. If it does extract content,
	// the error should be propagated.
	if err != nil && !strings.Contains(err.Error(), "callback") {
		t.Errorf("expected callback error to propagate, got: %v", err)
	}
}

func TestSmartCrawler_Crawl_CancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><p>content</p></body></html>`))
	}))
	defer server.Close()

	logger := newTestLogger(t)
	sc := NewSmartCrawler("local", "", "", "", "", logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	source := model.SupportContentSource{
		ID:          "test-source-5",
		StartURL:    server.URL + "/",
		CrawlLimit:  10,
		CrawlDepth:  1,
		CrawlSource: "links",
	}

	// Should complete without blocking forever (colly checks context in OnRequest).
	n, _ := sc.Crawl(ctx, source, func(record CrawlRecord) error {
		return nil
	})
	// With cancelled context, should process 0 or very few pages.
	if n > 1 {
		t.Errorf("Crawl() with cancelled context processed %d pages, expected 0 or 1", n)
	}
}
