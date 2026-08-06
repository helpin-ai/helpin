package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type stubWorkspaceSearchProvider struct {
	response *model.SearchResponse
}

func (s stubWorkspaceSearchProvider) SearchLimit(context.Context, string, string, int) (*model.SearchResponse, error) {
	return s.response, nil
}

func TestSearchWorkspaceRanksExactTaskKeyAndPaginates(t *testing.T) {
	env := newPMCommandTestEnv(t)
	env.service.workspaceSearch = stubWorkspaceSearchProvider{response: &model.SearchResponse{
		Tasks: []model.SearchResult{{ID: "task-488", Name: "Attribution follow-up", TaskKey: "USE-488"}},
	}}

	out, err := env.service.Execute(context.Background(), env.meta("workspace", env.workspaceID), "workspace.search", json.RawMessage(`{"query":"USE-488","entity_types":["task"],"limit":1,"offset":0}`))
	if err != nil {
		t.Fatalf("search workspace: %v", err)
	}
	var result struct {
		Results []struct {
			EntityType string   `json:"entity_type"`
			ID         string   `json:"id"`
			Key        string   `json:"key"`
			MatchedOn  []string `json:"matched_on"`
		} `json:"results"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("decode search result: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].EntityType != "task" || result.Results[0].ID != "task-488" || result.Results[0].Key != "USE-488" || result.Limit != 1 || result.Offset != 0 {
		t.Fatalf("unexpected search result: %#v", result)
	}
	foundExactKey := false
	for _, matchedOn := range result.Results[0].MatchedOn {
		foundExactKey = foundExactKey || matchedOn == "exact_key"
	}
	if !foundExactKey {
		t.Fatalf("exact task key was not ranked explicitly: %#v", result.Results[0].MatchedOn)
	}
}

func TestSearchWorkspaceRejectsInaccessibleRequestedType(t *testing.T) {
	env := newPMCommandTestEnv(t)
	env.service.SetAuthorizationService(authorization.NewAuthzService(nil, nil, nil))
	meta := env.meta("workspace", env.workspaceID)
	meta.ActorRole = "role-without-crm-access"

	allowed := env.service.allowedWorkspaceSearchTypes(meta)
	if allowed["crm_company"] || allowed["support_conversation"] {
		t.Fatalf("restricted types unexpectedly searchable: %#v", allowed)
	}
}
