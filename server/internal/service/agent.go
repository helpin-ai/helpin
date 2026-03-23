package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

type supportRunSummary struct {
	DraftReply *supportDraftReply `json:"draft_reply,omitempty"`
}

type supportDraftReply struct {
	Content           string  `json:"content"`
	IsInternal        bool    `json:"is_internal"`
	SenderDisplayName *string `json:"sender_display_name,omitempty"`
	ApprovalRequired  bool    `json:"approval_required"`
}

const stuckPostRunThreshold = 2 * time.Minute
const staleQueuedRunThreshold = 30 * time.Second
const defaultSystemProductPlannerName = "Epic Planner"

// AgentService contains agent business logic.
type AgentService struct {
	agentRepo        *repository.AgentRepository
	runRepo          *repository.AgentRunRepository
	runMessageRepo   *repository.AgentRunMessageRepository
	artifactRepo     *repository.AgentRunArtifactRepository
	storyRepo        *repository.PMStoryRepository
	storyLinkRepo    *repository.PMStoryLinkRepository
	epicRepo         *repository.PMEpicRepository
	conversationRepo *repository.SupportConversationRepository
	messageRepo      *repository.SupportMessageRepository
	handoffRepo      *repository.AgentHandoffRepository
	settingsRepo     *repository.SettingsRepository
	docsSpaceRepo    *repository.DocsSpaceRepository
	docsDocumentRepo *repository.DocsDocumentRepository
	docsContentRepo  *repository.DocsContentRepository
	docsVersionRepo  *repository.DocsVersionRepository
	docsLinkRepo     *repository.DocsLinkRepository
	runEngine        *temporalapp.RunEngine
	gitService       *GitService
	storyService     *PMStoryService
	activitySvc      *PMActivityService
	wsPublisher      *websocket.Publisher
	ruleEngine       *AutomationRuleEngine
	anthropicAPIKey  string
	openAIAPIKey     string
	openRouterAPIKey string
}

// NewAgentService creates a new AgentService.
func NewAgentService(
	agentRepo *repository.AgentRepository,
	runRepo *repository.AgentRunRepository,
	runMessageRepo *repository.AgentRunMessageRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
	storyRepo *repository.PMStoryRepository,
	storyLinkRepo *repository.PMStoryLinkRepository,
	epicRepo *repository.PMEpicRepository,
	conversationRepo *repository.SupportConversationRepository,
	messageRepo *repository.SupportMessageRepository,
	handoffRepo *repository.AgentHandoffRepository,
	settingsRepo *repository.SettingsRepository,
	docsSpaceRepo *repository.DocsSpaceRepository,
	docsDocumentRepo *repository.DocsDocumentRepository,
	docsContentRepo *repository.DocsContentRepository,
	docsVersionRepo *repository.DocsVersionRepository,
	docsLinkRepo *repository.DocsLinkRepository,
	runEngine *temporalapp.RunEngine,
	gitService *GitService,
	storyService *PMStoryService,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
) *AgentService {
	return &AgentService{
		agentRepo:        agentRepo,
		runRepo:          runRepo,
		runMessageRepo:   runMessageRepo,
		artifactRepo:     artifactRepo,
		storyRepo:        storyRepo,
		storyLinkRepo:    storyLinkRepo,
		epicRepo:         epicRepo,
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		handoffRepo:      handoffRepo,
		settingsRepo:     settingsRepo,
		docsSpaceRepo:    docsSpaceRepo,
		docsDocumentRepo: docsDocumentRepo,
		docsContentRepo:  docsContentRepo,
		docsVersionRepo:  docsVersionRepo,
		docsLinkRepo:     docsLinkRepo,
		runEngine:        runEngine,
		gitService:       gitService,
		storyService:     storyService,
		activitySvc:      activitySvc,
		wsPublisher:      wsPublisher,
	}
}

func (s *AgentService) SetModelProviderConfig(anthropicAPIKey, openAIAPIKey, openRouterAPIKey string) *AgentService {
	s.anthropicAPIKey = strings.TrimSpace(anthropicAPIKey)
	s.openAIAPIKey = strings.TrimSpace(openAIAPIKey)
	s.openRouterAPIKey = strings.TrimSpace(openRouterAPIKey)
	return s
}

// SetRuleEngine sets the automation rule engine (breaks circular dependency).
func (s *AgentService) SetRuleEngine(engine *AutomationRuleEngine) *AgentService {
	s.ruleEngine = engine
	return s
}

// SeedWorkspaceDefaults creates workspace-scoped built-in agents.
func (s *AgentService) SeedWorkspaceDefaults(ctx context.Context, workspaceID, actorID string) error {
	_, err := s.ensureBuiltInAgent(ctx, workspaceID, actorID, model.AgentPresetEpicPlanner)
	return err
}

func (s *AgentService) ensureSystemProductPlannerAgent(ctx context.Context, workspaceID, actorID string) (*model.Agent, error) {
	return s.ensureBuiltInAgent(ctx, workspaceID, actorID, model.AgentPresetEpicPlanner)
}

func (s *AgentService) ensureBuiltInAgent(ctx context.Context, workspaceID, actorID, presetKey string) (*model.Agent, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	presetKey = normalizePresetKey(presetKey)
	preset, ok := agentPresetDefinition(presetKey)
	if !ok {
		return nil, fmt.Errorf("unsupported built-in preset %q", presetKey)
	}

	existing, err := s.agentRepo.GetSystemByPreset(ctx, workspaceID, presetKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		changed := false
		beforePresetKey := existing.PresetKey
		beforeRole := existing.Role
		beforeRuntimeKind := existing.RuntimeKind
		beforeTriggerMode := existing.TriggerMode
		beforeApprovalMode := existing.ApprovalMode
		beforeDefaultInvocationMode := existing.DefaultInvocationMode
		beforeAllowedTools := string(existing.AllowedTools)
		beforeAllowedCommands := string(existing.AllowedCommands)
		beforeAllowedTargets := string(existing.AllowedTargets)
		hadPlanningNotes := existing.PlanningNotes != nil
		if existing.PlanningNotes != nil || (presetKey == model.AgentPresetEpicPlanner && productPlannerPromptNeedsRefresh(existing.SystemPrompt)) {
			existing.SystemPrompt = storedSystemPromptForPreset(presetKey, nil, existing.PlanningNotes)
			existing.PlanningNotes = nil
			changed = true
		}
		if existing.PresetKey != presetKey {
			existing.PresetKey = presetKey
			changed = true
		}
		if strings.TrimSpace(existing.RuntimeKind) == "" {
			existing.RuntimeKind = preset.RuntimeKind
			changed = true
		}
		if strings.TrimSpace(existing.DefaultInvocationMode) == "" {
			existing.DefaultInvocationMode = preset.DefaultInvocationMode
			changed = true
		}
		normalizeAgentRecord(existing)
		if existing.PresetKey != beforePresetKey ||
			existing.Role != beforeRole ||
			existing.RuntimeKind != beforeRuntimeKind ||
			existing.TriggerMode != beforeTriggerMode ||
			existing.ApprovalMode != beforeApprovalMode ||
			existing.DefaultInvocationMode != beforeDefaultInvocationMode ||
			string(existing.AllowedTools) != beforeAllowedTools ||
			string(existing.AllowedCommands) != beforeAllowedCommands ||
			string(existing.AllowedTargets) != beforeAllowedTargets ||
			(hadPlanningNotes && existing.PlanningNotes == nil) {
			changed = true
		}
		if changed {
			if err := s.agentRepo.Update(ctx, existing); err != nil {
				return nil, err
			}
		}
		return existing, nil
	}

	systemPrompt := storedSystemPromptForPreset(presetKey, nil, nil)
	agent := &model.Agent{
		WorkspaceID:           workspaceID,
		IsSystem:              true,
		Name:                  defaultSystemProductPlannerName,
		PresetKey:             presetKey,
		Role:                  preset.DefaultRole,
		Status:                "idle",
		RuntimeKind:           preset.RuntimeKind,
		Skills:                json.RawMessage("[]"),
		TriggerMode:           preset.DefaultTriggerMode,
		SystemPrompt:          systemPrompt,
		PlanningNotes:         nil,
		AllowedTools:          mustJSONStringSlice(preset.AllowedTools),
		AllowedCommands:       mustJSONStringSlice(preset.AllowedCommands),
		AllowedTargets:        mustJSONStringSlice(preset.AllowedTargetTypes),
		ApprovalMode:          preset.ApprovalMode,
		MaxConcurrentRuns:     1,
		DefaultInvocationMode: preset.DefaultInvocationMode,
	}
	normalizeAgentRecord(agent)
	if err := s.validateModelRouting(agent); err != nil {
		return nil, err
	}
	if err := s.agentRepo.Create(ctx, agent); err != nil {
		return nil, err
	}
	if s.activitySvc != nil {
		newValue := agent.Name
		_ = s.activitySvc.Log(ctx, agent.WorkspaceID, "agent", agent.ID, &actorID, "created", nil, nil, &newValue, nil)
	}
	s.publishSimpleEvent("created", "agent", agent.ID, agent.WorkspaceID, actorID)
	return agent, nil
}

