package agentcontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildSystemPromptDirectEpicRunUsesAgentSystemPrompt(t *testing.T) {
	systemPrompt := "You are the saved planner prompt."

	prompt := BuildSystemPrompt(
		&model.Agent{
			Name:         "Planner",
			PresetKey:    model.AgentPresetEpicPlanner,
			SystemPrompt: &systemPrompt,
		},
		nil,
		&model.PMEpic{Name: "Billing refresh"},
		nil,
		"",
		"",
		nil,
	)

	if !strings.Contains(prompt, systemPrompt) {
		t.Fatalf("expected prompt to contain stored system prompt\n%s", prompt)
	}
	for _, unexpected := range []string{
		"Planning methodology:",
		"Think like an analyst first",
		"Treat these as secondary preferences",
		"Use tools to update the epic's canonical product spec",
	} {
		if strings.Contains(prompt, unexpected) {
			t.Fatalf("did not expect direct epic prompt to contain %q\n%s", unexpected, prompt)
		}
	}
}

func TestBuildUserPromptDirectEpicRunIsContextOnly(t *testing.T) {
	prompt := BuildUserPrompt(
		nil,
		nil,
		&model.PMEpic{Name: "Billing refresh"},
		nil,
		nil,
		nil,
		nil,
		nil,
		"",
		"Run mode: interactive\nOperator notes:\nFocus on B2B admins first.",
	)

	if strings.Contains(prompt, "Please work on epic: **Billing refresh**") {
		t.Fatalf("did not expect imperative epic framing\n%s", prompt)
	}
	if !strings.Contains(prompt, "Epic: **Billing refresh**") {
		t.Fatalf("expected epic context in prompt\n%s", prompt)
	}
	if !strings.Contains(prompt, "Context:") {
		t.Fatalf("expected explicit context heading in prompt\n%s", prompt)
	}
	for _, unexpected := range []string{
		"Use the planner tools to create or update the canonical PRD",
		"Please plan and execute the epic setup directly",
		"Please complete this task. Start by reading the relevant files to understand the codebase, then implement the changes.",
	} {
		if strings.Contains(prompt, unexpected) {
			t.Fatalf("did not expect direct epic user prompt to contain %q\n%s", unexpected, prompt)
		}
	}
	if !strings.Contains(prompt, "Run mode: interactive") {
		t.Fatalf("expected initial instructions to be preserved\n%s", prompt)
	}
}

func TestBuildUserPromptIncludesArtifactContext(t *testing.T) {
	prompt := BuildUserPrompt(
		nil,
		nil,
		&model.PMEpic{Name: "Billing refresh"},
		nil,
		nil,
		nil,
		nil,
		&ArtifactContext{
			Entries: []ArtifactContextEntry{
				{
					Label:   "Current preview for prd_draft",
					Source:  "run_preview",
					Status:  "draft",
					Format:  "markdown",
					Content: "# Problem\n\nCurrent draft body",
				},
			},
		},
		"",
		"",
	)

	if !strings.Contains(prompt, "Current persisted artifacts:") {
		t.Fatalf("expected artifact context heading in prompt\n%s", prompt)
	}
	if !strings.Contains(prompt, "Current preview for prd_draft [source=run_preview, status=draft, format=markdown]:") {
		t.Fatalf("expected artifact context metadata in prompt\n%s", prompt)
	}
	if !strings.Contains(prompt, "Current draft body") {
		t.Fatalf("expected artifact content in prompt\n%s", prompt)
	}
}

