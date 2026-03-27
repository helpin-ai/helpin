package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func setupDocsOrderingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-ordering-%d?mode=memory&cache=shared", time.Now().UnixNano())
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
		`CREATE TABLE docs_space_teams (
			space_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (space_id, team_id)
		)`,
		`CREATE TABLE docs_collections (
			id TEXT PRIMARY KEY,
			space_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
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

func seedDocsSpace(t *testing.T, db *gorm.DB, space model.DocsSpace) {
	t.Helper()
	if err := db.Create(&space).Error; err != nil {
		t.Fatalf("seed docs space %s: %v", space.ID, err)
	}
}

func seedDocsCollection(t *testing.T, db *gorm.DB, coll model.DocsCollection) {
	t.Helper()
	if err := db.Create(&coll).Error; err != nil {
		t.Fatalf("seed docs collection %s: %v", coll.ID, err)
	}
}

func seedDocsOrderingDocument(t *testing.T, db *gorm.DB, doc model.DocsDocument) {
	t.Helper()
	if err := db.Create(&doc).Error; err != nil {
		t.Fatalf("seed docs document %s: %v", doc.ID, err)
	}
}

func TestDocsOrdering_MoveDeleteAndTypeChange(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-docs"
		userID      = "user-docs"
	)

	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)

	t.Run("Move appends to target bucket and normalizes source bucket", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		spaceRepo := repository.NewDocsSpaceRepository(db)
		docRepo := repository.NewDocsDocumentRepository(db)
		docSvc := NewDocsDocumentService(docRepo, spaceRepo)
		ctx := context.Background()

		seedDocsSpace(t, db, model.DocsSpace{
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
		seedDocsSpace(t, db, model.DocsSpace{
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

		seedDocsOrderingDocument(t, db, model.DocsDocument{
			ID:          "doc-move",
			WorkspaceID: workspaceID,
			SpaceID:     "space-a",
			Title:       "Move me",
			Status:      model.DocStatusDraft,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsOrderingDocument(t, db, model.DocsDocument{
			ID:          "doc-source-2",
			WorkspaceID: workspaceID,
			SpaceID:     "space-a",
			Title:       "Stay here",
			Status:      model.DocStatusDraft,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Position:    1,
			CreatedBy:   userID,
			CreatedAt:   now.Add(time.Minute),
			UpdatedAt:   now.Add(time.Minute),
		})
		seedDocsOrderingDocument(t, db, model.DocsDocument{
			ID:          "doc-target-1",
			WorkspaceID: workspaceID,
			SpaceID:     "space-b",
			Title:       "Already there",
			Status:      model.DocStatusDraft,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now.Add(2 * time.Minute),
			UpdatedAt:   now.Add(2 * time.Minute),
		})

		if _, err := docSvc.Move(ctx, "doc-move", model.MoveDocsDocumentRequest{SpaceID: "space-b"}); err != nil {
			t.Fatalf("Move: %v", err)
		}

		var moved model.DocsDocument
		if err := db.WithContext(ctx).Where("id = ?", "doc-move").First(&moved).Error; err != nil {
			t.Fatalf("load moved doc: %v", err)
		}
		if moved.SpaceID != "space-b" {
			t.Fatalf("moved doc space_id = %q, want %q", moved.SpaceID, "space-b")
		}
		if moved.Position != 1 {
			t.Fatalf("moved doc position = %d, want 1", moved.Position)
		}

		var sourceDocs []model.DocsDocument
		if err := db.WithContext(ctx).
			Where("space_id = ? AND deleted_at IS NULL", "space-a").
			Order("position ASC").
			Find(&sourceDocs).Error; err != nil {
			t.Fatalf("load source docs: %v", err)
		}
		if len(sourceDocs) != 1 || sourceDocs[0].ID != "doc-source-2" || sourceDocs[0].Position != 0 {
			t.Fatalf("source docs after move = %+v, want only doc-source-2 at position 0", sourceDocs)
		}
	})

	t.Run("Collection create derives slug when request slug is omitted", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		spaceRepo := repository.NewDocsSpaceRepository(db)
		collectionRepo := repository.NewDocsCollectionRepository(db)
		collectionSvc := NewDocsCollectionService(collectionRepo, spaceRepo)
		ctx := context.Background()

		seedDocsSpace(t, db, model.DocsSpace{
			ID:          "space-collections",
			WorkspaceID: workspaceID,
			Name:        "Knowledge Base",
			Slug:        "knowledge-base",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		created, err := collectionSvc.Create(ctx, workspaceID, "space-collections", model.CreateDocsCollectionRequest{
			Name: "Getting Started",
		}, userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		if created.Slug != "getting-started" {
			t.Fatalf("created collection slug = %q, want %q", created.Slug, "getting-started")
		}
	})

	t.Run("Delete appends docs into uncategorized and normalizes remaining collections", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		repo := repository.NewDocsCollectionRepository(db)
		ctx := context.Background()

		seedDocsCollection(t, db, model.DocsCollection{
			ID:          "coll-a",
			SpaceID:     "space-a",
			WorkspaceID: workspaceID,
			Name:        "Collection A",
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsCollection(t, db, model.DocsCollection{
			ID:          "coll-b",
			SpaceID:     "space-a",
			WorkspaceID: workspaceID,
			Name:        "Collection B",
			Position:    1,
			CreatedBy:   userID,
			CreatedAt:   now.Add(time.Minute),
			UpdatedAt:   now.Add(time.Minute),
		})

		seedDocsOrderingDocument(t, db, model.DocsDocument{
			ID:          "doc-uncat",
			WorkspaceID: workspaceID,
			SpaceID:     "space-a",
			Title:       "Uncategorized",
			Status:      model.DocStatusDraft,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		collAID := "coll-a"
		seedDocsOrderingDocument(t, db, model.DocsDocument{
			ID:           "doc-a-1",
			WorkspaceID:  workspaceID,
			SpaceID:      "space-a",
			CollectionID: &collAID,
			Title:        "Collection doc 1",
			Status:       model.DocStatusDraft,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     0,
			CreatedBy:    userID,
			CreatedAt:    now.Add(time.Minute),
			UpdatedAt:    now.Add(time.Minute),
		})
		seedDocsOrderingDocument(t, db, model.DocsDocument{
			ID:           "doc-a-2",
			WorkspaceID:  workspaceID,
			SpaceID:      "space-a",
			CollectionID: &collAID,
			Title:        "Collection doc 2",
			Status:       model.DocStatusDraft,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     1,
			CreatedBy:    userID,
			CreatedAt:    now.Add(2 * time.Minute),
			UpdatedAt:    now.Add(2 * time.Minute),
		})

		if err := repo.Delete(ctx, "coll-a"); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		var uncatDocs []model.DocsDocument
		if err := db.WithContext(ctx).
			Where("space_id = ? AND collection_id IS NULL AND deleted_at IS NULL", "space-a").
			Order("position ASC").
			Find(&uncatDocs).Error; err != nil {
			t.Fatalf("load uncategorized docs: %v", err)
		}

		gotIDs := []string{uncatDocs[0].ID, uncatDocs[1].ID, uncatDocs[2].ID}
		wantIDs := []string{"doc-uncat", "doc-a-1", "doc-a-2"}
		for i := range wantIDs {
			if gotIDs[i] != wantIDs[i] {
				t.Fatalf("uncategorized docs[%d] = %q, want %q (full order %v)", i, gotIDs[i], wantIDs[i], gotIDs)
			}
			if uncatDocs[i].Position != i {
				t.Fatalf("uncategorized docs[%d] position = %d, want %d", i, uncatDocs[i].Position, i)
			}
		}

		var remaining model.DocsCollection
		if err := db.WithContext(ctx).Where("id = ?", "coll-b").First(&remaining).Error; err != nil {
			t.Fatalf("load remaining collection: %v", err)
		}
		if remaining.Position != 0 {
			t.Fatalf("remaining collection position = %d, want 0", remaining.Position)
		}
	})

	t.Run("Update type appends to target section and normalizes old section", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		repo := repository.NewDocsSpaceRepository(db)
		svc := NewDocsSpaceService(repo)
		ctx := context.Background()

		seedDocsSpace(t, db, model.DocsSpace{
			ID:          "space-internal-a",
			WorkspaceID: workspaceID,
			Name:        "Internal A",
			Slug:        "internal-a",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsSpace(t, db, model.DocsSpace{
			ID:          "space-internal-b",
			WorkspaceID: workspaceID,
			Name:        "Internal B",
			Slug:        "internal-b",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    1,
			CreatedBy:   userID,
			CreatedAt:   now.Add(time.Minute),
			UpdatedAt:   now.Add(time.Minute),
		})
		seedDocsSpace(t, db, model.DocsSpace{
			ID:          "space-external-a",
			WorkspaceID: workspaceID,
			Name:        "External A",
			Slug:        "external-a",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now.Add(2 * time.Minute),
			UpdatedAt:   now.Add(2 * time.Minute),
		})

		targetType := model.SpaceTypeExternalCapable
		if _, err := svc.Update(ctx, "space-internal-a", model.UpdateDocsSpaceRequest{Type: &targetType}); err != nil {
			t.Fatalf("Update: %v", err)
		}

		var moved model.DocsSpace
		if err := db.WithContext(ctx).Where("id = ?", "space-internal-a").First(&moved).Error; err != nil {
			t.Fatalf("load moved space: %v", err)
		}
		if moved.Type != model.SpaceTypeExternalCapable {
			t.Fatalf("moved space type = %q, want %q", moved.Type, model.SpaceTypeExternalCapable)
		}
		if moved.Position != 1 {
			t.Fatalf("moved space position = %d, want 1", moved.Position)
		}

		var remaining model.DocsSpace
		if err := db.WithContext(ctx).Where("id = ?", "space-internal-b").First(&remaining).Error; err != nil {
			t.Fatalf("load remaining internal space: %v", err)
		}
		if remaining.Position != 0 {
			t.Fatalf("remaining internal space position = %d, want 0", remaining.Position)
		}
	})
}
