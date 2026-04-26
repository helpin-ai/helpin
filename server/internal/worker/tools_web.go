package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"
)

const braveSearchAPIURL = "https://api.search.brave.com/res/v1/web/search"
const exaSearchAPIURL = "https://api.exa.ai/search"
const defaultWebFetchUserAgent = "HelpinAgent/1.0 (+https://helpin.ai)"
const maxWebFetchBodyBytes = 2 * 1024 * 1024
const maxWebFetchLinks = 80

var webFetchHTTPClient = &http.Client{Timeout: 20 * time.Second}
var allowPrivateWebFetchHostsForTests bool

var allowedExaSearchTypes = map[string]struct{}{
	"auto":           {},
	"neural":         {},
	"fast":           {},
	"instant":        {},
	"deep-lite":      {},
	"deep":           {},
	"deep-reasoning": {},
}

var allowedExaCategories = map[string]struct{}{
	"company":          {},
	"research paper":   {},
	"news":             {},
	"personal site":    {},
	"financial report": {},
	"people":           {},
}

type WebSearchClient interface {
	Search(ctx context.Context, query WebSearchQuery) ([]WebSearchResult, error)
}

type WebSearchQuery struct {
	Query           string
	Count           int
	Freshness       string
	DomainAllowlist []string
}

type WebSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet"`
	PublishedAt string `json:"published_at,omitempty"`
}

type BraveSearchClient struct {
	apiKey     string
	httpClient *http.Client
}

type ExaSearchClient struct {
	apiKey     string
	apiURL     string
	httpClient *http.Client
}

type ExaSearchRequest struct {
	Query              string             `json:"query"`
	AdditionalQueries  []string           `json:"additionalQueries,omitempty"`
	SystemPrompt       string             `json:"systemPrompt,omitempty"`
	Type               string             `json:"type,omitempty"`
	Category           string             `json:"category,omitempty"`
	UserLocation       string             `json:"userLocation,omitempty"`
	NumResults         int                `json:"numResults,omitempty"`
	IncludeDomains     []string           `json:"includeDomains,omitempty"`
	ExcludeDomains     []string           `json:"excludeDomains,omitempty"`
	StartPublishedDate string             `json:"startPublishedDate,omitempty"`
	EndPublishedDate   string             `json:"endPublishedDate,omitempty"`
	StartCrawlDate     string             `json:"startCrawlDate,omitempty"`
	EndCrawlDate       string             `json:"endCrawlDate,omitempty"`
	Moderation         bool               `json:"moderation,omitempty"`
	Contents           *ExaSearchContents `json:"contents,omitempty"`
	OutputSchema       map[string]any     `json:"outputSchema,omitempty"`
}

type ExaSearchContents struct {
	Text          *ExaTextConfig       `json:"text,omitempty"`
	Highlights    *ExaHighlightsConfig `json:"highlights,omitempty"`
	Summary       *ExaSummaryConfig    `json:"summary,omitempty"`
	MaxAgeHours   *int                 `json:"maxAgeHours,omitempty"`
	Subpages      *int                 `json:"subpages,omitempty"`
	SubpageTarget any                  `json:"subpageTarget,omitempty"`
	Extras        *ExaExtrasConfig     `json:"extras,omitempty"`
}

type ExaTextConfig struct {
	MaxCharacters   int  `json:"maxCharacters,omitempty"`
	IncludeHTMLTags bool `json:"includeHtmlTags,omitempty"`
}

type ExaHighlightsConfig struct {
	MaxCharacters int    `json:"maxCharacters,omitempty"`
	Query         string `json:"query,omitempty"`
}

type ExaSummaryConfig struct {
	Query  string         `json:"query,omitempty"`
	Schema map[string]any `json:"schema,omitempty"`
}

type ExaExtrasConfig struct {
	Links      *int `json:"links,omitempty"`
	ImageLinks *int `json:"imageLinks,omitempty"`
}

type ExaSearchResponse struct {
	RequestID   string            `json:"requestId"`
	SearchType  string            `json:"searchType"`
	Results     []ExaSearchResult `json:"results"`
	Output      *ExaOutput        `json:"output,omitempty"`
	CostDollars map[string]any    `json:"costDollars,omitempty"`
}

type ExaSearchResult struct {
	Title           string            `json:"title"`
	URL             string            `json:"url"`
	ID              string            `json:"id"`
	PublishedDate   *string           `json:"publishedDate,omitempty"`
	Author          *string           `json:"author,omitempty"`
	Image           string            `json:"image,omitempty"`
	Favicon         string            `json:"favicon,omitempty"`
	Text            string            `json:"text,omitempty"`
	Highlights      []string          `json:"highlights,omitempty"`
	HighlightScores []float64         `json:"highlightScores,omitempty"`
	Summary         string            `json:"summary,omitempty"`
	Subpages        []ExaSearchResult `json:"subpages,omitempty"`
	Extras          map[string]any    `json:"extras,omitempty"`
}

