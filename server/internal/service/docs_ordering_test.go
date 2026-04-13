package service

import (
	"context"
	"errors"
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
		docSvc := NewDocsDocumentService(docRepo, spaceRepo, nil)
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
		collectionSvc := NewDocsCollectionService(collectionRepo, spaceRepo, nil)
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
		if len(created.PublicID) != docsHelpcenterPublicIDLength {
			t.Fatalf("created collection public_id = %q, want %d chars", created.PublicID, docsHelpcenterPublicIDLength)
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
		svc := NewDocsSpaceService(repo, nil)
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

// TestDocsCollection_TreeSchemaFields verifies that the new parent_collection_id
// and depth fields on DocsCollection round-trip through the DB correctly at the
// model layer. This is the minimum bar for Task 1: the schema is in place and
// existing flows still work, without introducing any repository or service
// changes (those come in later tasks).
func TestDocsCollection_TreeSchemaFields(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-tree"
		userID      = "user-tree"
	)

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)

	t.Run("existing collection loads with nil parent and zero depth", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		ctx := context.Background()

		seedDocsSpace(t, db, model.DocsSpace{
			ID:          "space-tree",
			WorkspaceID: workspaceID,
			Name:        "Tree space",
			Slug:        "tree-space",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsCollection(t, db, model.DocsCollection{
			ID:          "coll-root",
			SpaceID:     "space-tree",
			WorkspaceID: workspaceID,
			Name:        "Root",
			Slug:        "root",
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		var loaded model.DocsCollection
		if err := db.WithContext(ctx).Where("id = ?", "coll-root").First(&loaded).Error; err != nil {
			t.Fatalf("load collection: %v", err)
		}
		if loaded.ParentCollectionID != nil {
			t.Fatalf("ParentCollectionID = %v, want nil", loaded.ParentCollectionID)
		}
		if loaded.Depth != 0 {
			t.Fatalf("Depth = %d, want 0", loaded.Depth)
		}
	})

	t.Run("child collection persists parent_collection_id and depth", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		ctx := context.Background()

		seedDocsSpace(t, db, model.DocsSpace{
			ID:          "space-tree",
			WorkspaceID: workspaceID,
			Name:        "Tree space",
			Slug:        "tree-space",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsCollection(t, db, model.DocsCollection{
			ID:          "coll-parent",
			SpaceID:     "space-tree",
			WorkspaceID: workspaceID,
			Name:        "Parent",
			Slug:        "parent",
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})

		parentID := "coll-parent"
		seedDocsCollection(t, db, model.DocsCollection{
			ID:                 "coll-child",
			SpaceID:            "space-tree",
			WorkspaceID:        workspaceID,
			ParentCollectionID: &parentID,
			Depth:              1,
			Name:               "Child",
			Slug:               "child",
			Position:           0,
			CreatedBy:          userID,
			CreatedAt:          now.Add(time.Minute),
			UpdatedAt:          now.Add(time.Minute),
		})

		var loaded model.DocsCollection
		if err := db.WithContext(ctx).Where("id = ?", "coll-child").First(&loaded).Error; err != nil {
			t.Fatalf("load child collection: %v", err)
		}
		if loaded.ParentCollectionID == nil {
			t.Fatalf("ParentCollectionID = nil, want %q", parentID)
		}
		if *loaded.ParentCollectionID != parentID {
			t.Fatalf("ParentCollectionID = %q, want %q", *loaded.ParentCollectionID, parentID)
		}
		if loaded.Depth != 1 {
			t.Fatalf("Depth = %d, want 1", loaded.Depth)
		}
	})

	t.Run("create and update request DTOs carry parent_collection_id", func(t *testing.T) {
		// Pure struct-level check — no DB needed. Guards against accidental
		// removal of the field from the request DTOs in a later refactor.
		empty := ""
		createReq := model.CreateDocsCollectionRequest{ParentCollectionID: &empty}
		if createReq.ParentCollectionID == nil {
			t.Fatalf("CreateDocsCollectionRequest.ParentCollectionID is not wired")
		}

		updateReq := model.UpdateDocsCollectionRequest{ParentCollectionID: &empty}
		if updateReq.ParentCollectionID == nil {
			t.Fatalf("UpdateDocsCollectionRequest.ParentCollectionID is not wired")
		}
	})
}

// seedTreeCollection is a small convenience helper that builds a typical
// DocsCollection seed row with sane defaults so test bodies stay compact.
func seedTreeCollection(t *testing.T, db *gorm.DB, id, spaceID, workspaceID string, parentID *string, depth, position int, name string) {
	t.Helper()
	slug := name
	seedDocsCollection(t, db, model.DocsCollection{
		ID:                 id,
		SpaceID:            spaceID,
		WorkspaceID:        workspaceID,
		ParentCollectionID: parentID,
		Depth:              depth,
		Name:               name,
		Slug:               slug,
		Position:           position,
		CreatedBy:          "user-tree",
		CreatedAt:          time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC).Add(time.Duration(position) * time.Minute),
		UpdatedAt:          time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC).Add(time.Duration(position) * time.Minute),
	})
}

// TestDocsCollectionRepo_Tree exercises the tree-aware repository helpers
// added in Task 2: ListChildren, ListAncestors, ListDescendants,
// NextPositionInBucket, ReorderSiblings, and Reparent. Each subtest starts
// from a fresh in-memory DB so state cannot leak across cases.
func TestDocsCollectionRepo_Tree(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-tree"
		spaceID     = "space-tree"
	)

	seedSpace := func(t *testing.T, db *gorm.DB) {
		t.Helper()
		seedDocsSpace(t, db, model.DocsSpace{
			ID:          spaceID,
			WorkspaceID: workspaceID,
			Name:        "Tree space",
			Slug:        "tree-space",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    0,
			CreatedBy:   "user-tree",
			CreatedAt:   time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		})
	}

	t.Run("ListChildren returns only the requested bucket in position order", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		seedSpace(t, db)
		ctx := context.Background()
		repo := repository.NewDocsCollectionRepository(db)

		// Top-level: A (0), B (1). Nested under A: A1 (0), A2 (1).
		seedTreeCollection(t, db, "A", spaceID, workspaceID, nil, 0, 0, "A")
		seedTreeCollection(t, db, "B", spaceID, workspaceID, nil, 0, 1, "B")
		parentA := "A"
		seedTreeCollection(t, db, "A1", spaceID, workspaceID, &parentA, 1, 0, "A1")
		seedTreeCollection(t, db, "A2", spaceID, workspaceID, &parentA, 1, 1, "A2")

		topLevel, err := repo.ListChildren(ctx, spaceID, nil)
		if err != nil {
			t.Fatalf("ListChildren(nil): %v", err)
		}
		if got, want := ids(topLevel), []string{"A", "B"}; !equalIDs(got, want) {
			t.Fatalf("top-level ids = %v, want %v", got, want)
		}

		underA, err := repo.ListChildren(ctx, spaceID, &parentA)
		if err != nil {
			t.Fatalf("ListChildren(A): %v", err)
		}
		if got, want := ids(underA), []string{"A1", "A2"}; !equalIDs(got, want) {
			t.Fatalf("under-A ids = %v, want %v", got, want)
		}
	})

	t.Run("ListAncestors walks up a three-level chain in order", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		seedSpace(t, db)
		ctx := context.Background()
		repo := repository.NewDocsCollectionRepository(db)

		root := "root"
		child := "child"
		grand := "grand"
		seedTreeCollection(t, db, root, spaceID, workspaceID, nil, 0, 0, "root")
		rootID := root
		seedTreeCollection(t, db, child, spaceID, workspaceID, &rootID, 1, 0, "child")
		childID := child
		seedTreeCollection(t, db, grand, spaceID, workspaceID, &childID, 2, 0, "grand")

		got, err := repo.ListAncestors(ctx, grand)
		if err != nil {
			t.Fatalf("ListAncestors(grand): %v", err)
		}
		// Expected order: immediate parent first, then grandparent.
		if want := []string{"child", "root"}; !equalIDs(ids(got), want) {
			t.Fatalf("ancestors = %v, want %v", ids(got), want)
		}
	})

	t.Run("ListAncestors returns empty for a top-level collection", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		seedSpace(t, db)
		ctx := context.Background()
		repo := repository.NewDocsCollectionRepository(db)

		seedTreeCollection(t, db, "root", spaceID, workspaceID, nil, 0, 0, "root")

		got, err := repo.ListAncestors(ctx, "root")
		if err != nil {
			t.Fatalf("ListAncestors: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("ancestors = %v, want empty", ids(got))
		}
	})

	t.Run("ListDescendants walks two levels BFS", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		seedSpace(t, db)
		ctx := context.Background()
		repo := repository.NewDocsCollectionRepository(db)

		seedTreeCollection(t, db, "root", spaceID, workspaceID, nil, 0, 0, "root")
		rootID := "root"
		seedTreeCollection(t, db, "c1", spaceID, workspaceID, &rootID, 1, 0, "c1")
		seedTreeCollection(t, db, "c2", spaceID, workspaceID, &rootID, 1, 1, "c2")
		c1 := "c1"
		seedTreeCollection(t, db, "c1a", spaceID, workspaceID, &c1, 2, 0, "c1a")
		c2 := "c2"
		seedTreeCollection(t, db, "c2a", spaceID, workspaceID, &c2, 2, 0, "c2a")

		got, err := repo.ListDescendants(ctx, "root")
		if err != nil {
			t.Fatalf("ListDescendants: %v", err)
		}
		// Level 1 before level 2; within a level, ordered by position.
		if want := []string{"c1", "c2", "c1a", "c2a"}; !equalIDs(ids(got), want) {
			t.Fatalf("descendants = %v, want %v", ids(got), want)
		}
	})

	t.Run("NextPositionInBucket scopes to the requested parent", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		seedSpace(t, db)
		ctx := context.Background()
		repo := repository.NewDocsCollectionRepository(db)

		seedTreeCollection(t, db, "A", spaceID, workspaceID, nil, 0, 0, "A")
		seedTreeCollection(t, db, "B", spaceID, workspaceID, nil, 0, 1, "B")
		a := "A"
		seedTreeCollection(t, db, "A1", spaceID, workspaceID, &a, 1, 0, "A1")

		top, err := repo.NextPositionInBucket(ctx, spaceID, nil)
		if err != nil {
			t.Fatalf("NextPositionInBucket(nil): %v", err)
		}
		if top != 2 {
			t.Fatalf("top next position = %d, want 2", top)
		}

		underA, err := repo.NextPositionInBucket(ctx, spaceID, &a)
		if err != nil {
			t.Fatalf("NextPositionInBucket(A): %v", err)
		}
		if underA != 1 {
			t.Fatalf("under-A next position = %d, want 1", underA)
		}
	})

	t.Run("ReorderSiblings only touches the targeted bucket", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		seedSpace(t, db)
		ctx := context.Background()
		repo := repository.NewDocsCollectionRepository(db)

		// Three top-level collections and two nested under the second one.
		seedTreeCollection(t, db, "A", spaceID, workspaceID, nil, 0, 0, "A")
		seedTreeCollection(t, db, "B", spaceID, workspaceID, nil, 0, 1, "B")
		seedTreeCollection(t, db, "C", spaceID, workspaceID, nil, 0, 2, "C")
		b := "B"
		seedTreeCollection(t, db, "B1", spaceID, workspaceID, &b, 1, 0, "B1")
		seedTreeCollection(t, db, "B2", spaceID, workspaceID, &b, 1, 1, "B2")

		// Reorder the top-level bucket: C, A, B.
		if err := repo.ReorderSiblings(ctx, spaceID, nil, []string{"C", "A", "B"}); err != nil {
			t.Fatalf("ReorderSiblings(nil): %v", err)
		}

		topLevel, err := repo.ListChildren(ctx, spaceID, nil)
		if err != nil {
			t.Fatalf("ListChildren(nil): %v", err)
		}
		if got, want := ids(topLevel), []string{"C", "A", "B"}; !equalIDs(got, want) {
			t.Fatalf("top-level after reorder = %v, want %v", got, want)
		}

		// Nested bucket positions must be untouched.
		underB, err := repo.ListChildren(ctx, spaceID, &b)
		if err != nil {
			t.Fatalf("ListChildren(B): %v", err)
		}
		if got, want := ids(underB), []string{"B1", "B2"}; !equalIDs(got, want) {
			t.Fatalf("under-B after reorder = %v, want %v", got, want)
		}
	})

	t.Run("Reparent recalculates descendant depths and normalizes both buckets", func(t *testing.T) {
		db := setupDocsOrderingTestDB(t)
		seedSpace(t, db)
		ctx := context.Background()
		repo := repository.NewDocsCollectionRepository(db)

		// Two top-level roots; "mover" lives under rootA and carries one child.
		seedTreeCollection(t, db, "rootA", spaceID, workspaceID, nil, 0, 0, "rootA")
		seedTreeCollection(t, db, "rootB", spaceID, workspaceID, nil, 0, 1, "rootB")
		rootA := "rootA"
		seedTreeCollection(t, db, "mover", spaceID, workspaceID, &rootA, 1, 0, "mover")
		seedTreeCollection(t, db, "sibling", spaceID, workspaceID, &rootA, 1, 1, "sibling")
		mover := "mover"
		seedTreeCollection(t, db, "moverChild", spaceID, workspaceID, &mover, 2, 0, "moverChild")

		// Reparent mover from under rootA to the top of the space.
		if err := repo.Reparent(ctx, "mover", nil); err != nil {
			t.Fatalf("Reparent(mover -> top): %v", err)
		}

		// mover should now be at depth 0 with parent_collection_id NULL.
		var moved model.DocsCollection
		if err := db.WithContext(ctx).Where("id = ?", "mover").First(&moved).Error; err != nil {
			t.Fatalf("load mover: %v", err)
		}
		if moved.ParentCollectionID != nil {
			t.Fatalf("mover.ParentCollectionID = %v, want nil", moved.ParentCollectionID)
		}
		if moved.Depth != 0 {
			t.Fatalf("mover.Depth = %d, want 0", moved.Depth)
		}

		// moverChild should be depth 1 now (was depth 2).
		var movedChild model.DocsCollection
		if err := db.WithContext(ctx).Where("id = ?", "moverChild").First(&movedChild).Error; err != nil {
			t.Fatalf("load moverChild: %v", err)
		}
		if movedChild.Depth != 1 {
			t.Fatalf("moverChild.Depth = %d, want 1", movedChild.Depth)
		}
		if movedChild.ParentCollectionID == nil || *movedChild.ParentCollectionID != "mover" {
			t.Fatalf("moverChild.ParentCollectionID = %v, want mover", movedChild.ParentCollectionID)
		}

		// Old bucket (under rootA) must be normalized: sibling at position 0.
		underA, err := repo.ListChildren(ctx, spaceID, &rootA)
		if err != nil {
			t.Fatalf("ListChildren(rootA): %v", err)
		}
		if len(underA) != 1 || underA[0].ID != "sibling" || underA[0].Position != 0 {
			t.Fatalf("under-A after reparent = %+v, want [sibling@0]", underA)
		}

		// New bucket (top-level) must contain rootA, rootB, mover — positions 0,1,2.
		topLevel, err := repo.ListChildren(ctx, spaceID, nil)
		if err != nil {
			t.Fatalf("ListChildren(nil): %v", err)
		}
		if got, want := ids(topLevel), []string{"rootA", "rootB", "mover"}; !equalIDs(got, want) {
			t.Fatalf("top-level after reparent = %v, want %v", got, want)
		}
		for i, c := range topLevel {
			if c.Position != i {
				t.Fatalf("top-level[%d].Position = %d, want %d", i, c.Position, i)
			}
		}
	})
}