func TestBuildUserPromptIncludesWorkspaceContext(t *testing.T) {
	prompt := BuildUserPromptWithRunInput(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"",
		"",
		&model.AgentRunInputPayload{
			WorkspaceContext: &model.AgentRunWorkspaceContext{
				Name:                  "Acme",
				WebsiteURL:            "https://acme.com",
				CompanyProductContext: "Acme helps support and product teams answer customers with repo-aware context.",
			},
		},
	)

	for _, want := range []string{
		"Workspace Context",
		"Workspace: **Acme**",
		"Website: https://acme.com",
		"Acme helps support and product teams",
		"Use this as high-level workspace context.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestBuildUserPromptTaskPlannerUsesNeutralPlanningContext(t *testing.T) {
	prompt := BuildUserPrompt(
		nil,
		&model.PMTask{Name: "Inbox triage automation"},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		model.PlanningStageTaskPlanDoc,
		"Operator notes:\nFocus on approval UX.",
	)

	for _, marker := range []string{
		"Task: **Inbox triage automation**",
		"Planning stage: task_plan_doc",
		"Operator notes:",
	} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("expected prompt to contain %q\n%s", marker, prompt)
		}
	}
	for _, snippet := range []string{
		"Create a reviewable task planning document",
		"open questions",
	} {
		if strings.Contains(prompt, snippet) {
			t.Fatalf("did not expect duplicated task-plan guidance %q\n%s", snippet, prompt)
		}
	}
	if strings.Contains(prompt, "Please complete this task. Start by reading the relevant files to understand the codebase, then implement the changes.") {
		t.Fatalf("did not expect implementation-oriented task prompt\n%s", prompt)
	}
}

func TestBuildUserPromptTaskPlannerLabelsParentEpicAsBackground(t *testing.T) {
	taskDescription := "<p>Instrument producer send operations.</p>"
	epicDescription := "<p>Observability PRD details.</p>"
	prompt := BuildUserPrompt(
		nil,
		&model.PMTask{Name: "Instrument Kafka producer send operations with metrics", Description: &taskDescription},
		&model.PMEpic{Name: "Kafka observability", Description: &epicDescription},
		nil,
		nil,
		nil,
		nil,
		nil,
		model.PlanningStageTaskPlanDoc,
		"",
	)

	for _, marker := range []string{
		"Target task: **Instrument Kafka producer send operations with metrics**",
		"This run is scoped to the target task. Parent epic/PRD context below is background only.",
		"Target task description:",
		"Parent epic background: **Kafka observability**",
		"Parent epic description:",
	} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("expected prompt to contain %q\n%s", marker, prompt)
		}
	}
	if strings.Contains(prompt, "\nEpic: **Kafka observability**") {
		t.Fatalf("expected parent epic to be labeled as background\n%s", prompt)
	}
}

func TestBuildUserPromptReviewAgentUsesNeutralTaskContext(t *testing.T) {
	prompt := BuildUserPrompt(
		&model.Agent{PresetKey: model.AgentPresetReviewAgent},
		&model.PMTask{Name: "Inbox triage automation"},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"",
		"",
	)

	if strings.Contains(prompt, "Please work on the task: **Inbox triage automation**") {
		t.Fatalf("did not expect imperative task framing\n%s", prompt)
	}
	if !strings.Contains(prompt, "Task: **Inbox triage automation**") {
		t.Fatalf("expected neutral task context\n%s", prompt)
	}
}

func TestBuildUserPromptNormalizesRichTextDescriptionsToMarkdown(t *testing.T) {
	description := "<h2>Scope</h2><p><strong>Important</strong> rollout</p><ul><li>First</li></ul>"

	prompt := BuildUserPrompt(
		nil,
		&model.PMTask{Name: "Inbox triage automation", Description: &description},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"",
		"",
	)

	for _, snippet := range []string{"## Scope", "**Important** rollout", "- First"} {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("expected prompt to contain %q\n%s", snippet, prompt)
		}
	}
	if strings.Contains(prompt, "<strong>") || strings.Contains(prompt, "<ul>") {
		t.Fatalf("expected prompt to avoid raw HTML\n%s", prompt)
	}
}

func TestBuildUserPromptOmitsSavedSystemPromptAndKeepsContext(t *testing.T) {
	systemPrompt := "Use the repo conventions and keep changes incremental."

	prompt := BuildUserPrompt(
		&model.Agent{SystemPrompt: &systemPrompt},
		&model.PMTask{Name: "Inbox triage automation"},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"",
		"",
	)

	if strings.Contains(prompt, systemPrompt) {
		t.Fatalf("did not expect user prompt to repeat saved system prompt\n%s", prompt)
	}
	if !strings.HasPrefix(prompt, "Context:\nCurrent system date is: ") || !strings.Contains(prompt, "\nTask: **Inbox triage automation**") {
		t.Fatalf("expected user prompt to start with context\n%s", prompt)
	}
}

