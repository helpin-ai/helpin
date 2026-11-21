package slack

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

// Client calls the Slack Web API with bounded requests.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient creates a client for Slack's public API.
func NewClient() *Client {
	return &Client{BaseURL: "https://slack.com/api/", HTTP: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// APIError carries a Slack error code or rate limit delay.
type APIError struct {
	Code       string
	RetryAfter time.Duration
}

func (e *APIError) Error() string { return fmt.Sprintf("Slack request failed (%s)", e.Code) }

// Call invokes a JSON Web API method with a bot token.
func (c *Client) Call(ctx context.Context, token, method string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	return c.do(req, output)
}

func (c *Client) do(req *http.Request, out any) error {
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 429 {
		seconds, _ := strconv.Atoi(res.Header.Get("Retry-After"))
		return &APIError{Code: "ratelimited", RetryAfter: time.Duration(seconds) * time.Second}
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("Slack HTTP status %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	var status struct {
		OK    bool
		Error string
	}
	if err = json.Unmarshal(data, &status); err != nil {
		return err
	}
	if !status.OK {
		return &APIError{Code: status.Error}
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

// Installation contains values returned by the OAuth code exchange.
type Installation struct {
	Token     string                    `json:"access_token"`
	BotUserID string                    `json:"bot_user_id"`
	Team      struct{ ID, Name string } `json:"team"`
	User      struct{ ID string }       `json:"authed_user"`
}

// Exchange uses Slack's single-use authorization code to acquire a bot installation.
func (c *Client) Exchange(ctx context.Context, clientID, secret, redirect, code string) (Installation, error) {
	v := url.Values{"redirect_uri": {redirect}, "code": {code}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"oauth.v2.access", bytes.NewBufferString(v.Encode()))
	var out Installation
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, secret)
	if err = c.do(req, &out); err == nil && (out.Token == "" || out.Team.ID == "" || out.BotUserID == "") {
		err = errors.New("Slack returned an incomplete installation")
	}
	return out, err
}

// Get invokes a read-only Slack API method with query parameters.
func (c *Client) Get(ctx context.Context, token, method string, v url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+method+"?"+v.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return c.do(req, out)
}

// MessagePermalink returns Slack's archive link for an individual message, including thread context.
func (c *Client) MessagePermalink(ctx context.Context, token, channel, ts string) (string, error) {
	var out struct{ Permalink string }
	err := c.Get(ctx, token, "chat.getPermalink", url.Values{"channel": {channel}, "message_ts": {ts}}, &out)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			switch apiErr.Code {
			case "message_not_found", "channel_not_found", "not_in_channel", "team_access_not_granted", "invalid_auth", "token_revoked", "account_inactive":
				return "", nil
			}
		}
		return "", err
	}
	u, err := url.Parse(out.Permalink)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return "", nil
	}
	host := u.Hostname()
	if host != "slack.com" && !strings.HasSuffix(host, ".slack.com") && host != "slack-gov.com" && !strings.HasSuffix(host, ".slack-gov.com") {
		return "", nil
	}
	if u.EscapedPath() != "/archives/"+channel+"/p"+strings.ReplaceAll(ts, ".", "") {
		return "", nil
	}
	return out.Permalink, nil
}
