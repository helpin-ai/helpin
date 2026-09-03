package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestManagedDocumentPromptsRequireArtifactInsertionTool(t *testing.T) {
	documentationPrompt := defaultSystemPromptForPreset(model.AgentPresetDocumentationAgent)
	if documentationPrompt == nil {
		t.Fatal("expected documentation prompt")
	}
	prompts := map[string]string{
		"ask agent":           askAgentSystemPrompt(),
		"documentation agent": *documentationPrompt,
	}
	for name, prompt := range prompts {
		for _, required := range []string{
			"Required document artifact embedding policy",
			"write_document_content does not embed",
			"call insert_document_artifact",
			"Use artifact_id, not artifact_ref",
			"Do not claim an artifact is embedded until insert_document_artifact succeeds",
		} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("%s prompt missing %q:\n%s", name, required, prompt)
			}
		}
	}
}

func TestManagedAskAgentExecutionPolicyPrefersNarrowRepositoryReads(t *testing.T) {
	prompt := agentcontract.EnsureAskAgentExecutionPolicy(model.AgentPresetAskAgent, askAgentSystemPrompt())
	for _, required := range []string{
		"Before checkout_repositories, call list_repositories",
		"returned repository_id (preferred) or exact repo_full_name",
		"Never pass a display name or bare repository name",
		"use read_symbol directly when you know a declaration name",
		"locate exact files or lines with repository_search or list_symbols",
		"use read_files for bounded known spans",
		"use list_symbols before paging through a file when you do not know the declaration name",
		"continue exactly from next_start_line; do not restart the same range or increase limit_lines",
		"Use trace_symbol for callers or callees",
		"Do not use reads for broad exploration or re-read a whole file",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("Ask Agent prompt missing repository navigation guidance %q:\n%s", required, prompt)
		}
	}
}

func TestManagedAskAgentExecutionPolicyIsEfficientAndAlwaysCurrent(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
	}{
		{name: "managed", prompt: askAgentSystemPrompt()},
		{name: "custom", prompt: "You are the workspace concierge. Preserve this custom identity."},
		{name: "stale header", prompt: "Custom identity.\n\n## Required Ask Agent execution policy\n\nUse the old rules."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &model.Agent{
				PresetKey:    model.AgentPresetAskAgent,
				RuntimeKind:  "native_sdk",
				SystemPrompt: &tt.prompt,
			}
			effective := runtimeAgentFromHelpinAgent(agent, "helpin").SystemPrompt
			for _, required := range []string{
				"## Required Ask Agent execution policy v2",
				"Begin with the smallest targeted action",
				"After every tool result",
				"resolve a material uncertainty",
				"Do not explore merely to build a complete picture",
				"Distinguish confirmed findings",
				"Do not announce routine tool calls",
			} {
				if !strings.Contains(effective, required) {
					t.Fatalf("effective Ask prompt missing %q:\n%s", required, effective)
				}
			}
			if count := strings.Count(effective, "## Required Ask Agent execution policy v2"); count != 1 {
				t.Fatalf("current Ask execution policy count = %d, want 1:\n%s", count, effective)
			}
		})
	}

	once := agentcontract.EnsureAskAgentExecutionPolicy(model.AgentPresetAskAgent, "Custom identity.")
	twice := agentcontract.EnsureAskAgentExecutionPolicy(model.AgentPresetAskAgent, once)
	if count := strings.Count(twice, "## Required Ask Agent execution policy v2"); count != 1 {
		t.Fatalf("repeated Ask policy assembly count = %d, want 1:\n%s", count, twice)
	}
}

func TestAskAgentPresetPromptDoesNotDuplicateProductExecutionPolicy(t *testing.T) {
	prompt := askAgentSystemPrompt()
	for _, duplicated := range []string{
		"Prefer doing sequential work yourself",
		"For complex or long requests, call update_plan early",
		"Do not delegate merely because a request has multiple steps",
	} {
		if strings.Contains(prompt, duplicated) {
			t.Fatalf("Ask preset prompt duplicates product execution policy %q:\n%s", duplicated, prompt)
		}
	}
}

