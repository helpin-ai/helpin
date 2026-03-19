package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

const defaultMaxChainDepth = 10

// Context key for passing RuleExecutionContext through MoveToState calls.
type ruleExecCtxKeyType struct{}

var ruleExecCtxKey = ruleExecCtxKeyType{}

func withRuleExecCtx(ctx context.Context, execCtx *model.RuleExecutionContext) context.Context {
	return context.WithValue(ctx, ruleExecCtxKey, execCtx)
}

func ruleExecCtxFromContext(ctx context.Context) *model.RuleExecutionContext {
	if v, ok := ctx.Value(ruleExecCtxKey).(*model.RuleExecutionContext); ok {
		return v
	}
	return nil
}

// AutomationRuleEngine evaluates automation rules against events and executes actions.
type AutomationRuleEngine struct {
	ruleRepo        *repository.AutomationRuleRepository
	storyRepo       *repository.PMStoryRepository
	workflowRepo    *repository.PMWorkflowRepository
	deliveryRepo    *repository.StoryDeliveryTargetRepository
	agentService    *AgentService
	storyService    *PMStoryService
	flowService     *FlowService
	gitService      *GitService
	commandService  *InternalCommandService
	notificationSvc *NotificationService
	activitySvc     *PMActivityService
	wsPublisher     *websocket.Publisher
	healthObserver  AutomationHealthObserver
	logger          *slog.Logger
}

// NewAutomationRuleEngine creates a new AutomationRuleEngine.
func NewAutomationRuleEngine(
	ruleRepo *repository.AutomationRuleRepository,
	storyRepo *repository.PMStoryRepository,
	workflowRepo *repository.PMWorkflowRepository,
	deliveryRepo *repository.StoryDeliveryTargetRepository,
	gitService *GitService,
	notificationSvc *NotificationService,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
) *AutomationRuleEngine {
	return &AutomationRuleEngine{
		ruleRepo:        ruleRepo,
		storyRepo:       storyRepo,
		workflowRepo:    workflowRepo,
		deliveryRepo:    deliveryRepo,
		gitService:      gitService,
		notificationSvc: notificationSvc,
		activitySvc:     activitySvc,
		wsPublisher:     wsPublisher,
		logger:          slog.Default().With("service", "automation_rule_engine"),
	}
}

// SetAgentService sets the agent service (breaks circular dependency).
func (e *AutomationRuleEngine) SetAgentService(svc *AgentService) *AutomationRuleEngine {
	e.agentService = svc
	return e
}

// SetStoryService sets the story service (breaks circular dependency).
func (e *AutomationRuleEngine) SetStoryService(svc *PMStoryService) *AutomationRuleEngine {
	e.storyService = svc
	return e
}

// SetHealthObserver sets the health observer for recording rule execution results.
func (e *AutomationRuleEngine) SetHealthObserver(obs AutomationHealthObserver) *AutomationRuleEngine {
	e.healthObserver = obs
	return e
}

// SetCommandService sets the internal command service for run_command actions.
func (e *AutomationRuleEngine) SetCommandService(svc *InternalCommandService) *AutomationRuleEngine {
	e.commandService = svc
	return e
}

// SetFlowService sets the flow service for start_flow actions.
func (e *AutomationRuleEngine) SetFlowService(svc *FlowService) *AutomationRuleEngine {
	e.flowService = svc
	return e
}