type ExaOutput struct {
	Content   any            `json:"content,omitempty"`
	Grounding []ExaGrounding `json:"grounding,omitempty"`
}

type ExaGrounding struct {
	Field      string        `json:"field,omitempty"`
	Citations  []ExaCitation `json:"citations,omitempty"`
	Confidence string        `json:"confidence,omitempty"`
}

type ExaCitation struct {
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
}

type exaSearchToolInput struct {
	Query              string                  `json:"query"`
	Type               string                  `json:"type"`
	NumResults         int                     `json:"num_results"`
	Category           string                  `json:"category"`
	UserLocation       string                  `json:"user_location"`
	IncludeDomains     []string                `json:"include_domains"`
	ExcludeDomains     []string                `json:"exclude_domains"`
	StartPublishedDate string                  `json:"start_published_date"`
	EndPublishedDate   string                  `json:"end_published_date"`
	StartCrawlDate     string                  `json:"start_crawl_date"`
	EndCrawlDate       string                  `json:"end_crawl_date"`
	AdditionalQueries  []string                `json:"additional_queries"`
	SystemPrompt       string                  `json:"system_prompt"`
	Moderation         bool                    `json:"moderation"`
	Contents           *exaSearchContentsInput `json:"contents"`
	OutputSchema       map[string]any          `json:"output_schema"`
}

type exaSearchContentsInput struct {
	Text          *exaTextInput       `json:"text"`
	Highlights    *exaHighlightsInput `json:"highlights"`
	Summary       *exaSummaryInput    `json:"summary"`
	MaxAgeHours   *int                `json:"max_age_hours"`
	Subpages      *int                `json:"subpages"`
	SubpageTarget any                 `json:"subpage_target"`
	Extras        *exaExtrasInput     `json:"extras"`
}

type exaTextInput struct {
	MaxCharacters   int  `json:"max_characters"`
	IncludeHTMLTags bool `json:"include_html_tags"`
}

type exaHighlightsInput struct {
	MaxCharacters int    `json:"max_characters"`
	Query         string `json:"query"`
}

type exaSummaryInput struct {
	Query  string         `json:"query"`
	Schema map[string]any `json:"schema"`
}

type exaExtrasInput struct {
	Links      *int `json:"links"`
	ImageLinks *int `json:"image_links"`
}

type fetchURLToolInput struct {
	URL           string `json:"url"`
	MaxCharacters int    `json:"max_characters"`
	IncludeHTML   bool   `json:"include_html"`
}

type crawlURLToolInput struct {
	URL                  string   `json:"url"`
	MaxPages             int      `json:"max_pages"`
	MaxDepth             int      `json:"max_depth"`
	MaxCharactersPerPage int      `json:"max_characters_per_page"`
	IncludePatterns      []string `json:"include_patterns"`
	PathKeywords         []string `json:"path_keywords"`
}

type exaToolResponse struct {
	Query       string                  `json:"query"`
	RequestID   string                  `json:"request_id,omitempty"`
	SearchType  string                  `json:"search_type,omitempty"`
	ResultCount int                     `json:"result_count"`
	Results     []exaToolResponseResult `json:"results"`
	Output      *ExaOutput              `json:"output,omitempty"`
	CostDollars map[string]any          `json:"cost_dollars,omitempty"`
}

type exaToolResponseResult struct {
	Title           string                  `json:"title"`
	URL             string                  `json:"url"`
	ID              string                  `json:"id,omitempty"`
	PublishedDate   *string                 `json:"published_date,omitempty"`
	Author          *string                 `json:"author,omitempty"`
	Image           string                  `json:"image,omitempty"`
	Favicon         string                  `json:"favicon,omitempty"`
	Text            string                  `json:"text,omitempty"`
	Highlights      []string                `json:"highlights,omitempty"`
	HighlightScores []float64               `json:"highlight_scores,omitempty"`
	Summary         string                  `json:"summary,omitempty"`
	Subpages        []exaToolResponseResult `json:"subpages,omitempty"`
	Extras          map[string]any          `json:"extras,omitempty"`
}

type fetchURLToolResponse struct {
	URL         string                 `json:"url"`
	FinalURL    string                 `json:"final_url,omitempty"`
	Status      int                    `json:"status"`
	ContentType string                 `json:"content_type,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Text        string                 `json:"text,omitempty"`
	HTML        string                 `json:"html,omitempty"`
	Links       []webPageLink          `json:"links,omitempty"`
	Truncated   bool                   `json:"truncated,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type crawlURLToolResponse struct {
	URL            string                 `json:"url"`
	MaxPages       int                    `json:"max_pages"`
	MaxDepth       int                    `json:"max_depth"`
	PageCount      int                    `json:"page_count"`
	Pages          []fetchURLToolResponse `json:"pages"`
	SkippedLinks   int                    `json:"skipped_links,omitempty"`
	DiscoveredURLs []string               `json:"discovered_urls,omitempty"`
}

