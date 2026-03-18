package service

import (
	"slices"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestFlowTemplateRegistryIncludesCrossModuleTemplates(t *testing.T) {
	specs := flowTemplateSpecs()
	templateIDs := make([]string, 0, len(specs))
	for _, spec := range specs {
		templateIDs = append(templateIDs, spec.TemplateID)
	}

	expected := []string{
		model.FlowTemplateEpicPlanningV1,
		model.FlowTemplateEpicPlanningV2,
		model.FlowTemplateStoryCompletionV1,
		model.FlowTemplateCRMDealReviewV1,
	}
	for _, templateID := range expected {
		if !slices.Contains(templateIDs, templateID) {
			t.Fatalf("expected template registry to include %q, got %v", templateID, templateIDs)
		}
	}
}

func TestEpicPlanningV2ApprovalNodesSupportRequestChanges(t *testing.T) {
	def, ok := lookupFlowTemplate(model.FlowTemplateEpicPlanningV2)
	if !ok {
		t.Fatalf("expected epic planning v2 template to be registered")
	}

	specApproval := def.nodes[model.FlowNodeSpecApproval]
	if !slices.Contains(specApproval.spec.Actions, model.FlowActionRequestChanges) {
		t.Fatalf("expected spec approval to allow request_changes, got %v", specApproval.spec.Actions)
	}
	if got := derefString(specApproval.spec.LoopbackNodeID); got != model.FlowNodeSpecDraft {
		t.Fatalf("expected spec approval loopback to %q, got %q", model.FlowNodeSpecDraft, got)
	}

	planApproval := def.nodes[model.FlowNodePlanApproval]
	if !slices.Contains(planApproval.spec.Actions, model.FlowActionRequestChanges) {
		t.Fatalf("expected plan approval to allow request_changes, got %v", planApproval.spec.Actions)
	}
	if got := derefString(planApproval.spec.LoopbackNodeID); got != model.FlowNodeStoryPlan {
		t.Fatalf("expected plan approval loopback to %q, got %q", model.FlowNodeStoryPlan, got)
	}
}

func TestCrossModuleTemplatesExposeExpectedApplyCommands(t *testing.T) {
	storyCompletion, ok := lookupFlowTemplate(model.FlowTemplateStoryCompletionV1)
	if !ok {
		t.Fatalf("expected story completion template to be registered")
	}
	if got := derefString(storyCompletion.nodes[model.FlowNodeCreateFollowups].spec.CommandName); got != "pm.create_followup_stories" {
		t.Fatalf("expected story completion apply command %q, got %q", "pm.create_followup_stories", got)
	}
	if storyCompletion.spec.TargetType != "story" {
		t.Fatalf("expected story completion target type story, got %q", storyCompletion.spec.TargetType)
	}

	dealReview, ok := lookupFlowTemplate(model.FlowTemplateCRMDealReviewV1)
	if !ok {
		t.Fatalf("expected deal review template to be registered")
	}
	if got := derefString(dealReview.nodes[model.FlowNodeApplyDealActions].spec.CommandName); got != "crm.apply_deal_actions" {
		t.Fatalf("expected deal review apply command %q, got %q", "crm.apply_deal_actions", got)
	}
	if dealReview.spec.TargetType != "crm_deal" {
		t.Fatalf("expected deal review target type crm_deal, got %q", dealReview.spec.TargetType)
	}
}
