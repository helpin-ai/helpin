package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func newDocsHelpcenterPublicHandlerForTest(db *gorm.DB) *DocsHandler {
	translationSvc := service.NewDocsHelpcenterTranslationService(
		repository.NewDocsHelpcenterTranslationRepository(db),
		repository.NewDocsHelpcenterRepository(db, false),
		repository.NewDocsHelpcenterPublicationRepository(db),
		repository.NewDocsRedirectRepository(db),
		repository.NewDocsDocumentRepository(db, false),
		repository.NewDocsContentRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db, false),
		nil,
	)

	helpcenterSvc := service.NewDocsHelpcenterService(
		repository.NewDocsHelpcenterRepository(db, false),
		repository.NewDocsHelpcenterPublicationRepository(db),
		repository.NewDocsDocumentRepository(db, false),
		repository.NewDocsContentRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db, false),
		nil,
		nil,
		nil,
	)
	helpcenterSvc.SetTranslationService(translationSvc)

	spaceSvc := service.NewDocsSpaceService(repository.NewDocsSpaceRepository(db), nil)
	spaceSvc.SetTranslationService(translationSvc)

	return &DocsHandler{
		spaceSvc:       spaceSvc,
		helpcenterSvc:  helpcenterSvc,
		translationSvc: translationSvc,
		searchSvc:      service.NewDocsSearchService(repository.NewDocsSearchRepository(db)),
	}
}

func TestDocsHelpcenterPublicLocale_DisabledLocaleRedirectsToDefault(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 3, 25, 20, 0, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)
	h := newDocsHelpcenterPublicHandlerForTest(db)

	req := httptest.NewRequest(http.MethodGet, "/api/hc/handler-i18n/es/spaces", nil)
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", map[string]string{
		"subdomain": "handler-i18n",
		"locale":    "es",
	})
	rec := httptest.NewRecorder()

	h.PublicGetSpaces(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusFound, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/api/hc/handler-i18n/en/spaces" {
		t.Fatalf("location = %q, want %q", location, "/api/hc/handler-i18n/en/spaces")
	}
}

func TestDocsHelpcenterPublicLocale_SingleLocaleSpacesDoesNotRedirect(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 3, 25, 20, 5, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)
	if err := db.Exec(`
		UPDATE docs_helpcenter_configs
		SET default_locale = ?, enabled_locales = ?, show_language_switcher = ?, fallback_to_default_locale = ?
		WHERE workspace_id = ?
	`, "en", `["en"]`, false, true, "ws-handler-i18n").Error; err != nil {
		t.Fatalf("restrict help center to single locale: %v", err)
	}
	h := newDocsHelpcenterPublicHandlerForTest(db)

	req := httptest.NewRequest(http.MethodGet, "/api/hc/handler-i18n/spaces", nil)
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", map[string]string{
		"subdomain": "handler-i18n",
	})
	rec := httptest.NewRecorder()

	h.PublicGetSpaces(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var spaces []model.PublicSpaceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &spaces); err != nil {
		t.Fatalf("unmarshal spaces: %v, body = %s", err, rec.Body.String())
	}
	if len(spaces) == 0 {
		t.Fatalf("len(spaces) = %d, want at least 1, body = %s", len(spaces), rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "" {
		t.Fatalf("location = %q, want empty", location)
	}
}

func TestDocsHelpcenterPublicLocale_ConfigDoesNotMutateCollectionMirrors(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 3, 25, 20, 7, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)
	if err := db.Exec(`
		UPDATE docs_helpcenter_configs
		SET default_locale = ?, enabled_locales = ?, show_language_switcher = ?, fallback_to_default_locale = ?
		WHERE workspace_id = ?
	`, "en", `["en"]`, false, true, "ws-handler-i18n").Error; err != nil {
		t.Fatalf("restrict help center to single locale: %v", err)
	}
	if err := db.Exec(`UPDATE docs_collections SET slug = '' WHERE id = ?`, "collection-handler-i18n").Error; err != nil {
		t.Fatalf("clear collection slug: %v", err)
	}
	if err := db.Exec(`UPDATE docs_helpcenter_collection_translations SET slug = NULL WHERE collection_id = ? AND locale = ?`, "collection-handler-i18n", "en").Error; err != nil {
		t.Fatalf("clear collection translation slug: %v", err)
	}
	h := newDocsHelpcenterPublicHandlerForTest(db)

	req := httptest.NewRequest(http.MethodGet, "/api/hc/handler-i18n/config", nil)
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", map[string]string{
		"subdomain": "handler-i18n",
	})
	rec := httptest.NewRecorder()

	h.PublicGetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != helpcenterCachePublicRead {
		t.Fatalf("Cache-Control = %q, want %q", got, helpcenterCachePublicRead)
	}

	var storedCollection model.DocsCollection
	if err := db.Where("id = ?", "collection-handler-i18n").First(&storedCollection).Error; err != nil {
		t.Fatalf("load collection: %v", err)
	}
	if storedCollection.Slug != "" {
		t.Fatalf("stored collection slug = %q, want empty", storedCollection.Slug)
	}
}

