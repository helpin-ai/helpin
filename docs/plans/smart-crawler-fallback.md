# Smart Crawler with Automatic Fallback

## Overview

The website content sync feature currently relies on Cloudflare Browser Rendering's `/crawl` endpoint. When rate-limited (HTTP 429), syncs fail. This plan introduces a self-hosted fallback using **Colly** (crawling) + **go-trafilatura** (content extraction) that kicks in automatically — no user configuration required.

## Problem

- Cloudflare Browser Rendering has per-account rate limits on the `/crawl` endpoint
- When hit, the sync activity fails and retries (up to 3 attempts), all of which fail if the limit persists
- Users see "Failed" status with no recourse other than waiting

## Solution

A `SmartCrawler` controlled by an environment variable (`CRAWLER_MODE`) with three modes:

| Mode | `CRAWLER_MODE` value | Behavior |
|------|---------------------|----------|
| **Cloudflare only** | `cloudflare` | Uses Cloudflare Browser Rendering exclusively. Fails if rate-limited. |
| **Local only** | `local` | Uses Colly + Trafilatura exclusively. No Cloudflare dependency. |
| **Cloudflare with fallback** | `cloudflare_with_fallback` (default) | Tries Cloudflare first. On 429/rate-limit, auto-falls back to Colly + Trafilatura. |

The mode is configured via environment variable — **not** exposed to end users in the UI. Operators choose the strategy at deployment time based on their infrastructure.

---

## Technology Stack

