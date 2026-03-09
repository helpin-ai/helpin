package worker

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestBuildSystemPromptStructuredPlanningUsesMethodologyPack(t *testing.T) {
	systemPrompt := "Prefer shorter drafts."
	planningNotes := "Prioritize support-ticket evidence."

	prompt := BuildSystemPrompt(
		&model.Agent{
			Name:          "Planner",
			AgentKind:     "llm",
			AgentClass:    model.AgentClassProductPlanner,
			SystemPrompt:  &systemPrompt,
			PlanningNotes: &planningNotes,
		},
		nil,
		&model.PMEpic{Name: "Billing refresh"},
		nil,
		model.PlanningStageDraftSpec,
		model.PlanningMethodologyStructuredV1,
		nil,
	)

	for _, expected := range []string{
		"Workspace planning methodology: structured_v1",
		"Think like an analyst first",
		"Think like a PM second",
		`"sources":[{"title":"..."`,
		"## Planner Notes",
		planningNotes,
		"## Advanced Planner Notes",
		"must not override the workflow stage requirements",
		"Do not embed a Research Sources section inside spec_markdown",
		"Return JSON only, with no markdown fences.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to contain %q\n%s", expected, prompt)
		}
	}
}

func TestBuildSystemPromptBasicPlanningKeepsSimpleGuidance(t *testing.T) {
	prompt := BuildSystemPrompt(
		&model.Agent{
			Name:       "Planner",
			AgentKind:  "llm",
			AgentClass: model.AgentClassProductPlanner,
		},
		nil,
		&model.PMEpic{Name: "Billing refresh"},
		nil,
		model.PlanningStagePlanStories,
		model.PlanningMethodologyBasicV1,
		nil,
	)

	if !strings.Contains(prompt, "Workspace planning methodology: basic_v1") {
		t.Fatalf("expected methodology marker in prompt\n%s", prompt)
	}
	if !strings.Contains(prompt, "Turn the approved spec into concrete stories.") {
		t.Fatalf("expected basic story guidance in prompt\n%s", prompt)
	}
	if strings.Contains(prompt, "Think like an architect first") {
		t.Fatalf("did not expect structured planning stance in basic methodology\n%s", prompt)
	}
}

func TestBuildSystemPromptNonEpicPreservesAgentSystemPrompt(t *testing.T) {
	systemPrompt := "You are a careful engineer."

	prompt := BuildSystemPrompt(
		&model.Agent{
			Name:         "Engineer",
			AgentKind:    "llm",
			AgentClass:   model.AgentClassEngineer,
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
