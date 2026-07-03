package service

import (
	"fmt"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// delegationExpectations maps target type → whether shouldDelegateRunToAgentRuntime
// must return true for a given agent. Target types absent from every map are
// asserted false via the shared target list below.
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

func TestShouldDelegateRunToAgentRuntimeMatrix(t *testing.T) {
	tests := []struct {
		name      string
		agent     *model.Agent
		delegated map[string]bool
	}{
		{
			name:      "nil agent",
			agent:     nil,
			delegated: nil,
		},
		{
			name:  "marketer preset delegates workspace only",
			agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetMarketer, RuntimeKind: "native_sdk"},
			delegated: map[string]bool{
				"workspace": true,
			},
		},
		{
			name:  "documentation agent preset delegates workspace and document",
			agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetDocumentationAgent, RuntimeKind: "native_sdk"},
			delegated: map[string]bool{
				"workspace": true,
				"document":  true,
			},
		},
		{
			name:  "crm operator preset delegates workspace and crm targets",
			agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetCRMOperator, RuntimeKind: "native_sdk"},
			delegated: map[string]bool{
				"workspace":   true,
				"crm_contact": true,
				"crm_company": true,
				"crm_deal":    true,
			},
		},
		{
			name:      "code builder preset never delegates",
			agent:     &model.Agent{IsSystem: true, PresetKey: model.AgentPresetCodeBuilder, RuntimeKind: "native_sdk"},
			delegated: nil,
		},
		{
			name:      "review agent preset never delegates",
			agent:     &model.Agent{IsSystem: true, PresetKey: model.AgentPresetReviewAgent, RuntimeKind: "native_sdk"},
			delegated: nil,
		},
		{
			name:      "epic planner preset never delegates",
			agent:     &model.Agent{IsSystem: true, PresetKey: model.AgentPresetEpicPlanner, RuntimeKind: "native_sdk"},
			delegated: nil,
		},
		{
			name:      "task planner preset never delegates",
			agent:     &model.Agent{IsSystem: true, PresetKey: model.AgentPresetTaskPlanner, RuntimeKind: "native_sdk"},
			delegated: nil,
		},
		{
			// support_coverage_gap must stay false: it is not an allowed
			// support-agent target and stays on the local executor.
			name:  "support agent preset delegates support_conversation only",
			agent: &model.Agent{IsSystem: true, PresetKey: model.AgentPresetSupportAgent, RuntimeKind: "native_sdk"},
			delegated: map[string]bool{
				"support_conversation": true,
			},
		},
		{
			name:      "command agent preset never delegates",
			agent:     &model.Agent{IsSystem: true, PresetKey: model.AgentPresetCommandAgent, RuntimeKind: "native_sdk"},
			delegated: nil,
		},
		{
			name:  "custom native_sdk agent delegates workspace, document, and crm targets",
			agent: &model.Agent{Name: "Custom Runner", RuntimeKind: "native_sdk"},
			delegated: map[string]bool{
				"workspace":   true,
				"document":    true,
				"crm_contact": true,
				"crm_company": true,
				"crm_deal":    true,
			},
		},
		{
			// Codex custom agents stay on the local Temporal executor: their
			// auth flows and interaction schema handling still live there.
			name:      "custom codex agent never delegates",
			agent:     &model.Agent{Name: "Codex Runner", RuntimeKind: "codex"},
			delegated: nil,
		},
		{
			name:      "custom opencode agent never delegates",
			agent:     &model.Agent{Name: "Opencode Runner", RuntimeKind: "opencode"},
			delegated: nil,
		},
		{
			// The DB default runtime kind is opencode; an unset value must not
			// be treated as native_sdk.
			name:      "custom agent with empty runtime kind never delegates",
			agent:     &model.Agent{Name: "Unset Runtime"},
			delegated: nil,
		},
		{
			// Agent named after a system agent but without a preset routes by
			// the custom-agent rules, not by name.
			name:      "custom agent named Mira without native_sdk does not delegate",
			agent:     &model.Agent{Name: "Mira", RuntimeKind: "opencode"},
			delegated: nil,
		},
		{
			name:      "system agent without preset never delegates",
			agent:     &model.Agent{IsSystem: true, RuntimeKind: "native_sdk"},
			delegated: nil,
		},
		{
			// Custom agents cloned from a delegated preset route by preset,
			// regardless of runtime kind or is_system.
			name:  "non-system marketer-preset agent delegates workspace only",
			agent: &model.Agent{PresetKey: model.AgentPresetMarketer, RuntimeKind: "codex"},
			delegated: map[string]bool{
				"workspace": true,
			},
		},
	}

	for _, tt := range tests {
		for _, targetType := range delegationTargetTypes {
			want := tt.delegated[targetType]
			t.Run(fmt.Sprintf("%s/%s", tt.name, targetType), func(t *testing.T) {
				if got := shouldDelegateRunToAgentRuntime(tt.agent, targetType); got != want {
					t.Errorf("shouldDelegateRunToAgentRuntime(%s) = %v, want %v", targetType, got, want)
				}
			})
		}
	}
}

func TestShouldDelegateRunToAgentRuntimeTrimsTargetType(t *testing.T) {
	agent := &model.Agent{PresetKey: model.AgentPresetMarketer}
	if !shouldDelegateRunToAgentRuntime(agent, " workspace ") {
		t.Fatal("target type should be trimmed before matching")
	}
	if shouldDelegateRunToAgentRuntime(agent, "") {
		t.Fatal("empty target type should not delegate")
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
		t.Fatal("flag on plus matching predicate should delegate")
	}
	if flagOn.delegatesRunToAgentRuntime(agent, "task") {
		t.Fatal("flag on must still respect the predicate")
	}

	var nilService *AgentService
	if nilService.delegatesRunToAgentRuntime(agent, "workspace") {
		t.Fatal("nil service must not delegate")
	}
}
