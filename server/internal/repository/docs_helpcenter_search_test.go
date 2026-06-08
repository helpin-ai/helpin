package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDocsHelpcenterSearchRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-helpcenter-search-repo-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE docs_helpcenter_search_entries (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			locale TEXT NOT NULL,
			entry_key TEXT NOT NULL,
			entry_type TEXT NOT NULL,
			content TEXT NOT NULL,
			section_title TEXT,
			anchor TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			rank_weight REAL NOT NULL DEFAULT 1,
			search_config TEXT NOT NULL DEFAULT 'simple',
			search_vector TEXT NOT NULL DEFAULT '',
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create search entries table: %v", err)
	}
	return db
}

func TestDocsHelpcenterSearchRepository_ReplaceArticleEntriesRemovesStaleLocaleEntries(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterSearchRepoTestDB(t)
	repo := NewDocsHelpcenterSearchRepository(db)
	ctx := context.Background()

	initial := []model.DocsHelpcenterSearchEntry{
		{WorkspaceID: "ws-1", DocumentID: "doc-1", Locale: "en", EntryKey: "title", EntryType: model.DocsHelpcenterSearchEntryTypeTitle, Content: "Old title", RankWeight: 8, SearchConfig: "simple"},
		{WorkspaceID: "ws-1", DocumentID: "doc-1", Locale: "en", EntryKey: "heading:old", EntryType: model.DocsHelpcenterSearchEntryTypeHeading, Content: "Old heading", RankWeight: 6, SearchConfig: "simple"},
	}
	if err := repo.ReplaceArticleEntries(ctx, "doc-1", "en", initial); err != nil {
		t.Fatalf("replace initial entries: %v", err)
	}

	next := []model.DocsHelpcenterSearchEntry{
		{WorkspaceID: "ws-1", DocumentID: "doc-1", Locale: "en", EntryKey: "title", EntryType: model.DocsHelpcenterSearchEntryTypeTitle, Content: "New title", RankWeight: 8, SearchConfig: "simple"},
	}
	if err := repo.ReplaceArticleEntries(ctx, "doc-1", "en", next); err != nil {
		t.Fatalf("replace next entries: %v", err)
	}

	var entries []model.DocsHelpcenterSearchEntry
	if err := db.Order("entry_key").Find(&entries).Error; err != nil {
		t.Fatalf("load entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1: %+v", len(entries), entries)
	}
	if entries[0].Content != "New title" {
		t.Fatalf("entry content = %q, want New title", entries[0].Content)
	}
}
