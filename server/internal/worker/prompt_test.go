package worker

import (
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

func TestBuildSystemPromptLegacyPlanningStageKeepsMethodologyPack(t *testing.T) {
	prompt := BuildSystemPrompt(
		&model.Agent{
			Name:      "Planner",
			PresetKey: model.AgentPresetEpicPlanner,
		},
		nil,
		&model.PMEpic{Name: "Billing refresh"},
		nil,
		model.PlanningStagePlanStories,
		model.PlanningMethodologyBasicV1,
		nil,
	)

	if !strings.Contains(prompt, "Planning methodology: basic_v1") {
		t.Fatalf("expected methodology marker in prompt\n%s", prompt)
	}
	if !strings.Contains(prompt, "Turn the approved spec into concrete stories.") {
		t.Fatalf("expected basic story guidance in prompt\n%s", prompt)
	}
	if strings.Contains(prompt, "Think like an architect first") {
		t.Fatalf("did not expect structured planning stance in basic methodology\n%s", prompt)
	}
}

func TestBuildUserPromptDirectEpicRunIsContextOnly(t *testing.T) {
	prompt := BuildUserPrompt(
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

	if !strings.Contains(prompt, "Please work on epic: **Billing refresh**") {
		t.Fatalf("expected epic context in prompt\n%s", prompt)
	}
	for _, unexpected := range []string{
		"Use the planner tools to create or update the canonical PRD",
		"Please plan and execute the epic setup directly",
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

func TestBuildExecutionSupplementPromptIncludesResumeGuidanceAndArtifacts(t *testing.T) {
	supplement := BuildExecutionSupplementPrompt(
		&model.AgentRun{InvocationMode: model.InvocationModeInteractive},
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
		"Use the latest persisted artifacts below as the current source of truth",
		"Latest draft body",
	} {
		if !strings.Contains(supplement, marker) {
			t.Fatalf("expected supplement to contain %q\n%s", marker, supplement)
		}
	}
}

func TestProviderSupportsResponseContinuation(t *testing.T) {
	if !ProviderSupportsResponseContinuation(model.AgentModelProviderOpenAI) {
		t.Fatal("expected openai to support response continuation")
	}
	for _, provider := range []string{
		model.AgentModelProviderOpenRouter,
		model.AgentModelProviderOpenRouterResponses,
		model.AgentModelProviderAnthropic,
	} {
		if ProviderSupportsResponseContinuation(provider) {
			t.Fatalf("expected provider %q not to support response continuation", provider)
		}
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
		&model.PMStory{Name: "Implement feature flag"},
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
		&model.PMStory{Name: "Implement feature flag"},
		nil,
		nil,
		"",
		"",
		nil,
	)

	for _, expected := range []string{
		"Start by locating the relevant code with list_directory, ripgrep, search_files, or list_symbols before reading large files.",
		"read_file now returns a bounded window by default; use offset_line to continue and use read_file_range for targeted spans.",
		"Prefer edit_file for focused in-place changes and apply_patch for coordinated multi-file edits.",
		"Use write_file for new files or full rewrites only after you have read the current file state.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to contain %q\n%s", expected, prompt)
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
