package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type scriptedDocsTranslationLLM struct {
	response string
	err      error
	requests []llm.ChatRequest
}

func (f *scriptedDocsTranslationLLM) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	return &llm.ChatResponse{Content: f.response}, nil
}

func setupDocsHelpcenterTranslationServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-helpcenter-service-i18n-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	stmts := []string{
		`CREATE TABLE docs_spaces (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			icon TEXT,
			visibility TEXT NOT NULL,
			type TEXT NOT NULL,
			default_review_days INTEGER,
			is_system BOOLEAN NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_collections (
			id TEXT PRIMARY KEY,
			space_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			name TEXT NOT NULL,
			slug TEXT NOT NULL DEFAULT '',
			description TEXT,
			icon TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_documents (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			visibility TEXT NOT NULL,
			owner_id TEXT,
			team_id TEXT,
			template_key TEXT,
			excerpt TEXT,
			icon TEXT,
			tags TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			is_pinned BOOLEAN NOT NULL DEFAULT 0,
			is_publicly_shared BOOLEAN NOT NULL DEFAULT 0,
			share_token TEXT,
			is_locked BOOLEAN NOT NULL DEFAULT 0,
			locked_by TEXT,
			last_reviewed_at DATETIME,
			next_review_at DATETIME,
			published_at DATETIME,
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_contents (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE,
			content JSON,
			content_text TEXT,
			word_count INTEGER NOT NULL DEFAULT 0,
			import_source_html TEXT,
			import_source_system TEXT,
			import_source_object_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_configs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL UNIQUE,
			subdomain TEXT NOT NULL,
			custom_domain TEXT,
			brand_name TEXT NOT NULL,
			brand_logo_url TEXT,
			brand_logo_dark_url TEXT,
			brand_color TEXT NOT NULL,
			favicon_url TEXT,
			theme_mode TEXT NOT NULL,
			header_links JSON,
			footer_config JSON,
			homepage_config JSON,
			space_nav_config JSON,
			search_placeholder TEXT,
			default_locale TEXT NOT NULL DEFAULT 'en',
			enabled_locales TEXT,
			show_language_switcher BOOLEAN NOT NULL DEFAULT 0,
			fallback_to_default_locale BOOLEAN NOT NULL DEFAULT 1,
			is_published BOOLEAN NOT NULL DEFAULT 0,
			seo_title TEXT,
			seo_description TEXT,
			support_email TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_articles (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL DEFAULT '',
			seo_title TEXT,
			seo_description TEXT,
			helpful_count INTEGER NOT NULL DEFAULT 0,
			not_helpful_count INTEGER NOT NULL DEFAULT 0,
			view_count INTEGER NOT NULL DEFAULT 0,
			public_published_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_space_translations (
			id TEXT PRIMARY KEY,
			space_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			locale TEXT NOT NULL,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'draft',
			source_updated_at DATETIME,
			source_synced BOOLEAN NOT NULL DEFAULT 0,
			published_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(space_id, locale),
			UNIQUE(workspace_id, locale, slug)
		)`,
		`CREATE TABLE docs_helpcenter_collection_translations (
			id TEXT PRIMARY KEY,
			collection_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			locale TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			slug TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			source_updated_at DATETIME,
			source_synced BOOLEAN NOT NULL DEFAULT 0,
			published_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(collection_id, locale),
			UNIQUE(space_id, locale, slug)
		)`,
		`CREATE TABLE docs_helpcenter_article_translations (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			locale TEXT NOT NULL,
			title TEXT NOT NULL,
			slug TEXT NOT NULL,
			excerpt TEXT,
			content JSON,
			content_text TEXT,
			seo_title TEXT,
			seo_description TEXT,
			status TEXT NOT NULL DEFAULT 'draft',
			source_updated_at DATETIME,
			source_synced BOOLEAN NOT NULL DEFAULT 0,
			published_at DATETIME,
			view_count INTEGER NOT NULL DEFAULT 0,
			helpful_count INTEGER NOT NULL DEFAULT 0,
			not_helpful_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(document_id, locale),
			UNIQUE(space_id, locale, slug)
		)`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create docs helpcenter translation service test table: %v", err)
		}
	}

	return db
}

func seedDocsHelpcenterTranslationServiceConfig(t *testing.T, db *gorm.DB, cfg model.DocsHelpcenterConfig) {
	t.Helper()
	if err := db.Create(&cfg).Error; err != nil {
		t.Fatalf("seed config: %v", err)
	}
}

