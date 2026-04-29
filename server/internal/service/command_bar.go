package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const maxCommandBarPlanSteps = 5

type commandBarTriggerContextPayload struct {
	PlanID      string                      `json:"plan_id,omitempty"`
	Prompt      string                      `json:"prompt,omitempty"`
	PageContext model.CommandBarPageContext `json:"page_context"`
	Steps       []model.CommandBarPlanStep  `json:"steps"`
	RunCount    int                         `json:"run_count"`
	StepIndex   int                         `json:"step_index"`
}

type CommandBarService struct {
	agentService  *AgentService
	planRepo      *repository.CommandBarPlanRepository
	unmetRepo     *repository.CommandBarUnmetIntentRepository
	dismissalRepo *repository.CommandBarPlanDismissalRepository
	llmProvider   llm.Provider
}

func NewCommandBarService(agentService *AgentService, planRepo *repository.CommandBarPlanRepository, unmetRepo *repository.CommandBarUnmetIntentRepository, dismissalRepo *repository.CommandBarPlanDismissalRepository, llmProvider llm.Provider) *CommandBarService {
	return &CommandBarService{
		agentService:  agentService,
		planRepo:      planRepo,
		unmetRepo:     unmetRepo,
		dismissalRepo: dismissalRepo,
		llmProvider:   llmProvider,
	}
}

