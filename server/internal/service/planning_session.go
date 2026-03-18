package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

// PlanningWorkflowStarter abstracts the Temporal RunEngine to avoid import cycles.
type PlanningWorkflowStarter interface {
	StartPlanningSession(ctx context.Context, sessionID string) error
	SignalPlanningMessage(ctx context.Context, sessionID string) error
	SignalPlanningFinalize(ctx context.Context, sessionID, actorID string) error
	SignalPlanningAbandon(ctx context.Context, sessionID string) error
	SignalFlowChildState(ctx context.Context, flowRunID, nodeRunID, childType, childID, childStatus string) error
}

// PlanningSessionService manages interactive planning sessions.
type PlanningSessionService struct {
	sessionRepo      *repository.PlanningSessionRepository
	epicRepo         *repository.PMEpicRepository
	agentRepo        *repository.AgentRepository
	settingsRepo     *repository.SettingsRepository
	docsContentRepo  *repository.DocsContentRepository
	docsVersionRepo  *repository.DocsVersionRepository
	docsLinkRepo     *repository.DocsLinkRepository
	docsDocumentRepo *repository.DocsDocumentRepository
	docsSpaceRepo    *repository.DocsSpaceRepository
	modelFactory     *worker.EinoModelFactory
	toolRegistry     *worker.ToolRegistry
	streamer         websocket.SessionStreamer // sends stream events (tokens, tool results) to WS clients
	publisher        websocket.EventPublisher  // broadcasts entity events for query invalidation
	workflow         PlanningWorkflowStarter
}

// NewPlanningSessionService creates a new service.
func NewPlanningSessionService(
	sessionRepo *repository.PlanningSessionRepository,
	epicRepo *repository.PMEpicRepository,
	agentRepo *repository.AgentRepository,
	settingsRepo *repository.SettingsRepository,
	docsContentRepo *repository.DocsContentRepository,
	docsVersionRepo *repository.DocsVersionRepository,
	docsLinkRepo *repository.DocsLinkRepository,
	docsDocumentRepo *repository.DocsDocumentRepository,
	docsSpaceRepo *repository.DocsSpaceRepository,
	modelFactory *worker.EinoModelFactory,
	toolRegistry *worker.ToolRegistry,
	streamer websocket.SessionStreamer,
	publisher websocket.EventPublisher,
) *PlanningSessionService {
	return &PlanningSessionService{
		sessionRepo:      sessionRepo,
		epicRepo:         epicRepo,
		agentRepo:        agentRepo,
		settingsRepo:     settingsRepo,
		docsContentRepo:  docsContentRepo,
		docsVersionRepo:  docsVersionRepo,
		docsLinkRepo:     docsLinkRepo,
		docsDocumentRepo: docsDocumentRepo,
		docsSpaceRepo:    docsSpaceRepo,
		modelFactory:     modelFactory,
		toolRegistry:     toolRegistry,
		streamer:         streamer,
		publisher:        publisher,
	}
}

// SetWorkflowStarter sets the Temporal workflow starter (called after DI wiring to break cycles).
func (s *PlanningSessionService) SetWorkflowStarter(wf PlanningWorkflowStarter) {
	s.workflow = wf
}

// StartSession creates a new interactive planning session for an epic.
func (s *PlanningSessionService) StartSession(ctx context.Context, workspaceID, epicID, actorID string, req model.StartPlanningSessionRequest) (*model.PlanningSession, error) {
	// Validate epic.
	epicWithStats, err := s.epicRepo.GetByID(ctx, epicID)
	if err != nil {
		return nil, fmt.Errorf("get epic: %w", err)
	}
	if epicWithStats == nil {
		return nil, fmt.Errorf("epic not found")
	}
	epic := &epicWithStats.Epic
	if epic.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("epic not found")
	}

	// Check no active session exists.
	existing, err := s.sessionRepo.GetActiveByEpicID(ctx, epicID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if req.FlowNodeRunID != nil && existing.FlowNodeRunID != nil && *req.FlowNodeRunID == *existing.FlowNodeRunID {
			return existing, nil
		}
		if req.FlowRunID != nil && existing.FlowRunID != nil && *req.FlowRunID == *existing.FlowRunID {
			return existing, nil
		}
		return nil, fmt.Errorf("an active planning session already exists for this epic")
	}

	// Validate agent.
	agentID := req.AgentID
	if agentID == "" && epic.OrchestratorAgentID != nil {
		agentID = *epic.OrchestratorAgentID
	}
	if agentID == "" {
		return nil, fmt.Errorf("no product planner assigned to this epic")
	}
	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	normalizeAgentRecord(agent)
	if err := validateAgentTarget(agent, "epic"); err != nil {
		return nil, err
	}
	if !agentSupportsMode(agent, model.InvocationModeInteractive) {
		return nil, fmt.Errorf("selected planner does not support interactive mode")
	}

	// Resolve methodology.
	settings, err := s.settingsRepo.GetWorkspaceSettings(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	methodology := "structured_v1"
	if settings != nil && settings.PlanningMethodology != "" {
		methodology = settings.PlanningMethodology
	}

	// Build context snapshot.
	contextSnapshot := s.buildContextSnapshot(ctx, epic, req.AdditionalContext)
	contextJSON, _ := json.Marshal(contextSnapshot)

	// Create session.
	session := &model.PlanningSession{
		WorkspaceID:         workspaceID,
		EpicID:              epicID,
		AgentID:             agentID,
		FlowRunID:           req.FlowRunID,
		FlowNodeRunID:       req.FlowNodeRunID,
		Status:              model.PlanningSessionStatusActive,
		PlanningMethodology: methodology,
		AllowedTools:        normalizeJSONSlice(req.AllowedTools),
		SpecDocumentID:      epic.SpecDocumentID,
		ContextSnapshot:     contextJSON,
		StartedBy:           &actorID,
		StartedAt:           time.Now(),
		LastActiveAt:        time.Now(),
	}
	session, err = s.sessionRepo.Create(ctx, session)
	if err != nil {
		return nil, err
	}

	// Update epic state.
	epic.PlanningState = model.EpicPlanningStateInSession
	epic.ActivePlanningSessionID = &session.ID
	if err := s.epicRepo.Update(ctx, epic); err != nil {
		return nil, fmt.Errorf("update epic: %w", err)
	}

	// Start the Temporal workflow (clones repo, runs initial agent turn).
	if s.workflow == nil {
		return nil, fmt.Errorf("interactive planning sessions require Temporal to be configured")
	}
	if err := s.workflow.StartPlanningSession(ctx, session.ID); err != nil {
		return nil, fmt.Errorf("start planning workflow: %w", err)
	}

	// Publish events.
	s.publisher.Publish(websocket.Event{
		Action: "updated", Entity: "epic", EntityID: epicID,
		WorkspaceID: workspaceID, ActorID: actorID,
	})

	return session, nil
}

