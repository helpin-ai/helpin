package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// In-process command-bar step scheduling on AgentService: plan advancement, DAG/fan-out scheduling, and git orchestration steps.

func (s *AgentService) startCommandBarPlanStep(ctx context.Context, workspaceID, actorID, text string, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep, stepIndex int, planID string, parentRunID *string) (*model.AgentRun, error) {
	if s == nil {
		return nil, fmt.Errorf("agent service is not configured")
	}
	if stepIndex < 0 || stepIndex >= len(steps) {
		return nil, fmt.Errorf("command bar step index %d is out of range", stepIndex)
	}
	step := steps[stepIndex]
	target := normalizeCommandBarPageContext(step.Target, workspaceID)
	if target.EntityType == "" || target.EntityID == "" {
		target = pageContext
	}
	if err := validateCommandBarSupportedTarget(target.EntityType); err != nil {
		return nil, err
	}
	agentID := strings.TrimSpace(step.AgentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required for step %d", stepIndex+1)
	}

	triggerContext, err := buildCommandBarTriggerContext(text, pageContext, steps, stepIndex, planID)
	if err != nil {
		return nil, err
	}
	additionalContext := commandBarAdditionalContext(step.Instructions, target, stepIndex, len(steps))
	event := (*model.AgentRunEventContext)(nil)
	if parentRunID != nil && strings.TrimSpace(*parentRunID) != "" {
		reason := "command_bar_next_step"
		event = &model.AgentRunEventContext{
			RunID:  parentRunID,
			Reason: &reason,
		}
	}
	actor := (*string)(nil)
	if strings.TrimSpace(actorID) != "" {
		actor = &actorID
	}
	// allowActiveParentRun: a next step chained from its parent run must never
	// conflict with that parent. For delegated (agent-runtime) children the
	// parent's terminal status may not be persisted yet when the finalizer
	// advances the plan, so the parent can still look active in the database.
	return s.startTargetRunWithOptions(ctx, workspaceID, target.EntityType, target.EntityID, model.StartAgentRunRequest{
		AgentID:           agentID,
		AdditionalContext: &additionalContext,
		AllowedTools:      step.AllowedTools,
	}, actor, triggerContext, event, parentRunID, startTargetRunOptions{allowActiveParentRun: parentRunID != nil})
}

// AdvanceCommandBarPlanAfterRun advances the command-bar plan that owns the
// given terminal run, loading the run's persisted state. Agent Runtime
// finalization calls this after the terminal status is already persisted.
func (s *AgentService) AdvanceCommandBarPlanAfterRun(ctx context.Context, completedRunID string) (*model.AgentRun, error) {
	if s == nil || s.runRepo == nil {
		return nil, nil
	}
	completedRunID = strings.TrimSpace(completedRunID)
	if completedRunID == "" {
		return nil, nil
	}
	run, err := s.runRepo.GetByIDAny(ctx, completedRunID)
	if err != nil || run == nil {
		return nil, err
	}
	return s.advanceCommandBarPlanForRun(ctx, run)
}

// AdvanceCommandBarPlanForDelegatedRun advances the command-bar plan for a
// delegated agent-runtime run whose terminal status is only known in memory:
// the projection dispatches finalizers before persisting the terminal row, so
// re-loading the run here would observe a stale (still active) status.
func (s *AgentService) AdvanceCommandBarPlanForDelegatedRun(ctx context.Context, run *model.AgentRun) (*model.AgentRun, error) {
	if s == nil || s.runRepo == nil || run == nil {
		return nil, nil
	}
	return s.advanceCommandBarPlanForRun(ctx, run)
}

