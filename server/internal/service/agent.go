package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
const liveCodexPauseHeartbeatFreshThreshold = 2 * time.Minute
const defaultSystemEpicPlannerName = "Atlas"

func defaultSystemAgentNameForPresetKey(presetKey string) string {
	switch normalizePresetKey(presetKey) {
	case model.AgentPresetEpicPlanner:
		return defaultSystemEpicPlannerName
	case model.AgentPresetStoryPlanner:
		return "Scribe"
	case model.AgentPresetCRMOperator:
		return "CRM Operator"
	case model.AgentPresetSupportAgent:
		return "Echo"
	case model.AgentPresetCodeBuilder:
		return "Forge"
	case model.AgentPresetReviewAgent:
		return "Lens"
	default:
		return "Agent"
	}
}

// AgentService contains agent business logic.
type AgentService struct {
	agentRepo                  *repository.AgentRepository
	workspacePresetVersionRepo *repository.WorkspaceAgentPresetVersionRepository
	runRepo                    *repository.AgentRunRepository
	runMessageRepo             *repository.AgentRunMessageRepository
	artifactRepo               *repository.AgentRunArtifactRepository
	storyRepo                  *repository.PMStoryRepository
	storyLinkRepo              *repository.PMStoryLinkRepository
	epicRepo                   *repository.PMEpicRepository
	conversationRepo           *repository.SupportConversationRepository
	messageRepo                *repository.SupportMessageRepository
	handoffRepo                *repository.AgentHandoffRepository
	settingsRepo               *repository.SettingsRepository
	docsSpaceRepo              *repository.DocsSpaceRepository
	docsDocumentRepo           *repository.DocsDocumentRepository
	docsContentRepo            *repository.DocsContentRepository
	docsVersionRepo            *repository.DocsVersionRepository
	docsLinkRepo               *repository.DocsLinkRepository
	runEngine                  *temporalapp.RunEngine
	gitService                 *GitService
	storyService               *PMStoryService
	workflowService            *PMWorkflowService
	activitySvc                *PMActivityService
	wsPublisher                *websocket.Publisher
	ruleEngine                 *AutomationRuleEngine
	codexAuthManager           *worker.CodexAuthManager
	anthropicAPIKey            string
	openAIAPIKey               string
	openRouterAPIKey           string
	codexOpenAIAuthMode        string
	codexChatGPTOAuthEnabled   bool
	codexChatGPTAccessToken    string
	codexChatGPTAccountID      string
}

// NewAgentService creates a new AgentService.
func NewAgentService(
	agentRepo *repository.AgentRepository,
	workspacePresetVersionRepo *repository.WorkspaceAgentPresetVersionRepository,
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
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
		runRepo:                    runRepo,
		runMessageRepo:             runMessageRepo,
		artifactRepo:               artifactRepo,
		storyRepo:                  storyRepo,
		storyLinkRepo:              storyLinkRepo,
		epicRepo:                   epicRepo,
		conversationRepo:           conversationRepo,
		messageRepo:                messageRepo,
		handoffRepo:                handoffRepo,
		settingsRepo:               settingsRepo,
		docsSpaceRepo:              docsSpaceRepo,
		docsDocumentRepo:           docsDocumentRepo,
		docsContentRepo:            docsContentRepo,
		docsVersionRepo:            docsVersionRepo,
		docsLinkRepo:               docsLinkRepo,
		runEngine:                  runEngine,
		gitService:                 gitService,
		storyService:               storyService,
		activitySvc:                activitySvc,
		wsPublisher:                wsPublisher,
	}
}

func (s *AgentService) SetModelProviderConfig(
	anthropicAPIKey, openAIAPIKey, openRouterAPIKey string,
	codexOpenAIAuthMode string,
	codexChatGPTOAuthEnabled bool,
	codexChatGPTAccessToken string,
	codexChatGPTAccountID string,
) *AgentService {
	s.anthropicAPIKey = strings.TrimSpace(anthropicAPIKey)
	s.openAIAPIKey = strings.TrimSpace(openAIAPIKey)
	s.openRouterAPIKey = strings.TrimSpace(openRouterAPIKey)
	s.codexOpenAIAuthMode = strings.TrimSpace(codexOpenAIAuthMode)
	s.codexChatGPTOAuthEnabled = codexChatGPTOAuthEnabled
	s.codexChatGPTAccessToken = strings.TrimSpace(codexChatGPTAccessToken)
	s.codexChatGPTAccountID = strings.TrimSpace(codexChatGPTAccountID)
	return s
}

func (s *AgentService) SetCodexAuthManager(manager *worker.CodexAuthManager) *AgentService {
	s.codexAuthManager = manager
	return s
}

// SetRuleEngine sets the automation rule engine (breaks circular dependency).
func (s *AgentService) SetRuleEngine(engine *AutomationRuleEngine) *AgentService {
	s.ruleEngine = engine
	return s
}

func (s *AgentService) SetWorkflowService(workflowService *PMWorkflowService) *AgentService {
	s.workflowService = workflowService
	return s
}

// SeedWorkspaceDefaults creates workspace-scoped built-in agents.
func (s *AgentService) SeedWorkspaceDefaults(ctx context.Context, workspaceID, actorID string) error {
	return s.ensureWorkspaceBuiltInAgents(ctx, workspaceID, actorID)
}

func (s *AgentService) ensureSystemProductPlannerAgent(ctx context.Context, workspaceID, actorID string) (*model.Agent, error) {
	return s.ensureBuiltInAgent(ctx, workspaceID, actorID, model.AgentPresetEpicPlanner)
}

func (s *AgentService) ensureWorkspaceBuiltInAgents(ctx context.Context, workspaceID, actorID string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	for _, presetKey := range builtInPresetKeys() {
		if err := s.reconcileBuiltInPresetAgent(ctx, workspaceID, actorID, presetKey); err != nil {
			return err
		}
	}
	return nil
}

