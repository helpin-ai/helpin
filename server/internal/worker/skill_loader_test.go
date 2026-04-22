package worker

import "testing"

func TestGetBuiltInSkillLoadsPackagedSkill(t *testing.T) {
	skill, ok := GetBuiltInSkill("approval_protocol")
	if !ok {
		t.Fatal("expected approval_protocol skill")
	}
	if skill.PackagePath != "system/approval_protocol" {
		t.Fatalf("expected package path system/approval_protocol, got %q", skill.PackagePath)
	}
	if skill.Title != "Approval Protocol" {
		t.Fatalf("expected Approval Protocol title, got %q", skill.Title)
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
