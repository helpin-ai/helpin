package agentcontract

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractJSONObjectFindsBalancedJSONInsideProse(t *testing.T) {
	raw := "I drafted this for review.\n\n{\"title\":\"Brace test\",\"spec_markdown\":\"# Spec\\n- include {literal} braces\",\"summary\":\"ok\"}\nThanks."
	got := extractJSONObject(raw)
	if got == "" {
		t.Fatal("expected embedded JSON object to be extracted")
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("expected extracted payload to be valid JSON: %v", err)
	}
	if payload["title"] != "Brace test" {
		t.Fatalf("unexpected extracted title: %q", payload["title"])
	}
}

func TestNormalizeTaskPlanPreviewContentRejectsStringTaskEntries(t *testing.T) {
	_, err := NormalizeTaskPlanPreviewContent(json.RawMessage(`{
		"summary":"Need to replace with correct structured payload.",
		"proposed_tasks":["task_1"]
	}`))
	if err == nil {
		t.Fatal("expected invalid task-plan preview content to be rejected")
	}
	if !strings.Contains(err.Error(), "task plan content proposed_tasks entries must be task objects") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeTaskPlanPreviewContentPreservesCanonicalProposedTasks(t *testing.T) {
	normalized, err := NormalizeTaskPlanPreviewContent(json.RawMessage(`{
		"summary":"Breakdown",
		"proposed_tasks":[
			{
				"ref":"task_1",
				"name":"Task A",
				"description":"Do A",
				"task_type":"feature",
				"acceptance_criteria":["works"],
				"dependency_refs":[]
			}
		]
	}`))
	if err != nil {
		t.Fatalf("NormalizeTaskPlanPreviewContent returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(normalized, &payload); err != nil {
		t.Fatalf("unmarshal normalized payload: %v", err)
	}
	if _, ok := payload["proposed_tasks"]; !ok {
		t.Fatalf("expected canonical proposed_tasks key, got %#v", payload)
	}
	if tasks, ok := payload["proposed_tasks"].([]any); !ok || len(tasks) != 1 {
		t.Fatalf("expected normalized proposed_tasks array, got %#v", payload)
	}
}
