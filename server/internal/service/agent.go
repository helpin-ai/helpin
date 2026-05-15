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
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/automationcatalog"
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

var ErrAssignedAgentNotFound = errors.New("assigned agent not found")

const supportAutoTriggerType = "support.auto"

var (
	ErrWorkspacePresetVersionNotFound = errors.New("workspace preset version not found")
	ErrWorkspacePresetVersionPinned   = errors.New("workspace preset version is pinned")
)

func defaultSystemAgentNameForPresetKey(presetKey string) string {
	switch normalizePresetKey(presetKey) {
	case model.AgentPresetEpicPlanner:
		return defaultSystemEpicPlannerName
	case model.AgentPresetTaskPlanner:
		return "Scribe"
	case model.AgentPresetCRMOperator:
		return "Beacon"
	case model.AgentPresetSupportAgent:
		return "Echo"
	case model.AgentPresetDocumentationAgent:
		return "Quill"
	case model.AgentPresetCodeBuilder:
		return "Forge"
	case model.AgentPresetReviewAgent:
		return "Lens"
	case model.AgentPresetCommandAgent:
		return "Command Agent"
	default:
		return "Agent"
	}
}

func manualRunTriggerContext() *model.AgentRunTriggerContext {
	now := time.Now().UTC()
	return &model.AgentRunTriggerContext{
		Source:      model.AgentRunTriggerSourceManual,
		TriggerType: model.AgentRunTriggerTypeManual,
		FiredAt:     &now,
	}
}

func systemRunTriggerContext(triggerType string) *model.AgentRunTriggerContext {
	triggerType = strings.TrimSpace(triggerType)
	if triggerType == "" {
		return nil
	}
	now := time.Now().UTC()
	return &model.AgentRunTriggerContext{
		Source:      model.AgentRunTriggerSourceSystem,
		TriggerType: triggerType,
		FiredAt:     &now,
	}
}

func agentRunActivityMetadata(agent *model.Agent, run *model.AgentRun, action string) map[string]interface{} {
	md := map[string]interface{}{
		"run_action":   action,
		"runtime_kind": string(run.RuntimeKind),
		"run_id":       run.ID,
	}
	if agent != nil {
		md["agent_id"] = agent.ID
		md["agent_name"] = agent.Name
		if agent.PresetKey != "" {
			md["agent_preset_key"] = agent.PresetKey
		}
	}
	return md
}

func buildAgentRunInputPayload(targetType, targetID string, trigger *model.AgentRunTriggerContext, event *model.AgentRunEventContext, output *model.AgentRunOutputContext, additionalContext *string, allowedTools []string) ([]byte, error) {
	payload := model.AgentRunInputPayload{
		Trigger:      trigger,
		Event:        event,
		Output:       output,
		AllowedTools: normalizeStringSlice(allowedTools),
	}
	payload.SetTarget(targetType, targetID)
	if additionalContext != nil {
		payload.AdditionalContext = strings.TrimSpace(*additionalContext)
	}
	return json.Marshal(payload)
}

func validateRunAllowedTools(requested []string, agent *model.Agent) error {
	requested = normalizeStringSlice(requested)
	if len(requested) == 0 {
		return nil
	}
	allowedSet := map[string]bool{}
	for _, tool := range parseJSONStringSlice(agent.AllowedTools) {
		allowedSet[tool] = true
	}
	// Output-bound review tools are safe to grant per run. They validate the
	// run output context before doing anything, so older document agents can use
	// new review-candidate flows without requiring an agent row migration first.
	allowedSet["publish_document_change_proposal"] = true
	allowedSet["publish_ai_section_candidate"] = true
	for _, tool := range requested {
		if !allowedSet[tool] {
			return fmt.Errorf("tool %q is not allowed for agent %s", tool, strings.TrimSpace(agent.Name))
		}
	}
	return nil
}

