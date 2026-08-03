package service

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

type contentSourceDeleteStarter struct {
	cancel func(ctx context.Context, workspaceID, contentSourceID string) error
}

func (s *contentSourceDeleteStarter) QueueContentSourceSync(context.Context, string, string) error {
	return nil
}

func (s *contentSourceDeleteStarter) QueueContentSourceReindex(context.Context, string, string) error {
	return nil
}

func (s *contentSourceDeleteStarter) CancelContentSourceSync(ctx context.Context, workspaceID, contentSourceID string) error {
	if s.cancel == nil {
		return nil
	}
	return s.cancel(ctx, workspaceID, contentSourceID)
}

func TestDeleteContentSourceCancelsWorkflowBeforeRemovingIndexedData(t *testing.T) {
	db := newContentSourceDeleteTestDB(t)
	seedContentSourceDeleteRows(t, db)

	cancelCalled := false
	starter := &contentSourceDeleteStarter{cancel: func(_ context.Context, workspaceID, contentSourceID string) error {
		cancelCalled = true
		if workspaceID != "ws-1" || contentSourceID != "source-1" {
			t.Fatalf("unexpected cancellation target: %s/%s", workspaceID, contentSourceID)
		}
		for _, table := range []string{"support_content_sources", "agent_content_sources", "support_content_pages", "support_content_chunks"} {
			if got := countContentSourceDeleteRows(t, db, table); got != 1 {
				t.Fatalf("%s rows before cancellation = %d, want 1", table, got)
			}
		}
		return nil
	}}
	service := newContentSourceDeleteService(db, starter)

	if err := service.Delete(context.Background(), "ws-1", "source-1"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if !cancelCalled {
		t.Fatal("Delete() did not cancel the content source workflow")
	}
	for _, table := range []string{"support_content_sources", "agent_content_sources", "support_content_pages", "support_content_chunks"} {
		if got := countContentSourceDeleteRows(t, db, table); got != 0 {
			t.Fatalf("%s rows after deletion = %d, want 0", table, got)
		}
	}
}

func TestDeleteContentSourceKeepsDataWhenWorkflowCancellationFails(t *testing.T) {
	db := newContentSourceDeleteTestDB(t)
	seedContentSourceDeleteRows(t, db)

	cancelErr := errors.New("Temporal unavailable")
	starter := &contentSourceDeleteStarter{cancel: func(context.Context, string, string) error {
		return cancelErr
	}}
	service := newContentSourceDeleteService(db, starter)

	err := service.Delete(context.Background(), "ws-1", "source-1")
	if !errors.Is(err, cancelErr) {
		t.Fatalf("Delete() should return the cancellation error, got %v", err)
	}
	for _, table := range []string{"support_content_sources", "agent_content_sources", "support_content_pages", "support_content_chunks"} {
		if got := countContentSourceDeleteRows(t, db, table); got != 1 {
			t.Fatalf("%s rows after failed cancellation = %d, want 1", table, got)
		}
	}
}

func newContentSourceDeleteService(db *gorm.DB, starter SupportContentSyncWorkflowStarter) *SupportContentSourceService {
	sourceRepo := repository.NewSupportContentSourceRepository(db)
	pageRepo := repository.NewSupportContentPageRepository(db)
	chunkRepo := repository.NewSupportContentChunkRepository(db)
	linkRepo := repository.NewAgentContentSourceRepository(db)
	syncService := &SupportContentSyncService{starter: starter}
	return NewSupportContentSourceService(sourceRepo, nil, linkRepo, pageRepo, chunkRepo, syncService)
}

func newContentSourceDeleteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newTestDB(t)
	for _, ddl := range []string{
		`CREATE TABLE support_content_sources (id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, start_url TEXT)`,
		`CREATE TABLE agent_content_sources (id TEXT PRIMARY KEY, agent_id TEXT, content_source_id TEXT, workspace_id TEXT)`,
		`CREATE TABLE support_content_pages (id TEXT PRIMARY KEY, workspace_id TEXT, content_source_id TEXT, url TEXT)`,
		`CREATE TABLE support_content_chunks (id TEXT PRIMARY KEY, workspace_id TEXT, content_source_id TEXT, page_id TEXT)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("create content source delete table: %v", err)
		}
	}
	return db
}

func seedContentSourceDeleteRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, statement := range []string{
		`INSERT INTO support_content_sources (id, workspace_id, name, start_url) VALUES ('source-1', 'ws-1', 'Website', 'https://example.test')`,
		`INSERT INTO agent_content_sources (id, agent_id, content_source_id, workspace_id) VALUES ('link-1', 'agent-1', 'source-1', 'ws-1')`,
		`INSERT INTO support_content_pages (id, workspace_id, content_source_id, url) VALUES ('page-1', 'ws-1', 'source-1', 'https://example.test')`,
		`INSERT INTO support_content_chunks (id, workspace_id, content_source_id, page_id) VALUES ('chunk-1', 'ws-1', 'source-1', 'page-1')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed content source delete row: %v", err)
		}
	}
}

func countContentSourceDeleteRows(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var count int64
	if err := db.Table(table).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}
