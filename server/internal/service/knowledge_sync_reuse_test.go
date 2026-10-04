package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type syncReuseEmbedder struct{ calls int }

func (p *syncReuseEmbedder) CreateEmbeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	p.calls++
	return (internalKnowledgeEmbeddingProvider{}).CreateEmbeddings(ctx, req)
}

type syncReuseCrawler struct {
	text  string
	calls int
}

func (c *syncReuseCrawler) Crawl(_ context.Context, source model.SupportContentSource, onPage func(crawler.CrawlRecord) error) (int, error) {
	c.calls++
	err := onPage(crawler.CrawlRecord{URL: source.StartURL, Title: "Guide", HTTPStatus: 200, Markdown: c.text})
	return 1, err
}

func newSyncReuseDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newTestDB(t)
	// SQLite has no gen_random_uuid(); use equivalent fixture defaults.
	for _, entity := range []any{&model.SupportContentSource{}, &model.SupportContentPage{}, &model.SupportContentChunk{}} {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(entity); err != nil {
			t.Fatal(err)
		}
		for _, field := range stmt.Schema.Fields {
			if field.Name == "ID" {
				field.DefaultValue = "(lower(hex(randomblob(16))))"
			}
			if field.Name == "LastSyncStartedAt" || field.Name == "LastSyncCompletedAt" {
				field.DataType = "datetime"
			}
		}
		if err := db.AutoMigrate(entity); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestWebsiteSyncReusesUnchangedEmbeddingsButStillFetches(t *testing.T) {
	for _, change := range []string{"none", "content", "model", "version", "dimensions", "missing", "empty_vector", "extra_old_chunk"} {
		t.Run(change, func(t *testing.T) {
			db := newSyncReuseDB(t)
			ctx := context.Background()
			repo := repository.NewSupportContentSourceRepository(db)
			source := &model.SupportContentSource{ID: "site", WorkspaceID: "ws", Name: "Guide", SourceType: "website", StartURL: "https://example.test/docs"}
			if err := repo.Create(ctx, source); err != nil {
				t.Fatal(err)
			}
			embedder := &syncReuseEmbedder{}
			crawl := &syncReuseCrawler{text: "# Guide\nConnect your account."}
			svc := NewSupportContentSyncService(repo, repository.NewSupportContentPageRepository(db), repository.NewSupportContentChunkRepository(db), embedder, "", crawl, nil, nil)
			if err := svc.RunSourceSync(ctx, "ws", "site"); err != nil {
				t.Fatal(err)
			}
			var first model.SupportContentChunk
			if err := db.First(&first).Error; err != nil {
				t.Fatal(err)
			}
			switch change {
			case "content":
				crawl.text += "\nVerify the connection."
			case "model":
				svc.embeddingModel = "different-model"
			case "version":
				if err := db.Model(&first).Update("embedding_version", "old-version").Error; err != nil {
					t.Fatal(err)
				}
			case "dimensions":
				if err := db.Model(&first).Update("embedding_dimensions", 3).Error; err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := db.Delete(&first).Error; err != nil {
					t.Fatal(err)
				}
			case "empty_vector":
				if err := db.Model(&first).Update("embedding", "").Error; err != nil {
					t.Fatal(err)
				}
			case "extra_old_chunk":
				extra := first
				extra.ID = "old-chunk"
				extra.ChunkIndex = 1
				extra.EmbeddingVersion = "old-version"
				if err := db.Create(&extra).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := svc.RunSourceSync(ctx, "ws", "site"); err != nil {
				t.Fatal(err)
			}
			wantCalls := 2
			if change == "none" {
				wantCalls = 1
			}
			if embedder.calls != wantCalls {
				t.Fatalf("embedding calls=%d, want %d", embedder.calls, wantCalls)
			}
			if crawl.calls != 2 {
				t.Fatalf("fetches=%d, want 2", crawl.calls)
			}
			if change == "none" {
				var after model.SupportContentChunk
				if err := db.First(&after).Error; err != nil {
					t.Fatal(err)
				}
				if after.ID != first.ID {
					t.Fatal("unchanged indexed chunk was replaced")
				}
				if err := svc.RunSourceReindex(ctx, "ws", "site"); err != nil {
					t.Fatal(err)
				}
				if embedder.calls != 1 {
					t.Fatal("reindex paid to embed unchanged content")
				}
			}
			stored, err := repo.GetByID(ctx, "site")
			if err != nil {
				t.Fatal(err)
			}
			if stored.IndexedPages != 1 || stored.IndexedChunks != 1 || stored.SyncStatus != "ready" {
				t.Fatalf("sync counts/status=%+v", stored)
			}
		})
	}
}

func TestDocsSyncReusesUnchangedEmbeddings(t *testing.T) {
	db := newInternalKnowledgeTestDB(t)
	ctx := context.Background()
	insertKnowledgeSpace(t, db, "ws", "space", model.SpaceTypeInternal)
	for _, statement := range []string{
		`INSERT INTO agent_knowledge_sources (id, agent_id, space_id, workspace_id) VALUES ('source', 'agent', 'space', 'ws')`,
		`INSERT INTO docs_documents (id, workspace_id, space_id, title, status) VALUES ('doc', 'ws', 'space', 'Guide', 'published')`,
		`INSERT INTO docs_contents (id, document_id, content_text) VALUES ('content', 'doc', 'Connect your account.')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	embedder := &syncReuseEmbedder{}
	svc := NewDocsEmbeddingService(repository.NewDocsChunkRepository(db), nil, repository.NewAgentKnowledgeSourceRepository(db), repository.NewDocsContentRepository(db), repository.NewDocsSpaceRepository(db), nil, repository.NewDocsDocumentRepository(db), embedder, "", nil)
	for i := 0; i < 2; i++ {
		if err := svc.RunSpaceSync(ctx, "ws", "space"); err != nil {
			t.Fatal(err)
		}
	}
	if embedder.calls != 1 {
		t.Fatalf("unchanged docs embedding calls=%d, want 1", embedder.calls)
	}
	if err := db.Exec(`UPDATE docs_contents SET content_text = 'Connect and verify your account.'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.RunSpaceSync(ctx, "ws", "space"); err != nil {
		t.Fatal(err)
	}
	if embedder.calls != 2 {
		t.Fatalf("changed docs embedding calls=%d, want 2", embedder.calls)
	}
}

func TestUploadedFileSyncAndReindexReuseUnchangedEmbeddings(t *testing.T) {
	db := newSyncReuseDB(t)
	ctx := context.Background()
	repo := repository.NewSupportContentSourceRepository(db)
	key, contentType, fileName := "guide.txt", "text/plain", "guide.txt"
	source := &model.SupportContentSource{ID: "file", WorkspaceID: "ws", Name: "Guide", SourceType: "file", StartURL: "file://guide.txt", StorageKey: &key, ContentType: &contentType, FileName: &fileName}
	if err := repo.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	embedder := &syncReuseEmbedder{}
	svc := NewSupportContentSyncService(repo, repository.NewSupportContentPageRepository(db), repository.NewSupportContentChunkRepository(db), embedder, "", nil, &fakeAttachmentStore{}, nil)
	for i := 0; i < 2; i++ {
		if err := svc.RunSourceSync(ctx, "ws", "file"); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.RunSourceReindex(ctx, "ws", "file"); err != nil {
		t.Fatal(err)
	}
	if embedder.calls != 1 {
		t.Fatalf("unchanged uploaded file embedding calls=%d, want 1", embedder.calls)
	}
}

type dailySyncStarter struct {
	syncs, reindexes []string
	onSync           func(string) error
}

func (s *dailySyncStarter) QueueContentSourceSync(_ context.Context, workspaceID, sourceID string) error {
	s.syncs = append(s.syncs, workspaceID+"/"+sourceID)
	if s.onSync != nil {
		return s.onSync(sourceID)
	}
	return nil
}
func (s *dailySyncStarter) QueueContentSourceReindex(_ context.Context, workspaceID, sourceID string) error {
	s.reindexes = append(s.reindexes, workspaceID+"/"+sourceID)
	return nil
}

func TestDailySyncQueuesIdleWebsitesAcrossWorkspaces(t *testing.T) {
	db := newSyncReuseDB(t)
	ctx := context.Background()
	repo := repository.NewSupportContentSourceRepository(db)
	for i, status := range []string{"ready", "failed", "stale", "queued", "running", "disabled"} {
		source := &model.SupportContentSource{ID: fmt.Sprintf("site-%d", i), WorkspaceID: fmt.Sprintf("ws-%d", i), Name: "Guide", SourceType: "website", StartURL: "https://example.test", SyncStatus: status}
		if err := repo.Create(ctx, source); err != nil {
			t.Fatal(err)
		}
	}
	for _, source := range []*model.SupportContentSource{
		{ID: "file", WorkspaceID: "ws", Name: "File", SourceType: "file", StartURL: "file://guide.txt", SyncStatus: "ready"},
		{ID: "recent", WorkspaceID: "ws", Name: "Recent", SourceType: "website", StartURL: "https://example.test", SyncStatus: "ready", LastSyncStartedAt: ptrDailyTime(time.Now())},
	} {
		if err := repo.Create(ctx, source); err != nil {
			t.Fatal(err)
		}
	}
	starter := &dailySyncStarter{}
	svc := NewSupportContentSyncService(repo, nil, nil, internalKnowledgeEmbeddingProvider{}, "", &syncReuseCrawler{}, nil, starter)
	if err := svc.QueueDailySourceSync(ctx); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(starter.syncs) != "[ws-0/site-0 ws-1/site-1 ws-2/site-2]" || len(starter.reindexes) != 0 {
		t.Fatalf("daily queues=%v reindexes=%v", starter.syncs, starter.reindexes)
	}
	if err := svc.QueueDailySourceSync(ctx); err != nil {
		t.Fatal(err)
	}
	if len(starter.syncs) != 3 {
		t.Fatal("daily retry queued duplicate active sources")
	}
}

func ptrDailyTime(value time.Time) *time.Time { return &value }

func TestDailySyncSkipsWebsiteThatBecameActiveDuringDispatch(t *testing.T) {
	db := newSyncReuseDB(t)
	ctx := context.Background()
	repo := repository.NewSupportContentSourceRepository(db)
	for _, id := range []string{"a", "b"} {
		if err := repo.Create(ctx, &model.SupportContentSource{ID: id, WorkspaceID: "ws", Name: id, SourceType: "website", StartURL: "https://example.test", SyncStatus: "ready"}); err != nil {
			t.Fatal(err)
		}
	}
	starter := &dailySyncStarter{onSync: func(id string) error {
		if id == "a" {
			return db.Model(&model.SupportContentSource{}).Where("id = ?", "b").Update("sync_status", "running").Error
		}
		return nil
	}}
	svc := NewSupportContentSyncService(repo, nil, nil, internalKnowledgeEmbeddingProvider{}, "", &syncReuseCrawler{}, nil, starter)
	if err := svc.QueueDailySourceSync(ctx); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(starter.syncs) != "[ws/a]" {
		t.Fatalf("daily sync duplicated an active website: %v", starter.syncs)
	}
}

func TestDailySyncQueueFailureDoesNotBlockOtherWorkspaces(t *testing.T) {
	db := newSyncReuseDB(t)
	ctx := context.Background()
	repo := repository.NewSupportContentSourceRepository(db)
	for _, id := range []string{"a", "b"} {
		if err := repo.Create(ctx, &model.SupportContentSource{ID: id, WorkspaceID: id, Name: id, SourceType: "website", StartURL: "https://example.test", SyncStatus: "ready"}); err != nil {
			t.Fatal(err)
		}
	}
	queueErr := errors.New("workflow unavailable")
	starter := &dailySyncStarter{onSync: func(id string) error {
		if id == "a" {
			return queueErr
		}
		return nil
	}}
	svc := NewSupportContentSyncService(repo, nil, nil, internalKnowledgeEmbeddingProvider{}, "", &syncReuseCrawler{}, nil, starter)
	if err := svc.QueueDailySourceSync(ctx); !errors.Is(err, queueErr) {
		t.Fatalf("daily queue error=%v", err)
	}
	if fmt.Sprint(starter.syncs) != "[a/a b/b]" {
		t.Fatalf("other workspace was blocked: %v", starter.syncs)
	}
}

func TestSourceListSuppliesNextDailyTimeOnlyForEnabledWebsites(t *testing.T) {
	db := newSyncReuseDB(t)
	ctx := context.Background()
	repo := repository.NewSupportContentSourceRepository(db)
	for _, source := range []*model.SupportContentSource{
		{ID: "site", WorkspaceID: "ws", Name: "Site", SourceType: "website", StartURL: "https://example.test", SyncStatus: "ready"},
		{ID: "disabled", WorkspaceID: "ws", Name: "Disabled", SourceType: "website", StartURL: "https://example.test", SyncStatus: "disabled"},
		{ID: "file", WorkspaceID: "ws", Name: "File", SourceType: "file", StartURL: "file://guide.txt", SyncStatus: "ready"},
	} {
		if err := repo.Create(ctx, source); err != nil {
			t.Fatal(err)
		}
	}
	svc := NewSupportContentSourceService(repo, nil, nil, nil, nil, nil, nil)
	sources, err := svc.List(ctx, "ws")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		if source.ID == "site" {
			if source.NextSyncAt == nil || source.NextSyncAt.Hour() != 2 || !source.NextSyncAt.After(time.Now()) {
				t.Fatalf("next daily time=%v", source.NextSyncAt)
			}
		} else if source.NextSyncAt != nil {
			t.Fatalf("non-scheduled source got next sync: %s", source.ID)
		}
	}
}

func TestWebsiteSyncNowCrawlsRatherThanReindexing(t *testing.T) {
	db := newSyncReuseDB(t)
	ctx := context.Background()
	repo := repository.NewSupportContentSourceRepository(db)
	if err := repo.Create(ctx, &model.SupportContentSource{ID: "site", WorkspaceID: "ws", Name: "Guide", SourceType: "website", StartURL: "https://example.test", SyncStatus: "ready"}); err != nil {
		t.Fatal(err)
	}
	starter := &dailySyncStarter{}
	syncService := NewSupportContentSyncService(repo, nil, nil, internalKnowledgeEmbeddingProvider{}, "", &syncReuseCrawler{}, nil, starter)
	svc := NewSupportContentSourceService(repo, nil, nil, nil, nil, syncService, nil)
	if err := svc.Reindex(ctx, "ws", "site"); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(starter.syncs) != "[ws/site]" || len(starter.reindexes) != 0 {
		t.Fatalf("manual refresh syncs=%v reindexes=%v", starter.syncs, starter.reindexes)
	}
}
