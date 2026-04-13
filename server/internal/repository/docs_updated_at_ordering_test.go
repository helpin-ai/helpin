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

func setupDocsUpdatedAtOrderingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-updated-at-ordering-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	stmts := []string{
		`CREATE TABLE docs_spaces (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			icon TEXT,
			visibility TEXT NOT NULL,
			type TEXT NOT NULL,
			default_review_days INTEGER,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
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
			description TEXT,
			icon TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			visibility TEXT NOT NULL,
			owner_id TEXT,
			team_id TEXT,
			template_key TEXT,
			excerpt TEXT,
			icon TEXT,
			tags TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			is_pinned BOOLEAN NOT NULL DEFAULT 0,
			is_publicly_shared BOOLEAN NOT NULL DEFAULT 0,
			share_token TEXT,
			is_locked BOOLEAN NOT NULL DEFAULT 0,
			locked_by TEXT,
			last_reviewed_at DATETIME,
			next_review_at DATETIME,
			published_at DATETIME,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create docs test table: %v", err)
		}
	}

	return db
}

func seedDocsUpdatedAtSpace(t *testing.T, db *gorm.DB, space model.DocsSpace) {
	t.Helper()
	if err := db.Create(&space).Error; err != nil {
		t.Fatalf("seed docs space %s: %v", space.ID, err)
	}
}

func seedDocsUpdatedAtCollection(t *testing.T, db *gorm.DB, coll model.DocsCollection) {
	t.Helper()
	if err := db.Create(&coll).Error; err != nil {
		t.Fatalf("seed docs collection %s: %v", coll.ID, err)
	}
}

func seedDocsUpdatedAtDocument(t *testing.T, db *gorm.DB, doc model.DocsDocument) {
	t.Helper()
	if err := db.Create(&doc).Error; err != nil {
		t.Fatalf("seed docs document %s: %v", doc.ID, err)
	}
}

func TestDocsOrdering_PositionOnlyWritesDoNotTouchUpdatedAt(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-updated-at"
		userID      = "user-updated-at"
	)

	ptr := func(value string) *string { return &value }
	ctx := context.Background()
	now := time.Date(2026, 3, 25, 10, 30, 0, 0, time.UTC)

	t.Run("reorder keeps updated_at stable for spaces collections and documents", func(t *testing.T) {
		db := setupDocsUpdatedAtOrderingTestDB(t)
		spaceRepo := NewDocsSpaceRepository(db)
		collectionRepo := NewDocsCollectionRepository(db)
		docRepo := NewDocsDocumentRepository(db)

		seedDocsUpdatedAtSpace(t, db, model.DocsSpace{
			ID:          "space-a",
			WorkspaceID: workspaceID,
			Name:        "Space A",
			Slug:        "space-a",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsUpdatedAtSpace(t, db, model.DocsSpace{
			ID:          "space-b",
			WorkspaceID: workspaceID,
			Name:        "Space B",
			Slug:        "space-b",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    1,
			CreatedBy:   userID,
			CreatedAt:   now.Add(time.Minute),
			UpdatedAt:   now.Add(time.Minute),
		})
		seedDocsUpdatedAtCollection(t, db, model.DocsCollection{
			ID:          "coll-a",
			SpaceID:     "space-a",
			WorkspaceID: workspaceID,
			Name:        "Collection A",
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsUpdatedAtCollection(t, db, model.DocsCollection{
			ID:          "coll-b",
			SpaceID:     "space-a",
			WorkspaceID: workspaceID,
			Name:        "Collection B",
			Position:    1,
			CreatedBy:   userID,
			CreatedAt:   now.Add(time.Minute),
			UpdatedAt:   now.Add(time.Minute),
		})
		seedDocsUpdatedAtDocument(t, db, model.DocsDocument{
			ID:           "doc-a",
			WorkspaceID:  workspaceID,
			SpaceID:      "space-a",
			CollectionID: ptr("coll-a"),
			Title:        "Document A",
			Status:       model.DocStatusDraft,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     0,
			CreatedBy:    userID,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		seedDocsUpdatedAtDocument(t, db, model.DocsDocument{
			ID:           "doc-b",
			WorkspaceID:  workspaceID,
			SpaceID:      "space-a",
			CollectionID: ptr("coll-a"),
			Title:        "Document B",
			Status:       model.DocStatusDraft,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     1,
			CreatedBy:    userID,
			CreatedAt:    now.Add(time.Minute),
			UpdatedAt:    now.Add(time.Minute),
		})

		if err := spaceRepo.Reorder(ctx, workspaceID, model.SpaceTypeInternal, []string{"space-b", "space-a"}); err != nil {
			t.Fatalf("Reorder spaces: %v", err)
		}
		if err := collectionRepo.ReorderSiblings(ctx, "space-a", nil, []string{"coll-b", "coll-a"}); err != nil {
			t.Fatalf("ReorderSiblings collections: %v", err)
		}
		if err := docRepo.Reorder(ctx, "space-a", ptr("coll-a"), []string{"doc-b", "doc-a"}); err != nil {
			t.Fatalf("Reorder documents: %v", err)
		}

		var space model.DocsSpace
		if err := db.WithContext(ctx).Where("id = ?", "space-a").First(&space).Error; err != nil {
			t.Fatalf("load space: %v", err)
		}
		if !space.UpdatedAt.Equal(now) {
			t.Fatalf("space updated_at = %s, want %s", space.UpdatedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
		}

		var coll model.DocsCollection
		if err := db.WithContext(ctx).Where("id = ?", "coll-a").First(&coll).Error; err != nil {
			t.Fatalf("load collection: %v", err)
		}
		if !coll.UpdatedAt.Equal(now) {
			t.Fatalf("collection updated_at = %s, want %s", coll.UpdatedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
		}

		var doc model.DocsDocument
		if err := db.WithContext(ctx).Where("id = ?", "doc-a").First(&doc).Error; err != nil {
			t.Fatalf("load document: %v", err)
		}
		if !doc.UpdatedAt.Equal(now) {
			t.Fatalf("document updated_at = %s, want %s", doc.UpdatedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
		}
	})

	t.Run("normalization keeps updated_at stable for spaces collections and documents", func(t *testing.T) {
		db := setupDocsUpdatedAtOrderingTestDB(t)
		spaceRepo := NewDocsSpaceRepository(db)
		collectionRepo := NewDocsCollectionRepository(db)
		docRepo := NewDocsDocumentRepository(db)

		seedDocsUpdatedAtSpace(t, db, model.DocsSpace{
			ID:          "space-c",
			WorkspaceID: workspaceID,
			Name:        "Space C",
			Slug:        "space-c",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    3,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsUpdatedAtSpace(t, db, model.DocsSpace{
			ID:          "space-d",
			WorkspaceID: workspaceID,
			Name:        "Space D",
			Slug:        "space-d",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    7,
			CreatedBy:   userID,
			CreatedAt:   now.Add(time.Minute),
			UpdatedAt:   now.Add(time.Minute),
		})
		seedDocsUpdatedAtCollection(t, db, model.DocsCollection{
			ID:          "coll-c",
			SpaceID:     "space-c",
			WorkspaceID: workspaceID,
			Name:        "Collection C",
			Position:    4,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsUpdatedAtCollection(t, db, model.DocsCollection{
			ID:          "coll-d",
			SpaceID:     "space-c",
			WorkspaceID: workspaceID,
			Name:        "Collection D",
			Position:    9,
			CreatedBy:   userID,
			CreatedAt:   now.Add(time.Minute),
			UpdatedAt:   now.Add(time.Minute),
		})
		seedDocsUpdatedAtDocument(t, db, model.DocsDocument{
			ID:           "doc-c",
			WorkspaceID:  workspaceID,
			SpaceID:      "space-c",
			CollectionID: ptr("coll-c"),
			Title:        "Document C",
			Status:       model.DocStatusDraft,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     5,
			CreatedBy:    userID,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		seedDocsUpdatedAtDocument(t, db, model.DocsDocument{
			ID:           "doc-d",
			WorkspaceID:  workspaceID,
			SpaceID:      "space-c",
			CollectionID: ptr("coll-c"),
			Title:        "Document D",
			Status:       model.DocStatusDraft,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     12,
			CreatedBy:    userID,
			CreatedAt:    now.Add(time.Minute),
			UpdatedAt:    now.Add(time.Minute),
		})

		if err := spaceRepo.NormalizeSection(ctx, workspaceID, model.SpaceTypeInternal); err != nil {
			t.Fatalf("Normalize spaces: %v", err)
		}
		if err := collectionRepo.NormalizeSpace(ctx, "space-c"); err != nil {
			t.Fatalf("Normalize collections: %v", err)
		}
		if err := docRepo.NormalizeBucket(ctx, "space-c", ptr("coll-c")); err != nil {
			t.Fatalf("Normalize documents: %v", err)
		}

		var space model.DocsSpace
		if err := db.WithContext(ctx).Where("id = ?", "space-c").First(&space).Error; err != nil {
			t.Fatalf("load normalized space: %v", err)
		}
		if !space.UpdatedAt.Equal(now) {
			t.Fatalf("normalized space updated_at = %s, want %s", space.UpdatedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
		}

		var coll model.DocsCollection
		if err := db.WithContext(ctx).Where("id = ?", "coll-c").First(&coll).Error; err != nil {
			t.Fatalf("load normalized collection: %v", err)
		}
		if !coll.UpdatedAt.Equal(now) {
			t.Fatalf("normalized collection updated_at = %s, want %s", coll.UpdatedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
		}

		var doc model.DocsDocument
		if err := db.WithContext(ctx).Where("id = ?", "doc-c").First(&doc).Error; err != nil {
			t.Fatalf("load normalized document: %v", err)
		}
		if !doc.UpdatedAt.Equal(now) {
			t.Fatalf("normalized document updated_at = %s, want %s", doc.UpdatedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
		}
	})
}
