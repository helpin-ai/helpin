package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// flowTemplateResolver converts DB-backed FlowTemplate records into the runtime
// flowTemplateDefinition format used by the flow engine. It implements dual lookup:
// DB templates take priority, with hardcoded templates as fallback.
type flowTemplateResolver struct {
	templateRepo *repository.FlowTemplateRepository
}

func newFlowTemplateResolver(templateRepo *repository.FlowTemplateRepository) *flowTemplateResolver {
	return &flowTemplateResolver{templateRepo: templateRepo}
}

// resolveTemplate looks up a template by slug. DB templates (both workspace-specific
// and builtins) take priority, with hardcoded Go definitions as a last-resort fallback.
// The generic DB execution path now includes agent validation, fallbacks, and input
// normalization, so DB-backed builtins no longer need the hardcoded Go paths.
func (r *flowTemplateResolver) resolveTemplate(ctx context.Context, workspaceID, templateSlug string) (flowTemplateDefinition, bool) {
	if r.templateRepo != nil {
		wsID := workspaceID
		if wsID == "" {
			// Temporal activities may not carry workspace context; builtin
			// templates have NULL workspace_id so the repo query still works.
			wsID = "00000000-0000-0000-0000-000000000000"
		}
		dbTemplate, err := r.templateRepo.GetBySlug(ctx, wsID, templateSlug)
		if err == nil && dbTemplate != nil && len(dbTemplate.Nodes) > 0 {
			def, err := r.buildDefinitionFromDB(dbTemplate)
			if err == nil {
				return def, true
			}
		}
	}
	// Use hardcoded Go definitions as fallback for templates not yet in DB.
	def, ok := lookupHardcodedFlowTemplate(templateSlug)
	return def, ok
}

// listTemplateSpecs returns FlowSpec objects for all templates visible to a workspace.
func (r *flowTemplateResolver) listTemplateSpecs(ctx context.Context, workspaceID string) []model.FlowSpec {
	// Start with hardcoded specs.
	hardcoded := hardcodedFlowTemplateSpecs()
	seen := make(map[string]bool, len(hardcoded))
	for _, spec := range hardcoded {
		seen[spec.TemplateID] = true
	}

	// Add DB templates that don't shadow hardcoded ones (or replace them).
	if r.templateRepo != nil && workspaceID != "" {
		dbTemplates, err := r.templateRepo.ListByWorkspace(ctx, workspaceID)
		if err == nil {
			for i := range dbTemplates {
				t := &dbTemplates[i]
				spec := r.dbTemplateToFlowSpec(t)
				if seen[spec.TemplateID] {
					// DB template overrides hardcoded — replace in the list.
					for j, hs := range hardcoded {
						if hs.TemplateID == spec.TemplateID {
							hardcoded[j] = spec
							break
						}
					}
				} else {
					hardcoded = append(hardcoded, spec)
					seen[spec.TemplateID] = true
				}
			}
		}
	}
	return hardcoded
}

// buildDefinitionFromDB converts a DB FlowTemplate with nodes into a flowTemplateDefinition.
func (r *flowTemplateResolver) buildDefinitionFromDB(t *model.FlowTemplate) (flowTemplateDefinition, error) {
	if len(t.Nodes) == 0 {
		return flowTemplateDefinition{}, fmt.Errorf("template %q has no nodes", t.TemplateSlug)
	}

	nodeMap := make(map[string]flowNodeDefinition, len(t.Nodes))
	nodeSpecs := make([]model.FlowNodeSpec, 0, len(t.Nodes))

	for i := range t.Nodes {
		node := &t.Nodes[i]
		nodeDef := r.buildNodeDefinition(node)
		nodeMap[node.NodeSlug] = nodeDef
		nodeSpecs = append(nodeSpecs, nodeDef.spec)
	}

	templateName := t.Name
	if templateName == "" {
		templateName = t.TemplateSlug
	}
	description := ""
	if t.Description != nil {
		description = *t.Description
	}

	def := flowTemplateDefinition{
		spec: model.FlowSpec{
			TemplateID:      t.TemplateSlug,
			Name:            templateName,
			Description:     description,
			TemplateVersion: t.Version,
			TargetType:      t.TargetType,
			Nodes:           nodeSpecs,
		},
		nodes:         nodeMap,
		initialNodeID: t.InitialNodeSlug,
		startRun: func(ctx context.Context, svc *FlowService, workspaceID, actorID string, req model.StartFlowRunRequest) (*model.FlowRunView, error) {
			return svc.startGenericDBFlow(ctx, workspaceID, actorID, req, t)
		},
		clearActiveTarget: func(ctx context.Context, svc *FlowService, run *model.FlowRun) error {
			return svc.clearActiveTargetForRun(ctx, run)
		},
	}

	return def, nil
}

