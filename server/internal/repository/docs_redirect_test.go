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
		target_path TEXT,
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
				public_id TEXT NOT NULL DEFAULT '',
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
			public_id TEXT NOT NULL DEFAULT '',
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

func TestDocsRedirectRepository_GetBySourcePathDoesNotRepairLegacyDoubleSlashRows(t *testing.T) {
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
	if found != nil {
		t.Fatalf("expected hot-path lookup to avoid read-time repair, got %#v", found)
	}

	if err := MigrateDocsRedirectPaths(db); err != nil {
		t.Fatalf("migrate redirect paths: %v", err)
	}
	found, err = repo.GetBySourcePath(ctx, "ws-1", "/legacy-article")
	if err != nil {
		t.Fatalf("get by source path after migration: %v", err)
	}
	if found == nil || found.SourcePath != "/legacy-article" {
		t.Fatalf("expected migration-normalized redirect, got %#v", found)
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
	if err := db.Exec(`INSERT INTO docs_helpcenter_articles (id, document_id, public_id, slug) VALUES (?, ?, ?, ?)`,
		"article-1", "doc-1", "aa11bb22", "start-here11",
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
	if err := db.Exec(`INSERT INTO docs_helpcenter_articles (id, document_id, public_id, slug) VALUES (?, ?, ?, ?)`,
		"article-2", "doc-2", "cc33dd44", "start-here12",
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

func TestDocsRedirectRepository_UpsertImported(t *testing.T) {
	t.Run("refreshes destination for the same imported source", func(t *testing.T) {
		db := setupDocsRedirectTestDB(t)
		repo := NewDocsRedirectRepository(db)
		ctx := context.Background()

		oldSlug := "setup"
		oldPath := "/articles/setup-oldpublic"
		created, err := repo.UpsertImported(ctx, &model.DocsRedirect{
			ID:                   "redirect-old",
			WorkspaceID:          "ws-1",
			SourcePath:           "/article/1-setup",
			TargetCollectionSlug: "guides",
			TargetArticleSlug:    &oldSlug,
			TargetPath:           &oldPath,
			Type:                 model.RedirectTypeImported,
			SourceSystem:         docsRedirectStringPtr("helpscout"),
			SourceObjectType:     docsRedirectStringPtr("article"),
			SourceObjectID:       docsRedirectStringPtr("source-1"),
		})
		if err != nil || !created {
			t.Fatalf("initial upsert: created=%v err=%v", created, err)
		}

		newSlug := "setup-updated"
		newPath := "/articles/setup-updated-newpublic"
		created, err = repo.UpsertImported(ctx, &model.DocsRedirect{
			ID:                   "redirect-new",
			WorkspaceID:          "ws-1",
			SourcePath:           "/article/1-setup",
			TargetCollectionSlug: "new-guides",
			TargetArticleSlug:    &newSlug,
			TargetPath:           &newPath,
			Type:                 model.RedirectTypeImported,
			SourceSystem:         docsRedirectStringPtr("HelpScout"),
			SourceObjectType:     docsRedirectStringPtr("article"),
			SourceObjectID:       docsRedirectStringPtr("source-1"),
		})
		if err != nil || created {
			t.Fatalf("refresh upsert: created=%v err=%v", created, err)
		}

		got, err := repo.GetBySourcePath(ctx, "ws-1", "/article/1-setup")
		if err != nil {
			t.Fatalf("get refreshed redirect: %v", err)
		}
		if got == nil || got.ID != "redirect-old" || got.TargetPath == nil || *got.TargetPath != newPath {
			t.Fatalf("redirect destination was not refreshed: %+v", got)
		}
		if got.TargetCollectionSlug != "new-guides" || got.TargetArticleSlug == nil || *got.TargetArticleSlug != newSlug {
			t.Fatalf("redirect slugs were not refreshed: %+v", got)
		}
	})

	t.Run("preserves a manual redirect at the same source path", func(t *testing.T) {
		db := setupDocsRedirectTestDB(t)
		repo := NewDocsRedirectRepository(db)
		ctx := context.Background()

		manualPath := "/articles/custom-target"
		manual := model.DocsRedirect{
			ID:                   "manual-1",
			WorkspaceID:          "ws-1",
			SourcePath:           "/article/1-setup",
			TargetCollectionSlug: "custom",
			TargetPath:           &manualPath,
			Type:                 model.RedirectTypeManual,
		}
		if err := db.Create(&manual).Error; err != nil {
			t.Fatalf("seed manual redirect: %v", err)
		}

		importPath := "/articles/setup-imported"
		created, err := repo.UpsertImported(ctx, &model.DocsRedirect{
			ID:                   "imported-1",
			WorkspaceID:          "ws-1",
			SourcePath:           "/article/1-setup",
			TargetCollectionSlug: "guides",
			TargetPath:           &importPath,
			Type:                 model.RedirectTypeImported,
			SourceSystem:         docsRedirectStringPtr("helpscout"),
			SourceObjectID:       docsRedirectStringPtr("source-1"),
		})
		if err != nil || created {
			t.Fatalf("upsert over manual: created=%v err=%v", created, err)
		}

		got, err := repo.GetBySourcePath(ctx, "ws-1", "/article/1-setup")
		if err != nil {
			t.Fatalf("get manual redirect: %v", err)
		}
		if got == nil || got.Type != model.RedirectTypeManual || got.TargetPath == nil || *got.TargetPath != manualPath {
			t.Fatalf("manual redirect was changed: %+v", got)
		}
	})

	t.Run("preserves an imported redirect owned by another source object", func(t *testing.T) {
		db := setupDocsRedirectTestDB(t)
		repo := NewDocsRedirectRepository(db)
		ctx := context.Background()

		originalPath := "/articles/original"
		original := model.DocsRedirect{
			ID:                   "imported-original",
			WorkspaceID:          "ws-1",
			SourcePath:           "/shared-route",
			TargetCollectionSlug: "original",
			TargetPath:           &originalPath,
			Type:                 model.RedirectTypeImported,
			SourceSystem:         docsRedirectStringPtr("nextra"),
			SourceObjectID:       docsRedirectStringPtr("page-1"),
		}
		if err := db.Create(&original).Error; err != nil {
			t.Fatalf("seed imported redirect: %v", err)
		}

		newPath := "/articles/replacement"
		created, err := repo.UpsertImported(ctx, &model.DocsRedirect{
			ID:                   "imported-replacement",
			WorkspaceID:          "ws-1",
			SourcePath:           "/shared-route",
			TargetCollectionSlug: "replacement",
			TargetPath:           &newPath,
			Type:                 model.RedirectTypeImported,
			SourceSystem:         docsRedirectStringPtr("helpscout"),
			SourceObjectID:       docsRedirectStringPtr("article-1"),
		})
		if err != nil || created {
			t.Fatalf("upsert over other import: created=%v err=%v", created, err)
		}

		got, err := repo.GetBySourcePath(ctx, "ws-1", "/shared-route")
		if err != nil {
			t.Fatalf("get imported redirect: %v", err)
		}
		if got == nil || got.ID != original.ID || got.TargetPath == nil || *got.TargetPath != originalPath {
			t.Fatalf("unrelated imported redirect was changed: %+v", got)
		}
	})
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

	t.Run("preserves manual redirect at the same source path as the new auto redirect", func(t *testing.T) {
		db := setupDocsRedirectTestDB(t)
		repo := NewDocsRedirectRepository(db)
		ctx := context.Background()

		// A user has already created a manual redirect at
		// /A/start-here pointing somewhere of their own choosing.
		manualTarget := "custom-target-slug"
		manual := model.DocsRedirect{
			ID:                   "manual-1",
			WorkspaceID:          "ws-1",
			SourcePath:           "/A/start-here",
			TargetCollectionSlug: "elsewhere",
			TargetArticleSlug:    &manualTarget,
			Type:                 model.RedirectTypeManual,
		}
		if err := db.Create(&manual).Error; err != nil {
			t.Fatalf("seed manual redirect: %v", err)
		}

		// Now an article move tries to write an auto redirect at the
		// same source path. UpsertWithReconciliation must NOT mutate
		// the manual row — user intent wins.
		autoSlug := "start-here"
		if err := repo.UpsertWithReconciliation(ctx, &model.DocsRedirect{
			WorkspaceID:          "ws-1",
			SourcePath:           "/A/start-here",
			TargetCollectionSlug: "B",
			TargetArticleSlug:    &autoSlug,
			Type:                 model.RedirectTypeAutoArticleMove,
		}); err != nil {
			t.Fatalf("upsert (manual at same source): %v", err)
		}

		kept, err := repo.GetBySourcePath(ctx, "ws-1", "/A/start-here")
		if err != nil {
			t.Fatalf("get manual redirect: %v", err)
		}
		if kept == nil {
			t.Fatalf("manual redirect was removed")
		}
		if kept.Type != model.RedirectTypeManual {
			t.Fatalf("type = %q, want manual (untouched)", kept.Type)
		}
		if kept.TargetCollectionSlug != "elsewhere" {
			t.Fatalf("target collection slug = %q, want elsewhere (untouched)", kept.TargetCollectionSlug)
		}
		if kept.TargetArticleSlug == nil || *kept.TargetArticleSlug != manualTarget {
			t.Fatalf("target article slug = %v, want %q (untouched)", kept.TargetArticleSlug, manualTarget)
		}
	})

	t.Run("rejects non-auto redirect types", func(t *testing.T) {
		db := setupDocsRedirectTestDB(t)
		repo := NewDocsRedirectRepository(db)
		ctx := context.Background()

		slug := "x"
		err := repo.UpsertWithReconciliation(ctx, &model.DocsRedirect{
			WorkspaceID:          "ws-1",
			SourcePath:           "/old",
			TargetCollectionSlug: "new",
			TargetArticleSlug:    &slug,
			Type:                 model.RedirectTypeManual,
		})
		if err == nil {
			t.Fatalf("expected error when upserting a manual type, got nil")
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
