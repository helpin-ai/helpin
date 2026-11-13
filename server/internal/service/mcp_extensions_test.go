package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func newMCPExtensionTestService() *MCPService {
	return &MCPService{
		tools:       map[string]MCPToolDefinition{"get_task": {Name: "get_task"}},
		toolSchemas: map[string]*jsonschema.Resolved{},
	}
}

func mcpTestExtension(name string) MCPToolExtension {
	return MCPToolExtension{
		Definition: MCPToolDefinition{
			Name: name, Description: "Test tool.", Toolset: MCPToolsetCRM, Scope: MCPScopeCRMWrite,
			Permission: authorization.PermCRMEdit, Mutating: true,
			InputSchema: map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{"domain": map[string]any{"type": "string", "minLength": 3}},
				"required":   []string{"domain"},
			},
		},
		Execute: func(_ context.Context, call MCPToolCall, arguments json.RawMessage) (*MCPToolResult, error) {
			return &MCPToolResult{Summary: "ran for " + call.WorkspaceID + " as " + call.UserID + "/" + call.ServicePrincipalID, Data: json.RawMessage(arguments)}, nil
		},
	}
}

func TestRegisterToolExtensionsRejectsUnsafeDefinitions(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*MCPToolExtension)
		wantErr string
	}{
		{name: "no name", change: func(e *MCPToolExtension) { e.Definition.Name = " " }, wantErr: "no name"},
		{name: "no handler", change: func(e *MCPToolExtension) { e.Execute = nil }, wantErr: "no handler"},
		{name: "built-in name", change: func(e *MCPToolExtension) { e.Definition.Name = "get_task" }, wantErr: "already registered"},
		{name: "unknown scope", change: func(e *MCPToolExtension) { e.Definition.Scope = "helpin.everything" }, wantErr: "unknown scope"},
		{name: "unknown toolset", change: func(e *MCPToolExtension) { e.Definition.Toolset = "admin" }, wantErr: "unknown toolset"},
		{name: "internal command", change: func(e *MCPToolExtension) { e.Definition.CommandName = "workspace.search" }, wantErr: "internal command"},
		{name: "broken schema", change: func(e *MCPToolExtension) { e.Definition.InputSchema = map[string]any{"type": 7} }, wantErr: "invalid input schema"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newMCPExtensionTestService()
			extension := mcpTestExtension("create_thing")
			tt.change(&extension)
			err := service.RegisterToolExtensions(extension)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("RegisterToolExtensions error = %v, want one containing %q", err, tt.wantErr)
			}
			if len(service.tools) != 1 || len(service.extensions) != 0 {
				t.Fatalf("a rejected extension was added: %d tools, %d extensions", len(service.tools), len(service.extensions))
			}
		})
	}
}

func TestRegisteredExtensionIsValidatedScopedAndExecuted(t *testing.T) {
	service := newMCPExtensionTestService()
	extension := mcpTestExtension("create_thing")
	extension.Available = func(workspaceID string) bool { return workspaceID == "home" }
	if err := service.RegisterToolExtensions(extension); err != nil {
		t.Fatalf("RegisterToolExtensions: %v", err)
	}
	tool := service.tools["create_thing"]
	if tool.Title != "Create thing" || !tool.IdempotentHint {
		t.Fatalf("definition = %+v", tool)
	}
	// A write gets the idempotency key every built-in write requires.
	if err := service.validateMCPToolArguments("create_thing", json.RawMessage(`{"domain":"acme.com"}`)); !errors.Is(err, ErrMCPInvalidArguments) {
		t.Fatalf("arguments without an idempotency key error = %v", err)
	}
	if err := service.validateMCPToolArguments("create_thing", json.RawMessage(`{"domain":"acme.com","idempotency_key":"demo-acme-1"}`)); err != nil {
		t.Fatalf("valid arguments rejected: %v", err)
	}
	if err := service.validateMCPToolArguments("create_thing", json.RawMessage(`{"domain":"acme.com","idempotency_key":"demo-acme-1","extra":true}`)); !errors.Is(err, ErrMCPInvalidArguments) {
		t.Fatalf("unknown argument error = %v", err)
	}

	if !service.extensionToolAvailable("create_thing", "home") || service.extensionToolAvailable("create_thing", "elsewhere") {
		t.Fatal("availability does not follow the extension's rule")
	}
	if !service.extensionToolAvailable("get_task", "elsewhere") {
		t.Fatal("a built-in tool was hidden by the extension check")
	}
	// canUseTool refuses the tool outside its workspaces before any role check.
	outside := &model.MCPPrincipal{WorkspaceID: "elsewhere", Scopes: []string{MCPScopeCRMWrite}, Toolsets: []string{MCPToolsetCRM}}
	service.config.CRMEnabled = true
	if service.canUseTool(context.Background(), outside, &authorization.Actor{Role: "owner"}, tool) {
		t.Fatal("the tool is usable in a workspace it is not offered to")
	}

	principal := &model.MCPPrincipal{WorkspaceID: "home", UserID: "user-1", ServicePrincipalID: "sp-1"}
	result, err := service.executeAuthorizedMCPTool(context.Background(), principal, &authorization.Actor{}, tool, json.RawMessage(`{"domain":"acme.com"}`))
	if err != nil || result.Summary != "ran for home as user-1/sp-1" {
		t.Fatalf("execute = %+v, %v", result, err)
	}
}

func TestExtensionErrorsReachTheCaller(t *testing.T) {
	service := newMCPExtensionTestService()
	extension := mcpTestExtension("create_thing")
	extension.Execute = func(context.Context, MCPToolCall, json.RawMessage) (*MCPToolResult, error) {
		return nil, &MCPToolError{Code: "invalid_request", Message: "No company with that domain exists."}
	}
	if err := service.RegisterToolExtensions(extension); err != nil {
		t.Fatal(err)
	}
	_, err := service.executeAuthorizedMCPTool(context.Background(), &model.MCPPrincipal{WorkspaceID: "home"}, &authorization.Actor{}, service.tools["create_thing"], json.RawMessage(`{}`))
	var toolErr *MCPToolError
	if !errors.As(err, &toolErr) || toolErr.Message != "No company with that domain exists." {
		t.Fatalf("error = %v", err)
	}
}