// EvaluateEvent finds matching rules for an event and executes their actions.
func (e *AutomationRuleEngine) EvaluateEvent(ctx context.Context, event model.AutomationEvent, execCtx *model.RuleExecutionContext) {
	if e == nil {
		return
	}

	if execCtx == nil {
		execCtx = &model.RuleExecutionContext{
			Depth:    0,
			MaxDepth: defaultMaxChainDepth,
		}
	}

	if execCtx.Depth >= execCtx.MaxDepth {
		e.logger.ErrorContext(ctx, "automation rule chain depth exceeded",
			"max_depth", execCtx.MaxDepth,
			"workspace_id", event.WorkspaceID,
			"story_id", event.StoryID,
			"trigger_type", event.TriggerType,
		)
		return
	}

	rules, err := e.ruleRepo.ListMatchingRules(ctx, event.WorkspaceID, event.TriggerType)
	if err != nil {
		e.logger.ErrorContext(ctx, "failed to list matching rules",
			"error", err,
			"workspace_id", event.WorkspaceID,
			"trigger_type", event.TriggerType,
		)
		return
	}

	// Only load the story for story-based triggers.
	story, err := e.resolveStoryIfNeeded(ctx, event)
	if err != nil {
		e.logger.ErrorContext(ctx, "failed to load story for rule evaluation",
			"error", err,
			"story_id", event.StoryID,
		)
		return
	}

	for _, rule := range rules {
		if containsString(execCtx.FiredRuleIDs, rule.ID) {
			continue
		}

		if !e.matchesTriggerConfig(ctx, rule, event) {
			continue
		}

		if !e.matchesScope(rule, story, event) {
			continue
		}

		e.logger.InfoContext(ctx, "executing automation rule",
			"rule_id", rule.ID,
			"rule_name", rule.Name,
			"trigger_type", rule.TriggerType,
			"action_type", rule.ActionType,
			"story_id", event.StoryID,
			"depth", execCtx.Depth,
		)

		if err := e.executeAction(ctx, &rule, event, story, execCtx); err != nil {
			e.logger.ErrorContext(ctx, "automation rule action failed",
				"error", err,
				"rule_id", rule.ID,
				"rule_name", rule.Name,
				"action_type", rule.ActionType,
				"story_id", event.StoryID,
			)
			e.observeFailure(ctx, event.WorkspaceID, rule.ID, err)
		} else {
			e.observeSuccess(ctx, event.WorkspaceID, rule.ID)
		}

		if rule.StopOnMatch {
			break
		}
	}
}

// resolveStoryIfNeeded loads the story for story-based triggers, returns nil for cron triggers.
func (e *AutomationRuleEngine) resolveStoryIfNeeded(ctx context.Context, event model.AutomationEvent) (*model.PMStory, error) {
	switch event.TriggerType {
	case model.TriggerCron:
		return nil, nil
	default:
		if event.StoryID == "" {
			return nil, nil
		}
		story, err := e.storyRepo.GetRawByID(ctx, event.StoryID)
		if err != nil {
			return nil, err
		}
		if story == nil {
			return nil, fmt.Errorf("story %s not found", event.StoryID)
		}
		return story, nil
	}
}

func (e *AutomationRuleEngine) matchesTriggerConfig(ctx context.Context, rule model.AutomationRule, event model.AutomationEvent) bool {
	switch rule.TriggerType {
	case model.TriggerStoryStateEntered:
		var cfg model.TriggerConfigStateEntered
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err != nil {
			return false
		}
		// Match by exact state ID
		if cfg.StateID != "" {
			return cfg.StateID == event.StateID
		}
		// Match by state type (e.g. "started", "done") — used by epic automations
		if cfg.StateType != "" && event.StateID != "" {
			state, err := e.workflowRepo.GetStateByID(ctx, event.StateID)
			if err != nil || state == nil {
				return false
			}
			return state.StateType == cfg.StateType
		}
		return false

	case model.TriggerAgentRunApproved:
		var cfg model.TriggerConfigRunApproved
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err != nil {
			return false
		}
		return cfg.StateID != "" && cfg.StateID == event.StateID

	case model.TriggerCron:
		var cfg model.TriggerConfigCron
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err != nil {
			return false
		}
		// Cron rules always match when evaluated during their category's tick
		return cfg.Category != ""

	default:
		return false
	}
}

func (e *AutomationRuleEngine) matchesScope(rule model.AutomationRule, story *model.PMStory, event model.AutomationEvent) bool {
	if story != nil {
		if rule.WorkflowID != nil && *rule.WorkflowID != "" && *rule.WorkflowID != story.WorkflowID {
			return false
		}
		if rule.TeamID != nil && *rule.TeamID != "" {
			if story.TeamID == nil || *story.TeamID != *rule.TeamID {
				return false
			}
		}
		return true
	}
	// No story — match only on TeamID from event
	if rule.TeamID != nil && *rule.TeamID != "" {
		return event.TeamID != "" && *rule.TeamID == event.TeamID
	}
	return true
}

