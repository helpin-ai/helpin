package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
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
		"Approval requests happen inline in the same chat.",
		"## Current Facts And Next-Step Rules",
		"If an approved spec exists and tasks already exist:",
		"If an approved spec exists and no tasks exist yet:",
		"If no approved spec exists but a draft PRD already exists:",
		"If approved PRD persistence is already complete:",
		"`" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolRequestUserInput) + "`",
		"`" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolRequestApproval) + "`",
		"`" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolPublishPRDDraft) + "`",
		"`" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolPublishTaskPlan) + "`",
		"Use `isOther: true` instead of adding an explicit Other option.",
		"`files_to_modify` must be an array of objects",
		"\"name\": \"Add tracking helper\"",
		"\"dependency_refs\": [\"task_1\"]",
		"\"task_type\": \"feature\"",
		"\"test_strategy\": [\"...\"]",
		"The value of `content` must be a JSON object.",
		"Inside `proposed_tasks`, use the canonical field names `name` and `task_type`.",
		"Use `dependency_refs` only for refs that appear elsewhere in the same `proposed_tasks` array.",
		"load the `task_plan_publishing` skill with `" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolReadSkill) + "` before publishing or requesting approval",
		"`" + agentcontract.RuntimeToolNameForPrompt("list_workspace_teams") + "`",
		"Call `" + agentcontract.RuntimeToolNameForPrompt("ensure_epic_spec_doc") + "` with `{}`",
		"Call `" + agentcontract.RuntimeToolNameForPrompt("write_document_content") + "` with that `document_id`",
		"Call `" + agentcontract.RuntimeToolNameForPrompt("approve_epic_spec") + "` with `{}`",
		"Continue to task planning only after all three product tool calls succeed.",
		"call `" + agentcontract.RuntimeToolNameForPrompt("create_task_batch") + "` with the full approved `proposed_tasks` array",
		"Do not claim the task plan was applied based on approval alone.",
		"Only treat the phase as approved when the human gives a clear, explicit approval.",
		"Do not complete the run immediately after PRD approval.",
		"Do not end the run with a prose-only acknowledgement after change feedback.",
		"Thin context includes a sparse epic description",
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
}

func TestTaskPlannerSystemPromptIncludesDocApprovalLoop(t *testing.T) {
	prompt := defaultSystemPromptForPreset(model.AgentPresetTaskPlanner)
	if prompt == nil {
		t.Fatal("expected task planner prompt")
	}
	if strings.Contains(*prompt, "for Helpin") {
		t.Fatalf("expected task planner prompt to be workspace-generic\n%s", *prompt)
	}

	for _, snippet := range []string{
		"You are Scribe, the workspace task planner. You run a focused planning conversation for one task or work item.",
		"`" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolPublishTaskPlanDoc) + "`",
		"`" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolRequestUserInput) + "`",
		"`" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolRequestApproval) + "`",
		"call `" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolPublishTaskPlanDoc) + "`, then call `" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolRequestApproval) + "` with `phase=\"task_doc\"`, then stop.",
		"\"phase\": \"prd|tasks|task_doc\"",
		"`content` is required and must contain the full current markdown draft being reviewed.",
		"Never call the tool with only `title` or with empty `content`.",
		"After approval, call `" + agentcontract.RuntimeToolNameForPrompt("ensure_task_plan_doc") + "` with `{}`.",
		"Call `" + agentcontract.RuntimeToolNameForPrompt("write_document_content") + "` with that `document_id` and the full approved markdown draft as `content`.",
		"Do not claim the document was persisted or attached based on the approval alone.",
		"Revise the active planning document, republish the full replacement draft with `" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolPublishTaskPlanDoc) + "`, and request another approval request with `phase=\"task_doc\"` when the revision is ready.",
		"Produce a planning document, not code.",
		"keep repository interactions read-only",
		"Do not modify code, create files, apply patches, or change git state in this run.",
	} {
		if !strings.Contains(*prompt, snippet) {
			t.Fatalf("expected task planner prompt to contain %q\n%s", snippet, *prompt)
		}
	}
}

