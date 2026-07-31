package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type internalKnowledgeEmbeddingProvider struct{}

func (internalKnowledgeEmbeddingProvider) CreateEmbeddings(_ context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	vectors := make([][]float32, len(req.Inputs))
	for idx := range req.Inputs {
		vectors[idx] = make([]float32, docsEmbeddingDimensions)
		vectors[idx][0] = 1
	}
	return &llm.EmbeddingResponse{Vectors: vectors}, nil
}

func TestAgentKnowledgeSourceServiceAllowsInternalAndHelpCenterSpaces(t *testing.T) {
	db := newInternalKnowledgeTestDB(t)
	ctx := context.Background()

	insertKnowledgeSpace(t, db, "ws-1", "internal-space", model.SpaceTypeInternal)
	insertKnowledgeSpace(t, db, "ws-1", "public-space", model.SpaceTypeExternalCapable)
	insertKnowledgeSpace(t, db, "ws-1", "unsupported-space", "private_external")

	knowledgeRepo := repository.NewAgentKnowledgeSourceRepository(db)
	service := NewAgentKnowledgeSourceService(
		knowledgeRepo,
		repository.NewDocsSpaceRepository(db),
		nil,
	)

	if err := service.Set(ctx, "ws-1", "agent-1", []string{"internal-space", "public-space"}); err != nil {
		t.Fatalf("Set returned error for supported spaces: %v", err)
	}

	sources, err := service.List(ctx, "ws-1", "agent-1")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("List returned %d sources, want 2: %+v", len(sources), sources)
	}

	typesBySpaceID := make(map[string]string, len(sources))
	for _, source := range sources {
		typesBySpaceID[source.SpaceID] = source.SpaceType
	}
	if typesBySpaceID["internal-space"] != model.SpaceTypeInternal {
		t.Fatalf("internal source type = %q, want %q", typesBySpaceID["internal-space"], model.SpaceTypeInternal)
	}
	if typesBySpaceID["public-space"] != model.SpaceTypeExternalCapable {
		t.Fatalf("public source type = %q, want %q", typesBySpaceID["public-space"], model.SpaceTypeExternalCapable)
	}

	if err := service.Set(ctx, "ws-1", "agent-1", []string{"unsupported-space"}); err == nil {
		t.Fatal("Set accepted an unsupported docs space type")
	}
}

