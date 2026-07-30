// Package externalmcp implements the outbound MCP protocol boundary. It keeps
// browser OAuth and discovery in Helpin while agent-runtime receives only a
// per-run credential and exact tool allowlist.
package externalmcp

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

	"golang.org/x/net/publicsuffix"

	mcpauth "github.com/helpin-ai/agent-runtime-go/mcpauth"
)

const maxResponseBytes = 1 << 20

// RemoteError is safe to surface; it intentionally excludes response bodies,
// headers, URLs with query strings, and credentials.
type RemoteError struct {
	Operation string
	Status    int
	Code      string
}

func (e *RemoteError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("external MCP %s failed with HTTP %d", e.Operation, e.Status)
	}
	return fmt.Sprintf("external MCP %s failed", e.Operation)
}

func IsStatus(err error, status int) bool {
	var remoteErr *RemoteError
	return errors.As(err, &remoteErr) && remoteErr.Status == status || mcpauth.IsStatus(err, status)
}

// Client is a server-scoped, SSRF-hardened HTTP/MCP client.
type Client struct {
	httpClient *http.Client
	endpoint   *url.URL
	baseSite   string
	allowed    []string
	allowLocal bool
}

func NewClient(endpoint string, allowedHosts []string, allowLocal bool) (*Client, error) {
	endpoint = strings.TrimSpace(endpoint)
	u, err := url.Parse(endpoint)
	if err != nil || len(endpoint) > 2048 || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return nil, fmt.Errorf("external MCP endpoint is invalid")
	}
	if u.Scheme != "https" && !(allowLocal && u.Scheme == "http" && isLocalHostname(u.Hostname())) {
		return nil, fmt.Errorf("external MCP endpoint must use HTTPS")
	}
	if !hostMatchesAllowlist(u.Hostname(), allowedHosts) {
		return nil, fmt.Errorf("external MCP endpoint host is not allowed")
	}
	baseSite, _ := publicsuffix.EffectiveTLDPlusOne(strings.ToLower(u.Hostname()))
	client := &Client{endpoint: u, baseSite: baseSite, allowed: append([]string(nil), allowedHosts...), allowLocal: allowLocal}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConnsPerHost:   4,
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid outbound MCP address")
		}
		addrs, err := resolveAllowedHost(ctx, host, allowLocal)
		if err != nil {
			return nil, err
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
		return nil, fmt.Errorf("external MCP host could not be reached")
	}
	client.httpClient = &http.Client{
		Transport: responseLimitRoundTripper{base: transport, maxBytes: maxResponseBytes},
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("external MCP redirect limit exceeded")
			}
			if len(via) == 0 || !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
				return fmt.Errorf("external MCP cross-host redirect is not allowed")
			}
			return client.ValidateRelatedURL(req.URL.String())
		},
	}
	return client, nil
}

type responseLimitRoundTripper struct {
	base     http.RoundTripper
	maxBytes int64
}

func (t responseLimitRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	if resp.ContentLength > t.maxBytes {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("external MCP response exceeds size limit")
	}
	resp.Body = &limitedResponseBody{body: resp.Body, remaining: t.maxBytes}
	return resp, nil
}

type limitedResponseBody struct {
	body      io.ReadCloser
	remaining int64
	checked   bool
}

func (b *limitedResponseBody) Read(p []byte) (int, error) {
	if b.remaining > 0 {
		if int64(len(p)) > b.remaining {
			p = p[:b.remaining]
		}
		n, err := b.body.Read(p)
		b.remaining -= int64(n)
		return n, err
	}
	if b.checked {
		return 0, io.EOF
	}
	b.checked = true
	var probe [1]byte
	n, err := b.body.Read(probe[:])
	if n > 0 {
		return 0, fmt.Errorf("external MCP response exceeds size limit")
	}
	return 0, err
}

func (b *limitedResponseBody) Close() error { return b.body.Close() }

func (c *Client) Endpoint() string { return c.endpoint.String() }

func (c *Client) HTTPClient() *http.Client { return c.httpClient }

// ValidateRelatedURL permits the configured MCP host and authorization hosts
// on the same registrable site. Every dial still rejects non-public addresses.
func (c *Client) ValidateRelatedURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("external MCP discovered URL is invalid")
	}
	if u.Scheme != "https" && !(c.allowLocal && u.Scheme == "http" && isLocalHostname(u.Hostname())) {
		return fmt.Errorf("external MCP discovered URL must use HTTPS")
	}
	if strings.EqualFold(u.Hostname(), c.endpoint.Hostname()) {
		return nil
	}
	if hostMatchesAllowlist(u.Hostname(), c.allowed) {
		return nil
	}
	site, _ := publicsuffix.EffectiveTLDPlusOne(strings.ToLower(u.Hostname()))
	if c.baseSite == "" || site == "" || !strings.EqualFold(site, c.baseSite) {
		return fmt.Errorf("external MCP discovered URL is outside the approved site")
	}
	return nil
}

func hostMatchesAllowlist(host string, allowed []string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	for _, pattern := range allowed {
		pattern = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(pattern), "."))
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

func resolveAllowedHost(ctx context.Context, host string, allowLocal bool) ([]net.IPAddr, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return nil, fmt.Errorf("external MCP host could not be resolved")
	}
	localEndpoint := allowLocal && isLocalHostname(host)
	for _, addr := range addrs {
		ip := addr.IP
		if localEndpoint {
			if !ip.IsLoopback() {
				return nil, fmt.Errorf("external MCP localhost resolves outside loopback")
			}
			continue
		}
		if isNonPublicIP(ip) {
			return nil, fmt.Errorf("external MCP host resolves to a non-public address")
		}
	}
	return addrs, nil
}

func isNonPublicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	// Go's IsPrivate intentionally excludes shared address space. It is still
	// unsuitable for an Internet-only connector and can reach provider/VPC
	// infrastructure in some deployments.
	_, shared, _ := net.ParseCIDR("100.64.0.0/10")
	return shared.Contains(ip)
}

func isLocalHostname(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}

type headerRoundTripper struct {
	base    http.RoundTripper
	headers http.Header
	status  *int
}

func (t headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	for key, values := range t.headers {
		for _, value := range values {
			clone.Header.Add(key, value)
		}
	}
	resp, err := t.base.RoundTrip(clone)
	if resp != nil && t.status != nil {
		*t.status = resp.StatusCode
	}
	return resp, err
}

func (c *Client) clientWithHeaders(headers http.Header, status *int) *http.Client {
	base := c.httpClient.Transport
	clone := *c.httpClient
	clone.Transport = headerRoundTripper{base: base, headers: headers.Clone(), status: status}
	return &clone
}