func TestBuildExecutionSupplementPromptIncludesResumeGuidanceAndArtifacts(t *testing.T) {
	supplement := BuildExecutionSupplementPrompt(
		&model.AgentRun{InvocationMode: model.InvocationModeInteractive},
		map[string]string{
			"target_id":      "deal-123",
			"crm_contact_id": "contact-456",
		},
		&ArtifactContext{
			Entries: []ArtifactContextEntry{
				{
					Label:   "Current preview for prd_draft",
					Source:  "run_preview",
					Status:  "draft",
					Format:  "markdown",
					Content: "# Problem\n\nLatest draft body",
				},
			},
		},
	)

	for _, marker := range []string{
		"This is an interactive transcript that may resume after a human reply.",
		"Do not treat a human reply as the end of the run by default.",
		"Treat the durable run facts below as the authoritative identifiers",
		"Durable run facts:",
		"- crm_contact_id=contact-456",
		"- target_id=deal-123",
		"Use the latest persisted artifacts below as the current source of truth",
		"Latest draft body",
	} {
		if !strings.Contains(supplement, marker) {
			t.Fatalf("expected supplement to contain %q\n%s", marker, supplement)
		}
	}
}

func TestBuildRuntimeSystemPromptWithStagedForgeSkillsKeepsPresetPreamble(t *testing.T) {
	agent := &model.Agent{
		Name:                      "Forge",
		PresetKey:                 model.AgentPresetCodeBuilder,
		ResolvedSkillInstructions: "Implement the requested story directly in the repository.\nDo not push the branch.",
	}

	prompt := BuildRuntimeSystemPrompt(
		agent,
		&model.PMTask{Name: "Implement metrics"},
		nil,
		nil,
		"",
		"",
		nil,
		false,
		false,
	)

	if !strings.Contains(prompt, "You are Forge, the workspace code builder.") {
		t.Fatalf("expected staged Forge prompt to keep preset preamble\n%s", prompt)
	}
	if strings.Contains(prompt, "Implement the requested story directly in the repository.") {
		t.Fatalf("expected staged Forge prompt to omit inline skill body\n%s", prompt)
	}
	if strings.Contains(prompt, "## Current Task") {
		t.Fatalf("did not expect staged Forge prompt to duplicate task context\n%s", prompt)
	}
	for _, unexpected := range []string{
		"Use the provided tools to read, write, and search files.",
		"list_directory, ripgrep, search_files, or list_symbols",
		"Gather context incrementally before broad repository reads or edits.",
		"Prefer targeted inspection of the relevant code before making broad changes.",
		"Keep code changes focused and validate them with practical checks when possible.",
		"Commit and push your changes when the task is complete.",
	} {
		if strings.Contains(prompt, unexpected) {
			t.Fatalf("did not expect runtime prompt to contain native/tool-specific rule %q\n%s", unexpected, prompt)
		}
	}
	for _, expected := range []string{
		"- Work within the cloned repository only.",
		"- Run tests after making changes when possible.",
		"- Leave Helpin artifacts and summaries in a state a human can review.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected runtime prompt to contain generic rule %q\n%s", expected, prompt)
		}
	}
}

func TestBuildRuntimeSystemPromptWithStagedLensSkillsKeepsPresetPreamble(t *testing.T) {
	agent := &model.Agent{
		Name:                      "Lens",
		PresetKey:                 model.AgentPresetReviewAgent,
		ResolvedSkillInstructions: "Review the implementation and identify risks first.",
	}

	prompt := BuildRuntimeSystemPrompt(
		agent,
		&model.PMTask{Name: "Audit onboarding flow"},
		nil,
		nil,
		"",
		"",
		nil,
		false,
		false,
	)

	if !strings.Contains(prompt, "You are Lens, the workspace reviewer.") {
		t.Fatalf("expected staged Lens prompt to keep preset preamble\n%s", prompt)
	}
	if strings.Contains(prompt, "Review the implementation and identify risks first.") {
		t.Fatalf("expected staged Lens prompt to omit inline skill body\n%s", prompt)
	}
}

