package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
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

// AgentService contains agent business logic.
type AgentService struct {
	agentRepo    *repository.AgentRepository
	runRepo      *repository.AgentRunRepository
	artifactRepo *repository.AgentRunArtifactRepository
	jobRepo      *repository.AgentJobRepository
	storyRepo    *repository.PMStoryRepository
	ticketRepo   *repository.SupportTicketRepository
	messageRepo  *repository.SupportMessageRepository
	handoffRepo  *repository.AgentHandoffRepository
	activitySvc  *PMActivityService
	wsPublisher  *websocket.Publisher
}

// NewAgentService creates a new AgentService.
func NewAgentService(
	agentRepo *repository.AgentRepository,
	runRepo *repository.AgentRunRepository,
	artifactRepo *repository.AgentRunArtifactRepository,
	jobRepo *repository.AgentJobRepository,
	storyRepo *repository.PMStoryRepository,
	ticketRepo *repository.SupportTicketRepository,
	messageRepo *repository.SupportMessageRepository,
	handoffRepo *repository.AgentHandoffRepository,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
) *AgentService {
	return &AgentService{
		agentRepo:    agentRepo,
		runRepo:      runRepo,
		artifactRepo: artifactRepo,
		jobRepo:      jobRepo,
		storyRepo:    storyRepo,
		ticketRepo:   ticketRepo,
		messageRepo:  messageRepo,
		handoffRepo:  handoffRepo,
		activitySvc:  activitySvc,
		wsPublisher:  wsPublisher,
	}
}

// ListAgents returns all agents in a workspace.
func (s *AgentService) ListAgents(ctx context.Context, workspaceID string) ([]model.Agent, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.agentRepo.List(ctx, workspaceID)
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
	return agent, nil
}

// ListRuntimeProfiles returns the available runtime profiles.
func (s *AgentService) ListRuntimeProfiles() []model.RuntimeProfile {
	return worker.ListRuntimeProfiles()
}

// CreateAgent creates a new agent.
func (s *AgentService) CreateAgent(ctx context.Context, req model.CreateAgentRequest, actorID string) (*model.Agent, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if req.AgentKind != "human" && req.AgentKind != "llm" {
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

	runtimeKind := stringOrDefault(req.RuntimeKind, "native_claude")
	capabilityProfile := stringOrDefault(req.CapabilityProfile, defaultCapabilityProfileForRole(req.Role))
	triggerMode := stringOrDefault(req.TriggerMode, "manual")
	if err := validateRuntimeKind(runtimeKind); err != nil {
		return nil, err
	}
	if err := validateCapabilityProfile(capabilityProfile); err != nil {
		return nil, err
	}
	if err := validateTriggerMode(triggerMode); err != nil {
		return nil, err
	}

	agent := &model.Agent{
		WorkspaceID:        req.WorkspaceID,
		Name:               strings.TrimSpace(req.Name),
		AgentKind:          req.AgentKind,
		Role:               strings.TrimSpace(req.Role),
		Status:             "idle",
		BackingUserID:      req.BackingUserID,
		RuntimeKind:        runtimeKind,
		CapabilityProfile:  capabilityProfile,
		Skills:             skills,
		TriggerMode:        triggerMode,
		Model:              req.Model,
		SystemPrompt:       req.SystemPrompt,
		Tools:              tools,
		MonthlyTokenBudget: req.MonthlyTokenBudget,
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

	if req.Name != nil {
		agent.Name = strings.TrimSpace(*req.Name)
	}
	if req.Role != nil {
		agent.Role = strings.TrimSpace(*req.Role)
		if req.CapabilityProfile == nil || *req.CapabilityProfile == "" {
			agent.CapabilityProfile = defaultCapabilityProfileForRole(agent.Role)
		}
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
	}
	if req.CapabilityProfile != nil && strings.TrimSpace(*req.CapabilityProfile) != "" {
		if err := validateCapabilityProfile(*req.CapabilityProfile); err != nil {
			return nil, err
		}
		agent.CapabilityProfile = *req.CapabilityProfile
	}
	if req.Skills != nil {
		agent.Skills = req.Skills
	}
	if req.TriggerMode != nil && strings.TrimSpace(*req.TriggerMode) != "" {
		if err := validateTriggerMode(*req.TriggerMode); err != nil {
			return nil, err
		}
		agent.TriggerMode = *req.TriggerMode
	}
	if req.Model != nil {
		agent.Model = req.Model
	}
	if req.SystemPrompt != nil {
		agent.SystemPrompt = req.SystemPrompt
	}
	if req.Tools != nil {
		agent.Tools = req.Tools
	}
	if req.MonthlyTokenBudget != nil {
		agent.MonthlyTokenBudget = req.MonthlyTokenBudget
	}
	if req.ActiveStoryID != nil {
		agent.ActiveStoryID = req.ActiveStoryID
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
	return s.runRepo.ListByAgent(ctx, workspaceID, agentID, pagination)
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
	return run, nil
}

// ListRunArtifacts returns artifacts for a run.
func (s *AgentService) ListRunArtifacts(ctx context.Context, workspaceID, runID string) ([]model.AgentRunArtifact, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.artifactRepo.ListByRun(ctx, workspaceID, runID)
}

// RunAgent creates a new story-targeted agent run and enqueues it.
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

	agent, err := s.requireRunnableAgent(ctx, workspaceID, *story.AssignedAgentID)
	if err != nil {
		return nil, err
	}

	input, _ := json.Marshal(map[string]any{
		"story_id": storyID,
	})

	run, err := s.createRun(ctx, createRunParams{
		workspaceID: workspaceID,
		agent:       agent,
		targetType:  "story",
		targetID:    storyID,
		storyID:     &storyID,
		actorID:     actorID,
		input:       input,
	})
	if err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "story", storyID, &actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), nil)
	s.publishRunEvent(run, actorID)

	return run, nil
}