// advanceCommandBarPlanForRun advances the owning command-bar plan using the
// caller-provided run as the authoritative view of the terminal transition.
func (s *AgentService) advanceCommandBarPlanForRun(ctx context.Context, run *model.AgentRun) (*model.AgentRun, error) {
	payload, ok := commandBarRunPayload(run)
	if !ok {
		return nil, nil
	}
	planKind := commandBarPlanKindForSteps(payload.Steps)
	if planKind == model.CommandBarPlanKindTaskPipeline || planKind == model.CommandBarPlanKindDAG {
		if s.commandBarPlanRepo != nil && payload.PlanID != "" {
			if _, err := s.startReadyCommandBarPlanSteps(ctx, CommandBarPlanInput{
				WorkspaceID: run.WorkspaceID,
				ActorID:     derefString(run.TriggeredByUserID),
				PlanID:      payload.PlanID,
				Prompt:      payload.Prompt,
				PageContext: payload.PageContext,
				Steps:       payload.Steps,
			}); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	if planKind == model.CommandBarPlanKindFanOut {
		return s.advanceFanOutCommandBarPlan(ctx, run, payload)
	}
	if run.Status == model.AgentRunStatusFailed {
		if s.commandBarPlanRepo != nil && payload.PlanID != "" {
			_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, payload.PlanID, commandBarTerminalRunMessage(run, payload, "failed"))
		}
		return nil, nil
	}
	if run.Status == model.AgentRunStatusCancelled {
		if s.commandBarPlanRepo != nil && payload.PlanID != "" {
			_ = s.commandBarPlanRepo.MarkCancelled(ctx, run.WorkspaceID, payload.PlanID)
		}
		return nil, nil
	}
	if run.Status != model.AgentRunStatusCompleted {
		return nil, nil
	}
	if payload.RunCount <= 1 || payload.StepIndex+1 >= len(payload.Steps) {
		if s.commandBarPlanRepo != nil && payload.PlanID != "" && payload.StepIndex+1 >= len(payload.Steps) {
			_ = s.commandBarPlanRepo.MarkCompleted(ctx, run.WorkspaceID, payload.PlanID)
		}
		return nil, nil
	}
	if s.commandBarPlanRepo != nil && payload.PlanID != "" {
		plan, err := s.commandBarPlanRepo.GetByID(ctx, run.WorkspaceID, payload.PlanID)
		if err != nil {
			return nil, err
		}
		if plan != nil && plan.Status == model.CommandBarPlanStatusCancelled {
			return nil, nil
		}
	}
	if existing, err := s.runRepo.FindByParentRunID(ctx, run.WorkspaceID, run.ID); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	actorID := derefString(run.TriggeredByUserID)
	nextIndex := payload.StepIndex + 1
	planID := firstNonEmptyString(payload.PlanID, uuid.NewString())
	nextRun, err := s.startCommandBarPlanStep(ctx, run.WorkspaceID, actorID, payload.Prompt, payload.PageContext, payload.Steps, nextIndex, planID, &run.ID)
	if err != nil {
		if existing, findErr := s.runRepo.FindByParentRunID(ctx, run.WorkspaceID, run.ID); findErr == nil && existing != nil {
			if s.commandBarPlanRepo != nil && planID != "" {
				_ = s.commandBarPlanRepo.SetStepRun(ctx, run.WorkspaceID, planID, nextIndex, existing.ID)
			}
			return existing, nil
		}
		if s.commandBarPlanRepo != nil && planID != "" {
			_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, planID, err.Error())
		}
		return nil, err
	}
	if s.commandBarPlanRepo != nil && planID != "" && nextRun != nil {
		_ = s.commandBarPlanRepo.SetStepRun(ctx, run.WorkspaceID, planID, nextIndex, nextRun.ID)
	}
	return nextRun, nil
}

