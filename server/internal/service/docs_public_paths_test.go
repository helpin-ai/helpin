package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildDocsHelpcenterCollectionKey(t *testing.T) {
	tests := []struct {
		name     string
		slug     string
		publicID string
		want     string
	}{
		{name: "slug and public id", slug: "getting-started", publicID: "ABC123EF", want: "getting-started-abc123ef"},
		{name: "trims slash wrapped slug", slug: "/getting-started/", publicID: "abc123ef", want: "getting-started-abc123ef"},
		{name: "public id only", slug: "", publicID: "abc123ef", want: "abc123ef"},
		{name: "slug only legacy fallback", slug: "getting-started", publicID: "", want: "getting-started"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildDocsHelpcenterCollectionKey(tt.slug, tt.publicID); got != tt.want {
				t.Fatalf("buildDocsHelpcenterCollectionKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseDocsHelpcenterCollectionKey(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		wantSlug     string
		wantPublicID string
		wantOK       bool
	}{
		{name: "slug and public id", key: "getting-started-ABC123EF", wantSlug: "getting-started", wantPublicID: "abc123ef", wantOK: true},
		{name: "trims slash wrapped key", key: "/getting-started-abc123ef/", wantSlug: "getting-started", wantPublicID: "abc123ef", wantOK: true},
		{name: "slug with dashes", key: "how-to-get-started-abc123ef", wantSlug: "how-to-get-started", wantPublicID: "abc123ef", wantOK: true},
		{name: "slug only legacy fallback", key: "getting-started", wantOK: false},
		{name: "bare public id", key: "abc123ef", wantSlug: "", wantPublicID: "abc123ef", wantOK: true},
		{name: "bare public id uppercase", key: "ABC123EF", wantSlug: "", wantPublicID: "abc123ef", wantOK: true},
		{name: "invalid public id suffix", key: "getting-started-zzzzzzzz", wantOK: false},
		{name: "short suffix", key: "getting-started-abc123", wantOK: false},
		{name: "empty", key: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSlug, gotPublicID, gotOK := parseDocsHelpcenterCollectionKey(tt.key)
			if gotOK != tt.wantOK || gotSlug != tt.wantSlug || gotPublicID != tt.wantPublicID {
				t.Fatalf("parseDocsHelpcenterCollectionKey() = (%q, %q, %v), want (%q, %q, %v)", gotSlug, gotPublicID, gotOK, tt.wantSlug, tt.wantPublicID, tt.wantOK)
			}
		})
	}
}

func TestParseDocsHelpcenterArticleKey(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		wantSlug     string
		wantPublicID string
		wantOK       bool
	}{
		{name: "slug and public id", key: "getting-started-abc123ef", wantSlug: "getting-started", wantPublicID: "abc123ef", wantOK: true},
		{name: "bare public id", key: "abc123ef", wantSlug: "", wantPublicID: "abc123ef", wantOK: true},
		{name: "bare public id uppercase", key: "ABC123EF", wantSlug: "", wantPublicID: "abc123ef", wantOK: true},
		{name: "slug only legacy", key: "getting-started", wantOK: false},
		{name: "invalid hex", key: "zzzzzzzz", wantOK: false},
		{name: "empty", key: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSlug, gotPublicID, gotOK := parseDocsHelpcenterArticleKey(tt.key)
			if gotOK != tt.wantOK || gotSlug != tt.wantSlug || gotPublicID != tt.wantPublicID {
				t.Fatalf("parseDocsHelpcenterArticleKey() = (%q, %q, %v), want (%q, %q, %v)", gotSlug, gotPublicID, gotOK, tt.wantSlug, tt.wantPublicID, tt.wantOK)
			}
		})
	}
}

func TestBuildDocsHelpcenterCollectionCanonicalPathUsesCollectionKey(t *testing.T) {
	cfg := &model.DocsHelpcenterConfig{
		DefaultLocale:  "en",
		EnabledLocales: model.DocsStringArray{"en", "fr"},
	}

	if got := buildDocsHelpcenterCollectionCanonicalPath(cfg, "fr", "getting-started", "ABC123EF"); got != "/fr/c/getting-started-abc123ef" {
		t.Fatalf("localized collection path = %q", got)
	}

	if got := buildDocsHelpcenterCollectionCanonicalPath(nil, "", "getting-started", "abc123ef"); got != "/c/getting-started-abc123ef" {
		t.Fatalf("default collection path = %q", got)
	}

	if got := buildDocsHelpcenterCollectionCanonicalPath(nil, "", "getting-started", ""); got != "/c/getting-started" {
		t.Fatalf("legacy collection path fallback = %q", got)
	}
}