// SendMessage saves a human message and triggers the agent's streaming response.
func (s *PlanningSessionService) SendMessage(ctx context.Context, workspaceID, sessionID, actorID string, req model.SendPlanningMessageRequest) (*model.PlanningSessionMessage, error) {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil || session.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("planning session not found")
	}
	if session.Status != model.PlanningSessionStatusActive && session.Status != model.PlanningSessionStatusPaused {
		return nil, fmt.Errorf("planning session is not active")
	}

	// Resume if paused.
	if session.Status == model.PlanningSessionStatusPaused {
		session.Status = model.PlanningSessionStatusActive
		if err := s.sessionRepo.Update(ctx, session); err != nil {
			return nil, err
		}
	}

	// Determine message type.
	messageType := model.PlanningMessageTypeMessage
	if req.MessageType != nil && *req.MessageType != "" {
		messageType = *req.MessageType
	}

	// Save user message.
	userMsg := &model.PlanningSessionMessage{
		SessionID:   sessionID,
		Role:        "user",
		Content:     req.Content,
		MessageType: messageType,
	}
	userMsg, err = s.sessionRepo.CreateMessage(ctx, userMsg)
	if err != nil {
		return nil, err
	}

	// Update last active.
	_ = s.sessionRepo.UpdateLastActive(ctx, sessionID)

	// Publish user message event.
	userEventData, _ := json.Marshal(map[string]string{"role": "user"})
	s.publisher.Publish(websocket.Event{
		Action: "created", Entity: "planning_session_message",
		EntityID: userMsg.ID, ParentType: "planning_session", ParentID: sessionID,
		WorkspaceID: workspaceID, ActorID: actorID, Data: userEventData,
	})

	// Signal Temporal workflow to run agent turn.
	if s.workflow == nil {
		return nil, fmt.Errorf("planning session workflow is not available")
	}
	if err := s.workflow.SignalPlanningMessage(ctx, sessionID); err != nil {
		return nil, fmt.Errorf("signal planning message: %w", err)
	}

	return userMsg, nil
}

// FinalizeSession copies the existing spec_draft to docs and completes the session immediately.
func (s *PlanningSessionService) FinalizeSession(ctx context.Context, workspaceID, sessionID, actorID string) (*model.PlanningSession, error) {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil || session.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("planning session not found")
	}
	if session.Status != model.PlanningSessionStatusActive && session.Status != model.PlanningSessionStatusPaused {
		return nil, fmt.Errorf("planning session is not active")
	}

	// Validate that a spec draft exists — nothing to finalize without one.
	if strings.TrimSpace(session.SpecDraft) == "" {
		return nil, fmt.Errorf("no spec draft to finalize — ask the agent to produce a draft first")
	}

	// Ensure a spec doc exists on the epic (create if needed, like DraftEpicSpec did).
	epicWithStats, err := s.epicRepo.GetByID(ctx, session.EpicID)
	if err != nil || epicWithStats == nil {
		return nil, fmt.Errorf("epic not found: %s", session.EpicID)
	}
	epic := &epicWithStats.Epic

	specDoc, err := s.ensureEpicSpecDocument(ctx, workspaceID, epic, actorID)
	if err != nil {
		return nil, fmt.Errorf("ensure spec document: %w", err)
	}
	session.SpecDocumentID = &specDoc.ID

	// Write spec_draft to the doc.
	s.writeSpecToDoc(ctx, specDoc.ID, session.SpecDraft, actorID)

	// Complete the session.
	now := time.Now()
	session.Status = model.PlanningSessionStatusCompleted
	session.CompletedAt = &now
	if err := s.sessionRepo.Update(ctx, session); err != nil {
		return nil, err
	}

	// Update epic state.
	epic.PlanningState = model.EpicPlanningStateAwaitingSpecApproval
	epic.ActivePlanningSessionID = nil
	_ = s.epicRepo.Update(ctx, epic)

	// Signal Temporal workflow to abandon (cleanup workspace — no agent turn needed).
	if s.workflow != nil {
		_ = s.workflow.SignalPlanningAbandon(ctx, sessionID)
		if session.FlowRunID != nil && session.FlowNodeRunID != nil {
			_ = s.workflow.SignalFlowChildState(ctx, *session.FlowRunID, *session.FlowNodeRunID, "planning_session", session.ID, model.PlanningSessionStatusCompleted)
		}
	}

	s.publisher.Publish(websocket.Event{
		Action: "updated", Entity: "planning_session", EntityID: sessionID,
		WorkspaceID: workspaceID, ActorID: actorID,
	})
	s.publisher.Publish(websocket.Event{
		Action: "updated", Entity: "epic", EntityID: session.EpicID,
		WorkspaceID: workspaceID, ActorID: actorID,
	})

	return session, nil
}

