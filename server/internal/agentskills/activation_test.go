package agentskills

import (
	"strings"
	"testing"

	worker "github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSelectNativeActiveSkillsEpicDraftSpecSelectsPRDSkills(t *testing.T) {
	refs := model.AgentSkillRefs{
		{Key: "prd_task_plan_approval"},
		{Key: "product_prd_authorship"},
		{Key: "coding_task_decomposition"},
		{Key: "epic_planning_state_routing"},
		{Key: "engineering_planner_operating_rules"},
	}
	definitions := []worker.SkillDefinition{
		{Key: "prd_task_plan_approval", SourceKind: "built_in", Instructions: "approval"},
		{Key: "product_prd_authorship", SourceKind: "built_in", Instructions: "prd"},
		{Key: "coding_task_decomposition", SourceKind: "built_in", Instructions: "tasks"},
		{Key: "epic_planning_state_routing", SourceKind: "built_in", Instructions: "routing"},
		{Key: "engineering_planner_operating_rules", SourceKind: "built_in", Instructions: "general"},
	}

	selection := SelectNativeActiveSkills(refs, definitions, NativeActiveSelectionContext{
		PresetKey:     model.AgentPresetEpicPlanner,
		TargetType:    "epic",
		PlanningStage: model.PlanningStageDraftSpec,
	})

	if got := testSkillKeys(selection.Refs); len(got) != 4 || got[0] != "prd_task_plan_approval" || got[1] != "product_prd_authorship" || got[2] != "epic_planning_state_routing" || got[3] != "engineering_planner_operating_rules" {
		t.Fatalf("unexpected active refs %#v", got)
	}
	if selection.Instructions != "approval\n\nprd\n\nrouting\n\ngeneral" {
		t.Fatalf("unexpected compiled instructions %q", selection.Instructions)
	}
}

func TestSelectNativeActiveSkillsEpicPlanTasksSelectsTaskSkills(t *testing.T) {
	refs := model.AgentSkillRefs{
		{Key: "prd_task_plan_approval"},
		{Key: "product_prd_authorship"},
		{Key: "coding_task_decomposition"},
		{Key: "epic_planning_state_routing"},
	}
	definitions := []worker.SkillDefinition{
		{Key: "prd_task_plan_approval", SourceKind: "built_in", Instructions: "approval"},
		{Key: "product_prd_authorship", SourceKind: "built_in", Instructions: "prd"},
		{Key: "coding_task_decomposition", SourceKind: "built_in", Instructions: "tasks"},
		{Key: "epic_planning_state_routing", SourceKind: "built_in", Instructions: "routing"},
	}

	selection := SelectNativeActiveSkills(refs, definitions, NativeActiveSelectionContext{
		PresetKey:     model.AgentPresetEpicPlanner,
		TargetType:    "epic",
		PlanningStage: model.PlanningStagePlanTasks,
	})

	if got := testSkillKeys(selection.Refs); len(got) != 3 || got[0] != "prd_task_plan_approval" || got[1] != "coding_task_decomposition" || got[2] != "epic_planning_state_routing" {
		t.Fatalf("unexpected active refs %#v", got)
	}
}

func TestSelectNativeActiveSkillsKeepsWorkspaceSkillsDefaultActive(t *testing.T) {
	skillID := "skill-123"
	refs := model.AgentSkillRefs{
		{Key: "prd_task_plan_approval"},
		{SkillID: &skillID, Key: "workspace_planner_extension"},
	}
	definitions := []worker.SkillDefinition{
		{Key: "prd_task_plan_approval", SourceKind: "built_in", Instructions: "approval"},
		{Key: "workspace_planner_extension", SourceKind: model.WorkspaceSkillSourceWorkspace, Instructions: "workspace"},
	}

	selection := SelectNativeActiveSkills(refs, definitions, NativeActiveSelectionContext{
		PresetKey:     model.AgentPresetTaskPlanner,
		TargetType:    "task",
		PlanningStage: model.PlanningStageTaskPlanDoc,
	})

	if got := testSkillKeys(selection.Refs); len(got) != 2 || got[0] != "prd_task_plan_approval" || got[1] != "workspace_planner_extension" {
		t.Fatalf("unexpected active refs %#v", got)
	}
}

func TestSelectNativeActiveSkillsDocumentationSupportTargetSelectsSupportGapSkills(t *testing.T) {
	refs := model.AgentSkillRefs{
		{Key: "docs_architecture_review"},
		{Key: "public_help_doc_writing"},
		{Key: "api_reference_doc_writing"},
		{Key: "internal_docs_maintenance"},
		{Key: "public_help_docs_maintenance"},
		{Key: "api_docs_maintenance"},
		{Key: "post_release_docs_update"},
		{Key: "support_gap_docs_update"},
	}
	definitions := []worker.SkillDefinition{
		{Key: "docs_architecture_review", SourceKind: "built_in", Instructions: "ia"},
		{Key: "public_help_doc_writing", SourceKind: "built_in", Instructions: "help-writing"},
		{Key: "api_reference_doc_writing", SourceKind: "built_in", Instructions: "api-writing"},
		{Key: "internal_docs_maintenance", SourceKind: "built_in", Instructions: "internal"},
		{Key: "public_help_docs_maintenance", SourceKind: "built_in", Instructions: "public"},
		{Key: "api_docs_maintenance", SourceKind: "built_in", Instructions: "api-maintenance"},
		{Key: "post_release_docs_update", SourceKind: "built_in", Instructions: "release"},
		{Key: "support_gap_docs_update", SourceKind: "built_in", Instructions: "gap"},
	}

	selection := SelectNativeActiveSkills(refs, definitions, NativeActiveSelectionContext{
		PresetKey:  model.AgentPresetDocumentationAgent,
		TargetType: "support_conversation",
	})

	if got := testSkillKeys(selection.Refs); strings.Join(got, ",") != "docs_architecture_review,public_help_doc_writing,public_help_docs_maintenance,support_gap_docs_update" {
		t.Fatalf("unexpected active refs %#v", got)
	}
	if strings.Contains(selection.Instructions, "api-writing") || strings.Contains(selection.Instructions, "release") {
		t.Fatalf("unexpected inactive documentation instructions included %q", selection.Instructions)
	}
}

func TestSelectNativeActiveSkillsFallsBackToFullSetForUnknownStage(t *testing.T) {
	refs := model.AgentSkillRefs{
		{Key: "prd_task_plan_approval"},
		{Key: "product_prd_authorship"},
	}
	definitions := []worker.SkillDefinition{
		{Key: "prd_task_plan_approval", SourceKind: "built_in", Instructions: "approval"},
		{Key: "product_prd_authorship", SourceKind: "built_in", Instructions: "prd"},
	}

	selection := SelectNativeActiveSkills(refs, definitions, NativeActiveSelectionContext{
		PresetKey:     model.AgentPresetEpicPlanner,
		TargetType:    "epic",
		PlanningStage: "unknown_phase",
	})

	if got := testSkillKeys(selection.Refs); len(got) != 2 || got[0] != "prd_task_plan_approval" || got[1] != "product_prd_authorship" {
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
