package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestDocsBlockServiceCreateReorderDelete(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)

	blockRepo, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	if _, err := contentSvc.Save(ctx, documentID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"First"}]},{"type":"paragraph","content":[{"type":"text","text":"Second"}]}]}`), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list initial blocks: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 initial blocks, got %d", len(blocks))
	}

	createdContent, err := blockSvc.Create(ctx, documentID, &blocks[0].ID, json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Inserted"}]}`), "30000000-0000-0000-0000-000000000002")
	if err != nil {
		t.Fatalf("create block: %v", err)
	}
	blocks, err = blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks after create: %v", err)
	}
	assertBlockTexts(t, blocks, []string{"First", "Inserted", "Second"})
	if !strings.Contains(string(createdContent.Content), "Inserted") {
		t.Fatalf("expected aggregate content to include inserted block, got %s", string(createdContent.Content))
	}

	if _, err := blockSvc.Reorder(ctx, documentID, []string{blocks[2].ID, blocks[0].ID, blocks[1].ID}, "30000000-0000-0000-0000-000000000003"); err != nil {
		t.Fatalf("reorder blocks: %v", err)
	}
	blocks, err = blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks after reorder: %v", err)
	}
	assertBlockTexts(t, blocks, []string{"Second", "First", "Inserted"})

	if _, err := blockSvc.Delete(ctx, documentID, blocks[2].ID, "30000000-0000-0000-0000-000000000004"); err != nil {
		t.Fatalf("delete block: %v", err)
	}
	blocks, err = blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks after delete: %v", err)
	}
	assertBlockTexts(t, blocks, []string{"Second", "First"})
}

func TestDocsBlockServicePatchRejectsStaleRevision(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)

	blockRepo, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	if _, err := contentSvc.Save(ctx, documentID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Original"}]}]}`), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list initial blocks: %v", err)
	}
	block := blocks[0]

	if _, err := blockSvc.Patch(ctx, documentID, block.ID, block.Revision, json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Updated"}]}`), "30000000-0000-0000-0000-000000000002"); err != nil {
		t.Fatalf("patch block: %v", err)
	}
	if _, err := blockSvc.Patch(ctx, documentID, block.ID, block.Revision, json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Stale"}]}`), "30000000-0000-0000-0000-000000000003"); !errors.Is(err, ErrDocsStaleBlockRevision) {
		t.Fatalf("expected stale revision error, got %v", err)
	}
}

func TestDocsBlockServiceRejectsLockedDocumentMutation(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)

	blockRepo, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	if _, err := contentSvc.Save(ctx, documentID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Original"}]}]}`), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list initial blocks: %v", err)
	}
	if err := db.Exec(`UPDATE docs_documents SET is_locked = 1 WHERE id = ?`, documentID).Error; err != nil {
		t.Fatalf("lock document: %v", err)
	}

	_, err = blockSvc.Patch(ctx, documentID, blocks[0].ID, blocks[0].Revision, json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Updated"}]}`), "30000000-0000-0000-0000-000000000002")
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("expected locked document error, got %v", err)
	}
}

func newDocsBlockServiceTestServices(db *gorm.DB) (*repository.DocsBlockRepository, *DocsContentService, *DocsBlockService) {
	contentRepo := repository.NewDocsContentRepository(db)
	blockRepo := repository.NewDocsBlockRepository(db)
	docRepo := repository.NewDocsDocumentRepository(db)
	contentRepo.SetBlockRepository(blockRepo)
	contentSvc := NewDocsContentService(contentRepo, docRepo, nil)
	blockSvc := NewDocsBlockService(blockRepo, contentSvc, docRepo)
	return blockRepo, contentSvc, blockSvc
}

func setupDocsBlockServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-block-service-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	stmts := []string{
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL DEFAULT '',
			collection_id TEXT,
			title TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			visibility TEXT NOT NULL DEFAULT 'workspace_wide',
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
			created_by TEXT NOT NULL DEFAULT '',
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_contents (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE,
			content JSON,
			content_text TEXT,
			word_count INTEGER NOT NULL DEFAULT 0,
			import_source_html TEXT,
			import_source_system TEXT,
			import_source_object_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_blocks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			parent_id TEXT,
			type TEXT NOT NULL,
			content JSON NOT NULL DEFAULT '{}',
			content_text TEXT,
			sort_key TEXT NOT NULL DEFAULT '~',
			revision INTEGER NOT NULL DEFAULT 1,
			authored_by TEXT,
			last_edited_by TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	return db
}

func insertDocsBlockServiceTestDocument(t *testing.T, db *gorm.DB, documentID, workspaceID string, locked bool) {
	t.Helper()
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, title, is_locked, updated_at) VALUES (?, ?, 'Block Test', ?, ?)`, documentID, workspaceID, locked, time.Now().UTC()).Error; err != nil {
		t.Fatalf("insert document: %v", err)
	}
}

func assertBlockTexts(t *testing.T, blocks []model.DocsBlock, expected []string) {
	t.Helper()
	if len(blocks) != len(expected) {
		t.Fatalf("expected %d blocks, got %d: %#v", len(expected), len(blocks), blocks)
	}
	for i, want := range expected {
		if blocks[i].ContentText != want {
			t.Fatalf("block %d text = %q, want %q", i, blocks[i].ContentText, want)
		}
	}
}
