package mcpserver

import (
	"context"
	"fmt"
	"github.com/alicebob/miniredis/v2"
	"github.com/helpin-ai/helpin/server/internal/ratelimit"
	"github.com/redis/go-redis/v9"
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

func TestRequestLimiterSharesActorBudgetAcrossTokens(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	limiter := &requestLimiter{shared: ratelimit.New(client, ratelimit.Config{RequestsPerMinute: 2, ExpensivePerMinute: 1})}
	ctx := context.Background()
	principal := &model.MCPPrincipal{ConnectionID: "first-token", WorkspaceID: "ws1", UserID: "alice"}
	if !limiter.Allow(ctx, principal) {
		t.Fatal("first request blocked")
	}
	principal.ConnectionID = "second-token"
	if !limiter.Allow(ctx, principal) || limiter.Allow(ctx, principal) {
		t.Fatal("token rotation bypassed budget")
	}
	search := service.MCPToolDefinition{Name: "search_workspace"}
	if !limiter.AllowTool(ctx, principal, search) || limiter.AllowTool(ctx, principal, search) {
		t.Fatal("expensive ceiling not applied")
	}
	if !limiter.AllowTool(ctx, principal, service.MCPToolDefinition{Name: "get_task"}) {
		t.Fatal("read used expensive ceiling")
	}
	principal.WorkspaceID = "ws2"
	if !limiter.Allow(ctx, principal) || !limiter.AllowTool(ctx, principal, search) {
		t.Fatal("another workspace blocked")
	}
	principal.WorkspaceID = "ws1"
	principal.UserID = ""
	principal.ServicePrincipalID = "service1"
	if !limiter.Allow(ctx, principal) || !limiter.AllowTool(ctx, principal, search) {
		t.Fatal("service principal blocked")
	}
	principal.ServicePrincipalID = "service2"
	if !limiter.AllowTool(ctx, principal, search) {
		t.Fatal("service principals share budget")
	}
}

func TestPublicToolErrorSurfacesTypedCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "typed tool error keeps its code",
			err:  fmt.Errorf("wrapped: %w", &service.MCPToolError{Code: service.MCPErrorCodeDocumentPublished, Message: "Unpublish first."}),
			want: "DOC_IS_PUBLISHED: Unpublish first.",
		},
		{
			name: "sentinel errors keep their public message",
			err:  service.ErrMCPNotFound,
			want: "The requested Helpin record was not found in the connected workspace.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := publicToolError(tt.err); got != tt.want {
				t.Fatalf("publicToolError() = %q, want %q", got, tt.want)
			}
		})
	}
}
