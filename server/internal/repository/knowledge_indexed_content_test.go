package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestContentSourceIndexedPagesExcludeUnindexedAndFailedPages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE support_content_pages (id TEXT, workspace_id TEXT, content_source_id TEXT, url TEXT, title TEXT, http_status INTEGER,
		 content_format TEXT, content_hash TEXT, content_text TEXT, last_crawled_at DATETIME, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE support_content_chunks (id TEXT, workspace_id TEXT, content_source_id TEXT, page_id TEXT)`,
		`INSERT INTO support_content_pages (id, workspace_id, content_source_id, title, http_status) VALUES
		 ('indexed', 'ws', 'source', 'Indexed page', 200), ('unindexed', 'ws', 'source', 'No chunks', 200),
		 ('failed', 'ws', 'source', 'Failed page', 404), ('other', 'other-ws', 'source', 'Other workspace', 200),
		 ('different', 'ws', 'different-source', 'Other source', 200)`,
		`INSERT INTO support_content_chunks VALUES ('1', 'ws', 'source', 'indexed'), ('2', 'ws', 'source', 'indexed'),
		 ('3', 'ws', 'source', 'failed'), ('4', 'other-ws', 'source', 'other'), ('5', 'ws', 'different-source', 'different'), ('6', 'other-ws', 'source', 'unindexed')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	pages, err := NewSupportContentPageRepository(db).ListIndexedByContentSourceID(context.Background(), "ws", "source")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || pages[0].ID != "indexed" {
		t.Fatalf("indexed pages = %+v, want one indexed page", pages)
	}
}
