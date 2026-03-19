package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// validNodeTypes enumerates node types accepted for template nodes.
var validNodeTypes = map[string]bool{
	model.FlowNodeTypeInteractiveAgent: true,
	model.FlowNodeTypeAgentTask:        true,
	model.FlowNodeTypeApprovalGate:     true,
	model.FlowNodeTypeSystemAction:     true,
	model.FlowNodeTypeTerminal:         true,
}

// CreateTemplate creates a new flow template with its nodes.
func (s *FlowService) CreateTemplate(ctx context.Context, workspaceID, actorID string, req model.CreateFlowTemplateRequest) (*model.FlowTemplateView, error) {
	if err := s.validateTemplateRequest(req); err != nil {
		return nil, err
	}

	// Check slug uniqueness within workspace.
	if s.templateRepo != nil {
		existing, err := s.templateRepo.GetBySlug(ctx, workspaceID, req.TemplateSlug)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, fmt.Errorf("template slug %q already exists", req.TemplateSlug)
		}
	}

	t := &model.FlowTemplate{
		WorkspaceID:     &workspaceID,
		Name:            req.Name,
		Description:     req.Description,
		TemplateSlug:    req.TemplateSlug,
		Version:         1,
		TargetType:      req.TargetType,
		InitialNodeSlug: req.InitialNodeSlug,
		IsBuiltin:       false,
		Status:          model.FlowTemplateStatusActive,
		CreatedBy:       &actorID,
	}

	if err := s.templateRepo.CreateTemplate(ctx, t); err != nil {
		return nil, err
	}

	// Create nodes.
	nodes := make([]model.FlowTemplateNode, 0, len(req.Nodes))
	for i, nodeReq := range req.Nodes {
		node := model.FlowTemplateNode{
			TemplateID:           t.ID,
			NodeSlug:             nodeReq.NodeSlug,
			Label:                nodeReq.Label,
			NodeType:             nodeReq.NodeType,
			Position:             nodeReq.Position,
			NextNodeSlug:         nodeReq.NextNodeSlug,
			LoopbackNodeSlug:     nodeReq.LoopbackNodeSlug,
			AgentInputKey:        nodeReq.AgentInputKey,
			SystemPrompt:         nodeReq.SystemPrompt,
			AllowedTools:         defaultJSONArray(nodeReq.AllowedTools),
			OutputTag:            nodeReq.OutputTag,
			Actions:              defaultJSONArray(nodeReq.Actions),
			Retryable:            nodeReq.Retryable,
			CommandName:          nodeReq.CommandName,
			ApproveCommandName:   nodeReq.ApproveCommandName,
			FeedbackFromNode:     nodeReq.FeedbackFromNode,
			AdditionalContextKey: nodeReq.AdditionalContextKey,
			FallbackAgentKey:     nodeReq.FallbackAgentKey,
		}
		if node.Position == 0 {
			node.Position = i
		}
		if err := s.templateRepo.CreateNode(ctx, &node); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	s.logger.InfoContext(ctx, "flow template created",
		"template_id", t.ID,
		"template_slug", t.TemplateSlug,
		"workspace_id", workspaceID,
	)

	return &model.FlowTemplateView{Template: *t, Nodes: nodes}, nil
}

// UpdateTemplate updates an existing flow template's metadata.
func (s *FlowService) UpdateTemplate(ctx context.Context, workspaceID, templateID string, req model.UpdateFlowTemplateRequest) (*model.FlowTemplateView, error) {
	t, err := s.templateRepo.GetByID(ctx, workspaceID, templateID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("flow template not found")
	}
	if t.IsBuiltin && t.WorkspaceID == nil {
		return nil, fmt.Errorf("cannot modify builtin templates")
	}

	if req.Name != nil {
		t.Name = *req.Name
	}
	if req.Description != nil {
		t.Description = req.Description
	}
	if req.InitialNodeSlug != nil {
		t.InitialNodeSlug = *req.InitialNodeSlug
	}
	if req.Status != nil {
		t.Status = *req.Status
	}

	if err := s.templateRepo.UpdateTemplate(ctx, t); err != nil {
		return nil, err
	}
	return &model.FlowTemplateView{Template: *t, Nodes: t.Nodes}, nil
}

