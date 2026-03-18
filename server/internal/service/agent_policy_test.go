package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestValidateAgentTargetEnforcesOpinionatedTargetMapping(t *testing.T) {
	tests := []struct {
		name      string
		agent     model.Agent
		target    string
		shouldErr bool
	}{
		{
			name:   "product planner can run on epics",
			agent:  model.Agent{AgentKind: "llm", AgentClass: model.AgentClassProductPlanner},
			target: "epic",
		},
		{
			name:   "product planner can run on stories",
			agent:  model.Agent{AgentKind: "llm", AgentClass: model.AgentClassProductPlanner},
			target: "story",
		},
		{
			name:   "product planner can run on crm deals",
			agent:  model.Agent{AgentKind: "llm", AgentClass: model.AgentClassProductPlanner},
			target: "crm_deal",
		},
		{
			name:   "engineer can run on stories",
			agent:  model.Agent{AgentKind: "llm", AgentClass: model.AgentClassEngineer},
			target: "story",
		},
		{
			name:      "engineer cannot run on epics",
			agent:     model.Agent{AgentKind: "llm", AgentClass: model.AgentClassEngineer},
			target:    "epic",
			shouldErr: true,
		},
		{
			name:   "reviewer can run on stories",
			agent:  model.Agent{AgentKind: "llm", AgentClass: model.AgentClassReviewer},
			target: "story",
		},
		{
			name:   "support can run on support conversations",
			agent:  model.Agent{AgentKind: "llm", AgentClass: model.AgentClassSupport},
			target: "support_conversation",
		},
		{
			name:      "human cannot run anywhere",
			agent:     model.Agent{AgentKind: "human", AgentClass: model.AgentClassHuman},
			target:    "story",
			shouldErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAgentTarget(&tc.agent, tc.target)
			if tc.shouldErr && err == nil {
				t.Fatalf("expected error for target %q and agent class %q", tc.target, tc.agent.AgentClass)
			}
			if !tc.shouldErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeAgentRecordMapsLegacyAliases(t *testing.T) {
	agent := &model.Agent{
		AgentKind:         "llm",
		CapabilityProfile: "orchestrator",
		TriggerMode:       "manual",
	}

	normalizeAgentRecord(agent)

	if agent.AgentClass != model.AgentClassProductPlanner {
		t.Fatalf("expected orchestrator alias to normalize to %q, got %q", model.AgentClassProductPlanner, agent.AgentClass)
	}
	if agent.CapabilityProfile != model.AgentClassProductPlanner {
		t.Fatalf("expected capability profile to normalize to %q, got %q", model.AgentClassProductPlanner, agent.CapabilityProfile)
	}
}

func TestNormalizeAgentRecordPreservesExplicitCustomCapabilityProfile(t *testing.T) {
	agent := &model.Agent{
		AgentKind:         "llm",
		AgentClass:        model.AgentClassProductPlanner,
		CapabilityProfile: "planner_with_docs_focus",
		TriggerMode:       "manual",
	}

	normalizeAgentRecord(agent)

	if agent.CapabilityProfile != "planner_with_docs_focus" {
		t.Fatalf("expected explicit capability profile to be preserved, got %q", agent.CapabilityProfile)
	}
}

func TestResolvePlanningMethodologyDefaults(t *testing.T) {
	svc := &AgentService{}

	if got := svc.resolvePlanningMethodology(context.Background(), "ws_123", ""); got != model.PlanningMethodologyStructuredV1 {
		t.Fatalf("expected default methodology %q, got %q", model.PlanningMethodologyStructuredV1, got)
	}
	if got := svc.resolvePlanningMethodology(context.Background(), "ws_123", model.PlanningMethodologyBasicV1); got != model.PlanningMethodologyBasicV1 {
		t.Fatalf("expected explicit methodology %q, got %q", model.PlanningMethodologyBasicV1, got)
	}
}

func TestAgentDefaultsDerivedFromClass(t *testing.T) {
	if got := agentKindForAgentClass(model.AgentClassProductPlanner); got != "llm" {
		t.Fatalf("expected product planner agent kind llm, got %q", got)
	}
	if got := agentKindForAgentClass(model.AgentClassHuman); got != "human" {
		t.Fatalf("expected human agent kind human, got %q", got)
	}
	if got := defaultRoleForAgentClass(model.AgentClassReviewer); got != "Reviewer" {
		t.Fatalf("expected reviewer role label, got %q", got)
	}
	if got := defaultRuntimeKindForAgentClass(model.AgentClassProductPlanner); got != "native_sdk" {
		t.Fatalf("expected planner runtime default native_sdk, got %q", got)
	}
	if got := defaultRuntimeKindForAgentClass(model.AgentClassSupport); got != "native_sdk" {
		t.Fatalf("expected support runtime default native_sdk, got %q", got)
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
			name: "native sdk supports interactive",
			agent: model.Agent{
				AgentKind:   "llm",
				AgentClass:  model.AgentClassProductPlanner,
				RuntimeKind: "native_sdk",
			},
			supported: true,
		},
		{
			name: "opencode does not support interactive",
			agent: model.Agent{
				AgentKind:   "llm",
				AgentClass:  model.AgentClassProductPlanner,
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

func TestNormalizeAgentRecordClearsIncompatibleFields(t *testing.T) {
	budget := 100
	planningNotes := "use specs first"
	systemPrompt := "do the work"
	agent := &model.Agent{
		AgentKind:          "llm",
		AgentClass:         model.AgentClassHuman,
		RuntimeKind:        "",
		PlanningNotes:      &planningNotes,
		SystemPrompt:       &systemPrompt,
		MonthlyTokenBudget: &budget,
		Skills:             []byte(`["x"]`),
		Tools:              []byte(`["y"]`),
	}

	normalizeAgentRecord(agent)

	if agent.AgentKind != "human" {
		t.Fatalf("expected human agent kind, got %q", agent.AgentKind)
	}
	if agent.Model != nil || agent.SystemPrompt != nil || agent.PlanningNotes != nil || agent.MonthlyTokenBudget != nil {
		t.Fatalf("expected human normalization to clear llm fields: %+v", agent)
	}
	if agent.Provider != nil {
		t.Fatalf("expected human normalization to clear provider, got %+v", agent.Provider)
	}
	if string(agent.Skills) != "[]" || string(agent.Tools) != "[]" {
		t.Fatalf("expected human normalization to clear skills/tools, got skills=%s tools=%s", string(agent.Skills), string(agent.Tools))
	}
	if agent.RuntimeKind == "" {
		t.Fatal("expected runtime kind default to be populated")
	}
}
