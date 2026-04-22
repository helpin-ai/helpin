package worker

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestListBuiltInSkillsContainsExpectedKeys(t *testing.T) {
	skills := ListBuiltInSkills()
	keys := make([]string, 0, len(skills))
	for _, skill := range skills {
		keys = append(keys, skill.Key)
	}
	for _, key := range []string{
		"approval_protocol",
		"prd_authorship",
		"task_decomposition",
		"epic_state_routing",
		"general_agent_behavior",
		"task_planner_context",
		"code_builder",
		"review_agent",
		"crm_operator",
		"support_agent",
	} {
		if !containsString(keys, key) {
			t.Fatalf("expected built-in skill %q in registry, got %v", key, keys)
		}
	}
}

func TestCompilePresetInstructionsIncludesPreambleAndSkills(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetEpicPlanner)
	if !ok {
		t.Fatal("expected epic planner bundle")
	}
	compiled := CompilePresetInstructions(bundle.Preamble, bundle.SkillKeys)
	for _, snippet := range []string{
		"You are Epic Planner.",
		"Approval requests happen inline in the same chat.",
		"## PRD Work",
		"## Current Facts And Next-Step Rules",
		"## General Rules",
	} {
		if !strings.Contains(compiled, snippet) {
			t.Fatalf("expected compiled prompt to contain %q\n%s", snippet, compiled)
		}
	}
}

func TestInstructionTemplateVersionForPresetIsDeterministic(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetReviewAgent)
	if !ok {
		t.Fatal("expected review bundle")
	}
	versionA := InstructionTemplateVersionForPreset(bundle.Preamble, bundle.SkillKeys)
	versionB := InstructionTemplateVersionForPreset(bundle.Preamble, bundle.SkillKeys)
	if versionA == "" {
		t.Fatal("expected non-empty template version")
	}
	if versionA != versionB {
		t.Fatalf("expected deterministic version, got %q and %q", versionA, versionB)
	}
}

func TestTaskPlannerBundleUsesTaskPlanDocSkillStack(t *testing.T) {
	bundle, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetTaskPlanner)
	if !ok {
		t.Fatal("expected task planner bundle")
	}
	if !containsString(bundle.SkillKeys, "task_planner_context") {
		t.Fatalf("expected task planner context skill, got %v", bundle.SkillKeys)
	}
	if containsString(bundle.SkillKeys, "task_decomposition") {
		t.Fatalf("did not expect epic task-plan skill in task planner bundle, got %v", bundle.SkillKeys)
	}
}

func TestListSkillCatalogReturnsBuiltInSkills(t *testing.T) {
	catalog := ListSkillCatalog()
	if len(catalog.Skills) == 0 {
		t.Fatal("expected built-in skills in catalog")
	}
	found := false
	for _, skill := range catalog.Skills {
		if skill.Key != "approval_protocol" {
			continue
		}
		found = true
		if skill.SourceKind != "built_in" {
			t.Fatalf("expected built_in source kind, got %q", skill.SourceKind)
		}
		if !containsString(skill.RequiredTools, ToolRequestApproval) {
			t.Fatalf("expected request approval requirement, got %v", skill.RequiredTools)
		}
		if !containsString(skill.Presets, model.AgentPresetEpicPlanner) || !containsString(skill.Presets, model.AgentPresetTaskPlanner) {
			t.Fatalf("expected planner preset mappings, got %v", skill.Presets)
		}
	}
	if !found {
		t.Fatal("expected approval_protocol in skill catalog")
	}
}

func TestReviewAgentSkillDeclaresCompletionInteractionPolicy(t *testing.T) {
	skill, ok := GetBuiltInSkill("review_agent")
	if !ok {
		t.Fatal("expected review_agent built-in skill")
	}
	if len(skill.RequiredTools) != 0 {
		t.Fatalf("expected review_agent to avoid transport-specific required tools, got %v", skill.RequiredTools)
	}
	if !containsString(skill.Policy.CompletionRequiresInteractionKinds, model.AgentRunInteractionKindReviewCheckpoint) {
		t.Fatalf("expected review checkpoint completion requirement, got %v", skill.Policy.CompletionRequiresInteractionKinds)
	}
	if !containsString(skill.Policy.CompletionRequiresInteractionKinds, ToolRequestUserInput) {
		t.Fatalf("expected user input completion requirement, got %v", skill.Policy.CompletionRequiresInteractionKinds)
	}
	contract, ok := skill.Policy.InteractionContract(InteractionKindReviewCheckpoint)
	if !ok {
		t.Fatal("expected review_checkpoint interaction contract")
	}
	if contract.Schema != "review_checkpoint_v1" {
		t.Fatalf("expected review checkpoint schema, got %q", contract.Schema)
	}
	if contract.Transports["native_sdk"].ToolName != ToolRequestReviewCheckpoint {
		t.Fatalf("expected native_sdk review checkpoint tool transport, got %+v", contract.Transports["native_sdk"])
	}
	if contract.Transports["codex"].BlockLabel != "helpin-review" {
		t.Fatalf("expected codex review checkpoint block label, got %+v", contract.Transports["codex"])
	}
	inputContract, ok := skill.Policy.InteractionContract(InteractionKindRequestUserInput)
	if !ok {
		t.Fatal("expected request_user_input interaction contract")
	}
	if inputContract.Schema != "request_user_input_v1" {
		t.Fatalf("expected request user input schema, got %q", inputContract.Schema)
	}
	if inputContract.Transports["native_sdk"].ToolName != ToolRequestUserInput {
		t.Fatalf("expected native_sdk request_user_input tool transport, got %+v", inputContract.Transports["native_sdk"])
	}
	if inputContract.Transports["codex"].Type != InteractionTransportTypeRuntimeBridge {
		t.Fatalf("expected codex request_user_input runtime bridge transport, got %+v", inputContract.Transports["codex"])
	}
}

func TestApprovalProtocolSkillDeclaresPlannerCompletionInteractionPolicy(t *testing.T) {
	skill, ok := GetBuiltInSkill("approval_protocol")
	if !ok {
		t.Fatal("expected approval_protocol built-in skill")
	}
	if !containsString(skill.Policy.CompletionRequiresInteractionKinds, InteractionKindApprovalRequest) {
		t.Fatalf("expected approval request completion requirement, got %v", skill.Policy.CompletionRequiresInteractionKinds)
	}
	if !containsString(skill.Policy.CompletionRequiresInteractionKinds, InteractionKindRequestUserInput) {
		t.Fatalf("expected user input completion requirement, got %v", skill.Policy.CompletionRequiresInteractionKinds)
	}
	contract, ok := skill.Policy.InteractionContract(InteractionKindApprovalRequest)
	if !ok {
		t.Fatal("expected approval_request interaction contract")
	}
	if contract.Transports["native_sdk"].ToolName != ToolRequestApproval {
		t.Fatalf("expected native_sdk approval request tool transport, got %+v", contract.Transports["native_sdk"])
	}
	inputContract, ok := skill.Policy.InteractionContract(InteractionKindRequestUserInput)
	if !ok {
		t.Fatal("expected request_user_input interaction contract")
	}
	if inputContract.Transports["native_sdk"].ToolName != ToolRequestUserInput {
		t.Fatalf("expected native_sdk request_user_input tool transport, got %+v", inputContract.Transports["native_sdk"])
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
