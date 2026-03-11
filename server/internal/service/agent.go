package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// AgentService contains agent business logic.
type AgentService struct {
	agentRepo        *repository.AgentRepository
	runRepo          *repository.AgentRunRepository
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
	anthropicAPIKey  string
	openAIAPIKey     string
	openRouterAPIKey string
}

// NewAgentService creates a new AgentService.
func NewAgentService(
	agentRepo *repository.AgentRepository,
	runRepo *repository.AgentRunRepository,
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

// ListRuntimeProfiles returns the available runtime profiles.
func (s *AgentService) ListRuntimeProfiles() []model.RuntimeProfile {
	return worker.ListRuntimeProfiles()
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
	if strings.TrimSpace(req.AgentKind) != "" && req.AgentKind != "human" && req.AgentKind != "llm" {
		return nil, fmt.Errorf("agent_kind must be 'human' or 'llm'")
	}

	tools := req.Tools
	if tools == nil {
		tools = json.RawMessage("[]")
	}

	skills := req.Skills
	if skills == nil {
		skills = json.RawMessage("[]")
	}

	agentClass := normalizeAgentClass(stringOrDefault(req.AgentClass, ""), stringOrDefault(req.CapabilityProfile, ""), req.Role, req.AgentKind)
	agentKind := strings.TrimSpace(req.AgentKind)
	if agentKind == "" {
		agentKind = agentKindForAgentClass(agentClass)
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = defaultRoleForAgentClass(agentClass)
	}
	runtimeKind := strings.TrimSpace(stringOrDefault(req.RuntimeKind, defaultRuntimeKindForAgentClass(agentClass)))
	triggerMode := stringOrDefault(req.TriggerMode, defaultTriggerModeForAgentClass(agentClass))
	if err := validateRuntimeKind(runtimeKind); err != nil {
		return nil, err
	}
	if err := validateAgentClass(agentClass); err != nil {
		return nil, err
	}
	if err := validateTriggerModeForAgentClass(triggerMode, agentClass); err != nil {
		return nil, err
	}
	if agentKind == "human" && agentClass != model.AgentClassHuman {
		return nil, fmt.Errorf("human agents must use agent_class human")
	}
	if agentKind == "llm" && agentClass == model.AgentClassHuman {
		return nil, fmt.Errorf("agent_class human requires agent_kind human")
	}

	agent := &model.Agent{
		WorkspaceID:        req.WorkspaceID,
		Name:               strings.TrimSpace(req.Name),
		AgentKind:          agentKind,
		AgentClass:         agentClass,
		Role:               role,
		Status:             "idle",
		BackingUserID:      trimPtr(req.BackingUserID),
		RuntimeKind:        runtimeKind,
		CapabilityProfile:  capabilityProfileForAgentClass(agentClass),
		Skills:             skills,
		TriggerMode:        triggerMode,
		Provider:           trimPtr(req.Provider),
		Model:              trimPtr(req.Model),
		SystemPrompt:       trimPtr(req.SystemPrompt),
		PlanningNotes:      trimPtr(req.PlanningNotes),
		Tools:              tools,
		MonthlyTokenBudget: normalizeTokenBudget(req.MonthlyTokenBudget),
	}
	normalizeAgentRecord(agent)
	if err := s.validateModelRouting(agent); err != nil {
		return nil, err
	}

	if err := s.agentRepo.Create(ctx, agent); err != nil {
		return nil, err
	}

	newValue := agent.Name
	_ = s.activitySvc.Log(ctx, agent.WorkspaceID, "agent", agent.ID, &actorID, "created", nil, nil, &newValue, nil)

	s.publishSimpleEvent("created", "agent", agent.ID, agent.WorkspaceID, actorID)

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

	classChanged := false
	if req.Name != nil {
		agent.Name = strings.TrimSpace(*req.Name)
	}
	if req.AgentClass != nil {
		agent.AgentClass = strings.TrimSpace(*req.AgentClass)
		classChanged = true
	}
	if req.Role != nil {
		agent.Role = strings.TrimSpace(*req.Role)
	}
	if req.Status != nil {
		agent.Status = *req.Status
	}
	if req.BackingUserID != nil {
		agent.BackingUserID = req.BackingUserID
	}
	if req.RuntimeKind != nil && strings.TrimSpace(*req.RuntimeKind) != "" {
		if err := validateRuntimeKind(*req.RuntimeKind); err != nil {
			return nil, err
		}
		agent.RuntimeKind = *req.RuntimeKind
	} else if classChanged {
		agent.RuntimeKind = defaultRuntimeKindForAgentClass(agent.AgentClass)
	}
	if req.CapabilityProfile != nil && strings.TrimSpace(*req.CapabilityProfile) != "" {
		profile := worker.NormalizeCapabilityProfile(*req.CapabilityProfile)
		if err := validateAgentClass(profile); err != nil {
			return nil, err
		}
		agent.AgentClass = profile
	}
	if req.Skills != nil {
		agent.Skills = req.Skills
	}
	if req.TriggerMode != nil && strings.TrimSpace(*req.TriggerMode) != "" {
		agent.TriggerMode = *req.TriggerMode
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
	if req.PlanningNotes != nil {
		agent.PlanningNotes = trimPtr(req.PlanningNotes)
	}
	if req.Tools != nil {
		agent.Tools = req.Tools
	}
	if req.MonthlyTokenBudget != nil {
		agent.MonthlyTokenBudget = normalizeTokenBudget(req.MonthlyTokenBudget)
	}
	if req.ActiveStoryID != nil {
		agent.ActiveStoryID = req.ActiveStoryID
	}
	if classChanged && (req.Role == nil || strings.TrimSpace(*req.Role) == "") {
		agent.Role = defaultRoleForAgentClass(agent.AgentClass)
	}
	normalizeAgentRecord(agent)
	if err := validateAgentClass(agent.AgentClass); err != nil {
		return nil, err
	}
	if agent.AgentKind == "human" && agent.AgentClass != model.AgentClassHuman {
		return nil, fmt.Errorf("human agents must use agent_class human")
	}
	if agent.AgentKind != "human" && agent.AgentClass == model.AgentClassHuman {
		return nil, fmt.Errorf("agent_class human requires agent_kind human")
	}
	if err := validateTriggerModeForAgentClass(agent.TriggerMode, agent.AgentClass); err != nil {
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

	if agent.AgentKind == "llm" && agent.TriggerMode == "auto_on_assignment" {
		if _, err := s.RunAgent(ctx, workspaceID, storyID, actorID); err != nil {
			if errors.Is(err, ErrStoryDeliveryTargetRequired) {
				return nil
			}
			return err
		}
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
	story, err := s.storyRepo.GetRawByID(ctx, storyID)
	if err != nil {
		return nil, fmt.Errorf("get story: %w", err)
	}
	if story == nil {
		return nil, fmt.Errorf("story not found")
	}
	if story.AssignedAgentID == nil || *story.AssignedAgentID == "" {
		return nil, fmt.Errorf("no agent assigned to this story")
	}

	agent, err := s.requireRunnableAgent(ctx, workspaceID, *story.AssignedAgentID, "story")
	if err != nil {
		return nil, err
	}
	profile := worker.GetRuntimeProfile(agent.CapabilityProfile)
	deliveryTarget, err := s.gitService.ResolveStoryDeliveryTargetForRun(ctx, workspaceID, storyID, profile)
	if err != nil {
		return nil, err
	}

	input, _ := json.Marshal(map[string]any{
		"story_id": storyID,
	})

	run, err := s.createRun(ctx, createRunParams{
		workspaceID: workspaceID,
		agent:       agent,
		profile:     profile,
		targetType:  "story",
		targetID:    storyID,
		storyID:     &storyID,
		actorID:     actorID,
		input:       input,
		delivery:    deliveryTarget,
	})
	if err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "story", storyID, &actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), nil)
	s.publishRunEvent(run, actorID)

	return run, nil
}

// RunConversationAgent creates a new conversation-targeted agent run and starts its Temporal workflow.
func (s *AgentService) RunConversationAgent(ctx context.Context, workspaceID, conversationID, actorID string) (*model.AgentRun, error) {
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
	profile := worker.GetRuntimeProfile(agent.CapabilityProfile)

	input, _ := json.Marshal(map[string]any{
		"conversation_id": conversationID,
		"source":          conversation.Source,
	})

	run, err := s.createRun(ctx, createRunParams{
		workspaceID:    workspaceID,
		agent:          agent,
		profile:        profile,
		targetType:     "support_conversation",
		targetID:       conversationID,
		conversationID: &conversationID,
		actorID:        actorID,
		input:          input,
	})
	if err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversationID, &actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), nil)
	s.publishRunEvent(run, actorID)

	return run, nil
}

