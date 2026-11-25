//go:build integration

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/helpscout"
	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/publichelp"
)

// newZendeskImportSite serves a small Zendesk help center: two categories,
// a section nested deeper than Helpin allows, links between articles, a
// video and an image. token,
// when set, is required on every API request.
func newZendeskImportSite(t *testing.T, token string) *httptest.Server {
	t.Helper()
	var site *httptest.Server
	site = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := site.URL
		api, isAPI := strings.CutPrefix(r.URL.Path, "/api/v2/help_center/")
		if isAPI && token != "" {
			if user, password, ok := r.BasicAuth(); !ok || user != "admin@acme.com/token" || password != token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		var body any
		switch {
		case r.URL.Path == "/img/setup.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))
			return
		case r.URL.Path == "/hc/en-us/articles/150-renumbered":
			// An article that was given a new number; the old address forwards.
			http.Redirect(w, r, "/hc/en-us/articles/110-Set-up-SSO", http.StatusMovedPermanently)
			return
		case !isAPI:
			_, _ = w.Write([]byte(`<html><head><title>Acme Support</title><script src="https://static.zdassets.com/hc.js"></script></head></html>`))
			return
		case api == "locales.json":
			body = map[string]any{"locales": []string{"en-us"}, "default_locale": "en-us"}
		case api == "en-us/categories.json":
			body = map[string]any{"categories": []map[string]any{
				{"id": 1, "name": "Basics", "description": "Start here", "position": 0, "html_url": base + "/hc/en-us/categories/1-Basics"},
				{"id": 2, "name": "Billing", "position": 1, "html_url": base + "/hc/en-us/categories/2-Billing"},
			}}
		case api == "en-us/sections.json":
			body = map[string]any{"sections": []map[string]any{
				{"id": 10, "name": "Setup", "category_id": 1, "position": 0, "html_url": base + "/hc/en-us/sections/10-Setup"},
				{"id": 11, "name": "Single sign-on", "category_id": 1, "parent_section_id": 10, "position": 0, "html_url": base + "/hc/en-us/sections/11-SSO"},
				{"id": 20, "name": "Invoices", "category_id": 2, "position": 0, "html_url": base + "/hc/en-us/sections/20-Invoices"},
			}}
		case api == "en-us/articles.json":
			body = map[string]any{"articles": []map[string]any{
				{"id": 100, "title": "Install Acme", "section_id": 10, "position": 0, "html_url": base + "/hc/en-us/articles/100-Install-Acme"},
				{"id": 110, "title": "Set up SSO", "section_id": 11, "position": 0, "html_url": base + "/hc/en-us/articles/110-Set-up-SSO"},
				{"id": 200, "title": "Read an invoice", "section_id": 20, "position": 0, "html_url": base + "/hc/en-us/articles/200-Read-an-invoice"},
				{"id": 201, "title": "Draft", "section_id": 20, "draft": true, "html_url": base + "/hc/en-us/articles/201-Draft"},
			}}
		case api == "en-us/articles/100.json":
			body = map[string]any{"article": map[string]any{"id": 100, "title": "Install Acme", "html_url": base + "/hc/en-us/articles/100-Install-Acme",
				"body": `<h2 id="start">Start</h2><p>Then <a href="/hc/en-us/articles/110-renamed#idp">set up SSO</a>, see <a href="` + base + `/hc/en-us/sections/20">invoices</a> ` +
					`or the <a href="/hc/en-us/articles/999-gone">old guide</a>.</p><p><strong><img src="/img/setup.png" alt="Setup"></strong></p>` +
					`<p><iframe src="//www.youtube-nocookie.com/embed/abc123"></iframe></p>`}}
		case api == "en-us/articles/110.json":
			body = map[string]any{"article": map[string]any{"id": 110, "title": "Set up SSO", "html_url": base + "/hc/en-us/articles/110-Set-up-SSO",
				"body": `<h2 id="idp">Identity provider</h2><p>Back to <a href="` + base + `/hc/en-us/articles/100">installing</a>.</p>`}}
		case api == "en-us/articles/200.json":
			body = map[string]any{"article": map[string]any{"id": 200, "title": "Read an invoice", "html_url": base + "/hc/en-us/articles/200-Read-an-invoice",
				"body": `<p>Every invoice lists your plan. See <a href="/hc/en-us/articles/150-renumbered">single sign-on</a>.</p>`}}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	t.Cleanup(site.Close)
	return site
}

// TestPublicReaderImportPostgres runs an import through a platform reader, as
// every help center but Intercom's is read: collections with their nesting,
// articles converted from HTML, images, links, redirects, and a second run
// that imports nothing twice.
func TestPublicReaderImportPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	site := newZendeskImportSite(t, "")

	// The fake listens on loopback, which the production clients refuse.
	plain := &http.Client{Timeout: 10 * time.Second}
	originalImageClient := helpscout.ImageHTTPClient
	helpscout.ImageHTTPClient = plain
	defer func() { helpscout.ImageHTTPClient = originalImageClient }()
	uploader := &countingImageUploader{}
	svc := newIntercomDocsImportService(db, "")
	svc.publicIntercom = intercom.NewPublicClientWithHTTP(plain)
	svc.publicFetch = publichelp.NewFetcherWithHTTP(plain)
	svc.imageUploader = uploader

	found, err := svc.CheckPublicHelpCenter(ctx, site.URL+"/hc/en-us/articles/100-Install-Acme")
	if err != nil || found.Platform != "zendesk" || found.URL != site.URL+"/hc/en-us/" || found.Name != "Acme Support" {
		t.Fatalf("check = %+v, %v", found, err)
	}
	preview, err := svc.PreviewPublicHelpCenter(ctx, model.PublicHelpCenterImportRequest{URL: site.URL})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Platform != "zendesk" || preview.Source != "zendesk_public" || preview.URL != site.URL+"/hc/en-us/" || preview.Name != "Acme Support" ||
		preview.Collections != 4 || preview.CollectionsMerged != 1 || preview.Articles != 3 || len(preview.TopCollections) != 2 || preview.TopCollections[0].Articles != 2 {
		t.Fatalf("preview = %+v", preview)
	}
	title, text, err := svc.ReadPublicArticle(ctx, preview.Source, site.URL+"/hc/en-us/articles/200-Read-an-invoice")
	if err != nil || title != "Read an invoice" || !strings.HasPrefix(text, "Every invoice lists your plan.") {
		t.Fatalf("read article = %q, %q, %v", title, text, err)
	}

	space, err := svc.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{Name: "Help", Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeExternalCapable}, userID)
	if err != nil {
		t.Fatal(err)
	}
	first := runPublicDocsImportFrom(t, svc, preview.Source, preview.URL, workspaceID, space.ID, userID, "published")
	if first.Status != model.DocsImportStatusDone || first.Total != 3 || first.Completed != 3 || first.Failed != 0 {
		t.Fatalf("first import: status=%s total=%d completed=%d failed=%d failures=%s error=%v",
			first.Status, first.Total, first.Completed, first.Failed, first.Failures, first.Error)
	}
	var summary model.ImportSummary
	if err := json.Unmarshal(first.Summary, &summary); err != nil {
		t.Fatal(err)
	}
	// 100→110, 100→section 20, 110→100, and 200→110 through the address that
	// forwards to it. The link to a removed article stays.
	if summary.SourceSystem != "zendesk_public" || summary.CollectionsCreated != 4 || summary.CollectionsMerged != 1 || summary.ArticlesPublished != 3 ||
		summary.ArticlesOutsideCollections != 0 || summary.LinksRewritten != 4 || summary.UnresolvedLinksTotal != 1 ||
		summary.ImagesCopied != 1 || summary.FailedImagesTotal != 0 || len(uploader.uploads) != 1 {
		t.Fatalf("summary = %+v, uploads = %v", summary, uploader.uploads)
	}

	articles := loadPublicImportedArticles(t, db, space.ID)
	// "Single sign-on" sits one level deeper than Helpin nests, so its article
	// lands in the section above.
	for title, want := range map[string]string{"Install Acme": "Setup", "Set up SSO": "Setup", "Read an invoice": "Invoices"} {
		if got := articles[title]; got.Collection != want || got.Status != model.DocStatusPublished || got.Published == nil {
			t.Errorf("%s: collection=%q status=%q published=%v; want collection %q, published", title, got.Collection, got.Status, got.Published, want)
		}
	}
	var basics, setup, invoices model.DocsCollection
	db.Where("space_id = ? AND name = ?", space.ID, "Basics").First(&basics)
	db.Where("space_id = ? AND name = ?", space.ID, "Setup").First(&setup)
	db.Where("space_id = ? AND name = ?", space.ID, "Invoices").First(&invoices)
	if derefString(setup.ParentCollectionID) != basics.ID || derefString(basics.Description) != "Start here" {
		t.Fatalf("nesting: setup under %v, basics described as %q", setup.ParentCollectionID, derefString(basics.Description))
	}
	install := articles["Install Acme"]
	if install.Slug != "install-acme" {
		t.Errorf("slug = %q, want the address without its number", install.Slug)
	}
	for _, want := range []string{
		`"href":"/articles/set-up-sso-` + articles["Set up SSO"].PublicID + `#idp"`,
		`"href":"/c/invoices-` + invoices.PublicID + `"`,
		// Kept as written, and absolute: the article it led to is gone.
		`"href":"` + site.URL + `/hc/en-us/articles/999-gone"`,
		// An image inside bold text and a video inside a paragraph both survive.
		`"src":"https://files.helpin.example/`, `"type":"videoEmbed"`,
		`"id":"start"`,
	} {
		if !strings.Contains(install.Content, want) {
			t.Errorf("install content lacks %s\n%s", want, install.Content)
		}
	}
	if want := `"href":"/articles/set-up-sso-` + articles["Set up SSO"].PublicID + `"`; !strings.Contains(articles["Read an invoice"].Content, want) {
		t.Errorf("a link to a renumbered article was not followed: want %s in %s", want, articles["Read an invoice"].Content)
	}
	var redirects int64
	db.Model(&model.DocsRedirect{}).Where("workspace_id = ? AND source_system = ? AND source_path IN ?", workspaceID, "zendesk_public",
		[]string{"/hc/en-us/articles/100-Install-Acme", "/hc/en-us/sections/10-Setup"}).Count(&redirects)
	if redirects != 2 {
		t.Fatalf("redirects for old addresses = %d, want 2", redirects)
	}

	second := runPublicDocsImportFrom(t, svc, preview.Source, preview.URL, workspaceID, space.ID, userID, "published")
	var secondSummary model.ImportSummary
	if err := json.Unmarshal(second.Summary, &secondSummary); err != nil {
		t.Fatal(err)
	}
	var documents, collections int64
	db.Model(&model.DocsDocument{}).Where("space_id = ? AND deleted_at IS NULL", space.ID).Count(&documents)
	db.Model(&model.DocsCollection{}).Where("space_id = ? AND deleted_at IS NULL", space.ID).Count(&collections)
	if second.Status != model.DocsImportStatusDone || second.Failed != 0 || documents != 3 || collections != 4 ||
		secondSummary.LinksRewritten != 0 || secondSummary.CollectionsCreated != 0 || len(uploader.uploads) != 1 {
		t.Fatalf("second import: status=%s failed=%d documents=%d collections=%d summary=%+v uploads=%d",
			second.Status, second.Failed, documents, collections, secondSummary, len(uploader.uploads))
	}
}

