package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Command-bar plan lifecycle: list, detail, dismiss, cancel, resume, retry, promotion, tool catalog.

func (s *CommandBarService) ListPlans(ctx context.Context, workspaceID, actorID string, limit int) (*model.CommandBarPlanListResponse, error) {
	if s == nil || s.planRepo == nil {
		return &model.CommandBarPlanListResponse{Plans: []model.CommandBarPlanSummary{}}, nil
	}
	trimmedActor := strings.TrimSpace(actorID)
	dismissed := map[string]struct{}{}
	if s.dismissalRepo != nil && trimmedActor != "" {
		ids, err := s.dismissalRepo.ListDismissedPlanIDs(ctx, workspaceID, trimmedActor)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			dismissed[id] = struct{}{}
		}
	}
	// Pull a few extra records so dismissals don't shrink the visible window
	// below the requested limit.
	fetchLimit := limit
	if fetchLimit <= 0 || fetchLimit > 50 {
		fetchLimit = 20
	}
	if len(dismissed) > 0 {
		fetchLimit += len(dismissed)
		if fetchLimit > 50 {
			fetchLimit = 50
		}
	}
	records, err := s.planRepo.ListRecent(ctx, workspaceID, trimmedActor, fetchLimit)
	if err != nil {
		return nil, err
	}
	summaries := make([]model.CommandBarPlanSummary, 0, len(records))
	for _, record := range records {
		if _, hidden := dismissed[record.ID]; hidden {
			continue
		}
		summary, err := s.commandBarPlanSummaryForRecord(ctx, workspaceID, record)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
		if limit > 0 && len(summaries) >= limit {
			break
		}
	}
	return &model.CommandBarPlanListResponse{Plans: summaries}, nil
}

// ListEntityPlans returns command-bar plans targeting the given entity (e.g. an
// epic), hydrated with their agent runs, regardless of which actor triggered
// them. Visibility is enforced at the route by entity-read permissions, so this
// deliberately skips the actor filter and dismissal handling used by ListPlans.
func (s *CommandBarService) ListEntityPlans(ctx context.Context, workspaceID, entityType, entityID string, limit int) (*model.CommandBarPlanListResponse, error) {
	if s == nil || s.planRepo == nil {
		return &model.CommandBarPlanListResponse{Plans: []model.CommandBarPlanSummary{}}, nil
	}
	records, err := s.planRepo.ListByEntity(ctx, workspaceID, entityType, entityID, limit)
	if err != nil {
		return nil, err
	}
	summaries := make([]model.CommandBarPlanSummary, 0, len(records))
	for _, record := range records {
		summary, err := s.commandBarPlanSummaryForRecord(ctx, workspaceID, record)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return &model.CommandBarPlanListResponse{Plans: summaries}, nil
}

// DismissPlans hides the given plans from the actor's command runs rail. Only
// plans owned by the actor can be dismissed; unknown or unauthorized plan IDs
// are silently skipped so a stale client cannot enumerate other users' runs.
func (s *CommandBarService) DismissPlans(ctx context.Context, workspaceID, actorID string, planIDs []string) error {
	if s == nil || s.dismissalRepo == nil {
		return fmt.Errorf("command bar service is not configured for dismissals")
	}
	trimmedActor := strings.TrimSpace(actorID)
	if trimmedActor == "" {
		return fmt.Errorf("actor is required")
	}
	allowed := make([]string, 0, len(planIDs))
	for _, id := range planIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		plan, err := s.planRepo.GetByID(ctx, workspaceID, id)
		if err != nil {
			return err
		}
		if plan == nil || !commandBarPlanOwnedByActor(plan, trimmedActor) {
			continue
		}
		allowed = append(allowed, id)
	}
	if len(allowed) == 0 {
		return nil
	}
	return s.dismissalRepo.Dismiss(ctx, workspaceID, trimmedActor, allowed)
}

