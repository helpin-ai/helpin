package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/iconcatalog"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestExecuteSpecialMCPSearchIcons(t *testing.T) {
	service := &MCPService{}
	result, err := service.executeSpecialMCPTool(
		context.Background(), &model.MCPPrincipal{}, nil, "search_icons",
		json.RawMessage(`{"query":"rocket","limit":3}`),
	)
	if err != nil {
		t.Fatalf("executeSpecialMCPTool() error = %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("search_icons data = %T", result.Data)
	}
	items, ok := data["items"].([]iconcatalog.SearchResult)
	if !ok || len(items) == 0 || len(items) > 3 || items[0].ID != "rocket" {
		t.Fatalf("search_icons items = %#v", data["items"])
	}
}

func TestExecuteSpecialMCPListSpaces(t *testing.T) {
	service, principal, actor, spaces, _ := setupMCPDocsToolTest()
	result, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "list_spaces", json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("executeSpecialMCPTool() error = %v", err)
	}
	if result.Summary != "Returned 1 Docs spaces." || spaces.listCalls != 1 {
		t.Fatalf("executeSpecialMCPTool() = %#v, list calls %d", result, spaces.listCalls)
	}
}

func TestExecuteSpecialMCPListCollectionsFiltersByAccessibleSpace(t *testing.T) {
	service, principal, actor, _, collections := setupMCPDocsToolTest()
	result, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "list_collections",
		json.RawMessage(`{"space_id":"space-1"}`),
	)
	if err != nil {
		t.Fatalf("executeSpecialMCPTool() error = %v", err)
	}
	if result.Summary != "Returned 1 Docs collections." || len(collections.listSpaceIDs) != 1 ||
		collections.listSpaceIDs[0] != "space-1" {
		t.Fatalf("executeSpecialMCPTool() = %#v, list spaces %v", result, collections.listSpaceIDs)
	}
}

func TestExecuteSpecialMCPCreateSpace(t *testing.T) {
	service, principal, actor, spaces, _ := setupMCPDocsToolTest()
	result, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "create_space",
		json.RawMessage(`{"name":" API docs ","type":"internal","visibility":"workspace_wide"}`),
	)
	if err != nil {
		t.Fatalf("executeSpecialMCPTool() error = %v", err)
	}
	if result.Summary != "Docs space API docs created." || spaces.createRequest.Name != "API docs" ||
		spaces.createWorkspaceID != principal.WorkspaceID || spaces.createUserID != principal.UserID {
		t.Fatalf("executeSpecialMCPTool() = %#v, create request %#v", result, spaces.createRequest)
	}
}

func TestExecuteSpecialMCPCreateCollection(t *testing.T) {
	service, principal, actor, _, collections := setupMCPDocsToolTest()
	result, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "create_collection",
		json.RawMessage(`{"space_id":"space-1","name":" Guides "}`),
	)
	if err != nil {
		t.Fatalf("executeSpecialMCPTool() error = %v", err)
	}
	if result.Summary != "Docs collection Guides created." || collections.createRequest.Name != "Guides" ||
		collections.createWorkspaceID != principal.WorkspaceID || collections.createSpaceID != "space-1" ||
		collections.createUserID != principal.UserID {
		t.Fatalf("executeSpecialMCPTool() = %#v, create request %#v", result, collections.createRequest)
	}
}

func TestExecuteSpecialMCPCreateCollectionRejectsInaccessibleSpace(t *testing.T) {
	service, principal, actor, spaces, collections := setupMCPDocsToolTest()
	spaces.getResults["space-other"] = &model.DocsSpaceWithTeams{DocsSpace: model.DocsSpace{
		ID: "space-other", WorkspaceID: "workspace-other", Name: "Other",
	}}
	_, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "create_collection",
		json.RawMessage(`{"space_id":"space-other","name":"Guides"}`),
	)
	if !errors.Is(err, ErrMCPNotFound) {
		t.Fatalf("executeSpecialMCPTool() error = %v, want ErrMCPNotFound", err)
	}
	if collections.createCalls != 0 {
		t.Fatalf("collection create calls = %d, want 0", collections.createCalls)
	}
}

