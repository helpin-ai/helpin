//go:build integration

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/helpscout"
	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/intercom/intercomtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// countingImageUploader stores nothing and hands back a stable address.
type countingImageUploader struct{ uploads []string }

func (u *countingImageUploader) UploadImage(_ context.Context, workspaceID, filename string, data io.Reader, contentType string) (string, error) {
	if _, err := io.Copy(io.Discard, data); err != nil {
		return "", err
	}
	u.uploads = append(u.uploads, filename+" "+contentType)
	return fmt.Sprintf("https://files.helpin.example/%s/%d-%s", workspaceID, len(u.uploads), filename), nil
}

func paragraphBlock(html string) map[string]any {
	return map[string]any{"type": "paragraph", "text": html}
}

// seedPublicImportSite builds a help center that exercises every part of a
// public import: nesting deeper than Helpin allows, links between articles
// and to collections, an article that only links reach, a dead link, another
// company's link on Intercom's shared host, and images that do and do not
// copy.
func seedPublicImportSite() *intercomtest.PublicSite {
	site := intercomtest.NewPublicSite()
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	// A real image that the server labels as text, as some CDNs do.
	site.Images["/img/dashboard.png"] = intercomtest.PublicImage{ContentType: "text/plain", Body: png}
	site.Images["/img/not-an-image.png"] = intercomtest.PublicImage{ContentType: "text/html", Body: []byte("<html>expired</html>")}
	link := func(path, text string) string { return fmt.Sprintf(`<a href="%s%s">%s</a>`, site.URL, path, text) }
	site.Articles["11"] = intercomtest.PublicArticle{ID: "11", Slug: "getting-started", Title: "Getting started", Description: "First steps with Acme",
		Blocks: []map[string]any{
			{"type": "heading", "text": "Welcome", "idAttribute": "welcome"},
			paragraphBlock("Read about " + link("/en/articles/12-invoices#pay", "invoices") + ", browse " + link("/en/collections/2-billing", "billing") +
				", see the " + link("/en/articles/99-hidden-tips", "hidden tips") + " and the " + link("/en/articles/404-gone", "retired page") + "."),
			paragraphBlock(`Elsewhere: <a href="https://intercom.help/other/en/articles/12-theirs">another company</a> and <a href="https://example.org/">a site</a>.`),
			{"type": "image", "url": site.URL + "/img/dashboard.png?signature=first", "text": "Dashboard", "displayWidth": 550, "align": "center"},
			{"type": "image", "url": site.URL + "/img/not-an-image.png"},
		}}
	site.Articles["12"] = intercomtest.PublicArticle{ID: "12", Slug: "invoices", Title: "Invoices",
		Blocks: []map[string]any{
			{"type": "heading", "text": "Pay", "idAttribute": "pay"},
			paragraphBlock("Back to " + link("/en/articles/11-getting-started", "getting started") + "."),
			// The same image under a different signature is copied once.
			{"type": "image", "url": site.URL + "/img/dashboard.png?signature=second"},
		}}
	// Its data carries a different ID than its address; the address number is
	// what listings and links use, so it must be the one the import keys on.
	site.Articles["13"] = intercomtest.PublicArticle{ID: "13", Slug: "plans", Title: "Plans", DataID: "777013", Blocks: []map[string]any{paragraphBlock("Plans.")}}
	site.Articles["14"] = intercomtest.PublicArticle{ID: "14", Slug: "refunds", Title: "Refunds", Blocks: []map[string]any{paragraphBlock("Refunds.")}}
	site.Articles["15"] = intercomtest.PublicArticle{ID: "15", Slug: "chargebacks", Title: "Chargebacks",
		Blocks: []map[string]any{paragraphBlock("See " + link("/en/collections/4-disputes", "disputes") + ".")}}
	// Reachable only through links, and it links on to another such article.
	site.Articles["99"] = intercomtest.PublicArticle{ID: "99", Slug: "hidden-tips", Title: "Hidden tips", Description: "Not in any collection",
		Blocks: []map[string]any{paragraphBlock("More in " + link("/en/articles/98-deeper-tips", "deeper tips") + ".")}}
	site.Articles["98"] = intercomtest.PublicArticle{ID: "98", Slug: "deeper-tips", Title: "Deeper tips", Blocks: []map[string]any{paragraphBlock("Deep.")}}
	site.Collections = []intercomtest.PublicCollection{
		{ID: "1", Slug: "basics", Name: "Basics", Description: "Start here", ArticleIDs: []string{"11", "12"}},
		{ID: "2", Slug: "billing", Name: "Billing", ArticleIDs: []string{"13"}, Children: []intercomtest.PublicCollection{
			{ID: "3", Slug: "refunds", Name: "Refunds", ArticleIDs: []string{"14"}, Children: []intercomtest.PublicCollection{
				// One level deeper than Helpin nests collections.
				{ID: "4", Slug: "disputes", Name: "Disputes", ArticleIDs: []string{"15"}},
			}},
		}},
	}
	return site
}