func buildContinuationAdditionalContext(run *model.AgentRun, content string) string {
	content = strings.TrimSpace(content)
	if run == nil {
		return content
	}

	sections := []string{
		fmt.Sprintf("Continue from the previous run on the same %s. Reuse prior progress, artifacts, and transcript context instead of restarting from scratch unless necessary.", strings.TrimSpace(run.TargetType)),
	}
	if parentID := strings.TrimSpace(run.ID); parentID != "" {
		sections = append(sections, fmt.Sprintf("Previous run ID: %s", parentID))
	}
	if reason := strings.TrimSpace(derefString(run.ErrorMessage)); reason != "" {
		sections = append(sections, "Previous run failure reason:\n"+reason)
	}
	if content != "" {
		sections = append(sections, "Human follow-up:\n"+content)
	} else {
		sections = append(sections, "Human follow-up:\nRetry the work and continue from the previous progress.")
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

// AgentService contains agent business logic.
type AgentService struct {
	agentRepo                  *repository.AgentRepository
	agentTemplateRepo          *repository.AgentTemplateRepository
	workspacePresetVersionRepo *repository.WorkspaceAgentPresetVersionRepository
	runRepo                    *repository.AgentRunRepository
	commandBarPlanRepo         *repository.CommandBarPlanRepository
	triggerExecutionRepo       *repository.AgentTriggerExecutionRepository
	runMessageRepo             *repository.AgentRunMessageRepository
	artifactRepo               *repository.AgentRunArtifactRepository
	interactionRepo            *repository.AgentRunInteractionRepository
	sessionSnapshotRepo        *repository.CodingSessionStateSnapshotRepository
	taskRepo                   *repository.PMTaskRepository
	taskLinkRepo               *repository.PMTaskLinkRepository
	epicRepo                   *repository.PMEpicRepository
	conversationRepo           *repository.SupportConversationRepository
	messageRepo                *repository.SupportMessageRepository
	supportCoverageSvc         *SupportCoverageService
	handoffRepo                *repository.AgentHandoffRepository
	automationRuleRepo         *repository.AutomationRuleRepository
	installationRepo           *repository.SupportInboxInstallationRepository
	settingsRepo               *repository.SettingsRepository
	docsSpaceRepo              *repository.DocsSpaceRepository
	docsDocumentRepo           *repository.DocsDocumentRepository
	docsContentRepo            *repository.DocsContentRepository
	docsVersionRepo            *repository.DocsVersionRepository
	docsLinkRepo               *repository.DocsLinkRepository
	crmContactRepo             *repository.CRMContactRepository
	crmDealRepo                *repository.CRMDealRepository
	userRepo                   *repository.UserRepository
	workspaceSkillRepo         *repository.WorkspaceSkillRepository
	runEngine                  *temporalapp.RunEngine
	gitService                 *GitService
	taskService                *PMTaskService
	workflowService            *PMWorkflowService
	activitySvc                *PMActivityService
	notificationService        *NotificationService
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
	skillPackageStore          skillPackageStore
	agentDraftLLM              agentDraftLLM
}

// NewAgentService creates a new AgentService.
func NewAgentService(
	agentRepo *repository.AgentRepository,
	workspacePresetVersionRepo *repository.WorkspaceAgentPresetVersionRepository,
	runRepo *repository.AgentRunRepository,
	runMessageRepo *repository.AgentRunMessageRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
	interactionRepo *repository.AgentRunInteractionRepository,
	sessionSnapshotRepo *repository.CodingSessionStateSnapshotRepository,
	taskRepo *repository.PMTaskRepository,
	taskLinkRepo *repository.PMTaskLinkRepository,
	epicRepo *repository.PMEpicRepository,
	conversationRepo *repository.SupportConversationRepository,
	messageRepo *repository.SupportMessageRepository,
	handoffRepo *repository.AgentHandoffRepository,
	automationRuleRepo *repository.AutomationRuleRepository,
	installationRepo *repository.SupportInboxInstallationRepository,
	settingsRepo *repository.SettingsRepository,
	docsSpaceRepo *repository.DocsSpaceRepository,
	docsDocumentRepo *repository.DocsDocumentRepository,
	docsContentRepo *repository.DocsContentRepository,
	docsVersionRepo *repository.DocsVersionRepository,
	docsLinkRepo *repository.DocsLinkRepository,
	runEngine *temporalapp.RunEngine,
	gitService *GitService,
	taskService *PMTaskService,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
) *AgentService {
	return &AgentService{
		agentRepo:                  agentRepo,
		workspacePresetVersionRepo: workspacePresetVersionRepo,
		runRepo:                    runRepo,
		runMessageRepo:             runMessageRepo,
		artifactRepo:               artifactRepo,
		interactionRepo:            interactionRepo,
		sessionSnapshotRepo:        sessionSnapshotRepo,
		taskRepo:                   taskRepo,
		taskLinkRepo:               taskLinkRepo,
		epicRepo:                   epicRepo,
		conversationRepo:           conversationRepo,
		messageRepo:                messageRepo,
		handoffRepo:                handoffRepo,
		automationRuleRepo:         automationRuleRepo,
		installationRepo:           installationRepo,
		settingsRepo:               settingsRepo,
		docsSpaceRepo:              docsSpaceRepo,
		docsDocumentRepo:           docsDocumentRepo,
		docsContentRepo:            docsContentRepo,
		docsVersionRepo:            docsVersionRepo,
		docsLinkRepo:               docsLinkRepo,
		runEngine:                  runEngine,
		gitService:                 gitService,
		taskService:                taskService,
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

func (s *AgentService) SetTriggerExecutionRepository(repo *repository.AgentTriggerExecutionRepository) *AgentService {
	s.triggerExecutionRepo = repo
	return s
}

func (s *AgentService) SetCommandBarPlanRepository(repo *repository.CommandBarPlanRepository) *AgentService {
	s.commandBarPlanRepo = repo
	return s
}

func (s *AgentService) SetAgentTemplateRepository(repo *repository.AgentTemplateRepository) *AgentService {
	s.agentTemplateRepo = repo
	return s
}

// SetUserRepository injects the user repository so coding sessions can hydrate the triggering actor.
func (s *AgentService) SetUserRepository(repo *repository.UserRepository) *AgentService {
	s.userRepo = repo
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

func (s *AgentService) SetNotificationService(notificationService *NotificationService) *AgentService {
	s.notificationService = notificationService
	return s
}

func (s *AgentService) SetCRMRepositories(contactRepo *repository.CRMContactRepository, dealRepo *repository.CRMDealRepository) *AgentService {
	s.crmContactRepo = contactRepo
	s.crmDealRepo = dealRepo
	return s
}

func (s *AgentService) SetSupportCoverageService(supportCoverageService *SupportCoverageService) *AgentService {
	s.supportCoverageSvc = supportCoverageService
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
	productDefaultVersionKey := defaultPresetVersionKeyForPresetKey(presetKey)

	existing, err := s.agentRepo.GetSystemByPreset(ctx, workspaceID, presetKey)
	if err != nil {
		return nil, err
	}
	presetVersionKey := productDefaultVersionKey
	if existing != nil {
		currentVersionKey := normalizePresetVersionKey(existing.PresetVersionKey)
		if currentVersionKey != "" && currentVersionKey != productDefaultVersionKey {
			workspaceVersionResolved := false
			if s.workspacePresetVersionRepo != nil && strings.TrimSpace(workspaceID) != "" {
				version, lookupErr := s.workspacePresetVersionRepo.GetByVersionKey(ctx, workspaceID, presetKey, currentVersionKey)
				if lookupErr != nil {
					slog.ErrorContext(ctx, "failed to resolve workspace preset version during reconcile",
						"workspace_id", workspaceID,
						"preset_key", presetKey,
						"version_key", currentVersionKey,
						"error", lookupErr,
					)
				} else if version != nil {
					workspaceVersionResolved = true
				}
			}
			if workspaceVersionResolved {
				presetVersionKey = currentVersionKey
			} else if _, resolved := agentPresetVersionDefinition(presetKey, currentVersionKey); !resolved {
				slog.WarnContext(ctx, "falling back to product default preset version during reconcile",
					"workspace_id", workspaceID,
					"preset_key", presetKey,
					"missing_version_key", currentVersionKey,
					"fallback_version_key", productDefaultVersionKey,
				)
			}
		}
	}
	preset, ok := s.resolvePresetDefinition(ctx, workspaceID, presetKey, presetVersionKey)
	if !ok {
		return nil, fmt.Errorf("unsupported built-in preset %q", presetKey)
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
		beforeExecutionConfig := string(normalizeExecutionConfigJSON(existing.ExecutionConfig))
		beforeSystemPrompt := trimPtr(existing.SystemPrompt)
		beforeInstructionTemplateVersion := strings.TrimSpace(existing.InstructionTemplateVersion)
		hadPlanningNotes := existing.PlanningNotes != nil
		syncedSystemPrompt, syncedInstructionTemplateVersion := syncManagedSystemPromptForPreset(presetKey, existing.SystemPrompt, existing.PlanningNotes, existing.InstructionTemplateVersion)
		if !((beforeSystemPrompt == nil && trimPtr(syncedSystemPrompt) == nil) || (beforeSystemPrompt != nil && trimPtr(syncedSystemPrompt) != nil && *beforeSystemPrompt == *trimPtr(syncedSystemPrompt))) {
			existing.SystemPrompt = syncedSystemPrompt
			changed = true
		}
		if beforeInstructionTemplateVersion != syncedInstructionTemplateVersion {
			existing.InstructionTemplateVersion = syncedInstructionTemplateVersion
			changed = true
		}
		if existing.PlanningNotes != nil {
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
		expectedAllowedTools := normalizeAllowedToolsJSON(mustJSONStringSlice(preset.AllowedTools))
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
		if trimPtr(existing.Provider) == nil && trimPtr(preset.Provider) != nil {
			existing.Provider = trimPtr(preset.Provider)
			changed = true
		}
		if trimPtr(existing.Model) == nil && trimPtr(preset.Model) != nil {
			existing.Model = trimPtr(preset.Model)
			changed = true
		}
		expectedExecutionConfig := normalizeExecutionConfigJSON(preset.ExecutionConfig)
		if string(normalizeExecutionConfigJSON(existing.ExecutionConfig)) == "{}" && string(expectedExecutionConfig) != "{}" {
			existing.ExecutionConfig = expectedExecutionConfig
			changed = true
		}
		if ok && strings.TrimSpace(preset.RuntimeKind) != "" && strings.TrimSpace(existing.RuntimeKind) != strings.TrimSpace(preset.RuntimeKind) {
			existing.RuntimeKind = preset.RuntimeKind
			changed = true
		} else if strings.TrimSpace(existing.RuntimeKind) == "" {
			existing.RuntimeKind = preset.RuntimeKind
			changed = true
		}
		if strings.TrimSpace(existing.DefaultInvocationMode) != strings.TrimSpace(preset.DefaultInvocationMode) {
			existing.DefaultInvocationMode = preset.DefaultInvocationMode
			changed = true
		}
		normalizeAgentRecord(existing)
		if existing.PresetKey != beforePresetKey ||
			strings.TrimSpace(existing.InstructionTemplateVersion) != beforeInstructionTemplateVersion ||
			existing.Role != beforeRole ||
			existing.RuntimeKind != beforeRuntimeKind ||
			existing.TriggerMode != beforeTriggerMode ||
			existing.ApprovalMode != beforeApprovalMode ||
			existing.DefaultInvocationMode != beforeDefaultInvocationMode ||
			string(existing.AllowedTools) != beforeAllowedTools ||
			string(existing.AllowedCommands) != beforeAllowedCommands ||
			string(existing.AllowedTargets) != beforeAllowedTargets ||
			string(normalizeExecutionConfigJSON(existing.ExecutionConfig)) != beforeExecutionConfig ||
			((beforeSystemPrompt == nil) != (trimPtr(existing.SystemPrompt) == nil)) ||
			(beforeSystemPrompt != nil && trimPtr(existing.SystemPrompt) != nil && *beforeSystemPrompt != *trimPtr(existing.SystemPrompt)) ||
			(hadPlanningNotes && existing.PlanningNotes == nil) {
			changed = true
		}
		if changed {
			if err := s.agentRepo.Update(ctx, existing); err != nil {
				return nil, err
			}
		}
		materializeAgentSystemPrompt(existing)
		return existing, nil
	}

	agent := &model.Agent{
		WorkspaceID:                workspaceID,
		IsSystem:                   true,
		Name:                       defaultSystemAgentNameForPresetKey(presetKey),
		PresetKey:                  presetKey,
		PresetVersionKey:           presetVersionKey,
		Role:                       preset.DefaultRole,
		Status:                     "idle",
		RuntimeKind:                preset.RuntimeKind,
		Skills:                     model.AgentSkillRefs{},
		TriggerMode:                preset.DefaultTriggerMode,
		Provider:                   trimPtr(preset.Provider),
		Model:                      trimPtr(preset.Model),
		ExecutionConfig:            normalizeExecutionConfigJSON(preset.ExecutionConfig),
		SystemPrompt:               nil,
		InstructionTemplateVersion: strings.TrimSpace(preset.InstructionTemplateVersion),
		PlanningNotes:              nil,
		AllowedTools:               normalizeAllowedToolsJSON(mustJSONStringSlice(preset.AllowedTools)),
		AllowedCommands:            mustJSONStringSlice(preset.AllowedCommands),
		AllowedTargets:             mustJSONStringSlice(preset.AllowedTargetTypes),
		ApprovalMode:               preset.ApprovalMode,
		MaxConcurrentRuns:          1,
		DefaultInvocationMode:      preset.DefaultInvocationMode,
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
	materializeAgentSystemPrompt(agent)
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

func materializeAgentSystemPrompt(agent *model.Agent) {
	if agent == nil {
		return
	}
	agent.SystemPrompt = resolveEffectiveSystemPromptForPreset(agent.EffectivePresetKey(), agent.SystemPrompt)
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
		if err := s.validateAndMaterializeAgentSkills(ctx, &agents[idx]); err != nil {
			slog.WarnContext(ctx, "failed to resolve agent skills for list", "agent_id", agents[idx].ID, "workspace_id", workspaceID, "error", err)
			agents[idx].ResolvedSkillInstructions = ""
		}
		materializeAgentSystemPrompt(&agents[idx])
	}
	return agents, nil
}

func (s *AgentService) ListAgentsForActor(ctx context.Context, workspaceID string, actor *authorization.Actor) ([]model.Agent, error) {
	agents, err := s.ListAgents(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if actor != nil && (actor.Role == "admin" || actor.Role == "owner") {
		return agents, nil
	}
	actorTeamIDs := map[string]struct{}{}
	if actor != nil {
		actorTeamIDs = make(map[string]struct{}, len(actor.TeamMemberships))
		for _, tm := range actor.TeamMemberships {
			teamID := strings.TrimSpace(tm.TeamID)
			if teamID != "" {
				actorTeamIDs[teamID] = struct{}{}
			}
		}
	}
	filtered := make([]model.Agent, 0, len(agents))
	for _, agent := range agents {
		if agentVisibleToActorTeams(agent, actorTeamIDs) {
			filtered = append(filtered, agent)
		}
	}
	return filtered, nil
}

// RequireActorCanUseAgent enforces the actor boundary for team-scoped agents.
// Agent team access controls who can see and manually start an agent; tools and target RBAC
// control what the agent can access after it starts.
func (s *AgentService) RequireActorCanUseAgent(ctx context.Context, workspaceID, agentID string, actor *authorization.Actor) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	agent, err := s.GetAgent(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}
	if actor != nil && (actor.Role == "admin" || actor.Role == "owner") {
		return nil
	}
	actorTeamIDs := map[string]struct{}{}
	if actor != nil {
		actorTeamIDs = make(map[string]struct{}, len(actor.TeamMemberships))
		for _, tm := range actor.TeamMemberships {
			teamID := strings.TrimSpace(tm.TeamID)
			if teamID != "" {
				actorTeamIDs[teamID] = struct{}{}
			}
		}
	}
	if !agentVisibleToActorTeams(*agent, actorTeamIDs) {
		return fmt.Errorf("agent is not available to this actor")
	}
	return nil
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
	if err := s.validateAndMaterializeAgentSkills(ctx, agent); err != nil {
		return nil, err
	}
	materializeAgentSystemPrompt(agent)
	return agent, nil
}

// GetAgentUsageSummary returns the inbound trigger bindings for an agent.
func (s *AgentService) GetAgentUsageSummary(ctx context.Context, workspaceID, id string) (*model.AgentTriggerUsageSummary, error) {
	agent, err := s.GetAgent(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}

	items := make([]model.AgentTriggerUsage, 0, 8)

	if s.automationRuleRepo != nil {
		rules, err := s.automationRuleRepo.ListByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("list automation rules for agent usage: %w", err)
		}
		managePath := "/w/$slug/settings/workflows"
		for _, rule := range rules {
			if rule.ActionType != model.ActionStartAgentRun {
				continue
			}
			var cfg model.ActionConfigRunAgent
			if err := json.Unmarshal(rule.ActionConfig, &cfg); err != nil {
				continue
			}
			if strings.TrimSpace(cfg.AgentID) != agent.ID {
				continue
			}
			ruleID := rule.ID
			triggerType := rule.TriggerType
			path := managePath
			executionSearch := automationcatalog.ExecutionSearchPresetForTrigger(
				model.AgentRunTriggerSourceAutomationRule,
				rule.TriggerType,
				"",
				&ruleID,
			)
			items = append(items, model.AgentTriggerUsage{
				ID:              "automation_rule:" + rule.ID,
				Kind:            "automation_rule",
				Title:           rule.Name,
				Description:     describeAutomationRuleBinding(rule),
				TriggerType:     &triggerType,
				Enabled:         rule.Enabled,
				ReferenceID:     &ruleID,
				ReferenceType:   strPtr("automation_rule"),
				ManagePath:      &path,
				ExecutionSearch: executionSearch,
			})
		}
	}

	if s.installationRepo != nil {
		inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("get support installation for agent usage: %w", err)
		}
		if inst != nil {
			settings := parseSettings(inst.Settings)
			if settings.AIEnabled && strings.TrimSpace(derefString(settings.AIAgentID)) == agent.ID {
				triggerType := supportAutoTriggerType
				managePath := "/w/$slug/settings/chat-general"
				items = append(items, model.AgentTriggerUsage{
					ID:              "support.widget_message",
					Kind:            "support_widget",
					Title:           "Support widget AI auto-replies",
					Description:     "Runs this agent automatically on new visitor messages in the chat widget.",
					TriggerType:     &triggerType,
					Enabled:         true,
					ManagePath:      &managePath,
					ExecutionSearch: automationcatalog.ExecutionSearchPresetForDefinitionID("support.widget_message", nil),
				})
			}
		}
	}

	usageByID := make(map[string]*model.AgentTriggerUsage, len(items))
	usageByReference := make(map[string]*model.AgentTriggerUsage)
	for idx := range items {
		usageByID[items[idx].ID] = &items[idx]
		if items[idx].ReferenceType != nil && *items[idx].ReferenceType == "automation_rule" && items[idx].ReferenceID != nil {
			usageByReference[*items[idx].ReferenceID] = &items[idx]
		}
	}
	if s.triggerExecutionRepo != nil {
		executions, err := s.triggerExecutionRepo.ListByAgent(ctx, workspaceID, agent.ID, 200)
		if err != nil {
			return nil, fmt.Errorf("list trigger executions for agent usage: %w", err)
		}
		for _, execution := range executions {
			var item *model.AgentTriggerUsage
			if derefString(execution.ReferenceType) == "automation_rule" && execution.ReferenceID != nil {
				item = usageByReference[derefString(execution.ReferenceID)]
			}
			if item == nil {
				item = usageByID[execution.BindingID]
			}
			if item == nil {
				if def, ok := automationcatalog.ResolveDefinitionForExecution(execution.BindingID, execution.BindingKind, derefString(execution.TriggerType)); ok {
					item = usageByID[def.ID]
				}
			}
			if item == nil {
				continue
			}
			if item.LastTriggeredAt == nil || execution.FiredAt.After(*item.LastTriggeredAt) {
				ts := execution.FiredAt
				item.LastTriggeredAt = &ts
			}
			switch strings.TrimSpace(execution.Status) {
			case model.AgentTriggerExecutionStatusCompleted:
				if item.LastSuccessAt == nil {
					successAt := execution.FiredAt
					if execution.CompletedAt != nil {
						successAt = *execution.CompletedAt
					}
					ts := successAt
					item.LastSuccessAt = &ts
				}
			case model.AgentTriggerExecutionStatusFailed:
				if item.LastErrorAt == nil {
					errorAt := execution.FiredAt
					if execution.CompletedAt != nil {
						errorAt = *execution.CompletedAt
					}
					ts := errorAt
					item.LastErrorAt = &ts
					item.LastError = execution.ErrorMessage
				}
			}
			if len(item.RecentExecutions) < 5 {
				item.RecentExecutions = append(item.RecentExecutions, model.AgentTriggerExecutionSummary{
					ExecutionID:   execution.ID,
					RunID:         execution.RunID,
					Status:        execution.Status,
					TargetType:    derefString(execution.TargetType),
					TargetID:      derefString(execution.TargetID),
					FiredAt:       execution.FiredAt,
					StartedAt:     execution.StartedAt,
					CompletedAt:   execution.CompletedAt,
					ErrorMessage:  execution.ErrorMessage,
					TriggerType:   execution.TriggerType,
					ReferenceID:   execution.ReferenceID,
					ReferenceType: execution.ReferenceType,
				})
			}
		}
	} else if s.runRepo != nil {
		runs, _, err := s.runRepo.ListByAgent(ctx, workspaceID, agent.ID, model.PMPagination{Page: 1, PerPage: 100})
		if err != nil {
			return nil, fmt.Errorf("list agent runs for trigger usage: %w", err)
		}
		for _, run := range runs {
			usageID, triggeredAt := usageBindingIDForRun(run)
			if usageID == "" {
				continue
			}
			item := usageByID[usageID]
			if item == nil {
				if def, ok := automationcatalog.ResolveDefinitionForExecution(usageID, "", ""); ok {
					item = usageByID[def.ID]
				}
			}
			if item == nil {
				continue
			}
			if item.LastTriggeredAt == nil || triggeredAt.After(*item.LastTriggeredAt) {
				ts := triggeredAt
				item.LastTriggeredAt = &ts
			}
			if run.Status == model.AgentRunStatusCompleted && item.LastSuccessAt == nil {
				completedAt := run.CreatedAt
				if run.CompletedAt != nil {
					completedAt = *run.CompletedAt
				}
				ts := completedAt
				item.LastSuccessAt = &ts
			}
			if run.Status == model.AgentRunStatusFailed && item.LastErrorAt == nil {
				errorAt := run.CreatedAt
				if run.CompletedAt != nil {
					errorAt = *run.CompletedAt
				}
				ts := errorAt
				item.LastErrorAt = &ts
				item.LastError = run.ErrorMessage
			}
			if len(item.RecentExecutions) < 5 {
				item.RecentExecutions = append(item.RecentExecutions, model.AgentTriggerExecutionSummary{
					ExecutionID:  run.ID,
					RunID:        &run.ID,
					Status:       run.Status,
					TargetType:   run.TargetType,
					TargetID:     run.TargetID,
					FiredAt:      triggeredAt,
					StartedAt:    run.StartedAt,
					CompletedAt:  run.CompletedAt,
					ErrorMessage: run.ErrorMessage,
				})
			}
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Title < items[j].Title
	})

	return &model.AgentTriggerUsageSummary{
		AgentID:   agent.ID,
		AgentName: agent.Name,
		Items:     items,
	}, nil
}

func describeAutomationRuleBinding(rule model.AutomationRule) string {
	description := describeAutomationRuleTrigger(rule)
	var actionCfg model.ActionConfigRunAgent
	if err := json.Unmarshal(rule.ActionConfig, &actionCfg); err == nil {
		targetType := strings.TrimSpace(actionCfg.TargetType)
		targetID := strings.TrimSpace(actionCfg.TargetID)
		if targetType != "" && targetID != "" {
			description += fmt.Sprintf(" Uses fixed %s target `%s`.", targetType, targetID)
		}
	}
	return description
}

func describeAutomationRuleTrigger(rule model.AutomationRule) string {
	switch rule.TriggerType {
	case model.TriggerTaskStateEntered:
		var cfg model.TriggerConfigStateEntered
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err == nil {
			if strings.TrimSpace(cfg.StateID) != "" {
				return "Runs when a task enters the configured workflow state."
			}
			if strings.TrimSpace(cfg.StateType) != "" {
				return fmt.Sprintf("Runs when a task enters any `%s` workflow state.", strings.TrimSpace(cfg.StateType))
			}
		}
		return "Runs when a task enters a matching workflow state."
	case model.TriggerAgentRunApproved:
		return "Runs after an interactive task run is explicitly approved."
	case model.TriggerGitHubPush:
		var cfg model.TriggerConfigGitHubPush
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err == nil {
			parts := make([]string, 0, 2)
			if strings.TrimSpace(cfg.Branch) != "" {
				parts = append(parts, fmt.Sprintf("branch `%s`", strings.TrimSpace(cfg.Branch)))
			}
			if strings.TrimSpace(cfg.RepoFullName) != "" {
				parts = append(parts, fmt.Sprintf("repo `%s`", strings.TrimSpace(cfg.RepoFullName)))
			}
			if len(parts) > 0 {
				return "Runs when GitHub receives a push for " + strings.Join(parts, " in ") + "."
			}
		}
		return "Runs when GitHub receives a push webhook."
	case model.TriggerGitHubPROpened:
		return describeGitHubPullRequestTrigger(rule, "opened")
	case model.TriggerGitHubPRMerged:
		return describeGitHubPullRequestTrigger(rule, "merged")
	case model.TriggerGitHubPRClosed:
		return describeGitHubPullRequestTrigger(rule, "closed")
	case model.TriggerGitHubPRReviewReq:
		return describeGitHubPullRequestTrigger(rule, "review requested")
	case model.TriggerGitHubReleasePub:
		var cfg model.TriggerConfigGitHubReleasePublished
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err == nil {
			parts := make([]string, 0, 2)
			if strings.TrimSpace(cfg.TagName) != "" {
				parts = append(parts, fmt.Sprintf("tag `%s`", strings.TrimSpace(cfg.TagName)))
			}
			if strings.TrimSpace(cfg.RepoFullName) != "" {
				parts = append(parts, fmt.Sprintf("repo `%s`", strings.TrimSpace(cfg.RepoFullName)))
			}
			if len(parts) > 0 {
				return "Runs when a GitHub release is published for " + strings.Join(parts, " in ") + "."
			}
		}
		return "Runs when a GitHub release is published."
	case model.TriggerGitHubCheckSuite:
		var cfg model.TriggerConfigGitHubCheckSuiteCompleted
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err == nil {
			parts := make([]string, 0, 3)
			if strings.TrimSpace(cfg.Conclusion) != "" {
				parts = append(parts, fmt.Sprintf("conclusion `%s`", strings.TrimSpace(cfg.Conclusion)))
			}
			if strings.TrimSpace(cfg.Branch) != "" {
				parts = append(parts, fmt.Sprintf("branch `%s`", strings.TrimSpace(cfg.Branch)))
			}
			if strings.TrimSpace(cfg.RepoFullName) != "" {
				parts = append(parts, fmt.Sprintf("repo `%s`", strings.TrimSpace(cfg.RepoFullName)))
			}
			if len(parts) > 0 {
				return "Runs when a GitHub check suite completes for " + strings.Join(parts, " in ") + "."
			}
		}
		return "Runs when a GitHub check suite completes."
	case model.TriggerCron:
		var cfg model.TriggerConfigCron
		if err := json.Unmarshal(rule.TriggerConfig, &cfg); err == nil && strings.TrimSpace(cfg.Category) != "" {
			return fmt.Sprintf("Runs on automation-rule cron category `%s`.", strings.TrimSpace(cfg.Category))
		}
		return "Runs on the automation-rule cron schedule."
	default:
		return "Runs from an automation rule."
	}
}

func describeGitHubPullRequestTrigger(rule model.AutomationRule, action string) string {
	var cfg model.TriggerConfigGitHubPullRequest
	if err := json.Unmarshal(rule.TriggerConfig, &cfg); err == nil {
		parts := make([]string, 0, 2)
		if strings.TrimSpace(cfg.BaseBranch) != "" {
			parts = append(parts, fmt.Sprintf("PR base branch `%s`", strings.TrimSpace(cfg.BaseBranch)))
		}
		if strings.TrimSpace(cfg.RepoFullName) != "" {
			parts = append(parts, fmt.Sprintf("repo `%s`", strings.TrimSpace(cfg.RepoFullName)))
		}
		if len(parts) > 0 {
			return "Runs when a GitHub pull request is " + action + " for " + strings.Join(parts, " in ") + "."
		}
	}
	return "Runs when a GitHub pull request is " + action + "."
}

func usageBindingIDForRun(run model.AgentRun) (string, time.Time) {
	triggeredAt := run.CreatedAt
	if len(run.Input) == 0 {
		return "", triggeredAt
	}

	var input model.AgentRunInputPayload
	if err := json.Unmarshal(run.Input, &input); err != nil || input.Trigger == nil {
		return "", triggeredAt
	}
	if input.Trigger.FiredAt != nil {
		triggeredAt = *input.Trigger.FiredAt
	}

	switch strings.TrimSpace(input.Trigger.Source) {
	case model.AgentRunTriggerSourceAutomationRule:
		if bindingID, _, ok := automationcatalog.ResolveBindingForTrigger(input.Trigger.Source, input.Trigger.TriggerType, targetTypeFromRun(run)); ok {
			return bindingID, triggeredAt
		}
	case model.AgentRunTriggerSourceSystem:
		switch strings.TrimSpace(input.Trigger.TriggerType) {
		case supportAutoTriggerType:
			return "support.widget_message", triggeredAt
		case "task.assigned_agent_state_change":
			return "task.assigned_agent_state_change", triggeredAt
		}
	}

	return "", triggeredAt
}

func targetTypeFromRun(run model.AgentRun) string {
	return strings.TrimSpace(run.TargetType)
}

func triggerExecutionBinding(trigger *model.AgentRunTriggerContext, targetType string) (bindingID, bindingKind string, referenceID, referenceType *string, ok bool) {
	if trigger == nil {
		return "", "", nil, nil, false
	}

	bindingID, bindingKind, ok = automationcatalog.ResolveBindingForTrigger(
		strings.TrimSpace(trigger.Source),
		strings.TrimSpace(trigger.TriggerType),
		strings.TrimSpace(targetType),
	)
	if !ok {
		return "", "", nil, nil, false
	}
	if strings.TrimSpace(trigger.Source) == model.AgentRunTriggerSourceAutomationRule && trigger.RuleID != nil && strings.TrimSpace(*trigger.RuleID) != "" {
		ruleID := strings.TrimSpace(*trigger.RuleID)
		refType := "automation_rule"
		return bindingID, bindingKind, &ruleID, &refType, true
	}
	return bindingID, bindingKind, nil, nil, true
}

func triggerExecutionStatus(run *model.AgentRun, err error) string {
	if run != nil && strings.TrimSpace(run.Status) != "" {
		return strings.TrimSpace(run.Status)
	}
	if err == nil {
		return model.AgentTriggerExecutionStatusQueued
	}
	if isSkippedTriggerExecutionError(err) {
		return model.AgentTriggerExecutionStatusSkipped
	}
	return model.AgentTriggerExecutionStatusFailed
}

func isSkippedTriggerExecutionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrTaskDeliveryTargetRequired) {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "already active") || strings.Contains(msg, "skipping")
}

func (s *AgentService) recordTriggerExecution(
	ctx context.Context,
	workspaceID, agentID string,
	trigger *model.AgentRunTriggerContext,
	targetType, targetID string,
	run *model.AgentRun,
	err error,
) {
	if s == nil || s.triggerExecutionRepo == nil || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(agentID) == "" {
		return
	}

	bindingID, bindingKind, referenceID, referenceType, ok := triggerExecutionBinding(trigger, targetType)
	if !ok {
		return
	}

	firedAt := time.Now().UTC()
	triggerType := (*string)(nil)
	if trigger != nil {
		if trigger.FiredAt != nil {
			firedAt = *trigger.FiredAt
		}
		if strings.TrimSpace(trigger.TriggerType) != "" {
			triggerValue := strings.TrimSpace(trigger.TriggerType)
			triggerType = &triggerValue
		}
	}

	targetType = strings.TrimSpace(targetType)
	targetID = strings.TrimSpace(targetID)
	status := triggerExecutionStatus(run, err)
	var (
		runID        *string
		startedAt    *time.Time
		completedAt  *time.Time
		errorMessage *string
	)
	if run != nil {
		runID = &run.ID
		startedAt = run.StartedAt
		completedAt = run.CompletedAt
		if run.ErrorMessage != nil && strings.TrimSpace(*run.ErrorMessage) != "" {
			errorMessage = run.ErrorMessage
		}
		if targetType == "" {
			targetType = strings.TrimSpace(run.TargetType)
		}
		if targetID == "" {
			targetID = strings.TrimSpace(run.TargetID)
		}
	}
	if err != nil && (errorMessage == nil || strings.TrimSpace(*errorMessage) == "") {
		msg := strings.TrimSpace(err.Error())
		if msg != "" {
			errorMessage = &msg
		}
	}
	if completedAt == nil && (status == model.AgentTriggerExecutionStatusFailed || status == model.AgentTriggerExecutionStatusSkipped) {
		completedAt = &firedAt
	}

	execution := &model.AgentTriggerExecution{
		WorkspaceID:   workspaceID,
		AgentID:       agentID,
		BindingID:     bindingID,
		BindingKind:   bindingKind,
		TriggerType:   triggerType,
		ReferenceID:   referenceID,
		ReferenceType: referenceType,
		TargetType:    nilIfEmpty(targetType),
		TargetID:      nilIfEmpty(targetID),
		RunID:         runID,
		Status:        status,
		ErrorMessage:  errorMessage,
		FiredAt:       firedAt,
		StartedAt:     startedAt,
		CompletedAt:   completedAt,
	}
	if createErr := s.triggerExecutionRepo.Create(ctx, execution); createErr != nil {
		slog.WarnContext(ctx, "failed to record trigger execution",
			"workspace_id", workspaceID,
			"agent_id", agentID,
			"binding_id", bindingID,
			"error", createErr,
		)
	}
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
		WorkspaceID:                workspaceID,
		FamilyKey:                  familyKey,
		VersionKey:                 versionKey,
		Label:                      label,
		Description:                trimPtr(req.Description),
		SourceVersionKey:           trimPtr(req.SourceVersionKey),
		RuntimeKind:                runtimeKind,
		Provider:                   trimPtr(req.Provider),
		Model:                      nil,
		ExecutionConfig:            normalizeExecutionConfigJSON(req.ExecutionConfig),
		SystemPrompt:               trimPtr(req.SystemPrompt),
		InstructionSkills:          mustJSONStringSlice(nil),
		InstructionTemplateVersion: strings.TrimSpace(basePreset.InstructionTemplateVersion),
		AllowedTools:               normalizeAllowedToolsJSON(mustJSONStringSlice(basePreset.AllowedTools)),
		SupportedModes:             mustJSONStringSlice(normalizedSupportedModes),
		ApprovalMode:               "never",
		DefaultInvocationMode:      defaultInvocationMode,
		CreatedBy:                  trimPtr(&actorID),
	}
	if req.AllowedTools != nil {
		version.AllowedTools = normalizeAllowedToolsJSON(normalizeJSONSlice(req.AllowedTools))
	}
	if req.Model != nil {
		modelValue := strings.TrimSpace(*req.Model)
		version.Model = &modelValue
	}
	// If preamble or skills are provided, compile system_prompt from them.
	hasPreamble := req.InstructionPreamble != nil
	hasSkills := req.InstructionSkills != nil
	if hasPreamble || hasSkills {
		preamble := strings.TrimSpace(stringOrDefault(req.InstructionPreamble, basePreset.InstructionPreamble))
		skills := basePreset.InstructionSkills
		if hasSkills {
			skills = parseJSONStringSlice(req.InstructionSkills)
		}
		version.InstructionPreamble = &preamble
		version.InstructionSkills = mustJSONStringSlice(skills)
		compiled := worker.CompilePresetInstructions(preamble, skills)
		version.SystemPrompt = &compiled
		version.InstructionTemplateVersion = worker.InstructionTemplateVersionForPreset(preamble, skills)
	} else if version.SystemPrompt == nil {
		version.SystemPrompt = trimPtr(basePreset.SystemPrompt)
	} else {
		// Raw system_prompt override — instruction decomposition no longer applies.
		emptyPreamble := ""
		version.InstructionPreamble = &emptyPreamble
		version.InstructionSkills = mustJSONStringSlice(nil)
		version.InstructionTemplateVersion = ""
	}
	if version.Provider == nil {
		version.Provider = trimPtr(basePreset.Provider)
	}
	if req.Model == nil && version.Model == nil {
		version.Model = trimPtr(basePreset.Model)
	}
	if string(version.ExecutionConfig) == "{}" {
		version.ExecutionConfig = normalizeExecutionConfigJSON(basePreset.ExecutionConfig)
	}
	versionValidationAgent := &model.Agent{
		IsSystem:         true,
		PresetKey:        familyKey,
		PresetVersionKey: version.VersionKey,
		RuntimeKind:      version.RuntimeKind,
		Provider:         version.Provider,
		Model:            version.Model,
		ExecutionConfig:  version.ExecutionConfig,
	}
	if _, err := parseAndValidateExecutionConfig(versionValidationAgent); err != nil {
		return nil, err
	}
	if err := s.workspacePresetVersionRepo.Create(ctx, version); err != nil {
		return nil, err
	}
	definition := workspacePresetDefinition(basePreset, *version)
	slog.InfoContext(ctx, "workspace preset version created",
		"workspace_id", workspaceID,
		"family_key", familyKey,
		"version_key", version.VersionKey,
		"actor_id", strings.TrimSpace(actorID),
	)
	return &definition, nil
}

func workspacePresetBaseDefinition(familyKey string) (model.AgentPresetDefinition, error) {
	base, ok := agentPresetVersionDefinition(familyKey, defaultPresetVersionKeyForPresetKey(familyKey))
	if !ok {
		return model.AgentPresetDefinition{}, fmt.Errorf("unsupported preset family %q", familyKey)
	}
	return base, nil
}

func workspacePresetCurrentDefinition(version *model.WorkspaceAgentPresetVersion) (model.AgentPresetDefinition, error) {
	if version == nil {
		return model.AgentPresetDefinition{}, fmt.Errorf("workspace preset version is required")
	}
	base, err := workspacePresetBaseDefinition(normalizePresetKey(version.FamilyKey))
	if err != nil {
		return model.AgentPresetDefinition{}, err
	}
	return workspacePresetDefinition(base, *version), nil
}

func (s *AgentService) cloneWithTx(tx *gorm.DB) *AgentService {
	if tx == nil {
		return s
	}
	clone := *s
	if s.agentRepo != nil {
		clone.agentRepo = s.agentRepo.WithTx(tx)
	}
	if s.workspacePresetVersionRepo != nil {
		clone.workspacePresetVersionRepo = s.workspacePresetVersionRepo.WithTx(tx)
	}
	return &clone
}

func (s *AgentService) applyPresetToSystemAgent(agent *model.Agent, preset model.AgentPresetDefinition, presetVersionKey string) {
	if agent == nil {
		return
	}
	systemPresetKey := normalizePresetKey(agent.PresetKey)
	if systemPresetKey == "" {
		systemPresetKey = normalizePresetKey(preset.FamilyKey)
	}
	if systemPresetKey == "" {
		systemPresetKey = model.AgentPresetEpicPlanner
	}
	presetVersionKey = normalizePresetVersionKey(presetVersionKey)
	if presetVersionKey == "" {
		presetVersionKey = defaultPresetVersionKeyForPresetKey(systemPresetKey)
	}

	agent.PresetKey = systemPresetKey
	agent.PresetVersionKey = presetVersionKey
	if strings.TrimSpace(preset.RuntimeKind) != "" {
		agent.RuntimeKind = preset.RuntimeKind
	} else {
		agent.RuntimeKind = defaultRuntimeKindForPresetKey(systemPresetKey)
	}
	if strings.TrimSpace(preset.DefaultTriggerMode) != "" {
		agent.TriggerMode = preset.DefaultTriggerMode
	} else {
		agent.TriggerMode = defaultTriggerModeForPresetKey(systemPresetKey)
	}
	agent.Provider = trimPtr(preset.Provider)
	agent.Model = trimPtr(preset.Model)
	agent.ExecutionConfig = normalizeExecutionConfigJSON(preset.ExecutionConfig)
	agent.AllowedTools = normalizeAllowedToolsJSON(mustJSONStringSlice(preset.AllowedTools))
	agent.AllowedCommands = mustJSONStringSlice(preset.AllowedCommands)
	agent.AllowedTargets = mustJSONStringSlice(preset.AllowedTargetTypes)
	agent.TeamID = nil
	agent.TeamIDs = nil
	agent.ApprovalMode = "never"
	agent.DefaultInvocationMode = preset.DefaultInvocationMode
	if preset.Scope == "workspace" {
		agent.SystemPrompt = preset.SystemPrompt
		agent.InstructionTemplateVersion = preset.InstructionTemplateVersion
	} else {
		agent.SystemPrompt, agent.InstructionTemplateVersion = syncManagedSystemPromptForPreset(systemPresetKey, agent.SystemPrompt, agent.PlanningNotes, agent.InstructionTemplateVersion)
	}
	agent.PlanningNotes = nil
}

func (s *AgentService) validateSystemAgentPresetState(ctx context.Context, agent *model.Agent, preset model.AgentPresetDefinition) error {
	if agent == nil {
		return nil
	}
	normalizeAgentRecord(agent)
	if err := s.validateAndMaterializeAgentSkills(ctx, agent); err != nil {
		return err
	}
	if err := validateAgentPresetKey(agent.PresetKey); err != nil {
		return err
	}
	if err := validateRuntimeForAgentWithPreset(agent, &preset); err != nil {
		return err
	}
	if err := validateTriggerModeForAgent(agent.TriggerMode, agent); err != nil {
		return err
	}
	if err := s.validateModelRouting(agent); err != nil {
		return err
	}
	return nil
}

func (s *AgentService) propagateWorkspacePresetVersionToPinnedAgents(ctx context.Context, workspaceID string, version *model.WorkspaceAgentPresetVersion, preset model.AgentPresetDefinition) error {
	if s.agentRepo == nil || version == nil {
		return nil
	}
	agents, err := s.agentRepo.List(ctx, workspaceID)
	if err != nil {
		return err
	}
	for idx := range agents {
		agent := &agents[idx]
		if !agent.IsSystem {
			continue
		}
		if normalizePresetKey(agent.PresetKey) != normalizePresetKey(version.FamilyKey) {
			continue
		}
		if normalizePresetVersionKey(agent.PresetVersionKey) != normalizePresetVersionKey(version.VersionKey) {
			continue
		}
		s.applyPresetToSystemAgent(agent, preset, version.VersionKey)
		if err := s.validateSystemAgentPresetState(ctx, agent, preset); err != nil {
			return err
		}
		if err := s.agentRepo.Update(ctx, agent); err != nil {
			return err
		}
	}
	return nil
}

// UpdateWorkspacePresetVersion applies ordinary edits to a workspace preset version.
// Label changes are treated like any other edit and are tracked only via UpdatedBy/LastEditedAt.
func (s *AgentService) UpdateWorkspacePresetVersion(ctx context.Context, workspaceID, versionID string, req model.UpdateWorkspaceAgentPresetVersionRequest, actorID string) (*model.AgentPresetDefinition, error) {
	if s.workspacePresetVersionRepo == nil {
		return nil, fmt.Errorf("workspace preset version repository is not configured")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	versionID = strings.TrimSpace(versionID)
	if workspaceID == "" || versionID == "" {
		return nil, fmt.Errorf("workspace_id and version id are required")
	}

	var definition *model.AgentPresetDefinition
	txDB := s.workspacePresetVersionRepo.DB()
	if txDB == nil {
		return nil, fmt.Errorf("workspace preset version repository db is not configured")
	}
	if err := txDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txSvc := s.cloneWithTx(tx)

		version, err := txSvc.workspacePresetVersionRepo.GetByID(ctx, workspaceID, versionID)
		if err != nil {
			return err
		}
		if version == nil {
			return fmt.Errorf("%w: version %q not found", ErrWorkspacePresetVersionNotFound, versionID)
		}

		currentPreset, err := workspacePresetCurrentDefinition(version)
		if err != nil {
			return err
		}
		familyKey := normalizePresetKey(version.FamilyKey)

		label := strings.TrimSpace(currentPreset.VersionLabel)
		if req.Label != nil {
			label = strings.TrimSpace(*req.Label)
		}
		if label == "" {
			return fmt.Errorf("label is required")
		}

		runtimeKind := strings.TrimSpace(currentPreset.RuntimeKind)
		if req.RuntimeKind != nil {
			runtimeKind = strings.TrimSpace(*req.RuntimeKind)
		}
		if runtimeKind == "" {
			runtimeKind = currentPreset.RuntimeKind
		}
		if err := validateRuntimeKind(runtimeKind); err != nil {
			return err
		}
		if !runtimeAllowedForPreset(familyKey, runtimeKind) {
			return fmt.Errorf("runtime_kind %q is not supported for family %q", runtimeKind, familyKey)
		}

		supportedModes := slices.Clone(currentPreset.SupportedModes)
		if len(req.SupportedModes) > 0 {
			supportedModes = parseJSONStringSlice(req.SupportedModes)
		}
		normalizedSupportedModes, err := validateSupportedModes(runtimeKind, supportedModes)
		if err != nil {
			return err
		}

		defaultInvocationMode := strings.TrimSpace(currentPreset.DefaultInvocationMode)
		if req.DefaultInvocationMode != nil {
			defaultInvocationMode = strings.TrimSpace(*req.DefaultInvocationMode)
		}
		if defaultInvocationMode == "" {
			defaultInvocationMode = currentPreset.DefaultInvocationMode
		}
		if !slices.Contains(normalizedSupportedModes, defaultInvocationMode) {
			return fmt.Errorf("default_invocation_mode %q must be included in supported_modes", defaultInvocationMode)
		}

		version.Label = label
		if req.Description != nil {
			version.Description = trimPtr(req.Description)
		} else {
			version.Description = trimPtr(&currentPreset.Description)
		}
		version.RuntimeKind = runtimeKind
		if req.Provider != nil {
			version.Provider = trimPtr(req.Provider)
		} else {
			version.Provider = trimPtr(currentPreset.Provider)
		}
		if req.Model != nil {
			modelValue := strings.TrimSpace(*req.Model)
			version.Model = &modelValue
		} else if currentPreset.Model != nil {
			modelValue := strings.TrimSpace(*currentPreset.Model)
			version.Model = &modelValue
		} else {
			version.Model = nil
		}
		if req.ExecutionConfig != nil {
			version.ExecutionConfig = normalizeExecutionConfigJSON(req.ExecutionConfig)
		} else {
			version.ExecutionConfig = normalizeExecutionConfigJSON(currentPreset.ExecutionConfig)
		}
		if req.AllowedTools != nil {
			version.AllowedTools = normalizeAllowedToolsJSON(normalizeJSONSlice(req.AllowedTools))
		} else {
			version.AllowedTools = normalizeAllowedToolsJSON(mustJSONStringSlice(currentPreset.AllowedTools))
		}
		version.SupportedModes = mustJSONStringSlice(normalizedSupportedModes)
		version.DefaultInvocationMode = defaultInvocationMode
		version.ApprovalMode = "never"

		hasPreamble := req.InstructionPreamble != nil
		hasSkills := req.InstructionSkills != nil
		if hasPreamble || hasSkills {
			preamble := currentPreset.InstructionPreamble
			if req.InstructionPreamble != nil {
				preamble = strings.TrimSpace(*req.InstructionPreamble)
			}
			skills := slices.Clone(currentPreset.InstructionSkills)
			if hasSkills {
				skills = parseJSONStringSlice(req.InstructionSkills)
			}
			version.InstructionPreamble = &preamble
			version.InstructionSkills = mustJSONStringSlice(skills)
			compiled := worker.CompilePresetInstructions(preamble, skills)
			version.SystemPrompt = &compiled
			version.InstructionTemplateVersion = worker.InstructionTemplateVersionForPreset(preamble, skills)
		} else if req.SystemPrompt != nil {
			version.SystemPrompt = trimPtr(req.SystemPrompt)
			version.InstructionTemplateVersion = ""
			emptyPreamble := ""
			version.InstructionPreamble = &emptyPreamble
			version.InstructionSkills = mustJSONStringSlice(nil)
		} else {
			version.SystemPrompt = trimPtr(currentPreset.SystemPrompt)
			version.InstructionTemplateVersion = strings.TrimSpace(currentPreset.InstructionTemplateVersion)
			preamble := currentPreset.InstructionPreamble
			version.InstructionPreamble = &preamble
			version.InstructionSkills = mustJSONStringSlice(currentPreset.InstructionSkills)
		}

		now := time.Now().UTC()
		version.UpdatedBy = trimPtr(&actorID)
		version.LastEditedAt = &now

		currentDefinition, err := workspacePresetCurrentDefinition(version)
		if err != nil {
			return err
		}
		versionValidationAgent := &model.Agent{
			IsSystem:  true,
			PresetKey: familyKey,
		}
		txSvc.applyPresetToSystemAgent(versionValidationAgent, currentDefinition, version.VersionKey)
		if err := txSvc.validateSystemAgentPresetState(ctx, versionValidationAgent, currentDefinition); err != nil {
			return err
		}
		if err := txSvc.workspacePresetVersionRepo.Update(ctx, version); err != nil {
			return err
		}
		if err := txSvc.propagateWorkspacePresetVersionToPinnedAgents(ctx, workspaceID, version, currentDefinition); err != nil {
			return err
		}
		definition = &currentDefinition
		return nil
	}); err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "workspace preset version updated",
		"workspace_id", workspaceID,
		"family_key", normalizePresetKey(definition.FamilyKey),
		"version_key", definition.VersionKey,
		"actor_id", strings.TrimSpace(actorID),
	)
	return definition, nil
}

