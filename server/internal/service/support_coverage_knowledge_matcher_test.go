package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCoverageKnowledgeMatcherTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:coverage_knowledge_matcher_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	tables := []string{
		`CREATE TABLE docs_spaces (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_articles (
			document_id TEXT PRIMARY KEY,
			public_published_at DATETIME
		)`,
		`CREATE TABLE docs_chunks (
			id TEXT PRIMARY KEY,
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
			content_hash TEXT NOT NULL DEFAULT '',
			embedding TEXT NOT NULL DEFAULT '',
			embedding_provider TEXT NOT NULL DEFAULT 'openai',
			embedding_model TEXT NOT NULL DEFAULT 'text-embedding-3-small',
			embedding_version TEXT NOT NULL DEFAULT 'content-chunk-v1',
			embedding_dimensions INTEGER NOT NULL DEFAULT 1536,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_content_chunks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			content_source_id TEXT NOT NULL,
			page_id TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			section_key TEXT NOT NULL DEFAULT '',
			heading_path TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			url TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			search_content TEXT NOT NULL DEFAULT '',
			previous_chunk_index INTEGER,
			next_chunk_index INTEGER,
			content_hash TEXT NOT NULL DEFAULT '',
			embedding TEXT NOT NULL DEFAULT '',
			embedding_provider TEXT NOT NULL DEFAULT 'openai',
			embedding_model TEXT NOT NULL DEFAULT 'text-embedding-3-small',
			embedding_version TEXT NOT NULL DEFAULT 'content-chunk-v1',
			embedding_dimensions INTEGER NOT NULL DEFAULT 1536,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE support_content_sources (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			sync_status TEXT NOT NULL
		)`,
		`CREATE TABLE support_content_pages (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			content_source_id TEXT NOT NULL,
			http_status INTEGER NOT NULL
		)`,
	}
	for _, stmt := range tables {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO docs_spaces (id, type) VALUES (?, ?)`, "space-public", model.SpaceTypeInternal).Error; err != nil {
		t.Fatalf("seed docs space: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_documents (id, status) VALUES (?, ?)`, "doc-1", model.DocStatusPublished).Error; err != nil {
		t.Fatalf("seed docs document: %v", err)
	}
	if err := db.Exec(`INSERT INTO support_content_sources (id, workspace_id, sync_status) VALUES (?, ?, ?)`, "source-website", "ws-1", model.KnowledgeSourceSyncReady).Error; err != nil {
		t.Fatalf("seed content source: %v", err)
	}
	if err := db.Exec(`INSERT INTO support_content_pages (id, workspace_id, content_source_id, http_status) VALUES (?, ?, ?, ?)`, "page-1", "ws-1", "source-website", 200).Error; err != nil {
		t.Fatalf("seed content page: %v", err)
	}
	return db
}