// ListAgents returns all agents in a workspace.
func (s *AgentService) ListAgents(ctx context.Context, workspaceID string) ([]model.Agent, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	agents, err := s.agentRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for idx := range agents {
		normalizeAgentRecord(&agents[idx])
	}
	return agents, nil
}

// GetAgent returns a single agent.
func (s *AgentService) GetAgent(ctx context.Context, workspaceID, id string) (*model.Agent, error) {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	normalizeAgentRecord(agent)
	return agent, nil
}

func (s *AgentService) ListAgentPresets() []model.AgentPresetDefinition {
	return ListAgentPresets()
}

// ListToolCatalog returns the tool catalog with categories and preset mappings.
func (s *AgentService) ListToolCatalog() model.ToolCatalogResponse {
	catalog := worker.ListToolCatalog()
	presets := ListAgentPresets()
	for idx := range catalog.Tools {
		tool := &catalog.Tools[idx]
		presetKeys := make([]string, 0, len(presets))
		for _, preset := range presets {
			if slices.Contains(preset.AllowedTools, tool.Name) {
				presetKeys = append(presetKeys, preset.Key)
			}
		}
		tool.Presets = presetKeys
	}
	return catalog
}

func (s *AgentService) ListModelProviders() []model.AgentModelProviderOption {
	options := make([]model.AgentModelProviderOption, 0, 3)
	if s.isModelProviderConfigured(model.AgentModelProviderAnthropic) {
		options = append(options, model.AgentModelProviderOption{
			Value:            model.AgentModelProviderAnthropic,
			Label:            "Anthropic",
			ModelPlaceholder: "claude-sonnet-4-20250514",
		})
	}
	if s.isModelProviderConfigured(model.AgentModelProviderOpenAI) {
		options = append(options, model.AgentModelProviderOption{
			Value:            model.AgentModelProviderOpenAI,
			Label:            "OpenAI",
			ModelPlaceholder: "gpt-5-mini",
		})
	}
	if s.isModelProviderConfigured(model.AgentModelProviderOpenRouter) {
		options = append(options, model.AgentModelProviderOption{
			Value:            model.AgentModelProviderOpenRouter,
			Label:            "OpenRouter",
			ModelPlaceholder: "openai/gpt-5-mini",
		})
	}
	return options
}

// CreateAgent creates a new agent.
func (s *AgentService) CreateAgent(ctx context.Context, req model.CreateAgentRequest, actorID string) (*model.Agent, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}

	skills := req.Skills
	if skills == nil {
		skills = json.RawMessage("[]")
	}

	presetKey := normalizePresetKey(stringOrDefault(req.PresetKey, ""))
	if presetKey == "" {
		presetKey = defaultPresetKeyForAgent(false)
	}
	if err := validateAgentPresetKey(presetKey); err != nil {
		return nil, err
	}
	preset, _ := agentPresetDefinition(presetKey)

	role := strings.TrimSpace(req.Role)
	if role == "" {
		if preset.DefaultRole != "" {
			role = preset.DefaultRole
		} else {
			role = "Agent"
		}
	}
	runtimeKind := strings.TrimSpace(stringOrDefault(req.RuntimeKind, preset.RuntimeKind))
	if runtimeKind == "" {
		runtimeKind = "opencode"
	}
	triggerMode := stringOrDefault(req.TriggerMode, preset.DefaultTriggerMode)
	if triggerMode == "" {
		triggerMode = "manual"
	}
	if err := validateRuntimeKind(runtimeKind); err != nil {
		return nil, err
	}

	approvalMode := preset.ApprovalMode
	if req.ApprovalMode != nil && *req.ApprovalMode != "" {
		approvalMode = *req.ApprovalMode
	}
	maxConcurrentRuns := 1
	if req.MaxConcurrentRuns != nil && *req.MaxConcurrentRuns > 0 {
		maxConcurrentRuns = *req.MaxConcurrentRuns
	}

	agent := &model.Agent{
		WorkspaceID:           req.WorkspaceID,
		Name:                  strings.TrimSpace(req.Name),
		PresetKey:             presetKey,
		Role:                  role,
		Status:                "idle",
		RuntimeKind:           runtimeKind,
		Skills:                skills,
		TriggerMode:           triggerMode,
		Provider:              trimPtr(req.Provider),
		Model:                 trimPtr(req.Model),
		SystemPrompt:          storedSystemPromptForPreset(presetKey, req.SystemPrompt, req.PlanningNotes),
		PlanningNotes:         nil,
		MonthlyTokenBudget:    normalizeTokenBudget(req.MonthlyTokenBudget),
		TeamID:                trimPtr(req.TeamID),
		AllowedTools:          sliceOrPresetJSON(req.AllowedTools, preset.AllowedTools),
		AllowedCommands:       sliceOrPresetJSON(req.AllowedCommands, preset.AllowedCommands),
		AllowedTargets:        sliceOrPresetJSON(req.AllowedTargets, preset.AllowedTargetTypes),
		Schedule:              trimPtr(req.Schedule),
		ApprovalMode:          approvalMode,
		MaxConcurrentRuns:     maxConcurrentRuns,
		DefaultInvocationMode: stringOrDefault(req.DefaultInvocationMode, preset.DefaultInvocationMode),
	}
	normalizeAgentRecord(agent)
	if err := validateTriggerModeForAgent(agent.TriggerMode, agent); err != nil {
		return nil, err
	}
	if err := s.validateModelRouting(agent); err != nil {
		return nil, err
	}

	if err := s.agentRepo.Create(ctx, agent); err != nil {
		return nil, err
	}

	newValue := agent.Name
	_ = s.activitySvc.Log(ctx, agent.WorkspaceID, "agent", agent.ID, &actorID, "created", nil, nil, &newValue, nil)

	s.publishSimpleEvent("created", "agent", agent.ID, agent.WorkspaceID, actorID)

	// Start cron schedule if configured.
	if s.runEngine != nil && agent.Schedule != nil && *agent.Schedule != "" {
		if err := s.runEngine.StartSchedule(ctx, agent.ID, agent.WorkspaceID, *agent.Schedule); err != nil {
			slog.ErrorContext(ctx, "failed to start agent schedule", "agent_id", agent.ID, "error", err)
		}
	}

	return agent, nil
}

