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

func TestResolveAgentProfileTreatsReadFilesAsRepositoryAccess(t *testing.T) {
	resolved := ResolveAgentProfile(&model.Agent{
		RuntimeKind:  "native_sdk",
		AllowedTools: []byte(`["read_files"]`),
	})

	if !resolved.RequiresRepo {
		t.Fatal("read_files must require repository access")
	}
}

func TestResolveAgentProfileGivesExternalA2AAgentsNoToolsOrRepo(t *testing.T) {
	// Connected agents are stored with empty tool lists, which must not fall
	// back to the default profile's repository tools.
	resolved := ResolveAgentProfile(&model.Agent{
		RuntimeKind:     model.AgentRuntimeKindA2A,
		AllowedTools:    []byte(`[]`),
		AllowedCommands: []byte(`[]`),
		AllowedTargets:  []byte(`["task"]`),
		ApprovalMode:    "never",
	})

	if resolved.RequiresRepo {
		t.Fatal("external A2A agents must not require a repository")
	}
	if len(resolved.Tools) != 0 || len(resolved.Commands) != 0 {
		t.Fatalf("external A2A agents get no Helpin tools, got tools %v commands %v", resolved.Tools, resolved.Commands)
	}
	if resolved.Queue != QueueForRuntime(model.AgentRuntimeKindA2A, model.InvocationModeAutonomous) {
		t.Fatalf("unexpected queue %q", resolved.Queue)
	}
	if !slices.Equal(resolved.TargetTypes, []string{"task"}) {
		t.Fatalf("unexpected targets %v", resolved.TargetTypes)
	}
}

func TestResolveApprovalStateByMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want string
	}{
		{name: "no approval", mode: "never", want: "not_required"},
		{name: "risk based", mode: "risk_based", want: "not_required"},
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

func TestResolveAgentProfileUsesCodingQueue(t *testing.T) {
	agent := &model.Agent{
		PresetKey:   model.AgentPresetCodeBuilder,
		RuntimeKind: "native_sdk",
	}

	resolved := ResolveAgentProfile(agent, model.InvocationModeAutonomous)

	if resolved.Queue != "agent-native-coding" {
		t.Fatalf("expected queue %q, got %q", "agent-native-coding", resolved.Queue)
	}
}

func TestResolveAgentProfileUsesInteractiveCodingQueue(t *testing.T) {
	agent := &model.Agent{
		PresetKey:   model.AgentPresetCodeBuilder,
		RuntimeKind: "native_sdk",
	}

	resolved := ResolveAgentProfile(agent, model.InvocationModeInteractive)

	if resolved.Queue != "agent-native-coding" {
		t.Fatalf("expected queue %q, got %q", "agent-native-coding", resolved.Queue)
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

func TestResolveAgentProfileDoesNotAddSkillToolsWithoutAvailableSkills(t *testing.T) {
	agent := &model.Agent{
		PresetKey:   model.AgentPresetCodeBuilder,
		RuntimeKind: "native_sdk",
	}

	resolved := ResolveAgentProfile(agent, model.InvocationModeAutonomous)

	for _, toolName := range []string{ToolListAvailableSkills, ToolSearchAvailableSkills, ToolReadSkill} {
		if slices.Contains(resolved.Tools, toolName) {
			t.Fatalf("did not expect tools to include an unavailable skill tool %q, got %v", toolName, resolved.Tools)
		}
	}
}
