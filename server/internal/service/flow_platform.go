package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

type flowTaskRunInput struct {
	AdditionalContext string   `json:"additional_context,omitempty"`
	AllowedTools      []string `json:"allowed_tools,omitempty"`
	FlowOutputKind    string   `json:"flow_output_kind,omitempty"`
}

func (s *FlowService) startConfiguredEpicPlanningFlow(ctx context.Context, workspaceID, actorID string, req model.StartFlowRunRequest) (*model.FlowRunView, error) {
	epicWithStats, err := s.epicRepo.GetByID(ctx, req.TargetID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epicWithStats == nil || epicWithStats.Epic.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("epic not found")
	}
	input, specPlannerID, storyPlannerID, err := s.resolveEpicPlanningInput(ctx, workspaceID, &epicWithStats.Epic, req.Input)
	if err != nil {
		return nil, err
	}
	specSnapshot := map[string]any{
		"template_id":            req.TemplateID,
		"target_type":            req.TargetType,
		"target_id":              req.TargetID,
		"spec_planner_agent_id":  specPlannerID,
		"story_planner_agent_id": storyPlannerID,
		"additional_context":     input.AdditionalContext,
		"started_by":             actorID,
	}
	return s.startConfiguredRun(ctx, workspaceID, actorID, req, input, specSnapshot)
}

func (s *FlowService) startStoryCompletionFlow(ctx context.Context, workspaceID, actorID string, req model.StartFlowRunRequest) (*model.FlowRunView, error) {
	if s.storyRepo == nil {
		return nil, fmt.Errorf("story repository is not configured")
	}
	story, err := s.storyRepo.GetRawByID(ctx, req.TargetID)
	if err != nil {
		return nil, fmt.Errorf("get story: %w", err)
	}
	if story == nil || story.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("story not found")
	}
	input, err := s.resolveStoryCompletionInput(ctx, workspaceID, req.Input)
	if err != nil {
		return nil, err
	}
	specSnapshot := map[string]any{
		"template_id":        req.TemplateID,
		"target_type":        req.TargetType,
		"target_id":          req.TargetID,
		"agent_id":           input.AgentID,
		"additional_context": input.AdditionalContext,
		"story_name":         story.Name,
		"started_by":         actorID,
	}
	return s.startConfiguredRun(ctx, workspaceID, actorID, req, input, specSnapshot)
}

func (s *FlowService) StartAgentStoryRun(ctx context.Context, workspaceID, actorID, storyID, agentID string) (*model.FlowRunView, error) {
	if s.storyRepo == nil {
		return nil, fmt.Errorf("story repository is not configured")
	}
	story, err := s.storyRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return nil, fmt.Errorf("get story: %w", err)
	}
	if story == nil || story.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("story not found")
	}

	resolvedAgentID := strings.TrimSpace(agentID)
	if resolvedAgentID == "" {
		resolvedAgentID = strings.TrimSpace(derefString(story.AssignedAgentID))
	}
	if resolvedAgentID == "" {
		return nil, fmt.Errorf("no agent assigned to this story")
	}

	if s.agentService == nil {
		return nil, fmt.Errorf("agent service is not configured")
	}
	agent, err := s.agentService.requireRunnableAgent(ctx, workspaceID, resolvedAgentID, "story")
	if err != nil {
		return nil, err
	}
	if !agentSupportsMode(agent, model.InvocationModeAutonomous) {
		return nil, fmt.Errorf("selected agent does not support autonomous mode")
	}

	profile := worker.GetRuntimeProfile(agent.CapabilityProfile)
	if s.agentService.gitService != nil {
		if _, err := s.agentService.gitService.ResolveStoryDeliveryTargetForRun(ctx, workspaceID, storyID, profile); err != nil {
			return nil, err
		}
	}

	req := model.StartFlowRunRequest{
		TemplateID: model.FlowTemplateAgentStoryRun,
		TargetType: "story",
		TargetID:   storyID,
		Input:      mustJSON(map[string]any{"agent_id": resolvedAgentID}),
	}
	return s.StartRun(ctx, workspaceID, actorID, req)
}

