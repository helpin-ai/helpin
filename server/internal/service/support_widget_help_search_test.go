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

type fakeKnowledgeSearch struct {
	surface string
	results []model.PublicSearchResultResponse
}

func (f *fakeKnowledgeSearch) Search(ctx context.Context, _, _, _, _, spaceSlug string, _ int) ([]model.PublicSearchResultResponse, error) {
	f.surface, _ = ctx.Value(knowledgeSurfaceKey{}).(string)
	if spaceSlug != "" {
		panic("the widget searches every space once")
	}
	return f.results, nil
}

// The widget uses the shared search once and keeps only its allowed spaces.
func TestWidgetHelpSearchUsesSharedSearchWithinAllowedSpaces(t *testing.T) {
	search := &fakeKnowledgeSearch{results: []model.PublicSearchResultResponse{
		{ID: "a", Title: "Guide", SpaceSlug: "guides"},
		{ID: "b", Title: "Internal runbook", SpaceSlug: "runbooks"},
		{ID: "c", Title: "API", SpaceSlug: "api"},
	}}
	svc := &SupportInboxService{knowledgeSearch: search}
	got, err := svc.searchWidgetHelp(context.Background(), "ws", "en", "export", []model.WidgetHelpSpace{{Slug: "guides"}, {Slug: "api"}}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Fatalf("results = %+v", got)
	}
	if search.surface != "widget_help" {
		t.Fatalf("surface = %q", search.surface)
	}
}

// When the shared search finds nothing in the widget's spaces, per-space full
// text still runs, so results are never worse than before. An empty database
// makes that search fail visibly, proving it was reached.
func TestWidgetHelpSearchFallsBackToSpaceFullText(t *testing.T) {
	search := &fakeKnowledgeSearch{results: []model.PublicSearchResultResponse{{ID: "b", SpaceSlug: "runbooks"}}}
	svc := &SupportInboxService{knowledgeSearch: search, docsSearchRepo: repository.NewDocsSearchRepository(newEmptySQLite(t))}
	if _, err := svc.searchWidgetHelp(context.Background(), "ws", "en", "export", []model.WidgetHelpSpace{{Slug: "guides"}}, 8); err == nil {
		t.Fatal("full-text fallback did not run")
	}
}

func newEmptySQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}
