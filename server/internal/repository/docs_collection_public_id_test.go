package repository

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupDocsCollectionPublicIDRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:docs-collection-public-id?mode=memory&cache=shared"), &gorm.Config{})
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

func TestDocsCollectionRepositoryGetByPublicID(t *testing.T) {
	db := setupDocsCollectionPublicIDRepoTestDB(t)
	repo := NewDocsCollectionRepository(db)
	ctx := context.Background()

	_, err := repo.Create(ctx, &model.DocsCollection{
		ID:          "11111111-1111-1111-1111-111111111111",
		WorkspaceID: "workspace-1",
		SpaceID:     "space-1",
		Name:        "Getting Started",
		Slug:        "getting-started",
		PublicID:    "abc123ef",
		CreatedBy:   "user-1",
	})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}

	got, err := repo.GetByPublicID(ctx, "ABC123EF")
	if err != nil {
		t.Fatalf("get by public id: %v", err)
	}
	if got == nil || got.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("GetByPublicID returned %#v", got)
	}

	taken, err := repo.PublicIDExists(ctx, "abc123ef", "")
	if err != nil {
		t.Fatalf("public id exists: %v", err)
	}
	if !taken {
		t.Fatalf("expected public id to be taken")
	}

	taken, err = repo.PublicIDExists(ctx, "abc123ef", got.ID)
	if err != nil {
		t.Fatalf("public id exists with exclude: %v", err)
	}
	if taken {
		t.Fatalf("expected excluded public id not to be taken")
	}
}