func (s *AgentService) startReadyCommandBarPlanSteps(ctx context.Context, input CommandBarPlanInput) (*CommandBarPlanProgress, error) {
	progress := &CommandBarPlanProgress{Status: model.CommandBarPlanStatusRunning}
	if s == nil || s.commandBarPlanRepo == nil || s.runRepo == nil {
		return &CommandBarPlanProgress{Terminal: true, Status: "not_configured"}, nil
	}
	plan, err := s.commandBarPlanRepo.GetByID(ctx, input.WorkspaceID, input.PlanID)
	if err != nil || plan == nil {
		return progress, err
	}
	if plan.Status == model.CommandBarPlanStatusCancelled || plan.Status == model.CommandBarPlanStatusFailed || plan.Status == model.CommandBarPlanStatusCompleted {
		return &CommandBarPlanProgress{Terminal: true, Status: plan.Status}, nil
	}

	steps := input.Steps
	if len(steps) == 0 {
		_ = json.Unmarshal(plan.Steps, &steps)
	}
	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	runIDs := make([]string, 0, len(runIDsByStep))
	for _, runID := range runIDsByStep {
		runIDs = append(runIDs, runID)
	}
	runs, err := s.runRepo.ListByIDs(ctx, input.WorkspaceID, runIDs)
	if err != nil {
		return nil, err
	}
	runsByID := make(map[string]model.AgentRun, len(runs))
	activeCount := 0
	for _, run := range runs {
		if stepIndex := commandBarStepIndexForRun(runIDsByStep, run.ID); stepIndex >= 0 && stepIndex < len(steps) && isCommandBarOrchestrationStep(steps[stepIndex]) && model.IsAgentRunActiveStatus(run.Status) {
			updated, err := s.advanceRunningCommandBarOrchestrationRun(ctx, input, steps, stepIndex, &run)
			if err != nil {
				_ = s.commandBarPlanRepo.MarkFailed(ctx, input.WorkspaceID, input.PlanID, err.Error())
				return nil, err
			}
			if updated != nil {
				run = *updated
			}
		}
		runsByID[run.ID] = run
		if model.IsAgentRunActiveStatus(run.Status) {
			activeCount++
		}
		if run.Status == model.AgentRunStatusFailed || run.Status == model.AgentRunStatusCancelled {
			payload := commandBarTriggerContextPayload{PlanID: input.PlanID, Steps: steps, StepIndex: commandBarStepIndexForRun(runIDsByStep, run.ID)}
			_ = s.commandBarPlanRepo.MarkFailed(ctx, input.WorkspaceID, input.PlanID, commandBarTerminalRunMessage(&run, payload, string(run.Status)))
			return &CommandBarPlanProgress{Terminal: true, Status: model.CommandBarPlanStatusFailed}, nil
		}
	}

	allStarted := len(steps) > 0 && len(runIDsByStep) >= len(steps)
	allCompleted := allStarted
	for _, runID := range runIDsByStep {
		run, ok := runsByID[runID]
		if !ok || run.Status != model.AgentRunStatusCompleted {
			allCompleted = false
			break
		}
	}
	if allCompleted {
		_ = s.commandBarPlanRepo.MarkCompleted(ctx, input.WorkspaceID, input.PlanID)
		return &CommandBarPlanProgress{Terminal: true, Status: model.CommandBarPlanStatusCompleted}, nil
	}

	started := 0
	observedConcurrentProgress := false
	for index, step := range steps {
		if strings.TrimSpace(runIDsByStep[index]) != "" {
			continue
		}
		if !commandBarStepDependenciesSatisfied(step, runIDsByStep, runsByID) {
			continue
		}
		latestPlan, err := s.commandBarPlanRepo.GetByID(ctx, input.WorkspaceID, input.PlanID)
		if err != nil {
			return nil, err
		}
		if latestPlan == nil {
			return progress, nil
		}
		if latestPlan.Status == model.CommandBarPlanStatusCancelled || latestPlan.Status == model.CommandBarPlanStatusFailed || latestPlan.Status == model.CommandBarPlanStatusCompleted {
			return &CommandBarPlanProgress{Terminal: true, Status: latestPlan.Status}, nil
		}
		latestRunIDsByStep := decodeCommandBarPlanRunIDs(latestPlan.RunIDsByStep)
		if existingRunID := strings.TrimSpace(latestRunIDsByStep[index]); existingRunID != "" {
			runIDsByStep[index] = existingRunID
			observedConcurrentProgress = true
			if existing, err := s.runRepo.GetByID(ctx, input.WorkspaceID, existingRunID); err == nil && existing != nil {
				runsByID[existing.ID] = *existing
				if model.IsAgentRunActiveStatus(existing.Status) {
					activeCount++
				}
			}
			continue
		}
		parentRunID := commandBarParentRunIDForStep(steps, index, runIDsByStep, runsByID)
		var run *model.AgentRun
		if isCommandBarOrchestrationStep(step) {
			run, err = s.startCommandBarOrchestrationStep(ctx, input, steps, index, parentRunID)
		} else {
			run, err = s.startCommandBarPlanStep(ctx, input.WorkspaceID, input.ActorID, input.Prompt, input.PageContext, steps, index, input.PlanID, parentRunID)
		}
		if err != nil {
			if existing := s.commandBarExistingRunForStep(ctx, input.WorkspaceID, parentRunID, input.PlanID, steps, index); existing != nil {
				run = existing
			} else {
				_ = s.commandBarPlanRepo.MarkFailed(ctx, input.WorkspaceID, input.PlanID, err.Error())
				return nil, err
			}
		}
		runIDsByStep[index] = run.ID
		started++
		progress.Started = append(progress.Started, run.ID)
		if err := s.commandBarPlanRepo.SetStepRun(ctx, input.WorkspaceID, input.PlanID, index, run.ID); err != nil {
			return nil, err
		}
	}
	if started == 0 && activeCount == 0 && !observedConcurrentProgress {
		_ = s.commandBarPlanRepo.MarkFailed(ctx, input.WorkspaceID, input.PlanID, "Command-bar plan has no runnable steps; check task dependencies for a cycle or missing completed prerequisite.")
		return &CommandBarPlanProgress{Terminal: true, Status: model.CommandBarPlanStatusFailed}, nil
	}
	return progress, nil
}

