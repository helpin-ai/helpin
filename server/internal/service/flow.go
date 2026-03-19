package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type FlowService struct {
	flowRepo        *repository.FlowRepository
	epicRepo        *repository.PMEpicRepository
	storyRepo       *repository.PMStoryRepository
	crmDealRepo     *repository.CRMDealRepository
	agentRepo       *repository.AgentRepository
	runRepo         *repository.AgentRunRepository
	sessionRepo     *repository.PlanningSessionRepository
	agentService    *AgentService
	planningService *PlanningSessionService
	commandService  *InternalCommandService
	runEngine       *temporalapp.RunEngine
	wsPublisher     *websocket.Publisher
	logger          *slog.Logger
}

func NewFlowService(
	flowRepo *repository.FlowRepository,
	epicRepo *repository.PMEpicRepository,
	storyRepo *repository.PMStoryRepository,
	crmDealRepo *repository.CRMDealRepository,
	agentRepo *repository.AgentRepository,
	runRepo *repository.AgentRunRepository,
	sessionRepo *repository.PlanningSessionRepository,
	agentService *AgentService,
	planningService *PlanningSessionService,
	runEngine *temporalapp.RunEngine,
	wsPublisher *websocket.Publisher,
) *FlowService {
	return &FlowService{
		flowRepo:        flowRepo,
		epicRepo:        epicRepo,
		storyRepo:       storyRepo,
		crmDealRepo:     crmDealRepo,
		agentRepo:       agentRepo,
		runRepo:         runRepo,
		sessionRepo:     sessionRepo,
		agentService:    agentService,
		planningService: planningService,
		runEngine:       runEngine,
		wsPublisher:     wsPublisher,
		logger:          slog.Default().With("service", "flow"),
	}
}

func (s *FlowService) SetCommandService(commandService *InternalCommandService) {
	s.commandService = commandService
}

func (s *FlowService) StartRun(ctx context.Context, workspaceID, actorID string, req model.StartFlowRunRequest) (*model.FlowRunView, error) {
	templateID := strings.TrimSpace(req.TemplateID)
	if templateID == "" {
		return nil, fmt.Errorf("template_id is required")
	}
	def, ok := lookupFlowTemplate(templateID)
	if !ok {
		return nil, fmt.Errorf("unsupported flow template: %s", templateID)
	}
	if strings.TrimSpace(req.TargetType) == "" {
		return nil, fmt.Errorf("target_type is required")
	}
	if req.TargetType != def.spec.TargetType {
		return nil, fmt.Errorf("template %s requires target_type %s", templateID, def.spec.TargetType)
	}
	return def.startRun(ctx, s, workspaceID, actorID, req)
}

