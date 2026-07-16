package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestValidateMCPRedirectURI(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "HTTPS client", value: "https://client.example/oauth/callback"},
		{name: "loopback HTTP", value: "http://127.0.0.1:48765/callback"},
		{name: "reject remote HTTP", value: "http://client.example/callback", wantErr: true},
		{name: "reject fragment", value: "https://client.example/callback#token", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateMCPRedirectURI(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateMCPRedirectURI() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestValidateMCPToolArgumentsRejectsAdditionalProperties(t *testing.T) {
	resolved, err := resolveMCPToolSchema(map[string]any{
		"type":                 "object",
		"properties":           map[string]any{"query": map[string]any{"type": "string"}},
		"required":             []string{"query"},
		"additionalProperties": false,
	})
	if err != nil {
		t.Fatalf("resolveMCPToolSchema() error = %v", err)
	}
	service := &MCPService{toolSchemas: map[string]*jsonschema.Resolved{"search": resolved}}
	err = service.validateMCPToolArguments("search", json.RawMessage(`{"query":"roadmap","workspace_id":"other"}`))
	if !errors.Is(err, ErrMCPInvalidArguments) {
		t.Fatalf("validateMCPToolArguments() error = %v, want ErrMCPInvalidArguments", err)
	}
}

func TestVerifyMCPCodeChallenge(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])
	if !verifyMCPCodeChallenge(verifier, challenge) {
		t.Fatal("verifyMCPCodeChallenge() rejected a valid S256 verifier")
	}
	if verifyMCPCodeChallenge(verifier+"x", challenge) {
		t.Fatal("verifyMCPCodeChallenge() accepted a different verifier")
	}
}

func TestConstrainMCPGrantAppliesPolicyAndReadOnly(t *testing.T) {
	service := &MCPService{}
	policy := &model.MCPWorkspacePolicy{
		AllowedToolsets: mustMCPJSON([]string{MCPToolsetContext, MCPToolsetPM}),
		AllowedScopes:   mustMCPJSON([]string{MCPScopeContextRead, MCPScopePMRead, MCPScopePMWrite}),
		EnforceReadOnly: true,
	}
	toolsets, scopes, readOnly, err := service.constrainGrant(
		policy,
		[]string{MCPToolsetContext, MCPToolsetPM, MCPToolsetDocs},
		[]string{MCPScopeContextRead, MCPScopePMRead, MCPScopePMWrite},
		false,
	)
	if err != nil {
		t.Fatalf("constrainGrant() error = %v", err)
	}
	if !readOnly || containsMCPValue(scopes, MCPScopePMWrite) || containsMCPValue(toolsets, MCPToolsetDocs) {
		t.Fatalf("constrainGrant() = toolsets %v scopes %v readOnly %v", toolsets, scopes, readOnly)
	}
}

func TestConstrainMCPGrantPreservesApprovedWriteScopes(t *testing.T) {
	service := &MCPService{}
	policy := &model.MCPWorkspacePolicy{
		AllowedToolsets: mustMCPJSON([]string{MCPToolsetContext, MCPToolsetDocs, MCPToolsetAgents}),
		AllowedScopes: mustMCPJSON([]string{
			MCPScopeContextRead, MCPScopeDocsRead, MCPScopeDocsWrite,
			MCPScopeAgentsRead, MCPScopeAgentsRun,
		}),
		EnforceReadOnly: false,
	}
	toolsets, scopes, readOnly, err := service.constrainGrant(
		policy,
		[]string{MCPToolsetContext, MCPToolsetDocs, MCPToolsetAgents},
		[]string{
			MCPScopeContextRead, MCPScopeDocsRead, MCPScopeDocsWrite,
			MCPScopeAgentsRead, MCPScopeAgentsRun,
		},
		false,
	)
	if err != nil {
		t.Fatalf("constrainGrant() error = %v", err)
	}
	if readOnly || !containsMCPValue(scopes, MCPScopeDocsWrite) ||
		!containsMCPValue(scopes, MCPScopeAgentsRun) ||
		!containsMCPValue(toolsets, MCPToolsetDocs) {
		t.Fatalf("constrainGrant() = toolsets %v scopes %v readOnly %v", toolsets, scopes, readOnly)
	}
}

