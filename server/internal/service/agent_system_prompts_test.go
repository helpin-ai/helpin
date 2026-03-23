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
		"`request_human_input`",
		"`request_human_approval`",
		"`publish_preview`",
		"Each question must be single-select.",
		"`files_to_modify` must be an array of objects",
		"`list_workspace_teams`",
		"`panel_key=\"prd_draft\"`",
		"`panel_key=\"story_plan\"`",
		"Call `ensure_epic_spec_doc`.",
		"Call `approve_epic_spec`",
		"Call `create_story_batch` to create the stories.",
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
