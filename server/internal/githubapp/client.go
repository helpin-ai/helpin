package githubapp

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Repository is the subset of GitHub repository metadata needed by the product.
type Repository struct {
	ID            int64
	FullName      string
	DefaultBranch string
	Private       bool
	Archived      bool
	Permissions   map[string]bool
}

// Installation is the subset of GitHub installation metadata needed for onboarding.
type Installation struct {
	ID           int64
	AppID        int64
	AccountLogin string
	AccountType  string
	HTMLURL      string
}

type Branch struct {
	Name string
}

// Client creates GitHub App JWTs and installation tokens.
type Client struct {
	appID      string
	privateKey *rsa.PrivateKey
	apiBaseURL string
	httpClient *http.Client
}

// NewClient returns a GitHub App client, or nil when config is incomplete.
func NewClient(appID, privateKeyPEM string) (*Client, error) {
	appID = strings.TrimSpace(appID)
	privateKeyPEM = strings.TrimSpace(privateKeyPEM)
	if appID == "" || privateKeyPEM == "" {
		return nil, nil
	}

	privateKey, err := parsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, err
	}

	return &Client{
		appID:      appID,
		privateKey: privateKey,
		apiBaseURL: "https://api.github.com",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// MintInstallationToken returns a short-lived installation token.
func (c *Client) MintInstallationToken(ctx context.Context, installationID string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("github app is not configured")
	}
	appJWT, err := c.createAppJWT()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%s/access_tokens", c.apiBaseURL, installationID), nil)
	if err != nil {
		return "", fmt.Errorf("build github installation token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request github installation token: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Token   string `json:"token"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode github installation token response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("github installation token failed (%d): %s", resp.StatusCode, payload.Message)
	}
	if payload.Token == "" {
		return "", fmt.Errorf("github installation token response was empty")
	}
	return payload.Token, nil
}

// ListInstallationRepositories returns repositories visible to the installation.
func (c *Client) ListInstallationRepositories(ctx context.Context, installationID string) ([]Repository, error) {
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/installation/repositories?per_page=100", nil)
	if err != nil {
		return nil, fmt.Errorf("build github installation repositories request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github installation repositories: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Repositories []struct {
			ID            int64           `json:"id"`
			FullName      string          `json:"full_name"`
			DefaultBranch string          `json:"default_branch"`
			Private       bool            `json:"private"`
			Archived      bool            `json:"archived"`
			Permissions   map[string]bool `json:"permissions"`
		} `json:"repositories"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github repositories response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github repository sync failed (%d): %s", resp.StatusCode, payload.Message)
	}

	repos := make([]Repository, 0, len(payload.Repositories))
	for _, repo := range payload.Repositories {
		repos = append(repos, Repository{
			ID:            repo.ID,
			FullName:      repo.FullName,
			DefaultBranch: repo.DefaultBranch,
			Private:       repo.Private,
			Archived:      repo.Archived,
			Permissions:   repo.Permissions,
		})
	}
	return repos, nil
}

// GetInstallation returns metadata for a GitHub App installation.
func (c *Client) GetInstallation(ctx context.Context, installationID string) (*Installation, error) {
	if c == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	appJWT, err := c.createAppJWT()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/app/installations/%s", c.apiBaseURL, installationID), nil)
	if err != nil {
		return nil, fmt.Errorf("build github installation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github installation: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		ID      int64  `json:"id"`
		AppID   int64  `json:"app_id"`
		HTMLURL string `json:"html_url"`
		Account struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"account"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github installation response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github installation lookup failed (%d): %s", resp.StatusCode, payload.Message)
	}

	return &Installation{
		ID:           payload.ID,
		AppID:        payload.AppID,
		AccountLogin: payload.Account.Login,
		AccountType:  payload.Account.Type,
		HTMLURL:      payload.HTMLURL,
	}, nil
}

// ListRepositoryBranches returns repository branch names visible to the installation.
func (c *Client) ListRepositoryBranches(ctx context.Context, installationID, owner, repo string) ([]Branch, error) {
	if c == nil {
		return nil, fmt.Errorf("github app is not configured")
	}
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}

	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}

	branches := make([]Branch, 0, 64)
	for page := 1; ; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/branches?per_page=100&page=%d", c.apiBaseURL, owner, repo, page), nil)
		if err != nil {
			return nil, fmt.Errorf("build github branches request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request github branches: %w", err)
		}

		var payload []struct {
			Name string `json:"name"`
		}
		var errorPayload struct {
			Message string `json:"message"`
		}
		if resp.StatusCode >= 300 {
			_ = json.NewDecoder(resp.Body).Decode(&errorPayload)
			resp.Body.Close()
			return nil, fmt.Errorf("github branches failed (%d): %s", resp.StatusCode, errorPayload.Message)
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode github branches response: %w", err)
		}
		resp.Body.Close()

		for _, branch := range payload {
			name := strings.TrimSpace(branch.Name)
			if name == "" {
				continue
			}
			branches = append(branches, Branch{Name: name})
		}
		if len(payload) < 100 {
			break
		}
	}
	return branches, nil
}

// MergeBranch merges the head branch into the base branch using the GitHub REST API.
func (c *Client) MergeBranch(ctx context.Context, installationID, owner, repo, base, head, commitMessage string) error {
	if c == nil {
		return fmt.Errorf("github app is not configured")
	}
	token, err := c.MintInstallationToken(ctx, installationID)
	if err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]string{
		"base":           base,
		"head":           head,
		"commit_message": commitMessage,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/repos/%s/%s/merges", c.apiBaseURL, owner, repo),
		strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("build github merge request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request github merge: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 {
		// Base already contains head — nothing to merge
		return nil
	}
	if resp.StatusCode == 201 {
		// Merge commit created successfully
		return nil
	}

	var payload struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)

	if resp.StatusCode == 409 {
		return fmt.Errorf("merge conflict: %s", payload.Message)
	}
	return fmt.Errorf("github merge failed (%d): %s", resp.StatusCode, payload.Message)
}

func (c *Client) createAppJWT() (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Issuer:    c.appID,
		IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	})
	signed, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign github app jwt: %w", err)
	}
	return signed, nil
}

func parsePrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	privateKeyPEM = normalizePrivateKeyPEM(privateKeyPEM)

	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("invalid github app private key")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse github app private key: %w", err)
	}
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("github app private key must be RSA, got %T", keyAny)
	}
	return key, nil
}

func normalizePrivateKeyPEM(privateKeyValue string) string {
	privateKeyValue = strings.ReplaceAll(strings.TrimSpace(privateKeyValue), `\n`, "\n")
	if privateKeyValue == "" || strings.Contains(privateKeyValue, "BEGIN ") {
		return privateKeyValue
	}

	decoded, ok := decodeBase64PrivateKey(privateKeyValue)
	if !ok {
		return privateKeyValue
	}

	decodedValue := strings.ReplaceAll(strings.TrimSpace(string(decoded)), `\n`, "\n")
	if strings.Contains(decodedValue, "BEGIN ") {
		return decodedValue
	}
	return privateKeyValue
}

func decodeBase64PrivateKey(privateKeyValue string) ([]byte, bool) {
	compact := strings.Join(strings.Fields(privateKeyValue), "")
	if compact == "" {
		return nil, false
	}

	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := encoding.DecodeString(compact)
		if err == nil {
			return decoded, true
		}
	}
	return nil, false
}

// InstallationIDString normalizes a numeric installation ID for storage.
func InstallationIDString(id int64) string {
	return strconv.FormatInt(id, 10)
}
