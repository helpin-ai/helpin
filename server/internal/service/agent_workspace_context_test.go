package service

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildAgentRunInputPayloadIncludesWorkspaceContext(t *testing.T) {
	payload, err := buildAgentRunInputPayload(
		"workspace",
		"ws-1",
		nil,
		nil,
		nil,
		nil,
		nil,
		&model.AgentRunWorkspaceContext{
			Name:                  "Acme",
			WebsiteURL:            "https://acme.com",
			CompanyProductContext: "Acme helps support teams answer customers from product and repo context.",
		},
	)
	if err != nil {
		t.Fatalf("buildAgentRunInputPayload() error = %v", err)
	}

	var input model.AgentRunInputPayload
	if err := json.Unmarshal(payload, &input); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if input.WorkspaceContext == nil {
		t.Fatalf("expected workspace_context in payload: %s", string(payload))
	}
	if input.WorkspaceContext.Name != "Acme" {
		t.Fatalf("workspace context name = %q", input.WorkspaceContext.Name)
	}
	if input.WorkspaceContext.CompanyProductContext == "" {
		t.Fatalf("expected company product context in payload: %#v", input.WorkspaceContext)
	}
}