// GetTemplate returns a flow template by ID with its nodes.
func (s *FlowService) GetTemplate(ctx context.Context, workspaceID, templateID string) (*model.FlowTemplateView, error) {
	t, err := s.templateRepo.GetByID(ctx, workspaceID, templateID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("flow template not found")
	}
	return &model.FlowTemplateView{Template: *t, Nodes: t.Nodes}, nil
}

// ListDBTemplates returns all templates visible to a workspace.
func (s *FlowService) ListDBTemplates(ctx context.Context, workspaceID string) ([]model.FlowTemplate, error) {
	return s.templateRepo.ListByWorkspace(ctx, workspaceID)
}

// DeleteTemplate archives a flow template.
func (s *FlowService) DeleteTemplate(ctx context.Context, workspaceID, templateID string) error {
	return s.templateRepo.DeleteTemplate(ctx, workspaceID, templateID)
}

// DuplicateTemplate creates an editable workspace copy of a template (typically a builtin).
func (s *FlowService) DuplicateTemplate(ctx context.Context, workspaceID, sourceTemplateID, actorID string) (*model.FlowTemplateView, error) {
	src, err := s.templateRepo.GetByID(ctx, workspaceID, sourceTemplateID)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, fmt.Errorf("flow template not found")
	}

	newSlug := src.TemplateSlug + ".custom"
	copyName := src.Name + " (Copy)"

	t := &model.FlowTemplate{
		WorkspaceID:     &workspaceID,
		Name:            copyName,
		Description:     src.Description,
		TemplateSlug:    newSlug,
		Version:         1,
		TargetType:      src.TargetType,
		InitialNodeSlug: src.InitialNodeSlug,
		IsBuiltin:       false,
		Status:          model.FlowTemplateStatusActive,
		CreatedBy:       &actorID,
	}

	if err := s.templateRepo.CreateTemplate(ctx, t); err != nil {
		return nil, fmt.Errorf("duplicate template: %w", err)
	}

	nodes := make([]model.FlowTemplateNode, 0, len(src.Nodes))
	for _, srcNode := range src.Nodes {
		node := model.FlowTemplateNode{
			TemplateID:           t.ID,
			NodeSlug:             srcNode.NodeSlug,
			Label:                srcNode.Label,
			NodeType:             srcNode.NodeType,
			Position:             srcNode.Position,
			NextNodeSlug:         srcNode.NextNodeSlug,
			LoopbackNodeSlug:     srcNode.LoopbackNodeSlug,
			AgentInputKey:        srcNode.AgentInputKey,
			DefaultAgentID:       srcNode.DefaultAgentID,
			SystemPrompt:         srcNode.SystemPrompt,
			AllowedTools:         srcNode.AllowedTools,
			OutputTag:            srcNode.OutputTag,
			Actions:              srcNode.Actions,
			Retryable:            srcNode.Retryable,
			CommandName:          srcNode.CommandName,
			ApproveCommandName:   srcNode.ApproveCommandName,
			FeedbackFromNode:     srcNode.FeedbackFromNode,
			AdditionalContextKey: srcNode.AdditionalContextKey,
			FallbackAgentKey:     srcNode.FallbackAgentKey,
		}
		if err := s.templateRepo.CreateNode(ctx, &node); err != nil {
			return nil, fmt.Errorf("duplicate template node %q: %w", srcNode.NodeSlug, err)
		}
		nodes = append(nodes, node)
	}

	s.logger.InfoContext(ctx, "flow template duplicated",
		"source_template_id", sourceTemplateID,
		"new_template_id", t.ID,
		"workspace_id", workspaceID,
	)

	return &model.FlowTemplateView{Template: *t, Nodes: nodes}, nil
}