type webPageLink struct {
	Text string `json:"text,omitempty"`
	URL  string `json:"url"`
}

type fetchedWebPage struct {
	response fetchURLToolResponse
	links    []webPageLink
}

func NewBraveSearchClient(apiKey string) *BraveSearchClient {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil
	}
	return &BraveSearchClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func NewExaSearchClient(apiKey string) *ExaSearchClient {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil
	}
	return &ExaSearchClient{
		apiKey: apiKey,
		apiURL: exaSearchAPIURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *BraveSearchClient) Search(ctx context.Context, query WebSearchQuery) ([]WebSearchResult, error) {
	if c == nil || c.apiKey == "" {
		return nil, fmt.Errorf("brave search is not configured")
	}

	count := query.Count
	if count <= 0 {
		count = 5
	}
	if count > 10 {
		count = 10
	}

	apiURL, err := url.Parse(braveSearchAPIURL)
	if err != nil {
		return nil, fmt.Errorf("parse brave search URL: %w", err)
	}

	params := apiURL.Query()
	params.Set("q", formatWebSearchQuery(query.Query, query.DomainAllowlist))
	params.Set("count", fmt.Sprintf("%d", count))
	if freshness := strings.TrimSpace(query.Freshness); freshness != "" {
		params.Set("freshness", freshness)
	}
	apiURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create brave search request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send brave search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brave search API returned status %d", resp.StatusCode)
	}

	var payload struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Age         string `json:"age"`
				PageAge     string `json:"page_age"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode brave search response: %w", err)
	}

	results := make([]WebSearchResult, 0, len(payload.Web.Results))
	for _, item := range payload.Web.Results {
		title := strings.TrimSpace(item.Title)
		resultURL := strings.TrimSpace(item.URL)
		snippet := strings.TrimSpace(item.Description)
		if title == "" || resultURL == "" {
			continue
		}
		results = append(results, WebSearchResult{
			Title:       title,
			URL:         resultURL,
			Snippet:     snippet,
			PublishedAt: firstNonEmpty(strings.TrimSpace(item.PageAge), strings.TrimSpace(item.Age)),
		})
	}

	return results, nil
}

func (c *ExaSearchClient) Search(ctx context.Context, query ExaSearchRequest) (*ExaSearchResponse, error) {
	if c == nil || c.apiKey == "" {
		return nil, fmt.Errorf("exa search is not configured")
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal exa search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create exa search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send exa search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return nil, fmt.Errorf("exa search API returned status %d", resp.StatusCode)
		}
		message := strings.TrimSpace(string(body))
		if message == "" {
			return nil, fmt.Errorf("exa search API returned status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("exa search API returned status %d: %s", resp.StatusCode, message)
	}

	var payload ExaSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode exa search response: %w", err)
	}
	return &payload, nil
}

func (r *ToolRegistry) toolWebSearchBrave(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("execution context is required")
	}
	if r == nil || r.webSearch == nil {
		return "", fmt.Errorf("brave search is not configured on this worker")
	}

	var params struct {
		Query           string   `json:"query"`
		Count           int      `json:"count"`
		Freshness       string   `json:"freshness"`
		DomainAllowlist []string `json:"domain_allowlist"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(params.Query) == "" {
		return "", fmt.Errorf("query is required")
	}

	results, err := r.webSearch.Search(ctx.Context, WebSearchQuery{
		Query:           strings.TrimSpace(params.Query),
		Count:           params.Count,
		Freshness:       strings.TrimSpace(params.Freshness),
		DomainAllowlist: params.DomainAllowlist,
	})
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(map[string]any{
		"query":   strings.TrimSpace(params.Query),
		"results": results,
	})
	if err != nil {
		return "", fmt.Errorf("marshal search results: %w", err)
	}
	return string(payload), nil
}

func (r *ToolRegistry) toolWebSearchExa(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("execution context is required")
	}
	if r == nil || r.exaSearch == nil {
		return "", fmt.Errorf("exa search is not configured on this worker")
	}

	var params exaSearchToolInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	request, err := buildExaSearchRequest(params)
	if err != nil {
		return "", err
	}

	results, err := r.exaSearch.Search(ctx.Context, request)
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(exaToolResponse{
		Query:       request.Query,
		RequestID:   strings.TrimSpace(results.RequestID),
		SearchType:  strings.TrimSpace(results.SearchType),
		ResultCount: len(results.Results),
		Results:     normalizeExaToolResults(results.Results),
		Output:      results.Output,
		CostDollars: results.CostDollars,
	})
	if err != nil {
		return "", fmt.Errorf("marshal exa search results: %w", err)
	}
	return string(payload), nil
}

func toolFetchURL(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("execution context is required")
	}
	var params fetchURLToolInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	page, err := fetchWebPage(ctx.Context, params.URL, clampWebFetchCharacters(params.MaxCharacters), params.IncludeHTML)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(page.response)
	if err != nil {
		return "", fmt.Errorf("marshal fetch_url result: %w", err)
	}
	return string(payload), nil
}

