package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestDocsHandlerPatchBlockReturnsConflictForStaleRevision(t *testing.T) {
	h, blockRepo, blockSvc, db := setupDocsBlockHandlerTest(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockHandlerTestDocument(t, db, documentID, workspaceID, false)

	if _, err := h.contentSvc.Save(ctx, documentID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Original"}]}]}`), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks: %v", err)
	}
	block := blocks[0]
	if _, err := blockSvc.Patch(ctx, documentID, block.ID, block.Revision, json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Current"}]}`), "30000000-0000-0000-0000-000000000002"); err != nil {
		t.Fatalf("advance block revision: %v", err)
	}

	rec := httptest.NewRecorder()
	req := docsBlockPatchRequest(t, documentID, block.ID, model.DocsBlockPatchRequest{
		Revision: block.Revision,
		Content:  json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Stale"}]}`),
	})
	h.PatchBlock(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if !contains(rec.Body.String(), "block revision is stale") {
		t.Fatalf("body = %q, want stale revision message", rec.Body.String())
	}
}

func TestDocsHandlerPatchBlockReturnsForbiddenForLockedDocument(t *testing.T) {
	h, blockRepo, _, db := setupDocsBlockHandlerTest(t)
	ctx := context.Background()
	documentID := "10000000-0000-0000-0000-000000000001"
	workspaceID := "20000000-0000-0000-0000-000000000001"
	insertDocsBlockHandlerTestDocument(t, db, documentID, workspaceID, false)

	if _, err := h.contentSvc.Save(ctx, documentID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Original"}]}]}`), "30000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("save initial content: %v", err)
	}
	blocks, err := blockRepo.ListByDocument(ctx, documentID, false)
	if err != nil {
		t.Fatalf("list blocks: %v", err)
	}
	if err := db.Exec(`UPDATE docs_documents SET is_locked = 1 WHERE id = ?`, documentID).Error; err != nil {
		t.Fatalf("lock document: %v", err)
	}

	rec := httptest.NewRecorder()
	req := docsBlockPatchRequest(t, documentID, blocks[0].ID, model.DocsBlockPatchRequest{
		Revision: blocks[0].Revision,
		Content:  json.RawMessage(`{"type":"paragraph","content":[{"type":"text","text":"Locked"}]}`),
	})
	h.PatchBlock(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !contains(rec.Body.String(), "locked") {
		t.Fatalf("body = %q, want locked message", rec.Body.String())
	}
}

func setupDocsBlockHandlerTest(t *testing.T) (*DocsHandler, *repository.DocsBlockRepository, *service.DocsBlockService, *gorm.DB) {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-block-handler-%d?mode=memory&cache=shared", time.Now().UnixNano())
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

	contentRepo := repository.NewDocsContentRepository(db)
	blockRepo := repository.NewDocsBlockRepository(db)
	docRepo := repository.NewDocsDocumentRepository(db)
	contentRepo.SetBlockRepository(blockRepo)
	contentSvc := service.NewDocsContentService(contentRepo, docRepo, nil)
	blockSvc := service.NewDocsBlockService(blockRepo, contentSvc, docRepo)
	h := &DocsHandler{contentSvc: contentSvc, blockSvc: blockSvc}
	return h, blockRepo, blockSvc, db
}

func insertDocsBlockHandlerTestDocument(t *testing.T, db *gorm.DB, documentID, workspaceID string, locked bool) {
	t.Helper()
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, title, is_locked, updated_at) VALUES (?, ?, 'Block Handler Test', ?, ?)`, documentID, workspaceID, locked, time.Now().UTC()).Error; err != nil {
		t.Fatalf("insert document: %v", err)
	}
}

func docsBlockPatchRequest(t *testing.T, documentID, blockID string, body model.DocsBlockPatchRequest) *http.Request {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/docs/documents/"+documentID+"/blocks/"+blockID, bytes.NewReader(payload))
	req = req.WithContext(middleware.WithUserID(req.Context(), "30000000-0000-0000-0000-000000000999"))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("docId", documentID)
	routeCtx.URLParams.Add("blockId", blockID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}
