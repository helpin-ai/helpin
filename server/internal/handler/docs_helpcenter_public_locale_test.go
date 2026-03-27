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
		repository.NewDocsHelpcenterRepository(db),
		repository.NewDocsHelpcenterPublicationRepository(db),
		repository.NewDocsRedirectRepository(db),
		repository.NewDocsDocumentRepository(db),
		repository.NewDocsContentRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		nil,
	)

	helpcenterSvc := service.NewDocsHelpcenterService(
		repository.NewDocsHelpcenterRepository(db),
		repository.NewDocsHelpcenterPublicationRepository(db),
		repository.NewDocsDocumentRepository(db),
		repository.NewDocsContentRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		nil,
		nil,
	)
	helpcenterSvc.SetTranslationService(translationSvc)

	spaceSvc := service.NewDocsSpaceService(repository.NewDocsSpaceRepository(db))
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