func TestExecuteSpecialMCPUpdateSpaceAndCollection(t *testing.T) {
	service, principal, actor, spaces, collections := setupMCPDocsToolTest()
	result, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "update_space",
		json.RawMessage(`{"space_id":"space-1","name":"Developer docs"}`),
	)
	if err != nil {
		t.Fatalf("update_space error = %v", err)
	}
	if result.Summary != "Docs space Developer docs updated." || spaces.updateID != "space-1" || spaces.updateRequest.Name == nil || *spaces.updateRequest.Name != "Developer docs" {
		t.Fatalf("update_space result = %#v, request %#v", result, spaces.updateRequest)
	}

	result, err = service.executeSpecialMCPTool(
		context.Background(), principal, actor, "update_collection",
		json.RawMessage(`{"collection_id":"collection-1","description":"Start here"}`),
	)
	if err != nil {
		t.Fatalf("update_collection error = %v", err)
	}
	if result.Summary != "Docs collection Guides updated." || collections.updateID != "collection-1" || collections.updateRequest.Description == nil || *collections.updateRequest.Description != "Start here" {
		t.Fatalf("update_collection result = %#v, request %#v", result, collections.updateRequest)
	}
}

func TestExecuteSpecialMCPMoveDocumentValidatesTargetCollection(t *testing.T) {
	service, principal, actor, _, _ := setupMCPDocsToolTest()
	documents := &fakeMCPDocsDocumentService{documents: map[string]*model.DocsDocument{
		"document-1": {ID: "document-1", WorkspaceID: principal.WorkspaceID, SpaceID: "space-1", Title: "API guide"},
	}}
	service.documents = documents

	result, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "move_document",
		json.RawMessage(`{"document_id":"document-1","space_id":"space-1","collection_id":"collection-1"}`),
	)
	if err != nil {
		t.Fatalf("move_document error = %v", err)
	}
	if result.Summary != "Document API guide moved." || documents.moveRequest.SpaceID != "space-1" || documents.moveRequest.CollectionID == nil || *documents.moveRequest.CollectionID != "collection-1" {
		t.Fatalf("move_document result = %#v, request %#v", result, documents.moveRequest)
	}
}

func TestExecuteSpecialMCPMoveDocumentDoesNotMovePublishedDocumentAcrossSpaces(t *testing.T) {
	service, principal, actor, spaces, _ := setupMCPDocsToolTest()
	spaces.getResults["space-2"] = &model.DocsSpaceWithTeams{DocsSpace: model.DocsSpace{ID: "space-2", WorkspaceID: principal.WorkspaceID, Name: "Other"}}
	documents := &fakeMCPDocsDocumentService{documents: map[string]*model.DocsDocument{
		"document-1": {ID: "document-1", WorkspaceID: principal.WorkspaceID, SpaceID: "space-1", Title: "Published guide", Status: model.DocStatusPublished},
	}}
	service.documents = documents

	_, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "move_document",
		json.RawMessage(`{"document_id":"document-1","space_id":"space-2"}`),
	)
	if err == nil {
		t.Fatal("expected published cross-space move to be rejected")
	}
	if documents.moveRequest.SpaceID != "" {
		t.Fatalf("unexpected move request %#v", documents.moveRequest)
	}
}