// buildNodeDefinition converts a single DB FlowTemplateNode into a flowNodeDefinition.
func (r *flowTemplateResolver) buildNodeDefinition(node *model.FlowTemplateNode) flowNodeDefinition {
	allowedTools := node.AllowedToolsList()
	actions := node.ActionsList()
	requiredMode := node.RequiredMode()

	spec := model.FlowNodeSpec{
		ID:           node.NodeSlug,
		Label:        node.Label,
		Type:         node.NodeType,
		RequiredMode: requiredMode,
		Actions:      actions,
		AllowedTools: allowedTools,
	}
	if node.CommandName != nil && *node.CommandName != "" {
		spec.CommandName = node.CommandName
	}
	if node.LoopbackNodeSlug != nil && *node.LoopbackNodeSlug != "" {
		spec.LoopbackNodeID = node.LoopbackNodeSlug
	}

	def := flowNodeDefinition{
		spec:               spec,
		nextNodeID:         derefString(node.NextNodeSlug),
		retryable:          node.Retryable,
		commandName:        derefString(node.CommandName),
		approveCommandName: derefString(node.ApproveCommandName),
	}

	// Build generic callbacks based on node type and config.
	switch node.NodeType {
	case model.FlowNodeTypeInteractiveAgent:
		nodeCopy := *node // capture for closure
		def.buildInteractiveLaunch = func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (flowInteractiveLaunch, error) {
			return svc.buildGenericInteractiveLaunch(ctx, run, &nodeCopy)
		}

	case model.FlowNodeTypeAgentTask:
		nodeCopy := *node
		def.buildAgentTaskLaunch = func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (flowAgentTaskLaunch, error) {
			return svc.buildGenericAgentTaskLaunch(ctx, run, &nodeCopy)
		}

	case model.FlowNodeTypeSystemAction:
		if node.CommandName != nil && *node.CommandName != "" {
			nodeCopy := *node
			def.buildCommandInput = func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) (json.RawMessage, error) {
				return svc.buildGenericCommandInput(ctx, run, &nodeCopy)
			}
		}

	case model.FlowNodeTypeApprovalGate:
		if node.ApproveCommandName != nil && *node.ApproveCommandName != "" {
			cmdName := *node.ApproveCommandName
			def.buildApproveCommandInput = func(ctx context.Context, svc *FlowService, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string, payload json.RawMessage) (json.RawMessage, error) {
				switch cmdName {
				case "pm.approve_epic_spec":
					return svc.buildApproveSpecCommandInput(payload)
				default:
					return payload, nil
				}
			}
		}
	}

	return def
}

// dbTemplateToFlowSpec converts a DB template to a FlowSpec for API responses.
func (r *flowTemplateResolver) dbTemplateToFlowSpec(t *model.FlowTemplate) model.FlowSpec {
	nodes := make([]model.FlowNodeSpec, 0, len(t.Nodes))
	for i := range t.Nodes {
		node := &t.Nodes[i]
		spec := model.FlowNodeSpec{
			ID:           node.NodeSlug,
			Label:        node.Label,
			Type:         node.NodeType,
			RequiredMode: node.RequiredMode(),
			Actions:      node.ActionsList(),
			AllowedTools: node.AllowedToolsList(),
		}
		if node.CommandName != nil {
			spec.CommandName = node.CommandName
		}
		if node.LoopbackNodeSlug != nil {
			spec.LoopbackNodeID = node.LoopbackNodeSlug
		}
		nodes = append(nodes, spec)
	}
	description := ""
	if t.Description != nil {
		description = *t.Description
	}
	return model.FlowSpec{
		TemplateID:      t.TemplateSlug,
		Name:            t.Name,
		Description:     description,
		TemplateVersion: t.Version,
		TargetType:      t.TargetType,
		Nodes:           nodes,
	}
}