func (s *AgentService) ensureBuiltInAgent(ctx context.Context, workspaceID, actorID, presetKey string) (*model.Agent, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	presetKey = normalizePresetKey(presetKey)
	presetVersionKey := defaultPresetVersionKeyForPresetKey(presetKey)
	preset, ok := s.resolvePresetDefinition(ctx, workspaceID, presetKey, presetVersionKey)
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
		expectedName := defaultSystemAgentNameForPresetKey(presetKey)
		if strings.TrimSpace(existing.Name) != expectedName {
			existing.Name = expectedName
			changed = true
		}
		if existing.PresetVersionKey != presetVersionKey {
			existing.PresetVersionKey = presetVersionKey
			changed = true
		}
		expectedAllowedTools := mustJSONStringSlice(preset.AllowedTools)
		expectedAllowedCommands := mustJSONStringSlice(preset.AllowedCommands)
		expectedAllowedTargets := mustJSONStringSlice(preset.AllowedTargetTypes)
		if string(existing.AllowedTools) != string(expectedAllowedTools) {
			existing.AllowedTools = expectedAllowedTools
			changed = true
		}
		if string(existing.AllowedCommands) != string(expectedAllowedCommands) {
			existing.AllowedCommands = expectedAllowedCommands
			changed = true
		}
		if string(existing.AllowedTargets) != string(expectedAllowedTargets) {
			existing.AllowedTargets = expectedAllowedTargets
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
		Name:                  defaultSystemAgentNameForPresetKey(presetKey),
		PresetKey:             presetKey,
		PresetVersionKey:      presetVersionKey,
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
	if s.activitySvc != nil && strings.TrimSpace(actorID) != "" {
		newValue := agent.Name
		_ = s.activitySvc.Log(ctx, agent.WorkspaceID, "agent", agent.ID, &actorID, "created", nil, nil, &newValue, nil)
	}
	if strings.TrimSpace(actorID) != "" {
		s.publishSimpleEvent("created", "agent", agent.ID, agent.WorkspaceID, actorID)
	}
	return agent, nil
}

func (s *AgentService) reconcileBuiltInPresetAgent(ctx context.Context, workspaceID, actorID, presetKey string) error {
	presetKey = normalizePresetKey(presetKey)
	if presetKey == "" {
		return nil
	}
	if _, ok := agentPresetDefinition(presetKey); !ok {
		return fmt.Errorf("unsupported built-in preset %q", presetKey)
	}

	agents, err := s.agentRepo.List(ctx, workspaceID)
	if err != nil {
		return err
	}

	systemCandidates := make([]model.Agent, 0)
	for _, agent := range agents {
		if normalizePresetKey(agent.PresetKey) != presetKey {
			continue
		}
		if agent.IsSystem {
			systemCandidates = append(systemCandidates, agent)
		}
	}

	systemAgent, err := s.ensureBuiltInAgent(ctx, workspaceID, actorID, presetKey)
	if err != nil {
		return err
	}

	for _, candidate := range systemCandidates {
		if systemAgent != nil && candidate.ID == systemAgent.ID {
			continue
		}
		if err := s.agentRepo.Delete(ctx, workspaceID, candidate.ID); err != nil {
			return err
		}
		if s.activitySvc != nil && strings.TrimSpace(actorID) != "" {
			_ = s.activitySvc.Log(ctx, workspaceID, "agent", candidate.ID, &actorID, "deleted", nil, nil, nil, nil)
		}
	}

	return nil
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

func (s *AgentService) ListAgentPresets(ctx context.Context, workspaceID string) []model.AgentPresetDefinition {
	productPresets := ListAgentPresets()
	if strings.TrimSpace(workspaceID) == "" || s.workspacePresetVersionRepo == nil {
		return productPresets
	}
	workspaceVersions, err := s.workspacePresetVersionRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list workspace preset versions", "workspace_id", workspaceID, "error", err)
		return productPresets
	}
	if len(workspaceVersions) == 0 {
		return productPresets
	}
	byFamily := make(map[string]model.AgentPresetDefinition, len(productPresets))
	for _, preset := range productPresets {
		if preset.IsDefaultVersion {
			byFamily[preset.FamilyKey] = preset
		}
	}
	out := make([]model.AgentPresetDefinition, 0, len(productPresets)+len(workspaceVersions))
	out = append(out, productPresets...)
	for _, version := range workspaceVersions {
		base, ok := byFamily[normalizePresetKey(version.FamilyKey)]
		if !ok {
			continue
		}
		out = append(out, workspacePresetDefinition(base, version))
	}
	return out
}

func (s *AgentService) resolvePresetDefinition(ctx context.Context, workspaceID, presetKey, presetVersionKey string) (model.AgentPresetDefinition, bool) {
	familyKey := normalizePresetKey(presetKey)
	if familyKey == "" {
		return model.AgentPresetDefinition{}, false
	}
	if s.workspacePresetVersionRepo != nil && strings.TrimSpace(workspaceID) != "" && strings.TrimSpace(presetVersionKey) != "" {
		version, err := s.workspacePresetVersionRepo.GetByVersionKey(ctx, workspaceID, familyKey, presetVersionKey)
		if err != nil {
			slog.ErrorContext(ctx, "failed to resolve workspace preset version", "workspace_id", workspaceID, "family_key", familyKey, "version_key", presetVersionKey, "error", err)
		} else if version != nil {
			base, ok := agentPresetVersionDefinition(familyKey, defaultPresetVersionKeyForPresetKey(familyKey))
			if !ok {
				return model.AgentPresetDefinition{}, false
			}
			return workspacePresetDefinition(base, *version), true
		}
	}
	return agentPresetVersionDefinition(familyKey, presetVersionKey)
}

func (s *AgentService) CreateWorkspacePresetVersion(ctx context.Context, req model.CreateWorkspaceAgentPresetVersionRequest, actorID string) (*model.AgentPresetDefinition, error) {
	if s.workspacePresetVersionRepo == nil {
		return nil, fmt.Errorf("workspace preset version repository is not configured")
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	familyKey := normalizePresetKey(req.FamilyKey)
	if workspaceID == "" || familyKey == "" {
		return nil, fmt.Errorf("workspace_id and family_key are required")
	}
	baseVersionKey := strings.TrimSpace(stringOrDefault(req.SourceVersionKey, ""))
	basePreset, ok := s.resolvePresetDefinition(ctx, workspaceID, familyKey, baseVersionKey)
	if !ok {
		return nil, fmt.Errorf("source preset version %q is not supported for family %q", baseVersionKey, familyKey)
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		return nil, fmt.Errorf("label is required")
	}
	runtimeKind := strings.TrimSpace(stringOrDefault(req.RuntimeKind, basePreset.RuntimeKind))
	if runtimeKind == "" {
		runtimeKind = basePreset.RuntimeKind
	}
	if err := validateRuntimeKind(runtimeKind); err != nil {
		return nil, err
	}
	if !runtimeAllowedForPreset(familyKey, runtimeKind) {
		return nil, fmt.Errorf("runtime_kind %q is not supported for family %q", runtimeKind, familyKey)
	}
	supportedModes := supportedModesForRuntime(runtimeKind)
	if len(req.SupportedModes) > 0 {
		supportedModes = parseJSONStringSlice(req.SupportedModes)
	}
	normalizedSupportedModes, err := validateSupportedModes(runtimeKind, supportedModes)
	if err != nil {
		return nil, err
	}
	defaultInvocationMode := strings.TrimSpace(stringOrDefault(req.DefaultInvocationMode, basePreset.DefaultInvocationMode))
	if defaultInvocationMode == "" {
		defaultInvocationMode = basePreset.DefaultInvocationMode
	}
	if !slices.Contains(normalizedSupportedModes, defaultInvocationMode) {
		return nil, fmt.Errorf("default_invocation_mode %q must be included in supported_modes", defaultInvocationMode)
	}
	versionKey := fmt.Sprintf("%s_workspace_%d", familyKey, time.Now().UTC().UnixNano())
	version := &model.WorkspaceAgentPresetVersion{
		WorkspaceID:           workspaceID,
		FamilyKey:             familyKey,
		VersionKey:            versionKey,
		Label:                 label,
		Description:           trimPtr(req.Description),
		SourceVersionKey:      trimPtr(req.SourceVersionKey),
		RuntimeKind:           runtimeKind,
		Provider:              trimPtr(req.Provider),
		Model:                 trimPtr(req.Model),
		SystemPrompt:          trimPtr(req.SystemPrompt),
		AllowedTools:          mustJSONStringSlice(basePreset.AllowedTools),
		SupportedModes:        mustJSONStringSlice(normalizedSupportedModes),
		ApprovalMode:          "never",
		DefaultInvocationMode: defaultInvocationMode,
		CreatedBy:             trimPtr(&actorID),
	}
	if len(req.AllowedTools) > 0 {
		version.AllowedTools = normalizeJSONSlice(req.AllowedTools)
	}
	if version.SystemPrompt == nil {
		version.SystemPrompt = trimPtr(basePreset.SystemPrompt)
	}
	if version.Provider == nil {
		version.Provider = trimPtr(basePreset.Provider)
	}
	if version.Model == nil {
		version.Model = trimPtr(basePreset.Model)
	}
	if err := s.workspacePresetVersionRepo.Create(ctx, version); err != nil {
		return nil, err
	}
	definition := workspacePresetDefinition(basePreset, *version)
	return &definition, nil
}

// ListToolCatalog returns the tool catalog with categories and preset mappings.
func (s *AgentService) ListToolCatalog() model.ToolCatalogResponse {
	catalog := worker.ListToolCatalog()
	presets := ListAgentPresets()
	for idx := range catalog.Tools {
		tool := &catalog.Tools[idx]
		presetSet := make(map[string]struct{}, len(presets))
		presetKeys := make([]string, 0, len(presets))
		for _, preset := range presets {
			if slices.Contains(preset.AllowedTools, tool.Name) {
				if _, exists := presetSet[preset.Key]; exists {
					continue
				}
				presetSet[preset.Key] = struct{}{}
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
	if s.isModelProviderConfigured(model.AgentModelProviderOpenAI) || s.isCodexOpenAIConfigured() {
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
	if req.PresetKey != nil && strings.TrimSpace(*req.PresetKey) != "" {
		return nil, fmt.Errorf("custom agents cannot specify preset_key")
	}
	if req.PresetVersionKey != nil && strings.TrimSpace(*req.PresetVersionKey) != "" {
		return nil, fmt.Errorf("custom agents cannot specify preset_version_key")
	}

	skills := req.Skills
	if skills == nil {
		skills = json.RawMessage("[]")
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "Custom Agent"
	}
	runtimeKind := strings.TrimSpace(stringOrDefault(req.RuntimeKind, "opencode"))
	if runtimeKind == "" {
		runtimeKind = "opencode"
	}
	triggerMode := stringOrDefault(req.TriggerMode, "manual")
	if triggerMode == "" {
		triggerMode = "manual"
	}
	if err := validateRuntimeKind(runtimeKind); err != nil {
		return nil, err
	}

	approvalMode := "never"
	if req.ApprovalMode != nil && *req.ApprovalMode != "" {
		approvalMode = *req.ApprovalMode
	}
	maxConcurrentRuns := 1
	if req.MaxConcurrentRuns != nil && *req.MaxConcurrentRuns > 0 {
		maxConcurrentRuns = *req.MaxConcurrentRuns
	}

	agent := &model.Agent{
		WorkspaceID:            req.WorkspaceID,
		Name:                   strings.TrimSpace(req.Name),
		PresetKey:              "",
		PresetVersionKey:       "",
		SourcePresetKey:        "",
		SourcePresetVersionKey: "",
		Role:                   role,
		Status:                 "idle",
		RuntimeKind:            runtimeKind,
		Skills:                 skills,
		TriggerMode:            triggerMode,
		Provider:               trimPtr(req.Provider),
		Model:                  trimPtr(req.Model),
		SystemPrompt:           trimPtr(req.SystemPrompt),
		PlanningNotes:          nil,
		MonthlyTokenBudget:     normalizeTokenBudget(req.MonthlyTokenBudget),
		TeamID:                 trimPtr(req.TeamID),
		AllowedTools:           normalizeJSONSlice(req.AllowedTools),
		AllowedCommands:        normalizeJSONSlice(req.AllowedCommands),
		AllowedTargets:         sliceOrPresetJSON(req.AllowedTargets, []string{"story"}),
		Schedule:               trimPtr(req.Schedule),
		ApprovalMode:           approvalMode,
		MaxConcurrentRuns:      maxConcurrentRuns,
		DefaultInvocationMode:  stringOrDefault(req.DefaultInvocationMode, model.InvocationModeAutonomous),
	}
	normalizeAgentRecord(agent)
	if err := validateRuntimeForAgent(agent); err != nil {
		return nil, err
	}
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
	previousSchedule := ""
	if agent.Schedule != nil {
		previousSchedule = *agent.Schedule
	}
	if agent.IsSystem {
		systemPresetKey := normalizePresetKey(agent.PresetKey)
		if systemPresetKey == "" {
			systemPresetKey = model.AgentPresetEpicPlanner
		}
		if req.PresetKey != nil && normalizePresetKey(*req.PresetKey) != systemPresetKey {
			return nil, fmt.Errorf("system agent preset cannot be changed")
		}
		if req.TeamID != nil && trimPtr(req.TeamID) != nil {
			return nil, fmt.Errorf("system agent cannot be restricted to a team")
		}
	} else {
		if req.PresetKey != nil && strings.TrimSpace(*req.PresetKey) != "" {
			return nil, fmt.Errorf("custom agents cannot specify preset_key")
		}
		if req.PresetVersionKey != nil && strings.TrimSpace(*req.PresetVersionKey) != "" {
			return nil, fmt.Errorf("custom agents cannot specify preset_version_key")
		}
	}

	presetChanged := false
	previousPresetKey := normalizePresetKey(agent.EffectivePresetKey())
	if previousPresetKey == "" {
		previousPresetKey = defaultPresetKeyForAgent(agent.IsSystem)
	}
	if req.Name != nil {
		agent.Name = strings.TrimSpace(*req.Name)
	}
	if req.PresetKey != nil && agent.IsSystem {
		agent.PresetKey = normalizePresetKey(*req.PresetKey)
		presetChanged = true
	}
	if req.PresetVersionKey != nil && agent.IsSystem {
		agent.PresetVersionKey = normalizePresetVersionKey(*req.PresetVersionKey)
		presetChanged = true
	} else if req.PresetKey != nil && agent.IsSystem {
		agent.PresetVersionKey = defaultPresetVersionKeyForPresetKey(agent.PresetKey)
		presetChanged = true
	}
	if req.Role != nil {
		agent.Role = strings.TrimSpace(*req.Role)
	}
	if req.Status != nil {
		agent.Status = *req.Status
	}
	resolvedPresetKey := normalizePresetKey(agent.EffectivePresetKey())
	if resolvedPresetKey == "" {
		resolvedPresetKey = defaultPresetKeyForAgent(agent.IsSystem)
	}
	resolvedPresetVersionKey := normalizePresetVersionKey(agent.EffectivePresetVersionKey())
	if resolvedPresetVersionKey == "" {
		resolvedPresetVersionKey = defaultPresetVersionKeyForPresetKey(resolvedPresetKey)
	}
	preset, hasPreset := s.resolvePresetDefinition(ctx, workspaceID, resolvedPresetKey, resolvedPresetVersionKey)
	if agent.IsSystem && req.RuntimeKind != nil && strings.TrimSpace(*req.RuntimeKind) != "" {
		expectedRuntimeKind := defaultRuntimeKindForPresetKey(resolvedPresetKey)
		if hasPreset && strings.TrimSpace(preset.RuntimeKind) != "" {
			expectedRuntimeKind = strings.TrimSpace(preset.RuntimeKind)
		}
		if strings.TrimSpace(*req.RuntimeKind) != expectedRuntimeKind {
			return nil, fmt.Errorf("system agent runtime must match preset runtime %q", expectedRuntimeKind)
		}
	}
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
		agent.SystemPrompt = storedSystemPromptForPreset(resolvedPresetKey, agent.SystemPrompt, legacyPlanningNotes)
		if req.PlanningNotes != nil || resolvedPresetKey != model.AgentPresetEpicPlanner {
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
	if req.Schedule != nil {
		agent.Schedule = trimPtr(req.Schedule)
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
		systemPresetKey := normalizePresetKey(agent.PresetKey)
		if systemPresetKey == "" {
			systemPresetKey = model.AgentPresetEpicPlanner
		}
		agent.PresetKey = systemPresetKey
		agent.PresetVersionKey = resolvedPresetVersionKey
		if hasPreset && strings.TrimSpace(preset.RuntimeKind) != "" {
			agent.RuntimeKind = preset.RuntimeKind
		} else {
			agent.RuntimeKind = defaultRuntimeKindForPresetKey(systemPresetKey)
		}
		if hasPreset && strings.TrimSpace(preset.DefaultTriggerMode) != "" {
			agent.TriggerMode = preset.DefaultTriggerMode
		} else {
			agent.TriggerMode = defaultTriggerModeForPresetKey(systemPresetKey)
		}
		agent.TeamID = nil
		agent.Schedule = nil
		agent.ApprovalMode = "never"
		agent.SystemPrompt = storedSystemPromptForPreset(systemPresetKey, agent.SystemPrompt, agent.PlanningNotes)
		agent.PlanningNotes = nil
	} else {
		agent.SourcePresetKey = ""
		agent.SourcePresetVersionKey = ""
		agent.PresetKey = ""
		agent.PresetVersionKey = ""
	}
	normalizeAgentRecord(agent)
	if err := validateAgentPresetKey(agent.PresetKey); err != nil {
		return nil, err
	}
	if hasPreset {
		if err := validateRuntimeForAgentWithPreset(agent, &preset); err != nil {
			return nil, err
		}
	} else if err := validateRuntimeForAgent(agent); err != nil {
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
	finalSchedule := ""
	if agent.Schedule != nil {
		finalSchedule = *agent.Schedule
	}
	scheduleChanged := previousSchedule != finalSchedule
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
	return s.normalizeRunCollection(s.reconcileStuckRuns(ctx, runs)), total, nil
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
	return s.normalizeRunCollection(s.reconcileStuckRuns(ctx, runs)), total, nil
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
	model.NormalizeAgentRunPauseState(run)
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
	return s.normalizeRunCollection(s.reconcileStuckRuns(ctx, runs)), nil
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
	conversation, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
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
	if !model.IsAgentRunActiveStatus(run.Status) {
		return nil, fmt.Errorf("only queued, running, or paused runs can be cancelled")
	}

	now := time.Now()
	run.Status = "cancelled"
	run.PauseReason = model.AgentRunPauseReasonNone
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

func (s *AgentService) StartCodexDeviceCodeAuth(ctx context.Context, workspaceID, runID, actorID string) (*model.CodexAuthState, error) {
	if s.codexAuthManager == nil {
		return nil, fmt.Errorf("codex device-code auth is not configured")
	}

	run, agent, err := s.loadRunAndAgentForCodexAuth(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureRunSupportsCodexDeviceCode(run, agent); err != nil {
		return nil, err
	}

	authState, err := s.codexAuthManager.StartDeviceCode(ctx, run, agent, func(callbackCtx context.Context, state *model.CodexAuthState) {
		if state == nil {
			return
		}
		if err := s.applyCodexAuthState(callbackCtx, workspaceID, runID, actorID, state, state.State == model.CodexAuthStateConnected); err != nil {
			slog.ErrorContext(callbackCtx, "failed to apply codex auth state update",
				"error", err,
				"workspace_id", workspaceID,
				"run_id", runID,
				"state", state.State,
			)
		}
	})
	if err != nil {
		return nil, err
	}
	if err := s.applyCodexAuthState(ctx, workspaceID, runID, actorID, authState, authState != nil && authState.State == model.CodexAuthStateConnected); err != nil {
		return nil, err
	}
	return authState, nil
}

func (s *AgentService) CancelCodexDeviceCodeAuth(ctx context.Context, workspaceID, runID, actorID string) (*model.CodexAuthState, error) {
	if s.codexAuthManager == nil {
		return nil, fmt.Errorf("codex device-code auth is not configured")
	}

	run, agent, err := s.loadRunAndAgentForCodexAuth(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureRunSupportsCodexDeviceCode(run, agent); err != nil {
		return nil, err
	}

	return s.codexAuthManager.CancelDeviceCode(ctx, runID)
}

// ResumeRun resumes a paused interactive run using one generic intent path.
func (s *AgentService) ResumeRun(ctx context.Context, workspaceID, runID, actorID string, req model.ResumeAgentRunRequest) (*model.AgentRun, error) {
	run, _, err := s.resumeRunWithIntent(ctx, workspaceID, runID, actorID, req)
	if err != nil {
		return nil, err
	}
	if req.Intent == model.AgentRunResumeIntentApprove && s.ruleEngine != nil && run.TargetType == "story" && run.StoryID != nil {
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

// SendRunMessage appends a user message to a paused interactive run and resumes the workflow.
func (s *AgentService) SendRunMessage(ctx context.Context, workspaceID, runID, actorID string, req model.SendAgentRunMessageRequest) (*model.AgentRunMessage, error) {
	_, message, err := s.resumeRunWithIntent(ctx, workspaceID, runID, actorID, model.ResumeAgentRunRequest{
		Intent:  model.AgentRunResumeIntentReply,
		Content: req.Content,
	})
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, fmt.Errorf("resume did not create a run message")
	}
	return message, nil
}

func (s *AgentService) resumeRunWithIntent(ctx context.Context, workspaceID, runID, actorID string, req model.ResumeAgentRunRequest) (*model.AgentRun, *model.AgentRunMessage, error) {
	intent := normalizeResumeIntent(req.Intent)
	if intent == "" {
		return nil, nil, fmt.Errorf("intent is required")
	}

	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, nil, err
	}
	if run.Status == model.AgentRunStatusPaused && run.PauseReason == model.AgentRunPauseReasonHumanApproval && run.ApprovalState != "pending" {
		run.ApprovalState = "pending"
	}
	if !model.IsAgentRunPausedStatus(run.Status) {
		return nil, nil, fmt.Errorf("run is not paused for human input")
	}
	if run.PauseReason == model.AgentRunPauseReasonAuthentication {
		return nil, nil, fmt.Errorf("run is waiting for authentication")
	}
	model.NormalizeAgentRunPauseState(run)
	liveCodexPause, err := s.shouldUseLiveCodexPausePath(ctx, run)
	if err != nil {
		return nil, nil, err
	}

	var (
		message     *model.AgentRunMessage
		signal      temporalapp.RunResumeSignal
		replyText   string
		stage       = "resuming"
		approvalSet bool
	)
	previousStatus := run.Status
	previousPauseReason := run.PauseReason
	previousApprovalState := run.ApprovalState
	previousCompletedAt := run.CompletedAt
	previousExecutionStage := run.ExecutionStage
	previousHeartbeatAt := run.LastHeartbeatAt

	switch intent {
	case model.AgentRunResumeIntentReply:
		if s.runMessageRepo == nil {
			return nil, nil, fmt.Errorf("run messages are not configured")
		}
		replyText = strings.TrimSpace(req.Content)
		if replyText == "" {
			return nil, nil, fmt.Errorf("content is required")
		}
		message, err = s.createRunMessage(ctx, run, "user", "user_reply", replyText)
		if err != nil {
			return nil, nil, err
		}
		if err := s.maybePersistApprovedInteractivePreview(ctx, run, actorID, replyText); err != nil {
			return nil, nil, err
		}
		signalIntent := model.AgentRunResumeIntentReply
		if run.PauseReason == model.AgentRunPauseReasonHumanApproval {
			if isExplicitInteractiveApprovalReply(replyText) {
				run.ApprovalState = "approved"
				approvalSet = true
				stage = "approved"
				signalIntent = model.AgentRunResumeIntentApprove
			} else {
				run.ApprovalState = "rejected"
			}
		}
		signal = temporalapp.RunResumeSignal{
			Intent:  signalIntent,
			Content: replyText,
		}
	case model.AgentRunResumeIntentApprove:
		if run.ApprovalState != "pending" {
			return nil, nil, fmt.Errorf("run does not require approval")
		}
		replyText = strings.TrimSpace(req.Content)
		if replyText == "" {
			replyText = "approve"
		}
		if req.SendMessage || strings.TrimSpace(req.Content) != "" {
			if s.runMessageRepo == nil {
				return nil, nil, fmt.Errorf("run messages are not configured")
			}
			message, err = s.createRunMessage(ctx, run, "user", "approval", replyText)
			if err != nil {
				return nil, nil, err
			}
		}
		if err := s.maybePersistApprovedInteractivePreview(ctx, run, actorID, replyText); err != nil {
			return nil, nil, err
		}
		run.ApprovalState = "approved"
		approvalSet = true
		stage = "approved"
		signal = temporalapp.RunResumeSignal{
			Intent:  model.AgentRunResumeIntentApprove,
			Content: replyText,
		}
	case model.AgentRunResumeIntentRequestChanges:
		if s.runMessageRepo == nil {
			return nil, nil, fmt.Errorf("run messages are not configured")
		}
		replyText = strings.TrimSpace(req.Content)
		if replyText == "" {
			return nil, nil, fmt.Errorf("content is required")
		}
		if run.PauseReason != model.AgentRunPauseReasonHumanApproval || run.ApprovalState != "pending" {
			return nil, nil, fmt.Errorf("run is not awaiting approval")
		}
		message, err = s.createRunMessage(ctx, run, "user", "request_changes", replyText)
		if err != nil {
			return nil, nil, err
		}
		run.ApprovalState = "rejected"
		signal = temporalapp.RunResumeSignal{
			Intent:  model.AgentRunResumeIntentRequestChanges,
			Content: replyText,
		}
	default:
		return nil, nil, fmt.Errorf("unsupported intent %q", req.Intent)
	}

	now := time.Now()
	run.ExecutionStage = strPtr(stage)
	run.LastHeartbeatAt = &now
	run.CompletedAt = nil
	if !approvalSet && intent != model.AgentRunResumeIntentRequestChanges && run.ApprovalState != "pending" && run.ApprovalState != "rejected" {
		run.ApprovalState = "not_required"
	}
	if liveCodexPause {
		if err := s.runRepo.Update(ctx, run); err != nil {
			return nil, nil, err
		}
		s.publishRunEvent(run, actorID)
		return run, message, nil
	}
	run.Status = model.AgentRunStatusRunning
	run.PauseReason = model.AgentRunPauseReasonNone
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, nil, err
	}
	if err := s.markAgentWorking(ctx, workspaceID, run.AgentID, run.StoryID); err != nil {
		return nil, nil, err
	}
	workflowID := strings.TrimSpace(derefString(run.WorkflowID))
	if workflowID == "" {
		workflowID = temporalapp.WorkflowIDForRun(run.ID)
	}
	if err := s.runEngine.SignalResume(ctx, workflowID, strings.TrimSpace(derefString(run.WorkflowRunID)), signal); err != nil {
		slog.ErrorContext(ctx, "failed to signal agent run resume",
			"run_id", run.ID,
			"workflow_id", workflowID,
			"workflow_run_id", strings.TrimSpace(derefString(run.WorkflowRunID)),
			"intent", signal.Intent,
			"error", err)
		run.Status = previousStatus
		run.PauseReason = previousPauseReason
		run.ApprovalState = previousApprovalState
		run.CompletedAt = previousCompletedAt
		run.ExecutionStage = previousExecutionStage
		run.LastHeartbeatAt = previousHeartbeatAt
		if updateErr := s.runRepo.Update(ctx, run); updateErr != nil {
			return nil, nil, updateErr
		}
		_ = s.markAgentIdle(ctx, workspaceID, run.AgentID)
		return nil, nil, fmt.Errorf("resume workflow signal failed: %w", err)
	}
	s.publishRunEvent(run, actorID)
	return run, message, nil
}

func (s *AgentService) shouldUseLiveCodexPausePath(ctx context.Context, run *model.AgentRun) (bool, error) {
	if s == nil || run == nil || s.artifactRepo == nil {
		return false, nil
	}
	if strings.TrimSpace(run.RuntimeKind) != "codex" {
		return false, nil
	}
	if !model.IsAgentRunPausedStatus(run.Status) {
		return false, nil
	}
	switch run.PauseReason {
	case model.AgentRunPauseReasonHumanApproval, model.AgentRunPauseReasonHumanInput:
	default:
		return false, nil
	}
	if run.LastHeartbeatAt == nil || time.Since(run.LastHeartbeatAt.UTC()) > liveCodexPauseHeartbeatFreshThreshold {
		return false, nil
	}
	snapshot, err := worker.LoadCodexSessionSnapshot(ctx, s.artifactRepo, run)
	if err != nil {
		return false, err
	}
	return snapshot != nil && snapshot.HasPendingRequest, nil
}

func normalizeResumeIntent(intent string) string {
	switch strings.TrimSpace(strings.ToLower(intent)) {
	case model.AgentRunResumeIntentReply:
		return model.AgentRunResumeIntentReply
	case model.AgentRunResumeIntentApprove:
		return model.AgentRunResumeIntentApprove
	case model.AgentRunResumeIntentRequestChanges:
		return model.AgentRunResumeIntentRequestChanges
	default:
		return ""
	}
}

func (s *AgentService) loadRunAndAgentForCodexAuth(ctx context.Context, workspaceID, runID string) (*model.AgentRun, *model.Agent, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, nil, err
	}
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, run.AgentID)
	if err != nil {
		return nil, nil, err
	}
	if agent == nil {
		return nil, nil, fmt.Errorf("agent not found")
	}
	return run, agent, nil
}

func (s *AgentService) ensureRunSupportsCodexDeviceCode(run *model.AgentRun, agent *model.Agent) error {
	if run == nil || agent == nil {
		return fmt.Errorf("run and agent are required")
	}
	if strings.TrimSpace(run.RuntimeKind) != "codex" && strings.TrimSpace(agent.RuntimeKind) != "codex" {
		return fmt.Errorf("run does not use the codex runtime")
	}
	if run.Status == model.AgentRunStatusCompleted || run.Status == model.AgentRunStatusFailed || run.Status == model.AgentRunStatusCancelled {
		return fmt.Errorf("run is not active")
	}

	provider := model.AgentModelProviderOpenAI
	if agent.Provider != nil && strings.TrimSpace(*agent.Provider) != "" {
		provider = normalizeModelProvider(strings.TrimSpace(*agent.Provider))
	}
	if provider != model.AgentModelProviderOpenAI {
		return fmt.Errorf("codex device-code auth only supports provider openai")
	}
	if !s.isCodexOpenAIDeviceCodeEnabled() {
		return fmt.Errorf("CODEX_OPENAI_AUTH_MODE must be %q to use device-code auth", "chatgpt_device_code")
	}
	return nil
}

func (s *AgentService) applyCodexAuthState(ctx context.Context, workspaceID, runID, actorID string, authState *model.CodexAuthState, autoResume bool) error {
	if authState == nil {
		return nil
	}

	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return err
	}
	if err := s.appendCodexAuthArtifact(ctx, run, authState); err != nil {
		return err
	}

	now := time.Now()
	run.ErrorMessage = nil
	switch strings.TrimSpace(authState.State) {
	case model.CodexAuthStateConnected:
		run.Status = model.AgentRunStatusRunning
		run.PauseReason = model.AgentRunPauseReasonNone
		run.ExecutionStage = strPtr("auth_completed")
		run.LastHeartbeatAt = &now
		run.CompletedAt = nil
		if run.ApprovalState != "pending" && run.ApprovalState != "rejected" {
			run.ApprovalState = "not_required"
		}
		if err := s.runRepo.Update(ctx, run); err != nil {
			return err
		}
		if err := s.markAgentWorking(ctx, workspaceID, run.AgentID, run.StoryID); err != nil {
			return err
		}
		if autoResume && s.runEngine != nil {
			_ = s.runEngine.SignalResume(ctx, derefString(run.WorkflowID), derefString(run.WorkflowRunID), temporalapp.RunResumeSignal{
				Intent: model.AgentRunResumeIntentAuthCompleted,
			})
		}
	default:
		run.Status = model.AgentRunStatusPaused
		run.PauseReason = model.AgentRunPauseReasonAuthentication
		run.ExecutionStage = strPtr("awaiting_auth")
		run.LastHeartbeatAt = &now
		run.CompletedAt = nil
		if run.ApprovalState != "pending" && run.ApprovalState != "rejected" {
			run.ApprovalState = "not_required"
		}
		if err := s.runRepo.Update(ctx, run); err != nil {
			return err
		}
	}

	s.publishRunEvent(run, actorID)
	return nil
}

func (s *AgentService) appendCodexAuthArtifact(ctx context.Context, run *model.AgentRun, authState *model.CodexAuthState) error {
	if s.artifactRepo == nil || run == nil || authState == nil {
		return nil
	}
	sequenceNo, err := s.artifactRepo.NextSequence(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	content, err := json.Marshal(authState)
	if err != nil {
		return fmt.Errorf("marshal codex auth artifact: %w", err)
	}
	return s.artifactRepo.Create(ctx, &model.AgentRunArtifact{
		WorkspaceID:   run.WorkspaceID,
		RunID:         run.ID,
		ArtifactType:  model.AgentRunArtifactTypeCodexAuthState,
		Format:        "json",
		StorageMode:   "inline",
		InlineContent: strPtr(string(content)),
		Metadata:      json.RawMessage("{}"),
		SequenceNo:    sequenceNo,
	})
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
	existingArtifacts, err := s.artifactRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return err
	}
	sourceMessage, approval, preview, err := latestApprovalCheckpoint(messages, existingArtifacts)
	if err != nil {
		return err
	}
	if sourceMessage == nil || approval == nil {
		return nil
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
	if preview == nil {
		preview, err = latestRunPreviewArtifact(existingArtifacts, previewPanelKeyForApprovalPhase(approval.Phase))
		if err != nil {
			return err
		}
	}
	if preview == nil {
		return nil
	}

	content := append(json.RawMessage(nil), preview.Content...)
	if strings.EqualFold(strings.TrimSpace(approval.Phase), "stories") && strings.EqualFold(strings.TrimSpace(preview.Format), worker.PreviewFormatJSON) {
		normalizedContent, err := worker.NormalizeStoryPlanPreviewContent(content)
		if err != nil {
			if approvedPreviewDebugEnabled() {
				slog.ErrorContext(ctx, "approved story plan preview normalization failed during approval persistence",
					"run_id", run.ID,
					"workspace_id", run.WorkspaceID,
					"phase", strings.TrimSpace(approval.Phase),
					"panel_key", strings.TrimSpace(preview.PanelKey),
					"format", strings.TrimSpace(preview.Format),
					"content_preview", previewDebugSnippet(content, 1600),
					"error", err,
				)
			}
			return fmt.Errorf("approved story plan preview content must be valid JSON matching the canonical story-plan shape {summary, proposed_stories}; use story fields like name, description, story_type, acceptance_criteria, and dependency_refs")
		}
		content = normalizedContent
	}
	if approvedPreviewDebugEnabled() {
		slog.InfoContext(ctx, "persisting approved interactive preview",
			"run_id", run.ID,
			"workspace_id", run.WorkspaceID,
			"phase", strings.TrimSpace(approval.Phase),
			"panel_key", strings.TrimSpace(preview.PanelKey),
			"format", strings.TrimSpace(preview.Format),
			"source_message_id", sourceMessage.ID,
			"content_preview", previewDebugSnippet(content, 1600),
		)
	}

	payload := model.ApprovedRunPreview{
		Phase:           strings.TrimSpace(approval.Phase),
		ApprovalTitle:   strings.TrimSpace(approval.Title),
		ApprovalSummary: strings.TrimSpace(approval.Summary),
		PanelKey:        strings.TrimSpace(preview.PanelKey),
		PreviewTitle:    strings.TrimSpace(preview.Title),
		Format:          strings.TrimSpace(preview.Format),
		Content:         content,
		SourceMessageID: sourceMessage.ID,
		ApprovedBy:      strings.TrimSpace(actorID),
		ApprovedAt:      time.Now().UTC(),
	}
	return s.saveJSONArtifact(ctx, run, model.AgentRunArtifactTypeApprovedPreview, "json", payload)
}

func latestApprovalCheckpoint(messages []model.AgentRunMessage, artifacts []model.AgentRunArtifact) (*model.AgentRunMessage, *model.ApprovalRequest, *worker.PublishedPreview, error) {
	return latestApprovalCheckpointFromArtifacts(messages, artifacts)
}

func latestApprovalCheckpointFromArtifacts(messages []model.AgentRunMessage, artifacts []model.AgentRunArtifact) (*model.AgentRunMessage, *model.ApprovalRequest, *worker.PublishedPreview, error) {
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeHumanApprovalRequest || artifact.InlineContent == nil {
			continue
		}

		var approval model.ApprovalRequest
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &approval); err != nil {
			return nil, nil, nil, fmt.Errorf("parse approval request artifact: %w", err)
		}
		if strings.TrimSpace(approval.Title) == "" {
			continue
		}

		assistantSequenceNo := artifactAssistantMessageSequenceNo(artifact)
		if assistantSequenceNo <= 0 {
			continue
		}
		sourceMessage := findAssistantMessageBySequence(messages, assistantSequenceNo)
		if sourceMessage == nil {
			continue
		}

		preview, err := latestRunPreviewArtifactForAssistantSequence(artifacts, assistantSequenceNo, previewPanelKeyForApprovalPhase(approval.Phase))
		if err != nil {
			return nil, nil, nil, err
		}
		if preview == nil {
			preview, err = latestRunPreviewArtifact(artifacts, previewPanelKeyForApprovalPhase(approval.Phase))
			if err != nil {
				return nil, nil, nil, err
			}
		}
		return sourceMessage, &approval, preview, nil
	}
	return nil, nil, nil, nil
}

func previewPanelKeyForApprovalPhase(phase string) string {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "prd":
		return "prd_draft"
	case "story_doc":
		return "story_plan_doc"
	case "stories":
		return "story_plan"
	default:
		return ""
	}
}

func latestRunPreviewArtifact(artifacts []model.AgentRunArtifact, panelKey string) (*worker.PublishedPreview, error) {
	targetKey := strings.ToLower(strings.TrimSpace(panelKey))
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != worker.RunPreviewArtifactType || artifact.InlineContent == nil {
			continue
		}

		var payload worker.PublishedPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			return nil, fmt.Errorf("parse run preview artifact: %w", err)
		}
		if targetKey != "" && strings.ToLower(strings.TrimSpace(payload.PanelKey)) != targetKey {
			continue
		}
		return &payload, nil
	}
	return nil, nil
}

func latestRunPreviewArtifactForAssistantSequence(artifacts []model.AgentRunArtifact, assistantSequenceNo int, panelKey string) (*worker.PublishedPreview, error) {
	if assistantSequenceNo <= 0 {
		return nil, nil
	}
	targetKey := strings.ToLower(strings.TrimSpace(panelKey))
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != worker.RunPreviewArtifactType || artifact.InlineContent == nil {
			continue
		}
		if artifactAssistantMessageSequenceNo(artifact) != assistantSequenceNo {
			continue
		}
		var payload worker.PublishedPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			return nil, fmt.Errorf("parse run preview artifact: %w", err)
		}
		if targetKey != "" && strings.ToLower(strings.TrimSpace(payload.PanelKey)) != targetKey {
			continue
		}
		return &payload, nil
	}
	return nil, nil
}

func artifactAssistantMessageSequenceNo(artifact model.AgentRunArtifact) int {
	if len(artifact.Metadata) == 0 || string(artifact.Metadata) == "null" {
		return 0
	}
	var metadata struct {
		AssistantMessageSequenceNo int `json:"assistant_message_sequence_no"`
	}
	if err := json.Unmarshal(artifact.Metadata, &metadata); err != nil {
		return 0
	}
	return metadata.AssistantMessageSequenceNo
}

func findAssistantMessageBySequence(messages []model.AgentRunMessage, sequenceNo int) *model.AgentRunMessage {
	if sequenceNo <= 0 {
		return nil
	}
	for i := range messages {
		if messages[i].SequenceNo == sequenceNo && strings.TrimSpace(messages[i].Role) == "assistant" {
			return &messages[i]
		}
	}
	return nil
}

func approvedPreviewDebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_PREVIEW_DEBUG"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func previewDebugSnippet(raw json.RawMessage, max int) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return ""
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err == nil {
		trimmed = compact.String()
	}
	if max > 0 && len(trimmed) > max {
		return trimmed[:max] + "...(truncated)"
	}
	return trimmed
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
	return s.ResumeRun(ctx, workspaceID, runID, actorID, model.ResumeAgentRunRequest{
		Intent:      model.AgentRunResumeIntentApprove,
		SendMessage: req.SendMessage,
	})
}