func (s *AgentService) DeleteWorkspacePresetVersion(ctx context.Context, workspaceID, versionID, actorID string) error {
	if s.workspacePresetVersionRepo == nil {
		return fmt.Errorf("workspace preset version repository is not configured")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	versionID = strings.TrimSpace(versionID)
	if workspaceID == "" || versionID == "" {
		return fmt.Errorf("workspace_id and version id are required")
	}

	version, err := s.workspacePresetVersionRepo.GetByID(ctx, workspaceID, versionID)
	if err != nil {
		return err
	}
	if version == nil {
		return fmt.Errorf("%w: version %q not found", ErrWorkspacePresetVersionNotFound, versionID)
	}

	agents, err := s.agentRepo.List(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		if !agent.IsSystem {
			continue
		}
		if normalizePresetKey(agent.PresetKey) != normalizePresetKey(version.FamilyKey) {
			continue
		}
		if normalizePresetVersionKey(agent.PresetVersionKey) != strings.TrimSpace(version.VersionKey) {
			continue
		}
		return fmt.Errorf("%w: cannot delete preset version while pinned to %s", ErrWorkspacePresetVersionPinned, strings.TrimSpace(agent.Name))
	}

	if err := s.workspacePresetVersionRepo.Delete(ctx, workspaceID, versionID); err != nil {
		return err
	}
	slog.InfoContext(ctx, "workspace preset version deleted",
		"workspace_id", workspaceID,
		"family_key", normalizePresetKey(version.FamilyKey),
		"version_key", strings.TrimSpace(version.VersionKey),
		"actor_id", strings.TrimSpace(actorID),
	)
	return nil
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
			Value:                   model.AgentModelProviderAnthropic,
			Label:                   "Anthropic",
			ModelPlaceholder:        "claude-sonnet-4-6",
			SupportsReasoningEffort: false,
			SupportsServiceTier:     false,
		})
	}
	if s.isModelProviderConfigured(model.AgentModelProviderOpenAI) || s.isCodexOpenAIConfigured() {
		options = append(options, model.AgentModelProviderOption{
			Value:                     model.AgentModelProviderOpenAI,
			Label:                     "OpenAI",
			ModelPlaceholder:          "gpt-5.5",
			SupportsReasoningEffort:   true,
			SupportedReasoningEfforts: slices.Clone(supportedAgentReasoningEfforts),
			SupportsServiceTier:       true,
			SupportedServiceTiers:     slices.Clone(supportedAgentServiceTiers),
		})
	}
	if s.isModelProviderConfigured(model.AgentModelProviderOpenRouter) {
		options = append(options, model.AgentModelProviderOption{
			Value:                     model.AgentModelProviderOpenRouter,
			Label:                     "OpenRouter",
			ModelPlaceholder:          "openai/gpt-5.5",
			SupportsReasoningEffort:   true,
			SupportedReasoningEfforts: slices.Clone(supportedAgentReasoningEfforts),
			SupportsServiceTier:       false,
		})
	}
	return options
}

