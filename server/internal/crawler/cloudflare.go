package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CloudflareCrawlClient calls Cloudflare Browser Rendering crawl APIs.
type CloudflareCrawlClient struct {
	accountID string
	apiToken  string
	baseURL   string
	client    *http.Client
}

type cloudflareEnvelope[T any] struct {
	Success bool `json:"success"`
	Result  T    `json:"result"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// CloudflareCrawlJobResult represents the response from the Cloudflare crawl API.
type CloudflareCrawlJobResult struct {
	ID                 string                  `json:"id"`
	Status             string                  `json:"status"`
	BrowserSecondsUsed float64                 `json:"browserSecondsUsed"`
	Total              int                     `json:"total"`
	Finished           int                     `json:"finished"`
	Records            []CloudflareCrawlRecord `json:"records"`
	Cursor             flexString              `json:"cursor"`
}

// flexString unmarshals both JSON strings and numbers into a Go string.
// Cloudflare's crawl API returns cursor as a number or a string depending on
// pagination state.
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	// Attempt string first.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	// Fall back to number -> string.
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexString(n.String())
		return nil
	}
	// Treat anything else (including null) as empty.
	*f = ""
	return nil
}

// CloudflareCrawlRecord is a single page result from the Cloudflare crawl API.
type CloudflareCrawlRecord struct {
	URL      string            `json:"url"`
	Status   string            `json:"status"`
	HTML     string            `json:"html"`
	Markdown string            `json:"markdown"`
	JSON     json.RawMessage   `json:"json"`
	Metadata map[string]any    `json:"metadata"`
}

// NewCloudflareCrawlClient creates a Cloudflare Browser Rendering crawl client.
// Returns nil if required credentials are missing.
func NewCloudflareCrawlClient(accountID, apiToken, baseURL string) *CloudflareCrawlClient {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(apiToken) == "" {
		return nil
	}
	trimmedBase := strings.TrimSpace(baseURL)
	if trimmedBase == "" {
		trimmedBase = "https://api.cloudflare.com/client/v4"
	}
	return &CloudflareCrawlClient{
		accountID: strings.TrimSpace(accountID),
		apiToken:  strings.TrimSpace(apiToken),
		baseURL:   strings.TrimRight(trimmedBase, "/"),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// StartCrawl submits a crawl job to the Cloudflare Browser Rendering API and
// returns the job ID.
func (c *CloudflareCrawlClient) StartCrawl(ctx context.Context, source model.SupportContentSource) (string, error) {
	if c == nil {
		return "", fmt.Errorf("cloudflare crawl client is not configured")
	}
	body := map[string]any{
		"url":           source.StartURL,
		"crawlPurposes": normalizePurposes([]string(source.CrawlPurposes)),
		"limit":         source.CrawlLimit,
		"depth":         source.CrawlDepth,
		"source":        source.CrawlSource,
		"formats":       normalizeFormats([]string(source.Formats)),
		"render":        source.Render,
		"maxAge":        source.MaxAgeSeconds,
	}
	if source.ModifiedSince != nil {
		body["modifiedSince"] = source.ModifiedSince.Unix()
	}

	options := map[string]any{}
	if source.IncludeExternalLinks {
		options["includeExternalLinks"] = true
	}
	if source.IncludeSubdomains {
		options["includeSubdomains"] = true
	}
	if len(source.IncludePatterns) > 0 {
		options["includePatterns"] = []string(source.IncludePatterns)
	}
	if len(source.ExcludePatterns) > 0 {
		options["excludePatterns"] = []string(source.ExcludePatterns)
	}
	if len(options) > 0 {
		body["options"] = options
	}

	if sliceContains(normalizeFormats([]string(source.Formats)), model.ContentSourceFormatJSON) {
		jsonOptions := map[string]any{}
		if source.JSONPrompt != nil && strings.TrimSpace(*source.JSONPrompt) != "" {
			jsonOptions["prompt"] = strings.TrimSpace(*source.JSONPrompt)
		}
		if len(source.JSONResponseFormat) > 0 {
			var parsed any
			if err := json.Unmarshal(source.JSONResponseFormat, &parsed); err == nil {
				jsonOptions["response_format"] = parsed
			}
		}
		if len(jsonOptions) > 0 {
			body["jsonOptions"] = jsonOptions
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/accounts/"+c.accountID+"/browser-rendering/crawl", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("start cloudflare crawl: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var envelope cloudflareEnvelope[string]
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return "", fmt.Errorf("decode cloudflare crawl start: %w", err)
	}
	if resp.StatusCode >= 300 || !envelope.Success {
		return "", fmt.Errorf("cloudflare crawl start failed: %s", joinCloudflareErrors(envelope.Errors, resp.Status))
	}
	return strings.TrimSpace(envelope.Result), nil
}

// GetCrawlResult retrieves the status and records for a crawl job. Use limit
// and cursor for pagination; use status to filter records (e.g. "completed").
func (c *CloudflareCrawlClient) GetCrawlResult(ctx context.Context, jobID string, limit int, cursor string, status string) (*CloudflareCrawlJobResult, error) {
	if c == nil {
		return nil, fmt.Errorf("cloudflare crawl client is not configured")
	}
	endpoint := c.baseURL + "/accounts/" + c.accountID + "/browser-rendering/crawl/" + strings.TrimSpace(jobID)
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	if strings.TrimSpace(cursor) != "" {
		query.Set("cursor", strings.TrimSpace(cursor))
	}
	if strings.TrimSpace(status) != "" {
		query.Set("status", strings.TrimSpace(status))
	}
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get cloudflare crawl result: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var envelope cloudflareEnvelope[CloudflareCrawlJobResult]
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return nil, fmt.Errorf("decode cloudflare crawl result: %w", err)
	}
	if resp.StatusCode >= 300 || !envelope.Success {
		return nil, fmt.Errorf("cloudflare crawl result failed: %s", joinCloudflareErrors(envelope.Errors, resp.Status))
	}
	return &envelope.Result, nil
}

// joinCloudflareErrors formats API error messages into a single string.
func joinCloudflareErrors(errors []struct{ Message string `json:"message"` }, fallback string) string {
	if len(errors) == 0 {
		return fallback
	}
	parts := make([]string, 0, len(errors))
	for _, err := range errors {
		if strings.TrimSpace(err.Message) != "" {
			parts = append(parts, strings.TrimSpace(err.Message))
		}
	}
	if len(parts) == 0 {
		return fallback
	}
	return strings.Join(parts, "; ")
}

const defaultCloudflarePollInterval = 5 * time.Second

// crawlWithCloudflare performs a full Cloudflare Browser Rendering crawl and
// delivers each completed page via the onPage callback. It polls until the job
// finishes, then paginates through all completed records.
func crawlWithCloudflare(
	ctx context.Context,
	client *CloudflareCrawlClient,
	source model.SupportContentSource,
	logger *slog.Logger,
	onPage func(CrawlRecord) error,
) (int, error) {
	// 1. Start the crawl job.
	jobID, err := client.StartCrawl(ctx, source)
	if err != nil {
		return 0, err
	}
	logger.Info("cloudflare crawl started",
		"source_id", source.ID,
		"job_id", jobID,
		"start_url", source.StartURL,
	)

	// 2. Poll until the job is no longer running.
	for {
		job, err := client.GetCrawlResult(ctx, jobID, 1, "", "")
		if err != nil {
			return 0, err
		}
		switch job.Status {
		case "running":
			logger.Debug("cloudflare crawl polling",
				"job_id", jobID,
				"finished", job.Finished,
				"total", job.Total,
			)
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(defaultCloudflarePollInterval):
			}
		case "completed":
			logger.Info("cloudflare crawl completed",
				"job_id", jobID,
				"total", job.Total,
				"finished", job.Finished,
			)
			goto fetch
		default:
			return 0, fmt.Errorf("crawl job ended with status %s", job.Status)
		}
	}

fetch:
	// 3. Paginate through all completed records.
	var (
		cursor    string
		pageCount int
	)
	for {
		job, err := client.GetCrawlResult(ctx, jobID, 100, cursor, "completed")
		if err != nil {
			return pageCount, err
		}
		for _, record := range job.Records {
			text := cfRecordText(record)
			if strings.TrimSpace(text) == "" {
				continue
			}

			cr := CrawlRecord{
				URL:        strings.TrimSpace(record.URL),
				Title:      cfRecordTitle(record),
				HTTPStatus: cfRecordHTTPStatus(record),
				Markdown:   text,
				HTML:       record.HTML,
				Metadata:   record.Metadata,
			}
			if err := onPage(cr); err != nil {
				return pageCount, fmt.Errorf("onPage callback: %w", err)
			}
			pageCount++
		}

		nextCursor := strings.TrimSpace(string(job.Cursor))
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	logger.Info("cloudflare crawl records processed",
		"source_id", source.ID,
		"job_id", jobID,
		"pages", pageCount,
	)
	return pageCount, nil
}

// cfRecordText extracts the best available text content from a Cloudflare
// crawl record. It prefers Markdown, then pretty-printed JSON, then raw HTML.
func cfRecordText(record CloudflareCrawlRecord) string {
	if strings.TrimSpace(record.Markdown) != "" {
		return strings.TrimSpace(record.Markdown)
	}
	if len(record.JSON) > 0 && string(record.JSON) != "null" {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, record.JSON, "", "  "); err != nil {
			return string(record.JSON)
		}
		return strings.TrimSpace(pretty.String())
	}
	if strings.TrimSpace(record.HTML) != "" {
		return strings.TrimSpace(record.HTML)
	}
	return ""
}

// cfRecordTitle extracts the page title from Cloudflare record metadata.
func cfRecordTitle(record CloudflareCrawlRecord) string {
	if title, ok := record.Metadata["title"].(string); ok && strings.TrimSpace(title) != "" {
		return strings.TrimSpace(strings.ToValidUTF8(title, ""))
	}
	return strings.TrimSpace(record.URL)
}

// cfRecordHTTPStatus extracts the HTTP status code from Cloudflare record metadata.
func cfRecordHTTPStatus(record CloudflareCrawlRecord) int {
	switch value := record.Metadata["status"].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

// normalizePurposes validates crawl purpose values, returning defaults when
// the input is empty.
func normalizePurposes(input []string) []string {
	valid := map[string]struct{}{
		model.ContentSourcePurposeSearch:  {},
		model.ContentSourcePurposeAIInput: {},
		model.ContentSourcePurposeAITrain: {},
	}
	result := make([]string, 0, len(input))
	for _, v := range input {
		trimmed := strings.TrimSpace(v)
		if _, ok := valid[trimmed]; ok {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return []string{model.ContentSourcePurposeSearch, model.ContentSourcePurposeAIInput}
	}
	return result
}

// normalizeFormats validates content format values, returning defaults when
// the input is empty.
func normalizeFormats(input []string) []string {
	valid := map[string]struct{}{
		model.ContentSourceFormatHTML:     {},
		model.ContentSourceFormatMarkdown: {},
		model.ContentSourceFormatJSON:     {},
	}
	result := make([]string, 0, len(input))
	for _, v := range input {
		trimmed := strings.TrimSpace(v)
		if _, ok := valid[trimmed]; ok {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return []string{model.ContentSourceFormatMarkdown}
	}
	return result
}

// sliceContains reports whether values contains the expected string.
func sliceContains(values []string, expected string) bool {
	for _, v := range values {
		if v == expected {
			return true
		}
	}
	return false
}
