package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Public reads stopped backfilling mirror rows, so every write that changes a
// mirrored field must refresh the mirror itself. These cover the writes that
// used to rely on the read-time backfill.

const (
	mirrorWritesWorkspaceID  = "ws-mirror-writes"
	mirrorWritesUserID       = "user-mirror-writes"
	mirrorWritesSpaceID      = "space-mirror-writes"
	mirrorWritesCollectionID = "collection-mirror-writes"
	mirrorWritesDocumentID   = "document-mirror-writes"
)

func seedMirrorWritesHelpCenter(t *testing.T, spaceType string) *gorm.DB {
	t.Helper()

	ptr := func(value string) *string { return &value }
	now := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	db := setupDocsHelpcenterTranslationServiceTestDB(t)

	seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
		ID:             "cfg-mirror-writes",
		WorkspaceID:    mirrorWritesWorkspaceID,
		Subdomain:      "mirror-writes",
		BrandName:      "Mirror Writes",
		BrandColor:     "#000000",
		ThemeMode:      "system",
		HeaderLinks:    json.RawMessage(`[]`),
		FooterConfig:   json.RawMessage(`{}`),
		HomepageConfig: json.RawMessage(`{}`),
		SpaceNavConfig: json.RawMessage(`{}`),
		DefaultLocale:  "en",
		EnabledLocales: model.DocsStringArray{"en"},
		IsPublished:    true,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
		ID:          mirrorWritesSpaceID,
		WorkspaceID: mirrorWritesWorkspaceID,
		Name:        "Help Center",
		Slug:        "help-center",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        spaceType,
		CreatedBy:   mirrorWritesUserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
		ID:          mirrorWritesCollectionID,
		SpaceID:     mirrorWritesSpaceID,
		WorkspaceID: mirrorWritesWorkspaceID,
		Name:        "Basics",
		Slug:        "basics",
		CreatedBy:   mirrorWritesUserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationServiceDocument(t, db, model.DocsDocument{
		ID:           mirrorWritesDocumentID,
		WorkspaceID:  mirrorWritesWorkspaceID,
		SpaceID:      mirrorWritesSpaceID,
		CollectionID: ptr(mirrorWritesCollectionID),
		Title:        "Start Here",
		Status:       model.DocStatusPublished,
		Visibility:   model.SpaceVisibilityWorkspaceWide,
		PublishedAt:  &now,
		CreatedBy:    mirrorWritesUserID,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	seedDocsHelpcenterTranslationServiceContent(t, db, model.DocsContent{
		ID:          "content-mirror-writes",
		DocumentID:  mirrorWritesDocumentID,
		Content:     json.RawMessage(`{"type":"doc","content":[]}`),
		ContentText: "",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationServiceArticle(t, db, model.DocsHelpcenterArticle{
		ID:                "article-mirror-writes",
		DocumentID:        mirrorWritesDocumentID,
		PublicID:          "ab12cd34",
		Slug:              "start-here",
		PublicPublishedAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	return db
}

func loadMirrorWritesArticleTranslation(t *testing.T, db *gorm.DB) model.DocsHelpcenterArticleTranslation {
	t.Helper()
	var translation model.DocsHelpcenterArticleTranslation
	if err := db.Where("document_id = ? AND locale = ?", mirrorWritesDocumentID, "en").
		First(&translation).Error; err != nil {
		t.Fatalf("load article mirror: %v", err)
	}
	return translation
}

func TestUpdateArticleSlugRefreshesMirror(t *testing.T) {
	db := seedMirrorWritesHelpCenter(t, model.SpaceTypeExternalCapable)
	ctx := context.Background()
	svc := newDocsHelpcenterPublicServiceForTest(db)
	if err := svc.translationSvc.EnsureDefaultLocaleMirrorsForWorkspace(ctx, mirrorWritesWorkspaceID); err != nil {
		t.Fatalf("seed mirrors: %v", err)
	}

	if err := svc.UpdateArticleSlug(ctx, mirrorWritesWorkspaceID, mirrorWritesDocumentID, "getting-started"); err != nil {
		t.Fatalf("UpdateArticleSlug: %v", err)
	}

	translation := loadMirrorWritesArticleTranslation(t, db)
	if translation.Slug == nil || *translation.Slug != "getting-started" {
		t.Fatalf("mirror slug = %v, want getting-started", translation.Slug)
	}
	nav, err := svc.GetSpaceNavigation(ctx, mirrorWritesWorkspaceID, "en", "help-center")
	if err != nil {
		t.Fatalf("GetSpaceNavigation: %v", err)
	}
	if len(nav) != 1 || len(nav[0].Articles) != 1 || nav[0].Articles[0].Slug != "getting-started" {
		t.Fatalf("nav = %+v, want the renamed article", nav)
	}
}

func TestUpdateArticleMetadataRefreshesMirror(t *testing.T) {
	db := seedMirrorWritesHelpCenter(t, model.SpaceTypeExternalCapable)
	ctx := context.Background()
	svc := newDocsHelpcenterPublicServiceForTest(db)
	if err := svc.translationSvc.EnsureDefaultLocaleMirrorsForWorkspace(ctx, mirrorWritesWorkspaceID); err != nil {
		t.Fatalf("seed mirrors: %v", err)
	}

	title := "Getting started with Mirror Writes"
	if _, err := svc.UpdateArticleMetadata(ctx, mirrorWritesWorkspaceID, mirrorWritesDocumentID, model.UpdateDocsHelpcenterArticleMetadataRequest{OGTitle: &title}); err != nil {
		t.Fatalf("UpdateArticleMetadata: %v", err)
	}

	translation := loadMirrorWritesArticleTranslation(t, db)
	if translation.OGTitle == nil || *translation.OGTitle != title {
		t.Fatalf("mirror og_title = %v, want %q", translation.OGTitle, title)
	}
}

func TestSpaceBecomingExternalWritesMirrors(t *testing.T) {
	db := seedMirrorWritesHelpCenter(t, model.SpaceTypeInternal)
	ctx := context.Background()
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS docs_space_teams (space_id TEXT NOT NULL, team_id TEXT NOT NULL)`).Error; err != nil {
		t.Fatalf("create docs_space_teams: %v", err)
	}

	spaceSvc := NewDocsSpaceService(repository.NewDocsSpaceRepository(db), nil)
	spaceSvc.SetTranslationService(newDocsHelpcenterTranslationServiceForTest(db))
	external := model.SpaceTypeExternalCapable
	if _, err := spaceSvc.Update(ctx, mirrorWritesSpaceID, model.UpdateDocsSpaceRequest{Type: &external}); err != nil {
		t.Fatalf("Update space type: %v", err)
	}

	var spaceMirrors, collectionMirrors int64
	db.Model(&model.DocsHelpcenterSpaceTranslation{}).Where("space_id = ?", mirrorWritesSpaceID).Count(&spaceMirrors)
	db.Model(&model.DocsHelpcenterCollectionTranslation{}).Where("collection_id = ?", mirrorWritesCollectionID).Count(&collectionMirrors)
	if spaceMirrors != 1 || collectionMirrors != 1 {
		t.Fatalf("mirrors after space became external: space=%d collection=%d, want 1 and 1", spaceMirrors, collectionMirrors)
	}

	nav, err := newDocsHelpcenterPublicServiceForTest(db).GetSpaceNavigation(ctx, mirrorWritesWorkspaceID, "en", "help-center")
	if err != nil {
		t.Fatalf("GetSpaceNavigation: %v", err)
	}
	if len(nav) != 1 || nav[0].Slug != "basics" {
		t.Fatalf("nav = %+v, want the basics collection", nav)
	}
}