func (s *AgentService) commandBarExistingChildRunForParent(ctx context.Context, workspaceID string, parentRunID *string) *model.AgentRun {
	if s == nil || s.runRepo == nil || parentRunID == nil || strings.TrimSpace(*parentRunID) == "" {
		return nil
	}
	existing, err := s.runRepo.FindByParentRunID(ctx, workspaceID, strings.TrimSpace(*parentRunID))
	if err != nil {
		return nil
	}
	return existing
}

func (s *AgentService) commandBarExistingRunForStep(ctx context.Context, workspaceID string, parentRunID *string, planID string, steps []model.CommandBarPlanStep, stepIndex int) *model.AgentRun {
	existing := s.commandBarExistingChildRunForParent(ctx, workspaceID, parentRunID)
	if existing == nil || !commandBarRunMatchesStep(existing, planID, steps, stepIndex) {
		return nil
	}
	return existing
}

func commandBarRunMatchesStep(run *model.AgentRun, planID string, steps []model.CommandBarPlanStep, stepIndex int) bool {
	if run == nil || stepIndex < 0 || stepIndex >= len(steps) {
		return false
	}
	step := steps[stepIndex]
	if strings.TrimSpace(run.AgentID) != strings.TrimSpace(step.AgentID) {
		return false
	}
	if normalizeCommandBarTargetType(run.TargetType) != normalizeCommandBarTargetType(step.Target.EntityType) || strings.TrimSpace(run.TargetID) != strings.TrimSpace(step.Target.EntityID) {
		return false
	}
	payload, ok := commandBarRunPayload(run)
	if !ok {
		return false
	}
	return strings.TrimSpace(payload.PlanID) == strings.TrimSpace(planID) && payload.StepIndex == stepIndex
}