// RunTicketAgent creates a new ticket-targeted agent run and enqueues it.
func (s *AgentService) RunTicketAgent(ctx context.Context, workspaceID, ticketID, actorID string) (*model.AgentRun, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, fmt.Errorf("get ticket: %w", err)
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket not found")
	}
	if ticket.AssignedAgentID == nil || *ticket.AssignedAgentID == "" {
		return nil, fmt.Errorf("no agent assigned to this ticket")
	}

	agent, err := s.requireRunnableAgent(ctx, workspaceID, *ticket.AssignedAgentID)
	if err != nil {
		return nil, err
	}

	input, _ := json.Marshal(map[string]any{
		"ticket_id": ticketID,
		"source":    ticket.Source,
	})

	run, err := s.createRun(ctx, createRunParams{
		workspaceID: workspaceID,
		agent:       agent,
		targetType:  "support_ticket",
		targetID:    ticketID,
		ticketID:    &ticketID,
		actorID:     actorID,
		input:       input,
	})
	if err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "support_ticket", ticketID, &actorID, "updated", strPtr("agent_run"), nil, strPtr("started"), nil)
	s.publishRunEvent(run, actorID)

	return run, nil
}

// CancelRun cancels a queued or running agent run.
func (s *AgentService) CancelRun(ctx context.Context, workspaceID, runID, actorID string) (*model.AgentRun, error) {
	run, err := s.GetAgentRun(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	if run.Status != "queued" && run.Status != "running" {
		return nil, fmt.Errorf("only queued or running runs can be cancelled")
	}

	now := time.Now()
	run.Status = "cancelled"
	run.CompletedAt = &now
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
	if s.jobRepo != nil {
		_ = s.jobRepo.CompleteByRunID(ctx, run.ID)
	}

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

	if run.TargetType == "support_ticket" && run.TicketID != nil {
		var summary supportRunSummary
		_ = json.Unmarshal(run.OutputSummary, &summary)
		if summary.DraftReply == nil || strings.TrimSpace(summary.DraftReply.Content) == "" {
			return nil, fmt.Errorf("run has no support draft to approve")
		}

		if req.SendMessage {
			ticket, err := s.ticketRepo.GetByID(ctx, workspaceID, *run.TicketID)
			if err != nil || ticket == nil {
				return nil, fmt.Errorf("ticket not found")
			}

			msg := &model.SupportMessage{
				WorkspaceID:       workspaceID,
				TicketID:          *run.TicketID,
				SenderType:        "agent",
				SenderAgentID:     &run.AgentID,
				SenderDisplayName: summary.DraftReply.SenderDisplayName,
				Content:           summary.DraftReply.Content,
				IsInternal:        summary.DraftReply.IsInternal,
			}
			if err := s.messageRepo.Create(ctx, msg); err != nil {
				return nil, fmt.Errorf("create approved support reply: %w", err)
			}

			s.wsPublisher.Publish(websocket.Event{
				Action:      "created",
				Entity:      "support_message",
				EntityID:    msg.ID,
				WorkspaceID: workspaceID,
				ParentType:  "support_ticket",
				ParentID:    *run.TicketID,
				ActorID:     actorID,
			})
		}
	}

	run.ApprovalState = "approved"
	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, err
	}
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
		StoryID:     run.StoryID,
		TicketID:    run.TicketID,
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
	targetType  string
	targetID    string
	storyID     *string
	ticketID    *string
	actorID     string
	input       []byte
}