// AbandonSession cancels the session and resets epic state.
func (s *PlanningSessionService) AbandonSession(ctx context.Context, workspaceID, sessionID, actorID string) error {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil || session.WorkspaceID != workspaceID {
		return fmt.Errorf("planning session not found")
	}

	session.Status = model.PlanningSessionStatusAbandoned
	now := time.Now()
	session.CompletedAt = &now
	if err := s.sessionRepo.Update(ctx, session); err != nil {
		return err
	}

	// Reset epic.
	epicWithStats, _ := s.epicRepo.GetByID(ctx, session.EpicID)
	if epicWithStats != nil {
		epic := &epicWithStats.Epic
		epic.PlanningState = model.EpicPlanningStateNotStarted
		epic.ActivePlanningSessionID = nil
		_ = s.epicRepo.Update(ctx, epic)
	}

	// Signal Temporal workflow to stop (cleanup workspace).
	if s.workflow != nil {
		_ = s.workflow.SignalPlanningAbandon(ctx, sessionID)
		if session.FlowRunID != nil && session.FlowNodeRunID != nil {
			_ = s.workflow.SignalFlowChildState(ctx, *session.FlowRunID, *session.FlowNodeRunID, "planning_session", session.ID, model.PlanningSessionStatusAbandoned)
		}
	}

	s.publisher.Publish(websocket.Event{
		Action: "updated", Entity: "planning_session", EntityID: sessionID,
		WorkspaceID: workspaceID, ActorID: actorID,
	})
	s.publisher.Publish(websocket.Event{
		Action: "updated", Entity: "epic", EntityID: session.EpicID,
		WorkspaceID: workspaceID, ActorID: actorID,
	})
	return nil
}

// GetSession returns a planning session by ID.
func (s *PlanningSessionService) GetSession(ctx context.Context, workspaceID, sessionID string) (*model.PlanningSession, error) {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil || session.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("planning session not found")
	}
	return session, nil
}

// GetSessionMessages returns all messages for a planning session.
func (s *PlanningSessionService) GetSessionMessages(ctx context.Context, workspaceID, sessionID string) ([]model.PlanningSessionMessage, error) {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil || session.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("planning session not found")
	}
	return s.sessionRepo.ListMessages(ctx, sessionID)
}

// GetActiveSessionByEpicID returns the active planning session for an epic, if any.
func (s *PlanningSessionService) GetActiveSessionByEpicID(ctx context.Context, workspaceID, epicID string) (*model.PlanningSession, error) {
	session, err := s.sessionRepo.GetActiveByEpicID(ctx, epicID)
	if err != nil {
		return nil, err
	}
	if session != nil && session.WorkspaceID != workspaceID {
		return nil, nil
	}
	return session, nil
}

// --- Exported methods for Temporal activities ---

// RunAgentTurnWithContext executes a streaming agent turn with a sandboxed ExecutionContext.
// Called by PlanningSessionActivities.RunTurnActivity from the Temporal worker.
func (s *PlanningSessionService) RunAgentTurnWithContext(ctx context.Context, sessionID string, execCtx *worker.ExecutionContext) error {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil || session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	agent, _ := s.agentRepo.GetByID(ctx, session.WorkspaceID, session.AgentID)
	s.runAgentTurn(ctx, session, agent, execCtx)
	return nil
}

// RunFinalizationTurnWithContext is kept for backwards compatibility with in-flight workflows
// but is now a no-op — finalization is handled synchronously in FinalizeSession.
func (s *PlanningSessionService) RunFinalizationTurnWithContext(ctx context.Context, sessionID, actorID string, execCtx *worker.ExecutionContext) error {
	slog.Warn("RunFinalizationTurnWithContext called but finalization is now synchronous", "session_id", sessionID)
	return nil
}

// --- Internal: agent turn execution ---