func (s *FlowService) startCRMDealReviewFlow(ctx context.Context, workspaceID, actorID string, req model.StartFlowRunRequest) (*model.FlowRunView, error) {
	if s.crmDealRepo == nil {
		return nil, fmt.Errorf("CRM deal repository is not configured")
	}
	deal, err := s.crmDealRepo.GetByID(ctx, req.TargetID)
	if err != nil {
		return nil, fmt.Errorf("get deal: %w", err)
	}
	if deal == nil || deal.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("deal not found")
	}
	input, err := s.resolveCRMDealReviewInput(ctx, workspaceID, req.Input)
	if err != nil {
		return nil, err
	}
	specSnapshot := map[string]any{
		"template_id":        req.TemplateID,
		"target_type":        req.TargetType,
		"target_id":          req.TargetID,
		"agent_id":           input.AgentID,
		"additional_context": input.AdditionalContext,
		"deal_name":          deal.Name,
		"started_by":         actorID,
	}
	return s.startConfiguredRun(ctx, workspaceID, actorID, req, input, specSnapshot)
}

func (s *FlowService) startConfiguredRun(ctx context.Context, workspaceID, actorID string, req model.StartFlowRunRequest, input any, specSnapshot map[string]any) (*model.FlowRunView, error) {
	def, ok := s.resolveTemplate(ctx, workspaceID, strings.TrimSpace(req.TemplateID))
	if !ok {
		return nil, fmt.Errorf("unsupported flow template: %s", req.TemplateID)
	}
	active, err := s.flowRepo.GetActiveRunByTarget(ctx, workspaceID, req.TemplateID, req.TargetType, req.TargetID)
	if err != nil {
		return nil, err
	}
	if active != nil {
		if err := s.ensureCurrentNodeAttempt(ctx, active, actorID); err != nil {
			s.logger.WarnContext(ctx, "failed to recover active flow run before reuse",
				"flow_run_id", active.ID,
				"template_id", active.TemplateID,
				"target_type", active.TargetType,
				"target_id", active.TargetID,
				"error", err,
			)
		}
		if s.runEngine != nil {
			if startErr := s.runEngine.StartFlowRun(ctx, active.ID, actorID); startErr != nil {
				s.logger.WarnContext(ctx, "failed to ensure active flow workflow is running",
					"flow_run_id", active.ID,
					"template_id", active.TemplateID,
					"target_type", active.TargetType,
					"target_id", active.TargetID,
					"error", startErr,
				)
			}
		}
		return s.GetRunView(ctx, workspaceID, active.ID)
	}

	triggerType := strings.TrimSpace(req.TriggerType)
	if triggerType == "" {
		triggerType = model.FlowTriggerManual
	}
	triggerPayload := req.TriggerPayload
	if len(triggerPayload) == 0 {
		triggerPayload = json.RawMessage("{}")
	}

	currentNodeID := def.initialNodeID
	run := &model.FlowRun{
		WorkspaceID:     workspaceID,
		TemplateID:      def.spec.TemplateID,
		TemplateVersion: def.spec.TemplateVersion,
		TargetType:      req.TargetType,
		TargetID:        req.TargetID,
		Status:          model.FlowStatusRunning,
		CurrentNodeID:   &currentNodeID,
		TriggerType:     triggerType,
		TriggerPayload:  triggerPayload,
		Input:           mustJSON(input),
		OutputSummary:   json.RawMessage("{}"),
		SpecSnapshot:    mustJSON(specSnapshot),
		StartedBy:       &actorID,
	}
	if err := s.flowRepo.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	if err := s.setActiveTargetForRun(ctx, run); err != nil {
		return nil, err
	}
	if s.runEngine == nil {
		return nil, fmt.Errorf("flow workflow is not available")
	}
	if err := s.runEngine.StartFlowRun(ctx, run.ID, actorID); err != nil {
		errMsg := err.Error()
		now := time.Now()
		run.Status = model.FlowStatusFailed
		run.CompletedAt = &now
		run.CancellationReason = &errMsg
		_ = s.flowRepo.UpdateRun(ctx, run)
		_ = s.clearActiveTargetForRun(ctx, run)
		return nil, err
	}
	s.publishRunEvent(run, actorID)
	return s.GetRunView(ctx, workspaceID, run.ID)
}

func (s *FlowService) setActiveTargetForRun(ctx context.Context, run *model.FlowRun) error {
	if run == nil || run.TargetType != "epic" {
		return nil
	}
	epicWithStats, err := s.epicRepo.GetByID(ctx, run.TargetID)
	if err != nil || epicWithStats == nil {
		return err
	}
	epic := &epicWithStats.Epic
	epic.ActiveFlowRunID = &run.ID
	return s.epicRepo.Update(ctx, epic)
}

func (s *FlowService) clearActiveTargetForRun(ctx context.Context, run *model.FlowRun) error {
	if run == nil || run.TargetType != "epic" {
		return nil
	}
	return s.clearEpicActiveFlow(ctx, run.TargetID)
}

