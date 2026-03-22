package crawler

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	sitemap "github.com/oxffaa/gopher-parse-sitemap"
)

// discoverSitemapURLs fetches sitemap.xml (and any sitemap index entries)
// for the given start URL and returns all discovered page URLs.
// Falls back to parsing robots.txt for Sitemap: directives if the default
// sitemap path returns a non-200 status.
func discoverSitemapURLs(ctx context.Context, client *http.Client, startURL string, logger *slog.Logger) []string {
	parsed, err := url.Parse(startURL)
	if err != nil {
		return nil
	}
	baseURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	// Try /sitemap.xml first.
	urls := fetchSitemap(ctx, client, baseURL+"/sitemap.xml", logger)
	if len(urls) > 0 {
		return urls
	}

	// Fall back to robots.txt Sitemap: directives.
	sitemapURLs := parseSitemapFromRobots(ctx, client, baseURL+"/robots.txt", logger)
	for _, sitemapURL := range sitemapURLs {
		urls = append(urls, fetchSitemap(ctx, client, sitemapURL, logger)...)
	}

	return urls
}

// fetchSitemap fetches and parses a sitemap (or sitemap index) URL.
func fetchSitemap(ctx context.Context, client *http.Client, sitemapURL string, logger *slog.Logger) []string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Helpin-Crawler/1.0")

	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("sitemap fetch failed", "url", sitemapURL, "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var urls []string

	// Try parsing as a sitemap index first, then as a regular sitemap.
	err = sitemap.Parse(resp.Body, func(entry sitemap.Entry) error {
		if loc := strings.TrimSpace(entry.GetLocation()); loc != "" {
			urls = append(urls, loc)
		}
		return nil
	})
	if err != nil {
		logger.Debug("sitemap parse failed", "url", sitemapURL, "error", err)
	}

	// If we got sitemap index entries (URLs ending in .xml), expand them.
	expanded := make([]string, 0, len(urls))
	for _, u := range urls {
		if strings.HasSuffix(strings.ToLower(u), ".xml") || strings.HasSuffix(strings.ToLower(u), ".xml.gz") {
			childURLs := fetchSitemap(ctx, client, u, logger)
			expanded = append(expanded, childURLs...)
		} else {
			expanded = append(expanded, u)
		}
	}

	return expanded
}

// parseSitemapFromRobots fetches robots.txt and extracts Sitemap: directives.
func parseSitemapFromRobots(ctx context.Context, client *http.Client, robotsURL string, logger *slog.Logger) []string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Helpin-Crawler/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var sitemapURLs []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
			value := strings.TrimSpace(line[len("sitemap:"):])
			if value != "" {
				sitemapURLs = append(sitemapURLs, value)
			}
		}
	}

	if len(sitemapURLs) > 0 {
		logger.Debug("found sitemaps in robots.txt", "count", len(sitemapURLs))
	}
	return sitemapURLs
}
