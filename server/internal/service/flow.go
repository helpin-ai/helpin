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
	agentRepo       *repository.AgentRepository
	runRepo         *repository.AgentRunRepository
	sessionRepo     *repository.PlanningSessionRepository
	agentService    *AgentService
	planningService *PlanningSessionService
	runEngine       *temporalapp.RunEngine
	wsPublisher     *websocket.Publisher
	logger          *slog.Logger
}

func NewFlowService(
	flowRepo *repository.FlowRepository,
	epicRepo *repository.PMEpicRepository,
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

func (s *FlowService) StartRun(ctx context.Context, workspaceID, actorID string, req model.StartFlowRunRequest) (*model.FlowRunView, error) {
	templateID := strings.TrimSpace(req.TemplateID)
	if templateID == "" {
		return nil, fmt.Errorf("template_id is required")
	}
	if req.TargetType != "epic" {
		return nil, fmt.Errorf("unsupported target_type: %s", req.TargetType)
	}
	if templateID != model.FlowTemplateEpicPlanningV1 {
		return nil, fmt.Errorf("unsupported flow template: %s", templateID)
	}
	return s.startEpicPlanningFlow(ctx, workspaceID, actorID, req)
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
	return []model.FlowSpec{flowSpec(model.FlowTemplateEpicPlanningV1)}
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
	if !s.supportsNodeAction(nodeRun, action) {
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
	if nodeRun.NodeID != model.FlowNodeStoryPlanning && nodeRun.NodeID != model.FlowNodeCreateStories {
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

func (s *FlowService) startEpicPlanningFlow(ctx context.Context, workspaceID, actorID string, req model.StartFlowRunRequest) (*model.FlowRunView, error) {
	epicWithStats, err := s.epicRepo.GetByID(ctx, req.TargetID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epicWithStats == nil || epicWithStats.Epic.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &epicWithStats.Epic

	active, err := s.flowRepo.GetActiveRunByTarget(ctx, workspaceID, req.TemplateID, req.TargetType, req.TargetID)
	if err != nil {
		return nil, err
	}
	if active != nil {
		return s.GetRunView(ctx, workspaceID, active.ID)
	}

	input, specPlannerID, storyPlannerID, err := s.resolveEpicPlanningInput(ctx, workspaceID, epic, req.Input)
	if err != nil {
		return nil, err
	}

	specSnapshot, _ := json.Marshal(map[string]any{
		"template_id":            model.FlowTemplateEpicPlanningV1,
		"target_type":            req.TargetType,
		"target_id":              req.TargetID,
		"spec_planner_agent_id":  specPlannerID,
		"story_planner_agent_id": storyPlannerID,
		"additional_context":     input.AdditionalContext,
		"orchestrator_agent_id":  derefString(epic.OrchestratorAgentID),
		"started_by":             actorID,
	})

	triggerType := strings.TrimSpace(req.TriggerType)
	if triggerType == "" {
		triggerType = model.FlowTriggerManual
	}
	triggerPayload := req.TriggerPayload
	if len(triggerPayload) == 0 {
		triggerPayload = json.RawMessage("{}")
	}

	currentNode := model.FlowNodeEnsureSpecDoc
	flowInput, _ := json.Marshal(input)
	run := &model.FlowRun{
		WorkspaceID:     workspaceID,
		TemplateID:      model.FlowTemplateEpicPlanningV1,
		TemplateVersion: 1,
		TargetType:      req.TargetType,
		TargetID:        req.TargetID,
		Status:          model.FlowStatusRunning,
		CurrentNodeID:   &currentNode,
		TriggerType:     triggerType,
		TriggerPayload:  triggerPayload,
		Input:           flowInput,
		OutputSummary:   json.RawMessage("{}"),
		SpecSnapshot:    specSnapshot,
		StartedBy:       &actorID,
	}
	if err := s.flowRepo.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	epic.ActiveFlowRunID = &run.ID
	if err := s.epicRepo.Update(ctx, epic); err != nil {
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
		_ = s.clearEpicActiveFlow(ctx, run.TargetID)
		return nil, err
	}
	s.publishRunEvent(run, actorID)
	return s.GetRunView(ctx, workspaceID, run.ID)
}

func (s *FlowService) bootstrapEpicPlanningRun(ctx context.Context, run *model.FlowRun, actorID string) error {
	if run == nil {
		return fmt.Errorf("flow run not found")
	}
	if derefString(run.CurrentNodeID) == model.FlowNodeSpecPlanning && run.Status == model.FlowStatusAwaitingInput {
		existingSpecNode, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, run.ID, model.FlowNodeSpecPlanning)
		if err == nil && existingSpecNode != nil && existingSpecNode.ChildSessionID != nil && *existingSpecNode.ChildSessionID != "" {
			return nil
		}
	}
	var input model.StartEpicPlanningFlowInput
	if err := json.Unmarshal(run.Input, &input); err != nil {
		return fmt.Errorf("invalid flow input: %w", err)
	}

	doc, err := s.agentService.EnsureEpicSpecDocument(ctx, run.WorkspaceID, run.TargetID, actorID)
	if err != nil {
		errMsg := err.Error()
		if markErr := s.markRunFailedByID(ctx, run.ID, model.FlowNodeEnsureSpecDoc, errMsg); markErr != nil {
			return markErr
		}
		return err
	}

	now := time.Now()
	ensureNode, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, run.ID, model.FlowNodeEnsureSpecDoc)
	if err != nil {
		return err
	}
	if ensureNode == nil {
		ensureOutput, _ := json.Marshal(map[string]any{"spec_document_id": doc.ID})
		ensureNode = &model.FlowNodeRun{
			FlowRunID:    run.ID,
			NodeID:       model.FlowNodeEnsureSpecDoc,
			NodeType:     model.FlowNodeTypeSystemAction,
			Status:       model.FlowNodeStatusCompleted,
			AttemptCount: 1,
			Output:       ensureOutput,
			StartedAt:    &now,
			CompletedAt:  &now,
		}
		if err := s.flowRepo.CreateNodeRun(ctx, ensureNode); err != nil {
			return err
		}
	}

	specPlannerID := strings.TrimSpace(input.SpecPlannerAgentID)
	specNode, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, run.ID, model.FlowNodeSpecPlanning)
	if err != nil {
		return err
	}
	if specNode == nil {
		specNode = &model.FlowNodeRun{
			FlowRunID:    run.ID,
			NodeID:       model.FlowNodeSpecPlanning,
			NodeType:     model.FlowNodeTypeInteractiveAgent,
			Status:       model.FlowNodeStatusAwaitingInput,
			AttemptCount: 1,
			AgentID:      &specPlannerID,
			Input:        json.RawMessage("{}"),
			Output:       json.RawMessage("{}"),
			StartedAt:    &now,
		}
		if err := s.flowRepo.CreateNodeRun(ctx, specNode); err != nil {
			return err
		}
	}

	if specNode.ChildSessionID == nil || *specNode.ChildSessionID == "" {
		existingSession, err := s.planningService.GetActiveSessionByEpicID(ctx, run.WorkspaceID, run.TargetID)
		if err != nil {
			return err
		}
		if existingSession != nil && existingSession.FlowNodeRunID != nil && *existingSession.FlowNodeRunID == specNode.ID {
			specNode.ChildSessionID = &existingSession.ID
			if err := s.flowRepo.UpdateNodeRun(ctx, specNode); err != nil {
				return err
			}
		}
	}

	if specNode.ChildSessionID == nil || *specNode.ChildSessionID == "" {
		session, err := s.planningService.StartSession(ctx, run.WorkspaceID, run.TargetID, actorID, model.StartPlanningSessionRequest{
			AgentID:           specPlannerID,
			AdditionalContext: stringPtrOrNil(strings.TrimSpace(input.AdditionalContext)),
			FlowRunID:         &run.ID,
			FlowNodeRunID:     &specNode.ID,
		})
		if err != nil {
			errMsg := err.Error()
			specNode.Status = model.FlowNodeStatusFailed
			specNode.ErrorMessage = &errMsg
			specNode.CompletedAt = &now
			_ = s.flowRepo.UpdateNodeRun(ctx, specNode)
			run.Status = model.FlowStatusFailed
			run.CompletedAt = &now
			run.CancellationReason = &errMsg
			_ = s.flowRepo.UpdateRun(ctx, run)
			s.publishNodeEvent(specNode, run.WorkspaceID, actorID)
			s.publishRunEvent(run, actorID)
			return err
		}
		specNode.ChildSessionID = &session.ID
		if err := s.flowRepo.UpdateNodeRun(ctx, specNode); err != nil {
			return err
		}
	}
	currentNode := model.FlowNodeSpecPlanning
	run.Status = model.FlowStatusAwaitingInput
	run.CurrentNodeID = &currentNode
	run.CompletedAt = nil
	run.CancellationReason = nil
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.publishNodeEvent(ensureNode, run.WorkspaceID, actorID)
	s.publishNodeEvent(specNode, run.WorkspaceID, actorID)
	s.publishRunEvent(run, actorID)
	return nil
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

func (s *FlowService) approveSpecNode(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string, payload json.RawMessage) error {
	var req model.ApproveEpicSpecRequest
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &req); err != nil {
			return fmt.Errorf("invalid approval payload: %w", err)
		}
	}
	summary, err := s.agentService.ApproveEpicSpec(ctx, run.WorkspaceID, run.TargetID, actorID, req)
	if err != nil {
		return err
	}
	now := time.Now()
	output, _ := json.Marshal(summary)
	nodeRun.Status = model.FlowNodeStatusCompleted
	nodeRun.Output = output
	nodeRun.CompletedAt = &now
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}

	storyPlannerID, additionalContext, err := s.readStoryPlanningContext(run.Input)
	if err != nil {
		return err
	}
	storyNode := &model.FlowNodeRun{
		FlowRunID:    run.ID,
		NodeID:       model.FlowNodeStoryPlanning,
		NodeType:     model.FlowNodeTypeAgentTask,
		Status:       model.FlowNodeStatusRunning,
		AttemptCount: 1,
		AgentID:      &storyPlannerID,
		Input:        json.RawMessage("{}"),
		Output:       json.RawMessage("{}"),
	}
	startedAt := time.Now()
	storyNode.StartedAt = &startedAt
	if err := s.flowRepo.CreateNodeRun(ctx, storyNode); err != nil {
		return err
	}
	planRun, err := s.startStoryPlanningChildRun(ctx, run, storyNode, actorID, storyPlannerID, additionalContext)
	if err != nil {
		errMsg := err.Error()
		storyNode.Status = model.FlowNodeStatusFailed
		storyNode.ErrorMessage = &errMsg
		storyNode.CompletedAt = &startedAt
		_ = s.flowRepo.UpdateNodeRun(ctx, storyNode)
		run.Status = model.FlowStatusFailed
		run.CompletedAt = &startedAt
		run.CancellationReason = &errMsg
		_ = s.flowRepo.UpdateRun(ctx, run)
		return err
	}
	storyNode.ChildRunID = &planRun.ID
	if err := s.flowRepo.UpdateNodeRun(ctx, storyNode); err != nil {
		return err
	}
	currentNode := model.FlowNodeStoryPlanning
	run.Status = model.FlowStatusRunning
	run.CurrentNodeID = &currentNode
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
	s.publishNodeEvent(storyNode, run.WorkspaceID, actorID)
	s.publishRunEvent(run, actorID)
	return nil
}

