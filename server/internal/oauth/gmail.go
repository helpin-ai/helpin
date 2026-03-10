package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	gmailAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	gmailTokenURL = "https://oauth2.googleapis.com/token"
)

// GmailOAuthClient handles Gmail OAuth2 flow.
type GmailOAuthClient struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
}

// NewGmailOAuthClient creates a new Gmail OAuth client.
func NewGmailOAuthClient(clientID, clientSecret, redirectURL string) *GmailOAuthClient {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &GmailOAuthClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// GenerateAuthURL returns the Google OAuth consent URL.
func (c *GmailOAuthClient) GenerateAuthURL(state string) string {
	params := url.Values{
		"client_id":     {c.clientID},
		"redirect_uri":  {c.redirectURL},
		"response_type": {"code"},
		"scope":         {"https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/gmail.send https://www.googleapis.com/auth/gmail.modify https://www.googleapis.com/auth/calendar.readonly"},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
		"state":         {state},
	}
	return gmailAuthURL + "?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens.
func (c *GmailOAuthClient) ExchangeCode(ctx context.Context, code string) (*TokenPair, error) {
	return c.tokenRequest(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"redirect_uri":  {c.redirectURL},
	})
}

// RefreshToken refreshes an expired access token.
func (c *GmailOAuthClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	pair, err := c.tokenRequest(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
	})
	if err != nil {
		return nil, err
	}
	// Google doesn't always return a new refresh token
	if pair.RefreshToken == "" {
		pair.RefreshToken = refreshToken
	}
	return pair, nil
}

func (c *GmailOAuthClient) tokenRequest(ctx context.Context, params url.Values) (*TokenPair, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", gmailTokenURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	return &TokenPair{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}