func commandBarStepDependenciesSatisfied(step model.CommandBarPlanStep, runIDsByStep map[int]string, runsByID map[string]model.AgentRun) bool {
	for _, dep := range step.DependsOnStepIndexes {
		runID := strings.TrimSpace(runIDsByStep[dep])
		if runID == "" {
			return false
		}
		run, ok := runsByID[runID]
		if !ok || run.Status != model.AgentRunStatusCompleted {
			return false
		}
	}
	return true
}

func commandBarParentRunIDForStep(steps []model.CommandBarPlanStep, stepIndex int, runIDsByStep map[int]string, runsByID map[string]model.AgentRun) *string {
	if stepIndex < 0 || stepIndex >= len(steps) {
		return nil
	}
	step := steps[stepIndex]
	if len(step.DependsOnStepIndexes) != 1 {
		return nil
	}
	dependencyIndex := step.DependsOnStepIndexes[0]
	if commandBarDependencyConsumerCount(steps, dependencyIndex) != 1 {
		return nil
	}
	runID := strings.TrimSpace(runIDsByStep[dependencyIndex])
	if runID == "" {
		return nil
	}
	run, ok := runsByID[runID]
	if !ok || run.Status != model.AgentRunStatusCompleted {
		return nil
	}
	id := run.ID
	return &id
}

func commandBarDependencyConsumerCount(steps []model.CommandBarPlanStep, dependencyIndex int) int {
	count := 0
	for _, step := range steps {
		for _, dep := range step.DependsOnStepIndexes {
			if dep == dependencyIndex {
				count++
				break
			}
		}
	}
	return count
}

type commandBarOrchestrationOutput struct {
	Type                string `json:"type,omitempty"`
	Status              string `json:"status,omitempty"`
	Message             string `json:"message,omitempty"`
	EpicBranch          string `json:"epic_branch,omitempty"`
	BaseBranch          string `json:"base_branch,omitempty"`
	TaskBranch          string `json:"task_branch,omitempty"`
	ConflictRunID       string `json:"conflict_run_id,omitempty"`
	ConflictAttempt     int    `json:"conflict_attempt,omitempty"`
	FinalPullRequestURL string `json:"final_pull_request_url,omitempty"`
}

func (s *AgentService) startCommandBarOrchestrationStep(ctx context.Context, input CommandBarPlanInput, steps []model.CommandBarPlanStep, stepIndex int, parentRunID *string) (*model.AgentRun, error) {
	step := steps[stepIndex]
	now := time.Now()
	triggerContext, err := buildCommandBarTriggerContext(input.Prompt, input.PageContext, steps, stepIndex, input.PlanID)
	if err != nil {
		return nil, err
	}
	runInput, err := json.Marshal(model.AgentRunInputPayload{
		Trigger: triggerContext,
		Target: &model.AgentRunTargetContext{
			TargetType: step.Target.EntityType,
			TargetID:   step.Target.EntityID,
		},
		AdditionalContext: step.Instructions,
	})
	if err != nil {
		return nil, err
	}
	run := &model.AgentRun{
		ID:                uuid.NewString(),
		WorkspaceID:       input.WorkspaceID,
		AgentID:           step.AgentID,
		TargetType:        firstNonEmptyString(step.Target.EntityType, input.PageContext.EntityType),
		TargetID:          firstNonEmptyString(step.Target.EntityID, input.PageContext.EntityID),
		RuntimeKind:       "internal",
		InvocationMode:    "autonomous",
		ParentRunID:       parentRunID,
		ApprovalState:     "not_required",
		PauseReason:       model.AgentRunPauseReasonNone,
		TriggeredByUserID: stringPtrIfNotEmpty(input.ActorID),
		Status:            model.AgentRunStatusRunning,
		ExecutionStage:    strPtr(step.StepType),
		StartedAt:         &now,
		LastHeartbeatAt:   &now,
		Input:             runInput,
		OutputSummary:     json.RawMessage(`{}`),
	}
	if step.Target.EntityType == "task" {
		taskID := step.Target.EntityID
		run.TaskID = &taskID
	}
	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, err
	}
	updated, err := s.executeCommandBarOrchestrationRun(ctx, input, steps, stepIndex, run)
	if err != nil {
		run.Status = model.AgentRunStatusFailed
		run.ErrorMessage = strPtr(err.Error())
		completedAt := time.Now()
		run.CompletedAt = &completedAt
		run.LastHeartbeatAt = &completedAt
		_ = s.runRepo.Update(ctx, run)
		s.runRepo.Notify(ctx, run)
		return nil, err
	}
	s.runRepo.Notify(ctx, updated)
	return updated, nil
}