func (s *FlowService) approvePlanNode(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string, payload json.RawMessage) error {
	storyNode, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, run.ID, model.FlowNodeStoryPlanning)
	if err != nil {
		return err
	}
	if storyNode == nil || storyNode.ChildRunID == nil || *storyNode.ChildRunID == "" {
		return fmt.Errorf("story planning run is missing")
	}

	var req model.ConfirmPlanningRequest
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &req); err != nil {
			return fmt.Errorf("invalid plan approval payload: %w", err)
		}
	}
	req.RunID = *storyNode.ChildRunID

	createNode, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, run.ID, model.FlowNodeCreateStories)
	if err != nil {
		return err
	}
	now := time.Now()
	if createNode == nil || createNode.Status == model.FlowNodeStatusCompleted {
		createNode = &model.FlowNodeRun{
			FlowRunID:    run.ID,
			NodeID:       model.FlowNodeCreateStories,
			NodeType:     model.FlowNodeTypeSystemAction,
			Status:       model.FlowNodeStatusRunning,
			AttemptCount: 1,
			Input:        mustJSON(req),
			Output:       json.RawMessage("{}"),
			StartedAt:    &now,
		}
		if err := s.flowRepo.CreateNodeRun(ctx, createNode); err != nil {
			return err
		}
	} else {
		createNode.AttemptCount++
		createNode.Status = model.FlowNodeStatusRunning
		createNode.ErrorMessage = nil
		createNode.Input = mustJSON(req)
		createNode.StartedAt = &now
		createNode.CompletedAt = nil
		if err := s.flowRepo.UpdateNodeRun(ctx, createNode); err != nil {
			return err
		}
	}
	currentCreateNode := model.FlowNodeCreateStories
	run.Status = model.FlowStatusRunning
	run.CurrentNodeID = &currentCreateNode
	run.CompletedAt = nil
	run.CancellationReason = nil
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}

	createdStories, err := s.agentService.ConfirmEpicRun(ctx, run.WorkspaceID, run.TargetID, *storyNode.ChildRunID, actorID, req)
	if err != nil {
		errMsg := err.Error()
		createNode.Status = model.FlowNodeStatusFailed
		createNode.ErrorMessage = &errMsg
		createNode.CompletedAt = &now
		_ = s.flowRepo.UpdateNodeRun(ctx, createNode)
		run.Status = model.FlowStatusFailed
		run.CompletedAt = &now
		run.CancellationReason = &errMsg
		run.CurrentNodeID = &currentCreateNode
		_ = s.flowRepo.UpdateRun(ctx, run)
		return err
	}

	createdIDs := make([]string, 0, len(createdStories))
	for _, story := range createdStories {
		createdIDs = append(createdIDs, story.ID)
	}

	nodeRun.Status = model.FlowNodeStatusCompleted
	nodeRun.Output = mustJSON(map[string]any{"approved_run_id": *storyNode.ChildRunID})
	nodeRun.CompletedAt = &now
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	createNode.Status = model.FlowNodeStatusCompleted
	createNode.Output = mustJSON(map[string]any{"created_story_ids": createdIDs})
	createNode.CompletedAt = &now
	if err := s.flowRepo.UpdateNodeRun(ctx, createNode); err != nil {
		return err
	}

	doneNode := &model.FlowNodeRun{
		FlowRunID:    run.ID,
		NodeID:       model.FlowNodeDone,
		NodeType:     model.FlowNodeTypeTerminal,
		Status:       model.FlowNodeStatusCompleted,
		AttemptCount: 1,
		Output:       mustJSON(map[string]any{"created_story_ids": createdIDs}),
		StartedAt:    &now,
		CompletedAt:  &now,
	}
	if err := s.flowRepo.CreateNodeRun(ctx, doneNode); err != nil {
		return err
	}

	run.Status = model.FlowStatusCompleted
	run.CurrentNodeID = strPtr(model.FlowNodeDone)
	run.CompletedAt = &now
	run.OutputSummary = mustJSON(map[string]any{"created_story_ids": createdIDs})
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if err := s.clearEpicActiveFlow(ctx, run.TargetID); err != nil {
		return err
	}
	s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
	s.publishNodeEvent(createNode, run.WorkspaceID, actorID)
	s.publishNodeEvent(doneNode, run.WorkspaceID, actorID)
	s.publishRunEvent(run, actorID)
	return nil
}