func (s *CommandBarService) GetPlan(ctx context.Context, workspaceID, actorID, planID string) (*model.CommandBarPlanDetailResponse, error) {
	if s == nil || s.planRepo == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	record, err := s.planRepo.GetByID(ctx, workspaceID, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}
	if record == nil || !commandBarPlanOwnedByActor(record, actorID) {
		return nil, fmt.Errorf("command bar plan not found")
	}
	summary, err := s.commandBarPlanSummaryForRecord(ctx, workspaceID, *record)
	if err != nil {
		return nil, err
	}
	return &model.CommandBarPlanDetailResponse{Plan: summary}, nil
}

// GetWorkspacePlan loads a plan without the dock's owner gate, for
// team-actionable flows (epic-page resume/retry) where any member with
// command-bar edit permission may act on another actor's delivery. Callers
// remain responsible for per-step authorization.
func (s *CommandBarService) GetWorkspacePlan(ctx context.Context, workspaceID, planID string) (*model.CommandBarPlanDetailResponse, error) {
	if s == nil || s.planRepo == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	record, err := s.planRepo.GetByID(ctx, workspaceID, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	summary, err := s.commandBarPlanSummaryForRecord(ctx, workspaceID, *record)
	if err != nil {
		return nil, err
	}
	return &model.CommandBarPlanDetailResponse{Plan: summary}, nil
}

func (s *CommandBarService) commandBarPlanSummaryForRecord(ctx context.Context, workspaceID string, record model.CommandBarPlanRecord) (model.CommandBarPlanSummary, error) {
	runIDsByStep := decodeCommandBarPlanRunIDs(record.RunIDsByStep)
	runIDs := make([]string, 0, len(runIDsByStep))
	for _, runID := range runIDsByStep {
		runIDs = append(runIDs, runID)
	}
	runs, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, runIDs)
	if err != nil {
		return model.CommandBarPlanSummary{}, err
	}
	return commandBarPlanSummary(record, runs), nil
}

func (s *CommandBarService) CancelPlan(ctx context.Context, workspaceID, actorID, planID string) (*model.CommandBarCancelPlanResponse, error) {
	if s == nil || s.planRepo == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	plan, err := s.planRepo.GetByID(ctx, workspaceID, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	if !commandBarPlanOwnedByActor(plan, actorID) {
		return nil, fmt.Errorf("command bar plan not found")
	}
	if err := s.planRepo.MarkCancelled(ctx, workspaceID, plan.ID); err != nil {
		return nil, err
	}
	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	runIDs := make([]string, 0, len(runIDsByStep))
	for _, runID := range runIDsByStep {
		runIDs = append(runIDs, runID)
	}
	runs, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, runIDs)
	if err != nil {
		return nil, err
	}
	updatedRuns := make([]model.AgentRun, 0, len(runs))
	for _, run := range runs {
		if model.IsAgentRunActiveStatus(run.Status) {
			updated, err := s.agentService.CancelRun(ctx, workspaceID, run.ID, actorID)
			if err == nil && updated != nil {
				updatedRuns = append(updatedRuns, *updated)
				continue
			}
		}
		updatedRuns = append(updatedRuns, run)
	}
	updatedPlan, err := s.planRepo.GetByID(ctx, workspaceID, plan.ID)
	if err != nil {
		return nil, err
	}
	return &model.CommandBarCancelPlanResponse{
		Plan: commandBarPlanSummary(*updatedPlan, updatedRuns),
		Runs: updatedRuns,
	}, nil
}

