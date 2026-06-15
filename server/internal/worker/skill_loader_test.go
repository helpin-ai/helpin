package worker

import "testing"

func TestGetBuiltInSkillLoadsPackagedSkill(t *testing.T) {
	skill, ok := GetBuiltInSkill("prd_task_plan_approval")
	if !ok {
		t.Fatal("expected prd_task_plan_approval skill")
	}
	if skill.PackagePath != "system/approval_protocol" {
		t.Fatalf("expected package path system/approval_protocol, got %q", skill.PackagePath)
	}
	if skill.Title != "PRD and Task Plan Approval" {
		t.Fatalf("expected PRD and Task Plan Approval title, got %q", skill.Title)
	}
	if skill.Description == "" {
		t.Fatal("expected non-empty skill description")
	}
	if skill.Instructions == "" {
		t.Fatal("expected non-empty skill instructions")
	}
	if !containsString(skill.Policy.CompletionRequiresInteractionKinds, InteractionKindApprovalRequest) {
		t.Fatalf("expected approval request completion requirement, got %v", skill.Policy.CompletionRequiresInteractionKinds)
	}
	if !containsString(skill.Policy.CompletionRequiresInteractionKinds, InteractionKindRequestUserInput) {
		t.Fatalf("expected user input completion requirement, got %v", skill.Policy.CompletionRequiresInteractionKinds)
	}
}

func TestGetBuiltInSkillAcceptsLegacySkillAlias(t *testing.T) {
	skill, ok := GetBuiltInSkill("approval_protocol")
	if !ok {
		t.Fatal("expected legacy approval_protocol alias to resolve")
	}
	if skill.Key != "prd_task_plan_approval" {
		t.Fatalf("expected canonical prd_task_plan_approval key, got %q", skill.Key)
	}
}

func TestGetBuiltInSkillAcceptsInterimSkillAlias(t *testing.T) {
	skill, ok := GetBuiltInSkill("planning_approval_protocol")
	if !ok {
		t.Fatal("expected interim planning_approval_protocol alias to resolve")
	}
	if skill.Key != "prd_task_plan_approval" {
		t.Fatalf("expected canonical prd_task_plan_approval key, got %q", skill.Key)
	}
}
