package worker

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestResolveNativeSupplementTransportMovesSupplementToTurnLocalForSelectivePath(t *testing.T) {
	systemPrompt, turnLocalInstructions, transport := resolveNativeSupplementTransport(
		&model.AgentRun{InvocationMode: model.InvocationModeInteractive},
		&ExecutionContext{
			NativeSelectivePathEnabled: true,
			TurnLocalInstructions:      "Existing turn-local contract.",
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
