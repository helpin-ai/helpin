package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

// staticLinkPaths returns a lookup that only knows the given live targets and
// records the IDs it was asked for.
func staticLinkPaths(paths map[string]string) publicationLinkPathLookup {
	return func(_ context.Context, documentIDs []string) (map[string]string, error) {
		out := map[string]string{}
		for _, id := range documentIDs {
			if path, ok := paths[id]; ok {
				out[id] = path
			}
		}
		return out, nil
	}
}

const linkTestDraft = `{"type":"doc","content":[{"type":"paragraph","content":[
	{"type":"text","text":"See "},
	{"type":"text","text":"Billing","marks":[{"type":"link","attrs":{"href":"helpin://documents/target-doc","target":"_blank"}},{"type":"bold"}]},
	{"type":"text","text":" and "},
	{"type":"text","text":"Billing again","marks":[{"type":"link","attrs":{"href":"helpin://documents/target-doc"}}]}
]}]}`

func renderResolvedLinks(t *testing.T, snapshot json.RawMessage, paths map[string]string) string {
	t.Helper()
	resolved, err := resolvePublicDocumentLinks(context.Background(), snapshot, staticLinkPaths(paths))
	if err != nil {
		t.Fatalf("resolvePublicDocumentLinks() error = %v", err)
	}
	html, err := tiptap.RenderHTML(resolved)
	if err != nil {
		t.Fatalf("RenderHTML() error = %v", err)
	}
	return html
}

func TestResolvePublicDocumentLinksTargetPublishedAfterSource(t *testing.T) {
	draft := json.RawMessage(linkTestDraft)
	// Source is published while the target is not live: the snapshot keeps
	// plain text.
	snapshot, err := materializePublicationDocumentLinks(context.Background(), draft, staticLinkPaths(nil))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if html := renderResolvedLinks(t, snapshot, nil); strings.Contains(html, "<a ") || !strings.Contains(html, "<strong>Billing</strong>") {
		t.Fatalf("unlinked render = %s", html)
	}
	// Target goes live later; the unchanged snapshot now renders the link.
	html := renderResolvedLinks(t, snapshot, map[string]string{"target-doc": "/articles/billing-ba8b1552"})
	if strings.Count(html, `href="/articles/billing-ba8b1552"`) != 2 || strings.Contains(html, "helpin://") {
		t.Fatalf("render after target publish = %s", html)
	}
	if !strings.Contains(html, "<strong>") {
		t.Fatalf("non-link marks dropped: %s", html)
	}
	if !publicationContentEqual(draft, snapshot) {
		t.Fatal("snapshot must still equal the draft")
	}
}

func TestResolvePublicDocumentLinksUnpublishedTargetBecomesText(t *testing.T) {
	draft := json.RawMessage(linkTestDraft)
	snapshot, err := materializePublicationDocumentLinks(context.Background(), draft, staticLinkPaths(map[string]string{"target-doc": "/articles/billing-ba8b1552"}))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	html := renderResolvedLinks(t, snapshot, nil)
	if strings.Contains(html, "<a ") || !strings.Contains(html, "Billing again") {
		t.Fatalf("render after target unpublish = %s", html)
	}
	if !publicationContentEqual(draft, snapshot) {
		t.Fatal("snapshot must still equal the draft")
	}
}

func TestResolvePublicDocumentLinksFollowsSlugChange(t *testing.T) {
	snapshot, err := materializePublicationDocumentLinks(context.Background(), json.RawMessage(linkTestDraft), staticLinkPaths(map[string]string{"target-doc": "/articles/old-slug-ba8b1552"}))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	html := renderResolvedLinks(t, snapshot, map[string]string{"target-doc": "/articles/new-slug-ba8b1552"})
	if !strings.Contains(html, `href="/articles/new-slug-ba8b1552"`) || strings.Contains(html, "old-slug") {
		t.Fatalf("render after slug change = %s", html)
	}
}

