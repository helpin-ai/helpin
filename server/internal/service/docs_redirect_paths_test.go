package service

import "testing"

func TestBuildDocsRedirectPath_CollectionlessArticleUsesSingleLeadingSlash(t *testing.T) {
	articleSlug := "updated-article"

	got := buildDocsRedirectPath("", &articleSlug)

	if got != "/updated-article" {
		t.Fatalf("expected collectionless article path to use single leading slash, got %q", got)
	}
}

func TestNormalizeDocsRedirectSourcePath_CollapsesExtraLeadingSlashes(t *testing.T) {
	got := normalizeDocsRedirectSourcePath("//legacy-article")

	if got != "/legacy-article" {
		t.Fatalf("expected normalized source path, got %q", got)
	}
}
