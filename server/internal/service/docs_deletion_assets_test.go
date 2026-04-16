package service

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeDocsAssetStore struct {
	publicBase string
	deleted    []string
}

func (s *fakeDocsAssetStore) PublicURL(key string) string {
	return fmt.Sprintf("%s/%s", s.publicBase, key)
}

func (s *fakeDocsAssetStore) DeleteObject(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

func setupDocsDeletionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:docs-deletion-%d?mode=memory&cache=shared", time.Now().UnixNano())
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
		`CREATE TABLE docs_space_teams (
			space_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			created_at DATETIME,
			PRIMARY KEY (space_id, team_id)
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
			document_id TEXT NOT NULL,
			content JSON,
			content_text TEXT,
			word_count INTEGER NOT NULL DEFAULT 0,
			import_source_html TEXT,
			import_source_system TEXT,
			import_source_object_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_versions (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			content JSON,
			content_text TEXT,
			snapshot_label TEXT,
			version_type TEXT NOT NULL DEFAULT 'manual',
			word_count INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE docs_links (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			linked_object_type TEXT NOT NULL,
			linked_object_id TEXT NOT NULL,
			link_context TEXT NOT NULL DEFAULT 'attached',
			created_by TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE docs_chunks (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			title TEXT,
			content TEXT,
			content_hash TEXT,
			embedding TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_articles (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			collection_id TEXT,
			slug TEXT,
			seo_title TEXT,
			seo_description TEXT,
			public_published_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
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
			published_at DATETIME,
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
			updated_at DATETIME
		)`,
		`CREATE TABLE docs_helpcenter_collection_translations (
			id TEXT PRIMARY KEY,
			collection_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			space_id TEXT NOT NULL,
			locale TEXT NOT NULL,
			name TEXT NOT NULL,
			slug TEXT,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'draft',
			source_updated_at DATETIME,
			source_synced BOOLEAN NOT NULL DEFAULT 0,
			published_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
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
			updated_at DATETIME
		)`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create docs deletion table: %v", err)
		}
	}
	return db
}