// UpdateAgent updates an existing agent.
func (s *AgentService) UpdateAgent(ctx context.Context, workspaceID, id string, req model.UpdateAgentRequest, actorID string) (*model.Agent, error) {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.IsSystem {
		if req.PresetKey != nil && normalizePresetKey(*req.PresetKey) != model.AgentPresetEpicPlanner {
			return nil, fmt.Errorf("system planner preset cannot be changed")
		}
		if req.RuntimeKind != nil && strings.TrimSpace(*req.RuntimeKind) != "" && strings.TrimSpace(*req.RuntimeKind) != defaultRuntimeKindForPresetKey(model.AgentPresetEpicPlanner) {
			return nil, fmt.Errorf("system planner runtime cannot be changed")
		}
		if req.TeamID != nil && trimPtr(req.TeamID) != nil {
			return nil, fmt.Errorf("system planner cannot be restricted to a team")
		}
	}

	presetChanged := false
	previousPresetKey := normalizePresetKey(agent.PresetKey)
	if req.Name != nil {
		agent.Name = strings.TrimSpace(*req.Name)
	}
	if req.PresetKey != nil {
		agent.PresetKey = normalizePresetKey(*req.PresetKey)
		presetChanged = true
	}
	if req.Role != nil {
		agent.Role = strings.TrimSpace(*req.Role)
	}
	if req.Status != nil {
		agent.Status = *req.Status
	}
	preset, hasPreset := presetDefinitionForAgent(agent)
	if req.RuntimeKind != nil && strings.TrimSpace(*req.RuntimeKind) != "" {
		if err := validateRuntimeKind(*req.RuntimeKind); err != nil {
			return nil, err
		}
		agent.RuntimeKind = *req.RuntimeKind
	} else if presetChanged {
		if hasPreset && preset.RuntimeKind != "" {
			agent.RuntimeKind = preset.RuntimeKind
		} else {
			agent.RuntimeKind = "opencode"
		}
	}
	if req.Skills != nil {
		agent.Skills = req.Skills
	}
	if req.TriggerMode != nil && strings.TrimSpace(*req.TriggerMode) != "" {
		agent.TriggerMode = *req.TriggerMode
	} else if presetChanged {
		if hasPreset && preset.DefaultTriggerMode != "" {
			agent.TriggerMode = preset.DefaultTriggerMode
		} else {
			agent.TriggerMode = "manual"
		}
	}
	if req.Provider != nil {
		agent.Provider = trimPtr(req.Provider)
	}
	if req.Model != nil {
		agent.Model = trimPtr(req.Model)
	}
	if req.SystemPrompt != nil {
		agent.SystemPrompt = trimPtr(req.SystemPrompt)
	}
	if req.SystemPrompt != nil || req.PlanningNotes != nil || presetChanged {
		var legacyPlanningNotes *string
		if req.PlanningNotes != nil {
			legacyPlanningNotes = req.PlanningNotes
		}
		if presetChanged && req.SystemPrompt == nil {
			previousDefaultPrompt := defaultSystemPromptForPreset(previousPresetKey)
			if agent.SystemPrompt == nil || (previousDefaultPrompt != nil && *agent.SystemPrompt == *previousDefaultPrompt) {
				agent.SystemPrompt = nil
			}
		}
		agent.SystemPrompt = storedSystemPromptForPreset(agent.PresetKey, agent.SystemPrompt, legacyPlanningNotes)
		if req.PlanningNotes != nil || normalizePresetKey(agent.PresetKey) != model.AgentPresetEpicPlanner {
			agent.PlanningNotes = nil
		}
	}
	if req.MonthlyTokenBudget != nil {
		agent.MonthlyTokenBudget = normalizeTokenBudget(req.MonthlyTokenBudget)
	}
	if req.ActiveStoryID != nil {
		agent.ActiveStoryID = req.ActiveStoryID
	}
	if req.TeamID != nil {
		agent.TeamID = trimPtr(req.TeamID)
	}
	if req.AllowedTools != nil {
		agent.AllowedTools = normalizeJSONSlice(req.AllowedTools)
	} else if presetChanged && hasPreset {
		agent.AllowedTools = mustJSONStringSlice(preset.AllowedTools)
	}
	if req.AllowedCommands != nil {
		agent.AllowedCommands = normalizeJSONSlice(req.AllowedCommands)
	} else if presetChanged && hasPreset {
		agent.AllowedCommands = mustJSONStringSlice(preset.AllowedCommands)
	}
	if req.AllowedTargets != nil {
		agent.AllowedTargets = normalizeJSONSlice(req.AllowedTargets)
	} else if presetChanged && hasPreset {
		agent.AllowedTargets = mustJSONStringSlice(preset.AllowedTargetTypes)
	}
	scheduleChanged := false
	if req.Schedule != nil {
		oldSchedule := ""
		if agent.Schedule != nil {
			oldSchedule = *agent.Schedule
		}
		agent.Schedule = trimPtr(req.Schedule)
		newSchedule := ""
		if agent.Schedule != nil {
			newSchedule = *agent.Schedule
		}
		scheduleChanged = oldSchedule != newSchedule
	}
	if req.ApprovalMode != nil && *req.ApprovalMode != "" {
		agent.ApprovalMode = *req.ApprovalMode
	} else if presetChanged && hasPreset && preset.ApprovalMode != "" {
		agent.ApprovalMode = preset.ApprovalMode
	}
	if req.MaxConcurrentRuns != nil && *req.MaxConcurrentRuns > 0 {
		agent.MaxConcurrentRuns = *req.MaxConcurrentRuns
	}
	if req.DefaultInvocationMode != nil {
		agent.DefaultInvocationMode = strings.TrimSpace(*req.DefaultInvocationMode)
	} else if presetChanged && hasPreset {
		agent.DefaultInvocationMode = preset.DefaultInvocationMode
	}
	if presetChanged && (req.Role == nil || strings.TrimSpace(*req.Role) == "") {
		if hasPreset && preset.DefaultRole != "" {
			agent.Role = preset.DefaultRole
		} else {
			agent.Role = "Agent"
		}
	}
	if agent.IsSystem {
		agent.PresetKey = model.AgentPresetEpicPlanner
		agent.RuntimeKind = defaultRuntimeKindForPresetKey(model.AgentPresetEpicPlanner)
		agent.TriggerMode = defaultTriggerModeForPresetKey(model.AgentPresetEpicPlanner)
		agent.TeamID = nil
		agent.SystemPrompt = storedSystemPromptForPreset(model.AgentPresetEpicPlanner, agent.SystemPrompt, agent.PlanningNotes)
		agent.PlanningNotes = nil
	}
	normalizeAgentRecord(agent)
	if err := validateAgentPresetKey(agent.PresetKey); err != nil {
		return nil, err
	}
	if err := validateTriggerModeForAgent(agent.TriggerMode, agent); err != nil {
		return nil, err
	}
	if err := s.validateModelRouting(agent); err != nil {
		return nil, err
	}

	if err := s.agentRepo.Update(ctx, agent); err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, agent.WorkspaceID, "agent", agent.ID, &actorID, "updated", nil, nil, nil, nil)

	s.publishSimpleEvent("updated", "agent", agent.ID, agent.WorkspaceID, actorID)

	// Sync cron schedule if it changed.
	if scheduleChanged && s.runEngine != nil {
		_ = s.runEngine.StopSchedule(ctx, agent.ID)
		if agent.Schedule != nil && *agent.Schedule != "" {
			if err := s.runEngine.StartSchedule(ctx, agent.ID, agent.WorkspaceID, *agent.Schedule); err != nil {
				slog.ErrorContext(ctx, "failed to start agent schedule", "agent_id", agent.ID, "error", err)
			}
		}
	}

	return agent, nil
}

