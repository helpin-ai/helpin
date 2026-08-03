package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Command-bar plan dispatch: normalization, validation, and plan-kind execution branching.

func (s *CommandBarService) DispatchPlan(ctx context.Context, workspaceID, actorID string, req model.CommandBarDispatchRequest) (*model.CommandBarDispatchResponse, error) {
	return s.dispatchPlanCore(ctx, workspaceID, actorID, req, dispatchPlanParams{})
}

// dispatchPlanParams carries dock-chat linkage for plans launched by a chat's
// orchestrator agent (via the agents.* command tools). Empty for HTTP
// dispatches.
type dispatchPlanParams struct {
	parentChatRunID       *string
	dockChatID            *string
	supportConversationID *string
}

func (s *CommandBarService) dispatchPlanCore(ctx context.Context, workspaceID, actorID string, req model.CommandBarDispatchRequest, params dispatchPlanParams) (*model.CommandBarDispatchResponse, error) {
	if s == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar service is not configured")
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	pageContext := normalizeCommandBarPageContext(req.PageContext, workspaceID)
	if len(req.Steps) == 0 {
		return nil, fmt.Errorf("at least one plan step is required")
	}
	if len(req.Steps) > maxCommandBarPlanSteps {
		return nil, fmt.Errorf("command bar plans are limited to %d steps", maxCommandBarPlanSteps)
	}

	steps := normalizeCommandBarPlanSteps(req.Steps, pageContext)
	for i, step := range steps {
		if err := validateCommandBarSupportedTarget(step.Target.EntityType); err != nil {
			return nil, err
		}
		if strings.TrimSpace(step.AgentID) == "" {
			return nil, fmt.Errorf("agent_id is required for step %d", i+1)
		}
		if step.PlanKind == model.CommandBarPlanKindOneShotCommand && len(step.AllowedTools) == 0 {
			return nil, fmt.Errorf("sub-agent step %d requires at least one enabled tool", i+1)
		}
	}
	if err := s.validateDispatchSteps(ctx, workspaceID, steps); err != nil {
		return nil, err
	}
	planID := uuid.NewString()
	if s.planRepo != nil {
		record, err := newCommandBarPlanRecord(workspaceID, actorID, planID, text, pageContext, steps)
		if err != nil {
			return nil, err
		}
		record.ParentChatRunID = params.parentChatRunID
		record.DockChatID = params.dockChatID
		record.SupportConversationID = params.supportConversationID
		if err := s.planRepo.Create(ctx, record); err != nil {
			return nil, err
		}
	}
	planKind := commandBarPlanKindForSteps(steps)
	if planKind == model.CommandBarPlanKindTaskPipeline || planKind == model.CommandBarPlanKindDAG {
		if _, err := s.agentService.startReadyCommandBarPlanSteps(ctx, CommandBarPlanInput{
			WorkspaceID: workspaceID,
			ActorID:     actorID,
			PlanID:      planID,
			Prompt:      text,
			PageContext: pageContext,
			Steps:       steps,
		}); err != nil {
			if s.planRepo != nil {
				_ = s.planRepo.MarkFailed(ctx, workspaceID, planID, err.Error())
			}
			return nil, err
		}
		return &model.CommandBarDispatchResponse{
			PlanID:   planID,
			Steps:    steps,
			RunCount: len(steps),
			Runs:     []model.AgentRun{},
		}, nil
	}
	if planKind == model.CommandBarPlanKindFanOut {
		runs := make([]model.AgentRun, 0, len(steps))
		for index := range steps {
			run, err := s.agentService.startCommandBarPlanStep(ctx, workspaceID, actorID, text, pageContext, steps, index, planID, nil)
			if err != nil {
				if s.planRepo != nil {
					_ = s.planRepo.MarkFailed(ctx, workspaceID, planID, err.Error())
				}
				return nil, err
			}
			if s.planRepo != nil {
				_ = s.planRepo.SetStepRun(ctx, workspaceID, planID, index, run.ID)
			}
			runs = append(runs, *run)
		}
		return &model.CommandBarDispatchResponse{
			PlanID:   planID,
			Steps:    steps,
			RunCount: len(steps),
			Runs:     runs,
		}, nil
	}
	run, err := s.agentService.startCommandBarPlanStep(ctx, workspaceID, actorID, text, pageContext, steps, 0, planID, nil)
	if err != nil {
		if s.planRepo != nil {
			_ = s.planRepo.MarkFailed(ctx, workspaceID, planID, err.Error())
		}
		return nil, err
	}
	if s.planRepo != nil {
		_ = s.planRepo.SetStepRun(ctx, workspaceID, planID, 0, run.ID)
	}
	return &model.CommandBarDispatchResponse{
		PlanID:   planID,
		Steps:    steps,
		RunCount: len(steps),
		Runs:     []model.AgentRun{*run},
	}, nil
}

