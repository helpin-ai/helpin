package agentcontract

import (
	"slices"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDocumentationRuntimeProfileIncludesOrganizationTools(t *testing.T) {
	profile := GetRuntimeProfile(model.AgentPresetDocumentationAgent)
	if profile.RuntimeKind != "native_sdk" {
		t.Fatalf("documentation runtime profile = %q, want native_sdk", profile.RuntimeKind)
	}
	for _, tool := range []string{"search_workspace", "list_spaces", "create_space", "create_collection", "update_space", "update_collection", "move_document", "link_document_to_object", "complete_support_coverage_gap"} {
		if !slices.Contains(profile.AllowedTools, tool) {
			t.Fatalf("documentation runtime profile is missing %q", tool)
		}
	}
}

func TestWorkspaceSearchRuntimeProfileAlignment(t *testing.T) {
	for _, presetKey := range []string{
		model.AgentPresetEpicPlanner,
		model.AgentPresetTaskPlanner,
		model.AgentPresetCRMOperator,
		model.AgentPresetMarketer,
		model.AgentPresetDocumentationAgent,
	} {
		profile := GetRuntimeProfile(presetKey)
		if !slices.Contains(profile.AllowedTools, "search_workspace") {
			t.Errorf("runtime profile %q is missing search_workspace", presetKey)
		}
	}

	for _, presetKey := range []string{
		model.AgentPresetSupportAgent,
		model.AgentPresetCodeBuilder,
		model.AgentPresetReviewAgent,
	} {
		profile := GetRuntimeProfile(presetKey)
		if slices.Contains(profile.AllowedTools, "search_workspace") {
			t.Errorf("narrow runtime profile %q must not expose search_workspace", presetKey)
		}
	}
}

func TestExcludedPresetRuntimeProfilesHaveNoNewPMTools(t *testing.T) {
	newPMTools := []string{
		"list_workspace_members", "list_pm_labels", "get_task",
		"list_epics", "get_epic", "list_sprints", "get_sprint",
		"list_sprint_tasks", "list_objectives", "get_objective",
		"update_task", "create_task_checklist_item", "update_task_checklist_item",
		"add_pm_comment", "create_epic", "update_epic", "create_sprint",
		"update_sprint", "create_objective", "update_objective",
		"create_key_result", "update_key_result",
	}

	for _, presetKey := range []string{
		model.AgentPresetSupportAgent,
		model.AgentPresetDocumentationAgent,
		model.AgentPresetCodeBuilder,
		model.AgentPresetReviewAgent,
	} {
		profile := GetRuntimeProfile(presetKey)
		for _, toolName := range newPMTools {
			if slices.Contains(profile.AllowedTools, toolName) {
				t.Fatalf("excluded preset profile %q unexpectedly contains new PM tool %q", presetKey, toolName)
			}
		}
	}
}
