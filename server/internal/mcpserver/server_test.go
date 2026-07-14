package mcpserver

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestProtocolToolAnnotationsAreHintsFromDefinition(t *testing.T) {
	tool := protocolTool(service.MCPToolDefinition{
		Name: "create_task", Title: "Create task", Description: "Create a task.",
		InputSchema: map[string]any{"type": "object"}, Mutating: true, IdempotentHint: true,
	})
	if tool.Annotations == nil || tool.Annotations.ReadOnlyHint || !tool.Annotations.IdempotentHint {
		t.Fatalf("protocolTool() annotations = %#v", tool.Annotations)
	}
	if tool.OutputSchema == nil {
		t.Fatal("protocolTool() omitted the structured output schema")
	}
}

func TestRequestLimiterSeparatesPrincipalAndWorkspaceLimits(t *testing.T) {
	limiter := newRequestLimiter()
	principal := &model.MCPPrincipal{ConnectionID: "connection-1", WorkspaceID: "workspace-1"}
	for i := 0; i < 60; i++ {
		if !limiter.Allow(principal) {
			t.Fatalf("Allow() denied request %d before the connection limit", i+1)
		}
	}
	if limiter.Allow(principal) {
		t.Fatal("Allow() accepted request above the connection limit")
	}
}

func TestRequestLimiterAppliesToolClassLimits(t *testing.T) {
	limiter := newRequestLimiter()
	principal := &model.MCPPrincipal{ConnectionID: "connection-1", WorkspaceID: "workspace-1", UserID: "user-1"}
	search := service.MCPToolDefinition{Name: "search_workspace"}
	for i := 0; i < 20; i++ {
		if !limiter.AllowTool(principal, search) {
			t.Fatalf("AllowTool() denied search %d before the class limit", i+1)
		}
	}
	if limiter.AllowTool(principal, search) {
		t.Fatal("AllowTool() accepted a search above the class limit")
	}
}
