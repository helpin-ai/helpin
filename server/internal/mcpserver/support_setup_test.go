package mcpserver

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"strings"
	"testing"
	"time"
)

func TestSupportSetupPromptRequiresAvailableTool(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		t.Run(map[bool]string{false: "denied", true: "allowed"}[allowed], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
			var tools []service.MCPToolDefinition
			if allowed {
				tools = []service.MCPToolDefinition{{Name: "get_support_setup"}}
			}
			(&Handler{}).addWorkflowPrompts(server, tools)
			st, ct := mcp.NewInMemoryTransports()
			ss, err := server.Connect(ctx, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ss.Close()
			client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
			cs, err := client.Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cs.Close()
			prompts, err := cs.ListPrompts(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, prompt := range prompts.Prompts {
				if prompt.Name == "setup_customer_support" {
					found = true
				}
			}
			if found != allowed {
				t.Fatalf("prompt visible = %v, want %v", found, allowed)
			}
			if allowed {
				result, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "setup_customer_support"})
				if err != nil {
					t.Fatal(err)
				}
				text := result.Messages[0].Content.(*mcp.TextContent).Text
				if !strings.Contains(text, "get_support_setup") || !strings.Contains(text, "browser") {
					t.Fatalf("incomplete workflow: %s", text)
				}
			}
		})
	}
}