func runPublicDocsImport(t *testing.T, svc *DocsImportService, sourceURL, workspaceID, spaceID, userID, status string) model.DocsImportJob {
	t.Helper()
	ctx := context.Background()
	job := &model.DocsImportJob{ID: uuid.NewString(), WorkspaceID: workspaceID, SpaceID: &spaceID, Source: docsImportSourceIntercomPublic,
		Status: model.DocsImportStatusPending, Failures: json.RawMessage("[]"), Config: json.RawMessage("{}"), StartedBy: userID}
	if err := svc.importRepo.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	if err := svc.runPublicIntercomImport(ctx, job.ID, model.DocsImportStartRequest{SourceURL: sourceURL, ImportStatus: status}, spaceID, workspaceID, userID, nil); err != nil {
		t.Fatal(err)
	}
	done, err := svc.importRepo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	return *done
}

type publicImportedArticle struct {
	DocumentID string
	Title      string
	Excerpt    *string
	Status     string
	Position   int
	Collection string
	Slug       string
	PublicID   string
	Content    string
	Published  *time.Time
}

func loadPublicImportedArticles(t *testing.T, db *gorm.DB, spaceID string) map[string]publicImportedArticle {
	t.Helper()
	var rows []publicImportedArticle
	if err := db.Raw(`
		SELECT d.id AS document_id, d.title, d.excerpt, d.status, d.position, COALESCE(c.name, '') AS collection,
		       a.slug, a.public_id, a.public_published_at AS published, dc.content::text AS content
		FROM docs_documents d
		JOIN docs_contents dc ON dc.document_id = d.id
		LEFT JOIN docs_collections c ON c.id = d.collection_id
		LEFT JOIN docs_helpcenter_articles a ON a.document_id = d.id
		WHERE d.space_id = ? AND d.deleted_at IS NULL`, spaceID).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	byTitle := make(map[string]publicImportedArticle, len(rows))
	for _, row := range rows {
		row.Content = compactTestJSON(t, row.Content)
		byTitle[row.Title] = row
	}
	return byTitle
}

// compactTestJSON removes the spacing PostgreSQL adds when it prints jsonb.
func compactTestJSON(t *testing.T, raw string) string {
	t.Helper()
	var out bytes.Buffer
	if err := json.Compact(&out, []byte(raw)); err != nil {
		t.Fatalf("compact %q: %v", raw, err)
	}
	return out.String()
}

func TestPublicHelpCenterImportPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	site := seedPublicImportSite()
	defer site.Close()

	// The fake listens on loopback, which the production clients refuse.
	plain := &http.Client{Timeout: 10 * time.Second}
	originalImageClient := helpscout.ImageHTTPClient
	helpscout.ImageHTTPClient = plain
	defer func() { helpscout.ImageHTTPClient = originalImageClient }()
	uploader := &countingImageUploader{}
	svc := newIntercomDocsImportService(db, "")
	svc.publicIntercom = intercom.NewPublicClientWithHTTP(plain)
	svc.imageUploader = uploader

	preview, err := svc.PreviewPublicHelpCenter(ctx, model.PublicHelpCenterImportRequest{URL: site.URL})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Platform != model.PublicHelpCenterPlatformIntercom || preview.URL != site.Home() || preview.Name != "Acme Help Center" ||
		preview.Collections != 3 || preview.CollectionsMerged != 1 || preview.Articles != 5 ||
		len(preview.TopCollections) != 2 || preview.TopCollections[1].Name != "Billing" || preview.TopCollections[1].Articles != 3 {
		t.Fatalf("preview = %+v", preview)
	}

	space, err := svc.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{Name: "Help", Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeExternalCapable}, userID)
	if err != nil {
		t.Fatal(err)
	}

	first := runPublicDocsImport(t, svc, site.URL, workspaceID, space.ID, userID, "published")
	if first.Status != model.DocsImportStatusDone || first.Total != 7 || first.Completed != 7 || first.Failed != 0 {
		t.Fatalf("first import: status=%s total=%d completed=%d failed=%d failures=%s error=%v",
			first.Status, first.Total, first.Completed, first.Failed, first.Failures, first.Error)
	}
	var summary model.ImportSummary
	if err := json.Unmarshal(first.Summary, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.SourceURL != site.Home() || summary.SourceName != "Acme Help Center" ||
		summary.CollectionsCreated != 4 || summary.CollectionsMerged != 1 ||
		summary.ArticlesPublished != 7 || summary.ArticlesDrafted != 0 || summary.ArticlesOutsideCollections != 2 {
		t.Fatalf("summary counts: %+v", summary)
	}
	// 11→12, 11→billing, 11→99, 12→11, 15→disputes (folded into Refunds), 99→98.
	if summary.LinksRewritten != 6 {
		t.Fatalf("links rewritten = %d, want 6", summary.LinksRewritten)
	}
	if summary.UnresolvedLinksTotal != 1 || len(summary.UnresolvedLinks) != 1 ||
		!strings.HasSuffix(summary.UnresolvedLinks[0].URL, "/en/articles/404-gone") || summary.UnresolvedLinks[0].ArticleTitle != "Getting started" {
		t.Fatalf("unresolved links: %+v", summary.UnresolvedLinks)
	}
	if summary.ImagesCopied != 1 || len(uploader.uploads) != 1 || !strings.HasSuffix(uploader.uploads[0], "image/png") {
		t.Fatalf("images copied = %d, uploads = %v; want the mislabeled PNG stored once as image/png", summary.ImagesCopied, uploader.uploads)
	}
	if summary.FailedImagesTotal != 1 || len(summary.FailedImages) != 1 || summary.FailedImages[0].ArticleTitle != "Getting started" ||
		strings.Contains(summary.FailedImages[0].URL, "?") || summary.FailedImages[0].DocumentID == "" {
		t.Fatalf("failed images: %+v", summary.FailedImages)
	}

	articles := loadPublicImportedArticles(t, db, space.ID)
	if len(articles) != 7 {
		t.Fatalf("documents = %d, want 7", len(articles))
	}
	started, invoices := articles["Getting started"], articles["Invoices"]
	for title, want := range map[string]string{
		"Getting started": "Basics", "Invoices": "Basics", "Plans": "Billing", "Refunds": "Refunds",
		// Disputes sits too deep, so its article lands in Refunds.
		"Chargebacks": "Refunds", "Hidden tips": publicImportLinkedCollectionName, "Deeper tips": publicImportLinkedCollectionName,
	} {
		if got := articles[title]; got.Collection != want || got.Status != model.DocStatusPublished || got.Published == nil || got.PublicID == "" {
			t.Errorf("%s: collection=%q status=%q published=%v public_id=%q; want collection %q, published", title, got.Collection, got.Status, got.Published, got.PublicID, want)
		}
	}
	if started.Slug != "getting-started" || derefString(started.Excerpt) != "First steps with Acme" || derefString(articles["Hidden tips"].Excerpt) != "Not in any collection" {
		t.Fatalf("slug/excerpt: %q %q %q", started.Slug, derefString(started.Excerpt), derefString(articles["Hidden tips"].Excerpt))
	}
	// Source order survives: Getting started before Invoices in Basics.
	if started.Position >= invoices.Position {
		t.Fatalf("article order: getting started at %d, invoices at %d", started.Position, invoices.Position)
	}
	var billing, refunds model.DocsCollection
	db.Where("space_id = ? AND slug = ?", space.ID, "billing").First(&billing)
	db.Where("space_id = ? AND slug = ?", space.ID, "refunds").First(&refunds)
	if refunds.ParentCollectionID == nil || *refunds.ParentCollectionID != billing.ID {
		t.Fatalf("refunds parent = %v, want billing", refunds.ParentCollectionID)
	}

	for _, want := range []string{
		`"href":"/articles/invoices-` + invoices.PublicID + `#pay"`,
		`"href":"/c/billing-` + billing.PublicID + `"`,
		`"href":"/articles/hidden-tips-` + articles["Hidden tips"].PublicID + `"`,
		// Kept as written: the dead link, another company's article, another site.
		site.URL + `/en/articles/404-gone"`,
		`"https://intercom.help/other/en/articles/12-theirs"`,
		`"https://example.org/"`,
		// The copied image, at its display width, and the one left in place.
		`"src":"https://files.helpin.example/`, `"width":"550px"`, `"alignment":"center"`,
		`/img/not-an-image.png"`,
		// The heading anchor the rewritten "#pay" link relies on.
		`"id":"welcome"`,
	} {
		if !strings.Contains(started.Content, want) {
			t.Errorf("getting started content lacks %s\n%s", want, started.Content)
		}
	}
	if !strings.Contains(invoices.Content, `"href":"/articles/getting-started-`+started.PublicID+`"`) ||
		!strings.Contains(invoices.Content, `"src":"https://files.helpin.example/`) {
		t.Errorf("invoices content: %s", invoices.Content)
	}
	if !strings.Contains(articles["Chargebacks"].Content, `"href":"/c/refunds-`+refunds.PublicID+`"`) {
		t.Errorf("link to a folded collection: %s", articles["Chargebacks"].Content)
	}
	// What visitors read is the rewritten version, not the first publication.
	var live string
	if err := db.Raw(`SELECT content::text FROM docs_helpcenter_article_publications WHERE document_id = ? LIMIT 1`, started.DocumentID).Scan(&live).Error; err != nil {
		t.Fatal(err)
	}
	live = compactTestJSON(t, live)
	if !strings.Contains(live, "/articles/invoices-"+invoices.PublicID) || strings.Contains(live, site.URL+"/en/articles/12-invoices") {
		t.Errorf("published content still has source links: %s", live)
	}
	var redirects int64
	db.Model(&model.DocsRedirect{}).Where("workspace_id = ? AND source_system = ? AND source_path IN ?", workspaceID, docsImportSourceIntercomPublic,
		[]string{"/en/articles/11-getting-started", "/en/collections/2-billing"}).Count(&redirects)
	if redirects != 2 {
		t.Fatalf("redirects for old addresses = %d, want 2", redirects)
	}

	// Running again imports nothing twice and changes no link.
	articleRequests := site.RequestCount(".json")
	second := runPublicDocsImport(t, svc, site.URL, workspaceID, space.ID, userID, "published")
	var secondSummary model.ImportSummary
	if err := json.Unmarshal(second.Summary, &secondSummary); err != nil {
		t.Fatal(err)
	}
	var documents, collections int64
	db.Model(&model.DocsDocument{}).Where("space_id = ? AND deleted_at IS NULL", space.ID).Count(&documents)
	db.Model(&model.DocsCollection{}).Where("space_id = ? AND deleted_at IS NULL", space.ID).Count(&collections)
	if second.Status != model.DocsImportStatusDone || second.Failed != 0 || documents != 7 || collections != 4 ||
		secondSummary.LinksRewritten != 0 || secondSummary.CollectionsCreated != 0 || len(uploader.uploads) != 1 {
		t.Fatalf("second import: status=%s failed=%d documents=%d collections=%d summary=%+v uploads=%d",
			second.Status, second.Failed, documents, collections, secondSummary, len(uploader.uploads))
	}
	// Only the dead link is looked up again; no imported article is re-read.
	if extra := site.RequestCount(".json") - articleRequests; extra != 1 {
		t.Fatalf("second import read %d articles, want only the dead link lookup", extra)
	}
	if secondSummary.UnresolvedLinksTotal != 1 {
		t.Fatalf("second import unresolved links = %d, want the dead link reported again", secondSummary.UnresolvedLinksTotal)
	}
}

