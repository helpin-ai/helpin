package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

type captureSupportEventRecorder struct {
	events []SupportEventInput
}

func (r *captureSupportEventRecorder) RecordEventBestEffort(input SupportEventInput) {
	r.events = append(r.events, input)
}

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
		allowed_origins TEXT NOT NULL DEFAULT '{}',
		identity_verification_mode TEXT NOT NULL DEFAULT 'report_only',
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
		repository.NewDocsCollectionRepository(db, false),
		repository.NewDocsHelpcenterRepository(db, false),
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

func TestSupportInboxServiceSearchWidgetHelpArticles_UsesSelectedHelpSpaces(t *testing.T) {
	t.Parallel()

	const (
		workspaceID    = "ws-widget-help-search"
		widgetKey      = "wk-widget-help-search"
		allowedSpaceID = "space-search-allowed"
		otherSpaceID   = "space-search-other"
		allowedDocID   = "doc-search-allowed"
		otherDocID     = "doc-search-other"
	)

	now := time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC)
	ctx := context.Background()
	db := setupDocsHelpcenterTranslationServiceTestDB(t)

	mustExec(t, db, `CREATE TABLE support_widget_installations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		widget_key TEXT NOT NULL,
		secret_key TEXT NOT NULL,
		allowed_origins TEXT NOT NULL DEFAULT '{}',
		identity_verification_mode TEXT NOT NULL DEFAULT 'report_only',
		settings TEXT NOT NULL DEFAULT '{}',
		active BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE docs_helpcenter_search_entries (
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
	)`)

	seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
		ID:            "cfg-widget-help-search",
		WorkspaceID:   workspaceID,
		Subdomain:     "widget-help-search",
		BrandName:     "Widget Help Search",
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
		Name:        "Allowed Docs",
		Slug:        "allowed-docs",
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
	allowedSpaceSlug := "allowed-docs"
	otherSpaceSlug := "other-docs"
	seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
		ID:          "st-allowed",
		SpaceID:     allowedSpaceID,
		WorkspaceID: workspaceID,
		Locale:      "en",
		Name:        "Allowed Docs",
		Slug:        &allowedSpaceSlug,
		Status:      model.DocsHelpcenterTranslationStatusPublished,
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
		ID:          "st-other",
		SpaceID:     otherSpaceID,
		WorkspaceID: workspaceID,
		Locale:      "en",
		Name:        "Other Docs",
		Slug:        &otherSpaceSlug,
		Status:      model.DocsHelpcenterTranslationStatusPublished,
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	seedSearchableWidgetHelpArticle(t, db, workspaceID, allowedSpaceID, allowedDocID, "allowed-reset", "Allowed reset password", "allowed-reset-password", "aa11bb22", now)
	seedSearchableWidgetHelpArticle(t, db, workspaceID, otherSpaceID, otherDocID, "other-reset", "Other reset password", "other-reset-password", "cc33dd44", now)

	installationRepo := repository.NewSupportInboxInstallationRepository(db)
	if err := installationRepo.Create(ctx, &model.SupportWidgetInstallation{
		ID:          "inst-widget-help-search",
		WorkspaceID: workspaceID,
		WidgetKey:   widgetKey,
		SecretKey:   "sk-widget-help-search",
		Settings:    `{"widget_help_space_ids":["space-search-allowed"]}`,
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
		repository.NewDocsCollectionRepository(db, false),
		repository.NewDocsHelpcenterRepository(db, false),
	)
	svc.SetDocsSearchRepository(repository.NewDocsSearchRepository(db))

	results, err := svc.SearchWidgetHelpArticles(ctx, widgetKey, "reset", 8, "")
	if err != nil {
		t.Fatalf("SearchWidgetHelpArticles: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1: %#v", len(results), results)
	}
	if results[0].ID != allowedDocID {
		t.Fatalf("results[0].id = %q, want %q", results[0].ID, allowedDocID)
	}
	if results[0].ArticleKey != "allowed-reset-password-aa11bb22" {
		t.Fatalf("results[0].article_key = %q, want allowed-reset-password-aa11bb22", results[0].ArticleKey)
	}
}