func (s *AgentService) advanceRunningCommandBarOrchestrationRun(ctx context.Context, input CommandBarPlanInput, steps []model.CommandBarPlanStep, stepIndex int, run *model.AgentRun) (*model.AgentRun, error) {
	if run == nil || run.Status != model.AgentRunStatusRunning {
		return run, nil
	}
	var output commandBarOrchestrationOutput
	_ = json.Unmarshal(run.OutputSummary, &output)
	if output.ConflictRunID == "" {
		return run, nil
	}
	child, err := s.runRepo.GetByIDAny(ctx, output.ConflictRunID)
	if err != nil || child == nil {
		return run, err
	}
	if model.IsAgentRunActiveStatus(child.Status) {
		return run, nil
	}
	if child.Status != model.AgentRunStatusCompleted {
		run.Status = model.AgentRunStatusFailed
		run.ErrorMessage = strPtr("merge conflict resolution run did not complete successfully")
		now := time.Now()
		run.CompletedAt = &now
		run.LastHeartbeatAt = &now
		if err := s.runRepo.Update(ctx, run); err != nil {
			return nil, err
		}
		s.runRepo.Notify(ctx, run)
		return run, nil
	}
	return s.executeCommandBarOrchestrationRun(ctx, input, steps, stepIndex, run)
}

func (s *AgentService) executeCommandBarOrchestrationRun(ctx context.Context, input CommandBarPlanInput, steps []model.CommandBarPlanStep, stepIndex int, run *model.AgentRun) (*model.AgentRun, error) {
	if s.gitService == nil {
		return nil, fmt.Errorf("git service is not configured")
	}
	step := steps[stepIndex]
	switch step.StepType {
	case model.CommandBarStepTypeEnsureEpicBranch:
		target, err := s.gitService.EnsureEpicBranch(ctx, input.WorkspaceID, input.PageContext.EntityID, input.ActorID, run.ID)
		if err != nil {
			return nil, err
		}
		preparedTaskIDs := make(map[string]bool)
		for _, candidate := range steps {
			if candidate.Target.EntityType == "task" && candidate.Target.EntityID != "" {
				if preparedTaskIDs[candidate.Target.EntityID] {
					continue
				}
				preparedTaskIDs[candidate.Target.EntityID] = true
				if _, err := s.gitService.PrepareTaskForEpicBranch(ctx, input.WorkspaceID, candidate.Target.EntityID, input.PageContext.EntityID, input.ActorID); err != nil {
					return nil, err
				}
			}
		}
		return s.completeCommandBarOrchestrationRun(ctx, run, commandBarOrchestrationOutput{
			Type:       step.StepType,
			Status:     "completed",
			Message:    "Epic branch is ready and task delivery targets were based on it.",
			EpicBranch: derefString(target.EpicBranch),
			BaseBranch: derefString(target.BaseBranch),
		})
	case model.CommandBarStepTypeMergeTaskToEpic:
		target, err := s.gitService.MergeTaskBranchIntoEpic(ctx, input.WorkspaceID, step.Target.EntityID, input.PageContext.EntityID, run.ID)
		if err != nil {
			if commandBarIsMergeConflict(err) {
				return s.startCommandBarMergeConflictResolution(ctx, input, steps, stepIndex, run, err)
			}
			return nil, err
		}
		return s.completeCommandBarOrchestrationRun(ctx, run, commandBarOrchestrationOutput{
			Type:       step.StepType,
			Status:     "completed",
			Message:    "Task branch merged into epic branch.",
			EpicBranch: derefString(target.EpicBranch),
			BaseBranch: derefString(target.BaseBranch),
		})
	case model.CommandBarStepTypeOpenEpicPullRequest:
		target, err := s.gitService.OpenEpicFinalPullRequest(ctx, input.WorkspaceID, input.PageContext.EntityID, run.ID)
		if err != nil {
			return nil, err
		}
		return s.completeCommandBarOrchestrationRun(ctx, run, commandBarOrchestrationOutput{
			Type:                step.StepType,
			Status:              "completed",
			Message:             "Final epic pull request is open.",
			EpicBranch:          derefString(target.EpicBranch),
			BaseBranch:          derefString(target.BaseBranch),
			FinalPullRequestURL: derefString(target.FinalPRURL),
		})
	default:
		return nil, fmt.Errorf("unsupported command-bar orchestration step %q", step.StepType)
	}
}

