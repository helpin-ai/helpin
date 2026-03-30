package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"

	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportLinkPreviewMetadataKey = "link_previews"

var supportPreviewURLPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

// SupportMessageLinkPreviewer enriches support messages with persisted link preview metadata.
type SupportMessageLinkPreviewer interface {
	EnrichMessage(ctx context.Context, msg *model.SupportMessage)
}

// SupportLinkPreviewService fetches public page metadata, using configured proxies when available.
type SupportLinkPreviewService struct {
	logger      *slog.Logger
	clients     []*http.Client
	nextClient  atomic.Uint64
	maxPreviews int
	timeout     time.Duration
	userAgent   string
}

// NewSupportLinkPreviewService creates a link preview enricher.
func NewSupportLinkPreviewService(proxyURLs string) *SupportLinkPreviewService {
	logger := slog.Default().With("service", "support_link_preview")
	parsedProxyURLs := crawler.ParseProxyURLs(proxyURLs)
	clients := make([]*http.Client, 0, max(1, len(parsedProxyURLs)))

	for _, rawProxyURL := range parsedProxyURLs {
		proxyURL, err := url.Parse(rawProxyURL)
		if err != nil {
			logger.Warn("support link preview proxy ignored", "proxy_url", rawProxyURL, "error", err)
			continue
		}
		clients = append(clients, newSupportLinkPreviewHTTPClient(proxyURL))
	}
	if len(clients) == 0 {
		clients = append(clients, newSupportLinkPreviewHTTPClient(nil))
	}

	if len(parsedProxyURLs) > 0 {
		logger.Info("support link preview proxy configured", "proxy_count", len(clients))
	}

	return &SupportLinkPreviewService{
		logger:      logger,
		clients:     clients,
		maxPreviews: 3,
		timeout:     3500 * time.Millisecond,
		userAgent:   "HelpinLinkPreviewBot/1.0 (+https://helpin.ai)",
	}
}

func newSupportLinkPreviewHTTPClient(proxyURL *url.URL) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: (&net.Dialer{
			Timeout:   1500 * time.Millisecond,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   1500 * time.Millisecond,
		ResponseHeaderTimeout: 2500 * time.Millisecond,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   4 * time.Second,
	}
}

// EnrichMessage fetches and stores link previews in SupportMessage.Metadata.
func (s *SupportLinkPreviewService) EnrichMessage(ctx context.Context, msg *model.SupportMessage) {
	if s == nil || msg == nil {
		return
	}
	if msg.IsInternal || strings.TrimSpace(msg.Content) == "" || strings.EqualFold(strings.TrimSpace(msg.MessageType), "system") {
		return
	}

	urls := extractSupportPreviewURLs(msg.Content, s.maxPreviews)
	if len(urls) == 0 {
		return
	}

	previewCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	previews, err := s.fetchLinkPreviews(previewCtx, urls)
	if err != nil {
		s.logger.WarnContext(ctx, "support link preview fetch failed",
			"conversation_id", msg.ConversationID,
			"message_id", msg.ID,
			"error", err,
		)
	}
	if len(previews) == 0 {
		return
	}

	merged, err := mergeSupportLinkPreviewMetadata(msg.Metadata, previews)
	if err != nil {
		s.logger.WarnContext(ctx, "support link preview metadata merge failed",
			"conversation_id", msg.ConversationID,
			"message_id", msg.ID,
			"error", err,
		)
		return
	}
	msg.Metadata = merged
}

func (s *SupportLinkPreviewService) fetchLinkPreviews(ctx context.Context, urls []string) ([]model.SupportLinkPreview, error) {
	previews := make([]model.SupportLinkPreview, 0, len(urls))
	var firstErr error
	seen := make(map[string]struct{}, len(urls))

	for _, rawURL := range urls {
		preview, err := s.fetchLinkPreview(ctx, rawURL)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if preview == nil {
			continue
		}
		if _, ok := seen[preview.URL]; ok {
			continue
		}
		seen[preview.URL] = struct{}{}
		previews = append(previews, *preview)
	}

	return previews, firstErr
}

