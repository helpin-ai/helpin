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

type Client struct {
	clientID     string
	clientSecret string
	redirectURL  string
	webBaseURL   string
	apiBaseURL   string
	httpClient   *http.Client
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	CreatedAt    int64  `json:"created_at"`
	ExpiresIn    int64  `json:"expires_in"`
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	WebURL   string `json:"web_url"`
}

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

type Branch struct {
	Name string `json:"name"`
}

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

type ProjectWebhook struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

func NewClient(clientID, clientSecret, redirectURL, baseURL string) *Client {
	webBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if webBaseURL == "" {
		webBaseURL = "https://gitlab.com"
	}
	return &Client{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		redirectURL:  strings.TrimSpace(redirectURL),
		webBaseURL:   webBaseURL,
		apiBaseURL:   strings.TrimRight(webBaseURL, "/") + "/api/v4",
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.clientID != "" && c.clientSecret != "" && c.redirectURL != ""
}

func (c *Client) WebBaseURL() string {
	if c == nil || strings.TrimSpace(c.webBaseURL) == "" {
		return "https://gitlab.com"
	}
	return strings.TrimRight(c.webBaseURL, "/")
}

func (c *Client) AuthorizeURL(state string, scopes []string) string {
	u, _ := url.Parse(c.webBaseURL + "/oauth/authorize")
	q := u.Query()
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", c.redirectURL)
	q.Set("response_type", "code")
	q.Set("state", state)
	q.Set("scope", strings.Join(scopes, " "))
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	return c.tokenRequest(ctx, url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"code":          {strings.TrimSpace(code)},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.redirectURL},
	})
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	return c.tokenRequest(ctx, url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"refresh_token": {strings.TrimSpace(refreshToken)},
		"grant_type":    {"refresh_token"},
		"redirect_uri":  {c.redirectURL},
	})
}

func (c *Client) tokenRequest(ctx context.Context, form url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webBaseURL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request gitlab oauth token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var payload struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&payload)
		return nil, fmt.Errorf("gitlab oauth token failed (%d): %s %s", resp.StatusCode, payload.Error, payload.ErrorDescription)
	}
	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode gitlab oauth token: %w", err)
	}
	return &token, nil
}

func (c *Client) CurrentUser(ctx context.Context, accessToken string) (*User, error) {
	var user User
	if err := c.do(ctx, accessToken, http.MethodGet, c.apiBaseURL+"/user", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) ListProjects(ctx context.Context, accessToken, search string) ([]Project, error) {
	projects := make([]Project, 0, 128)
	for page := 1; ; page++ {
		u, _ := url.Parse(c.apiBaseURL + "/projects")
		q := u.Query()
		q.Set("membership", "true")
		q.Set("simple", "false")
		q.Set("per_page", "100")
		q.Set("page", strconv.Itoa(page))
		q.Set("order_by", "last_activity_at")
		if strings.TrimSpace(search) != "" {
			q.Set("search", strings.TrimSpace(search))
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