func (s *FlowService) resolveStoryCompletionInput(ctx context.Context, workspaceID string, raw json.RawMessage) (model.StartStoryCompletionFlowInput, error) {
	var input model.StartStoryCompletionFlowInput
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &input); err != nil {
			return input, fmt.Errorf("invalid flow input: %w", err)
		}
	}
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.AdditionalContext = strings.TrimSpace(input.AdditionalContext)
	if input.AgentID == "" {
		return input, fmt.Errorf("agent_id is required")
	}
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, input.AgentID)
	if err != nil || agent == nil {
		return input, fmt.Errorf("agent not found")
	}
	normalizeAgentRecord(agent)
	if err := validateAgentTarget(agent, "story"); err != nil {
		return input, err
	}
	if !agentSupportsMode(agent, model.InvocationModeAutonomous) {
		return input, fmt.Errorf("selected agent does not support autonomous mode")
	}
	return input, nil
}

func (s *FlowService) resolveCRMDealReviewInput(ctx context.Context, workspaceID string, raw json.RawMessage) (model.StartCRMDealReviewFlowInput, error) {
	var input model.StartCRMDealReviewFlowInput
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &input); err != nil {
			return input, fmt.Errorf("invalid flow input: %w", err)
		}
	}
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.AdditionalContext = strings.TrimSpace(input.AdditionalContext)
	if input.AgentID == "" {
		return input, fmt.Errorf("agent_id is required")
	}
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, input.AgentID)
	if err != nil || agent == nil {
		return input, fmt.Errorf("agent not found")
	}
	normalizeAgentRecord(agent)
	if err := validateAgentTarget(agent, "crm_deal"); err != nil {
		return input, err
	}
	if !agentSupportsMode(agent, model.InvocationModeAutonomous) {
		return input, fmt.Errorf("selected agent does not support autonomous mode")
	}
	return input, nil
}

func (s *FlowService) genericBootstrapRun(ctx context.Context, run *model.FlowRun, actorID string) error {
	if run == nil {
		return fmt.Errorf("flow run not found")
	}
	currentNodeID := derefString(run.CurrentNodeID)
	if currentNodeID == "" {
		def, ok := s.resolveTemplate(ctx, run.WorkspaceID, run.TemplateID)
		if !ok {
			return fmt.Errorf("unsupported flow template: %s", run.TemplateID)
		}
		currentNodeID = def.initialNodeID
		run.CurrentNodeID = &currentNodeID
		if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
			return err
		}
	}
	return s.enterNode(ctx, run, currentNodeID, actorID, false)
}

func (s *FlowService) enterNode(ctx context.Context, run *model.FlowRun, nodeID, actorID string, forceNew bool) error {
	def, nodeDef, err := s.lookupNodeForRun(ctx, run, nodeID)
	if err != nil {
		return err
	}
	switch nodeDef.spec.Type {
	case model.FlowNodeTypeSystemAction:
		return s.enterSystemActionNode(ctx, run, def, nodeDef, actorID, forceNew)
	case model.FlowNodeTypeInteractiveAgent:
		return s.enterInteractiveNode(ctx, run, def, nodeDef, actorID, forceNew)
	case model.FlowNodeTypeAgentTask:
		return s.enterAgentTaskNode(ctx, run, def, nodeDef, actorID, forceNew)
	case model.FlowNodeTypeApprovalGate:
		return s.enterApprovalNode(ctx, run, def, nodeDef, actorID, forceNew)
	case model.FlowNodeTypeTerminal:
		return s.enterTerminalNode(ctx, run, def, nodeDef, actorID, forceNew)
	default:
		return fmt.Errorf("unsupported node type %q", nodeDef.spec.Type)
	}
}

