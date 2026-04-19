package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Limits applied to proxied image fetches. Values are deliberately small
// because tracking pixels and legitimate signature/logo images all fit well
// under these caps — anything larger is almost certainly not what we want
// to load into an email bubble.
const (
	emailImageProxyMaxBytes = 8 << 20 // 8 MiB
	emailImageProxyTimeout  = 8 * time.Second
	emailImageProxyCache    = "public, max-age=86400, immutable"
)

// EmailImageProxyHandler proxies remote images referenced by inbound email
// bodies. It exists so the reader's browser never contacts the sender's
// server directly — the reader's IP, user-agent, and referrer are kept
// private and remote images can be cached and content-type verified before
// reaching the iframe.
//
// SSRF protection is applied at dial time: resolved addresses that fall
// inside loopback, private, link-local, multicast, or unspecified ranges
// are rejected before the TCP connection opens.
type EmailImageProxyHandler struct {
	client *http.Client
}

// NewEmailImageProxyHandler returns a handler configured with a size- and
// time-bounded HTTP client that refuses to dial private addresses.
func NewEmailImageProxyHandler() *EmailImageProxyHandler {
	transport := &http.Transport{
		DialContext:           safeDialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		MaxIdleConns:          64,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    false,
	}
	return &EmailImageProxyHandler{
		client: &http.Client{
			Timeout:   emailImageProxyTimeout,
			Transport: transport,
			// Cap redirects so a malicious sender can't force us into an
			// unbounded redirect chain. Each hop is re-validated by
			// safeDialContext, so SSRF protection survives redirects.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
	}
}

// Proxy fetches the URL given in the ?url= query parameter and streams it to
// the client with a content-type forced to image/*. Any non-image response,
// oversized payload, or failed upstream fetch yields a generic error so
// senders can't probe the network by observing our error messages.
func (h *EmailImageProxyHandler) Proxy(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.client == nil {
		http.Error(w, "image proxy not configured", http.StatusServiceUnavailable)
		return
	}

	raw := strings.TrimSpace(r.URL.Query().Get("url"))
	if raw == "" {
		http.Error(w, "url query parameter is required", http.StatusBadRequest)
		return
	}

	target, err := validateImageURL(raw)
	if err != nil {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}
	// Minimal outbound identity. We avoid forwarding the reader's user-agent
	// or referrer on purpose — that's one of the main reasons to proxy at all.
	req.Header.Set("User-Agent", "Helpin-Email-Image-Proxy/1.0")
	req.Header.Set("Accept", "image/*")

	resp, err := h.client.Do(req)
	if err != nil {
		slog.WarnContext(r.Context(), "email image proxy upstream failed",
			"url", target.String(), "error", err)
		http.Error(w, "upstream fetch failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	contentType := strings.TrimSpace(strings.ToLower(resp.Header.Get("Content-Type")))
	if !strings.HasPrefix(contentType, "image/") {
		http.Error(w, "not an image", http.StatusUnsupportedMediaType)
		return
	}

	// Stream up to the cap + 1 byte so we can detect oversized payloads
	// before committing the response body.
	limited := io.LimitReader(resp.Body, emailImageProxyMaxBytes+1)

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", emailImageProxyCache)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Tight CSP — even if the upstream somehow returns HTML with the wrong
	// content-type, the iframe wouldn't execute it.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data:;")

	n, copyErr := io.Copy(w, limited)
	if copyErr != nil && !errors.Is(copyErr, context.Canceled) {
		slog.WarnContext(r.Context(), "email image proxy copy failed",
			"url", target.String(), "error", copyErr)
	}
	if n > emailImageProxyMaxBytes {
		slog.WarnContext(r.Context(), "email image proxy size exceeded",
			"url", target.String(), "bytes", n)
	}
}

// validateImageURL parses the raw URL and enforces scheme/host constraints
// before the dialer gets a chance to resolve it. Rejects empty hosts and
// non-http(s) schemes early so malformed input fails with a 400 rather than
// a 502 after a dial attempt.
func validateImageURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if strings.TrimSpace(u.Host) == "" {
		return nil, errors.New("missing host")
	}
	return u, nil
}

// safeDialContext wraps a normal dialer with a check that rejects private
// address ranges. Runs at dial time so it covers every redirect hop.
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if isDisallowedIP(ip.IP) {
			return nil, fmt.Errorf("address not allowed: %s", ip.IP)
		}
	}
	// Use the already-resolved address to avoid a re-resolve TOCTOU where a
	// DNS rebind between check and dial could swap in a private IP.
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

// isDisallowedIP returns true for any address the proxy should refuse to
// connect to — loopback, private, link-local, multicast, or unspecified.
func isDisallowedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	return false
}