// --- Generic launch builders for DB-backed templates ---

// buildGenericInteractiveLaunch builds a launch config for interactive agent nodes
// using node config from the DB rather than hardcoded logic.
func (s *FlowService) buildGenericInteractiveLaunch(ctx context.Context, run *model.FlowRun, node *model.FlowTemplateNode) (flowInteractiveLaunch, error) {
	agentID, err := s.resolveAgentIDFromFlowInput(run, node)
	if err != nil {
		return flowInteractiveLaunch{}, err
	}

	additionalContext := s.resolveAdditionalContext(ctx, run, node)

	// Determine stage from output_tag.
	stage := ""
	if node.OutputTag != nil {
		switch *node.OutputTag {
		case "spec_draft":
			stage = model.PlanningSessionStageDraftSpec
		case "story_plan":
			stage = model.PlanningSessionStagePlanStories
		default:
			stage = *node.OutputTag
		}
	}

	return flowInteractiveLaunch{
		AgentID:           agentID,
		AdditionalContext: additionalContext,
		AllowedTools:      node.AllowedToolsList(),
		Stage:             stage,
		SystemPrompt:      derefString(node.SystemPrompt),
	}, nil
}

// buildGenericAgentTaskLaunch builds a launch config for agent task nodes.
func (s *FlowService) buildGenericAgentTaskLaunch(ctx context.Context, run *model.FlowRun, node *model.FlowTemplateNode) (flowAgentTaskLaunch, error) {
	agentID, err := s.resolveAgentIDFromFlowInput(run, node)
	if err != nil {
		return flowAgentTaskLaunch{}, err
	}

	additionalContext := s.resolveAdditionalContext(ctx, run, node)

	payload := flowTaskRunInput{
		AdditionalContext: additionalContext,
		AllowedTools:      node.AllowedToolsList(),
	}
	if node.OutputTag != nil && *node.OutputTag != "" {
		payload.FlowOutputKind = *node.OutputTag
	}

	return flowAgentTaskLaunch{
		AgentID:      agentID,
		Input:        mustJSON(payload),
		AllowedTools: node.AllowedToolsList(),
		SystemPrompt: derefString(node.SystemPrompt),
	}, nil
}

// buildGenericCommandInput builds command input from the flow run state.
// Dispatches to specialized builders for known commands, falls back to
// passing the flow run input through.
func (s *FlowService) buildGenericCommandInput(ctx context.Context, run *model.FlowRun, node *model.FlowTemplateNode) (json.RawMessage, error) {
	commandName := derefString(node.CommandName)
	switch commandName {
	case "pm.create_story_batch":
		return s.buildCreateStoriesCommandInput(ctx, run)
	case "pm.create_followup_stories":
		return s.buildStoryCompletionCommandInput(ctx, run)
	case "crm.apply_deal_actions":
		return s.buildCRMDealReviewCommandInput(ctx, run)
	default:
		return run.Input, nil
	}
}

