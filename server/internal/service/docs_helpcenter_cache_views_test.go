package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/cache"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func mirrorWritesViewCounts(t *testing.T, db *gorm.DB) (article, translation int) {
	t.Helper()
	var a model.DocsHelpcenterArticle
	if err := db.Where("document_id = ?", mirrorWritesDocumentID).First(&a).Error; err != nil {
		t.Fatalf("load article: %v", err)
	}
	var tr model.DocsHelpcenterArticleTranslation
	if err := db.Where("document_id = ? AND locale = ?", mirrorWritesDocumentID, "en").First(&tr).Error; err != nil {
		t.Fatalf("load translation: %v", err)
	}
	return a.ViewCount, tr.ViewCount
}

func TestRecordPublicArticleView(t *testing.T) {
	db := seedMirrorWritesHelpCenter(t, model.SpaceTypeExternalCapable)
	ctx := context.Background()
	svc := newDocsHelpcenterPublicServiceForTest(db)
	svc.SetHelpcenterCache(cache.NewLRU(64))
	if err := svc.translationSvc.EnsureDefaultLocaleMirrorsForWorkspace(ctx, mirrorWritesWorkspaceID); err != nil {
		t.Fatalf("seed mirrors: %v", err)
	}
	const key = "start-here-ab12cd34"
	if err := db.Exec(`INSERT INTO docs_helpcenter_article_publications
		(id, document_id, workspace_id, space_id, collection_id, locale, title, slug, content, published_at)
		VALUES ('pub-mirror-writes', ?, ?, ?, ?, 'en', 'Start Here', 'start-here', NULL, ?)`,
		mirrorWritesDocumentID, mirrorWritesWorkspaceID, mirrorWritesSpaceID, mirrorWritesCollectionID,
		time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)).Error; err != nil {
		t.Fatalf("seed publication: %v", err)
	}

	// Reads, cached or not, no longer count views.
	for i := 0; i < 2; i++ {
		if a, err := svc.GetPublicArticleByCanonicalKey(ctx, mirrorWritesWorkspaceID, key); err != nil || a == nil {
			t.Fatalf("GetPublicArticleByCanonicalKey = %v, %v; want article", a, err)
		}
		if a, err := svc.GetPublicArticleByLocalizedCanonicalKey(ctx, mirrorWritesWorkspaceID, "en", key); err != nil || a == nil {
			t.Fatalf("GetPublicArticleByLocalizedCanonicalKey = %v, %v; want article", a, err)
		}
	}
	if a, tr := mirrorWritesViewCounts(t, db); a != 0 || tr != 0 {
		t.Fatalf("view counts after reads = (%d, %d), want (0, 0)", a, tr)
	}

	if err := svc.RecordPublicArticleView(ctx, mirrorWritesWorkspaceID, "", key); err != nil {
		t.Fatalf("RecordPublicArticleView canonical: %v", err)
	}
	if a, tr := mirrorWritesViewCounts(t, db); a != 1 || tr != 0 {
		t.Fatalf("view counts after canonical view = (%d, %d), want (1, 0)", a, tr)
	}

	if err := svc.RecordPublicArticleView(ctx, mirrorWritesWorkspaceID, "en", key); err != nil {
		t.Fatalf("RecordPublicArticleView localized: %v", err)
	}
	if _, tr := mirrorWritesViewCounts(t, db); tr != 1 {
		t.Fatalf("translation view count after localized view = %d, want 1", tr)
	}

	err := svc.RecordPublicArticleView(ctx, mirrorWritesWorkspaceID, "", "missing-zz999999")
	if !errors.Is(err, ErrPublicArticleNotFound) {
		t.Fatalf("RecordPublicArticleView unknown key error = %v, want ErrPublicArticleNotFound", err)
	}
}

type countingCache struct {
	mu          sync.Mutex
	invalidated [][]string
}

func (c *countingCache) Get(context.Context, string) ([]byte, bool, error) { return nil, false, nil }

func (c *countingCache) Set(context.Context, string, []byte, time.Duration, ...string) error {
	return nil
}

func (c *countingCache) InvalidateTags(_ context.Context, tags ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.invalidated = append(c.invalidated, tags)
	return nil
}

func (c *countingCache) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.invalidated)
}

func TestInvalidateHelpcenterCacheRunsOnceMoreAfterBurst(t *testing.T) {
	db := seedMirrorWritesHelpCenter(t, model.SpaceTypeExternalCapable)
	ctx := context.Background()
	svc := newDocsHelpcenterPublicServiceForTest(db)
	counter := &countingCache{}
	svc.SetHelpcenterCache(counter)
	svc.hcReinvalidateDelay = 50 * time.Millisecond

	for i := 0; i < 3; i++ {
		svc.InvalidateHelpcenterCacheForWorkspace(ctx, mirrorWritesWorkspaceID)
	}
	if got := counter.calls(); got != 3 {
		t.Fatalf("immediate invalidations = %d, want 3", got)
	}

	deadline := time.Now().Add(2 * time.Second)
	for counter.calls() < 4 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(150 * time.Millisecond) // a second delayed run would land here
	if got := counter.calls(); got != 4 {
		t.Fatalf("invalidations after the delay = %d, want 4 (one debounced re-run)", got)
	}

	last := counter.invalidated[3]
	want := map[string]bool{
		HelpcenterCacheTagWorkspace(mirrorWritesWorkspaceID): true,
		HelpcenterCacheTagIdentifier("mirror-writes"):        true,
	}
	if len(last) != len(want) {
		t.Fatalf("delayed tags = %v, want workspace and host tags", last)
	}
	for _, tag := range last {
		if !want[tag] {
			t.Fatalf("delayed tags = %v, want workspace and host tags", last)
		}
	}
}
