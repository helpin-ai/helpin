package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type scriptedDocsTranslationHandlerLLM struct {
	response string
}

func (f *scriptedDocsTranslationHandlerLLM) ChatCompletion(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: f.response}, nil
}

func setupDocsHelpcenterTranslationHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-helpcenter-handler-i18n-%d?mode=memory&cache=shared", time.Now().UnixNano())
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
			sort_key TEXT NOT NULL DEFAULT '~',
			created_by TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE docs_collections (
			id TEXT PRIMARY KEY,
			space_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			parent_collection_id TEXT,
			depth INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			public_id TEXT NOT NULL DEFAULT '',
			slug TEXT NOT NULL DEFAULT '',
			description TEXT,
			icon TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			sort_key TEXT NOT NULL DEFAULT '~',
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
			sort_key TEXT NOT NULL DEFAULT '~',
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
		`CREATE TABLE docs_change_proposals (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_configs (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL UNIQUE,
			subdomain TEXT NOT NULL,
			custom_domain TEXT,
			custom_domain_status TEXT,
			custom_domain_token TEXT,
			custom_domain_verified_at DATETIME,
			custom_domain_checked_at DATETIME,
			custom_domain_last_error TEXT,
			custom_domain_failing_since DATETIME,
			custom_domain_alerted_status TEXT,
			public_url_mode TEXT NOT NULL DEFAULT 'hosted_subdomain',
			reverse_proxy_host TEXT,
			reverse_proxy_base_path TEXT,
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
			protected_terms TEXT,
			show_language_switcher BOOLEAN NOT NULL DEFAULT 0,
			fallback_to_default_locale BOOLEAN NOT NULL DEFAULT 1,
			is_published BOOLEAN NOT NULL DEFAULT 0,
			chat_widget_enabled BOOLEAN NOT NULL DEFAULT 1,
			ai_answers_enabled BOOLEAN NOT NULL DEFAULT 1,
			seo_title TEXT,
			seo_description TEXT,
			og_title TEXT,
			og_description TEXT,
			og_image_url TEXT,
			og_image_alt TEXT,
			support_email TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_articles (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE,
			public_id TEXT NOT NULL DEFAULT '',
			slug TEXT NOT NULL DEFAULT '',
			seo_title TEXT,
			seo_description TEXT,
			og_title TEXT,
			og_description TEXT,
			og_image_url TEXT,
			og_image_alt TEXT,
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
			slug TEXT,
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
			slug TEXT,
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
			slug TEXT,
			excerpt TEXT,
			content JSON,
			content_text TEXT,
			seo_title TEXT,
			seo_description TEXT,
			og_title TEXT,
			og_description TEXT,
			og_image_url TEXT,
			og_image_alt TEXT,
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
		`CREATE TABLE docs_helpcenter_article_publications (
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
			og_title TEXT,
			og_description TEXT,
			og_image_url TEXT,
			og_image_alt TEXT,
			published_at DATETIME NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(document_id, locale),
			UNIQUE(space_id, locale, slug)
		)`,
		`CREATE TABLE docs_helpcenter_search_entries (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			locale TEXT NOT NULL,
			entry_key TEXT NOT NULL,
			entry_type TEXT NOT NULL,
			content TEXT NOT NULL,
			section_title TEXT,
			anchor TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			rank_weight REAL NOT NULL DEFAULT 1,
			search_config TEXT NOT NULL DEFAULT 'simple',
			search_vector TEXT NOT NULL DEFAULT '',
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create docs helpcenter translation handler test table: %v", err)
		}
	}

	return db
}