func (s *AgentService) createRun(ctx context.Context, params createRunParams) (*model.AgentRun, error) {
	activeRun, err := s.runRepo.FindActiveByTarget(ctx, params.workspaceID, params.targetType, params.targetID)
	if err != nil {
		return nil, err
	}
	if activeRun != nil {
		return nil, fmt.Errorf("an agent run is already active for this %s", params.targetType)
	}

	profile := worker.GetRuntimeProfile(params.agent.CapabilityProfile)
	approvalState := "not_required"
	if profile.ApprovalRequired || params.targetType == "support_ticket" {
		approvalState = "pending"
	}

	run := &model.AgentRun{
		WorkspaceID:       params.workspaceID,
		AgentID:           params.agent.ID,
		StoryID:           params.storyID,
		TicketID:          params.ticketID,
		TargetType:        params.targetType,
		TargetID:          params.targetID,
		RuntimeKind:       params.agent.RuntimeKind,
		ApprovalState:     approvalState,
		TriggeredByUserID: &params.actorID,
		Status:            "queued",
		Input:             json.RawMessage(params.input),
		OutputSummary:     json.RawMessage("{}"),
	}
	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, err
	}

	if s.jobRepo != nil {
		job := &model.AgentJob{
			WorkspaceID: params.workspaceID,
			RunID:       run.ID,
			Status:      "pending",
			MaxAttempts: 3,
		}
		if err := s.jobRepo.Create(ctx, job); err != nil {
			return nil, fmt.Errorf("create job: %w", err)
		}
	}

	params.agent.Status = "working"
	if params.storyID != nil {
		params.agent.ActiveStoryID = params.storyID
	} else {
		params.agent.ActiveStoryID = nil
	}
	_ = s.agentRepo.Update(ctx, params.agent)

	return run, nil
}

func (s *AgentService) requireRunnableAgent(ctx context.Context, workspaceID, agentID string) (*model.Agent, error) {
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("assigned agent not found")
	}
	if agent.AgentKind != "llm" {
		return nil, fmt.Errorf("only LLM agents can be run")
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
	s.wsPublisher.Publish(websocket.Event{
		Action:      action,
		Entity:      entity,
		EntityID:    entityID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})
}

func (s *AgentService) publishRunEvent(run *model.AgentRun, actorID string) {
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
	case "support":
		return "support"
	case "reviewer", "reviewer_tester", "tester":
		return "reviewer_tester"
	case "orchestrator":
		return "orchestrator"
	case "human_proxy", "human":
		return "human_proxy"
	default:
		return "engineer"
	}
}

func stringOrDefault(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}

func validateRuntimeKind(runtimeKind string) error {
	switch runtimeKind {
	case "native_claude", "claude_code", "openclaw", "zeroclaw":
		return nil
	default:
		return fmt.Errorf("runtime_kind must be one of native_claude, claude_code, openclaw, zeroclaw")
	}
}

func validateCapabilityProfile(profileName string) error {
	for _, profile := range worker.ListRuntimeProfiles() {
		if profile.Name == profileName {
			return nil
		}
	}
	return fmt.Errorf("unknown capability_profile %q", profileName)
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