func (s *FlowService) enterSystemActionNode(ctx context.Context, run *model.FlowRun, def flowTemplateDefinition, nodeDef flowNodeDefinition, actorID string, forceNew bool) error {
	if s.commandService == nil {
		return fmt.Errorf("internal command service is not configured")
	}
	now := time.Now()
	nodeRun, err := s.prepareNodeRunAttempt(ctx, run, nodeDef, forceNew, model.FlowNodeStatusRunning, &now)
	if err != nil {
		return err
	}
	if nodeDef.buildCommandInput != nil {
		input, err := nodeDef.buildCommandInput(ctx, s, run, nodeRun, actorID)
		if err != nil {
			return err
		}
		nodeRun.Input = input
	}
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	commandName := nodeDef.commandName
	if commandName == "" {
		commandName = derefString(nodeDef.spec.CommandName)
	}
	output, err := s.commandService.Execute(ctx, s.commandMeta(run, nodeRun, actorID), commandName, nodeRun.Input)
	if err != nil {
		errMsg := err.Error()
		nodeRun.Status = model.FlowNodeStatusFailed
		nodeRun.ErrorMessage = &errMsg
		nodeRun.CompletedAt = &now
		_ = s.flowRepo.UpdateNodeRun(ctx, nodeRun)
		return s.markRunFailed(ctx, run, nodeRun, errMsg)
	}
	nodeRun.Status = model.FlowNodeStatusCompleted
	nodeRun.Output = output
	nodeRun.ErrorMessage = nil
	nodeRun.CompletedAt = &now
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
	if nodeDef.nextNodeID == "" {
		return nil
	}
	return s.enterNode(ctx, run, nodeDef.nextNodeID, actorID, false)
}

func (s *FlowService) enterInteractiveNode(ctx context.Context, run *model.FlowRun, def flowTemplateDefinition, nodeDef flowNodeDefinition, actorID string, forceNew bool) error {
	launch, err := nodeDef.buildInteractiveLaunch(ctx, s, run, nil, actorID)
	if err != nil {
		return err
	}
	now := time.Now()
	nodeRun, err := s.prepareNodeRunAttempt(ctx, run, nodeDef, forceNew, model.FlowNodeStatusAwaitingInput, &now)
	if err != nil {
		return err
	}
	nodeRun.AgentID = &launch.AgentID
	nodeRun.Input = mustJSON(map[string]any{
		"additional_context": launch.AdditionalContext,
		"allowed_tools":      launch.AllowedTools,
	})
	if nodeRun.ChildSessionID == nil || *nodeRun.ChildSessionID == "" {
		session, err := s.planningService.StartSession(ctx, run.WorkspaceID, run.TargetID, actorID, model.StartPlanningSessionRequest{
			AgentID:            launch.AgentID,
			AdditionalContext:  stringPtrOrNil(launch.AdditionalContext),
			FlowRunID:          &run.ID,
			FlowNodeRunID:      &nodeRun.ID,
			AllowedTools:       mustJSON(launch.AllowedTools),
			Stage:              launch.Stage,
			CustomSystemPrompt: launch.SystemPrompt,
		})
		if err != nil {
			errMsg := err.Error()
			nodeRun.Status = model.FlowNodeStatusFailed
			nodeRun.ErrorMessage = &errMsg
			nodeRun.CompletedAt = &now
			_ = s.flowRepo.UpdateNodeRun(ctx, nodeRun)
			return s.markRunFailed(ctx, run, nodeRun, errMsg)
		}
		nodeRun.ChildSessionID = &session.ID
	}
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	run.Status = model.FlowStatusAwaitingInput
	run.CurrentNodeID = &nodeDef.spec.ID
	run.CompletedAt = nil
	run.CancellationReason = nil
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
	s.publishRunEvent(run, actorID)
	return nil
}

func (s *FlowService) enterAgentTaskNode(ctx context.Context, run *model.FlowRun, def flowTemplateDefinition, nodeDef flowNodeDefinition, actorID string, forceNew bool) error {
	launch, err := nodeDef.buildAgentTaskLaunch(ctx, s, run, nil, actorID)
	if err != nil {
		return err
	}
	now := time.Now()
	nodeRun, err := s.prepareNodeRunAttempt(ctx, run, nodeDef, forceNew, model.FlowNodeStatusRunning, &now)
	if err != nil {
		return err
	}
	nodeRun.AgentID = &launch.AgentID
	nodeRun.Input = launch.Input
	if nodeRun.ChildRunID == nil || *nodeRun.ChildRunID == "" {
		childRun, err := s.startFlowAgentTaskRun(ctx, run, nodeRun, actorID, launch)
		if err != nil {
			errMsg := err.Error()
			nodeRun.Status = model.FlowNodeStatusFailed
			nodeRun.ErrorMessage = &errMsg
			nodeRun.CompletedAt = &now
			_ = s.flowRepo.UpdateNodeRun(ctx, nodeRun)
			return s.markRunFailed(ctx, run, nodeRun, errMsg)
		}
		nodeRun.ChildRunID = &childRun.ID
	}
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	run.Status = model.FlowStatusRunning
	run.CurrentNodeID = &nodeDef.spec.ID
	run.CompletedAt = nil
	run.CancellationReason = nil
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
	s.publishRunEvent(run, actorID)
	return nil
}

