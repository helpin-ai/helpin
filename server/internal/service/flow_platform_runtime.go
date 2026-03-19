package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

func (s *FlowService) genericHandleApprovalAction(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID, action string, payload json.RawMessage) error {
	def, nodeDef, err := s.lookupNodeForRun(ctx, run, nodeRun.NodeID)
	if err != nil {
		return err
	}
	now := time.Now()
	decision := model.FlowApprovalDecision{
		Decision:  action,
		ActorID:   actorID,
		DecidedAt: &now,
	}
	switch action {
	case model.FlowActionApprove:
		decision.OverridePayload = normalizeRawJSON(payload)
	case model.FlowActionRequestChanges:
		var req model.FlowRequestChangesPayload
		if err := json.Unmarshal(payload, &req); err != nil {
			return fmt.Errorf("invalid request_changes payload: %w", err)
		}
		if strings.TrimSpace(req.Comment) == "" {
			return fmt.Errorf("request_changes comment is required")
		}
		decision.Comment = strings.TrimSpace(req.Comment)
		decision.StructuredFeedback = normalizeRawJSON(req.StructuredFeedback)
	case model.FlowActionReject:
		decision.OverridePayload = normalizeRawJSON(payload)
	default:
		return fmt.Errorf("unsupported approval action %q", action)
	}

	output := mustJSON(decision)
	nodeRun.Output = output
	nodeRun.CompletedAt = &now

	switch action {
	case model.FlowActionReject:
		nodeRun.Status = model.FlowNodeStatusCancelled
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
		if def.clearActiveTarget != nil {
			if err := def.clearActiveTarget(ctx, s, run); err != nil {
				return err
			}
		}
		s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
		s.publishRunEvent(run, actorID)
		return nil
	case model.FlowActionRequestChanges:
		nodeRun.Status = model.FlowNodeStatusCompleted
		if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
			return err
		}
		s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
		return s.enterNode(ctx, run, derefString(nodeDef.spec.LoopbackNodeID), actorID, true)
	case model.FlowActionApprove:
		nodeRun.Status = model.FlowNodeStatusCompleted
		if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
			return err
		}
		if nodeDef.approveCommandName != "" {
			commandInput := json.RawMessage("{}")
			if nodeDef.buildApproveCommandInput != nil {
				commandInput, err = nodeDef.buildApproveCommandInput(ctx, s, run, nodeRun, actorID, payload)
				if err != nil {
					return err
				}
			}
			commandOutput, err := s.commandService.Execute(ctx, s.commandMeta(run, nodeRun, actorID), nodeDef.approveCommandName, commandInput)
			if err != nil {
				errMsg := err.Error()
				nodeRun.ErrorMessage = &errMsg
				_ = s.flowRepo.UpdateNodeRun(ctx, nodeRun)
				return s.markRunFailed(ctx, run, nodeRun, errMsg)
			}
			nodeRun.Output = mustJSON(map[string]any{
				"decision":         decision.Decision,
				"actor_id":         decision.ActorID,
				"decided_at":       decision.DecidedAt,
				"override_payload": normalizeRawJSON(decision.OverridePayload),
				"command_result":   json.RawMessage(commandOutput),
			})
			if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
				return err
			}
		}
		s.publishNodeEvent(nodeRun, run.WorkspaceID, actorID)
		if nodeDef.nextNodeID == "" {
			s.publishRunEvent(run, actorID)
			return nil
		}
		return s.enterNode(ctx, run, nodeDef.nextNodeID, actorID, false)
	default:
		return nil
	}
}

func (s *FlowService) genericRetryNode(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, actorID string) error {
	_, nodeDef, err := s.lookupNodeForRun(ctx, run, nodeRun.NodeID)
	if err != nil {
		return err
	}
	if !nodeDef.retryable {
		return fmt.Errorf("retry is not supported for node %q", nodeRun.NodeID)
	}
	return s.enterNode(ctx, run, nodeRun.NodeID, actorID, true)
}

func (s *FlowService) genericProgressRunState(ctx context.Context, flowRunID string) (temporalapp.FlowProgressResult, error) {
	run, err := s.GetRunByIDAny(ctx, flowRunID)
	if err != nil {
		return temporalapp.FlowProgressResult{}, err
	}
	if run == nil {
		return temporalapp.FlowProgressResult{}, fmt.Errorf("flow run not found")
	}
	if err := s.genericSyncRunState(ctx, run); err != nil {
		return temporalapp.FlowProgressResult{}, err
	}
	state, err := s.LoadRuntimeState(ctx, flowRunID)
	if err != nil {
		return temporalapp.FlowProgressResult{}, err
	}
	waitingChild := false
	if state.CurrentNodeID != "" {
		_, nodeDef, lookupErr := s.lookupNodeForRun(ctx, run, state.CurrentNodeID)
		if lookupErr == nil && run.Status == model.FlowStatusRunning && nodeDef.spec.Type == model.FlowNodeTypeAgentTask {
			waitingChild = true
		}
	}
	return temporalapp.FlowProgressResult{State: state, WaitingChild: waitingChild}, nil
}