func (e *AutomationRuleEngine) executeAction(ctx context.Context, rule *model.AutomationRule, event model.AutomationEvent, story *model.PMStory, execCtx *model.RuleExecutionContext) error {
	switch rule.ActionType {
	case model.ActionRunAgent:
		return fmt.Errorf("run_agent action is deprecated — convert to start_flow with template pm.agent_story_run")

	case model.ActionMoveToState:
		var cfg model.ActionConfigMoveToState
		if err := json.Unmarshal(rule.ActionConfig, &cfg); err != nil {
			return fmt.Errorf("parse move_to_state config: %w", err)
		}
		return e.executeMoveToState(ctx, rule, event, story, cfg, execCtx)

	case model.ActionMergeBranch:
		var cfg model.ActionConfigMergeBranch
		if err := json.Unmarshal(rule.ActionConfig, &cfg); err != nil {
			return fmt.Errorf("parse merge_branch config: %w", err)
		}
		return e.executeMergeBranch(ctx, rule, event, story, cfg)

	case model.ActionRunCommand:
		var cfg model.ActionConfigRunCommand
		if err := json.Unmarshal(rule.ActionConfig, &cfg); err != nil {
			return fmt.Errorf("parse run_command config: %w", err)
		}
		return e.executeRunCommand(ctx, rule, event, story, cfg)

	case model.ActionStartFlow:
		var cfg model.ActionConfigStartFlow
		if err := json.Unmarshal(rule.ActionConfig, &cfg); err != nil {
			return fmt.Errorf("parse start_flow config: %w", err)
		}
		return e.executeStartFlow(ctx, rule, event, story, cfg)

	default:
		return fmt.Errorf("unknown action type: %s", rule.ActionType)
	}
}

// executeRunAgent was removed — all run_agent rules were migrated to start_flow
// in migration 059. Any remaining run_agent rules will error at runtime.

func (e *AutomationRuleEngine) executeMoveToState(ctx context.Context, rule *model.AutomationRule, event model.AutomationEvent, story *model.PMStory, cfg model.ActionConfigMoveToState, execCtx *model.RuleExecutionContext) error {
	if e.storyService == nil {
		return fmt.Errorf("story service not configured")
	}
	if cfg.TargetStateID == "" {
		return fmt.Errorf("target_state_id is required in move_to_state config")
	}

	// Guard: if story is already in the target state, no-op (prevents loops)
	if story.WorkflowStateID == cfg.TargetStateID {
		e.logger.InfoContext(ctx, "skipping move_to_state: story already in target state",
			"rule_id", rule.ID,
			"story_id", event.StoryID,
			"state_id", cfg.TargetStateID,
		)
		return nil
	}

	// Build the chain context for the resulting state_entered event.
	// We pass it via ctx so MoveToState's EvaluateEvent call uses it
	// (preserving chain depth and fired-rule dedup).
	newExecCtx := &model.RuleExecutionContext{
		OriginEventID: execCtx.OriginEventID,
		Depth:         execCtx.Depth + 1,
		MaxDepth:      execCtx.MaxDepth,
		FiredRuleIDs:  append(append([]string{}, execCtx.FiredRuleIDs...), rule.ID),
	}
	chainCtx := withRuleExecCtx(ctx, newExecCtx)

	// Move the story. MoveToState triggers EvaluateEvent (with chain context
	// from chainCtx) — no separate call needed here.
	_, err := e.storyService.MoveToState(chainCtx, event.StoryID, model.MoveStoryRequest{
		StateID: cfg.TargetStateID,
	}, "system")
	if err != nil {
		return fmt.Errorf("move story to state: %w", err)
	}

	_ = e.activitySvc.Log(ctx, event.WorkspaceID, "story", event.StoryID, nil,
		fmt.Sprintf("automation rule '%s' advanced story to next stage", rule.Name),
		nil, nil, nil, nil)

	return nil
}

