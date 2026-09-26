package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestDocsUpdateDocumentMetadataCommandUpdatesOnlyBoundedFields(t *testing.T) {
	db := setupDocsDeletionTestDB(t)
	space := model.DocsSpace{ID: "space-doc-meta", WorkspaceID: "ws-doc-meta", Name: "Internal", Slug: "internal", Type: model.SpaceTypeInternal, Visibility: model.SpaceVisibilityWorkspaceWide, CreatedBy: "owner-doc-meta"}
	if err := db.Create(&space).Error; err != nil {
		t.Fatalf("seed space: %v", err)
	}
	ownerID := "owner-doc-meta"
	excerpt := "old excerpt"
	doc := model.DocsDocument{ID: "doc-meta", WorkspaceID: "ws-doc-meta", SpaceID: space.ID, Title: "Old title", Status: model.DocStatusDraft, Visibility: model.SpaceVisibilityWorkspaceWide, OwnerID: &ownerID, Excerpt: &excerpt, CreatedBy: ownerID}
	if err := db.Create(&doc).Error; err != nil {
		t.Fatalf("seed document: %v", err)
	}

	docRepo := repository.NewDocsDocumentRepository(db)
	spaceRepo := repository.NewDocsSpaceRepository(db)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetDocsCreateDependencies(NewDocsDocumentService(docRepo, spaceRepo, nil, false), nil)
	svc.SetDocsOrganizationServices(NewDocsSpaceService(spaceRepo, nil), nil)

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-doc-meta", ActorID: ownerID, ActorRole: model.RoleOwner, TargetType: "document", TargetID: doc.ID}, "docs.update_document_metadata", json.RawMessage(`{
		"document_id":"doc-meta","title":"GTM &amp; Marketing","clear_owner":true,"clear_excerpt":true,"tags":["runbook","approved"],"is_pinned":true
	}`))
	if err != nil {
		t.Fatalf("update document metadata: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if result["title"] != "GTM & Marketing" || result["is_pinned"] != true {
		t.Fatalf("unexpected result: %#v", result)
	}
	updated, err := docRepo.GetByID(context.Background(), doc.ID)
	if err != nil {
		t.Fatalf("load updated document: %v", err)
	}
	if updated.OwnerID != nil || updated.Excerpt != nil || updated.Title != "GTM & Marketing" || !updated.IsPinned || len(updated.Tags) != 2 {
		t.Fatalf("bounded metadata was not persisted: %#v", updated)
	}
}

func TestDocsUpdateDocumentMetadataRejectsCrossWorkspaceDocument(t *testing.T) {
	db := setupDocsDeletionTestDB(t)
	space := model.DocsSpace{ID: "space-doc-b", WorkspaceID: "ws-doc-b", Name: "B", Slug: "b", Type: model.SpaceTypeInternal, Visibility: model.SpaceVisibilityWorkspaceWide, CreatedBy: "owner-doc-b"}
	if err := db.Create(&space).Error; err != nil {
		t.Fatal(err)
	}
	doc := model.DocsDocument{ID: "doc-b", WorkspaceID: "ws-doc-b", SpaceID: space.ID, Title: "Secret", Status: model.DocStatusDraft, Visibility: model.SpaceVisibilityWorkspaceWide, CreatedBy: "owner-doc-b"}
	if err := db.Create(&doc).Error; err != nil {
		t.Fatal(err)
	}
	docRepo := repository.NewDocsDocumentRepository(db)
	spaceRepo := repository.NewDocsSpaceRepository(db)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetDocsCreateDependencies(NewDocsDocumentService(docRepo, spaceRepo, nil, false), nil)
	svc.SetDocsOrganizationServices(NewDocsSpaceService(spaceRepo, nil), nil)
	_, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-doc-a", ActorID: "owner-doc-a", ActorRole: model.RoleOwner}, "docs.update_document_metadata", json.RawMessage(`{"document_id":"doc-b","title":"Stolen"}`))
	if err == nil || !strings.Contains(err.Error(), "document not found") {
		t.Fatalf("expected scoped not-found error, got %v", err)
	}
}

func TestDocsUpdateDocumentMetadataRejectsConflictingTarget(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	_, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws", TargetType: "document", TargetID: "doc-1"}, "docs.update_document_metadata", json.RawMessage(`{"document_id":"doc-2","title":"No"}`))
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("expected conflicting target error, got %v", err)
	}
}

func TestCreateDocumentNormalizesAgentTitleOnce(t *testing.T) {
	for _, tt := range []struct{ title, want string }{
		{"Helpin Open Source — GTM &amp; Marketing Plan", "Helpin Open Source — GTM & Marketing Plan"},
		{"Examples &amp;amp;", "Examples &amp;"},
		{"GTM & Marketing", "GTM & Marketing"},
	} {
		t.Run(tt.title, func(t *testing.T) {
			db := setupDocsDeletionTestDB(t)
			space := model.DocsSpace{ID: "space", WorkspaceID: "ws", Name: "Internal", Slug: "internal", Type: model.SpaceTypeInternal, Visibility: model.SpaceVisibilityWorkspaceWide, CreatedBy: "owner"}
			if err := db.Create(&space).Error; err != nil {
				t.Fatal(err)
			}
			repo := repository.NewDocsDocumentRepository(db)
			svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
			svc.SetDocsCreateDependencies(NewDocsDocumentService(repo, repository.NewDocsSpaceRepository(db), nil, false), nil)
			input, _ := json.Marshal(map[string]string{"space_id": "space", "title": tt.title})
			output, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws", ActorID: "owner", ActorRole: model.RoleOwner}, "docs.create_document", input)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Title string `json:"title"`
			}
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatal(err)
			}
			if result.Title != tt.want {
				t.Fatalf("title = %q, want %q", result.Title, tt.want)
			}
			var stored model.DocsDocument
			if err := db.First(&stored).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Title != tt.want {
				t.Fatalf("stored title = %q, want %q", stored.Title, tt.want)
			}
		})
	}
}