// TestDocsCollectionService_TreeValidation exercises the Task 3 validation
// rules for Create and Update in the service layer. It uses in-memory
// SQLite through the real repository so the tree helpers added in Task 2
// are exercised end-to-end from the service's perspective.
func TestDocsCollectionService_TreeValidation(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-val"
		userID      = "user-val"
	)

	setup := func(t *testing.T) (*DocsCollectionService, *gorm.DB) {
		t.Helper()
		db := setupDocsOrderingTestDB(t)
		collectionRepo := repository.NewDocsCollectionRepository(db)
		spaceRepo := repository.NewDocsSpaceRepository(db)
		svc := NewDocsCollectionService(collectionRepo, spaceRepo, nil)
		return svc, db
	}

	seedSpace := func(t *testing.T, db *gorm.DB, id string) {
		t.Helper()
		seedDocsSpace(t, db, model.DocsSpace{
			ID:          id,
			WorkspaceID: workspaceID,
			Name:        id,
			Slug:        id,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		})
	}

	t.Run("Create top-level collection sets depth 0 and nil parent", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space")
		ctx := context.Background()

		created, err := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "Top"}, userID)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if created.Depth != 0 {
			t.Fatalf("depth = %d, want 0", created.Depth)
		}
		if created.ParentCollectionID != nil {
			t.Fatalf("ParentCollectionID = %v, want nil", created.ParentCollectionID)
		}
	})

	t.Run("Create nested collection inherits parent's depth + 1", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space")
		ctx := context.Background()

		parent, err := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "Parent"}, userID)
		if err != nil {
			t.Fatalf("Create parent: %v", err)
		}
		parentID := parent.ID
		child, err := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{
			Name:               "Child",
			ParentCollectionID: &parentID,
		}, userID)
		if err != nil {
			t.Fatalf("Create child: %v", err)
		}
		if child.Depth != 1 {
			t.Fatalf("child depth = %d, want 1", child.Depth)
		}
		if child.ParentCollectionID == nil || *child.ParentCollectionID != parentID {
			t.Fatalf("child parent = %v, want %q", child.ParentCollectionID, parentID)
		}
	})

	t.Run("Create rejects a parent that lives in a different space", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space-a")
		seedSpace(t, db, "space-b")
		ctx := context.Background()

		parent, err := svc.Create(ctx, workspaceID, "space-a", model.CreateDocsCollectionRequest{Name: "P"}, userID)
		if err != nil {
			t.Fatalf("Create parent: %v", err)
		}
		parentID := parent.ID

		_, err = svc.Create(ctx, workspaceID, "space-b", model.CreateDocsCollectionRequest{
			Name:               "X",
			ParentCollectionID: &parentID,
		}, userID)
		if err == nil {
			t.Fatalf("expected cross-space parent error, got nil")
		}
	})

	t.Run("Create rejects a missing parent", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space")
		ctx := context.Background()

		ghost := "does-not-exist"
		_, err := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{
			Name:               "X",
			ParentCollectionID: &ghost,
		}, userID)
		if err == nil {
			t.Fatalf("expected missing parent error, got nil")
		}
	})

	t.Run("Create rejects a third level (depth 2)", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space")
		ctx := context.Background()

		root, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "root"}, userID)
		rootID := root.ID
		lvl1, err := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "l1", ParentCollectionID: &rootID}, userID)
		if err != nil {
			t.Fatalf("Create lvl1: %v", err)
		}
		lvl1ID := lvl1.ID

		_, err = svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "l2", ParentCollectionID: &lvl1ID}, userID)
		if err == nil {
			t.Fatalf("expected depth-exceeded error on 3rd level, got nil")
		}
	})

	t.Run("Update reparent under a valid sibling works", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space")
		ctx := context.Background()

		a, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "A"}, userID)
		b, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "B"}, userID)

		aID := a.ID
		moved, err := svc.Update(ctx, b.ID, model.UpdateDocsCollectionRequest{
			ParentCollectionID: &aID,
		})
		if err != nil {
			t.Fatalf("Update reparent: %v", err)
		}
		if moved.ParentCollectionID == nil || *moved.ParentCollectionID != aID {
			t.Fatalf("moved.ParentCollectionID = %v, want %q", moved.ParentCollectionID, aID)
		}
		if moved.Depth != 1 {
			t.Fatalf("moved.Depth = %d, want 1", moved.Depth)
		}
	})

	t.Run("Update rejects self-parent", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space")
		ctx := context.Background()

		a, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "A"}, userID)
		aID := a.ID

		_, err := svc.Update(ctx, a.ID, model.UpdateDocsCollectionRequest{
			ParentCollectionID: &aID,
		})
		if err == nil {
			t.Fatalf("expected self-parent error, got nil")
		}
	})

	t.Run("Update rejects a descendant as the new parent", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space")
		ctx := context.Background()

		root, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "root"}, userID)
		rootID := root.ID
		child, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "child", ParentCollectionID: &rootID}, userID)
		childID := child.ID

		_, err := svc.Update(ctx, root.ID, model.UpdateDocsCollectionRequest{
			ParentCollectionID: &childID,
		})
		if err == nil {
			t.Fatalf("expected descendant-parent cycle error, got nil")
		}
	})

	t.Run("Update rejects a reparent that would push a descendant past max depth", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space")
		ctx := context.Background()

		// Build: root -> mid -> leaf. Also a separate depth-1 other.
		root, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "root"}, userID)
		rootID := root.ID
		mid, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "mid", ParentCollectionID: &rootID}, userID)
		midID := mid.ID
		_, _ = svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "leaf", ParentCollectionID: &midID}, userID)

		other, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "other"}, userID)
		otherID := other.ID
		otherChild, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "otherChild", ParentCollectionID: &otherID}, userID)
		otherChildID := otherChild.ID

		// Moving "mid" (which has "leaf" at depth 2) under "otherChild"
		// (depth 1) would place mid at depth 2 and leaf at depth 3 → reject.
		_, err := svc.Update(ctx, mid.ID, model.UpdateDocsCollectionRequest{
			ParentCollectionID: &otherChildID,
		})
		if err == nil {
			t.Fatalf("expected depth-overflow error on subtree move, got nil")
		}
	})

	t.Run("Create rejects a duplicate slug in the same workspace", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space")
		ctx := context.Background()

		// First create with an explicit slug succeeds.
		slug := "shared"
		if _, err := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{
			Name: "First",
			Slug: &slug,
		}, userID); err != nil {
			t.Fatalf("first create: %v", err)
		}

		// Second create that slugifies to the same value fails with the
		// typed sentinel so the handler maps it to 409.
		_, err := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{
			Name: "shared",
		}, userID)
		if err == nil {
			t.Fatalf("expected duplicate slug error, got nil")
		}
		if !errors.Is(err, ErrDocsCollectionSlugTaken) {
			t.Fatalf("err = %v, want ErrDocsCollectionSlugTaken", err)
		}
	})

	t.Run("Update with unchanged parent does not reshuffle siblings", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space")
		ctx := context.Background()

		// Create two top-level siblings in order a, b.
		a, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "a"}, userID)
		b, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "b"}, userID)

		// Echo the current (nil) parent back via the empty-string
		// sentinel on a simple rename. Before the fix, Reparent would
		// append `a` to the end of its own bucket and shuffle b ahead
		// of it.
		empty := ""
		newName := "Alpha"
		if _, err := svc.Update(ctx, a.ID, model.UpdateDocsCollectionRequest{
			Name:               &newName,
			ParentCollectionID: &empty,
		}); err != nil {
			t.Fatalf("Update: %v", err)
		}

		// Order should remain [a, b] with contiguous positions.
		collectionRepo := repository.NewDocsCollectionRepository(db)
		topLevel, err := collectionRepo.ListChildren(ctx, "space", nil)
		if err != nil {
			t.Fatalf("ListChildren: %v", err)
		}
		if len(topLevel) != 2 || topLevel[0].ID != a.ID || topLevel[1].ID != b.ID {
			t.Fatalf("top-level after no-op reparent = %+v, want [a, b]", topLevel)
		}
		if topLevel[0].Position != 0 || topLevel[1].Position != 1 {
			t.Fatalf("positions = %d,%d, want 0,1", topLevel[0].Position, topLevel[1].Position)
		}
	})

	t.Run("Update reparent to top level uses empty-string sentinel", func(t *testing.T) {
		svc, db := setup(t)
		seedSpace(t, db, "space")
		ctx := context.Background()

		parent, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "parent"}, userID)
		parentID := parent.ID
		child, _ := svc.Create(ctx, workspaceID, "space", model.CreateDocsCollectionRequest{Name: "child", ParentCollectionID: &parentID}, userID)

		empty := ""
		moved, err := svc.Update(ctx, child.ID, model.UpdateDocsCollectionRequest{
			ParentCollectionID: &empty,
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if moved.ParentCollectionID != nil {
			t.Fatalf("moved.ParentCollectionID = %v, want nil", moved.ParentCollectionID)
		}
		if moved.Depth != 0 {
			t.Fatalf("moved.Depth = %d, want 0", moved.Depth)
		}
	})
}

