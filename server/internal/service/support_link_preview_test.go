package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type recordingSupportLinkScanner struct {
	urls []string
}

type blockingSupportLinkScanner struct{}

func (blockingSupportLinkScanner) Scan(ctx context.Context, normalizedURL string) model.SupportLinkSecurity {
	<-ctx.Done()
	now := time.Now().UTC()
	return model.SupportLinkSecurity{URL: normalizedURL, Status: supportLinkSecurityUnknown, CheckedAt: now, ExpiresAt: now.Add(30 * time.Second)}
}

type staticSupportResolver struct {
	addresses []netip.Addr
	err       error
}

func (r staticSupportResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r.addresses, r.err
}

func (s *recordingSupportLinkScanner) Scan(_ context.Context, normalizedURL string) model.SupportLinkSecurity {
	s.urls = append(s.urls, normalizedURL)
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	return model.SupportLinkSecurity{
		URL: normalizedURL, Status: supportLinkSecurityNoMatch,
		CheckedAt: now, ExpiresAt: now.Add(5 * time.Minute),
	}
}

func TestExtractSupportPreviewURLs(t *testing.T) {
	got := extractSupportPreviewURLs("Check https://example.com/docs, https://example.com/docs#top and https://example.org/page).", 3)
	if len(got) != 2 {
		t.Fatalf("expected 2 unique urls, got %#v", got)
	}
	if got[0] != "https://example.com/docs" {
		t.Fatalf("first url = %q, want https://example.com/docs", got[0])
	}
	if got[1] != "https://example.org/page" {
		t.Fatalf("second url = %q, want https://example.org/page", got[1])
	}
}

func TestExtractSupportPreviewURLsAcceptsUppercaseScheme(t *testing.T) {
	got := extractSupportPreviewURLs("Open HTTPS://Example.COM:443/path", 3)
	if len(got) != 1 || got[0] != "https://example.com/path" {
		t.Fatalf("urls = %#v", got)
	}
}

func TestSupportLinkPreviewErrorKindDoesNotExposeURL(t *testing.T) {
	err := errors.New(`Get "https://secret.example/path?token=value": connection refused`)
	kind := supportLinkPreviewErrorKind(err)
	if strings.Contains(kind, "secret.example") || strings.Contains(kind, "token") {
		t.Fatalf("error kind exposed URL: %q", kind)
	}
}

func TestMergeSupportLinkPreviewMetadataPreservesExistingFields(t *testing.T) {
	metadata, err := mergeSupportLinkPreviewMetadata(`{"ai_auto_reply":true}`, []model.SupportLinkPreview{{
		URL:   "https://example.com",
		Title: "Example",
		Host:  "example.com",
	}})
	if err != nil {
		t.Fatalf("mergeSupportLinkPreviewMetadata: %v", err)
	}
	if !strings.Contains(metadata, `"ai_auto_reply":true`) {
		t.Fatalf("expected AI metadata to be preserved, got %q", metadata)
	}
	if !strings.Contains(metadata, `"link_previews"`) {
		t.Fatalf("expected link previews to be added, got %q", metadata)
	}
}

func TestEnrichMessagePersistsSecurityWhenPreviewFails(t *testing.T) {
	scanner := &recordingSupportLinkScanner{}
	service := NewSupportLinkPreviewService("")
	service.SetLinkScanner(scanner)
	service.resolver = staticSupportResolver{addresses: []netip.Addr{netip.MustParseAddr("93.184.216.34")}}
	service.clients = []*http.Client{{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("preview unavailable")
	})}}
	msg := &model.SupportMessage{SenderType: "customer", Content: "Open http://example.com/path", Metadata: `{"existing":true}`}

	service.EnrichMessage(context.Background(), msg)

	if len(scanner.urls) != 1 || scanner.urls[0] != "http://example.com/path" {
		t.Fatalf("scanned urls = %#v", scanner.urls)
	}
	if !strings.Contains(msg.Metadata, `"link_security"`) || !strings.Contains(msg.Metadata, `"existing":true`) {
		t.Fatalf("metadata = %s", msg.Metadata)
	}
}

func TestEnrichMessagePreviewSurvivesScannerTimeout(t *testing.T) {
	service := NewSupportLinkPreviewService("")
	service.SetLinkScanner(blockingSupportLinkScanner{})
	service.timeout = 50 * time.Millisecond
	service.resolver = staticSupportResolver{addresses: []netip.Addr{netip.MustParseAddr("93.184.216.34")}}
	service.clients = []*http.Client{{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/html"}},
			Body: io.NopCloser(strings.NewReader(`<html><title>Available preview</title></html>`)), Request: req,
		}, nil
	})}}
	msg := &model.SupportMessage{SenderType: "customer", Content: "https://example.com/path"}

	service.EnrichMessage(context.Background(), msg)

	if !strings.Contains(msg.Metadata, `"title":"Available preview"`) {
		t.Fatalf("metadata = %s", msg.Metadata)
	}
}

