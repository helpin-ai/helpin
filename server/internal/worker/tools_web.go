package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const braveSearchAPIURL = "https://api.search.brave.com/res/v1/web/search"

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

func (r *ToolRegistry) toolWebSearch(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("execution context is required")
	}
	if ctx.PlanningStage != model.PlanningStageDraftSpec {
		return "", fmt.Errorf("web search is only available during the draft_spec stage")
	}
	if !ctx.PlanningWebSearchEnabled {
		return "", fmt.Errorf("web search is not enabled for this workspace")
	}
	if model.NormalizePlanningWebSearchProvider(ctx.PlanningWebSearchProvider) != model.PlanningWebSearchProviderBrave {
		return "", fmt.Errorf("unsupported planning web search provider %q", ctx.PlanningWebSearchProvider)
	}
	if r == nil || r.webSearch == nil {
		return "", fmt.Errorf("web search provider is not configured on this worker")
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