func (s *FlowService) GetRunView(ctx context.Context, workspaceID, flowRunID string) (*model.FlowRunView, error) {
	run, err := s.flowRepo.GetRunByID(ctx, workspaceID, flowRunID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("flow run not found")
	}
	nodeRuns, err := s.flowRepo.ListNodeRuns(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	return &model.FlowRunView{
		Run:      run,
		Spec:     flowSpec(run.TemplateID),
		NodeRuns: nodeRuns,
	}, nil
}

func (s *FlowService) ListNodeRuns(ctx context.Context, workspaceID, flowRunID string) ([]model.FlowNodeRun, error) {
	run, err := s.flowRepo.GetRunByID(ctx, workspaceID, flowRunID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("flow run not found")
	}
	return s.flowRepo.ListNodeRuns(ctx, flowRunID)
}

func (s *FlowService) ListRuns(ctx context.Context, workspaceID string, limit, offset int) (*model.FlowRunListResponse, error) {
	runs, total, err := s.flowRepo.ListRuns(ctx, workspaceID, limit, offset)
	if err != nil {
		return nil, err
	}
	views := make([]model.FlowRunView, len(runs))
	for i, run := range runs {
		nodeRuns, err := s.flowRepo.ListNodeRuns(ctx, run.ID)
		if err != nil {
			s.logger.WarnContext(ctx, "failed to fetch node runs for flow run", "flow_run_id", run.ID, "error", err)
			nodeRuns = nil
		}
		views[i] = model.FlowRunView{
			Run:      &runs[i],
			Spec:     flowSpec(run.TemplateID),
			NodeRuns: nodeRuns,
		}
	}
	return &model.FlowRunListResponse{Data: views, Total: total}, nil
}

func (s *FlowService) ListTemplates() []model.FlowSpec {
	return flowTemplateSpecs()
}

func (s *FlowService) SendInteractiveMessage(ctx context.Context, workspaceID, flowRunID, nodeRunID, actorID string, req model.FlowInteractiveMessageRequest) (*model.PlanningSessionMessage, error) {
	nodeRun, _, err := s.requireInteractiveNode(ctx, workspaceID, flowRunID, nodeRunID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("content is required")
	}
	// Chat messages bypass the flow workflow. The parent flow only gates
	// node transitions such as finalize/approve/retry/cancel.
	return s.planningService.SendMessage(ctx, workspaceID, *nodeRun.ChildSessionID, actorID, model.SendPlanningMessageRequest{
		Content: req.Content,
	})
}

func (s *FlowService) ListInteractiveMessages(ctx context.Context, workspaceID, flowRunID, nodeRunID string) ([]model.PlanningSessionMessage, error) {
	nodeRun, _, err := s.requireInteractiveNode(ctx, workspaceID, flowRunID, nodeRunID)
	if err != nil {
		return nil, err
	}
	return s.planningService.GetSessionMessages(ctx, workspaceID, *nodeRun.ChildSessionID)
}

func (s *FlowService) SendNodeAction(ctx context.Context, workspaceID, flowRunID, nodeRunID, actorID string, req model.FlowNodeActionRequest) (*model.FlowRunView, error) {
	run, err := s.flowRepo.GetRunByID(ctx, workspaceID, flowRunID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("flow run not found")
	}
	nodeRun, err := s.flowRepo.GetNodeRunByID(ctx, flowRunID, nodeRunID)
	if err != nil {
		return nil, err
	}
	if nodeRun == nil {
		return nil, fmt.Errorf("flow node run not found")
	}
	action := strings.TrimSpace(req.ActionType)
	if !s.supportsNodeAction(run.TemplateID, nodeRun, action) {
		return nil, fmt.Errorf("unsupported action %q for node %q", action, nodeRun.NodeID)
	}
	if s.runEngine == nil {
		return nil, fmt.Errorf("flow workflow is not available")
	}
	if err := s.runEngine.SignalFlowRun(ctx, run.ID, temporalapp.FlowRunSignal{
		Type:      temporalapp.FlowSignalTypeNodeAction,
		NodeRunID: nodeRunID,
		Action:    action,
		ActorID:   actorID,
		Payload:   req.Payload,
	}); err != nil {
		return nil, err
	}
	return s.GetRunView(ctx, workspaceID, flowRunID)
}

func (s *FlowService) GetRunByIDAny(ctx context.Context, flowRunID string) (*model.FlowRun, error) {
	return s.flowRepo.GetRunByIDAny(ctx, flowRunID)
}

func (s *FlowService) GetNodeRunByIDAny(ctx context.Context, nodeRunID string) (*model.FlowNodeRun, error) {
	return s.flowRepo.GetNodeRunByIDAny(ctx, nodeRunID)
}

func (s *FlowService) LoadRuntimeState(ctx context.Context, flowRunID string) (temporalapp.FlowRuntimeState, error) {
	run, err := s.GetRunByIDAny(ctx, flowRunID)
	if err != nil {
		return temporalapp.FlowRuntimeState{}, err
	}
	if run == nil {
		return temporalapp.FlowRuntimeState{}, fmt.Errorf("flow run not found")
	}
	state := temporalapp.FlowRuntimeState{
		RunID:         run.ID,
		Status:        run.Status,
		CurrentNodeID: derefString(run.CurrentNodeID),
	}
	if state.CurrentNodeID == "" {
		return state, nil
	}
	nodeRuns, err := s.flowRepo.ListNodeRuns(ctx, run.ID)
	if err != nil {
		return temporalapp.FlowRuntimeState{}, err
	}
	if activeNode := latestNodeRun(nodeRuns, state.CurrentNodeID); activeNode != nil {
		state.CurrentNodeRunID = activeNode.ID
	}
	return state, nil
}

func (s *FlowService) CancelRun(ctx context.Context, workspaceID, flowRunID, actorID string) (*model.FlowRunView, error) {
	run, err := s.flowRepo.GetRunByID(ctx, workspaceID, flowRunID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("flow run not found")
	}

	// Already terminal — nothing to do.
	if run.Status == model.FlowStatusCancelled || run.Status == model.FlowStatusCompleted || run.Status == model.FlowStatusFailed {
		return s.GetRunView(ctx, workspaceID, flowRunID)
	}

	if s.runEngine == nil {
		return nil, fmt.Errorf("flow workflow is not available")
	}

	// Try the graceful signal path first.
	if err := s.runEngine.SignalFlowRun(ctx, run.ID, temporalapp.FlowRunSignal{
		Type:    temporalapp.FlowSignalTypeCancel,
		ActorID: actorID,
		Reason:  "cancelled_by_user",
	}); err != nil {
		s.logger.WarnContext(ctx, "flow cancel signal failed, will force-cancel",
			"flow_run_id", run.ID, "error", err)
	} else {
		// Give the workflow a moment to process the signal.
		time.Sleep(500 * time.Millisecond)

		// Re-read to check if the workflow handled it.
		run, err = s.flowRepo.GetRunByID(ctx, workspaceID, flowRunID)
		if err != nil {
			return nil, err
		}
		if run.Status == model.FlowStatusCancelled || run.Status == model.FlowStatusCompleted || run.Status == model.FlowStatusFailed {
			return s.GetRunView(ctx, workspaceID, flowRunID)
		}
	}

	// Workflow is stuck or slow — force-cancel.
	s.logger.InfoContext(ctx, "flow cancel: forcing termination",
		"flow_run_id", run.ID, "status", run.Status)

	if err := s.runEngine.TerminateFlowRun(ctx, run.ID, "cancelled_by_user"); err != nil {
		s.logger.WarnContext(ctx, "flow terminate failed", "flow_run_id", run.ID, "error", err)
	}
	if err := s.cancelRunNow(ctx, run, actorID, "cancelled_by_user"); err != nil {
		return nil, fmt.Errorf("force cancel flow run: %w", err)
	}
	return s.GetRunView(ctx, workspaceID, flowRunID)
}

func (s *FlowService) RetryNode(ctx context.Context, workspaceID, flowRunID, nodeRunID, actorID string) (*model.FlowRunView, error) {
	run, err := s.flowRepo.GetRunByID(ctx, workspaceID, flowRunID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("flow run not found")
	}
	nodeRun, err := s.flowRepo.GetNodeRunByID(ctx, flowRunID, nodeRunID)
	if err != nil {
		return nil, err
	}
	if nodeRun == nil {
		return nil, fmt.Errorf("flow node run not found")
	}
	if nodeRun.Status != model.FlowNodeStatusFailed && nodeRun.Status != model.FlowNodeStatusCancelled {
		return nil, fmt.Errorf("only failed or cancelled nodes can be retried")
	}
	def, nodeDef, err := s.lookupNode(run.TemplateID, nodeRun.NodeID)
	if err != nil {
		return nil, err
	}
	_ = def
	if !nodeDef.retryable {
		return nil, fmt.Errorf("retry is not supported for node %q", nodeRun.NodeID)
	}
	if s.runEngine == nil {
		return nil, fmt.Errorf("flow workflow is not available")
	}
	if err := s.runEngine.SignalFlowRun(ctx, run.ID, temporalapp.FlowRunSignal{
		Type:      temporalapp.FlowSignalTypeRetry,
		NodeRunID: nodeRunID,
		ActorID:   actorID,
	}); err != nil {
		return nil, err
	}
	return s.GetRunView(ctx, workspaceID, flowRunID)
}

func (s *FlowService) resolveEpicPlanningInput(ctx context.Context, workspaceID string, epic *model.PMEpic, raw json.RawMessage) (model.StartEpicPlanningFlowInput, string, string, error) {
	var input model.StartEpicPlanningFlowInput
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &input); err != nil {
			return input, "", "", fmt.Errorf("invalid flow input: %w", err)
		}
	}

	specPlannerID := strings.TrimSpace(input.SpecPlannerAgentID)
	if specPlannerID == "" {
		specPlannerID = strings.TrimSpace(derefString(epic.OrchestratorAgentID))
	}
	if specPlannerID == "" {
		return input, "", "", fmt.Errorf("spec_planner_agent_id is required")
	}
	storyPlannerID := strings.TrimSpace(input.StoryPlannerAgentID)
	if storyPlannerID == "" {
		storyPlannerID = specPlannerID
		input.StoryPlannerAgentID = storyPlannerID
	}
	input.SpecPlannerAgentID = specPlannerID
	input.AdditionalContext = strings.TrimSpace(input.AdditionalContext)

	specPlanner, err := s.agentRepo.GetByID(ctx, workspaceID, specPlannerID)
	if err != nil || specPlanner == nil {
		return input, "", "", fmt.Errorf("spec planner agent not found")
	}
	normalizeAgentRecord(specPlanner)
	if err := validateAgentTarget(specPlanner, "epic"); err != nil {
		return input, "", "", err
	}
	if !agentSupportsMode(specPlanner, model.InvocationModeInteractive) {
		return input, "", "", fmt.Errorf("selected spec planner does not support interactive mode")
	}

	storyPlanner, err := s.agentRepo.GetByID(ctx, workspaceID, storyPlannerID)
	if err != nil || storyPlanner == nil {
		return input, "", "", fmt.Errorf("story planner agent not found")
	}
	normalizeAgentRecord(storyPlanner)
	if err := validateAgentTarget(storyPlanner, "epic"); err != nil {
		return input, "", "", err
	}
	if !agentSupportsMode(storyPlanner, model.InvocationModeAutonomous) {
		return input, "", "", fmt.Errorf("selected story planner does not support autonomous mode")
	}

	return input, specPlannerID, storyPlannerID, nil
}

