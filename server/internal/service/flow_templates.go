package service

import (
	"context"
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type flowInteractiveLaunch struct {
	AgentID           string
	AdditionalContext string
	AllowedTools      []string
	Stage             string
	SystemPrompt      string
}

type flowAgentTaskLaunch struct {
	AgentID      string
	Input        json.RawMessage
	AllowedTools []string
	SystemPrompt string
}

type flowNodeDefinition struct {
	spec                     model.FlowNodeSpec
	nextNodeID               string
	retryable                bool
	commandName              string
	approveCommandName       string
	buildInteractiveLaunch   func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (flowInteractiveLaunch, error)
	buildAgentTaskLaunch     func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (flowAgentTaskLaunch, error)
	buildCommandInput        func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (json.RawMessage, error)
	buildApprovalInput       func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun) (json.RawMessage, error)
	buildApproveCommandInput func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string, payload json.RawMessage) (json.RawMessage, error)
}

type flowTemplateDefinition struct {
	spec              model.FlowSpec
	nodes             map[string]flowNodeDefinition
	initialNodeID     string
	startRun          func(ctx context.Context, svc *FlowService, workspaceID, actorID string, req model.StartFlowRunRequest) (*model.FlowRunView, error)
	clearActiveTarget func(ctx context.Context, svc *FlowService, run *model.FlowRun) error
}

func hardcodedFlowTemplateSpecs() []model.FlowSpec {
	defs := hardcodedFlowTemplateDefinitions()
	out := make([]model.FlowSpec, 0, len(defs))
	for _, def := range defs {
		out = append(out, def.spec)
	}
	return out
}

func lookupHardcodedFlowTemplate(templateID string) (flowTemplateDefinition, bool) {
	def, ok := hardcodedFlowTemplateDefinitions()[templateID]
	return def, ok
}

func hardcodedFlowTemplateDefinitions() map[string]flowTemplateDefinition {
	return map[string]flowTemplateDefinition{
		model.FlowTemplateEpicPlanningV2:    epicPlanningV2Template(),
		model.FlowTemplateStoryCompletionV1: storyCompletionTemplate(),
		model.FlowTemplateAgentStoryRun:     agentStoryRunTemplate(),
		model.FlowTemplateCRMDealReviewV1:   crmDealReviewTemplate(),
	}
}

func epicPlanningV2Template() flowTemplateDefinition {
	specLoopback := model.FlowNodeSpecDraft
	planLoopback := model.FlowNodeStoryPlan
	nodes := []flowNodeDefinition{
		{
			spec: makeFlowNodeSpec(model.FlowNodeEnsureSpecDoc, model.FlowNodeTypeSystemAction, "", nil, nil, "docs.ensure_spec_doc", nil),
			nextNodeID:  model.FlowNodeSpecDraft,
			commandName: "docs.ensure_spec_doc",
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeSpecDraft, model.FlowNodeTypeInteractiveAgent, model.InvocationModeInteractive, []string{model.FlowActionFinalize}, planningReadOnlyTools(), "", nil),
			nextNodeID: model.FlowNodeSpecApproval,
			buildInteractiveLaunch: func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (flowInteractiveLaunch, error) {
				return svc.buildEpicSpecInteractiveLaunch(ctx, run, true)
			},
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeSpecApproval, model.FlowNodeTypeApprovalGate, "", []string{model.FlowActionApprove, model.FlowActionRequestChanges, model.FlowActionReject}, nil, "", &specLoopback),
			nextNodeID:         model.FlowNodeStoryPlan,
			approveCommandName: "pm.approve_epic_spec",
			buildApproveCommandInput: func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string, payload json.RawMessage) (json.RawMessage, error) {
				return svc.buildApproveSpecCommandInput(payload)
			},
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeStoryPlan, model.FlowNodeTypeInteractiveAgent, model.InvocationModeInteractive, []string{model.FlowActionFinalize}, epicStoryPlanningTools(), "", nil),
			nextNodeID: model.FlowNodePlanApproval,
			buildInteractiveLaunch: func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (flowInteractiveLaunch, error) {
				return svc.buildEpicStoryPlanningInteractiveLaunch(ctx, run)
			},
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodePlanApproval, model.FlowNodeTypeApprovalGate, "", []string{model.FlowActionApprove, model.FlowActionRequestChanges, model.FlowActionReject}, nil, "", &planLoopback),
			nextNodeID: model.FlowNodeCreateStories,
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeCreateStories, model.FlowNodeTypeSystemAction, "", nil, nil, "pm.create_story_batch", nil),
			nextNodeID:  model.FlowNodeDone,
			retryable:   true,
			commandName: "pm.create_story_batch",
			buildCommandInput: func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (json.RawMessage, error) {
				return svc.buildCreateStoriesCommandInput(ctx, run)
			},
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeDone, model.FlowNodeTypeTerminal, "", nil, nil, "", nil),
		},
	}
	return flowTemplateDefinition{
		spec: model.FlowSpec{
			TemplateID:        model.FlowTemplateEpicPlanningV2,
			Name:              hardcodedTemplateNames[model.FlowTemplateEpicPlanningV2],
			TemplateVersion:   2,
			TargetType:        "epic",
			SupportedTriggers: []string{model.FlowTriggerManual, model.FlowTriggerInternalDomainHook},
			Nodes:             publicFlowNodes(nodes),
		},
		nodes:             toFlowNodeMap(nodes),
		initialNodeID:     model.FlowNodeEnsureSpecDoc,
		startRun: func(ctx context.Context, svc *FlowService, workspaceID, actorID string, req model.StartFlowRunRequest) (*model.FlowRunView, error) {
			return svc.startConfiguredEpicPlanningFlow(ctx, workspaceID, actorID, req)
		},
		clearActiveTarget: func(ctx context.Context, svc *FlowService, run *model.FlowRun) error {
			return svc.clearActiveTargetForRun(ctx, run)
		},
	}
}