func toolCrawlURL(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("execution context is required")
	}
	var params crawlURLToolInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	root, err := parseAllowedWebFetchURL(params.URL)
	if err != nil {
		return "", err
	}
	maxPages := clampCrawlMaxPages(params.MaxPages)
	maxDepth := clampCrawlMaxDepth(params.MaxDepth)
	maxChars := clampCrawlPageCharacters(params.MaxCharactersPerPage)
	includePatterns := normalizeCrawlPatterns(params.IncludePatterns)
	pathKeywords := normalizeCrawlPatterns(params.PathKeywords)
	if len(pathKeywords) == 0 {
		pathKeywords = defaultCrawlPathKeywords()
	}

	type queuedURL struct {
		url   string
		depth int
	}
	queue := []queuedURL{{url: root.String(), depth: 0}}
	seen := map[string]bool{canonicalWebURL(root): true}
	pages := make([]fetchURLToolResponse, 0, maxPages)
	discovered := make([]string, 0)
	skipped := 0

	for len(queue) > 0 && len(pages) < maxPages {
		next := queue[0]
		queue = queue[1:]
		page, err := fetchWebPage(ctx.Context, next.url, maxChars, false)
		if err != nil {
			skipped++
			continue
		}
		if page.response.Status >= http.StatusBadRequest {
			skipped++
			continue
		}
		pages = append(pages, page.response)
		if next.depth >= maxDepth {
			continue
		}
		candidates := filterCrawlLinks(root, page.links, includePatterns, pathKeywords)
		for _, link := range candidates {
			parsed, err := parseAllowedWebFetchURL(link.URL)
			if err != nil || !sameWebHost(root, parsed) {
				skipped++
				continue
			}
			key := canonicalWebURL(parsed)
			if seen[key] {
				continue
			}
			seen[key] = true
			queue = append(queue, queuedURL{url: parsed.String(), depth: next.depth + 1})
			if len(discovered) < maxWebFetchLinks {
				discovered = append(discovered, parsed.String())
			}
		}
	}

	result := crawlURLToolResponse{
		URL:            root.String(),
		MaxPages:       maxPages,
		MaxDepth:       maxDepth,
		PageCount:      len(pages),
		Pages:          pages,
		SkippedLinks:   skipped,
		DiscoveredURLs: discovered,
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal crawl_url result: %w", err)
	}
	return string(payload), nil
}

func webSearchBraveToolDescription() string {
	return "Search the public web with Brave Search. Use this for market context, standards, competitors, and external evidence. Returns normalized JSON results."
}

func webSearchBraveToolSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query to run",
			},
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results to return (default 5, max 10)",
			},
			"freshness": map[string]interface{}{
				"type":        "string",
				"description": "Optional freshness hint such as pd, pw, pm, or py",
			},
			"domain_allowlist": map[string]interface{}{
				"type":        "array",
				"description": "Optional list of domains to prioritize",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"query"},
	}
}

func fetchURLToolDescription() string {
	return "Fetch a specific public URL and return extracted title, description, readable text, and links. Use this after finding or knowing an exact changelog, release notes, blog, docs, or source URL."
}

func fetchURLToolSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "Public http(s) URL to fetch directly.",
			},
			"max_characters": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum extracted text characters to return. Defaults to 12000, max 30000.",
			},
			"include_html": map[string]interface{}{
				"type":        "boolean",
				"description": "When true, include a truncated raw HTML excerpt. Prefer false unless structure matters.",
			},
		},
		"required": []string{"url"},
	}
}

func crawlURLToolDescription() string {
	return "Crawl a small number of same-host public pages from a starting URL, prioritizing changelog, release notes, updates, announcements, docs, and roadmap paths. Use this to discover official update pages when search results are thin."
}

func crawlURLToolSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "Public http(s) URL to start crawling from.",
			},
			"max_pages": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum pages to fetch. Defaults to 8, max 20.",
			},
			"max_depth": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum same-host link depth. Defaults to 1, max 2.",
			},
			"max_characters_per_page": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum extracted text characters per page. Defaults to 6000, max 15000.",
			},
			"include_patterns": map[string]interface{}{
				"type":        "array",
				"description": "Optional URL/text substrings to include, such as changelog or release-notes.",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"path_keywords": map[string]interface{}{
				"type":        "array",
				"description": "Optional path/link-text keywords to prioritize. Defaults to common update-page keywords.",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"url"},
	}
}

func webSearchExaToolDescription() string {
	return "Search the web with Exa's neural search engine. Supports category filters, semantic search types, content extraction, domain filters, freshness controls, and synthesized structured output. Returns JSON results with titles, URLs, and extracted content."
}

func webSearchExaToolSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Natural-language search query",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Search type: auto (default), neural, fast, instant, deep-lite, deep, or deep-reasoning",
				"enum":        []string{"auto", "neural", "fast", "instant", "deep-lite", "deep", "deep-reasoning"},
			},
			"num_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results to return (default 5, max 10 in this tool)",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"description": "Optional category: company, research paper, news, personal site, financial report, or people",
				"enum":        []string{"company", "research paper", "news", "personal site", "financial report", "people"},
			},
			"user_location": map[string]interface{}{
				"type":        "string",
				"description": "Optional two-letter ISO country code such as US to bias results geographically",
			},
			"include_domains": map[string]interface{}{
				"type":        "array",
				"description": "Only return results from these domains",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"exclude_domains": map[string]interface{}{
				"type":        "array",
				"description": "Exclude results from these domains. Not supported for company or people categories",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"start_published_date": map[string]interface{}{
				"type":        "string",
				"description": "Only return links published after this ISO 8601 timestamp",
			},
			"end_published_date": map[string]interface{}{
				"type":        "string",
				"description": "Only return links published before this ISO 8601 timestamp",
			},
			"start_crawl_date": map[string]interface{}{
				"type":        "string",
				"description": "Only return links crawled after this ISO 8601 timestamp",
			},
			"end_crawl_date": map[string]interface{}{
				"type":        "string",
				"description": "Only return links crawled before this ISO 8601 timestamp",
			},
			"additional_queries": map[string]interface{}{
				"type":        "array",
				"description": "Optional extra query variations for deep-search modes",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"system_prompt": map[string]interface{}{
				"type":        "string",
				"description": "Optional synthesis instructions for output shaping and source preferences",
			},
			"moderation": map[string]interface{}{
				"type":        "boolean",
				"description": "Filter unsafe content from results",
			},
			"output_schema": map[string]interface{}{
				"type":        "object",
				"description": "Optional JSON schema for synthesized output.content",
			},
			"contents": map[string]interface{}{
				"type":        "object",
				"description": "Optional content extraction settings. Choose exactly one of text, highlights, or summary. Defaults to highlights with max_characters 4000 when omitted.",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"max_characters":    map[string]interface{}{"type": "integer"},
							"include_html_tags": map[string]interface{}{"type": "boolean"},
						},
					},
					"highlights": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"max_characters": map[string]interface{}{"type": "integer"},
							"query":          map[string]interface{}{"type": "string"},
						},
					},
					"summary": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query":  map[string]interface{}{"type": "string"},
							"schema": map[string]interface{}{"type": "object"},
						},
					},
					"max_age_hours": map[string]interface{}{
						"type":        "integer",
						"description": "0 always livecrawls, -1 uses cache only",
					},
					"subpages": map[string]interface{}{
						"type":        "integer",
						"description": "Optional number of subpages to crawl per result",
					},
					"subpage_target": map[string]interface{}{
						"description": "Optional keyword or structure used to prioritize subpages",
					},
					"extras": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"links":       map[string]interface{}{"type": "integer"},
							"image_links": map[string]interface{}{"type": "integer"},
						},
					},
				},
			},
		},
		"required": []string{"query"},
	}
}

func formatWebSearchQuery(query string, domainAllowlist []string) string {
	query = strings.TrimSpace(query)
	if len(domainAllowlist) == 0 {
		return query
	}

	siteTerms := make([]string, 0, len(domainAllowlist))
	for _, domain := range domainAllowlist {
		domain = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://"))
		domain = strings.TrimSuffix(domain, "/")
		if domain == "" {
			continue
		}
		siteTerms = append(siteTerms, "site:"+domain)
	}
	if len(siteTerms) == 0 {
		return query
	}

	return fmt.Sprintf("(%s) %s", strings.Join(siteTerms, " OR "), query)
}

func buildExaSearchRequest(input exaSearchToolInput) (ExaSearchRequest, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return ExaSearchRequest{}, fmt.Errorf("query is required")
	}

	request := ExaSearchRequest{
		Query:              query,
		AdditionalQueries:  normalizeStringList(input.AdditionalQueries),
		SystemPrompt:       strings.TrimSpace(input.SystemPrompt),
		Type:               firstNonEmpty(strings.TrimSpace(input.Type), "auto"),
		Category:           strings.TrimSpace(input.Category),
		UserLocation:       strings.ToUpper(strings.TrimSpace(input.UserLocation)),
		NumResults:         clampExaNumResults(input.NumResults),
		IncludeDomains:     normalizeDomainList(input.IncludeDomains),
		ExcludeDomains:     normalizeDomainList(input.ExcludeDomains),
		StartPublishedDate: strings.TrimSpace(input.StartPublishedDate),
		EndPublishedDate:   strings.TrimSpace(input.EndPublishedDate),
		StartCrawlDate:     strings.TrimSpace(input.StartCrawlDate),
		EndCrawlDate:       strings.TrimSpace(input.EndCrawlDate),
		Moderation:         input.Moderation,
		OutputSchema:       normalizeObject(input.OutputSchema),
		Contents:           buildExaContents(input.Contents),
	}

	if err := validateExaSearchRequest(request); err != nil {
		return ExaSearchRequest{}, err
	}
	return request, nil
}