func (s *CommandBarService) ResumePlan(ctx context.Context, workspaceID, actorID, planID string) (*model.CommandBarResumePlanResponse, error) {
	if s == nil || s.planRepo == nil || s.agentService == nil || s.agentService.runRepo == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	plan, err := s.planRepo.GetByID(ctx, workspaceID, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}
	// Resume is intentionally NOT owner-gated: epic delivery plans surface to
	// the whole team on the epic page, and any member with command-bar edit
	// permission (enforced at the router) may revive a stalled delivery.
	if plan == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	if plan.Status != model.CommandBarPlanStatusRunning {
		return nil, fmt.Errorf("only running command bar plans can be resumed")
	}
	var pageContext model.CommandBarPageContext
	if err := json.Unmarshal(plan.PageContext, &pageContext); err != nil {
		return nil, fmt.Errorf("decode command bar plan context: %w", err)
	}
	var steps []model.CommandBarPlanStep
	if err := json.Unmarshal(plan.Steps, &steps); err != nil {
		return nil, fmt.Errorf("decode command bar plan steps: %w", err)
	}
	progress, err := s.agentService.startReadyCommandBarPlanSteps(ctx, CommandBarPlanInput{
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		PlanID:      plan.ID,
		Prompt:      plan.Prompt,
		PageContext: pageContext,
		Steps:       steps,
	})
	if err != nil {
		return nil, err
	}
	updatedPlan, err := s.planRepo.GetByID(ctx, workspaceID, plan.ID)
	if err != nil {
		return nil, err
	}
	if updatedPlan == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	latestRunIDsByStep := decodeCommandBarPlanRunIDs(updatedPlan.RunIDsByStep)
	latestRunIDs := make([]string, 0, len(latestRunIDsByStep))
	for _, runID := range latestRunIDsByStep {
		latestRunIDs = append(latestRunIDs, runID)
	}
	latestRuns, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, latestRunIDs)
	if err != nil {
		return nil, err
	}
	var startedRun *model.AgentRun
	if progress != nil && len(progress.Started) > 0 {
		for i := range latestRuns {
			if latestRuns[i].ID == progress.Started[0] {
				startedRun = &latestRuns[i]
				break
			}
		}
	}
	return &model.CommandBarResumePlanResponse{
		Plan: commandBarPlanSummary(*updatedPlan, latestRuns),
		Run:  startedRun,
		Runs: latestRuns,
	}, nil
}

func (s *CommandBarService) RetryPlanFromStep(ctx context.Context, workspaceID, actorID, planID string, req model.CommandBarRetryPlanRequest) (*model.CommandBarRetryPlanResponse, error) {
	if s == nil || s.planRepo == nil || s.agentService == nil || s.agentService.runRepo == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}
	plan, err := s.planRepo.GetByID(ctx, workspaceID, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	// Like ResumePlan, retry is team-actionable (not owner-gated): a failed
	// epic delivery can be retried by any member with command-bar edit
	// permission. Retried runs are attributed to the retrying actor.
	//
	// A plan can be left in status "running" with all of its child runs
	// failed or cancelled (e.g. runs cancelled mid-delivery) — a zombie that
	// resume cannot revive because there is nothing ready to start. Retry is
	// the only way out, so only reject while work is genuinely in flight.
	if plan.Status == model.CommandBarPlanStatusRunning {
		activeIDs := make([]string, 0)
		for _, runID := range decodeCommandBarPlanRunIDs(plan.RunIDsByStep) {
			activeIDs = append(activeIDs, runID)
		}
		runs, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, activeIDs)
		if err != nil {
			return nil, err
		}
		for _, run := range runs {
			if model.IsAgentRunActiveStatus(run.Status) {
				return nil, fmt.Errorf("command bar plan still has active runs; cancel them or wait before retrying")
			}
		}
	}
	var pageContext model.CommandBarPageContext
	if err := json.Unmarshal(plan.PageContext, &pageContext); err != nil {
		return nil, fmt.Errorf("decode command bar plan context: %w", err)
	}
	var steps []model.CommandBarPlanStep
	if err := json.Unmarshal(plan.Steps, &steps); err != nil {
		return nil, fmt.Errorf("decode command bar plan steps: %w", err)
	}
	stepIndex := req.StepIndex
	if stepIndex < 0 || stepIndex >= len(steps) {
		return nil, fmt.Errorf("step_index must be between 0 and %d", len(steps)-1)
	}
	planKind := commandBarPlanKindForSteps(steps)
	if planKind == model.CommandBarPlanKindTaskPipeline || planKind == model.CommandBarPlanKindDAG {
		return s.retryFailedDAGRuns(ctx, workspaceID, actorID, *plan, pageContext, steps)
	}
	if planKind == model.CommandBarPlanKindFanOut {
		return s.retryFailedFanOutRuns(ctx, workspaceID, actorID, *plan, pageContext, steps)
	}
	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	for index, runID := range runIDsByStep {
		if index < stepIndex {
			continue
		}
		if run, err := s.agentService.runRepo.GetByID(ctx, workspaceID, runID); err == nil && run != nil && model.IsAgentRunActiveStatus(run.Status) {
			_, _ = s.agentService.CancelRun(ctx, workspaceID, run.ID, actorID)
		}
		delete(runIDsByStep, index)
	}
	var parentRunID *string
	if stepIndex > 0 {
		previousRunID := strings.TrimSpace(runIDsByStep[stepIndex-1])
		if previousRunID == "" {
			return nil, fmt.Errorf("cannot retry from step %d without a previous run", stepIndex+1)
		}
		parentRunID = &previousRunID
	}
	run, err := s.agentService.startCommandBarPlanStep(ctx, workspaceID, actorID, plan.Prompt, pageContext, steps, stepIndex, plan.ID, parentRunID)
	if err != nil {
		_ = s.planRepo.MarkFailed(ctx, workspaceID, plan.ID, err.Error())
		return nil, err
	}
	runIDsByStep[stepIndex] = run.ID
	rawRunIDs, _ := json.Marshal(runIDsByStep)
	if err := s.planRepo.RestartStepRun(ctx, workspaceID, plan.ID, stepIndex, rawRunIDs); err != nil {
		return nil, err
	}
	updatedPlan, err := s.planRepo.GetByID(ctx, workspaceID, plan.ID)
	if err != nil {
		return nil, err
	}
	return &model.CommandBarRetryPlanResponse{
		Plan: commandBarPlanSummary(*updatedPlan, []model.AgentRun{*run}),
		Run:  run,
		Runs: []model.AgentRun{*run},
	}, nil
}