func TestReviewAgentSystemPromptIncludesInteractiveLoop(t *testing.T) {
	prompt := defaultSystemPromptForPreset(model.AgentPresetReviewAgent)
	if prompt == nil {
		t.Fatal("expected review prompt")
	}
	for _, snippet := range []string{
		"You are Lens, the workspace reviewer.",
		"`" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolRequestUserInput) + "`",
		"`" + agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolRequestReviewCheckpoint) + "`",
		"Treat review as an interactive loop, not a one-shot report.",
		"After the initial findings pass, produce a `review_checkpoint` handoff and stop.",
		"Do not finish immediately after posting findings unless the latest human reply clearly says the review is done",
		"If the human asks you to implement changes based on the review",
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
		"You are Forge, the workspace code builder.",
		"Implement the requested story or task directly in the repository",
		"Finish with a local commit only",
	} {
		if !strings.Contains(*prompt, snippet) {
			t.Fatalf("expected code builder prompt to contain %q\n%s", snippet, *prompt)
		}
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

func TestLegacyPromptIsManaged(t *testing.T) {
	cases := []struct {
		name      string
		presetKey string
		prompt    string
		want      bool
	}{
		{
			name:      "epic planner legacy prompt",
			presetKey: model.AgentPresetEpicPlanner,
			prompt:    "1. `prd_draft`\n2. `awaiting_prd_approval`\n3. `persist_prd`\n4. `task_plan`\n5. `awaiting_task_approval`\n6. `create_tasks`",
			want:      true,
		},
		{
			name:      "task planner legacy prompt",
			presetKey: model.AgentPresetTaskPlanner,
			prompt:    "You are Task Planner for Helpin. Use `publish_preview` and `request_human_approval`.",
			want:      true,
		},
		{
			name:      "code builder legacy prompt",
			presetKey: model.AgentPresetCodeBuilder,
			prompt:    "You are Builder, an AI coding agent. You write clean, correct code and follow existing project conventions.",
			want:      true,
		},
		{
			name:      "review agent branded prompt",
			presetKey: model.AgentPresetReviewAgent,
			prompt:    "You are Review Agent for Helpin.\n- Inspect the relevant code and run targeted validation when possible.",
			want:      true,
		},
		{
			name:      "persisted Echo default prompt",
			presetKey: model.AgentPresetSupportAgent,
			prompt:    "You are Echo, the workspace support agent. You are chatting live with a customer inside a support conversation.\nEvery turn MUST end with exactly one call to send_support_reply.\nFor any factual or product question, call search_knowledge FIRST.",
			want:      true,
		},
		{
			name:      "custom prompt stays custom",
			presetKey: model.AgentPresetEpicPlanner,
			prompt:    "You are my special planner. Speak in haiku and keep all output under 3 lines.",
			want:      false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prompt := tc.prompt
			if got := legacyPromptIsManaged(tc.presetKey, &prompt); got != tc.want {
				t.Fatalf("legacyPromptIsManaged(%q) = %v, want %v", tc.presetKey, got, tc.want)
			}
		})
	}

	currentPrompt := defaultSystemPromptForPreset(model.AgentPresetEpicPlanner)
	if currentPrompt == nil {
		t.Fatal("expected planner prompt")
	}
	if !legacyPromptIsManaged(model.AgentPresetEpicPlanner, currentPrompt) {
		t.Fatal("expected current built-in planner prompt to be treated as managed for legacy migration")
	}
}

func TestSyncManagedSystemPromptForPresetPreservesCustomPrompt(t *testing.T) {
	customPrompt := "You are a custom system agent override."
	prompt, version := syncManagedSystemPromptForPreset(model.AgentPresetEpicPlanner, &customPrompt, nil, "")
	if prompt == nil || *prompt != customPrompt {
		t.Fatalf("expected custom prompt to be preserved, got %+v", prompt)
	}
	if version != "" {
		t.Fatalf("expected custom prompt version to remain empty, got %q", version)
	}
}

func TestSyncManagedSystemPromptForPresetMigratesLegacyManagedPrompt(t *testing.T) {
	legacyPrompt := "1. `prd_draft`\n2. `awaiting_prd_approval`\n3. `persist_prd`\n4. `task_plan`\n5. `awaiting_task_approval`\n6. `create_tasks`"
	prompt, version := syncManagedSystemPromptForPreset(model.AgentPresetEpicPlanner, &legacyPrompt, nil, "")
	if prompt != nil {
		t.Fatalf("expected managed prompt to be stored as nil, got %+v", prompt)
	}
	if version == "" {
		t.Fatal("expected managed prompt sync to stamp a template version")
	}
}

func TestSyncManagedSystemPromptForPresetMergesLegacyPlanningNotes(t *testing.T) {
	notes := "Always mention release risk."
	prompt, version := syncManagedSystemPromptForPreset(model.AgentPresetEpicPlanner, nil, &notes, "")
	if prompt == nil || !strings.Contains(*prompt, "## Additional Instructions") || !strings.Contains(*prompt, notes) {
		t.Fatalf("expected planning notes to be merged into a preserved prompt, got %+v", prompt)
	}
	if version != "" {
		t.Fatalf("expected merged legacy planning notes to clear template version, got %q", version)
	}
}
