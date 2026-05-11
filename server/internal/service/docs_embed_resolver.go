package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"

	"github.com/helpin-ai/helpin/server/internal/crawler"
)

var ErrDocsInvalidEmbedURL = errors.New("invalid embed url")

// DocsResolvedEmbed is a public-link preview suitable for a docs rich embed.
type DocsResolvedEmbed struct {
	URL         string  `json:"url"`
	Provider    string  `json:"provider"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	ImageURL    *string `json:"image_url,omitempty"`
}

// DocsEmbedResolverService fetches metadata for rich embed blocks.
type DocsEmbedResolverService struct {
	clients    []*http.Client
	nextClient atomic.Uint64
	timeout    time.Duration
	userAgent  string
}

func NewDocsEmbedResolverService(proxyURLs string) *DocsEmbedResolverService {
	parsedProxyURLs := crawler.ParseProxyURLs(proxyURLs)
	clients := make([]*http.Client, 0, max(1, len(parsedProxyURLs)))
	for _, rawProxyURL := range parsedProxyURLs {
		proxyURL, err := url.Parse(rawProxyURL)
		if err != nil {
			continue
		}
		clients = append(clients, newDocsEmbedResolverHTTPClient(proxyURL))
	}
	if len(clients) == 0 {
		clients = append(clients, newDocsEmbedResolverHTTPClient(nil))
	}
	return &DocsEmbedResolverService{
		clients:   clients,
		timeout:   4500 * time.Millisecond,
		userAgent: "HelpinDocsEmbedBot/1.0 (+https://helpin.ai)",
	}
}

func newDocsEmbedResolverHTTPClient(proxyURL *url.URL) *http.Client {
	client := newSupportLinkPreviewHTTPClient(proxyURL)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		if req == nil || req.URL == nil || !isAllowedSupportPreviewURL(req.URL) {
			return ErrDocsInvalidEmbedURL
		}
		return nil
	}
	return client
}

func (s *DocsEmbedResolverService) Resolve(ctx context.Context, rawURL string) (*DocsResolvedEmbed, error) {
	if s == nil {
		return fallbackDocsResolvedEmbed(rawURL)
	}
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("parse embed url: %w", err)
	}
	if parsedURL.Scheme == "" {
		parsedURL, err = url.Parse("https://" + strings.TrimSpace(rawURL))
		if err != nil {
			return nil, fmt.Errorf("parse embed url: %w", err)
		}
	}
	parsedURL.Fragment = ""
	if !isAllowedSupportPreviewURL(parsedURL) {
		return nil, ErrDocsInvalidEmbedURL
	}

	resolveCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(resolveCtx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build embed request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,text/plain;q=0.5,*/*;q=0.1")

	resp, err := s.nextHTTPClient().Do(req)
	if err != nil {
		if errors.Is(err, ErrDocsInvalidEmbedURL) {
			return nil, ErrDocsInvalidEmbedURL
		}
		fallback, fallbackErr := fallbackDocsResolvedEmbed(parsedURL.String())
		if fallbackErr != nil {
			return nil, err
		}
		return fallback, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fallbackDocsResolvedEmbed(parsedURL.String())
	}

	finalURL := resp.Request.URL
	if finalURL == nil {
		finalURL = parsedURL
	}
	if !isAllowedSupportPreviewURL(finalURL) {
		return nil, ErrDocsInvalidEmbedURL
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	mediaType := ""
	if contentType != "" {
		mediaType, _, _ = mime.ParseMediaType(contentType)
	}
	if mediaType != "" && !strings.HasPrefix(mediaType, "text/html") && !strings.HasPrefix(mediaType, "text/plain") {
		return fallbackDocsResolvedEmbed(finalURL.String())
	}

	bodyReader := io.LimitReader(resp.Body, 1024*1024)
	if decodedReader, err := charset.NewReader(bodyReader, contentType); err == nil {
		bodyReader = decodedReader
	}

	doc, err := html.Parse(bodyReader)
	if err != nil {
		return fallbackDocsResolvedEmbed(finalURL.String())
	}

	preview := extractSupportLinkPreview(finalURL, doc)
	resolved := &DocsResolvedEmbed{
		URL:      preview.URL,
		Provider: docsEmbedProviderName(preview.Host),
		Title:    preview.Title,
	}
	if preview.Description != nil {
		resolved.Description = *preview.Description
	}
	if preview.ImageURL != nil {
		resolved.ImageURL = preview.ImageURL
	}
	if resolved.URL == "" {
		resolved.URL = finalURL.String()
	}
	if resolved.Title == "" {
		resolved.Title = fallbackSupportLinkTitle(finalURL)
	}
	return resolved, nil
}

func (s *DocsEmbedResolverService) nextHTTPClient() *http.Client {
	if len(s.clients) == 0 {
		return http.DefaultClient
	}
	idx := s.nextClient.Add(1) - 1
	return s.clients[idx%uint64(len(s.clients))]
}

func fallbackDocsResolvedEmbed(rawURL string) (*DocsResolvedEmbed, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, err
	}
	if parsedURL.Scheme == "" {
		parsedURL, err = url.Parse("https://" + strings.TrimSpace(rawURL))
		if err != nil {
			return nil, err
		}
	}
	if !isAllowedSupportPreviewURL(parsedURL) {
		return nil, ErrDocsInvalidEmbedURL
	}
	return &DocsResolvedEmbed{
		URL:      parsedURL.String(),
		Provider: docsEmbedProviderName(parsedURL.Hostname()),
		Title:    fallbackSupportLinkTitle(parsedURL),
	}, nil
}

func docsEmbedProviderName(host string) string {
	host = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www.")
	switch {
	case host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") || host == "youtu.be":
		return "YouTube"
	case host == "loom.com" || strings.HasSuffix(host, ".loom.com"):
		return "Loom"
	case host == "figma.com" || strings.HasSuffix(host, ".figma.com"):
		return "Figma"
	case host == "github.com" || strings.HasSuffix(host, ".github.com"):
		return "GitHub"
	case host == "linear.app" || strings.HasSuffix(host, ".linear.app"):
		return "Linear"
	case host == "notion.so" || strings.HasSuffix(host, ".notion.so"):
		return "Notion"
	case host != "":
		return host
	default:
		return "Embed"
	}
}