func (s *CommandBarService) retryFailedDAGRuns(ctx context.Context, workspaceID, actorID string, plan model.CommandBarPlanRecord, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep) (*model.CommandBarRetryPlanResponse, error) {
	if s == nil || s.planRepo == nil || s.agentService == nil || s.agentService.runRepo == nil {
		return nil, fmt.Errorf("command bar plan service is not configured")
	}

	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	runIDs := make([]string, 0, len(runIDsByStep))
	for _, runID := range runIDsByStep {
		runIDs = append(runIDs, runID)
	}
	runs, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, runIDs)
	if err != nil {
		return nil, err
	}
	runsByID := make(map[string]model.AgentRun, len(runs))
	for _, run := range runs {
		runsByID[run.ID] = run
	}

	failedIndexes := make([]int, 0)
	for stepIndex, runID := range runIDsByStep {
		run, ok := runsByID[runID]
		if !ok {
			continue
		}
		if run.Status == model.AgentRunStatusFailed || run.Status == model.AgentRunStatusCancelled {
			failedIndexes = append(failedIndexes, stepIndex)
		}
	}
	if len(failedIndexes) == 0 {
		return nil, fmt.Errorf("no failed DAG steps to retry")
	}
	slices.Sort(failedIndexes)
	for _, stepIndex := range failedIndexes {
		delete(runIDsByStep, stepIndex)
	}
	updatedRunIDs, err := json.Marshal(runIDsByStep)
	if err != nil {
		return nil, fmt.Errorf("encode retried DAG run ids: %w", err)
	}
	if err := s.planRepo.RestartStepRun(ctx, workspaceID, plan.ID, failedIndexes[0], updatedRunIDs); err != nil {
		return nil, err
	}

	progress, err := s.agentService.startReadyCommandBarPlanSteps(ctx, CommandBarPlanInput{
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		PlanID:      plan.ID,
		Prompt:      plan.Prompt,
		PageContext: pageContext,
		Steps:       steps,
	})
	if err != nil {
		return nil, err
	}

	updatedPlan, err := s.planRepo.GetByID(ctx, workspaceID, plan.ID)
	if err != nil {
		return nil, err
	}
	if updatedPlan == nil {
		return nil, fmt.Errorf("command bar plan not found")
	}
	latestRunIDsByStep := decodeCommandBarPlanRunIDs(updatedPlan.RunIDsByStep)
	latestRunIDs := make([]string, 0, len(latestRunIDsByStep))
	for _, runID := range latestRunIDsByStep {
		latestRunIDs = append(latestRunIDs, runID)
	}
	latestRuns, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, latestRunIDs)
	if err != nil {
		return nil, err
	}
	var startedRun *model.AgentRun
	if progress != nil && len(progress.Started) > 0 {
		for i := range latestRuns {
			if latestRuns[i].ID == progress.Started[0] {
				startedRun = &latestRuns[i]
				break
			}
		}
	}
	return &model.CommandBarRetryPlanResponse{
		Plan: commandBarPlanSummary(*updatedPlan, latestRuns),
		Run:  startedRun,
		Runs: latestRuns,
	}, nil
}

