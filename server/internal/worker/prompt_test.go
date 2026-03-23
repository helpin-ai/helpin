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
			Name:       "Planner",
			PresetKey:  model.AgentPresetEpicPlanner,
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