func storyCompletionTemplate() flowTemplateDefinition {
	loopback := model.FlowNodeCompletionAssessment
	nodes := []flowNodeDefinition{
		{
			spec: makeFlowNodeSpec(model.FlowNodeCompletionAssessment, model.FlowNodeTypeAgentTask, model.InvocationModeAutonomous, nil, storyCompletionTools(), "", nil),
			nextNodeID: model.FlowNodeCompletionReview,
			buildAgentTaskLaunch: func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (flowAgentTaskLaunch, error) {
				return svc.buildStoryCompletionLaunch(ctx, run)
			},
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeCompletionReview, model.FlowNodeTypeApprovalGate, "", []string{model.FlowActionApprove, model.FlowActionRequestChanges, model.FlowActionReject}, nil, "", &loopback),
			nextNodeID: model.FlowNodeCreateFollowups,
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeCreateFollowups, model.FlowNodeTypeSystemAction, "", nil, nil, "pm.create_followup_stories", nil),
			nextNodeID:  model.FlowNodeDone,
			commandName: "pm.create_followup_stories",
			buildCommandInput: func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (json.RawMessage, error) {
				return svc.buildStoryCompletionCommandInput(ctx, run)
			},
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeDone, model.FlowNodeTypeTerminal, "", nil, nil, "", nil),
		},
	}
	return flowTemplateDefinition{
		spec: model.FlowSpec{
			TemplateID:        model.FlowTemplateStoryCompletionV1,
			Name:              hardcodedTemplateNames[model.FlowTemplateStoryCompletionV1],
			TemplateVersion:   1,
			TargetType:        "story",
			SupportedTriggers: []string{model.FlowTriggerManual, model.FlowTriggerInternalDomainHook, model.FlowTriggerStoryStateEntered},
			Nodes:             publicFlowNodes(nodes),
		},
		nodes:         toFlowNodeMap(nodes),
		initialNodeID: model.FlowNodeCompletionAssessment,
		startRun: func(ctx context.Context, svc *FlowService, workspaceID, actorID string, req model.StartFlowRunRequest) (*model.FlowRunView, error) {
			return svc.startStoryCompletionFlow(ctx, workspaceID, actorID, req)
		},
	}
}

func agentStoryRunTemplate() flowTemplateDefinition {
	nodes := []flowNodeDefinition{
		{
			spec:       makeFlowNodeSpec("implement", model.FlowNodeTypeAgentTask, model.InvocationModeAutonomous, nil, nil, "", nil),
			nextNodeID: model.FlowNodeDone,
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeDone, model.FlowNodeTypeTerminal, "", nil, nil, "", nil),
		},
	}
	return flowTemplateDefinition{
		spec: model.FlowSpec{
			TemplateID:        model.FlowTemplateAgentStoryRun,
			Name:              hardcodedTemplateNames[model.FlowTemplateAgentStoryRun],
			TemplateVersion:   1,
			TargetType:        "story",
			SupportedTriggers: []string{model.FlowTriggerManual, model.FlowTriggerStoryStateEntered},
			Nodes:             publicFlowNodes(nodes),
		},
		nodes:         toFlowNodeMap(nodes),
		initialNodeID: "implement",
	}
}