func (s *FlowService) enterApprovalNode(ctx context.Context, run *model.FlowRun, def flowTemplateDefinition, nodeDef flowNodeDefinition, actorID string, forceNew bool) error {
	now := time.Now()
	nodeRun, err := s.prepareNodeRunAttempt(ctx, run, nodeDef, forceNew, model.FlowNodeStatusAwaitingApproval, &now)
	if err != nil {
		return err
	}
	if nodeDef.buildApprovalInput != nil {
		input, err := nodeDef.buildApprovalInput(ctx, s, run, nodeRun)
		if err != nil {
			return err
		}
		nodeRun.Input = input
	}
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	run.Status = model.FlowStatusAwaitingApproval
	run.CurrentNodeID = &nodeDef.spec.ID
	run.CompletedAt = nil
	run.CancellationReason = nil
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
	s.publishRunEvent(run, actorID)
	return nil
}

func (s *FlowService) enterTerminalNode(ctx context.Context, run *model.FlowRun, def flowTemplateDefinition, nodeDef flowNodeDefinition, actorID string, forceNew bool) error {
	now := time.Now()
	nodeRun, err := s.prepareNodeRunAttempt(ctx, run, nodeDef, forceNew, model.FlowNodeStatusCompleted, &now)
	if err != nil {
		return err
	}
	nodeRun.Output = run.OutputSummary
	nodeRun.CompletedAt = &now
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	run.Status = model.FlowStatusCompleted
	run.CurrentNodeID = &nodeDef.spec.ID
	run.CompletedAt = &now
	run.CancellationReason = nil
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if def.clearActiveTarget != nil {
		if err := def.clearActiveTarget(ctx, s, run); err != nil {
			return err
		}
	}
	s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
	s.publishRunEvent(run, actorID)
	return nil
}

func (s *FlowService) prepareNodeRunAttempt(ctx context.Context, run *model.FlowRun, nodeDef flowNodeDefinition, forceNew bool, status string, startedAt *time.Time) (*model.FlowNodeRun, error) {
	prev, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, run.ID, nodeDef.spec.ID)
	if err != nil {
		return nil, err
	}
	if prev != nil && !forceNew {
		prev.Status = status
		prev.ErrorMessage = nil
		prev.CompletedAt = nil
		prev.StartedAt = startedAt
		return prev, nil
	}
	attempt := 1
	if prev != nil {
		attempt = prev.AttemptCount + 1
	}
	nodeRun := &model.FlowNodeRun{
		FlowRunID:    run.ID,
		NodeID:       nodeDef.spec.ID,
		NodeType:     nodeDef.spec.Type,
		Status:       status,
		AttemptCount: attempt,
		Input:        json.RawMessage("{}"),
		Output:       json.RawMessage("{}"),
		StartedAt:    startedAt,
	}
	if err := s.flowRepo.CreateNodeRun(ctx, nodeRun); err != nil {
		return nil, err
	}
	return nodeRun, nil
}

func (s *FlowService) startFlowAgentTaskRun(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string, launch flowAgentTaskLaunch) (*model.AgentRun, error) {
	agent, err := s.agentService.requireRunnableAgent(ctx, run.WorkspaceID, launch.AgentID, run.TargetType)
	if err != nil {
		return nil, err
	}
	profile := worker.GetRuntimeProfile(agent.CapabilityProfile)
	delivery, storyID, err := s.resolveFlowRunDeliveryTarget(ctx, run, profile)
	if err != nil {
		return nil, err
	}
	return s.agentService.createRun(ctx, createRunParams{
		workspaceID:        run.WorkspaceID,
		agent:              agent,
		profile:            profile,
		targetType:         run.TargetType,
		targetID:           run.TargetID,
		storyID:            storyID,
		flowRunID:          &run.ID,
		flowNodeRunID:      &nodeRun.ID,
		actorID:            &actorID,
		input:              launch.Input,
		delivery:           delivery,
		customSystemPrompt: launch.SystemPrompt,
	})
}

func (s *FlowService) resolveFlowRunDeliveryTarget(ctx context.Context, run *model.FlowRun, profile model.RuntimeProfile) (*model.StoryDeliveryTarget, *string, error) {
	if run.TargetType != "story" || s.storyRepo == nil || s.agentService == nil || s.agentService.gitService == nil {
		return nil, nil, nil
	}
	story, err := s.storyRepo.GetRawByID(ctx, run.TargetID)
	if err != nil {
		return nil, nil, err
	}
	if story == nil {
		return nil, nil, fmt.Errorf("story not found")
	}
	target, err := s.agentService.gitService.ResolveStoryDeliveryTargetForRun(ctx, run.WorkspaceID, story.ID, profile)
	return target, &story.ID, err
}

