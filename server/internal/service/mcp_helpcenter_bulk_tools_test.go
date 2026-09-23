package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPublishDocumentsContinuesPastPerDocumentErrors(t *testing.T) {
	env, _ := setupMCPHelpcenterBulkTest(t)
	env.documents.documents["document-1"].Status = model.DocStatusDraft
	env.documents.documents["document-locked"] = &model.DocsDocument{
		ID: "document-locked", WorkspaceID: env.principal.WorkspaceID, SpaceID: "space-1",
		Title: "Locked", Status: model.DocStatusDraft, IsLocked: true,
	}
	result, err := env.run("publish_documents", `{"document_ids":["document-1","missing","document-locked"]}`)
	if err != nil {
		t.Fatalf("publish_documents error = %v", err)
	}
	data := result.Data.(map[string]any)
	results := data["results"].([]map[string]any)
	want := []struct{ status, code string }{
		{mcpBatchPublishStatusPublished, ""},
		{mcpBatchPublishStatusError, "NOT_FOUND"},
		{mcpBatchPublishStatusError, MCPErrorCodeDocumentLocked},
	}
	if len(results) != len(want) {
		t.Fatalf("results = %#v", results)
	}
	for index, expected := range want {
		if results[index]["status"] != expected.status || (expected.code != "" && results[index]["code"] != expected.code) {
			t.Errorf("result %d = %#v, want status %s code %s", index, results[index], expected.status, expected.code)
		}
	}
	if results[0]["live_slug"] != "derived" {
		t.Errorf("live_slug = %v, want derived", results[0]["live_slug"])
	}
	if data["published"] != 1 || data["failed"] != 2 || data["complete"] != true {
		t.Fatalf("counts = %#v", data)
	}
	if env.lifecycle.publishCalls != 1 {
		t.Fatalf("publish calls = %d, want 1", env.lifecycle.publishCalls)
	}
}

func TestPublishDocumentsOnlyIfChanged(t *testing.T) {
	tests := []struct {
		name          string
		unpublished   bool
		wantStatus    string
		wantPublishes int
	}{
		{name: "skips a live article without changes", unpublished: false, wantStatus: mcpBatchPublishStatusSkipped},
		{name: "publishes a live article with changes", unpublished: true, wantStatus: mcpBatchPublishStatusPublished, wantPublishes: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, admin := setupMCPHelpcenterBulkTest(t)
			admin.markLive()
			admin.unpublishedChanges = tt.unpublished
			result, err := env.run("publish_documents", `{"document_ids":["document-1"],"only_if_changed":true}`)
			if err != nil {
				t.Fatalf("publish_documents error = %v", err)
			}
			entry := result.Data.(map[string]any)["results"].([]map[string]any)[0]
			if entry["status"] != tt.wantStatus {
				t.Fatalf("status = %v, want %s", entry["status"], tt.wantStatus)
			}
			if entry["live_slug"] == nil {
				t.Fatalf("live_slug missing from %#v", entry)
			}
			if env.lifecycle.publishCalls != tt.wantPublishes {
				t.Fatalf("publish calls = %d, want %d", env.lifecycle.publishCalls, tt.wantPublishes)
			}
		})
	}
}

func TestPublishDocumentsStopsBeforeDeadline(t *testing.T) {
	env, _ := setupMCPHelpcenterBulkTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := env.service.executeSpecialMCPTool(ctx, env.principal, env.actor, "publish_documents",
		json.RawMessage(`{"document_ids":["document-1","document-2"]}`))
	if err != nil {
		t.Fatalf("publish_documents error = %v", err)
	}
	data := result.Data.(map[string]any)
	remaining := data["remaining_document_ids"].([]string)
	if data["complete"] != false || len(remaining) != 2 || env.lifecycle.publishCalls != 0 {
		t.Fatalf("data = %#v, publish calls = %d", data, env.lifecycle.publishCalls)
	}
}

