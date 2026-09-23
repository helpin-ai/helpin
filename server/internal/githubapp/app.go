package githubapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ErrInstallationNotFound reports that GitHub has no installation with the
// given ID for the configured App.
var ErrInstallationNotFound = errors.New("github app installation not found")

// App is the subset of the authenticated GitHub App's metadata Helpin shows.
type App struct {
	ID         int64
	Slug       string
	Name       string
	HTMLURL    string
	OwnerLogin string
	OwnerType  string
}

// GetApp returns the authenticated App (GET /app, signed with the App JWT).
func (c *Client) GetApp(ctx context.Context) (*App, error) {
	if c == nil {
		return nil, ErrNotConfigured
	}
	appJWT, err := c.createAppJWT(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/app", nil)
	if err != nil {
		return nil, fmt.Errorf("build github app request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request github app: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		ID      int64  `json:"id"`
		Slug    string `json:"slug"`
		Name    string `json:"name"`
		HTMLURL string `json:"html_url"`
		Owner   struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"owner"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode github app response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github app lookup failed (%d): %s", resp.StatusCode, payload.Message)
	}
	return &App{
		ID:         payload.ID,
		Slug:       payload.Slug,
		Name:       payload.Name,
		HTMLURL:    payload.HTMLURL,
		OwnerLogin: payload.Owner.Login,
		OwnerType:  payload.Owner.Type,
	}, nil
}