func (e *AutomationRuleEngine) executeMergeBranch(ctx context.Context, rule *model.AutomationRule, event model.AutomationEvent, story *model.PMStory, cfg model.ActionConfigMergeBranch) error {
	if e.gitService == nil {
		return fmt.Errorf("git service not configured")
	}
	if strings.TrimSpace(cfg.TargetBranch) == "" {
		return fmt.Errorf("target_branch is required in merge_branch config")
	}

	// Load delivery target to get working branch
	target, err := e.deliveryRepo.GetByStory(ctx, event.WorkspaceID, event.StoryID)
	if err != nil {
		return fmt.Errorf("load delivery target: %w", err)
	}
	if target == nil || target.WorkingBranch == nil || *target.WorkingBranch == "" {
		e.logger.WarnContext(ctx, "skipping merge_branch: no working branch",
			"rule_id", rule.ID,
			"story_id", event.StoryID,
		)
		return nil
	}
	if target.RepoFullName == nil || *target.RepoFullName == "" {
		e.logger.WarnContext(ctx, "skipping merge_branch: no repository configured",
			"rule_id", rule.ID,
			"story_id", event.StoryID,
		)
		return nil
	}

	// Resolve {base_branch} variable
	resolvedBranch := cfg.TargetBranch
	if strings.Contains(resolvedBranch, "{base_branch}") {
		if target.BaseBranch != nil && *target.BaseBranch != "" {
			resolvedBranch = strings.ReplaceAll(resolvedBranch, "{base_branch}", *target.BaseBranch)
		} else {
			e.logger.WarnContext(ctx, "skipping merge_branch: {base_branch} used but story has no base branch",
				"rule_id", rule.ID,
				"story_id", event.StoryID,
			)
			return nil
		}
	}

	if err := e.gitService.MergeBranch(ctx, event.WorkspaceID, event.StoryID, resolvedBranch); err != nil {
		e.logger.ErrorContext(ctx, "merge_branch failed",
			"error", err,
			"rule_id", rule.ID,
			"story_id", event.StoryID,
			"working_branch", *target.WorkingBranch,
			"target_branch", resolvedBranch,
		)
		// Don't halt the pipeline for merge failures — log and continue
		return nil
	}

	_ = e.activitySvc.Log(ctx, event.WorkspaceID, "story", event.StoryID, nil,
		fmt.Sprintf("automation rule '%s' merged %s into %s", rule.Name, *target.WorkingBranch, resolvedBranch),
		nil, nil, nil, nil)

	return nil
}

func (e *AutomationRuleEngine) executeRunCommand(ctx context.Context, rule *model.AutomationRule, event model.AutomationEvent, story *model.PMStory, cfg model.ActionConfigRunCommand) error {
	if e.commandService == nil {
		return fmt.Errorf("command service not configured")
	}
	if cfg.CommandName == "" {
		return fmt.Errorf("command_name is required in run_command config")
	}

	targetType := event.TargetType
	targetID := event.TargetID
	if targetType == "" && story != nil {
		targetType = "story"
		targetID = story.ID
	}

	meta := model.InternalCommandContext{
		WorkspaceID: event.WorkspaceID,
		ActorID:     "system",
		TargetType:  targetType,
		TargetID:    targetID,
	}

	// Merge story context into input for epic commands: the command needs the
	// epic_id, which only the story carries. Inject it so the command can
	// resolve the correct entity.
	input := cfg.Input
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	if story != nil && story.EpicID != nil && *story.EpicID != "" {
		input = e.mergeEpicIDIntoInput(input, *story.EpicID)
	}

	_, err := e.commandService.Execute(ctx, meta, cfg.CommandName, input)
	if err != nil {
		return fmt.Errorf("run command %q: %w", cfg.CommandName, err)
	}
	return nil
}

// mergeEpicIDIntoInput injects epic_id into the input JSON if not already present.
func (e *AutomationRuleEngine) mergeEpicIDIntoInput(input json.RawMessage, epicID string) json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(input, &m); err != nil {
		return input
	}
	if _, exists := m["epic_id"]; exists {
		return input
	}
	m["epic_id"] = json.RawMessage(fmt.Sprintf("%q", epicID))
	merged, err := json.Marshal(m)
	if err != nil {
		return input
	}
	return merged
}

