package githubapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ManifestConversion is the App created by GitHub from a manifest. PEM,
// ClientSecret and WebhookSecret are secrets.
type ManifestConversion struct {
	ID            int64
	Slug          string
	Name          string
	ClientID      string
	ClientSecret  string
	WebhookSecret string
	PEM           string
	HTMLURL       string
	OwnerLogin    string
	OwnerType     string
}

// Credentials converts the manifest result into App credentials.
func (m ManifestConversion) Credentials() Credentials {
	return Credentials{
		AppID:         strconv.FormatInt(m.ID, 10),
		Slug:          m.Slug,
		ClientID:      m.ClientID,
		ClientSecret:  m.ClientSecret,
		PrivateKey:    m.PEM,
		WebhookSecret: m.WebhookSecret,
		HTMLURL:       m.HTMLURL,
	}
}

// ManifestClient exchanges App manifest codes for App credentials.
type ManifestClient struct {
	apiBaseURL string
	httpClient *http.Client
}

// NewManifestClient returns a client for the GitHub API at apiBaseURL
// ("https://api.github.com" when empty). httpClient may be nil.
func NewManifestClient(apiBaseURL string, httpClient *http.Client) *ManifestClient {
	apiBaseURL = strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	if apiBaseURL == "" {
		apiBaseURL = "https://api.github.com"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &ManifestClient{apiBaseURL: apiBaseURL, httpClient: httpClient}
}

// Convert completes the manifest flow by exchanging the temporary code GitHub
// returned to the redirect URL. Codes are single use and expire after an hour.
func (c *ManifestClient) Convert(ctx context.Context, code string) (*ManifestConversion, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("manifest code is required")
	}
	endpoint := fmt.Sprintf("%s/app-manifests/%s/conversions", c.apiBaseURL, url.PathEscape(code))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build github manifest conversion request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github manifest conversion: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read github manifest conversion response: %w", err)
	}
	var payload struct {
		ID            int64  `json:"id"`
		Slug          string `json:"slug"`
		Name          string `json:"name"`
		ClientID      string `json:"client_id"`
		ClientSecret  string `json:"client_secret"`
		WebhookSecret string `json:"webhook_secret"`
		PEM           string `json:"pem"`
		HTMLURL       string `json:"html_url"`
		Message       string `json:"message"`
		Owner         struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"owner"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode github manifest conversion response (%d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github manifest conversion failed (%d): %s", resp.StatusCode, payload.Message)
	}
	if payload.ID == 0 || strings.TrimSpace(payload.PEM) == "" {
		return nil, fmt.Errorf("github manifest conversion returned incomplete credentials")
	}
	if _, err := parsePrivateKey(payload.PEM); err != nil {
		return nil, err
	}
	return &ManifestConversion{
		ID:            payload.ID,
		Slug:          payload.Slug,
		Name:          payload.Name,
		ClientID:      payload.ClientID,
		ClientSecret:  payload.ClientSecret,
		WebhookSecret: payload.WebhookSecret,
		PEM:           payload.PEM,
		HTMLURL:       payload.HTMLURL,
		OwnerLogin:    payload.Owner.Login,
		OwnerType:     payload.Owner.Type,
	}, nil
}