func TestEnrichMessageDoesNotScanTrustedAuthorLinks(t *testing.T) {
	for _, senderType := range []string{"user", "agent", "ai"} {
		t.Run(senderType, func(t *testing.T) {
			scanner := &recordingSupportLinkScanner{}
			service := NewSupportLinkPreviewService("")
			service.SetLinkScanner(scanner)
			service.resolver = staticSupportResolver{addresses: []netip.Addr{netip.MustParseAddr("93.184.216.34")}}
			service.clients = []*http.Client{{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/html"}},
					Body: io.NopCloser(strings.NewReader(`<html><title>Trusted author preview</title></html>`)), Request: req,
				}, nil
			})}}
			msg := &model.SupportMessage{SenderType: senderType, Content: "http://example.com/path"}

			service.EnrichMessage(context.Background(), msg)

			if len(scanner.urls) != 0 {
				t.Fatalf("scanned urls = %#v, want none", scanner.urls)
			}
			if !strings.Contains(msg.Metadata, `"title":"Trusted author preview"`) {
				t.Fatalf("metadata = %s", msg.Metadata)
			}
		})
	}
}

func TestValidateSupportPreviewURLRejectsPrivateDNSAnswers(t *testing.T) {
	parsed := mustParseURL(t, "https://example.com/path")
	resolver := staticSupportResolver{addresses: []netip.Addr{
		netip.MustParseAddr("93.184.216.34"),
		netip.MustParseAddr("10.0.0.4"),
	}}

	if _, err := validateSupportPreviewURL(context.Background(), resolver, parsed); err == nil {
		t.Fatal("expected mixed public/private DNS answer to be rejected")
	}
}

func TestValidateSupportPreviewURLAllowsPublicHTTP(t *testing.T) {
	parsed := mustParseURL(t, "http://example.com/path")
	resolver := staticSupportResolver{addresses: []netip.Addr{netip.MustParseAddr("93.184.216.34")}}

	addresses, err := validateSupportPreviewURL(context.Background(), resolver, parsed)
	if err != nil {
		t.Fatalf("validate public URL: %v", err)
	}
	if len(addresses) != 1 || addresses[0].String() != "93.184.216.34" {
		t.Fatalf("addresses = %#v", addresses)
	}
}

func TestFetchLinkPreviewKeepsScannedDestination(t *testing.T) {
	service := NewSupportLinkPreviewService("")
	service.resolver = staticSupportResolver{addresses: []netip.Addr{netip.MustParseAddr("93.184.216.34")}}
	service.clients = []*http.Client{{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/html"}},
			Body:    io.NopCloser(strings.NewReader(`<html><head><title>Example</title><link rel="canonical" href="https://other.example/page"></head></html>`)),
			Request: req,
		}, nil
	})}}

	preview, err := service.fetchLinkPreview(context.Background(), "http://example.com/original")
	if err != nil {
		t.Fatalf("fetch preview: %v", err)
	}
	if preview == nil || preview.URL != "http://example.com/original" {
		t.Fatalf("preview = %#v", preview)
	}
}

func TestExtractSupportLinkPreviewReadsOpenGraphAndCanonical(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`
		<html>
			<head>
				<title>Fallback Title</title>
				<meta property="og:title" content="OG Title" />
				<meta property="og:description" content="OG description" />
				<meta property="og:site_name" content="Example Docs" />
				<meta property="og:image" content="/cover.png" />
				<link rel="canonical" href="https://example.com/docs/guide" />
			</head>
			<body>hi</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	pageURL := mustParseURL(t, "https://example.com/docs/guide?ref=chat")
	preview := extractSupportLinkPreview(pageURL, doc)
	if preview.Title != "OG Title" {
		t.Fatalf("title = %q, want OG Title", preview.Title)
	}
	if preview.URL != "https://example.com/docs/guide" {
		t.Fatalf("url = %q, want canonical url", preview.URL)
	}
	if preview.Description == nil || *preview.Description != "OG description" {
		t.Fatalf("description = %#v, want OG description", preview.Description)
	}
	if preview.SiteName == nil || *preview.SiteName != "Example Docs" {
		t.Fatalf("site_name = %#v, want Example Docs", preview.SiteName)
	}
	if preview.ImageURL == nil || *preview.ImageURL != "https://example.com/cover.png" {
		t.Fatalf("image_url = %#v, want resolved image", preview.ImageURL)
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url %q: %v", raw, err)
	}
	return parsed
}