func (s *CommandBarService) retryFailedFanOutRuns(ctx context.Context, workspaceID, actorID string, plan model.CommandBarPlanRecord, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep) (*model.CommandBarRetryPlanResponse, error) {
	runIDsByStep := decodeCommandBarPlanRunIDs(plan.RunIDsByStep)
	retrySteps := make([]int, 0)
	for index, runID := range runIDsByStep {
		run, err := s.agentService.runRepo.GetByID(ctx, workspaceID, runID)
		if err != nil {
			return nil, err
		}
		if run == nil {
			continue
		}
		if model.IsAgentRunActiveStatus(run.Status) {
			return nil, fmt.Errorf("fan-out plan still has active runs; wait for them to finish or cancel the plan")
		}
		if run.Status == model.AgentRunStatusFailed || run.Status == model.AgentRunStatusCancelled {
			retrySteps = append(retrySteps, index)
		}
	}
	slices.Sort(retrySteps)
	if len(retrySteps) == 0 {
		return nil, fmt.Errorf("fan-out plan has no failed or cancelled targets to retry")
	}
	started := make([]model.AgentRun, 0, len(retrySteps))
	for _, index := range retrySteps {
		if index < 0 || index >= len(steps) {
			continue
		}
		run, err := s.agentService.startCommandBarPlanStep(ctx, workspaceID, actorID, plan.Prompt, pageContext, steps, index, plan.ID, nil)
		if err != nil {
			_ = s.planRepo.MarkFailed(ctx, workspaceID, plan.ID, err.Error())
			return nil, err
		}
		runIDsByStep[index] = run.ID
		started = append(started, *run)
	}
	rawRunIDs, _ := json.Marshal(runIDsByStep)
	if err := s.planRepo.RestartStepRun(ctx, workspaceID, plan.ID, retrySteps[0], rawRunIDs); err != nil {
		return nil, err
	}
	updatedPlan, err := s.planRepo.GetByID(ctx, workspaceID, plan.ID)
	if err != nil {
		return nil, err
	}
	resp := &model.CommandBarRetryPlanResponse{
		Plan: commandBarPlanSummary(*updatedPlan, started),
		Runs: started,
	}
	if len(started) > 0 {
		resp.Run = &started[0]
	}
	return resp, nil
}

