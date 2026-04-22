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

func TestExtractLatestApprovalRequestPrefersToolInvocation(t *testing.T) {
	approval := ExtractLatestApprovalRequest([]appmodel.ToolInvocation{
		{
			ToolName: ToolRequestApproval,
			Input: json.RawMessage(`{
				"phase": "prd",
				"preview_panel_key": "prd_draft",
				"title": "Approve PRD",
				"summary": "Review the latest draft."
			}`),
		},
	})
	if approval == nil {
		t.Fatal("expected approval request")
	}
	if approval.PreviewPanelKey != "prd_draft" {
		t.Fatalf("expected preview panel key to round-trip, got %q", approval.PreviewPanelKey)
	}
	if approval.Phase != "prd" || approval.Title != "Approve PRD" || approval.Summary != "Review the latest draft." {
		t.Fatalf("unexpected approval request: %#v", approval)
	}
}

func TestExtractLatestReviewCheckpointRequestPrefersToolInvocation(t *testing.T) {
	review := ExtractLatestReviewCheckpointRequest([]appmodel.ToolInvocation{
		{
			ToolName: ToolRequestReviewCheckpoint,
			Input: json.RawMessage(`{
				"phase": "crm_review",
				"preview_panel_key": "deal_summary",
				"title": "Approve the stage change",
				"summary": "Move ACME to verbal commit.",
				"findings": [
					{
						"title": "Stage progression is premature",
						"body": "The latest note confirms pricing interest but not a verbal commitment.",
						"priority": "P1",
						"code_location": "crm/deal_stage.go:88"
					}
				],
				"overall_correctness": "incorrect",
				"overall_explanation": "Evidence does not support moving the deal to verbal commit."
			}`),
		},
	})
	if review == nil {
		t.Fatal("expected review checkpoint request")
	}
	if review.PreviewPanelKey != "deal_summary" {
		t.Fatalf("expected preview panel key to round-trip, got %q", review.PreviewPanelKey)
	}
	if review.Phase != "crm_review" || review.Title != "Approve the stage change" || review.Summary != "Move ACME to verbal commit." {
		t.Fatalf("unexpected review checkpoint request: %#v", review)
	}
	if len(review.Findings) != 1 || review.Findings[0].Priority != "P1" {
		t.Fatalf("expected structured findings to be preserved, got %#v", review)
	}
	if review.Findings[0].ID != "finding_1" {
		t.Fatalf("expected missing finding id to be normalized, got %#v", review.Findings[0])
	}
	if review.OverallCorrectness != "incorrect" {
		t.Fatalf("expected overall correctness to be preserved, got %#v", review)
	}
}