func (s *SupportLinkPreviewService) fetchLinkPreview(ctx context.Context, rawURL string) (*model.SupportLinkPreview, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("parse preview url: %w", err)
	}
	if !isAllowedSupportPreviewURL(parsedURL) {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build preview request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,text/plain;q=0.5,*/*;q=0.1")

	resp, err := s.nextHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("request preview: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("request preview: status %d", resp.StatusCode)
	}

	finalURL := resp.Request.URL
	if finalURL == nil {
		finalURL = parsedURL
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	mediaType := ""
	if contentType != "" {
		mediaType, _, _ = mime.ParseMediaType(contentType)
	}
	if mediaType != "" && !strings.HasPrefix(mediaType, "text/html") && !strings.HasPrefix(mediaType, "text/plain") {
		preview := fallbackSupportLinkPreview(finalURL)
		return &preview, nil
	}

	bodyReader := io.LimitReader(resp.Body, 1024*1024)
	if decodedReader, err := charset.NewReader(bodyReader, contentType); err == nil {
		bodyReader = decodedReader
	}

	doc, err := html.Parse(bodyReader)
	if err != nil {
		return nil, fmt.Errorf("parse preview html: %w", err)
	}

	preview := extractSupportLinkPreview(finalURL, doc)
	return &preview, nil
}

func (s *SupportLinkPreviewService) nextHTTPClient() *http.Client {
	if len(s.clients) == 0 {
		return http.DefaultClient
	}
	idx := s.nextClient.Add(1) - 1
	return s.clients[idx%uint64(len(s.clients))]
}

func extractSupportPreviewURLs(content string, limit int) []string {
	if limit <= 0 {
		limit = 3
	}
	matches := supportPreviewURLPattern.FindAllString(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	result := make([]string, 0, min(limit, len(matches)))
	for _, candidate := range matches {
		cleaned := trimSupportPreviewURL(candidate)
		if cleaned == "" {
			continue
		}
		parsed, err := url.Parse(cleaned)
		if err != nil || !isAllowedSupportPreviewURL(parsed) {
			continue
		}
		parsed.Fragment = ""
		normalized := parsed.String()
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
		if len(result) == limit {
			break
		}
	}
	return result
}

func trimSupportPreviewURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	for trimmed != "" {
		last := trimmed[len(trimmed)-1]
		switch last {
		case '.', ',', '!', '?', ':', ';':
			trimmed = trimmed[:len(trimmed)-1]
		case ')':
			if strings.Count(trimmed, "(") < strings.Count(trimmed, ")") {
				trimmed = trimmed[:len(trimmed)-1]
				continue
			}
			return trimmed
		case ']':
			if strings.Count(trimmed, "[") < strings.Count(trimmed, "]") {
				trimmed = trimmed[:len(trimmed)-1]
				continue
			}
			return trimmed
		case '}':
			if strings.Count(trimmed, "{") < strings.Count(trimmed, "}") {
				trimmed = trimmed[:len(trimmed)-1]
				continue
			}
			return trimmed
		default:
			return trimmed
		}
	}
	return ""
}

func isAllowedSupportPreviewURL(parsed *url.URL) bool {
	if parsed == nil {
		return false
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return false
	}
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".local") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
			return false
		}
	}
	return true
}

func extractSupportLinkPreview(pageURL *url.URL, doc *html.Node) model.SupportLinkPreview {
	title := ""
	ogTitle := ""
	twitterTitle := ""
	description := ""
	ogDescription := ""
	twitterDescription := ""
	siteName := ""
	ogSiteName := ""
	ogImage := ""
	twitterImage := ""
	canonicalURL := ""
	ogURL := ""

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode {
			switch node.DataAtom {
			case atom.Title:
				if title == "" && node.FirstChild != nil {
					title = strings.TrimSpace(node.FirstChild.Data)
				}
			case atom.Meta:
				name := strings.ToLower(strings.TrimSpace(supportHTMLAttr(node, "name")))
				property := strings.ToLower(strings.TrimSpace(supportHTMLAttr(node, "property")))
				content := strings.TrimSpace(supportHTMLAttr(node, "content"))
				switch {
				case property == "og:title" && ogTitle == "":
					ogTitle = content
				case property == "og:description" && ogDescription == "":
					ogDescription = content
				case property == "og:site_name" && ogSiteName == "":
					ogSiteName = content
				case property == "og:image" && ogImage == "":
					ogImage = content
				case property == "og:url" && ogURL == "":
					ogURL = content
				case name == "twitter:title" && twitterTitle == "":
					twitterTitle = content
				case name == "twitter:description" && twitterDescription == "":
					twitterDescription = content
				case (name == "twitter:image" || name == "twitter:image:src") && twitterImage == "":
					twitterImage = content
				case name == "description" && description == "":
					description = content
				case name == "application-name" && siteName == "":
					siteName = content
				}
			case atom.Link:
				rel := strings.ToLower(strings.TrimSpace(supportHTMLAttr(node, "rel")))
				href := strings.TrimSpace(supportHTMLAttr(node, "href"))
				if strings.Contains(rel, "canonical") && canonicalURL == "" {
					canonicalURL = href
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	finalTitle := firstNonEmptySupportLinkValue(ogTitle, twitterTitle, title)
	if finalTitle == "" {
		finalTitle = fallbackSupportLinkTitle(pageURL)
	}

	finalDescription := normalizedSupportLinkText(firstNonEmptySupportLinkValue(ogDescription, twitterDescription, description))
	finalSiteName := normalizedSupportLinkText(firstNonEmptySupportLinkValue(ogSiteName, siteName))
	finalImageURL := resolveSupportLinkPreviewURL(pageURL, firstNonEmptySupportLinkValue(ogImage, twitterImage))
	finalTargetURL := resolveSupportLinkPreviewURL(pageURL, firstNonEmptySupportLinkValue(ogURL, canonicalURL))
	if finalTargetURL == "" && pageURL != nil {
		finalTargetURL = pageURL.String()
	}

	preview := model.SupportLinkPreview{
		URL:   finalTargetURL,
		Title: normalizedSupportLinkText(finalTitle),
		Host:  supportLinkPreviewHost(pageURL),
	}
	if preview.Title == "" {
		preview.Title = fallbackSupportLinkTitle(pageURL)
	}
	if finalDescription != "" {
		preview.Description = &finalDescription
	}
	if finalSiteName != "" {
		preview.SiteName = &finalSiteName
	}
	if finalImageURL != "" {
		preview.ImageURL = &finalImageURL
	}
	return preview
}

func supportHTMLAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

func resolveSupportLinkPreviewURL(base *url.URL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if base == nil {
		return parsed.String()
	}
	return base.ResolveReference(parsed).String()
}

func normalizedSupportLinkText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func firstNonEmptySupportLinkValue(values ...string) string {
	for _, value := range values {
		if trimmed := normalizedSupportLinkText(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func fallbackSupportLinkPreview(pageURL *url.URL) model.SupportLinkPreview {
	target := ""
	if pageURL != nil {
		target = pageURL.String()
	}
	return model.SupportLinkPreview{
		URL:   target,
		Title: fallbackSupportLinkTitle(pageURL),
		Host:  supportLinkPreviewHost(pageURL),
	}
}

func fallbackSupportLinkTitle(pageURL *url.URL) string {
	if pageURL == nil {
		return "Link preview"
	}
	lastSegment := strings.TrimSpace(path.Base(pageURL.EscapedPath()))
	lastSegment = strings.Trim(lastSegment, "/")
	lastSegment, _ = url.PathUnescape(lastSegment)
	lastSegment = strings.ReplaceAll(lastSegment, "-", " ")
	lastSegment = strings.ReplaceAll(lastSegment, "_", " ")
	lastSegment = normalizedSupportLinkText(lastSegment)
	if lastSegment != "" && lastSegment != "." {
		return lastSegment
	}
	host := supportLinkPreviewHost(pageURL)
	if host != "" {
		return host
	}
	return "Link preview"
}

func supportLinkPreviewHost(pageURL *url.URL) string {
	if pageURL == nil {
		return ""
	}
	return strings.TrimSpace(pageURL.Hostname())
}

func mergeSupportLinkPreviewMetadata(existing string, previews []model.SupportLinkPreview) (string, error) {
	if len(previews) == 0 {
		return existing, nil
	}

	metadata := map[string]any{}
	if trimmed := strings.TrimSpace(existing); trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &metadata); err != nil {
			return "", fmt.Errorf("unmarshal existing metadata: %w", err)
		}
	}
	metadata[supportLinkPreviewMetadataKey] = previews

	encoded, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal preview metadata: %w", err)
	}
	return string(encoded), nil
}