// CancelRun cancels a queued, running, or approval-pending agent run.
func (s *AgentService) CancelRun(ctx context.Context, workspaceID, runID, actorID string) (*model.AgentRun, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run.Status != "queued" && run.Status != "running" && run.Status != "awaiting_approval" {
		return nil, fmt.Errorf("only queued, running, or approval-pending runs can be cancelled")
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

// ApproveRun approves a pending outcome, publishing support drafts when requested.
func (s *AgentService) ApproveRun(ctx context.Context, workspaceID, runID, actorID string, req model.ApproveAgentRunRequest) (*model.AgentRun, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run.ApprovalState != "pending" {
		return nil, fmt.Errorf("run does not require approval")
	}

	if run.TargetType == "support_conversation" && run.ConversationID != nil {
		var summary supportRunSummary
		_ = json.Unmarshal(run.OutputSummary, &summary)
		if summary.DraftReply == nil || strings.TrimSpace(summary.DraftReply.Content) == "" {
			return nil, fmt.Errorf("run has no support draft to approve")
		}

		if req.SendMessage {
			conversation, err := s.conversationRepo.GetByID(ctx, workspaceID, *run.ConversationID)
			if err != nil || conversation == nil {
				return nil, fmt.Errorf("conversation not found")
			}

			msg := &model.SupportMessage{
				WorkspaceID:       workspaceID,
				ConversationID:    *run.ConversationID,
				SenderType:        "agent",
				SenderAgentID:     &run.AgentID,
				SenderDisplayName: summary.DraftReply.SenderDisplayName,
				Content:           summary.DraftReply.Content,
				IsInternal:        summary.DraftReply.IsInternal,
				MessageType:       "reply",
			}
			if err := s.messageRepo.Create(ctx, msg); err != nil {
				return nil, fmt.Errorf("create approved support reply: %w", err)
			}

			s.wsPublisher.Publish(websocket.Event{
				Action:      "created",
				Entity:      "support_message",
				EntityID:    msg.ID,
				WorkspaceID: workspaceID,
				ParentType:  "support_conversation",
				ParentID:    *run.ConversationID,
				ActorID:     actorID,
			})
		}
	}

	run.ApprovalState = "approved"
	if run.Status == "awaiting_approval" {
		now := time.Now()
		run.Status = "completed"
		run.CompletedAt = &now
		run.ExecutionStage = strPtr("approved")
	}
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	_ = s.runEngine.SignalApprove(ctx, derefString(run.WorkflowID), derefString(run.WorkflowRunID))
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
		WorkspaceID: run.WorkspaceID,
		FromAgentID: &run.AgentID,
		ToAgentID:   req.ToAgentID,
		ToUserID:    req.ToUserID,
		StoryID:        run.StoryID,
		ConversationID: run.ConversationID,
		RunID:       &run.ID,
		HandoffType: handoffType,
		Reason:      req.Reason,
		Context:     contextJSON,
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
	workspaceID string
	agent       *model.Agent
	profile     model.RuntimeProfile
	targetType  string
	targetID    string
	storyID        *string
	conversationID *string
	actorID     string
	input       []byte
	delivery    *model.StoryDeliveryTarget
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
	if activeRun != nil && (activeRun.Status == "queued" || activeRun.Status == "running" || activeRun.Status == "awaiting_approval") {
		return nil, fmt.Errorf("an agent run is already active for this %s", params.targetType)
	}

	approvalState := "not_required"
	if params.profile.ApprovalRequired || params.targetType == "support_conversation" {
		approvalState = "pending"
	}
	taskQueue := temporalapp.QueueForProfile(params.profile.Name)

	run := &model.AgentRun{
		WorkspaceID:       params.workspaceID,
		AgentID:           params.agent.ID,
		StoryID:           params.storyID,
		ConversationID:    params.conversationID,
		TargetType:        params.targetType,
		TargetID:          params.targetID,
		RuntimeKind:       params.agent.RuntimeKind,
		ApprovalState:     approvalState,
		TriggeredByUserID: &params.actorID,
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
	if agent.AgentKind != "llm" {
		return nil, fmt.Errorf("only LLM agents can be run")
	}
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
	event := websocket.Event{
		Action:      "updated",
		Entity:      "agent_run",
		EntityID:    run.ID,
		WorkspaceID: run.WorkspaceID,
		ActorID:     actorID,
		ParentType:  run.TargetType,
		ParentID:    run.TargetID,
	}
	s.wsPublisher.Publish(event)
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

func defaultCapabilityProfileForRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "planner", "prd", "product":
		return model.AgentClassProductPlanner
	case "support":
		return model.AgentClassSupport
	case "reviewer", "reviewer_tester", "tester":
		return model.AgentClassReviewer
	case "orchestrator":
		return model.AgentClassProductPlanner
	case "human_proxy", "human":
		return model.AgentClassHuman
	default:
		return model.AgentClassEngineer
	}
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
	case "opencode", "native_claude", "claude_code", "openclaw", "zeroclaw":
		return nil
	default:
		return fmt.Errorf("runtime_kind must be one of opencode, native_claude, claude_code, openclaw, zeroclaw")
	}
}

func validateCapabilityProfile(profileName string) error {
	return validateAgentClass(worker.NormalizeCapabilityProfile(profileName))
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
	if agent == nil || agent.AgentKind == "human" {
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
