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
	// Mirror the production (workspace_id, source_path) unique index so
	// OnConflict clauses resolve identically in test and prod.
	if err := db.Exec(`CREATE UNIQUE INDEX idx_docs_redirects_ws_source ON docs_redirects (workspace_id, source_path)`).Error; err != nil {
		t.Fatalf("create unique index on docs_redirects: %v", err)
	}

	for _, extra := range []string{
		`CREATE TABLE docs_collections (
			id TEXT PRIMARY KEY,
			space_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			parent_collection_id TEXT,
			depth INTEGER NOT NULL DEFAULT 0,
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

// TestDocsRedirectRepository_UpsertWithReconciliation verifies the new
// Task 7 helper: it upserts on (workspace_id, source_path), breaks any
// chain where the new target was itself a redirect source, and is a
// no-op for self-referential requests.
func TestDocsRedirectRepository_UpsertWithReconciliation(t *testing.T) {
	t.Parallel()

	t.Run("initial insert creates the row", func(t *testing.T) {
		db := setupDocsRedirectTestDB(t)
		repo := NewDocsRedirectRepository(db)
		ctx := context.Background()

		slug := "start-here"
		err := repo.UpsertWithReconciliation(ctx, &model.DocsRedirect{
			WorkspaceID:          "ws-1",
			SourcePath:           "/old-coll/start-here",
			TargetCollectionSlug: "new-coll",
			TargetArticleSlug:    &slug,
			Type:                 model.RedirectTypeAutoArticleMove,
		})
		if err != nil {
			t.Fatalf("upsert: %v", err)
		}
		got, err := repo.GetBySourcePath(ctx, "ws-1", "/old-coll/start-here")
		if err != nil {
			t.Fatalf("get by source path: %v", err)
		}
		if got == nil {
			t.Fatalf("redirect not found after upsert")
		}
		if got.TargetCollectionSlug != "new-coll" {
			t.Fatalf("target collection slug = %q, want new-coll", got.TargetCollectionSlug)
		}
		if got.Type != model.RedirectTypeAutoArticleMove {
			t.Fatalf("type = %q, want auto_article_move", got.Type)
		}
	})

	t.Run("second upsert on same source replaces target", func(t *testing.T) {
		db := setupDocsRedirectTestDB(t)
		repo := NewDocsRedirectRepository(db)
		ctx := context.Background()

		slug := "start-here"
		must := func(err error) {
			if err != nil {
				t.Fatalf("upsert: %v", err)
			}
		}
		must(repo.UpsertWithReconciliation(ctx, &model.DocsRedirect{
			WorkspaceID:          "ws-1",
			SourcePath:           "/old-coll/start-here",
			TargetCollectionSlug: "new-coll",
			TargetArticleSlug:    &slug,
			Type:                 model.RedirectTypeAutoArticleMove,
		}))
		must(repo.UpsertWithReconciliation(ctx, &model.DocsRedirect{
			WorkspaceID:          "ws-1",
			SourcePath:           "/old-coll/start-here",
			TargetCollectionSlug: "newer-coll",
			TargetArticleSlug:    &slug,
			Type:                 model.RedirectTypeAutoArticleMove,
		}))

		got, err := repo.GetBySourcePath(ctx, "ws-1", "/old-coll/start-here")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got == nil || got.TargetCollectionSlug != "newer-coll" {
			t.Fatalf("second upsert did not replace target: %+v", got)
		}
	})

	t.Run("A -> B -> A move leaves no cycle", func(t *testing.T) {
		db := setupDocsRedirectTestDB(t)
		repo := NewDocsRedirectRepository(db)
		ctx := context.Background()

		slug := "start-here"
		// Step 1: article moves from A to B. Emit /A/start-here -> /B/start-here.
		if err := repo.UpsertWithReconciliation(ctx, &model.DocsRedirect{
			WorkspaceID:          "ws-1",
			SourcePath:           "/A/start-here",
			TargetCollectionSlug: "B",
			TargetArticleSlug:    &slug,
			Type:                 model.RedirectTypeAutoArticleMove,
		}); err != nil {
			t.Fatalf("first move upsert: %v", err)
		}

		// Step 2: article moves back from B to A. Emit /B/start-here -> /A/start-here.
		// This must also DELETE the now-stale /A/start-here row because
		// /A/start-here is the new canonical path.
		if err := repo.UpsertWithReconciliation(ctx, &model.DocsRedirect{
			WorkspaceID:          "ws-1",
			SourcePath:           "/B/start-here",
			TargetCollectionSlug: "A",
			TargetArticleSlug:    &slug,
			Type:                 model.RedirectTypeAutoArticleMove,
		}); err != nil {
			t.Fatalf("second move upsert: %v", err)
		}

		// The old /A/start-here redirect must be gone — otherwise resolving
		// /A/start-here would chain through /B/start-here -> /A/start-here
		// which is the cycle we reconciled.
		old, err := repo.GetBySourcePath(ctx, "ws-1", "/A/start-here")
		if err != nil {
			t.Fatalf("get A: %v", err)
		}
		if old != nil {
			t.Fatalf("stale A redirect still present: %+v", old)
		}

		// The B redirect should point back to A.
		back, err := repo.GetBySourcePath(ctx, "ws-1", "/B/start-here")
		if err != nil {
			t.Fatalf("get B: %v", err)
		}
		if back == nil || back.TargetCollectionSlug != "A" {
			t.Fatalf("expected /B/start-here -> A, got %+v", back)
		}

		// Only one row should remain workspace-wide.
		var count int64
		if err := db.Model(&model.DocsRedirect{}).Where("workspace_id = ?", "ws-1").Count(&count).Error; err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 1 {
			t.Fatalf("redirect row count = %d, want 1", count)
		}
	})

	t.Run("preserves manual redirect at the new target path", func(t *testing.T) {
		db := setupDocsRedirectTestDB(t)
		repo := NewDocsRedirectRepository(db)
		ctx := context.Background()

		// Seed a user-created manual redirect at a canonical path.
		slug := "start-here"
		manual := model.DocsRedirect{
			ID:                   "manual-1",
			WorkspaceID:          "ws-1",
			SourcePath:           "/B/start-here",
			TargetCollectionSlug: "elsewhere",
			TargetArticleSlug:    &slug,
			Type:                 model.RedirectTypeManual,
		}
		if err := db.Create(&manual).Error; err != nil {
			t.Fatalf("seed manual redirect: %v", err)
		}

		// Now an article move writes an auto redirect whose target
		// path happens to match the manual redirect's source path.
		// UpsertWithReconciliation must preserve the manual row.
		if err := repo.UpsertWithReconciliation(ctx, &model.DocsRedirect{
			WorkspaceID:          "ws-1",
			SourcePath:           "/A/start-here",
			TargetCollectionSlug: "B",
			TargetArticleSlug:    &slug,
			Type:                 model.RedirectTypeAutoArticleMove,
		}); err != nil {
			t.Fatalf("upsert with manual preserved: %v", err)
		}

		// The manual redirect must still exist.
		kept, err := repo.GetBySourcePath(ctx, "ws-1", "/B/start-here")
		if err != nil {
			t.Fatalf("get manual redirect: %v", err)
		}
		if kept == nil || kept.Type != model.RedirectTypeManual || kept.TargetCollectionSlug != "elsewhere" {
			t.Fatalf("manual redirect was lost or mutated: %+v", kept)
		}

		// And the new auto redirect must exist alongside it.
		fresh, err := repo.GetBySourcePath(ctx, "ws-1", "/A/start-here")
		if err != nil {
			t.Fatalf("get new redirect: %v", err)
		}
		if fresh == nil || fresh.Type != model.RedirectTypeAutoArticleMove {
			t.Fatalf("new redirect missing or wrong type: %+v", fresh)
		}
	})

	t.Run("self-referential redirect is a no-op", func(t *testing.T) {
		db := setupDocsRedirectTestDB(t)
		repo := NewDocsRedirectRepository(db)
		ctx := context.Background()

		slug := "start-here"
		if err := repo.UpsertWithReconciliation(ctx, &model.DocsRedirect{
			WorkspaceID:          "ws-1",
			SourcePath:           "/A/start-here",
			TargetCollectionSlug: "A",
			TargetArticleSlug:    &slug,
			Type:                 model.RedirectTypeAutoArticleMove,
		}); err != nil {
			t.Fatalf("self upsert: %v", err)
		}
		var count int64
		if err := db.Model(&model.DocsRedirect{}).Where("workspace_id = ?", "ws-1").Count(&count).Error; err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 0 {
			t.Fatalf("self redirect was stored, count = %d", count)
		}
	})
}