func (s *FlowService) lookupNode(templateID, nodeID string) (flowTemplateDefinition, flowNodeDefinition, error) {
	return s.lookupNodeWithContext(context.Background(), "", templateID, nodeID)
}

func (s *FlowService) lookupNodeForRun(ctx context.Context, run *model.FlowRun, nodeID string) (flowTemplateDefinition, flowNodeDefinition, error) {
	return s.lookupNodeWithContext(ctx, run.WorkspaceID, run.TemplateID, nodeID)
}

func (s *FlowService) lookupNodeWithContext(ctx context.Context, workspaceID, templateID, nodeID string) (flowTemplateDefinition, flowNodeDefinition, error) {
	def, ok := s.resolveTemplate(ctx, workspaceID, templateID)
	if !ok {
		return flowTemplateDefinition{}, flowNodeDefinition{}, fmt.Errorf("unsupported flow template: %s", templateID)
	}
	nodeDef, ok := def.nodes[nodeID]
	if !ok {
		return flowTemplateDefinition{}, flowNodeDefinition{}, fmt.Errorf("unsupported node %q for template %q", nodeID, templateID)
	}
	return def, nodeDef, nil
}

func (s *FlowService) commandMeta(run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) model.InternalCommandContext {
	meta := model.InternalCommandContext{
		WorkspaceID: run.WorkspaceID,
		ActorID:     actorID,
		TargetType:  run.TargetType,
		TargetID:    run.TargetID,
	}
	if nodeRun != nil {
		meta.FlowRunID = run.ID
		meta.FlowNodeRunID = nodeRun.ID
		meta.AgentID = derefString(nodeRun.AgentID)
		meta.RunID = derefString(nodeRun.ChildRunID)
	}
	return meta
}

func (s *FlowService) buildEpicSpecInteractiveLaunch(ctx context.Context, run *model.FlowRun, includeFeedback bool) (flowInteractiveLaunch, error) {
	var input model.StartEpicPlanningFlowInput
	if err := json.Unmarshal(run.Input, &input); err != nil {
		return flowInteractiveLaunch{}, fmt.Errorf("invalid flow input: %w", err)
	}
	additionalContext := strings.TrimSpace(input.AdditionalContext)
	if includeFeedback {
		if feedback, err := s.latestReviewFeedback(ctx, run.ID, model.FlowNodeSpecApproval); err == nil && feedback != "" {
			additionalContext = joinFlowContext(additionalContext, "Reviewer feedback:\n"+feedback)
		}
	}
	return flowInteractiveLaunch{
		AgentID:           strings.TrimSpace(input.SpecPlannerAgentID),
		AdditionalContext: additionalContext,
		AllowedTools:      planningReadOnlyTools(),
		Stage:             model.PlanningSessionStageDraftSpec,
	}, nil
}

func (s *FlowService) buildEpicStoryPlanningInteractiveLaunch(ctx context.Context, run *model.FlowRun) (flowInteractiveLaunch, error) {
	storyPlannerID, additionalContext, err := s.readStoryPlanningContext(run.Input)
	if err != nil {
		return flowInteractiveLaunch{}, err
	}
	if feedback, err := s.latestReviewFeedback(ctx, run.ID, model.FlowNodePlanApproval); err == nil && feedback != "" {
		additionalContext = joinFlowContext(additionalContext, "Reviewer feedback:\n"+feedback)
	}
	return flowInteractiveLaunch{
		AgentID:           storyPlannerID,
		AdditionalContext: additionalContext,
		AllowedTools:      epicStoryPlanningTools(),
		Stage:             model.PlanningSessionStagePlanStories,
	}, nil
}

func (s *FlowService) buildEpicStoryPlanningLaunch(ctx context.Context, run *model.FlowRun, approvalNodeID string) (flowAgentTaskLaunch, error) {
	storyPlannerID, additionalContext, err := s.readStoryPlanningContext(run.Input)
	if err != nil {
		return flowAgentTaskLaunch{}, err
	}
	if feedback, err := s.latestReviewFeedback(ctx, run.ID, approvalNodeID); err == nil && feedback != "" {
		additionalContext = joinFlowContext(additionalContext, "Reviewer feedback:\n"+feedback)
	}
	input := map[string]any{
		"stage":              model.PlanningStagePlanStories,
		"additional_context": additionalContext,
		"allowed_tools":      epicStoryPlanningTools(),
	}
	return flowAgentTaskLaunch{
		AgentID:      storyPlannerID,
		Input:        mustJSON(input),
		AllowedTools: epicStoryPlanningTools(),
	}, nil
}