func (s *FlowService) rejectApprovalNode(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) error {
	now := time.Now()
	nodeRun.Status = model.FlowNodeStatusCancelled
	nodeRun.CompletedAt = &now
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	reason := "rejected"
	run.Status = model.FlowStatusCancelled
	run.CompletedAt = &now
	run.CancellationReason = &reason
	run.CurrentNodeID = strPtr(nodeRun.NodeID)
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if err := s.clearEpicActiveFlow(ctx, run.TargetID); err != nil {
		return err
	}
	s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
	s.publishRunEvent(run, actorID)
	return nil
}

func (s *FlowService) restartStoryPlanningNode(ctx context.Context, run *model.FlowRun, previous *model.FlowNodeRun, actorID string) error {
	storyPlannerID, additionalContext, err := s.readStoryPlanningContext(run.Input)
	if err != nil {
		return err
	}
	now := time.Now()
	retryNode := &model.FlowNodeRun{
		FlowRunID:    run.ID,
		NodeID:       model.FlowNodeStoryPlanning,
		NodeType:     model.FlowNodeTypeAgentTask,
		Status:       model.FlowNodeStatusRunning,
		AttemptCount: previous.AttemptCount + 1,
		AgentID:      &storyPlannerID,
		Input:        json.RawMessage("{}"),
		Output:       json.RawMessage("{}"),
		StartedAt:    &now,
	}
	if err := s.flowRepo.CreateNodeRun(ctx, retryNode); err != nil {
		return err
	}
	planRun, err := s.startStoryPlanningChildRun(ctx, run, retryNode, actorID, storyPlannerID, additionalContext)
	if err != nil {
		errMsg := err.Error()
		retryNode.Status = model.FlowNodeStatusFailed
		retryNode.ErrorMessage = &errMsg
		retryNode.CompletedAt = &now
		_ = s.flowRepo.UpdateNodeRun(ctx, retryNode)
		return err
	}
	retryNode.ChildRunID = &planRun.ID
	if err := s.flowRepo.UpdateNodeRun(ctx, retryNode); err != nil {
		return err
	}
	currentNode := model.FlowNodeStoryPlanning
	run.Status = model.FlowStatusRunning
	run.CurrentNodeID = &currentNode
	run.CompletedAt = nil
	run.CancellationReason = nil
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	s.publishNodeEvent(retryNode, run.WorkspaceID, actorID)
	s.publishRunEvent(run, actorID)
	return nil
}

