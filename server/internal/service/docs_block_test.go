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

func TestDocsBlockServiceInsertKeepsSiblingRevisions(t *testing.T) {
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
	second := blocks[1]

	// Inserting between First and Second shifts Second's position; its
	// revision must not change, so pending proposals against it stay valid.
	if _, err := blockSvc.Create(ctx, documentID, &blocks[0].ID, json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Inserted"}]}`), "30000000-0000-0000-0000-000000000002"); err != nil {
		t.Fatalf("create block: %v", err)
	}
	blocks, err = blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks after create: %v", err)
	}
	assertBlockTexts(t, blocks, []string{"First", "Inserted", "Second"})
	if blocks[2].Revision != second.Revision {
		t.Fatalf("second block revision = %d after insert, want %d", blocks[2].Revision, second.Revision)
	}

	if _, err := blockSvc.Patch(ctx, documentID, second.ID, second.Revision, json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Second updated"}]}`), "30000000-0000-0000-0000-000000000003"); err != nil {
		t.Fatalf("patch with pre-insert revision: %v", err)
	}
}

func TestDocsBlockServiceRejectsMalformedBlockContent(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)

	blockRepo, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	original := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Original"}]}]}`
	if _, err := contentSvc.Save(ctx, documentID, json.RawMessage(original), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list initial blocks: %v", err)
	}
	block := blocks[0]

	// "content" as a string instead of child nodes: accepted before, it broke
	// every reader of the document once stored.
	malformed := []struct {
		name    string
		content string
	}{
		{name: "content as string", content: `{"type":"paragraph","content":"Just text"}`},
		{name: "content as object", content: `{"type":"paragraph","content":{"type":"text","text":"Hi"}}`},
		{name: "nested content as string", content: `{"type":"bulletList","content":[{"type":"listItem","content":"x"}]}`},
	}
	for _, tt := range malformed {
		t.Run("patch/"+tt.name, func(t *testing.T) {
			if _, err := blockSvc.Patch(ctx, documentID, block.ID, block.Revision, json.RawMessage(tt.content), "30000000-0000-0000-0000-000000000002"); err == nil {
				t.Fatalf("Patch() accepted malformed block content %s", tt.content)
			}
		})
		t.Run("create/"+tt.name, func(t *testing.T) {
			if _, err := blockSvc.Create(ctx, documentID, nil, json.RawMessage(tt.content), "30000000-0000-0000-0000-000000000002"); err == nil {
				t.Fatalf("Create() accepted malformed block content %s", tt.content)
			}
		})
	}

	// The stored document must be untouched by the rejected writes.
	saved, err := contentSvc.Get(ctx, documentID)
	if err != nil {
		t.Fatalf("get content: %v", err)
	}
	if !strings.Contains(string(saved.Content), "Original") {
		t.Fatalf("document content changed after rejected writes: %s", string(saved.Content))
	}
	after, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks: %v", err)
	}
	if len(after) != 1 || after[0].Revision != block.Revision {
		t.Fatalf("blocks changed after rejected writes: %#v", after)
	}
}

func TestDocsBlockServiceEditorRoundTripKeepsRevisions(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)

	blockRepo, contentSvc, _ := newDocsBlockServiceTestServices(db)
	if _, err := contentSvc.Save(ctx, documentID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Alpha"}]}]}`), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list initial blocks: %v", err)
	}

	// The editor serializes the same node with different key order and
	// null-valued default attrs; that must not read as a content edit.
	editorSave := fmt.Sprintf(`{"type":"doc","content":[{"type":"paragraph","attrs":{"blockId":"%s","textAlign":null},"content":[{"type":"text","text":"Alpha"}]}]}`, blocks[0].ID)
	if _, err := contentSvc.Save(ctx, documentID, json.RawMessage(editorSave), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("editor resave: %v", err)
	}
	after, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks after resave: %v", err)
	}
	if len(after) != 1 || after[0].ID != blocks[0].ID {
		t.Fatalf("expected the same block to survive, got %#v", after)
	}
	if after[0].Revision != blocks[0].Revision {
		t.Fatalf("revision = %d after editor round trip, want %d", after[0].Revision, blocks[0].Revision)
	}
}

