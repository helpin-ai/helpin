package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const braveSearchAPIURL = "https://api.search.brave.com/res/v1/web/search"
const exaSearchAPIURL = "https://api.exa.ai/search"

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
	Title           string             `json:"title"`
	URL             string             `json:"url"`
	ID              string             `json:"id"`
	PublishedDate   *string            `json:"publishedDate,omitempty"`
	Author          *string            `json:"author,omitempty"`
	Image           string             `json:"image,omitempty"`
	Favicon         string             `json:"favicon,omitempty"`
	Text            string             `json:"text,omitempty"`
	Highlights      []string           `json:"highlights,omitempty"`
	HighlightScores []float64          `json:"highlightScores,omitempty"`
	Summary         string             `json:"summary,omitempty"`
	Subpages        []ExaSearchResult  `json:"subpages,omitempty"`
	Extras          map[string]any     `json:"extras,omitempty"`
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
	Query              string                 `json:"query"`
	Type               string                 `json:"type"`
	NumResults         int                    `json:"num_results"`
	Category           string                 `json:"category"`
	UserLocation       string                 `json:"user_location"`
	IncludeDomains     []string               `json:"include_domains"`
	ExcludeDomains     []string               `json:"exclude_domains"`
	StartPublishedDate string                 `json:"start_published_date"`
	EndPublishedDate   string                 `json:"end_published_date"`
	StartCrawlDate     string                 `json:"start_crawl_date"`
	EndCrawlDate       string                 `json:"end_crawl_date"`
	AdditionalQueries  []string               `json:"additional_queries"`
	SystemPrompt       string                 `json:"system_prompt"`
	Moderation         bool                   `json:"moderation"`
	Contents           *exaSearchContentsInput `json:"contents"`
	OutputSchema       map[string]any         `json:"output_schema"`
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
	Title           string                 `json:"title"`
	URL             string                 `json:"url"`
	ID              string                 `json:"id,omitempty"`
	PublishedDate   *string                `json:"published_date,omitempty"`
	Author          *string                `json:"author,omitempty"`
	Image           string                 `json:"image,omitempty"`
	Favicon         string                 `json:"favicon,omitempty"`
	Text            string                 `json:"text,omitempty"`
	Highlights      []string               `json:"highlights,omitempty"`
	HighlightScores []float64              `json:"highlight_scores,omitempty"`
	Summary         string                 `json:"summary,omitempty"`
	Subpages        []exaToolResponseResult `json:"subpages,omitempty"`
	Extras          map[string]any         `json:"extras,omitempty"`
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
				"description": "Optional content extraction settings. Defaults to highlights with max_characters 4000 when omitted.",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"max_characters": map[string]interface{}{"type": "integer"},
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
