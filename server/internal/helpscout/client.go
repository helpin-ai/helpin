package helpscout

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const baseURL = "https://docsapi.helpscout.net/v1"

// Client is an HTTP client for the HelpScout Docs API v1.
type Client struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient creates a new HelpScout Docs API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     slog.Default().With("component", "helpscout"),
	}
}

// ListCollections returns all collections from the HelpScout Docs site.
func (c *Client) ListCollections(ctx context.Context) ([]Collection, error) {
	var all []Collection

	err := c.fetchAllPages(ctx, "/collections", "collections", func(raw json.RawMessage) error {
		var page []Collection
		if err := json.Unmarshal(raw, &page); err != nil {
			return fmt.Errorf("unmarshal collections: %w", err)
		}
		all = append(all, page...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}

	c.logger.InfoContext(ctx, "listed collections", "count", len(all))
	return all, nil
}

// ListCategories returns all categories for the given collection.
func (c *Client) ListCategories(ctx context.Context, collectionID string) ([]Category, error) {
	var all []Category

	path := fmt.Sprintf("/collections/%s/categories", collectionID)
	err := c.fetchAllPages(ctx, path, "categories", func(raw json.RawMessage) error {
		var page []Category
		if err := json.Unmarshal(raw, &page); err != nil {
			return fmt.Errorf("unmarshal categories: %w", err)
		}
		all = append(all, page...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list categories for collection %s: %w", collectionID, err)
	}

	c.logger.InfoContext(ctx, "listed categories", "collection_id", collectionID, "count", len(all))
	return all, nil
}

// ListArticles returns all article references for the given collection.
func (c *Client) ListArticles(ctx context.Context, collectionID string) ([]ArticleRef, error) {
	var all []ArticleRef

	path := fmt.Sprintf("/collections/%s/articles", collectionID)
	err := c.fetchAllPages(ctx, path, "articles", func(raw json.RawMessage) error {
		var page []ArticleRef
		if err := json.Unmarshal(raw, &page); err != nil {
			return fmt.Errorf("unmarshal articles: %w", err)
		}
		all = append(all, page...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list articles for collection %s: %w", collectionID, err)
	}

	c.logger.InfoContext(ctx, "listed articles", "collection_id", collectionID, "count", len(all))
	return all, nil
}

// GetArticle fetches a single article by ID. If draft is true, the draft
// version is returned instead of the published version.
func (c *Client) GetArticle(ctx context.Context, articleID string, draft bool) (*Article, error) {
	path := fmt.Sprintf("/articles/%s", articleID)
	if draft {
		path += "?draft=true"
	}

	var wrapper struct {
		Article Article `json:"article"`
	}
	if err := c.doRequest(ctx, path, &wrapper); err != nil {
		return nil, fmt.Errorf("get article %s: %w", articleID, err)
	}

	c.logger.InfoContext(ctx, "fetched article", "article_id", articleID, "draft", draft)
	return &wrapper.Article, nil
}

// doRequest builds and executes an authenticated GET request against the
// HelpScout Docs API. It handles rate limiting (proactive slow-down and
// 429 retry) and decodes the JSON response into result.
func (c *Client) doRequest(ctx context.Context, path string, result any) error {
	url := baseURL + path

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.SetBasicAuth(c.apiKey, "")
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("execute request %s: %w", path, err)
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response body %s: %w", path, readErr)
		}

		// Handle 429 rate limit with retry.
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			c.logger.WarnContext(ctx, "rate limited by HelpScout, retrying",
				"path", path, "retry_after_seconds", retryAfter.Seconds())
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryAfter):
				continue
			}
		}

		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("invalid API key")
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("HelpScout API %s returned status %d: %s",
				path, resp.StatusCode, truncateBody(body))
		}

		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("decode response %s: %w", path, err)
		}

		// Proactive rate-limit back-off: if remaining quota is low, pause
		// briefly to avoid hitting a hard 429.
		c.maybeBackoff(ctx, resp.Header)

		return nil
	}
}

// fetchAllPages iterates through all pages of a paginated HelpScout endpoint.
// The itemsKey is the JSON key wrapping the array (e.g. "collections").
// The decode function is called for each page's raw item array.
func (c *Client) fetchAllPages(
	ctx context.Context,
	basePath string,
	itemsKey string,
	decode func(json.RawMessage) error,
) error {
	page := 1

	for {
		path := fmt.Sprintf("%s?page=%d", basePath, page)

		var raw json.RawMessage
		if err := c.doRequest(ctx, path, &raw); err != nil {
			return err
		}

		// Extract the pagination envelope and the items array.
		pr, err := extractPage(raw, itemsKey)
		if err != nil {
			return fmt.Errorf("parse page %d of %s: %w", page, basePath, err)
		}

		if err := decode(pr.Items); err != nil {
			return err
		}

		if page >= pr.Pages {
			return nil
		}
		page++
	}
}

// extractPage pulls pagination metadata and the named items array from a raw
// HelpScout API response.
func extractPage(raw json.RawMessage, itemsKey string) (*paginatedResponse, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	items, ok := envelope[itemsKey]
	if !ok {
		return nil, fmt.Errorf("missing %q key in response", itemsKey)
	}

	// Extract page/pages from the envelope (they sit at the top level).
	var pagination struct {
		Page  int `json:"page"`
		Pages int `json:"pages"`
	}
	if err := json.Unmarshal(raw, &pagination); err != nil {
		return nil, fmt.Errorf("unmarshal pagination: %w", err)
	}

	return &paginatedResponse{
		Items: items,
		Page:  pagination.Page,
		Pages: pagination.Pages,
	}, nil
}

// maybeBackoff sleeps briefly if the rate-limit remaining header is below a
// safe threshold.
func (c *Client) maybeBackoff(ctx context.Context, headers http.Header) {
	remaining := headers.Get("X-RateLimit-Remaining")
	if remaining == "" {
		return
	}

	n, err := strconv.Atoi(remaining)
	if err != nil {
		return
	}

	if n < 50 {
		c.logger.InfoContext(ctx, "rate limit low, pausing", "remaining", n)
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
}

// parseRetryAfter parses the Retry-After header value as seconds. Returns a
// default of 10 seconds if the header is missing or unparseable.
func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 10 * time.Second
	}

	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 10 * time.Second
	}

	return time.Duration(seconds) * time.Second
}

// truncateBody returns the first 200 bytes of the response body for error
// messages, avoiding overly long error strings.
func truncateBody(body []byte) string {
	if len(body) <= 200 {
		return string(body)
	}
	return string(body[:200]) + "..."
}