func TestPublicHelpCenterImportPartialFailurePostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	site := seedPublicImportSite()
	defer site.Close()
	plain := &http.Client{Timeout: 10 * time.Second}
	svc := newIntercomDocsImportService(db, "")
	svc.publicIntercom = intercom.NewPublicClientWithHTTP(plain)

	broken := site.Articles["13"]
	broken.Status = http.StatusForbidden
	site.Articles["13"] = broken

	space, err := svc.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{Name: "Help", Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeExternalCapable}, userID)
	if err != nil {
		t.Fatal(err)
	}
	// Drafts: nothing is published, and without storage images stay put.
	first := runPublicDocsImport(t, svc, site.URL, workspaceID, space.ID, userID, "draft")
	var failures []model.ImportFailure
	if err := json.Unmarshal(first.Failures, &failures); err != nil {
		t.Fatal(err)
	}
	if first.Status != model.DocsImportStatusDone || first.Completed != 6 || first.Failed != 1 || len(failures) != 1 ||
		failures[0].Title != "Plans" || !strings.Contains(failures[0].Error, "sign-in") {
		t.Fatalf("first import: status=%s completed=%d failed=%d failures=%s", first.Status, first.Completed, first.Failed, first.Failures)
	}
	var summary model.ImportSummary
	if err := json.Unmarshal(first.Summary, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.ArticlesDrafted != 6 || summary.ArticlesPublished != 0 || summary.ImagesCopied != 0 || summary.FailedImagesTotal != 3 || summary.LinksRewritten != 6 {
		t.Fatalf("draft summary: %+v", summary)
	}
	articles := loadPublicImportedArticles(t, db, space.ID)
	if len(articles) != 6 || articles["Getting started"].Status == model.DocStatusPublished || articles["Getting started"].Published != nil {
		t.Fatalf("draft import published something: %+v", articles["Getting started"])
	}

	// The article comes back; a retry imports just that one.
	broken.Status = 0
	site.Articles["13"] = broken
	second := runPublicDocsImport(t, svc, site.URL, workspaceID, space.ID, userID, "draft")
	var documents int64
	db.Model(&model.DocsDocument{}).Where("space_id = ? AND deleted_at IS NULL", space.ID).Count(&documents)
	if second.Status != model.DocsImportStatusDone || second.Completed != 7 || second.Failed != 0 || documents != 7 {
		t.Fatalf("retry: status=%s completed=%d failed=%d documents=%d failures=%s", second.Status, second.Completed, second.Failed, documents, second.Failures)
	}
	// Plans was created last but belongs before the Refunds subcollection.
	plans := loadPublicImportedArticles(t, db, space.ID)["Plans"]
	var refunds model.DocsCollection
	db.Where("space_id = ? AND slug = ?", space.ID, "refunds").First(&refunds)
	if plans.Collection != "Billing" || plans.Position >= refunds.Position {
		t.Fatalf("plans: collection=%q position=%d, refunds position=%d", plans.Collection, plans.Position, refunds.Position)
	}
}

