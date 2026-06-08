package repository

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDocsSearchRepository_PublicSearchPostgresSQLDoesNotSelectUndefinedSnippet(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("docs_search.go")
	if err != nil {
		t.Fatalf("read docs_search.go: %v", err)
	}
	if strings.Contains(string(source), "me.snippet AS snippet") {
		t.Fatalf("Postgres public search selects me.snippet, but matched_entries does not define a snippet column")
	}
}

func TestPublicSearchTermsFiltersStopwordsAndDuplicates(t *testing.T) {
	t.Parallel()

	terms := publicSearchTerms("Days to convert to DAYS")
	got := strings.Join(terms, ",")
	if got != "days,convert" {
		t.Fatalf("publicSearchTerms() = %q, want %q", got, "days,convert")
	}
}

func TestBuildPublicSearchSnippetHighlightsWholeTermsOnly(t *testing.T) {
	t.Parallel()

	snippet := buildPublicSearchSnippet(
		"Visitors took days to convert. Customers pressed the button automatically.",
		"days to convert",
	)
	if strings.Contains(snippet, "<mark>to</mark>") {
		t.Fatalf("snippet highlights stopword 'to': %s", snippet)
	}
	if strings.Contains(snippet, "cus<mark>to</mark>mer") ||
		strings.Contains(snippet, "but<mark>to</mark>n") ||
		strings.Contains(snippet, "au<mark>to</mark>matically") {
		t.Fatalf("snippet highlights query terms inside larger words: %s", snippet)
	}
	if !strings.Contains(snippet, "<mark>days</mark>") || !strings.Contains(snippet, "<mark>convert</mark>") {
		t.Fatalf("snippet did not highlight meaningful terms: %s", snippet)
	}
}

func setupDocsSearchPathTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-search-path-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	stmts := []string{
		`CREATE TABLE docs_collections (
			id TEXT PRIMARY KEY,
			space_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
				parent_collection_id TEXT,
				depth INTEGER NOT NULL DEFAULT 0,
				name TEXT NOT NULL,
				public_id TEXT NOT NULL DEFAULT '',
				slug TEXT NOT NULL DEFAULT '',
				deleted_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_collection_translations (
			id TEXT PRIMARY KEY,
			collection_id TEXT NOT NULL,
			locale TEXT NOT NULL,
			name TEXT NOT NULL,
			slug TEXT,
			status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create docs search path test table: %v", err)
		}
	}
	return db
}

// TestDocsSearchRepository_BuildLocalizedCollectionPath verifies the
// ancestor-path builder used to enrich public search results with
// breadcrumbs. It should:
//   - walk the ancestor chain top-down
//   - prefer the locale translation name when present
//   - fall back to the source name when no translation exists
//   - return nil when the collection has no ancestors worth showing
func TestDocsSearchRepository_BuildLocalizedCollectionPath(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (*gorm.DB, *DocsSearchRepository, context.Context) {
		t.Helper()
		db := setupDocsSearchPathTestDB(t)
		repo := NewDocsSearchRepository(db)
		return db, repo, context.Background()
	}

	seedCollection := func(t *testing.T, db *gorm.DB, id string, parent *string, name string) {
		t.Helper()
		if err := db.Exec(`
			INSERT INTO docs_collections (id, space_id, workspace_id, parent_collection_id, depth, name, slug)
			VALUES (?, ?, ?, ?, 0, ?, ?)
		`, id, "space-1", "ws-1", parent, name, id).Error; err != nil {
			t.Fatalf("seed collection %s: %v", id, err)
		}
	}

	seedTranslation := func(t *testing.T, db *gorm.DB, collectionID, locale, name string) {
		t.Helper()
		if err := db.Exec(`
			INSERT INTO docs_helpcenter_collection_translations
				(id, collection_id, locale, name, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, 'published', datetime('now'), datetime('now'))
		`, "tr-"+collectionID+"-"+locale, collectionID, locale, name).Error; err != nil {
			t.Fatalf("seed translation %s/%s: %v", collectionID, locale, err)
		}
	}

	t.Run("returns top-down localized path for a nested leaf", func(t *testing.T) {
		db, repo, ctx := setup(t)
		rootID := "root"
		midID := "mid"
		seedCollection(t, db, "root", nil, "Root")
		seedCollection(t, db, "mid", &rootID, "Middle")
		seedCollection(t, db, "leaf", &midID, "Leaf")

		// French translations for root + mid only. Leaf falls back.
		seedTranslation(t, db, "root", "fr", "Racine")
		seedTranslation(t, db, "mid", "fr", "Milieu")

		path, err := repo.buildLocalizedCollectionPath(ctx, "leaf", "fr")
		if err != nil {
			t.Fatalf("buildLocalizedCollectionPath: %v", err)
		}
		if path == nil {
			t.Fatalf("path = nil, want non-nil breadcrumb")
		}
		if *path != "Racine / Milieu / Leaf" {
			t.Fatalf("path = %q, want %q", *path, "Racine / Milieu / Leaf")
		}
	})

	t.Run("falls back to source names for an unsupported locale", func(t *testing.T) {
		db, repo, ctx := setup(t)
		rootID := "root"
		seedCollection(t, db, "root", nil, "Root")
		seedCollection(t, db, "child", &rootID, "Child")

		path, err := repo.buildLocalizedCollectionPath(ctx, "child", "de")
		if err != nil {
			t.Fatalf("buildLocalizedCollectionPath: %v", err)
		}
		if path == nil || *path != "Root / Child" {
			t.Fatalf("path = %v, want Root / Child", path)
		}
	})

	t.Run("returns nil for a top-level collection with no ancestors", func(t *testing.T) {
		db, repo, ctx := setup(t)
		seedCollection(t, db, "only", nil, "Only")
		path, err := repo.buildLocalizedCollectionPath(ctx, "only", "en")
		if err != nil {
			t.Fatalf("buildLocalizedCollectionPath: %v", err)
		}
		if path != nil {
			t.Fatalf("path = %v, want nil", path)
		}
	})

	t.Run("returns nil for a missing collection", func(t *testing.T) {
		_, repo, ctx := setup(t)
		path, err := repo.buildLocalizedCollectionPath(ctx, "ghost", "en")
		if err != nil {
			t.Fatalf("buildLocalizedCollectionPath: %v", err)
		}
		if path != nil {
			t.Fatalf("path = %v, want nil for missing id", path)
		}
	})
}

func setupDocsPublicSearchIndexTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-public-search-index-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	stmts := []string{
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_configs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			default_locale TEXT NOT NULL
		)`,
		`CREATE TABLE docs_helpcenter_articles (
			document_id TEXT PRIMARY KEY,
			public_id TEXT NOT NULL,
			public_published_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_article_publications (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			locale TEXT NOT NULL,
			title TEXT NOT NULL,
			slug TEXT NOT NULL,
			excerpt TEXT,
			content_text TEXT,
			published_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_space_translations (
			id TEXT PRIMARY KEY,
			space_id TEXT NOT NULL,
			locale TEXT NOT NULL,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			status TEXT NOT NULL,
			published_at DATETIME
		)`,
		`CREATE TABLE docs_collections (
			id TEXT PRIMARY KEY,
			space_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			parent_collection_id TEXT,
			depth INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			public_id TEXT NOT NULL DEFAULT '',
			slug TEXT NOT NULL DEFAULT '',
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_collection_translations (
			id TEXT PRIMARY KEY,
			collection_id TEXT NOT NULL,
			locale TEXT NOT NULL,
			name TEXT NOT NULL,
			slug TEXT,
			status TEXT NOT NULL,
			published_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_search_entries (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			locale TEXT NOT NULL,
			entry_key TEXT NOT NULL,
			entry_type TEXT NOT NULL,
			content TEXT NOT NULL,
			section_title TEXT,
			anchor TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			rank_weight REAL NOT NULL DEFAULT 1,
			search_config TEXT NOT NULL DEFAULT 'simple',
			search_vector TEXT NOT NULL DEFAULT '',
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create public search index test table: %v", err)
		}
	}
	return db
}

func TestDocsSearchRepository_PublicSearchUsesStructuredIndex(t *testing.T) {
	t.Parallel()

	db := setupDocsPublicSearchIndexTestDB(t)
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	repo := NewDocsSearchRepository(db)

	if err := db.Exec(`INSERT INTO docs_helpcenter_configs (id, workspace_id, default_locale) VALUES (?, ?, ?)`, "cfg-1", "ws-1", "en").Error; err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO docs_helpcenter_space_translations (id, space_id, locale, name, slug, status, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "space-tr-en", "space-1", "en", "Docs", "docs", model.DocsHelpcenterTranslationStatusPublished, now).Error; err != nil {
		t.Fatalf("seed space en: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO docs_helpcenter_space_translations (id, space_id, locale, name, slug, status, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "space-tr-fr", "space-1", "fr", "Docs FR", "docs", model.DocsHelpcenterTranslationStatusPublished, now).Error; err != nil {
		t.Fatalf("seed space fr: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO docs_collections (id, space_id, workspace_id, name, public_id, slug)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "collection-1", "space-1", "ws-1", "Billing", "colpub1", "billing").Error; err != nil {
		t.Fatalf("seed collection: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO docs_helpcenter_collection_translations (id, collection_id, locale, name, slug, status, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "collection-tr-en", "collection-1", "en", "Billing", "billing", model.DocsHelpcenterTranslationStatusPublished, now).Error; err != nil {
		t.Fatalf("seed collection translation: %v", err)
	}

	seedArticle := func(id, publicID, locale, title, slug, entryType, entryContent string, weight float64, anchor *string) {
		t.Helper()
		if err := db.Exec(`INSERT OR IGNORE INTO docs_documents (id, workspace_id, space_id, title, status) VALUES (?, ?, ?, ?, ?)`, id, "ws-1", "space-1", title, model.DocStatusPublished).Error; err != nil {
			t.Fatalf("seed document %s: %v", id, err)
		}
		if err := db.Exec(`INSERT OR IGNORE INTO docs_helpcenter_articles (document_id, public_id, public_published_at) VALUES (?, ?, ?)`, id, publicID, now).Error; err != nil {
			t.Fatalf("seed article %s: %v", id, err)
		}
		if err := db.Exec(`
			INSERT INTO docs_helpcenter_article_publications
				(id, document_id, workspace_id, space_id, collection_id, locale, title, slug, excerpt, content_text, published_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "pub-"+id+"-"+locale, id, "ws-1", "space-1", "collection-1", locale, title, slug, "Article excerpt", "legacy text deliberately does not include the misspelled query", now, now).Error; err != nil {
			t.Fatalf("seed publication %s/%s: %v", id, locale, err)
		}
		if err := db.Exec(`
			INSERT INTO docs_helpcenter_search_entries
				(id, workspace_id, document_id, locale, entry_key, entry_type, content, section_title, anchor, position, rank_weight, search_config)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "entry-"+id+"-"+locale+"-"+entryType, "ws-1", id, locale, entryType+":0", entryType, entryContent, "Troubleshooting", anchor, 0, weight, "simple").Error; err != nil {
			t.Fatalf("seed search entry %s/%s: %v", id, locale, err)
		}
	}

	anchor := "delivery-failures"
	seedArticle("doc-title", "pubtitle", "en", "Webhook troubleshooting", "webhook-troubleshooting", model.DocsHelpcenterSearchEntryTypeTitle, "Webhook troubleshooting", 8, nil)
	seedArticle("doc-body", "pubbody", "en", "Delivery settings", "delivery-settings", model.DocsHelpcenterSearchEntryTypeBody, "Webhook endpoint delivery failures and retries", 1, &anchor)
	seedArticle("doc-fr", "pubfr", "fr", "Webhook en francais", "webhook-fr", model.DocsHelpcenterSearchEntryTypeTitle, "Webhook en francais", 8, nil)

	results, err := repo.PublicSearch(context.Background(), "ws-1", "en", "webhok", "docs", 10)
	if err != nil {
		t.Fatalf("PublicSearch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2: %+v", len(results), results)
	}
	if results[0].ID != "doc-title" {
		t.Fatalf("first result id = %q, want title-weighted doc-title: %+v", results[0].ID, results)
	}
	if results[0].Locale != "en" || results[0].Title == "Webhook en francais" {
		t.Fatalf("first result locale/title = %q/%q, want only English results", results[0].Locale, results[0].Title)
	}
	if len(results[1].Matches) == 0 {
		t.Fatalf("second result matches = empty, want section match: %+v", results[1])
	}
	if results[1].Matches[0].Anchor == nil || *results[1].Matches[0].Anchor != anchor {
		t.Fatalf("second result match = %+v, want anchor %q", results[1].Matches[0], anchor)
	}
	if results[1].Matches[0].Snippet == "" {
		t.Fatalf("second result match snippet is empty: %+v", results[1].Matches[0])
	}
}
