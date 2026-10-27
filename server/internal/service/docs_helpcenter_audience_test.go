package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	audienceWorkspaceID = "ws-audience"
	audienceSpaceID     = "space-customers"
	audienceDocumentID  = "doc-renewal"
)

var audienceContent = json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Renewal pricing for enterprise plans is set per contract."}]}]}`)

// setupAudienceTestDB seeds a published help center with one space for
// signed-in customers holding one internally published document.
func setupAudienceTestDB(t *testing.T) (*gorm.DB, *DocsHelpcenterService) {
	t.Helper()
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	db := setupDocsHelpcenterTranslationServiceTestDB(t)
	mustExec(t, db, `CREATE TABLE docs_helpcenter_search_entries (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
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
	mustExec(t, db, `CREATE TABLE docs_space_teams (space_id TEXT NOT NULL, team_id TEXT NOT NULL)`)
	mustExec(t, db, `CREATE TABLE docs_chunks (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		space_id TEXT NOT NULL,
		document_id TEXT NOT NULL,
		block_id TEXT,
		chunk_index INTEGER NOT NULL,
		section_key TEXT NOT NULL DEFAULT '',
		heading_path TEXT NOT NULL DEFAULT '',
		block_range TEXT,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		search_content TEXT NOT NULL DEFAULT '',
		previous_chunk_index INTEGER,
		next_chunk_index INTEGER,
		content_hash TEXT NOT NULL DEFAULT '',
		embedding TEXT NOT NULL DEFAULT '',
		embedding_provider TEXT NOT NULL DEFAULT '',
		embedding_model TEXT NOT NULL DEFAULT '',
		embedding_version TEXT NOT NULL DEFAULT '',
		embedding_dimensions INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
		ID: "cfg-audience", WorkspaceID: audienceWorkspaceID, Subdomain: "audience", BrandName: "Acme",
		BrandColor: "#000000", ThemeMode: "system", DefaultLocale: "en", EnabledLocales: model.DocsStringArray{"en"},
		IsPublished: true, CreatedAt: now, UpdatedAt: now,
	})
	seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
		ID: audienceSpaceID, WorkspaceID: audienceWorkspaceID, Name: "Customer guides", Slug: "customer-guides",
		Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeExternalCapable,
		HelpcenterAudience: model.HelpcenterAudienceCustomers, CreatedBy: "user-1", CreatedAt: now, UpdatedAt: now,
	})
	seedDocsHelpcenterTranslationServiceDocument(t, db, model.DocsDocument{
		ID: audienceDocumentID, WorkspaceID: audienceWorkspaceID, SpaceID: audienceSpaceID, Title: "Renewal pricing",
		Status: model.DocStatusPublished, Visibility: model.SpaceVisibilityWorkspaceWide, CreatedBy: "user-1",
		PublishedAt: &now, CreatedAt: now, UpdatedAt: now,
	})
	seedDocsHelpcenterTranslationServiceContent(t, db, model.DocsContent{
		ID: "content-renewal", DocumentID: audienceDocumentID, Content: audienceContent, ContentText: "Renewal pricing for enterprise plans is set per contract.",
		CreatedAt: now, UpdatedAt: now,
	})
	svc := newDocsHelpcenterPublicServiceForTest(db)
	svc.SetSearchRepository(repository.NewDocsHelpcenterSearchRepository(db))
	return db, svc
}

func audienceArticle(t *testing.T, db *gorm.DB) model.DocsHelpcenterArticle {
	t.Helper()
	var article model.DocsHelpcenterArticle
	if err := db.Where("document_id = ?", audienceDocumentID).First(&article).Error; err != nil {
		t.Fatalf("load article: %v", err)
	}
	return article
}

func audienceSearchEntries(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Table("docs_helpcenter_search_entries").Where("document_id = ?", audienceDocumentID).Count(&count).Error; err != nil {
		t.Fatalf("count search entries: %v", err)
	}
	return count
}