// runAgentTurn executes a streaming Eino-backed agent turn and pushes events via WebSocket.
func (s *PlanningSessionService) runAgentTurn(ctx context.Context, session *model.PlanningSession, agent *model.Agent, execCtx *worker.ExecutionContext) {
	sessionID := session.ID

	if s.modelFactory == nil {
		slog.Error("planning session: Eino model factory not configured", "session_id", sessionID)
		s.sendStreamError(sessionID, "LLM provider not configured for agent")
		return
	}

	// Load conversation history.
	messages, err := s.sessionRepo.ListMessages(ctx, sessionID)
	if err != nil {
		slog.Error("planning session: failed to load messages", "error", err, "session_id", sessionID)
		s.sendStreamError(sessionID, "failed to load conversation history")
		return
	}

	systemPrompt := s.buildSystemPrompt(session, agent)
	history, err := s.buildExecutionMessages(messages, session)
	if err != nil {
		slog.Error("planning session: failed to build execution history", "error", err, "session_id", sessionID)
		s.sendStreamError(sessionID, "failed to prepare conversation history")
		return
	}
	tools := s.resolveTools(ctx, session)
	result, execErr := worker.ExecuteWithEino(ctx, s.modelFactory, agent, systemPrompt, history, tools, execCtx, s.toolRegistry, 25, func(event worker.ExecutionEvent) {
		s.streamExecutionEvent(sessionID, event)
	})
	if execErr != nil && !errors.Is(execErr, worker.ErrMaxToolStepsReached) {
		slog.Error("planning session: Eino execution failed", "error", execErr, "session_id", sessionID)
		s.sendStreamError(sessionID, "failed to get agent response")
		return
	}

	if errors.Is(execErr, worker.ErrMaxToolStepsReached) {
		s.sendStreamError(sessionID, "Agent reached the maximum number of tool steps and stopped before finishing.")
	}

	messageType := classifyMessage(result.AssistantText)
	contentBlocksJSON, _ := json.Marshal(result.AssistantBlocks)
	toolInvJSON, _ := json.Marshal(result.ToolInvocations)
	usageJSON, _ := json.Marshal(result.Usage)

	assistantMsg := &model.PlanningSessionMessage{
		SessionID:       sessionID,
		Role:            "assistant",
		Content:         result.AssistantText,
		MessageType:     messageType,
		ToolInvocations: toolInvJSON,
		ContentBlocks:   contentBlocksJSON,
		TokenUsage:      usageJSON,
	}
	assistantMsg, _ = s.sessionRepo.CreateMessage(ctx, assistantMsg)

	if draft := extractSpecDraft(result.AssistantText); draft != "" {
		s.persistSpecDraft(ctx, session, draft)
	}
	_ = s.sessionRepo.UpdateLastActive(ctx, sessionID)
	s.updateSessionTokenUsage(ctx, session, result.Usage)
	if assistantMsg != nil {
		assistantEventData, _ := json.Marshal(map[string]string{"role": "assistant"})
		s.publisher.Publish(websocket.Event{
			Action: "created", Entity: "planning_session_message",
			EntityID: assistantMsg.ID, ParentType: "planning_session", ParentID: sessionID,
			WorkspaceID: session.WorkspaceID, Data: assistantEventData,
		})
	}
	completedEvent := model.PlanningStreamEvent{
		Type:      "turn_completed",
		SessionID: sessionID,
	}
	if assistantMsg != nil {
		completedEvent.MessageID = assistantMsg.ID
	}
	s.streamer.SendToSession(sessionID, completedEvent)
}

// --- Helpers ---

func (s *PlanningSessionService) buildContextSnapshot(ctx context.Context, epic *model.PMEpic, additionalContext *string) map[string]interface{} {
	snapshot := map[string]interface{}{
		"epic_id":   epic.ID,
		"epic_name": epic.Name,
	}
	if epic.Description != nil {
		snapshot["epic_description"] = *epic.Description
	}
	if additionalContext != nil && *additionalContext != "" {
		snapshot["additional_context"] = *additionalContext
	}

	// Include existing spec content if available.
	if epic.SpecDocumentID != nil {
		content, err := s.docsContentRepo.GetByDocumentID(ctx, *epic.SpecDocumentID)
		if err == nil && content != nil && content.ContentText != "" {
			text := content.ContentText
			if len(text) > 20000 {
				text = text[:20000] + "\n... (truncated)"
			}
			snapshot["existing_spec"] = text
		}
	}

	// Include linked docs content.
	if s.docsLinkRepo != nil {
		links, err := s.docsLinkRepo.ListByObject(ctx, epic.WorkspaceID, "epic", epic.ID)
		if err == nil && len(links) > 0 {
			var linkedDocs []map[string]string
			for _, link := range links {
				// Skip the spec document itself — already included above.
				if epic.SpecDocumentID != nil && link.DocumentID == *epic.SpecDocumentID {
					continue
				}
				content, err := s.docsContentRepo.GetByDocumentID(ctx, link.DocumentID)
				if err != nil || content == nil || content.ContentText == "" {
					continue
				}
				text := content.ContentText
				if len(text) > 10000 {
					text = text[:10000] + "\n... (truncated)"
				}
				entry := map[string]string{
					"document_id": link.DocumentID,
					"context":     link.LinkContext,
					"content":     text,
				}
				if link.DocumentTitle != "" {
					entry["title"] = link.DocumentTitle
				}
				linkedDocs = append(linkedDocs, entry)
			}
			if len(linkedDocs) > 0 {
				snapshot["linked_documents"] = linkedDocs
			}
		}
	}

	return snapshot
}

