package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDocsAssetReferenceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:docs-asset-ref-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE docs_documents (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, deleted_at DATETIME)`,
		`CREATE TABLE docs_contents (id TEXT PRIMARY KEY, document_id TEXT NOT NULL, content TEXT, import_source_html TEXT)`,
		`CREATE TABLE docs_blocks (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, document_id TEXT NOT NULL, content TEXT, deleted_at DATETIME)`,
		`CREATE TABLE docs_versions (id TEXT PRIMARY KEY, document_id TEXT NOT NULL, content TEXT)`,
		`CREATE TABLE docs_helpcenter_article_publications (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, document_id TEXT NOT NULL, content TEXT)`,
		`CREATE TABLE docs_helpcenter_article_translations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, document_id TEXT NOT NULL, content TEXT)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	return db
}

func TestDocsAssetReferenceRepositoryHasSurvivingReferenceChecksContentTables(t *testing.T) {
	t.Parallel()

	const (
		workspaceID       = "ws-refs"
		deletedDocumentID = "doc-deleted"
		assetKey          = "docs-import/ws-refs/import 1/image.png"
	)

	cases := []struct {
		name string
		seed func(*gorm.DB)
	}{
		{
			name: "docs_contents.content",
			seed: func(db *gorm.DB) {
				db.Exec(`INSERT INTO docs_contents (id, document_id, content) VALUES (?, ?, ?)`, "content-1", "doc-live", fmt.Sprintf(`{"src":"%s"}`, assetKey))
			},
		},
		{
			name: "docs_contents.import_source_html",
			seed: func(db *gorm.DB) {
				db.Exec(`INSERT INTO docs_contents (id, document_id, import_source_html) VALUES (?, ?, ?)`, "content-1", "doc-live", fmt.Sprintf(`<img src="/%s">`, assetKey))
			},
		},
		{
			name: "docs_blocks.content",
			seed: func(db *gorm.DB) {
				db.Exec(`INSERT INTO docs_blocks (id, workspace_id, document_id, content) VALUES (?, ?, ?, ?)`, "block-1", workspaceID, "doc-live", fmt.Sprintf(`{"src":"%s"}`, assetKey))
			},
		},
		{
			name: "docs_versions.content",
			seed: func(db *gorm.DB) {
				db.Exec(`INSERT INTO docs_versions (id, document_id, content) VALUES (?, ?, ?)`, "version-1", "doc-live", fmt.Sprintf(`{"src":"%s"}`, assetKey))
			},
		},
		{
			name: "docs_helpcenter_article_publications.content",
			seed: func(db *gorm.DB) {
				db.Exec(`INSERT INTO docs_helpcenter_article_publications (id, workspace_id, document_id, content) VALUES (?, ?, ?, ?)`, "pub-1", workspaceID, "doc-live", fmt.Sprintf(`{"src":"%s"}`, assetKey))
			},
		},
		{
			name: "docs_helpcenter_article_translations.content",
			seed: func(db *gorm.DB) {
				db.Exec(`INSERT INTO docs_helpcenter_article_translations (id, workspace_id, document_id, content) VALUES (?, ?, ?, ?)`, "translation-1", workspaceID, "doc-live", fmt.Sprintf(`{"src":"%s"}`, assetKey))
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			db := setupDocsAssetReferenceTestDB(t)
			if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id) VALUES (?, ?)`, "doc-live", workspaceID).Error; err != nil {
				t.Fatalf("seed live doc: %v", err)
			}
			tc.seed(db)

			found, err := NewDocsAssetReferenceRepository(db).HasSurvivingReference(ctx, workspaceID, deletedDocumentID, assetKey)
			if err != nil {
				t.Fatalf("HasSurvivingReference: %v", err)
			}
			if !found {
				t.Fatalf("expected surviving reference in %s", tc.name)
			}
		})
	}
}

func TestDocsAssetReferenceRepositoryIgnoresDeletedDocumentAndSoftDeletedRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := setupDocsAssetReferenceTestDB(t)
	const (
		workspaceID       = "ws-refs"
		deletedDocumentID = "doc-deleted"
		assetKey          = "docs-import/ws-refs/import-1/image.png"
	)
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id) VALUES (?, ?)`, deletedDocumentID, workspaceID).Error; err != nil {
		t.Fatalf("seed deleted doc: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, deleted_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, "doc-soft-deleted", workspaceID).Error; err != nil {
		t.Fatalf("seed soft-deleted doc: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_contents (id, document_id, content) VALUES (?, ?, ?)`, "content-deleted", deletedDocumentID, fmt.Sprintf(`{"src":"%s"}`, assetKey)).Error; err != nil {
		t.Fatalf("seed deleted doc content: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_versions (id, document_id, content) VALUES (?, ?, ?)`, "version-soft-deleted", "doc-soft-deleted", fmt.Sprintf(`{"src":"%s"}`, assetKey)).Error; err != nil {
		t.Fatalf("seed soft-deleted version: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_blocks (id, workspace_id, document_id, content, deleted_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`, "block-deleted", workspaceID, "doc-live", fmt.Sprintf(`{"src":"%s"}`, assetKey)).Error; err != nil {
		t.Fatalf("seed deleted block: %v", err)
	}

	found, err := NewDocsAssetReferenceRepository(db).HasSurvivingReference(ctx, workspaceID, deletedDocumentID, assetKey)
	if err != nil {
		t.Fatalf("HasSurvivingReference: %v", err)
	}
	if found {
		t.Fatalf("expected deleted document and soft-deleted rows to be ignored")
	}
}