func (s *FlowService) requireInteractiveNode(ctx context.Context, workspaceID, flowRunID, nodeRunID string) (*model.FlowNodeRun, *model.FlowRun, error) {
	run, err := s.flowRepo.GetRunByID(ctx, workspaceID, flowRunID)
	if err != nil {
		return nil, nil, err
	}
	if run == nil {
		return nil, nil, fmt.Errorf("flow run not found")
	}
	nodeRun, err := s.flowRepo.GetNodeRunByID(ctx, flowRunID, nodeRunID)
	if err != nil {
		return nil, nil, err
	}
	if nodeRun == nil {
		return nil, nil, fmt.Errorf("flow node run not found")
	}
	if nodeRun.NodeType != model.FlowNodeTypeInteractiveAgent || nodeRun.ChildSessionID == nil || *nodeRun.ChildSessionID == "" {
		return nil, nil, fmt.Errorf("flow node is not an interactive node")
	}
	return nodeRun, run, nil
}

func (s *FlowService) finalizeInteractiveNode(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) error {
	if nodeRun.ChildSessionID == nil || *nodeRun.ChildSessionID == "" {
		return fmt.Errorf("interactive node has no session")
	}
	session, err := s.planningService.FinalizeSession(ctx, run.WorkspaceID, *nodeRun.ChildSessionID, actorID)
	if err != nil {
		return err
	}
	now := time.Now()
	output, _ := json.Marshal(map[string]any{
		"session_id":       session.ID,
		"spec_document_id": derefString(session.SpecDocumentID),
		"planning_status":  session.Status,
	})
	nodeRun.Status = model.FlowNodeStatusCompleted
	nodeRun.Output = output
	nodeRun.CompletedAt = &now
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}

	if _, err := s.ensureApprovalGateNode(ctx, run.ID, model.FlowNodeSpecApproval); err != nil {
		return err
	}
	currentNode := model.FlowNodeSpecApproval
	run.Status = model.FlowStatusAwaitingApproval
	run.CurrentNodeID = &currentNode
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
	s.publishRunEvent(run, actorID)
	return nil
}