func (e *AutomationRuleEngine) executeStartFlow(ctx context.Context, rule *model.AutomationRule, event model.AutomationEvent, story *model.PMStory, cfg model.ActionConfigStartFlow) error {
	if e.flowService == nil {
		return fmt.Errorf("flow service not configured")
	}
	if cfg.TemplateID == "" {
		return fmt.Errorf("template_id is required in start_flow config")
	}

	// Resolve target type and ID from the event or story context.
	targetType := event.TargetType
	targetID := event.TargetID
	if targetType == "" && story != nil {
		targetType = "story"
		targetID = story.ID
	}
	if targetType == "" || targetID == "" {
		return fmt.Errorf("start_flow requires a target (story or event with target_type/target_id)")
	}

	// Build flow input from the event context.
	flowInput := map[string]any{}
	if cfg.AgentID != "" {
		flowInput["agent_id"] = cfg.AgentID
	}
	if story != nil && story.AssignedAgentID != nil && *story.AssignedAgentID != "" {
		// Use story's assigned agent if no explicit agent in config.
		if _, ok := flowInput["agent_id"]; !ok {
			flowInput["agent_id"] = *story.AssignedAgentID
		}
	}
	if cfg.FlowInput != nil {
		// Merge additional input from config.
		var extra map[string]any
		if err := json.Unmarshal(cfg.FlowInput, &extra); err == nil {
			for k, v := range extra {
				flowInput[k] = v
			}
		}
	}

	// Assign agent to story if an agent_id was resolved (matches run_agent behavior).
	if story != nil {
		if agentID, ok := flowInput["agent_id"].(string); ok && agentID != "" {
			story.AssignedAgentID = &agentID
			if err := e.storyRepo.Update(ctx, story); err != nil {
				return fmt.Errorf("assign agent to story: %w", err)
			}
		}

		// Ensure delivery target exists so the flow's agent_task can run
		// (matches run_agent behavior).
		if e.gitService != nil {
			if _, err := e.gitService.GetStoryDeliveryTarget(ctx, event.WorkspaceID, event.StoryID); err != nil {
				e.logger.WarnContext(ctx, "failed to resolve delivery target for start_flow",
					"error", err,
					"rule_id", rule.ID,
					"story_id", event.StoryID,
				)
			}
		}
	}

	inputJSON, _ := json.Marshal(flowInput)

	req := model.StartFlowRunRequest{
		TemplateID:  cfg.TemplateID,
		TargetType:  targetType,
		TargetID:    targetID,
		Input:       inputJSON,
		TriggerType: event.TriggerType,
	}

	_, err := e.flowService.StartRun(ctx, event.WorkspaceID, "system", req)
	if err != nil {
		return fmt.Errorf("start flow %q: %w", cfg.TemplateID, err)
	}

	if story != nil {
		_ = e.activitySvc.Log(ctx, event.WorkspaceID, "story", event.StoryID, nil,
			fmt.Sprintf("automation rule '%s' started flow '%s'", rule.Name, cfg.TemplateID),
			nil, nil, nil, nil)
	}

	e.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "story",
		EntityID:    targetID,
		WorkspaceID: event.WorkspaceID,
	})

	return nil
}

// EvaluateCronRules loads and evaluates all enabled cron rules across workspaces.
// Called by the sprint automation Temporal workflow after pm_automations is fully retired.
func (e *AutomationRuleEngine) EvaluateCronRules(ctx context.Context, category string) {
	if e == nil {
		return
	}
	rules, err := e.ruleRepo.ListEnabledCronRules(ctx)
	if err != nil {
		e.logger.ErrorContext(ctx, "failed to list cron rules", "error", err)
		return
	}

	for _, rule := range rules {
		var cfg model.TriggerConfigCron
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err != nil {
			continue
		}
		if cfg.Category != category {
			continue
		}

		event := model.AutomationEvent{
			WorkspaceID: rule.WorkspaceID,
			TriggerType: model.TriggerCron,
		}
		if rule.TeamID != nil {
			event.TeamID = *rule.TeamID
		}

		if !e.matchesScope(rule, nil, event) {
			continue
		}

		e.logger.InfoContext(ctx, "executing cron automation rule",
			"rule_id", rule.ID,
			"rule_name", rule.Name,
			"action_type", rule.ActionType,
			"workspace_id", rule.WorkspaceID,
		)

		if err := e.executeAction(ctx, &rule, event, nil, &model.RuleExecutionContext{MaxDepth: defaultMaxChainDepth}); err != nil {
			e.logger.ErrorContext(ctx, "cron automation rule action failed",
				"error", err,
				"rule_id", rule.ID,
				"rule_name", rule.Name,
				"action_type", rule.ActionType,
			)
			e.observeFailure(ctx, event.WorkspaceID, rule.ID, err)
		} else {
			e.observeSuccess(ctx, event.WorkspaceID, rule.ID)
		}
	}
}