func TestPublishDocumentsRejectsOversizedBatch(t *testing.T) {
	env, _ := setupMCPHelpcenterBulkTest(t)
	ids := make([]string, _mcpPublishBatchMaxDocuments+1)
	for index := range ids {
		ids[index] = "document-" + string(rune('a'+index%26)) + string(rune('a'+index/26))
	}
	arguments, _ := json.Marshal(map[string]any{"document_ids": ids})
	_, err := env.run("publish_documents", string(arguments))
	if !errors.Is(err, ErrMCPInvalidArguments) {
		t.Fatalf("error = %v, want ErrMCPInvalidArguments", err)
	}
}

func TestUpdateHelpCenterCollectionSlug(t *testing.T) {
	t.Run("renames a section-numbered slug", func(t *testing.T) {
		env, admin := setupMCPHelpcenterBulkTest(t)
		result, err := env.run("update_help_center_collection_slug", `{"collection_id":"collection-1","slug":"Getting Started"}`)
		if err != nil {
			t.Fatalf("update slug error = %v", err)
		}
		if admin.slugCollectionID != "collection-1" || admin.slug != "getting-started" {
			t.Fatalf("UpdateCollectionSlug(%q, %q)", admin.slugCollectionID, admin.slug)
		}
		data := result.Data.(map[string]any)
		if data["old_slug"] != "01-guides" || data["slug"] != "getting-started" || data["redirected_from"] != "/01-guides" {
			t.Fatalf("data = %#v", data)
		}
	})
	t.Run("hides a collection in an inaccessible space", func(t *testing.T) {
		env, admin := setupMCPHelpcenterBulkTest(t)
		env.collections.listResults["space-1"][0].SpaceID = "space-hidden"
		_, err := env.run("update_help_center_collection_slug", `{"collection_id":"collection-1","slug":"guides"}`)
		if !errors.Is(err, ErrMCPNotFound) || admin.slugCollectionID != "" {
			t.Fatalf("error = %v, slug calls for %q", err, admin.slugCollectionID)
		}
	})
	t.Run("rejects a slug without letters or digits", func(t *testing.T) {
		env, _ := setupMCPHelpcenterBulkTest(t)
		_, err := env.run("update_help_center_collection_slug", `{"collection_id":"collection-1","slug":"--"}`)
		if !errors.Is(err, ErrMCPInvalidArguments) {
			t.Fatalf("error = %v, want ErrMCPInvalidArguments", err)
		}
	})
}

type mcpHelpcenterBulkTestEnv struct {
	*mcpDocsLifecycleTestEnv
	collections *fakeMCPDocsCollectionService
}

func setupMCPHelpcenterBulkTest(t *testing.T) (*mcpHelpcenterBulkTestEnv, *fakeMCPHelpcenterSlugAdmin) {
	t.Helper()
	env, admin := setupMCPHelpcenterTest(t)
	env.documents.documents["document-2"] = &model.DocsDocument{
		ID: "document-2", WorkspaceID: env.principal.WorkspaceID, SpaceID: "space-1", Title: "Second", Status: model.DocStatusDraft,
	}
	slugAdmin := &fakeMCPHelpcenterSlugAdmin{fakeMCPHelpcenterAdmin: admin}
	env.service.helpcenter = slugAdmin
	collections := env.service.collections.(*fakeMCPDocsCollectionService)
	collections.listResults["space-1"][0].Slug = "01-guides"
	return &mcpHelpcenterBulkTestEnv{mcpDocsLifecycleTestEnv: env, collections: collections}, slugAdmin
}

type fakeMCPHelpcenterSlugAdmin struct {
	*fakeMCPHelpcenterAdmin
	slugCollectionID string
	slug             string
}

func (f *fakeMCPHelpcenterSlugAdmin) UpdateCollectionSlug(_ context.Context, collectionID, slug string) (*model.DocsCollection, error) {
	f.slugCollectionID, f.slug = collectionID, slug
	return &model.DocsCollection{ID: collectionID, Name: "Guides", Slug: slug}, nil
}
