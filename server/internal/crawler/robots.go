package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

const crawlerUserAgent = "Helpin-Crawler/1.0"

var errRobotsDisallowed = errors.New("URL disallowed by robots.txt")

type robotsEntry struct {
	rules *robotstxt.RobotsData
	err   error
}

// robotsTransport checks every outbound fetch, including redirects and sitemap
// entries. Its cache is per crawl and keyed by scheme and authority. Unlike
// Colly's built-in check, a redirect cannot bypass this transport boundary.
type robotsTransport struct {
	base   http.RoundTripper
	ctx    context.Context
	mu     sync.Mutex
	cache  map[string]robotsEntry
	report func(string, error)
}

func newRobotsTransport(ctx context.Context, base http.RoundTripper, report func(string, error)) *robotsTransport {
	return &robotsTransport{base: base, ctx: ctx, cache: map[string]robotsEntry{}, report: report}
}

func (t *robotsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.allowed(req.URL); err != nil {
		t.report(req.URL.String(), err)
		return nil, err
	}
	// Robots policy is evaluated for this agent, so the request sent to the
	// origin must use the same identity, including redirected fetches.
	req = req.Clone(req.Context())
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set("User-Agent", crawlerUserAgent)
	return t.base.RoundTrip(req)
}

func (t *robotsTransport) allowed(u *url.URL) error {
	origin := u.Scheme + "://" + u.Host
	// Serialize the small, bounded robots fetch to avoid parallel cache misses.
	t.mu.Lock()
	entry, ok := t.cache[origin]
	if !ok {
		entry.rules, entry.err = t.load(origin)
		t.cache[origin] = entry
	}
	t.mu.Unlock()
	if entry.err != nil {
		return entry.err
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	if !entry.rules.TestAgent(path, "Helpin-Crawler") {
		return errRobotsDisallowed
	}
	return nil
}

func (t *robotsTransport) load(origin string) (*robotstxt.RobotsData, error) {
	ctx, cancel := context.WithTimeout(t.ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/robots.txt", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", crawlerUserAgent)
	// Robots discovery uses the raw transport to avoid recursive policy lookups.
	client := &http.Client{Transport: t.base, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many robots.txt redirects")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("robots.txt unavailable; retry sync after the site is reachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 || resp.StatusCode < 200 || resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, fmt.Errorf("robots.txt unavailable (HTTP %d); retry sync after the site is reachable", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return robotstxt.FromBytes(nil)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512000))
	if err != nil {
		return nil, fmt.Errorf("read robots.txt: %w", err)
	}
	rules, err := robotstxt.FromBytes(body)
	if err != nil {
		return nil, fmt.Errorf("parse robots.txt: %w", err)
	}
	return rules, nil
}