// CreateAgent creates a new agent.
func (s *AgentService) CreateAgent(ctx context.Context, req model.CreateAgentRequest, actorID string) (*model.Agent, error) {
	return s.createCustomAgent(ctx, req, actorID, nil)
}

func (s *AgentService) createCustomAgent(ctx context.Context, req model.CreateAgentRequest, actorID string, sourceTemplate *model.AgentTemplate) (*model.Agent, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if req.PresetKey != nil && strings.TrimSpace(*req.PresetKey) != "" {
		return nil, fmt.Errorf("custom agents cannot specify preset_key")
	}
	if req.PresetVersionKey != nil && strings.TrimSpace(*req.PresetVersionKey) != "" {
		return nil, fmt.Errorf("custom agents cannot specify preset_version_key")
	}

	skills := req.Skills.Normalize()
	if skills == nil {
		skills = model.AgentSkillRefs{}
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "Custom Agent"
	}
	runtimeKind := strings.TrimSpace(stringOrDefault(req.RuntimeKind, "native_sdk"))
	if runtimeKind == "" {
		runtimeKind = "native_sdk"
	}
	triggerMode := stringOrDefault(req.TriggerMode, "manual")
	if triggerMode == "" {
		triggerMode = "manual"
	}
	if err := validateRuntimeKind(runtimeKind); err != nil {
		return nil, err
	}

	approvalMode := "always"
	if req.ApprovalMode != nil && *req.ApprovalMode != "" {
		approvalMode = *req.ApprovalMode
	}
	maxConcurrentRuns := 1
	if req.MaxConcurrentRuns != nil && *req.MaxConcurrentRuns > 0 {
		maxConcurrentRuns = *req.MaxConcurrentRuns
	}
	teamIDs := resolveCreateAgentTeamIDs(req)

	agent := &model.Agent{
		WorkspaceID:                req.WorkspaceID,
		Name:                       strings.TrimSpace(req.Name),
		PresetKey:                  "",
		PresetVersionKey:           "",
		SourcePresetKey:            "",
		SourcePresetVersionKey:     "",
		Role:                       role,
		Status:                     "idle",
		RuntimeKind:                runtimeKind,
		Skills:                     skills,
		TriggerMode:                triggerMode,
		Provider:                   trimPtr(req.Provider),
		Model:                      trimPtr(req.Model),
		ExecutionConfig:            normalizeExecutionConfigJSON(req.ExecutionConfig),
		SystemPrompt:               trimPtr(req.SystemPrompt),
		InstructionTemplateVersion: "",
		PlanningNotes:              nil,
		MonthlyTokenBudget:         normalizeTokenBudget(req.MonthlyTokenBudget),
		TeamID:                     firstTeamIDPtr(teamIDs),
		TeamIDs:                    teamIDs,
		AllowedTools:               normalizeAllowedToolsJSON(normalizeJSONSlice(req.AllowedTools)),
		AllowedCommands:            normalizeJSONSlice(req.AllowedCommands),
		AllowedTargets:             sliceOrPresetJSON(req.AllowedTargets, []string{"task"}),
		ApprovalMode:               approvalMode,
		MaxConcurrentRuns:          maxConcurrentRuns,
		DefaultInvocationMode:      stringOrDefault(req.DefaultInvocationMode, model.InvocationModeInteractive),
	}
	if sourceTemplate != nil {
		agent.SourceTemplateID = &sourceTemplate.ID
		agent.SourceTemplateKey = strings.TrimSpace(sourceTemplate.Key)
	}
	normalizeAgentRecord(agent)
	if err := s.validateAndMaterializeAgentSkills(ctx, agent); err != nil {
		return nil, err
	}
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
	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, agent.WorkspaceID, "agent", agent.ID, &actorID, "created", nil, nil, &newValue, nil)
	}

	s.publishSimpleEvent("created", "agent", agent.ID, agent.WorkspaceID, actorID)

	materializeAgentSystemPrompt(agent)
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
	if req.TeamID != nil && req.TeamIDs != nil {
		return nil, fmt.Errorf("team_ids and legacy team_id cannot both be set")
	}
	if agent.IsSystem {
		systemPresetKey := normalizePresetKey(agent.PresetKey)
		if systemPresetKey == "" {
			systemPresetKey = model.AgentPresetEpicPlanner
		}
		if req.PresetKey != nil && normalizePresetKey(*req.PresetKey) != systemPresetKey {
			return nil, fmt.Errorf("system agent preset cannot be changed")
		}
		if (req.TeamID != nil && trimPtr(req.TeamID) != nil) || (req.TeamIDs != nil && len(normalizeServiceTeamIDs(*req.TeamIDs)) > 0) {
			return nil, fmt.Errorf("system agent cannot be restricted to a team")
		}
		if req.Skills != nil {
			return nil, fmt.Errorf("system agent skills are preset-owned")
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
	if agent.IsSystem && req.PresetVersionKey != nil && !hasPreset {
		return nil, fmt.Errorf("%w: preset version %q no longer exists", ErrWorkspacePresetVersionNotFound, resolvedPresetVersionKey)
	}
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
		agent.Skills = req.Skills.Normalize()
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
	if req.ExecutionConfig != nil {
		agent.ExecutionConfig = normalizeExecutionConfigJSON(req.ExecutionConfig)
	} else if presetChanged && hasPreset {
		agent.ExecutionConfig = normalizeExecutionConfigJSON(preset.ExecutionConfig)
	}
	if req.SystemPrompt != nil {
		agent.SystemPrompt = trimPtr(req.SystemPrompt)
		agent.InstructionTemplateVersion = ""
	}
	if req.PlanningNotes != nil {
		agent.PlanningNotes = trimPtr(req.PlanningNotes)
	}
	if req.MonthlyTokenBudget != nil {
		agent.MonthlyTokenBudget = normalizeTokenBudget(req.MonthlyTokenBudget)
	}
	if req.ActiveTaskID != nil {
		agent.ActiveTaskID = req.ActiveTaskID
	}
	if req.TeamIDs != nil {
		agent.TeamIDs = normalizeServiceTeamIDs(*req.TeamIDs)
		agent.TeamID = firstTeamIDPtr(agent.TeamIDs)
	} else if req.TeamID != nil {
		agent.TeamIDs = resolveLegacyAgentTeamIDs(req.TeamID)
		agent.TeamID = firstTeamIDPtr(agent.TeamIDs)
	}
	if req.AllowedTools != nil {
		agent.AllowedTools = normalizeAllowedToolsJSON(normalizeJSONSlice(req.AllowedTools))
	} else if presetChanged && hasPreset {
		agent.AllowedTools = normalizeAllowedToolsJSON(mustJSONStringSlice(preset.AllowedTools))
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
		if hasPreset {
			s.applyPresetToSystemAgent(agent, preset, resolvedPresetVersionKey)
			if req.Provider != nil {
				agent.Provider = trimPtr(req.Provider)
			}
			if req.Model != nil {
				agent.Model = trimPtr(req.Model)
			}
			if req.ExecutionConfig != nil {
				agent.ExecutionConfig = normalizeExecutionConfigJSON(req.ExecutionConfig)
			}
		} else {
			agent.PresetVersionKey = resolvedPresetVersionKey
			agent.RuntimeKind = defaultRuntimeKindForPresetKey(systemPresetKey)
			agent.TriggerMode = defaultTriggerModeForPresetKey(systemPresetKey)
			agent.TeamID = nil
			agent.TeamIDs = nil
			agent.ApprovalMode = "never"
			agent.SystemPrompt, agent.InstructionTemplateVersion = syncManagedSystemPromptForPreset(systemPresetKey, agent.SystemPrompt, agent.PlanningNotes, agent.InstructionTemplateVersion)
			agent.PlanningNotes = nil
		}
	} else {
		agent.SourcePresetKey = ""
		agent.SourcePresetVersionKey = ""
		agent.PresetKey = ""
		agent.PresetVersionKey = ""
		agent.InstructionTemplateVersion = ""
		if resolvedPresetKey != model.AgentPresetEpicPlanner {
			agent.PlanningNotes = nil
		}
	}
	normalizeAgentRecord(agent)
	if err := s.validateAndMaterializeAgentSkills(ctx, agent); err != nil {
		return nil, err
	}
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

	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, agent.WorkspaceID, "agent", agent.ID, &actorID, "updated", nil, nil, nil, nil)
	}

	s.publishSimpleEvent("updated", "agent", agent.ID, agent.WorkspaceID, actorID)
	materializeAgentSystemPrompt(agent)
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

	if err := s.agentRepo.Delete(ctx, workspaceID, id); err != nil {
		return err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "agent", id, &actorID, "deleted", nil, nil, nil, nil)

	s.publishSimpleEvent("deleted", "agent", id, workspaceID, actorID)

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
	normalized := s.normalizeRunCollection(s.reconcileStuckRuns(ctx, runs))
	s.enrichRunTargets(ctx, workspaceID, normalized)
	return normalized, total, nil
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
	normalized := s.normalizeRunCollection(s.reconcileStuckRuns(ctx, runs))
	s.enrichRunTargets(ctx, workspaceID, normalized)
	return normalized, total, nil
}

