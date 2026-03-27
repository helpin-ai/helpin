package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupDocsRedirectTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-redirect-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	stmt := `CREATE TABLE docs_redirects (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		source_path TEXT NOT NULL,
		target_collection_slug TEXT NOT NULL,
		target_article_slug TEXT,
		type TEXT NOT NULL,
		source_system TEXT,
		source_object_type TEXT,
		source_object_id TEXT,
		created_at DATETIME
	)`
	if err := db.Exec(stmt).Error; err != nil {
		t.Fatalf("create docs_redirects table: %v", err)
	}

	for _, extra := range []string{
		`CREATE TABLE docs_collections (
			id TEXT PRIMARY KEY,
			space_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			slug TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT
		)`,
		`CREATE TABLE docs_helpcenter_articles (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL DEFAULT ''
		)`,
	} {
		if err := db.Exec(extra).Error; err != nil {
			t.Fatalf("create docs redirect dependency table: %v", err)
		}
	}

	return db
}

func TestDocsRedirectRepository_ListNormalizesMalformedStoredPaths(t *testing.T) {
	db := setupDocsRedirectTestDB(t)
	repo := NewDocsRedirectRepository(db)
	ctx := context.Background()

	redirect := model.DocsRedirect{
		ID:                   "redir-1",
		WorkspaceID:          "ws-1",
		SourcePath:           "//start-here",
		TargetCollectionSlug: "",
		TargetArticleSlug:    docsRedirectStringPtr("start-here-2"),
		Type:                 model.RedirectTypeSlugChange,
	}
	if err := db.Create(&redirect).Error; err != nil {
		t.Fatalf("seed redirect: %v", err)
	}

	items, _, err := repo.List(ctx, "ws-1", model.DocsRedirectFilter{})
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 redirect, got %d", len(items))
	}
	if items[0].SourcePath != "/start-here" {
		t.Fatalf("expected normalized source_path, got %q", items[0].SourcePath)
	}

	var stored model.DocsRedirect
	if err := db.Where("id = ?", redirect.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload stored redirect: %v", err)
	}
	if stored.SourcePath != "/start-here" {
		t.Fatalf("expected stored source_path to be repaired, got %q", stored.SourcePath)
	}
}

func TestDocsRedirectRepository_GetBySourcePathFindsLegacyDoubleSlashRows(t *testing.T) {
	db := setupDocsRedirectTestDB(t)
	repo := NewDocsRedirectRepository(db)
	ctx := context.Background()

	redirect := model.DocsRedirect{
		ID:                   "redir-2",
		WorkspaceID:          "ws-1",
		SourcePath:           "//legacy-article",
		TargetCollectionSlug: "",
		TargetArticleSlug:    docsRedirectStringPtr("updated-article"),
		Type:                 model.RedirectTypeSlugChange,
	}
	if err := db.Create(&redirect).Error; err != nil {
		t.Fatalf("seed redirect: %v", err)
	}

	found, err := repo.GetBySourcePath(ctx, "ws-1", "/legacy-article")
	if err != nil {
		t.Fatalf("get by source path: %v", err)
	}
	if found == nil {
		t.Fatalf("expected redirect to be found after normalization")
	}
	if found.SourcePath != "/legacy-article" {
		t.Fatalf("expected normalized source_path, got %q", found.SourcePath)
	}
}

func TestDocsRedirectRepository_ListBackfillsMissingCollectionSlugForCategorizedArticle(t *testing.T) {
	db := setupDocsRedirectTestDB(t)
	repo := NewDocsRedirectRepository(db)
	ctx := context.Background()

	if err := db.Exec(`INSERT INTO docs_collections (id, space_id, workspace_id, name, slug) VALUES (?, ?, ?, ?, ?)`,
		"coll-1", "space-1", "ws-1", "Getting Started", "",
	).Error; err != nil {
		t.Fatalf("seed collection: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, space_id, collection_id) VALUES (?, ?, ?, ?)`,
		"doc-1", "ws-1", "space-1", "coll-1",
	).Error; err != nil {
		t.Fatalf("seed document: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_helpcenter_articles (id, document_id, slug) VALUES (?, ?, ?)`,
		"article-1", "doc-1", "start-here11",
	).Error; err != nil {
		t.Fatalf("seed helpcenter article: %v", err)
	}

	redirect := model.DocsRedirect{
		ID:                   "redir-3",
		WorkspaceID:          "ws-1",
		SourcePath:           "//start-here1",
		TargetCollectionSlug: "",
		TargetArticleSlug:    docsRedirectStringPtr("start-here11"),
		Type:                 model.RedirectTypeSlugChange,
	}
	if err := db.Create(&redirect).Error; err != nil {
		t.Fatalf("seed redirect: %v", err)
	}

	items, _, err := repo.List(ctx, "ws-1", model.DocsRedirectFilter{})
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 redirect, got %d", len(items))
	}
	if items[0].TargetCollectionSlug != "getting-started" {
		t.Fatalf("expected target collection slug to be backfilled, got %q", items[0].TargetCollectionSlug)
	}
	if items[0].SourcePath != "/getting-started/start-here1" {
		t.Fatalf("expected source path to include backfilled collection slug, got %q", items[0].SourcePath)
	}

	var storedCollection struct {
		Slug string
	}
	if err := db.Raw(`SELECT slug FROM docs_collections WHERE id = ?`, "coll-1").Scan(&storedCollection).Error; err != nil {
		t.Fatalf("reload collection: %v", err)
	}
	if storedCollection.Slug != "getting-started" {
		t.Fatalf("expected collection slug to be backfilled, got %q", storedCollection.Slug)
	}
}

func TestDocsRedirectRepository_ListRepairsBlankCollectionSlugAfterPathWasAlreadyNormalized(t *testing.T) {
	db := setupDocsRedirectTestDB(t)
	repo := NewDocsRedirectRepository(db)
	ctx := context.Background()

	if err := db.Exec(`INSERT INTO docs_collections (id, space_id, workspace_id, name, slug) VALUES (?, ?, ?, ?, ?)`,
		"coll-2", "space-1", "ws-1", "Getting Started", "",
	).Error; err != nil {
		t.Fatalf("seed collection: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, space_id, collection_id) VALUES (?, ?, ?, ?)`,
		"doc-2", "ws-1", "space-1", "coll-2",
	).Error; err != nil {
		t.Fatalf("seed document: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_helpcenter_articles (id, document_id, slug) VALUES (?, ?, ?)`,
		"article-2", "doc-2", "start-here12",
	).Error; err != nil {
		t.Fatalf("seed helpcenter article: %v", err)
	}

	redirect := model.DocsRedirect{
		ID:                   "redir-4",
		WorkspaceID:          "ws-1",
		SourcePath:           "/start-here2",
		TargetCollectionSlug: "",
		TargetArticleSlug:    docsRedirectStringPtr("start-here12"),
		Type:                 model.RedirectTypeSlugChange,
	}
	if err := db.Create(&redirect).Error; err != nil {
		t.Fatalf("seed redirect: %v", err)
	}

	items, _, err := repo.List(ctx, "ws-1", model.DocsRedirectFilter{})
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 redirect, got %d", len(items))
	}
	if items[0].TargetCollectionSlug != "getting-started" {
		t.Fatalf("expected target collection slug to be backfilled, got %q", items[0].TargetCollectionSlug)
	}
	if items[0].SourcePath != "/getting-started/start-here2" {
		t.Fatalf("expected source path to include collection slug, got %q", items[0].SourcePath)
	}
}

func docsRedirectStringPtr(value string) *string {
	return &value
}