// DuplicateFromSlug creates an editable workspace copy of a template identified by slug.
// This works for both DB-backed templates and hardcoded system templates.
func (s *FlowService) DuplicateFromSlug(ctx context.Context, workspaceID, templateSlug, actorID string) (*model.FlowTemplateView, error) {
	// Try DB first.
	if s.templateRepo != nil {
		src, err := s.templateRepo.GetBySlug(ctx, workspaceID, templateSlug)
		if err != nil {
			return nil, err
		}
		if src != nil {
			return s.DuplicateTemplate(ctx, workspaceID, src.ID, actorID)
		}
	}

	// Fall back to hardcoded system template.
	def, ok := lookupHardcodedFlowTemplate(templateSlug)
	if !ok {
		return nil, fmt.Errorf("flow template %q not found", templateSlug)
	}

	spec := def.spec
	newSlug := templateSlug + ".custom"
	copyName := spec.Name + " (Copy)"
	description := spec.Description

	t := &model.FlowTemplate{
		WorkspaceID:     &workspaceID,
		Name:            copyName,
		Description:     &description,
		TemplateSlug:    newSlug,
		Version:         1,
		TargetType:      spec.TargetType,
		InitialNodeSlug: def.initialNodeID,
		IsBuiltin:       false,
		Status:          model.FlowTemplateStatusActive,
		CreatedBy:       &actorID,
	}

	if err := s.templateRepo.CreateTemplate(ctx, t); err != nil {
		return nil, fmt.Errorf("duplicate system template: %w", err)
	}

	nodes := make([]model.FlowTemplateNode, 0, len(spec.Nodes))
	for i, nodeSpec := range spec.Nodes {
		node := model.FlowTemplateNode{
			TemplateID: t.ID,
			NodeSlug:   nodeSpec.ID,
			Label:      nodeSpec.Label,
			NodeType:   nodeSpec.Type,
			Position:   i,
			AllowedTools: defaultJSONArray(mustMarshalStringSlice(nodeSpec.AllowedTools)),
			Actions:      defaultJSONArray(mustMarshalStringSlice(nodeSpec.Actions)),
		}
		if nodeSpec.CommandName != nil {
			node.CommandName = nodeSpec.CommandName
		}
		if nodeSpec.LoopbackNodeID != nil {
			node.LoopbackNodeSlug = nodeSpec.LoopbackNodeID
		}
		// Resolve next node and additional fields from the hardcoded node definitions.
		if nodeDef, nok := def.nodes[nodeSpec.ID]; nok {
			if nodeDef.nextNodeID != "" {
				next := nodeDef.nextNodeID
				node.NextNodeSlug = &next
			}
			node.Retryable = nodeDef.retryable
			if nodeDef.commandName != "" {
				cmd := nodeDef.commandName
				node.CommandName = &cmd
			}
			if nodeDef.approveCommandName != "" {
				acn := nodeDef.approveCommandName
				node.ApproveCommandName = &acn
			}
		}
		// Populate system prompt for interactive/agent nodes from known stages.
		if node.NodeType == model.FlowNodeTypeInteractiveAgent || node.NodeType == model.FlowNodeTypeAgentTask {
			if stage, ok := nodeSlugToPromptStage[nodeSpec.ID]; ok && s.planningService != nil {
				prompt := s.planningService.DefaultSystemPrompt(stage)
				node.SystemPrompt = &prompt
			}
		}
		if err := s.templateRepo.CreateNode(ctx, &node); err != nil {
			return nil, fmt.Errorf("duplicate system template node %q: %w", nodeSpec.ID, err)
		}
		nodes = append(nodes, node)
	}

	s.logger.InfoContext(ctx, "system flow template duplicated",
		"source_slug", templateSlug,
		"new_template_id", t.ID,
		"workspace_id", workspaceID,
	)

	return &model.FlowTemplateView{Template: *t, Nodes: nodes}, nil
}

// outputTagToPromptStage maps output_tag values to planning session stages.
var outputTagToPromptStage = map[string]string{
	"spec_draft": model.PlanningSessionStageDraftSpec,
	"story_plan": model.PlanningSessionStagePlanStories,
}

// nodeSlugToPromptStage maps known hardcoded node slugs to planning session stages.
// Used when duplicating hardcoded templates that don't have output_tag on the spec.
var nodeSlugToPromptStage = map[string]string{
	model.FlowNodeSpecDraft: model.PlanningSessionStageDraftSpec,
	model.FlowNodeStoryPlan: model.PlanningSessionStagePlanStories,
}

