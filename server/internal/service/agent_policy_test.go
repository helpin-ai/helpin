package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestValidateAgentTargetEnforcesPresetTargetMapping(t *testing.T) {
	tests := []struct {
		name      string
		agent     model.Agent
		target    string
		shouldErr bool
	}{
		{
			name:   "epic planner can run on epics",
			agent:  model.Agent{PresetKey: model.AgentPresetEpicPlanner},
			target: "epic",
		},
		{
			name:   "epic planner can run on stories",
			agent:  model.Agent{PresetKey: model.AgentPresetEpicPlanner},
			target: "story",
		},
		{
			name:   "epic planner can run on crm deals",
			agent:  model.Agent{PresetKey: model.AgentPresetEpicPlanner},
			target: "crm_deal",
		},
		{
			name:   "code builder can run on stories",
			agent:  model.Agent{PresetKey: model.AgentPresetCodeBuilder},
			target: "story",
		},
		{
			name:      "code builder cannot run on epics",
			agent:     model.Agent{PresetKey: model.AgentPresetCodeBuilder},
			target:    "epic",
			shouldErr: true,
		},
		{
			name:   "review agent can run on stories",
			agent:  model.Agent{PresetKey: model.AgentPresetReviewAgent},
			target: "story",
		},
		{
			name:   "support agent can run on support conversations",
			agent:  model.Agent{PresetKey: model.AgentPresetSupportAgent},
			target: "support_conversation",
		},
		{
			name:      "unknown preset is not runnable",
			agent:     model.Agent{PresetKey: "unknown"},
			target:    "story",
			shouldErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAgentTarget(&tc.agent, tc.target)
			if tc.shouldErr && err == nil {
				t.Fatalf("expected error for target %q and preset %q", tc.target, tc.agent.PresetKey)
			}
			if !tc.shouldErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeAgentRecordDefaultsToPreset(t *testing.T) {
	agent := &model.Agent{
		TriggerMode: "manual",
	}

	normalizeAgentRecord(agent)

	if agent.PresetKey != model.AgentPresetCodeBuilder {
		t.Fatalf("expected blank agent to default to %q, got %q", model.AgentPresetCodeBuilder, agent.PresetKey)
	}
	if agent.RuntimeKind != "opencode" {
		t.Fatalf("expected default runtime opencode, got %q", agent.RuntimeKind)
	}
}

func TestAgentDefaultsDerivedFromPreset(t *testing.T) {
	if got := defaultRoleForPresetKey(model.AgentPresetReviewAgent); got != "Review Agent" {
		t.Fatalf("expected review preset role label, got %q", got)
	}
	if got := defaultRuntimeKindForPresetKey(model.AgentPresetEpicPlanner); got != "native_sdk" {
		t.Fatalf("expected planner runtime default native_sdk, got %q", got)
	}
	if got := defaultRuntimeKindForPresetKey(model.AgentPresetSupportAgent); got != "native_sdk" {
		t.Fatalf("expected support runtime default native_sdk, got %q", got)
	}
}

func TestPresetDefinitionForAgentDefaultsFromSystemFlag(t *testing.T) {
	systemAgent := &model.Agent{IsSystem: true}
	preset, ok := presetDefinitionForAgent(systemAgent)
	if !ok {
		t.Fatal("expected preset resolution for system agent")
	}
	if preset.Key != model.AgentPresetEpicPlanner {
		t.Fatalf("expected epic planner preset, got %q", preset.Key)
	}

	regularAgent := &model.Agent{}
	preset, ok = presetDefinitionForAgent(regularAgent)
	if !ok {
		t.Fatal("expected preset resolution for regular agent")
	}
	if preset.Key != model.AgentPresetCodeBuilder {
		t.Fatalf("expected code builder preset, got %q", preset.Key)
	}
}

func TestListAgentPresetsIncludesEpicPlanner(t *testing.T) {
	presets := ListAgentPresets()
	if len(presets) == 0 {
		t.Fatal("expected preset catalog")
	}

	found := false
	for _, preset := range presets {
		if preset.Key != model.AgentPresetEpicPlanner {
			continue
		}
		found = true
		if preset.RuntimeKind != "native_sdk" {
			t.Fatalf("expected epic planner runtime native_sdk, got %q", preset.RuntimeKind)
		}
		if preset.DefaultInvocationMode != model.InvocationModeInteractive {
			t.Fatalf("expected epic planner default mode interactive, got %q", preset.DefaultInvocationMode)
		}
	}
	if !found {
		t.Fatal("expected epic planner preset in catalog")
	}
}

func TestValidateRuntimeKindAllowsOnlyImplementedRuntimes(t *testing.T) {
	valid := []string{"opencode", "native_sdk"}
	for _, runtimeKind := range valid {
		if err := validateRuntimeKind(runtimeKind); err != nil {
			t.Fatalf("expected runtime %q to be valid, got %v", runtimeKind, err)
		}
	}

	invalid := []string{"native_claude", "claude_code", "openclaw", "zeroclaw"}
	for _, runtimeKind := range invalid {
		if err := validateRuntimeKind(runtimeKind); err == nil {
			t.Fatalf("expected runtime %q to be rejected", runtimeKind)
		}
	}
}

func TestAgentSupportsInteractiveRequiresNativeSDK(t *testing.T) {
	tests := []struct {
		name      string
		agent     model.Agent
		supported bool
	}{
		{
			name: "native sdk planner supports interactive",
			agent: model.Agent{
				PresetKey:   model.AgentPresetEpicPlanner,
				RuntimeKind: "native_sdk",
			},
			supported: true,
		},
		{
			name: "native sdk code builder supports interactive",
			agent: model.Agent{
				PresetKey:   model.AgentPresetCodeBuilder,
				RuntimeKind: "native_sdk",
			},
			supported: true,
		},
		{
			name: "opencode does not support interactive",
			agent: model.Agent{
				PresetKey:   model.AgentPresetCodeBuilder,
				RuntimeKind: "opencode",
			},
			supported: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			normalizeAgentRecord(&tc.agent)
			got := agentSupportsMode(&tc.agent, model.InvocationModeInteractive)
			if got != tc.supported {
				t.Fatalf("expected interactive support=%v, got %v (runtime=%q)", tc.supported, got, tc.agent.RuntimeKind)
			}
		})
	}
}

func TestNormalizeAgentRecordClearsPlannerOnlyFieldsForNonEpicPlanner(t *testing.T) {
	planningNotes := "use specs first"
	systemPrompt := "do the work"
	budget := 100
	agent := &model.Agent{
		PresetKey:           model.AgentPresetCodeBuilder,
		RuntimeKind:         "",
		PlanningNotes:       &planningNotes,
		SystemPrompt:        &systemPrompt,
		MonthlyTokenBudget:  &budget,
	}

	normalizeAgentRecord(agent)

	if agent.PlanningNotes != nil {
		t.Fatalf("expected non-planner normalization to clear planning notes, got %+v", agent.PlanningNotes)
	}
	if agent.SystemPrompt == nil || *agent.SystemPrompt != systemPrompt {
		t.Fatalf("expected system prompt to be preserved, got %+v", agent.SystemPrompt)
	}
	if agent.MonthlyTokenBudget == nil || *agent.MonthlyTokenBudget != budget {
		t.Fatalf("expected token budget to be preserved, got %+v", agent.MonthlyTokenBudget)
	}
	if agent.RuntimeKind == "" {
		t.Fatal("expected runtime kind default to be populated")
	}
}