func TestResolvePublicDocumentLinksHandlesUnrewrittenLinks(t *testing.T) {
	html := renderResolvedLinks(t, json.RawMessage(linkTestDraft), map[string]string{"target-doc": "/fr/articles/facturation-ba8b1552"})
	if !strings.Contains(html, `href="/fr/articles/facturation-ba8b1552"`) || strings.Contains(html, "helpin://") {
		t.Fatalf("render of raw links = %s", html)
	}
	html = renderResolvedLinks(t, json.RawMessage(linkTestDraft), nil)
	if strings.Contains(html, "helpin://") || strings.Contains(html, "<a ") {
		t.Fatalf("raw links to non-live targets must become text: %s", html)
	}
}

func TestResolvePublicDocumentLinksBatchesLookup(t *testing.T) {
	calls := 0
	lookup := func(_ context.Context, ids []string) (map[string]string, error) {
		calls++
		if len(ids) != 1 || ids[0] != "target-doc" {
			t.Fatalf("lookup ids = %v", ids)
		}
		return nil, nil
	}
	if _, err := resolvePublicDocumentLinks(context.Background(), json.RawMessage(linkTestDraft), lookup); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("lookup calls = %d, want 1", calls)
	}
}

// ─── Database-backed resolution ─────────────────────────────────────────────

const (
	linkTestWorkspaceID = "ws-links"
	linkTestUserID      = "user-links"
	linkTestSpaceID     = "space-links"
)

type linkTestArticle struct {
	documentID string
	workspace  string
	publicID   string
	slug       string
	status     string
	live       bool
	content    string
}

