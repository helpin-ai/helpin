//go:build integration

package crawler

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// TestCollyProxyCrawlContentpen verifies that the Colly crawler can reach
// contentpen.ai through the SmartProxy (Decodo) proxy and extract content.
//
// Run with:
//
//	cd server && go test -tags=integration -run TestCollyProxyCrawlContentpen -v ./internal/crawler/
//
// Requires CRAWLER_PROXY_URLS env var set to valid Decodo proxy URLs.
func TestCollyProxyCrawlContentpen(t *testing.T) {
	proxyRaw := os.Getenv("CRAWLER_PROXY_URLS")
	if proxyRaw == "" {
		t.Skip("CRAWLER_PROXY_URLS not set — skipping proxy crawl test")
	}

	proxyURLs := ParseProxyURLs(proxyRaw)
	if len(proxyURLs) == 0 {
		t.Fatal("CRAWLER_PROXY_URLS parsed to zero URLs")
	}
	t.Logf("using %d proxy URLs", len(proxyURLs))

	source := model.SupportContentSource{
		StartURL:          "https://contentpen.ai",
		CrawlDepth:        2,
		CrawlLimit:        10,
		IncludeSubdomains: false,
		CrawlSource:       "links",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var mu sync.Mutex
	var pages []CrawlRecord

	count, err := crawlWithColly(ctx, source, proxyURLs, integrationTestLogger(t), func(record CrawlRecord) error {
		mu.Lock()
		defer mu.Unlock()
		pages = append(pages, record)
		t.Logf("page %d: %s (status %d, %d bytes)", len(pages), record.URL, record.HTTPStatus, len(record.Markdown))
		return nil
	})

	if err != nil {
		t.Fatalf("crawlWithColly returned error: %v", err)
	}

	if count == 0 {
		t.Fatal("crawlWithColly returned 0 pages — proxy may be blocked or misconfigured")
	}

	t.Logf("total pages crawled: %d", count)

	// Verify we got the homepage at minimum.
	var foundHomepage bool
	for _, p := range pages {
		if p.URL == "https://contentpen.ai" || p.URL == "https://contentpen.ai/" {
			foundHomepage = true
			if p.HTTPStatus != 200 {
				t.Errorf("homepage returned status %d, want 200", p.HTTPStatus)
			}
			if len(strings.TrimSpace(p.Markdown)) < 50 {
				t.Errorf("homepage content too short (%d chars) — extraction may have failed", len(p.Markdown))
			}
			// Log a preview.
			preview := p.Markdown
			if len(preview) > 500 {
				preview = preview[:500] + "..."
			}
			t.Logf("homepage title: %s", p.Title)
			t.Logf("homepage preview:\n%s", preview)
		}
	}
	if !foundHomepage {
		t.Error("homepage (contentpen.ai) not found in crawl results")
	}

	// Log all crawled URLs for visibility.
	t.Log("--- all crawled URLs ---")
	for _, p := range pages {
		t.Logf("  %s  [%d] title=%q  content=%d bytes", p.URL, p.HTTPStatus, p.Title, len(p.Markdown))
	}
}

// TestCollyDirectCrawlContentpen tests crawling WITHOUT proxy to compare.
func TestCollyDirectCrawlContentpen(t *testing.T) {
	source := model.SupportContentSource{
		StartURL:          "https://contentpen.ai",
		CrawlDepth:        1,
		CrawlLimit:        3,
		IncludeSubdomains: false,
		CrawlSource:       "links",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var mu sync.Mutex
	var pages []CrawlRecord

	count, err := crawlWithColly(ctx, source, nil, integrationTestLogger(t), func(record CrawlRecord) error {
		mu.Lock()
		defer mu.Unlock()
		pages = append(pages, record)
		t.Logf("page: %s (status %d, %d bytes)", record.URL, record.HTTPStatus, len(record.Markdown))
		return nil
	})

	if err != nil {
		t.Fatalf("direct crawl error: %v", err)
	}

	t.Logf("direct crawl: %d pages", count)
	for _, p := range pages {
		t.Logf("  %s  [%d] title=%q", p.URL, p.HTTPStatus, p.Title)
	}
}

func integrationTestLogger(t *testing.T) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