func TestSupportInboxServiceSearchWidgetHelpArticles_RecordsNoResultsEvent(t *testing.T) {
	t.Parallel()

	const (
		workspaceID    = "ws-widget-help-search-event"
		widgetKey      = "wk-widget-help-search-event"
		allowedSpaceID = "space-search-event-allowed"
		documentID     = "doc-search-event"
	)

	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	db := setupDocsHelpcenterTranslationServiceTestDB(t)

	mustExec(t, db, `CREATE TABLE support_widget_installations (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		widget_key TEXT NOT NULL,
		secret_key TEXT NOT NULL,
		allowed_origins TEXT NOT NULL DEFAULT '{}',
		identity_verification_mode TEXT NOT NULL DEFAULT 'report_only',
		settings TEXT NOT NULL DEFAULT '{}',
		active BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE docs_helpcenter_search_entries (
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
	)`)

	seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
		ID:            "cfg-widget-help-search-event",
		WorkspaceID:   workspaceID,
		Subdomain:     "widget-help-search-event",
		BrandName:     "Widget Help Search Event",
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
		Name:        "Allowed Docs",
		Slug:        "allowed-docs",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeExternalCapable,
		CreatedBy:   "user-1",
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	allowedSpaceSlug := "allowed-docs"
	seedDocsHelpcenterTranslationServiceSpaceTranslation(t, db, model.DocsHelpcenterSpaceTranslation{
		ID:          "st-search-event-allowed",
		SpaceID:     allowedSpaceID,
		WorkspaceID: workspaceID,
		Locale:      "en",
		Name:        "Allowed Docs",
		Slug:        &allowedSpaceSlug,
		Status:      model.DocsHelpcenterTranslationStatusPublished,
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedSearchableWidgetHelpArticle(t, db, workspaceID, allowedSpaceID, documentID, "search-event-title", "Reset password", "reset-password", "dd55ee66", now)

	installationRepo := repository.NewSupportInboxInstallationRepository(db)
	if err := installationRepo.Create(ctx, &model.SupportWidgetInstallation{
		ID:          "inst-widget-help-search-event",
		WorkspaceID: workspaceID,
		WidgetKey:   widgetKey,
		SecretKey:   "sk-widget-help-search-event",
		Settings:    `{"widget_help_space_ids":["space-search-event-allowed"]}`,
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("create widget installation: %v", err)
	}

	recorder := &captureSupportEventRecorder{}
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
		repository.NewDocsCollectionRepository(db, false),
		repository.NewDocsHelpcenterRepository(db, false),
	)
	svc.SetDocsSearchRepository(repository.NewDocsSearchRepository(db))
	svc.SetSupportEventRecorder(recorder)

	results, err := svc.SearchWidgetHelpArticles(ctx, widgetKey, "analytics reports", 8, "anon-visitor-1")
	if err != nil {
		t.Fatalf("SearchWidgetHelpArticles: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("len(results) = %d, want 0: %#v", len(results), results)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(recorder.events))
	}
	event := recorder.events[0]
	if event.WorkspaceID != workspaceID {
		t.Fatalf("event.workspace_id = %q, want %q", event.WorkspaceID, workspaceID)
	}
	if event.EventType != model.SupportEventWidgetSearchPerformed {
		t.Fatalf("event.event_type = %q, want %q", event.EventType, model.SupportEventWidgetSearchPerformed)
	}
	if event.SourceSignal != "no_results" {
		t.Fatalf("event.source_signal = %q, want no_results", event.SourceSignal)
	}
	if event.IssueSummary != "analytics reports" {
		t.Fatalf("event.issue_summary = %q, want analytics reports", event.IssueSummary)
	}
	if event.Metadata["result_count"] != 0 {
		t.Fatalf("event.metadata.result_count = %#v, want 0", event.Metadata["result_count"])
	}
	if event.AnonymousID == nil || *event.AnonymousID != "anon-visitor-1" {
		t.Fatalf("event.anonymous_id = %v, want anon-visitor-1", event.AnonymousID)
	}
}

func seedSearchableWidgetHelpArticle(t *testing.T, db *gorm.DB, workspaceID, spaceID, documentID, searchEntryID, title, slug, publicID string, now time.Time) {
	t.Helper()
	seedDocsHelpcenterTranslationServiceDocument(t, db, model.DocsDocument{
		ID:          documentID,
		WorkspaceID: workspaceID,
		SpaceID:     spaceID,
		Title:       title,
		Status:      model.DocStatusPublished,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		CreatedBy:   "user-1",
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsHelpcenterTranslationServiceArticle(t, db, model.DocsHelpcenterArticle{
		ID:                "article-" + documentID,
		DocumentID:        documentID,
		PublicID:          publicID,
		Slug:              slug,
		PublicPublishedAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err := db.Create(&model.DocsHelpcenterArticlePublication{
		ID:          "pub-" + documentID,
		DocumentID:  documentID,
		WorkspaceID: workspaceID,
		SpaceID:     spaceID,
		Locale:      "en",
		Title:       title,
		Slug:        slug,
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed article publication: %v", err)
	}
	if err := db.Create(&model.DocsHelpcenterSearchEntry{
		ID:           searchEntryID,
		WorkspaceID:  workspaceID,
		DocumentID:   documentID,
		Locale:       "en",
		EntryKey:     "title",
		EntryType:    model.DocsHelpcenterSearchEntryTypeTitle,
		Content:      title,
		RankWeight:   8,
		SearchConfig: "simple",
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed search entry: %v", err)
	}
}