func seedLinkTestWorkspace(t *testing.T, db *gorm.DB, workspaceID, spaceID string, locales model.DocsStringArray) {
	t.Helper()
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	empty := json.RawMessage(`{}`)
	seedDocsHelpcenterTranslationServiceConfig(t, db, model.DocsHelpcenterConfig{
		ID:             "cfg-" + workspaceID,
		WorkspaceID:    workspaceID,
		Subdomain:      "sub-" + workspaceID,
		BrandName:      "Links",
		BrandColor:     "#000000",
		ThemeMode:      "system",
		HeaderLinks:    json.RawMessage(`[]`),
		FooterConfig:   empty,
		HomepageConfig: empty,
		SpaceNavConfig: empty,
		DefaultLocale:  "en",
		EnabledLocales: locales,
		IsPublished:    true,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	seedDocsHelpcenterTranslationServiceSpace(t, db, model.DocsSpace{
		ID:          spaceID,
		WorkspaceID: workspaceID,
		Name:        "Guides",
		Slug:        "guides-" + workspaceID,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		Type:        model.SpaceTypeExternalCapable,
		CreatedBy:   linkTestUserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

func seedLinkTestArticle(t *testing.T, db *gorm.DB, spaceID string, article linkTestArticle) {
	t.Helper()
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	status := article.status
	if status == "" {
		status = model.DocStatusPublished
	}
	seedDocsHelpcenterTranslationServiceDocument(t, db, model.DocsDocument{
		ID:          article.documentID,
		WorkspaceID: article.workspace,
		SpaceID:     spaceID,
		Title:       article.slug,
		Status:      status,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		CreatedBy:   linkTestUserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	ha := model.DocsHelpcenterArticle{
		ID:         "ha-" + article.documentID,
		DocumentID: article.documentID,
		Slug:       article.slug,
		PublicID:   article.publicID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if article.live {
		ha.PublicPublishedAt = &now
	}
	seedDocsHelpcenterTranslationServiceArticle(t, db, ha)
	if !article.live {
		return
	}
	seedLinkTestPublication(t, db, spaceID, article, "en", article.slug)
}

func seedLinkTestPublication(t *testing.T, db *gorm.DB, spaceID string, article linkTestArticle, locale, slug string) {
	t.Helper()
	content := article.content
	if content == "" {
		content = `{"type":"doc","content":[]}`
	}
	publication := model.DocsHelpcenterArticlePublication{
		ID:          "pub-" + article.documentID + "-" + locale,
		DocumentID:  article.documentID,
		WorkspaceID: article.workspace,
		SpaceID:     spaceID,
		Locale:      locale,
		Title:       slug,
		Slug:        slug,
		Content:     json.RawMessage(content),
		PublishedAt: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
	}
	if err := db.Create(&publication).Error; err != nil {
		t.Fatalf("seed publication: %v", err)
	}
}

func makeLinkTestTargetLive(t *testing.T, db *gorm.DB, target linkTestArticle) {
	t.Helper()
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	if err := db.Model(&model.DocsHelpcenterArticle{}).Where("document_id = ?", target.documentID).Update("public_published_at", now).Error; err != nil {
		t.Fatalf("publish target: %v", err)
	}
	seedLinkTestPublication(t, db, linkTestSpaceID, target, "en", target.slug)
}

func TestLiveArticleLinkPathLookupRules(t *testing.T) {
	db := setupDocsHelpcenterTranslationServiceTestDB(t)
	ctx := context.Background()
	seedLinkTestWorkspace(t, db, linkTestWorkspaceID, linkTestSpaceID, model.DocsStringArray{"en", "fr"})
	seedLinkTestWorkspace(t, db, "ws-other", "space-other", nil)

	seedLinkTestArticle(t, db, linkTestSpaceID, linkTestArticle{documentID: "doc-live", workspace: linkTestWorkspaceID, publicID: "aaaa1111", slug: "live", live: true})
	seedLinkTestArticle(t, db, linkTestSpaceID, linkTestArticle{documentID: "doc-fr", workspace: linkTestWorkspaceID, publicID: "bbbb2222", slug: "billing", live: true})
	seedLinkTestArticle(t, db, linkTestSpaceID, linkTestArticle{documentID: "doc-unpublished", workspace: linkTestWorkspaceID, publicID: "cccc3333", slug: "hidden"})
	seedLinkTestArticle(t, db, linkTestSpaceID, linkTestArticle{documentID: "doc-archived", workspace: linkTestWorkspaceID, publicID: "dddd4444", slug: "archived", live: true, status: model.DocStatusArchived})
	seedLinkTestArticle(t, db, "space-other", linkTestArticle{documentID: "doc-foreign", workspace: "ws-other", publicID: "eeee5555", slug: "foreign", live: true})

	// doc-fr has a published French translation; doc-live only has a draft one.
	frArticle := linkTestArticle{documentID: "doc-fr", workspace: linkTestWorkspaceID}
	seedLinkTestPublication(t, db, linkTestSpaceID, frArticle, "fr", "facturation")
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	for _, tr := range []model.DocsHelpcenterArticleTranslation{
		{ID: "tr-fr", DocumentID: "doc-fr", WorkspaceID: linkTestWorkspaceID, SpaceID: linkTestSpaceID, Locale: "fr", Title: "Facturation", Status: model.DocsHelpcenterTranslationStatusPublished, PublishedAt: &now, CreatedAt: now, UpdatedAt: now},
		{ID: "tr-live-fr", DocumentID: "doc-live", WorkspaceID: linkTestWorkspaceID, SpaceID: linkTestSpaceID, Locale: "fr", Title: "Direct", Status: model.DocsHelpcenterTranslationStatusDraft, CreatedAt: now, UpdatedAt: now},
	} {
		seedDocsHelpcenterTranslationServiceArticleTranslation(t, db, tr)
	}
	seedLinkTestPublication(t, db, linkTestSpaceID, linkTestArticle{documentID: "doc-live", workspace: linkTestWorkspaceID}, "fr", "stale-fr")

	repo := repository.NewDocsHelpcenterRepository(db, false)
	ids := []string{"doc-live", "doc-fr", "doc-unpublished", "doc-archived", "doc-foreign", "doc-missing"}

	paths, err := liveArticleLinkPathLookup(repo, linkTestWorkspaceID, "fr")(ctx, ids)
	if err != nil {
		t.Fatalf("lookup fr: %v", err)
	}
	want := map[string]string{
		"doc-fr":   "/fr/articles/facturation-bbbb2222",
		"doc-live": "/en/articles/live-aaaa1111",
	}
	if len(paths) != len(want) {
		t.Fatalf("fr paths = %v, want %v", paths, want)
	}
	for id, path := range want {
		if paths[id] != path {
			t.Fatalf("fr path[%s] = %q, want %q (all %v)", id, paths[id], path, paths)
		}
	}

	paths, err = liveArticleLinkPathLookup(repo, linkTestWorkspaceID, "")(ctx, ids)
	if err != nil {
		t.Fatalf("lookup default: %v", err)
	}
	if paths["doc-fr"] != "/en/articles/billing-bbbb2222" || len(paths) != 2 {
		t.Fatalf("default paths = %v", paths)
	}

	// The same IDs from another workspace's article never resolve here.
	paths, err = liveArticleLinkPathLookup(repo, "ws-other", "")(ctx, []string{"doc-live"})
	if err != nil || len(paths) != 0 {
		t.Fatalf("cross-workspace paths = %v, %v", paths, err)
	}
}

func TestPublicArticleRenderResolvesLinksAtReadTime(t *testing.T) {
	db := setupDocsHelpcenterTranslationServiceTestDB(t)
	ctx := context.Background()
	seedLinkTestWorkspace(t, db, linkTestWorkspaceID, linkTestSpaceID, nil)
	seedLinkTestWorkspace(t, db, "ws-other", "space-other", nil)

	target := linkTestArticle{documentID: "doc-target", workspace: linkTestWorkspaceID, publicID: "abcd1234", slug: "billing"}
	seedLinkTestArticle(t, db, linkTestSpaceID, target)
	seedLinkTestArticle(t, db, "space-other", linkTestArticle{documentID: "doc-foreign", workspace: "ws-other", publicID: "ffff0000", slug: "foreign", live: true})

	svc := newDocsHelpcenterPublicServiceForTest(db)
	draft := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","text":"Billing","marks":[{"type":"link","attrs":{"href":"helpin://documents/doc-target"}}]},
		{"type":"text","text":" / "},
		{"type":"text","text":"Foreign","marks":[{"type":"link","attrs":{"href":"helpin://documents/doc-foreign"}}]}
	]}]}`)
	// Publish the source while the target is still unpublished.
	snapshot, err := materializePublicationDocumentLinks(ctx, draft, liveArticleLinkPathLookup(svc.hcRepo, linkTestWorkspaceID, "en"))
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	seedLinkTestArticle(t, db, linkTestSpaceID, linkTestArticle{documentID: "doc-source", workspace: linkTestWorkspaceID, publicID: "12345678", slug: "start", live: true, content: string(snapshot)})

	read := func() string {
		t.Helper()
		article, err := svc.getPublicArticleByCanonicalKeyUncached(ctx, linkTestWorkspaceID, "start-12345678")
		if err != nil || article == nil || article.ContentHTML == nil {
			t.Fatalf("read article = %#v, %v", article, err)
		}
		return *article.ContentHTML
	}

	if html := read(); strings.Contains(html, "<a ") || !strings.Contains(html, "Billing") {
		t.Fatalf("before target publish: %s", html)
	}

	makeLinkTestTargetLive(t, db, target)
	html := read()
	if !strings.Contains(html, `href="/articles/billing-abcd1234"`) {
		t.Fatalf("after target publish, link missing: %s", html)
	}
	if strings.Contains(html, "ffff0000") || strings.Contains(html, "helpin://") {
		t.Fatalf("cross-workspace or in-app link leaked: %s", html)
	}

	if err := db.Model(&model.DocsHelpcenterArticlePublication{}).Where("document_id = ?", "doc-target").Update("slug", "payments").Error; err != nil {
		t.Fatal(err)
	}
	if html := read(); !strings.Contains(html, `href="/articles/payments-abcd1234"`) {
		t.Fatalf("after slug change: %s", html)
	}

	if err := db.Model(&model.DocsDocument{}).Where("id = ?", "doc-target").Update("status", model.DocStatusArchived).Error; err != nil {
		t.Fatal(err)
	}
	if html := read(); strings.Contains(html, "<a ") {
		t.Fatalf("after target archive: %s", html)
	}
}