func TestPlatformToolFlagsFilterRiskyCapabilities(t *testing.T) {
	service := &MCPService{config: MCPServiceConfig{
		PMWriteEnabled: false, DocsWriteEnabled: true, AgentRunEnabled: false,
		CRMEnabled: false, SupportEnabled: true,
	}}
	tests := []struct {
		name string
		tool MCPToolDefinition
		want bool
	}{
		{name: "PM reads remain available", tool: MCPToolDefinition{Toolset: MCPToolsetPM}, want: true},
		{name: "PM writes can be disabled", tool: MCPToolDefinition{Toolset: MCPToolsetPM, Mutating: true}},
		{name: "Docs writes can remain enabled", tool: MCPToolDefinition{Toolset: MCPToolsetDocs, Mutating: true}, want: true},
		{name: "Agent starts can be disabled", tool: MCPToolDefinition{Toolset: MCPToolsetAgents, Name: "start_agent_run"}},
		{name: "Agent reads remain available", tool: MCPToolDefinition{Toolset: MCPToolsetAgents, Name: "get_agent_run"}, want: true},
		{name: "CRM can be disabled", tool: MCPToolDefinition{Toolset: MCPToolsetCRM}},
		{name: "Support can remain enabled", tool: MCPToolDefinition{Toolset: MCPToolsetSupport}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := service.platformToolEnabled(test.tool); got != test.want {
				t.Fatalf("platformToolEnabled() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPublicMCPToolCatalogIsBoundedAndExcludesDeferredActions(t *testing.T) {
	commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	service := &MCPService{commands: commands}
	catalog := service.buildToolCatalog()
	if len(catalog) != 46 {
		t.Fatalf("buildToolCatalog() returned %d tools, want 46", len(catalog))
	}
	expectedTools := map[string]struct {
		toolset  string
		mutating bool
	}{
		"list_spaces":                {MCPToolsetDocs, false},
		"list_collections":           {MCPToolsetDocs, false},
		"search_icons":               {MCPToolsetDocs, false},
		"create_space":               {MCPToolsetDocs, true},
		"create_collection":          {MCPToolsetDocs, true},
		"update_space":               {MCPToolsetDocs, true},
		"update_collection":          {MCPToolsetDocs, true},
		"move_document":              {MCPToolsetDocs, true},
		"link_document_to_object":    {MCPToolsetDocs, true},
		"update_task":                {MCPToolsetPM, true},
		"create_task_batch":          {MCPToolsetPM, true},
		"set_task_dependencies":      {MCPToolsetPM, true},
		"list_task_checklist":        {MCPToolsetPM, false},
		"create_task_checklist_item": {MCPToolsetPM, true},
		"update_task_checklist_item": {MCPToolsetPM, true},
		"get_task_context":           {MCPToolsetPM, false},
	}
	deferred := map[string]bool{
		"write_document_content":     true,
		"draft_support_reply":        true,
		"send_support_reply":         true,
		"publish_prd_draft":          true,
		"delete_task":                true,
		"delete_task_checklist_item": true,
	}
	for _, tool := range catalog {
		if expected, ok := expectedTools[tool.Name]; ok {
			if tool.Toolset != expected.toolset || tool.Mutating != expected.mutating {
				t.Fatalf("tool %q = toolset %q mutating %v", tool.Name, tool.Toolset, tool.Mutating)
			}
			delete(expectedTools, tool.Name)
		}
		if deferred[tool.Name] {
			t.Fatalf("buildToolCatalog() exposed deferred action %q", tool.Name)
		}
	}
	if len(expectedTools) != 0 {
		t.Fatalf("buildToolCatalog() missing selected tools: %v", expectedTools)
	}
}

func TestPublicMCPToolCatalogSerializesValidRequiredArrays(t *testing.T) {
	commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	service := &MCPService{commands: commands}

	for _, tool := range service.buildToolCatalog() {
		encoded, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal input schema for %q: %v", tool.Name, err)
		}
		var schema map[string]any
		if err := json.Unmarshal(encoded, &schema); err != nil {
			t.Fatalf("unmarshal input schema for %q: %v", tool.Name, err)
		}
		required, present := schema["required"]
		if !present {
			continue
		}
		if _, ok := required.([]any); !ok {
			t.Fatalf("tool %q serialized required as %T, want array", tool.Name, required)
		}
	}
}
