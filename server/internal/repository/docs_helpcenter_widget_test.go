package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDocsCollectionRepository_GetBySlug(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationTestDB(t)
	ctx := context.Background()

	seedDocsHelpcenterTranslationCollection(t, db, model.DocsCollection{
		ID:          "collection-1",
		SpaceID:     "space-1",
		WorkspaceID: "ws-1",
		Name:        "Getting Started",
		Slug:        "getting-started",
		CreatedBy:   "user-1",
	})

	repo := NewDocsCollectionRepository(db)
	coll, err := repo.GetBySlug(ctx, "ws-1", "getting-started")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if coll == nil {
		t.Fatal("GetBySlug returned nil collection")
	}
	if coll.ID != "collection-1" {
		t.Fatalf("collection id = %q, want %q", coll.ID, "collection-1")
	}
}

func TestDocsHelpcenterRepository_WidgetArticleLookupsUsePublicID(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	seedDocsHelpcenterTranslationSpace(t, db, model.DocsSpace{
		ID:          "space-1",
		WorkspaceID: "ws-1",
		Name:        "Docs",
		Slug:        "docs",
		Visibility:  "workspace_wide",
		Type:        "space",
		CreatedBy:   "user-1",
	})
	seedDocsHelpcenterTranslationCollection(t, db, model.DocsCollection{
		ID:          "collection-1",
		SpaceID:     "space-1",
		WorkspaceID: "ws-1",
		Name:        "Getting Started",
		Slug:        "getting-started",
		CreatedBy:   "user-1",
	})
	seedDocsHelpcenterTranslationDocument(t, db, model.DocsDocument{
		ID:           "doc-1",
		WorkspaceID:  "ws-1",
		SpaceID:      "space-1",
		CollectionID: ptrString("collection-1"),
		Title:        "Workspace setup",
		Status:       model.DocStatusPublished,
		Visibility:   "workspace_wide",
		CreatedBy:    "user-1",
		PublishedAt:  &now,
	})
	seedDocsHelpcenterTranslationConfig(t, db, model.DocsHelpcenterConfig{
		ID:            "cfg-1",
		WorkspaceID:   "ws-1",
		Subdomain:     "docs",
		BrandName:     "Docs",
		BrandColor:    "#111111",
		ThemeMode:     "light",
		DefaultLocale: "en",
		IsPublished:   true,
	})
	seedDocsHelpcenterTranslationArticle(t, db, model.DocsHelpcenterArticle{
		ID:                "ha-1",
		DocumentID:        "doc-1",
		PublicID:          "884d78a2",
		Slug:              "workspace-setup",
		PublicPublishedAt: &now,
	})

	publicationRepo := NewDocsHelpcenterPublicationRepository(db)
	_, err := publicationRepo.UpsertArticlePublication(ctx, &model.DocsHelpcenterArticlePublication{
		DocumentID:   "doc-1",
		WorkspaceID:  "ws-1",
		SpaceID:      "space-1",
		CollectionID: ptrString("collection-1"),
		Locale:       "en",
		Title:        "Workspace setup",
		Slug:         "workspace-setup",
		Excerpt:      ptrString("How to configure your workspace"),
		Content:      json.RawMessage(`{"type":"doc","content":[]}`),
		ContentText:  "How to configure your workspace",
		PublishedAt:  now,
	})
	if err != nil {
		t.Fatalf("UpsertArticlePublication: %v", err)
	}

	repo := NewDocsHelpcenterRepository(db)

	articles, err := repo.ListWidgetArticlesByCollectionID(ctx, "collection-1")
	if err != nil {
		t.Fatalf("ListWidgetArticlesByCollectionID: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("len(articles) = %d, want 1", len(articles))
	}
	if articles[0].PublicID != "884d78a2" {
		t.Fatalf("articles[0].PublicID = %q, want %q", articles[0].PublicID, "884d78a2")
	}
	if articles[0].Slug != "workspace-setup" {
		t.Fatalf("articles[0].Slug = %q, want %q", articles[0].Slug, "workspace-setup")
	}

	doc, article, content, err := repo.GetPublicArticleByPublicIDInSpaces(ctx, []string{"space-1"}, "884d78a2")
	if err != nil {
		t.Fatalf("GetPublicArticleByPublicIDInSpaces: %v", err)
	}
	if doc == nil || article == nil || content == nil {
		t.Fatalf("expected doc/article/content, got doc=%#v article=%#v content=%#v", doc, article, content)
	}
	if doc.ID != "doc-1" {
		t.Fatalf("doc.ID = %q, want %q", doc.ID, "doc-1")
	}
	if article.PublicID != "884d78a2" {
		t.Fatalf("article.PublicID = %q, want %q", article.PublicID, "884d78a2")
	}
	if article.Slug != "workspace-setup" {
		t.Fatalf("article.Slug = %q, want %q", article.Slug, "workspace-setup")
	}
}