func (s *CommandBarService) ParseIntent(ctx context.Context, workspaceID, actorID string, req model.CommandBarParseRequest) (*model.CommandBarParseResponse, error) {
	if s == nil || s.agentService == nil {
		return nil, fmt.Errorf("command bar service is not configured")
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	pageContext := normalizeCommandBarPageContext(req.PageContext, workspaceID)
	if err := validateCommandBarSupportedTarget(pageContext.EntityType); err != nil {
		resp := s.noMatchResponse(ctx, workspaceID, actorID, text, pageContext, nil, err.Error())
		return resp, nil
	}

	agents, err := s.agentService.ListAgents(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	candidates := commandBarCandidatesForTarget(agents, pageContext.EntityType)
	if len(candidates) == 0 {
		reason := fmt.Sprintf("No available agent can run on %s.", pageContext.EntityType)
		resp := s.noMatchResponse(ctx, workspaceID, actorID, text, pageContext, nil, reason)
		return resp, nil
	}
	narrowCandidates := commandBarNarrowCandidates(candidates)

	if parsed := parseFanOutIntent(text, pageContext, narrowCandidates, candidates); parsed != nil {
		return parsed, nil
	}
	if parsed := parseExplicitNamedAgents(text, pageContext, candidates); parsed != nil {
		return parsed, nil
	}
	if parsed := parsePreferredOneShotCommandIntent(text, pageContext, candidates); parsed != nil {
		return parsed, nil
	}
	if parsed := parseIntentDeterministically(text, pageContext, narrowCandidates); parsed != nil {
		return parsed, nil
	}
	if parsed := s.parseIntentWithLLM(ctx, text, pageContext, narrowCandidates); parsed != nil && parsed.Status == model.CommandBarParseStatusPlan {
		return parsed, nil
	}
	if parsed := parseOneShotCommandIntent(text, pageContext, candidates); parsed != nil {
		return parsed, nil
	}

	reason := "No available agent matched this request with enough confidence."
	return s.noMatchResponse(ctx, workspaceID, actorID, text, pageContext, candidates, reason), nil
}

func (s *CommandBarService) DispatchPlan(ctx context.Context, workspaceID, actorID string, req model.CommandBarDispatchRequest) (*model.CommandBarDispatchResponse, error) {
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
			return nil, fmt.Errorf("one-shot command step %d requires at least one enabled tool", i+1)
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
		if err := s.planRepo.Create(ctx, record); err != nil {
			return nil, err
		}
	}
	if commandBarPlanKindForSteps(steps) == model.CommandBarPlanKindFanOut {
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
	for i, step := range steps {
		agent, ok := byID[step.AgentID]
		if !ok {
			return fmt.Errorf("agent not found for step %d", i+1)
		}
		if normalizePresetKey(agent.PresetKey) == model.AgentPresetCommandAgent {
			if step.PlanKind != model.CommandBarPlanKindOneShotCommand {
				return fmt.Errorf("command agent step %d must be dispatched as a one-shot command", i+1)
			}
			if len(step.AllowedTools) == 0 {
				return fmt.Errorf("command agent step %d requires a narrowed tool subset", i+1)
			}
		}
	}
	return nil
}

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

func (s *CommandBarService) RetryPlanFromStep(ctx context.Context, workspaceID, actorID, planID string, req model.CommandBarRetryPlanRequest) (*model.CommandBarRetryPlanResponse, error) {
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
	if plan.Status == model.CommandBarPlanStatusRunning {
		return nil, fmt.Errorf("running command bar plans cannot be retried")
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
	if commandBarPlanKindForSteps(steps) == model.CommandBarPlanKindFanOut {
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

func (s *CommandBarService) ListUnmetIntents(ctx context.Context, workspaceID, status string, limit int, includeSensitive bool) (*model.CommandBarUnmetIntentListResponse, error) {
	if s == nil || s.unmetRepo == nil {
		return &model.CommandBarUnmetIntentListResponse{Intents: []model.CommandBarUnmetIntentSummary{}}, nil
	}
	intents, err := s.unmetRepo.List(ctx, workspaceID, strings.TrimSpace(status), limit)
	if err != nil {
		return nil, err
	}
	summaries := make([]model.CommandBarUnmetIntentSummary, 0, len(intents))
	for _, intent := range intents {
		summaries = append(summaries, commandBarUnmetIntentSummary(intent, includeSensitive))
	}
	return &model.CommandBarUnmetIntentListResponse{Intents: summaries}, nil
}

func (s *CommandBarService) ReviewUnmetIntent(ctx context.Context, workspaceID, id string, req model.ReviewCommandBarUnmetIntentRequest) (*model.CommandBarUnmetIntentSummary, error) {
	if s == nil || s.unmetRepo == nil {
		return nil, fmt.Errorf("command bar unmet intent repository is not configured")
	}
	status := strings.TrimSpace(req.Status)
	switch status {
	case "open", "accepted", "rejected", "deferred":
	default:
		return nil, fmt.Errorf("unsupported unmet intent status %q", status)
	}
	intent, err := s.unmetRepo.Review(ctx, workspaceID, strings.TrimSpace(id), status, req.Notes)
	if err != nil {
		return nil, err
	}
	summary := commandBarUnmetIntentSummary(*intent, false)
	return &summary, nil
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
	return s.startTargetRun(ctx, workspaceID, target.EntityType, target.EntityID, model.StartAgentRunRequest{
		AgentID:           agentID,
		AdditionalContext: &additionalContext,
		AllowedTools:      step.AllowedTools,
	}, actor, triggerContext, event, parentRunID)
}

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
	payload, ok := commandBarRunPayload(run)
	if !ok {
		return nil, nil
	}
	if commandBarPlanKindForSteps(payload.Steps) == model.CommandBarPlanKindFanOut {
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

func commandBarTerminalRunMessage(run *model.AgentRun, payload commandBarTriggerContextPayload, fallback string) string {
	if run != nil && strings.TrimSpace(derefString(run.ErrorMessage)) != "" {
		return strings.TrimSpace(derefString(run.ErrorMessage))
	}
	stepNumber := payload.StepIndex + 1
	if stepNumber <= 0 {
		stepNumber = 1
	}
	return fmt.Sprintf("Command-bar step %d %s.", stepNumber, fallback)
}

func commandBarPlanOwnedByActor(plan *model.CommandBarPlanRecord, actorID string) bool {
	if plan == nil {
		return false
	}
	actorID = strings.TrimSpace(actorID)
	if actorID == "" || plan.ActorID == nil {
		return true
	}
	return strings.TrimSpace(*plan.ActorID) == actorID
}

func commandBarRunPayload(run *model.AgentRun) (commandBarTriggerContextPayload, bool) {
	var empty commandBarTriggerContextPayload
	if run == nil || len(run.Input) == 0 {
		return empty, false
	}
	var input model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &input); err != nil || input.Trigger == nil {
		return empty, false
	}
	if input.Trigger.Source != model.AgentRunTriggerSourceCommandBar || input.Trigger.TriggerType != model.AgentRunTriggerTypeCommandBar {
		return empty, false
	}
	if len(input.Trigger.Context) == 0 {
		return empty, false
	}
	var payload commandBarTriggerContextPayload
	if err := json.Unmarshal(input.Trigger.Context, &payload); err != nil {
		return empty, false
	}
	if payload.RunCount == 0 {
		payload.RunCount = len(payload.Steps)
	}
	return payload, true
}

func newCommandBarPlanRecord(workspaceID, actorID, planID, prompt string, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep) (*model.CommandBarPlanRecord, error) {
	pageContextRaw, err := json.Marshal(pageContext)
	if err != nil {
		return nil, err
	}
	stepsRaw, err := json.Marshal(steps)
	if err != nil {
		return nil, err
	}
	runIDsRaw, _ := json.Marshal(map[int]string{})
	var actor *string
	if strings.TrimSpace(actorID) != "" {
		actor = &actorID
	}
	return &model.CommandBarPlanRecord{
		ID:               planID,
		WorkspaceID:      workspaceID,
		ActorID:          actor,
		Status:           model.CommandBarPlanStatusRunning,
		Prompt:           strings.TrimSpace(prompt),
		PageContext:      pageContextRaw,
		Steps:            stepsRaw,
		RunIDsByStep:     runIDsRaw,
		CurrentStepIndex: 0,
		RunCount:         len(steps),
	}, nil
}

func decodeCommandBarPlanRunIDs(raw json.RawMessage) map[int]string {
	result := map[int]string{}
	if len(raw) == 0 {
		return result
	}
	var keyed map[string]string
	if err := json.Unmarshal(raw, &keyed); err == nil {
		for key, value := range keyed {
			if idx, scanErr := strconv.Atoi(key); scanErr == nil && strings.TrimSpace(value) != "" {
				result[idx] = value
			}
		}
		return result
	}
	_ = json.Unmarshal(raw, &result)
	return result
}

func commandBarPlanSummary(record model.CommandBarPlanRecord, runs []model.AgentRun) model.CommandBarPlanSummary {
	var pageContext model.CommandBarPageContext
	_ = json.Unmarshal(record.PageContext, &pageContext)
	var steps []model.CommandBarPlanStep
	_ = json.Unmarshal(record.Steps, &steps)
	return model.CommandBarPlanSummary{
		ID:               record.ID,
		Status:           record.Status,
		PlanKind:         commandBarPlanKindForSteps(steps),
		Prompt:           record.Prompt,
		PageContext:      pageContext,
		Steps:            steps,
		RunIDsByStep:     decodeCommandBarPlanRunIDs(record.RunIDsByStep),
		CurrentStepIndex: record.CurrentStepIndex,
		RunCount:         record.RunCount,
		ErrorMessage:     record.ErrorMessage,
		CancelledAt:      record.CancelledAt,
		CompletedAt:      record.CompletedAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
		Runs:             runs,
	}
}

func commandBarUnmetIntentSummary(intent model.CommandBarUnmetIntent, includeSensitive bool) model.CommandBarUnmetIntentSummary {
	var pageContext model.CommandBarPageContext
	_ = json.Unmarshal(intent.PageContext, &pageContext)
	var candidates []model.CommandBarAgent
	_ = json.Unmarshal(intent.CandidateAgents, &candidates)
	prompt := ""
	if includeSensitive {
		prompt = intent.Prompt
	}
	return model.CommandBarUnmetIntentSummary{
		ID:              intent.ID,
		WorkspaceID:     intent.WorkspaceID,
		ActorID:         intent.ActorID,
		Prompt:          prompt,
		PromptPreview:   commandBarPromptPreview(intent.Prompt),
		PromptRedacted:  !includeSensitive,
		PageContext:     pageContext,
		CandidateAgents: candidates,
		Reason:          intent.Reason,
		Status:          intent.Status,
		ReviewNotes:     intent.ReviewNotes,
		ReviewedAt:      intent.ReviewedAt,
		CreatedAt:       intent.CreatedAt,
	}
}

func commandBarPromptPreview(prompt string) string {
	prompt = strings.Join(strings.Fields(strings.TrimSpace(prompt)), " ")
	const maxPreviewRunes = 160
	runes := []rune(prompt)
	if len(runes) <= maxPreviewRunes {
		return prompt
	}
	return string(runes[:maxPreviewRunes]) + "..."
}

func validatePromotedAgentTargets(targets []string, sourceAgent *model.Agent) error {
	targets = normalizeStringSlice(targets)
	if len(targets) == 0 {
		return fmt.Errorf("at least one allowed target is required")
	}
	sourceAllowedTargets := parseJSONStringSlice(sourceAgent.AllowedTargets)
	sourceAllowedSet := make(map[string]bool, len(sourceAllowedTargets))
	for _, target := range sourceAllowedTargets {
		sourceAllowedSet[normalizeCommandBarTargetType(target)] = true
	}
	for _, target := range targets {
		normalized := normalizeCommandBarTargetType(target)
		if err := validateCommandBarSupportedTarget(normalized); err != nil {
			return err
		}
		if len(sourceAllowedSet) > 0 && !sourceAllowedSet[normalized] {
			return fmt.Errorf("target %q is outside source agent %s allowlist", normalized, strings.TrimSpace(sourceAgent.Name))
		}
	}
	return nil
}

func (s *CommandBarService) parseIntentWithLLM(ctx context.Context, text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	if s == nil || s.llmProvider == nil {
		return nil
	}
	candidateJSON, _ := json.Marshal(candidates)
	contextJSON, _ := json.Marshal(pageContext)
	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: fmt.Sprintf(`You classify command-bar requests for Helpin agent runs. Return strict JSON only. Choose agents only when the request clearly fits available candidates and target. If it does not fit, return no_matching_agent with a short reason. Plans may have 1-%d sequential steps. Each step instruction must be scoped to that agent only; do not ask one agent to invoke another agent.`, maxCommandBarPlanSteps),
		Messages: []llm.Message{{
			Role: "user",
			Content: fmt.Sprintf(`Request: %s
Page context: %s
Available agents: %s

Return JSON: {"status":"plan","steps":[{"agent_id":"...","instructions":"..."}],"rationale":"..."} or {"status":"no_matching_agent","reason":"..."}. For backward compatibility, {"status":"plan","agent_id":"...","instructions":"...","rationale":"..."} is also accepted.`, text, string(contextJSON), string(candidateJSON)),
		}},
		Temperature: 0,
		MaxTokens:   500,
		JSONMode:    true,
	})
	if err != nil || resp == nil {
		if err != nil {
			slog.WarnContext(ctx, "command bar llm parse failed", "error", err)
		}
		return nil
	}

	var parsed struct {
		Status       string `json:"status"`
		AgentID      string `json:"agent_id"`
		Instructions string `json:"instructions"`
		Rationale    string `json:"rationale"`
		Reason       string `json:"reason"`
		Steps        []struct {
			AgentID      string `json:"agent_id"`
			Instructions string `json:"instructions"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
		slog.WarnContext(ctx, "command bar llm parse returned invalid json", "error", err)
		return nil
	}
	if parsed.Status == model.CommandBarParseStatusNoMatchingAgent {
		return &model.CommandBarParseResponse{
			Status:      model.CommandBarParseStatusNoMatchingAgent,
			Reason:      firstNonEmptyString(strings.TrimSpace(parsed.Reason), "No available agent matched this request."),
			Suggestions: defaultCommandBarSuggestions(pageContext.EntityType),
			Candidates:  candidates,
		}
	}
	if parsed.Status != model.CommandBarParseStatusPlan {
		return nil
	}
	if len(parsed.Steps) > 0 {
		steps := make([]model.CommandBarPlanStep, 0, min(len(parsed.Steps), maxCommandBarPlanSteps))
		for i, parsedStep := range parsed.Steps {
			if i >= maxCommandBarPlanSteps {
				break
			}
			agent, ok := findCommandBarCandidateByID(candidates, parsedStep.AgentID)
			if !ok {
				return nil
			}
			steps = append(steps, model.CommandBarPlanStep{
				AgentID:      agent.ID,
				AgentKey:     agent.PresetKey,
				AgentName:    agent.Name,
				Target:       pageContext,
				Instructions: firstNonEmptyString(strings.TrimSpace(parsedStep.Instructions), fmt.Sprintf("Execute your normal %s role for the current target.", agent.Name)),
			})
		}
		if len(steps) == 0 {
			return nil
		}
		return commandBarMultiStepPlanResponse(steps, firstNonEmptyString(strings.TrimSpace(parsed.Rationale), "Matched request to available agents."), candidates)
	}
	agent, ok := findCommandBarCandidateByID(candidates, parsed.AgentID)
	if !ok {
		return nil
	}
	return commandBarPlanResponse(agent, pageContext, text, parsed.Instructions, parsed.Rationale, candidates)
}

func parseIntentDeterministically(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	lower := strings.ToLower(strings.TrimSpace(text))
	if parsed := parseExplicitNamedAgents(text, pageContext, candidates); parsed != nil {
		return parsed
	}

	preferredPresets := []string{}
	if containsAny(lower, "review", "qa", "test", "check quality", "audit") {
		preferredPresets = append(preferredPresets, model.AgentPresetReviewAgent)
	}
	if containsAny(lower, "implement", "build", "code", "fix", "bug", "pull request", "pr") {
		preferredPresets = append(preferredPresets, model.AgentPresetCodeBuilder)
	}
	if containsAny(lower, "break down", "breakdown", "plan", "spec", "acceptance criteria", "tasks", "stories") {
		if pageContext.EntityType == "epic" || pageContext.EntityType == "workspace" {
			preferredPresets = append(preferredPresets, model.AgentPresetEpicPlanner)
		}
		preferredPresets = append(preferredPresets, model.AgentPresetTaskPlanner)
	}
	if containsAny(lower, "deal", "crm", "contact", "buyer", "pipeline") {
		preferredPresets = append(preferredPresets, model.AgentPresetCRMOperator)
	}
	if containsAny(lower, "support", "ticket", "conversation", "reply", "customer") {
		preferredPresets = append(preferredPresets, model.AgentPresetSupportAgent)
	}

	for _, preset := range preferredPresets {
		if agent, ok := findCommandBarCandidateByPreset(candidates, preset); ok {
			return commandBarPlanResponse(agent, pageContext, text, text, "Matched request keywords to an available agent.", candidates)
		}
	}
	return nil
}

func (s *CommandBarService) noMatchResponse(ctx context.Context, workspaceID, actorID, text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent, reason string) *model.CommandBarParseResponse {
	reason = firstNonEmptyString(strings.TrimSpace(reason), "No available agent matched this request.")
	_ = s.logUnmetIntent(ctx, workspaceID, actorID, text, pageContext, candidates, reason)
	return &model.CommandBarParseResponse{
		Status:      model.CommandBarParseStatusNoMatchingAgent,
		Reason:      reason,
		Suggestions: defaultCommandBarSuggestions(pageContext.EntityType),
		Candidates:  candidates,
	}
}

func (s *CommandBarService) logUnmetIntent(ctx context.Context, workspaceID, actorID, text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent, reason string) error {
	if s == nil || s.unmetRepo == nil {
		return nil
	}
	pageContextJSON, _ := json.Marshal(pageContext)
	candidatesJSON, _ := json.Marshal(candidates)
	var actor *string
	if strings.TrimSpace(actorID) != "" {
		actor = &actorID
	}
	return s.unmetRepo.Create(ctx, &model.CommandBarUnmetIntent{
		WorkspaceID:     workspaceID,
		ActorID:         actor,
		Prompt:          strings.TrimSpace(text),
		PageContext:     pageContextJSON,
		CandidateAgents: candidatesJSON,
		Reason:          reason,
	})
}

func commandBarCandidatesForTarget(agents []model.Agent, targetType string) []model.CommandBarAgent {
	targetType = normalizeCommandBarTargetType(targetType)
	candidates := make([]model.CommandBarAgent, 0, len(agents))
	for _, agent := range agents {
		allowedTargets := parseJSONStringSlice(agent.AllowedTargets)
		if len(allowedTargets) > 0 && !slices.Contains(allowedTargets, targetType) {
			continue
		}
		candidates = append(candidates, model.CommandBarAgent{
			ID:             agent.ID,
			Name:           agent.Name,
			PresetKey:      normalizePresetKey(agent.PresetKey),
			Role:           agent.Role,
			AllowedTargets: allowedTargets,
			AllowedTools:   parseJSONStringSlice(agent.AllowedTools),
		})
	}
	return candidates
}

func commandBarNarrowCandidates(candidates []model.CommandBarAgent) []model.CommandBarAgent {
	narrow := make([]model.CommandBarAgent, 0, len(candidates))
	for _, candidate := range candidates {
		if isOneShotCommandAgent(candidate) {
			continue
		}
		narrow = append(narrow, candidate)
	}
	return narrow
}

func commandBarPlanResponse(agent model.CommandBarAgent, target model.CommandBarPageContext, text, instructions, rationale string, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	instructions = firstNonEmptyString(strings.TrimSpace(instructions), strings.TrimSpace(text))
	step := model.CommandBarPlanStep{
		AgentID:      agent.ID,
		AgentKey:     agent.PresetKey,
		AgentName:    agent.Name,
		Target:       target,
		Instructions: instructions,
	}
	return commandBarMultiStepPlanResponse([]model.CommandBarPlanStep{step}, firstNonEmptyString(strings.TrimSpace(rationale), "Matched request to an available agent."), candidates)
}

func commandBarMultiStepPlanResponse(steps []model.CommandBarPlanStep, rationale string, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	planKind := commandBarPlanKindForSteps(steps)
	guardrails := []model.CommandBarGuardrail{}
	if len(steps) > maxCommandBarPlanSteps {
		steps = steps[:maxCommandBarPlanSteps]
		planKind = commandBarPlanKindForSteps(steps)
		guardrails = append(guardrails, model.CommandBarGuardrail{
			Type:     "run_count_limit",
			Severity: "warning",
			Message:  fmt.Sprintf("Plan was limited to %d steps.", maxCommandBarPlanSteps),
		})
	}
	return &model.CommandBarParseResponse{
		Status: model.CommandBarParseStatusPlan,
		Plan: &model.CommandBarPlan{
			PlanKind:       planKind,
			Steps:          steps,
			RunCount:       len(steps),
			EstimatedRuns:  len(steps),
			MaxAllowedRuns: maxCommandBarPlanSteps,
			Guardrails:     guardrails,
		},
		Rationale:  firstNonEmptyString(strings.TrimSpace(rationale), "Matched request to available agents."),
		Candidates: candidates,
	}
}

func parseOneShotCommandIntent(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	agent, ok := findCommandBarCandidateByPreset(candidates, model.AgentPresetCommandAgent)
	if !ok {
		return nil
	}
	allowedTools, ok := oneShotCommandToolsForIntent(text, pageContext, agent.AllowedTools)
	if !ok || len(allowedTools) == 0 {
		return nil
	}
	instructions := commandBarOneShotInstructions(text, pageContext, allowedTools)
	step := model.CommandBarPlanStep{
		AgentID:      agent.ID,
		AgentKey:     agent.PresetKey,
		AgentName:    firstNonEmptyString(strings.TrimSpace(agent.Name), "Command Agent"),
		PlanKind:     model.CommandBarPlanKindOneShotCommand,
		Target:       pageContext,
		Instructions: instructions,
		AllowedTools: allowedTools,
	}
	resp := commandBarMultiStepPlanResponse(
		[]model.CommandBarPlanStep{step},
		"Prepared a one-shot command agent because no narrower saved agent matched this request.",
		candidates,
	)
	resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
		Type:     "one_shot_command",
		Severity: "info",
		Message:  "This is a one-shot run. It is not saved as a reusable agent unless you promote it after completion.",
	})
	return resp
}

func parsePreferredOneShotCommandIntent(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	if !shouldPreferOneShotCommandIntent(text, pageContext) {
		return nil
	}
	return parseOneShotCommandIntent(text, pageContext, candidates)
}

func shouldPreferOneShotCommandIntent(text string, pageContext model.CommandBarPageContext) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	targetType := normalizeCommandBarTargetType(pageContext.EntityType)
	if targetType != "crm_contact" && targetType != "crm_deal" {
		return false
	}
	if !containsAny(lower, "contact", "company", "account", "crm") {
		return false
	}
	return containsAny(lower,
		"find info",
		"find information",
		"research",
		"enrich",
		"update contact",
		"update company",
		"update account",
		"refresh contact",
		"refresh company",
	)
}

func commandBarFanOutPlanResponse(agent model.CommandBarAgent, targets []model.CommandBarPageContext, text, rationale string, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	if len(targets) == 0 {
		return nil
	}
	steps := make([]model.CommandBarPlanStep, 0, min(len(targets), maxCommandBarPlanSteps))
	for i, target := range targets {
		if i >= maxCommandBarPlanSteps {
			break
		}
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:      agent.ID,
			AgentKey:     agent.PresetKey,
			AgentName:    agent.Name,
			PlanKind:     model.CommandBarPlanKindFanOut,
			Target:       target,
			Instructions: firstNonEmptyString(strings.TrimSpace(text), fmt.Sprintf("Run on %s.", target.DisplayTitle)),
		})
	}
	resp := commandBarMultiStepPlanResponse(steps, firstNonEmptyString(strings.TrimSpace(rationale), "Prepared a fan-out plan across selected targets."), candidates)
	resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
		Type:     "fan_out_confirmation",
		Severity: "warning",
		Message:  fmt.Sprintf("Fan-out will start %d independent runs at once. Review the target list before confirming.", len(resp.Plan.Steps)),
	})
	if len(targets) > maxCommandBarPlanSteps {
		resp.Plan.Guardrails = append(resp.Plan.Guardrails, model.CommandBarGuardrail{
			Type:     "fan_out_target_limit",
			Severity: "warning",
			Message:  fmt.Sprintf("Fan-out is limited to the first %d targets.", maxCommandBarPlanSteps),
		})
	}
	return resp
}

func parseFanOutIntent(text string, pageContext model.CommandBarPageContext, narrowCandidates, allCandidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	targets := commandBarFanOutTargetsFromContext(text, pageContext)
	if len(targets) == 0 {
		return nil
	}
	agent, ok := commandBarFanOutAgentForText(text, pageContext, narrowCandidates)
	if !ok {
		return nil
	}
	return commandBarFanOutPlanResponse(agent, targets, text, "Prepared a fan-out plan across concrete related targets.", allCandidates)
}

func commandBarFanOutAgentForText(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) (model.CommandBarAgent, bool) {
	if len(candidates) == 0 {
		return model.CommandBarAgent{}, false
	}
	for _, candidate := range candidates {
		if indexCommandBarPhrase(text, candidate.Name) >= 0 {
			return candidate, true
		}
		if candidate.PresetKey != "" && indexCommandBarPhrase(text, strings.ReplaceAll(candidate.PresetKey, "_", " ")) >= 0 {
			return candidate, true
		}
	}
	if parsed := parseIntentDeterministically(text, pageContext, candidates); parsed != nil && parsed.Plan != nil && len(parsed.Plan.Steps) > 0 {
		return findCommandBarCandidateByID(candidates, parsed.Plan.Steps[0].AgentID)
	}
	return model.CommandBarAgent{}, false
}

func commandBarFanOutTargetsFromContext(text string, pageContext model.CommandBarPageContext) []model.CommandBarPageContext {
	lower := strings.ToLower(strings.TrimSpace(text))
	if !containsAny(lower, "all", "each", "every", "across", "fan out", "fan-out", "multiple") {
		return nil
	}
	if len(pageContext.RelatedIDs) == 0 {
		return nil
	}
	type candidate struct {
		keys       []string
		targetType string
		label      string
	}
	candidates := []candidate{
		{keys: []string{"task_ids", "story_ids", "stories", "tasks"}, targetType: "task", label: "Task"},
		{keys: []string{"document_ids", "doc_ids", "documents", "docs"}, targetType: "document", label: "Document"},
		{keys: []string{"crm_deal_ids", "deal_ids", "deals"}, targetType: "crm_deal", label: "Deal"},
		{keys: []string{"crm_contact_ids", "contact_ids", "contacts"}, targetType: "crm_contact", label: "Contact"},
	}
	for _, item := range candidates {
		ids := []string{}
		for _, key := range item.keys {
			ids = append(ids, pageContext.RelatedIDs[key]...)
		}
		ids = normalizeStringSlice(ids)
		if len(ids) == 0 {
			continue
		}
		if !fanOutTextMentionsTarget(lower, item.targetType) {
			continue
		}
		targets := make([]model.CommandBarPageContext, 0, len(ids))
		for _, id := range ids {
			targets = append(targets, model.CommandBarPageContext{
				EntityType:   item.targetType,
				EntityID:     id,
				DisplayTitle: fmt.Sprintf("%s %s", item.label, shortCommandBarID(id)),
			})
		}
		return targets
	}
	return nil
}

func fanOutTextMentionsTarget(text, targetType string) bool {
	switch targetType {
	case "task":
		return containsAny(text, "task", "tasks", "story", "stories", "child")
	case "document":
		return containsAny(text, "doc", "docs", "document", "documents", "article", "articles")
	case "crm_deal":
		return containsAny(text, "deal", "deals")
	case "crm_contact":
		return containsAny(text, "contact", "contacts")
	default:
		return false
	}
}

func shortCommandBarID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func oneShotCommandToolsForIntent(text string, pageContext model.CommandBarPageContext, agentTools []string) ([]string, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" || containsAny(lower, "delete workspace", "delete all", "remove workspace") {
		return nil, false
	}
	tools := []string{"update_plan", "request_user_input"}
	recognized := false
	targetType := normalizeCommandBarTargetType(pageContext.EntityType)

	if targetType == "document" || containsAny(lower, "doc", "document", "article", "knowledge base", "stale") {
		recognized = true
		tools = append(tools, "list_documents", "list_collections", "read_document", "search_documents")
	}
	if containsAny(lower, "web", "website", "url", "internet", "research", "source", "sources", "stale", "latest", "fetch", "crawl", "find info", "find information", "enrich") {
		recognized = true
		tools = append(tools, "web_search_exa", "web_search_brave", "fetch_url", "crawl_url")
	}
	if containsAny(lower, "update doc", "update document", "refresh doc", "refresh document", "rewrite", "edit doc", "edit document", "write doc", "write document", "stale") {
		recognized = true
		tools = append(tools, "request_approval", "write_document_content")
	}
	if containsAny(lower, "create doc", "create document", "new doc", "new document", "draft doc", "draft document", "write a doc", "write an article") {
		recognized = true
		tools = append(tools, "create_document")
	}
	if targetType == "task" || containsAny(lower, "task", "story", "comment") {
		tools = append(tools, "get_task_context")
	}
	if containsAny(lower, "comment", "add note", "write note") && (targetType == "task" || containsAny(lower, "task", "story")) {
		recognized = true
		tools = append(tools, "add_task_comment")
	}
	if containsAny(lower, "create task", "create tasks", "new task", "new tasks", "create story", "new story", "make a task", "turn this into tasks", "follow-up task", "follow up task") {
		recognized = true
		tools = append(tools, "list_workspace_teams", "list_team_workflows_with_stages", "create_task")
	}
	if targetType == "crm_contact" || targetType == "crm_deal" || containsAny(lower, "crm", "deal", "contact", "buyer", "pipeline") {
		recognized = true
		tools = append(tools, "list_deals", "list_contacts", "list_buyer_signals")
	}
	if (targetType == "crm_contact" || targetType == "crm_deal") && containsAny(lower, "update", "refresh", "enrich", "find info", "find information", "research") {
		recognized = true
		tools = append(tools, "request_approval")
		if targetType == "crm_contact" || containsAny(lower, "contact") {
			tools = append(tools, "enrich_crm_contact")
		}
		if containsAny(lower, "company", "account") {
			tools = append(tools, "enrich_crm_company")
		}
	}
	if containsAny(lower, "deal note", "crm note", "add note to deal") {
		recognized = true
		tools = append(tools, "add_deal_note")
	}
	if containsAny(lower, "update deal", "move deal", "change stage") {
		recognized = true
		tools = append(tools, "request_approval", "update_deal_stage")
	}
	if containsAny(lower, "summarize", "explain", "answer", "compare", "analyze", "find", "check") {
		recognized = true
	}
	if !recognized {
		return nil, false
	}
	allowedSet := make(map[string]bool, len(agentTools))
	for _, tool := range agentTools {
		allowedSet[strings.TrimSpace(tool)] = true
	}
	filtered := make([]string, 0, len(tools))
	seen := map[string]bool{}
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" || seen[tool] {
			continue
		}
		if len(allowedSet) > 0 && !allowedSet[tool] {
			continue
		}
		seen[tool] = true
		filtered = append(filtered, tool)
	}
	return filtered, len(filtered) > 0
}

func commandBarOneShotInstructions(text string, pageContext model.CommandBarPageContext, tools []string) string {
	goal, plan, constraints := oneShotExecutionBrief(text, pageContext, tools)
	parts := []string{"One-shot execution brief"}
	parts = append(parts, "Goal:\n- "+goal)
	if len(plan) > 0 {
		lines := make([]string, 0, len(plan))
		for i, step := range plan {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, step))
		}
		parts = append(parts, "Plan:\n"+strings.Join(lines, "\n"))
	}
	constraints = append([]string{
		"Run as a one-shot command agent for the current target.",
		"Do not create or save a reusable agent.",
		"Use only the enabled tools for this run.",
	}, constraints...)
	if hasAnyTool(tools, "write_document_content", "create_document", "create_task", "add_task_comment", "add_deal_note", "update_deal_stage", "enrich_crm_contact", "enrich_crm_company") {
		constraints = append(constraints, "The user confirmed this command-bar plan; keep mutations limited to the requested action and target.")
	}
	if len(constraints) > 0 {
		lines := make([]string, 0, len(constraints))
		for _, constraint := range constraints {
			lines = append(lines, "- "+constraint)
		}
		parts = append(parts, "Constraints:\n"+strings.Join(lines, "\n"))
	}
	if pageContext.EntityType != "" || pageContext.EntityID != "" {
		parts = append(parts, fmt.Sprintf("Target: %s %s (%s).", pageContext.EntityType, pageContext.EntityID, pageContext.DisplayTitle))
	}
	parts = append(parts, "User request:\n"+strings.TrimSpace(text))
	return strings.Join(parts, "\n\n")
}

func oneShotExecutionBrief(text string, pageContext model.CommandBarPageContext, tools []string) (string, []string, []string) {
	_ = tools
	lower := strings.ToLower(strings.TrimSpace(text))
	targetType := normalizeCommandBarTargetType(pageContext.EntityType)
	switch {
	case targetType == "crm_contact" || targetType == "crm_deal":
		goal := "Research the CRM target and produce high-confidence CRM updates for the requested contact, company, or deal context."
		plan := []string{
			"Review the current CRM target and related CRM context available through the enabled tools.",
			"Search the web for public contact, company, role, domain, and buyer-signal evidence.",
			"Fetch authoritative sources before relying on search snippets.",
			"Extract proposed CRM updates with source URLs and confidence notes.",
			"Apply only CRM mutations supported by the enabled tools; otherwise return exact proposed field changes for review.",
		}
		constraints := []string{
			"Do not invent contact, company, title, domain, funding, or employment facts.",
			"Use guarded CRM enrichment tools for CRM writes; names, existing email, existing phone, company name, and existing domain are protected server-side.",
			"Ask for approval before any high-impact CRM mutation.",
		}
		return goal, plan, constraints
	case targetType == "document" || containsAny(lower, "doc", "document", "article", "stale"):
		goal := "Research and update the document only where the requested change is supported by the current document context and sources."
		plan := []string{
			"Read the current document and identify the sections relevant to the request.",
			"Use web or document search tools only where more evidence is needed.",
			"Fetch source pages before treating web results as facts.",
			"Draft the smallest safe content change that satisfies the request.",
			"Write the document only if the enabled tools support it; otherwise return the proposed patch.",
		}
		constraints := []string{
			"Preserve the document's existing structure and tone unless the user requested a rewrite.",
			"Do not replace sourced content with weaker evidence.",
			"Ask for approval before broad rewrites or uncertain factual changes.",
		}
		return goal, plan, constraints
	case targetType == "task" || containsAny(lower, "task", "story", "comment"):
		goal := "Complete the requested task-level action using the current task context and the enabled tools."
		plan := []string{
			"Review the current task context and identify the exact requested output.",
			"Gather any missing workspace/team context needed for the action.",
			"Create tasks or add comments only when the request is explicit and the enabled tools support it.",
			"Summarize what changed and any follow-up needed.",
		}
		constraints := []string{
			"Keep mutations limited to the current task or clearly requested workspace target.",
			"Do not create duplicate tasks when an existing task should be updated or referenced.",
		}
		return goal, plan, constraints
	default:
		goal := "Complete the confirmed one-shot command for the current target."
		plan := []string{
			"Review the provided target context and the user's request.",
			"Use the enabled tools to gather only the context needed for this command.",
			"Perform supported mutations carefully, or return proposed changes when a write tool is unavailable.",
			"Summarize the result and any sources or follow-up actions.",
		}
		constraints := []string{
			"Keep the run scoped to the confirmed command and target.",
			"Ask for clarification or approval when the request is ambiguous or risky.",
		}
		return goal, plan, constraints
	}
}

func hasAnyTool(tools []string, needles ...string) bool {
	for _, tool := range tools {
		if slices.Contains(needles, tool) {
			return true
		}
	}
	return false
}

func commandBarPlanKindForSteps(steps []model.CommandBarPlanStep) string {
	if len(steps) == 0 {
		return model.CommandBarPlanKindKnownAgent
	}
	allFanOut := true
	for _, step := range steps {
		if step.PlanKind != model.CommandBarPlanKindFanOut {
			allFanOut = false
			break
		}
	}
	if allFanOut {
		return model.CommandBarPlanKindFanOut
	}
	for _, step := range steps {
		if step.PlanKind != model.CommandBarPlanKindOneShotCommand {
			return model.CommandBarPlanKindKnownAgent
		}
	}
	return model.CommandBarPlanKindOneShotCommand
}

func isOneShotCommandAgent(agent model.CommandBarAgent) bool {
	return normalizePresetKey(agent.PresetKey) == model.AgentPresetCommandAgent
}

func parseExplicitNamedAgents(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	type match struct {
		index int
		agent model.CommandBarAgent
	}
	matches := make([]match, 0)
	for _, agent := range candidates {
		if idx := indexCommandBarPhrase(text, agent.Name); idx >= 0 {
			matches = append(matches, match{index: idx, agent: agent})
			continue
		}
		if agent.PresetKey != "" {
			presetPhrase := strings.ReplaceAll(strings.ToLower(agent.PresetKey), "_", " ")
			if idx := indexCommandBarPhrase(text, presetPhrase); idx >= 0 {
				matches = append(matches, match{index: idx, agent: agent})
			}
		}
	}
	if len(matches) == 0 {
		return nil
	}
	slices.SortFunc(matches, func(a, b match) int {
		if a.index < b.index {
			return -1
		}
		if a.index > b.index {
			return 1
		}
		return strings.Compare(a.agent.Name, b.agent.Name)
	})

	seen := map[string]bool{}
	agents := make([]model.CommandBarAgent, 0, len(matches))
	for _, item := range matches {
		if seen[item.agent.ID] {
			continue
		}
		seen[item.agent.ID] = true
		agents = append(agents, item.agent)
	}
	if len(agents) == 0 {
		return nil
	}
	if len(agents) == 1 && isOneShotCommandAgent(agents[0]) {
		return parseOneShotCommandIntent(text, pageContext, candidates)
	}
	agents = slices.DeleteFunc(agents, isOneShotCommandAgent)
	if len(agents) == 0 {
		return nil
	}
	steps := make([]model.CommandBarPlanStep, 0, len(agents))
	for i, agent := range agents {
		steps = append(steps, model.CommandBarPlanStep{
			AgentID:      agent.ID,
			AgentKey:     agent.PresetKey,
			AgentName:    agent.Name,
			Target:       pageContext,
			Instructions: commandBarNamedAgentStepInstructions(agent, agents, i),
		})
	}
	rationale := "Matched explicit agent name."
	if len(steps) > 1 {
		rationale = "Matched explicit agent names in request order."
	}
	return commandBarMultiStepPlanResponse(steps, rationale, candidates)
}

func commandBarNamedAgentStepInstructions(agent model.CommandBarAgent, agents []model.CommandBarAgent, stepIndex int) string {
	name := strings.TrimSpace(agent.Name)
	if name == "" {
		name = "this agent"
	}
	parts := []string{
		fmt.Sprintf("Execute your normal %s role for the current target.", name),
	}
	if len(agents) > 1 {
		parts = append(parts, fmt.Sprintf("This is command-bar step %d of %d.", stepIndex+1, len(agents)))
		otherNames := make([]string, 0, len(agents)-1)
		for i, other := range agents {
			if i == stepIndex {
				continue
			}
			if otherName := strings.TrimSpace(other.Name); otherName != "" {
				otherNames = append(otherNames, otherName)
			}
		}
		if len(otherNames) > 0 {
			parts = append(parts, fmt.Sprintf("Do not invoke or run %s yourself; Helpin schedules those as separate command-bar steps.", strings.Join(otherNames, ", ")))
		}
		if stepIndex > 0 {
			parts = append(parts, "Use the previous linked run as prior-step context when it is available.")
		}
	}
	return strings.Join(parts, " ")
}

func indexCommandBarPhrase(text, phrase string) int {
	text = strings.ToLower(strings.TrimSpace(text))
	phrase = strings.ToLower(strings.TrimSpace(phrase))
	if text == "" || phrase == "" {
		return -1
	}
	offset := 0
	for offset < len(text) {
		idx := strings.Index(text[offset:], phrase)
		if idx < 0 {
			return -1
		}
		idx += offset
		beforeOK := idx == 0 || !isCommandBarWordByte(text[idx-1])
		after := idx + len(phrase)
		afterOK := after == len(text) || !isCommandBarWordByte(text[after])
		if beforeOK && afterOK {
			return idx
		}
		offset = idx + 1
	}
	return -1
}

func isCommandBarWordByte(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')
}

func normalizeCommandBarPageContext(ctx model.CommandBarPageContext, workspaceID string) model.CommandBarPageContext {
	ctx.EntityType = normalizeCommandBarTargetType(ctx.EntityType)
	ctx.EntityID = strings.TrimSpace(ctx.EntityID)
	ctx.DisplayTitle = strings.TrimSpace(ctx.DisplayTitle)
	if ctx.EntityType == "" {
		ctx.EntityType = "workspace"
	}
	if ctx.EntityType == "workspace" && ctx.EntityID == "" {
		ctx.EntityID = strings.TrimSpace(workspaceID)
	}
	if ctx.DisplayTitle == "" {
		ctx.DisplayTitle = ctx.EntityType
	}
	return ctx
}

func normalizeCommandBarPlanSteps(steps []model.CommandBarPlanStep, fallbackTarget model.CommandBarPageContext) []model.CommandBarPlanStep {
	normalized := make([]model.CommandBarPlanStep, 0, len(steps))
	for _, step := range steps {
		target := step.Target
		if strings.TrimSpace(target.EntityType) == "" || strings.TrimSpace(target.EntityID) == "" {
			target = fallbackTarget
		}
		target.EntityType = normalizeCommandBarTargetType(target.EntityType)
		target.EntityID = strings.TrimSpace(target.EntityID)
		target.DisplayTitle = strings.TrimSpace(target.DisplayTitle)
		if target.DisplayTitle == "" {
			target.DisplayTitle = fallbackTarget.DisplayTitle
		}
		step.AgentID = strings.TrimSpace(step.AgentID)
		step.AgentKey = normalizePresetKey(step.AgentKey)
		step.AgentName = strings.TrimSpace(step.AgentName)
		step.PlanKind = normalizeCommandBarPlanKind(step.PlanKind)
		step.Target = target
		step.Instructions = strings.TrimSpace(step.Instructions)
		normalized = append(normalized, step)
	}
	return normalized
}

func normalizeCommandBarPlanKind(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case model.CommandBarPlanKindOneShotCommand, "one_shot", "one-shot", "one-shot-command":
		return model.CommandBarPlanKindOneShotCommand
	case model.CommandBarPlanKindFanOut, "fanout", "fan-out":
		return model.CommandBarPlanKindFanOut
	default:
		return ""
	}
}

func normalizeCommandBarTargetType(targetType string) string {
	targetType = strings.TrimSpace(strings.ToLower(targetType))
	switch targetType {
	case "story":
		return "task"
	case "deal":
		return "crm_deal"
	case "contact":
		return "crm_contact"
	case "doc":
		return "document"
	default:
		return targetType
	}
}

func validateCommandBarSupportedTarget(targetType string) error {
	switch normalizeCommandBarTargetType(targetType) {
	case "task", "epic", "workspace", "document", "crm_contact", "crm_deal":
		return nil
	default:
		return fmt.Errorf("command bar v1 does not support %s targets yet", targetType)
	}
}

func buildCommandBarTriggerContext(text string, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep, stepIndex int, planID string) (*model.AgentRunTriggerContext, error) {
	now := time.Now().UTC()
	raw, err := json.Marshal(commandBarTriggerContextPayload{
		PlanID:      strings.TrimSpace(planID),
		Prompt:      strings.TrimSpace(text),
		PageContext: pageContext,
		Steps:       steps,
		RunCount:    len(steps),
		StepIndex:   stepIndex,
	})
	if err != nil {
		return nil, err
	}
	return &model.AgentRunTriggerContext{
		Source:      model.AgentRunTriggerSourceCommandBar,
		TriggerType: model.AgentRunTriggerTypeCommandBar,
		FiredAt:     &now,
		Context:     raw,
	}, nil
}

func commandBarAdditionalContext(instructions string, pageContext model.CommandBarPageContext, stepIndex, stepCount int) string {
	instructions = strings.TrimSpace(instructions)
	parts := []string{
		"This run was started from the workspace command bar.",
		fmt.Sprintf("Command bar plan step: %d of %d.", stepIndex+1, stepCount),
	}
	if instructions != "" {
		parts = append(parts, "Step instruction:\n"+instructions)
	}
	if stepCount > 1 {
		parts = append(parts, "Other command-bar plan steps are scheduled separately by Helpin. Do not invoke those agents yourself.")
	}
	if pageContext.EntityType != "" || pageContext.EntityID != "" {
		parts = append(parts, fmt.Sprintf("Command bar page context: %s %s (%s).", pageContext.EntityType, pageContext.EntityID, pageContext.DisplayTitle))
	}
	return strings.Join(parts, "\n\n")
}

func findCommandBarCandidateByID(candidates []model.CommandBarAgent, id string) (model.CommandBarAgent, bool) {
	id = strings.TrimSpace(id)
	for _, candidate := range candidates {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return model.CommandBarAgent{}, false
}

func findCommandBarCandidateByPreset(candidates []model.CommandBarAgent, preset string) (model.CommandBarAgent, bool) {
	preset = normalizePresetKey(preset)
	for _, candidate := range candidates {
		if normalizePresetKey(candidate.PresetKey) == preset {
			return candidate, true
		}
	}
	return model.CommandBarAgent{}, false
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func defaultCommandBarSuggestions(targetType string) []string {
	switch normalizeCommandBarTargetType(targetType) {
	case "task":
		return []string{"Ask Code Builder to implement this task", "Ask Review Agent to review this task", "Ask Task Planner to refine this task"}
	case "epic":
		return []string{"Ask Epic Planner to break this epic into tasks", "Ask Review Agent to review this epic"}
	default:
		return []string{"Try a request that names an existing agent", "Open an epic or task and try again with page context"}
	}
}