// --- CRUD methods ---

// CreateRule creates a new automation rule with validation.
func (e *AutomationRuleEngine) CreateRule(ctx context.Context, workspaceID string, req model.CreateAutomationRuleRequest) (*model.AutomationRule, error) {
	if err := e.validateRuleRequest(req.TriggerType, req.TriggerConfig, req.ActionType, req.ActionConfig); err != nil {
		return nil, err
	}

	rule := &model.AutomationRule{
		WorkspaceID:   workspaceID,
		Name:          req.Name,
		Description:   req.Description,
		Enabled:       true,
		TeamID:        req.TeamID,
		WorkflowID:    req.WorkflowID,
		TriggerType:   req.TriggerType,
		TriggerConfig: req.TriggerConfig,
		ActionType:    req.ActionType,
		ActionConfig:  req.ActionConfig,
	}
	if req.Position != nil {
		rule.Position = *req.Position
	}
	if req.StopOnMatch != nil {
		rule.StopOnMatch = *req.StopOnMatch
	}

	if err := e.ruleRepo.Create(ctx, rule); err != nil {
		return nil, err
	}

	e.logger.InfoContext(ctx, "automation rule created",
		"rule_id", rule.ID,
		"name", rule.Name,
		"trigger_type", rule.TriggerType,
		"action_type", rule.ActionType,
		"workspace_id", workspaceID,
	)
	return rule, nil
}