// TestPublicReaderImportNeedsTokenPostgres covers a Zendesk help center that
// refuses anonymous reads: the address alone asks for a token, and the token
// reads it.
func TestPublicReaderImportNeedsTokenPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	site := newZendeskImportSite(t, "secret")
	plain := &http.Client{Timeout: 10 * time.Second}
	svc := newIntercomDocsImportService(db, "")
	svc.publicIntercom = intercom.NewPublicClientWithHTTP(plain)
	svc.publicFetch = publichelp.NewFetcherWithHTTP(plain)

	if _, err := svc.PreviewPublicHelpCenter(ctx, model.PublicHelpCenterImportRequest{URL: site.URL}); !errors.Is(err, ErrPublicImportNeedsToken) {
		t.Fatalf("preview without a token = %v, want ErrPublicImportNeedsToken", err)
	}
	if _, err := svc.PreviewPublicHelpCenter(ctx, model.PublicHelpCenterImportRequest{URL: site.URL, Email: "admin@acme.com", APIToken: "wrong"}); !errors.Is(err, ErrPublicImportNeedsToken) {
		t.Fatalf("preview with a wrong token = %v, want ErrPublicImportNeedsToken", err)
	}
	preview, err := svc.PreviewPublicHelpCenter(ctx, model.PublicHelpCenterImportRequest{URL: site.URL, Email: "admin@acme.com", APIToken: "secret"})
	if err != nil || preview.Articles != 3 {
		t.Fatalf("preview with the token = %+v, %v", preview, err)
	}
}
