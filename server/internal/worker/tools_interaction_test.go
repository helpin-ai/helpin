package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

func TestRequestUserInputToolReturnsAwaitingInputPayload(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolRequestUserInput: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolRequestUserInput, json.RawMessage(`{
		"questions": [
			{
				"id": "q1",
				"header": "Owner",
				"question": "Who owns this deal?",
				"isOther": true,
				"options": [
					{ "label": "Sales" },
					{ "label": "Support" }
				]
			}
		]
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"status":"paused"`) || !strings.Contains(output, `"pause_reason":"human_input"`) || !strings.Contains(output, `"id":"q1"`) {
		t.Fatalf("expected paused human_input payload, got %s", output)
	}
}

func TestLegacyRequestHumanInputAliasExecutesWhenCanonicalToolIsAllowed(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolRequestUserInput: true,
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
	if !strings.Contains(output, `"pause_reason":"human_input"`) {
		t.Fatalf("expected alias to pause for human input, got %s", output)
	}
}

func TestExtractLatestHumanApprovalRequestPrefersToolInvocation(t *testing.T) {
	approval := ExtractLatestHumanApprovalRequest([]appmodel.ToolInvocation{
		{
			ToolName: ToolRequestReviewCheckpoint,
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

func TestRequestReviewCheckpointToolReturnsAwaitingApprovalPayload(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolRequestReviewCheckpoint: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolRequestReviewCheckpoint, json.RawMessage(`{
		"phase": "prd",
		"title": "Approve PRD",
		"summary": "Review the latest draft."
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"status":"paused"`) || !strings.Contains(output, `"pause_reason":"human_approval"`) || !strings.Contains(output, `"phase":"prd"`) {
		t.Fatalf("expected paused human_approval payload, got %s", output)
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
	if !strings.Contains(output, `"status":"published"`) || !strings.Contains(output, `"panel_key":"prd_draft"`) {
		t.Fatalf("expected published payload, got %s", output)
	}
}

func TestPreviewMarkdownToolPublishesSlot(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPreviewMarkdown: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPreviewMarkdown, json.RawMessage(`{
		"slot": "story_plan_doc",
		"title": "Story Planning Document",
		"content": "# Outcome\nBody"
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan_doc"`, `"format":"markdown"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected markdown preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPreviewJSONToolPublishesSlot(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPreviewJSON: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPreviewJSON, json.RawMessage(`{
		"slot": "story_plan",
		"title": "Story Plan",
		"content": {"summary":"Slice plan","proposed_stories":[]}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected json preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishPRDDraftToolPublishesCanonicalPreview(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishPRDDraft: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishPRDDraft, json.RawMessage(`{
		"content": "# Problem\nBody"
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"prd_draft"`, `"title":"PRD Draft"`, `"format":"markdown"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected prd draft preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanToolPublishesCanonicalPreview(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishStoryPlan: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishStoryPlan, json.RawMessage(`{
		"content": {"summary":"Slice plan","proposed_stories":[]}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan"`, `"title":"Story Plan"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected story plan preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanToolAcceptsNestedPreviewPayload(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishStoryPlan: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishStoryPlan, json.RawMessage(`{
		"preview": {
			"title": "Story Plan",
			"content": {"summary":"Slice plan","proposed_stories":[]}
		}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected nested story plan preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanToolAcceptsRawPlanObject(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishStoryPlan: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishStoryPlan, json.RawMessage(`{
		"title": "Story Plan",
		"summary":"Slice plan",
		"proposed_stories":[]
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected raw story plan object preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanToolRejectsJSONStringContent(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishStoryPlan: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPublishStoryPlan, json.RawMessage("{\n"+
		"  \"title\": \"Story Plan\",\n"+
		"  \"content\": \"Here is the plan in the required format:\\n```json\\n{\\\"summary\\\":\\\"Breakdown\\\",\\\"proposed_stories\\\":[{\\\"ref\\\":\\\"story_1\\\",\\\"name\\\":\\\"Story A\\\",\\\"description\\\":\\\"Do A\\\",\\\"story_type\\\":\\\"feature\\\",\\\"acceptance_criteria\\\":[\\\"works\\\"]}]}\\n```\"\n"+
		"}"))
	if err == nil {
		t.Fatal("expected publish_story_plan to reject stringified JSON content")
	}
	if !strings.Contains(err.Error(), "publish_story_plan content must be a JSON object with summary and proposed_stories") {
		t.Fatalf("expected structured story plan error, got %v", err)
	}
}

func TestPublishStoryPlanToolRejectsNonObjectJSONStringContent(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishStoryPlan: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPublishStoryPlan, json.RawMessage(`{
		"title": "Story Plan",
		"content": "STORY PLAN: do the work"
	}`))
	if err == nil {
		t.Fatal("expected publish_story_plan to reject non-object string content")
	}
	if !strings.Contains(err.Error(), "publish_story_plan content must be a JSON object with summary and proposed_stories") {
		t.Fatalf("expected repair-oriented story plan error, got %v", err)
	}
}

func TestPublishStoryPlanToolReusesLastPublishedContentOnMalformedRetry(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishStoryPlan: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPublishStoryPlan, json.RawMessage(`{
		"content": {"summary":"Slice plan","proposed_stories":[]}
	}`))
	if err != nil {
		t.Fatalf("initial publish returned error: %v", err)
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishStoryPlan, json.RawMessage(`{
		"title": "Story Plan"
	}`))
	if err != nil {
		t.Fatalf("malformed retry should have reused cached content, got error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected cached story plan preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanDocToolReusesPreviewMarkdownContentOnMalformedRetry(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishStoryPlanDoc: true,
			ToolPreviewMarkdown:     true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPreviewMarkdown, json.RawMessage(`{
		"slot": "story_plan_doc",
		"title": "Story Planning Document",
		"content": "# Outcome\nImplementation-ready plan"
	}`))
	if err != nil {
		t.Fatalf("preview_md returned error: %v", err)
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishStoryPlanDoc, json.RawMessage(`{
		"title": "Story Planning Document"
	}`))
	if err != nil {
		t.Fatalf("malformed publish_story_plan_doc retry should have reused cached markdown, got error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan_doc"`, `"format":"markdown"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected cached story planning doc preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanDocToolUsesCurrentAssistantDraftOnFirstMalformedCall(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context:              context.Background(),
		CurrentAssistantText: "# Outcome\nImplementation-ready plan",
		AllowedTools: map[string]bool{
			ToolPublishStoryPlanDoc: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishStoryPlanDoc, json.RawMessage(`{
		"title": "Story Planning Document"
	}`))
	if err != nil {
		t.Fatalf("first malformed publish_story_plan_doc call should have used assistant draft, got error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan_doc"`, `"format":"markdown"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected assistant-draft story planning doc preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanDocToolPublishesCanonicalPreview(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishStoryPlanDoc: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishStoryPlanDoc, json.RawMessage(`{
		"content": "# Outcome\nImplementation-ready plan"
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan_doc"`, `"title":"Story Planning Document"`, `"format":"markdown"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected story planning doc preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanDocToolReturnsRepairOrientedMissingContentError(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishStoryPlanDoc: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPublishStoryPlanDoc, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected missing content error")
	}
	if !strings.Contains(err.Error(), `publish_story_plan_doc is missing content; include markdown in "content"`) {
		t.Fatalf("expected repair-oriented missing content error, got %v", err)
	}
}

func TestPublishPreviewToolReturnsRepairOrientedMissingPanelError(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishPreview: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPublishPreview, json.RawMessage(`{
		"title": "Missing panel key",
		"format": "markdown",
		"content": "# Draft"
	}`))
	if err == nil {
		t.Fatal("expected missing panel_key error")
	}
	if !strings.Contains(err.Error(), `publish_preview is missing panel_key; include "panel_key" or use preview_md/preview_json with "slot"`) {
		t.Fatalf("expected repair-oriented panel_key error, got %v", err)
	}
}

func TestPublishPreviewToolAcceptsPanelKeyAlias(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishPreview: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishPreview, json.RawMessage(`{
		"panelKey": "story_plan",
		"title": "Story Plan",
		"format": "json",
		"content": {
			"summary": "Slice plan",
			"proposed_stories": []
		}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"panel_key":"story_plan"`) {
		t.Fatalf("expected alias payload to normalize panel_key, got %s", output)
	}
}

func TestPublishPreviewToolAcceptsNestedPreviewPayload(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishPreview: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishPreview, json.RawMessage(`{
		"preview": {
			"panel_key": "prd_draft",
			"title": "PRD Draft",
			"format": "markdown",
			"content": "# Problem\nBody"
		}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"panel_key":"prd_draft"`) {
		t.Fatalf("expected nested preview payload to normalize panel_key, got %s", output)
	}
}

func TestPublishPreviewToolInfersStoryPlanPanelKey(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishPreview: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishPreview, json.RawMessage(`{
		"title": "Story Plan",
		"format": "json",
		"content": {
			"summary": "Slice plan",
			"proposed_stories": []
		}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"panel_key":"story_plan"`) {
		t.Fatalf("expected inferred story_plan panel key, got %s", output)
	}
}

func TestPublishPreviewToolInfersStoryPlanFormatAndTitle(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishPreview: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishPreview, json.RawMessage(`{
		"content": {
			"summary": "Slice plan",
			"proposed_stories": []
		}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan"`, `"title":"Story Plan"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected inferred preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishPreviewToolUsesEpicPlannerContextForStoryPlan(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		Agent: &appmodel.Agent{
			PresetKey: appmodel.AgentPresetEpicPlanner,
		},
		TargetType: "epic",
		AllowedTools: map[string]bool{
			ToolPublishPreview: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishPreview, json.RawMessage(`{
		"title": "Story Plan: Kafka Streams Performance Enhancement",
		"format": "json",
		"content": {
			"summary": "Slice plan",
			"proposed_stories": []
		}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan"`, `"title":"Story Plan: Kafka Streams Performance Enhancement"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected epic planner preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishPreviewToolUsesStoryPlannerContextForPlanningDoc(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		Agent: &appmodel.Agent{
			PresetKey: appmodel.AgentPresetStoryPlanner,
		},
		PlanningStage: appmodel.PlanningStageStoryPlanDoc,
		TargetType:    "story",
		AllowedTools: map[string]bool{
			ToolPublishPreview: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishPreview, json.RawMessage(`{
		"title": "Story Plan",
		"content": "# Outcome\nImplementation-ready plan"
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"story_plan_doc"`, `"title":"Story Plan"`, `"format":"markdown"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected story planner preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestExtractLatestPublishedPreviewPrefersToolInvocation(t *testing.T) {
	preview := ExtractLatestPublishedPreview([]appmodel.ToolInvocation{
		{
			ToolName: ToolPublishStoryPlan,
			Input: json.RawMessage(`{
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
