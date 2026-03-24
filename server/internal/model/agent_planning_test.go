package model

import (
	"encoding/json"
	"testing"
)

func TestStoryImplementationBriefUnmarshalAllowsStringFilesToModify(t *testing.T) {
	var brief StoryImplementationBrief
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