func seedDocsHelpcenterTranslationServiceSpace(t *testing.T, db *gorm.DB, space model.DocsSpace) {
	t.Helper()
	if err := db.Create(&space).Error; err != nil {
		t.Fatalf("seed space: %v", err)
	}
}

func seedDocsHelpcenterTranslationServiceCollection(t *testing.T, db *gorm.DB, coll model.DocsCollection) {
	t.Helper()
	if err := db.Create(&coll).Error; err != nil {
		t.Fatalf("seed collection: %v", err)
	}
}

func seedDocsHelpcenterTranslationServiceDocument(t *testing.T, db *gorm.DB, doc model.DocsDocument) {
	t.Helper()
	if err := db.Create(&doc).Error; err != nil {
		t.Fatalf("seed document: %v", err)
	}
}

func seedDocsHelpcenterTranslationServiceContent(t *testing.T, db *gorm.DB, content model.DocsContent) {
	t.Helper()
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("seed content: %v", err)
	}
}

func seedDocsHelpcenterTranslationServiceArticle(t *testing.T, db *gorm.DB, article model.DocsHelpcenterArticle) {
	t.Helper()
	if err := db.Create(&article).Error; err != nil {
		t.Fatalf("seed helpcenter article: %v", err)
	}
}

func seedDocsHelpcenterTranslationServiceSpaceTranslation(t *testing.T, db *gorm.DB, translation model.DocsHelpcenterSpaceTranslation) {
	t.Helper()
	if err := db.Create(&translation).Error; err != nil {
		t.Fatalf("seed space translation: %v", err)
	}
}

func seedDocsHelpcenterTranslationServiceCollectionTranslation(t *testing.T, db *gorm.DB, translation model.DocsHelpcenterCollectionTranslation) {
	t.Helper()
	if err := db.Create(&translation).Error; err != nil {
		t.Fatalf("seed collection translation: %v", err)
	}
}

func seedDocsHelpcenterTranslationServiceArticleTranslation(t *testing.T, db *gorm.DB, translation model.DocsHelpcenterArticleTranslation) {
	t.Helper()
	if err := db.Create(&translation).Error; err != nil {
		t.Fatalf("seed article translation: %v", err)
	}
}

func newDocsHelpcenterTranslationServiceForTest(db *gorm.DB) *DocsHelpcenterTranslationService {
	return newDocsHelpcenterTranslationServiceForTestWithLLM(db, nil)
}

func newDocsHelpcenterTranslationServiceForTestWithLLM(db *gorm.DB, llmProvider llm.Provider) *DocsHelpcenterTranslationService {
	return NewDocsHelpcenterTranslationService(
		repository.NewDocsHelpcenterTranslationRepository(db),
		repository.NewDocsHelpcenterRepository(db),
		repository.NewDocsDocumentRepository(db),
		repository.NewDocsContentRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		llmProvider,
	)
}

func newDocsHelpcenterPublicServiceForTest(db *gorm.DB) *DocsHelpcenterService {
	svc := NewDocsHelpcenterService(
		repository.NewDocsHelpcenterRepository(db),
		repository.NewDocsDocumentRepository(db),
		repository.NewDocsContentRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		nil,
		nil,
	)
	svc.SetTranslationService(newDocsHelpcenterTranslationServiceForTest(db))
	return svc
}

