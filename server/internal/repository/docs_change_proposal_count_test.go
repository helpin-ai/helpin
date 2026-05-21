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

func setupDocsProposalCountTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-proposal-count-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	stmts := []string{
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
			block_id TEXT,
			agent_id TEXT,
			agent_run_id TEXT,
			scope TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			revision INTEGER NOT NULL DEFAULT 0,
			summary TEXT NOT NULL,
			content_markdown TEXT NOT NULL,
			content BLOB NOT NULL,
			sources BLOB NOT NULL DEFAULT '[]',
			created_by TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create docs proposal count test table: %v", err)
		}
	}
	return db
}

func TestDocsDocumentRepositoryListIncludesPendingChangeProposalCount(t *testing.T) {
	ctx := context.Background()
	db := setupDocsProposalCountTestDB(t)
	docRepo := NewDocsDocumentRepository(db)
	proposalRepo := NewDocsChangeProposalRepository(db)

	doc := &model.DocsDocument{
		ID:          "doc-1",
		WorkspaceID: "ws-1",
		SpaceID:     "space-1",
		Title:       "Billing",
		Status:      model.DocStatusPublished,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		CreatedBy:   "user-1",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := db.WithContext(ctx).Create(doc).Error; err != nil {
		t.Fatalf("seed doc: %v", err)
	}

	createProposal := func(id, status string) {
		t.Helper()
		if _, err := proposalRepo.Create(ctx, &model.DocsChangeProposal{
			ID:              id,
			WorkspaceID:     "ws-1",
			DocumentID:      "doc-1",
			Scope:           "document",
			Status:          status,
			Summary:         "Update docs",
			ContentMarkdown: "New docs",
			Content:         []byte(`{"type":"doc","content":[]}`),
			Sources:         []byte(`[]`),
			CreatedBy:       "agent-1",
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		}); err != nil {
			t.Fatalf("seed proposal %s: %v", id, err)
		}
	}
	createProposal("proposal-pending-1", model.DocsChangeProposalStatusPending)
	createProposal("proposal-pending-2", model.DocsChangeProposalStatusPending)
	createProposal("proposal-applied", model.DocsChangeProposalStatusApplied)
	createProposal("proposal-discarded", model.DocsChangeProposalStatusDiscarded)

	docs, err := docRepo.List(ctx, "ws-1", nil, nil, nil, nil, "", true)
	if err != nil {
		t.Fatalf("list docs: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d, want 1", len(docs))
	}
	if docs[0].PendingChangeProposalCount != 2 {
		t.Fatalf("PendingChangeProposalCount = %d, want 2", docs[0].PendingChangeProposalCount)
	}

	if err := proposalRepo.Resolve(ctx, "ws-1", "proposal-pending-1", model.DocsChangeProposalStatusApplied, "user-1"); err != nil {
		t.Fatalf("resolve proposal: %v", err)
	}

	docs, err = docRepo.List(ctx, "ws-1", nil, nil, nil, nil, "", true)
	if err != nil {
		t.Fatalf("list docs after resolve: %v", err)
	}
	if docs[0].PendingChangeProposalCount != 1 {
		t.Fatalf("PendingChangeProposalCount after resolve = %d, want 1", docs[0].PendingChangeProposalCount)
	}

	if err := proposalRepo.Resolve(ctx, "ws-1", "proposal-pending-2", model.DocsChangeProposalStatusDiscarded, "user-1"); err != nil {
		t.Fatalf("discard proposal: %v", err)
	}

	docs, err = docRepo.List(ctx, "ws-1", nil, nil, nil, nil, "", true)
	if err != nil {
		t.Fatalf("list docs after discard: %v", err)
	}
	if docs[0].PendingChangeProposalCount != 0 {
		t.Fatalf("PendingChangeProposalCount after discard = %d, want 0", docs[0].PendingChangeProposalCount)
	}
}

func TestDocsChangeProposalRepositoryGetByDocumentIDAndIDReturnsResolvedProposal(t *testing.T) {
	ctx := context.Background()
	db := setupDocsProposalCountTestDB(t)
	repo := NewDocsChangeProposalRepository(db)

	if err := db.WithContext(ctx).Create(&model.DocsDocument{
		ID:          "doc-1",
		WorkspaceID: "ws-1",
		SpaceID:     "space-1",
		Title:       "Billing",
		Status:      model.DocStatusPublished,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		CreatedBy:   "user-1",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("seed doc: %v", err)
	}
	if _, err := repo.Create(ctx, &model.DocsChangeProposal{
		ID:              "proposal-applied",
		WorkspaceID:     "ws-1",
		DocumentID:      "doc-1",
		Scope:           "document",
		Status:          model.DocsChangeProposalStatusApplied,
		Summary:         "Update docs",
		ContentMarkdown: "New docs",
		Content:         []byte(`{"type":"doc","content":[]}`),
		Sources:         []byte(`[]`),
		CreatedBy:       "agent-1",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	got, err := repo.GetByDocumentIDAndID(ctx, "ws-1", "doc-1", "proposal-applied")
	if err != nil {
		t.Fatalf("get proposal: %v", err)
	}
	if got == nil {
		t.Fatal("got nil proposal, want resolved proposal")
	}
	if got.Status != model.DocsChangeProposalStatusApplied {
		t.Fatalf("Status = %q, want applied", got.Status)
	}

	wrongDoc, err := repo.GetByDocumentIDAndID(ctx, "ws-1", "other-doc", "proposal-applied")
	if err != nil {
		t.Fatalf("get wrong doc proposal: %v", err)
	}
	if wrongDoc != nil {
		t.Fatalf("wrong doc proposal = %#v, want nil", wrongDoc)
	}
}