func TestDocsHelpcenterPublicLocale_ConfigFiltersInvalidFeaturedCollectionCards(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 4, 8, 20, 12, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)

	homepageConfig := json.RawMessage(`{
		"hero_title":"How can we help?",
		"hero_subtitle":"Search our knowledge base or browse topics below",
		"featured_cards":[
			{
				"title":"Welcome & Quick Start",
				"description":"",
				"icon":"airplay",
				"link_type":"collection",
				"link_value":"",
				"space_slug":"getting-started"
			},
			{
				"title":"Basics",
				"description":"Old description",
				"icon":"rocket",
				"link_type":"collection",
				"link_value":"basics",
				"space_slug":"getting-started"
			}
		]
	}`)
	if err := db.Exec(`UPDATE docs_helpcenter_configs SET homepage_config = ? WHERE workspace_id = ?`, homepageConfig, "ws-handler-i18n").Error; err != nil {
		t.Fatalf("update homepage config: %v", err)
	}

	h := newDocsHelpcenterPublicHandlerForTest(db)
	req := httptest.NewRequest(http.MethodGet, "/api/hc/handler-i18n/config", nil)
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", map[string]string{
		"subdomain": "handler-i18n",
	})
	rec := httptest.NewRecorder()

	h.PublicGetConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var cfg model.DocsHelpcenterConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v, body = %s", err, rec.Body.String())
	}

	var homepage model.HelpcenterHomepageConfig
	if err := json.Unmarshal(cfg.HomepageConfig, &homepage); err != nil {
		t.Fatalf("unmarshal homepage config: %v, raw = %s", err, string(cfg.HomepageConfig))
	}

	if len(homepage.FeaturedCards) != 1 {
		t.Fatalf("len(featured_cards) = %d, want %d", len(homepage.FeaturedCards), 1)
	}
	if got := homepage.FeaturedCards[0].Title; got != "Basics" {
		t.Fatalf("featured_cards[0].title = %q, want %q", got, "Basics")
	}
	if got := homepage.FeaturedCards[0].LinkValue; got != "basics" {
		t.Fatalf("featured_cards[0].link_value = %q, want %q", got, "basics")
	}
}