// DeleteAgent removes an agent.
func (s *AgentService) DeleteAgent(ctx context.Context, workspaceID, id, actorID string) error {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	if agent.IsSystem {
		return fmt.Errorf("system agents cannot be deleted")
	}

	// Stop any active cron schedule.
	if s.runEngine != nil {
		_ = s.runEngine.StopSchedule(ctx, id)
	}

	if err := s.agentRepo.Delete(ctx, workspaceID, id); err != nil {
		return err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "agent", id, &actorID, "deleted", nil, nil, nil, nil)

	s.publishSimpleEvent("deleted", "agent", id, workspaceID, actorID)

	return nil
}

// AssignAgentToStory assigns an agent to a story.
func (s *AgentService) AssignAgentToStory(ctx context.Context, workspaceID, storyID, agentID, actorID string) error {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	if err := validateAgentTarget(agent, "story"); err != nil {
		return err
	}

	story, err := s.storyRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return fmt.Errorf("get story: %w", err)
	}
	if story == nil {
		return fmt.Errorf("story not found")
	}

	story.AssignedAgentID = &agentID
	if err := s.storyRepo.Update(ctx, story); err != nil {
		return fmt.Errorf("update story: %w", err)
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "story", storyID, &actorID, "updated", strPtr("assigned_agent_id"), nil, &agent.Name, nil)

	s.publishSimpleEvent("updated", "story", storyID, workspaceID, actorID)

	if _, err := s.RunAgent(ctx, workspaceID, storyID, actorID); err != nil {
		if errors.Is(err, ErrStoryDeliveryTargetRequired) {
			return nil
		}
		return err
	}

	return nil
}

// ListAgentRuns returns runs for an agent.
func (s *AgentService) ListAgentRuns(ctx context.Context, workspaceID, agentID string, pagination model.PMPagination) ([]model.AgentRun, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	runs, total, err := s.runRepo.ListByAgent(ctx, workspaceID, agentID, pagination)
	if err != nil {
		return nil, 0, err
	}
	return s.reconcileStuckRuns(ctx, runs), total, nil
}

// ListWorkspaceRuns returns runs across the entire workspace.
func (s *AgentService) ListWorkspaceRuns(ctx context.Context, workspaceID string, pagination model.PMPagination) ([]model.AgentRun, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	runs, total, err := s.runRepo.ListByWorkspace(ctx, workspaceID, pagination)
	if err != nil {
		return nil, 0, err
	}
	return s.reconcileStuckRuns(ctx, runs), total, nil
}

// GetAgentRun returns a single run.
func (s *AgentService) GetAgentRun(ctx context.Context, workspaceID, runID string) (*model.AgentRun, error) {
	run, err := s.runRepo.GetByID(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("agent run not found")
	}
	if updated := s.reconcileStuckRun(ctx, run); updated != nil {
		run = updated
	}
	return run, nil
}

// ListRunArtifacts returns artifacts for a run.
func (s *AgentService) ListRunArtifacts(ctx context.Context, workspaceID, runID string) ([]model.AgentRunArtifact, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.artifactRepo.ListByRun(ctx, workspaceID, runID)
}

// ListRunMessages returns persisted conversation history for a run.
func (s *AgentService) ListRunMessages(ctx context.Context, workspaceID, runID string) ([]model.AgentRunMessage, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if s.runMessageRepo == nil {
		return []model.AgentRunMessage{}, nil
	}
	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("agent run not found")
	}
	return s.runMessageRepo.ListByRun(ctx, workspaceID, runID)
}

// ListTargetRuns returns runs for a specific target object.
func (s *AgentService) ListTargetRuns(ctx context.Context, workspaceID, targetType, targetID string) ([]model.AgentRun, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(targetType) == "" || strings.TrimSpace(targetID) == "" {
		return nil, fmt.Errorf("target_type and target_id are required")
	}
	runs, err := s.runRepo.ListByTarget(ctx, workspaceID, targetType, targetID)
	if err != nil {
		return nil, err
	}
	return s.reconcileStuckRuns(ctx, runs), nil
}

// RunAgent creates a new story-targeted agent run and starts its Temporal workflow.
func (s *AgentService) RunAgent(ctx context.Context, workspaceID, storyID, actorID string) (*model.AgentRun, error) {
	return s.StartTargetRun(ctx, workspaceID, "story", storyID, model.StartAgentRunRequest{}, actorID)
}

// RunEpicAgent starts a direct planner run for an epic.
func (s *AgentService) RunEpicAgent(ctx context.Context, workspaceID, epicID, actorID string, req model.StartAgentRunRequest) (*model.AgentRun, error) {
	return s.StartTargetRun(ctx, workspaceID, "epic", epicID, req, actorID)
}

// StartTargetRun starts a direct agent run for a supported target type.
func (s *AgentService) StartTargetRun(ctx context.Context, workspaceID, targetType, targetID string, req model.StartAgentRunRequest, actorID string) (*model.AgentRun, error) {
	switch targetType {
	case "story":
		story, err := s.storyRepo.GetRawByID(ctx, targetID)
		if err != nil {
			return nil, fmt.Errorf("get story: %w", err)
		}
		if story == nil || story.WorkspaceID != workspaceID {
			return nil, fmt.Errorf("story not found")
		}

		agentID := strings.TrimSpace(req.AgentID)
		if agentID == "" && story.AssignedAgentID != nil {
			agentID = strings.TrimSpace(*story.AssignedAgentID)
		}
		if agentID == "" {
			return nil, fmt.Errorf("no agent assigned to this story")
		}
		if story.AssignedAgentID == nil || *story.AssignedAgentID != agentID {
			if err := s.AssignAgentToStory(ctx, workspaceID, story.ID, agentID, actorID); err != nil {
				return nil, err
			}
			story.AssignedAgentID = &agentID
		}

		agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "story")
		if err != nil {
			return nil, err
		}
		resolved := worker.ResolveAgentProfile(agent, resolveInvocationMode(agent))

		var delivery *model.StoryDeliveryTarget
		if s.gitService != nil {
			delivery, err = s.gitService.ResolveStoryDeliveryTargetForRun(ctx, workspaceID, story.ID, resolved.RequiresRepo)
			if err != nil {
				return nil, err
			}
		}

		input := map[string]any{"story_id": story.ID}
		if req.AdditionalContext != nil && strings.TrimSpace(*req.AdditionalContext) != "" {
			input["additional_context"] = strings.TrimSpace(*req.AdditionalContext)
		}
		payload, _ := json.Marshal(input)

		run, err := s.createRun(ctx, createRunParams{
			workspaceID:    workspaceID,
			agent:          agent,
			targetType:     "story",
			targetID:       story.ID,
			storyID:        &story.ID,
			actorID:        &actorID,
			input:          payload,
			delivery:       delivery,
			invocationMode: resolveInvocationMode(agent),
		})
		if err != nil {
			return nil, err
		}
		if s.activitySvc != nil {
			_ = s.activitySvc.Log(ctx, workspaceID, "story", story.ID, &actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), nil)
		}
		s.publishRunEvent(run, actorID)
		return run, nil

	case "epic":
		epicWithStats, err := s.epicRepo.GetByID(ctx, targetID)
		if err != nil {
			return nil, fmt.Errorf("get epic: %w", err)
		}
		if epicWithStats == nil || epicWithStats.Epic.WorkspaceID != workspaceID {
			return nil, fmt.Errorf("epic not found")
		}
		epic := &epicWithStats.Epic

		agentID := strings.TrimSpace(req.AgentID)
		if agentID == "" {
			systemPlanner, err := s.ensureSystemProductPlannerAgent(ctx, workspaceID, actorID)
			if err != nil {
				return nil, err
			}
			agentID = systemPlanner.ID
		}

		agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "epic")
		if err != nil {
			if strings.TrimSpace(req.AgentID) == "" {
				systemPlanner, ensureErr := s.ensureSystemProductPlannerAgent(ctx, workspaceID, actorID)
				if ensureErr == nil && systemPlanner != nil && systemPlanner.ID != agentID {
					agent, err = s.requireRunnableAgent(ctx, workspaceID, systemPlanner.ID, "epic")
					if err == nil {
						agentID = systemPlanner.ID
					}
				}
			}
			if err != nil {
				return nil, err
			}
		}
		if agent.IsSystem && normalizePresetKey(agent.PresetKey) == model.AgentPresetEpicPlanner {
			refreshedPlanner, err := s.ensureSystemProductPlannerAgent(ctx, workspaceID, actorID)
			if err != nil {
				return nil, err
			}
			if refreshedPlanner != nil {
				agent = refreshedPlanner
				agentID = refreshedPlanner.ID
			}
		}
		input := map[string]any{"epic_id": epic.ID}
		if req.AdditionalContext != nil && strings.TrimSpace(*req.AdditionalContext) != "" {
			input["additional_context"] = strings.TrimSpace(*req.AdditionalContext)
		}
		payload, _ := json.Marshal(input)

		run, err := s.createRun(ctx, createRunParams{
			workspaceID:    workspaceID,
			agent:          agent,
			targetType:     "epic",
			targetID:       epic.ID,
			actorID:        &actorID,
			input:          payload,
			invocationMode: resolveInvocationMode(agent),
		})
		if err != nil {
			return nil, err
		}

		epic.LastPlanningRunID = &run.ID
		if err := s.epicRepo.Update(ctx, epic); err != nil {
			return nil, err
		}
		if s.activitySvc != nil {
			_ = s.activitySvc.Log(ctx, workspaceID, "epic", epic.ID, &actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), nil)
		}
		s.publishRunEvent(run, actorID)
		return run, nil

	default:
		return nil, fmt.Errorf("unsupported target type %q", targetType)
	}
}

