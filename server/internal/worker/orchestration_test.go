package worker

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractPlanningProposalParsesJSONAndFencedJSON(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "plain json",
			content: `{"summary":"Breakdown","proposed_tasks":[{"ref":"task_1","name":"Task A","description":"Do A","task_type":"feature","estimate":3,"acceptance_criteria":["works"],"dependency_refs":[]}]}`,
		},
		{
			name:    "fenced json",
			content: "```json\n{\"summary\":\"Breakdown\",\"proposed_tasks\":[{\"ref\":\"task_1\",\"name\":\"Task A\",\"description\":\"Do A\",\"task_type\":\"feature\",\"estimate\":3}]}\n```",
		},
		{
			name:    "prefixed prose",
			content: "I drafted the task plan below.\n\n{\"summary\":\"Breakdown\",\"proposed_tasks\":[{\"ref\":\"task_1\",\"name\":\"Task A\",\"description\":\"Do A\",\"task_type\":\"feature\",\"estimate\":3,\"acceptance_criteria\":[\"works\"]}]}",
		},
		{
			name:    "prose with fenced json",
			content: "Here is the plan in the required format:\n```json\n{\"summary\":\"Breakdown\",\"proposed_tasks\":[{\"ref\":\"task_1\",\"name\":\"Task A\",\"description\":\"Do A\",\"task_type\":\"feature\",\"estimate\":3}]}\n```",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proposal, err := extractPlanningProposal([]Message{
				{
					Role: "assistant",
					Content: []ContentBlock{
						{Type: "text", Text: tc.content},
					},
				},
			}, "epic-1", "ver-1", 123)
			if err != nil {
				t.Fatalf("extractPlanningProposal returned error: %v", err)
			}
			if proposal.EpicID != "epic-1" {
				t.Fatalf("expected epic ID epic-1, got %q", proposal.EpicID)
			}
			if proposal.SpecVersionID != "ver-1" {
				t.Fatalf("expected spec version ver-1, got %q", proposal.SpecVersionID)
			}
			if proposal.TokensUsed != 123 {
				t.Fatalf("expected tokens 123, got %d", proposal.TokensUsed)
			}
			if len(proposal.ProposedTasks) != 1 || proposal.ProposedTasks[0].Name != "Task A" {
				t.Fatalf("unexpected proposal tasks: %+v", proposal.ProposedTasks)
			}
		})
	}
}

func TestExtractProductSpecDraftParsesJSON(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "plain json",
			content: `{"title":"Payments Spec","summary":"Summary","spec_markdown":"# Payments\n\n## Goals\n- Ship it","risks":["scope"],"assumptions":["paid plans only"],"open_questions":["migration?"],"sources":[{"title":"PCI DSS","url":"https://example.com/pci","note":"compliance baseline"}]}`,
		},
		{
			name:    "prefixed prose",
			content: "I created a draft spec.\n\n{\"title\":\"Payments Spec\",\"summary\":\"Summary\",\"spec_markdown\":\"# Payments\\n\\n## Goals\\n- Ship it\",\"risks\":[\"scope\"],\"open_questions\":[\"migration?\"]}",
		},
		{
			name:    "fenced with prose",
			content: "Here is the structured output:\n```json\n{\"title\":\"Payments Spec\",\"summary\":\"Summary\",\"spec_markdown\":\"# Payments\\n\\n## Goals\\n- Ship it\",\"risks\":[\"scope\"],\"open_questions\":[\"migration?\"]}\n```",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			draft, err := extractProductSpecDraft([]Message{
				{
					Role: "assistant",
					Content: []ContentBlock{
						{Type: "text", Text: tc.content},
					},
				},
			})
			if err != nil {
				t.Fatalf("extractProductSpecDraft returned error: %v", err)
			}
			if draft.Title != "Payments Spec" {
				t.Fatalf("unexpected title: %q", draft.Title)
			}
			if draft.SpecMarkdown == "" {
				t.Fatal("expected spec markdown to be populated")
			}
			if tc.name == "plain json" && len(draft.Assumptions) != 1 {
				t.Fatalf("expected assumptions to be parsed, got %#v", draft.Assumptions)
			}
			if tc.name == "plain json" && len(draft.Sources) != 1 {
				t.Fatalf("expected sources to be parsed, got %#v", draft.Sources)
			}
		})
	}
}

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
