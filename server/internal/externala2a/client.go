// Package externala2a implements Helpin's outbound boundary to remote
// Agent2Agent (A2A) agents: agent card discovery and file downloads through an
// SSRF-hardened HTTP client. Agent Runtime owns the A2A message exchange.
package externala2a

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WellKnownCardPath is the A2A agent card discovery path.
const WellKnownCardPath = "/.well-known/agent-card.json"

const maxCardBytes = 1 << 20

// ErrTooLarge reports a response body over the caller's byte limit.
var ErrTooLarge = errors.New("external agent response exceeds size limit")

// Options configures the outbound client.
type Options struct {
	// AllowedPrivateHosts lists exact hosts or *.domain patterns that may
	// resolve to private, loopback or link-local addresses and may use plain
	// HTTP. Every other host must be public and use HTTPS.
	AllowedPrivateHosts []string
	// Timeout bounds one whole request, including reading the body.
	Timeout time.Duration
}

// Client fetches agent cards and agent-published files.
type Client struct {
	httpClient     *http.Client
	allowedPrivate []string
}

// NewClient returns a client that dials only public addresses unless the
// target host is explicitly allowlisted.
func NewClient(opts Options) *Client {
	client := &Client{allowedPrivate: normalizeHostPatterns(opts.AllowedPrivateHosts)}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConnsPerHost:   4,
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid external agent address")
		}
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addrs) == 0 {
			return nil, fmt.Errorf("external agent host could not be resolved")
		}
		private := client.AllowsPrivateHost(host)
		for _, addr := range addrs {
			if !private && isNonPublicIP(addr.IP) {
				return nil, fmt.Errorf("external agent host resolves to a non-public address")
			}
		}
		for _, addr := range addrs {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
		return nil, fmt.Errorf("external agent host could not be reached")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	client.httpClient = &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("external agent redirect limit exceeded")
			}
			if _, err := client.ValidateURL(req.URL.String()); err != nil {
				return err
			}
			// Credentials are bound to the host the user configured.
			if len(via) > 0 && !strings.EqualFold(req.URL.Hostname(), via[0].URL.Hostname()) {
				req.Header.Del("Authorization")
			}
			return nil
		},
	}
	return client
}

// AllowsPrivateHost reports whether host is on the private-network allowlist.
func (c *Client) AllowsPrivateHost(host string) bool {
	return c != nil && hostMatches(host, c.allowedPrivate)
}

// ValidateURL checks that raw is an absolute URL the client may fetch: HTTPS
// for public hosts, HTTP or HTTPS for allowlisted private hosts, and no
// embedded credentials.
func (c *Client) ValidateURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || !u.IsAbs() || u.Hostname() == "" || u.User != nil {
		return nil, fmt.Errorf("URL must be an absolute http(s) URL without credentials")
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
	case "http":
		if !c.AllowsPrivateHost(u.Hostname()) {
			return nil, fmt.Errorf("URL must use HTTPS")
		}
	default:
		return nil, fmt.Errorf("URL must use HTTPS")
	}
	if !c.AllowsPrivateHost(u.Hostname()) {
		if ip := net.ParseIP(u.Hostname()); ip != nil && isNonPublicIP(ip) {
			return nil, fmt.Errorf("URL host is not a public address")
		}
		if isLocalHostname(u.Hostname()) {
			return nil, fmt.Errorf("URL host is not a public address")
		}
	}
	return u, nil
}

// NormalizeCardURL accepts a full agent card URL or an agent base URL and
// returns the URL to fetch. A base URL gets the well-known card path.
func (c *Client) NormalizeCardURL(raw string) (string, error) {
	u, err := c.ValidateURL(raw)
	if err != nil {
		return "", err
	}
	u.Fragment = ""
	if strings.HasSuffix(strings.ToLower(u.Path), ".json") {
		return u.String(), nil
	}
	u.Path = strings.TrimRight(u.Path, "/") + WellKnownCardPath
	u.RawPath = ""
	return u.String(), nil
}

// FetchCard downloads and parses an agent card. The token, when present, is
// sent as a bearer credential for agents that protect their card.
func (c *Client) FetchCard(ctx context.Context, cardURL, token string) (*Card, []byte, error) {
	u, err := c.ValidateURL(cardURL)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("agent card URL is invalid")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("A2A-Version", "1.0")
	if token = strings.TrimSpace(token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("could not fetch agent card: %s", sanitizeTransportError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, nil, fmt.Errorf("agent card request failed with HTTP %d", resp.StatusCode)
	}
	raw, err := readLimited(resp.Body, maxCardBytes)
	if err != nil {
		return nil, nil, err
	}
	card, err := ParseCard(raw)
	if err != nil {
		return nil, nil, err
	}
	return card, raw, nil
}

// Download is a fetched file whose body the caller must close.
type Download struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
	FinalURL      *url.URL
}

// Download fetches a file published by an agent. Bodies over maxBytes fail
// while reading with ErrTooLarge. The token is sent only to bearerHost.
func (c *Client) Download(ctx context.Context, rawURL, bearerHost, token string, maxBytes int64) (*Download, error) {
	u, err := c.ValidateURL(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("file URL is invalid")
	}
	if token = strings.TrimSpace(token); token != "" && bearerHost != "" && strings.EqualFold(u.Hostname(), bearerHost) {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not download file: %s", sanitizeTransportError(err))
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("file download failed with HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		_ = resp.Body.Close()
		return nil, ErrTooLarge
	}
	return &Download{
		Body:          &limitedBody{body: resp.Body, remaining: maxBytes},
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: resp.ContentLength,
		FinalURL:      resp.Request.URL,
	}, nil
}

func readLimited(body io.Reader, maxBytes int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("could not read agent card")
	}
	if int64(len(raw)) > maxBytes {
		return nil, ErrTooLarge
	}
	return raw, nil
}

type limitedBody struct {
	body      io.ReadCloser
	remaining int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		var probe [1]byte
		if n, _ := b.body.Read(probe[:]); n > 0 {
			return 0, ErrTooLarge
		}
		return 0, io.EOF
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.body.Read(p)
	b.remaining -= int64(n)
	return n, err
}

func (b *limitedBody) Close() error { return b.body.Close() }

// sanitizeTransportError keeps dial-policy messages and drops URLs, which may
// carry signed query strings.
func sanitizeTransportError(err error) string {
	message := err.Error()
	for _, safe := range []string{"non-public address", "could not be resolved", "could not be reached", "redirect limit", "must use HTTPS", "not a public address"} {
		if strings.Contains(message, safe) {
			return "the host " + safe
		}
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(message, "Client.Timeout") {
		return "the request timed out"
	}
	return "the connection failed"
}

func normalizeHostPatterns(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func hostMatches(host string, patterns []string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return false
	}
	for _, pattern := range patterns {
		if pattern == host {
			return true
		}
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(host, suffix) && host != strings.TrimPrefix(suffix, ".") {
				return true
			}
		}
	}
	return false
}

func isNonPublicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	// Shared address space (RFC 6598) can reach provider infrastructure.
	_, shared, _ := net.ParseCIDR("100.64.0.0/10")
	return shared.Contains(ip)
}

func isLocalHostname(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}