// RunConversationAgent creates a new conversation-targeted agent run and starts its Temporal workflow.
func (s *AgentService) RunConversationAgent(ctx context.Context, workspaceID, conversationID, actorID string) (*model.AgentRun, error) {
	return s.runConversationAgent(ctx, workspaceID, conversationID, &actorID)
}

// RunConversationAgentAuto starts a support conversation run without a user actor.
func (s *AgentService) RunConversationAgentAuto(ctx context.Context, workspaceID, conversationID string) (*model.AgentRun, error) {
	return s.runConversationAgent(ctx, workspaceID, conversationID, nil)
}

func (s *AgentService) runConversationAgent(ctx context.Context, workspaceID, conversationID string, actorID *string) (*model.AgentRun, error) {
	conversation, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conversation.AssignedAgentID == nil || *conversation.AssignedAgentID == "" {
		return nil, fmt.Errorf("no agent assigned to this conversation")
	}

	agent, err := s.requireRunnableAgent(ctx, workspaceID, *conversation.AssignedAgentID, "support_conversation")
	if err != nil {
		return nil, err
	}
	input, _ := json.Marshal(map[string]any{
		"conversation_id": conversationID,
		"source":          conversation.Source,
	})

	run, err := s.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "support_conversation",
		targetID:       conversationID,
		conversationID: &conversationID,
		actorID:        actorID,
		input:          input,
		invocationMode: resolveInvocationMode(agent),
	})
	if err != nil {
		return nil, err
	}

	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversationID, actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), nil)
	}
	s.publishRunEvent(run, derefString(actorID))

	return run, nil
}

// CancelRun cancels a queued, running, or approval-pending agent run.
func (s *AgentService) CancelRun(ctx context.Context, workspaceID, runID, actorID string) (*model.AgentRun, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run.Status != "queued" && run.Status != "running" && run.Status != "awaiting_approval" {
		if run.Status != "awaiting_input" {
			return nil, fmt.Errorf("only queued, running, paused, or approval-pending runs can be cancelled")
		}
	}

	now := time.Now()
	run.Status = "cancelled"
	run.CompletedAt = &now
	run.ExecutionStage = strPtr("cancelled")
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	_ = s.runEngine.CancelRun(ctx, derefString(run.WorkflowID), derefString(run.WorkflowRunID))

	_ = s.markAgentIdle(ctx, workspaceID, run.AgentID)
	s.publishRunEvent(run, actorID)

	return run, nil
}

// SendRunMessage appends a user message to an awaiting-input run and resumes the workflow.
func (s *AgentService) SendRunMessage(ctx context.Context, workspaceID, runID, actorID string, req model.SendAgentRunMessageRequest) (*model.AgentRunMessage, error) {
	if s.runMessageRepo == nil {
		return nil, fmt.Errorf("run messages are not configured")
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run.Status != model.AgentRunStatusAwaitingInput {
		return nil, fmt.Errorf("run is not awaiting input")
	}

	message, err := s.createRunMessage(ctx, run, "user", "user_reply", content)
	if err != nil {
		return nil, err
	}
	if err := s.maybePersistApprovedInteractivePreview(ctx, run, actorID, content); err != nil {
		return nil, err
	}

	now := time.Now()
	run.Status = model.AgentRunStatusRunning
	run.ExecutionStage = strPtr("resuming")
	run.LastHeartbeatAt = &now
	run.CompletedAt = nil
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	if err := s.markAgentWorking(ctx, workspaceID, run.AgentID, run.StoryID); err != nil {
		return nil, err
	}
	_ = s.runEngine.SignalMessage(ctx, derefString(run.WorkflowID), derefString(run.WorkflowRunID), content)
	s.publishRunEvent(run, actorID)
	return message, nil
}

func (s *AgentService) maybePersistApprovedInteractivePreview(ctx context.Context, run *model.AgentRun, actorID, reply string) error {
	if s.runMessageRepo == nil || s.artifactRepo == nil || run == nil {
		return nil
	}
	if run.InvocationMode != model.InvocationModeInteractive {
		return nil
	}
	if !isExplicitInteractiveApprovalReply(reply) {
		return nil
	}

	messages, err := s.runMessageRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	sourceMessage, approval, preview, err := latestApprovalCheckpoint(messages)
	if err != nil {
		return err
	}
	if sourceMessage == nil || approval == nil || preview == nil {
		return nil
	}
	existingArtifacts, err := s.artifactRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	for _, artifact := range existingArtifacts {
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeApprovedPreview || artifact.InlineContent == nil {
			continue
		}
		var existing model.ApprovedRunPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &existing); err != nil {
			continue
		}
		if strings.TrimSpace(existing.SourceMessageID) == sourceMessage.ID {
			return nil
		}
	}

	payload := model.ApprovedRunPreview{
		Phase:           strings.TrimSpace(approval.Phase),
		ApprovalTitle:   strings.TrimSpace(approval.Title),
		ApprovalSummary: strings.TrimSpace(approval.Summary),
		PanelKey:        strings.TrimSpace(preview.PanelKey),
		PreviewTitle:    strings.TrimSpace(preview.Title),
		Format:          strings.TrimSpace(preview.Format),
		Content:         append(json.RawMessage(nil), preview.Content...),
		SourceMessageID: sourceMessage.ID,
		ApprovedBy:      strings.TrimSpace(actorID),
		ApprovedAt:      time.Now().UTC(),
	}
	return s.saveJSONArtifact(ctx, run, model.AgentRunArtifactTypeApprovedPreview, "json", payload)
}

