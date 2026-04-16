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
	if strings.Contains(*prompt, "for Helpin") {
		t.Fatalf("expected planner prompt to be workspace-generic\n%s", *prompt)
	}

	requiredSnippets := []string{
		"You run the full PRD-to-tasks loop inside a single interactive agent run.",
		"There is no hidden planner phase machine deciding the next step for you.",
		"Approval checkpoints happen inline in the same chat:",
		"## Current Facts And Next-Step Rules",
		"If an approved spec exists and tasks already exist:",
		"If an approved spec exists and no tasks exist yet:",
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
		"Inside `proposed_tasks`, use the canonical field names `name` and `task_type`.",
		"Use `dependency_refs` only for refs that appear elsewhere in the same `proposed_tasks` array.",
		"`list_workspace_teams`",
		"platform will persist the approved PRD artifact",
		"platform will apply the approved task plan artifact and create the tasks",
		"Only treat the phase as approved when the human gives a clear, explicit approval.",
		"This run is read-only with respect to the repository.",
		"do not modify code, create files, apply patches, or change git state",
		"### Vertical Slicing (Critical)",
		"### Blocker & Enabler Consolidation",
		"### Task Separation & Scoping",
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
	if strings.Contains(*prompt, "for Helpin") {
		t.Fatalf("expected task planner prompt to be workspace-generic\n%s", *prompt)
	}

	for _, snippet := range []string{
		"Run a single interactive planning conversation for one task.",
		"`publish_task_plan_doc`",
		"`request_user_input`",
		"`request_review_checkpoint`",
		"`phase=\"task_doc\"`",
		"`content` is required and must contain the full current markdown draft being reviewed.",
		"Never call the tool with only `title` or with empty `content`.",
		"platform will persist and link the approved preview",
		"Produce a planning document, not code.",
		"keep repository interactions read-only",
		"Do not modify code, create files, apply patches, or change git state in this run.",
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
	legacyPrompt := "You are Story Planner for Helpin.\nUse `publish_preview` and `request_human_approval` once the runtime tells you the current planner phase."
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

	legacyBrandedPrompt := "You are Code Builder for Helpin.\n- Implement the requested story or task directly in the repository."
	if !codeBuilderPromptNeedsRefresh(&legacyBrandedPrompt) {
		t.Fatal("expected branded code builder prompt to require refresh")
	}

	legacyCurrentPrompt := "You are Code Builder.\n\n- Implement the requested story or task directly in the repository.\n- Use the available tools to inspect code, make changes, run relevant validation, and prepare delivery artifacts."
	if !codeBuilderPromptNeedsRefresh(&legacyCurrentPrompt) {
		t.Fatal("expected previous code builder prompt to require refresh")
	}

	currentPrompt := defaultSystemPromptForPreset(model.AgentPresetCodeBuilder)
	if currentPrompt == nil {
		t.Fatal("expected code builder prompt")
	}
	if codeBuilderPromptNeedsRefresh(currentPrompt) {
		t.Fatal("expected current code builder prompt to remain valid")
	}
	if !strings.Contains(*currentPrompt, "Finish with a local commit only.") {
		t.Fatalf("expected current code builder prompt to require local commits only, got %q", *currentPrompt)
	}
	if !strings.Contains(*currentPrompt, "Remote delivery is backend-managed") {
		t.Fatalf("expected current code builder prompt to mention backend-managed delivery, got %q", *currentPrompt)
	}
}

func TestReviewAgentSystemPromptIncludesInteractiveLoop(t *testing.T) {
	prompt := defaultSystemPromptForPreset(model.AgentPresetReviewAgent)
	if prompt == nil {
		t.Fatal("expected review prompt")
	}
	for _, snippet := range []string{
		"You are Review Agent.",
		"`request_user_input`",
		"Treat review as an interactive loop, not a one-shot report.",
		"Do not finish immediately after posting findings unless the latest human reply clearly says the review is done",
		"If the human asks you to implement changes based on the review",
		"Treat the shared task context, branch metadata, task plan, and linked docs as review context",
		"Start by inspecting the existing branch diff and targeted validation against the configured base branch",
	} {
		if !strings.Contains(*prompt, snippet) {
			t.Fatalf("expected review prompt to contain %q\n%s", snippet, *prompt)
		}
	}
}

func TestCodeBuilderSystemPromptIncludesGenericExecutionContextGuidance(t *testing.T) {
	prompt := defaultSystemPromptForPreset(model.AgentPresetCodeBuilder)
	if prompt == nil {
		t.Fatal("expected code builder prompt")
	}
	for _, snippet := range []string{
		"Treat the shared task context, branch metadata, task plan, and linked docs as the authoritative execution brief",
		"Operate on the existing working branch against the configured base branch",
	} {
		if !strings.Contains(*prompt, snippet) {
			t.Fatalf("expected code builder prompt to contain %q\n%s", snippet, *prompt)
		}
	}
}

func TestReviewAgentPromptNeedsRefreshForLegacyPrompt(t *testing.T) {
	legacyPrompt := "You are Review Agent.\n\n- Inspect the relevant code and run targeted validation when possible.\n- Focus on correctness, regressions, missing tests, and delivery risk.\n- Report findings first, ordered by severity, with concrete file references when available.\n- Avoid low-signal commentary and avoid proposing unnecessary rewrites."
	if !reviewAgentPromptNeedsRefresh(&legacyPrompt) {
		t.Fatal("expected legacy review prompt to require refresh")
	}

	currentPrompt := defaultSystemPromptForPreset(model.AgentPresetReviewAgent)
	if currentPrompt == nil {
		t.Fatal("expected review prompt")
	}
	if reviewAgentPromptNeedsRefresh(currentPrompt) {
		t.Fatal("expected current review prompt to remain valid")
	}
}

func TestBuiltInNonPlannerPromptsAreWorkspaceGeneric(t *testing.T) {
	for _, presetKey := range []string{
		model.AgentPresetCRMOperator,
		model.AgentPresetSupportAgent,
		model.AgentPresetCodeBuilder,
		model.AgentPresetReviewAgent,
	} {
		prompt := defaultSystemPromptForPreset(presetKey)
		if prompt == nil {
			t.Fatalf("expected prompt for %q", presetKey)
		}
		if strings.Contains(*prompt, "for Helpin") {
			t.Fatalf("expected prompt for %q to be workspace-generic\n%s", presetKey, *prompt)
		}
	}
}

func TestBuiltInPromptNeedsGenericWorkspaceRefresh(t *testing.T) {
	cases := map[string]string{
		model.AgentPresetCRMOperator:  "You are CRM Operator for Helpin.\n- Work inside the current run using the allowed CRM, docs, and support tools.",
		model.AgentPresetSupportAgent: "You are Support Agent for Helpin.\n- Read the full conversation before drafting a reply.",
		model.AgentPresetReviewAgent:  "You are Review Agent for Helpin.\n- Inspect the relevant code and run targeted validation when possible.",
	}
	for presetKey, legacyPrompt := range cases {
		if !builtInPromptNeedsGenericWorkspaceRefresh(presetKey, &legacyPrompt) {
			t.Fatalf("expected %q branded prompt to require refresh", presetKey)
		}
	}
	currentPrompt := defaultSystemPromptForPreset(model.AgentPresetSupportAgent)
	if currentPrompt == nil {
		t.Fatal("expected support prompt")
	}
	if builtInPromptNeedsGenericWorkspaceRefresh(model.AgentPresetSupportAgent, currentPrompt) {
		t.Fatal("expected current support prompt to remain valid")
	}
}