func TestExecuteSpecialMCPLinkDocumentRequiresAccessibleObject(t *testing.T) {
	service, principal, actor, _, _ := setupMCPDocsToolTest()
	service.documents = &fakeMCPDocsDocumentService{documents: map[string]*model.DocsDocument{
		"document-1": {ID: "document-1", WorkspaceID: principal.WorkspaceID, SpaceID: "space-1", Title: "API guide"},
	}}
	links := &fakeMCPDocsLinkService{}
	service.docsLinks = links
	service.docsRefs = fakeMCPDocsReferenceResolver{response: &model.ResolveDocsEntityRefsResponse{Refs: []model.DocsResolvedEntityRef{{
		EntityType: model.LinkedObjectTask, EntityID: "task-1", Status: docsEntityRefStatusAvailable, Access: docsEntityRefAccessGranted,
	}}}}

	result, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "link_document_to_object",
		json.RawMessage(`{"document_id":"document-1","linked_object_type":"task","linked_object_id":"task-1"}`),
	)
	if err != nil {
		t.Fatalf("link_document_to_object error = %v", err)
	}
	if result.Summary != "Document linked." || links.request.LinkedObjectID != "task-1" || links.request.LinkContext != model.LinkContextAttached {
		t.Fatalf("link result = %#v, request %#v", result, links.request)
	}
}

func setupMCPDocsToolTest() (
	*MCPService,
	*model.MCPPrincipal,
	*authorization.Actor,
	*fakeMCPDocsSpaceService,
	*fakeMCPDocsCollectionService,
) {
	space := model.DocsSpaceWithTeams{DocsSpace: model.DocsSpace{
		ID: "space-1", WorkspaceID: "workspace-1", Name: "API docs",
	}}
	spaces := &fakeMCPDocsSpaceService{
		listResults: []model.DocsSpaceWithTeams{space},
		getResults:  map[string]*model.DocsSpaceWithTeams{"space-1": &space},
	}
	collections := &fakeMCPDocsCollectionService{
		listResults: map[string][]model.DocsCollection{
			"space-1": {{ID: "collection-1", WorkspaceID: "workspace-1", SpaceID: "space-1", Name: "Guides"}},
		},
	}
	service := &MCPService{spaces: spaces, collections: collections}
	principal := &model.MCPPrincipal{
		WorkspaceID: "workspace-1", UserID: "user-1",
		Toolsets: []string{MCPToolsetDocs, MCPToolsetPM},
		Scopes:   []string{MCPScopeDocsRead, MCPScopeDocsWrite, MCPScopePMRead},
	}
	actor := &authorization.Actor{WorkspaceID: "workspace-1", UserID: "user-1", Role: "admin"}
	return service, principal, actor, spaces, collections
}

type fakeMCPDocsSpaceService struct {
	listResults       []model.DocsSpaceWithTeams
	getResults        map[string]*model.DocsSpaceWithTeams
	listCalls         int
	createWorkspaceID string
	createUserID      string
	createRequest     model.CreateDocsSpaceRequest
	updateID          string
	updateRequest     model.UpdateDocsSpaceRequest
}

func (f *fakeMCPDocsSpaceService) List(
	_ context.Context,
	_ string,
	_ *authorization.Actor,
) ([]model.DocsSpaceWithTeams, error) {
	f.listCalls++
	return f.listResults, nil
}

func (f *fakeMCPDocsSpaceService) Get(
	_ context.Context,
	spaceID string,
	_ *authorization.Actor,
) (*model.DocsSpaceWithTeams, error) {
	return f.getResults[spaceID], nil
}

func (f *fakeMCPDocsSpaceService) Create(
	_ context.Context,
	workspaceID string,
	request model.CreateDocsSpaceRequest,
	userID string,
) (*model.DocsSpaceWithTeams, error) {
	f.createWorkspaceID = workspaceID
	f.createUserID = userID
	f.createRequest = request
	return &model.DocsSpaceWithTeams{DocsSpace: model.DocsSpace{
		ID: "space-created", WorkspaceID: workspaceID, Name: request.Name,
	}}, nil
}