func TestAskAgentDoesNotRetryPricingConfigurationFailures(t *testing.T) {
	prompt := askAgentSystemPrompt()
	for _, required := range []string{
		"model unavailable under current pricing",
		"pricing configuration missing",
		"non-retriable",
		"do not retry it through another agent, target, or launch method",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("Ask Agent prompt missing non-retriable pricing guidance %q", required)
		}
	}
}

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
		"`" + agentcontract.CanonicalToolName(agentcontract.ToolRequestUserInput) + "`",
		"`" + agentcontract.CanonicalToolName(agentcontract.ToolRequestApproval) + "`",
		"`" + agentcontract.CanonicalToolName(agentcontract.ToolPublishPRDDraft) + "`",
		"`" + agentcontract.CanonicalToolName(agentcontract.ToolPublishTaskPlan) + "`",
		"Use `isOther: true` instead of adding an explicit Other option.",
		"`files_to_modify` must be an array of objects",
		"\"name\": \"Add tracking helper\"",
		"\"dependency_refs\": [\"task_1\"]",
		"\"task_type\": \"feature\"",
		"\"test_strategy\": [\"...\"]",
		"The value of `content` must be a JSON object.",
		"Inside `proposed_tasks`, use the canonical field names `name` and `task_type`.",
		"Use `dependency_refs` only for refs that appear elsewhere in the same `proposed_tasks` array.",
		"load the `task_plan_publishing` skill with `" + agentcontract.CanonicalToolName(agentcontract.ToolReadSkill) + "` before publishing or requesting approval",
		"`" + agentcontract.CanonicalToolName("list_workspace_teams") + "`",
		"Call `" + agentcontract.CanonicalToolName("ensure_epic_spec_doc") + "` with `{}`",
		"Call `" + agentcontract.CanonicalToolName("write_document_content") + "` with that `document_id`",
		"Call `" + agentcontract.CanonicalToolName("approve_epic_spec") + "` with `{}`",
		"Continue to task planning only after all three product tool calls succeed.",
		"call `" + agentcontract.CanonicalToolName("create_task_batch") + "` with the full approved `proposed_tasks` array",
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
		"`" + agentcontract.CanonicalToolName(agentcontract.ToolPublishTaskPlanDoc) + "`",
		"`" + agentcontract.CanonicalToolName(agentcontract.ToolRequestUserInput) + "`",
		"`" + agentcontract.CanonicalToolName(agentcontract.ToolRequestApproval) + "`",
		"call `" + agentcontract.CanonicalToolName(agentcontract.ToolPublishTaskPlanDoc) + "`, then call `" + agentcontract.CanonicalToolName(agentcontract.ToolRequestApproval) + "` with `phase=\"task_doc\"`, then stop.",
		"\"phase\": \"prd|tasks|task_doc\"",
		"`content` is required and must contain the full current markdown draft being reviewed.",
		"Never call the tool with only `title` or with empty `content`.",
		"After approval, call `" + agentcontract.CanonicalToolName("ensure_task_plan_doc") + "` with `{}`.",
		"Call `" + agentcontract.CanonicalToolName("write_document_content") + "` with that `document_id` and the full approved markdown draft as `content`.",
		"Do not claim the document was persisted or attached based on the approval alone.",
		"Revise the active planning document, republish the full replacement draft with `" + agentcontract.CanonicalToolName(agentcontract.ToolPublishTaskPlanDoc) + "`, and request another approval request with `phase=\"task_doc\"` when the revision is ready.",
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
		"`" + agentcontract.CanonicalToolName(agentcontract.ToolRequestUserInput) + "`",
		"`" + agentcontract.CanonicalToolName(agentcontract.ToolRequestReviewCheckpoint) + "`",
		"Treat review as an interactive loop, not a one-shot report.",
		"After the initial findings pass, produce a `review_checkpoint` handoff and stop.",
		"Do not finish immediately after the initial findings pass unless the latest human reply clearly says the review is done",
		"Do not ask for the same missing value again in the current run.",
		"Do not offer a selectable option whose label merely promises to provide it",
		"Ask at most once for a missing path, credential, deployment configuration, or other dependency outside the available workspace.",
		"Treat an inaccessible external dependency as a delivery blocker, not as a new review finding.",
		"finish whenever no useful in-scope action remains; an explicit \"done\" reply is not required.",
		"If an approved finding remains blocked only by an unavailable external dependency, report the partial completion and blocker once and finish as well.",
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
		"Implement the requested task directly in the repository",
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