// resolveDefaultPrompt returns the default system prompt for a node if its
// output_tag maps to a known planning session stage.
func (s *FlowService) resolveDefaultPrompt(node *model.FlowTemplateNode) *string {
	if s.planningService == nil || node.OutputTag == nil {
		return nil
	}
	stage, ok := outputTagToPromptStage[*node.OutputTag]
	if !ok {
		return nil
	}
	prompt := s.planningService.DefaultSystemPrompt(stage)
	return &prompt
}

// BackfillBuiltinPrompts populates system_prompt on builtin template nodes
// that have interactive_agent or agent_task type but no stored prompt.
// This is idempotent and called on startup.
func (s *FlowService) BackfillBuiltinPrompts(ctx context.Context) {
	if s.templateRepo == nil || s.planningService == nil {
		return
	}
	// ListByWorkspace with empty workspace returns only builtins.
	templates, err := s.templateRepo.ListByWorkspace(ctx, "")
	if err != nil {
		s.logger.WarnContext(ctx, "backfill builtin prompts: failed to list templates", "error", err)
		return
	}
	for _, t := range templates {
		if !t.IsBuiltin || t.WorkspaceID != nil {
			continue
		}
		for i := range t.Nodes {
			node := &t.Nodes[i]
			if node.NodeType != model.FlowNodeTypeInteractiveAgent && node.NodeType != model.FlowNodeTypeAgentTask {
				continue
			}
			if node.SystemPrompt != nil && *node.SystemPrompt != "" {
				continue
			}
			prompt := s.resolveDefaultPrompt(node)
			if prompt == nil {
				continue
			}
			node.SystemPrompt = prompt
			if err := s.templateRepo.UpdateNode(ctx, node); err != nil {
				s.logger.WarnContext(ctx, "backfill builtin prompt: failed to update node",
					"template_id", t.ID, "node_slug", node.NodeSlug, "error", err)
			} else {
				s.logger.InfoContext(ctx, "backfilled system prompt on builtin node",
					"template_id", t.ID, "node_slug", node.NodeSlug)
			}
		}
	}
}

func mustMarshalStringSlice(s []string) json.RawMessage {
	if len(s) == 0 {
		return json.RawMessage("[]")
	}
	b, _ := json.Marshal(s)
	return b
}

// --- Node CRUD ---

// CreateTemplateNode adds a node to a template.
func (s *FlowService) CreateTemplateNode(ctx context.Context, workspaceID, templateID string, req model.CreateFlowTemplateNodeRequest) (*model.FlowTemplateNode, error) {
	t, err := s.templateRepo.GetByID(ctx, workspaceID, templateID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("flow template not found")
	}
	if t.IsBuiltin && t.WorkspaceID == nil {
		return nil, fmt.Errorf("cannot modify builtin templates")
	}

	if err := s.validateNodeRequest(req); err != nil {
		return nil, err
	}

	node := &model.FlowTemplateNode{
		TemplateID:           templateID,
		NodeSlug:             req.NodeSlug,
		Label:                req.Label,
		NodeType:             req.NodeType,
		Position:             req.Position,
		NextNodeSlug:         req.NextNodeSlug,
		LoopbackNodeSlug:     req.LoopbackNodeSlug,
		AgentInputKey:        req.AgentInputKey,
		DefaultAgentID:       req.DefaultAgentID,
		SystemPrompt:         req.SystemPrompt,
		AllowedTools:         defaultJSONArray(req.AllowedTools),
		OutputTag:            req.OutputTag,
		Actions:              defaultJSONArray(req.Actions),
		Retryable:            req.Retryable,
		CommandName:          req.CommandName,
		ApproveCommandName:   req.ApproveCommandName,
		FeedbackFromNode:     req.FeedbackFromNode,
		AdditionalContextKey: req.AdditionalContextKey,
		FallbackAgentKey:     req.FallbackAgentKey,
	}
	if err := s.templateRepo.CreateNode(ctx, node); err != nil {
		return nil, err
	}
	return node, nil
}