func TestCoverageKnowledgeMatcherSearchesDocsAndWebsiteContent(t *testing.T) {
	db := setupCoverageKnowledgeMatcherTestDB(t)
	now := time.Now()
	if err := db.Create(&model.DocsChunk{ID: "doc-chunk-1", WorkspaceID: "ws-1", SpaceID: "space-public", DocumentID: "doc-1", ChunkIndex: 0, Title: "Refund docs", Content: "Refund policy for customers", UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed docs chunk: %v", err)
	}
	if err := db.Create(&model.SupportContentChunk{ID: "content-chunk-1", WorkspaceID: "ws-1", ContentSourceID: "source-website", PageID: "page-1", ChunkIndex: 0, Title: "Pricing page", URL: "https://example.com/pricing", Content: "Refunds for prospects", UpdatedAt: now.Add(time.Minute)}).Error; err != nil {
		t.Fatalf("seed content chunk: %v", err)
	}
	matcher := NewCoverageKnowledgeMatcher(
		repository.NewDocsChunkRepository(db),
		repository.NewSupportContentChunkRepository(db),
		nil,
		"",
	)

	candidates, err := matcher.MatchKnowledge(context.Background(), "ws-1", []string{"space-public"}, []string{"source-website"}, "refund policy", 10)
	if err != nil {
		t.Fatalf("MatchKnowledge: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected docs and website candidates, got %d: %+v", len(candidates), candidates)
	}
	byTarget := map[string]CoverageKnowledgeCandidate{}
	for _, candidate := range candidates {
		byTarget[candidate.TargetType] = candidate
	}
	if byTarget["docs"].DocumentID != "doc-1" || byTarget["docs"].SourceType != "docs" {
		t.Fatalf("expected docs candidate, got %+v", byTarget["docs"])
	}
	if byTarget["website_page"].PageID != "page-1" || byTarget["website_page"].SourceType != "website" {
		t.Fatalf("expected website candidate, got %+v", byTarget["website_page"])
	}
}

func TestCoverageKnowledgeMatcherReturnsEmptyWithoutKnowledgeSources(t *testing.T) {
	db := setupCoverageKnowledgeMatcherTestDB(t)
	matcher := NewCoverageKnowledgeMatcher(
		repository.NewDocsChunkRepository(db),
		repository.NewSupportContentChunkRepository(db),
		nil,
		"",
	)

	candidates, err := matcher.MatchKnowledge(context.Background(), "ws-1", nil, nil, "refund policy", 10)
	if err != nil {
		t.Fatalf("MatchKnowledge: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates, got %+v", candidates)
	}
}

func TestCoverageKnowledgeMatcherDedupesChunksFromSameTarget(t *testing.T) {
	db := setupCoverageKnowledgeMatcherTestDB(t)
	now := time.Now()
	chunks := []model.DocsChunk{
		{ID: "doc-chunk-1", WorkspaceID: "ws-1", SpaceID: "space-public", DocumentID: "doc-1", ChunkIndex: 0, Title: "Refund docs", Content: "First chunk", UpdatedAt: now},
		{ID: "doc-chunk-2", WorkspaceID: "ws-1", SpaceID: "space-public", DocumentID: "doc-1", ChunkIndex: 1, Title: "Refund docs", Content: "Second chunk", UpdatedAt: now.Add(time.Minute)},
	}
	if err := db.Create(&chunks).Error; err != nil {
		t.Fatalf("seed docs chunks: %v", err)
	}
	matcher := NewCoverageKnowledgeMatcher(repository.NewDocsChunkRepository(db), nil, nil, "")

	candidates, err := matcher.MatchKnowledge(context.Background(), "ws-1", []string{"space-public"}, nil, "refund policy", 10)
	if err != nil {
		t.Fatalf("MatchKnowledge: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected one deduped candidate, got %+v", candidates)
	}
	if candidates[0].DocumentID != "doc-1" {
		t.Fatalf("expected doc-1, got %+v", candidates[0])
	}
}

func TestCoverageKnowledgeMatcherFallsBackToLexicalWhenEmbeddingProviderNil(t *testing.T) {
	db := setupCoverageKnowledgeMatcherTestDB(t)
	if err := db.Create(&model.SupportContentChunk{ID: "content-chunk-1", WorkspaceID: "ws-1", ContentSourceID: "source-website", PageID: "page-1", ChunkIndex: 0, Title: "Security", URL: "https://example.com/security", Content: "Security content"}).Error; err != nil {
		t.Fatalf("seed content chunk: %v", err)
	}
	matcher := NewCoverageKnowledgeMatcher(nil, repository.NewSupportContentChunkRepository(db), nil, "")

	candidates, err := matcher.MatchKnowledge(context.Background(), "ws-1", nil, []string{"source-website"}, "security", 10)
	if err != nil {
		t.Fatalf("MatchKnowledge: %v", err)
	}
	if len(candidates) != 1 || candidates[0].PageID != "page-1" {
		t.Fatalf("expected lexical content candidate, got %+v", candidates)
	}
}
