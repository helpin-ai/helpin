package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// RestartPausedEpicStep cancels a paused run and replaces only its DAG step.
// The caller explicitly chooses the credentials for this and future AI steps.
func (s *CommandBarService) RestartPausedEpicStep(ctx context.Context, workspaceID, actorID, planID string, req model.CommandBarRestartPausedStepRequest) (*model.CommandBarRetryPlanResponse, error) {
	if s == nil || s.planRepo == nil || s.agentService == nil || s.agentService.runRepo == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	if !req.ReviewedPartialWork {
		return nil, fmt.Errorf("review existing task branch work before restarting this step")
	}
	plan, err := s.planRepo.GetByID(ctx, workspaceID, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("delivery plan not found")
	}
	if plan.Status != model.CommandBarPlanStatusRunning && plan.Status != model.CommandBarPlanStatusFailed {
		return nil, fmt.Errorf("this delivery plan cannot be restarted")
	}
	var page model.CommandBarPageContext
	if err := json.Unmarshal(plan.PageContext, &page); err != nil {
		return nil, err
	}
	if page.EntityType != "epic" {
		return nil, fmt.Errorf("paused-step restart is available only for epic deliveries")
	}
	var steps []model.CommandBarPlanStep
	if err := json.Unmarshal(plan.Steps, &steps); err != nil {
		return nil, err
	}
	if req.StepIndex < 0 || req.StepIndex >= len(steps) || isCommandBarOrchestrationStep(steps[req.StepIndex]) {
		return nil, fmt.Errorf("select an AI delivery step")
	}
	ids := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	runID := strings.TrimSpace(ids[req.StepIndex])
	if runID == "" || runID != strings.TrimSpace(req.ExpectedRunID) {
		return nil, fmt.Errorf("delivery step changed; reload before restarting")
	}
	run, err := s.agentService.runRepo.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("delivery run not found")
	}
	if run.Status != model.AgentRunStatusPaused && run.Status != model.AgentRunStatusCancelled && run.Status != model.AgentRunStatusFailed {
		return nil, fmt.Errorf("only a paused, cancelled, or failed delivery run can be restarted")
	}
	params, err := s.WithEpicDeliveryProfile(ctx, workspaceID, actorID, req.AIProfileID, dispatchPlanParams{})
	if err != nil {
		return nil, err
	}
	if run.Status == model.AgentRunStatusPaused {
		run, err = s.agentService.CancelRun(ctx, workspaceID, run.ID, actorID)
		if err != nil {
			return nil, err
		}
		if run.Status != model.AgentRunStatusCancelled {
			return nil, fmt.Errorf("cancellation is in progress; retry when the run is cancelled")
		}
	}
	runIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		runIDs = append(runIDs, id)
	}
	runs, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, runIDs)
	if err != nil {
		return nil, err
	}
	runsByID := make(map[string]model.AgentRun, len(runs))
	for _, item := range runs {
		runsByID[item.ID] = item
	}
	if !commandBarStepDependenciesSatisfied(steps[req.StepIndex], ids, runsByID) {
		return nil, fmt.Errorf("delivery step prerequisites are not complete")
	}
	var binding []byte
	if params.profileBinding != nil {
		binding, err = json.Marshal(params.profileBinding)
		if err != nil {
			return nil, err
		}
	}
	if err := s.planRepo.BeginPausedStepRestart(ctx, workspaceID, plan.ID, req.StepIndex, runID, binding); err != nil {
		return nil, err
	}
	parentRunID := commandBarParentRunIDForStep(steps, req.StepIndex, ids, runsByID)
	newRun, err := s.agentService.startCommandBarPlanStep(ctx, workspaceID, actorID, plan.Prompt, page, steps, req.StepIndex, plan.ID, parentRunID)
	if err != nil {
		_ = s.planRepo.RollbackPausedStepRestart(ctx, workspaceID, plan.ID, req.StepIndex, runID, plan.ProfileBinding)
		return nil, err
	}
	if err := s.planRepo.SetStepRun(ctx, workspaceID, plan.ID, req.StepIndex, newRun.ID); err != nil {
		_, _ = s.agentService.CancelRun(ctx, workspaceID, newRun.ID, actorID)
		_ = s.planRepo.RollbackPausedStepRestart(ctx, workspaceID, plan.ID, req.StepIndex, runID, plan.ProfileBinding)
		return nil, err
	}
	updated, err := s.GetWorkspacePlan(ctx, workspaceID, plan.ID)
	if err != nil {
		return nil, err
	}
	return &model.CommandBarRetryPlanResponse{Plan: updated.Plan, Run: newRun, Runs: []model.AgentRun{*newRun}}, nil
}