// TestDocsDocumentMoves_NestedCollections verifies that the existing
// document move / reorder pipeline works for documents whose owning
// collection is nested below the top of the space. The plan's Task 4
// keeps `collection_id` as the single owning bucket — nothing about the
// move semantics should change based on the collection's depth.
//
// These tests serve as regression coverage: if a future refactor breaks
// nested-collection bucketing for documents, these fail loudly.
func TestDocsDocumentMoves_NestedCollections(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-doc-tree"
		userID      = "user-doc-tree"
		spaceID     = "space-doc-tree"
	)

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)

	// Common setup: a space with a top-level collection and two nested
	// children ("nest-a", "nest-b") under it.
	setup := func(t *testing.T) (*DocsDocumentService, *gorm.DB) {
		t.Helper()
		db := setupDocsOrderingTestDB(t)
		spaceRepo := repository.NewDocsSpaceRepository(db)
		docRepo := repository.NewDocsDocumentRepository(db)
		svc := NewDocsDocumentService(docRepo, spaceRepo, nil)

		seedDocsSpace(t, db, model.DocsSpace{
			ID:          spaceID,
			WorkspaceID: workspaceID,
			Name:        "Doc tree space",
			Slug:        "doc-tree-space",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsCollection(t, db, model.DocsCollection{
			ID: "parent", SpaceID: spaceID, WorkspaceID: workspaceID,
			Name: "Parent", Slug: "parent", Position: 0,
			CreatedBy: userID, CreatedAt: now, UpdatedAt: now,
		})
		parentID := "parent"
		seedDocsCollection(t, db, model.DocsCollection{
			ID: "nest-a", SpaceID: spaceID, WorkspaceID: workspaceID,
			ParentCollectionID: &parentID, Depth: 1,
			Name: "Nest A", Slug: "nest-a", Position: 0,
			CreatedBy: userID, CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute),
		})
		seedDocsCollection(t, db, model.DocsCollection{
			ID: "nest-b", SpaceID: spaceID, WorkspaceID: workspaceID,
			ParentCollectionID: &parentID, Depth: 1,
			Name: "Nest B", Slug: "nest-b", Position: 1,
			CreatedBy: userID, CreatedAt: now.Add(2 * time.Minute), UpdatedAt: now.Add(2 * time.Minute),
		})
		return svc, db
	}

	seedDoc := func(t *testing.T, db *gorm.DB, id, collectionID string, pos int) {
		t.Helper()
		coll := collectionID
		seedDocsOrderingDocument(t, db, model.DocsDocument{
			ID:           id,
			WorkspaceID:  workspaceID,
			SpaceID:      spaceID,
			CollectionID: &coll,
			Title:        id,
			Status:       model.DocStatusDraft,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     pos,
			CreatedBy:    userID,
			CreatedAt:    now.Add(time.Duration(pos) * time.Minute),
			UpdatedAt:    now.Add(time.Duration(pos) * time.Minute),
		})
	}

	t.Run("Move document between nested collections", func(t *testing.T) {
		svc, db := setup(t)
		ctx := context.Background()

		seedDoc(t, db, "doc-1", "nest-a", 0)
		seedDoc(t, db, "doc-2", "nest-a", 1)
		seedDoc(t, db, "doc-3", "nest-b", 0)

		targetColl := "nest-b"
		if _, err := svc.Move(ctx, "doc-1", model.MoveDocsDocumentRequest{
			SpaceID:      spaceID,
			CollectionID: &targetColl,
		}); err != nil {
			t.Fatalf("Move: %v", err)
		}

		// doc-1 should now be in nest-b at position 1 (appended).
		var moved model.DocsDocument
		if err := db.WithContext(ctx).Where("id = ?", "doc-1").First(&moved).Error; err != nil {
			t.Fatalf("load moved doc: %v", err)
		}
		if moved.CollectionID == nil || *moved.CollectionID != "nest-b" {
			t.Fatalf("moved.CollectionID = %v, want nest-b", moved.CollectionID)
		}
		if moved.Position != 1 {
			t.Fatalf("moved.Position = %d, want 1", moved.Position)
		}

		// Source bucket (nest-a) should contain only doc-2 at position 0.
		var sourceDocs []model.DocsDocument
		if err := db.WithContext(ctx).
			Where("space_id = ? AND collection_id = ? AND deleted_at IS NULL", spaceID, "nest-a").
			Order("position ASC").
			Find(&sourceDocs).Error; err != nil {
			t.Fatalf("load source: %v", err)
		}
		if len(sourceDocs) != 1 || sourceDocs[0].ID != "doc-2" || sourceDocs[0].Position != 0 {
			t.Fatalf("source after move = %+v, want [doc-2@0]", sourceDocs)
		}
	})

	t.Run("Reorder within a nested collection", func(t *testing.T) {
		svc, db := setup(t)
		ctx := context.Background()

		seedDoc(t, db, "doc-a", "nest-a", 0)
		seedDoc(t, db, "doc-b", "nest-a", 1)
		seedDoc(t, db, "doc-c", "nest-a", 2)

		nestA := "nest-a"
		if err := svc.ReorderDocuments(ctx, spaceID, model.ReorderDocsDocumentsRequest{
			CollectionID: &nestA,
			DocumentIDs:  []string{"doc-c", "doc-a", "doc-b"},
		}); err != nil {
			t.Fatalf("ReorderDocuments: %v", err)
		}

		var docs []model.DocsDocument
		if err := db.WithContext(ctx).
			Where("space_id = ? AND collection_id = ? AND deleted_at IS NULL", spaceID, "nest-a").
			Order("position ASC").
			Find(&docs).Error; err != nil {
			t.Fatalf("load nest-a: %v", err)
		}
		want := []string{"doc-c", "doc-a", "doc-b"}
		if len(docs) != 3 {
			t.Fatalf("nest-a after reorder = %d docs, want 3", len(docs))
		}
		for i, d := range docs {
			if d.ID != want[i] || d.Position != i {
				t.Fatalf("nest-a[%d] = %s@%d, want %s@%d", i, d.ID, d.Position, want[i], i)
			}
		}
	})

	t.Run("Contiguous positions preserved after move", func(t *testing.T) {
		svc, db := setup(t)
		ctx := context.Background()

		seedDoc(t, db, "s1", "nest-a", 0)
		seedDoc(t, db, "s2", "nest-a", 1)
		seedDoc(t, db, "s3", "nest-a", 2)
		seedDoc(t, db, "t1", "nest-b", 0)

		targetColl := "nest-b"
		if _, err := svc.Move(ctx, "s2", model.MoveDocsDocumentRequest{
			SpaceID:      spaceID,
			CollectionID: &targetColl,
		}); err != nil {
			t.Fatalf("Move: %v", err)
		}

		// nest-a: [s1@0, s3@1]
		var sourceDocs []model.DocsDocument
		if err := db.WithContext(ctx).
			Where("space_id = ? AND collection_id = ? AND deleted_at IS NULL", spaceID, "nest-a").
			Order("position ASC").
			Find(&sourceDocs).Error; err != nil {
			t.Fatalf("load nest-a: %v", err)
		}
		if len(sourceDocs) != 2 {
			t.Fatalf("nest-a size after move = %d, want 2", len(sourceDocs))
		}
		for i, d := range sourceDocs {
			if d.Position != i {
				t.Fatalf("nest-a[%d] position = %d, want %d", i, d.Position, i)
			}
		}

		// nest-b: [t1@0, s2@1]
		var targetDocs []model.DocsDocument
		if err := db.WithContext(ctx).
			Where("space_id = ? AND collection_id = ? AND deleted_at IS NULL", spaceID, "nest-b").
			Order("position ASC").
			Find(&targetDocs).Error; err != nil {
			t.Fatalf("load nest-b: %v", err)
		}
		if len(targetDocs) != 2 {
			t.Fatalf("nest-b size after move = %d, want 2", len(targetDocs))
		}
		for i, d := range targetDocs {
			if d.Position != i {
				t.Fatalf("nest-b[%d] position = %d, want %d", i, d.Position, i)
			}
		}
	})
}

