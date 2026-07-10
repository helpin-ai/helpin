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
	if len(catalog) != 30 {
		t.Fatalf("buildToolCatalog() returned %d tools, want 30", len(catalog))
	}
	deferred := map[string]bool{
		"write_document_content": true,
		"draft_support_reply":    true,
		"send_support_reply":     true,
		"publish_prd_draft":      true,
		"delete_task":            true,
	}
	for _, tool := range catalog {
		if deferred[tool.Name] {
			t.Fatalf("buildToolCatalog() exposed deferred action %q", tool.Name)
		}
	}
}