func TestDocsHelpcenterTranslationService(t *testing.T) {
	t.Parallel()

	const (
		workspaceID  = "ws-hc-i18n"
		userID       = "user-hc-i18n"
		spaceID      = "space-hc-i18n"
		collectionID = "collection-hc-i18n"
		documentID   = "document-hc-i18n"
	)

	now := time.Date(2026, 3, 25, 16, 0, 0, 0, time.UTC)
	ptr := func(value string) *string { return &value }
	jsonEmptyArray := json.RawMessage(`[]`)
	jsonEmptyObject := json.RawMessage(`{}`)

	setupBase := func(t *testing.T) (*gorm.DB, *DocsHelpcenterTranslationService, context.Context) {
		t.Helper()
		db := setupDocsHelpcenterTranslationServiceTestDB(t)

		seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
			ID:                      "cfg-hc-i18n",
			WorkspaceID:             workspaceID,
			Subdomain:               "hc-i18n",
			BrandName:               "HC I18n",
			BrandColor:              "#000000",
			ThemeMode:               "system",
			HeaderLinks:             jsonEmptyArray,
			FooterConfig:            jsonEmptyObject,
			HomepageConfig:          jsonEmptyObject,
			SpaceNavConfig:          jsonEmptyObject,
			DefaultLocale:           "en",
			EnabledLocales:          model.DocsStringArray{"en", "fr"},
			ShowLanguageSwitcher:    true,
			FallbackToDefaultLocale: true,
			IsPublished:             true,
			CreatedAt:               now,
			UpdatedAt:               now,
		})
		seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
			ID:          spaceID,
			WorkspaceID: workspaceID,
			Name:        "Getting Started",
			Slug:        "getting-started",
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeExternalCapable,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
			ID:          collectionID,
			SpaceID:     spaceID,
			WorkspaceID: workspaceID,
			Name:        "Basics",
			Slug:        "basics",
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceDocument(t, db, model.DocsDocument{
			ID:           documentID,
			WorkspaceID:  workspaceID,
			SpaceID:      spaceID,
			CollectionID: ptr(collectionID),
			Title:        "Start Here",
			Status:       model.DocStatusPublished,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Excerpt:      ptr("How to begin"),
			Position:     0,
			PublishedAt:  &now,
			CreatedBy:    userID,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		seedDocsHelpcenterTranslationServiceContent(t, db, model.DocsContent{
			ID:          "content-hc-i18n",
			DocumentID:  documentID,
			Content:     json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Welcome"}]}]}`),
			ContentText: "Welcome",
			WordCount:   1,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		seedDocsHelpcenterTranslationServiceArticle(t, db, model.DocsHelpcenterArticle{
			ID:                "article-hc-i18n",
			DocumentID:        documentID,
			Slug:              "start-here",
			SEOTitle:          ptr("SEO Start Here"),
			SEODescription:    ptr("SEO description"),
			ViewCount:         4,
			HelpfulCount:      2,
			NotHelpfulCount:   1,
			PublicPublishedAt: &now,
			CreatedAt:         now,
			UpdatedAt:         now,
		})

		return db, newDocsHelpcenterTranslationServiceForTest(db), context.Background()
	}

	t.Run("SyncDefaultLocaleArticleMirror mirrors source content into default locale", func(t *testing.T) {
		db, svc, ctx := setupBase(t)

		translation, err := svc.SyncDefaultLocaleArticleMirror(ctx, documentID)
		if err != nil {
			t.Fatalf("SyncDefaultLocaleArticleMirror: %v", err)
		}
		if translation.Locale != "en" || translation.Title != "Start Here" || translation.Slug != "start-here" {
			t.Fatalf("unexpected mirrored translation: %+v", translation)
		}
		if translation.Status != model.DocsHelpcenterTranslationStatusPublished || !translation.SourceSynced {
			t.Fatalf("mirrored translation status/source sync = %+v", translation)
		}

		var stored model.DocsHelpcenterArticleTranslation
		if err := db.WithContext(ctx).Where("document_id = ? AND locale = ?", documentID, "en").First(&stored).Error; err != nil {
			t.Fatalf("load mirrored translation: %v", err)
		}
		if string(stored.Content) != `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Welcome"}]}]}` {
			t.Fatalf("stored mirrored content = %s, want source content", string(stored.Content))
		}
	})

	t.Run("MarkArticleTranslationsForSourceChange marks non-default locales needs_review", func(t *testing.T) {
		db, svc, ctx := setupBase(t)
		if _, err := svc.SyncDefaultLocaleArticleMirror(ctx, documentID); err != nil {
			t.Fatalf("SyncDefaultLocaleArticleMirror: %v", err)
		}

		newer := now.Add(2 * time.Hour)
		if err := db.Model(&model.DocsDocument{}).Where("id = ?", documentID).Update("updated_at", newer).Error; err != nil {
			t.Fatalf("bump document updated_at: %v", err)
		}
		seedDocsHelpcenterTranslationServiceArticleTranslation(t, db, model.DocsHelpcenterArticleTranslation{
			ID:              "fr-translation",
			DocumentID:      documentID,
			WorkspaceID:     workspaceID,
			SpaceID:         spaceID,
			CollectionID:    ptr(collectionID),
			Locale:          "fr",
			Title:           "Commencer ici",
			Slug:            "commencer-ici",
			Status:          model.DocsHelpcenterTranslationStatusPublished,
			SourceUpdatedAt: &now,
			SourceSynced:    true,
			PublishedAt:     &now,
			CreatedAt:       now,
			UpdatedAt:       now,
		})

		if err := svc.MarkArticleTranslationsForSourceChange(ctx, documentID); err != nil {
			t.Fatalf("MarkArticleTranslationsForSourceChange: %v", err)
		}

		var fr model.DocsHelpcenterArticleTranslation
		if err := db.WithContext(ctx).Where("document_id = ? AND locale = ?", documentID, "fr").First(&fr).Error; err != nil {
			t.Fatalf("load fr translation: %v", err)
		}
		if fr.Status != model.DocsHelpcenterTranslationStatusNeedsReview {
			t.Fatalf("fr translation status = %q, want %q", fr.Status, model.DocsHelpcenterTranslationStatusNeedsReview)
		}
		if fr.SourceSynced {
			t.Fatal("fr translation source_synced = true, want false")
		}
		if fr.SourceUpdatedAt == nil || !fr.SourceUpdatedAt.Equal(newer) {
			t.Fatalf("fr translation source_updated_at = %+v, want %s", fr.SourceUpdatedAt, newer.Format(time.RFC3339))
		}

		var en model.DocsHelpcenterArticleTranslation
		if err := db.WithContext(ctx).Where("document_id = ? AND locale = ?", documentID, "en").First(&en).Error; err != nil {
			t.Fatalf("load en translation: %v", err)
		}
		if en.Status != model.DocsHelpcenterTranslationStatusPublished {
			t.Fatalf("en translation status = %q, want published", en.Status)
		}
	})

	t.Run("PublishArticleTranslation requires published translated parents", func(t *testing.T) {
		db, svc, ctx := setupBase(t)
		seedDocsHelpcenterTranslationServiceArticleTranslation(t, db, model.DocsHelpcenterArticleTranslation{
			ID:              "fr-article",
			DocumentID:      documentID,
			WorkspaceID:     workspaceID,
			SpaceID:         spaceID,
			CollectionID:    ptr(collectionID),
			Locale:          "fr",
			Title:           "Commencer ici",
			Slug:            "commencer-ici",
			Status:          model.DocsHelpcenterTranslationStatusDraft,
			SourceUpdatedAt: &now,
			SourceSynced:    true,
			CreatedAt:       now,
			UpdatedAt:       now,
		})

		if _, err := svc.PublishArticleTranslation(ctx, documentID, "fr"); err == nil {
			t.Fatal("expected publish without parents to fail")
		}

		seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
			ID:              "fr-space",
			SpaceID:         spaceID,
			WorkspaceID:     workspaceID,
			Locale:          "fr",
			Name:            "Demarrage",
			Slug:            "demarrage",
			Status:          model.DocsHelpcenterTranslationStatusPublished,
			SourceUpdatedAt: &now,
			SourceSynced:    true,
			PublishedAt:     &now,
			CreatedAt:       now,
			UpdatedAt:       now,
		})

		if _, err := svc.PublishArticleTranslation(ctx, documentID, "fr"); err == nil {
			t.Fatal("expected publish without collection translation to fail")
		}

		seedDocsHelpcenterTranslationServiceCollectionTranslation(t, db, model.DocsHelpcenterCollectionTranslation{
			ID:              "fr-collection",
			CollectionID:    collectionID,
			WorkspaceID:     workspaceID,
			SpaceID:         spaceID,
			Locale:          "fr",
			Name:            "Bases",
			Slug:            "bases",
			Status:          model.DocsHelpcenterTranslationStatusPublished,
			SourceUpdatedAt: &now,
			SourceSynced:    true,
			PublishedAt:     &now,
			CreatedAt:       now,
			UpdatedAt:       now,
		})

		published, err := svc.PublishArticleTranslation(ctx, documentID, "fr")
		if err != nil {
			t.Fatalf("PublishArticleTranslation with parents: %v", err)
		}
		if published.Status != model.DocsHelpcenterTranslationStatusPublished || published.PublishedAt == nil {
			t.Fatalf("published article translation = %+v, want published with timestamp", published)
		}
	})

	t.Run("ResolveArticleTranslation falls back to default locale when requested locale is missing", func(t *testing.T) {
		_, svc, ctx := setupBase(t)
		if _, err := svc.SyncDefaultLocaleArticleMirror(ctx, documentID); err != nil {
			t.Fatalf("SyncDefaultLocaleArticleMirror: %v", err)
		}

		translation, resolvedLocale, fellBack, err := svc.ResolveArticleTranslation(ctx, documentID, "fr")
		if err != nil {
			t.Fatalf("ResolveArticleTranslation: %v", err)
		}
		if resolvedLocale != "en" || !fellBack {
			t.Fatalf("resolved locale = %q, fellBack = %t, want en/true", resolvedLocale, fellBack)
		}
		if translation.Locale != "en" || translation.Title != "Start Here" {
			t.Fatalf("resolved translation = %+v, want default locale mirror", translation)
		}
	})

	t.Run("GenerateArticleTranslationDraft creates a draft translation from AI output", func(t *testing.T) {
		db, _, ctx := setupBase(t)
		provider := &scriptedDocsTranslationLLM{
			response: `{"title":"Commencer ici","slug":"commencer-ici","excerpt":"Guide de demarrage rapide","seo_title":"Commencer ici","seo_description":"Guide d'aide en francais","body":"Bonjour\n\nBienvenue dans Helpin."}`,
		}
		svc := newDocsHelpcenterTranslationServiceForTestWithLLM(db, provider)

		translation, err := svc.GenerateArticleTranslationDraft(ctx, documentID, "fr")
		if err != nil {
			t.Fatalf("GenerateArticleTranslationDraft: %v", err)
		}
		if translation.Locale != "fr" || translation.Title != "Commencer ici" || translation.Slug != "commencer-ici" {
			t.Fatalf("unexpected generated translation: %+v", translation)
		}
		if translation.Status != model.DocsHelpcenterTranslationStatusDraft {
			t.Fatalf("generated translation status = %q, want draft", translation.Status)
		}
		if !translation.SourceSynced {
			t.Fatal("generated translation source_synced = false, want true")
		}
		if len(translation.Content) == 0 || !strings.Contains(string(translation.Content), "Bienvenue dans Helpin.") {
			t.Fatalf("generated translation content = %s, want translated body", string(translation.Content))
		}
		if len(provider.requests) != 1 {
			t.Fatalf("llm requests = %d, want 1", len(provider.requests))
		}
	})
}