// TestDocsCollection_TreeDelete exercises the Task 5 safe-delete rules:
// when a collection is deleted, its direct child collections and direct
// articles are flattened up one level to the deleted node's parent (or
// become top-level / uncategorized when the deleted node was top-level).
// No descendant content is lost; all descendant depths are recalculated.
func TestDocsCollection_TreeDelete(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-del"
		userID      = "user-del"
		spaceID     = "space-del"
	)

	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)

	setup := func(t *testing.T) (*DocsCollectionService, *repository.DocsCollectionRepository, *repository.DocsDocumentRepository, *gorm.DB) {
		t.Helper()
		db := setupDocsOrderingTestDB(t)
		collectionRepo := repository.NewDocsCollectionRepository(db)
		spaceRepo := repository.NewDocsSpaceRepository(db)
		docRepo := repository.NewDocsDocumentRepository(db)
		svc := NewDocsCollectionService(collectionRepo, spaceRepo, nil)

		seedDocsSpace(t, db, model.DocsSpace{
			ID:          spaceID,
			WorkspaceID: workspaceID,
			Name:        "Del space",
			Slug:        "del-space",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		return svc, collectionRepo, docRepo, db
	}

	// seedCollection inserts a raw collection row with a fixed ID so tests
	// can refer to each node by name.
	seedCollection := func(t *testing.T, db *gorm.DB, id string, parentID *string, depth, pos int) {
		t.Helper()
		seedDocsCollection(t, db, model.DocsCollection{
			ID: id, SpaceID: spaceID, WorkspaceID: workspaceID,
			ParentCollectionID: parentID, Depth: depth,
			Name: id, Slug: id, Position: pos,
			CreatedBy: userID,
			CreatedAt: now.Add(time.Duration(pos) * time.Minute),
			UpdatedAt: now.Add(time.Duration(pos) * time.Minute),
		})
	}

	seedDoc := func(t *testing.T, db *gorm.DB, id string, collectionID *string, pos int) {
		t.Helper()
		seedDocsOrderingDocument(t, db, model.DocsDocument{
			ID:           id,
			WorkspaceID:  workspaceID,
			SpaceID:      spaceID,
			CollectionID: collectionID,
			Title:        id,
			Status:       model.DocStatusDraft,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     pos,
			CreatedBy:    userID,
			CreatedAt:    now.Add(time.Duration(pos) * time.Minute),
			UpdatedAt:    now.Add(time.Duration(pos) * time.Minute),
		})
	}

	t.Run("Delete top-level collection flattens children to top-level", func(t *testing.T) {
		svc, collectionRepo, _, db := setup(t)
		ctx := context.Background()

		// Tree:
		//   root (delete)
		//    ├─ child (depth 1)
		//    │   └─ grand (depth 2)
		//    └─ sibling (depth 1)
		//   other (depth 0)
		seedCollection(t, db, "root", nil, 0, 0)
		seedCollection(t, db, "other", nil, 0, 1)
		root := "root"
		seedCollection(t, db, "child", &root, 1, 0)
		seedCollection(t, db, "sibling", &root, 1, 1)
		child := "child"
		seedCollection(t, db, "grand", &child, 2, 0)

		if err := svc.Delete(ctx, "root"); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		// root should be soft-deleted.
		deleted, err := collectionRepo.GetByID(ctx, "root")
		if err != nil {
			t.Fatalf("GetByID root: %v", err)
		}
		if deleted != nil {
			t.Fatalf("root still live after delete")
		}

		// child and sibling should now be top-level (depth 0, nil parent).
		topLevel, err := collectionRepo.ListChildren(ctx, spaceID, nil)
		if err != nil {
			t.Fatalf("ListChildren(nil): %v", err)
		}
		if want := []string{"other", "child", "sibling"}; !equalIDs(ids(topLevel), want) {
			t.Fatalf("top-level after delete = %v, want %v", ids(topLevel), want)
		}
		for i, c := range topLevel {
			if c.Position != i {
				t.Fatalf("top-level[%d].Position = %d, want %d", i, c.Position, i)
			}
			if c.Depth != 0 {
				t.Fatalf("top-level[%d].Depth = %d, want 0", i, c.Depth)
			}
		}

		// grand should now be depth 1 (was 2), parent still child.
		var grandRow model.DocsCollection
		if err := db.WithContext(ctx).Where("id = ?", "grand").First(&grandRow).Error; err != nil {
			t.Fatalf("load grand: %v", err)
		}
		if grandRow.Depth != 1 {
			t.Fatalf("grand.Depth = %d, want 1", grandRow.Depth)
		}
		if grandRow.ParentCollectionID == nil || *grandRow.ParentCollectionID != "child" {
			t.Fatalf("grand.ParentCollectionID = %v, want child", grandRow.ParentCollectionID)
		}
	})

	t.Run("Delete nested collection moves direct articles to the parent", func(t *testing.T) {
		svc, _, docRepo, db := setup(t)
		ctx := context.Background()

		// Tree:
		//   root
		//    └─ target (delete)
		//         ├─ doc-a
		//         └─ doc-b
		//   (root already has its own doc-root)
		seedCollection(t, db, "root", nil, 0, 0)
		root := "root"
		seedCollection(t, db, "target", &root, 1, 0)
		target := "target"

		seedDoc(t, db, "doc-root", &root, 0)
		seedDoc(t, db, "doc-a", &target, 0)
		seedDoc(t, db, "doc-b", &target, 1)

		if err := svc.Delete(ctx, "target"); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		// All three docs should now live under root, positions contiguous.
		space := spaceID
		docs, err := docRepo.List(ctx, workspaceID, &space, &root, nil, nil, "", true)
		if err != nil {
			t.Fatalf("List docs under root: %v", err)
		}
		if len(docs) != 3 {
			t.Fatalf("root doc bucket = %d, want 3", len(docs))
		}
		want := []string{"doc-root", "doc-a", "doc-b"}
		for i, d := range docs {
			if d.ID != want[i] {
				t.Fatalf("doc[%d] = %s, want %s", i, d.ID, want[i])
			}
			if d.Position != i {
				t.Fatalf("doc[%d].Position = %d, want %d", i, d.Position, i)
			}
			if d.CollectionID == nil || *d.CollectionID != "root" {
				t.Fatalf("doc[%d].CollectionID = %v, want root", i, d.CollectionID)
			}
		}
	})

	t.Run("Delete preserves grandchildren depths and no content is orphaned", func(t *testing.T) {
		svc, collectionRepo, docRepo, db := setup(t)
		ctx := context.Background()

		// Tree:
		//   root
		//    ├─ mid (delete)
		//    │    ├─ leaf (depth 2)
		//    │    └─ sub (depth 2)
		//    └─ other-mid
		seedCollection(t, db, "root", nil, 0, 0)
		root := "root"
		seedCollection(t, db, "mid", &root, 1, 0)
		seedCollection(t, db, "other-mid", &root, 1, 1)
		mid := "mid"
		seedCollection(t, db, "leaf", &mid, 2, 0)
		seedCollection(t, db, "sub", &mid, 2, 1)

		seedDoc(t, db, "mid-doc", &mid, 0)

		if err := svc.Delete(ctx, "mid"); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		// Children of mid should now sit under root at depth 1,
		// appended after other-mid which was already there.
		underRoot, err := collectionRepo.ListChildren(ctx, spaceID, &root)
		if err != nil {
			t.Fatalf("ListChildren(root): %v", err)
		}
		want := []string{"other-mid", "leaf", "sub"}
		if !equalIDs(ids(underRoot), want) {
			t.Fatalf("under-root after delete = %v, want %v", ids(underRoot), want)
		}
		for i, c := range underRoot {
			if c.Position != i {
				t.Fatalf("under-root[%d].Position = %d, want %d", i, c.Position, i)
			}
			if c.Depth != 1 {
				t.Fatalf("under-root[%d].Depth = %d, want 1", i, c.Depth)
			}
		}

		// mid's article should now live under root.
		space := spaceID
		docs, err := docRepo.List(ctx, workspaceID, &space, &root, nil, nil, "", true)
		if err != nil {
			t.Fatalf("List docs under root: %v", err)
		}
		if len(docs) != 1 || docs[0].ID != "mid-doc" {
			t.Fatalf("root doc bucket = %+v, want [mid-doc]", docs)
		}
	})
}

func ids(colls []model.DocsCollection) []string {
	out := make([]string, len(colls))
	for i, c := range colls {
		out[i] = c.ID
	}
	return out
}

func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
