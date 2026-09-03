package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
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
	"task",
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
	req, err := runtimeStartRunRequest(run, agent, runtimeAgentFromHelpinAgent(agent, "helpin"))
	if err != nil {
		t.Fatalf("runtimeStartRunRequest returned error: %v", err)
	}
	if req.Metadata["preset_key"] != model.AgentPresetEpicPlanner || req.Metadata["planning_stage"] != model.PlanningStagePlanTasks {
		t.Fatalf("missing planner selectors: %#v", req.Metadata)
	}
}

func TestRuntimeStartRunRequestScopesCompletionToCoverageTarget(t *testing.T) {
	run := &model.AgentRun{
		ID: "run-gap", WorkspaceID: "workspace-1", AgentID: "agent-custom",
		TargetType: "support_coverage_gap", TargetID: "gap-1",
	}
	agent := &model.Agent{ID: "agent-custom"}
	req, err := runtimeStartRunRequest(run, agent, runtimeAgentFromHelpinAgent(agent, "helpin"))
	if err != nil {
		t.Fatalf("runtimeStartRunRequest returned error: %v", err)
	}
	required, ok := req.Metadata["completion_required_tools"].([]string)
	wantRequired := []string{agentcontract.ToolCompleteSupportCoverageGap}
	if !ok || !slices.Equal(required, wantRequired) {
		t.Fatalf("coverage completion tools = %#v", req.Metadata["completion_required_tools"])
	}

	run.TargetType = "document"
	req, err = runtimeStartRunRequest(run, agent, runtimeAgentFromHelpinAgent(agent, "helpin"))
	if err != nil {
		t.Fatalf("document runtimeStartRunRequest returned error: %v", err)
	}
	if _, exists := req.Metadata["completion_required_tools"]; exists {
		t.Fatalf("document run inherited coverage completion tools: %#v", req.Metadata)
	}
}

func TestRunAllowedToolsForCoverageTargetRequiresAgentAuthorization(t *testing.T) {
	agent := &model.Agent{Name: "Custom docs agent", AllowedTools: json.RawMessage(`["read_file"]`)}
	_, err := runAllowedToolsForTargetContract(agent, "support_coverage_gap", nil)
	if err == nil || !strings.Contains(err.Error(), agentcontract.ToolCompleteSupportCoverageGap) {
		t.Fatalf("expected missing completion tool error, got %v", err)
	}
}

func TestRunAllowedToolsForCoverageTargetPreservesRequiredToolWhenNarrowed(t *testing.T) {
	agent := &model.Agent{
		Name:         "Custom docs agent",
		AllowedTools: json.RawMessage(`["read_file","complete_support_coverage_gap"]`),
	}
	tools, err := runAllowedToolsForTargetContract(agent, "support_coverage_gap", []string{"read_file"})
	if err != nil {
		t.Fatalf("runAllowedToolsForTargetContract returned error: %v", err)
	}
	want := []string{"read_files", agentcontract.ToolCompleteSupportCoverageGap}
	if !slices.Equal(tools, want) {
		t.Fatalf("coverage run tools = %#v, want %#v", tools, want)
	}
}

func TestRunAllowedToolsForCoverageTargetInheritsAuthorizedAgentTools(t *testing.T) {
	agent := &model.Agent{
		Name:         "Custom docs agent",
		AllowedTools: json.RawMessage(`["complete_support_coverage_gap"]`),
	}
	tools, err := runAllowedToolsForTargetContract(agent, "support_coverage_gap", nil)
	if err != nil {
		t.Fatalf("runAllowedToolsForTargetContract returned error: %v", err)
	}
	if tools != nil {
		t.Fatalf("inherited run tools = %#v, want nil", tools)
	}
}

func TestSupportCoverageGapRunInstructionsRequireDocsAndRepositoryVerification(t *testing.T) {
	instructions := supportCoverageGapAgentInstructions(&model.SupportCoverageGapDetail{
		SupportCoverageGap: model.SupportCoverageGap{V1GapType: "feature_overview"},
	})
	for _, expected := range []string{
		"Search the current workspace documentation before deciding the disposition",
		"Inspect a repository only when product or feature implementation is a relevant source of truth",
		"a successful search with no relevant match is valid evidence for feature_not_found",
		"Finish every run by calling complete_support_coverage_gap with the disposition",
	} {
		if !strings.Contains(instructions, expected) {
			t.Fatalf("expected coverage instructions to contain %q\n%s", expected, instructions)
		}
	}
}

func TestRuntimeStartRunRequestUsesProjectedAgentToolSubset(t *testing.T) {
	run := &model.AgentRun{
		ID:         "run-1",
		AgentID:    "agent-1",
		TargetType: "workspace",
		TargetID:   "workspace-1",
		Input:      json.RawMessage(`{"allowed_tools":["list_available_skills","list_tasks"]}`),
	}
	agent := &model.Agent{ID: "agent-1"}
	runtimeAgent := AgentRuntimeAgent{AllowedTools: []string{"list_tasks"}}

	req, err := runtimeStartRunRequest(run, agent, runtimeAgent)
	if err != nil {
		t.Fatalf("runtimeStartRunRequest returned error: %v", err)
	}
	if len(req.AllowedTools) != 1 || req.AllowedTools[0] != "list_tasks" {
		t.Fatalf("run tools must be narrowed to the projected agent contract, got %#v", req.AllowedTools)
	}
}

func TestRuntimeStartRunRequestRejectsDisjointProjectedAgentTools(t *testing.T) {
	run := &model.AgentRun{
		ID:         "run-1",
		AgentID:    "agent-1",
		TargetType: "workspace",
		TargetID:   "workspace-1",
		Input:      json.RawMessage(`{"allowed_tools":["list_available_skills"]}`),
	}

	_, err := runtimeStartRunRequest(
		run,
		&model.Agent{ID: "agent-1"},
		AgentRuntimeAgent{AllowedTools: []string{"list_tasks"}},
	)
	if err == nil || !strings.Contains(err.Error(), "do not overlap") {
		t.Fatalf("expected a repair-oriented disjoint tool error, got %v", err)
	}
}

func TestInitialAgentRunContextReturnsExactEnrichedInstructions(t *testing.T) {
	payload, err := buildAgentRunInputPayload("task", "task-1", nil, nil, nil, strPtr("Operator notes:\nKeep scope narrow."), nil)
	if err != nil {
		t.Fatal(err)
	}
	run := &model.AgentRun{Input: json.RawMessage(payload)}
	if got := initialAgentRunContext(run); got != "Operator notes:\nKeep scope narrow." {
		t.Fatalf("initial context = %q", got)
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