func (s *CommandBarService) validateDispatchSteps(ctx context.Context, workspaceID string, steps []model.CommandBarPlanStep) error {
	agents, err := s.agentService.ListAgents(ctx, workspaceID)
	if err != nil {
		return err
	}
	byID := make(map[string]model.Agent, len(agents))
	for _, agent := range agents {
		byID[agent.ID] = agent
	}
	if err := validateCommandBarStepDependencies(steps, maxCommandBarDAGInitialFanOut); err != nil {
		return err
	}
	for i, step := range steps {
		agent, ok := byID[step.AgentID]
		if !ok {
			return fmt.Errorf("agent not found for step %d", i+1)
		}
		if isCommandBarOrchestrationStep(step) {
			continue
		}
		if err := validateCommandBarStepTargetForAgent(step, &agent, i); err != nil {
			return err
		}
		if err := validateRunAllowedTools(step.AllowedTools, &agent); err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}
		if normalizePresetKey(agent.PresetKey) == model.AgentPresetCommandAgent {
			if step.PlanKind != model.CommandBarPlanKindOneShotCommand && step.PlanKind != model.CommandBarPlanKindDAG {
				return fmt.Errorf("sub-agent step %d must be dispatched as a delegated command or DAG step", i+1)
			}
			if len(step.AllowedTools) == 0 {
				return fmt.Errorf("sub-agent step %d requires a limited tool set", i+1)
			}
		}
	}
	return nil
}

func isCommandBarOrchestrationStep(step model.CommandBarPlanStep) bool {
	switch strings.TrimSpace(step.StepType) {
	case model.CommandBarStepTypeEnsureEpicBranch,
		model.CommandBarStepTypeMergeTaskToEpic,
		model.CommandBarStepTypeResolveMergeConflict,
		model.CommandBarStepTypeOpenEpicPullRequest:
		return true
	default:
		return false
	}
}

func validateCommandBarStepTargetForAgent(step model.CommandBarPlanStep, agent *model.Agent, stepIndex int) error {
	targetType := normalizeCommandBarTargetType(step.Target.EntityType)
	if err := validateCommandBarSupportedTarget(targetType); err != nil {
		return err
	}
	allowedTargets := parseJSONStringSlice(agent.AllowedTargets)
	if requiredTargets := commandBarRequiredTargetTypesForStep(step, *agent); len(requiredTargets) > 0 {
		allowedTargets = commandBarIntersectTargetTypes(allowedTargets, requiredTargets)
		if len(allowedTargets) == 0 {
			return fmt.Errorf("step %d selected tools are outside agent %s target allowlist", stepIndex+1, strings.TrimSpace(agent.Name))
		}
	}
	if len(allowedTargets) == 0 {
		return nil
	}
	allowedTargets = normalizeCommandBarTargetTypes(allowedTargets)
	if !slices.Contains(allowedTargets, targetType) {
		return fmt.Errorf("step %d target %q is outside agent %s target allowlist", stepIndex+1, targetType, strings.TrimSpace(agent.Name))
	}
	return nil
}

func validateCommandBarStepDependencies(steps []model.CommandBarPlanStep, maxFanOut int) error {
	for i, step := range steps {
		for _, dep := range step.DependsOnStepIndexes {
			if dep < 0 || dep >= len(steps) {
				return fmt.Errorf("step %d dependency index %d is out of range", i+1, dep)
			}
			if dep == i {
				return fmt.Errorf("step %d cannot depend on itself", i+1)
			}
		}
	}
	if commandBarHasDependencyCycle(steps) {
		return fmt.Errorf("command bar plan dependencies contain a cycle")
	}
	if maxFanOut > 0 && commandBarPlanKindForSteps(steps) == model.CommandBarPlanKindDAG {
		ready := 0
		for _, step := range steps {
			if len(step.DependsOnStepIndexes) == 0 {
				ready++
			}
		}
		if ready > maxFanOut {
			return fmt.Errorf("DAG plans are limited to %d initially runnable steps", maxFanOut)
		}
	}
	return nil
}

func commandBarHasDependencyCycle(steps []model.CommandBarPlanStep) bool {
	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := make([]int, len(steps))
	var visit func(int) bool
	visit = func(index int) bool {
		if state[index] == visiting {
			return true
		}
		if state[index] == visited {
			return false
		}
		state[index] = visiting
		for _, dep := range steps[index].DependsOnStepIndexes {
			if dep >= 0 && dep < len(steps) && visit(dep) {
				return true
			}
		}
		state[index] = visited
		return false
	}
	for i := range steps {
		if visit(i) {
			return true
		}
	}
	return false
}