func latestApprovalCheckpoint(messages []model.AgentRunMessage) (*model.AgentRunMessage, *model.ApprovalRequest, *worker.PublishedPreview, error) {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Role != "assistant" || len(message.ToolInvocations) == 0 || string(message.ToolInvocations) == "null" {
			continue
		}
		var invocations []model.ToolInvocation
		if err := json.Unmarshal(message.ToolInvocations, &invocations); err != nil {
			return nil, nil, nil, fmt.Errorf("parse assistant tool invocations: %w", err)
		}
		approvalIndex := -1
		for idx := len(invocations) - 1; idx >= 0; idx-- {
			if strings.TrimSpace(invocations[idx].ToolName) == worker.ToolRequestHumanApproval {
				approvalIndex = idx
				break
			}
		}
		if approvalIndex == -1 {
			continue
		}
		approval := worker.ExtractLatestHumanApprovalRequest(invocations[:approvalIndex+1])
		if approval == nil {
			continue
		}
		preview := worker.ExtractLatestPublishedPreview(invocations[:approvalIndex], "")
		if preview == nil {
			continue
		}
		return &message, approval, preview, nil
	}
	return nil, nil, nil, nil
}

func isExplicitInteractiveApprovalReply(reply string) bool {
	normalized := strings.ToLower(strings.TrimSpace(reply))
	if normalized == "" || strings.Contains(normalized, "?") {
		return false
	}

	for _, marker := range []string{
		"change", "changes", "revise", "revision", "update", "updates", "edit",
		"fix", "before approval", "before you", "before we", "add ", "but ",
		"however", "except", "can you", "could you", "would you", "question",
		"concern", "clarify", "clarification", "one more", "edge case",
	} {
		if strings.Contains(normalized, marker) {
			return false
		}
	}

	for _, marker := range []string{
		"approve", "approved", "approval", "looks good", "lgtm", "ship it",
		"good to go", "works for me", "sounds good", "go ahead",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// ApproveRun approves a pending outcome, publishing support drafts when requested.
func (s *AgentService) ApproveRun(ctx context.Context, workspaceID, runID, actorID string, req model.ApproveAgentRunRequest) (*model.AgentRun, error) {
	_ = req
	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run.ApprovalState != "pending" {
		return nil, fmt.Errorf("run does not require approval")
	}

	now := time.Now()
	run.ApprovalState = "approved"
	if run.Status == "awaiting_approval" {
		run.Status = model.AgentRunStatusRunning
		run.CompletedAt = nil
		run.LastHeartbeatAt = &now
		run.ExecutionStage = strPtr("approved")
	}
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	if err := s.markAgentWorking(ctx, workspaceID, run.AgentID, run.StoryID); err != nil {
		return nil, err
	}
	_ = s.runEngine.SignalApprove(ctx, derefString(run.WorkflowID), derefString(run.WorkflowRunID))
	s.publishRunEvent(run, actorID)

	// Evaluate automation rules for the run approval event
	if s.ruleEngine != nil && run.TargetType == "story" && run.StoryID != nil {
		story, storyErr := s.storyRepo.GetRawByID(ctx, *run.StoryID)
		if storyErr == nil && story != nil {
			s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
				WorkspaceID: run.WorkspaceID,
				TriggerType: model.TriggerAgentRunApproved,
				StoryID:     story.ID,
				StateID:     story.WorkflowStateID,
				AgentID:     run.AgentID,
				RunID:       run.ID,
			}, nil)
		}
	}

	return run, nil
}

func (s *AgentService) RequestRunChanges(ctx context.Context, workspaceID, runID, actorID string, req model.SendAgentRunRequestChangesRequest) (*model.AgentRun, error) {
	if s.runMessageRepo == nil {
		return nil, fmt.Errorf("run messages are not configured")
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run.Status != model.AgentRunStatusAwaitingApproval || run.ApprovalState != "pending" {
		return nil, fmt.Errorf("run is not awaiting approval")
	}

	if _, err := s.createRunMessage(ctx, run, "user", "request_changes", content); err != nil {
		return nil, err
	}

	now := time.Now()
	run.ApprovalState = "rejected"
	run.Status = model.AgentRunStatusRunning
	run.CompletedAt = nil
	run.LastHeartbeatAt = &now
	run.ExecutionStage = strPtr("resuming")
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	if err := s.markAgentWorking(ctx, workspaceID, run.AgentID, run.StoryID); err != nil {
		return nil, err
	}
	_ = s.runEngine.SignalMessage(ctx, derefString(run.WorkflowID), derefString(run.WorkflowRunID), content)
	s.publishRunEvent(run, actorID)
	return run, nil
}

// HandoffRun records an explicit handoff from a run to another agent or user.
func (s *AgentService) HandoffRun(ctx context.Context, workspaceID, runID, actorID string, req model.HandoffAgentRunRequest) (*model.AgentRun, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, fmt.Errorf("reason is required")
	}

	contextJSON := req.Context
	if contextJSON == nil {
		contextJSON = json.RawMessage("{}")
	}

	handoffType := "agent_to_human"
	if req.ToAgentID != nil && *req.ToAgentID != "" {
		handoffType = "agent_to_agent"
	}

	handoff := &model.AgentHandoff{
		WorkspaceID:    run.WorkspaceID,
		FromAgentID:    &run.AgentID,
		ToAgentID:      req.ToAgentID,
		ToUserID:       req.ToUserID,
		StoryID:        run.StoryID,
		ConversationID: run.ConversationID,
		RunID:          &run.ID,
		HandoffType:    handoffType,
		Reason:         req.Reason,
		Context:        contextJSON,
	}
	if err := s.handoffRepo.Create(ctx, handoff); err != nil {
		return nil, err
	}

	handoffState := "handoff_requested"
	if req.HandoffState != nil && *req.HandoffState != "" {
		handoffState = *req.HandoffState
	}
	run.HandoffState = &handoffState
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	_ = s.runEngine.SignalHandoff(ctx, derefString(run.WorkflowID), derefString(run.WorkflowRunID), req)

	payload, _ := json.MarshalIndent(map[string]any{
		"reason":       req.Reason,
		"to_agent_id":  req.ToAgentID,
		"to_user_id":   req.ToUserID,
		"handoff_type": handoffType,
	}, "", "  ")
	_ = s.saveArtifact(ctx, run, "handoff_note", "json", string(payload), 999999)

	s.publishRunEvent(run, actorID)

	return run, nil
}

type createRunParams struct {
	workspaceID    string
	agent          *model.Agent
	targetType     string
	targetID       string
	storyID        *string
	conversationID *string
	actorID        *string
	input          []byte
	delivery       *model.StoryDeliveryTarget
	invocationMode string
}

func (s *AgentService) createRun(ctx context.Context, params createRunParams) (*model.AgentRun, error) {
	activeRun, err := s.runRepo.FindActiveByTarget(ctx, params.workspaceID, params.targetType, params.targetID)
	if err != nil {
		return nil, err
	}
	if activeRun != nil {
		if updated := s.reconcileStuckRun(ctx, activeRun); updated != nil {
			activeRun = updated
		}
	}
	if activeRun != nil && (activeRun.Status == "queued" || activeRun.Status == "running" || activeRun.Status == "awaiting_input" || activeRun.Status == "awaiting_approval") {
		if activeRun.AgentID == params.agent.ID {
			return activeRun, nil
		}
		return nil, fmt.Errorf("an agent run is already active for this %s", params.targetType)
	}

	resolved := worker.ResolveAgentProfile(params.agent, params.invocationMode)
	approvalState := worker.ResolveApprovalState(resolved)
	taskQueue := resolved.Queue

	run := &model.AgentRun{
		WorkspaceID:       params.workspaceID,
		AgentID:           params.agent.ID,
		StoryID:           params.storyID,
		ConversationID:    params.conversationID,
		TargetType:        params.targetType,
		TargetID:          params.targetID,
		RuntimeKind:       params.agent.RuntimeKind,
		InvocationMode:    defaultString(params.invocationMode, model.InvocationModeAutonomous),
		ApprovalState:     approvalState,
		TriggeredByUserID: params.actorID,
		Status:            "queued",
		TaskQueue:         &taskQueue,
		RunnerPool:        &taskQueue,
		Input:             json.RawMessage(params.input),
		OutputSummary:     json.RawMessage("{}"),
	}
	if params.delivery != nil {
		run.DeliveryTargetID = &params.delivery.ID
		run.RepositoryID = params.delivery.RepositoryID
		run.RepoFullName = params.delivery.RepoFullName
		run.BaseBranch = params.delivery.BaseBranch
		run.WorkingBranch = params.delivery.WorkingBranch
	}
	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, err
	}

	params.agent.Status = "working"
	if params.storyID != nil {
		params.agent.ActiveStoryID = params.storyID
	} else {
		params.agent.ActiveStoryID = nil
	}
	_ = s.agentRepo.Update(ctx, params.agent)

	workflowID, workflowRunID, err := s.runEngine.StartRun(ctx, run)
	if err != nil {
		errMsg := err.Error()
		now := time.Now()
		run.Status = "failed"
		run.CompletedAt = &now
		run.ErrorMessage = &errMsg
		run.ExecutionStage = strPtr("failed_to_start")
		_ = s.runRepo.Update(ctx, run)
		_ = s.markAgentIdle(ctx, params.workspaceID, params.agent.ID)
		return nil, err
	}
	run.WorkflowID = &workflowID
	run.WorkflowRunID = &workflowRunID
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}

	return run, nil
}

