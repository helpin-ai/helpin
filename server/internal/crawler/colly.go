package crawler

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
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
		logger.Debug("crawl request started", "url", r.URL.String(), "depth", r.Depth)
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
		rawHTML := string(r.Body)

		var contentText, title string
		var metadata map[string]any

		trafOpts := trafilatura.Options{
			OriginalURL:        parsedURL,
			EnableFallback:     true,
			FallbackCandidates: &trafilatura.FallbackCandidates{},
			Focus:              trafilatura.FavorRecall,
			ExcludeTables:      false,
			IncludeLinks:       true,
		}

		// Article/docs pages → trafilatura primary (best at extracting
		// main body and stripping boilerplate).
		// All other pages → HTML-to-markdown primary (preserves structured
		// content like pricing tables, feature lists, FAQs).
		if isArticlePath(pageURL.String()) {
			result, err := trafilatura.Extract(bytes.NewReader(r.Body), trafOpts)
			if err == nil && result != nil && strings.TrimSpace(result.ContentText) != "" {
				contentText = result.ContentText
				title = result.Metadata.Title
				metadata = map[string]any{
					"title":       result.Metadata.Title,
					"author":      result.Metadata.Author,
					"description": result.Metadata.Description,
					"language":    result.Metadata.Language,
				}
			} else {
				// Trafilatura failed on an article page — fall back.
				contentText = HTMLToMarkdown(rawHTML, parsedURL)
				title = ExtractTitle(rawHTML)
				metadata = map[string]any{"title": title}
			}
		} else {
			contentText = HTMLToMarkdown(rawHTML, parsedURL)
			title = ExtractTitle(rawHTML)
			metadata = map[string]any{"title": title}

			// If trafilatura extracts more for this page, prefer it.
			result, err := trafilatura.Extract(bytes.NewReader(r.Body), trafOpts)
			if err == nil && result != nil {
				trafText := strings.TrimSpace(result.ContentText)
				if len(trafText) > len(strings.TrimSpace(contentText)) {
					contentText = trafText
					title = result.Metadata.Title
					metadata = map[string]any{
						"title":       result.Metadata.Title,
						"author":      result.Metadata.Author,
						"description": result.Metadata.Description,
						"language":    result.Metadata.Language,
					}
				}
			}
		}

		if strings.TrimSpace(contentText) == "" {
			return
		}

		record := CrawlRecord{
			URL:        pageURL.String(),
			Title:      title,
			HTTPStatus: r.StatusCode,
			Markdown:   contentText,
			HTML:       rawHTML,
			Metadata:   metadata,
		}

		// Responses are processed concurrently. Reserve the page while holding
		// the callback lock so several responses cannot all pass the limit check
		// and make the crawl exceed its configured page budget.
		mu.Lock()
		accepted := false
		if crawlErr == nil && int(pageCount.Load()) < limit {
			if err := onPage(record); err != nil {
				crawlErr = fmt.Errorf("onPage callback: %w", err)
			} else {
				pageCount.Add(1)
				accepted = true
			}
		}
		currentPageCount := pageCount.Load()
		mu.Unlock()

		if !accepted {
			return
		}
		logger.Info("crawl page extracted",
			"url", pageURL.String(),
			"title", title,
			"status", r.StatusCode,
			"content_length", len(contentText),
			"page_count", currentPageCount,
			"limit", limit,
		)
	})

	collector.OnError(func(r *colly.Response, err error) {
		logger.Warn("crawl request failed", "url", r.Request.URL.String(), "status", r.StatusCode, "error", err)
	})

	// Create an HTTP client for sitemap discovery (reuses proxy if configured).
	sitemapClient := &http.Client{Timeout: 15 * time.Second}
	if len(proxyURLs) > 0 {
		if proxyURL, err := url.Parse(proxyURLs[0]); err == nil {
			sitemapClient.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}

	// For combined discovery, crawl the entry page and its navigation first.
	// This keeps a large blog sitemap from consuming the entire page budget
	// before core pages linked from the homepage (for example /pricing) are
	// even queued. Sitemaps then fill any remaining capacity with orphaned or
	// deeper pages.
	if source.CrawlSource != "sitemaps" {
		_ = collector.Visit(source.StartURL)
		collector.Wait()
	}

	// Seed with sitemap URLs if source discovery allows it and the link crawl
	// left capacity available.
	if source.CrawlSource != "links" && int(pageCount.Load()) < limit {
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
		collector.Wait()
	}

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

// articlePathSegments are URL path segments that indicate an article/docs page
// where trafilatura's article extraction produces better results than generic
// HTML-to-markdown conversion.
var articlePathSegments = []string{
	"/blog/", "/blog",
	"/docs/", "/docs",
	"/article/", "/articles/",
	"/post/", "/posts/",
	"/guide/", "/guides/",
	"/tutorial/", "/tutorials/",
	"/help/", "/knowledge-base/",
	"/changelog/", "/changelog",
	"/news/", "/news",
	"/wiki/", "/wiki",
}

// isArticlePath returns true if the URL path suggests article/blog/docs content
// where trafilatura excels at extracting the main body.
func isArticlePath(rawURL string) bool {
	parsed, err := url.Parse(strings.ToLower(rawURL))
	if err != nil {
		return false
	}
	path := parsed.Path
	for _, seg := range articlePathSegments {
		if strings.Contains(path, seg) {
			return true
		}
	}
	return false
}
