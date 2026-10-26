//go:build integration

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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
	if err != nil || preview.WorkspaceName != "Acme" || preview.Collections != 2 || preview.ArticlesPublished != 2 || preview.ArticlesDraft != 1 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if _, err := svc.PreviewIntercom(ctx, model.IntercomDocsImportRequest{AccessToken: "wrong"}); err == nil {
		t.Fatal("a rejected token previewed")
	}

	space, err := svc.spaceSvc.Create(ctx, workspaceID, model.CreateDocsSpaceRequest{Name: "Help", Visibility: model.SpaceVisibilityWorkspaceWide, Type: model.SpaceTypeExternalCapable}, userID)
	if err != nil {
		t.Fatal(err)
	}
	run := func() model.DocsImportJob {
		t.Helper()
		job := &model.DocsImportJob{ID: uuid.NewString(), WorkspaceID: workspaceID, SpaceID: &space.ID, Source: docsImportSourceIntercom,
			Status: model.DocsImportStatusPending, Failures: json.RawMessage("[]"), Config: json.RawMessage("{}"), StartedBy: userID}
		if err := svc.importRepo.Create(ctx, job); err != nil {
			t.Fatal(err)
		}
		if err := svc.runIntercomImport(ctx, job.ID, model.DocsImportStartRequest{APIKey: intercomtest.Token}, space.ID, workspaceID, userID, nil); err != nil {
			t.Fatal(err)
		}
		done, err := svc.importRepo.GetByID(ctx, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		return *done
	}
	first := run()
	if first.Status != model.DocsImportStatusDone || first.Completed != 3 || first.Failed != 0 {
		t.Fatalf("first import: status=%s completed=%d failed=%d failures=%s error=%v", first.Status, first.Completed, first.Failed, first.Failures, first.Error)
	}
	var summary model.ImportSummary
	_ = json.Unmarshal(first.Summary, &summary)
	if summary.CollectionsCreated != 2 || summary.ArticlesPublished != 2 || summary.ArticlesDrafted != 1 || summary.ArticlesUncategorized != 1 {
		t.Fatalf("summary: %+v", summary)
	}
	var collections []model.DocsCollection
	db.Where("space_id = ?", space.ID).Order("slug").Find(&collections)
	if len(collections) != 2 || collections[0].Slug != "billing" || collections[1].Slug != "invoices" ||
		collections[1].ParentCollectionID == nil || *collections[1].ParentCollectionID != collections[0].ID {
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
	if second.Status != model.DocsImportStatusDone || second.Completed != 3 || documents != 3 {
		t.Fatalf("second import: status=%s completed=%d documents=%d", second.Status, second.Completed, documents)
	}
	_ = time.Second
}
