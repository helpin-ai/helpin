package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestDocsEmbedResolverServiceResolveOpenGraph(t *testing.T) {
	html := `<!doctype html><html><head>
			<meta property="og:title" content="Demo Doc">
			<meta property="og:description" content="A useful embedded page">
			<meta property="og:image" content="/preview.png">
			<meta property="og:site_name" content="Demo">
		</head><body>ok</body></html>`

	resolver := &DocsEmbedResolverService{
		clients: []*http.Client{{
			Transport: docsEmbedRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
					Body:       io.NopCloser(bytes.NewBufferString(html)),
					Request:    req,
				}, nil
			}),
		}},
		timeout: time.Second,
	}
	got, err := resolver.Resolve(context.Background(), "https://example.com/page")
	if err != nil {
		t.Fatalf("resolve embed: %v", err)
	}
	if got.Title != "Demo Doc" {
		t.Fatalf("title = %q, want Demo Doc", got.Title)
	}
	if got.Description != "A useful embedded page" {
		t.Fatalf("description = %q", got.Description)
	}
	if got.ImageURL == nil || *got.ImageURL != "https://example.com/preview.png" {
		t.Fatalf("image url = %#v", got.ImageURL)
	}
}

func TestDocsEmbedResolverServiceRejectsPrivateURL(t *testing.T) {
	resolver := NewDocsEmbedResolverService("")
	if _, err := resolver.Resolve(context.Background(), "http://127.0.0.1/internal"); err == nil {
		t.Fatal("expected private URL to be rejected")
	}
}

type docsEmbedRoundTripFunc func(*http.Request) (*http.Response, error)

func (f docsEmbedRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
