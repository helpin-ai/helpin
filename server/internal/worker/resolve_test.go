package worker

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestResolveAgentProfileUsesInteractiveNativeQueue(t *testing.T) {
	agent := &model.Agent{
		PresetKey:   model.AgentPresetEpicPlanner,
		RuntimeKind: "native_sdk",
	}

	resolved := ResolveAgentProfile(agent, model.InvocationModeInteractive)

	if resolved.Queue != "agent-native-interactive" {
		t.Fatalf("expected queue %q, got %q", "agent-native-interactive", resolved.Queue)
	}
}

func TestResolveAgentProfileUsesOpenCodeQueue(t *testing.T) {
	agent := &model.Agent{
		PresetKey:   model.AgentPresetCodeBuilder,
		RuntimeKind: "opencode",
	}

	resolved := ResolveAgentProfile(agent, model.InvocationModeAutonomous)

	if resolved.Queue != "agent-opencode-autonomous" {
		t.Fatalf("expected queue %q, got %q", "agent-opencode-autonomous", resolved.Queue)
	}
}

func TestResolveAgentProfileUsesCodexQueue(t *testing.T) {
	agent := &model.Agent{
		PresetKey:   model.AgentPresetCodeBuilder,
		RuntimeKind: "codex",
	}

	resolved := ResolveAgentProfile(agent, model.InvocationModeAutonomous)

	if resolved.Queue != "agent-codex-autonomous" {
		t.Fatalf("expected queue %q, got %q", "agent-codex-autonomous", resolved.Queue)
	}
}

func TestResolveAgentProfileUsesInteractiveCodexQueue(t *testing.T) {
	agent := &model.Agent{
		PresetKey:   model.AgentPresetCodeBuilder,
		RuntimeKind: "codex",
	}

	resolved := ResolveAgentProfile(agent, model.InvocationModeInteractive)

	if resolved.Queue != "agent-codex-interactive" {
		t.Fatalf("expected queue %q, got %q", "agent-codex-interactive", resolved.Queue)
	}
}
