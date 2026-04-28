package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type CommandBarService struct {
	agentService *AgentService
	unmetRepo    *repository.CommandBarUnmetIntentRepository
	llmProvider  llm.Provider
}

func NewCommandBarService(agentService *AgentService, unmetRepo *repository.CommandBarUnmetIntentRepository, llmProvider llm.Provider) *CommandBarService {
	return &CommandBarService{
		agentService: agentService,
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

	if parsed := s.parseIntentWithLLM(ctx, text, pageContext, candidates); parsed != nil {
		if parsed.Status == model.CommandBarParseStatusNoMatchingAgent {
			_ = s.logUnmetIntent(ctx, workspaceID, actorID, text, pageContext, candidates, parsed.Reason)
		}
		return parsed, nil
	}
	if parsed := parseIntentDeterministically(text, pageContext, candidates); parsed != nil {
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
	if len(req.Steps) != 1 {
		return nil, fmt.Errorf("v1 command bar dispatch requires exactly one step")
	}

	step := req.Steps[0]
	target := normalizeCommandBarPageContext(step.Target, workspaceID)
	if target.EntityType == "" || target.EntityID == "" {
		target = pageContext
	}
	if err := validateCommandBarSupportedTarget(target.EntityType); err != nil {
		return nil, err
	}
	agentID := strings.TrimSpace(step.AgentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	triggerContext, err := buildCommandBarTriggerContext(text, pageContext, []model.CommandBarPlanStep{step})
	if err != nil {
		return nil, err
	}
	additionalContext := commandBarAdditionalContext(text, strings.TrimSpace(step.Instructions), pageContext)

	run, err := s.agentService.startTargetRun(ctx, workspaceID, target.EntityType, target.EntityID, model.StartAgentRunRequest{
		AgentID:           agentID,
		AdditionalContext: &additionalContext,
	}, &actorID, triggerContext, nil, nil)
	if err != nil {
		return nil, err
	}
	return &model.CommandBarDispatchResponse{Runs: []model.AgentRun{*run}}, nil
}

func (s *CommandBarService) parseIntentWithLLM(ctx context.Context, text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	if s == nil || s.llmProvider == nil {
		return nil
	}
	candidateJSON, _ := json.Marshal(candidates)
	contextJSON, _ := json.Marshal(pageContext)
	resp, err := s.llmProvider.ChatCompletion(ctx, llm.ChatRequest{
		SystemPrompt: `You classify command-bar requests for Helpin agent runs. Return strict JSON only. Choose an agent only when the request clearly fits one available candidate and target. If it does not fit, return no_matching_agent with a short reason. V1 supports exactly one step.`,
		Messages: []llm.Message{{
			Role: "user",
			Content: fmt.Sprintf(`Request: %s
Page context: %s
Available agents: %s

Return JSON: {"status":"plan","agent_id":"...","instructions":"...","rationale":"..."} or {"status":"no_matching_agent","reason":"..."}.`, text, string(contextJSON), string(candidateJSON)),
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
	agent, ok := findCommandBarCandidateByID(candidates, parsed.AgentID)
	if !ok {
		return nil
	}
	return commandBarPlanResponse(agent, pageContext, text, parsed.Instructions, parsed.Rationale, candidates)
}

func parseIntentDeterministically(text string, pageContext model.CommandBarPageContext, candidates []model.CommandBarAgent) *model.CommandBarParseResponse {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, agent := range candidates {
		if lower == strings.ToLower(agent.Name) || strings.Contains(lower, strings.ToLower(agent.Name)) {
			return commandBarPlanResponse(agent, pageContext, text, text, "Matched explicit agent name.", candidates)
		}
		if agent.PresetKey != "" && strings.Contains(lower, strings.ReplaceAll(strings.ToLower(agent.PresetKey), "_", " ")) {
			return commandBarPlanResponse(agent, pageContext, text, text, "Matched explicit agent preset.", candidates)
		}
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
	return &model.CommandBarParseResponse{
		Status: model.CommandBarParseStatusPlan,
		Plan: &model.CommandBarPlan{
			Steps:    []model.CommandBarPlanStep{step},
			RunCount: 1,
		},
		Rationale:  firstNonEmptyString(strings.TrimSpace(rationale), "Matched request to an available agent."),
		Candidates: candidates,
	}
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

func normalizeCommandBarTargetType(targetType string) string {
	targetType = strings.TrimSpace(strings.ToLower(targetType))
	switch targetType {
	case "story":
		return "task"
	case "deal":
		return "crm_deal"
	case "doc":
		return "document"
	default:
		return targetType
	}
}

func validateCommandBarSupportedTarget(targetType string) error {
	switch normalizeCommandBarTargetType(targetType) {
	case "task", "epic", "workspace":
		return nil
	default:
		return fmt.Errorf("command bar v1 does not support %s targets yet", targetType)
	}
}

func buildCommandBarTriggerContext(text string, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep) (*model.AgentRunTriggerContext, error) {
	now := time.Now().UTC()
	raw, err := json.Marshal(map[string]interface{}{
		"prompt":       strings.TrimSpace(text),
		"page_context": pageContext,
		"steps":        steps,
		"run_count":    len(steps),
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

func commandBarAdditionalContext(text, instructions string, pageContext model.CommandBarPageContext) string {
	parts := []string{
		"This run was started from the workspace command bar.",
		"User request:\n" + strings.TrimSpace(text),
	}
	if instructions != "" && instructions != strings.TrimSpace(text) {
		parts = append(parts, "Parsed instructions:\n"+instructions)
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
