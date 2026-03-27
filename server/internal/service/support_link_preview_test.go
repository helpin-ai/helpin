package service

import (
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/helpin-ai/helpin/server/internal/model"
)

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