// resolveAgentIDFromFlowInput extracts the agent ID from the flow run input using
// the node's agent_input_key configuration.
func (s *FlowService) resolveAgentIDFromFlowInput(run *model.FlowRun, node *model.FlowTemplateNode) (string, error) {
	key := "agent_id"
	if node.AgentInputKey != nil && *node.AgentInputKey != "" {
		key = *node.AgentInputKey
	}

	var inputMap map[string]json.RawMessage
	if err := json.Unmarshal(run.Input, &inputMap); err != nil {
		return "", fmt.Errorf("parse flow input: %w", err)
	}

	raw, ok := inputMap[key]
	if !ok {
		// Fall back to default agent from template node config.
		if node.DefaultAgentID != nil && *node.DefaultAgentID != "" {
			return *node.DefaultAgentID, nil
		}
		return "", fmt.Errorf("agent input key %q not found in flow input", key)
	}

	var agentID string
	if err := json.Unmarshal(raw, &agentID); err != nil {
		return "", fmt.Errorf("invalid agent_id value for key %q: %w", key, err)
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		// Fall back to default agent from template node config.
		if node.DefaultAgentID != nil && *node.DefaultAgentID != "" {
			return *node.DefaultAgentID, nil
		}
		return "", fmt.Errorf("agent_id is empty for key %q", key)
	}
	return agentID, nil
}

// resolveAdditionalContext builds the additional context string for a node,
// including feedback from previous approval gates and context from the flow input.
func (s *FlowService) resolveAdditionalContext(ctx context.Context, run *model.FlowRun, node *model.FlowTemplateNode) string {
	var parts []string

	// Extract additional_context from flow input.
	var inputMap map[string]json.RawMessage
	if err := json.Unmarshal(run.Input, &inputMap); err == nil {
		contextKey := "additional_context"
		if node.AdditionalContextKey != nil && *node.AdditionalContextKey != "" {
			contextKey = *node.AdditionalContextKey
		}
		if raw, ok := inputMap[contextKey]; ok {
			var text string
			if err := json.Unmarshal(raw, &text); err == nil {
				parts = append(parts, strings.TrimSpace(text))
			}
		}
	}

	// Include feedback from a prior approval node if configured.
	if node.FeedbackFromNode != nil && *node.FeedbackFromNode != "" {
		if feedback, err := s.latestReviewFeedback(ctx, run.ID, *node.FeedbackFromNode); err == nil && feedback != "" {
			parts = append(parts, "Reviewer feedback:\n"+feedback)
		}
	}

	return joinFlowContext(parts...)
}

// startGenericDBFlow starts a flow run for a DB-backed template using generic input resolution.
// It validates the target, resolves/validates all agent references, and normalizes the input
// before handing off to startConfiguredRun.
func (s *FlowService) startGenericDBFlow(ctx context.Context, workspaceID, actorID string, req model.StartFlowRunRequest, t *model.FlowTemplate) (*model.FlowRunView, error) {
	// Validate target exists.
	if err := s.validateFlowTarget(ctx, workspaceID, req.TargetType, req.TargetID); err != nil {
		return nil, err
	}

	// Parse input.
	var inputMap map[string]json.RawMessage
	if len(req.Input) > 0 {
		if err := json.Unmarshal(req.Input, &inputMap); err != nil {
			return nil, fmt.Errorf("invalid flow input: %w", err)
		}
	}
	if inputMap == nil {
		inputMap = map[string]json.RawMessage{}
	}

	// Resolve, normalize, and validate all agent references before starting the run.
	if err := s.resolveAndValidateAgents(ctx, workspaceID, req.TargetType, req.TargetID, t, inputMap); err != nil {
		return nil, err
	}

	// Build spec snapshot.
	specSnapshot := map[string]any{
		"template_id":     req.TemplateID,
		"template_slug":   t.TemplateSlug,
		"target_type":     req.TargetType,
		"target_id":       req.TargetID,
		"started_by":      actorID,
		"template_source": "db",
	}

	return s.startConfiguredRun(ctx, workspaceID, actorID, req, inputMap, specSnapshot)
}