func TestDocsHelpcenterPublicLocale_ListSpacesUsesLocaleTranslations(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-hc-public-spaces"
		userID      = "user-hc-public-spaces"
		spaceID     = "space-hc-public-spaces"
	)

	now := time.Date(2026, 3, 25, 19, 0, 0, 0, time.UTC)
	jsonEmptyArray := json.RawMessage(`[]`)
	jsonEmptyObject := json.RawMessage(`{}`)

	db := setupDocsHelpcenterTranslationServiceTestDB(t)
	ctx := context.Background()

	seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
		ID:                      "cfg-hc-public-spaces",
		WorkspaceID:             workspaceID,
		Subdomain:               "hc-public-spaces",
		BrandName:               "HC Public Spaces",
		BrandColor:              "#000000",
		ThemeMode:               "system",
		HeaderLinks:             jsonEmptyArray,
		FooterConfig:            jsonEmptyObject,
		HomepageConfig:          jsonEmptyObject,
		SpaceNavConfig:          jsonEmptyObject,
		DefaultLocale:           "en",
		EnabledLocales:          model.DocsStringArray{"en", "fr"},
		ShowLanguageSwitcher:    true,
		FallbackToDefaultLocale: true,
		IsPublished:             true,
		CreatedAt:               now,
		UpdatedAt:               now,
	})
	seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
		ID:          spaceID,
		WorkspaceID: workspaceID,
		Name:        "Getting Started",
		Slug:        "getting-started",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeExternalCapable,
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
		ID:              "en-space-public-spaces",
		SpaceID:         spaceID,
		WorkspaceID:     workspaceID,
		Locale:          "en",
		Name:            "Getting Started",
		Slug:            "getting-started",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
		ID:              "fr-space-public-spaces",
		SpaceID:         spaceID,
		WorkspaceID:     workspaceID,
		Locale:          "fr",
		Name:            "Demarrage",
		Slug:            "demarrage",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	})

	svc := newDocsHelpcenterPublicServiceForTest(db)
	spaces, err := svc.ListPublicSpaces(ctx, workspaceID, "fr")
	if err != nil {
		t.Fatalf("ListPublicSpaces: %v", err)
	}
	if len(spaces) != 1 {
		t.Fatalf("len(spaces) = %d, want 1", len(spaces))
	}
	if spaces[0].Name != "Demarrage" || spaces[0].Slug != "demarrage" {
		t.Fatalf("space = %+v, want French translation", spaces[0])
	}
}