func TestDocsDocumentDeleteHardDeletesGraphAndExclusiveAssets(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-assets"
		spaceID     = "space-assets"
		userID      = "user-assets"
	)
	ctx := context.Background()
	db := setupDocsDeletionTestDB(t)
	now := time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)

	seedDocsSpace(t, db, model.DocsSpace{
		ID:          spaceID,
		WorkspaceID: workspaceID,
		Name:        "Assets",
		Slug:        "assets",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeInternal,
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	for _, doc := range []model.DocsDocument{
		{
			ID:          "doc-delete",
			WorkspaceID: workspaceID,
			SpaceID:     spaceID,
			Title:       "Delete",
			Status:      model.DocStatusDraft,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Position:    0,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "doc-keep",
			WorkspaceID: workspaceID,
			SpaceID:     spaceID,
			Title:       "Keep",
			Status:      model.DocStatusDraft,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Position:    1,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	} {
		seedDocsOrderingDocument(t, db, doc)
	}

	uniqueKey := "docs-import/ws-assets/import-1/unique.png"
	sharedKey := "docs-import/ws-assets/import-1/shared.png"
	contentRaw := fmt.Sprintf(`{"type":"doc","content":[{"type":"image","attrs":{"src":"https://cdn.helpin.test/%s"}},{"type":"image","attrs":{"src":"https://cdn.helpin.test/%s"}}]}`, uniqueKey, sharedKey)
	keepRaw := fmt.Sprintf(`{"type":"doc","content":[{"type":"image","attrs":{"src":"https://cdn.helpin.test/%s"}}]}`, sharedKey)

	if err := db.Exec(`INSERT INTO docs_contents (id, document_id, content, import_source_html, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"content-delete", "doc-delete", []byte(contentRaw), fmt.Sprintf(`<img src="https://cdn.helpin.test/%s">`, uniqueKey), now, now).Error; err != nil {
		t.Fatalf("seed deleted content: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_contents (id, document_id, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"content-keep", "doc-keep", []byte(keepRaw), now, now).Error; err != nil {
		t.Fatalf("seed kept content: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_versions (id, document_id, content, content_text, version_type, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"version-delete", "doc-delete", []byte(contentRaw), "", "manual", userID, now).Error; err != nil {
		t.Fatalf("seed version: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_links (id, workspace_id, document_id, linked_object_type, linked_object_id, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"link-delete", workspaceID, "doc-delete", "support_conversation", "conv-1", userID, now).Error; err != nil {
		t.Fatalf("seed link: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_chunks (id, workspace_id, space_id, document_id, chunk_index, title, content, content_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"chunk-delete", workspaceID, spaceID, "doc-delete", 0, "Delete", "content", "hash", now, now).Error; err != nil {
		t.Fatalf("seed chunk: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_helpcenter_articles (id, document_id, workspace_id, space_id, slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"article-delete", "doc-delete", workspaceID, spaceID, "delete", now, now).Error; err != nil {
		t.Fatalf("seed helpcenter article: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_helpcenter_article_publications (id, document_id, workspace_id, space_id, locale, title, slug, content, published_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"publication-delete", "doc-delete", workspaceID, spaceID, "en", "Delete", "delete", []byte(contentRaw), now, now, now).Error; err != nil {
		t.Fatalf("seed publication: %v", err)
	}
	if err := db.Exec(`INSERT INTO docs_helpcenter_article_translations (id, document_id, workspace_id, space_id, locale, title, slug, content, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"translation-delete", "doc-delete", workspaceID, spaceID, "fr", "Delete", "delete", []byte(contentRaw), "published", now, now).Error; err != nil {
		t.Fatalf("seed translation: %v", err)
	}

	docRepo := repository.NewDocsDocumentRepository(db)
	svc := NewDocsDocumentService(docRepo, repository.NewDocsSpaceRepository(db), nil)
	store := &fakeDocsAssetStore{publicBase: "https://cdn.helpin.test"}
	svc.SetDeletionDependencies(DocsDocumentDeletionDependencies{
		ContentRepo:     repository.NewDocsContentRepository(db),
		VersionRepo:     repository.NewDocsVersionRepository(db),
		LinkRepo:        repository.NewDocsLinkRepository(db),
		ChunkRepo:       repository.NewDocsChunkRepository(db),
		HelpcenterRepo:  repository.NewDocsHelpcenterRepository(db),
		PublicationRepo: repository.NewDocsHelpcenterPublicationRepository(db),
		TranslationRepo: repository.NewDocsHelpcenterTranslationRepository(db),
		AssetStore:      store,
	})

	if err := svc.Delete(ctx, "doc-delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var docCount int64
	if err := db.WithContext(ctx).Table("docs_documents").Where("id = ?", "doc-delete").Count(&docCount).Error; err != nil {
		t.Fatalf("count docs_documents: %v", err)
	}
	if docCount != 0 {
		t.Fatalf("docs_documents rows for deleted document = %d, want 0", docCount)
	}

	for _, table := range []string{
		"docs_contents",
		"docs_versions",
		"docs_links",
		"docs_chunks",
		"docs_helpcenter_articles",
		"docs_helpcenter_article_publications",
		"docs_helpcenter_article_translations",
	} {
		var count int64
		if err := db.WithContext(ctx).Table(table).Where("document_id = ?", "doc-delete").Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s rows for deleted document = %d, want 0", table, count)
		}
	}

	if kept, err := docRepo.GetByID(ctx, "doc-keep"); err != nil {
		t.Fatalf("load kept doc: %v", err)
	} else if kept == nil {
		t.Fatalf("kept document was deleted")
	}

	sort.Strings(store.deleted)
	if got, want := store.deleted, []string{uniqueKey}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("deleted assets = %v, want %v", got, want)
	}
}

func TestDocsCollectionDeleteWithPermanentDependenciesDeletesSubtreeAndAssets(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-collection-assets"
		spaceID     = "space-collection-assets"
		userID      = "user-collection-assets"
	)
	ctx := context.Background()
	db := setupDocsDeletionTestDB(t)
	now := time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC)

	seedDocsSpace(t, db, model.DocsSpace{
		ID:          spaceID,
		WorkspaceID: workspaceID,
		Name:        "Collection Assets",
		Slug:        "collection-assets",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeInternal,
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsCollection(t, db, model.DocsCollection{
		ID:          "root",
		WorkspaceID: workspaceID,
		SpaceID:     spaceID,
		Name:        "Root",
		Slug:        "root",
		PublicID:    "rootpub",
		Depth:       0,
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	rootID := "root"
	seedDocsCollection(t, db, model.DocsCollection{
		ID:                 "child",
		WorkspaceID:        workspaceID,
		SpaceID:            spaceID,
		ParentCollectionID: &rootID,
		Name:               "Child",
		Slug:               "child",
		PublicID:           "childpub",
		Depth:              1,
		Position:           0,
		CreatedBy:          userID,
		CreatedAt:          now,
		UpdatedAt:          now,
	})

	assetKey := "docs-import/ws-collection-assets/import-1/collection.png"
	for _, doc := range []model.DocsDocument{
		{
			ID:           "doc-root",
			WorkspaceID:  workspaceID,
			SpaceID:      spaceID,
			CollectionID: &rootID,
			Title:        "Root doc",
			Status:       model.DocStatusDraft,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     0,
			CreatedBy:    userID,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "doc-child",
			WorkspaceID:  workspaceID,
			SpaceID:      spaceID,
			CollectionID: docsDeletionStringPtr("child"),
			Title:        "Child doc",
			Status:       model.DocStatusDraft,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     0,
			CreatedBy:    userID,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	} {
		seedDocsOrderingDocument(t, db, doc)
	}
	contentRaw := fmt.Sprintf(`{"type":"doc","content":[{"type":"image","attrs":{"src":"https://cdn.helpin.test/%s"}}]}`, assetKey)
	if err := db.Exec(`INSERT INTO docs_contents (id, document_id, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"content-child", "doc-child", []byte(contentRaw), now, now).Error; err != nil {
		t.Fatalf("seed child content: %v", err)
	}

	docRepo := repository.NewDocsDocumentRepository(db)
	spaceRepo := repository.NewDocsSpaceRepository(db)
	collectionRepo := repository.NewDocsCollectionRepository(db)
	store := &fakeDocsAssetStore{publicBase: "https://cdn.helpin.test"}
	docSvc := NewDocsDocumentService(docRepo, spaceRepo, nil)
	docSvc.SetDeletionDependencies(DocsDocumentDeletionDependencies{
		ContentRepo:     repository.NewDocsContentRepository(db),
		VersionRepo:     repository.NewDocsVersionRepository(db),
		LinkRepo:        repository.NewDocsLinkRepository(db),
		ChunkRepo:       repository.NewDocsChunkRepository(db),
		HelpcenterRepo:  repository.NewDocsHelpcenterRepository(db),
		PublicationRepo: repository.NewDocsHelpcenterPublicationRepository(db),
		TranslationRepo: repository.NewDocsHelpcenterTranslationRepository(db),
		AssetStore:      store,
	})
	collectionSvc := NewDocsCollectionService(collectionRepo, spaceRepo, nil)
	collectionSvc.SetPermanentDeleteDependencies(docRepo, docSvc, repository.NewDocsHelpcenterTranslationRepository(db))

	if err := collectionSvc.Delete(ctx, "", "root"); err != nil {
		t.Fatalf("Delete collection: %v", err)
	}

	var collectionCount int64
	if err := db.WithContext(ctx).Table("docs_collections").Where("id IN ?", []string{"root", "child"}).Count(&collectionCount).Error; err != nil {
		t.Fatalf("count collections: %v", err)
	}
	if collectionCount != 0 {
		t.Fatalf("collection rows after delete = %d, want 0", collectionCount)
	}
	var docCount int64
	if err := db.WithContext(ctx).Table("docs_documents").Where("id IN ?", []string{"doc-root", "doc-child"}).Count(&docCount).Error; err != nil {
		t.Fatalf("count docs: %v", err)
	}
	if docCount != 0 {
		t.Fatalf("document rows after collection delete = %d, want 0", docCount)
	}
	if got, want := store.deleted, []string{assetKey}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("deleted assets = %v, want %v", got, want)
	}
}

func TestDocsCollectionDeleteImpactCountsSubtreeDocsAndPublicDocs(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-impact"
		spaceID     = "space-impact"
		userID      = "user-impact"
	)
	ctx := context.Background()
	db := setupDocsDeletionTestDB(t)
	now := time.Date(2026, 4, 16, 11, 30, 0, 0, time.UTC)

	seedDocsSpace(t, db, model.DocsSpace{
		ID:          spaceID,
		WorkspaceID: workspaceID,
		Name:        "Impact",
		Slug:        "impact",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeExternalCapable,
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsCollection(t, db, model.DocsCollection{
		ID:          "impact-root",
		WorkspaceID: workspaceID,
		SpaceID:     spaceID,
		Name:        "Impact Root",
		Slug:        "impact-root",
		PublicID:    "impactroot",
		Depth:       0,
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	rootID := "impact-root"
	seedDocsCollection(t, db, model.DocsCollection{
		ID:                 "impact-child",
		WorkspaceID:        workspaceID,
		SpaceID:            spaceID,
		ParentCollectionID: &rootID,
		Name:               "Impact Child",
		Slug:               "impact-child",
		PublicID:           "impactchild",
		Depth:              1,
		Position:           0,
		CreatedBy:          userID,
		CreatedAt:          now,
		UpdatedAt:          now,
	})

	for _, doc := range []model.DocsDocument{
		{
			ID:           "impact-draft",
			WorkspaceID:  workspaceID,
			SpaceID:      spaceID,
			CollectionID: &rootID,
			Title:        "Draft",
			Status:       model.DocStatusDraft,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     0,
			CreatedBy:    userID,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "impact-archived",
			WorkspaceID:  workspaceID,
			SpaceID:      spaceID,
			CollectionID: docsDeletionStringPtr("impact-child"),
			Title:        "Archived",
			Status:       model.DocStatusArchived,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     1,
			CreatedBy:    userID,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "impact-published",
			WorkspaceID:  workspaceID,
			SpaceID:      spaceID,
			CollectionID: docsDeletionStringPtr("impact-child"),
			Title:        "Published",
			Status:       model.DocStatusPublished,
			Visibility:   model.SpaceVisibilityWorkspaceWide,
			Position:     2,
			CreatedBy:    userID,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	} {
		seedDocsOrderingDocument(t, db, doc)
	}
	if err := db.Exec(`INSERT INTO docs_helpcenter_articles (id, document_id, workspace_id, space_id, slug, public_published_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"impact-public-article", "impact-published", workspaceID, spaceID, "published", now, now, now).Error; err != nil {
		t.Fatalf("seed public article: %v", err)
	}

	docRepo := repository.NewDocsDocumentRepository(db)
	collectionSvc := NewDocsCollectionService(repository.NewDocsCollectionRepository(db), repository.NewDocsSpaceRepository(db), nil)
	collectionSvc.SetPermanentDeleteDependencies(docRepo, NewDocsDocumentService(docRepo, repository.NewDocsSpaceRepository(db), nil), repository.NewDocsHelpcenterTranslationRepository(db))
	collectionSvc.SetHelpcenterRepository(repository.NewDocsHelpcenterRepository(db))

	impact, err := collectionSvc.GetDeleteImpact(ctx, "", "impact-root")
	if err != nil {
		t.Fatalf("GetDeleteImpact: %v", err)
	}

	if impact.CollectionID != "impact-root" || impact.CollectionName != "Impact Root" || impact.SpaceID != spaceID {
		t.Fatalf("impact identity = %+v", impact)
	}
	if impact.CollectionCount != 2 {
		t.Fatalf("CollectionCount = %d, want 2", impact.CollectionCount)
	}
	if impact.DocumentCount != 3 {
		t.Fatalf("DocumentCount = %d, want 3", impact.DocumentCount)
	}
	if impact.ArchivedDocumentCount != 1 {
		t.Fatalf("ArchivedDocumentCount = %d, want 1", impact.ArchivedDocumentCount)
	}
	if impact.PublishedDocumentCount != 1 {
		t.Fatalf("PublishedDocumentCount = %d, want 1", impact.PublishedDocumentCount)
	}
	if impact.PublicDocumentCount != 1 {
		t.Fatalf("PublicDocumentCount = %d, want 1", impact.PublicDocumentCount)
	}
}

func TestDocsSpaceDeleteWithPermanentDependenciesDeletesSpaceGraph(t *testing.T) {
	t.Parallel()

	const (
		workspaceID = "ws-space-assets"
		spaceID     = "space-delete-assets"
		userID      = "user-space-assets"
	)
	ctx := context.Background()
	db := setupDocsDeletionTestDB(t)
	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)

	seedDocsSpace(t, db, model.DocsSpace{
		ID:          spaceID,
		WorkspaceID: workspaceID,
		Name:        "Space Assets",
		Slug:        "space-assets",
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeInternal,
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsCollection(t, db, model.DocsCollection{
		ID:          "space-coll",
		WorkspaceID: workspaceID,
		SpaceID:     spaceID,
		Name:        "Space Collection",
		Slug:        "space-coll",
		PublicID:    "spacecoll",
		Depth:       0,
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	seedDocsOrderingDocument(t, db, model.DocsDocument{
		ID:          "space-doc",
		WorkspaceID: workspaceID,
		SpaceID:     spaceID,
		Title:       "Space doc",
		Status:      model.DocStatusArchived,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Position:    0,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	docRepo := repository.NewDocsDocumentRepository(db)
	spaceRepo := repository.NewDocsSpaceRepository(db)
	collectionRepo := repository.NewDocsCollectionRepository(db)
	translationRepo := repository.NewDocsHelpcenterTranslationRepository(db)
	docSvc := NewDocsDocumentService(docRepo, spaceRepo, nil)
	docSvc.SetDeletionDependencies(DocsDocumentDeletionDependencies{
		ContentRepo:     repository.NewDocsContentRepository(db),
		VersionRepo:     repository.NewDocsVersionRepository(db),
		LinkRepo:        repository.NewDocsLinkRepository(db),
		ChunkRepo:       repository.NewDocsChunkRepository(db),
		HelpcenterRepo:  repository.NewDocsHelpcenterRepository(db),
		PublicationRepo: repository.NewDocsHelpcenterPublicationRepository(db),
		TranslationRepo: translationRepo,
		AssetStore:      &fakeDocsAssetStore{publicBase: "https://cdn.helpin.test"},
	})
	spaceSvc := NewDocsSpaceService(spaceRepo, nil)
	spaceSvc.SetPermanentDeleteDependencies(collectionRepo, docRepo, docSvc, translationRepo)

	if err := spaceSvc.Delete(ctx, "", spaceID); err != nil {
		t.Fatalf("Delete space: %v", err)
	}

	for table, where := range map[string]string{
		"docs_spaces":      "id = ?",
		"docs_collections": "space_id = ?",
		"docs_documents":   "space_id = ?",
	} {
		var count int64
		if err := db.WithContext(ctx).Table(table).Where(where, spaceID).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s rows after space delete = %d, want 0", table, count)
		}
	}
}

func docsDeletionStringPtr(v string) *string {
	return &v
}
