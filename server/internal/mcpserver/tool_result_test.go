package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestToolCallResultRepeatsDataAsText(t *testing.T) {
	result := &service.MCPToolResult{
		Summary: "List CRM companies completed.",
		Data:    map[string]any{"companies": []any{map[string]any{"name": "100Hires", "custom_properties": map[string]any{"fit": "yes"}}}},
		Links:   map[string]string{"app": "https://app.example/crm"},
	}
	call := toolCallResult(result)
	text := call.Content[0].(*mcp.TextContent).Text
	summary, encoded, found := strings.Cut(text, "\n\n")
	if !found || summary != result.Summary {
		t.Fatalf("text = %q, want summary then JSON", text)
	}
	var payload struct {
		Data struct {
			Companies []struct {
				CustomProperties map[string]any `json:"custom_properties"`
			} `json:"companies"`
		} `json:"data"`
		Links map[string]string `json:"links"`
	}
	if err := json.Unmarshal([]byte(encoded), &payload); err != nil {
		t.Fatalf("decode text JSON: %v", err)
	}
	if payload.Data.Companies[0].CustomProperties["fit"] != "yes" || payload.Links["app"] == "" {
		t.Fatalf("payload = %+v", payload)
	}
	if call.StructuredContent != result {
		t.Fatal("structured content must still carry the result")
	}
}

func TestToolCallResultWithoutDataIsSummaryOnly(t *testing.T) {
	call := toolCallResult(&service.MCPToolResult{Summary: "Done."})
	if text := call.Content[0].(*mcp.TextContent).Text; text != "Done." {
		t.Fatalf("text = %q", text)
	}
}