func crmDealReviewTemplate() flowTemplateDefinition {
	loopback := model.FlowNodeDealReview
	nodes := []flowNodeDefinition{
		{
			spec: makeFlowNodeSpec(model.FlowNodeDealReview, model.FlowNodeTypeAgentTask, model.InvocationModeAutonomous, nil, crmDealReviewTools(), "", nil),
			nextNodeID: model.FlowNodeDealReviewApproval,
			buildAgentTaskLaunch: func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (flowAgentTaskLaunch, error) {
				return svc.buildCRMDealReviewLaunch(ctx, run)
			},
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeDealReviewApproval, model.FlowNodeTypeApprovalGate, "", []string{model.FlowActionApprove, model.FlowActionRequestChanges, model.FlowActionReject}, nil, "", &loopback),
			nextNodeID: model.FlowNodeApplyDealActions,
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeApplyDealActions, model.FlowNodeTypeSystemAction, "", nil, nil, "crm.apply_deal_actions", nil),
			nextNodeID:  model.FlowNodeDone,
			commandName: "crm.apply_deal_actions",
			buildCommandInput: func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (json.RawMessage, error) {
				return svc.buildCRMDealReviewCommandInput(ctx, run)
			},
		},
		{
			spec: makeFlowNodeSpec(model.FlowNodeDone, model.FlowNodeTypeTerminal, "", nil, nil, "", nil),
		},
	}
	return flowTemplateDefinition{
		spec: model.FlowSpec{
			TemplateID:        model.FlowTemplateCRMDealReviewV1,
			Name:              hardcodedTemplateNames[model.FlowTemplateCRMDealReviewV1],
			TemplateVersion:   1,
			TargetType:        "crm_deal",
			SupportedTriggers: []string{model.FlowTriggerManual, model.FlowTriggerInternalDomainHook},
			Nodes:             publicFlowNodes(nodes),
		},
		nodes:         toFlowNodeMap(nodes),
		initialNodeID: model.FlowNodeDealReview,
		startRun: func(ctx context.Context, svc *FlowService, workspaceID, actorID string, req model.StartFlowRunRequest) (*model.FlowRunView, error) {
			return svc.startCRMDealReviewFlow(ctx, workspaceID, actorID, req)
		},
	}
}

func toFlowNodeMap(nodes []flowNodeDefinition) map[string]flowNodeDefinition {
	out := make(map[string]flowNodeDefinition, len(nodes))
	for _, node := range nodes {
		out[node.spec.ID] = node
	}
	return out
}

// hardcodedNodeLabels maps node IDs to display labels for the hardcoded templates.
var hardcodedNodeLabels = map[string]string{
	model.FlowNodeEnsureSpecDoc:        "Ensure Spec Document",
	model.FlowNodeSpecDraft:            "Draft Specification",
	model.FlowNodeSpecApproval:         "Approve Specification",
	model.FlowNodeStoryPlan:            "Plan Stories",
	model.FlowNodePlanApproval:         "Approve Story Plan",
	model.FlowNodeCreateStories:        "Create Stories",
	model.FlowNodeCompletionAssessment: "Assess Completion",
	model.FlowNodeCompletionReview:     "Review Follow-Ups",
	model.FlowNodeCreateFollowups:      "Create Follow-Ups",
	"implement":                         "Implement Story",
	model.FlowNodeDealReview:           "Review Deal",
	model.FlowNodeDealReviewApproval:   "Approve Deal Actions",
	model.FlowNodeApplyDealActions:     "Apply Deal Actions",
	model.FlowNodeDone:                 "Complete",
}

// hardcodedTemplateNames maps template IDs to display names.
var hardcodedTemplateNames = map[string]string{
	model.FlowTemplateEpicPlanningV2:    "Epic Planning",
	model.FlowTemplateStoryCompletionV1: "Story Completion",
	model.FlowTemplateAgentStoryRun:     "Agent Story Run",
	model.FlowTemplateCRMDealReviewV1:   "Deal Review",
}

func publicFlowNodes(nodes []flowNodeDefinition) []model.FlowNodeSpec {
	out := make([]model.FlowNodeSpec, 0, len(nodes))
	for _, node := range nodes {
		spec := node.spec
		if spec.Label == "" {
			spec.Label = hardcodedNodeLabels[spec.ID]
		}
		out = append(out, spec)
	}
	return out
}

func makeFlowNodeSpec(id, nodeType, requiredMode string, actions, allowedTools []string, commandName string, loopbackNodeID *string) model.FlowNodeSpec {
	spec := model.FlowNodeSpec{
		ID:           id,
		Type:         nodeType,
		RequiredMode: requiredMode,
		Actions:      actions,
		AllowedTools: allowedTools,
	}
	if commandName != "" {
		spec.CommandName = strPtr(commandName)
	}
	if loopbackNodeID != nil && *loopbackNodeID != "" {
		spec.LoopbackNodeID = loopbackNodeID
	}
	return spec
}

func planningReadOnlyTools() []string {
	return []string{
		"read_file",
		"read_file_range",
		"list_directory",
		"search_files",
		"ripgrep",
		"grep",
		"list_symbols",
		"web_search",
		"list_documents",
		"read_document",
		"search_documents",
	}
}

func epicStoryPlanningTools() []string {
	return []string{
		"read_file",
		"read_file_range",
		"list_directory",
		"search_files",
		"ripgrep",
		"grep",
		"list_symbols",
		"web_search",
		"list_documents",
		"read_document",
		"search_documents",
	}
}

func storyCompletionTools() []string {
	return []string{
		"list_documents",
		"read_document",
		"search_documents",
		"list_story_checklist",
	}
}

func crmDealReviewTools() []string {
	return []string{
		"list_deals",
		"list_buyer_signals",
		"list_contacts",
	}
}
