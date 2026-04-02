package model

import (
	"encoding/json"
	"testing"
)

func TestTaskImplementationBriefUnmarshalAllowsStringFilesToModify(t *testing.T) {
	var brief TaskImplementationBrief
	err := json.Unmarshal([]byte(`{
		"approach": "Follow the existing planner flow",
		"files_to_modify": [
			"server/internal/worker/tools.go",
			{"path":" frontend/src/components/pm/AgentRunDrawer.tsx ","action":"CREATE","description":" add inline rendering "}
		],
		"test_strategy": "Add regression coverage"
	}`), &brief)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if len(brief.FilesToModify) != 2 {
		t.Fatalf("expected 2 file changes, got %#v", brief.FilesToModify)
	}
	if brief.FilesToModify[0].Path != "server/internal/worker/tools.go" {
		t.Fatalf("expected string entry to become path, got %#v", brief.FilesToModify[0])
	}
	if brief.FilesToModify[0].Action != "modify" {
		t.Fatalf("expected string entry to default action to modify, got %#v", brief.FilesToModify[0])
	}
	if brief.FilesToModify[1].Path != "frontend/src/components/pm/AgentRunDrawer.tsx" {
		t.Fatalf("expected object path to be trimmed, got %#v", brief.FilesToModify[1])
	}
	if brief.FilesToModify[1].Action != "create" {
		t.Fatalf("expected object action to normalize, got %#v", brief.FilesToModify[1])
	}
	if brief.FilesToModify[1].Description != "add inline rendering" {
		t.Fatalf("expected description to trim whitespace, got %#v", brief.FilesToModify[1])
	}
}

func TestFileChangeUnmarshalDefaultsInvalidActionToModify(t *testing.T) {
	var change FileChange
	err := json.Unmarshal([]byte(`{
		"path": "server/internal/model/agent_planning.go",
		"action": "rewrite",
		"description": "normalize planner output"
	}`), &change)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if change.Action != "modify" {
		t.Fatalf("expected invalid action to default to modify, got %#v", change)
	}
}

func TestTaskImplementationBriefUnmarshalAllowsArrayTestStrategy(t *testing.T) {
	var brief TaskImplementationBrief
	err := json.Unmarshal([]byte(`{
		"approach": "Follow the approved proposal",
		"files_to_modify": [
			{"path":"server/internal/worker/tools.go","action":"modify","description":"widen schema"}
		],
		"test_strategy": ["Add decoder regression coverage", "Verify story creation still works"]
	}`), &brief)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	expected := "Add decoder regression coverage\nVerify story creation still works"
	if brief.TestStrategy != expected {
		t.Fatalf("expected joined test strategy %q, got %q", expected, brief.TestStrategy)
	}
}

func TestProposedTaskUnmarshalAllowsTitleAndTypeAliases(t *testing.T) {
	var story ProposedTask
	err := json.Unmarshal([]byte(`{
		"ref": "story_1",
		"title": "Add 4xx error metrics tracking infrastructure",
		"description": "Add the foundational metrics tracking function.",
		"type": "feature",
		"acceptance_criteria": ["works"]
	}`), &story)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if story.Name != "Add 4xx error metrics tracking infrastructure" {
		t.Fatalf("expected title alias to populate Name, got %#v", story)
	}
	if story.TaskType != "feature" {
		t.Fatalf("expected type alias to populate TaskType, got %#v", story)
	}
}

func TestTaskImplementationBriefUnmarshalReturnsRepairOrientedTestStrategyError(t *testing.T) {
	var brief TaskImplementationBrief
	err := json.Unmarshal([]byte(`{
		"approach": "Follow the approved proposal",
		"files_to_modify": [],
		"test_strategy": 123
	}`), &brief)
	if err == nil {
		t.Fatal("expected test_strategy validation error")
	}
	if err.Error() != "implementation_brief.test_strategy must be a string or array of strings" {
		t.Fatalf("unexpected error: %v", err)
	}
}
