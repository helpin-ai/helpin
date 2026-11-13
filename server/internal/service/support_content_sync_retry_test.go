package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

type retryingSyncCrawler struct {
	failures int
	crawler  *syncReuseCrawler
	empty    bool
}

func (c *retryingSyncCrawler) Crawl(ctx context.Context, source model.SupportContentSource, onPage func(crawler.CrawlRecord) error) (int, error) {
	if c.empty {
		return 0, nil
	}
	if c.failures > 0 {
		c.failures--
		return 0, context.DeadlineExceeded
	}
	return c.crawler.Crawl(ctx, source, onPage)
}

func TestEmptyWebsiteRefreshPreservesKnowledge(t *testing.T) {
	db := newSyncReuseDB(t)
	repo := repository.NewSupportContentSourceRepository(db)
	ctx := context.Background()
	if err := repo.Create(ctx, &model.SupportContentSource{ID: "site", WorkspaceID: "ws", Name: "Guide", SourceType: "website", StartURL: "https://example.test/docs"}); err != nil {
		t.Fatal(err)
	}
	crawl := &retryingSyncCrawler{crawler: &syncReuseCrawler{text: "# Guide\nConnect your account."}}
	svc := NewSupportContentSyncService(repo, repository.NewSupportContentPageRepository(db), repository.NewSupportContentChunkRepository(db), &syncReuseEmbedder{}, "", crawl, nil, nil)
	if err := svc.RunSourceSync(ctx, "ws", "site"); err != nil {
		t.Fatal(err)
	}
	crawl.empty = true
	if err := svc.RunSourceSync(ctx, "ws", "site"); err == nil {
		t.Fatal("empty crawl reported success")
	}
	source, err := repo.GetByID(ctx, "site")
	if err != nil {
		t.Fatal(err)
	}
	if source.SyncStatus != "failed" || source.IndexedPages != 1 || source.IndexedChunks < 1 {
		t.Fatalf("incorrect empty refresh state: %+v", source)
	}
	for _, table := range []string{"support_content_pages", "support_content_chunks"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count == 0 {
			t.Fatalf("lost indexed knowledge in %s: count=%d err=%v", table, count, err)
		}
	}
}

func TestContentSyncRetryStateAndPreservedKnowledge(t *testing.T) {
	for _, tc := range []struct {
		name     string
		failures int
		states   []string
	}{
		{"recovers", 1, []string{"queued", "ready"}},
		{"exhausted", 3, []string{"queued", "queued", "failed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newSyncReuseDB(t)
			repo := repository.NewSupportContentSourceRepository(db)
			source := &model.SupportContentSource{ID: "site", WorkspaceID: "ws", Name: "Guide", SourceType: "website", StartURL: "https://example.test/docs"}
			if err := repo.Create(context.Background(), source); err != nil {
				t.Fatal(err)
			}
			crawl := &retryingSyncCrawler{crawler: &syncReuseCrawler{text: "# Guide\nConnect your account."}}
			svc := NewSupportContentSyncService(repo, repository.NewSupportContentPageRepository(db), repository.NewSupportContentChunkRepository(db), &syncReuseEmbedder{}, "", crawl, nil, nil)
			if err := svc.RunSourceSync(context.Background(), "ws", "site"); err != nil {
				t.Fatal(err)
			}
			crawl.failures = tc.failures
			var states []string
			suite := testsuite.WorkflowTestSuite{}
			env := suite.NewTestWorkflowEnvironment()
			env.RegisterActivityWithOptions(func(ctx context.Context, input temporalapp.ContentSourceSyncInput) error {
				err := svc.RunSourceSync(ctx, input.WorkspaceID, input.ContentSourceID)
				state, readErr := repo.GetByID(ctx, "site")
				if readErr != nil {
					return readErr
				}
				states = append(states, state.SyncStatus)
				if state.SyncStatus == "queued" && (state.LastSyncError != nil || state.LastSyncCompletedAt != nil) {
					t.Error("pending retry was exposed as a completed failure")
				}
				if state.IndexedPages != 1 || state.IndexedChunks < 1 {
					t.Error("retry lost indexed counts")
				}
				return err
			}, activity.RegisterOptions{Name: "ContentSourceSyncActivities.SyncContentSourceActivity"})
			env.ExecuteWorkflow(temporalapp.ContentSourceSyncWorkflow, temporalapp.ContentSourceSyncInput{WorkspaceID: "ws", ContentSourceID: "site"})
			if !reflect.DeepEqual(states, tc.states) {
				t.Fatalf("states=%v want %v", states, tc.states)
			}
			if (env.GetWorkflowError() != nil) != (tc.failures == 3) {
				t.Fatalf("workflow error=%v", env.GetWorkflowError())
			}
			for _, table := range []string{"support_content_pages", "support_content_chunks"} {
				var count int64
				if err := db.Table(table).Count(&count).Error; err != nil || count == 0 {
					t.Fatalf("lost indexed knowledge in %s: count=%d err=%v", table, count, err)
				}
			}
		})
	}
}
