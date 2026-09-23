package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// WorkspaceContextFetcher reads the visible text of one website page.
type WorkspaceContextFetcher interface {
	FetchText(ctx context.Context, rawURL string) (string, error)
}

// HTTPWorkspaceContextFetcher fetches pages over HTTP. A nil Client uses a
// shared client that connects only to public addresses.
type HTTPWorkspaceContextFetcher struct {
	Client *http.Client
}

// workspaceContextHTTPClient is shared so connections are reused. It resolves
// every host itself, dials only the validated public address (so DNS cannot
// rebind between check and use), revalidates each redirect, and never uses a
// proxy.
var workspaceContextHTTPClient = newWorkspaceContextHTTPClient(net.DefaultResolver)

// FetchText returns the page's visible text, or an error for non-2xx responses.
func (f HTTPWorkspaceContextFetcher) FetchText(ctx context.Context, rawURL string) (string, error) {
	client := f.Client
	if client == nil {
		client = workspaceContextHTTPClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Helpin-Onboarding/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,text/plain;q=0.8")
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("fetch %s: status %d", rawURL, res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, workspaceContextMaxPageBytes))
	if err != nil {
		return "", err
	}
	return htmlToPlainText(string(body)), nil
}

// newWorkspaceContextHTTPClient reuses the support link preview address guard,
// which blocks loopback, private, shared, link-local (including cloud metadata),
// multicast, reserved and documentation ranges.
func newWorkspaceContextHTTPClient(resolver supportLinkResolver) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("split website address: %w", err)
			}
			addresses, err := validateSupportPreviewHost(ctx, resolver, host)
			if err != nil {
				return nil, err
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].String(), port))
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("website redirect limit exceeded")
			}
			_, err := validateSupportPreviewURL(req.Context(), resolver, req.URL)
			return err
		},
	}
}

// fetchWorkspaceContextPages fetches the candidate pages concurrently and
// joins the readable ones in candidate order. Every fetch observes ctx, so the
// caller's deadline bounds the whole read.
func fetchWorkspaceContextPages(ctx context.Context, fetcher WorkspaceContextFetcher, baseURL string) string {
	candidates := workspaceContextCandidates(baseURL)
	texts := make([]string, len(candidates))
	var wg sync.WaitGroup
	for i, candidate := range candidates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.ErrorContext(ctx, "website context fetch panicked", "panic", r)
				}
			}()
			text, err := fetcher.FetchText(ctx, candidate)
			if err != nil {
				slog.DebugContext(ctx, "website context page unavailable", "url", candidate, "error", err)
				return
			}
			texts[i] = strings.TrimSpace(text)
		}()
	}
	wg.Wait()
	var b strings.Builder
	for i, text := range texts {
		if text == "" {
			continue
		}
		b.WriteString("\n\nURL: ")
		b.WriteString(candidates[i])
		b.WriteString("\n")
		b.WriteString(text)
	}
	return b.String()
}

func workspaceContextCandidates(baseURL string) []string {
	paths := []string{"", "/features", "/product", "/solutions", "/pricing", "/about"}
	seen := make(map[string]bool, len(paths))
	candidates := make([]string, 0, len(paths))
	for _, path := range paths {
		candidate := baseURL
		if path != "" {
			candidate = joinWebsitePath(baseURL, path)
		}
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}
	return candidates
}

func joinWebsitePath(baseURL string, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

func netipParseHost(host string) (netip.Addr, error) {
	return netip.ParseAddr(strings.Trim(host, "[]"))
}
