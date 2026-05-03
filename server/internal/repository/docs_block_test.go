package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDocsContentUpsertSyncsAddressableBlocks(t *testing.T) {
	db := setupDocsBlockTestDB(t)
	ctx := context.Background()

	contentRepo := NewDocsContentRepository(db)
	blockRepo := NewDocsBlockRepository(db)
	contentRepo.SetBlockRepository(blockRepo)

	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockTestDocument(t, db, documentID, workspaceID)

	initial := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"First block"}]},{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Second block"}]}]}`)
	saved, err := contentRepo.UpsertWithActor(ctx, documentID, initial, "30000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("upsert content: %v", err)
	}
	if !strings.Contains(string(saved.Content), `"blockId"`) {
		t.Fatalf("expected normalized aggregate content to include block IDs: %s", string(saved.Content))
	}

	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 synced blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "paragraph" || blocks[0].ContentText != "First block" || blocks[0].Revision != 1 {
		t.Fatalf("unexpected first block: %#v", blocks[0])
	}
	if blocks[0].AuthoredBy == nil || *blocks[0].AuthoredBy != "30000000-0000-0000-0000-000000000001" {
		t.Fatalf("expected authored_by to be set on first save, got %#v", blocks[0].AuthoredBy)
	}

	var aggregate map[string]any
	if err := json.Unmarshal(saved.Content, &aggregate); err != nil {
		t.Fatalf("unmarshal normalized content: %v", err)
	}
	children := aggregate["content"].([]any)
	first := children[0].(map[string]any)
	first["content"] = []any{map[string]any{"type": "text", "text": "First block edited"}}
	edited, _ := json.Marshal(aggregate)

	if _, err := contentRepo.UpsertWithActor(ctx, documentID, edited, "30000000-0000-0000-0000-000000000002"); err != nil {
		t.Fatalf("upsert edited content: %v", err)
	}
	blocks, err = blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list edited blocks: %v", err)
	}
	if blocks[0].ID == "" || blocks[0].Revision != 2 || blocks[0].ContentText != "First block edited" {
		t.Fatalf("expected edited first block revision/content, got %#v", blocks[0])
	}
	if blocks[0].AuthoredBy == nil || *blocks[0].AuthoredBy != "30000000-0000-0000-0000-000000000001" {
		t.Fatalf("expected authored_by to remain original actor, got %#v", blocks[0].AuthoredBy)
	}
	if blocks[0].LastEditedBy == nil || *blocks[0].LastEditedBy != "30000000-0000-0000-0000-000000000002" {
		t.Fatalf("expected last_edited_by to update on changed block, got %#v", blocks[0].LastEditedBy)
	}
	if blocks[1].Revision != 1 {
		t.Fatalf("expected unchanged block revision to remain 1, got %d", blocks[1].Revision)
	}
}

func TestDocsContentUpsertRegeneratesCrossDocumentBlockID(t *testing.T) {
	db := setupDocsBlockTestDB(t)
	ctx := context.Background()

	contentRepo := NewDocsContentRepository(db)
	blockRepo := NewDocsBlockRepository(db)
	contentRepo.SetBlockRepository(blockRepo)

	workspaceID := "20000000-0000-0000-0000-000000000001"
	documentOneID := "10000000-0000-0000-0000-000000000001"
	documentTwoID := "10000000-0000-0000-0000-000000000002"
	insertDocsBlockTestDocument(t, db, documentOneID, workspaceID)
	insertDocsBlockTestDocument(t, db, documentTwoID, workspaceID)

	copiedID := "40000000-0000-0000-0000-000000000001"
	first := json.RawMessage(fmt.Sprintf(`{"type":"doc","content":[{"type":"paragraph","attrs":{"blockId":%q},"content":[{"type":"text","text":"Original"}]}]}`, copiedID))
	if _, err := contentRepo.UpsertWithActor(ctx, documentOneID, first, "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("upsert first document: %v", err)
	}

	second := json.RawMessage(fmt.Sprintf(`{"type":"doc","content":[{"type":"paragraph","attrs":{"blockId":%q},"content":[{"type":"text","text":"Copied"}]}]}`, copiedID))
	saved, err := contentRepo.UpsertWithActor(ctx, documentTwoID, second, "30000000-0000-0000-0000-000000000002")
	if err != nil {
		t.Fatalf("upsert second document: %v", err)
	}
	if strings.Contains(string(saved.Content), copiedID) {
		t.Fatalf("expected copied cross-document block ID to be regenerated: %s", string(saved.Content))
	}

	blocks, err := blockRepo.ListByDocument(ctx, documentTwoID, false)
	if err != nil {
		t.Fatalf("list second document blocks: %v", err)
	}
	if len(blocks) != 1 || blocks[0].ID == copiedID {
		t.Fatalf("expected regenerated block row, got %#v", blocks)
	}
}

func setupDocsBlockTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-blocks-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	stmts := []string{
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			is_locked BOOLEAN NOT NULL DEFAULT 0,
			updated_at DATETIME
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

func insertDocsBlockTestDocument(t *testing.T, db *gorm.DB, documentID, workspaceID string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, is_locked, updated_at) VALUES (?, ?, 0, ?)`, documentID, workspaceID, time.Now().UTC()).Error; err != nil {
		t.Fatalf("insert document: %v", err)
	}
}
