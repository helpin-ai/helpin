package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/idna"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	googleWebRiskLookupEndpoint = "https://webrisk.googleapis.com/v1/uris:search"

	supportLinkSecurityNoMatch   = "no_match"
	supportLinkSecurityMalicious = "malicious"
	supportLinkSecurityUnknown   = "unknown"

	googleWebRiskCacheLimit = 2048
)

var googleWebRiskThreatTypes = []string{"MALWARE", "SOCIAL_ENGINEERING", "UNWANTED_SOFTWARE"}

// SupportLinkScanner checks the current reputation of one normalized URL.
type SupportLinkScanner interface {
	Scan(ctx context.Context, normalizedURL string) model.SupportLinkSecurity
}

type googleWebRiskCacheEntry struct {
	verdict model.SupportLinkSecurity
}

// GoogleWebRiskClient checks URLs with Google's Web Risk Lookup API.
type GoogleWebRiskClient struct {
	apiKey   string
	endpoint string
	client   *http.Client
	now      func() time.Time

	mu    sync.Mutex
	cache map[string]googleWebRiskCacheEntry
}

// NewGoogleWebRiskClient creates a Web Risk client. An empty key disables provider calls.
func NewGoogleWebRiskClient(apiKey string) *GoogleWebRiskClient {
	return newGoogleWebRiskClient(strings.TrimSpace(apiKey), googleWebRiskLookupEndpoint, &http.Client{
		Timeout: 2 * time.Second,
	}, time.Now)
}

func newGoogleWebRiskClient(apiKey, endpoint string, client *http.Client, now func() time.Time) *GoogleWebRiskClient {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	if now == nil {
		now = time.Now
	}
	return &GoogleWebRiskClient{
		apiKey: strings.TrimSpace(apiKey), endpoint: endpoint, client: client, now: now,
		cache: make(map[string]googleWebRiskCacheEntry),
	}
}

// Scan returns malicious only for a current, valid provider match. Failures are unknown.
func (c *GoogleWebRiskClient) Scan(ctx context.Context, normalizedURL string) model.SupportLinkSecurity {
	now := c.now().UTC()
	if cached, ok := c.cached(normalizedURL, now); ok {
		return cached
	}
	if c.apiKey == "" {
		return c.store(normalizedURL, newSupportLinkSecurity(normalizedURL, supportLinkSecurityUnknown, nil, now, now.Add(30*time.Second)))
	}

	requestURL, err := url.Parse(c.endpoint)
	if err != nil {
		return c.storeUnknown(normalizedURL, now)
	}
	requestURL.RawQuery = googleWebRiskLookupQuery(normalizedURL).Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return c.storeUnknown(normalizedURL, now)
	}
	req.Header.Set("x-goog-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return c.storeUnknown(normalizedURL, now)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.storeUnknown(normalizedURL, now)
	}

	var payload struct {
		Threat *struct {
			ThreatTypes []string  `json:"threatTypes"`
			ExpireTime  time.Time `json:"expireTime"`
		} `json:"threat"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 64*1024))
	if err := decoder.Decode(&payload); err != nil {
		return c.storeUnknown(normalizedURL, now)
	}
	if payload.Threat == nil || len(payload.Threat.ThreatTypes) == 0 {
		return c.store(normalizedURL, newSupportLinkSecurity(normalizedURL, supportLinkSecurityNoMatch, nil, now, now.Add(5*time.Minute)))
	}
	threats := supportedWebRiskThreats(payload.Threat.ThreatTypes)
	if len(threats) == 0 || !payload.Threat.ExpireTime.After(now) {
		return c.storeUnknown(normalizedURL, now)
	}
	return c.store(normalizedURL, newSupportLinkSecurity(normalizedURL, supportLinkSecurityMalicious, threats, now, payload.Threat.ExpireTime.UTC()))
}

func googleWebRiskLookupQuery(normalizedURL string) url.Values {
	query := url.Values{"uri": []string{normalizedURL}}
	for _, threatType := range googleWebRiskThreatTypes {
		query.Add("threatTypes", threatType)
	}
	return query
}

func supportedWebRiskThreats(values []string) []string {
	allowed := make(map[string]struct{}, len(googleWebRiskThreatTypes))
	for _, value := range googleWebRiskThreatTypes {
		allowed[value] = struct{}{}
	}
	seen := make(map[string]struct{}, len(values))
	var result []string
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func newSupportLinkSecurity(rawURL, status string, threats []string, checkedAt, expiresAt time.Time) model.SupportLinkSecurity {
	return model.SupportLinkSecurity{
		URL: rawURL, Status: status, ThreatTypes: threats,
		CheckedAt: checkedAt.UTC(), ExpiresAt: expiresAt.UTC(),
	}
}

func (c *GoogleWebRiskClient) storeUnknown(rawURL string, now time.Time) model.SupportLinkSecurity {
	return c.store(rawURL, newSupportLinkSecurity(rawURL, supportLinkSecurityUnknown, nil, now, now.Add(30*time.Second)))
}

func (c *GoogleWebRiskClient) cached(rawURL string, now time.Time) (model.SupportLinkSecurity, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache[rawURL]
	if !ok || !entry.verdict.ExpiresAt.After(now) {
		delete(c.cache, rawURL)
		return model.SupportLinkSecurity{}, false
	}
	return entry.verdict, true
}

func (c *GoogleWebRiskClient) store(rawURL string, verdict model.SupportLinkSecurity) model.SupportLinkSecurity {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= googleWebRiskCacheLimit {
		var oldestURL string
		var oldest time.Time
		for cacheURL, entry := range c.cache {
			if oldestURL == "" || entry.verdict.ExpiresAt.Before(oldest) {
				oldestURL = cacheURL
				oldest = entry.verdict.ExpiresAt
			}
		}
		delete(c.cache, oldestURL)
	}
	c.cache[rawURL] = googleWebRiskCacheEntry{verdict: verdict}
	return verdict
}

func normalizeSupportLinkURL(raw string, trimMessagePunctuation bool) (string, error) {
	candidate := strings.TrimSpace(raw)
	if trimMessagePunctuation {
		candidate = trimSupportPreviewURL(candidate)
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("URL credentials are not allowed")
	}
	hostname, err := idna.Lookup.ToASCII(strings.ToLower(parsed.Hostname()))
	if err != nil || hostname == "" {
		return "", fmt.Errorf("invalid URL hostname")
	}
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		parsed.Host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		parsed.Host = "[" + hostname + "]"
	} else {
		parsed.Host = hostname
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return parsed.String(), nil
}