// UpdateTemplateNode updates a node within a template.
func (s *FlowService) UpdateTemplateNode(ctx context.Context, workspaceID, templateID, nodeID string, req model.UpdateFlowTemplateNodeRequest) (*model.FlowTemplateNode, error) {
	t, err := s.templateRepo.GetByID(ctx, workspaceID, templateID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, fmt.Errorf("flow template not found")
	}
	if t.IsBuiltin && t.WorkspaceID == nil {
		return nil, fmt.Errorf("cannot modify builtin templates")
	}

	node, err := s.templateRepo.GetNodeByID(ctx, templateID, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("flow template node not found")
	}

	if req.Label != nil {
		node.Label = *req.Label
	}
	if req.NodeType != nil {
		node.NodeType = *req.NodeType
	}
	if req.Position != nil {
		node.Position = *req.Position
	}
	if req.NextNodeSlug != nil {
		node.NextNodeSlug = req.NextNodeSlug
	}
	if req.LoopbackNodeSlug != nil {
		node.LoopbackNodeSlug = req.LoopbackNodeSlug
	}
	if req.AgentInputKey != nil {
		node.AgentInputKey = req.AgentInputKey
	}
	if req.DefaultAgentID != nil {
		node.DefaultAgentID = req.DefaultAgentID
	}
	if req.SystemPrompt != nil {
		node.SystemPrompt = req.SystemPrompt
	}
	if req.AllowedTools != nil {
		node.AllowedTools = *req.AllowedTools
	}
	if req.OutputTag != nil {
		node.OutputTag = req.OutputTag
	}
	if req.Actions != nil {
		node.Actions = *req.Actions
	}
	if req.Retryable != nil {
		node.Retryable = *req.Retryable
	}
	if req.CommandName != nil {
		node.CommandName = req.CommandName
	}
	if req.ApproveCommandName != nil {
		node.ApproveCommandName = req.ApproveCommandName
	}
	if req.FeedbackFromNode != nil {
		node.FeedbackFromNode = req.FeedbackFromNode
	}
	if req.AdditionalContextKey != nil {
		node.AdditionalContextKey = req.AdditionalContextKey
	}
	if req.FallbackAgentKey != nil {
		node.FallbackAgentKey = req.FallbackAgentKey
	}

	if err := s.templateRepo.UpdateNode(ctx, node); err != nil {
		return nil, err
	}
	return node, nil
}

// DeleteTemplateNode removes a node from a template.
func (s *FlowService) DeleteTemplateNode(ctx context.Context, workspaceID, templateID, nodeID string) error {
	t, err := s.templateRepo.GetByID(ctx, workspaceID, templateID)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("flow template not found")
	}
	if t.IsBuiltin && t.WorkspaceID == nil {
		return fmt.Errorf("cannot modify builtin templates")
	}
	return s.templateRepo.DeleteNode(ctx, templateID, nodeID)
}

// --- Validation ---

func (s *FlowService) validateTemplateRequest(req model.CreateFlowTemplateRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(req.TemplateSlug) == "" {
		return fmt.Errorf("template_slug is required")
	}
	if strings.TrimSpace(req.TargetType) == "" {
		return fmt.Errorf("target_type is required")
	}
	if strings.TrimSpace(req.InitialNodeSlug) == "" {
		return fmt.Errorf("initial_node_slug is required")
	}
	if len(req.Nodes) == 0 {
		return fmt.Errorf("at least one node is required")
	}
	for _, node := range req.Nodes {
		if err := s.validateNodeRequest(node); err != nil {
			return err
		}
	}
	// Verify initial node exists.
	found := false
	for _, node := range req.Nodes {
		if node.NodeSlug == req.InitialNodeSlug {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("initial_node_slug %q must reference one of the template nodes", req.InitialNodeSlug)
	}
	return nil
}

func (s *FlowService) validateNodeRequest(req model.CreateFlowTemplateNodeRequest) error {
	if strings.TrimSpace(req.NodeSlug) == "" {
		return fmt.Errorf("node_slug is required")
	}
	if strings.TrimSpace(req.Label) == "" {
		return fmt.Errorf("label is required")
	}
	if !validNodeTypes[req.NodeType] {
		return fmt.Errorf("invalid node_type %q", req.NodeType)
	}
	return nil
}

func defaultJSONArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	return raw
}