func TestDocsBlockServiceCreateBlocksInsertsMultiple(t *testing.T) {
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

	_, createdIDs, err := blockSvc.CreateBlocks(ctx, documentID, &blocks[0].ID, false, []json.RawMessage{
		json.RawMessage(`{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Middle heading"}]}`),
		json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Middle body"}]}`),
	}, "30000000-0000-0000-0000-000000000002")
	if err != nil {
		t.Fatalf("create blocks: %v", err)
	}
	if len(createdIDs) != 2 {
		t.Fatalf("expected 2 created block ids, got %d", len(createdIDs))
	}
	blocks, err = blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks after create: %v", err)
	}
	assertBlockTexts(t, blocks, []string{"First", "Middle heading", "Middle body", "Second"})
	if blocks[1].ID != createdIDs[0] || blocks[2].ID != createdIDs[1] {
		t.Fatalf("created ids %v do not match inserted blocks %q, %q", createdIDs, blocks[1].ID, blocks[2].ID)
	}

	if _, _, err := blockSvc.CreateBlocks(ctx, documentID, nil, true, []json.RawMessage{
		json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Prepended"}]}`),
	}, "30000000-0000-0000-0000-000000000003"); err != nil {
		t.Fatalf("create blocks at start: %v", err)
	}
	blocks, err = blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks after prepend: %v", err)
	}
	assertBlockTexts(t, blocks, []string{"Prepended", "First", "Middle heading", "Middle body", "Second"})
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

func TestDocsBlockServiceLogsBlockActivity(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	actorID := "30000000-0000-0000-0000-000000000002"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)

	blockRepo, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	blockSvc.SetActivityService(NewPMActivityService(repository.NewPMActivityRepository(db)))
	if _, err := contentSvc.Save(ctx, documentID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Original"}]}]}`), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list initial blocks: %v", err)
	}

	if _, err := blockSvc.Patch(ctx, documentID, blocks[0].ID, blocks[0].Revision, json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Updated"}]}`), actorID); err != nil {
		t.Fatalf("patch block: %v", err)
	}

	var activity model.PMActivityLog
	if err := db.Where("entity_type = ? AND entity_id = ? AND action = ?", "doc", documentID, "block_updated").First(&activity).Error; err != nil {
		t.Fatalf("load activity: %v", err)
	}
	if activity.WorkspaceID != workspaceID {
		t.Fatalf("workspace_id = %q, want %q", activity.WorkspaceID, workspaceID)
	}
	if activity.ActorID == nil || *activity.ActorID != actorID {
		t.Fatalf("actor_id = %v, want %q", activity.ActorID, actorID)
	}
	if !strings.Contains(string(activity.Metadata), blocks[0].ID) {
		t.Fatalf("metadata does not include block id %q: %s", blocks[0].ID, string(activity.Metadata))
	}
}

func TestDocsBlockServiceMarkStaleFromSupport(t *testing.T) {
	db := setupDocsBlockServiceTestDB(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockServiceTestDocument(t, db, documentID, workspaceID, false)

	blockRepo, contentSvc, blockSvc := newDocsBlockServiceTestServices(db)
	blockSvc.SetActivityService(NewPMActivityService(repository.NewPMActivityRepository(db)))
	if _, err := contentSvc.Save(ctx, documentID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Refunds take 5 days"}]}]}`), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list initial blocks: %v", err)
	}

	if err := blockSvc.MarkStaleFromSupport(ctx, workspaceID, documentID, blocks[0].ID, "gap-1", "Customers report refunds now take 10 days", "article_feedback"); err != nil {
		t.Fatalf("mark stale: %v", err)
	}

	content, err := contentSvc.Get(ctx, documentID)
	if err != nil {
		t.Fatalf("get content: %v", err)
	}
	var aggregate aggregateDoc
	if err := json.Unmarshal(content.Content, &aggregate); err != nil {
		t.Fatalf("decode content: %v", err)
	}
	attrs, _ := aggregate.Content[0]["attrs"].(map[string]any)
	if attrs["staleState"] != "support_gap" {
		t.Fatalf("staleState = %v, want support_gap", attrs["staleState"])
	}
	if attrs["staleGapId"] != "gap-1" {
		t.Fatalf("staleGapId = %v, want gap-1", attrs["staleGapId"])
	}

	var count int64
	if err := db.Model(&model.PMActivityLog{}).Where("entity_id = ? AND action = ?", documentID, "block_marked_stale").Count(&count).Error; err != nil {
		t.Fatalf("count activity: %v", err)
	}
	if count != 1 {
		t.Fatalf("stale activity count = %d, want 1", count)
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
		`CREATE TABLE pm_activity_log (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			actor_id TEXT,
			action TEXT NOT NULL,
			field_name TEXT,
			old_value TEXT,
			new_value TEXT,
			metadata JSON,
			created_at DATETIME
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
