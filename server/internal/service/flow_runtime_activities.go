package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

type FlowRuntimeActivities struct {
	flowService *FlowService
}

func NewFlowRuntimeActivities(flowService *FlowService) *FlowRuntimeActivities {
	return &FlowRuntimeActivities{flowService: flowService}
}

func (a *FlowRuntimeActivities) BootstrapRunActivity(ctx context.Context, flowRunID, actorID string) error {
	run, err := a.flowService.GetRunByIDAny(ctx, flowRunID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("flow run not found")
	}
	return a.flowService.genericBootstrapRun(ctx, run, actorID)
}

func (a *FlowRuntimeActivities) BootstrapEpicPlanningRunActivity(ctx context.Context, flowRunID, actorID string) error {
	run, err := a.flowService.GetRunByIDAny(ctx, flowRunID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("flow run not found")
	}
	return a.flowService.bootstrapEpicPlanningRun(ctx, run, actorID)
}

func (a *FlowRuntimeActivities) HandleApprovalActionActivity(ctx context.Context, flowRunID, actorID, action string, payload json.RawMessage) error {
	run, nodeRun, err := a.flowService.loadCurrentNode(ctx, flowRunID)
	if err != nil {
		return err
	}
	return a.flowService.genericHandleApprovalAction(ctx, run, nodeRun, actorID, action, payload)
}

func (a *FlowRuntimeActivities) FinalizeInteractiveNodeActivity(ctx context.Context, flowRunID, actorID string) error {
	run, nodeRun, err := a.flowService.loadCurrentNode(ctx, flowRunID)
	if err != nil {
		return err
	}
	return a.flowService.finalizeInteractiveNode(ctx, run, nodeRun, actorID)
}

func (a *FlowRuntimeActivities) ApproveSpecNodeActivity(ctx context.Context, flowRunID, actorID string, payload json.RawMessage) error {
	run, nodeRun, err := a.flowService.loadCurrentNode(ctx, flowRunID)
	if err != nil {
		return err
	}
	return a.flowService.approveSpecNode(ctx, run, nodeRun, actorID, payload)
}

func (a *FlowRuntimeActivities) ApprovePlanNodeActivity(ctx context.Context, flowRunID, actorID string, payload json.RawMessage) error {
	run, nodeRun, err := a.flowService.loadCurrentNode(ctx, flowRunID)
	if err != nil {
		return err
	}
	return a.flowService.approvePlanNode(ctx, run, nodeRun, actorID, payload)
}

func (a *FlowRuntimeActivities) RejectApprovalNodeActivity(ctx context.Context, flowRunID, actorID string) error {
	run, nodeRun, err := a.flowService.loadCurrentNode(ctx, flowRunID)
	if err != nil {
		return err
	}
	return a.flowService.rejectApprovalNode(ctx, run, nodeRun, actorID)
}

func (a *FlowRuntimeActivities) RetryNodeActivity(ctx context.Context, flowRunID, nodeRunID, actorID string) error {
	run, err := a.flowService.GetRunByIDAny(ctx, flowRunID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("flow run not found")
	}
	nodeRun, err := a.flowService.GetNodeRunByIDAny(ctx, nodeRunID)
	if err != nil {
		return err
	}
	if nodeRun == nil || nodeRun.FlowRunID != flowRunID {
		return fmt.Errorf("flow node run not found")
	}
	if run.TemplateID != model.FlowTemplateEpicPlanningV1 {
		return a.flowService.genericRetryNode(ctx, run, nodeRun, actorID)
	}
	switch nodeRun.NodeID {
	case model.FlowNodeStoryPlanning:
		return a.flowService.restartStoryPlanningNode(ctx, run, nodeRun, actorID)
	case model.FlowNodeCreateStories:
		return a.flowService.retryCreateStoriesNode(ctx, run, nodeRun, actorID)
	default:
		return fmt.Errorf("retry is not supported for node %q", nodeRun.NodeID)
	}
}

func (a *FlowRuntimeActivities) CancelRunActivity(ctx context.Context, flowRunID, actorID, reason string) error {
	run, err := a.flowService.GetRunByIDAny(ctx, flowRunID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("flow run not found")
	}
	return a.flowService.cancelRunNow(ctx, run, actorID, reason)
}

func (a *FlowRuntimeActivities) ProgressRunStateActivity(ctx context.Context, flowRunID string) (temporalapp.FlowProgressResult, error) {
	run, err := a.flowService.GetRunByIDAny(ctx, flowRunID)
	if err != nil {
		return temporalapp.FlowProgressResult{}, err
	}
	if run == nil {
		return temporalapp.FlowProgressResult{}, fmt.Errorf("flow run not found")
	}
	if run.TemplateID != model.FlowTemplateEpicPlanningV1 {
		return a.flowService.genericProgressRunState(ctx, flowRunID)
	}
	if err := a.flowService.syncRunState(ctx, run); err != nil {
		return temporalapp.FlowProgressResult{}, err
	}
	state, err := a.LoadRunStateActivity(ctx, flowRunID)
	if err != nil {
		return temporalapp.FlowProgressResult{}, err
	}
	return temporalapp.FlowProgressResult{
		State:        state,
		WaitingChild: state.Status == model.FlowStatusRunning && state.CurrentNodeID == model.FlowNodeStoryPlanning,
	}, nil
}

func (a *FlowRuntimeActivities) LoadRunStateActivity(ctx context.Context, flowRunID string) (temporalapp.FlowRuntimeState, error) {
	return a.flowService.LoadRuntimeState(ctx, flowRunID)
}

func (a *FlowRuntimeActivities) HandleChildStateActivity(ctx context.Context, flowRunID, nodeRunID, childType, childID, childStatus string) error {
	run, err := a.flowService.GetRunByIDAny(ctx, flowRunID)
	if err != nil {
		return err
	}
	if run != nil && run.TemplateID != model.FlowTemplateEpicPlanningV1 {
		return a.flowService.genericHandleChildState(ctx, flowRunID, nodeRunID, childType, childID, childStatus)
	}
	return a.flowService.HandleChildState(ctx, flowRunID, nodeRunID, childType, childID, childStatus)
}