func (s *FlowService) startStoryPlanningChildRun(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID, plannerAgentID, additionalContext string) (*model.AgentRun, error) {
	if nodeRun.ChildRunID != nil && *nodeRun.ChildRunID != "" {
		existing, err := s.runRepo.GetByIDAny(ctx, *nodeRun.ChildRunID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}
	activeRun, err := s.runRepo.FindActiveByTarget(ctx, run.WorkspaceID, "epic", run.TargetID)
	if err != nil {
		return nil, err
	}
	if activeRun != nil && (activeRun.FlowNodeRunID != nil && *activeRun.FlowNodeRunID == nodeRun.ID) {
		return activeRun, nil
	}
	return s.agentService.StartEpicStoryPlanningForFlow(ctx, run.WorkspaceID, run.TargetID, actorID, plannerAgentID, additionalContext, run.ID, nodeRun.ID)
}

func (s *FlowService) retryCreateStoriesNode(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) error {
	var req model.ConfirmPlanningRequest
	if err := json.Unmarshal(nodeRun.Input, &req); err != nil {
		return fmt.Errorf("create stories node input is invalid: %w", err)
	}
	storyNode, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, run.ID, model.FlowNodeStoryPlanning)
	if err != nil {
		return err
	}
	if storyNode == nil || storyNode.ChildRunID == nil || *storyNode.ChildRunID == "" {
		return fmt.Errorf("story planning run is missing")
	}
	req.RunID = *storyNode.ChildRunID
	now := time.Now()
	nodeRun.AttemptCount++
	nodeRun.Status = model.FlowNodeStatusRunning
	nodeRun.ErrorMessage = nil
	nodeRun.StartedAt = &now
	nodeRun.CompletedAt = nil
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	createdStories, err := s.agentService.ConfirmEpicRun(ctx, run.WorkspaceID, run.TargetID, *storyNode.ChildRunID, actorID, req)
	if err != nil {
		errMsg := err.Error()
		nodeRun.Status = model.FlowNodeStatusFailed
		nodeRun.ErrorMessage = &errMsg
		nodeRun.CompletedAt = &now
		_ = s.flowRepo.UpdateNodeRun(ctx, nodeRun)
		return err
	}
	createdIDs := make([]string, 0, len(createdStories))
	for _, story := range createdStories {
		createdIDs = append(createdIDs, story.ID)
	}
	nodeRun.Status = model.FlowNodeStatusCompleted
	nodeRun.Output = mustJSON(map[string]any{"created_story_ids": createdIDs})
	nodeRun.CompletedAt = &now
	if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
		return err
	}
	doneNode := &model.FlowNodeRun{
		FlowRunID:    run.ID,
		NodeID:       model.FlowNodeDone,
		NodeType:     model.FlowNodeTypeTerminal,
		Status:       model.FlowNodeStatusCompleted,
		AttemptCount: 1,
		Output:       mustJSON(map[string]any{"created_story_ids": createdIDs}),
		StartedAt:    &now,
		CompletedAt:  &now,
	}
	if err := s.flowRepo.CreateNodeRun(ctx, doneNode); err != nil {
		return err
	}
	run.Status = model.FlowStatusCompleted
	run.CurrentNodeID = strPtr(model.FlowNodeDone)
	run.CompletedAt = &now
	run.CancellationReason = nil
	run.OutputSummary = mustJSON(map[string]any{"created_story_ids": createdIDs})
	if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
		return err
	}
	if err := s.clearEpicActiveFlow(ctx, run.TargetID); err != nil {
		return err
	}
	s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
	s.publishNodeEvent(doneNode, run.WorkspaceID, actorID)
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
	if err := s.clearEpicActiveFlow(ctx, run.TargetID); err != nil {
		return err
	}
	s.publishRunEvent(run, actorID)
	return nil
}

