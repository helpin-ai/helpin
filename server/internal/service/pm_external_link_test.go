package service

import (
	"context"
	"testing"
)

type stubExternalLinkMetadataResolver struct {
	embed *DocsResolvedEmbed
	err   error
}

func (s stubExternalLinkMetadataResolver) Resolve(_ context.Context, _ string) (*DocsResolvedEmbed, error) {
	return s.embed, s.err
}

func TestPMExternalLinkResolveLinkMetadataUsesFetchedTitle(t *testing.T) {
	svc := NewPMExternalLinkService(nil, nil)
	svc.SetMetadataResolver(stubExternalLinkMetadataResolver{embed: &DocsResolvedEmbed{
		URL:   "https://example.com/docs",
		Title: "Example Docs",
	}})

	gotURL, gotTitle := svc.resolveLinkMetadata(context.Background(), "example.com/docs", "")
	if gotURL != "https://example.com/docs" {
		t.Fatalf("url = %q, want https://example.com/docs", gotURL)
	}
	if gotTitle != "Example Docs" {
		t.Fatalf("title = %q, want Example Docs", gotTitle)
	}
}

func TestPMExternalLinkResolveLinkMetadataKeepsProvidedTitle(t *testing.T) {
	svc := NewPMExternalLinkService(nil, nil)
	svc.SetMetadataResolver(stubExternalLinkMetadataResolver{embed: &DocsResolvedEmbed{
		URL:   "https://example.com/fetched",
		Title: "Fetched Title",
	}})

	gotURL, gotTitle := svc.resolveLinkMetadata(context.Background(), "https://example.com/manual", "Manual Title")
	if gotURL != "https://example.com/manual" {
		t.Fatalf("url = %q, want https://example.com/manual", gotURL)
	}
	if gotTitle != "Manual Title" {
		t.Fatalf("title = %q, want Manual Title", gotTitle)
	}
}
