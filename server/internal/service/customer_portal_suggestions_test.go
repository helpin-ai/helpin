package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type countingEmbedder struct {
	calls  int
	fail   bool
	inputs []string
}

func (e *countingEmbedder) CreateEmbeddings(_ context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	e.calls++
	e.inputs = append(e.inputs, req.Inputs...)
	if e.fail {
		return nil, errors.New("provider down")
	}
	return &llm.EmbeddingResponse{Vectors: [][]float32{{0.25, 0.5}}}, nil
}

func TestPortalSuggestionQuery(t *testing.T) {
	long := strings.Repeat("word ", 200)
	tests := []struct {
		name, text, want string
		ok               bool
	}{
		{"too short", "export", "", false},
		{"one long word", "exportfailures", "", false},
		{"two short words", "a b", "", false},
		{"collapses whitespace", "  Export \n\t fails on Safari ", "Export fails on Safari", true},
		{"cut to the answer limit", long, strings.TrimSpace(long[:portalSuggestionMaxChars]), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := portalSuggestionQuery(tt.text)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("portalSuggestionQuery = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPortalSuggestionSnippet(t *testing.T) {
	excerpt := "From the excerpt"
	withMatch := model.PublicSearchResultResponse{Excerpt: &excerpt, Matches: []model.PublicSearchMatchResponse{{Snippet: " "}, {Snippet: "Best passage"}}}
	if got := portalSuggestionSnippet(withMatch); got != "Best passage" {
		t.Fatalf("snippet = %q", got)
	}
	if got := portalSuggestionSnippet(model.PublicSearchResultResponse{Excerpt: &excerpt}); got != excerpt {
		t.Fatalf("excerpt fallback = %q", got)
	}
	highlighted := model.PublicSearchResultResponse{Matches: []model.PublicSearchMatchResponse{{Snippet: "<mark>Install</mark>ing &amp; setup"}}}
	if got := portalSuggestionSnippet(highlighted); got != "Installing & setup" {
		t.Fatalf("full-text snippet = %q", got)
	}
}

// Suggestions return nothing, without touching search, when the request
// cannot be searched or the portal takes no requests.
func TestSuggestArticlesGuards(t *testing.T) {
	svc := &CustomerPortalService{}
	ws := &PortalWorkspace{ID: "ws", Settings: model.SupportInboxSettings{PortalEnabled: true, PortalIntakeEnabled: true}}
	for _, tt := range []struct {
		name string
		ws   *PortalWorkspace
		text string
	}{
		{"short text", ws, "help"},
		{"intake off", &PortalWorkspace{ID: "ws", Settings: model.SupportInboxSettings{PortalEnabled: true}}, "Export fails on Safari"},
		{"no knowledge search", ws, "Export fails on Safari"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.SuggestArticles(context.Background(), tt.ws, tt.text)
			if err != nil || got.Articles == nil || len(got.Articles) != 0 || got.SearchCompleted {
				t.Fatalf("SuggestArticles = %v, %v; want an empty list", got, err)
			}
		})
	}
}

func TestPortalSuggestionsWithoutHelpSpacesDoNotCompleteSearch(t *testing.T) {
	db := newEmptySQLite(t)
	if err := db.Exec(`CREATE TABLE docs_helpcenter_configs (id TEXT PRIMARY KEY, workspace_id TEXT, is_published BOOLEAN)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO docs_helpcenter_configs VALUES ('config', 'ws', true)`).Error; err != nil {
		t.Fatal(err)
	}
	svc := &CustomerPortalService{
		knowledge: &HelpcenterAISearchService{},
		inbox:     &SupportInboxService{docsHelpcenterRepo: repository.NewDocsHelpcenterRepository(db)},
	}
	ws := &PortalWorkspace{ID: "ws", Settings: model.SupportInboxSettings{PortalIntakeEnabled: true}}
	got, err := svc.SuggestArticles(context.Background(), ws, "Export fails on Safari")
	if err != nil || got.SearchCompleted || len(got.Articles) != 0 {
		t.Fatalf("suggestions without help spaces = %+v, %v", got, err)
	}
}

func TestHelpcenterQueryEmbeddingKey(t *testing.T) {
	base := helpcenterQueryEmbeddingKey("ws", "model", "Export fails on Safari")
	if helpcenterQueryEmbeddingKey("ws", "model", "  Export   fails\non Safari ") != base {
		t.Fatal("whitespace changed the cache key")
	}
	for _, other := range []string{
		helpcenterQueryEmbeddingKey("other-ws", "model", "Export fails on Safari"),
		helpcenterQueryEmbeddingKey("ws", "other-model", "Export fails on Safari"),
		helpcenterQueryEmbeddingKey("ws", "model", "export fails on safari"),
	} {
		if other == base {
			t.Fatal("distinct workspace, model or text shared a cache key")
		}
	}
}

// Type-ahead repeats queries; each distinct query is embedded once.
func TestHelpcenterQueryEmbeddingIsCached(t *testing.T) {
	embedder := &countingEmbedder{}
	svc := &HelpcenterAISearchService{
		embeddingProvider: embedder,
		embeddingModel:    "text-embedding-3-small",
		redis:             redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}),
	}
	ctx := WithKnowledgeSurface(context.Background(), "portal_intake")
	first := svc.queryEmbedding(ctx, "ws", "Export fails on Safari")
	second := svc.queryEmbedding(ctx, "ws", "Export  fails on Safari")
	if first == "" || first != second {
		t.Fatalf("embeddings %q and %q", first, second)
	}
	if embedder.calls != 1 {
		t.Fatalf("provider called %d times, want 1", embedder.calls)
	}
	svc.queryEmbedding(ctx, "ws", "Invoices are missing")
	if embedder.calls != 2 {
		t.Fatalf("a new query was not embedded: %d calls", embedder.calls)
	}
}

func TestHelpcenterQueryEmbeddingFailureIsNotCached(t *testing.T) {
	embedder := &countingEmbedder{fail: true}
	svc := &HelpcenterAISearchService{
		embeddingProvider: embedder,
		embeddingModel:    "text-embedding-3-small",
		redis:             redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}),
	}
	if got := svc.queryEmbedding(context.Background(), "ws", "Export fails on Safari"); got != "" {
		t.Fatalf("failed embedding returned %q", got)
	}
	embedder.fail = false
	if got := svc.queryEmbedding(context.Background(), "ws", "Export fails on Safari"); got == "" || embedder.calls != 2 {
		t.Fatalf("retry after failure: %q after %d calls", got, embedder.calls)
	}
}

func TestHelpcenterQueryEmbeddingWithoutRedis(t *testing.T) {
	embedder := &countingEmbedder{}
	svc := &HelpcenterAISearchService{embeddingProvider: embedder, embeddingModel: "m"}
	if svc.queryEmbedding(context.Background(), "ws", "Export fails") == "" || svc.queryEmbedding(context.Background(), "ws", "Export fails") == "" {
		t.Fatal("embedding without a cache failed")
	}
	if embedder.calls != 2 {
		t.Fatalf("calls = %d", embedder.calls)
	}
}

func TestRelevantChunksDropsLooseNeighbours(t *testing.T) {
	chunks := []repository.DocsChunkSearchResult{
		{ID: "keyword", LexicalScore: 0.4, VectorScore: 0.1},
		{ID: "similar", VectorScore: helpcenterSemanticRelevanceFloor},
		{ID: "loose", VectorScore: 0.3},
	}
	got := relevantChunks(chunks)
	if len(got) != 2 || got[0].ID != "keyword" || got[1].ID != "similar" {
		t.Fatalf("relevant chunks = %+v", got)
	}
}