// ListRecentRunsForActor returns the most recent runs the given user triggered
// in the workspace. Used by the command runs rail to rehydrate standalone runs
// after a refresh.
func (s *AgentService) ListRecentRunsForActor(ctx context.Context, workspaceID, actorID string, limit int) ([]model.AgentRun, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if actorID == "" {
		return []model.AgentRun{}, nil
	}
	runs, err := s.runRepo.ListRecentForActor(ctx, workspaceID, actorID, limit)
	if err != nil {
		return nil, err
	}
	normalized := s.normalizeRunCollection(s.reconcileStuckRuns(ctx, runs))
	s.enrichRunTargets(ctx, workspaceID, normalized)
	return normalized, nil
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
	single := []model.AgentRun{*run}
	s.enrichRunTargets(ctx, workspaceID, single)
	run.TargetInfo = single[0].TargetInfo
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
	normalized := s.normalizeRunCollection(s.reconcileStuckRuns(ctx, runs))
	s.enrichRunTargets(ctx, workspaceID, normalized)
	return normalized, nil
}

// RunAgent creates a new task-targeted agent run and starts its Temporal workflow.
func (s *AgentService) RunAgent(ctx context.Context, workspaceID, taskID, actorID string) (*model.AgentRun, error) {
	return s.RunTaskAgent(ctx, workspaceID, taskID, actorID, model.StartAgentRunRequest{})
}

// RunTaskAgent starts a task-targeted agent run for an explicit agent.
func (s *AgentService) RunTaskAgent(ctx context.Context, workspaceID, taskID, actorID string, req model.StartAgentRunRequest) (*model.AgentRun, error) {
	task, err := s.taskRepo.GetRawByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	if task == nil || task.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("task not found")
	}

	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	req.AgentID = agentID
	return s.StartTargetRun(ctx, workspaceID, "task", task.ID, req, actorID)
}

// RunEpicAgent starts a direct planner run for an epic.
func (s *AgentService) RunEpicAgent(ctx context.Context, workspaceID, epicID, actorID string, req model.StartAgentRunRequest) (*model.AgentRun, error) {
	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
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
			agentID = refreshedPlanner.ID
		}
	}

	req.AgentID = agentID
	run, err := s.StartTargetRun(ctx, workspaceID, "epic", epic.ID, req, actorID)
	if err != nil {
		return nil, err
	}

	epic.LastPlanningRunID = &run.ID
	if err := s.epicRepo.Update(ctx, epic); err != nil {
		return nil, err
	}
	return run, nil
}

// StartTargetRun starts a direct agent run for a supported target type.
func (s *AgentService) StartTargetRun(ctx context.Context, workspaceID, targetType, targetID string, req model.StartAgentRunRequest, actorID string) (*model.AgentRun, error) {
	return s.startTargetRun(ctx, workspaceID, targetType, targetID, req, strPtr(actorID), manualRunTriggerContext(), nil, nil)
}

func supportCoverageGapRunContext(detail *model.SupportCoverageGapDetail, extra *string) string {
	sections := make([]string, 0, 5)
	if trimmed := strings.TrimSpace(derefString(extra)); trimmed != "" {
		sections = append(sections, "Operator notes:\n"+trimmed)
	}
	if detail == nil {
		return strings.TrimSpace(strings.Join(sections, "\n\n"))
	}

	var b strings.Builder
	b.WriteString("Support coverage gap context:\n")
	writeRunFact := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		b.WriteString(fmt.Sprintf("- %s=%s\n", label, value))
	}
	writeRunFact("gap_id", detail.ID)
	writeRunFact("title", detail.Title)
	writeRunFact("topic", firstNonEmptyCoverageContext(detail.TopicTitle, detail.IssueKey))
	writeRunFact("gap_kind", detail.GapKind)
	writeRunFact("gap_category", detail.GapCategory)
	writeRunFact("v1_gap_type", detail.V1GapType)
	writeRunFact("recommended_action", supportCoverageGapAgentAction(detail))
	writeRunFact("failure_mode", detail.FailureMode)
	writeRunFact("source_signal", detail.SourceSignal)
	writeRunFact("status", detail.Status)
	b.WriteString(fmt.Sprintf("- confidence=%.2f\n", detail.Confidence))
	b.WriteString(fmt.Sprintf("- evidence_count=%d\n", detail.EvidenceCount))
	if detail.AnalysisExplanation != nil {
		b.WriteString("\nAnalysis explanation:\n")
		writeRunFact("customer_need", detail.AnalysisExplanation.CustomerNeed)
		writeRunFact("ai_failure", detail.AnalysisExplanation.AIFailure)
		writeRunFact("human_resolution", detail.AnalysisExplanation.HumanResolution)
		writeRunFact("decision_reason", detail.AnalysisExplanation.DecisionReason)
	}
	if len(detail.RelatedArticles) > 0 {
		b.WriteString("\nRelated docs:\n")
		for i, article := range detail.RelatedArticles {
			if i >= 8 {
				break
			}
			title := firstNonEmptyCoverageContext(article.ArticleTitle, "Untitled article")
			b.WriteString(fmt.Sprintf("- document_id=%s title=%q\n", article.DocumentID, title))
		}
	}
	if len(detail.Recommendations) > 0 {
		b.WriteString("\nRecommendations:\n")
		for i, rec := range detail.Recommendations {
			if i >= 6 {
				break
			}
			b.WriteString(fmt.Sprintf("- type=%s priority=%s target_type=%s target_id=%s title=%q\n",
				strings.TrimSpace(rec.RecommendationType),
				strings.TrimSpace(rec.Priority),
				strings.TrimSpace(rec.TargetType),
				strings.TrimSpace(derefString(rec.TargetID)),
				strings.TrimSpace(rec.TargetTitle),
			))
			if change := strings.TrimSpace(rec.SuggestedChange); change != "" {
				b.WriteString("  suggested_change: " + truncateRunContextText(change, 700) + "\n")
			}
			if notes := strings.TrimSpace(rec.ImplementationNotes); notes != "" {
				b.WriteString("  implementation_notes: " + truncateRunContextText(notes, 500) + "\n")
			}
			if rationale := strings.TrimSpace(rec.Rationale); rationale != "" {
				b.WriteString("  rationale: " + truncateRunContextText(rationale, 500) + "\n")
			}
		}
	}
	if len(detail.Evidence) > 0 {
		b.WriteString("\nEvidence excerpts:\n")
		for i, ev := range detail.Evidence {
			if i >= 10 {
				break
			}
			b.WriteString(fmt.Sprintf("- evidence_type=%s source_signal=%s sender=%s conversation_id=%s document_id=%s\n",
				strings.TrimSpace(ev.EvidenceType),
				strings.TrimSpace(ev.SourceSignal),
				strings.TrimSpace(ev.SenderRole),
				strings.TrimSpace(derefString(ev.ConversationID)),
				strings.TrimSpace(derefString(ev.DocumentID)),
			))
			if excerpt := strings.TrimSpace(ev.Excerpt); excerpt != "" {
				b.WriteString("  excerpt: " + truncateRunContextText(excerpt, 700) + "\n")
			}
		}
	}
	sections = append(sections, strings.TrimSpace(b.String()))
	sections = append(sections, supportCoverageGapAgentInstructions(detail))
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func supportCoverageGapAgentAction(detail *model.SupportCoverageGapDetail) string {
	if detail == nil {
		return "investigate_documentation_gap"
	}
	if primary := primarySupportCoverageRecommendationType(detail); primary != "" {
		switch primary {
		case model.SupportCoverageFixCreateArticle, model.SupportCoverageFixCreateWebsitePage:
			return "write_new_doc_or_route_to_better_docs_surface"
		case model.SupportCoverageFixUpdateArticle, model.SupportCoverageFixUpdateWebsitePage:
			return "improve_existing_doc_and_avoid_duplicate_docs"
		case model.SupportCoverageFixAddData:
			return "identify_missing_data_and_prepare_docs_or_data_handoff"
		case model.SupportCoverageFixAddAction:
			return "identify_missing_action_and_prepare_docs_or_product_handoff"
		case model.SupportCoverageFixDefinePolicy:
			return "write_or_update_policy_docs"
		case model.SupportCoverageFixImproveWorkflow:
			return "update_internal_workflow_docs_or_handoff_process_gap"
		case model.SupportCoverageFixNoFix:
			return "summarize_no_documentation_fix_and_request_human_decision"
		}
	}
	switch strings.TrimSpace(detail.V1GapType) {
	case model.SupportCoverageV1GapMissingArticle:
		return "write_new_doc_or_route_to_better_docs_surface"
	case model.SupportCoverageV1GapWeakArticle:
		return "improve_existing_doc_and_avoid_duplicate_docs"
	case model.SupportCoverageV1GapOutdatedOrConflictingArticle:
		return "reconcile_outdated_or_conflicting_docs"
	case model.SupportCoverageV1GapNeedsReview:
		return "investigate_and_request_clarification_before_drafting"
	default:
		return "investigate_documentation_gap"
	}
}