func (s *FlowService) syncRunState(ctx context.Context, run *model.FlowRun) error {
	nodeRuns, err := s.flowRepo.ListNodeRuns(ctx, run.ID)
	if err != nil {
		return err
	}
	currentNodeID := derefString(run.CurrentNodeID)
	activeNode := latestNodeRun(nodeRuns, currentNodeID)
	if activeNode == nil {
		return nil
	}

	beforeRunStatus := run.Status
	beforeCurrentNodeID := derefString(run.CurrentNodeID)
	beforeNodeStatus := activeNode.Status
	childID := ""
	childStatus := ""

	logReconciled := func() {
		if beforeRunStatus == run.Status && beforeCurrentNodeID == derefString(run.CurrentNodeID) && beforeNodeStatus == activeNode.Status {
			return
		}
		s.logger.InfoContext(ctx, "flow reconciliation updated state",
			"flow_run_id", run.ID,
			"node_run_id", activeNode.ID,
			"node_id", activeNode.NodeID,
			"previous_run_status", beforeRunStatus,
			"new_run_status", run.Status,
			"previous_current_node_id", beforeCurrentNodeID,
			"new_current_node_id", derefString(run.CurrentNodeID),
			"previous_node_status", beforeNodeStatus,
			"new_node_status", activeNode.Status,
			"child_id", childID,
			"child_status", childStatus,
		)
	}

	switch activeNode.NodeID {
	case model.FlowNodeSpecPlanning:
		if activeNode.ChildSessionID == nil || *activeNode.ChildSessionID == "" {
			return nil
		}
		session, err := s.sessionRepo.GetByID(ctx, *activeNode.ChildSessionID)
		if err != nil || session == nil {
			return err
		}
		childID = session.ID
		childStatus = session.Status
		if err := s.applyPlanningSessionState(ctx, run, activeNode, session, ""); err != nil {
			return err
		}
		logReconciled()
		return nil
	case model.FlowNodeStoryPlanning:
		if activeNode.ChildRunID == nil || *activeNode.ChildRunID == "" {
			return nil
		}
		childRun, err := s.runRepo.GetByIDAny(ctx, *activeNode.ChildRunID)
		if err != nil || childRun == nil {
			return err
		}
		childID = childRun.ID
		childStatus = childRun.Status
		if err := s.applyAgentRunState(ctx, run, activeNode, childRun, ""); err != nil {
			return err
		}
		logReconciled()
		return nil
	}
	return nil
}

