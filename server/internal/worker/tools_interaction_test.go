package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

func TestRequestHumanInputToolReturnsAwaitingInputPayload(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolRequestHumanInput: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolRequestHumanInput, json.RawMessage(`{
		"questions": [
			{
				"id": "q1",
				"text": "Who owns this deal?",
				"options": [
					{ "value": "sales", "label": "Sales" },
					{ "value": "other", "label": "Other", "freetext": true }
				]
			}
		]
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"status": "awaiting_input"`) || !strings.Contains(output, `"id": "q1"`) {
		t.Fatalf("expected awaiting_input payload, got %s", output)
	}
}

func TestExtractLatestHumanApprovalRequestPrefersToolInvocation(t *testing.T) {
	approval := ExtractLatestHumanApprovalRequest([]appmodel.ToolInvocation{
		{
			ToolName: ToolRequestHumanApproval,
			Input: json.RawMessage(`{
				"phase": "crm_review",
				"title": "Approve the stage change",
				"summary": "Move ACME to verbal commit."
			}`),
		},
	})
	if approval == nil {
		t.Fatal("expected approval request")
	}
	if approval.Phase != "crm_review" || approval.Title != "Approve the stage change" || approval.Summary != "Move ACME to verbal commit." {
		t.Fatalf("unexpected approval request: %#v", approval)
	}
}

func TestPublishPreviewToolReturnsPublishedPayload(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishPreview: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishPreview, json.RawMessage(`{
		"panel_key": "prd_draft",
		"title": "PRD Draft",
		"format": "markdown",
		"content": "# Problem\nBody"
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"status": "published"`) || !strings.Contains(output, `"panel_key": "prd_draft"`) {
		t.Fatalf("expected published payload, got %s", output)
	}
}

func TestExtractLatestPublishedPreviewPrefersToolInvocation(t *testing.T) {
	preview := ExtractLatestPublishedPreview([]appmodel.ToolInvocation{
		{
			ToolName: ToolPublishPreview,
			Input: json.RawMessage(`{
				"panel_key": "story_plan",
				"title": "Story Plan",
				"format": "json",
				"content": {
					"summary": "Slice plan",
					"proposed_stories": []
				}
			}`),
		},
	}, "story_plan")
	if preview == nil {
		t.Fatal("expected published preview")
	}
	if preview.PanelKey != "story_plan" || preview.Title != "Story Plan" || preview.Format != PreviewFormatJSON {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	if !json.Valid(preview.Content) {
		t.Fatalf("expected preview content to be valid json, got %s", string(preview.Content))
	}
}