// UpdateRule updates an existing automation rule.
func (e *AutomationRuleEngine) UpdateRule(ctx context.Context, workspaceID, ruleID string, req model.UpdateAutomationRuleRequest) (*model.AutomationRule, error) {
	rule, err := e.ruleRepo.GetByID(ctx, workspaceID, ruleID)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, fmt.Errorf("automation rule not found")
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Description != nil {
		rule.Description = req.Description
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.TriggerType != nil {
		rule.TriggerType = *req.TriggerType
	}
	if req.TriggerConfig != nil {
		rule.TriggerConfig = *req.TriggerConfig
	}
	if req.ActionType != nil {
		rule.ActionType = *req.ActionType
	}
	if req.ActionConfig != nil {
		rule.ActionConfig = *req.ActionConfig
	}
	if req.Position != nil {
		rule.Position = *req.Position
	}
	if req.StopOnMatch != nil {
		rule.StopOnMatch = *req.StopOnMatch
	}

	if err := e.validateRuleRequest(rule.TriggerType, rule.TriggerConfig, rule.ActionType, rule.ActionConfig); err != nil {
		return nil, err
	}

	if err := e.ruleRepo.Update(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// DeleteRule deletes an automation rule.
func (e *AutomationRuleEngine) DeleteRule(ctx context.Context, workspaceID, ruleID string) error {
	return e.ruleRepo.Delete(ctx, workspaceID, ruleID)
}

// GetRule returns a single automation rule.
func (e *AutomationRuleEngine) GetRule(ctx context.Context, workspaceID, ruleID string) (*model.AutomationRule, error) {
	return e.ruleRepo.GetByID(ctx, workspaceID, ruleID)
}

// ListRules returns all automation rules for a workspace.
func (e *AutomationRuleEngine) ListRules(ctx context.Context, workspaceID string) ([]model.AutomationRule, error) {
	return e.ruleRepo.ListByWorkspace(ctx, workspaceID)
}

// ListRulesByWorkflow returns automation rules for a specific workflow.
func (e *AutomationRuleEngine) ListRulesByWorkflow(ctx context.Context, workspaceID, workflowID string) ([]model.AutomationRule, error) {
	return e.ruleRepo.ListByWorkflow(ctx, workspaceID, workflowID)
}

// --- Validation ---

func (e *AutomationRuleEngine) validateRuleRequest(triggerType string, triggerConfig json.RawMessage, actionType string, actionConfig json.RawMessage) error {
	switch triggerType {
	case model.TriggerStoryStateEntered:
		var cfg model.TriggerConfigStateEntered
		if err := json.Unmarshal(triggerConfig, &cfg); err != nil {
			return fmt.Errorf("invalid trigger_config for %s: %w", triggerType, err)
		}
		if cfg.StateID == "" && cfg.StateType == "" {
			return fmt.Errorf("state_id or state_type is required in trigger_config for %s", triggerType)
		}
	case model.TriggerAgentRunApproved:
		var cfg model.TriggerConfigRunApproved
		if err := json.Unmarshal(triggerConfig, &cfg); err != nil {
			return fmt.Errorf("invalid trigger_config for %s: %w", triggerType, err)
		}
		if cfg.StateID == "" {
			return fmt.Errorf("state_id is required in trigger_config for %s", triggerType)
		}
	case model.TriggerCron:
		var cfg model.TriggerConfigCron
		if err := json.Unmarshal(triggerConfig, &cfg); err != nil {
			return fmt.Errorf("invalid trigger_config for %s: %w", triggerType, err)
		}
		if cfg.Category == "" {
			return fmt.Errorf("category is required in trigger_config for %s", triggerType)
		}
	default:
		return fmt.Errorf("unsupported trigger_type: %s", triggerType)
	}

	switch actionType {
	case model.ActionRunAgent:
		return fmt.Errorf("run_agent is deprecated — use start_flow with template pm.agent_story_run instead")
	case model.ActionMoveToState:
		var cfg model.ActionConfigMoveToState
		if err := json.Unmarshal(actionConfig, &cfg); err != nil {
			return fmt.Errorf("invalid action_config for %s: %w", actionType, err)
		}
		if cfg.TargetStateID == "" {
			return fmt.Errorf("target_state_id is required in action_config for %s", actionType)
		}
	case model.ActionMergeBranch:
		var cfg model.ActionConfigMergeBranch
		if err := json.Unmarshal(actionConfig, &cfg); err != nil {
			return fmt.Errorf("invalid action_config for %s: %w", actionType, err)
		}
		if strings.TrimSpace(cfg.TargetBranch) == "" {
			return fmt.Errorf("target_branch is required in action_config for %s", actionType)
		}
	case model.ActionRunCommand:
		var cfg model.ActionConfigRunCommand
		if err := json.Unmarshal(actionConfig, &cfg); err != nil {
			return fmt.Errorf("invalid action_config for %s: %w", actionType, err)
		}
		if cfg.CommandName == "" {
			return fmt.Errorf("command_name is required in action_config for %s", actionType)
		}
	case model.ActionStartFlow:
		var cfg model.ActionConfigStartFlow
		if err := json.Unmarshal(actionConfig, &cfg); err != nil {
			return fmt.Errorf("invalid action_config for %s: %w", actionType, err)
		}
		if cfg.TemplateID == "" {
			return fmt.Errorf("template_id is required in action_config for %s", actionType)
		}
	default:
		return fmt.Errorf("unsupported action_type: %s", actionType)
	}

	return nil
}

// --- Health observation ---

func (e *AutomationRuleEngine) observeSuccess(ctx context.Context, workspaceID, ruleID string) {
	if e.healthObserver == nil {
		return
	}
	_ = e.healthObserver.ObserveSuccess(ctx, workspaceID, "automation_rule", model.AutomationScopeWorkspace, workspaceID, model.JSONB{
		"rule_id": ruleID,
	})
}

func (e *AutomationRuleEngine) observeFailure(ctx context.Context, workspaceID, ruleID string, cause error) {
	if e.healthObserver == nil || cause == nil {
		return
	}
	_ = e.healthObserver.ObserveFailure(ctx, workspaceID, "automation_rule", model.AutomationScopeWorkspace, workspaceID, cause.Error(), model.JSONB{
		"rule_id": ruleID,
	})
}

// containsString is defined in pm_import.go — reused here.