func TestDocsHelpcenterPublicLocale_NavigationBackfillsMissingCollectionSlug(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 3, 25, 20, 8, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)
	if err := db.Exec(`
		UPDATE docs_helpcenter_configs
		SET default_locale = ?, enabled_locales = ?, show_language_switcher = ?, fallback_to_default_locale = ?
		WHERE workspace_id = ?
	`, "en", `["en"]`, false, true, "ws-handler-i18n").Error; err != nil {
		t.Fatalf("restrict help center to single locale: %v", err)
	}
	if err := db.Exec(`UPDATE docs_collections SET slug = '' WHERE id = ?`, "collection-handler-i18n").Error; err != nil {
		t.Fatalf("clear collection slug: %v", err)
	}
	if err := db.Exec(`UPDATE docs_helpcenter_collection_translations SET slug = NULL WHERE collection_id = ? AND locale = ?`, "collection-handler-i18n", "en").Error; err != nil {
		t.Fatalf("clear collection translation slug: %v", err)
	}
	h := newDocsHelpcenterPublicHandlerForTest(db)

	req := httptest.NewRequest(http.MethodGet, "/api/hc/handler-i18n/spaces/getting-started/navigation", nil)
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", map[string]string{
		"subdomain": "handler-i18n",
		"spaceSlug": "getting-started",
	})
	rec := httptest.NewRecorder()

	h.PublicGetSpaceNavigation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var nav []model.PublicNavCollection
	if err := json.Unmarshal(rec.Body.Bytes(), &nav); err != nil {
		t.Fatalf("unmarshal nav: %v, body = %s", err, rec.Body.String())
	}
	if len(nav) == 0 {
		t.Fatalf("len(nav) = %d, want at least 1, body = %s", len(nav), rec.Body.String())
	}
	if nav[0].Slug != "basics" {
		t.Fatalf("nav[0].slug = %q, want %q", nav[0].Slug, "basics")
	}
	if len(nav[0].Articles) == 0 {
		t.Fatalf("len(nav[0].articles) = %d, want at least 1", len(nav[0].Articles))
	}
	if nav[0].Articles[0].PublicID != "handler123" {
		t.Fatalf("nav[0].articles[0].public_id = %q, want %q", nav[0].Articles[0].PublicID, "handler123")
	}

	var storedCollection model.DocsCollection
	if err := db.Where("id = ?", "collection-handler-i18n").First(&storedCollection).Error; err != nil {
		t.Fatalf("load collection: %v", err)
	}
	if storedCollection.Slug != "basics" {
		t.Fatalf("stored collection slug = %q, want %q", storedCollection.Slug, "basics")
	}
}