func (s *PlanningSessionService) buildSystemPrompt(session *model.PlanningSession, agent *model.Agent) string {
	var b strings.Builder

	// --- Identity ---
	b.WriteString("You are a disciplined analyst and product manager working interactively with a human product owner. ")
	b.WriteString("Your job is to synthesize context into a canonical product specification (PRD) without drifting into implementation.\n\n")

	// --- Internal Stance ---
	b.WriteString("## Internal Stance\n\n")
	b.WriteString("- Think like an analyst first: identify the real problem, users affected, and evidence from source material.\n")
	b.WriteString("- Think like a PM second: convert that into goals, non-goals, requirements, scenarios, risks, and open questions.\n")
	b.WriteString("- Separate problem framing from solution detail.\n")
	b.WriteString("- Prefer normative requirement language (\"The system SHALL\") and concrete scenarios.\n")
	b.WriteString("- Make unknowns explicit — surface them as open questions or assumptions.\n")
	b.WriteString("- Distinguish assumptions from open questions so the human can resolve them.\n")
	b.WriteString("- Keep the spec product-facing; do not decompose into implementation tasks.\n\n")

	// --- Spec Abstraction Level ---
	b.WriteString("## Spec Abstraction Level\n\n")
	b.WriteString("This is a PRODUCT SPECIFICATION (PRD), not a technical design document.\n")
	b.WriteString("Stories with implementation details will be created from this spec later.\n\n")
	b.WriteString("DO include in the spec:\n")
	b.WriteString("- Problem statement and user impact\n")
	b.WriteString("- Goals and non-goals\n")
	b.WriteString("- Functional requirements (what the system does, not how)\n")
	b.WriteString("- User-facing scenarios with GIVEN/WHEN/THEN acceptance criteria\n")
	b.WriteString("- Constraints, edge cases, and boundary conditions\n")
	b.WriteString("- Risks, assumptions, and open questions\n\n")
	b.WriteString("DO NOT include:\n")
	b.WriteString("- Code snippets, type definitions, or interface signatures\n")
	b.WriteString("- Database schemas, SQL, or migration details\n")
	b.WriteString("- Internal service architecture or function signatures\n")
	b.WriteString("- Step-by-step implementation algorithms\n")
	b.WriteString("- Specific library/package choices or version numbers\n")
	b.WriteString("- Cache TTLs, polling intervals, or other implementation constants\n\n")
	b.WriteString("Example — WRONG (story-level):\n")
	b.WriteString("\"Implement a CompileWeeklyInsights service with a GetOrCompile method\n")
	b.WriteString(" that queries analytics_competitors table and caches for 7 days\"\n\n")
	b.WriteString("Example — RIGHT (PRD-level):\n")
	b.WriteString("\"The system SHALL compile weekly visibility insights per brand,\n")
	b.WriteString(" using cached results when available within the current reporting period\"\n\n")

	// --- Your Approach ---
	b.WriteString("## Your Approach\n\n")
	b.WriteString("Follow this decision flow on your FIRST turn:\n\n")
	b.WriteString("1. Read the epic context (description, linked docs, existing spec) provided in the first user message.\n")
	b.WriteString("2. If the context is CLEAR (you understand the problem, users, and goals):\n")
	b.WriteString("   - Explore the codebase first using tools to understand existing behavior.\n")
	b.WriteString("   - Then ask targeted questions about edge cases, scope boundaries, or ambiguities using the structured question format.\n")
	b.WriteString("3. If the context is AMBIGUOUS (unclear problem, vague scope, missing user context):\n")
	b.WriteString("   - Ask 2-3 scope-gating questions FIRST using the structured question format.\n")
	b.WriteString("   - Do NOT explore the codebase yet — wait for answers.\n\n")
	b.WriteString("On subsequent turns:\n")
	b.WriteString("- After receiving answers, explore the codebase if you haven't yet.\n")
	b.WriteString("- Ask follow-up questions if needed (still using the structured format).\n")
	b.WriteString("- When you have enough information, write the PRD.\n\n")
	b.WriteString("Ask questions progressively. Do not ask all questions at once — limit to 2-4 per turn.\n")
	b.WriteString("Focus on: What is the core problem? Who experiences it? What does success look like?\n")
	b.WriteString("Do NOT ask about implementation details, technology choices, or architecture.\n\n")
	b.WriteString("When the human confirms a decision, acknowledge it and note it as confirmed. ")
	b.WriteString("When they redirect, update your understanding.\n\n")

	// --- Asking Questions ---
	b.WriteString("## Asking Questions\n\n")
	b.WriteString("When you need to ask the human questions, ALWAYS use the structured <questions> XML format.\n")
	b.WriteString("Do NOT write questions as inline markdown text — the UI renders <questions> blocks as interactive forms.\n\n")
	b.WriteString("Format:\n")
	b.WriteString("<questions>\n")
	b.WriteString("  <question id=\"q1\">\n")
	b.WriteString("    <text>Who is the primary user for this feature?</text>\n")
	b.WriteString("    <option value=\"admin\">Workspace admins who manage team settings</option>\n")
	b.WriteString("    <option value=\"member\">Regular team members who use the feature daily</option>\n")
	b.WriteString("    <option value=\"both\">Both admins and members, with different permission levels</option>\n")
	b.WriteString("    <option value=\"other\" freetext=\"true\">Other (please specify)</option>\n")
	b.WriteString("  </question>\n")
	b.WriteString("</questions>\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Each question MUST have a unique id (q1, q2, q3, etc.)\n")
	b.WriteString("- Provide 2-5 <option> elements per question with concise, descriptive labels\n")
	b.WriteString("- ALWAYS include a final <option value=\"other\" freetext=\"true\">Other (please specify)</option> for flexibility\n")
	b.WriteString("- You CAN write markdown text before the <questions> block to provide context\n")
	b.WriteString("- Limit to 2-4 questions per turn to avoid overwhelming the user\n")
	b.WriteString("- The human's answers will arrive formatted as:\n")
	b.WriteString("  Q1: [question text]\n")
	b.WriteString("  → [value]: [selected label or free-text answer]\n\n")

	// --- Proposing Spec Content ---
	b.WriteString("## Proposing Spec Content\n\n")
	b.WriteString("As you gather enough information, propose spec content by wrapping it in a single <spec_draft> tag.\n")
	b.WriteString("The spec is ONE document — each time you propose, include the full updated PRD, not just a fragment.\n")
	b.WriteString("The content inside <spec_draft> is rendered live in a preview panel alongside the conversation.\n\n")
	b.WriteString("Example:\n\n")
	b.WriteString("<spec_draft>\n")
	b.WriteString("# Epic Title\n\n")
	b.WriteString("## Problem\nBrief description of the problem and user impact...\n\n")
	b.WriteString("## Goals\n- Goal 1\n- Goal 2\n\n## Non-Goals\n- Non-goal 1\n\n")
	b.WriteString("## Requirements\n")
	b.WriteString("- The system SHALL ...\n")
	b.WriteString("- The system SHALL ...\n\n")
	b.WriteString("## Scenarios\n")
	b.WriteString("GIVEN ...\nWHEN ...\nTHEN ...\n\n")
	b.WriteString("## Risks & Assumptions\n- ...\n\n")
	b.WriteString("## Open Questions\n- ...\n")
	b.WriteString("</spec_draft>\n\n")
	b.WriteString("You do not need to propose the full spec on every turn — only when you have meaningful content to show.\n")
	b.WriteString("Each <spec_draft> replaces the previous one, so always include the complete current state.\n\n")

	// --- Self-Check ---
	b.WriteString("## Self-Check Before Proposing Spec Content\n\n")
	b.WriteString("Before writing spec content, verify it covers:\n")
	b.WriteString("- Problem: Is the problem clearly stated with user impact?\n")
	b.WriteString("- Goals: Are success criteria defined? Are non-goals explicit?\n")
	b.WriteString("- Requirements: Are they functional (what, not how)? Do they use normative language?\n")
	b.WriteString("- Scenarios: Are GIVEN/WHEN/THEN acceptance criteria testable?\n")
	b.WriteString("- Risks & Assumptions: Are unknowns surfaced? Are assumptions distinguished from open questions?\n")
	b.WriteString("- Abstraction: Does it read like a PRD, not a technical design doc?\n\n")

	// --- Tools ---
	b.WriteString("## Tools\n\n")
	b.WriteString("You have read-only access to the codebase. Use tools to understand WHAT exists, not to design HOW to change it. ")
	b.WriteString("When discussing a feature that touches existing modules, read the relevant files first. ")
	b.WriteString("Mention what you found — e.g., 'I checked server/internal/auth/middleware.go and the current JWT implementation uses...'\n")
	b.WriteString("Don't use tools excessively — read what's relevant, not the entire codebase.\n\n")

	// --- Finalization ---
	b.WriteString("## Finalization\n\n")
	b.WriteString("When you believe you have enough information to write a complete spec, tell the human and ")
	b.WriteString("summarize the key decisions made. Wait for them to confirm before finalizing.\n\n")
	b.WriteString("When asked to finalize, write the complete PRD inside a single <spec_draft> tag. ")
	b.WriteString("Focus on requirements, scenarios, and acceptance criteria, not implementation details. ")
	b.WriteString("The entire spec must be inside the <spec_draft> tag.\n\n")

	// --- Planner Notes ---
	if agent != nil && agent.PlanningNotes != nil && *agent.PlanningNotes != "" {
		b.WriteString("## Planner Notes\n\n")
		b.WriteString(*agent.PlanningNotes)
		b.WriteString("\n\n")
	}

	return b.String()
}