func (f *fakeMCPDocsSpaceService) Update(
	_ context.Context,
	spaceID string,
	request model.UpdateDocsSpaceRequest,
) (*model.DocsSpaceWithTeams, error) {
	f.updateID = spaceID
	f.updateRequest = request
	name := "API docs"
	if request.Name != nil {
		name = *request.Name
	}
	return &model.DocsSpaceWithTeams{DocsSpace: model.DocsSpace{ID: spaceID, WorkspaceID: "workspace-1", Name: name}}, nil
}

type fakeMCPDocsCollectionService struct {
	listResults       map[string][]model.DocsCollection
	listSpaceIDs      []string
	createCalls       int
	createWorkspaceID string
	createSpaceID     string
	createUserID      string
	createRequest     model.CreateDocsCollectionRequest
	updateID          string
	updateRequest     model.UpdateDocsCollectionRequest
}

type fakeMCPDocsDocumentService struct {
	documents   map[string]*model.DocsDocument
	moveRequest model.MoveDocsDocumentRequest
}

func (f *fakeMCPDocsDocumentService) Get(_ context.Context, documentID string) (*model.DocsDocument, error) {
	return f.documents[documentID], nil
}

func (f *fakeMCPDocsDocumentService) Move(_ context.Context, documentID string, request model.MoveDocsDocumentRequest) (*model.DocsDocument, error) {
	f.moveRequest = request
	document := *f.documents[documentID]
	document.SpaceID = request.SpaceID
	document.CollectionID = request.CollectionID
	return &document, nil
}

type fakeMCPDocsLinkService struct {
	request model.CreateDocsLinkRequest
}

func (f *fakeMCPDocsLinkService) Create(_ context.Context, workspaceID, documentID string, request model.CreateDocsLinkRequest, userID string) (*model.DocsLink, error) {
	f.request = request
	return &model.DocsLink{ID: "link-1", WorkspaceID: workspaceID, DocumentID: documentID, LinkedObjectType: request.LinkedObjectType, LinkedObjectID: request.LinkedObjectID, LinkContext: request.LinkContext, CreatedBy: userID}, nil
}

type fakeMCPDocsReferenceResolver struct {
	response *model.ResolveDocsEntityRefsResponse
}

func (f fakeMCPDocsReferenceResolver) Resolve(context.Context, string, model.ResolveDocsEntityRefsRequest) (*model.ResolveDocsEntityRefsResponse, error) {
	return f.response, nil
}

func (f *fakeMCPDocsCollectionService) Get(
	_ context.Context,
	collectionID string,
) (*model.DocsCollection, error) {
	for _, collections := range f.listResults {
		for index := range collections {
			if collections[index].ID == collectionID {
				collection := collections[index]
				return &collection, nil
			}
		}
	}
	return nil, nil
}

func (f *fakeMCPDocsCollectionService) List(
	_ context.Context,
	spaceID string,
) ([]model.DocsCollection, error) {
	f.listSpaceIDs = append(f.listSpaceIDs, spaceID)
	return f.listResults[spaceID], nil
}

func (f *fakeMCPDocsCollectionService) Create(
	_ context.Context,
	workspaceID string,
	spaceID string,
	request model.CreateDocsCollectionRequest,
	userID string,
) (*model.DocsCollection, error) {
	f.createCalls++
	f.createWorkspaceID = workspaceID
	f.createSpaceID = spaceID
	f.createUserID = userID
	f.createRequest = request
	return &model.DocsCollection{
		ID: "collection-created", WorkspaceID: workspaceID, SpaceID: spaceID, Name: request.Name,
	}, nil
}

func (f *fakeMCPDocsCollectionService) Update(
	_ context.Context,
	collectionID string,
	request model.UpdateDocsCollectionRequest,
) (*model.DocsCollection, error) {
	f.updateID = collectionID
	f.updateRequest = request
	name := "Guides"
	if request.Name != nil {
		name = *request.Name
	}
	return &model.DocsCollection{ID: collectionID, WorkspaceID: "workspace-1", SpaceID: "space-1", Name: name}, nil
}