func TestRequestApprovalToolReturnsAwaitingApprovalPayload(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolRequestApproval: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolRequestApproval, json.RawMessage(`{
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
		"slot": "task_plan_doc",
		"title": "Task Planning Document",
		"content": "# Outcome\nBody"
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan_doc"`, `"format":"markdown"`} {
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
		"slot": "task_plan",
		"title": "Task Plan",
		"content": {"summary":"Slice plan","proposed_tasks":[{"ref":"task_1","name":"Task A","description":"Do A","task_type":"feature","acceptance_criteria":["works"],"dependency_refs":[]}]}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan"`, `"format":"json"`} {
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
			ToolPublishTaskPlan: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlan, json.RawMessage(`{
		"content": {"summary":"Slice plan","proposed_tasks":[{"ref":"task_1","name":"Task A","description":"Do A","task_type":"feature","acceptance_criteria":["works"],"dependency_refs":[]}]}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan"`, `"title":"Task Plan"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected task plan preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanToolAcceptsNestedPreviewPayload(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishTaskPlan: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlan, json.RawMessage(`{
		"preview": {
			"title": "Task Plan",
			"content": {"summary":"Slice plan","proposed_tasks":[{"ref":"task_1","name":"Task A","description":"Do A","task_type":"feature","acceptance_criteria":["works"],"dependency_refs":[]}]}
		}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected nested task plan preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanToolAcceptsRawPlanObject(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishTaskPlan: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlan, json.RawMessage(`{
		"title": "Task Plan",
		"summary":"Slice plan",
		"proposed_tasks":[{"ref":"task_1","name":"Task A","description":"Do A","task_type":"feature","acceptance_criteria":["works"],"dependency_refs":[]}]
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected raw task plan object preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanToolRejectsJSONStringContent(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishTaskPlan: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlan, json.RawMessage("{\n"+
		"  \"title\": \"Task Plan\",\n"+
		"  \"content\": \"Here is the plan in the required format:\\n```json\\n{\\\"summary\\\":\\\"Breakdown\\\",\\\"proposed_tasks\\\":[{\\\"ref\\\":\\\"task_1\\\",\\\"name\\\":\\\"Task A\\\",\\\"description\\\":\\\"Do A\\\",\\\"task_type\\\":\\\"feature\\\",\\\"acceptance_criteria\\\":[\\\"works\\\"]}]}\\n```\"\n"+
		"}"))
	if err == nil {
		t.Fatal("expected publish_task_plan to reject stringified JSON content")
	}
	if !strings.Contains(err.Error(), "publish_task_plan content must be a JSON object with summary and proposed_tasks") {
		t.Fatalf("expected structured task plan error, got %v", err)
	}
}

func TestPublishStoryPlanToolRejectsNonObjectJSONStringContent(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishTaskPlan: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlan, json.RawMessage(`{
		"title": "Task Plan",
		"content": "STORY PLAN: do the work"
	}`))
	if err == nil {
		t.Fatal("expected publish_task_plan to reject non-object string content")
	}
	if !strings.Contains(err.Error(), "publish_task_plan content must be a JSON object with summary and proposed_tasks") {
		t.Fatalf("expected repair-oriented task plan error, got %v", err)
	}
}

func TestPublishStoryPlanToolRejectsStringTaskEntries(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishTaskPlan: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlan, json.RawMessage(`{
		"title": "Task Plan",
		"content": {
			"summary": "Need to replace with correct structured payload.",
			"proposed_tasks": ["task_1"]
		}
	}`))
	if err == nil {
		t.Fatal("expected publish_task_plan to reject string task entries")
	}
	if !strings.Contains(err.Error(), "publish_task_plan requires content.proposed_tasks to be an array of task objects") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPublishStoryPlanToolRejectsRawToolArgumentWrapper(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishTaskPlan: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlan, json.RawMessage(`{
		"raw": "{\"title\":\"Task Plan: Increase Performance - Events Pipeline\""
	}`))
	if err == nil {
		t.Fatal("expected publish_task_plan to reject raw tool argument wrapper")
	}
	if !strings.Contains(err.Error(), "publish_task_plan input must be a JSON object with structured fields; do not send a raw string wrapper") {
		t.Fatalf("expected malformed raw wrapper error, got %v", err)
	}
}

func TestPublishStoryPlanToolAcceptsWrappedRawJSONObject(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishTaskPlan: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlan, json.RawMessage(`{
		"raw": "{\"title\":\"Task Plan\",\"content\":{\"summary\":\"Slice plan\",\"proposed_tasks\":[{\"ref\":\"task_1\",\"name\":\"Task A\",\"description\":\"Do A\",\"task_type\":\"feature\",\"acceptance_criteria\":[\"works\"],\"dependency_refs\":[]}]}}"
	}`))
	if err != nil {
		t.Fatalf("expected wrapped raw json object to be accepted, got %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan"`, `"title":"Task Plan"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected accepted wrapped payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanToolRejectsMalformedRetryWithoutReusingStaleContent(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishTaskPlan: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlan, json.RawMessage(`{
		"content": {"summary":"Slice plan","proposed_tasks":[{"ref":"task_1","name":"Task A","description":"Do A","task_type":"feature","acceptance_criteria":["works"],"dependency_refs":[]}]}
	}`))
	if err != nil {
		t.Fatalf("initial publish returned error: %v", err)
	}

	_, err = registry.ExecuteAllowed(ctx, ToolPublishTaskPlan, json.RawMessage(`{
		"title": "Task Plan"
	}`))
	if err == nil {
		t.Fatal("expected malformed retry to fail without reusing cached task-plan content")
	}
	if !strings.Contains(err.Error(), `publish_task_plan is missing content; include the task plan JSON object in "content"`) {
		t.Fatalf("unexpected malformed retry error: %v", err)
	}
}

func TestPublishStoryPlanDocToolReusesPreviewMarkdownContentOnMalformedRetry(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishTaskPlanDoc: true,
			ToolPreviewMarkdown:    true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPreviewMarkdown, json.RawMessage(`{
		"slot": "task_plan_doc",
		"title": "Task Planning Document",
		"content": "# Outcome\nImplementation-ready plan"
	}`))
	if err != nil {
		t.Fatalf("preview_md returned error: %v", err)
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlanDoc, json.RawMessage(`{
		"title": "Task Planning Document"
	}`))
	if err != nil {
		t.Fatalf("malformed publish_task_plan_doc retry should have reused cached markdown, got error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan_doc"`, `"format":"markdown"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected cached task planning doc preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanDocToolUsesCurrentAssistantDraftOnFirstMalformedCall(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context:              context.Background(),
		CurrentAssistantText: "# Outcome\nImplementation-ready plan",
		AllowedTools: map[string]bool{
			ToolPublishTaskPlanDoc: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlanDoc, json.RawMessage(`{
		"title": "Task Planning Document"
	}`))
	if err != nil {
		t.Fatalf("first malformed publish_task_plan_doc call should have used assistant draft, got error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan_doc"`, `"format":"markdown"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected assistant-draft task planning doc preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanDocToolFallsBackToExistingTaskPlanDocument(t *testing.T) {
	registry := NewToolRegistry(nil)
	documentID := "doc-task-plan-1"
	ctx := &ExecutionContext{
		Context: context.Background(),
		Task: &appmodel.PMTask{
			PlanDocumentID: &documentID,
		},
		Services: &ServiceBridge{
			GetDocumentContent: func(ctx context.Context, id string) (string, error) {
				if id != documentID {
					t.Fatalf("expected document id %q, got %q", documentID, id)
				}
				return "# Outcome\nRecovered draft", nil
			},
		},
		AllowedTools: map[string]bool{
			ToolPublishTaskPlanDoc: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlanDoc, json.RawMessage(`{
		"title": "Task Planning Document"
	}`))
	if err != nil {
		t.Fatalf("publish_task_plan_doc should have reused the existing task plan doc draft, got error: %v", err)
	}
	if !strings.Contains(output, `"panel_key":"task_plan_doc"`) {
		t.Fatalf("expected task plan doc publish payload, got %s", output)
	}
	preview, ok := ctx.PublishedPreviews["task_plan_doc"]
	if !ok {
		t.Fatal("expected published preview to be cached")
	}
	var content string
	if err := json.Unmarshal(preview.Content, &content); err != nil {
		t.Fatalf("expected markdown content, got %v", err)
	}
	if content != "# Outcome\nRecovered draft" {
		t.Fatalf("expected recovered draft content, got %q", content)
	}
}

func TestPublishStoryPlanDocToolPublishesCanonicalPreview(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishTaskPlanDoc: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlanDoc, json.RawMessage(`{
		"content": "# Outcome\nImplementation-ready plan"
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan_doc"`, `"title":"Task Planning Document"`, `"format":"markdown"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected task planning doc preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishStoryPlanDocToolReturnsRepairOrientedMissingContentError(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishTaskPlanDoc: true,
		},
	}

	_, err := registry.ExecuteAllowed(ctx, ToolPublishTaskPlanDoc, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected missing content error")
	}
	if !strings.Contains(err.Error(), `publish_task_plan_doc is missing content; include markdown in "content"`) {
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
		"panelKey": "task_plan",
		"title": "Task Plan",
		"format": "json",
		"content": {
			"summary": "Slice plan",
			"proposed_tasks": [{"ref":"task_1","name":"Task A","description":"Do A","task_type":"feature","acceptance_criteria":["works"],"dependency_refs":[]}]
		}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"panel_key":"task_plan"`) {
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

func TestPublishPreviewToolInfersTaskPlanPanelKey(t *testing.T) {
	registry := NewToolRegistry(nil)
	ctx := &ExecutionContext{
		Context: context.Background(),
		AllowedTools: map[string]bool{
			ToolPublishPreview: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishPreview, json.RawMessage(`{
		"title": "Task Plan",
		"format": "json",
		"content": {
			"summary": "Slice plan",
			"proposed_tasks": [{"ref":"task_1","name":"Task A","description":"Do A","task_type":"feature","acceptance_criteria":["works"],"dependency_refs":[]}]
		}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	if !strings.Contains(output, `"panel_key":"task_plan"`) {
		t.Fatalf("expected inferred task_plan panel key, got %s", output)
	}
}

func TestPublishPreviewToolInfersTaskPlanFormatAndTitle(t *testing.T) {
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
			"proposed_tasks": [{"ref":"task_1","name":"Task A","description":"Do A","task_type":"feature","acceptance_criteria":["works"],"dependency_refs":[]}]
		}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan"`, `"title":"Task Plan"`, `"format":"json"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected inferred preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestPublishPreviewToolUsesEpicPlannerContextForTaskPlan(t *testing.T) {
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
		"title": "Task Plan: Kafka Streams Performance Enhancement",
		"format": "json",
		"content": {
			"summary": "Slice plan",
			"proposed_tasks": [{"ref":"task_1","name":"Task A","description":"Do A","task_type":"feature","acceptance_criteria":["works"],"dependency_refs":[]}]
		}
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan"`, `"title":"Task Plan: Kafka Streams Performance Enhancement"`, `"format":"json"`} {
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
			PresetKey: appmodel.AgentPresetTaskPlanner,
		},
		PlanningStage: appmodel.PlanningStageTaskPlanDoc,
		TargetType:    "story",
		AllowedTools: map[string]bool{
			ToolPublishPreview: true,
		},
	}

	output, err := registry.ExecuteAllowed(ctx, ToolPublishPreview, json.RawMessage(`{
		"title": "Task Plan",
		"content": "# Outcome\nImplementation-ready plan"
	}`))
	if err != nil {
		t.Fatalf("ExecuteAllowed returned error: %v", err)
	}
	for _, snippet := range []string{`"panel_key":"task_plan_doc"`, `"title":"Task Plan"`, `"format":"markdown"`} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected task planner preview payload to contain %q, got %s", snippet, output)
		}
	}
}

func TestExtractLatestPublishedPreviewPrefersToolInvocation(t *testing.T) {
	preview := ExtractLatestPublishedPreview([]appmodel.ToolInvocation{
		{
			ToolName: ToolPublishTaskPlan,
			Input: json.RawMessage(`{
				"content": {
					"summary": "Slice plan",
					"proposed_tasks": [{"ref":"task_1","name":"Task A","description":"Do A","task_type":"feature","acceptance_criteria":["works"],"dependency_refs":[]}]
				}
			}`),
		},
	}, "task_plan")
	if preview == nil {
		t.Fatal("expected published preview")
	}
	if preview.PanelKey != "task_plan" || preview.Title != "Task Plan" || preview.Format != PreviewFormatJSON {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	if !json.Valid(preview.Content) {
		t.Fatalf("expected preview content to be valid json, got %s", string(preview.Content))
	}
}