func (s *CommandBarService) PromoteRunToAgent(ctx context.Context, workspaceID, actorID, runID string, req model.PromoteCommandBarRunRequest) (*model.PromoteCommandBarRunResponse, error) {
	if s == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar service is not configured")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	run, err := s.agentService.runRepo.GetByID(ctx, workspaceID, strings.TrimSpace(runID))
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("run not found")
	}
	payload, ok := commandBarRunPayload(run)
	if !ok {
		return nil, fmt.Errorf("only command-bar runs can be promoted")
	}
	if run.Status != model.AgentRunStatusCompleted {
		return nil, fmt.Errorf("only completed command-bar runs can be promoted")
	}
	if payload.StepIndex < 0 || payload.StepIndex >= len(payload.Steps) {
		return nil, fmt.Errorf("command-bar step metadata is invalid")
	}
	sourceAgent, err := s.agentService.agentRepo.GetByID(ctx, workspaceID, run.AgentID)
	if err != nil {
		return nil, err
	}
	if sourceAgent == nil {
		return nil, fmt.Errorf("source agent not found")
	}
	step := payload.Steps[payload.StepIndex]
	allowedTools := step.AllowedTools
	if len(allowedTools) == 0 {
		allowedTools = parseJSONStringSlice(sourceAgent.AllowedTools)
	}
	if len(req.AllowedTools) > 0 {
		allowedTools = normalizeStringSlice(req.AllowedTools)
	}
	if err := validateRunAllowedTools(allowedTools, sourceAgent); err != nil {
		return nil, err
	}
	allowedTargets := []string{strings.TrimSpace(run.TargetType)}
	if len(req.AllowedTargets) > 0 {
		allowedTargets = normalizeStringSlice(req.AllowedTargets)
	}
	if err := validatePromotedAgentTargets(allowedTargets, sourceAgent); err != nil {
		return nil, err
	}
	role := strings.TrimSpace(derefString(req.Description))
	if role == "" {
		role = fmt.Sprintf("Reusable agent promoted from command-bar run %s. Original step instruction: %s", run.ID, strings.TrimSpace(step.Instructions))
	}
	planningNotes := fmt.Sprintf("Promoted from command-bar run %s in plan %s. Source agent: %s. Source target: %s/%s.", run.ID, payload.PlanID, sourceAgent.Name, run.TargetType, run.TargetID)
	agent, err := s.agentService.CreateAgent(ctx, model.CreateAgentRequest{
		WorkspaceID:           workspaceID,
		Name:                  name,
		Role:                  role,
		RuntimeKind:           &sourceAgent.RuntimeKind,
		Provider:              sourceAgent.Provider,
		Model:                 sourceAgent.Model,
		ExecutionConfig:       json.RawMessage(sourceAgent.ExecutionConfig),
		PlanningNotes:         strPtr(planningNotes),
		AllowedTools:          mustJSONStringSlice(allowedTools),
		AllowedCommands:       sourceAgent.AllowedCommands,
		AllowedTargets:        mustJSONStringSlice(allowedTargets),
		DefaultInvocationMode: &sourceAgent.DefaultInvocationMode,
	}, actorID)
	if err != nil {
		return nil, err
	}
	return &model.PromoteCommandBarRunResponse{Agent: *agent}, nil
}

func (s *CommandBarService) GetAgentToolCatalog(ctx context.Context, workspaceID, agentID string, selectedTools []string) (*model.CommandBarToolCatalogResponse, error) {
	if s == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar service is not configured")
	}
	agent, err := s.agentService.agentRepo.GetByID(ctx, workspaceID, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	// Tool catalog access is workspace-scoped, not command-bar-candidate scoped,
	// so admins can inspect saved agents before deciding whether to expose them.
	allowedTools := normalizeStringSlice(parseJSONStringSlice(agent.AllowedTools))
	allowedSet := make(map[string]bool, len(allowedTools))
	for _, tool := range allowedTools {
		allowedSet[tool] = true
	}
	selectedTools = normalizeStringSlice(selectedTools)
	selectedSet := make(map[string]bool, len(selectedTools))
	for _, tool := range selectedTools {
		selectedSet[tool] = true
	}
	validation := make([]string, 0)
	validationSet := map[string]bool{}
	addValidation := func(message string) {
		if validationSet[message] {
			return
		}
		validationSet[message] = true
		validation = append(validation, message)
	}
	catalog := s.agentService.ListToolCatalog()
	entries := make([]model.CommandBarToolCatalogEntry, 0, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		allowed := allowedSet[tool.Name]
		entry := model.CommandBarToolCatalogEntry{
			ID:          tool.Name,
			Name:        tool.Name,
			Description: tool.Description,
			Category:    tool.Category,
			InputSchema: tool.InputSchema,
			Allowed:     allowed,
			Selected:    selectedSet[tool.Name],
		}
		if !allowed {
			entry.DisabledReason = "Tool is outside this agent's allowlist."
			if entry.Selected {
				addValidation(fmt.Sprintf("tool %q is outside this agent's allowlist", tool.Name))
			}
		}
		entries = append(entries, entry)
	}
	for _, tool := range selectedTools {
		if !allowedSet[tool] {
			addValidation(fmt.Sprintf("tool %q is outside this agent's allowlist", tool))
		}
	}
	return &model.CommandBarToolCatalogResponse{
		AgentID:        agent.ID,
		AllowedTools:   allowedTools,
		SelectedTools:  selectedTools,
		Tools:          entries,
		Categories:     catalog.Categories,
		Validation:     validation,
		AllowedTargets: parseJSONStringSlice(agent.AllowedTargets),
	}, nil
}
