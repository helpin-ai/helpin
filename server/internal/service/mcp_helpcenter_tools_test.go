package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestGetHelpCenterArticle(t *testing.T) {
	env, admin := setupMCPHelpcenterTest(t)
	admin.markLive()
	admin.article.HelpfulCount, admin.article.NotHelpfulCount, admin.article.ViewCount = 4, 1, 90
	result, err := env.run("get_help_center_article", `{"document_id":"document-1"}`)
	if err != nil {
		t.Fatalf("get_help_center_article error = %v", err)
	}
	data := result.Data.(map[string]any)
	feedback := data["feedback"].(map[string]int)
	if data["help_center_live"] != true || data["live_slug"] != "guide" || feedback["helpful"] != 4 || feedback["views"] != 90 {
		t.Fatalf("article data = %#v", data)
	}
}

func TestUpdateHelpCenterArticleMetadata(t *testing.T) {
	t.Run("keeps omitted fields and clears nulls", func(t *testing.T) {
		env, admin := setupMCPHelpcenterTest(t)
		_, err := env.run("update_help_center_article_metadata",
			`{"document_id":"document-1","og_title":"  Getting started  ","og_image_alt":null}`)
		if err != nil {
			t.Fatalf("update error = %v", err)
		}
		request := admin.metadataRequest
		if request.OGTitle == nil || *request.OGTitle != "Getting started" || request.OGDescription != nil ||
			request.OGImageAlt == nil || *request.OGImageAlt != "" {
			t.Fatalf("metadata request = %#v", request)
		}
	})
	t.Run("refuses a non-HTTPS image", func(t *testing.T) {
		env, _ := setupMCPHelpcenterTest(t)
		_, err := env.run("update_help_center_article_metadata",
			`{"document_id":"document-1","og_image_url":"http://example.com/a.png"}`)
		assertMCPToolErrorCode(t, err, MCPErrorCodeURLNotPublic)
	})
	t.Run("hides another workspace's document", func(t *testing.T) {
		env, admin := setupMCPHelpcenterTest(t)
		env.documents.documents["document-1"].WorkspaceID = "workspace-other"
		_, err := env.run("update_help_center_article_metadata", `{"document_id":"document-1","og_title":"x"}`)
		if !errors.Is(err, ErrMCPNotFound) || admin.metadataCalls != 0 {
			t.Fatalf("error = %v, metadata calls = %d", err, admin.metadataCalls)
		}
	})
}

func TestHelpCenterRedirectTools(t *testing.T) {
	env, admin := setupMCPHelpcenterTest(t)
	if _, err := env.run("create_help_center_redirect",
		`{"source_path":"/articles/old-roadmap","target_collection_slug":"views","target_article_slug":""}`); err != nil {
		t.Fatalf("create redirect error = %v", err)
	}
	if admin.redirectRequest.SourcePath != "/articles/old-roadmap" || admin.redirectRequest.TargetArticleSlug != nil {
		t.Fatalf("redirect request = %#v", admin.redirectRequest)
	}
	result, err := env.run("list_help_center_redirects", `{"per_page":500}`)
	if err != nil {
		t.Fatalf("list redirects error = %v", err)
	}
	if admin.listFilter.PerPage != 100 || admin.listFilter.Page != 1 || result.Data.(map[string]any)["total"] != int64(1) {
		t.Fatalf("filter = %#v, result = %#v", admin.listFilter, result.Data)
	}
}

func setupMCPHelpcenterTest(t *testing.T) (*mcpDocsLifecycleTestEnv, *fakeMCPHelpcenterAdmin) {
	t.Helper()
	env := setupMCPDocsLifecycleTest(t, model.DocStatusPublished, model.SpaceTypeExternalCapable)
	admin := &fakeMCPHelpcenterAdmin{fakeMCPHelpcenterPublisher: env.helpcenter}
	env.service.helpcenter = admin
	return env, admin
}

type fakeMCPHelpcenterAdmin struct {
	*fakeMCPHelpcenterPublisher
	metadataCalls   int
	metadataRequest model.UpdateDocsHelpcenterArticleMetadataRequest
	redirectRequest model.CreateDocsRedirectRequest
	listFilter      model.DocsRedirectFilter
}

func (f *fakeMCPHelpcenterAdmin) EnrichDocumentPublishState(_ context.Context, document *model.DocsDocument) error {
	if f.article != nil && f.article.PublicPublishedAt != nil {
		live := *f.article.PublicPublishedAt
		slug := f.article.Slug
		document.LivePublishedAt, document.LiveSlug = &live, &slug
	}
	return nil
}

func (f *fakeMCPHelpcenterAdmin) UpdateArticleMetadata(_ context.Context, _, documentID string, request model.UpdateDocsHelpcenterArticleMetadataRequest) (*model.DocsHelpcenterArticle, error) {
	f.metadataCalls++
	f.metadataRequest = request
	return &model.DocsHelpcenterArticle{DocumentID: documentID, OGTitle: request.OGTitle}, nil
}

func (f *fakeMCPHelpcenterAdmin) ListRedirects(_ context.Context, _ string, filter model.DocsRedirectFilter) ([]model.DocsRedirect, int64, error) {
	f.listFilter = filter
	return []model.DocsRedirect{{ID: "redirect-1", CreatedAt: time.Now()}}, 1, nil
}

func (f *fakeMCPHelpcenterAdmin) CreateRedirect(_ context.Context, _ string, request model.CreateDocsRedirectRequest) (*model.DocsRedirect, error) {
	f.redirectRequest = request
	return &model.DocsRedirect{ID: "redirect-1"}, nil
}