func (s *AgentService) requireRunnableAgent(ctx context.Context, workspaceID, agentID, targetType string) (*model.Agent, error) {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("assigned agent not found")
	}
	normalizeAgentRecord(agent)
	if err := validateAgentTarget(agent, targetType); err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *AgentService) markAgentIdle(ctx context.Context, workspaceID, agentID string) error {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return err
	}
	agent.Status = "idle"
	agent.ActiveStoryID = nil
	return s.agentRepo.Update(ctx, agent)
}

func (s *AgentService) markAgentWorking(ctx context.Context, workspaceID, agentID string, storyID *string) error {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return err
	}
	agent.Status = "working"
	agent.ActiveStoryID = storyID
	return s.agentRepo.Update(ctx, agent)
}

func (s *AgentService) publishSimpleEvent(action, entity, entityID, workspaceID, actorID string) {
	if s.wsPublisher == nil {
		return
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      action,
		Entity:      entity,
		EntityID:    entityID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})
}

func (s *AgentService) publishRunEvent(run *model.AgentRun, actorID string) {
	if s.wsPublisher == nil {
		return
	}
	data, _ := json.Marshal(map[string]string{"status": run.Status})
	event := websocket.Event{
		Action:      "updated",
		Entity:      "agent_run",
		EntityID:    run.ID,
		WorkspaceID: run.WorkspaceID,
		ActorID:     actorID,
		ParentType:  run.TargetType,
		ParentID:    run.TargetID,
		Data:        data,
	}
	s.wsPublisher.Publish(event)
}

func (s *AgentService) publishRunMessageEvent(run *model.AgentRun, message *model.AgentRunMessage, actorID string) {
	if s.wsPublisher == nil || run == nil || message == nil {
		return
	}
	data, _ := json.Marshal(message)
	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "agent_run_message",
		EntityID:    message.ID,
		WorkspaceID: run.WorkspaceID,
		ActorID:     actorID,
		ParentType:  "agent_run",
		ParentID:    run.ID,
		Data:        data,
	})
}

func (s *AgentService) createRunMessage(ctx context.Context, run *model.AgentRun, role, messageType, content string) (*model.AgentRunMessage, error) {
	if s.runMessageRepo == nil {
		return nil, fmt.Errorf("run messages are not configured")
	}
	sequenceNo, err := s.runMessageRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, err
	}
	message := &model.AgentRunMessage{
		WorkspaceID: run.WorkspaceID,
		RunID:       run.ID,
		Role:        role,
		Content:     content,
		MessageType: messageType,
		SequenceNo:  sequenceNo,
	}
	if err := s.runMessageRepo.Create(ctx, message); err != nil {
		return nil, err
	}
	s.publishRunMessageEvent(run, message, run.AgentID)
	return message, nil
}

func resolveInvocationMode(agent *model.Agent) string {
	if agent == nil {
		return model.InvocationModeAutonomous
	}
	normalizeAgentRecord(agent)
	return agent.DefaultInvocationMode
}

func (s *AgentService) saveArtifact(ctx context.Context, run *model.AgentRun, artifactType, format, content string, seqNo int) error {
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: &content,
		Metadata:      json.RawMessage("{}"),
		SequenceNo:    seqNo,
	}
	return s.artifactRepo.Create(ctx, artifact)
}

func (s *AgentService) saveJSONArtifact(ctx context.Context, run *model.AgentRun, artifactType, format string, payload any) error {
	if s.artifactRepo == nil || run == nil {
		return nil
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal artifact %s: %w", artifactType, err)
	}
	seqNo, err := s.artifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	artifact := &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  artifactType,
		Format:        format,
		StorageMode:   "inline",
		InlineContent: strPtr(string(content)),
		Metadata:      json.RawMessage("{}"),
		SequenceNo:    seqNo,
	}
	return s.artifactRepo.Create(ctx, artifact)
}

func (s *AgentService) pushVisitorConversationRefresh(ctx context.Context, workspaceID string, conversation *model.SupportConversation) {
	if conversation == nil || conversation.AnonymousID == nil || strings.TrimSpace(*conversation.AnonymousID) == "" {
		return
	}
	conversations, err := s.conversationRepo.ListByAnonymousID(ctx, workspaceID, *conversation.AnonymousID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to refresh visitor conversations after support reply", "workspace_id", workspaceID, "conversation_id", conversation.ID, "error", err)
		return
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	listJSON, _ := json.Marshal(map[string]any{"conversations": conversations})
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_visitor_conversations",
		EntityID:    *conversation.AnonymousID,
		WorkspaceID: workspaceID,
		Data:        listJSON,
	})
}

// GetRunnerHealth returns the configured shared runner pools and active runs for a workspace.
func (s *AgentService) GetRunnerHealth(ctx context.Context, workspaceID string) temporalapp.RunnerHealth {
	if s.runEngine == nil {
		return temporalapp.RunnerHealth{}
	}

	health := s.runEngine.Health()
	if workspaceID == "" {
		return health
	}

	activeRuns, err := s.runRepo.ListActive(ctx, workspaceID, 100)
	if err != nil {
		return health
	}
	activeRuns = s.reconcileStuckRuns(ctx, activeRuns)

	queueIndex := make(map[string]int, len(health.Queues))
	for idx, queue := range health.Queues {
		queueIndex[queue.Name] = idx
	}

	now := time.Now()
	for _, run := range activeRuns {
		if run.Status != "queued" && run.Status != "running" && run.Status != "awaiting_approval" {
			continue
		}
		taskQueue := stringOrDefault(run.TaskQueue, temporalapp.QueueAutomation)
		idx, ok := queueIndex[taskQueue]
		if !ok {
			health.Queues = append(health.Queues, temporalapp.RunnerQueueHealth{Name: taskQueue})
			idx = len(health.Queues) - 1
			queueIndex[taskQueue] = idx
		}

		queue := health.Queues[idx]
		queue.ActiveRuns++
		switch run.Status {
		case "queued":
			queue.QueuedRuns++
		case "running":
			queue.RunningRuns++
		case "awaiting_approval":
			queue.AwaitingApprovalRuns++
		}
		if run.LastHeartbeatAt != nil && (queue.LatestHeartbeatAt == nil || run.LastHeartbeatAt.After(*queue.LatestHeartbeatAt)) {
			queue.LatestHeartbeatAt = run.LastHeartbeatAt
		}
		health.Queues[idx] = queue

		stale := false
		if run.Status == "running" {
			stale = run.LastHeartbeatAt == nil || now.Sub(*run.LastHeartbeatAt) > 2*time.Minute
		} else if run.Status == "queued" {
			stale = now.Sub(run.CreatedAt) > 10*time.Minute
		}

		health.ActiveRuns = append(health.ActiveRuns, temporalapp.RunnerActiveRun{
			ID:              run.ID,
			AgentID:         run.AgentID,
			TargetType:      run.TargetType,
			TargetID:        run.TargetID,
			Status:          run.Status,
			TaskQueue:       taskQueue,
			RunnerPool:      stringOrDefault(run.RunnerPool, taskQueue),
			ExecutionStage:  run.ExecutionStage,
			LastHeartbeatAt: run.LastHeartbeatAt,
			StartedAt:       run.StartedAt,
			CreatedAt:       run.CreatedAt,
			WorkflowID:      run.WorkflowID,
			Stale:           stale,
		})
	}

	return health
}

