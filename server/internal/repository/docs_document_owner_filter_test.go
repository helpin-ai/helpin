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

func setupDocsOwnerFilterTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-owner-filter-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
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
			sort_key TEXT NOT NULL DEFAULT '~',
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
		`CREATE TABLE docs_change_proposals (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			status TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create docs owner filter test table: %v", err)
		}
	}

	return db
}

func TestDocsDocumentRepositoryListWithOwner(t *testing.T) {
	ctx := context.Background()
	db := setupDocsOwnerFilterTestDB(t)
	repo := NewDocsDocumentRepository(db)
	now := time.Now().UTC()
	ownerOne := "member-1"
	ownerTwo := "member-2"

	documents := []model.DocsDocument{
		{
			ID: "owned-published", WorkspaceID: "ws-1", SpaceID: "space-1",
			Title: "Published", Status: model.DocStatusPublished,
			Visibility: model.SpaceVisibilityWorkspaceWide, OwnerID: &ownerOne,
			CreatedBy: "user-1", CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "owned-archived", WorkspaceID: "ws-1", SpaceID: "space-1",
			Title: "Archived", Status: model.DocStatusArchived,
			Visibility: model.SpaceVisibilityWorkspaceWide, OwnerID: &ownerOne,
			CreatedBy: "user-1", CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "other-owner", WorkspaceID: "ws-1", SpaceID: "space-1",
			Title: "Other owner", Status: model.DocStatusPublished,
			Visibility: model.SpaceVisibilityWorkspaceWide, OwnerID: &ownerTwo,
			CreatedBy: "user-2", CreatedAt: now, UpdatedAt: now,
		},
	}
	for i := range documents {
		if err := db.WithContext(ctx).Create(&documents[i]).Error; err != nil {
			t.Fatalf("seed document %s: %v", documents[i].ID, err)
		}
	}

	got, err := repo.ListWithOwner(ctx, "ws-1", nil, nil, nil, nil, &ownerOne, "", true)
	if err != nil {
		t.Fatalf("list documents by owner: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	for _, doc := range got {
		if doc.OwnerID == nil || *doc.OwnerID != ownerOne {
			t.Errorf("document %s owner = %v, want %s", doc.ID, doc.OwnerID, ownerOne)
		}
	}
}