func TestBuildRuntimeSystemPromptWithStagedCustomSkillsPreservesExplicitSystemPrompt(t *testing.T) {
	systemPrompt := "You are a careful integration engineer. Favor minimal blast radius."
	agent := &model.Agent{
		Name:                      "Custom Builder",
		SystemPrompt:              &systemPrompt,
		ResolvedSkillInstructions: "Always inspect deployment manifests before editing app code.",
	}

	prompt := BuildRuntimeSystemPrompt(
		agent,
		&model.PMTask{Name: "Tune deployment config"},
		nil,
		nil,
		"",
		"",
		nil,
		false,
		false,
	)

	if !strings.Contains(prompt, systemPrompt) {
		t.Fatalf("expected staged custom prompt to preserve explicit system prompt\n%s", prompt)
	}
	if strings.Contains(prompt, "Always inspect deployment manifests before editing app code.") {
		t.Fatalf("expected staged custom prompt to omit inline skill body\n%s", prompt)
	}
}

func TestBuildRuntimeSystemPromptWithoutStagedSkillsStillInlinesResolvedSkillText(t *testing.T) {
	agent := &model.Agent{
		Name:                      "Forge",
		PresetKey:                 model.AgentPresetCodeBuilder,
		ResolvedSkillInstructions: "Implement the requested story directly in the repository.\nDo not push the branch.",
	}

	prompt := BuildRuntimeSystemPrompt(
		agent,
		&model.PMTask{Name: "Implement metrics"},
		nil,
		nil,
		"",
		"",
		nil,
		true,
		true,
	)

	if !strings.Contains(prompt, "You are Forge, the workspace code builder.") {
		t.Fatalf("expected unstaged Forge prompt to keep preset prompt\n%s", prompt)
	}
	if !strings.Contains(prompt, "Implement the requested story directly in the repository.") {
		t.Fatalf("expected unstaged Forge prompt to inline skill body\n%s", prompt)
	}
	if strings.Contains(prompt, "## Current Task") {
		t.Fatalf("did not expect runtime prompt to duplicate task context\n%s", prompt)
	}
}

func TestBuildSystemPromptNonEpicPreservesAgentSystemPrompt(t *testing.T) {
	systemPrompt := "You are a careful engineer."

	prompt := BuildSystemPrompt(
		&model.Agent{
			Name:         "Engineer",
			PresetKey:    model.AgentPresetCodeBuilder,
			SystemPrompt: &systemPrompt,
		},
		&model.PMTask{Name: "Implement feature flag"},
		nil,
		nil,
		"",
		"",
		nil,
	)

	if !strings.Contains(prompt, systemPrompt) {
		t.Fatalf("expected non-epic prompt to preserve custom system prompt\n%s", prompt)
	}
}

func TestBuildSystemPromptStoryIncludesSearchFirstAndGuardedEditGuidance(t *testing.T) {
	systemPrompt := "You are a careful engineer."

	prompt := BuildSystemPrompt(
		&model.Agent{
			Name:         "Engineer",
			PresetKey:    model.AgentPresetCodeBuilder,
			SystemPrompt: &systemPrompt,
		},
		&model.PMTask{Name: "Implement feature flag"},
		nil,
		nil,
		"",
		"",
		nil,
	)

	for _, expected := range []string{
		"When you know a declaration's name, start with `find_symbol` to locate it and `read_symbol` to read it in full; both return exact line ranges, so you never guess an offset.",
		"Otherwise locate the relevant code with `list_directory`, `ripgrep`, `search_files`, or `list_symbols` before reading large files.",
		"Prefer search-first, then narrow reads: use `find_symbol`, `ripgrep`, `search_files`, or `list_symbols` to find exact files or symbols before any broad file read.",
		"Before changing or deleting a declaration, call `find_callers` to see what depends on it.",
		"`read_file` now returns a smaller bounded window by default; use offset_line to continue and use `read_file_range` for targeted spans.",
		"Prefer `read_file_range` once you know the relevant lines. Do not use `read_files` for broad repo exploration; reserve it for a few known files with small excerpts.",
		"Prefer `edit_file` for focused in-place changes and `apply_patch` for coordinated multi-file edits.",
		"Use `write_file` for new files or full rewrites only after you have read the current file state.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to contain %q\n%s", expected, prompt)
		}
	}
}

