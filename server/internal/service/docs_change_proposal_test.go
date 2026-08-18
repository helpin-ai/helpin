package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestProposalApplyLabel(t *testing.T) {
	t.Parallel()

	longSummary := strings.Repeat("a", 120)

	tests := []struct {
		name     string
		proposal *model.DocsChangeProposal
		want     string
	}{
		{
			name:     "nil proposal returns generic label",
			proposal: nil,
			want:     "Applied proposal",
		},
		{
			name:     "empty summary returns generic label",
			proposal: &model.DocsChangeProposal{Summary: ""},
			want:     "Applied proposal",
		},
		{
			name:     "whitespace-only summary returns generic label",
			proposal: &model.DocsChangeProposal{Summary: "   \n\t  "},
			want:     "Applied proposal",
		},
		{
			name:     "summary is included after Applied prefix",
			proposal: &model.DocsChangeProposal{Summary: "Update billing FAQ"},
			want:     "Applied: Update billing FAQ",
		},
		{
			name:     "summary is trimmed before use",
			proposal: &model.DocsChangeProposal{Summary: "  Update billing FAQ\n"},
			want:     "Applied: Update billing FAQ",
		},
		{
			name:     "long summary is truncated with ellipsis",
			proposal: &model.DocsChangeProposal{Summary: longSummary},
			want:     "Applied: " + strings.Repeat("a", 80) + "…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := proposalApplyLabel(tt.proposal)
			if got != tt.want {
				t.Errorf("proposalApplyLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func setupDocsChangeProposalServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:docs-change-proposal-service-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	stmts := []string{
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			visibility TEXT NOT NULL,
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
			base_markdown TEXT NOT NULL DEFAULT '',
			content BLOB NOT NULL,
			sources BLOB NOT NULL DEFAULT '[]',
			created_by TEXT NOT NULL,
			resolved_by TEXT,
			resolved_at DATETIME,
			created_at DATETIME,
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
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}
	return db
}

func TestDocsChangeProposalServiceGetReturnsResolvedProposal(t *testing.T) {
	ctx := context.Background()
	db := setupDocsChangeProposalServiceTestDB(t)
	docRepo := repository.NewDocsDocumentRepository(db)
	proposalRepo := repository.NewDocsChangeProposalRepository(db)
	svc := NewDocsChangeProposalService(proposalRepo, docRepo, nil, nil, nil, nil)

	if err := db.WithContext(ctx).Exec(
		`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"doc-1",
		"ws-1",
		"space-1",
		"Billing",
		model.DocStatusPublished,
		model.SpaceVisibilityWorkspaceWide,
		"user-1",
		time.Now().UTC(),
		time.Now().UTC(),
	).Error; err != nil {
		t.Fatalf("seed doc: %v", err)
	}
	if _, err := proposalRepo.Create(ctx, &model.DocsChangeProposal{
		ID:              "proposal-1",
		WorkspaceID:     "ws-1",
		DocumentID:      "doc-1",
		Scope:           "document",
		Status:          model.DocsChangeProposalStatusApplied,
		Summary:         "Update docs",
		ContentMarkdown: "New docs",
		Content:         []byte(`{"type":"doc","content":[]}`),
		Sources:         []byte(`[{"type":"document","label":"Refund policy"}]`),
		CreatedBy:       "agent-1",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	proposal, err := svc.Get(ctx, "ws-1", "doc-1", "proposal-1")
	if err != nil {
		t.Fatalf("get proposal: %v", err)
	}
	if proposal == nil {
		t.Fatal("got nil proposal")
	}
	if proposal.Status != model.DocsChangeProposalStatusApplied {
		t.Fatalf("Status = %q, want applied", proposal.Status)
	}
	var sources []model.DocsChangeProposalSource
	if err := json.Unmarshal(proposal.Sources, &sources); err != nil {
		t.Fatalf("unmarshal sanitized sources: %v", err)
	}
	if len(sources) != 1 || sources[0].Type != "document" || sources[0].Label != "Refund policy" {
		t.Fatalf("sources = %#v", sources)
	}
}

func TestDocsChangeProposalServiceGetReturnsPendingAppliedAndDiscardedProposals(t *testing.T) {
	ctx := context.Background()
	db := setupDocsChangeProposalServiceTestDB(t)
	docRepo := repository.NewDocsDocumentRepository(db)
	proposalRepo := repository.NewDocsChangeProposalRepository(db)
	svc := NewDocsChangeProposalService(proposalRepo, docRepo, nil, nil, nil, nil)
	now := time.Now().UTC()

	if err := db.WithContext(ctx).Exec(
		`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"doc-1", "ws-1", "space-1", "Billing", model.DocStatusPublished, model.SpaceVisibilityWorkspaceWide, "user-1", now, now,
	).Error; err != nil {
		t.Fatalf("seed doc: %v", err)
	}
	for _, status := range []string{
		model.DocsChangeProposalStatusPending,
		model.DocsChangeProposalStatusApplied,
		model.DocsChangeProposalStatusDiscarded,
	} {
		if _, err := proposalRepo.Create(ctx, &model.DocsChangeProposal{
			ID:              "proposal-" + status,
			WorkspaceID:     "ws-1",
			DocumentID:      "doc-1",
			Scope:           "document",
			Status:          status,
			Summary:         "Update docs",
			ContentMarkdown: "New docs",
			Content:         []byte(`{"type":"doc","content":[]}`),
			Sources:         []byte(`[]`),
			CreatedBy:       "agent-1",
			CreatedAt:       now,
			UpdatedAt:       now,
		}); err != nil {
			t.Fatalf("seed proposal %s: %v", status, err)
		}
		got, err := svc.Get(ctx, "ws-1", "doc-1", "proposal-"+status)
		if err != nil {
			t.Fatalf("get proposal %s: %v", status, err)
		}
		if got.Status != status {
			t.Fatalf("Status = %q, want %q", got.Status, status)
		}
	}
}

type recordingProposalVersionSnapshotter struct {
	versionType string
	proposalID  string
}

func (r *recordingProposalVersionSnapshotter) SnapshotOnProposalApply(_ context.Context, _ string, _ string, proposal *model.DocsChangeProposal) (*model.DocsVersion, error) {
	r.proposalID = proposal.ID
	r.versionType = model.VersionTypeProposalApply
	return &model.DocsVersion{VersionType: model.VersionTypeProposalApply}, nil
}

func TestDocsChangeProposalServiceCreateCapturesBaseMarkdown(t *testing.T) {
	ctx := context.Background()
	db := setupDocsChangeProposalServiceTestDB(t)
	docRepo := repository.NewDocsDocumentRepository(db)
	proposalRepo := repository.NewDocsChangeProposalRepository(db)
	contentSvc := NewDocsContentService(repository.NewDocsContentRepository(db), docRepo, nil)
	svc := NewDocsChangeProposalService(proposalRepo, docRepo, contentSvc, nil, nil, nil)
	now := time.Now().UTC()

	if err := db.WithContext(ctx).Exec(
		`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"doc-1", "ws-1", "space-1", "Billing", model.DocStatusPublished, model.SpaceVisibilityWorkspaceWide, "user-1", now, now,
	).Error; err != nil {
		t.Fatalf("seed doc: %v", err)
	}
	if err := db.WithContext(ctx).Exec(
		`INSERT INTO docs_contents (id, document_id, content, content_text, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"content-1", "doc-1",
		[]byte(`{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Refunds"}]},{"type":"paragraph","content":[{"type":"text","text":"Refunds take 5 days."}]}]}`),
		"Refunds\nRefunds take 5 days.", now, now,
	).Error; err != nil {
		t.Fatalf("seed content: %v", err)
	}

	created, err := svc.Create(ctx, "ws-1", model.CreateDocsChangeProposalRequest{
		Scope:           "document",
		DocumentID:      "doc-1",
		Summary:         "Update refund policy",
		ContentMarkdown: "## Refunds\n\nRefunds take 10 days.",
		Content:         []byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Refunds take 10 days."}]}]}`),
		CreatedBy:       "agent-1",
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	if !strings.Contains(created.BaseMarkdown, "## Refunds") || !strings.Contains(created.BaseMarkdown, "Refunds take 5 days.") {
		t.Fatalf("BaseMarkdown = %q, want current content rendered as markdown", created.BaseMarkdown)
	}
	var updated model.DocsDocument
	if err := db.WithContext(ctx).Where("id = ?", "doc-1").First(&updated).Error; err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if !updated.UpdatedAt.After(now) {
		t.Fatalf("document updated_at = %s, want it refreshed after proposal creation at %s", updated.UpdatedAt, now)
	}
}

func TestDocsChangeProposalServiceApplyRecordsProposalApplyVersionSnapshot(t *testing.T) {
	ctx := context.Background()
	db := setupDocsChangeProposalServiceTestDB(t)
	docRepo := repository.NewDocsDocumentRepository(db)
	proposalRepo := repository.NewDocsChangeProposalRepository(db)
	contentSvc := NewDocsContentService(repository.NewDocsContentRepository(db), docRepo, nil)
	versionSnapshotter := &recordingProposalVersionSnapshotter{}
	svc := NewDocsChangeProposalService(proposalRepo, docRepo, contentSvc, nil, versionSnapshotter, nil)
	now := time.Now().UTC()

	if err := db.WithContext(ctx).Exec(
		`INSERT INTO docs_documents (id, workspace_id, space_id, title, status, visibility, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"doc-1", "ws-1", "space-1", "Billing", model.DocStatusPublished, model.SpaceVisibilityWorkspaceWide, "user-1", now, now,
	).Error; err != nil {
		t.Fatalf("seed doc: %v", err)
	}
	if _, err := proposalRepo.Create(ctx, &model.DocsChangeProposal{
		ID:              "proposal-1",
		WorkspaceID:     "ws-1",
		DocumentID:      "doc-1",
		Scope:           "document",
		Status:          model.DocsChangeProposalStatusPending,
		Summary:         "Update docs",
		ContentMarkdown: "New docs",
		Content:         []byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"New docs"}]}]}`),
		Sources:         []byte(`[]`),
		CreatedBy:       "agent-1",
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	proposal, _, err := svc.Apply(ctx, "ws-1", "doc-1", "proposal-1", "user-1")
	if err != nil {
		t.Fatalf("apply proposal: %v", err)
	}
	if proposal.Status != model.DocsChangeProposalStatusApplied {
		t.Fatalf("Status = %q, want applied", proposal.Status)
	}
	if versionSnapshotter.proposalID != "proposal-1" {
		t.Fatalf("snapshot proposalID = %q, want proposal-1", versionSnapshotter.proposalID)
	}
	if versionSnapshotter.versionType != model.VersionTypeProposalApply {
		t.Fatalf("snapshot versionType = %q, want %q", versionSnapshotter.versionType, model.VersionTypeProposalApply)
	}
}