func seedDocsHelpcenterTranslationHandlerFixture(t *testing.T, db *gorm.DB, now time.Time) {
	t.Helper()

	const (
		workspaceID  = "ws-handler-i18n"
		userID       = "user-handler-i18n"
		spaceID      = "space-handler-i18n"
		collectionID = "collection-handler-i18n"
		documentID   = "document-handler-i18n"
	)

	jsonEmptyArray := json.RawMessage(`[]`)
	jsonEmptyObject := json.RawMessage(`{}`)
	excerpt := "How to begin"

	records := []any{
		&model.DocsHelpcenterConfig{
			ID:                      "cfg-handler-i18n",
			WorkspaceID:             workspaceID,
			Subdomain:               "handler-i18n",
			BrandName:               "Handler I18n",
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
		},
		&model.DocsSpace{
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
		},
		&model.DocsCollection{
			ID:          collectionID,
			SpaceID:     spaceID,
			WorkspaceID: workspaceID,
			Name:        "Basics",
			Slug:        "basics",
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		&model.DocsDocument{
			ID:           documentID,
			WorkspaceID:  workspaceID,
			SpaceID:      spaceID,
			CollectionID: func() *string { v := collectionID; return &v }(),
			Title:        "Start Here",
			Excerpt:      &excerpt,
			Status:       model.DocStatusPublished,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     0,
			PublishedAt:  &now,
			CreatedBy:    userID,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		&model.DocsContent{
			ID:          "content-handler-i18n",
			DocumentID:  documentID,
			Content:     json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Welcome to Helpin."}]}]}`),
			ContentText: "Welcome to Helpin.",
			WordCount:   3,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		&model.DocsHelpcenterArticle{
			ID:                "article-handler-i18n",
			DocumentID:        documentID,
			Slug:              "start-here",
			PublicID:          "handler123",
			PublicPublishedAt: &now,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		&model.DocsHelpcenterArticlePublication{
			ID:           "pub-handler-i18n-en",
			DocumentID:   documentID,
			WorkspaceID:  workspaceID,
			SpaceID:      spaceID,
			CollectionID: func() *string { v := collectionID; return &v }(),
			Locale:       "en",
			Title:        "Start Here",
			Slug:         "start-here",
			Excerpt:      &excerpt,
			Content:      json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Welcome to Helpin."}]}]}`),
			ContentText:  "Welcome to Helpin.",
			PublishedAt:  now,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("seed fixture %T: %v", record, err)
		}
	}
}

func newDocsHelpcenterTranslationHandlerForTest(db *gorm.DB) *DocsHandler {
	return newDocsHelpcenterTranslationHandlerForTestWithLLM(db, nil)
}

func newDocsHelpcenterTranslationHandlerForTestWithLLM(db *gorm.DB, llmProvider llm.Provider) *DocsHandler {
	translationSvc := service.NewDocsHelpcenterTranslationService(
		repository.NewDocsHelpcenterTranslationRepository(db),
		repository.NewDocsHelpcenterRepository(db, false),
		repository.NewDocsHelpcenterPublicationRepository(db),
		repository.NewDocsRedirectRepository(db),
		repository.NewDocsDocumentRepository(db, false),
		repository.NewDocsContentRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db, false),
		llmProvider,
	)
	return &DocsHandler{translationSvc: translationSvc}
}

func withWorkspaceAndRoute(req *http.Request, workspaceID string, params map[string]string) *http.Request {
	ctx := middleware.WithWorkspaceID(req.Context(), workspaceID)
	routeCtx := chi.NewRouteContext()
	for key, value := range params {
		routeCtx.URLParams.Add(key, value)
	}
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)
	return req.WithContext(ctx)
}

func TestDocsHelpcenterTranslationHandler_UpdateLocales(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 3, 25, 18, 0, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)
	h := newDocsHelpcenterTranslationHandlerForTest(db)

	req := httptest.NewRequest(http.MethodPut, "/api/docs/helpcenter/locales", strings.NewReader(`{"default_locale":"fr","enabled_locales":["en","fr"],"show_language_switcher":true,"fallback_to_default_locale":false}`))
	req.Header.Set("Content-Type", "application/json")
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", nil)
	rec := httptest.NewRecorder()

	h.UpdateHelpcenterLocales(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"default_locale":"fr"`) {
		t.Fatalf("body = %s, want updated default locale", body)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"fallback_to_default_locale":false`) {
		t.Fatalf("body = %s, want updated fallback flag", body)
	}
}

func TestDocsHelpcenterTranslationHandler_GetLocalesNormalizesEmptyArray(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 3, 25, 18, 0, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)
	h := newDocsHelpcenterTranslationHandlerForTest(db)

	if err := db.Exec(`UPDATE docs_helpcenter_configs SET enabled_locales = NULL WHERE workspace_id = ?`, "ws-handler-i18n").Error; err != nil {
		t.Fatalf("null enabled_locales: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/docs/helpcenter/locales", nil)
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", nil)
	rec := httptest.NewRecorder()

	h.GetHelpcenterLocales(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if body := rec.Body.String(); strings.Contains(body, `"enabled_locales":null`) {
		t.Fatalf("body = %s, want normalized enabled locales array", body)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"enabled_locales":["en"]`) {
		t.Fatalf("body = %s, want default locale fallback", body)
	}
}

func TestDocsHelpcenterTranslationHandler_UpsertArticleTranslation(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 3, 25, 18, 0, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)
	h := newDocsHelpcenterTranslationHandlerForTest(db)

	req := httptest.NewRequest(http.MethodPut, "/api/docs/documents/document-handler-i18n/helpcenter/translations", strings.NewReader(`{"locale":"fr","title":"Commencer ici","slug":"commencer-ici","excerpt":"Guide rapide","content":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Bonjour"}]}]},"seo_title":"Bonjour","seo_description":"Guide FR","status":"draft"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", map[string]string{"docId": "document-handler-i18n"})
	rec := httptest.NewRecorder()

	h.UpsertArticleTranslation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"locale":"fr"`) || !strings.Contains(body, `"slug":"commencer-ici"`) {
		t.Fatalf("body = %s, want created article translation", body)
	}
}

func TestDocsHelpcenterTranslationHandler_PublishArticleTranslationRejectsMissingParents(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 3, 25, 18, 0, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)
	if err := db.Create(&model.DocsHelpcenterArticleTranslation{
		ID:              "fr-article-handler",
		DocumentID:      "document-handler-i18n",
		WorkspaceID:     "ws-handler-i18n",
		SpaceID:         "space-handler-i18n",
		CollectionID:    func() *string { v := "collection-handler-i18n"; return &v }(),
		Locale:          "fr",
		Title:           "Commencer ici",
		Slug:            func() *string { v := "commencer-ici"; return &v }(),
		Status:          model.DocsHelpcenterTranslationStatusDraft,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed fr article translation: %v", err)
	}
	h := newDocsHelpcenterTranslationHandlerForTest(db)

	req := httptest.NewRequest(http.MethodPost, "/api/docs/documents/document-handler-i18n/helpcenter/translations/fr/publish", nil)
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", map[string]string{"docId": "document-handler-i18n", "locale": "fr"})
	rec := httptest.NewRecorder()

	h.PublishArticleTranslation(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "published space translation is required") {
		t.Fatalf("body = %s, want missing parent error", body)
	}
}

func TestDocsHelpcenterTranslationHandler_GenerateArticleTranslationDraft(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 3, 25, 18, 0, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)
	h := newDocsHelpcenterTranslationHandlerForTestWithLLM(db, &scriptedDocsTranslationHandlerLLM{
		response: `{"segments":[{"id":"meta:title","translated_text":"Commencer ici"},{"id":"meta:excerpt","translated_text":"Guide rapide"},{"id":"doc/0","translated_text":"Bienvenue dans Helpin."}]}`,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/docs/documents/document-handler-i18n/helpcenter/translations/fr/generate", nil)
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", map[string]string{"docId": "document-handler-i18n", "locale": "fr"})
	rec := httptest.NewRecorder()

	h.GenerateArticleTranslationDraft(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"locale":"fr"`) || !strings.Contains(body, `"slug":null`) || !strings.Contains(body, `Bienvenue dans Helpin.`) {
		t.Fatalf("body = %s, want generated article translation draft", body)
	}
}
