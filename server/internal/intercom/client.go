package intercom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the Intercom API in the US region.
const DefaultBaseURL = "https://api.intercom.io"

// APIVersion pins the response shapes this package reads.
const APIVersion = "2.11"

// ErrUnauthorized means the access token was rejected.
var ErrUnauthorized = errors.New("intercom rejected the access token")

// ErrNotFound means the requested object does not exist.
var ErrNotFound = errors.New("intercom object not found")

// ErrForbidden means the token is valid but lacks a permission the call
// needs.
var ErrForbidden = errors.New("the intercom access token lacks a permission")

// Client calls the Intercom REST API with an access token.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	// sleep waits between retries; tests replace it.
	sleep func(context.Context, time.Duration) error
}

// NewClient returns a client for the given access token. An empty baseURL
// uses DefaultBaseURL; other regions use their own host.
func NewClient(token, baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: 60 * time.Second},
		sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

// StatusError is an unexpected API response.
type StatusError struct {
	Status  int
	Message string
}

func (e *StatusError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("intercom API returned %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("intercom API returned %d", e.Status)
}

// do sends a request and decodes the JSON response into out. It retries
// rate limits and server errors with backoff.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return fmt.Errorf("encode intercom request: %w", err)
		}
	}
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	const attempts = 6
	for attempt := 1; ; attempt++ {
		request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("build intercom request: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+c.token)
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Intercom-Version", APIVersion)
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		response, err := c.http.Do(request)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempt >= attempts {
				return fmt.Errorf("call intercom: %w", err)
			}
			if err := c.sleep(ctx, backoff(attempt)); err != nil {
				return err
			}
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, 32<<20))
		response.Body.Close()
		switch {
		case readErr != nil:
			return fmt.Errorf("read intercom response: %w", readErr)
		case response.StatusCode == http.StatusUnauthorized:
			return ErrUnauthorized
		case response.StatusCode == http.StatusForbidden:
			return ErrForbidden
		case response.StatusCode == http.StatusNotFound:
			return ErrNotFound
		case response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500:
			if attempt >= attempts {
				return &StatusError{Status: response.StatusCode, Message: apiErrorMessage(data)}
			}
			if err := c.sleep(ctx, retryDelay(response, attempt, time.Now())); err != nil {
				return err
			}
			continue
		case response.StatusCode >= 300:
			return &StatusError{Status: response.StatusCode, Message: apiErrorMessage(data)}
		}
		if out == nil {
			return nil
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode intercom response: %w", err)
		}
		return nil
	}
}

func backoff(attempt int) time.Duration {
	return time.Duration(1<<min(attempt, 5)) * time.Second
}

// retryDelay honours X-RateLimit-Reset (a Unix time) or Retry-After.
func retryDelay(response *http.Response, attempt int, now time.Time) time.Duration {
	if reset, err := strconv.ParseInt(response.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		if wait := time.Unix(reset, 0).Sub(now); wait > 0 && wait <= 2*time.Minute {
			return wait + time.Second
		}
	}
	if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds > 0 && seconds <= 120 {
		return time.Duration(seconds) * time.Second
	}
	return backoff(attempt)
}

func apiErrorMessage(data []byte) string {
	var body struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(data, &body) == nil && len(body.Errors) > 0 {
		return body.Errors[0].Message
	}
	return ""
}

// Me returns the admin and workspace the token belongs to.
func (c *Client) Me(ctx context.Context) (*Me, error) {
	var me Me
	if err := c.do(ctx, http.MethodGet, "/me", nil, nil, &me); err != nil {
		return nil, err
	}
	return &me, nil
}

// ListCollections returns every Help Center collection.
func (c *Client) ListCollections(ctx context.Context) ([]Collection, error) {
	var all []Collection
	for page := 1; ; page++ {
		var response struct {
			Data  []Collection `json:"data"`
			Pages Pages        `json:"pages"`
		}
		query := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"50"}}
		if err := c.do(ctx, http.MethodGet, "/help_center/collections", query, nil, &response); err != nil {
			return nil, fmt.Errorf("list collections: %w", err)
		}
		all = append(all, response.Data...)
		if len(response.Data) == 0 || page >= response.Pages.TotalPages {
			return all, nil
		}
	}
}

// ListArticles returns every article with its body.
func (c *Client) ListArticles(ctx context.Context) ([]Article, error) {
	var all []Article
	for page := 1; ; page++ {
		var response struct {
			Data  []Article `json:"data"`
			Pages Pages     `json:"pages"`
		}
		query := url.Values{"page": {strconv.Itoa(page)}, "per_page": {"50"}}
		if err := c.do(ctx, http.MethodGet, "/articles", query, nil, &response); err != nil {
			return nil, fmt.Errorf("list articles: %w", err)
		}
		all = append(all, response.Data...)
		if len(response.Data) == 0 || page >= response.Pages.TotalPages {
			return all, nil
		}
	}
}

// ConversationPage is one page of a conversation search.
type ConversationPage struct {
	Conversations []ConversationRef
	// Next is the cursor for the following page; empty on the last page.
	Next       string
	TotalCount int
}

// SearchConversations lists conversations created at or after since, oldest
// first, one page at a time.
func (c *Client) SearchConversations(ctx context.Context, since time.Time, cursor string, perPage int) (*ConversationPage, error) {
	if perPage <= 0 || perPage > 150 {
		perPage = 50
	}
	pagination := map[string]any{"per_page": perPage}
	if cursor != "" {
		pagination["starting_after"] = cursor
	}
	body := map[string]any{
		"query":      map[string]any{"field": "created_at", "operator": ">", "value": since.Unix() - 1},
		"pagination": pagination,
		"sort":       map[string]any{"field": "created_at", "order": "ascending"},
	}
	var response struct {
		Conversations []ConversationRef `json:"conversations"`
		TotalCount    int               `json:"total_count"`
		Pages         Pages             `json:"pages"`
	}
	if err := c.do(ctx, http.MethodPost, "/conversations/search", nil, body, &response); err != nil {
		return nil, fmt.Errorf("search conversations: %w", err)
	}
	page := &ConversationPage{Conversations: response.Conversations, TotalCount: response.TotalCount}
	if response.Pages.Next != nil && len(response.Conversations) > 0 {
		page.Next = response.Pages.Next.StartingAfter
	}
	return page, nil
}

// GetConversation returns a conversation with its parts as plain text.
func (c *Client) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	var conversation Conversation
	query := url.Values{"display_as": {"plaintext"}}
	if err := c.do(ctx, http.MethodGet, "/conversations/"+url.PathEscape(id), query, nil, &conversation); err != nil {
		return nil, fmt.Errorf("get conversation %s: %w", id, err)
	}
	return &conversation, nil
}

// GetContact returns a contact.
func (c *Client) GetContact(ctx context.Context, id string) (*Contact, error) {
	var contact Contact
	if err := c.do(ctx, http.MethodGet, "/contacts/"+url.PathEscape(id), nil, nil, &contact); err != nil {
		return nil, fmt.Errorf("get contact %s: %w", id, err)
	}
	return &contact, nil
}