func supportCoverageGapAgentInstructions(detail *model.SupportCoverageGapDetail) string {
	action := supportCoverageGapAgentAction(detail)
	return strings.Join([]string{
		"Documentation Agent routing instructions:",
		"- Treat this support coverage gap as an operations inbox item, not a generic writing prompt.",
		"- First decide whether the fix belongs in public help docs, API docs, internal docs, multiple surfaces, or outside documentation.",
		"- Use recommended_action=" + action + " as the starting strategy, then verify it against evidence and related docs.",
		"- If this is a data, action, policy, or workflow gap, only create docs when documentation is part of the fix; otherwise prepare a concise handoff that names the owner, missing capability, and customer impact.",
		"- Prefer improving linked docs for weak or conflicting gaps; avoid creating duplicate articles.",
		"- For missing docs, write the right document type and place it in the appropriate collection or propose where it belongs.",
		"- For needs_review gaps, summarize the ambiguity and ask for clarification or create a review checkpoint before drafting.",
		"- Do not mark the gap resolved unless a draft, proposal, or explicit human handoff exists.",
	}, "\n")
}

func primarySupportCoverageRecommendationType(detail *model.SupportCoverageGapDetail) string {
	for _, rec := range detail.Recommendations {
		if strings.TrimSpace(rec.Priority) == model.SupportCoverageRecommendationPriorityPrimary {
			return strings.TrimSpace(rec.RecommendationType)
		}
	}
	if len(detail.Recommendations) > 0 {
		return strings.TrimSpace(detail.Recommendations[0].RecommendationType)
	}
	return ""
}

func firstNonEmptyCoverageContext(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncateRunContextText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit]) + "..."
}