func (s *FlowService) buildStoryCompletionLaunch(ctx context.Context, run *model.FlowRun) (flowAgentTaskLaunch, error) {
	var input model.StartStoryCompletionFlowInput
	if err := json.Unmarshal(run.Input, &input); err != nil {
		return flowAgentTaskLaunch{}, fmt.Errorf("invalid flow input: %w", err)
	}
	additionalContext := strings.TrimSpace(input.AdditionalContext)
	if feedback, err := s.latestReviewFeedback(ctx, run.ID, model.FlowNodeCompletionReview); err == nil && feedback != "" {
		additionalContext = joinFlowContext(additionalContext, "Reviewer feedback:\n"+feedback)
	}
	payload := flowTaskRunInput{
		AdditionalContext: additionalContext,
		AllowedTools:      storyCompletionTools(),
		FlowOutputKind:    "pm.story_completion_followups",
	}
	return flowAgentTaskLaunch{
		AgentID:      input.AgentID,
		Input:        mustJSON(payload),
		AllowedTools: storyCompletionTools(),
	}, nil
}

func (s *FlowService) buildCRMDealReviewLaunch(ctx context.Context, run *model.FlowRun) (flowAgentTaskLaunch, error) {
	var input model.StartCRMDealReviewFlowInput
	if err := json.Unmarshal(run.Input, &input); err != nil {
		return flowAgentTaskLaunch{}, fmt.Errorf("invalid flow input: %w", err)
	}
	additionalContext := strings.TrimSpace(input.AdditionalContext)
	if feedback, err := s.latestReviewFeedback(ctx, run.ID, model.FlowNodeDealReviewApproval); err == nil && feedback != "" {
		additionalContext = joinFlowContext(additionalContext, "Reviewer feedback:\n"+feedback)
	}
	payload := flowTaskRunInput{
		AdditionalContext: additionalContext,
		AllowedTools:      crmDealReviewTools(),
		FlowOutputKind:    "crm.deal_review_actions",
	}
	return flowAgentTaskLaunch{
		AgentID:      input.AgentID,
		Input:        mustJSON(payload),
		AllowedTools: crmDealReviewTools(),
	}, nil
}

func (s *FlowService) buildApproveSpecCommandInput(payload json.RawMessage) (json.RawMessage, error) {
	if len(payload) == 0 {
		return json.RawMessage("{}"), nil
	}
	var req model.ApproveEpicSpecRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("invalid approval payload: %w", err)
	}
	return mustJSON(req), nil
}

func (s *FlowService) buildCreateStoriesCommandInput(ctx context.Context, run *model.FlowRun) (json.RawMessage, error) {
	storyNodeID := model.FlowNodeStoryPlan
	approvalNodeID := model.FlowNodePlanApproval
	storyNode, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, run.ID, storyNodeID)
	if err != nil {
		return nil, err
	}
	if storyNode == nil {
		return nil, fmt.Errorf("story planning node not found")
	}

	var req model.ConfirmPlanningRequest

	if storyNode.ChildRunID != nil && *storyNode.ChildRunID != "" {
		// Autonomous path (legacy).
		req.RunID = *storyNode.ChildRunID
	} else if storyNode.ChildSessionID != nil && *storyNode.ChildSessionID != "" {
		// Interactive path — read proposed stories from node output.
		var nodeOutput struct {
			PlanDraft json.RawMessage `json:"plan_draft"`
		}
		if err := json.Unmarshal(storyNode.Output, &nodeOutput); err == nil && len(nodeOutput.PlanDraft) > 0 {
			var proposal model.OrchestrationProposal
			if err := json.Unmarshal(nodeOutput.PlanDraft, &proposal); err == nil {
				req.ProposedStories = proposal.ProposedStories
			}
		}
	} else {
		return nil, fmt.Errorf("story planning run is missing")
	}

	// Apply approval overrides.
	if decision, err := s.latestApprovalDecision(ctx, run.ID, approvalNodeID); err == nil && len(decision.OverridePayload) > 0 {
		var override model.ConfirmPlanningRequest
		if err := json.Unmarshal(decision.OverridePayload, &override); err == nil {
			if len(override.ProposedStories) > 0 {
				req.ProposedStories = override.ProposedStories
			}
		}
	}
	return mustJSON(req), nil
}