func TestDocsHelpcenterPublicLocale_GetArticleFallsBackToDefaultLocale(t *testing.T) {
	t.Parallel()

	const (
		workspaceID  = "ws-hc-public-article"
		userID       = "user-hc-public-article"
		spaceID      = "space-hc-public-article"
		collectionID = "collection-hc-public-article"
		documentID   = "document-hc-public-article"
	)

	ptr := func(value string) *string { return &value }
	now := time.Date(2026, 3, 25, 19, 30, 0, 0, time.UTC)
	jsonEmptyArray := json.RawMessage(`[]`)
	jsonEmptyObject := json.RawMessage(`{}`)

	db := setupDocsHelpcenterTranslationServiceTestDB(t)
	ctx := context.Background()

	seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
		ID:                      "cfg-hc-public-article",
		WorkspaceID:             workspaceID,
		Subdomain:               "hc-public-article",
		BrandName:               "HC Public Article",
		BrandColor:              "#000000",
		ThemeMode:               "system",
		HeaderLinks:             jsonEmptyArray,
		FooterConfig:            jsonEmptyObject,
		HomepageConfig:          jsonEmptyObject,
		SpaceNavConfig:          jsonEmptyObject,
		DefaultLocale:           "en",
		EnabledLocales:          model.DocsStringArray{"en", "fr"},
		ShowLanguageSwitcher:    true,
		FallbackToDefaultLocale: true,
		IsPublished:             true,
		CreatedAt:               now,
		UpdatedAt:               now,
	})
	seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
		ID:          spaceID,
		WorkspaceID: workspaceID,
		Name:        "Getting Started",
		Slug:        "getting-started",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeExternalCapable,
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationServiceCollection(t, db, model.DocsCollection{
		ID:          collectionID,
		SpaceID:     spaceID,
		WorkspaceID: workspaceID,
		Name:        "Basics",
		Slug:        "basics",
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationServiceDocument(t, db, model.DocsDocument{
		ID:           documentID,
		WorkspaceID:  workspaceID,
		SpaceID:      spaceID,
		CollectionID: ptr(collectionID),
		Title:        "Start Here",
		Status:       model.DocStatusPublished,
		Visibility:   model.SpaceVisibilityWorkspaceWide,
		Excerpt:      ptr("How to begin"),
		Position:     0,
		PublishedAt:  &now,
		CreatedBy:    userID,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	seedDocsHelpcenterTranslationServiceArticle(t, db, model.DocsHelpcenterArticle{
		ID:                "article-hc-public-article",
		DocumentID:        documentID,
		Slug:              "start-here",
		PublicPublishedAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
		ID:              "en-space-public-article",
		SpaceID:         spaceID,
		WorkspaceID:     workspaceID,
		Locale:          "en",
		Name:            "Getting Started",
		Slug:            "getting-started",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	seedDocsHelpcenterTranslationServiceCollectionTranslation(t, db, model.DocsHelpcenterCollectionTranslation{
		ID:              "en-collection-public-article",
		CollectionID:    collectionID,
		WorkspaceID:     workspaceID,
		SpaceID:         spaceID,
		Locale:          "en",
		Name:            "Basics",
		Slug:            "basics",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	seedDocsHelpcenterTranslationServiceArticleTranslation(t, db, model.DocsHelpcenterArticleTranslation{
		ID:              "en-article-public-article",
		DocumentID:      documentID,
		WorkspaceID:     workspaceID,
		SpaceID:         spaceID,
		CollectionID:    ptr(collectionID),
		Locale:          "en",
		Title:           "Start Here",
		Slug:            "start-here",
		Excerpt:         ptr("How to begin"),
		Content:         json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Welcome"}]}]}`),
		ContentText:     "Welcome",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
		ID:              "fr-space-public-article",
		SpaceID:         spaceID,
		WorkspaceID:     workspaceID,
		Locale:          "fr",
		Name:            "Demarrage",
		Slug:            "demarrage",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	seedDocsHelpcenterTranslationServiceCollectionTranslation(t, db, model.DocsHelpcenterCollectionTranslation{
		ID:              "fr-collection-public-article",
		CollectionID:    collectionID,
		WorkspaceID:     workspaceID,
		SpaceID:         spaceID,
		Locale:          "fr",
		Name:            "Bases",
		Slug:            "bases",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	})

	svc := newDocsHelpcenterPublicServiceForTest(db)
	article, err := svc.GetPublicArticle(ctx, workspaceID, "fr", "demarrage", "bases", "start-here")
	if err != nil {
		t.Fatalf("GetPublicArticle: %v", err)
	}
	if article == nil {
		t.Fatal("article = nil, want fallback article")
	}
	if article.Title != "Start Here" || article.Slug != "start-here" {
		t.Fatalf("article = %+v, want default-locale fallback article", article)
	}
}