// resolveAndValidateAgents iterates over all agent-bearing nodes in the template,
// resolves agent IDs via fallback chain (input key → fallback input key → default agent →
// target-level default), validates each agent exists and supports the required mode,
// and back-populates resolved IDs into inputMap so downstream node launchers find them.
func (s *FlowService) resolveAndValidateAgents(ctx context.Context, workspaceID, targetType, targetID string, t *model.FlowTemplate, inputMap map[string]json.RawMessage) error {
	for _, node := range t.Nodes {
		if node.NodeType != model.FlowNodeTypeInteractiveAgent && node.NodeType != model.FlowNodeTypeAgentTask {
			continue
		}

		key := "agent_id"
		if node.AgentInputKey != nil && *node.AgentInputKey != "" {
			key = *node.AgentInputKey
		}

		agentID := extractStringFromInput(inputMap, key)

		// Fallback 1: copy from another input key (e.g. story_planner → spec_planner).
		if agentID == "" && node.FallbackAgentKey != nil && *node.FallbackAgentKey != "" {
			agentID = extractStringFromInput(inputMap, *node.FallbackAgentKey)
		}

		// Fallback 2: node's static DefaultAgentID.
		if agentID == "" && node.DefaultAgentID != nil && *node.DefaultAgentID != "" {
			agentID = *node.DefaultAgentID
		}

		// Fallback 3: target-level default (e.g. epic → OrchestratorAgentID).
		if agentID == "" {
			agentID = s.resolveTargetDefaultAgent(ctx, targetType, targetID)
		}

		if agentID == "" {
			return fmt.Errorf("agent is required for node %q (input key %q)", node.NodeSlug, key)
		}

		// Validate agent exists and supports the required mode.
		agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
		if err != nil || agent == nil {
			return fmt.Errorf("agent not found for node %q: %s", node.NodeSlug, agentID)
		}
		normalizeAgentRecord(agent)
		if err := validateAgentTarget(agent, targetType); err != nil {
			return err
		}
		requiredMode := node.RequiredMode()
		if requiredMode != "" && !agentSupportsMode(agent, requiredMode) {
			return fmt.Errorf("agent %q does not support %s mode required by node %q", agent.Name, requiredMode, node.NodeSlug)
		}

		// Back-populate into inputMap so downstream resolvers find the value.
		inputMap[key] = mustJSON(agentID)
	}
	return nil
}

// resolveTargetDefaultAgent extracts the default agent from the target entity.
// For epics this is the OrchestratorAgentID; for stories it is the assigned agent.
func (s *FlowService) resolveTargetDefaultAgent(ctx context.Context, targetType, targetID string) string {
	if targetType == "epic" {
		epicWithStats, err := s.epicRepo.GetByID(ctx, targetID)
		if err == nil && epicWithStats != nil {
			return derefString(epicWithStats.Epic.OrchestratorAgentID)
		}
	}
	if targetType == "story" && s.storyRepo != nil {
		story, err := s.storyRepo.GetRawByID(ctx, targetID)
		if err == nil && story != nil {
			return derefString(story.AssignedAgentID)
		}
	}
	return ""
}

// extractStringFromInput extracts a trimmed string value from a JSON input map.
func extractStringFromInput(inputMap map[string]json.RawMessage, key string) string {
	raw, ok := inputMap[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

// validateFlowTarget validates that the target entity exists.
func (s *FlowService) validateFlowTarget(ctx context.Context, workspaceID, targetType, targetID string) error {
	switch targetType {
	case "epic":
		epicWithStats, err := s.epicRepo.GetByID(ctx, targetID)
		if err != nil {
			return fmt.Errorf("get epic: %w", err)
		}
		if epicWithStats == nil || epicWithStats.Epic.WorkspaceID != workspaceID {
			return fmt.Errorf("epic not found")
		}
	case "story":
		if s.storyRepo == nil {
			return fmt.Errorf("story repository is not configured")
		}
		story, err := s.storyRepo.GetRawByID(ctx, targetID)
		if err != nil {
			return fmt.Errorf("get story: %w", err)
		}
		if story == nil || story.WorkspaceID != workspaceID {
			return fmt.Errorf("story not found")
		}
	case "crm_deal":
		if s.crmDealRepo == nil {
			return fmt.Errorf("CRM deal repository is not configured")
		}
		deal, err := s.crmDealRepo.GetByID(ctx, targetID)
		if err != nil {
			return fmt.Errorf("get deal: %w", err)
		}
		if deal == nil || deal.WorkspaceID != workspaceID {
			return fmt.Errorf("deal not found")
		}
	}
	return nil
}