func TestPublishInCustomerSpaceStaysOffPublicPaths(t *testing.T) {
	ctx := context.Background()
	db, svc := setupAudienceTestDB(t)
	if err := svc.PublishExternally(ctx, audienceDocumentID, "", audienceContent); err != nil {
		t.Fatalf("publish: %v", err)
	}
	article := audienceArticle(t, db)
	if article.PublicPublishedAt != nil || article.CustomerPublishedAt == nil {
		t.Fatalf("published article = public %v, customer %v; want customer only", article.PublicPublishedAt, article.CustomerPublishedAt)
	}
	if n := audienceSearchEntries(t, db); n != 0 {
		t.Fatalf("public search entries = %d, want 0", n)
	}

	hcRepo := repository.NewDocsHelpcenterRepository(db, false)
	spaces, err := repository.NewDocsSpaceRepository(db).ListPublicByWorkspace(ctx, audienceWorkspaceID)
	if err != nil || len(spaces) != 0 {
		t.Fatalf("public spaces = %v, %v; want none", spaces, err)
	}
	if public, _, _, err := hcRepo.GetPublicArticleByPublicID(ctx, audienceWorkspaceID, article.PublicID); err != nil || public != nil {
		t.Fatalf("public article by public id = %v, %v; want none", public, err)
	}
	searchable, err := repository.NewDocsChunkRepository(db).ListSearchableExternalSpaceIDs(ctx, audienceWorkspaceID)
	if err != nil || len(searchable) != 0 {
		t.Fatalf("publicly searchable spaces = %v, %v; want none", searchable, err)
	}
	widgetSpaces, err := (&SupportInboxService{docsSpaceRepo: repository.NewDocsSpaceRepository(db)}).resolveWidgetHelpSpaces(ctx, audienceWorkspaceID, []string{audienceSpaceID})
	if err != nil || len(widgetSpaces) != 0 {
		t.Fatalf("widget spaces = %v, %v; want none", widgetSpaces, err)
	}

	private, err := hcRepo.GetCustomerHelpArticle(ctx, audienceWorkspaceID, "en", article.PublicID)
	if err != nil || private == nil || private.Title != "Renewal pricing" || len(private.Content) == 0 {
		t.Fatalf("customer article = %+v, %v", private, err)
	}

	if err := svc.UnpublishExternally(ctx, audienceDocumentID); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if article := audienceArticle(t, db); article.LivePublishedAt() != nil {
		t.Fatalf("unpublished article is still live: %+v", article)
	}
}

func TestMoveSpaceAudienceMovesLiveArticles(t *testing.T) {
	ctx := context.Background()
	db, svc := setupAudienceTestDB(t)
	if err := svc.PublishExternally(ctx, audienceDocumentID, "", audienceContent); err != nil {
		t.Fatalf("publish: %v", err)
	}
	spaces := NewDocsSpaceService(repository.NewDocsSpaceRepository(db), nil)
	spaces.SetHelpcenterSync(svc)
	public := model.HelpcenterAudiencePublic
	if _, err := spaces.Update(ctx, audienceSpaceID, model.UpdateDocsSpaceRequest{HelpcenterAudience: &public}); err != nil {
		t.Fatalf("make space public: %v", err)
	}
	if article := audienceArticle(t, db); article.PublicPublishedAt == nil || article.CustomerPublishedAt != nil {
		t.Fatalf("after moving to public = %+v; want public only", article)
	}
	if n := audienceSearchEntries(t, db); n == 0 {
		t.Fatal("moving to public did not rebuild search entries")
	}

	customers := model.HelpcenterAudienceCustomers
	if _, err := spaces.Update(ctx, audienceSpaceID, model.UpdateDocsSpaceRequest{HelpcenterAudience: &customers}); err != nil {
		t.Fatalf("make space private: %v", err)
	}
	if article := audienceArticle(t, db); article.PublicPublishedAt != nil || article.CustomerPublishedAt == nil {
		t.Fatalf("after moving to customers = %+v; want customer only", article)
	}
	if n := audienceSearchEntries(t, db); n != 0 {
		t.Fatalf("search entries after moving to customers = %d, want 0", n)
	}

	invalid := "everyone"
	if _, err := spaces.Update(ctx, audienceSpaceID, model.UpdateDocsSpaceRequest{HelpcenterAudience: &invalid}); err == nil {
		t.Fatal("an unknown audience was accepted")
	}
}

