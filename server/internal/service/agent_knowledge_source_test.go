package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestAgentKnowledgeSourceServiceSetScopedSources(t *testing.T) {
	ctx := context.Background()
	db := newAgentKnowledgeSourceTestDB(t)

	workspaceID := "workspace-1"
	agentID := "agent-1"
	spaceID := "space-1"
	collectionID := "collection-1"
	documentID := "document-1"

	if err := db.Exec(`INSERT INTO agents (id, workspace_id, name) VALUES (?, ?, ?)`, agentID, workspaceID, "Support Agent").Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_spaces (id, workspace_id, name, type) VALUES (?, ?, ?, ?)`, spaceID, workspaceID, "Help Center", model.SpaceTypeExternalCapable).Error; err != nil {
		t.Fatalf("create docs space: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_collections (id, workspace_id, space_id, name) VALUES (?, ?, ?, ?)`, collectionID, workspaceID, spaceID, "Getting Started").Error; err != nil {
		t.Fatalf("create docs collection: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_documents (id, workspace_id, space_id, collection_id, title, status, created_by) VALUES (?, ?, ?, ?, ?, ?, ?)`, documentID, workspaceID, spaceID, collectionID, "Install", "published", "user-1").Error; err != nil {
		t.Fatalf("create docs document: %v", err)
	}

	svc := NewAgentKnowledgeSourceService(
		repository.NewAgentKnowledgeSourceRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsDocumentRepository(db),
		nil,
	)

	err := svc.SetScoped(ctx, workspaceID, agentID, []model.KnowledgeSourceScopeRequest{
		{ScopeType: model.KnowledgeSourceScopeSpace, SpaceID: spaceID},
		{ScopeType: model.KnowledgeSourceScopeCollection, SpaceID: spaceID, CollectionID: &collectionID},
		{ScopeType: model.KnowledgeSourceScopeArticle, SpaceID: spaceID, DocumentID: &documentID},
		{ScopeType: model.KnowledgeSourceScopeArticle, SpaceID: spaceID, DocumentID: &documentID},
	})
	if err != nil {
		t.Fatalf("SetScoped returned error: %v", err)
	}

	sources, err := svc.List(ctx, workspaceID, agentID)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(sources) != 3 {
		t.Fatalf("len(sources) = %d, want 3", len(sources))
	}

	seen := map[string]bool{}
	for _, source := range sources {
		seen[source.ScopeType] = true
		if source.ScopeType == model.KnowledgeSourceScopeCollection {
			if source.CollectionID == nil || *source.CollectionID != collectionID {
				t.Fatalf("collection source collection_id = %v, want %s", source.CollectionID, collectionID)
			}
			if source.CollectionName != "Getting Started" {
				t.Fatalf("collection source name = %q, want Getting Started", source.CollectionName)
			}
		}
		if source.ScopeType == model.KnowledgeSourceScopeArticle {
			if source.DocumentID == nil || *source.DocumentID != documentID {
				t.Fatalf("article source document_id = %v, want %s", source.DocumentID, documentID)
			}
			if source.DocumentTitle != "Install" {
				t.Fatalf("article source title = %q, want Install", source.DocumentTitle)
			}
		}
	}
	for _, scopeType := range []string{model.KnowledgeSourceScopeSpace, model.KnowledgeSourceScopeCollection, model.KnowledgeSourceScopeArticle} {
		if !seen[scopeType] {
			t.Fatalf("missing scope type %s", scopeType)
		}
	}
}

func TestAgentKnowledgeSourceServiceSetScopedSourcesRollsBackOnInsertFailure(t *testing.T) {
	ctx := context.Background()
	db := newAgentKnowledgeSourceTestDB(t)

	workspaceID := "workspace-1"
	agentID := "agent-1"
	spaceID := "space-1"
	collectionID := "collection-1"

	if err := db.Exec(`INSERT INTO agents (id, workspace_id, name) VALUES (?, ?, ?)`, agentID, workspaceID, "Support Agent").Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_spaces (id, workspace_id, name, type) VALUES (?, ?, ?, ?)`, spaceID, workspaceID, "Help Center", model.SpaceTypeExternalCapable).Error; err != nil {
		t.Fatalf("create docs space: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_collections (id, workspace_id, space_id, name) VALUES (?, ?, ?, ?)`, collectionID, workspaceID, spaceID, "Getting Started").Error; err != nil {
		t.Fatalf("create docs collection: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO agent_knowledge_sources (id, agent_id, scope_type, space_id, workspace_id, sync_status)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "source-existing", agentID, model.KnowledgeSourceScopeSpace, spaceID, workspaceID, model.KnowledgeSourceSyncReady).Error; err != nil {
		t.Fatalf("create existing source: %v", err)
	}
	if err := db.Exec(`
		CREATE TRIGGER fail_collection_source_insert
		BEFORE INSERT ON agent_knowledge_sources
		WHEN NEW.scope_type = 'collection'
		BEGIN
			SELECT RAISE(ABORT, 'forced collection insert failure');
		END
	`).Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	svc := NewAgentKnowledgeSourceService(
		repository.NewAgentKnowledgeSourceRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsDocumentRepository(db),
		nil,
	)

	err := svc.SetScoped(ctx, workspaceID, agentID, []model.KnowledgeSourceScopeRequest{
		{ScopeType: model.KnowledgeSourceScopeCollection, SpaceID: spaceID, CollectionID: &collectionID},
	})
	if err == nil {
		t.Fatal("SetScoped error = nil, want forced insert error")
	}

	var remaining []model.AgentKnowledgeSource
	if err := db.Where("agent_id = ?", agentID).Find(&remaining).Error; err != nil {
		t.Fatalf("list remaining sources: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != "source-existing" || remaining[0].ScopeType != model.KnowledgeSourceScopeSpace {
		t.Fatalf("remaining sources = %+v, want original source after rollback", remaining)
	}
}

func newAgentKnowledgeSourceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE agents (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			name text NOT NULL,
			is_system boolean DEFAULT false,
			status text DEFAULT 'idle',
			runtime_kind text DEFAULT 'opencode',
			trigger_mode text DEFAULT 'manual',
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE docs_spaces (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			name text NOT NULL,
			type text NOT NULL,
			position integer DEFAULT 0,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`CREATE TABLE docs_collections (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			space_id text NOT NULL,
			parent_collection_id text,
			depth integer DEFAULT 0,
			name text NOT NULL,
			position integer DEFAULT 0,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`CREATE TABLE docs_documents (
			id text PRIMARY KEY,
			workspace_id text NOT NULL,
			space_id text NOT NULL,
			collection_id text,
			title text NOT NULL,
			status text NOT NULL,
			created_by text NOT NULL,
			position integer DEFAULT 0,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`CREATE TABLE agent_knowledge_sources (
			id text PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			agent_id text NOT NULL,
			scope_type text NOT NULL DEFAULT 'space',
			space_id text NOT NULL,
			collection_id text,
			document_id text,
			workspace_id text NOT NULL,
			sync_status text NOT NULL DEFAULT 'queued',
			sync_progress integer NOT NULL DEFAULT 0,
			indexed_documents integer NOT NULL DEFAULT 0,
			indexed_chunks integer NOT NULL DEFAULT 0,
			last_sync_error text,
			last_sync_started_at datetime,
			last_sync_completed_at datetime,
			created_at datetime,
			updated_at datetime
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test table: %v", err)
		}
	}
	return db
}
