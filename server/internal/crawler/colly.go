package crawler

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/proxy"
	trafilatura "github.com/markusmobius/go-trafilatura"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// crawlWithColly performs a BFS crawl using Colly for link traversal and
// go-trafilatura for content extraction.
func crawlWithColly(
	ctx context.Context,
	source model.SupportContentSource,
	proxyURLs []string,
	logger *slog.Logger,
	onPage func(CrawlRecord) error,
) (int, error) {
	depth := source.CrawlDepth
	if depth <= 0 {
		depth = 2
	}
	limit := source.CrawlLimit
	if limit <= 0 {
		limit = 100
	}

	filter := NewURLFilter(source)

	collector := colly.NewCollector(
		colly.MaxDepth(depth),
		colly.Async(true),
	)

	// Rate limiting: 5 parallel requests, 500ms delay per domain.
	_ = collector.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 5,
		Delay:       500 * time.Millisecond,
	})

	collector.SetRequestTimeout(30 * time.Second)

	// Proxy rotation (Decodo/Smartproxy or custom).
	if len(proxyURLs) > 0 {
		switcher, err := proxy.RoundRobinProxySwitcher(proxyURLs...)
		if err == nil {
			collector.SetProxyFunc(switcher)
			logger.Info("crawler proxy enabled", "count", len(proxyURLs))
		} else {
			logger.Warn("crawler proxy setup failed, using direct connection", "error", err)
		}
	}

	// Domain scoping.
	baseDomain := extractDomain(source.StartURL)
	allowedDomains := []string{baseDomain}
	if source.IncludeSubdomains {
		allowedDomains = append(allowedDomains, "*."+baseDomain)
	}
	collector.AllowedDomains = allowedDomains

	var (
		pageCount atomic.Int32
		mu        sync.Mutex
		crawlErr  error
	)

	// Check context cancellation before each request.
	collector.OnRequest(func(r *colly.Request) {
		if ctx.Err() != nil {
			r.Abort()
			return
		}
		if int(pageCount.Load()) >= limit {
			r.Abort()
			return
		}
	})

	// Follow links if source discovery allows it.
	if source.CrawlSource != "sitemaps" {
		collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
			if int(pageCount.Load()) >= limit {
				return
			}
			link := e.Request.AbsoluteURL(e.Attr("href"))
			if link == "" {
				return
			}
			link = NormalizeURL(link)
			if !filter.Allowed(link) {
				return
			}
			_ = e.Request.Visit(link)
		})
	}

	// Extract content from each response.
	collector.OnResponse(func(r *colly.Response) {
		if int(pageCount.Load()) >= limit {
			return
		}
		if ctx.Err() != nil {
			return
		}

		// Only process HTML pages.
		contentType := r.Headers.Get("Content-Type")
		if contentType != "" &&
			!bytes.Contains([]byte(contentType), []byte("text/html")) &&
			!bytes.Contains([]byte(contentType), []byte("application/xhtml")) {
			return
		}

		pageURL := r.Request.URL
		parsedURL, _ := url.Parse(pageURL.String())

		// Extract with trafilatura.
		result, err := trafilatura.Extract(bytes.NewReader(r.Body), trafilatura.Options{
			OriginalURL:        parsedURL,
			EnableFallback:     true,
			FallbackCandidates: &trafilatura.FallbackCandidates{},
			Focus:              trafilatura.FavorRecall,
			ExcludeTables:      false,
		})
		if err != nil || result == nil {
			logger.Debug("trafilatura extraction failed", "url", pageURL.String(), "error", err)
			return
		}

		if result.ContentText == "" {
			return
		}

		record := CrawlRecord{
			URL:        pageURL.String(),
			Title:      result.Metadata.Title,
			HTTPStatus: r.StatusCode,
			Markdown:   result.ContentText,
			HTML:       string(r.Body),
			Metadata: map[string]any{
				"title":       result.Metadata.Title,
				"author":      result.Metadata.Author,
				"description": result.Metadata.Description,
				"language":    result.Metadata.Language,
			},
		}

		mu.Lock()
		if crawlErr == nil {
			if err := onPage(record); err != nil {
				crawlErr = fmt.Errorf("onPage callback: %w", err)
			}
		}
		mu.Unlock()

		pageCount.Add(1)
	})

	collector.OnError(func(r *colly.Response, err error) {
		logger.Debug("crawl request failed", "url", r.Request.URL.String(), "status", r.StatusCode, "error", err)
	})

	// Create an HTTP client for sitemap discovery (reuses proxy if configured).
	sitemapClient := &http.Client{Timeout: 15 * time.Second}
	if len(proxyURLs) > 0 {
		if proxyURL, err := url.Parse(proxyURLs[0]); err == nil {
			sitemapClient.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}

	// Seed with sitemap URLs if source discovery allows it.
	if source.CrawlSource != "links" {
		sitemapURLs := discoverSitemapURLs(ctx, sitemapClient, source.StartURL, logger)
		logger.Info("sitemap discovery complete", "urls", len(sitemapURLs))
		for _, u := range sitemapURLs {
			if int(pageCount.Load()) >= limit {
				break
			}
			if filter.Allowed(u) {
				_ = collector.Visit(u)
			}
		}
	}

	// Start from the entry URL if source discovery allows it.
	if source.CrawlSource != "sitemaps" {
		_ = collector.Visit(source.StartURL)
	}

	collector.Wait()

	if crawlErr != nil {
		return int(pageCount.Load()), crawlErr
	}

	logger.Info("local crawl complete",
		"source_id", source.ID,
		"pages", pageCount.Load(),
		"start_url", source.StartURL,
	)
	return int(pageCount.Load()), nil
}