func (s *AgentService) startTargetRun(ctx context.Context, workspaceID, targetType, targetID string, req model.StartAgentRunRequest, actorID *string, trigger *model.AgentRunTriggerContext, event *model.AgentRunEventContext, parentRunID *string) (*model.AgentRun, error) {
	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	if strings.TrimSpace(targetType) == "doc" {
		targetType = "document"
	}

	switch targetType {
	case "task", "story":
		task, err := s.taskRepo.GetRawByID(ctx, targetID)
		if err != nil {
			return nil, fmt.Errorf("get task: %w", err)
		}
		if task == nil || task.WorkspaceID != workspaceID {
			return nil, fmt.Errorf("task not found")
		}

		agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "task")
		if err != nil {
			return nil, err
		}
		resolved := worker.ResolveAgentProfile(agent, resolveInvocationMode(agent))

		var delivery *model.TaskDeliveryTarget
		if s.gitService != nil {
			delivery, err = s.gitService.ResolveTaskDeliveryTargetForRun(ctx, workspaceID, task.ID, resolved.RequiresRepo)
			if err != nil {
				return nil, err
			}
		}

		if err := validateRunAllowedTools(req.AllowedTools, agent); err != nil {
			return nil, err
		}
		payload, err := buildAgentRunInputPayload("task", task.ID, trigger, event, req.Output, req.AdditionalContext, req.AllowedTools)
		if err != nil {
			return nil, fmt.Errorf("build task run input: %w", err)
		}

		run, err := s.createRun(ctx, createRunParams{
			workspaceID:    workspaceID,
			agent:          agent,
			targetType:     "task",
			targetID:       task.ID,
			parentRunID:    parentRunID,
			taskID:         &task.ID,
			actorID:        actorID,
			input:          payload,
			trigger:        trigger,
			delivery:       delivery,
			baseBranch:     req.BaseBranch,
			workingBranch:  req.WorkingBranch,
			invocationMode: resolveInvocationMode(agent),
		})
		if err != nil {
			return nil, err
		}
		if s.activitySvc != nil {
			_ = s.activitySvc.Log(ctx, workspaceID, "task", task.ID, actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), agentRunActivityMetadata(agent, run, "started"))
		}
		s.publishRunEvent(run, derefString(actorID))
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

		agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "epic")
		if err != nil {
			return nil, err
		}
		if err := validateRunAllowedTools(req.AllowedTools, agent); err != nil {
			return nil, err
		}
		payload, err := buildAgentRunInputPayload("epic", epic.ID, trigger, event, req.Output, req.AdditionalContext, req.AllowedTools)
		if err != nil {
			return nil, fmt.Errorf("build epic run input: %w", err)
		}

		run, err := s.createRun(ctx, createRunParams{
			workspaceID:    workspaceID,
			agent:          agent,
			targetType:     "epic",
			targetID:       epic.ID,
			parentRunID:    parentRunID,
			actorID:        actorID,
			input:          payload,
			trigger:        trigger,
			baseBranch:     req.BaseBranch,
			workingBranch:  req.WorkingBranch,
			invocationMode: resolveInvocationMode(agent),
		})
		if err != nil {
			return nil, err
		}

		if s.activitySvc != nil {
			_ = s.activitySvc.Log(ctx, workspaceID, "epic", epic.ID, actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), agentRunActivityMetadata(agent, run, "started"))
		}
		s.publishRunEvent(run, derefString(actorID))
		return run, nil

	case "repository":
		if s.gitService == nil {
			return nil, fmt.Errorf("git service not configured")
		}
		repo, err := s.gitService.GetRepositoryByID(ctx, workspaceID, targetID)
		if err != nil {
			return nil, fmt.Errorf("get repository: %w", err)
		}
		if repo == nil {
			return nil, fmt.Errorf("repository not found")
		}
		if repo.Archived || !repo.Selected {
			return nil, fmt.Errorf("repository is not available for agent runs")
		}

		agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "repository")
		if err != nil {
			return nil, err
		}
		if err := validateRunAllowedTools(req.AllowedTools, agent); err != nil {
			return nil, err
		}
		payload, err := buildAgentRunInputPayload("repository", repo.ID, trigger, event, req.Output, req.AdditionalContext, req.AllowedTools)
		if err != nil {
			return nil, fmt.Errorf("build repository run input: %w", err)
		}

		repoID := repo.ID
		repoFullName := strings.TrimSpace(repo.FullName)
		baseBranch := strings.TrimSpace(derefString(req.BaseBranch))
		if baseBranch == "" {
			baseBranch = strings.TrimSpace(repo.DefaultBranch)
		}
		if baseBranch == "" {
			baseBranch = "main"
		}
		workingBranch := strings.TrimSpace(derefString(req.WorkingBranch))

		run, err := s.createRun(ctx, createRunParams{
			workspaceID:    workspaceID,
			agent:          agent,
			targetType:     "repository",
			targetID:       repo.ID,
			parentRunID:    parentRunID,
			actorID:        actorID,
			input:          payload,
			trigger:        trigger,
			repositoryID:   &repoID,
			repoFullName:   strPtr(repoFullName),
			baseBranch:     strPtr(baseBranch),
			workingBranch:  nilIfEmpty(workingBranch),
			invocationMode: resolveInvocationMode(agent),
		})
		if err != nil {
			return nil, err
		}

		if s.activitySvc != nil {
			_ = s.activitySvc.Log(ctx, workspaceID, "git_repository", repo.ID, actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), agentRunActivityMetadata(agent, run, "started"))
		}
		s.publishRunEvent(run, derefString(actorID))
		return run, nil

	case "support_conversation":
		conversation, err := s.conversationRepo.GetByID(ctx, workspaceID, targetID, "", model.RoleOwner)
		if err != nil {
			return nil, fmt.Errorf("get conversation: %w", err)
		}
		if conversation == nil {
			return nil, fmt.Errorf("conversation not found")
		}

		agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "support_conversation")
		if err != nil {
			return nil, err
		}
		if err := validateRunAllowedTools(req.AllowedTools, agent); err != nil {
			return nil, err
		}
		input, err := buildAgentRunInputPayload("support_conversation", conversation.ID, trigger, event, req.Output, req.AdditionalContext, req.AllowedTools)
		if err != nil {
			return nil, fmt.Errorf("build conversation run input: %w", err)
		}

		run, err := s.createRun(ctx, createRunParams{
			workspaceID:    workspaceID,
			agent:          agent,
			targetType:     "support_conversation",
			targetID:       conversation.ID,
			parentRunID:    parentRunID,
			conversationID: &conversation.ID,
			actorID:        actorID,
			input:          input,
			trigger:        trigger,
			invocationMode: resolveInvocationMode(agent),
		})
		if err != nil {
			return nil, err
		}

		if s.activitySvc != nil {
			_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversation.ID, actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), agentRunActivityMetadata(agent, run, "started"))
		}
		s.publishRunEvent(run, derefString(actorID))
		return run, nil

	case "support_coverage_gap":
		if s.supportCoverageSvc == nil {
			return nil, fmt.Errorf("support coverage service not configured")
		}
		detail, err := s.supportCoverageSvc.GetGapDetail(ctx, workspaceID, targetID)
		if err != nil {
			return nil, fmt.Errorf("get support coverage gap: %w", err)
		}
		if detail == nil {
			return nil, fmt.Errorf("support coverage gap not found")
		}

		agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "support_coverage_gap")
		if err != nil {
			return nil, err
		}
		if err := validateRunAllowedTools(req.AllowedTools, agent); err != nil {
			return nil, err
		}

		context := supportCoverageGapRunContext(detail, req.AdditionalContext)
		input, err := buildAgentRunInputPayload("support_coverage_gap", detail.ID, trigger, event, req.Output, &context, req.AllowedTools)
		if err != nil {
			return nil, fmt.Errorf("build support coverage gap run input: %w", err)
		}

		run, err := s.createRun(ctx, createRunParams{
			workspaceID:    workspaceID,
			agent:          agent,
			targetType:     "support_coverage_gap",
			targetID:       detail.ID,
			parentRunID:    parentRunID,
			actorID:        actorID,
			input:          input,
			trigger:        trigger,
			invocationMode: resolveInvocationMode(agent),
		})
		if err != nil {
			return nil, err
		}

		if s.activitySvc != nil {
			_ = s.activitySvc.Log(ctx, workspaceID, "support_coverage_gap", detail.ID, actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), agentRunActivityMetadata(agent, run, "started"))
		}
		s.publishRunEvent(run, derefString(actorID))
		return run, nil

	case "document":
		if s.docsDocumentRepo == nil {
			return nil, fmt.Errorf("docs document repository not configured")
		}
		doc, err := s.docsDocumentRepo.GetByID(ctx, targetID)
		if err != nil {
			return nil, fmt.Errorf("get document: %w", err)
		}
		if doc == nil || doc.WorkspaceID != workspaceID {
			return nil, fmt.Errorf("document not found")
		}
		agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "document")
		if err != nil {
			return nil, err
		}
		if err := validateRunAllowedTools(req.AllowedTools, agent); err != nil {
			return nil, err
		}
		input, err := buildAgentRunInputPayload("document", doc.ID, trigger, event, req.Output, req.AdditionalContext, req.AllowedTools)
		if err != nil {
			return nil, fmt.Errorf("build document run input: %w", err)
		}
		run, err := s.createRun(ctx, createRunParams{
			workspaceID:    workspaceID,
			agent:          agent,
			targetType:     "document",
			targetID:       doc.ID,
			parentRunID:    parentRunID,
			actorID:        actorID,
			input:          input,
			trigger:        trigger,
			invocationMode: resolveInvocationMode(agent),
		})
		if err != nil {
			return nil, err
		}
		s.publishRunEvent(run, derefString(actorID))
		return run, nil

	case "crm_contact":
		if s.crmContactRepo == nil {
			return nil, fmt.Errorf("crm contact repository not configured")
		}
		contact, err := s.crmContactRepo.GetByID(ctx, targetID)
		if err != nil {
			return nil, fmt.Errorf("get crm contact: %w", err)
		}
		if contact == nil || contact.WorkspaceID != workspaceID {
			return nil, fmt.Errorf("crm contact not found")
		}
		agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "crm_contact")
		if err != nil {
			return nil, err
		}
		if err := validateRunAllowedTools(req.AllowedTools, agent); err != nil {
			return nil, err
		}
		input, err := buildAgentRunInputPayload("crm_contact", contact.ID, trigger, event, req.Output, req.AdditionalContext, req.AllowedTools)
		if err != nil {
			return nil, fmt.Errorf("build crm contact run input: %w", err)
		}
		run, err := s.createRun(ctx, createRunParams{
			workspaceID:    workspaceID,
			agent:          agent,
			targetType:     "crm_contact",
			targetID:       contact.ID,
			parentRunID:    parentRunID,
			actorID:        actorID,
			input:          input,
			trigger:        trigger,
			invocationMode: resolveInvocationMode(agent),
		})
		if err != nil {
			return nil, err
		}
		s.publishRunEvent(run, derefString(actorID))
		return run, nil

	case "crm_deal":
		if s.crmDealRepo == nil {
			return nil, fmt.Errorf("crm deal repository not configured")
		}
		deal, err := s.crmDealRepo.GetByID(ctx, targetID)
		if err != nil {
			return nil, fmt.Errorf("get crm deal: %w", err)
		}
		if deal == nil || deal.WorkspaceID != workspaceID {
			return nil, fmt.Errorf("crm deal not found")
		}
		agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "crm_deal")
		if err != nil {
			return nil, err
		}
		if err := validateRunAllowedTools(req.AllowedTools, agent); err != nil {
			return nil, err
		}
		input, err := buildAgentRunInputPayload("crm_deal", deal.ID, trigger, event, req.Output, req.AdditionalContext, req.AllowedTools)
		if err != nil {
			return nil, fmt.Errorf("build crm deal run input: %w", err)
		}
		run, err := s.createRun(ctx, createRunParams{
			workspaceID:    workspaceID,
			agent:          agent,
			targetType:     "crm_deal",
			targetID:       deal.ID,
			parentRunID:    parentRunID,
			actorID:        actorID,
			input:          input,
			trigger:        trigger,
			invocationMode: resolveInvocationMode(agent),
		})
		if err != nil {
			return nil, err
		}
		s.publishRunEvent(run, derefString(actorID))
		return run, nil

	case "workspace":
		if strings.TrimSpace(targetID) != workspaceID {
			return nil, fmt.Errorf("workspace target must match workspace id")
		}

		agent, err := s.requireRunnableAgent(ctx, workspaceID, agentID, "workspace")
		if err != nil {
			return nil, err
		}
		if err := validateRunAllowedTools(req.AllowedTools, agent); err != nil {
			return nil, err
		}
		input, err := buildAgentRunInputPayload("workspace", workspaceID, trigger, event, req.Output, req.AdditionalContext, req.AllowedTools)
		if err != nil {
			return nil, fmt.Errorf("build workspace run input: %w", err)
		}

		run, err := s.createRun(ctx, createRunParams{
			workspaceID:    workspaceID,
			agent:          agent,
			targetType:     "workspace",
			targetID:       workspaceID,
			parentRunID:    parentRunID,
			actorID:        actorID,
			input:          input,
			trigger:        trigger,
			invocationMode: resolveInvocationMode(agent),
		})
		if err != nil {
			return nil, err
		}

		if s.activitySvc != nil {
			_ = s.activitySvc.Log(ctx, workspaceID, "workspace", workspaceID, actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), agentRunActivityMetadata(agent, run, "started"))
		}
		s.publishRunEvent(run, derefString(actorID))
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
	trigger := manualRunTriggerContext()
	if actorID == nil || strings.TrimSpace(*actorID) == "" {
		trigger = systemRunTriggerContext(supportAutoTriggerType)
	}
	input, err := buildAgentRunInputPayload("support_conversation", conversationID, trigger, nil, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("build conversation run input: %w", err)
	}

	run, err := s.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		targetType:     "support_conversation",
		targetID:       conversationID,
		conversationID: &conversationID,
		actorID:        actorID,
		input:          input,
		trigger:        trigger,
		invocationMode: resolveInvocationMode(agent),
	})
	if err != nil {
		return nil, err
	}

	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversationID, actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), agentRunActivityMetadata(agent, run, "started"))
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
	if req.Intent == model.AgentRunResumeIntentApprove && s.ruleEngine != nil && (run.TargetType == "task" || run.TargetType == "story") && run.TaskID != nil {
		task, taskErr := s.taskRepo.GetRawByID(ctx, *run.TaskID)
		if taskErr == nil && task != nil {
			s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
				WorkspaceID: run.WorkspaceID,
				TriggerType: model.TriggerAgentRunApproved,
				TaskID:      task.ID,
				StoryID:     task.ID, // backward-compat alias
				StateID:     task.WorkflowStateID,
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

// ContinueTerminalRun creates a new child run from a failed or cancelled run.
func (s *AgentService) ContinueTerminalRun(ctx context.Context, workspaceID, runID, actorID string, req model.ContinueAgentRunRequest) (*model.AgentRun, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	switch strings.TrimSpace(run.Status) {
	case model.AgentRunStatusFailed, model.AgentRunStatusCancelled:
	default:
		return nil, fmt.Errorf("only failed or cancelled runs can be continued")
	}

	additionalContext := buildContinuationAdditionalContext(run, derefString(req.Content))
	var previousInput model.AgentRunInputPayload
	_ = json.Unmarshal(run.Input, &previousInput)
	startReq := model.StartAgentRunRequest{
		AgentID:           run.AgentID,
		AdditionalContext: &additionalContext,
		BaseBranch:        run.BaseBranch,
		WorkingBranch:     run.WorkingBranch,
		Output:            previousInput.Output,
	}
	return s.startTargetRun(
		ctx,
		workspaceID,
		run.TargetType,
		run.TargetID,
		startReq,
		strPtr(actorID),
		manualRunTriggerContext(),
		&model.AgentRunEventContext{
			RunID:  &run.ID,
			Reason: strPtr("continued_from_terminal_run"),
		},
		&run.ID,
	)
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
	if len(req.ResponsePayload) > 0 && strings.TrimSpace(string(req.ResponsePayload)) != "" && strings.TrimSpace(string(req.ResponsePayload)) != "null" {
		signal.ResponsePayload = append(json.RawMessage(nil), req.ResponsePayload...)
	}
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
		if err := s.resolveLatestPendingInteraction(ctx, run, actorID, previousPauseReason, signal.Intent, replyText, req.ResponsePayload); err != nil {
			slog.ErrorContext(ctx, "failed to resolve run interaction",
				"error", err,
				"workspace_id", run.WorkspaceID,
				"run_id", run.ID,
				"pause_reason", previousPauseReason,
				"intent", signal.Intent,
			)
		}
		s.publishRunEvent(run, actorID)
		return run, message, nil
	}
	run.Status = model.AgentRunStatusRunning
	run.PauseReason = model.AgentRunPauseReasonNone
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, nil, err
	}
	if err := s.markAgentWorking(ctx, workspaceID, run.AgentID, run.TaskID); err != nil {
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
	if err := s.resolveLatestPendingInteraction(ctx, run, actorID, previousPauseReason, signal.Intent, replyText, req.ResponsePayload); err != nil {
		slog.ErrorContext(ctx, "failed to resolve run interaction",
			"error", err,
			"workspace_id", run.WorkspaceID,
			"run_id", run.ID,
			"pause_reason", previousPauseReason,
			"intent", signal.Intent,
		)
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

func (s *AgentService) resolveLatestPendingInteraction(ctx context.Context, run *model.AgentRun, actorID, pauseReason, signalIntent, content string, explicitResponsePayload json.RawMessage) error {
	if s == nil || s.interactionRepo == nil || run == nil {
		return nil
	}

	interaction, err := s.interactionRepo.GetLatestPendingByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil || interaction == nil {
		return err
	}

	responsePayload := append(json.RawMessage(nil), explicitResponsePayload...)
	responseSchemaVersion := strings.TrimSpace(interaction.RequestSchemaVersion)
	if len(responsePayload) == 0 || strings.TrimSpace(string(responsePayload)) == "" || strings.TrimSpace(string(responsePayload)) == "null" {
		var err error
		responsePayload, responseSchemaVersion, err = buildInteractionResponsePayload(interaction, normalizeResolvedInteractionIntent(pauseReason, signalIntent), content)
		if err != nil {
			return err
		}
	} else if responseSchemaVersion == "" {
		responseSchemaVersion = model.AgentRunInteractionSchemaVersionHelpinV1
	}

	now := time.Now().UTC()
	interaction.Status = model.AgentRunInteractionStatusResolved
	interaction.ResponsePayload = responsePayload
	interaction.ResponseSchemaVersion = stringPtrIfNotEmpty(responseSchemaVersion)
	interaction.ResolvedBy = stringPtrIfNotEmpty(actorID)
	interaction.ResolvedAt = &now
	if err := s.interactionRepo.Update(ctx, interaction); err != nil {
		return err
	}
	if err := s.persistResolvedInteractionArtifacts(ctx, run, interaction, actorID); err != nil {
		return err
	}
	s.clearAgentAttentionNotification(ctx, run)
	return nil
}

func normalizeResolvedInteractionIntent(pauseReason, signalIntent string) string {
	switch normalizeResumeIntent(signalIntent) {
	case model.AgentRunResumeIntentApprove:
		return model.AgentRunResumeIntentApprove
	case model.AgentRunResumeIntentRequestChanges:
		return model.AgentRunResumeIntentRequestChanges
	case model.AgentRunResumeIntentReply:
		if strings.TrimSpace(pauseReason) == model.AgentRunPauseReasonHumanApproval {
			return model.AgentRunResumeIntentRequestChanges
		}
		return model.AgentRunResumeIntentReply
	default:
		return strings.TrimSpace(signalIntent)
	}
}

func buildInteractionResponsePayload(interaction *model.AgentRunInteraction, resolvedIntent, content string) (json.RawMessage, string, error) {
	if interaction == nil {
		return nil, "", nil
	}

	switch strings.TrimSpace(interaction.RequestSchemaVersion) {
	case model.AgentRunInteractionSchemaVersionCodexV2:
		switch strings.TrimSpace(interaction.InteractionKind) {
		case model.AgentRunInteractionKindRequestUserInput:
			payload, err := worker.BuildCodexUserInputResponseFromPayload(interaction.RequestPayload, content)
			return payload, model.AgentRunInteractionSchemaVersionCodexV2, err
		case model.AgentRunInteractionKindCommandExecutionApproval, model.AgentRunInteractionKindFileChangeApproval, model.AgentRunInteractionKindPermissionsApproval:
			payload, err := worker.BuildCodexApprovalResponseFromPayload(
				codexPendingKindForInteraction(strings.TrimSpace(interaction.InteractionKind)),
				interaction.RequestPayload,
				resolvedIntent == model.AgentRunResumeIntentApprove,
				resolvedIntent == model.AgentRunResumeIntentRequestChanges,
			)
			return payload, model.AgentRunInteractionSchemaVersionCodexV2, err
		}
	}

	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindRequestUserInput:
		payload, err := json.Marshal(map[string]any{
			"content": strings.TrimSpace(content),
		})
		return payload, model.AgentRunInteractionSchemaVersionHelpinV1, err
	case model.AgentRunInteractionKindApprovalRequest:
		payload := map[string]any{}
		switch resolvedIntent {
		case model.AgentRunResumeIntentApprove:
			payload["decision"] = "approve"
		default:
			payload["decision"] = "request_changes"
			if trimmed := strings.TrimSpace(content); trimmed != "" {
				payload["message"] = trimmed
			}
		}
		raw, err := json.Marshal(payload)
		return raw, model.AgentRunInteractionSchemaVersionHelpinV1, err
	case model.AgentRunInteractionKindReviewCheckpoint:
		payload := map[string]any{}
		switch resolvedIntent {
		case model.AgentRunResumeIntentApprove:
			payload["decision"] = "approve"
		default:
			payload["decision"] = "request_changes"
			if trimmed := strings.TrimSpace(content); trimmed != "" {
				payload["message"] = trimmed
			}
		}
		raw, err := json.Marshal(payload)
		return raw, model.AgentRunInteractionSchemaVersionHelpinV1, err
	default:
		payload, err := json.Marshal(map[string]any{
			"intent":  strings.TrimSpace(resolvedIntent),
			"content": strings.TrimSpace(content),
		})
		return payload, model.AgentRunInteractionSchemaVersionHelpinV1, err
	}
}

func (s *AgentService) persistResolvedInteractionArtifacts(ctx context.Context, run *model.AgentRun, interaction *model.AgentRunInteraction, actorID string) error {
	if s == nil || run == nil || interaction == nil {
		return nil
	}
	switch strings.TrimSpace(interaction.InteractionKind) {
	case model.AgentRunInteractionKindReviewCheckpoint:
		artifact := reviewDecisionArtifactFromInteraction(interaction, actorID)
		if artifact == nil {
			return nil
		}
		return s.saveJSONArtifact(ctx, run, model.AgentRunArtifactTypeReviewDecision, "json", artifact)
	default:
		return nil
	}
}

func reviewDecisionArtifactFromInteraction(interaction *model.AgentRunInteraction, actorID string) *model.ReviewDecisionArtifact {
	if interaction == nil || strings.TrimSpace(interaction.InteractionKind) != model.AgentRunInteractionKindReviewCheckpoint {
		return nil
	}

	var request model.ReviewCheckpointRequest
	if err := json.Unmarshal(interaction.RequestPayload, &request); err != nil {
		return nil
	}

	var response model.ReviewCheckpointResponse
	if err := json.Unmarshal(interaction.ResponsePayload, &response); err != nil {
		return nil
	}
	response.Decision = strings.TrimSpace(response.Decision)
	if response.Decision == "" {
		return nil
	}
	response.Message = strings.TrimSpace(response.Message)
	response.SelectionMode = strings.ToLower(strings.TrimSpace(response.SelectionMode))

	selectedIDs := make(map[string]struct{}, len(response.SelectedFindingIDs))
	for _, id := range response.SelectedFindingIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		selectedIDs[id] = struct{}{}
	}

	statusForSelection := "requested_changes"
	if response.Decision == "approve" {
		statusForSelection = "approved"
	} else if response.Decision == "skip" {
		statusForSelection = "skipped"
	}

	findings := make([]model.ReviewDecisionFinding, 0, len(request.Findings))
	for _, finding := range request.Findings {
		findingID := strings.TrimSpace(finding.ID)
		if findingID == "" {
			continue
		}
		if response.SelectionMode == "selected" {
			if _, ok := selectedIDs[findingID]; !ok {
				continue
			}
		}
		findings = append(findings, model.ReviewDecisionFinding{
			ID:           findingID,
			Title:        strings.TrimSpace(finding.Title),
			CodeLocation: strings.TrimSpace(finding.CodeLocation),
			Status:       statusForSelection,
		})
	}

	resolvedAt := time.Now().UTC()
	if interaction.ResolvedAt != nil && !interaction.ResolvedAt.IsZero() {
		resolvedAt = interaction.ResolvedAt.UTC()
	}

	return &model.ReviewDecisionArtifact{
		Phase:                      strings.TrimSpace(request.Phase),
		Title:                      strings.TrimSpace(request.Title),
		Summary:                    strings.TrimSpace(request.Summary),
		Decision:                   response.Decision,
		Message:                    response.Message,
		SelectionMode:              response.SelectionMode,
		Findings:                   findings,
		AssistantMessageSequenceNo: interactionAssistantSequenceNo(*interaction),
		ResolvedBy:                 strings.TrimSpace(actorID),
		ResolvedAt:                 resolvedAt,
	}
}

func codexPendingKindForInteraction(interactionKind string) string {
	switch strings.TrimSpace(interactionKind) {
	case model.AgentRunInteractionKindCommandExecutionApproval:
		return "command_execution"
	case model.AgentRunInteractionKindFileChangeApproval:
		return "file_change"
	case model.AgentRunInteractionKindPermissionsApproval:
		return "permissions"
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
		if err := s.markAgentWorking(ctx, workspaceID, run.AgentID, run.TaskID); err != nil {
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
	s.publishCodingSessionEvent(run, "auth.updated", map[string]any{
		"state":            authState.State,
		"provider":         authState.Provider,
		"auth_mode":        authState.AuthMode,
		"login_id":         derefString(authState.LoginID),
		"auth_url":         derefString(authState.AuthURL),
		"verification_url": derefString(authState.VerificationURL),
		"user_code":        derefString(authState.UserCode),
		"error":            derefString(authState.Error),
	}, actorID)
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
	sourceMessage, assistantSequenceNo, approval, preview, err := latestApprovalCheckpoint(messages, existingArtifacts)
	if err != nil {
		return err
	}
	if approval == nil {
		return nil
	}
	sourceMessageID := ""
	if sourceMessage != nil {
		sourceMessageID = strings.TrimSpace(sourceMessage.ID)
	}
	for _, artifact := range existingArtifacts {
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeApprovedPreview || artifact.InlineContent == nil {
			continue
		}
		var existing model.ApprovedRunPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &existing); err != nil {
			continue
		}
		if sourceMessageID != "" && strings.TrimSpace(existing.SourceMessageID) == sourceMessageID {
			return nil
		}
		if sourceMessageID == "" && assistantSequenceNo > 0 && existing.AssistantMessageSequenceNo == assistantSequenceNo {
			return nil
		}
	}
	if preview == nil {
		preview, err = latestRunPreviewArtifactForApproval(existingArtifacts, assistantSequenceNo, approval)
		if err != nil {
			return err
		}
	}
	if preview == nil {
		return nil
	}

	content := append(json.RawMessage(nil), preview.Content...)
	if strings.EqualFold(strings.TrimSpace(approval.Phase), "tasks") && strings.EqualFold(strings.TrimSpace(preview.Format), worker.PreviewFormatJSON) {
		normalizedContent, err := worker.NormalizeTaskPlanPreviewContent(content)
		if err != nil {
			if approvedPreviewDebugEnabled() {
				slog.ErrorContext(ctx, "approved task plan preview normalization failed during approval persistence",
					"run_id", run.ID,
					"workspace_id", run.WorkspaceID,
					"phase", strings.TrimSpace(approval.Phase),
					"panel_key", strings.TrimSpace(preview.PanelKey),
					"format", strings.TrimSpace(preview.Format),
					"content_preview", previewDebugSnippet(content, 1600),
					"error", err,
				)
			}
			return fmt.Errorf("approved task plan preview content must be valid JSON matching the canonical task-plan shape {summary, proposed_tasks}; use task fields like name, description, task_type, acceptance_criteria, and dependency_refs")
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
			"source_message_id", sourceMessageID,
			"assistant_message_sequence_no", assistantSequenceNo,
			"content_preview", previewDebugSnippet(content, 1600),
		)
	}

	payload := model.ApprovedRunPreview{
		Phase:                      strings.TrimSpace(approval.Phase),
		ApprovalTitle:              strings.TrimSpace(approval.Title),
		ApprovalSummary:            strings.TrimSpace(approval.Summary),
		PanelKey:                   strings.TrimSpace(preview.PanelKey),
		PreviewTitle:               strings.TrimSpace(preview.Title),
		Format:                     strings.TrimSpace(preview.Format),
		Content:                    content,
		SourceMessageID:            sourceMessageID,
		AssistantMessageSequenceNo: assistantSequenceNo,
		ApprovedBy:                 strings.TrimSpace(actorID),
		ApprovedAt:                 time.Now().UTC(),
	}
	return s.saveJSONArtifact(ctx, run, model.AgentRunArtifactTypeApprovedPreview, "json", payload)
}

func latestApprovalCheckpoint(messages []model.AgentRunMessage, artifacts []model.AgentRunArtifact) (*model.AgentRunMessage, int, *model.ApprovalRequest, *worker.PublishedPreview, error) {
	return latestApprovalCheckpointFromArtifacts(messages, artifacts)
}

func latestApprovalCheckpointFromArtifacts(messages []model.AgentRunMessage, artifacts []model.AgentRunArtifact) (*model.AgentRunMessage, int, *model.ApprovalRequest, *worker.PublishedPreview, error) {
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != model.AgentRunArtifactTypeHumanApprovalRequest || artifact.InlineContent == nil {
			continue
		}

		var approval model.ApprovalRequest
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &approval); err != nil {
			return nil, 0, nil, nil, fmt.Errorf("parse approval request artifact: %w", err)
		}
		approval.PreviewPanelKey = normalizeApprovalPreviewPanelKey(approval.PreviewPanelKey)
		if strings.TrimSpace(approval.Title) == "" {
			continue
		}

		assistantSequenceNo := artifactAssistantMessageSequenceNo(artifact)
		if assistantSequenceNo <= 0 {
			continue
		}
		sourceMessage := findAssistantMessageBySequence(messages, assistantSequenceNo)

		preview, err := latestRunPreviewArtifactForApproval(artifacts, assistantSequenceNo, &approval)
		if err != nil {
			return nil, 0, nil, nil, err
		}
		return sourceMessage, assistantSequenceNo, &approval, preview, nil
	}
	return nil, 0, nil, nil, nil
}

