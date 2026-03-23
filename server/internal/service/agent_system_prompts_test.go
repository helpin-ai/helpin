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
		"<questions>",
		"Do not emit `<question>` and `<options>` as sibling blocks.",
		"Each question must be single-select.",
		"`files_to_modify` must be an array of objects",
		"`list_workspace_teams`",
		"<spec_draft>",
		"<story_plan>",
		"<approval_request phase=\"prd|stories\">",
		"Call `ensure_epic_spec_doc`.",
		"Call `approve_epic_spec`",
		"Call `create_story_batch` to create the stories.",
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