func (s *FlowService) buildStoryCompletionCommandInput(ctx context.Context, run *model.FlowRun) (json.RawMessage, error) {
	assessment, err := s.storyCompletionAssessmentForApply(ctx, run)
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"followups": assessment.Followups}), nil
}

func (s *FlowService) buildCRMDealReviewCommandInput(ctx context.Context, run *model.FlowRun) (json.RawMessage, error) {
	plan, err := s.crmDealReviewPlanForApply(ctx, run)
	if err != nil {
		return nil, err
	}
	return mustJSON(plan), nil
}

func (s *FlowService) latestReviewFeedback(ctx context.Context, flowRunID, nodeID string) (string, error) {
	decision, err := s.latestApprovalDecision(ctx, flowRunID, nodeID)
	if err != nil || decision == nil {
		return "", err
	}
	var parts []string
	if text := strings.TrimSpace(decision.Comment); text != "" {
		parts = append(parts, text)
	}
	if len(decision.StructuredFeedback) > 0 && string(decision.StructuredFeedback) != "null" && string(decision.StructuredFeedback) != "{}" {
		parts = append(parts, "Structured feedback:\n"+string(decision.StructuredFeedback))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n")), nil
}

func (s *FlowService) latestApprovalDecision(ctx context.Context, flowRunID, nodeID string) (*model.FlowApprovalDecision, error) {
	nodeRun, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, flowRunID, nodeID)
	if err != nil || nodeRun == nil {
		return nil, err
	}
	var decision model.FlowApprovalDecision
	if err := json.Unmarshal(nodeRun.Output, &decision); err != nil {
		return nil, nil
	}
	return &decision, nil
}

func (s *FlowService) storyCompletionAssessmentForApply(ctx context.Context, run *model.FlowRun) (*model.StoryCompletionAssessment, error) {
	if decision, err := s.latestApprovalDecision(ctx, run.ID, model.FlowNodeCompletionReview); err == nil && decision != nil && len(decision.OverridePayload) > 0 {
		var assessment model.StoryCompletionAssessment
		if err := json.Unmarshal(decision.OverridePayload, &assessment); err == nil && len(assessment.Followups) > 0 {
			return &assessment, nil
		}
	}
	childRun, err := s.latestChildRun(ctx, run.ID, model.FlowNodeCompletionAssessment)
	if err != nil {
		return nil, err
	}
	if childRun == nil {
		return nil, fmt.Errorf("completion assessment run is missing")
	}
	var assessment model.StoryCompletionAssessment
	if err := json.Unmarshal(childRun.OutputSummary, &assessment); err != nil {
		return nil, fmt.Errorf("completion assessment output is invalid: %w", err)
	}
	return &assessment, nil
}

func (s *FlowService) crmDealReviewPlanForApply(ctx context.Context, run *model.FlowRun) (*model.CRMDealReviewActionPlan, error) {
	if decision, err := s.latestApprovalDecision(ctx, run.ID, model.FlowNodeDealReviewApproval); err == nil && decision != nil && len(decision.OverridePayload) > 0 {
		var plan model.CRMDealReviewActionPlan
		if err := json.Unmarshal(decision.OverridePayload, &plan); err == nil {
			return &plan, nil
		}
	}
	childRun, err := s.latestChildRun(ctx, run.ID, model.FlowNodeDealReview)
	if err != nil {
		return nil, err
	}
	if childRun == nil {
		return nil, fmt.Errorf("deal review run is missing")
	}
	var plan model.CRMDealReviewActionPlan
	if err := json.Unmarshal(childRun.OutputSummary, &plan); err != nil {
		return nil, fmt.Errorf("deal review output is invalid: %w", err)
	}
	return &plan, nil
}

func (s *FlowService) latestChildRun(ctx context.Context, flowRunID, nodeID string) (*model.AgentRun, error) {
	nodeRun, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, flowRunID, nodeID)
	if err != nil || nodeRun == nil || nodeRun.ChildRunID == nil || *nodeRun.ChildRunID == "" {
		return nil, err
	}
	return s.runRepo.GetByIDAny(ctx, *nodeRun.ChildRunID)
}

func joinFlowContext(parts ...string) string {
	joined := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			joined = append(joined, part)
		}
	}
	return strings.Join(joined, "\n\n")
}