func TestDocsEmbeddingServiceIndexesPublishedInternalDocs(t *testing.T) {
	db := newInternalKnowledgeTestDB(t)
	ctx := context.Background()

	insertKnowledgeSpace(t, db, "ws-1", "internal-space", model.SpaceTypeInternal)
	if err := db.Exec(`
		INSERT INTO agent_knowledge_sources
			(id, agent_id, space_id, workspace_id, sync_status, sync_progress, indexed_documents, indexed_chunks, created_at, updated_at)
		VALUES
			('source-1', 'agent-1', 'internal-space', 'ws-1', 'queued', 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`).Error; err != nil {
		t.Fatalf("insert knowledge source: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO docs_documents (id, workspace_id, space_id, title, status, updated_at)
		VALUES
			('published-doc', 'ws-1', 'internal-space', 'Enterprise guidance', 'published', CURRENT_TIMESTAMP),
			('draft-doc', 'ws-1', 'internal-space', 'Unapproved guidance', 'draft', CURRENT_TIMESTAMP)
	`).Error; err != nil {
		t.Fatalf("insert docs: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO docs_contents (id, document_id, content, content_text, word_count, created_at, updated_at)
		VALUES
			('content-1', 'published-doc', X'7B7D', 'Enterprise plans include SAML SSO and audit logs.', 8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			('content-2', 'draft-doc', X'7B7D', 'This draft must not be indexed.', 6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`).Error; err != nil {
		t.Fatalf("insert docs content: %v", err)
	}

	chunkRepo := repository.NewDocsChunkRepository(db)
	knowledgeRepo := repository.NewAgentKnowledgeSourceRepository(db)
	embeddingService := NewDocsEmbeddingService(
		chunkRepo,
		nil,
		knowledgeRepo,
		repository.NewDocsContentRepository(db),
		repository.NewDocsSpaceRepository(db),
		nil,
		repository.NewDocsDocumentRepository(db),
		internalKnowledgeEmbeddingProvider{},
		"",
		nil,
	)

	if err := embeddingService.RunSpaceSync(ctx, "ws-1", "internal-space"); err != nil {
		t.Fatalf("RunSpaceSync returned error: %v", err)
	}

	var chunks []model.DocsChunk
	if err := db.Order("document_id ASC").Find(&chunks).Error; err != nil {
		t.Fatalf("list docs chunks: %v", err)
	}
	if len(chunks) != 1 || chunks[0].DocumentID != "published-doc" {
		t.Fatalf("indexed chunks = %+v, want one chunk for published-doc", chunks)
	}

	results, err := chunkRepo.HybridSearch(ctx, "ws-1", []string{"internal-space"}, "enterprise", "", 5)
	if err != nil {
		t.Fatalf("HybridSearch returned error: %v", err)
	}
	if len(results) != 1 || results[0].SpaceType != model.SpaceTypeInternal {
		t.Fatalf("internal search results = %+v, want one internal result", results)
	}

	source, err := knowledgeRepo.GetByID(ctx, "source-1")
	if err != nil {
		t.Fatalf("get knowledge source: %v", err)
	}
	if source == nil || source.SyncStatus != model.KnowledgeSourceSyncReady || source.IndexedDocuments != 1 {
		t.Fatalf("knowledge source sync state = %+v, want ready with one indexed document", source)
	}
}

func newInternalKnowledgeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:internal-knowledge-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	statements := []string{
		`CREATE TABLE docs_spaces (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			icon TEXT,
			visibility TEXT NOT NULL DEFAULT 'workspace_wide',
			type TEXT NOT NULL,
			default_review_days INTEGER,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE agent_knowledge_sources (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			agent_id TEXT NOT NULL,
			scope_type TEXT NOT NULL DEFAULT 'space',
			space_id TEXT NOT NULL,
			collection_id TEXT,
			document_id TEXT,
			workspace_id TEXT NOT NULL,
			sync_status TEXT NOT NULL DEFAULT 'queued',
			sync_progress INTEGER NOT NULL DEFAULT 0,
			indexed_documents INTEGER NOT NULL DEFAULT 0,
			indexed_chunks INTEGER NOT NULL DEFAULT 0,
			last_sync_error TEXT,
			last_sync_started_at DATETIME,
			last_sync_completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(agent_id, scope_type, space_id, collection_id, document_id)
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			deleted_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_contents (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE,
			content BLOB,
			content_text TEXT NOT NULL,
			word_count INTEGER NOT NULL DEFAULT 0,
			import_source_html TEXT,
			import_source_system TEXT,
			import_source_object_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_articles (
			document_id TEXT PRIMARY KEY,
			public_published_at DATETIME
		)`,
		`CREATE TABLE docs_chunks (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			block_id TEXT,
			chunk_index INTEGER NOT NULL,
			section_key TEXT NOT NULL DEFAULT '',
			heading_path TEXT NOT NULL DEFAULT '',
			block_range TEXT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			search_content TEXT NOT NULL DEFAULT '',
			previous_chunk_index INTEGER,
			next_chunk_index INTEGER,
			content_hash TEXT NOT NULL,
			embedding TEXT NOT NULL,
			embedding_provider TEXT NOT NULL,
			embedding_model TEXT NOT NULL,
			embedding_version TEXT NOT NULL,
			embedding_dimensions INTEGER NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(document_id, chunk_index)
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create internal knowledge test schema: %v", err)
		}
	}
	return db
}

func insertKnowledgeSpace(t *testing.T, db *gorm.DB, workspaceID, spaceID, spaceType string) {
	t.Helper()
	if err := db.Exec(`
		INSERT INTO docs_spaces (id, workspace_id, name, slug, type, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'user-1', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, spaceID, workspaceID, spaceID, spaceID, spaceType).Error; err != nil {
		t.Fatalf("insert docs space %q: %v", spaceID, err)
	}
}
