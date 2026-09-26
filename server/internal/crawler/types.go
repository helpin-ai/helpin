// Package crawler provides a unified web content crawler with automatic
// fallback between Cloudflare Browser Rendering and a local Colly + Trafilatura
// pipeline.
package crawler

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ContentCrawler crawls a website and delivers extracted pages via callback.
type ContentCrawler interface {
	// Crawl performs the full crawl for the given source and calls onPage for
	// each successfully extracted page. It returns the total number of pages
	// processed and any terminal error.
	Crawl(ctx context.Context, source model.SupportContentSource, onPage func(CrawlRecord) error) (int, error)
}

// CrawlRecord is the unified output for each crawled page.
type CrawlRecord struct {
	// SkipReason is set for policy skips; these records contain no content.
	SkipReason string
	URL        string
	Title      string
	HTTPStatus int
	Markdown   string
	HTML       string
	Metadata   map[string]any
}
