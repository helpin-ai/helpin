//go:build integration

package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/intercom/intercomtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func newIntercomDocsImportService(db *gorm.DB, baseURL string) *DocsImportService {
	spaceRepo := repository.NewDocsSpaceRepository(db)
	collectionRepo := repository.NewDocsCollectionRepository(db)
	documentRepo := repository.NewDocsDocumentRepository(db)
	contentRepo := repository.NewDocsContentRepository(db)
	redirectRepo := repository.NewDocsRedirectRepository(db)
	helpcenter := NewDocsHelpcenterService(repository.NewDocsHelpcenterRepository(db), repository.NewDocsHelpcenterPublicationRepository(db),
		documentRepo, contentRepo, spaceRepo, collectionRepo, redirectRepo, nil, nil)
	svc := NewDocsImportService(repository.NewDocsImportRepository(db), NewDocsSpaceService(spaceRepo, nil),
		NewDocsCollectionService(collectionRepo, spaceRepo, nil, false), NewDocsDocumentService(documentRepo, spaceRepo, nil, false),
		NewDocsContentService(contentRepo, documentRepo, nil), helpcenter, redirectRepo, nil, nil, DocsImportAIConversionConfig{})
	svc.SetIntercomBaseURL(baseURL)
	return svc
}

func TestIntercomDocsImportPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	fake := intercomtest.New()
	defer fake.Close()
	parent := "100"
	fake.Collections = []intercom.Collection{
		{ID: "101", Name: "Invoices", URL: "https://intercom.help/acme/en/collections/101-invoices", ParentID: &parent, Order: 1},
		{ID: "100", Name: "Billing", Description: "Plans and payments", URL: "https://intercom.help/acme/en/collections/100-billing"},
		// Two collections that share a slug under different parents stay
		// apart.
		{ID: "102", Name: "FAQ", URL: "https://intercom.help/acme/en/collections/102-faq", ParentID: &parent, Order: 2},
		{ID: "103", Name: "FAQ", URL: "https://intercom.help/acme/en/collections/103-faq", Order: 3},
	}
	parentID := int64(101)
	collectionType := "collection"
	fake.Articles = []intercom.Article{
		{ID: "5001", Title: "Download an invoice", State: "published", URL: "https://intercom.help/acme/en/articles/5001-download-an-invoice",
			Body: "<h2>Steps</h2><p>Open <strong>Billing</strong> and click Download.</p><ul><li>PDF</li><li>CSV</li></ul>", ParentID: &parentID, ParentType: &collectionType},
		{ID: "5002", Title: "Upcoming pricing", State: "draft", Body: "<p>Not yet public.</p>", ParentID: &parentID, ParentType: &collectionType},
		{ID: "5003", Title: "Loose article", State: "published", URL: "https://intercom.help/acme/en/articles/5003-loose-article", Body: "<p>No collection.</p>"},
	}
	svc := newIntercomDocsImportService(db, fake.URL)

	preview, err := svc.PreviewIntercom(ctx, model.IntercomDocsImportRequest{AccessToken: intercomtest.Token})
	if err != nil || preview.WorkspaceName != "Acme" || preview.Collections != 4 || preview.ArticlesPublished != 2 || preview.ArticlesDraft != 1 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if _, err := svc.PreviewIntercom(ctx, model.IntercomDocsImportRequest{AccessToken: "wrong"}); err == nil {
		t.Fatal("a rejected token previewed")
	}

	space, err := svc.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{Name: "Help", Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeExternalCapable}, userID)
	if err != nil {
		t.Fatal(err)
	}
	// An unrelated collection already uses the "faq" slug.
	if _, err := svc.collectionSvc.Create(ctx, workspaceID, space.ID, model.CreateDocsCollectionRequest{Name: "Old questions", Slug: strp("faq")}, userID); err != nil {
		t.Fatal(err)
	}
	run := func() model.DocsImportJob {
		t.Helper()
		return runIntercomDocsImport(t, svc, workspaceID, space.ID, userID)
	}
	first := run()
	if first.Status != model.DocsImportStatusDone || first.Completed != 3 || first.Failed != 0 {
		t.Fatalf("first import: status=%s completed=%d failed=%d failures=%s error=%v", first.Status, first.Completed, first.Failed, first.Failures, first.Error)
	}
	var summary model.ImportSummary
	_ = json.Unmarshal(first.Summary, &summary)
	if summary.CollectionsCreated != 4 || summary.ArticlesPublished != 2 || summary.ArticlesDrafted != 1 || summary.ArticlesUncategorized != 1 {
		t.Fatalf("summary: %+v", summary)
	}
	var collections []model.DocsCollection
	db.Where("space_id = ?", space.ID).Order("slug").Find(&collections)
	bySlug := map[string]model.DocsCollection{}
	for _, collection := range collections {
		bySlug[collection.Slug] = collection
	}
	billing, invoices := bySlug["billing"], bySlug["invoices"]
	if len(collections) != 5 || invoices.ParentCollectionID == nil || *invoices.ParentCollectionID != billing.ID ||
		bySlug["faq"].Name != "Old questions" || bySlug["faq-2"].Name != "FAQ" || bySlug["faq-3"].Name != "FAQ" ||
		derefString(bySlug["faq-3"].ParentCollectionID) != billing.ID || bySlug["faq-2"].ParentCollectionID != nil {
		t.Fatalf("collections: %+v", collections)
	}
	var redirects int64
	db.Model(&model.DocsRedirect{}).Where("workspace_id = ? AND source_system = 'intercom' AND source_path = ?", workspaceID, "/acme/en/articles/5001-download-an-invoice").Count(&redirects)
	if redirects != 1 {
		t.Fatalf("article redirect missing: %d", redirects)
	}

	// Running again imports nothing twice.
	second := run()
	var documents int64
	db.Model(&model.DocsDocument{}).Where("space_id = ?", space.ID).Count(&documents)
	var collectionCount int64
	db.Model(&model.DocsCollection{}).Where("space_id = ?", space.ID).Count(&collectionCount)
	if second.Status != model.DocsImportStatusDone || second.Completed != 3 || documents != 3 || collectionCount != 5 {
		t.Fatalf("second import: status=%s completed=%d documents=%d collections=%d", second.Status, second.Completed, documents, collectionCount)
	}

	// An article that fails part way leaves no document behind, so the next
	// run does not duplicate it.
	internal, err := svc.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{Name: "Internal", Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeInternal}, userID)
	if err != nil {
		t.Fatal(err)
	}
	failed := runIntercomDocsImport(t, svc, workspaceID, internal.ID, userID)
	var leftovers int64
	db.Model(&model.DocsDocument{}).Where("space_id = ?", internal.ID).Count(&leftovers)
	if failed.Failed == 0 || leftovers != int64(failed.Completed) {
		t.Fatalf("failed import: completed=%d failed=%d documents=%d failures=%s", failed.Completed, failed.Failed, leftovers, failed.Failures)
	}
	if _, err := svc.StartIntercom(ctx, model.IntercomDocsImportRequest{AccessToken: intercomtest.Token, TargetSpaceID: &internal.ID}, workspaceID, userID); err == nil ||
		!strings.Contains(err.Error(), "Help Center") {
		t.Fatalf("an import into an internal space started: %v", err)
	}
}

func runIntercomDocsImport(t *testing.T, svc *DocsImportService, workspaceID, spaceID, userID string) model.DocsImportJob {
	t.Helper()
	ctx := context.Background()
	job := &model.DocsImportJob{ID: uuid.NewString(), WorkspaceID: workspaceID, SpaceID: &spaceID, Source: docsImportSourceIntercom,
		Status: model.DocsImportStatusPending, Failures: json.RawMessage("[]"), Config: json.RawMessage("{}"), StartedBy: userID}
	if err := svc.importRepo.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	if err := svc.runIntercomImport(ctx, job.ID, model.DocsImportStartRequest{APIKey: intercomtest.Token}, spaceID, workspaceID, userID, nil); err != nil {
		t.Fatal(err)
	}
	done, err := svc.importRepo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	return *done
}