func normalizeApprovalPreviewPanelKey(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	switch key {
	case worker.ToolPublishPRDDraft:
		return "prd_draft"
	case worker.ToolPublishTaskPlan:
		return "task_plan"
	case worker.ToolPublishTaskPlanDoc:
		return "task_plan_doc"
	default:
		return key
	}
}

func isCanonicalApprovalPreviewPanelKey(value string) bool {
	switch normalizeApprovalPreviewPanelKey(value) {
	case "prd_draft", "task_plan", "task_plan_doc":
		return true
	default:
		return false
	}
}

func latestRunPreviewArtifactForApproval(artifacts []model.AgentRunArtifact, assistantSequenceNo int, approval *model.ApprovalRequest) (*worker.PublishedPreview, error) {
	panelKey := ""
	if approval != nil {
		panelKey = normalizeApprovalPreviewPanelKey(approval.PreviewPanelKey)
	}
	preview, count, err := latestRunPreviewArtifactForAssistantSequence(artifacts, assistantSequenceNo, panelKey)
	if err != nil {
		return nil, err
	}
	if preview != nil {
		return preview, nil
	}
	if panelKey != "" {
		if !isCanonicalApprovalPreviewPanelKey(panelKey) {
			fallbackPreview, fallbackCount, err := latestRunPreviewArtifactForAssistantSequence(artifacts, assistantSequenceNo, "")
			if err != nil {
				return nil, err
			}
			if fallbackCount == 1 {
				return fallbackPreview, nil
			}
		}
		return latestRunPreviewArtifact(artifacts, panelKey)
	}
	if count > 1 {
		return nil, nil
	}
	return latestRunPreviewArtifact(artifacts, "")
}

func latestRunPreviewArtifact(artifacts []model.AgentRunArtifact, panelKey string) (*worker.PublishedPreview, error) {
	targetKey := normalizeApprovalPreviewPanelKey(panelKey)
	for i := len(artifacts) - 1; i >= 0; i-- {
		artifact := artifacts[i]
		if strings.TrimSpace(artifact.ArtifactType) != worker.RunPreviewArtifactType || artifact.InlineContent == nil {
			continue
		}

		var payload worker.PublishedPreview
		if err := json.Unmarshal([]byte(*artifact.InlineContent), &payload); err != nil {
			return nil, fmt.Errorf("parse run preview artifact: %w", err)
		}
		if targetKey != "" && normalizeApprovalPreviewPanelKey(payload.PanelKey) != targetKey {
			continue
		}
		return &payload, nil
	}
	return nil, nil
}

func latestRunPreviewArtifactForAssistantSequence(artifacts []model.AgentRunArtifact, assistantSequenceNo int, panelKey string) (*worker.PublishedPreview, int, error) {
	if assistantSequenceNo <= 0 {
		return nil, 0, nil
	}
	targetKey := normalizeApprovalPreviewPanelKey(panelKey)
	matchCount := 0
	var firstMatch *worker.PublishedPreview
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
			return nil, 0, fmt.Errorf("parse run preview artifact: %w", err)
		}
		if targetKey != "" && normalizeApprovalPreviewPanelKey(payload.PanelKey) != targetKey {
			continue
		}
		matchCount++
		if targetKey != "" {
			return &payload, matchCount, nil
		}
		if firstMatch == nil {
			previewCopy := payload
			firstMatch = &previewCopy
		}
	}
	if targetKey == "" && matchCount == 1 {
		return firstMatch, matchCount, nil
	}
	return nil, matchCount, nil
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
		Content:     req.Content,
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
		TaskID:         run.TaskID,
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
	parentRunID    *string
	taskID         *string
	conversationID *string
	actorID        *string
	input          []byte
	trigger        *model.AgentRunTriggerContext
	delivery       *model.TaskDeliveryTarget
	repositoryID   *string
	repoFullName   *string
	baseBranch     *string
	workingBranch  *string
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
			s.recordTriggerExecution(ctx, params.workspaceID, params.agent.ID, params.trigger, params.targetType, params.targetID, activeRun, nil)
			return activeRun, nil
		}
		err := fmt.Errorf("an agent run is already active for this %s", params.targetType)
		s.recordTriggerExecution(ctx, params.workspaceID, params.agent.ID, params.trigger, params.targetType, params.targetID, nil, err)
		return nil, err
	}

	resolved := worker.ResolveAgentProfile(params.agent, params.invocationMode)
	approvalState := worker.ResolveApprovalState(resolved)
	taskQueue := resolved.Queue

	run := &model.AgentRun{
		WorkspaceID:       params.workspaceID,
		AgentID:           params.agent.ID,
		TaskID:            params.taskID,
		ConversationID:    params.conversationID,
		TargetType:        params.targetType,
		TargetID:          params.targetID,
		ParentRunID:       params.parentRunID,
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
	if params.repositoryID != nil && strings.TrimSpace(*params.repositoryID) != "" {
		run.RepositoryID = params.repositoryID
	}
	if params.repoFullName != nil && strings.TrimSpace(*params.repoFullName) != "" {
		run.RepoFullName = params.repoFullName
	}
	if params.baseBranch != nil && strings.TrimSpace(*params.baseBranch) != "" {
		run.BaseBranch = params.baseBranch
	}
	if params.workingBranch != nil && strings.TrimSpace(*params.workingBranch) != "" {
		run.WorkingBranch = params.workingBranch
	}
	if err := s.runRepo.Create(ctx, run); err != nil {
		s.recordTriggerExecution(ctx, params.workspaceID, params.agent.ID, params.trigger, params.targetType, params.targetID, nil, err)
		return nil, err
	}
	s.recordTriggerExecution(ctx, params.workspaceID, params.agent.ID, params.trigger, params.targetType, params.targetID, run, nil)

	params.agent.Status = "working"
	if params.taskID != nil {
		params.agent.ActiveTaskID = params.taskID
	} else {
		params.agent.ActiveTaskID = nil
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
		return nil, ErrAssignedAgentNotFound
	}
	normalizeAgentRecord(agent)
	if err := s.validateAndMaterializeAgentSkills(ctx, agent); err != nil {
		return nil, err
	}
	if err := validateAgentTarget(agent, targetType); err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *AgentService) ValidateTemplateAgent(ctx context.Context, agent *model.Agent) error {
	if agent == nil {
		return fmt.Errorf("agent is required")
	}
	normalizeAgentRecord(agent)
	if err := s.validateAndMaterializeAgentSkills(ctx, agent); err != nil {
		return err
	}
	if err := validateRuntimeForAgent(agent); err != nil {
		return err
	}
	if err := validateTriggerModeForAgent(agent.TriggerMode, agent); err != nil {
		return err
	}
	return nil
}

func (s *AgentService) markAgentIdle(ctx context.Context, workspaceID, agentID string) error {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return err
	}
	agent.Status = "idle"
	agent.ActiveTaskID = nil
	return s.agentRepo.Update(ctx, agent)
}

func (s *AgentService) markAgentWorking(ctx context.Context, workspaceID, agentID string, taskID *string) error {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return err
	}
	agent.Status = "working"
	agent.ActiveTaskID = taskID
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
		"agent_id":     run.AgentID,
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
	s.publishCodingSessionUpdated(run, actorID)
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
	s.publishCodingSessionMessageEvent(run, message, actorID)
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

// enrichRunTargets populates AgentRun.TargetInfo with a human-readable title
// and (for pm_task) a task_key so UI can surface real identifiers instead of
// raw UUIDs. Unknown target types leave TargetInfo empty; the frontend keeps
// its existing fallback logic.
func (s *AgentService) enrichRunTargets(ctx context.Context, workspaceID string, runs []model.AgentRun) {
	if len(runs) == 0 || workspaceID == "" {
		return
	}

	taskIDs := make([]string, 0, len(runs))
	seen := make(map[string]struct{}, len(runs))
	for _, run := range runs {
		if run.TargetType != "pm_task" || run.TargetID == "" {
			continue
		}
		if _, ok := seen[run.TargetID]; ok {
			continue
		}
		seen[run.TargetID] = struct{}{}
		taskIDs = append(taskIDs, run.TargetID)
	}

	if len(taskIDs) == 0 || s.taskRepo == nil {
		return
	}

	tasks, err := s.taskRepo.ListByIDs(ctx, workspaceID, taskIDs)
	if err != nil {
		slog.WarnContext(ctx, "enrich run targets: list tasks failed", "error", err, "workspace_id", workspaceID)
		return
	}

	byID := make(map[string]model.PMTask, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}

	var workspaceKey string
	if s.taskService != nil && len(byID) > 0 {
		workspaceKey = s.taskService.GetWorkspaceKey(ctx, workspaceID)
	}

	for idx := range runs {
		run := &runs[idx]
		if run.TargetType != "pm_task" {
			continue
		}
		task, ok := byID[run.TargetID]
		if !ok {
			continue
		}
		info := &model.AgentRunTarget{
			TargetType: run.TargetType,
			TargetID:   run.TargetID,
			Title:      task.Name,
		}
		if workspaceKey != "" {
			info.TaskKey = model.FormatTaskKey(workspaceKey, task.DisplayID)
		}
		run.TargetInfo = info
	}
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

func resolveCreateAgentTeamIDs(req model.CreateAgentRequest) []string {
	if len(req.TeamIDs) > 0 {
		return normalizeServiceTeamIDs(req.TeamIDs)
	}
	return resolveLegacyAgentTeamIDs(req.TeamID)
}

func resolveLegacyAgentTeamIDs(teamID *string) []string {
	trimmed := strings.TrimSpace(derefString(teamID))
	if trimmed == "" {
		return nil
	}
	return []string{trimmed}
}

func normalizeServiceTeamIDs(teamIDs []string) []string {
	seen := make(map[string]struct{}, len(teamIDs))
	normalized := make([]string, 0, len(teamIDs))
	for _, teamID := range teamIDs {
		teamID = strings.TrimSpace(teamID)
		if teamID == "" {
			continue
		}
		if _, ok := seen[teamID]; ok {
			continue
		}
		seen[teamID] = struct{}{}
		normalized = append(normalized, teamID)
	}
	return normalized
}

func firstTeamIDPtr(teamIDs []string) *string {
	teamIDs = normalizeServiceTeamIDs(teamIDs)
	if len(teamIDs) == 0 {
		return nil
	}
	return strPtr(teamIDs[0])
}

func agentTeamIDsForScope(agent *model.Agent) []string {
	if agent == nil {
		return nil
	}
	teamIDs := normalizeServiceTeamIDs(agent.TeamIDs)
	if len(teamIDs) > 0 {
		return teamIDs
	}
	return resolveLegacyAgentTeamIDs(agent.TeamID)
}

func agentVisibleToActorTeams(agent model.Agent, actorTeamIDs map[string]struct{}) bool {
	teamIDs := agentTeamIDsForScope(&agent)
	if len(teamIDs) == 0 {
		return true
	}
	for _, teamID := range teamIDs {
		if _, ok := actorTeamIDs[teamID]; ok {
			return true
		}
	}
	return false
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
	config, err := parseAndValidateExecutionConfig(agent)
	if err != nil {
		return err
	}
	agent.ExecutionConfig = model.MarshalAgentExecutionConfig(config)
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
	} else if !s.isModelProviderConfigured(provider) {
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