func TestPublicHelpCenterImportIntoUsedSpacePostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	site := seedPublicImportSite()
	defer site.Close()
	svc := newIntercomDocsImportService(db, "")
	svc.publicIntercom = intercom.NewPublicClientWithHTTP(&http.Client{Timeout: 10 * time.Second})

	space, err := svc.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{Name: "Help", Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeExternalCapable}, userID)
	if err != nil {
		t.Fatal(err)
	}
	// The space already has its own collections, in an order someone chose,
	// and an article imported from a different help center.
	var own []string
	for _, name := range []string{"Our guides", "Our FAQ"} {
		collection, err := svc.collectionSvc.Create(ctx, workspaceID, space.ID, model.CreateDocsCollectionRequest{Name: name}, userID)
		if err != nil {
			t.Fatal(err)
		}
		own = append(own, collection.ID)
	}
	foreign, err := svc.documentSvc.Create(ctx, workspaceID, model.CreateDocsDocumentRequest{SpaceID: space.ID, CollectionID: &own[0], Title: "From elsewhere"}, userID)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := svc.contentSvc.Save(ctx, foreign.ID, json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`), userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.contentSvc.SetImportProvenance(ctx, saved.ID, "", docsImportSourceIntercomPublic, "555001"); err != nil {
		t.Fatal(err)
	}
	positions := func() map[string]int {
		var rows []model.DocsCollection
		db.Where("id IN ?", own).Find(&rows)
		out := map[string]int{}
		for _, row := range rows {
			out[row.ID] = row.Position
		}
		return out
	}
	before := positions()
	var foreignBefore model.DocsDocument
	db.First(&foreignBefore, "id = ?", foreign.ID)

	first := runPublicDocsImport(t, svc, site.URL, workspaceID, space.ID, userID, "draft")
	second := runPublicDocsImport(t, svc, site.URL, workspaceID, space.ID, userID, "draft")
	for name, job := range map[string]model.DocsImportJob{"first": first, "second": second} {
		// Seven articles of this help center; the other help center's article
		// is not counted as one of them.
		if job.Status != model.DocsImportStatusDone || job.Total != 7 || job.Completed != 7 || job.Failed != 0 {
			t.Fatalf("%s import: status=%s total=%d completed=%d failed=%d", name, job.Status, job.Total, job.Completed, job.Failed)
		}
	}
	after := positions()
	if after[own[0]] != before[own[0]] || after[own[1]] != before[own[1]] {
		t.Fatalf("the space's own collections moved: before %v, after %v", before, after)
	}
	var foreignAfter model.DocsDocument
	db.First(&foreignAfter, "id = ?", foreign.ID)
	if foreignAfter.Position != foreignBefore.Position || derefString(foreignAfter.CollectionID) != own[0] {
		t.Fatalf("the other help center's article moved: %+v -> %+v", foreignBefore, foreignAfter)
	}
	var documents int64
	db.Model(&model.DocsDocument{}).Where("space_id = ? AND deleted_at IS NULL", space.ID).Count(&documents)
	if documents != 8 {
		t.Fatalf("documents = %d, want 7 imported once plus the existing one", documents)
	}
}

func TestPublicHelpCenterImportUnusableSourcePostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	site := seedPublicImportSite()
	defer site.Close()
	site.NotHelpCenter = true
	svc := newIntercomDocsImportService(db, "")
	svc.publicIntercom = intercom.NewPublicClientWithHTTP(&http.Client{Timeout: 10 * time.Second})

	if _, err := svc.PreviewPublicHelpCenter(ctx, model.PublicHelpCenterImportRequest{URL: site.URL}); err == nil || !strings.Contains(err.Error(), "not an Intercom help center") {
		t.Fatalf("preview error = %v", err)
	}
	space, err := svc.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{Name: "Help", Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeExternalCapable}, userID)
	if err != nil {
		t.Fatal(err)
	}
	job := runPublicDocsImport(t, svc, site.URL, workspaceID, space.ID, userID, "published")
	if job.Status != model.DocsImportStatusFailed || job.Error == nil || !strings.Contains(*job.Error, "not an Intercom help center") {
		t.Fatalf("job: status=%s error=%v", job.Status, job.Error)
	}
	var documents, collections int64
	db.Model(&model.DocsDocument{}).Where("space_id = ?", space.ID).Count(&documents)
	db.Model(&model.DocsCollection{}).Where("space_id = ?", space.ID).Count(&collections)
	if documents != 0 || collections != 0 {
		t.Fatalf("a failed read left %d documents and %d collections", documents, collections)
	}
}