func (s *AgentService) completeCommandBarOrchestrationRun(ctx context.Context, run *model.AgentRun, output commandBarOrchestrationOutput) (*model.AgentRun, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	run.Status = model.AgentRunStatusCompleted
	run.PauseReason = model.AgentRunPauseReasonNone
	run.ExecutionStage = strPtr("completed")
	run.OutputSummary = raw
	run.CompletedAt = &now
	run.LastHeartbeatAt = &now
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	s.runRepo.Notify(ctx, run)
	return run, nil
}

func commandBarIsMergeConflict(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "merge conflict")
}

func (s *AgentService) startCommandBarMergeConflictResolution(ctx context.Context, input CommandBarPlanInput, steps []model.CommandBarPlanStep, stepIndex int, run *model.AgentRun, mergeErr error) (*model.AgentRun, error) {
	var existing commandBarOrchestrationOutput
	_ = json.Unmarshal(run.OutputSummary, &existing)
	if existing.ConflictAttempt >= 1 && existing.ConflictRunID != "" {
		return nil, fmt.Errorf("task branch still conflicts with epic branch after Forge conflict resolution: %w", mergeErr)
	}
	step := steps[stepIndex]
	forgeStep, ok := commandBarForgeStepForTarget(steps, step.Target.EntityID)
	if !ok {
		return nil, fmt.Errorf("merge conflict requires Forge conflict resolution, but no Forge step was found for task %s", step.Target.DisplayTitle)
	}
	triggerContext, err := buildCommandBarTriggerContext(input.Prompt, input.PageContext, steps, stepIndex, input.PlanID)
	if err != nil {
		return nil, err
	}
	additionalContext := strings.Join([]string{
		"Resolve the merge conflict blocking this epic integration.",
		"Your task branch could not be merged into the epic integration branch.",
		"Update the task branch so it merges cleanly into the epic branch, then push your changes.",
		"Do not merge the task branch into the epic branch yourself; Helpin will retry the backend merge after this run completes.",
		"Merge error: " + mergeErr.Error(),
	}, "\n")
	reason := "command_bar_merge_conflict"
	parentRunID := run.ID
	event := &model.AgentRunEventContext{
		RunID:  &run.ID,
		Reason: &reason,
	}
	actor := stringPtrIfNotEmpty(input.ActorID)
	child, err := s.startTargetRunWithOptions(ctx, input.WorkspaceID, step.Target.EntityType, step.Target.EntityID, model.StartAgentRunRequest{
		AgentID:           forgeStep.AgentID,
		AdditionalContext: &additionalContext,
		AllowedTools:      forgeStep.AllowedTools,
	}, actor, triggerContext, event, &parentRunID, startTargetRunOptions{allowActiveParentRun: true})
	if err != nil {
		return nil, err
	}
	output := commandBarOrchestrationOutput{
		Type:            step.StepType,
		Status:          "resolving_conflict",
		Message:         "Forge is resolving a merge conflict before Helpin retries the task-to-epic merge.",
		ConflictRunID:   child.ID,
		ConflictAttempt: existing.ConflictAttempt + 1,
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	run.Status = model.AgentRunStatusRunning
	run.ExecutionStage = strPtr("resolving_conflict")
	run.OutputSummary = raw
	run.LastHeartbeatAt = &now
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	s.runRepo.Notify(ctx, run)
	return run, nil
}

func commandBarForgeStepForTarget(steps []model.CommandBarPlanStep, taskID string) (model.CommandBarPlanStep, bool) {
	for _, step := range steps {
		if step.Target.EntityID == taskID && normalizePresetKey(step.AgentKey) == model.AgentPresetCodeBuilder {
			return step, true
		}
	}
	for _, step := range steps {
		if step.Target.EntityID == taskID && strings.EqualFold(strings.TrimSpace(step.AgentName), "Forge") {
			return step, true
		}
	}
	return model.CommandBarPlanStep{}, false
}

func commandBarStepIndexForRun(runIDsByStep map[int]string, runID string) int {
	for index, id := range runIDsByStep {
		if id == runID {
			return index
		}
	}
	return -1
}

func (s *AgentService) advanceFanOutCommandBarPlan(ctx context.Context, run *model.AgentRun, payload commandBarTriggerContextPayload) (*model.AgentRun, error) {
	if s.commandBarPlanRepo == nil || payload.PlanID == "" {
		return nil, nil
	}
	if run.Status == model.AgentRunStatusFailed {
		_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, payload.PlanID, commandBarTerminalRunMessage(run, payload, "failed"))
		return nil, nil
	}
	if run.Status == model.AgentRunStatusCancelled {
		_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, payload.PlanID, commandBarTerminalRunMessage(run, payload, "cancelled"))
		return nil, nil
	}
	if run.Status != model.AgentRunStatusCompleted {
		return nil, nil
	}
	plan, err := s.commandBarPlanRepo.GetByID(ctx, run.WorkspaceID, payload.PlanID)
	if err != nil || plan == nil {
		return nil, err
	}
	if plan.Status == model.CommandBarPlanStatusCancelled || plan.Status == model.CommandBarPlanStatusFailed {
		return nil, nil
	}
	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	if len(runIDsByStep) < len(payload.Steps) {
		return nil, nil
	}
	runIDs := make([]string, 0, len(runIDsByStep))
	for _, runID := range runIDsByStep {
		runIDs = append(runIDs, runID)
	}
	runs, err := s.runRepo.ListByIDs(ctx, run.WorkspaceID, runIDs)
	if err != nil {
		return nil, err
	}
	if len(runs) < len(payload.Steps) {
		return nil, nil
	}
	allCompleted := true
	for _, item := range runs {
		if item.ID == run.ID {
			// The caller's run carries the authoritative status: for delegated
			// runs the finalizer dispatches before the terminal row update
			// persists, so the freshly listed sibling row can still be stale.
			item = *run
		}
		switch item.Status {
		case model.AgentRunStatusCompleted:
		case model.AgentRunStatusFailed:
			_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, payload.PlanID, commandBarTerminalRunMessage(&item, payload, "failed"))
			return nil, nil
		case model.AgentRunStatusCancelled:
			_ = s.commandBarPlanRepo.MarkFailed(ctx, run.WorkspaceID, payload.PlanID, commandBarTerminalRunMessage(&item, payload, "cancelled"))
			return nil, nil
		default:
			allCompleted = false
		}
	}
	if allCompleted {
		_ = s.commandBarPlanRepo.MarkCompleted(ctx, run.WorkspaceID, payload.PlanID)
	}
	return nil, nil
}