| Package | Role | Why |
|---------|------|-----|
| [gocolly/colly](https://github.com/gocolly/colly) (25k+ stars) | Web crawling engine | BFS traversal, concurrent fetching, per-domain rate limiting, robots.txt compliance, built-in proxy rotation |
| [markusmobius/go-trafilatura](https://github.com/markusmobius/go-trafilatura) (1k+ stars) | Content extraction | Extracts clean text/markdown from HTML pages, handles complex layouts, multiple fallback extractors |
| [oxffaa/gopher-parse-sitemap](https://github.com/oxffaa/gopher-parse-sitemap) | Sitemap parsing | Lightweight, zero-dependency stream parser for sitemap.xml and sitemap index files |

### How They Work Together

```
┌─────────────────────────────────────────────────────────────┐
│                      SmartCrawler                           │
│                                                             │
│  ┌──────────────────┐     rate limit?    ┌───────────────┐  │
│  │   Cloudflare     │ ──── 429 ────────► │  Local Crawl  │  │
│  │   Browser API    │                    │               │  │
│  │                  │                    │  Sitemap      │  │
│  │  • JS rendering  │                    │  Parser       │  │
│  │  • Async crawl   │                    │    ↓          │  │
│  │  • Full features │                    │  Colly        │  │
│  └────────┬─────────┘                    │  (BFS crawl)  │  │
│           │                              │    ↓          │  │
│           │                              │  Trafilatura  │  │
│           │                              │  (extract)    │  │
│           ▼                              └───────┬───────┘  │
│    ┌──────────────┐                              │          │
│    │  onPage()    │ ◄────────────────────────────┘          │
│    │  callback    │                                         │
│    └──────┬───────┘                                         │
└───────────┼─────────────────────────────────────────────────┘
            ▼
    Chunk → Embed → Store in PostgreSQL
```

---

## Architecture

### Unified Interface

```go
// internal/crawler/crawler.go

// ContentCrawler crawls a website and delivers extracted pages via callback.
type ContentCrawler interface {
    Crawl(ctx context.Context, source model.SupportContentSource,
          onPage func(CrawlRecord) error) (totalDiscovered int, err error)
}

// CrawlRecord is the output for each crawled page.
type CrawlRecord struct {
    URL        string
    Title      string
    HTTPStatus int
    Markdown   string
    HTML       string
    Metadata   map[string]any
}
```

**Why callback-based?**
- Works for both Cloudflare (async poll → iterate) and Colly (sync per-page)
- Enables real-time progress updates and Temporal heartbeating
- No need to hold all pages in memory

### SmartCrawler (Single Implementation)

```go
type SmartCrawler struct {
    mode string // "cloudflare" | "local" | "cloudflare_with_fallback"

    // Cloudflare config (used when mode includes cloudflare)
    cfAccountID string
    cfAPIToken  string
    cfBaseURL   string

    // Local crawler config
    proxyURLs []string
    logger    *slog.Logger
}

func (c *SmartCrawler) Crawl(ctx, source, onPage) (int, error) {
    switch c.mode {
    case "local":
        return c.crawlWithColly(ctx, source, onPage)

    case "cloudflare":
        return c.crawlWithCloudflare(ctx, source, onPage)

    default: // "cloudflare_with_fallback"
        if !c.cloudflareConfigured() {
            c.logger.Warn("cloudflare not configured, using local crawler")
            return c.crawlWithColly(ctx, source, onPage)
        }
        total, err := c.crawlWithCloudflare(ctx, source, onPage)
        if err != nil && isRateLimitError(err) {
            c.logger.Warn("cloudflare rate limited, falling back to local crawler",
                "source_id", source.ID, "error", err)
            return c.crawlWithColly(ctx, source, onPage)
        }
        return total, err
    }
}
```

---

## Detailed Design

### Cloudflare Path (`crawlWithCloudflare`)

Existing logic moved from `support_content_sync.go`:
1. POST to `/browser-rendering/crawl` with source config → get job ID
2. Poll GET with `?limit=1` until status != "running"
3. Paginate completed records using cursor
4. For each record: call `onPage(record)`

### Local Path (`crawlWithColly`)

```go
func (c *SmartCrawler) crawlWithColly(ctx context.Context, source model.SupportContentSource, onPage func(CrawlRecord) error) (int, error) {
    collector := colly.NewCollector(
        colly.MaxDepth(source.CrawlDepth),
        colly.Async(true),
    )

    // Rate limiting: 5 parallel requests, 500ms delay per domain
    collector.Limit(&colly.LimitRule{
        DomainGlob:  "*",
        Parallelism: 5,
        Delay:       500 * time.Millisecond,
    })

    // Proxy rotation (if configured)
    if len(c.proxyURLs) > 0 {
        switcher, _ := proxy.RoundRobinProxySwitcher(c.proxyURLs...)
        collector.SetProxyFunc(switcher)
    }

    // Domain scoping
    baseDomain := extractDomain(source.StartURL)
    collector.AllowedDomains = []string{baseDomain}
    if source.IncludeSubdomains {
        collector.AllowedDomains = append(collector.AllowedDomains, "*."+baseDomain)
    }

    var (
        pageCount atomic.Int32
        mu        sync.Mutex
        crawlErr  error
    )

    // Follow links (if source allows)
    if source.CrawlSource != "sitemaps" {
        collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
            link := e.Request.AbsoluteURL(e.Attr("href"))
            if int(pageCount.Load()) >= source.CrawlLimit { return }
            if !matchesFilters(link, source) { return }
            e.Request.Visit(link)
        })
    }

    // Extract content from each page
    collector.OnResponse(func(r *colly.Response) {
        if int(pageCount.Load()) >= source.CrawlLimit {
            return
        }

        // Extract with trafilatura
        result, err := trafilatura.Extract(bytes.NewReader(r.Body), trafilatura.Options{
            OriginalURL: r.Request.URL,
            OutputFormat: "markdown",
        })
        if err != nil || result == nil { return }

        record := CrawlRecord{
            URL:        r.Request.URL.String(),
            Title:      result.Metadata.Title,
            HTTPStatus: r.StatusCode,
            Markdown:   result.ContentText,
            HTML:       string(r.Body),
        }

        mu.Lock()
        if err := onPage(record); err != nil {
            crawlErr = err
        }
        mu.Unlock()
        pageCount.Add(1)
    })

    // Seed with sitemap URLs
    if source.CrawlSource != "links" {
        sitemapURLs := discoverSitemapURLs(ctx, source.StartURL)
        for _, u := range sitemapURLs {
            if matchesFilters(u, source) {
                collector.Visit(u)
            }
        }
    }

    // Start crawl
    if source.CrawlSource != "sitemaps" {
        collector.Visit(source.StartURL)
    }
    collector.Wait()

    return int(pageCount.Load()), crawlErr
}
```

### Sitemap Discovery

```go
// internal/crawler/sitemap.go
func discoverSitemapURLs(ctx context.Context, startURL string) []string {
    // 1. Try fetching {baseURL}/sitemap.xml
    // 2. Parse with gopher-parse-sitemap (handles index files)
    // 3. If not found, try {baseURL}/robots.txt and extract Sitemap: directives
    // 4. Return deduplicated URL list
}
```

### URL Filtering

```go
// internal/crawler/filter.go
func matchesFilters(candidateURL string, source model.SupportContentSource) bool {
    // 1. Check domain scope (same domain, or subdomains if allowed)
    // 2. If include patterns set: URL must match at least one
    // 3. If exclude patterns set: URL must not match any
    // 4. Normalize URL (strip fragment, trailing slash)
}
```

### Proxy Configuration

```go
// internal/crawler/proxy.go
// Parses CRAWLER_PROXY_URLS env: "socks5://proxy1:1080,http://proxy2:8080"
func parseProxyURLs(commaSeparated string) []string
```

Uses Colly's built-in `proxy.RoundRobinProxySwitcher` — no custom rotation logic needed.

---

## File Changes Summary

### New Files

| File | Purpose |
|------|---------|
| `server/internal/crawler/crawler.go` | `ContentCrawler` interface, `CrawlRecord`, `SmartCrawler` |
| `server/internal/crawler/cloudflare.go` | `crawlWithCloudflare()` — moved from sync service |
| `server/internal/crawler/colly.go` | `crawlWithColly()` — Colly + Trafilatura |
| `server/internal/crawler/sitemap.go` | Sitemap discovery |
| `server/internal/crawler/filter.go` | URL filtering + normalization |
| `server/internal/crawler/proxy.go` | Proxy URL parsing |

### Modified Files

| File | Change |
|------|--------|
| `server/internal/config/config.go` | Add `CrawlerMode` and `CrawlerProxyURLs` fields |
| `server/.env.example` | Add `CRAWLER_MODE=` and `CRAWLER_PROXY_URLS=` |
| `server/internal/service/support_content_sync.go` | Replace `CloudflareCrawler` with `ContentCrawler`, simplify `RunSourceSync` to use callback |
| `server/cmd/api/main.go` | Create `SmartCrawler`, inject into sync service |
| `server/cmd/temporal-worker/main.go` | Same wiring as api/main.go |
| `server/internal/temporalapp/content_source_sync_workflow.go` | Increase timeout to 2hr, add heartbeat |
| `server/go.mod` | Add colly, go-trafilatura, gopher-parse-sitemap |

### No Changes Needed

- No model changes
- No migrations
- No frontend changes — crawler mode is controlled by env var, not user UI

---

## Sync Service Refactor

### Before (current)

```go
type SupportContentSyncService struct {
    crawler CloudflareCrawler  // tightly coupled
    // ...
}

func (s *Service) RunSourceSync(ctx, workspaceID, sourceID) error {
    jobID, _ := s.crawler.StartCrawl(ctx, source)
    job, _ := s.waitForCrawlCompletion(ctx, sourceID, jobID)
    records, _ := s.fetchCompletedRecords(ctx, jobID)
    // batch process all records
    for _, record := range records {
        // upsert page, chunk, embed, store
    }
}
```

### After (simplified)

```go
type SupportContentSyncService struct {
    crawler crawler.ContentCrawler  // single interface
    // ...
}

func (s *Service) RunSourceSync(ctx, workspaceID, sourceID) error {
    source := // fetch from DB
    _, err := s.crawler.Crawl(ctx, *source, func(record crawler.CrawlRecord) error {
        // 1. Extract text from record
        // 2. Upsert page
        // 3. Chunk text
        // 4. Embed chunks
        // 5. Store chunks
        // 6. Update progress + heartbeat
        return nil
    })
    // mark complete or failed
}
```

---

## Environment Variables

| Variable | Required | Default | Example | Purpose |
|----------|----------|---------|---------|---------|
| `CRAWLER_MODE` | No | `cloudflare_with_fallback` | `local` | Controls which crawl engine is used |
| `CLOUDFLARE_ACCOUNT_ID` | If mode includes cloudflare | — | `abc123` | Cloudflare account |
| `CLOUDFLARE_API_TOKEN` | If mode includes cloudflare | — | `token_xxx` | Cloudflare auth |
| `CLOUDFLARE_API_BASE_URL` | No | `https://api.cloudflare.com/client/v4` | — | Custom Cloudflare endpoint |
| `CRAWLER_PROXY_URLS` | No | — | `http://user:pass@gate.decodo.com:10001` | Proxy rotation for local crawler |

### Proxy Configuration with Decodo (Smartproxy)

Decodo provides a single gateway endpoint that auto-rotates IPs on each request — no need for a proxy pool:

| Proxy Type | Endpoint | Port | Use Case |
|------------|----------|------|----------|
| **Residential** | `gate.decodo.com` | `10001` | Better success rate, avoids blocks |
| **Datacenter** | `gate.decodo.com` | `10000` | Faster, cheaper, good for docs sites |

```bash
# Residential (recommended for most sites)
CRAWLER_PROXY_URLS=http://<username>:<password>@gate.decodo.com:10001

# Datacenter (faster, for well-known docs sites)
CRAWLER_PROXY_URLS=http://<username>:<password>@gate.decodo.com:10000

# Both (round-robin between residential and datacenter)
CRAWLER_PROXY_URLS=http://<username>:<password>@gate.decodo.com:10001,http://<username>:<password>@gate.decodo.com:10000
```

Colly's built-in `proxy.RoundRobinProxySwitcher` handles the rotation. With a single Decodo URL, every request automatically gets a different IP from Decodo's pool.

### Deployment Scenarios

- **Full Cloudflare + fallback** (recommended): Set all Cloudflare env vars. Leave `CRAWLER_MODE` empty (defaults to `cloudflare_with_fallback`).
- **Local only** (no Cloudflare account): Set `CRAWLER_MODE=local`. Optionally set `CRAWLER_PROXY_URLS` for Decodo proxies.
- **Cloudflare only** (no fallback): Set `CRAWLER_MODE=cloudflare` + Cloudflare env vars.

If `CRAWLER_MODE` includes cloudflare but credentials are missing, it logs a warning and falls back to local.

---

## Temporal Workflow Changes

| Setting | Before | After | Why |
|---------|--------|-------|-----|
| `StartToCloseTimeout` | 45 min | 2 hours | Local crawler is slower (sequential HTTP) |
| `HeartbeatTimeout` | none | 5 min | Detect stuck activities |

The `onPage` callback calls `activity.RecordHeartbeat(ctx, progress)` after each page.

---

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Cloudflare rate limited (429) | Auto-fallback to local crawler, log warning |
| Cloudflare not configured | Use local crawler directly |
| Both fail | Mark source as failed with error message |
| JS-rendered site + local crawler | Content may be incomplete (trafilatura can't execute JS) — Cloudflare is preferred for SPAs |
| Large site (1000+ pages) | Heartbeating prevents Temporal timeout; CrawlLimit enforced |
| Target site blocks crawler | Colly respects robots.txt; failed pages logged but don't stop the crawl |

---

## Implementation Order

1. **Install dependencies** — `go get` colly, trafilatura, gopher-parse-sitemap
2. **Build crawler package** — types → filter → proxy → sitemap → colly → cloudflare → smart crawler
3. **Config changes** — add `CrawlerMode` + `CrawlerProxyURLs`
4. **Refactor sync service** — replace Cloudflare-specific code with `ContentCrawler` interface
5. **Update DI wiring** — both main.go files
6. **Temporal adjustments** — timeout + heartbeat

## Testing

1. `go build ./...` — compiles
2. `go test ./internal/crawler/...` — unit tests for filter, proxy, sitemap parsing
3. **Integration**: Sync a website → works via Cloudflare
4. **Fallback**: Temporarily return fake 429 from Cloudflare client → auto-falls back to Colly
5. **Local-only**: Remove Cloudflare env vars → uses Colly+Trafilatura directly
6. **Proxy**: Set `CRAWLER_PROXY_URLS` → verify requests go through proxies
