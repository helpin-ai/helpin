package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDocsHelpcenterArticlePublicationRepository_UpsertAndRead(t *testing.T) {
	db := setupDocsHelpcenterTranslationTestDB(t)
	repo := NewDocsHelpcenterPublicationRepository(db)

	now := time.Now().UTC().Truncate(time.Second)
	publication := &model.DocsHelpcenterArticlePublication{
		DocumentID:     "doc-1",
		WorkspaceID:    "ws-1",
		SpaceID:        "space-1",
		CollectionID:   ptrString("collection-1"),
		Locale:         "en",
		Title:          "Getting started",
		Slug:           "getting-started",
		Excerpt:        ptrString("intro"),
		Content:        json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]}`),
		ContentText:    "hello",
		SEOTitle:       ptrString("Getting started"),
		SEODescription: ptrString("intro"),
		PublishedAt:    now,
	}

	stored, err := repo.UpsertArticlePublication(context.Background(), publication)
	if err != nil {
		t.Fatalf("upsert publication: %v", err)
	}
	if stored.Slug != "getting-started" {
		t.Fatalf("expected stored slug, got %q", stored.Slug)
	}

	loaded, err := repo.GetArticlePublication(context.Background(), "doc-1", "en")
	if err != nil {
		t.Fatalf("get publication: %v", err)
	}
	if loaded == nil || loaded.Title != "Getting started" {
		t.Fatalf("expected stored publication title, got %#v", loaded)
	}

	publication.Title = "Getting started updated"
	publication.ContentText = "updated"
	publication.Slug = "getting-started-2"
	stored, err = repo.UpsertArticlePublication(context.Background(), publication)
	if err != nil {
		t.Fatalf("upsert updated publication: %v", err)
	}
	if stored.Title != "Getting started updated" || stored.Slug != "getting-started-2" {
		t.Fatalf("expected upserted row to update, got %#v", stored)
	}
}

func TestDocsHelpcenterArticlePublicationRepository_ListByCollectionAndSlug(t *testing.T) {
	db := setupDocsHelpcenterTranslationTestDB(t)
	repo := NewDocsHelpcenterPublicationRepository(db)
	now := time.Now().UTC().Truncate(time.Second)

	for i, slug := range []string{"first-step", "second-step"} {
		_, err := repo.UpsertArticlePublication(context.Background(), &model.DocsHelpcenterArticlePublication{
			DocumentID:     "doc-" + string(rune('1'+i)),
			WorkspaceID:    "ws-1",
			SpaceID:        "space-1",
			CollectionID:   ptrString("collection-1"),
			Locale:         "en",
			Title:          slug,
			Slug:           slug,
			Content:        json.RawMessage(`{"type":"doc","content":[]}`),
			ContentText:    slug,
			PublishedAt:    now.Add(time.Duration(i) * time.Minute),
		})
		if err != nil {
			t.Fatalf("seed publication %s: %v", slug, err)
		}
	}

	listed, err := repo.ListArticlePublicationsByCollection(context.Background(), "collection-1", "en")
	if err != nil {
		t.Fatalf("list publications: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 publications, got %d", len(listed))
	}

	found, err := repo.GetArticlePublicationByCollectionSlug(context.Background(), "collection-1", "en", "second-step")
	if err != nil {
		t.Fatalf("get publication by slug: %v", err)
	}
	if found == nil || found.Slug != "second-step" {
		t.Fatalf("expected publication by slug, got %#v", found)
	}
}
