package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

func TestPlanningStageForDelegatedRun(t *testing.T) {
	taskPlanner := &model.Agent{IsSystem: true, PresetKey: model.AgentPresetTaskPlanner}
	if got := planningStageForDelegatedRun(taskPlanner, &model.PMTask{}, nil); got != model.PlanningStageTaskPlanDoc {
		t.Fatalf("task planner stage = %q", got)
	}

	epicPlanner := &model.Agent{IsSystem: true, PresetKey: model.AgentPresetEpicPlanner}
	if got := planningStageForDelegatedRun(epicPlanner, nil, &model.PMEpic{}); got != model.PlanningStageDraftSpec {
		t.Fatalf("new epic planner stage = %q", got)
	}
	approvedVersion := "version-1"
	if got := planningStageForDelegatedRun(epicPlanner, nil, &model.PMEpic{ApprovedSpecVersionID: &approvedVersion}); got != model.PlanningStagePlanTasks {
		t.Fatalf("approved epic planner stage = %q", got)
	}
	if got := planningStageForDelegatedRun(epicPlanner, nil, &model.PMEpic{PlanningState: model.EpicPlanningStateReadyForTaskPlanning}); got != model.PlanningStagePlanTasks {
		t.Fatalf("ready epic planner stage = %q", got)
	}
}

func TestRuntimeStartRunRequestPropagatesPlannerSelectors(t *testing.T) {
	payload, err := buildAgentRunInputPayload("epic", "epic-1", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err = withAgentRunPlanningStage(payload, model.PlanningStagePlanTasks)
	if err != nil {
		t.Fatal(err)
	}
	run := &model.AgentRun{ID: "run-1", WorkspaceID: "workspace-1", AgentID: "agent-1", TargetType: "epic", TargetID: "epic-1", Input: json.RawMessage(payload)}
	agent := &model.Agent{ID: "agent-1", PresetKey: model.AgentPresetEpicPlanner}
	req := runtimeStartRunRequest(run, agent)
	if req.Metadata["preset_key"] != model.AgentPresetEpicPlanner || req.Metadata["planning_stage"] != model.PlanningStagePlanTasks {
		t.Fatalf("missing planner selectors: %#v", req.Metadata)
	}
}

func TestBuildDelegatedEpicLaunchContextIncludesDurableFacts(t *testing.T) {
	description := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Reduce onboarding friction."}]}]}`
	teamID := "team-1"
	specID := "doc-1"
	epic := &model.PMEpic{
		ID:             "epic-1",
		WorkspaceID:    "workspace-1",
		Name:           "Onboarding",
		Description:    &description,
		TeamID:         &teamID,
		SpecDocumentID: &specID,
		PlanningState:  model.EpicPlanningStateAwaitingSpecApproval,
	}
	got := (&AgentService{}).buildDelegatedEpicLaunchContext(context.Background(), epic, strPtr("Keep scope narrow."))
	for _, want := range []string{"Operator notes", "Keep scope narrow", "Epic: **Onboarding**", "Reduce onboarding friction", "planning_state=awaiting_spec_approval", "team_id=team-1", "spec_document_id=doc-1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("context missing %q:\n%s", want, got)
		}
	}
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
