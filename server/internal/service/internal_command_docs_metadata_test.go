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
		"document_id":"doc-meta","title":"New title","clear_owner":true,"clear_excerpt":true,"tags":["runbook","approved"],"is_pinned":true
	}`))
	if err != nil {
		t.Fatalf("update document metadata: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if result["title"] != "New title" || result["is_pinned"] != true {
		t.Fatalf("unexpected result: %#v", result)
	}
	updated, err := docRepo.GetByID(context.Background(), doc.ID)
	if err != nil {
		t.Fatalf("load updated document: %v", err)
	}
	if updated.OwnerID != nil || updated.Excerpt != nil || updated.Title != "New title" || !updated.IsPinned || len(updated.Tags) != 2 {
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
