package agentskills

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

func TestSelectNativeActiveSkillsEpicDraftSpecSelectsPRDSkills(t *testing.T) {
	refs := model.AgentSkillRefs{
		{Key: "approval_protocol"},
		{Key: "prd_authorship"},
		{Key: "task_decomposition"},
		{Key: "epic_state_routing"},
		{Key: "general_agent_behavior"},
	}
	definitions := []worker.SkillDefinition{
		{Key: "approval_protocol", SourceKind: "built_in", Instructions: "approval"},
		{Key: "prd_authorship", SourceKind: "built_in", Instructions: "prd"},
		{Key: "task_decomposition", SourceKind: "built_in", Instructions: "tasks"},
		{Key: "epic_state_routing", SourceKind: "built_in", Instructions: "routing"},
		{Key: "general_agent_behavior", SourceKind: "built_in", Instructions: "general"},
	}

	selection := SelectNativeActiveSkills(refs, definitions, NativeActiveSelectionContext{
		PresetKey:     model.AgentPresetEpicPlanner,
		TargetType:    "epic",
		PlanningStage: model.PlanningStageDraftSpec,
	})

	if got := testSkillKeys(selection.Refs); len(got) != 4 || got[0] != "approval_protocol" || got[1] != "prd_authorship" || got[2] != "epic_state_routing" || got[3] != "general_agent_behavior" {
		t.Fatalf("unexpected active refs %#v", got)
	}
	if selection.Instructions != "approval\n\nprd\n\nrouting\n\ngeneral" {
		t.Fatalf("unexpected compiled instructions %q", selection.Instructions)
	}
}

func TestSelectNativeActiveSkillsEpicPlanTasksSelectsTaskSkills(t *testing.T) {
	refs := model.AgentSkillRefs{
		{Key: "approval_protocol"},
		{Key: "prd_authorship"},
		{Key: "task_decomposition"},
		{Key: "epic_state_routing"},
	}
	definitions := []worker.SkillDefinition{
		{Key: "approval_protocol", SourceKind: "built_in", Instructions: "approval"},
		{Key: "prd_authorship", SourceKind: "built_in", Instructions: "prd"},
		{Key: "task_decomposition", SourceKind: "built_in", Instructions: "tasks"},
		{Key: "epic_state_routing", SourceKind: "built_in", Instructions: "routing"},
	}

	selection := SelectNativeActiveSkills(refs, definitions, NativeActiveSelectionContext{
		PresetKey:     model.AgentPresetEpicPlanner,
		TargetType:    "epic",
		PlanningStage: model.PlanningStagePlanTasks,
	})

	if got := testSkillKeys(selection.Refs); len(got) != 3 || got[0] != "approval_protocol" || got[1] != "task_decomposition" || got[2] != "epic_state_routing" {
		t.Fatalf("unexpected active refs %#v", got)
	}
}

func TestSelectNativeActiveSkillsKeepsWorkspaceSkillsDefaultActive(t *testing.T) {
	skillID := "skill-123"
	refs := model.AgentSkillRefs{
		{Key: "approval_protocol"},
		{SkillID: &skillID, Key: "workspace_planner_extension"},
	}
	definitions := []worker.SkillDefinition{
		{Key: "approval_protocol", SourceKind: "built_in", Instructions: "approval"},
		{Key: "workspace_planner_extension", SourceKind: model.WorkspaceSkillSourceWorkspace, Instructions: "workspace"},
	}

	selection := SelectNativeActiveSkills(refs, definitions, NativeActiveSelectionContext{
		PresetKey:     model.AgentPresetTaskPlanner,
		TargetType:    "task",
		PlanningStage: model.PlanningStageTaskPlanDoc,
	})

	if got := testSkillKeys(selection.Refs); len(got) != 2 || got[0] != "approval_protocol" || got[1] != "workspace_planner_extension" {
		t.Fatalf("unexpected active refs %#v", got)
	}
}

func TestSelectNativeActiveSkillsFallsBackToFullSetForUnknownStage(t *testing.T) {
	refs := model.AgentSkillRefs{
		{Key: "approval_protocol"},
		{Key: "prd_authorship"},
	}
	definitions := []worker.SkillDefinition{
		{Key: "approval_protocol", SourceKind: "built_in", Instructions: "approval"},
		{Key: "prd_authorship", SourceKind: "built_in", Instructions: "prd"},
	}

	selection := SelectNativeActiveSkills(refs, definitions, NativeActiveSelectionContext{
		PresetKey:     model.AgentPresetEpicPlanner,
		TargetType:    "epic",
		PlanningStage: "unknown_phase",
	})

	if got := testSkillKeys(selection.Refs); len(got) != 2 || got[0] != "approval_protocol" || got[1] != "prd_authorship" {
		t.Fatalf("unexpected fallback refs %#v", got)
	}
}

func testSkillKeys(refs model.AgentSkillRefs) []string {
	keys := make([]string, 0, len(refs))
	for _, ref := range refs {
		keys = append(keys, ref.Key)
	}
	return keys
}