func TestDocsHelpcenterPublicLocale_SearchOnlyReturnsRequestedLocale(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 3, 25, 20, 15, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)

	ptr := func(value string) *string { return &value }

	if err := db.Create(&model.DocsHelpcenterSpaceTranslation{
		ID:              "en-space-public-search",
		SpaceID:         "space-handler-i18n",
		WorkspaceID:     "ws-handler-i18n",
		Locale:          "en",
		Name:            "Getting Started",
		Slug:            ptr("getting-started"),
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed en space translation: %v", err)
	}
	if err := db.Create(&model.DocsHelpcenterSpaceTranslation{
		ID:              "fr-space-public-search",
		SpaceID:         "space-handler-i18n",
		WorkspaceID:     "ws-handler-i18n",
		Locale:          "fr",
		Name:            "Demarrage",
		Slug:            ptr("demarrage"),
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed fr space translation: %v", err)
	}
	if err := db.Create(&model.DocsHelpcenterCollectionTranslation{
		ID:              "en-collection-public-search",
		CollectionID:    "collection-handler-i18n",
		WorkspaceID:     "ws-handler-i18n",
		SpaceID:         "space-handler-i18n",
		Locale:          "en",
		Name:            "Basics",
		Slug:            ptr("basics"),
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed en collection translation: %v", err)
	}
	if err := db.Create(&model.DocsHelpcenterCollectionTranslation{
		ID:              "fr-collection-public-search",
		CollectionID:    "collection-handler-i18n",
		WorkspaceID:     "ws-handler-i18n",
		SpaceID:         "space-handler-i18n",
		Locale:          "fr",
		Name:            "Bases",
		Slug:            ptr("bases"),
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed fr collection translation: %v", err)
	}
	if err := db.Create(&model.DocsHelpcenterArticleTranslation{
		ID:              "en-article-public-search",
		DocumentID:      "document-handler-i18n",
		WorkspaceID:     "ws-handler-i18n",
		SpaceID:         "space-handler-i18n",
		CollectionID:    ptr("collection-handler-i18n"),
		Locale:          "en",
		Title:           "Bonjour from English",
		Slug:            ptr("bonjour-from-english"),
		Content:         json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"bonjour in english"}]}]}`),
		ContentText:     "bonjour in english",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed en article translation: %v", err)
	}

	if err := db.Create(&model.DocsDocument{
		ID:           "document-handler-i18n-fr",
		WorkspaceID:  "ws-handler-i18n",
		SpaceID:      "space-handler-i18n",
		CollectionID: ptr("collection-handler-i18n"),
		Title:        "Canonical French Source",
		Status:       model.DocStatusPublished,
		Visibility:   model.SpaceVisibilityWorkspaceWide,
		Position:     1,
		PublishedAt:  &now,
		CreatedBy:    "user-handler-i18n",
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed fr document: %v", err)
	}
	if err := db.Create(&model.DocsHelpcenterArticle{
		ID:                "article-handler-i18n-fr",
		DocumentID:        "document-handler-i18n-fr",
		Slug:              "canonical-french-source",
		PublicPublishedAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error; err != nil {
		t.Fatalf("seed fr helpcenter article: %v", err)
	}
	if err := db.Create(&model.DocsHelpcenterArticleTranslation{
		ID:              "fr-article-public-search",
		DocumentID:      "document-handler-i18n-fr",
		WorkspaceID:     "ws-handler-i18n",
		SpaceID:         "space-handler-i18n",
		CollectionID:    ptr("collection-handler-i18n"),
		Locale:          "fr",
		Title:           "Bonjour en francais",
		Slug:            ptr("bonjour-fr"),
		Content:         json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"bonjour depuis le centre d'aide"}]}]}`),
		ContentText:     "bonjour depuis le centre d'aide",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed fr article translation: %v", err)
	}
	if err := db.Create(&model.DocsHelpcenterArticlePublication{
		ID:           "pub-handler-i18n-fr",
		DocumentID:   "document-handler-i18n-fr",
		WorkspaceID:  "ws-handler-i18n",
		SpaceID:      "space-handler-i18n",
		CollectionID: ptr("collection-handler-i18n"),
		Locale:       "fr",
		Title:        "Bonjour en francais",
		Slug:         "bonjour-fr",
		Content:      json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"bonjour depuis le centre d'aide"}]}]}`),
		ContentText:  "bonjour depuis le centre d'aide",
		PublishedAt:  now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed fr article publication: %v", err)
	}
	if err := db.Create(&model.DocsHelpcenterSearchEntry{
		ID:           "search-handler-i18n-fr-title",
		WorkspaceID:  "ws-handler-i18n",
		DocumentID:   "document-handler-i18n-fr",
		Locale:       "fr",
		EntryKey:     "title",
		EntryType:    model.DocsHelpcenterSearchEntryTypeTitle,
		Content:      "Bonjour en francais",
		Position:     0,
		RankWeight:   8,
		SearchConfig: "simple",
	}).Error; err != nil {
		t.Fatalf("seed fr search entry: %v", err)
	}

	h := newDocsHelpcenterPublicHandlerForTest(db)
	req := httptest.NewRequest(http.MethodGet, "/api/hc/handler-i18n/fr/search?q=bonjour", nil)
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", map[string]string{
		"subdomain": "handler-i18n",
		"locale":    "fr",
	})
	rec := httptest.NewRecorder()

	h.PublicSearchArticles(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var results []model.PublicSearchResultResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal results: %v, body = %s", err, rec.Body.String())
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1, body = %s", len(results), rec.Body.String())
	}
	if results[0].Title != "Bonjour en francais" {
		t.Fatalf("first result = %+v, want French translation result", results[0])
	}
	if strings.Contains(rec.Body.String(), "Bonjour from English") {
		t.Fatalf("body = %s, want no English translation results", rec.Body.String())
	}
}

func TestDocsHelpcenterPublicBootstrap_ReturnsLocaleConfigAndSpaces(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationHandlerTestDB(t)
	now := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	seedDocsHelpcenterTranslationHandlerFixture(t, db, now)
	h := newDocsHelpcenterPublicHandlerForTest(db)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/hc/handler-i18n/bootstrap?path=%2Ffr%2Farticles%2Fbonjour-a1b2c3d4",
		nil,
	)
	req = withWorkspaceAndRoute(req, "ws-handler-i18n", map[string]string{
		"subdomain": "handler-i18n",
	})
	rec := httptest.NewRecorder()

	h.PublicGetBootstrap(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response publicHelpcenterBootstrapResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal bootstrap: %v, body = %s", err, rec.Body.String())
	}
	if response.Locale != "fr" {
		t.Fatalf("locale = %q, want fr", response.Locale)
	}
	if response.Config.DocsHelpcenterConfig == nil {
		t.Fatal("config is nil")
	}
	if len(response.Spaces) == 0 {
		t.Fatal("spaces are empty")
	}
	if cacheControl := rec.Header().Get("Cache-Control"); cacheControl == "" {
		t.Fatal("Cache-Control header is empty")
	}
}
