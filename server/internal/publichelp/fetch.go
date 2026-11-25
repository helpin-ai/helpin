package publichelp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/netguard"
)

const (
	userAgent    = "Mozilla/5.0 (compatible; HelpinImporter/1.0; +https://helpin.ai)"
	maxBody      = 16 << 20
	maxAttempts  = 3
	maxRedirects = 5
)

// Page is a fetched page: the answer to one request after redirects.
type Page struct {
	// URL is the address the request ended at.
	URL    *url.URL
	Status int
	Header http.Header
	Body   []byte
}

// Challenged reports whether the site answered with a bot challenge instead
// of the page.
func (p *Page) Challenged() bool {
	if p.Status != http.StatusForbidden && p.Status != http.StatusServiceUnavailable {
		return false
	}
	return strings.EqualFold(p.Header.Get("Cf-Mitigated"), "challenge") || bytes.Contains(p.Body, []byte("Just a moment..."))
}

// Err is nil for a page that was served, and otherwise the condition its
// status stands for.
func (p *Page) Err() error {
	switch {
	case p.Status == http.StatusOK:
		return nil
	case p.Challenged():
		return ErrChallenged
	case p.Status == http.StatusNotFound || p.Status == http.StatusGone:
		return ErrNotFound
	case p.Status == http.StatusUnauthorized || p.Status == http.StatusForbidden:
		return ErrRestricted
	}
	return fmt.Errorf("%w: the site answered with status %d", ErrUnreachable, p.Status)
}

// Origin is the page's scheme and host, such as "https://help.example.com".
func (p *Page) Origin() string { return p.URL.Scheme + "://" + p.URL.Host }

// Fetcher gets public pages. It reaches only public addresses, so an address
// typed into an import form cannot make the server read internal hosts,
// including through a redirect.
type Fetcher struct {
	http *http.Client
	// sleep waits between retries; tests replace it.
	sleep func(context.Context, time.Duration) error
}

// NewFetcher returns a fetcher for the public internet.
func NewFetcher() *Fetcher {
	return NewFetcherWithHTTP(&http.Client{
		Timeout: 45 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           netguard.DialContext(false),
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConnsPerHost:   6,
		},
	})
}

// NewFetcherWithHTTP builds a fetcher on a caller-supplied client. Tests use
// it to reach a local server.
func NewFetcherWithHTTP(client *http.Client) *Fetcher {
	copied := *client
	copied.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		return nil
	}
	return &Fetcher{http: &copied, sleep: sleepContext}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Get fetches a page that must be served: any status but 200 is an error.
// edit, when given, changes each request before it is sent, for example to
// ask for another content type.
func (f *Fetcher) Get(ctx context.Context, target string, edit ...func(*http.Request)) (*Page, error) {
	page, err := f.Open(ctx, target, edit...)
	if err != nil {
		return nil, err
	}
	if err := page.Err(); err != nil {
		return nil, err
	}
	return page, nil
}

// Open fetches a page and returns it whatever its status, retrying brief
// failures. Only a request that got no answer is an error.
func (f *Fetcher) Open(ctx context.Context, target string, edit ...func(*http.Request)) (*Page, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		page, retryAfter, err := f.once(ctx, target, edit)
		if err == nil && retryAfter < 0 {
			return page, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err == nil {
			// The site asked for a pause; the last answer stands if it keeps asking.
			if attempt == maxAttempts {
				return page, nil
			}
		} else {
			lastErr = err
			if retryAfter < 0 || attempt == maxAttempts {
				break
			}
		}
		if err := f.sleep(ctx, retryAfter); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

// once performs one request. A negative retryAfter means the outcome is
// final.
func (f *Fetcher) once(ctx context.Context, target string, edit []func(*http.Request)) (*Page, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, -1, fmt.Errorf("%w: %q", ErrInvalidURL, target)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
	for _, change := range edit {
		change(req)
	}
	resp, err := f.http.Do(req)
	if err != nil {
		if errors.Is(err, netguard.ErrNonPublicAddress) {
			return nil, -1, fmt.Errorf("%w: the address is not on the public internet", ErrInvalidURL)
		}
		return nil, 2 * time.Second, fmt.Errorf("%w: %s", ErrUnreachable, req.URL.Hostname())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, 2 * time.Second, fmt.Errorf("%w: the page could not be read", ErrUnreachable)
	}
	if len(body) > maxBody {
		return nil, -1, fmt.Errorf("%w: a page was larger than expected", ErrUnreachable)
	}
	page := &Page{URL: resp.Request.URL, Status: resp.StatusCode, Header: resp.Header, Body: bytes.TrimSpace(body)}
	retry := time.Duration(-1)
	if !page.Challenged() && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError) {
		retry = retryDelay(resp.Header.Get("Retry-After"))
	}
	return page, retry, nil
}

// retryDelay honors a Retry-After of up to 30 seconds and otherwise waits two.
func retryDelay(header string) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds > 0 {
		return time.Duration(min(seconds, 30)) * time.Second
	}
	return 2 * time.Second
}