func TestBuildSystemPromptPlannerRunUsesReadOnlyRepoGuidance(t *testing.T) {
	systemPrompt := "You are a planner."

	prompt := BuildSystemPrompt(
		&model.Agent{
			Name:         "Planner",
			PresetKey:    model.AgentPresetTaskPlanner,
			SystemPrompt: &systemPrompt,
		},
		&model.PMTask{Name: "Plan inbox automation"},
		nil,
		nil,
		model.PlanningStageTaskPlanDoc,
		"",
		nil,
	)

	for _, expected := range []string{
		"Use the provided tools to inspect the repository and search for relevant context. Keep repository interactions read-only.",
		"This run is planning-only and read-only. Do not change code, create files, or alter git state.",
		"After approval, call `" + RuntimeToolNameForPrompt("ensure_task_plan_doc") + "` with `{}` to create or load and attach the canonical task planning document",
		"call `" + RuntimeToolNameForPrompt("write_document_content") + "` with the returned `document_id` and the full approved markdown",
		"Do not claim the planning document was persisted and do not finish the run until both product tool calls succeed.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected planner prompt to contain %q\n%s", expected, prompt)
		}
	}
	for _, unexpected := range []string{
		"Prefer edit_file for focused in-place changes and apply_patch for coordinated multi-file edits.",
		"Use write_file for new files or full rewrites only after you have read the current file state.",
	} {
		if strings.Contains(prompt, unexpected) {
			t.Fatalf("did not expect planner prompt to contain %q\n%s", unexpected, prompt)
		}
	}
}

func TestBuildSystemPromptSupportRunOmitsRepoEditingGuidance(t *testing.T) {
	systemPrompt := "You are a support agent."

	prompt := BuildSystemPrompt(
		&model.Agent{
			Name:         "Support",
			PresetKey:    model.AgentPresetSupportAgent,
			SystemPrompt: &systemPrompt,
		},
		nil,
		nil,
		&model.SupportConversation{Subject: "Login issue"},
		"",
		"",
		nil,
	)

	for _, unexpected := range []string{
		"Prefer edit_file for focused in-place changes and apply_patch for coordinated multi-file edits.",
		"Use write_file for new files or full rewrites only after you have read the current file state.",
	} {
		if strings.Contains(prompt, unexpected) {
			t.Fatalf("did not expect support prompt to contain %q\n%s", unexpected, prompt)
		}
	}
}

func TestBuildSystemPromptIncludesResolvedSkillInstructions(t *testing.T) {
	prompt := BuildSystemPrompt(
		&model.Agent{
			Name:                      "Custom Agent",
			ResolvedSkillInstructions: "Use the approval protocol skill instructions.",
		},
		nil,
		nil,
		nil,
		"",
		"",
		nil,
	)

	if !strings.Contains(prompt, "Use the approval protocol skill instructions.") {
		t.Fatalf("expected prompt to include resolved skill instructions\n%s", prompt)
	}
	if !strings.Contains(prompt, "You are Custom Agent, an AI coding agent.") {
		t.Fatalf("expected prompt to keep generic agent preamble\n%s", prompt)
	}
}

func TestBuildRuntimeSystemPromptSkipsBehaviorAndSkillTextWhenDisabled(t *testing.T) {
	systemPrompt := "Preset behavior instructions."
	prompt := BuildRuntimeSystemPrompt(
		&model.Agent{
			Name:                      "Custom Agent",
			SystemPrompt:              &systemPrompt,
			ResolvedSkillInstructions: "Resolved skill instructions.",
		},
		nil,
		nil,
		nil,
		"",
		"",
		nil,
		false,
		false,
	)

	if strings.Contains(prompt, "Resolved skill instructions.") {
		t.Fatalf("did not expect runtime prompt to include resolved skill instructions\n%s", prompt)
	}
	if !strings.Contains(prompt, "Preset behavior instructions.") {
		t.Fatalf("expected runtime prompt to preserve explicit system prompt identity\n%s", prompt)
	}
	if strings.Contains(prompt, "## Current Task") {
		t.Fatalf("did not expect runtime prompt to include target context\n%s", prompt)
	}
}

