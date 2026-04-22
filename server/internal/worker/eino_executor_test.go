package worker

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestResolveNativeSystemPromptSuppressesResolvedSkillTextForSelectivePath(t *testing.T) {
	systemPrompt, includesResolvedSkillText := resolveNativeSystemPrompt(&ExecutionContext{
		Agent: &model.Agent{
			Name:                      "Planner",
			PresetKey:                 model.AgentPresetEpicPlanner,
			ResolvedSkillInstructions: "Full aggregated skill blob.",
		},
		Epic:                       &model.PMEpic{Name: "Billing refresh"},
		NativeSelectivePathEnabled: true,
	}, nil)

	if includesResolvedSkillText {
		t.Fatal("did not expect selective native system prompt to include resolved skill text")
	}
	if strings.Contains(systemPrompt, "Full aggregated skill blob.") {
		t.Fatalf("did not expect selective native system prompt to inline full resolved skill instructions\n%s", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "You are Epic Planner.") {
		t.Fatalf("expected selective native system prompt to preserve base preset identity\n%s", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "## Current Epic") {
		t.Fatalf("expected selective native system prompt to preserve target context\n%s", systemPrompt)
	}
}

func TestResolveNativeSystemPromptKeepsResolvedSkillTextForLegacyPath(t *testing.T) {
	systemPrompt, includesResolvedSkillText := resolveNativeSystemPrompt(&ExecutionContext{
		Agent: &model.Agent{
			Name:                      "Planner",
			PresetKey:                 model.AgentPresetEpicPlanner,
			ResolvedSkillInstructions: "Full aggregated skill blob.",
		},
	}, nil)

	if !includesResolvedSkillText {
		t.Fatal("expected legacy native system prompt to include resolved skill text")
	}
	if !strings.Contains(systemPrompt, "Full aggregated skill blob.") {
		t.Fatalf("expected legacy native system prompt to keep resolved skill instructions\n%s", systemPrompt)
	}
}

func TestResolveNativeSupplementTransportMovesSupplementToTurnLocalForSelectivePath(t *testing.T) {
	systemPrompt, turnLocalInstructions, transport := resolveNativeSupplementTransport(
		&model.AgentRun{InvocationMode: model.InvocationModeInteractive},
		&ExecutionContext{
			NativeSelectivePathEnabled: true,
			TurnLocalInstructions:      "Existing turn-local contract.",
			ActiveSkillInstructions:    "Use only the PRD-related contracts.",
			RunFacts: map[string]string{
				"target_id": "epic-123",
			},
			ArtifactContext: &ArtifactContext{
				Entries: []ArtifactContextEntry{
					{
						Label:   "Current preview for prd_draft",
						Source:  "run_preview",
						Status:  "draft",
						Format:  "markdown",
						Content: "# PRD\n\nLatest draft body",
					},
				},
			},
		},
		"Base system prompt",
	)

	if systemPrompt != "Base system prompt" {
		t.Fatalf("expected selective path to leave system prompt unchanged, got %q", systemPrompt)
	}
	if transport != "turn_local" {
		t.Fatalf("expected turn_local supplement transport, got %q", transport)
	}
	if !strings.Contains(turnLocalInstructions, "Existing turn-local contract.") {
		t.Fatalf("expected existing turn-local instructions to be preserved, got %q", turnLocalInstructions)
	}
	if !strings.Contains(turnLocalInstructions, "Active skill instructions for this turn:") {
		t.Fatalf("expected active skill instructions in turn-local transport, got %q", turnLocalInstructions)
	}
	if !strings.Contains(turnLocalInstructions, "Use only the PRD-related contracts.") {
		t.Fatalf("expected active skill contract text in turn-local transport, got %q", turnLocalInstructions)
	}
	if !strings.Contains(turnLocalInstructions, "This is an interactive transcript that may resume after a human reply.") {
		t.Fatalf("expected execution supplement to move into turn-local instructions, got %q", turnLocalInstructions)
	}
	if !strings.Contains(turnLocalInstructions, "Latest draft body") {
		t.Fatalf("expected artifact context to remain present in turn-local instructions, got %q", turnLocalInstructions)
	}
}

func TestResolveNativeSupplementTransportKeepsSupplementInSystemPromptForLegacyPath(t *testing.T) {
	systemPrompt, turnLocalInstructions, transport := resolveNativeSupplementTransport(
		&model.AgentRun{InvocationMode: model.InvocationModeInteractive},
		&ExecutionContext{
			RunFacts: map[string]string{
				"target_id": "epic-123",
			},
		},
		"Base system prompt",
	)

	if !strings.Contains(systemPrompt, "## Current Run State") {
		t.Fatalf("expected legacy path to append supplement into system prompt, got %q", systemPrompt)
	}
	if transport != "system" {
		t.Fatalf("expected system supplement transport, got %q", transport)
	}
	if !strings.Contains(systemPrompt, "target_id=epic-123") {
		t.Fatalf("expected durable run facts in legacy system prompt supplement, got %q", systemPrompt)
	}
	if strings.TrimSpace(turnLocalInstructions) != "" {
		t.Fatalf("expected legacy path to leave turn-local instructions empty, got %q", turnLocalInstructions)
	}
}
