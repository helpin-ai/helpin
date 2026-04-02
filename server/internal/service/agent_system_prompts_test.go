package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDefaultProductPlannerSystemPromptIncludesInlineInteractiveLoop(t *testing.T) {
	prompt := defaultSystemPromptForPreset(model.AgentPresetEpicPlanner)
	if prompt == nil {
		t.Fatal("expected planner prompt")
	}

	requiredSnippets := []string{
		"You run the full PRD-to-stories loop inside a single interactive agent run.",
		"There is no hidden planner phase machine deciding the next step for you.",
		"Approval checkpoints happen inline in the same chat:",
		"## Current Facts And Next-Step Rules",
		"If an approved spec exists and stories already exist:",
		"If an approved spec exists and no stories exist yet:",
		"If no approved spec exists but a draft PRD already exists:",
		"If approved PRD persistence is already complete:",
		"`request_user_input`",
		"`request_review_checkpoint`",
		"`publish_prd_draft`",
		"`publish_task_plan`",
		"Use `isOther: true` instead of adding an explicit Other option.",
		"`files_to_modify` must be an array of objects",
		"\"name\": \"Add tracking helper\"",
		"\"dependency_refs\": [\"story_1\"]",
		"\"story_type\": \"feature\"",
		"\"test_strategy\": [\"...\"]",
		"The value of `content` must be a JSON object.",
		"Do not use `title` or `type` in story-plan JSON.",
		"Use `dependency_refs` only for refs that appear elsewhere in the same `proposed_stories` array.",
		"`list_workspace_teams`",
		"platform will persist the approved PRD artifact",
		"platform will apply the approved story plan artifact and create the stories",
		"Only treat the phase as approved when the human gives a clear, explicit approval.",
		"### Vertical Slicing (Critical)",
		"### Blocker & Enabler Consolidation",
		"### Story Separation & Scoping",
		"### Implementation Briefs (Required)",
		"### Acceptance Criteria (Required)",
		"GIVEN/WHEN/THEN",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(*prompt, snippet) {
			t.Fatalf("expected planner prompt to contain %q\n%s", snippet, *prompt)
		}
	}
	for _, legacyPhase := range []string{"awaiting_prd_approval", "awaiting_story_approval"} {
		if strings.Contains(*prompt, legacyPhase) {
			t.Fatalf("expected planner prompt to avoid legacy hard-approval phase %q\n%s", legacyPhase, *prompt)
		}
	}
	for _, branchMarker := range []string{"### Branch A:", "### Branch B:", "### Branch C:", "### Branch D:"} {
		if strings.Contains(*prompt, branchMarker) {
			t.Fatalf("expected planner prompt to avoid branch choreography %q\n%s", branchMarker, *prompt)
		}
	}
}

func TestProductPlannerPromptNeedsRefreshForLegacyApprovalPrompt(t *testing.T) {
	legacyPrompt := "1. `prd_draft`\n2. `awaiting_prd_approval`\n3. `persist_prd`\n5. `awaiting_story_approval`"
	if !productPlannerPromptNeedsRefresh(&legacyPrompt) {
		t.Fatal("expected legacy planner prompt to require refresh")
	}

	currentPrompt := defaultSystemPromptForPreset(model.AgentPresetEpicPlanner)
	if currentPrompt == nil {
		t.Fatal("expected planner prompt")
	}
	if productPlannerPromptNeedsRefresh(currentPrompt) {
		t.Fatal("expected current planner prompt to remain valid")
	}
}

func TestStoryPlannerSystemPromptIncludesDocApprovalLoop(t *testing.T) {
	prompt := defaultSystemPromptForPreset(model.AgentPresetTaskPlanner)
	if prompt == nil {
		t.Fatal("expected story planner prompt")
	}

	for _, snippet := range []string{
		"Run a single interactive planning conversation for one story.",
		"`publish_task_plan_doc`",
		"`request_user_input`",
		"`request_review_checkpoint`",
		"`phase=\"story_doc\"`",
		"platform will persist and link the approved preview",
		"Produce a planning document, not code.",
	} {
		if !strings.Contains(*prompt, snippet) {
			t.Fatalf("expected story planner prompt to contain %q\n%s", snippet, *prompt)
		}
	}
	for _, legacySnippet := range []string{
		"`ensure_story_plan_doc`",
		"`write_document_content`",
		"`link_document_to_object`",
	} {
		if strings.Contains(*prompt, legacySnippet) {
			t.Fatalf("expected story planner prompt to avoid legacy manual persistence step %q\n%s", legacySnippet, *prompt)
		}
	}
}

func TestStoryPlannerPromptNeedsRefreshForLegacyPrompt(t *testing.T) {
	legacyPrompt := "You are Story Planner for Helpin.\n- Ask clarifying questions inline.\n- Produce implementation-ready stories."
	if !storyPlannerPromptNeedsRefresh(&legacyPrompt) {
		t.Fatal("expected legacy story planner prompt to require refresh")
	}

	currentPrompt := defaultSystemPromptForPreset(model.AgentPresetTaskPlanner)
	if currentPrompt == nil {
		t.Fatal("expected story planner prompt")
	}
	if storyPlannerPromptNeedsRefresh(currentPrompt) {
		t.Fatal("expected current story planner prompt to remain valid")
	}
}

func TestCodeBuilderPromptNeedsRefreshForLegacyPrompt(t *testing.T) {
	legacyPrompt := "You are Builder, an AI coding agent. You write clean, correct code and follow existing project conventions."
	if !codeBuilderPromptNeedsRefresh(&legacyPrompt) {
		t.Fatal("expected legacy code builder prompt to require refresh")
	}

	currentPrompt := defaultSystemPromptForPreset(model.AgentPresetCodeBuilder)
	if currentPrompt == nil {
		t.Fatal("expected code builder prompt")
	}
	if codeBuilderPromptNeedsRefresh(currentPrompt) {
		t.Fatal("expected current code builder prompt to remain valid")
	}
}