func TestChangingSpaceToInternalUnpublishesArticles(t *testing.T) {
	ctx := context.Background()
	db, svc := setupAudienceTestDB(t)
	public := model.HelpcenterAudiencePublic
	spaces := NewDocsSpaceService(repository.NewDocsSpaceRepository(db), nil)
	spaces.SetHelpcenterSync(svc)
	if _, err := spaces.Update(ctx, audienceSpaceID, model.UpdateDocsSpaceRequest{HelpcenterAudience: &public}); err != nil {
		t.Fatalf("make space public: %v", err)
	}
	if err := svc.PublishExternally(ctx, audienceDocumentID, "", audienceContent); err != nil {
		t.Fatalf("publish: %v", err)
	}
	internal := model.SpaceTypeInternal
	if _, err := spaces.Update(ctx, audienceSpaceID, model.UpdateDocsSpaceRequest{Type: &internal}); err != nil {
		t.Fatalf("make space internal: %v", err)
	}
	if article := audienceArticle(t, db); article.LivePublishedAt() != nil {
		t.Fatalf("article stayed live after its space became internal: %+v", article)
	}
	if n := audienceSearchEntries(t, db); n != 0 {
		t.Fatalf("search entries = %d, want 0", n)
	}
}

func TestPortalHelpArticlesForSignedInCustomers(t *testing.T) {
	ctx := context.Background()
	db, svc := setupAudienceTestDB(t)
	if err := svc.PublishExternally(ctx, audienceDocumentID, "", audienceContent); err != nil {
		t.Fatalf("publish: %v", err)
	}
	mustExec(t, db, `INSERT INTO docs_chunks (workspace_id, space_id, document_id, chunk_index, title, content)
		VALUES (?, ?, ?, 0, 'Renewal pricing', 'Renewal pricing for enterprise plans is set per contract.')`,
		audienceWorkspaceID, audienceSpaceID, audienceDocumentID)
	portal := &CustomerPortalService{
		inbox:     &SupportInboxService{docsHelpcenterRepo: repository.NewDocsHelpcenterRepository(db, false), docsSpaceRepo: repository.NewDocsSpaceRepository(db)},
		knowledge: &HelpcenterAISearchService{chunkRepo: repository.NewDocsChunkRepository(db), searchRepo: repository.NewDocsSearchRepository(db)},
	}
	ws := &PortalWorkspace{ID: audienceWorkspaceID, Settings: model.SupportInboxSettings{PortalIntakeEnabled: true}}

	articles, err := portal.HelpArticles(ctx, ws)
	if err != nil || len(articles) != 1 || articles[0].Title != "Renewal pricing" || articles[0].SpaceName != "Customer guides" {
		t.Fatalf("help articles = %+v, %v", articles, err)
	}
	detail, err := portal.HelpArticle(ctx, ws, articles[0].ID)
	if err != nil || detail == nil || detail.ContentHTML == "" {
		t.Fatalf("help article = %+v, %v", detail, err)
	}
	if _, err := portal.HelpArticle(ctx, ws, "missing"); !errors.Is(err, ErrPortalHelpArticleNotFound) {
		t.Fatalf("missing article error = %v", err)
	}
	found, err := portal.SearchHelpArticles(ctx, ws, "renewal pricing")
	if err != nil || len(found) != 1 || found[0].ID != articles[0].ID || found[0].Snippet == "" {
		t.Fatalf("help search = %+v, %v", found, err)
	}

	anonymous, err := portal.SuggestArticles(ctx, ws, "How is renewal pricing set?", false)
	if err != nil || len(anonymous.Articles) != 0 {
		t.Fatalf("anonymous suggestions = %+v, %v; want none", anonymous, err)
	}
	signedIn, err := portal.SuggestArticles(ctx, ws, "How is renewal pricing set?", true)
	if err != nil || len(signedIn.Articles) != 1 || signedIn.Articles[0].PortalArticleID != articles[0].ID ||
		signedIn.Articles[0].ID != audienceDocumentID || signedIn.Articles[0].URL != "" || !signedIn.SearchCompleted {
		t.Fatalf("signed-in suggestions = %+v, %v", signedIn, err)
	}

	// Teammates see a private article the customer opened, without a public address.
	opened, err := portal.portalArticles(ctx, audienceWorkspaceID, []string{audienceDocumentID})
	if err != nil || len(opened) != 1 || opened[0].PortalArticleID != articles[0].ID || opened[0].URL != "" {
		t.Fatalf("opened articles = %+v, %v", opened, err)
	}

	// Unpublished articles leave the portal too.
	if err := svc.UnpublishExternally(ctx, audienceDocumentID); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if articles, err := portal.HelpArticles(ctx, ws); err != nil || len(articles) != 0 {
		t.Fatalf("help articles after unpublish = %+v, %v", articles, err)
	}
}
