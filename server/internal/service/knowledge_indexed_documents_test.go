package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestKnowledgeIndexedDocumentsRespectScopeAndOwnership(t *testing.T) {
	db := newAgentKnowledgeSourceTestDB(t)
	statements := []string{
		`CREATE TABLE docs_chunks (id TEXT, workspace_id TEXT, space_id TEXT, document_id TEXT)`,
		`CREATE TABLE docs_helpcenter_articles (document_id TEXT, public_published_at DATETIME)`,
		`INSERT INTO docs_spaces (id, workspace_id, name, type) VALUES ('space', 'ws', 'Guidance', 'internal')`,
		`INSERT INTO docs_spaces (id, workspace_id, name, type) VALUES ('public-space', 'ws', 'Help center', 'external_capable')`,
		`INSERT INTO docs_collections (id, workspace_id, space_id, name, parent_collection_id) VALUES
		 ('collection', 'ws', 'space', 'Billing', NULL), ('child', 'ws', 'space', 'Invoices', 'collection'), ('other', 'ws', 'space', 'Other', NULL)`,
		`INSERT INTO docs_documents (id, workspace_id, space_id, collection_id, title, status, created_by, deleted_at) VALUES
		 ('a', 'ws', 'space', 'collection', 'Address corrections', 'published', 'user', NULL),
		 ('b', 'ws', 'space', 'child', 'Billing invoices', 'published', 'user', NULL),
		 ('c', 'ws', 'space', 'other', 'Contact support', 'published', 'user', NULL),
		 ('empty', 'ws', 'space', 'collection', 'Empty article', 'published', 'user', NULL),
		 ('draft', 'ws', 'space', 'collection', 'Draft article', 'draft', 'user', NULL),
		 ('deleted', 'ws', 'space', 'collection', 'Deleted article', 'published', 'user', CURRENT_TIMESTAMP),
		 ('public', 'ws', 'public-space', NULL, 'Public article', 'published', 'user', NULL),
		 ('unreleased', 'ws', 'public-space', NULL, 'Unreleased article', 'published', 'user', NULL)`,
		`INSERT INTO docs_chunks VALUES ('a1', 'ws', 'space', 'a'), ('a2', 'ws', 'space', 'a'), ('b1', 'ws', 'space', 'b'),
		 ('c1', 'ws', 'space', 'c'), ('draft1', 'ws', 'space', 'draft'), ('deleted1', 'ws', 'space', 'deleted'), ('foreign', 'other-ws', 'space', 'empty'),
		 ('public1', 'ws', 'public-space', 'public'), ('unreleased1', 'ws', 'public-space', 'unreleased')`,
		`INSERT INTO docs_helpcenter_articles VALUES ('public', CURRENT_TIMESTAMP), ('unreleased', NULL)`,
		`INSERT INTO agent_knowledge_sources (id, workspace_id, agent_id, space_id, scope_type, collection_id, document_id) VALUES
		 ('whole', 'ws', 'agent', 'space', 'space', NULL, NULL),
		 ('collection-source', 'ws', 'agent', 'space', 'collection', 'collection', NULL),
		 ('article-source', 'ws', 'agent', 'space', 'article', NULL, 'a'),
		 ('public-source', 'ws', 'agent', 'public-space', 'space', NULL, NULL),
		 ('empty-source', 'ws', 'agent', 'space', 'article', NULL, 'empty')`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := NewAgentKnowledgeSourceServiceWithScopes(repository.NewAgentKnowledgeSourceRepository(db), repository.NewDocsSpaceRepository(db), repository.NewDocsCollectionRepository(db), repository.NewDocsDocumentRepository(db), nil)
	for _, tc := range []struct {
		name, workspace, agent, source string
		want                           []string
		wantError                      bool
	}{
		{"space", "ws", "agent", "whole", []string{"a", "b", "c"}, false},
		{"collection includes descendants", "ws", "agent", "collection-source", []string{"a", "b"}, false},
		{"single article", "ws", "agent", "article-source", []string{"a"}, false},
		{"only released public articles", "ws", "agent", "public-source", []string{"public"}, false},
		{"unindexed article", "ws", "agent", "empty-source", nil, false},
		{"other workspace", "other-ws", "agent", "whole", nil, true},
		{"other agent", "ws", "other-agent", "whole", nil, true},
		{"missing source", "ws", "agent", "missing", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			documents, err := svc.ListIndexedDocuments(context.Background(), tc.workspace, tc.agent, tc.source)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error %v", err, tc.wantError)
			}
			var ids []string
			for _, document := range documents {
				ids = append(ids, document.ID)
			}
			if !reflect.DeepEqual(ids, tc.want) {
				t.Fatalf("indexed documents = %v, want %v", ids, tc.want)
			}
		})
	}
}