func (s *FlowService) loadCurrentNode(ctx context.Context, flowRunID string) (*model.FlowRun, *model.FlowNodeRun, error) {
	run, err := s.flowRepo.GetRunByIDAny(ctx, flowRunID)
	if err != nil {
		return nil, nil, err
	}
	if run == nil {
		return nil, nil, fmt.Errorf("flow run not found")
	}
	currentNodeID := derefString(run.CurrentNodeID)
	if currentNodeID == "" {
		return nil, nil, fmt.Errorf("flow run has no current node")
	}
	nodeRun, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, run.ID, currentNodeID)
	if err != nil {
		return nil, nil, err
	}
	if nodeRun == nil {
		return nil, nil, fmt.Errorf("current flow node run not found")
	}
	return run, nodeRun, nil
}

func (s *FlowService) cancelRunNow(ctx context.Context, run *model.FlowRun, actorID, reason string) error {
	nodeRuns, err := s.flowRepo.ListNodeRuns(ctx, run.ID)
	if err != nil {
		return err
	}
	activeNode := latestNodeRun(nodeRuns, derefString(run.CurrentNodeID))
	if activeNode != nil {
		switch {
		case activeNode.ChildSessionID != nil && *activeNode.ChildSessionID != "":
			if err := s.planningService.AbandonSession(ctx, run.WorkspaceID, *activeNode.ChildSessionID, actorID); err != nil {
				return err
			}
		case activeNode.ChildRunID != nil && *activeNode.ChildRunID != "":
			if _, err := s.agentService.CancelRun(ctx, run.WorkspaceID, *activeNode.ChildRunID, actorID); err != nil {
				return err
			}
		}
		now := time.Now()
		activeNode.Status = model.FlowNodeStatusCancelled
		activeNode.CompletedAt = &now
		if err := s.flowRepo.UpdateNodeRun(ctx, activeNode); err != nil {
			return err
		}
		s.publishNodeEvent(activeNode, run.WorkspaceID, actorID)
	}

	now := time.Now()
	run.Status = model.FlowStatusCancelled
	run.CompletedAt = &now
	run.CancellationReason = &reason
	run.CurrentNodeID = strPtr(model.FlowNodeDone)
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if err := s.clearActiveTargetForRun(ctx, run); err != nil {
		return err
	}
	s.publishRunEvent(run, actorID)
	return nil
}

