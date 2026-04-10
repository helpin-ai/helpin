package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupDocsHelpcenterTranslationTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-helpcenter-i18n-%d?mode=memory&cache=shared", time.Now().UnixNano())
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
			protected_terms TEXT,
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
			published_at DATETIME NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(document_id, locale),
			UNIQUE(space_id, locale, slug)
		)`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create docs helpcenter translation test table: %v", err)
		}
	}

	return db
}

func ptrString(value string) *string {
	return &value
}

func helpcenterStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func seedDocsHelpcenterTranslationSpace(t *testing.T, db *gorm.DB, space model.DocsSpace) {
	t.Helper()
	if err := db.Create(&space).Error; err != nil {
		t.Fatalf("seed docs space %s: %v", space.ID, err)
	}
}

func seedDocsHelpcenterTranslationCollection(t *testing.T, db *gorm.DB, coll model.DocsCollection) {
	t.Helper()
	if err := db.Create(&coll).Error; err != nil {
		t.Fatalf("seed docs collection %s: %v", coll.ID, err)
	}
}

func seedDocsHelpcenterTranslationDocument(t *testing.T, db *gorm.DB, doc model.DocsDocument) {
	t.Helper()
	if err := db.Create(&doc).Error; err != nil {
		t.Fatalf("seed docs document %s: %v", doc.ID, err)
	}
}

func seedDocsHelpcenterTranslationContent(t *testing.T, db *gorm.DB, content model.DocsContent) {
	t.Helper()
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("seed docs content for document %s: %v", content.DocumentID, err)
	}
}

func seedDocsHelpcenterTranslationConfig(t *testing.T, db *gorm.DB, cfg model.DocsHelpcenterConfig) {
	t.Helper()
	if err := db.Create(&cfg).Error; err != nil {
		t.Fatalf("seed docs helpcenter config for workspace %s: %v", cfg.WorkspaceID, err)
	}
}

func seedDocsHelpcenterTranslationArticle(t *testing.T, db *gorm.DB, article model.DocsHelpcenterArticle) {
	t.Helper()
	if err := db.Create(&article).Error; err != nil {
		t.Fatalf("seed docs helpcenter article for document %s: %v", article.DocumentID, err)
	}
}

func TestDocsHelpcenterTranslationRepository_DefaultLocaleUniqueness(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationTestDB(t)
	repo := NewDocsHelpcenterTranslationRepository(db)
	ctx := context.Background()

	translation, err := repo.UpsertSpaceTranslation(ctx, &model.DocsHelpcenterSpaceTranslation{
		SpaceID:      "space-1",
		WorkspaceID:  "ws-1",
		Locale:       "en",
		Name:         "English name",
		Slug:         ptrString("english-name"),
		Status:       model.DocsHelpcenterTranslationStatusPublished,
		PublishedAt:  nil,
		SourceSynced: true,
	})
	if err != nil {
		t.Fatalf("UpsertSpaceTranslation first insert: %v", err)
	}

	updated, err := repo.UpsertSpaceTranslation(ctx, &model.DocsHelpcenterSpaceTranslation{
		ID:           translation.ID,
		SpaceID:      "space-1",
		WorkspaceID:  "ws-1",
		Locale:       "en",
		Name:         "English name updated",
		Slug:         ptrString("english-name"),
		Status:       model.DocsHelpcenterTranslationStatusNeedsReview,
		SourceSynced: false,
	})
	if err != nil {
		t.Fatalf("UpsertSpaceTranslation second upsert: %v", err)
	}

	var rows []model.DocsHelpcenterSpaceTranslation
	if err := db.WithContext(ctx).
		Where("space_id = ? AND locale = ?", "space-1", "en").
		Find(&rows).Error; err != nil {
		t.Fatalf("query space translations: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("space translation rows = %d, want 1", len(rows))
	}
	if rows[0].ID != updated.ID {
		t.Fatalf("space translation id = %q, want %q", rows[0].ID, updated.ID)
	}
	if rows[0].Name != "English name updated" {
		t.Fatalf("space translation name = %q, want updated value", rows[0].Name)
	}
	if rows[0].Status != model.DocsHelpcenterTranslationStatusNeedsReview {
		t.Fatalf("space translation status = %q, want %q", rows[0].Status, model.DocsHelpcenterTranslationStatusNeedsReview)
	}
}

func TestDocsHelpcenterTranslationRepository_SpaceSlugUniquePerWorkspaceLocale(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationTestDB(t)
	repo := NewDocsHelpcenterTranslationRepository(db)
	ctx := context.Background()

	if _, err := repo.UpsertSpaceTranslation(ctx, &model.DocsHelpcenterSpaceTranslation{
		SpaceID:      "space-1",
		WorkspaceID:  "ws-1",
		Locale:       "fr",
		Name:         "Aide",
		Slug:         ptrString("centre-aide"),
		Status:       model.DocsHelpcenterTranslationStatusPublished,
		SourceSynced: true,
	}); err != nil {
		t.Fatalf("UpsertSpaceTranslation first insert: %v", err)
	}

	if _, err := repo.UpsertSpaceTranslation(ctx, &model.DocsHelpcenterSpaceTranslation{
		SpaceID:      "space-2",
		WorkspaceID:  "ws-1",
		Locale:       "fr",
		Name:         "Autre aide",
		Slug:         ptrString("centre-aide"),
		Status:       model.DocsHelpcenterTranslationStatusPublished,
		SourceSynced: true,
	}); err == nil {
		t.Fatal("expected duplicate localized space slug to fail")
	}
}

func TestDocsHelpcenterTranslationRepository_CollectionSlugUniquePerSpaceLocale(t *testing.T) {
	t.Parallel()

	db := setupDocsHelpcenterTranslationTestDB(t)
	repo := NewDocsHelpcenterTranslationRepository(db)
	ctx := context.Background()

	if _, err := repo.UpsertCollectionTranslation(ctx, &model.DocsHelpcenterCollectionTranslation{
		CollectionID: "collection-1",
		WorkspaceID:  "ws-1",
		SpaceID:      "space-1",
		Locale:       "de",
		Name:         "Erste Sammlung",
		Slug:         ptrString("erste-sammlung"),
		Status:       model.DocsHelpcenterTranslationStatusPublished,
		SourceSynced: true,
	}); err != nil {
		t.Fatalf("UpsertCollectionTranslation first insert: %v", err)
	}

	if _, err := repo.UpsertCollectionTranslation(ctx, &model.DocsHelpcenterCollectionTranslation{
		CollectionID: "collection-2",
		WorkspaceID:  "ws-1",
		SpaceID:      "space-1",
		Locale:       "de",
		Name:         "Zweite Sammlung",
		Slug:         ptrString("erste-sammlung"),
		Status:       model.DocsHelpcenterTranslationStatusPublished,
		SourceSynced: true,
	}); err == nil {
		t.Fatal("expected duplicate localized collection slug in same space to fail")
	}
}

func TestDocsHelpcenterTranslationRepository_DefaultLocaleBackfillCreatesMirrorRows(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-backfill"
		spaceID     = "space-backfill"
		collID      = "collection-backfill"
		docID       = "doc-backfill"
		userID      = "user-backfill"
	)

	db := setupDocsHelpcenterTranslationTestDB(t)
	repo := NewDocsHelpcenterTranslationRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 3, 25, 14, 0, 0, 0, time.UTC)

	hero := json.RawMessage(`{"hero_title":"Help Center"}`)
	links := json.RawMessage(`[]`)
	footer := json.RawMessage(`{}`)
	spaceNav := json.RawMessage(`{}`)

	seedDocsHelpcenterTranslationConfig(t, db, model.DocsHelpcenterConfig{
		ID:                      "cfg-backfill",
		WorkspaceID:             workspaceID,
		Subdomain:               "backfill-test",
		BrandName:               "Backfill Test",
		BrandColor:              "#000000",
		HeaderLinks:             links,
		FooterConfig:            footer,
		HomepageConfig:          hero,
		SpaceNavConfig:          spaceNav,
		DefaultLocale:           "en",
		EnabledLocales:          model.DocsStringArray{"en", "fr"},
		ShowLanguageSwitcher:    true,
		FallbackToDefaultLocale: true,
		IsPublished:             true,
		CreatedAt:               now,
		UpdatedAt:               now,
	})
	seedDocsHelpcenterTranslationSpace(t, db, model.DocsSpace{
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
	description := "Everything you need to know"
	seedDocsHelpcenterTranslationCollection(t, db, model.DocsCollection{
		ID:          collID,
		SpaceID:     spaceID,
		WorkspaceID: workspaceID,
		Name:        "Basics",
		Slug:        "basics",
		Description: &description,
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	excerpt := "How to begin"
	seedDocsHelpcenterTranslationDocument(t, db, model.DocsDocument{
		ID:           docID,
		WorkspaceID:  workspaceID,
		SpaceID:      spaceID,
		CollectionID: func() *string { v := collID; return &v }(),
		Title:        "Start Here",
		Excerpt:      &excerpt,
		Status:       model.DocStatusPublished,
		Visibility:   model.SpaceVisibilityWorkspaceWide,
		Position:     0,
		PublishedAt:  &now,
		CreatedBy:    userID,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	seedDocsHelpcenterTranslationContent(t, db, model.DocsContent{
		ID:          "content-backfill",
		DocumentID:  docID,
		Content:     json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Welcome"}]}]}`),
		ContentText: "Welcome",
		WordCount:   1,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationArticle(t, db, model.DocsHelpcenterArticle{
		ID:                "hc-backfill",
		DocumentID:        docID,
		Slug:              "start-here",
		PublicPublishedAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	})

	if err := repo.BackfillDefaultLocaleMirrors(ctx, workspaceID); err != nil {
		t.Fatalf("BackfillDefaultLocaleMirrors: %v", err)
	}
	if err := repo.BackfillDefaultLocaleMirrors(ctx, workspaceID); err != nil {
		t.Fatalf("BackfillDefaultLocaleMirrors second run: %v", err)
	}

	var spaceTranslations []model.DocsHelpcenterSpaceTranslation
	if err := db.WithContext(ctx).Where("space_id = ?", spaceID).Find(&spaceTranslations).Error; err != nil {
		t.Fatalf("query space translations: %v", err)
	}
	if len(spaceTranslations) != 1 {
		t.Fatalf("space translations = %d, want 1", len(spaceTranslations))
	}
	if spaceTranslations[0].Locale != "en" || spaceTranslations[0].Name != "Getting Started" || helpcenterStringValue(spaceTranslations[0].Slug) != "getting-started" {
		t.Fatalf("unexpected space translation: %+v", spaceTranslations[0])
	}
	if spaceTranslations[0].Status != model.DocsHelpcenterTranslationStatusPublished || !spaceTranslations[0].SourceSynced {
		t.Fatalf("unexpected space translation status/source sync: %+v", spaceTranslations[0])
	}

	var collectionTranslations []model.DocsHelpcenterCollectionTranslation
	if err := db.WithContext(ctx).Where("collection_id = ?", collID).Find(&collectionTranslations).Error; err != nil {
		t.Fatalf("query collection translations: %v", err)
	}
	if len(collectionTranslations) != 1 {
		t.Fatalf("collection translations = %d, want 1", len(collectionTranslations))
	}
	if collectionTranslations[0].Locale != "en" || collectionTranslations[0].Name != "Basics" || helpcenterStringValue(collectionTranslations[0].Slug) != "basics" {
		t.Fatalf("unexpected collection translation: %+v", collectionTranslations[0])
	}
	if collectionTranslations[0].Description == nil || *collectionTranslations[0].Description != description {
		t.Fatalf("collection translation description = %+v, want %q", collectionTranslations[0].Description, description)
	}

	var articleTranslations []model.DocsHelpcenterArticleTranslation
	if err := db.WithContext(ctx).Where("document_id = ?", docID).Find(&articleTranslations).Error; err != nil {
		t.Fatalf("query article translations: %v", err)
	}
	if len(articleTranslations) != 1 {
		t.Fatalf("article translations = %d, want 1", len(articleTranslations))
	}
	if articleTranslations[0].Locale != "en" || articleTranslations[0].Title != "Start Here" || helpcenterStringValue(articleTranslations[0].Slug) != "start-here" {
		t.Fatalf("unexpected article translation: %+v", articleTranslations[0])
	}
	if articleTranslations[0].Excerpt == nil || *articleTranslations[0].Excerpt != excerpt {
		t.Fatalf("article translation excerpt = %+v, want %q", articleTranslations[0].Excerpt, excerpt)
	}
	if string(articleTranslations[0].Content) != `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Welcome"}]}]}` {
		t.Fatalf("article translation content = %s, want mirrored content", string(articleTranslations[0].Content))
	}
	if articleTranslations[0].Status != model.DocsHelpcenterTranslationStatusPublished || !articleTranslations[0].SourceSynced {
		t.Fatalf("unexpected article translation status/source sync: %+v", articleTranslations[0])
	}
	if articleTranslations[0].PublishedAt == nil || !articleTranslations[0].PublishedAt.Equal(now) {
		t.Fatalf("article translation published_at = %+v, want %s", articleTranslations[0].PublishedAt, now.Format(time.RFC3339))
	}
}