func (s *AgentService) RequestRunChanges(ctx context.Context, workspaceID, runID, actorID string, req model.SendAgentRunRequestChangesRequest) (*model.AgentRun, error) {
	return s.ResumeRun(ctx, workspaceID, runID, actorID, model.ResumeAgentRunRequest{
		Intent:  model.AgentRunResumeIntentRequestChanges,
		Content: req.Content,
	})
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
	if activeRun != nil && model.IsAgentRunActiveStatus(activeRun.Status) {
		if activeRun.AgentID == params.agent.ID {
			model.NormalizeAgentRunPauseState(activeRun)
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
		PauseReason:       model.AgentRunPauseReasonNone,
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
		run.PauseReason = model.AgentRunPauseReasonNone
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
	status, pauseReason := model.NormalizeAgentRunStatus(run.Status, run.PauseReason, run.ApprovalState, run.ExecutionStage)
	data, _ := json.Marshal(map[string]string{
		"status":       status,
		"pause_reason": pauseReason,
	})
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
	activeRuns = s.normalizeRunCollection(activeRuns)

	queueIndex := make(map[string]int, len(health.Queues))
	for idx, queue := range health.Queues {
		queueIndex[queue.Name] = idx
	}

	now := time.Now()
	for _, run := range activeRuns {
		if !model.IsAgentRunActiveStatus(run.Status) {
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
		case model.AgentRunStatusPaused:
			if run.PauseReason == model.AgentRunPauseReasonHumanApproval {
				queue.AwaitingApprovalRuns++
			}
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
	run.PauseReason = model.AgentRunPauseReasonNone
	run.CompletedAt = &now
	run.ErrorMessage = &errMsg
	run.ExecutionStage = strPtr("failed")
	run.LastHeartbeatAt = &now
	if err := s.runRepo.Update(ctx, run); err != nil {
		return run
	}
	_ = s.markAgentIdle(ctx, run.WorkspaceID, run.AgentID)
	s.runRepo.Notify(ctx, run)
	model.NormalizeAgentRunPauseState(run)
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
	run.PauseReason = model.AgentRunPauseReasonNone
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
	if run == nil || run.Status != "running" || !requiresPostRunReconciliation(run.RuntimeKind) {
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
	case "opencode_finished", "codex_finished", "persisting_changes", "pushing_changes", "finalizing":
		return true
	default:
		return false
	}
}

func requiresPostRunReconciliation(runtimeKind string) bool {
	switch strings.TrimSpace(runtimeKind) {
	case "opencode", "codex":
		return true
	default:
		return false
	}
}

func (s *AgentService) normalizeRunCollection(runs []model.AgentRun) []model.AgentRun {
	for idx := range runs {
		model.NormalizeAgentRunPauseState(&runs[idx])
	}
	return runs
}

func validateRuntimeKind(runtimeKind string) error {
	switch runtimeKind {
	case "opencode", "codex", "native_sdk":
		return nil
	default:
		return fmt.Errorf("runtime_kind must be one of opencode, codex, native_sdk")
	}
}

func validateSupportedModes(runtimeKind string, modes []string) ([]string, error) {
	allowed := supportedModesForRuntime(runtimeKind)
	if len(modes) == 0 {
		return allowed, nil
	}
	seen := make(map[string]struct{}, len(modes))
	normalized := make([]string, 0, len(modes))
	for _, mode := range modes {
		trimmed := strings.TrimSpace(mode)
		if trimmed == "" {
			continue
		}
		if !slices.Contains(allowed, trimmed) {
			return nil, fmt.Errorf("supported_mode %q is not valid for runtime_kind %q", trimmed, runtimeKind)
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("supported_modes must include at least one mode")
	}
	return normalized, nil
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
	if err := s.validateRuntimeProviderCompatibility(agent); err != nil {
		return err
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
	if strings.TrimSpace(agent.RuntimeKind) == "codex" && provider == model.AgentModelProviderOpenAI {
		if !s.isCodexOpenAIConfigured() {
			return fmt.Errorf("provider openai is not configured for codex (requires OPENAI_API_KEY or Helpin-managed ChatGPT OAuth)")
		}
		return nil
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

func (s *AgentService) validateRuntimeProviderCompatibility(agent *model.Agent) error {
	if agent == nil || strings.TrimSpace(agent.RuntimeKind) != "codex" {
		return nil
	}

	if agent.Provider == nil || strings.TrimSpace(*agent.Provider) == "" {
		if !s.isCodexOpenAIConfigured() && strings.TrimSpace(s.openRouterAPIKey) == "" {
			return fmt.Errorf("runtime_kind codex requires OPENAI_API_KEY, Helpin-managed ChatGPT OAuth, or OPENROUTER_API_KEY to be configured")
		}
		return nil
	}

	switch normalizeModelProvider(*agent.Provider) {
	case model.AgentModelProviderOpenAI:
		if !s.isCodexOpenAIConfigured() {
			return fmt.Errorf("runtime_kind codex with provider openai requires OPENAI_API_KEY or Helpin-managed ChatGPT OAuth")
		}
	case model.AgentModelProviderOpenRouter, model.AgentModelProviderOpenRouterResponses:
		if strings.TrimSpace(s.openRouterAPIKey) == "" {
			return fmt.Errorf("runtime_kind codex with provider openrouter requires OPENROUTER_API_KEY")
		}
	case model.AgentModelProviderAnthropic:
		return fmt.Errorf("runtime_kind codex requires provider openai or openrouter")
	default:
		return fmt.Errorf("runtime_kind codex requires provider openai or openrouter")
	}

	return nil
}

func (s *AgentService) isCodexOpenAIConfigured() bool {
	switch strings.ToLower(strings.TrimSpace(s.codexOpenAIAuthMode)) {
	case "", "api_key", "api-key", "api":
		return strings.TrimSpace(s.openAIAPIKey) != ""
	case "chatgpt_oauth", "oauth", "chatgpt", "chatgpt-auth":
		return s.codexChatGPTOAuthEnabled && strings.TrimSpace(s.codexChatGPTAccessToken) != "" && strings.TrimSpace(s.codexChatGPTAccountID) != ""
	case "chatgpt_device_code", "device_code", "chatgpt-device", "chatgpt-device-code", "chatgpt-managed":
		return true
	default:
		return false
	}
}

func (s *AgentService) isCodexOpenAIDeviceCodeEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(s.codexOpenAIAuthMode)) {
	case "chatgpt_device_code", "device_code", "chatgpt-device", "chatgpt-device-code", "chatgpt-managed":
		return true
	default:
		return false
	}
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