func (s *FlowService) markRunFailedByID(ctx context.Context, flowRunID, nodeID, message string) error {
	run, err := s.flowRepo.GetRunByIDAny(ctx, flowRunID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("flow run not found")
	}
	nodeRuns, err := s.flowRepo.ListNodeRuns(ctx, flowRunID)
	if err != nil {
		return err
	}
	nodeRun := latestNodeRun(nodeRuns, nodeID)
	if nodeRun == nil {
		now := time.Now()
		nodeRun = &model.FlowNodeRun{
			FlowRunID:    flowRunID,
			NodeID:       nodeID,
			NodeType:     model.FlowNodeTypeSystemAction,
			Status:       model.FlowNodeStatusFailed,
			AttemptCount: 1,
			ErrorMessage: &message,
			StartedAt:    &now,
			CompletedAt:  &now,
			Output:       json.RawMessage("{}"),
		}
		if err := s.flowRepo.CreateNodeRun(ctx, nodeRun); err != nil {
			return err
		}
	} else {
		now := time.Now()
		nodeRun.Status = model.FlowNodeStatusFailed
		nodeRun.ErrorMessage = &message
		nodeRun.CompletedAt = &now
		if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
			return err
		}
	}
	run.Status = model.FlowStatusFailed
	run.CompletedAt = nodeRun.CompletedAt
	run.CancellationReason = &message
	run.CurrentNodeID = &nodeID
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.publishNodeEvent(nodeRun, run.WorkspaceID, "")
	s.publishRunEvent(run, "")
	return nil
}