func (s *FlowService) HandleChildState(ctx context.Context, flowRunID, nodeRunID, childType, childID, childStatus string) error {
	run, err := s.GetRunByIDAny(ctx, flowRunID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("flow run not found")
	}
	nodeRun, err := s.GetNodeRunByIDAny(ctx, nodeRunID)
	if err != nil {
		return err
	}
	if nodeRun == nil || nodeRun.FlowRunID != flowRunID {
		return fmt.Errorf("flow node run not found")
	}

	switch childType {
	case temporalapp.FlowChildTypeAgentRun:
		childRun, err := s.runRepo.GetByIDAny(ctx, childID)
		if err != nil {
			return err
		}
		if childRun == nil {
			return fmt.Errorf("child agent run not found")
		}
		return s.applyAgentRunState(ctx, run, nodeRun, childRun, childStatus)
	case temporalapp.FlowChildTypePlanningSession:
		session, err := s.sessionRepo.GetByID(ctx, childID)
		if err != nil {
			return err
		}
		if session == nil {
			return fmt.Errorf("planning session not found")
		}
		return s.applyPlanningSessionState(ctx, run, nodeRun, session, childStatus)
	default:
		return nil
	}
}

func (s *FlowService) applyPlanningSessionState(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, session *model.PlanningSession, childStatus string) error {
	if run == nil || nodeRun == nil || session == nil || nodeRun.NodeID != model.FlowNodeSpecPlanning {
		return nil
	}
	status := strings.TrimSpace(childStatus)
	if status == "" {
		status = session.Status
	}
	switch status {
	case model.PlanningSessionStatusActive, model.PlanningSessionStatusPaused, model.PlanningSessionStatusFinalizing:
		return nil
	case model.PlanningSessionStatusAbandoned:
		if derefString(run.CurrentNodeID) != model.FlowNodeSpecPlanning {
			return nil
		}
		if err := s.markRunCancelled(ctx, run, nodeRun, "interactive_session_abandoned"); err != nil {
			return err
		}
		s.publishNodeEvent(nodeRun, run.WorkspaceID, "")
		s.publishRunEvent(run, "")
		return nil
	case model.PlanningSessionStatusCompleted:
		if derefString(run.CurrentNodeID) != model.FlowNodeSpecPlanning {
			return nil
		}
		now := time.Now()
		nodeRun.Status = model.FlowNodeStatusCompleted
		nodeRun.CompletedAt = &now
		nodeRun.Output = mustJSON(map[string]any{"session_id": session.ID, "spec_document_id": derefString(session.SpecDocumentID)})
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
		s.publishNodeEvent(nodeRun, run.WorkspaceID, "")
		s.publishRunEvent(run, "")
		return nil
	default:
		return nil
	}
}

