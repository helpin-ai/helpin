package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/helpscout"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestResolveHelpScoutCategoryTarget_PrefersStableMatchesAndFallsBackToUncategorized(t *testing.T) {
	mapping := helpscoutCategoryMapping{
		byID: map[string]helpscoutCategoryTarget{
			"cat-id": {collectionID: stringPtr("collection-a"), collectionSlug: "getting-started"},
		},
		bySlug: map[string]helpscoutCategoryTarget{
			"getting-started": {collectionID: stringPtr("collection-a"), collectionSlug: "getting-started"},
		},
		byName: map[string]helpscoutCategoryTarget{
			"getting started": {collectionID: stringPtr("collection-a"), collectionSlug: "getting-started"},
		},
		uncategorized: helpscoutCategoryTarget{collectionSlug: "uncategorized"},
	}

	tests := []struct {
		name       string
		categories []string
		wantID     *string
		wantSlug   string
	}{
		{name: "id match", categories: []string{"cat-id"}, wantID: stringPtr("collection-a"), wantSlug: "getting-started"},
		{name: "slug match", categories: []string{"getting-started"}, wantID: stringPtr("collection-a"), wantSlug: "getting-started"},
		{name: "name match", categories: []string{"Getting Started"}, wantID: stringPtr("collection-a"), wantSlug: "getting-started"},
		{name: "fallback", categories: []string{"unknown-category"}, wantID: nil, wantSlug: "uncategorized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := resolveHelpScoutCategoryTarget(mapping, tt.categories)
			if tt.wantID == nil {
				if target.collectionID != nil {
					t.Fatalf("expected no collection id for uncategorized fallback, got %#v", target.collectionID)
				}
			} else if target.collectionID == nil || *target.collectionID != *tt.wantID {
				t.Fatalf("expected collection id %q, got %#v", *tt.wantID, target.collectionID)
			}
			if target.collectionSlug != tt.wantSlug {
				t.Fatalf("expected collection slug %q, got %q", tt.wantSlug, target.collectionSlug)
			}
		})
	}
}

func TestImportSummaryIncludesQualityCounters(t *testing.T) {
	summary := model.ImportSummary{
		CollectionsCreated:             15,
		ArticlesPublished:              97,
		ArticlesDrafted:                18,
		RedirectsCreated:               130,
		ArticlesUncategorized:          13,
		ArticlesWithConversionWarnings: 9,
		HTMLBlockFallbacks:             2,
		ImageRewriteFailures:           3,
		NormalizedNoteBlocks:           4,
	}

	if summary.ArticlesUncategorized != 13 {
		t.Fatalf("expected uncategorized count to be tracked, got %d", summary.ArticlesUncategorized)
	}
	if summary.NormalizedNoteBlocks != 4 {
		t.Fatalf("expected normalized note block count to be tracked, got %d", summary.NormalizedNoteBlocks)
	}
}

func TestNormalizeDocsImportStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "draft", input: "draft", expect: "draft"},
		{name: "published", input: "published", expect: "published"},
		{name: "match source", input: "match_source", expect: "match_source"},
		{name: "legacy all draft", input: "all_draft", expect: "draft"},
		{name: "empty defaults to match source", input: "", expect: "match_source"},
		{name: "unknown defaults to match source", input: "weird", expect: "match_source"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeDocsImportStatus(tt.input); got != tt.expect {
				t.Fatalf("normalizeDocsImportStatus(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestHelpScoutImportBehavior(t *testing.T) {
	publishedNoDraft := helpscout.ArticleRef{Status: "published", HasDraft: false}
	publishedWithDraft := helpscout.ArticleRef{Status: "published", HasDraft: true}
	draftWithDraft := helpscout.ArticleRef{Status: "notpublished", HasDraft: true}

	if shouldImportHelpScoutDraft(publishedNoDraft, "match_source") {
		t.Fatal("published source without draft should not import draft content")
	}
	if shouldImportHelpScoutDraft(publishedWithDraft, "match_source") {
		t.Fatal("match_source should keep the published source version when Help Scout has unpublished draft changes")
	}
	if !shouldImportHelpScoutDraft(draftWithDraft, "draft") {
		t.Fatal("draft imports should use available draft content")
	}
	if !shouldImportHelpScoutDraft(draftWithDraft, "published") {
		t.Fatal("published imports should use draft content for source articles that are not yet published")
	}

	if shouldPublishImportedArticle(publishedWithDraft, "draft") {
		t.Fatal("draft imports must not publish articles")
	}
	if !shouldPublishImportedArticle(publishedWithDraft, "published") {
		t.Fatal("published imports should publish all articles")
	}
	if !shouldPublishImportedArticle(publishedWithDraft, "match_source") {
		t.Fatal("match_source should publish source-published articles")
	}
	if shouldPublishImportedArticle(draftWithDraft, "match_source") {
		t.Fatal("match_source should keep source-draft articles unpublished")
	}
}