func stringOrDefault(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}

func mustJSONStringSlice(values []string) json.RawMessage {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		trimmed = append(trimmed, value)
	}
	if len(trimmed) == 0 {
		return json.RawMessage("[]")
	}
	payload, err := json.Marshal(trimmed)
	if err != nil {
		return json.RawMessage("[]")
	}
	return payload
}

func sliceOrPresetJSON(raw json.RawMessage, fallback []string) json.RawMessage {
	if len(raw) > 0 && string(raw) != "null" {
		return normalizeJSONSlice(raw)
	}
	return mustJSONStringSlice(fallback)
}

func (s *AgentService) reconcileStuckRuns(ctx context.Context, runs []model.AgentRun) []model.AgentRun {
	if len(runs) == 0 {
		return runs
	}
	for idx := range runs {
		run := runs[idx]
		if updated := s.reconcileStuckRun(ctx, &run); updated != nil {
			runs[idx] = *updated
		}
	}
	return runs
}

func (s *AgentService) reconcileStuckRun(ctx context.Context, run *model.AgentRun) *model.AgentRun {
	if updated := s.reconcileStaleQueuedRun(ctx, run, time.Now()); updated != nil {
		run = updated
	}
	if !shouldFailStuckPostRun(run, time.Now()) {
		return run
	}

	now := time.Now()
	errMsg := "run was marked failed after OpenCode finished but finalization did not reach a terminal state"
	run.Status = "failed"
	run.CompletedAt = &now
	run.ErrorMessage = &errMsg
	run.ExecutionStage = strPtr("failed")
	run.LastHeartbeatAt = &now
	if err := s.runRepo.Update(ctx, run); err != nil {
		return run
	}
	_ = s.markAgentIdle(ctx, run.WorkspaceID, run.AgentID)
	s.runRepo.Notify(ctx, run)
	return run
}

func (s *AgentService) reconcileStaleQueuedRun(ctx context.Context, run *model.AgentRun, now time.Time) *model.AgentRun {
	if !shouldInspectQueuedRun(run, now) {
		return run
	}

	workflowID := strings.TrimSpace(derefString(run.WorkflowID))
	workflowRunID := strings.TrimSpace(derefString(run.WorkflowRunID))

	switch {
	case workflowID == "":
		return s.failStaleRun(ctx, run, now, "run remained queued but no Temporal workflow execution was recorded")
	case s.runEngine == nil:
		return run
	}

	state, err := s.runEngine.DescribeRun(ctx, workflowID, workflowRunID)
	if err != nil {
		slog.WarnContext(ctx, "failed to inspect queued Temporal execution", "run_id", run.ID, "workflow_id", workflowID, "error", err)
		return run
	}
	if !state.Exists {
		return s.failStaleRun(ctx, run, now, "run remained queued but the Temporal workflow execution was not found")
	}
	if !state.Open {
		status := strings.ToLower(strings.TrimSpace(state.Status.String()))
		if status == "" || status == "workflow_execution_status_unspecified" {
			status = "closed"
		}
		return s.failStaleRun(ctx, run, now, fmt.Sprintf("run remained queued but the Temporal workflow is already %s", status))
	}
	return run
}

func (s *AgentService) failStaleRun(ctx context.Context, run *model.AgentRun, now time.Time, errMsg string) *model.AgentRun {
	if run == nil || s.runRepo == nil {
		return run
	}
	run.Status = "failed"
	run.CompletedAt = &now
	run.ErrorMessage = &errMsg
	run.ExecutionStage = strPtr("failed_to_start")
	run.LastHeartbeatAt = &now
	if err := s.runRepo.Update(ctx, run); err != nil {
		slog.WarnContext(ctx, "failed to persist stale run reconciliation", "run_id", run.ID, "error", err)
		return run
	}
	_ = s.markAgentIdle(ctx, run.WorkspaceID, run.AgentID)
	s.runRepo.Notify(ctx, run)
	return run
}

func shouldFailStuckPostRun(run *model.AgentRun, now time.Time) bool {
	if run == nil || run.Status != "running" || run.RuntimeKind != "opencode" {
		return false
	}
	if !isStuckPostRunStage(run.ExecutionStage) {
		return false
	}
	if run.LastHeartbeatAt == nil {
		return true
	}
	return now.Sub(*run.LastHeartbeatAt) > stuckPostRunThreshold
}

func shouldInspectQueuedRun(run *model.AgentRun, now time.Time) bool {
	if run == nil || run.Status != "queued" {
		return false
	}
	return now.Sub(run.CreatedAt) > staleQueuedRunThreshold
}

func isStuckPostRunStage(stage *string) bool {
	switch derefString(stage) {
	case "opencode_finished", "persisting_changes", "pushing_changes", "finalizing":
		return true
	default:
		return false
	}
}

func validateRuntimeKind(runtimeKind string) error {
	switch runtimeKind {
	case "opencode", "native_sdk":
		return nil
	default:
		return fmt.Errorf("runtime_kind must be one of opencode, native_sdk")
	}
}

func validateTriggerMode(triggerMode string) error {
	switch triggerMode {
	case "manual", "auto_on_assignment", "auto_on_event":
		return nil
	default:
		return fmt.Errorf("trigger_mode must be one of manual, auto_on_assignment, auto_on_event")
	}
}

func strPtr(s string) *string {
	return &s
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *AgentService) validateModelRouting(agent *model.Agent) error {
	if agent == nil {
		return nil
	}
	if agent.Provider == nil {
		if agent.Model == nil || strings.TrimSpace(*agent.Model) == "" {
			return nil
		}
		return fmt.Errorf("provider is required when model is set")
	}

	provider := normalizeModelProvider(*agent.Provider)
	if err := validateModelProvider(provider); err != nil {
		return err
	}
	if !s.isModelProviderConfigured(provider) {
		switch provider {
		case model.AgentModelProviderAnthropic:
			return fmt.Errorf("provider anthropic is not configured (missing ANTHROPIC_API_KEY)")
		case model.AgentModelProviderOpenAI:
			return fmt.Errorf("provider openai is not configured (missing OPENAI_API_KEY)")
		case model.AgentModelProviderOpenRouter:
			return fmt.Errorf("provider openrouter is not configured (missing OPENROUTER_API_KEY)")
		default:
			return fmt.Errorf("provider %s is not configured", provider)
		}
	}
	return nil
}

func (s *AgentService) isModelProviderConfigured(provider string) bool {
	switch normalizeModelProvider(provider) {
	case model.AgentModelProviderAnthropic:
		return strings.TrimSpace(s.anthropicAPIKey) != ""
	case model.AgentModelProviderOpenAI:
		return strings.TrimSpace(s.openAIAPIKey) != ""
	case model.AgentModelProviderOpenRouter:
		return strings.TrimSpace(s.openRouterAPIKey) != ""
	default:
		return false
	}
}