func buildExaContents(input *exaSearchContentsInput) *ExaSearchContents {
	if input == nil {
		return defaultExaContents()
	}

	contents := &ExaSearchContents{
		MaxAgeHours:   input.MaxAgeHours,
		Subpages:      input.Subpages,
		SubpageTarget: input.SubpageTarget,
	}
	if input.Text != nil {
		contents.Text = &ExaTextConfig{
			MaxCharacters:   input.Text.MaxCharacters,
			IncludeHTMLTags: input.Text.IncludeHTMLTags,
		}
		if contents.Text.MaxCharacters <= 0 {
			contents.Text.MaxCharacters = 10000
		}
	}
	if input.Highlights != nil {
		contents.Highlights = &ExaHighlightsConfig{
			MaxCharacters: input.Highlights.MaxCharacters,
			Query:         strings.TrimSpace(input.Highlights.Query),
		}
		if contents.Highlights.MaxCharacters <= 0 {
			contents.Highlights.MaxCharacters = 4000
		}
	}
	if input.Summary != nil {
		contents.Summary = &ExaSummaryConfig{
			Query:  strings.TrimSpace(input.Summary.Query),
			Schema: normalizeObject(input.Summary.Schema),
		}
	}
	if input.Extras != nil {
		contents.Extras = &ExaExtrasConfig{
			Links:      input.Extras.Links,
			ImageLinks: input.Extras.ImageLinks,
		}
	}

	if contents.Text == nil && contents.Highlights == nil && contents.Summary == nil {
		contents.Highlights = &ExaHighlightsConfig{MaxCharacters: 4000}
	}
	return contents
}

func defaultExaContents() *ExaSearchContents {
	return &ExaSearchContents{
		Highlights: &ExaHighlightsConfig{
			MaxCharacters: 4000,
		},
	}
}

func validateExaSearchRequest(request ExaSearchRequest) error {
	if _, ok := allowedExaSearchTypes[request.Type]; !ok {
		return fmt.Errorf("type must be one of auto, neural, fast, instant, deep-lite, deep, or deep-reasoning")
	}

	if request.Category != "" {
		if _, ok := allowedExaCategories[request.Category]; !ok {
			return fmt.Errorf("category must be one of company, research paper, news, personal site, financial report, or people")
		}
	}

	if err := validateExaContents(request.Contents); err != nil {
		return err
	}

	switch request.Category {
	case "company", "people":
		if len(request.ExcludeDomains) > 0 {
			return fmt.Errorf("exclude_domains is not supported for category %q", request.Category)
		}
		if request.StartPublishedDate != "" || request.EndPublishedDate != "" || request.StartCrawlDate != "" || request.EndCrawlDate != "" {
			return fmt.Errorf("published and crawl date filters are not supported for category %q", request.Category)
		}
	}

	if request.Category == "people" {
		for _, domain := range request.IncludeDomains {
			if !isLinkedInDomain(domain) {
				return fmt.Errorf("include_domains only supports LinkedIn domains for category %q", request.Category)
			}
		}
	}

	return nil
}

