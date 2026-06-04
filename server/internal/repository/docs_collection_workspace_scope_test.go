package repository

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupDocsCollectionWorkspaceScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE docs_collections (
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
		)
	`).Error; err != nil {
		t.Fatalf("create docs_collections table: %v", err)
	}
	return db
}

func TestDocsCollectionRepositoryListByWorkspaceAndSpaceScopesToWorkspace(t *testing.T) {
	db := setupDocsCollectionWorkspaceScopeTestDB(t)
	repo := NewDocsCollectionRepository(db)
	ctx := context.Background()

	for _, coll := range []model.DocsCollection{
		{
			ID:          "coll-ws-1",
			WorkspaceID: "ws-1",
			SpaceID:     "shared-space",
			Name:        "Workspace 1 Collection",
			Slug:        "ws-1-collection",
			Position:    1,
			CreatedBy:   "user-1",
		},
		{
			ID:          "coll-ws-2",
			WorkspaceID: "ws-2",
			SpaceID:     "shared-space",
			Name:        "Workspace 2 Collection",
			Slug:        "ws-2-collection",
			Position:    0,
			CreatedBy:   "user-2",
		},
	} {
		coll := coll
		if _, err := repo.Create(ctx, &coll); err != nil {
			t.Fatalf("create collection %s: %v", coll.ID, err)
		}
	}

	got, err := repo.ListByWorkspaceAndSpace(ctx, "ws-1", "shared-space")
	if err != nil {
		t.Fatalf("list collections: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 workspace-scoped collection, got %d: %#v", len(got), got)
	}
	if got[0].ID != "coll-ws-1" {
		t.Fatalf("expected ws-1 collection, got %s", got[0].ID)
	}
}