func (s *FlowService) genericHandleChildState(ctx context.Context, flowRunID, nodeRunID, childType, childID, childStatus string) error {
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
	case temporalapp.FlowChildTypePlanningSession:
		session, err := s.sessionRepo.GetByID(ctx, childID)
		if err != nil {
			return err
		}
		if session == nil {
			return fmt.Errorf("planning session not found")
		}
		return s.applyPlanningSessionStateGeneric(ctx, run, nodeRun, session, childStatus)
	case temporalapp.FlowChildTypeAgentRun:
		childRun, err := s.runRepo.GetByIDAny(ctx, childID)
		if err != nil {
			return err
		}
		if childRun == nil {
			return fmt.Errorf("child agent run not found")
		}
		return s.applyAgentRunStateGeneric(ctx, run, nodeRun, childRun, childStatus)
	default:
		return nil
	}
}

func (s *FlowService) genericSyncRunState(ctx context.Context, run *model.FlowRun) error {
	if run == nil {
		return nil
	}
	currentNodeID := derefString(run.CurrentNodeID)
	if currentNodeID == "" {
		return nil
	}
	nodeRun, err := s.flowRepo.GetLatestNodeRunByNodeID(ctx, run.ID, currentNodeID)
	if err != nil {
		return err
	}
	if nodeRun == nil {
		return s.ensureCurrentNodeAttempt(ctx, run, "")
	}
	_, nodeDef, err := s.lookupNodeForRun(ctx, run, currentNodeID)
	if err != nil {
		return err
	}
	switch nodeDef.spec.Type {
	case model.FlowNodeTypeInteractiveAgent:
		if nodeRun.ChildSessionID == nil || *nodeRun.ChildSessionID == "" {
			return nil
		}
		session, err := s.sessionRepo.GetByID(ctx, *nodeRun.ChildSessionID)
		if err != nil || session == nil {
			return err
		}
		return s.applyPlanningSessionStateGeneric(ctx, run, nodeRun, session, "")
	case model.FlowNodeTypeAgentTask:
		if nodeRun.ChildRunID == nil || *nodeRun.ChildRunID == "" {
			return nil
		}
		childRun, err := s.runRepo.GetByIDAny(ctx, *nodeRun.ChildRunID)
		if err != nil || childRun == nil {
			return err
		}
		return s.applyAgentRunStateGeneric(ctx, run, nodeRun, childRun, "")
	default:
		return nil
	}
}

func (s *FlowService) applyPlanningSessionStateGeneric(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, session *model.PlanningSession, childStatus string) error {
	if run == nil || nodeRun == nil || session == nil {
		return nil
	}
	_, nodeDef, err := s.lookupNodeForRun(ctx, run, nodeRun.NodeID)
	if err != nil {
		return err
	}
	status := strings.TrimSpace(childStatus)
	if status == "" {
		status = session.Status
	}
	switch status {
	case model.PlanningSessionStatusActive, model.PlanningSessionStatusPaused, model.PlanningSessionStatusFinalizing:
		return nil
	case model.PlanningSessionStatusAbandoned:
		if derefString(run.CurrentNodeID) != nodeRun.NodeID {
			return nil
		}
		if err := s.markRunCancelled(ctx, run, nodeRun, "interactive_session_abandoned"); err != nil {
			return err
		}
		s.publishNodeEvent(nodeRun, run.WorkspaceID, "")
		s.publishRunEvent(run, "")
		return nil
	case model.PlanningSessionStatusCompleted:
		if derefString(run.CurrentNodeID) != nodeRun.NodeID {
			return nil
		}
		now := time.Now()
		nodeRun.Status = model.FlowNodeStatusCompleted
		nodeRun.CompletedAt = &now
		nodeRun.Output = mustJSON(map[string]any{
			"session_id":       session.ID,
			"spec_document_id": derefString(session.SpecDocumentID),
			"planning_status":  session.Status,
		})
		if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
			return err
		}
		s.publishNodeEvent(nodeRun, run.WorkspaceID, "")
		if nodeDef.nextNodeID == "" {
			return nil
		}
		return s.enterNode(ctx, run, nodeDef.nextNodeID, "", false)
	default:
		return nil
	}
}

func (s *FlowService) applyAgentRunStateGeneric(ctx context.Context, run *model.FlowRun, nodeRun *model.FlowNodeRun, childRun *model.AgentRun, childStatus string) error {
	if run == nil || nodeRun == nil || childRun == nil {
		return nil
	}
	_, nodeDef, err := s.lookupNodeForRun(ctx, run, nodeRun.NodeID)
	if err != nil {
		return err
	}
	status := strings.TrimSpace(childStatus)
	if status == "" {
		status = childRun.Status
	}
	switch status {
	case model.AgentRunStatusQueued, model.AgentRunStatusRunning:
		return nil
	case model.AgentRunStatusAwaitingApproval, model.AgentRunStatusCompleted:
		if derefString(run.CurrentNodeID) != nodeRun.NodeID {
			return nil
		}
		now := time.Now()
		nodeRun.Status = model.FlowNodeStatusCompleted
		nodeRun.CompletedAt = &now
		nodeRun.Output = mustJSON(map[string]any{
			"agent_run_id":    childRun.ID,
			"approval_state":  childRun.ApprovalState,
			"output_summary":  json.RawMessage(childRun.OutputSummary),
			"execution_stage": derefString(childRun.ExecutionStage),
		})
		if err := s.flowRepo.UpdateNodeRun(ctx, nodeRun); err != nil {
			return err
		}
		s.publishNodeEvent(nodeRun, run.WorkspaceID, "")
		if nodeDef.nextNodeID == "" {
			return nil
		}
		return s.enterNode(ctx, run, nodeDef.nextNodeID, "", false)
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

func normalizeRawJSON(raw json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	return raw
}