func (s *FlowService) ensureApprovalGateNode(ctx context.Context, flowRunID, nodeID string) (*model.FlowNodeRun, error) {
	existing, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, flowRunID, nodeID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	now := time.Now()
	nodeRun := &model.FlowNodeRun{
		FlowRunID:    flowRunID,
		NodeID:       nodeID,
		NodeType:     model.FlowNodeTypeApprovalGate,
		Status:       model.FlowNodeStatusAwaitingApproval,
		AttemptCount: 1,
		Input:        json.RawMessage("{}"),
		Output:       json.RawMessage("{}"),
		StartedAt:    &now,
	}
	if err := s.flowRepo.CreateNodeRun(ctx, nodeRun); err != nil {
		return nil, err
	}
	return nodeRun, nil
}

func (s *FlowService) readStoryPlanningContext(raw json.RawMessage) (storyPlannerID string, additionalContext string, err error) {
	var input model.StartEpicPlanningFlowInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return "", "", fmt.Errorf("invalid flow input: %w", err)
	}
	storyPlannerID = strings.TrimSpace(input.StoryPlannerAgentID)
	if storyPlannerID == "" {
		storyPlannerID = strings.TrimSpace(input.SpecPlannerAgentID)
	}
	if storyPlannerID == "" {
		return "", "", fmt.Errorf("story planner agent is missing from flow input")
	}
	return storyPlannerID, strings.TrimSpace(input.AdditionalContext), nil
}

func (s *FlowService) markRunFailed(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, message string) error {
	now := time.Now()
	nodeRun.Status = model.FlowNodeStatusFailed
	nodeRun.ErrorMessage = &message
	nodeRun.CompletedAt = &now
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	run.Status = model.FlowStatusFailed
	run.CompletedAt = &now
	run.CancellationReason = &message
	return s.flowRepo.UpdateRun(ctx, run)
}

func (s *FlowService) markRunCancelled(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, reason string) error {
	now := time.Now()
	nodeRun.Status = model.FlowNodeStatusCancelled
	nodeRun.CompletedAt = &now
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	run.Status = model.FlowStatusCancelled
	run.CompletedAt = &now
	run.CancellationReason = &reason
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	return s.clearActiveTargetForRun(ctx, run)
}

func (s *FlowService) clearEpicActiveFlow(ctx context.Context, epicID string) error {
	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil || epicWithStats == nil {
		return err
	}
	epic := &epicWithStats.Epic
	epic.ActiveFlowRunID = nil
	return s.epicRepo.Update(ctx, epic)
}

func (s *FlowService) publishRunEvent(run *model.FlowRun, actorID string) {
	if s.wsPublisher == nil || run == nil {
		return
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "flow_run",
		EntityID:    run.ID,
		WorkspaceID: run.WorkspaceID,
		ActorID:     actorID,
		ParentType:  run.TargetType,
		ParentID:    run.TargetID,
	})
}

func (s *FlowService) publishNodeEvent(nodeRun *model.FlowNodeRun, workspaceID, actorID string) {
	if s.wsPublisher == nil || nodeRun == nil {
		return
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "flow_node_run",
		EntityID:    nodeRun.ID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		ParentType:  "flow_run",
		ParentID:    nodeRun.FlowRunID,
	})
}

func flowSpec(templateID string) model.FlowSpec {
	def, ok := lookupFlowTemplate(templateID)
	if !ok {
		return model.FlowSpec{}
	}
	return def.spec
}

func (s *FlowService) supportsNodeAction(templateID string, nodeRun *model.FlowNodeRun, action string) bool {
	if nodeRun == nil {
		return false
	}
	spec := flowSpec(templateID)
	for _, node := range spec.Nodes {
		if node.ID != nodeRun.NodeID {
			continue
		}
		for _, supported := range node.Actions {
			if supported == action {
				return true
			}
		}
	}
	return false
}

func latestNodeRun(items []model.FlowNodeRun, nodeID string) *model.FlowNodeRun {
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].NodeID == nodeID {
			return &items[i]
		}
	}
	return nil
}

func mustJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}

func stringPtrOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
