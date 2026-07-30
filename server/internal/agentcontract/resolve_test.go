package agentcontract

import (
	"slices"
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

func TestResolveApprovalStateByMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want string
	}{
		{name: "no approval", mode: "never", want: "not_required"},
		{name: "mutating tools", mode: "mutating_tools", want: "not_required"},
		{name: "before run and tools", mode: "always", want: "pending"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveApprovalState(ResolvedProfile{ApprovalMode: tt.mode}); got != tt.want {
				t.Fatalf("ResolveApprovalState() = %q, want %q", got, tt.want)
			}
		})
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

func TestResolveAgentProfileAddsAvailableSkillToolsForNativeSkillAgent(t *testing.T) {
	agent := &model.Agent{
		IsSystem:         true,
		PresetKey:        model.AgentPresetMarketer,
		PresetVersionKey: "marketer_default",
		RuntimeKind:      "native_sdk",
		AllowedTools:     []byte(`["list_documents"]`),
	}

	resolved := ResolveAgentProfile(agent, model.InvocationModeInteractive)

	for _, toolName := range []string{ToolListAvailableSkills, ToolSearchAvailableSkills, ToolReadSkill} {
		if !slices.Contains(resolved.Tools, toolName) {
			t.Fatalf("expected native skill agent tools to include %q, got %v", toolName, resolved.Tools)
		}
	}
}

func TestResolveAgentProfileDoesNotAddAvailableSkillToolsForWorkspaceVersionWithNoSkills(t *testing.T) {
	agent := &model.Agent{
		IsSystem:         true,
		PresetKey:        model.AgentPresetMarketer,
		PresetVersionKey: "marketer_workspace_123",
		RuntimeKind:      "native_sdk",
		AllowedTools:     []byte(`["list_documents"]`),
	}

	resolved := ResolveAgentProfile(agent, model.InvocationModeInteractive)

	for _, toolName := range []string{ToolListAvailableSkills, ToolSearchAvailableSkills, ToolReadSkill} {
		if slices.Contains(resolved.Tools, toolName) {
			t.Fatalf("did not expect workspace version with empty skills to include %q, got %v", toolName, resolved.Tools)
		}
	}
}

func TestResolveAgentProfileDoesNotAddNativeAvailableSkillToolsForCodexAgent(t *testing.T) {
	agent := &model.Agent{
		PresetKey:   model.AgentPresetCodeBuilder,
		RuntimeKind: "codex",
	}

	resolved := ResolveAgentProfile(agent, model.InvocationModeAutonomous)

	for _, toolName := range []string{ToolListAvailableSkills, ToolSearchAvailableSkills, ToolReadSkill} {
		if slices.Contains(resolved.Tools, toolName) {
			t.Fatalf("did not expect codex tools to include native available-skill tool %q, got %v", toolName, resolved.Tools)
		}
	}
}