func validateExaContents(contents *ExaSearchContents) error {
	if contents == nil {
		return nil
	}

	contentModeCount := 0
	if contents.Text != nil {
		contentModeCount++
	}
	if contents.Highlights != nil {
		contentModeCount++
	}
	if contents.Summary != nil {
		contentModeCount++
	}
	if contentModeCount > 1 {
		return fmt.Errorf("contents must specify only one of text, highlights, or summary")
	}

	return nil
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeDomainList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := normalizeDomain(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeDomain(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimSuffix(trimmed, "/")
	if slash := strings.Index(trimmed, "/"); slash >= 0 {
		trimmed = trimmed[:slash]
	}
	return strings.TrimSpace(trimmed)
}

func fetchWebPage(ctx context.Context, rawURL string, maxCharacters int, includeHTML bool) (*fetchedWebPage, error) {
	parsed, err := parseAllowedWebFetchURL(rawURL)
	if err != nil {
		return nil, err
	}
	if err := validateWebFetchHost(ctx, parsed.Hostname()); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build fetch_url request: %w", err)
	}
	req.Header.Set("User-Agent", defaultWebFetchUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,text/plain;q=0.7,*/*;q=0.1")

	resp, err := webFetchHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request fetch_url: %w", err)
	}
	defer resp.Body.Close()

	finalURL := parsed.String()
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	mediaType := ""
	if contentType != "" {
		mediaType, _, _ = mime.ParseMediaType(contentType)
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxWebFetchBodyBytes))
	if readErr != nil {
		return nil, fmt.Errorf("read fetch_url response: %w", readErr)
	}
	decoded := body
	if reader, err := charset.NewReader(bytes.NewReader(body), contentType); err == nil {
		if converted, err := io.ReadAll(io.LimitReader(reader, maxWebFetchBodyBytes)); err == nil {
			decoded = converted
		}
	}

	result := fetchURLToolResponse{
		URL:         parsed.String(),
		FinalURL:    finalURL,
		Status:      resp.StatusCode,
		ContentType: contentType,
		Metadata: map[string]interface{}{
			"bytes_read": len(body),
		},
	}

	content := string(decoded)
	switch {
	case mediaType == "" || strings.HasPrefix(mediaType, "text/html") || strings.Contains(strings.ToLower(content[:minInt(len(content), 512)]), "<html"):
		doc, err := html.Parse(strings.NewReader(content))
		if err != nil {
			return nil, fmt.Errorf("parse fetch_url html: %w", err)
		}
		title, description, text, links := extractFetchedHTML(parsed, doc)
		result.Title = title
		result.Description = description
		result.Text, result.Truncated = truncateWithFlag(text, maxCharacters)
		result.Links = limitWebPageLinks(links, maxWebFetchLinks)
		if includeHTML {
			result.HTML, _ = truncateWithFlag(content, 20000)
		}
		return &fetchedWebPage{response: result, links: links}, nil
	case strings.HasPrefix(mediaType, "text/") || strings.Contains(mediaType, "json") || strings.Contains(mediaType, "xml"):
		result.Text, result.Truncated = truncateWithFlag(normalizeFetchedWhitespace(content), maxCharacters)
		return &fetchedWebPage{response: result}, nil
	default:
		result.Metadata["unsupported_media_type"] = mediaType
		return &fetchedWebPage{response: result}, nil
	}
}

func parseAllowedWebFetchURL(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, fmt.Errorf("url is required")
	}
	if !strings.Contains(trimmed, "://") && strings.Contains(trimmed, ".") && !strings.ContainsAny(trimmed, " \t\r\n") {
		trimmed = "https://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return nil, fmt.Errorf("url must use http or https")
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return nil, fmt.Errorf("url host is required")
	}
	parsed.Fragment = ""
	return parsed, nil
}

func validateWebFetchHost(ctx context.Context, host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("url host is required")
	}
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".local") {
		return fmt.Errorf("fetch_url cannot access localhost or .local hosts")
	}
	if allowPrivateWebFetchHostsForTests {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateWebFetchIP(ip) {
			return fmt.Errorf("fetch_url cannot access private or local IP addresses")
		}
		return nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return fmt.Errorf("resolve fetch_url host: %w", err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("resolve fetch_url host: no addresses")
	}
	for _, addr := range addrs {
		if isPrivateWebFetchIP(addr.IP) {
			return fmt.Errorf("fetch_url cannot access hosts resolving to private or local IP addresses")
		}
	}
	return nil
}

func isPrivateWebFetchIP(ip net.IP) bool {
	return ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

func extractFetchedHTML(baseURL *url.URL, doc *html.Node) (string, string, string, []webPageLink) {
	var title string
	var description string
	textParts := make([]string, 0, 256)
	links := make([]webPageLink, 0, 64)
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, hidden bool) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Script, atom.Style, atom.Noscript, atom.Svg:
				hidden = true
			case atom.Title:
				title = firstNonEmpty(title, normalizeFetchedWhitespace(nodeText(n)))
				hidden = true
			case atom.Meta:
				name := strings.ToLower(strings.TrimSpace(htmlNodeAttr(n, "name")))
				property := strings.ToLower(strings.TrimSpace(htmlNodeAttr(n, "property")))
				if name == "description" || property == "og:description" || property == "twitter:description" {
					description = firstNonEmpty(description, normalizeFetchedWhitespace(htmlNodeAttr(n, "content")))
				}
			case atom.A:
				if href := strings.TrimSpace(htmlNodeAttr(n, "href")); href != "" {
					if resolved := resolveWebLink(baseURL, href); resolved != "" {
						links = append(links, webPageLink{
							Text: normalizeFetchedWhitespace(nodeText(n)),
							URL:  resolved,
						})
					}
				}
			}
		}
		if n.Type == html.TextNode && !hidden {
			if text := normalizeFetchedWhitespace(n.Data); text != "" {
				textParts = append(textParts, text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, hidden)
		}
	}
	walk(doc, false)
	return title, description, normalizeFetchedWhitespace(strings.Join(textParts, " ")), dedupeWebPageLinks(links)
}

func nodeText(n *html.Node) string {
	var parts []string
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current == nil {
			return
		}
		if current.Type == html.TextNode {
			parts = append(parts, current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return strings.Join(parts, " ")
}

func htmlNodeAttr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, key) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func resolveWebLink(baseURL *url.URL, href string) string {
	if baseURL == nil {
		return ""
	}
	parsed, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return ""
	}
	resolved := baseURL.ResolveReference(parsed)
	if resolved == nil || (resolved.Scheme != "http" && resolved.Scheme != "https") {
		return ""
	}
	resolved.Fragment = ""
	return resolved.String()
}

func dedupeWebPageLinks(links []webPageLink) []webPageLink {
	if len(links) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(links))
	out := make([]webPageLink, 0, len(links))
	for _, link := range links {
		link.URL = strings.TrimSpace(link.URL)
		link.Text = truncateString(normalizeFetchedWhitespace(link.Text), 120)
		if link.URL == "" || seen[link.URL] {
			continue
		}
		seen[link.URL] = true
		out = append(out, link)
	}
	return out
}

func limitWebPageLinks(links []webPageLink, limit int) []webPageLink {
	if limit <= 0 || len(links) <= limit {
		return links
	}
	return append([]webPageLink(nil), links[:limit]...)
}

func filterCrawlLinks(root *url.URL, links []webPageLink, includePatterns, pathKeywords []string) []webPageLink {
	if len(links) == 0 {
		return nil
	}
	scored := make([]struct {
		link  webPageLink
		score int
	}, 0, len(links))
	for _, link := range links {
		parsed, err := parseAllowedWebFetchURL(link.URL)
		if err != nil || !sameWebHost(root, parsed) {
			continue
		}
		haystack := strings.ToLower(parsed.Path + " " + parsed.RawQuery + " " + link.Text)
		score := 0
		for _, pattern := range includePatterns {
			if pattern != "" && strings.Contains(haystack, pattern) {
				score += 10
			}
		}
		for _, keyword := range pathKeywords {
			if keyword != "" && strings.Contains(haystack, keyword) {
				score += 5
			}
		}
		if score == 0 && len(includePatterns) > 0 {
			continue
		}
		if score == 0 {
			score = 1
		}
		scored = append(scored, struct {
			link  webPageLink
			score int
		}{link: webPageLink{Text: link.Text, URL: parsed.String()}, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].link.URL < scored[j].link.URL
		}
		return scored[i].score > scored[j].score
	})
	out := make([]webPageLink, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.link)
	}
	return out
}

func sameWebHost(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Hostname(), b.Hostname())
}

func canonicalWebURL(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}
	clone := *parsed
	clone.Fragment = ""
	if clone.Path == "" {
		clone.Path = "/"
	}
	return strings.ToLower(clone.String())
}

func normalizeCrawlPatterns(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func defaultCrawlPathKeywords() []string {
	return []string{"changelog", "release", "releases", "release-notes", "updates", "whats-new", "what's new", "announcements", "roadmap", "docs", "blog"}
}

func clampWebFetchCharacters(value int) int {
	switch {
	case value <= 0:
		return 12000
	case value > 30000:
		return 30000
	default:
		return value
	}
}

func clampCrawlPageCharacters(value int) int {
	switch {
	case value <= 0:
		return 6000
	case value > 15000:
		return 15000
	default:
		return value
	}
}

func clampCrawlMaxPages(value int) int {
	switch {
	case value <= 0:
		return 8
	case value > 20:
		return 20
	default:
		return value
	}
}

func clampCrawlMaxDepth(value int) int {
	switch {
	case value <= 0:
		return 1
	case value > 2:
		return 2
	default:
		return value
	}
}

func normalizeFetchedWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func truncateWithFlag(value string, limit int) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if limit <= 0 || len(trimmed) <= limit {
		return trimmed, false
	}
	return strings.TrimSpace(trimmed[:limit]), true
}

func truncateString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isLinkedInDomain(value string) bool {
	normalized := normalizeDomain(value)
	return normalized == "linkedin.com" || strings.HasSuffix(normalized, ".linkedin.com")
}

func normalizeObject(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func clampExaNumResults(value int) int {
	switch {
	case value <= 0:
		return 5
	case value > 10:
		return 10
	default:
		return value
	}
}

func normalizeExaToolResults(results []ExaSearchResult) []exaToolResponseResult {
	if len(results) == 0 {
		return []exaToolResponseResult{}
	}

	out := make([]exaToolResponseResult, 0, len(results))
	for _, result := range results {
		item := exaToolResponseResult{
			Title:           strings.TrimSpace(result.Title),
			URL:             strings.TrimSpace(result.URL),
			ID:              strings.TrimSpace(result.ID),
			PublishedDate:   result.PublishedDate,
			Author:          result.Author,
			Image:           strings.TrimSpace(result.Image),
			Favicon:         strings.TrimSpace(result.Favicon),
			Text:            strings.TrimSpace(result.Text),
			Highlights:      result.Highlights,
			HighlightScores: result.HighlightScores,
			Summary:         strings.TrimSpace(result.Summary),
			Extras:          result.Extras,
		}
		if item.Title == "" || item.URL == "" {
			continue
		}
		if len(result.Subpages) > 0 {
			item.Subpages = normalizeExaToolResults(result.Subpages)
		}
		out = append(out, item)
	}
	return out
}