func (s *FlowService) applyAgentRunState(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, childRun *model.AgentRun, childStatus string) error {
	if run == nil || nodeRun == nil || childRun == nil || nodeRun.NodeID != model.FlowNodeStoryPlanning {
		return nil
	}
	status := strings.TrimSpace(childStatus)
	if status == "" {
		status = childRun.Status
	}
	switch status {
	case model.AgentRunStatusQueued, model.AgentRunStatusRunning:
		return nil
	case model.AgentRunStatusAwaitingApproval, model.AgentRunStatusCompleted:
		if derefString(run.CurrentNodeID) != model.FlowNodeStoryPlanning {
			return nil
		}
		now := time.Now()
		nodeRun.Status = model.FlowNodeStatusCompleted
		nodeRun.CompletedAt = &now
		nodeRun.Output = mustJSON(map[string]any{"agent_run_id": childRun.ID, "approval_state": childRun.ApprovalState})
		if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
			return err
		}
		if _, err := s.ensureApprovalGateNode(ctx, run.ID, model.FlowNodePlanApproval); err != nil {
			return err
		}
		currentNode := model.FlowNodePlanApproval
		run.Status = model.FlowStatusAwaitingApproval
		run.CurrentNodeID = &currentNode
		if err := s.flowRepo.UpdateRun(ctx, run); err != nil {
			return err
		}
		s.publishNodeEvent(nodeRun, run.WorkspaceID, "")
		s.publishRunEvent(run, "")
		return nil
	case model.AgentRunStatusFailed:
		errMsg := stringPtrValue(childRun.ErrorMessage)
		if errMsg == "" {
			errMsg = "agent run failed"
		}
		if err := s.markRunFailed(ctx, run, nodeRun, errMsg); err != nil {
			return err
		}
		s.publishNodeEvent(nodeRun, run.WorkspaceID, "")
		s.publishRunEvent(run, "")
		return nil
	case model.AgentRunStatusCancelled:
		if err := s.markRunCancelled(ctx, run, nodeRun, "agent_run_cancelled"); err != nil {
			return err
		}
		s.publishNodeEvent(nodeRun, run.WorkspaceID, "")
		s.publishRunEvent(run, "")
		return nil
	default:
		return nil
	}
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
	return s.clearEpicActiveFlow(ctx, run.TargetID)
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
	switch templateID {
	case model.FlowTemplateEpicPlanningV1:
		return model.FlowSpec{
			TemplateID:        model.FlowTemplateEpicPlanningV1,
			TemplateVersion:   1,
			TargetType:        "epic",
			SupportedTriggers: []string{model.FlowTriggerManual, model.FlowTriggerInternalDomainHook},
			Nodes: []model.FlowNodeSpec{
				{ID: model.FlowNodeEnsureSpecDoc, Type: model.FlowNodeTypeSystemAction},
				{ID: model.FlowNodeSpecPlanning, Type: model.FlowNodeTypeInteractiveAgent, RequiredMode: model.InvocationModeInteractive, Actions: []string{model.FlowActionFinalize}},
				{ID: model.FlowNodeSpecApproval, Type: model.FlowNodeTypeApprovalGate, Actions: []string{model.FlowActionApprove, model.FlowActionReject}},
				{ID: model.FlowNodeStoryPlanning, Type: model.FlowNodeTypeAgentTask, RequiredMode: model.InvocationModeAutonomous},
				{ID: model.FlowNodePlanApproval, Type: model.FlowNodeTypeApprovalGate, Actions: []string{model.FlowActionApprove, model.FlowActionReject}},
				{ID: model.FlowNodeCreateStories, Type: model.FlowNodeTypeSystemAction},
				{ID: model.FlowNodeDone, Type: model.FlowNodeTypeTerminal},
			},
		}
	default:
		return model.FlowSpec{}
	}
}

func (s *FlowService) supportsNodeAction(nodeRun *model.FlowNodeRun, action string) bool {
	switch {
	case nodeRun == nil:
		return false
	case nodeRun.NodeType == model.FlowNodeTypeInteractiveAgent && action == model.FlowActionFinalize:
		return true
	case nodeRun.NodeType == model.FlowNodeTypeApprovalGate && (action == model.FlowActionApprove || action == model.FlowActionReject):
		return true
	default:
		return false
	}
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
