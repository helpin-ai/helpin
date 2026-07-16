package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

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
	principal := &model.MCPPrincipal{WorkspaceID: "workspace-1", UserID: "user-1"}
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

type fakeMCPDocsCollectionService struct {
	listResults       map[string][]model.DocsCollection
	listSpaceIDs      []string
	createCalls       int
	createWorkspaceID string
	createSpaceID     string
	createUserID      string
	createRequest     model.CreateDocsCollectionRequest
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