func TestDocsHelpcenterRepository_ListPublicArticleTranslationsByCollection_FallsBackToDefaultLocaleMirror(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-public-list"
		spaceID     = "space-public-list"
		collID      = "collection-public-list"
		docID       = "doc-public-list"
		userID      = "user-public-list"
	)

	db := setupDocsHelpcenterTranslationTestDB(t)
	repo := NewDocsHelpcenterRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 4, 10, 17, 0, 0, 0, time.UTC)

	links := json.RawMessage(`[]`)
	footer := json.RawMessage(`{}`)
	hero := json.RawMessage(`{}`)
	spaceNav := json.RawMessage(`{}`)

	seedDocsHelpcenterTranslationConfig(t, db, model.DocsHelpcenterConfig{
		ID:                      "cfg-public-list",
		WorkspaceID:             workspaceID,
		Subdomain:               "replug",
		BrandName:               "Replug",
		BrandColor:              "#2b70fb",
		HeaderLinks:             links,
		FooterConfig:            footer,
		HomepageConfig:          hero,
		SpaceNavConfig:          spaceNav,
		DefaultLocale:           "en",
		EnabledLocales:          model.DocsStringArray{"en", "fr"},
		FallbackToDefaultLocale: true,
		IsPublished:             true,
		CreatedAt:               now,
		UpdatedAt:               now,
	})
	seedDocsHelpcenterTranslationSpace(t, db, model.DocsSpace{
		ID:          spaceID,
		WorkspaceID: workspaceID,
		Name:        "Help Center",
		Slug:        "help-center",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeExternalCapable,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationCollection(t, db, model.DocsCollection{
		ID:          collID,
		SpaceID:     spaceID,
		WorkspaceID: workspaceID,
		Name:        "Getting Started",
		Slug:        "getting-started",
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	excerpt := "Start here"
	seedDocsHelpcenterTranslationDocument(t, db, model.DocsDocument{
		ID:           docID,
		WorkspaceID:  workspaceID,
		SpaceID:      spaceID,
		CollectionID: ptrString(collID),
		Title:        "Legacy source title",
		Excerpt:      &excerpt,
		Status:       model.DocStatusPublished,
		Visibility:   model.SpaceVisibilityWorkspaceWide,
		PublishedAt:  &now,
		CreatedBy:    userID,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	seedDocsHelpcenterTranslationArticle(t, db, model.DocsHelpcenterArticle{
		ID:                "hc-public-list",
		DocumentID:        docID,
		Slug:              "legacy-source-title",
		PublicPublishedAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	})

	if err := db.Create(&model.DocsHelpcenterArticleTranslation{
		ID:              "hat-public-en",
		DocumentID:      docID,
		WorkspaceID:     workspaceID,
		SpaceID:         spaceID,
		CollectionID:    ptrString(collID),
		Locale:          "en",
		Title:           "Published from mirror",
		Slug:            ptrString("published-from-mirror"),
		Excerpt:         ptrString("Mirror excerpt"),
		Content:         json.RawMessage(`{"type":"doc","content":[]}`),
		ContentText:     "Mirror body",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed default locale article translation: %v", err)
	}
	if err := db.Create(&model.DocsHelpcenterArticleTranslation{
		ID:              "hat-public-fr",
		DocumentID:      docID,
		WorkspaceID:     workspaceID,
		SpaceID:         spaceID,
		CollectionID:    ptrString(collID),
		Locale:          "fr",
		Title:           "Publication manquante",
		Slug:            ptrString("publication-manquante"),
		Excerpt:         ptrString("FR excerpt"),
		Content:         json.RawMessage(`{"type":"doc","content":[]}`),
		ContentText:     "FR body",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed non-default locale article translation: %v", err)
	}

	translations, err := repo.ListPublicArticleTranslationsByCollection(ctx, collID, "en")
	if err != nil {
		t.Fatalf("ListPublicArticleTranslationsByCollection default locale fallback: %v", err)
	}
	if len(translations) != 1 {
		t.Fatalf("default locale public translations = %d, want 1", len(translations))
	}
	if translations[0].Title != "Published from mirror" || helpcenterStringValue(translations[0].Slug) != "published-from-mirror" {
		t.Fatalf("unexpected default locale translation: %+v", translations[0])
	}

	frTranslations, err := repo.ListPublicArticleTranslationsByCollection(ctx, collID, "fr")
	if err != nil {
		t.Fatalf("ListPublicArticleTranslationsByCollection non-default locale: %v", err)
	}
	if len(frTranslations) != 0 {
		t.Fatalf("non-default locale public translations = %d, want 0 without publication snapshot", len(frTranslations))
	}
}

func TestDocsHelpcenterRepository_GetPublicArticleTranslationByCollectionSlug_FallsBackToDefaultLocaleMirror(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-public-get"
		spaceID     = "space-public-get"
		collID      = "collection-public-get"
		docID       = "doc-public-get"
		userID      = "user-public-get"
	)

	db := setupDocsHelpcenterTranslationTestDB(t)
	repo := NewDocsHelpcenterRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 4, 10, 17, 30, 0, 0, time.UTC)

	links := json.RawMessage(`[]`)
	footer := json.RawMessage(`{}`)
	hero := json.RawMessage(`{}`)
	spaceNav := json.RawMessage(`{}`)

	seedDocsHelpcenterTranslationConfig(t, db, model.DocsHelpcenterConfig{
		ID:                      "cfg-public-get",
		WorkspaceID:             workspaceID,
		Subdomain:               "replug",
		BrandName:               "Replug",
		BrandColor:              "#2b70fb",
		HeaderLinks:             links,
		FooterConfig:            footer,
		HomepageConfig:          hero,
		SpaceNavConfig:          spaceNav,
		DefaultLocale:           "en",
		EnabledLocales:          model.DocsStringArray{"en"},
		FallbackToDefaultLocale: true,
		IsPublished:             true,
		CreatedAt:               now,
		UpdatedAt:               now,
	})
	seedDocsHelpcenterTranslationSpace(t, db, model.DocsSpace{
		ID:          spaceID,
		WorkspaceID: workspaceID,
		Name:        "Help Center",
		Slug:        "help-center",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeExternalCapable,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationCollection(t, db, model.DocsCollection{
		ID:          collID,
		SpaceID:     spaceID,
		WorkspaceID: workspaceID,
		Name:        "Getting Started",
		Slug:        "getting-started",
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationDocument(t, db, model.DocsDocument{
		ID:           docID,
		WorkspaceID:  workspaceID,
		SpaceID:      spaceID,
		CollectionID: ptrString(collID),
		Title:        "Legacy source title",
		Status:       model.DocStatusPublished,
		Visibility:   model.SpaceVisibilityWorkspaceWide,
		PublishedAt:  &now,
		CreatedBy:    userID,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	seedDocsHelpcenterTranslationArticle(t, db, model.DocsHelpcenterArticle{
		ID:                "hc-public-get",
		DocumentID:        docID,
		Slug:              "legacy-source-title",
		PublicPublishedAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	})

	if err := db.Create(&model.DocsHelpcenterArticleTranslation{
		ID:              "hat-public-get-en",
		DocumentID:      docID,
		WorkspaceID:     workspaceID,
		SpaceID:         spaceID,
		CollectionID:    ptrString(collID),
		Locale:          "en",
		Title:           "Published from mirror",
		Slug:            ptrString("published-from-mirror"),
		Excerpt:         ptrString("Mirror excerpt"),
		Content:         json.RawMessage(`{"type":"doc","content":[]}`),
		ContentText:     "Mirror body",
		Status:          model.DocsHelpcenterTranslationStatusPublished,
		SourceUpdatedAt: &now,
		SourceSynced:    true,
		PublishedAt:     &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("seed default locale article translation: %v", err)
	}

	translation, err := repo.GetPublicArticleTranslationByCollectionSlug(ctx, collID, "en", "published-from-mirror")
	if err != nil {
		t.Fatalf("GetPublicArticleTranslationByCollectionSlug default locale fallback: %v", err)
	}
	if translation == nil {
		t.Fatal("expected article translation from default locale mirror fallback")
	}
	if translation.Title != "Published from mirror" || helpcenterStringValue(translation.Slug) != "published-from-mirror" {
		t.Fatalf("unexpected translation: %+v", translation)
	}
}
