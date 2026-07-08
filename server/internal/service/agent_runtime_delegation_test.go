package service

import (
	"fmt"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Agent Runtime is the only execution path: every agent/target combination
// delegates when AGENT_RUNTIME_LAUNCH_ENABLED is on. The per-surface predicate
// tables were retired with the local Temporal executor.
var delegationTargetTypes = []string{
	"workspace",
	"document",
	"crm_contact",
	"crm_company",
	"crm_deal",
	"task",
	"story",
	"epic",
	"repository",
	"support_conversation",
	"support_coverage_gap",
}

func TestDelegatesRunToAgentRuntimeDelegatesEveryAgentAndTarget(t *testing.T) {
	svc := &AgentService{agentRuntimeLaunchEnabled: true}
	agents := []struct {
		name  string
		agent *model.Agent
	}{
		{name: "marketer preset", agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetMarketer, RuntimeKind: "native_sdk"}},
		{name: "epic planner preset", agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetEpicPlanner, RuntimeKind: "native_sdk"}},
		{name: "task planner preset", agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetTaskPlanner, RuntimeKind: "native_sdk"}},
		{name: "code builder preset", agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetCodeBuilder, RuntimeKind: "codex"}},
		{name: "support agent preset", agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetSupportAgent, RuntimeKind: "native_sdk"}},
		{name: "command agent preset", agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetCommandAgent, RuntimeKind: "native_sdk"}},
		{name: "system agent without preset", agent: &model.Agent{IsSystem: true, RuntimeKind: "native_sdk"}},
		{name: "custom native_sdk agent", agent: &model.Agent{Name: "Custom Runner", RuntimeKind: "native_sdk"}},
		{name: "custom codex agent", agent: &model.Agent{Name: "Codex Runner", RuntimeKind: "codex"}},
		{name: "custom opencode agent", agent: &model.Agent{Name: "Opencode Runner", RuntimeKind: "opencode"}},
		{name: "custom agent with empty runtime kind", agent: &model.Agent{Name: "Unset Runtime"}},
	}
	for _, tt := range agents {
		for _, targetType := range delegationTargetTypes {
			t.Run(fmt.Sprintf("%s/%s", tt.name, targetType), func(t *testing.T) {
				if !svc.delegatesRunToAgentRuntime(tt.agent, targetType) {
					t.Errorf("delegatesRunToAgentRuntime(%s) = false, want true", targetType)
				}
			})
		}
	}
}

func TestDelegatesRunToAgentRuntimeRejectsNilAgentAndEmptyTarget(t *testing.T) {
	svc := &AgentService{agentRuntimeLaunchEnabled: true}
	if svc.delegatesRunToAgentRuntime(nil, "workspace") {
		t.Fatal("nil agent must not delegate")
	}
	if svc.delegatesRunToAgentRuntime(&model.Agent{Name: "Runner"}, "") {
		t.Fatal("empty target type must not delegate")
	}
	if svc.delegatesRunToAgentRuntime(&model.Agent{Name: "Runner"}, "  ") {
		t.Fatal("blank target type must not delegate")
	}
}

func TestDelegatesRunToAgentRuntimeFlagShortCircuit(t *testing.T) {
	agent := &model.Agent{PresetKey: model.AgentPresetMarketer}

	flagOff := &AgentService{}
	if flagOff.delegatesRunToAgentRuntime(agent, "workspace") {
		t.Fatal("delegation must short-circuit when AGENT_RUNTIME_LAUNCH_ENABLED is off")
	}

	flagOn := &AgentService{agentRuntimeLaunchEnabled: true}
	if !flagOn.delegatesRunToAgentRuntime(agent, "workspace") {
		t.Fatal("flag on should delegate")
	}

	var nilService *AgentService
	if nilService.delegatesRunToAgentRuntime(agent, "workspace") {
		t.Fatal("nil service must not delegate")
	}
}
