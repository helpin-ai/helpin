package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportInboxServiceListWidgetHelpArticles_ResolvesBareSlugWithinAllowedSpaces(t *testing.T) {
	t.Parallel()

	const (
		workspaceID    = "ws-widget-help"
		widgetKey      = "wk-widget-help"
		allowedSpaceID = "space-allowed"
		otherSpaceID   = "space-other"
		allowedCollID  = "coll-allowed"
		otherCollID    = "coll-other"
		documentID     = "doc-allowed"
	)

	now := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	ctx := context.Background()
	db := setupDocsHelpcenterTranslationServiceTestDB(t)

	mustExec(t, db, `CREATE TABLE support_widget_installations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		widget_key TEXT NOT NULL,
		secret_key TEXT NOT NULL,
		settings TEXT NOT NULL DEFAULT '{}',
		active BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME,
		updated_at DATETIME
	)`)

	seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
		ID:            "cfg-widget-help",
		WorkspaceID:   workspaceID,
		Subdomain:     "widget-help",
		BrandName:     "Widget Help",
		BrandColor:    "#000000",
		ThemeMode:     "system",
		DefaultLocale: "en",
		EnabledLocales: model.DocsStringArray{
			"en",
		},
		IsPublished: true,
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
		ID:          allowedSpaceID,
		WorkspaceID: workspaceID,
		Name:        "Help Docs",
		Slug:        "help-docs",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeExternalCapable,
		CreatedBy:   "user-1",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
		ID:          otherSpaceID,
		WorkspaceID: workspaceID,
		Name:        "Other Docs",
		Slug:        "other-docs",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeExternalCapable,
		CreatedBy:   "user-1",
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
		ID:          allowedCollID,
		SpaceID:     allowedSpaceID,
		WorkspaceID: workspaceID,
		Name:        "Getting Started",
		Slug:        "getting-started",
		PublicID:    "abc123ef",
		CreatedBy:   "user-1",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
		ID:          otherCollID,
		SpaceID:     otherSpaceID,
		WorkspaceID: workspaceID,
		Name:        "Getting Started",
		Slug:        "getting-started",
		PublicID:    "def456ab",
		CreatedBy:   "user-1",
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	seedDocsHelpcenterTranslationServiceDocument(t, db, model.DocsDocument{
		ID:          documentID,
		WorkspaceID: workspaceID,
		SpaceID:     allowedSpaceID,
		CollectionID: func() *string {
			id := allowedCollID
			return &id
		}(),
		Title:      "Workspace Setup",
		Status:     model.DocStatusPublished,
		Visibility: model.SpaceVisibilityWorkspaceWide,
		CreatedBy:  "user-1",
		PublishedAt: func() *time.Time {
			tm := now
			return &tm
		}(),
		CreatedAt: now,
		UpdatedAt: now,
	})
	seedDocsHelpcenterTranslationServiceArticle(t, db, model.DocsHelpcenterArticle{
		ID:         "article-allowed",
		DocumentID: documentID,
		PublicID:   "884d78a2",
		Slug:       "workspace-setup",
		PublicPublishedAt: func() *time.Time {
			tm := now
			return &tm
		}(),
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err := db.Create(&model.DocsHelpcenterArticlePublication{
		ID:          "pub-allowed",
		DocumentID:  documentID,
		WorkspaceID: workspaceID,
		SpaceID:     allowedSpaceID,
		CollectionID: func() *string {
			id := allowedCollID
			return &id
		}(),
		Locale:      "en",
		Title:       "Workspace Setup",
		Slug:        "workspace-setup",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed article publication: %v", err)
	}

	installationRepo := repository.NewSupportInboxInstallationRepository(db)
	if err := installationRepo.Create(ctx, &model.SupportWidgetInstallation{
		ID:          "inst-widget-help",
		WorkspaceID: workspaceID,
		WidgetKey:   widgetKey,
		SecretKey:   "sk-widget-help",
		Settings:    `{"widget_help_space_ids":["space-allowed"]}`,
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("create widget installation: %v", err)
	}

	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		nil,
		installationRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsHelpcenterRepository(db),
	)

	articles, err := svc.ListWidgetHelpArticles(ctx, widgetKey, "getting-started")
	if err != nil {
		t.Fatalf("ListWidgetHelpArticles: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("len(articles) = %d, want 1", len(articles))
	}
	if articles[0].ID != documentID {
		t.Fatalf("articles[0].id = %q, want %q", articles[0].ID, documentID)
	}
	if articles[0].ArticleKey != "workspace-setup-884d78a2" {
		t.Fatalf("articles[0].article_key = %q, want %q", articles[0].ArticleKey, "workspace-setup-884d78a2")
	}
}