func TestBuildRuntimeSystemPromptDescribesAvailableSkillsWhenSkillTextDisabled(t *testing.T) {
	prompt := BuildRuntimeSystemPrompt(
		&model.Agent{
			Name:                      "Mira",
			PresetKey:                 model.AgentPresetMarketer,
			RuntimeKind:               "codex",
			Skills:                    model.AgentSkillRefs{{Key: "marketing_plan"}},
			ResolvedSkillInstructions: "Full marketer skill body should not be in the prompt.",
		},
		nil,
		nil,
		nil,
		"",
		"",
		nil,
		true,
		false,
	)

	if strings.Contains(prompt, "Full marketer skill body should not be in the prompt.") {
		t.Fatalf("did not expect runtime prompt to include full skill instructions\n%s", prompt)
	}
	for _, expected := range []string{
		"Available Skills",
		"Do not load every available skill by default.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected runtime prompt to include %q\n%s", expected, prompt)
		}
	}
}

func TestBuildRuntimeExecutionSupplementPromptOmitsArtifactContext(t *testing.T) {
	supplement := BuildRuntimeExecutionSupplementPrompt(
		&model.AgentRun{InvocationMode: model.InvocationModeInteractive},
		map[string]string{
			"target_id": "deal-123",
		},
	)

	for _, expected := range []string{
		"This is an interactive transcript that may resume after a human reply.",
		"emit a user-input handoff using the runtime-appropriate mechanism",
		"emit an approval or review handoff using the runtime-appropriate mechanism",
		"Durable run facts:",
		"- target_id=deal-123",
	} {
		if !strings.Contains(supplement, expected) {
			t.Fatalf("expected runtime supplement to contain %q\n%s", expected, supplement)
		}
	}
	for _, unexpected := range []string{
		"Current persisted artifacts:",
		"Use the latest persisted artifacts below as the current source of truth",
	} {
		if strings.Contains(supplement, unexpected) {
			t.Fatalf("did not expect runtime supplement to contain %q\n%s", unexpected, supplement)
		}
	}
}

func TestParseWorkflowConfigForAgent_UsesPlannerDefaultWhenFrontMatterOmitsMaxIterations(t *testing.T) {
	dir := t.TempDir()
	content := "---\ntimeout_minutes: 45\n---\nPlanner instructions"
	if err := os.WriteFile(filepath.Join(dir, "WORKFLOW.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write WORKFLOW.md: %v", err)
	}

	config := ParseWorkflowConfigForAgent(dir, &model.Agent{
		PresetKey:   model.AgentPresetEpicPlanner,
		RuntimeKind: "native_sdk",
	})
	if config == nil {
		t.Fatal("expected config")
	}
	if config.MaxIterations != plannerWorkflowMaxIterations {
		t.Fatalf("MaxIterations = %d, want %d", config.MaxIterations, plannerWorkflowMaxIterations)
	}
	if config.TimeoutMinutes != 45 {
		t.Fatalf("TimeoutMinutes = %d, want 45", config.TimeoutMinutes)
	}
}

func TestParseWorkflowConfigForAgent_RespectsExplicitMaxIterationsOverride(t *testing.T) {
	dir := t.TempDir()
	content := "---\nmax_iterations: 25\n---\nPlanner instructions"
	if err := os.WriteFile(filepath.Join(dir, "WORKFLOW.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write WORKFLOW.md: %v", err)
	}

	config := ParseWorkflowConfigForAgent(dir, &model.Agent{
		PresetKey:   model.AgentPresetTaskPlanner,
		RuntimeKind: "native_sdk",
	})
	if config == nil {
		t.Fatal("expected config")
	}
	if config.MaxIterations != 25 {
		t.Fatalf("MaxIterations = %d, want 25", config.MaxIterations)
	}
}
