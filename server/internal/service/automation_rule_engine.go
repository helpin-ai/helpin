package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/automationcatalog"
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

func nilIfEmpty(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

// AutomationRuleEngine evaluates automation rules against events and executes actions.
type AutomationRuleEngine struct {
	ruleRepo        *repository.AutomationRuleRepository
	taskRepo        *repository.PMTaskRepository
	workflowRepo    *repository.PMWorkflowRepository
	deliveryRepo    *repository.TaskDeliveryTargetRepository
	triggerExecRepo *repository.AgentTriggerExecutionRepository
	agentService    *AgentService
	storyService    *PMTaskService
	gitService      *GitService
	commandService  *InternalCommandService
	notificationSvc *NotificationService
	activitySvc     *PMActivityService
	wsPublisher     *websocket.Publisher
	healthObserver  AutomationHealthObserver
	runEngine       interface {
		StartRuleSchedule(ctx context.Context, ruleID, workspaceID, schedule string) error
		StopRuleSchedule(ctx context.Context, ruleID string) error
	}
	logger *slog.Logger
}

// NewAutomationRuleEngine creates a new AutomationRuleEngine.
func NewAutomationRuleEngine(
	ruleRepo *repository.AutomationRuleRepository,
	taskRepo *repository.PMTaskRepository,
	workflowRepo *repository.PMWorkflowRepository,
	deliveryRepo *repository.TaskDeliveryTargetRepository,
	gitService *GitService,
	notificationSvc *NotificationService,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
) *AutomationRuleEngine {
	return &AutomationRuleEngine{
		ruleRepo:        ruleRepo,
		taskRepo:        taskRepo,
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

// SetTaskService sets the task service (breaks circular dependency).
func (e *AutomationRuleEngine) SetTaskService(svc *PMTaskService) *AutomationRuleEngine {
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

func (e *AutomationRuleEngine) SetTriggerExecutionRepository(repo *repository.AgentTriggerExecutionRepository) *AutomationRuleEngine {
	e.triggerExecRepo = repo
	return e
}

func (e *AutomationRuleEngine) SetRunEngine(runEngine interface {
	StartRuleSchedule(ctx context.Context, ruleID, workspaceID, schedule string) error
	StopRuleSchedule(ctx context.Context, ruleID string) error
}) *AutomationRuleEngine {
	e.runEngine = runEngine
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
			"task_id", event.StoryID,
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
		e.logger.ErrorContext(ctx, "failed to load task for rule evaluation",
			"error", err,
			"task_id", event.StoryID,
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
			"task_id", event.StoryID,
			"depth", execCtx.Depth,
		)

		if err := e.executeAction(ctx, &rule, event, story, execCtx); err != nil {
			e.logger.ErrorContext(ctx, "automation rule action failed",
				"error", err,
				"rule_id", rule.ID,
				"rule_name", rule.Name,
				"action_type", rule.ActionType,
				"task_id", event.StoryID,
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
func (e *AutomationRuleEngine) resolveStoryIfNeeded(ctx context.Context, event model.AutomationEvent) (*model.PMTask, error) {
	switch event.TriggerType {
	case model.TriggerCron:
		return nil, nil
	default:
		if event.StoryID == "" {
			return nil, nil
		}
		story, err := e.taskRepo.GetRawByID(ctx, event.StoryID)
		if err != nil {
			return nil, err
		}
		if story == nil {
			return nil, fmt.Errorf("task %s not found", event.StoryID)
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

	case model.TriggerGitHubPush:
		var cfg model.TriggerConfigGitHubPush
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err != nil {
			return false
		}
		return matchGitHubPushConfig(cfg, event)

	case model.TriggerGitHubPROpened:
		var cfg model.TriggerConfigGitHubPullRequest
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err != nil {
			return false
		}
		return matchGitHubPullRequestConfig(cfg, event)

	case model.TriggerGitHubPRMerged:
		var cfg model.TriggerConfigGitHubPullRequest
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err != nil {
			return false
		}
		return matchGitHubPullRequestConfig(cfg, event)

	case model.TriggerGitHubPRReviewReq:
		var cfg model.TriggerConfigGitHubPullRequest
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err != nil {
			return false
		}
		return matchGitHubPullRequestConfig(cfg, event)

	case model.TriggerGitHubReleasePub:
		var cfg model.TriggerConfigGitHubReleasePublished
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err != nil {
			return false
		}
		if len(cfg.ReleaseKinds) > 0 && strings.TrimSpace(event.ReleaseKind) == "" && e.gitService != nil && strings.TrimSpace(event.WorkspaceID) != "" && strings.TrimSpace(event.RepoFullName) != "" && strings.TrimSpace(event.TagName) != "" {
			releaseKind, err := e.gitService.ResolveReleaseKind(ctx, event.WorkspaceID, event.RepoFullName, event.TagName)
			if err != nil {
				e.logger.WarnContext(ctx, "failed to classify github release event for automation matching",
					"workspace_id", event.WorkspaceID,
					"repo_full_name", event.RepoFullName,
					"tag_name", event.TagName,
					"error", err,
				)
				return false
			}
			event.ReleaseKind = releaseKind
		}
		return matchGitHubReleaseConfig(cfg, event)

	case model.TriggerGitHubCheckSuite:
		var cfg model.TriggerConfigGitHubCheckSuiteCompleted
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err != nil {
			return false
		}
		return matchGitHubCheckSuiteConfig(cfg, event)

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

func (e *AutomationRuleEngine) matchesScope(rule model.AutomationRule, story *model.PMTask, event model.AutomationEvent) bool {
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

func (e *AutomationRuleEngine) executeAction(ctx context.Context, rule *model.AutomationRule, event model.AutomationEvent, story *model.PMTask, execCtx *model.RuleExecutionContext) error {
	switch rule.ActionType {
	case model.ActionRunAgent:
		return fmt.Errorf("run_agent is no longer supported")

	case model.ActionStartAgentRun:
		var cfg model.ActionConfigRunAgent
		if err := json.Unmarshal(rule.ActionConfig, &cfg); err != nil {
			return fmt.Errorf("parse start_agent_run config: %w", err)
		}
		return e.executeStartAgentRun(ctx, rule, event, story, cfg)

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
		return fmt.Errorf("start_flow is no longer supported")

	default:
		return fmt.Errorf("unknown action type: %s", rule.ActionType)
	}
}

func (e *AutomationRuleEngine) executeStartAgentRun(ctx context.Context, rule *model.AutomationRule, event model.AutomationEvent, story *model.PMTask, cfg model.ActionConfigRunAgent) error {
	if e.agentService == nil {
		return fmt.Errorf("agent service not configured")
	}
	if strings.TrimSpace(cfg.AgentID) == "" {
		return fmt.Errorf("start_agent_run requires agent_id")
	}

	targetType := strings.TrimSpace(cfg.TargetType)
	targetID := strings.TrimSpace(cfg.TargetID)
	if targetType == "" && targetID == "" {
		targetType = strings.TrimSpace(event.TargetType)
		targetID = strings.TrimSpace(event.TargetID)
		if targetType == "" && targetID == "" {
			if story != nil {
				targetType = "task"
				targetID = story.ID
			} else if strings.TrimSpace(event.StoryID) != "" {
				targetType = "task"
				targetID = strings.TrimSpace(event.StoryID)
			}
		}
	}
	if targetType == "story" {
		targetType = "task"
	}
	if targetType == "" || targetID == "" {
		err := fmt.Errorf("start_agent_run requires target_type and target_id or an event target")
		e.recordStartAgentRunFailure(ctx, rule, event, cfg, targetType, targetID, err)
		return err
	}

	now := time.Now().UTC()
	trigger := &model.AgentRunTriggerContext{
		Source:      model.AgentRunTriggerSourceAutomationRule,
		TriggerType: event.TriggerType,
		RuleID:      &rule.ID,
		FiredAt:     &now,
	}
	eventContext := &model.AgentRunEventContext{
		StateID: nilIfEmpty(event.StateID),
		TeamID:  nilIfEmpty(event.TeamID),
		RunID:   nilIfEmpty(event.RunID),
		Reason:  strPtr(fmt.Sprintf("automation rule %q", rule.Name)),
	}
	if strings.HasPrefix(strings.TrimSpace(event.TriggerType), "github.") {
		eventContext.GitHub = &model.AgentRunGitHubEventContext{
			EventType:    strings.TrimPrefix(strings.TrimSpace(event.TriggerType), "github."),
			RepoFullName: strings.TrimSpace(event.RepoFullName),
			RepositoryID: strings.TrimSpace(event.RepositoryID),
		}
		if event.TriggerType == model.TriggerGitHubReleasePub {
			eventContext.GitHub.EventType = "release_published"
			eventContext.GitHub.Release = &model.AgentRunGitHubReleaseEventContext{
				TagName:         strings.TrimSpace(event.TagName),
				TargetCommitish: strings.TrimSpace(event.TargetCommitish),
				ReleaseName:     strings.TrimSpace(event.ReleaseName),
				ReleaseURL:      strings.TrimSpace(event.ReleaseURL),
				PublishedAt:     event.PublishedAt,
				IsPrerelease:    event.IsPrerelease,
			}
		}
	}
	outputContext, err := deriveRunOutputContext(event, cfg.Output)
	if err != nil {
		return err
	}
	baseBranch, workingBranch, err := e.resolveRunBranchOverrides(ctx, event, story, cfg)
	if err != nil {
		return fmt.Errorf("resolve branch overrides: %w", err)
	}
	if _, err := e.agentService.startTargetRun(ctx, event.WorkspaceID, targetType, targetID, model.StartAgentRunRequest{
		AgentID:           cfg.AgentID,
		AdditionalContext: cfg.AdditionalContext,
		BaseBranch:        nilIfEmpty(baseBranch),
		WorkingBranch:     nilIfEmpty(workingBranch),
		Output:            outputContext,
	}, nil, trigger, eventContext, nil); err != nil {
		return fmt.Errorf("start agent run: %w", err)
	}

	if e.activitySvc != nil {
		_ = e.activitySvc.Log(ctx, event.WorkspaceID, targetType, targetID, nil,
			fmt.Sprintf("automation rule '%s' started an agent run", rule.Name),
			nil, nil, nil, nil)
	}

	if e.wsPublisher != nil {
		e.wsPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      targetType,
			EntityID:    targetID,
			WorkspaceID: event.WorkspaceID,
		})
	}

	return nil
}

func (e *AutomationRuleEngine) resolveRunBranchOverrides(ctx context.Context, event model.AutomationEvent, story *model.PMTask, cfg model.ActionConfigRunAgent) (string, string, error) {
	baseBranch := strings.TrimSpace(cfg.BaseBranch)
	workingBranch := strings.TrimSpace(cfg.WorkingBranch)
	if baseBranch == "" && workingBranch == "" {
		return "", "", nil
	}

	needsDeliveryTarget := strings.Contains(baseBranch, "{") || strings.Contains(workingBranch, "{")
	if !needsDeliveryTarget {
		return baseBranch, workingBranch, nil
	}
	if e.gitService == nil {
		return "", "", fmt.Errorf("git service not configured")
	}

	taskID := firstNonEmptyString(
		func() string {
			if story == nil {
				return ""
			}
			return story.ID
		}(),
		event.StoryID,
		func() string {
			if strings.EqualFold(event.TargetType, "task") || strings.EqualFold(event.TargetType, "story") {
				return event.TargetID
			}
			return ""
		}(),
		event.TaskID,
	)
	if taskID == "" {
		return "", "", fmt.Errorf("branch templates require task context")
	}

	resolvedBaseBranch, resolvedWorkingBranch, err := e.gitService.ResolveTaskRunBranchValues(ctx, event.WorkspaceID, taskID)
	if err != nil {
		return "", "", fmt.Errorf("resolve task branch values: %w", err)
	}

	resolve := func(value, label string) (string, error) {
		value = strings.TrimSpace(value)
		if value == "" {
			return "", nil
		}
		type replacement struct {
			token string
			value string
		}
		replacements := []replacement{
			{token: "{task_branch}", value: resolvedWorkingBranch},
			{token: "{working_branch}", value: resolvedWorkingBranch},
			{token: "{base_branch}", value: resolvedBaseBranch},
		}
		for _, replacement := range replacements {
			if !strings.Contains(value, replacement.token) {
				continue
			}
			if replacement.value == "" {
				return "", fmt.Errorf("%s uses %s but the task has no value for it", label, replacement.token)
			}
			value = strings.ReplaceAll(value, replacement.token, replacement.value)
		}
		return strings.TrimSpace(value), nil
	}

	resolvedBase, err := resolve(baseBranch, "base_branch")
	if err != nil {
		return "", "", err
	}
	resolvedWorking, err := resolve(workingBranch, "working_branch")
	if err != nil {
		return "", "", err
	}
	return resolvedBase, resolvedWorking, nil
}

func (e *AutomationRuleEngine) recordStartAgentRunFailure(
	ctx context.Context,
	rule *model.AutomationRule,
	event model.AutomationEvent,
	cfg model.ActionConfigRunAgent,
	targetType, targetID string,
	err error,
) {
	if e == nil || e.triggerExecRepo == nil || rule == nil || strings.TrimSpace(cfg.AgentID) == "" {
		return
	}

	firedAt := time.Now().UTC()
	execution := &model.AgentTriggerExecution{
		WorkspaceID:   event.WorkspaceID,
		AgentID:       strings.TrimSpace(cfg.AgentID),
		BindingID:     resolveAutomationRuleBindingID(event.TriggerType),
		BindingKind:   "automation_rule",
		TriggerType:   nilIfEmpty(event.TriggerType),
		ReferenceID:   &rule.ID,
		ReferenceType: strPtr("automation_rule"),
		TargetType:    nilIfEmpty(targetType),
		TargetID:      nilIfEmpty(targetID),
		Status:        model.AgentTriggerExecutionStatusFailed,
		ErrorMessage:  nilIfEmpty(err.Error()),
		FiredAt:       firedAt,
		CompletedAt:   &firedAt,
	}
	if createErr := e.triggerExecRepo.Create(ctx, execution); createErr != nil {
		e.logger.WarnContext(ctx, "failed to record trigger execution failure",
			"workspace_id", event.WorkspaceID,
			"rule_id", rule.ID,
			"agent_id", cfg.AgentID,
			"error", createErr,
		)
	}
}

func resolveAutomationRuleBindingID(triggerType string) string {
	if bindingID, _, ok := automationcatalog.ResolveBindingForTrigger(model.AgentRunTriggerSourceAutomationRule, triggerType, ""); ok {
		return bindingID
	}
	return strings.TrimSpace(triggerType)
}

func (e *AutomationRuleEngine) executeMoveToState(ctx context.Context, rule *model.AutomationRule, event model.AutomationEvent, story *model.PMTask, cfg model.ActionConfigMoveToState, execCtx *model.RuleExecutionContext) error {
	if e.storyService == nil {
		return fmt.Errorf("task service not configured")
	}
	if cfg.TargetStateID == "" {
		return fmt.Errorf("target_state_id is required in move_to_state config")
	}

	// Guard: if story is already in the target state, no-op (prevents loops)
	if story.WorkflowStateID == cfg.TargetStateID {
		e.logger.InfoContext(ctx, "skipping move_to_state: task already in target state",
			"rule_id", rule.ID,
			"task_id", event.StoryID,
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
	_, err := e.storyService.MoveToState(chainCtx, event.StoryID, model.MoveTaskRequest{
		StateID: cfg.TargetStateID,
	}, "system")
	if err != nil {
		return fmt.Errorf("move task to state: %w", err)
	}

	_ = e.activitySvc.Log(ctx, event.WorkspaceID, "task", event.StoryID, nil,
		fmt.Sprintf("automation rule '%s' advanced task to next stage", rule.Name),
		nil, nil, nil, nil)

	return nil
}

func (e *AutomationRuleEngine) executeMergeBranch(ctx context.Context, rule *model.AutomationRule, event model.AutomationEvent, story *model.PMTask, cfg model.ActionConfigMergeBranch) error {
	if e.gitService == nil {
		return fmt.Errorf("git service not configured")
	}
	if strings.TrimSpace(cfg.TargetBranch) == "" {
		return fmt.Errorf("target_branch is required in merge_branch config")
	}

	// Load delivery target to get working branch
	target, err := e.deliveryRepo.GetByTask(ctx, event.WorkspaceID, event.StoryID)
	if err != nil {
		return fmt.Errorf("load delivery target: %w", err)
	}
	if target == nil || target.WorkingBranch == nil || *target.WorkingBranch == "" {
		e.logger.WarnContext(ctx, "skipping merge_branch: no working branch",
			"rule_id", rule.ID,
			"task_id", event.StoryID,
		)
		return nil
	}
	if target.RepoFullName == nil || *target.RepoFullName == "" {
		e.logger.WarnContext(ctx, "skipping merge_branch: no repository configured",
			"rule_id", rule.ID,
			"task_id", event.StoryID,
		)
		return nil
	}

	// Resolve {base_branch} variable
	resolvedBranch := cfg.TargetBranch
	if strings.Contains(resolvedBranch, "{base_branch}") {
		if target.BaseBranch != nil && *target.BaseBranch != "" {
			resolvedBranch = strings.ReplaceAll(resolvedBranch, "{base_branch}", *target.BaseBranch)
		} else {
			e.logger.WarnContext(ctx, "skipping merge_branch: {base_branch} used but task has no base branch",
				"rule_id", rule.ID,
				"task_id", event.StoryID,
			)
			return nil
		}
	}

	if err := e.gitService.MergeBranch(ctx, event.WorkspaceID, event.StoryID, resolvedBranch); err != nil {
		e.logger.ErrorContext(ctx, "merge_branch failed",
			"error", err,
			"rule_id", rule.ID,
			"task_id", event.StoryID,
			"working_branch", *target.WorkingBranch,
			"target_branch", resolvedBranch,
		)
		// Don't halt the pipeline for merge failures — log and continue
		return nil
	}

	_ = e.activitySvc.Log(ctx, event.WorkspaceID, "task", event.StoryID, nil,
		fmt.Sprintf("automation rule '%s' merged %s into %s", rule.Name, *target.WorkingBranch, resolvedBranch),
		nil, nil, nil, nil)

	return nil
}

func (e *AutomationRuleEngine) executeRunCommand(ctx context.Context, rule *model.AutomationRule, event model.AutomationEvent, story *model.PMTask, cfg model.ActionConfigRunCommand) error {
	if e.commandService == nil {
		return fmt.Errorf("command service not configured")
	}
	if cfg.CommandName == "" {
		return fmt.Errorf("command_name is required in run_command config")
	}

	targetType := event.TargetType
	targetID := event.TargetID
	if targetType == "" && story != nil {
		targetType = "task"
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

// ExecuteScheduledRule runs a cron-triggered automation rule from its dedicated schedule.
func (e *AutomationRuleEngine) ExecuteScheduledRule(ctx context.Context, workspaceID, ruleID string) error {
	if e == nil {
		return nil
	}
	rule, err := e.ruleRepo.GetByID(ctx, workspaceID, ruleID)
	if err != nil {
		return err
	}
	if rule == nil {
		return fmt.Errorf("automation rule not found")
	}
	if !rule.Enabled {
		return fmt.Errorf("automation rule is disabled")
	}
	if rule.TriggerType != model.TriggerCron {
		return fmt.Errorf("automation rule is not scheduled")
	}

	var actionCfg model.ActionConfigRunAgent
	if err := json.Unmarshal(rule.ActionConfig, &actionCfg); err != nil {
		return fmt.Errorf("parse start_agent_run config: %w", err)
	}

	targetType := strings.TrimSpace(actionCfg.TargetType)
	targetID := strings.TrimSpace(actionCfg.TargetID)
	if targetType == "" && targetID == "" {
		targetType = "workspace"
		targetID = workspaceID
	}
	if targetType == "story" {
		targetType = "task"
	}

	event := model.AutomationEvent{
		WorkspaceID: workspaceID,
		TriggerType: model.TriggerCron,
		TargetType:  targetType,
		TargetID:    targetID,
	}
	if rule.TeamID != nil {
		event.TeamID = strings.TrimSpace(*rule.TeamID)
	}

	e.logger.InfoContext(ctx, "executing scheduled automation rule",
		"rule_id", rule.ID,
		"rule_name", rule.Name,
		"workspace_id", workspaceID,
		"target_type", targetType,
		"target_id", targetID,
	)

	if err := e.executeAction(ctx, rule, event, nil, &model.RuleExecutionContext{MaxDepth: defaultMaxChainDepth}); err != nil {
		e.observeFailure(ctx, workspaceID, rule.ID, err)
		return err
	}
	e.observeSuccess(ctx, workspaceID, rule.ID)
	return nil
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
	if err := e.syncRuleSchedule(ctx, rule, false); err != nil {
		_ = e.ruleRepo.Delete(ctx, workspaceID, rule.ID)
		return nil, err
	}

	e.logger.InfoContext(ctx, "automation rule created",
		"rule_id", rule.ID,
		"name", rule.Name,
		"trigger_type", rule.TriggerType,
		"action_type", rule.ActionType,
		"workspace_id", workspaceID,
	)
	publishWorkspaceEventWithParent(e.wsPublisher, "created", "automation_rule", rule.ID, workspaceID, "", "workflow", derefString(rule.WorkflowID), map[string]any{
		"workflow_id": derefString(rule.WorkflowID),
	})
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
	wasScheduled := rule.TriggerType == model.TriggerCron && rule.Enabled

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
	if err := e.syncRuleSchedule(ctx, rule, wasScheduled); err != nil {
		return nil, err
	}
	publishWorkspaceEventWithParent(e.wsPublisher, "updated", "automation_rule", rule.ID, workspaceID, "", "workflow", derefString(rule.WorkflowID), map[string]any{
		"workflow_id": derefString(rule.WorkflowID),
	})
	return rule, nil
}

// DeleteRule deletes an automation rule.
func (e *AutomationRuleEngine) DeleteRule(ctx context.Context, workspaceID, ruleID string) error {
	rule, err := e.ruleRepo.GetByID(ctx, workspaceID, ruleID)
	if err != nil {
		return err
	}
	if rule == nil {
		return fmt.Errorf("automation rule not found")
	}
	if rule.TriggerType == model.TriggerCron && e.runEngine != nil {
		_ = e.runEngine.StopRuleSchedule(ctx, rule.ID)
	}
	if err := e.ruleRepo.Delete(ctx, workspaceID, ruleID); err != nil {
		return err
	}
	publishWorkspaceEventWithParent(e.wsPublisher, "deleted", "automation_rule", ruleID, workspaceID, "", "workflow", derefString(rule.WorkflowID), map[string]any{
		"workflow_id": derefString(rule.WorkflowID),
	})
	return nil
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

// EnsureScheduledRules ensures Temporal cron workflows exist for all enabled scheduled rules.
func (e *AutomationRuleEngine) EnsureScheduledRules(ctx context.Context) error {
	if e == nil || e.runEngine == nil {
		return nil
	}
	rules, err := e.ruleRepo.ListEnabledCronRules(ctx)
	if err != nil {
		return err
	}
	for idx := range rules {
		if err := e.syncRuleSchedule(ctx, &rules[idx], false); err != nil {
			return err
		}
	}
	return nil
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
	case model.TriggerGitHubPush:
		var cfg model.TriggerConfigGitHubPush
		if err := json.Unmarshal(triggerConfig, &cfg); err != nil {
			return fmt.Errorf("invalid trigger_config for %s: %w", triggerType, err)
		}
		if strings.TrimSpace(cfg.Branch) == "" && strings.TrimSpace(cfg.RepoFullName) == "" {
			return fmt.Errorf("branch or repo_full_name is required in trigger_config for %s", triggerType)
		}
	case model.TriggerGitHubPROpened:
		var cfg model.TriggerConfigGitHubPullRequest
		if err := json.Unmarshal(triggerConfig, &cfg); err != nil {
			return fmt.Errorf("invalid trigger_config for %s: %w", triggerType, err)
		}
		if strings.TrimSpace(cfg.BaseBranch) == "" && strings.TrimSpace(cfg.RepoFullName) == "" {
			return fmt.Errorf("base_branch or repo_full_name is required in trigger_config for %s", triggerType)
		}
	case model.TriggerGitHubPRMerged:
		var cfg model.TriggerConfigGitHubPullRequest
		if err := json.Unmarshal(triggerConfig, &cfg); err != nil {
			return fmt.Errorf("invalid trigger_config for %s: %w", triggerType, err)
		}
		if strings.TrimSpace(cfg.BaseBranch) == "" && strings.TrimSpace(cfg.RepoFullName) == "" {
			return fmt.Errorf("base_branch or repo_full_name is required in trigger_config for %s", triggerType)
		}
	case model.TriggerGitHubPRReviewReq:
		var cfg model.TriggerConfigGitHubPullRequest
		if err := json.Unmarshal(triggerConfig, &cfg); err != nil {
			return fmt.Errorf("invalid trigger_config for %s: %w", triggerType, err)
		}
		if strings.TrimSpace(cfg.BaseBranch) == "" && strings.TrimSpace(cfg.RepoFullName) == "" {
			return fmt.Errorf("base_branch or repo_full_name is required in trigger_config for %s", triggerType)
		}
	case model.TriggerGitHubReleasePub:
		var cfg model.TriggerConfigGitHubReleasePublished
		if err := json.Unmarshal(triggerConfig, &cfg); err != nil {
			return fmt.Errorf("invalid trigger_config for %s: %w", triggerType, err)
		}
		if strings.TrimSpace(cfg.TagName) == "" && strings.TrimSpace(cfg.RepoFullName) == "" && strings.TrimSpace(cfg.TagPattern) == "" && len(cfg.ReleaseKinds) == 0 {
			return fmt.Errorf("repo_full_name, tag_name, tag_pattern, or release_kinds is required in trigger_config for %s", triggerType)
		}
	case model.TriggerGitHubCheckSuite:
		var cfg model.TriggerConfigGitHubCheckSuiteCompleted
		if err := json.Unmarshal(triggerConfig, &cfg); err != nil {
			return fmt.Errorf("invalid trigger_config for %s: %w", triggerType, err)
		}
		if strings.TrimSpace(cfg.Branch) == "" && strings.TrimSpace(cfg.Conclusion) == "" && strings.TrimSpace(cfg.RepoFullName) == "" {
			return fmt.Errorf("branch, conclusion, or repo_full_name is required in trigger_config for %s", triggerType)
		}
	case model.TriggerCron:
		var cfg model.TriggerConfigCron
		if err := json.Unmarshal(triggerConfig, &cfg); err != nil {
			return fmt.Errorf("invalid trigger_config for %s: %w", triggerType, err)
		}
		if _, _, err := resolveCronTriggerConfig(cfg); err != nil {
			return fmt.Errorf("invalid trigger_config for %s: %w", triggerType, err)
		}
	default:
		return fmt.Errorf("unsupported trigger_type: %s", triggerType)
	}

	switch actionType {
	case model.ActionRunAgent:
		return fmt.Errorf("action_type %s is no longer supported", actionType)
	case model.ActionStartAgentRun:
		var cfg model.ActionConfigRunAgent
		if err := json.Unmarshal(actionConfig, &cfg); err != nil {
			return fmt.Errorf("invalid action_config for %s: %w", actionType, err)
		}
		if strings.TrimSpace(cfg.AgentID) == "" {
			return fmt.Errorf("agent_id is required in action_config for %s", actionType)
		}
		targetType := strings.TrimSpace(cfg.TargetType)
		targetID := strings.TrimSpace(cfg.TargetID)
		if (targetType == "") != (targetID == "") {
			return fmt.Errorf("target_type and target_id must both be set in action_config for %s", actionType)
		}
		if isGitHubAutomationTrigger(triggerType) && triggerType != model.TriggerGitHubReleasePub && (targetType == "" || targetID == "") {
			return fmt.Errorf("target_type and target_id are required in action_config for %s when trigger_type is %s", actionType, triggerType)
		}
		if cfg.Output != nil && strings.EqualFold(strings.TrimSpace(cfg.Output.Type), "docs_document") && strings.TrimSpace(cfg.Output.SpaceID) == "" {
			return fmt.Errorf("output.space_id is required when output.type is docs_document")
		}
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
		return fmt.Errorf("action_type %s is no longer supported", actionType)
	default:
		return fmt.Errorf("unsupported action_type: %s", actionType)
	}

	return nil
}

func isGitHubAutomationTrigger(triggerType string) bool {
	switch triggerType {
	case model.TriggerGitHubPush,
		model.TriggerGitHubPROpened,
		model.TriggerGitHubPRMerged,
		model.TriggerGitHubPRReviewReq,
		model.TriggerGitHubReleasePub,
		model.TriggerGitHubCheckSuite:
		return true
	default:
		return false
	}
}

func resolveCronTriggerConfig(cfg model.TriggerConfigCron) (schedule, preset string, err error) {
	schedule = strings.TrimSpace(cfg.Schedule)
	preset = strings.TrimSpace(cfg.Preset)
	category := strings.TrimSpace(cfg.Category)

	if preset == "" {
		switch category {
		case "workspace_hourly":
			preset = "hourly"
		case "workspace_daily":
			preset = "daily"
		case "workspace_weekly":
			preset = "weekly"
		}
	}

	if schedule == "" && preset != "" {
		switch preset {
		case "hourly":
			schedule = "0 * * * *"
		case "daily":
			schedule = "0 0 * * *"
		case "weekly":
			schedule = "0 0 * * 1"
		default:
			return "", "", fmt.Errorf("unsupported preset %q", preset)
		}
	}

	if schedule == "" && category != "" && strings.Contains(category, " ") {
		schedule = category
	}
	if schedule == "" {
		return "", "", fmt.Errorf("schedule or preset is required")
	}
	return schedule, preset, nil
}

func (e *AutomationRuleEngine) syncRuleSchedule(ctx context.Context, rule *model.AutomationRule, stopFirst bool) error {
	if e == nil || e.runEngine == nil || rule == nil {
		return nil
	}
	if stopFirst {
		_ = e.runEngine.StopRuleSchedule(ctx, rule.ID)
	}
	if rule.TriggerType != model.TriggerCron || !rule.Enabled {
		return nil
	}

	var cfg model.TriggerConfigCron
	if err := json.Unmarshal(rule.TriggerConfig, &cfg); err != nil {
		return fmt.Errorf("parse cron trigger config: %w", err)
	}
	schedule, _, err := resolveCronTriggerConfig(cfg)
	if err != nil {
		return err
	}
	return e.runEngine.StartRuleSchedule(ctx, rule.ID, rule.WorkspaceID, schedule)
}

func matchGitHubPushConfig(cfg model.TriggerConfigGitHubPush, event model.AutomationEvent) bool {
	if strings.TrimSpace(cfg.Branch) != "" && strings.TrimSpace(cfg.Branch) != strings.TrimSpace(event.Branch) {
		return false
	}
	if strings.TrimSpace(cfg.RepoFullName) != "" && !strings.EqualFold(strings.TrimSpace(cfg.RepoFullName), strings.TrimSpace(event.RepoFullName)) {
		return false
	}
	return strings.TrimSpace(event.Branch) != "" || strings.TrimSpace(event.RepoFullName) != ""
}

func matchGitHubPullRequestConfig(cfg model.TriggerConfigGitHubPullRequest, event model.AutomationEvent) bool {
	if strings.TrimSpace(cfg.BaseBranch) != "" && strings.TrimSpace(cfg.BaseBranch) != strings.TrimSpace(event.BaseBranch) {
		return false
	}
	if strings.TrimSpace(cfg.RepoFullName) != "" && !strings.EqualFold(strings.TrimSpace(cfg.RepoFullName), strings.TrimSpace(event.RepoFullName)) {
		return false
	}
	return strings.TrimSpace(event.BaseBranch) != "" || strings.TrimSpace(event.RepoFullName) != ""
}

func matchGitHubReleaseConfig(cfg model.TriggerConfigGitHubReleasePublished, event model.AutomationEvent) bool {
	if strings.TrimSpace(cfg.TagName) != "" && strings.TrimSpace(cfg.TagName) != strings.TrimSpace(event.TagName) {
		return false
	}
	if strings.TrimSpace(cfg.TagPattern) != "" {
		matched, err := path.Match(strings.TrimSpace(cfg.TagPattern), strings.TrimSpace(event.TagName))
		if err != nil || !matched {
			return false
		}
	}
	if strings.TrimSpace(cfg.RepoFullName) != "" && !strings.EqualFold(strings.TrimSpace(cfg.RepoFullName), strings.TrimSpace(event.RepoFullName)) {
		return false
	}
	if event.IsPrerelease && !cfg.IncludePrerelease && !containsReleaseKind(cfg.ReleaseKinds, "prerelease") {
		return false
	}
	if len(cfg.ReleaseKinds) > 0 && !containsReleaseKind(cfg.ReleaseKinds, event.ReleaseKind) {
		return false
	}
	return strings.TrimSpace(event.TagName) != "" || strings.TrimSpace(event.RepoFullName) != ""
}

func containsReleaseKind(kinds []string, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	for _, kind := range kinds {
		if strings.EqualFold(strings.TrimSpace(kind), candidate) {
			return true
		}
	}
	return false
}

func deriveRunOutputContext(event model.AutomationEvent, cfg *model.ActionConfigRunAgentOutput) (*model.AgentRunOutputContext, error) {
	if cfg == nil {
		return nil, nil
	}
	output := &model.AgentRunOutputContext{
		Type:           strings.TrimSpace(cfg.Type),
		SpaceID:        strings.TrimSpace(cfg.SpaceID),
		CollectionID:   cfg.CollectionID,
		IdempotencyKey: strings.TrimSpace(cfg.IdempotencyKey),
	}
	if strings.EqualFold(output.Type, "docs_document") && output.SpaceID == "" {
		return nil, fmt.Errorf("output.space_id is required when output.type is docs_document")
	}
	if output.IdempotencyKey == "" && event.TriggerType == model.TriggerGitHubReleasePub && strings.TrimSpace(event.RepositoryID) != "" && strings.TrimSpace(event.TagName) != "" {
		output.IdempotencyKey = fmt.Sprintf("release_notes:%s:%s", strings.TrimSpace(event.RepositoryID), strings.TrimSpace(event.TagName))
	}
	return output, nil
}

func matchGitHubCheckSuiteConfig(cfg model.TriggerConfigGitHubCheckSuiteCompleted, event model.AutomationEvent) bool {
	if strings.TrimSpace(cfg.Branch) != "" && strings.TrimSpace(cfg.Branch) != strings.TrimSpace(event.Branch) {
		return false
	}
	if strings.TrimSpace(cfg.Conclusion) != "" && strings.TrimSpace(cfg.Conclusion) != strings.TrimSpace(event.Conclusion) {
		return false
	}
	if strings.TrimSpace(cfg.RepoFullName) != "" && !strings.EqualFold(strings.TrimSpace(cfg.RepoFullName), strings.TrimSpace(event.RepoFullName)) {
		return false
	}
	return strings.TrimSpace(event.Branch) != "" || strings.TrimSpace(event.Conclusion) != "" || strings.TrimSpace(event.RepoFullName) != ""
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
