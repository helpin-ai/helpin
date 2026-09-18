package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type robotsSyncCrawler struct{ err error }

func (c robotsSyncCrawler) Crawl(ctx context.Context, source model.SupportContentSource, onPage func(crawler.CrawlRecord) error) (int, error) {
	if err := onPage(crawler.CrawlRecord{URL: source.StartURL, SkipReason: "robots_disallowed"}); err != nil {
		return 0, err
	}
	return 0, c.err
}

func TestContentSyncRobotsSkipsAndOutage(t *testing.T) {
	for _, outage := range []bool{false, true} {
		name := "disallowed"
		if outage {
			name = "unavailable"
		}
		t.Run(name, func(t *testing.T) {
			db := newContentSourceDeleteTestDB(t)
			seedContentSourceDeleteRows(t, db)
			for _, column := range []string{"sync_status TEXT", "sync_progress INTEGER", "indexed_pages INTEGER DEFAULT 1", "indexed_chunks INTEGER DEFAULT 1", "last_sync_error TEXT", "last_sync_warning TEXT", "last_crawl_job_id TEXT", "last_sync_started_at DATETIME", "last_sync_completed_at DATETIME", "updated_at DATETIME"} {
				if err := db.Exec("ALTER TABLE support_content_sources ADD COLUMN " + column).Error; err != nil {
					t.Fatal(err)
				}
			}
			contentCrawler := robotsSyncCrawler{}
			if outage {
				contentCrawler.err = errors.New("robots.txt unavailable")
			}
			repo := repository.NewSupportContentSourceRepository(db)
			service := NewSupportContentSyncService(repo, repository.NewSupportContentPageRepository(db), repository.NewSupportContentChunkRepository(db), internalKnowledgeEmbeddingProvider{}, "", contentCrawler, nil, nil)
			err := service.RunSourceSync(context.Background(), "ws-1", "source-1")
			if (err != nil) != outage {
				t.Fatalf("sync error=%v", err)
			}
			source, err := repo.GetByID(context.Background(), "source-1")
			if err != nil {
				t.Fatal(err)
			}
			if source.LastSyncWarning == nil || !strings.Contains(*source.LastSyncWarning, "1 URLs skipped") {
				t.Fatalf("warning=%v", source.LastSyncWarning)
			}
			wantRows := int64(0)
			wantStatus := model.KnowledgeSourceSyncReady
			if outage {
				wantRows = 1
				wantStatus = model.KnowledgeSourceSyncFailed
			}
			if source.SyncStatus != wantStatus {
				t.Fatalf("status=%s", source.SyncStatus)
			}
			for _, table := range []string{"support_content_pages", "support_content_chunks"} {
				if got := countContentSourceDeleteRows(t, db, table); got != wantRows {
					t.Fatalf("%s rows=%d want=%d", table, got, wantRows)
				}
			}
		})
	}
}
