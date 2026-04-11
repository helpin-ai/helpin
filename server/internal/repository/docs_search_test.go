package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
