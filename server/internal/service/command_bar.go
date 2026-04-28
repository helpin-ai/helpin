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
	agentService *AgentService
	planRepo     *repository.CommandBarPlanRepository
	unmetRepo    *repository.CommandBarUnmetIntentRepository
	llmProvider  llm.Provider
}

func NewCommandBarService(agentService *AgentService, planRepo *repository.CommandBarPlanRepository, unmetRepo *repository.CommandBarUnmetIntentRepository, llmProvider llm.Provider) *CommandBarService {
	return &CommandBarService{
		agentService: agentService,
		planRepo:     planRepo,
		unmetRepo:    unmetRepo,
		llmProvider:  llmProvider,
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

	if parsed := parseExplicitNamedAgents(text, pageContext, candidates); parsed != nil {
		return parsed, nil
	}
	if parsed := parseIntentDeterministically(text, pageContext, candidates); parsed != nil {
		return parsed, nil
	}
	if parsed := s.parseIntentWithLLM(ctx, text, pageContext, candidates); parsed != nil {
		if parsed.Status == model.CommandBarParseStatusNoMatchingAgent {
			_ = s.logUnmetIntent(ctx, workspaceID, actorID, text, pageContext, candidates, parsed.Reason)
		}
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

func (s *CommandBarService) ListPlans(ctx context.Context, workspaceID, actorID string, limit int) (*model.CommandBarPlanListResponse, error) {
	if s == nil || s.planRepo == nil {
		return &model.CommandBarPlanListResponse{Plans: []model.CommandBarPlanSummary{}}, nil
	}
	records, err := s.planRepo.ListRecent(ctx, workspaceID, strings.TrimSpace(actorID), limit)
	if err != nil {
		return nil, err
	}
	summaries := make([]model.CommandBarPlanSummary, 0, len(records))
	for _, record := range records {
		runIDsByStep := decodeCommandBarPlanRunIDs(record.RunIDsByStep)
		runIDs := make([]string, 0, len(runIDsByStep))
		for _, runID := range runIDsByStep {
			runIDs = append(runIDs, runID)
		}
		runs, err := s.agentService.runRepo.ListByIDs(ctx, workspaceID, runIDs)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, commandBarPlanSummary(record, runs))
	}
	return &model.CommandBarPlanListResponse{Plans: summaries}, nil
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
		Run:  *run,
	}, nil
}

func (s *CommandBarService) ListUnmetIntents(ctx context.Context, workspaceID, status string, limit int) (*model.CommandBarUnmetIntentListResponse, error) {
	if s == nil || s.unmetRepo == nil {
		return &model.CommandBarUnmetIntentListResponse{Intents: []model.CommandBarUnmetIntent{}}, nil
	}
	intents, err := s.unmetRepo.List(ctx, workspaceID, strings.TrimSpace(status), limit)
	if err != nil {
		return nil, err
	}
	return &model.CommandBarUnmetIntentListResponse{Intents: intents}, nil
}

func (s *CommandBarService) ReviewUnmetIntent(ctx context.Context, workspaceID, id string, req model.ReviewCommandBarUnmetIntentRequest) (*model.CommandBarUnmetIntent, error) {
	if s == nil || s.unmetRepo == nil {
		return nil, fmt.Errorf("command bar unmet intent repository is not configured")
	}
	status := strings.TrimSpace(req.Status)
	switch status {
	case "open", "accepted", "rejected", "deferred":
	default:
		return nil, fmt.Errorf("unsupported unmet intent status %q", status)
	}
	return s.unmetRepo.Review(ctx, workspaceID, strings.TrimSpace(id), status, req.Notes)
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
	allowedTargets := []string{strings.TrimSpace(run.TargetType)}
	role := strings.TrimSpace(derefString(req.Description))
	if role == "" {
		role = fmt.Sprintf("Reusable agent promoted from command-bar run %s. Original step instruction: %s", run.ID, strings.TrimSpace(step.Instructions))
	}
	agent, err := s.agentService.CreateAgent(ctx, model.CreateAgentRequest{
		WorkspaceID:           workspaceID,
		Name:                  name,
		Role:                  role,
		RuntimeKind:           &sourceAgent.RuntimeKind,
		Provider:              sourceAgent.Provider,
		Model:                 sourceAgent.Model,
		ExecutionConfig:       json.RawMessage(sourceAgent.ExecutionConfig),
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
	guardrails := []model.CommandBarGuardrail{}
	if len(steps) > maxCommandBarPlanSteps {
		steps = steps[:maxCommandBarPlanSteps]
		guardrails = append(guardrails, model.CommandBarGuardrail{
			Type:     "run_count_limit",
			Severity: "warning",
			Message:  fmt.Sprintf("Plan was limited to %d steps.", maxCommandBarPlanSteps),
		})
	}
	return &model.CommandBarParseResponse{
		Status: model.CommandBarParseStatusPlan,
		Plan: &model.CommandBarPlan{
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
		step.Target = target
		step.Instructions = strings.TrimSpace(step.Instructions)
		normalized = append(normalized, step)
	}
	return normalized
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
