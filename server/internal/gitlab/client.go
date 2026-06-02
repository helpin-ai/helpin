// Package gitlab is a minimal GitLab REST API client used with a caller-supplied
// access token (Personal/Group/Project Access Token). It is per-connection: callers
// construct a Client for a specific GitLab base URL and pass the token on each call.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client targets a single GitLab instance (gitlab.com or self-hosted).
type Client struct {
	webBaseURL string
	apiBaseURL string
	httpClient *http.Client
}

// User represents the authenticated GitLab user.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	WebURL   string `json:"web_url"`
}

// Project is a GitLab project listing entry.
type Project struct {
	ID                int64  `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
	WebURL            string `json:"web_url"`
	HTTPURLToRepo     string `json:"http_url_to_repo"`
	SSHURLToRepo      string `json:"ssh_url_to_repo"`
	Visibility        string `json:"visibility"`
	Archived          bool   `json:"archived"`
	Permissions       struct {
		ProjectAccess *struct {
			AccessLevel int `json:"access_level"`
		} `json:"project_access"`
		GroupAccess *struct {
			AccessLevel int `json:"access_level"`
		} `json:"group_access"`
	} `json:"permissions"`
}

// Branch is a GitLab repository branch.
type Branch struct {
	Name string `json:"name"`
}

// MergeRequest is a GitLab merge request.
type MergeRequest struct {
	IID       int    `json:"iid"`
	Title     string `json:"title"`
	WebURL    string `json:"web_url"`
	State     string `json:"state"`
	Source    string `json:"source_branch"`
	Target    string `json:"target_branch"`
	MergeSHA  string `json:"merge_commit_sha"`
	SHA       string `json:"sha"`
	MergedAt  string `json:"merged_at"`
	UpdatedAt string `json:"updated_at"`
}

// ProjectWebhook is a GitLab project webhook.
type ProjectWebhook struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

// NewClient constructs a Client for the given GitLab base URL. Empty defaults to
// https://gitlab.com.
func NewClient(baseURL string) *Client {
	webBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if webBaseURL == "" {
		webBaseURL = "https://gitlab.com"
	}
	return &Client{
		webBaseURL: webBaseURL,
		apiBaseURL: webBaseURL + "/api/v4",
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// WebBaseURL returns the configured GitLab web base URL.
func (c *Client) WebBaseURL() string {
	if c == nil || strings.TrimSpace(c.webBaseURL) == "" {
		return "https://gitlab.com"
	}
	return strings.TrimRight(c.webBaseURL, "/")
}

// CurrentUser returns the GitLab user the supplied access token belongs to.
func (c *Client) CurrentUser(ctx context.Context, accessToken string) (*User, error) {
	var user User
	if err := c.do(ctx, accessToken, http.MethodGet, c.apiBaseURL+"/user", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// ListProjects returns projects the access token has at least Reporter access to,
// optionally filtered by search. Pagination is capped: 1 page (100 projects) for
// the empty default list, 3 pages (300) when the caller supplies a search term.
// Users with broader access surface them via the search box.
func (c *Client) ListProjects(ctx context.Context, accessToken, search string) ([]Project, error) {
	search = strings.TrimSpace(search)
	maxPages := 1
	if search != "" {
		maxPages = 3
	}
	projects := make([]Project, 0, 128)
	for page := 1; page <= maxPages; page++ {
		u, _ := url.Parse(c.apiBaseURL + "/projects")
		q := u.Query()
		// min_access_level=20 (Reporter) includes projects accessible via group
		// inheritance. membership=true would exclude those on many GitLab versions.
		q.Set("min_access_level", "20")
		q.Set("simple", "false")
		q.Set("per_page", "100")
		q.Set("page", strconv.Itoa(page))
		q.Set("order_by", "last_activity_at")
		if search != "" {
			q.Set("search", search)
		}
		u.RawQuery = q.Encode()
		var batch []Project
		if err := c.do(ctx, accessToken, http.MethodGet, u.String(), nil, &batch); err != nil {
			return nil, err
		}
		projects = append(projects, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return projects, nil
}

// GetProject fetches a single project by numeric ID. Used to verify a repo the
// caller wants to wire is accessible, without re-listing all projects.
func (c *Client) GetProject(ctx context.Context, accessToken string, projectID int64) (*Project, error) {
	var project Project
	if err := c.do(ctx, accessToken, http.MethodGet,
		fmt.Sprintf("%s/projects/%d", c.apiBaseURL, projectID), nil, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// ListBranches returns the project's branches.
func (c *Client) ListBranches(ctx context.Context, accessToken string, projectID int64) ([]Branch, error) {
	branches := make([]Branch, 0, 64)
	for page := 1; ; page++ {
		u := fmt.Sprintf("%s/projects/%d/repository/branches?per_page=100&page=%d", c.apiBaseURL, projectID, page)
		var batch []Branch
		if err := c.do(ctx, accessToken, http.MethodGet, u, nil, &batch); err != nil {
			return nil, err
		}
		branches = append(branches, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return branches, nil
}

// ListMergeRequests returns open merge requests matching the source/target branches.
func (c *Client) ListMergeRequests(ctx context.Context, accessToken string, projectID int64, sourceBranch, targetBranch string) ([]MergeRequest, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/projects/%d/merge_requests", c.apiBaseURL, projectID))
	q := u.Query()
	q.Set("state", "opened")
	q.Set("source_branch", sourceBranch)
	q.Set("target_branch", targetBranch)
	q.Set("per_page", "20")
	u.RawQuery = q.Encode()
	var items []MergeRequest
	if err := c.do(ctx, accessToken, http.MethodGet, u.String(), nil, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// CreateMergeRequest creates a merge request authored by the token owner.
func (c *Client) CreateMergeRequest(ctx context.Context, accessToken string, projectID int64, sourceBranch, targetBranch, title, description string) (*MergeRequest, error) {
	payload := map[string]string{
		"source_branch": sourceBranch,
		"target_branch": targetBranch,
		"title":         title,
		"description":   description,
	}
	var mr MergeRequest
	if err := c.do(ctx, accessToken, http.MethodPost, fmt.Sprintf("%s/projects/%d/merge_requests", c.apiBaseURL, projectID), payload, &mr); err != nil {
		return nil, err
	}
	return &mr, nil
}

// UpsertProjectWebhook installs a webhook for the project if it is not already present.
func (c *Client) UpsertProjectWebhook(ctx context.Context, accessToken string, projectID int64, hookURL, secret string) (*ProjectWebhook, error) {
	var existing []ProjectWebhook
	if err := c.do(ctx, accessToken, http.MethodGet, fmt.Sprintf("%s/projects/%d/hooks", c.apiBaseURL, projectID), nil, &existing); err != nil {
		return nil, err
	}
	for _, hook := range existing {
		if strings.TrimSpace(hook.URL) == strings.TrimSpace(hookURL) {
			return &hook, nil
		}
	}
	payload := map[string]any{
		"url":                     hookURL,
		"token":                   secret,
		"push_events":             true,
		"merge_requests_events":   true,
		"pipeline_events":         true,
		"releases_events":         true,
		"enable_ssl_verification": true,
	}
	var hook ProjectWebhook
	if err := c.do(ctx, accessToken, http.MethodPost, fmt.Sprintf("%s/projects/%d/hooks", c.apiBaseURL, projectID), payload, &hook); err != nil {
		return nil, err
	}
	return &hook, nil
}

func (c *Client) do(ctx context.Context, accessToken, method, requestURL string, payload any, out any) error {
	var body []byte
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal gitlab request: %w", err)
		}
		body = data
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build gitlab request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request gitlab api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var payload struct {
			Message any    `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&payload)
		return fmt.Errorf("gitlab api failed (%d): %v %s", resp.StatusCode, payload.Message, payload.Error)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode gitlab response: %w", err)
	}
	return nil
}
