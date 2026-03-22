package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SmartCrawler implements ContentCrawler with automatic backend selection and
// optional fallback from Cloudflare to a local Colly + Trafilatura pipeline.
type SmartCrawler struct {
	mode      string // "cloudflare", "local", "cloudflare_with_fallback"
	cfClient  *CloudflareCrawlClient
	proxyURLs []string
	logger    *slog.Logger
}

// NewSmartCrawler creates a SmartCrawler that routes crawl requests to the
// appropriate backend based on mode. Supported modes:
//   - "local": always use the local Colly + Trafilatura crawler
//   - "cloudflare": always use the Cloudflare Browser Rendering API
//   - "cloudflare_with_fallback" (default): try Cloudflare first, fall back to
//     local on rate-limit errors (429 / "rate limit")
func NewSmartCrawler(mode, cfAccountID, cfAPIToken, cfBaseURL, proxyURLs string, logger *slog.Logger) *SmartCrawler {
	if logger == nil {
		logger = slog.Default()
	}
	normalizedMode := strings.TrimSpace(strings.ToLower(mode))
	switch normalizedMode {
	case "local", "cloudflare":
		// valid
	default:
		normalizedMode = "cloudflare_with_fallback"
	}

	return &SmartCrawler{
		mode:      normalizedMode,
		cfClient:  NewCloudflareCrawlClient(cfAccountID, cfAPIToken, cfBaseURL),
		proxyURLs: ParseProxyURLs(proxyURLs),
		logger:    logger.With("component", "smart_crawler"),
	}
}

// Crawl performs a website crawl using the configured backend and delivers each
// extracted page via the onPage callback. Returns the total number of pages
// processed and any terminal error.
func (s *SmartCrawler) Crawl(ctx context.Context, source model.SupportContentSource, onPage func(CrawlRecord) error) (int, error) {
	switch s.mode {
	case "local":
		return s.crawlLocal(ctx, source, onPage)

	case "cloudflare":
		return s.crawlCloudflare(ctx, source, onPage)

	default: // "cloudflare_with_fallback"
		return s.crawlWithFallback(ctx, source, onPage)
	}
}

func (s *SmartCrawler) crawlLocal(ctx context.Context, source model.SupportContentSource, onPage func(CrawlRecord) error) (int, error) {
	s.logger.Info("using local crawler", "source_id", source.ID, "start_url", source.StartURL)
	return crawlWithColly(ctx, source, s.proxyURLs, s.logger, onPage)
}

func (s *SmartCrawler) crawlCloudflare(ctx context.Context, source model.SupportContentSource, onPage func(CrawlRecord) error) (int, error) {
	if s.cfClient == nil {
		return 0, fmt.Errorf("cloudflare crawl client is not configured")
	}
	s.logger.Info("using cloudflare crawler", "source_id", source.ID, "start_url", source.StartURL)
	return crawlWithCloudflare(ctx, s.cfClient, source, s.logger, onPage)
}

func (s *SmartCrawler) crawlWithFallback(ctx context.Context, source model.SupportContentSource, onPage func(CrawlRecord) error) (int, error) {
	// If Cloudflare client is not configured, go straight to local.
	if s.cfClient == nil {
		s.logger.Info("cloudflare not configured, falling back to local crawler",
			"source_id", source.ID,
			"start_url", source.StartURL,
		)
		return s.crawlLocal(ctx, source, onPage)
	}

	// Try Cloudflare first.
	n, err := s.crawlCloudflare(ctx, source, onPage)
	if err == nil {
		return n, nil
	}

	// Fall back to local on rate-limit errors.
	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "429") || strings.Contains(errMsg, "rate limit") {
		s.logger.Warn("cloudflare rate limited, retrying with local crawler",
			"source_id", source.ID,
			"start_url", source.StartURL,
			"error", err,
		)
		return s.crawlLocal(ctx, source, onPage)
	}

	// Non-rate-limit error: propagate.
	return 0, err
}
