package agentcontract

import (
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestExtractPublishedPreviewsDropsUnknownPanelsWithoutTitle(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    string
	}{
		{name: "publish preview", toolName: ToolPublishPreview, input: `{"panel_key":"custom_panel","format":"markdown","content":"Preview body"}`},
		{name: "markdown slot preview", toolName: ToolPreviewMarkdown, input: `{"slot":"custom_panel","content":"Preview body"}`},
		{name: "json slot preview", toolName: ToolPreviewJSON, input: `{"slot":"custom_panel","content":{"status":"ready"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previews := ExtractPublishedPreviews([]model.ToolInvocation{{ToolName: tt.toolName, Input: json.RawMessage(tt.input)}})
			if len(previews) != 0 {
				t.Fatalf("expected invalid preview to be dropped, got %#v", previews)
			}
		})
	}
}

func TestExtractPublishedPreviewsUsesKnownPanelDefaultTitle(t *testing.T) {
	previews := ExtractPublishedPreviews([]model.ToolInvocation{{
		ToolName: ToolPublishPreview,
		Input:    json.RawMessage(`{"panel_key":"prd_draft","format":"markdown","content":"Preview body"}`),
	}})
	if len(previews) != 1 {
		t.Fatalf("expected one preview, got %#v", previews)
	}
	if previews[0].Title != "PRD Draft" {
		t.Fatalf("expected default title, got %#v", previews[0])
	}
}