func (s *PlanningSessionService) buildExecutionMessages(messages []model.PlanningSessionMessage, session *model.PlanningSession) ([]worker.ExecutionMessage, error) {
	if len(messages) == 0 {
		return []worker.ExecutionMessage{{
			Role:    "user",
			Content: s.buildInitialUserMessage(session),
		}}, nil
	}

	var history []worker.ExecutionMessage

	for _, msg := range messages {
		if msg.Role == "assistant" && len(msg.ContentBlocks) > 0 {
			blocks, err := decodePlanningBlocks(msg.ContentBlocks)
			if err == nil && len(blocks) > 0 {
				var assistantBlocks []worker.ExecutionBlock
				for _, block := range blocks {
					switch block.Type {
					case worker.ExecutionBlockTypeText, worker.ExecutionBlockTypeToolCall:
						assistantBlocks = append(assistantBlocks, block)
					case worker.ExecutionBlockTypeToolResult:
						if len(assistantBlocks) > 0 {
							history = append(history, worker.ExecutionMessage{
								Role:    "assistant",
								Content: extractTextFromBlocks(assistantBlocks),
								Blocks:  assistantBlocks,
							})
							assistantBlocks = nil
						}
						history = append(history, worker.ExecutionMessage{
							Role:    "tool",
							Content: block.Output,
							Blocks:  []worker.ExecutionBlock{block},
						})
					}
				}
				if len(assistantBlocks) > 0 {
					history = append(history, worker.ExecutionMessage{
						Role:    "assistant",
						Content: extractTextFromBlocks(assistantBlocks),
						Blocks:  assistantBlocks,
					})
				}
				continue
			}
		}

		history = append(history, worker.ExecutionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	if len(history) == 0 {
		return []worker.ExecutionMessage{{
			Role:    "user",
			Content: s.buildInitialUserMessage(session),
		}}, nil
	}

	if history[0].Role != "user" {
		history = append([]worker.ExecutionMessage{{
			Role:    "user",
			Content: s.buildInitialUserMessage(session),
		}}, history...)
	}

	return mergeConsecutiveExecutionMessages(history), nil
}

func (s *PlanningSessionService) buildInitialUserMessage(session *model.PlanningSession) string {
	var snapshot map[string]interface{}
	if err := json.Unmarshal(session.ContextSnapshot, &snapshot); err != nil {
		return "Please begin the planning session for this epic."
	}

	var b strings.Builder
	b.WriteString("I'd like to plan the following epic interactively with you.\n\n")

	if name, ok := snapshot["epic_name"].(string); ok && name != "" {
		b.WriteString("## Epic: ")
		b.WriteString(name)
		b.WriteString("\n\n")
	}

	if desc, ok := snapshot["epic_description"].(string); ok && desc != "" {
		b.WriteString("## Description\n\n")
		b.WriteString(desc)
		b.WriteString("\n\n")
	}

	if spec, ok := snapshot["existing_spec"].(string); ok && spec != "" {
		b.WriteString("## Existing Spec Draft\n\n")
		b.WriteString(spec)
		b.WriteString("\n\n")
	}

	if docs, ok := snapshot["linked_documents"].([]interface{}); ok && len(docs) > 0 {
		b.WriteString("## Linked Documents\n\n")
		for _, doc := range docs {
			if m, ok := doc.(map[string]interface{}); ok {
				if title, ok := m["title"].(string); ok && title != "" {
					b.WriteString("### ")
					b.WriteString(title)
					b.WriteString("\n\n")
				}
				if content, ok := m["content"].(string); ok && content != "" {
					b.WriteString(content)
					b.WriteString("\n\n")
				}
			}
		}
	}

	if additional, ok := snapshot["additional_context"].(string); ok && additional != "" {
		b.WriteString("## Additional Context\n\n")
		b.WriteString(additional)
		b.WriteString("\n\n")
	}

	b.WriteString("Please start by reviewing this information and asking me the most important scope-gating questions before drafting any spec content.")

	return b.String()
}

func (s *PlanningSessionService) resolveTools(ctx context.Context, session *model.PlanningSession) []worker.ToolDefinition {
	allowed := make(map[string]bool)
	configuredTools := parseJSONStringSlice(session.AllowedTools)
	if len(configuredTools) > 0 {
		for _, toolName := range configuredTools {
			allowed[toolName] = true
		}
	} else {
		for k, v := range model.PlanningSessionAllowedTools {
			allowed[k] = v
		}
	}

	// Check if web search is enabled.
	settings, _ := s.settingsRepo.GetWorkspaceSettings(ctx, session.WorkspaceID)
	if settings == nil || !settings.PlanningWebSearchEnabled {
		delete(allowed, "web_search")
	}

	return s.toolRegistry.DefinitionsFor(allowed)
}

func (s *PlanningSessionService) updateSessionTokenUsage(ctx context.Context, session *model.PlanningSession, usage worker.ExecutionUsage) {
	var current model.SessionTokenUsage
	_ = json.Unmarshal(session.TokenUsage, &current)
	current.Input += usage.InputTokens
	current.Output += usage.OutputTokens
	updated, _ := json.Marshal(current)
	session.TokenUsage = updated
	_ = s.sessionRepo.Update(ctx, session)
}

// persistSpecDraft replaces the session's spec_draft with the latest draft content.
func (s *PlanningSessionService) persistSpecDraft(ctx context.Context, session *model.PlanningSession, draft string) {
	session.SpecDraft = draft
	if err := s.sessionRepo.Update(ctx, session); err != nil {
		slog.Error("planning session: failed to persist spec draft", "error", err, "session_id", session.ID)
	}
}

func (s *PlanningSessionService) writeSpecToDoc(ctx context.Context, docID, content, actorID string) {
	// Update doc content — Upsert takes (ctx, docID, contentJSON).
	contentJSON, err := json.Marshal(content)
	if err != nil {
		slog.Error("planning session: failed to marshal spec content", "error", err)
		return
	}
	if _, err := s.docsContentRepo.Upsert(ctx, docID, contentJSON); err != nil {
		slog.Error("planning session: failed to write spec to doc", "error", err, "doc_id", docID)
		return
	}

	// Create a version.
	label := "AI Draft (Interactive)"
	wordCount := len(strings.Fields(content))
	_, err = s.docsVersionRepo.Create(ctx, docID, actorID, contentJSON, content, &label, "auto", wordCount)
	if err != nil {
		slog.Error("planning session: failed to create spec version", "error", err, "doc_id", docID)
	}
}

// ensureEpicSpecDocument creates a spec document for the epic if one doesn't exist.
// Mirrors AgentService.ensureEpicSpecDocument from agent_planning.go.
func (s *PlanningSessionService) ensureEpicSpecDocument(ctx context.Context, workspaceID string, epic *model.PMEpic, actorID string) (*model.DocsDocument, error) {
	if epic.SpecDocumentID != nil && strings.TrimSpace(*epic.SpecDocumentID) != "" {
		doc, err := s.docsDocumentRepo.GetByID(ctx, *epic.SpecDocumentID)
		if err != nil {
			return nil, err
		}
		if doc != nil {
			return doc, nil
		}
	}

	space, err := s.docsSpaceRepo.GetBySlug(ctx, workspaceID, productSpecsSpaceSlug)
	if err != nil {
		return nil, err
	}
	if space == nil {
		space, err = s.docsSpaceRepo.Create(ctx, &model.DocsSpace{
			WorkspaceID: workspaceID,
			Name:        productSpecsSpaceName,
			Slug:        productSpecsSpaceSlug,
			Visibility:  model.SpaceVisibilityWorkspaceWide,
			Type:        model.SpaceTypeInternal,
			IsSystem:    true,
			Position:    2,
			CreatedBy:   actorID,
		})
		if err != nil {
			return nil, fmt.Errorf("create product specs space: %w", err)
		}
	}

	teamID := epic.TeamID
	if teamID == nil {
		teamID = strPtr(actorID)
	}

	doc, err := s.docsDocumentRepo.Create(ctx, &model.DocsDocument{
		WorkspaceID: workspaceID,
		SpaceID:     space.ID,
		Title:       strings.TrimSpace(epic.Name) + " Product Spec",
		Status:      model.DocStatusDraft,
		Visibility:  model.SpaceVisibilityWorkspaceWide,
		OwnerID:     strPtr(actorID),
		TeamID:      teamID,
		TemplateKey: strPtr("product_spec"),
		Tags:        model.DocsStringArray{"product-spec", "epic"},
		CreatedBy:   actorID,
	})
	if err != nil {
		return nil, err
	}

	epic.SpecDocumentID = &doc.ID
	if err := s.epicRepo.Update(ctx, epic); err != nil {
		return nil, err
	}

	// Ensure epic ↔ doc link exists.
	links, linkErr := s.docsLinkRepo.ListByObject(ctx, workspaceID, model.LinkedObjectEpic, epic.ID)
	if linkErr == nil {
		found := false
		for _, link := range links {
			if link.DocumentID == doc.ID {
				found = true
				break
			}
		}
		if !found {
			_, _ = s.docsLinkRepo.Create(ctx, &model.DocsLink{
				WorkspaceID:      workspaceID,
				DocumentID:       doc.ID,
				LinkedObjectType: model.LinkedObjectEpic,
				LinkedObjectID:   epic.ID,
				LinkContext:      model.LinkContextCreatedFrom,
				CreatedBy:        actorID,
			})
		}
	}

	return doc, nil
}

func (s *PlanningSessionService) sendStreamError(sessionID, errMsg string) {
	s.streamer.SendToSession(sessionID, model.PlanningStreamEvent{
		Type:      "error",
		SessionID: sessionID,
		Error:     errMsg,
	})
}

func (s *PlanningSessionService) streamExecutionEvent(sessionID string, event worker.ExecutionEvent) {
	s.streamer.SendToSession(sessionID, model.PlanningStreamEvent{
		Type:          event.Type,
		SessionID:     sessionID,
		Text:          event.Text,
		ToolCallID:    event.ToolCallID,
		ToolName:      event.ToolName,
		ToolInput:     event.ToolInput,
		OutputSummary: event.OutputSummary,
		DurationMs:    event.DurationMs,
		Error:         event.Error,
	})
}

// --- Pure helpers ---

func extractTextFromBlocks(blocks []worker.ExecutionBlock) string {
	var parts []string
	for _, block := range blocks {
		if block.Type == worker.ExecutionBlockTypeText && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

var specDraftRe = regexp.MustCompile(`<spec_draft>\s*([\s\S]*?)\s*</spec_draft>`)
var questionsRe = regexp.MustCompile(`<questions>[\s\S]*?</questions>`)

func classifyMessage(text string) string {
	// Check for spec draft proposal.
	if specDraftRe.MatchString(text) {
		return model.PlanningMessageTypeProposal
	}

	// Check for structured questions block.
	if questionsRe.MatchString(text) {
		return model.PlanningMessageTypeQuestion
	}

	// Simple heuristics for other types.
	lower := strings.ToLower(text)
	if strings.Contains(lower, "?") && (strings.Contains(lower, "before i") || strings.Contains(lower, "let me ask") || strings.Contains(lower, "i need to understand")) {
		return model.PlanningMessageTypeQuestion
	}
	if strings.Contains(lower, "to summarize") || strings.Contains(lower, "key decisions") || strings.Contains(lower, "here's what we've decided") {
		return model.PlanningMessageTypeSummary
	}

	return model.PlanningMessageTypeMessage
}

// extractSpecDraft extracts the content of the last <spec_draft> tag from text.
// Returns empty string if no tag is found.
func extractSpecDraft(text string) string {
	matches := specDraftRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return ""
	}
	// Use the last match (most up-to-date draft).
	last := matches[len(matches)-1]
	if len(last) >= 2 {
		return strings.TrimSpace(last[1])
	}
	return ""
}

func mergeConsecutiveExecutionMessages(messages []worker.ExecutionMessage) []worker.ExecutionMessage {
	if len(messages) <= 1 {
		return messages
	}
	merged := []worker.ExecutionMessage{messages[0]}
	for i := 1; i < len(messages); i++ {
		last := &merged[len(merged)-1]
		if messages[i].Role != last.Role {
			merged = append(merged, messages[i])
			continue
		}
		last.Content = strings.TrimSpace(strings.Join([]string{last.Content, messages[i].Content}, "\n"))
		last.Blocks = append(last.Blocks, messages[i].Blocks...)
	}
	return merged
}

func truncateForDisplay(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func decodePlanningBlocks(raw json.RawMessage) ([]worker.ExecutionBlock, error) {
	var normalized []model.PlanningMessageBlock
	if err := json.Unmarshal(raw, &normalized); err == nil && len(normalized) > 0 {
		blocks := make([]worker.ExecutionBlock, 0, len(normalized))
		for _, block := range normalized {
			blocks = append(blocks, worker.ExecutionBlock{
				Type:       block.Type,
				Text:       block.Text,
				ToolCallID: block.ToolCallID,
				ToolName:   block.ToolName,
				Input:      block.Input,
				Output:     block.Output,
				IsError:    block.IsError,
			})
		}
		return blocks, nil
	}

	var legacy []worker.ContentBlock
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, err
	}
	blocks := make([]worker.ExecutionBlock, 0, len(legacy))
	for _, block := range legacy {
		switch block.Type {
		case "text":
			blocks = append(blocks, worker.ExecutionBlock{
				Type: worker.ExecutionBlockTypeText,
				Text: block.Text,
			})
		case "tool_use":
			blocks = append(blocks, worker.ExecutionBlock{
				Type:       worker.ExecutionBlockTypeToolCall,
				ToolCallID: block.ID,
				ToolName:   block.Name,
				Input:      block.Input,
			})
		case "tool_result":
			blocks = append(blocks, worker.ExecutionBlock{
				Type:       worker.ExecutionBlockTypeToolResult,
				ToolCallID: block.ToolUseID,
				Output:     block.Content,
				IsError:    block.IsError,
			})
		}
	}
	return blocks, nil
}
