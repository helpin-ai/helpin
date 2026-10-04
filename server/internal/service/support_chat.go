package service

// SupportChatService runs a support conversation as a long-lived agent-runtime
// chat-mode run (Echo). It replaces the in-app LLM pipeline: deterministic
// pre-gates stay here (state machine, idempotency, locks, hard-phrase
// escalation, caps, budget), the reasoning happens in the runtime, and
// support.send_reply / escalate_to_human are the only output paths.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	// supportChatMaxAITurns is an absolute per-conversation ceiling on AI
	// turns, in addition to the configured answer-turn limit.
	supportChatMaxAITurns = 30

	supportChatCarryForwardTurns = 20
	supportChatCarryForwardChar  = 500
	supportChatCarryForwardTotal = 6000
)

// SupportChatService owns the conversation↔chat-run lifecycle.
type SupportChatService struct {
	conversationRepo *repository.SupportConversationRepository
	messageRepo      *repository.SupportMessageRepository
	processingRepo   *repository.AIMessageProcessingRepository
	runRepo          *repository.AgentRunRepository
	planRepo         *repository.CommandBarPlanRepository
	agentService     *AgentService
	runCloser        supportChatRunCloser
	supportAIService *SupportAIService
	evidenceRepo     *repository.SupportRunEvidenceRepository
	followUpService  *SupportFollowUpService
	jev              *SupportJevService
}

// SetResearchEvidenceDependencies wires the store used to turn completed
// read-only child research into server-validatable support evidence.
func (s *SupportChatService) SetResearchEvidenceDependencies(evidenceRepo *repository.SupportRunEvidenceRepository) {
	if s == nil {
		return
	}
	s.evidenceRepo = evidenceRepo
}

type supportChatRunCloser interface {
	CancelRun(ctx context.Context, workspaceID, runID, actorID string) (*model.AgentRun, error)
}

// NewSupportChatService creates a SupportChatService.
func NewSupportChatService(
	conversationRepo *repository.SupportConversationRepository,
	messageRepo *repository.SupportMessageRepository,
	processingRepo *repository.AIMessageProcessingRepository,
	runRepo *repository.AgentRunRepository,
	planRepo *repository.CommandBarPlanRepository,
	agentService *AgentService,
	supportAIService *SupportAIService,
) *SupportChatService {
	if supportAIService != nil {
		supportAIService.runCloser = agentService
	}
	return &SupportChatService{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		processingRepo:   processingRepo,
		runRepo:          runRepo,
		planRepo:         planRepo,
		agentService:     agentService,
		runCloser:        agentService,
		supportAIService: supportAIService,
	}
}

// HandleVisitorMessage is the consumer entry point for one visitor message:
// deterministic pre-gates, then start / resume / defer the conversation's
// chat run. Errors bubble to the NATS consumer for retry; exhausted retries
// escalate to a human (the caller's responsibility).
func (s *SupportChatService) HandleVisitorMessage(ctx context.Context, workspaceID, conversationID string, msg *model.SupportMessage) error {
	if !supportAIMessageEligible(msg) {
		return nil
	}
	supportAI := s.supportAIService
	settings, err := supportAI.loadSettings(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("load support settings: %w", err)
	}
	portalMessage := msg.ViaChannel != nil && strings.EqualFold(*msg.ViaChannel, "portal")
	if !portalMessage && !shouldAutomaticallyProcessSupportAI(*settings) {
		return nil
	}

	conv, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return fmt.Errorf("get conversation: %w", err)
	}
	*settings = effectiveSupportAISettings(*settings, conv)
	if !shouldAutomaticallyProcessSupportAI(*settings) {
		if conv != nil && conv.Channel == "portal" {
			supportAI.stopPortalAITurn(ctx, workspaceID, conversationID)
		}
		return nil
	}
	if conv != nil && conv.Channel == "portal" {
		var dispatch struct{ ModeAtEnqueue string }
		if err := s.conversationRepo.DB().WithContext(ctx).Table("support_portal_ai_dispatches").
			Select("mode_at_enqueue").Where("workspace_id = ? AND conversation_id = ? AND source_message_id = ?", workspaceID, conversationID, msg.ID).
			Take(&dispatch).Error; err != nil {
			return nil
		}
		contacts, err := repository.NewCustomerPortalRepository(s.conversationRepo.DB()).ContactsForEmail(ctx, workspaceID, derefString(conv.CustomerEmail))
		if err != nil {
			return fmt.Errorf("check portal AI customer access: %w", err)
		}
		if !evaluatePortalEligibility(settings.EffectivePortalAccessMode(), contacts).Eligible {
			supportAI.stopPortalAITurn(ctx, workspaceID, conversationID)
			return nil
		}
	}
	agentID := strings.TrimSpace(derefString(settings.AIAgentID))
	if agentID == "" {
		return nil
	}
	if !supportAIConversationSupported(conv) || !model.SupportAIReplyAllowed(*settings, conv, msg) {
		return nil
	}

	// State machine gates (straight port of the pipeline's checks).
	if model.SupportAIConversationBlocked(conv) || !model.SupportAIReplyAllowed(*settings, conv, msg) {
		return nil
	}
	switch derefString(conv.AIState) {
	case "escalated":
		return nil
	case "resolved":
		// Visitor came back after resolution: reopen for AI handling.
		pending := "pending"
		if err := s.conversationRepo.UpdateFields(ctx, workspaceID, conversationID, map[string]any{
			"status":      model.SupportConversationStatusOpen,
			"resolved_at": nil, "ai_resolved_at": nil, "ai_resolution_type": nil,
			"ai_state":   &pending,
			"flow_state": model.SupportConversationFlowStateAIHandling,
		}); err != nil {
			return fmt.Errorf("reopen resolved conversation: %w", err)
		}
		conv.Status = model.SupportConversationStatusOpen
		conv.AIState = &pending
	}

	// Idempotency: one turn per source message, ever.
	processing, ok := s.processingRepo.BeginAttempt(ctx, workspaceID, msg.ID, conversationID)
	if !ok {
		return nil
	}

	// Per-conversation serialization.
	lockKey := "support:ai:lock:" + conversationID
	if !supportAI.acquireLock(ctx, lockKey) {
		// Another turn is being set up right now — park this message; the
		// pause hook drains it.
		_ = s.processingRepo.MarkDeferred(ctx, processing.ID)
		return nil
	}
	defer supportAI.releaseLock(ctx, lockKey)

	// Hard escalation phrases bypass the agent entirely.
	if reason := checkHardEscalation(msg.Content); reason != "" {
		if err := supportAI.EscalateToHumanForMessage(ctx, workspaceID, conversationID, msg.ID, reason); err != nil {
			return fmt.Errorf("hard escalation: %w", err)
		}
		_ = s.processingRepo.MarkCompleted(ctx, processing.ID, nil, 0)
		return nil
	}

	// Turn caps.
	history, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		history = nil
	}
	sanitized := sanitizeConversationHistory(history, msg.ID)
	if countAgentAITurns(sanitized, agentID) >= supportChatMaxAITurns ||
		countMaxFollowupAITurns(sanitized, agentID) >= settings.AIMaxFollowups {
		if err := supportAI.EscalateToHumanForMessage(ctx, workspaceID, conversationID, msg.ID, "max_followups_reached"); err != nil {
			return fmt.Errorf("turn cap escalation: %w", err)
		}
		_ = s.processingRepo.MarkCompleted(ctx, processing.ID, nil, 0)
		return nil
	}

	commonRoute := ""
	if err == nil && conv.Channel != "portal" {
		commonRoute = classifySupportCommonMessage(conv, msg, history)
	}
	if commonRoute == "" && s.jev != nil && err == nil {
		handled, assessErr := s.assessJevHandoff(ctx, conv, msg, history)
		if assessErr != nil {
			return assessErr
		}
		if handled {
			return s.processingRepo.MarkCompleted(ctx, processing.ID, nil, 0)
		}
	}

	// Budget gates.
	agent, err := s.agentService.GetAgent(ctx, workspaceID, agentID)
	if err != nil || agent == nil {
		if escErr := supportAI.EscalateToHumanForMessage(ctx, workspaceID, conversationID, msg.ID, "llm_provider_unavailable"); escErr != nil {
			return fmt.Errorf("agent unavailable escalation: %w", escErr)
		}
		_ = s.processingRepo.MarkCompleted(ctx, processing.ID, nil, 0)
		return nil
	}
	if !supportAI.checkTokenBudget(agent) {
		if err := supportAI.EscalateToHumanForMessage(ctx, workspaceID, conversationID, msg.ID, "token_budget_exhausted"); err != nil {
			return fmt.Errorf("budget escalation: %w", err)
		}
		_ = s.processingRepo.MarkCompleted(ctx, processing.ID, nil, 0)
		return nil
	}
	if !supportAI.consumeDailyReplyBudget(ctx, workspaceID) {
		if err := supportAI.EscalateToHumanForMessage(ctx, workspaceID, conversationID, msg.ID, "token_budget_exhausted"); err != nil {
			return fmt.Errorf("daily budget escalation: %w", err)
		}
		_ = s.processingRepo.MarkCompleted(ctx, processing.ID, nil, 0)
		return nil
	}

	supportAI.publishTypingIndicator(ctx, workspaceID, conversationID, true)
	if commonRoute == supportGreetingOperation {
		handled, err := s.replyToInitialGreeting(ctx, conv, msg, agent, processing, *settings)
		if err != nil || handled {
			supportAI.publishTypingIndicator(ctx, workspaceID, conversationID, false)
			return err
		}
	}

	composed := strings.TrimSpace(msg.Content)
	if feedback := supportVisitorFeedbackContext(history, *msg); feedback != "" {
		composed = feedback + "\n\n" + composed
	}
	return s.startOrResumeChatRun(ctx, conv, agent, processing, composed)
}

// startOrResumeChatRun delivers one composed turn into the conversation's
// chat run, creating or succeeding it as needed. Mirrors the dock lifecycle
// with one difference: a message arriving mid-turn is deferred, not rejected.
func (s *SupportChatService) startOrResumeChatRun(ctx context.Context, conv *model.SupportConversation, agent *model.Agent, processing *model.AIMessageProcessing, composed string) error {
	workspaceID := conv.WorkspaceID
	var currentRun *model.AgentRun
	if conv.AIActiveRunID != nil && strings.TrimSpace(*conv.AIActiveRunID) != "" {
		run, err := s.runRepo.GetByID(ctx, workspaceID, *conv.AIActiveRunID)
		if err != nil {
			return fmt.Errorf("get chat run: %w", err)
		}
		currentRun = run
	}

	switch {
	case currentRun == nil || !model.IsAgentRunActiveStatus(currentRun.Status):
		return s.startSupportChatRun(ctx, conv, agent, composed, currentRun, nil)
	case model.IsAgentRunPausedStatus(currentRun.Status):
		if currentRun.PauseReason != model.AgentRunPauseReasonUserMessage {
			// Paused on an interaction (approval etc.) — park the message;
			// the pause hook or sweep delivers it when the turn settles.
			return s.processingRepo.MarkDeferred(ctx, processing.ID)
		}
		if _, err := s.agentService.SendRunMessage(ctx, workspaceID, currentRun.ID, "", model.SendAgentRunMessageRequest{Content: composed}); err != nil {
			if isChatRunExpiredError(err) {
				return s.startSupportChatRun(ctx, conv, agent, composed, currentRun, nil)
			}
			return fmt.Errorf("resume chat run: %w", err)
		}
		return nil
	default:
		// Mid-turn: queue-and-coalesce.
		return s.processingRepo.MarkDeferred(ctx, processing.ID)
	}
}

// startSupportChatRun creates a (possibly successor) chat run for the
// conversation with transcript carry-forward and repoints ai_active_run_id.
func (s *SupportChatService) startSupportChatRun(ctx context.Context, conv *model.SupportConversation, agent *model.Agent, composed string, previousRun *model.AgentRun, pendingEvidence []model.SupportRunEvidence) error {
	if !supportAIConversationSupported(conv) {
		return nil
	}
	settings, err := s.supportAIService.loadSettings(ctx, conv.WorkspaceID)
	if err != nil {
		return err
	}
	allowed, err := s.channelAllowsPendingTurn(ctx, *settings, conv)
	if err != nil {
		return err
	}
	if !allowed {
		return nil
	}
	workspaceID := conv.WorkspaceID
	additional := composed
	var parentRunID *string
	if previousRun != nil || conv.AIResumedAt != nil || conv.AITurnCount > 0 {
		if carry := s.buildCarryForward(ctx, workspaceID, conv.ID, previousRun); carry != "" {
			additional = carry + "\n\n" + composed
		}
		if previousRun != nil {
			parentRunID = &previousRun.ID
		}
	}

	now := time.Now().UTC()
	trigger := &model.AgentRunTriggerContext{
		Source:      model.AgentRunTriggerSourceSystem,
		TriggerType: supportChatTriggerType,
		FiredAt:     &now,
	}
	input, err := buildAgentRunInputPayload("support_conversation", conv.ID, trigger, nil, nil, &additional, nil)
	if err != nil {
		return fmt.Errorf("build support chat input: %w", err)
	}

	run, err := s.agentService.createRun(ctx, createRunParams{
		workspaceID:          workspaceID,
		agent:                agent,
		targetType:           "support_conversation",
		targetID:             conv.ID,
		conversationID:       &conv.ID,
		parentRunID:          parentRunID,
		allowActiveParentRun: parentRunID != nil,
		input:                input,
		trigger:              trigger,
		invocationMode:       model.InvocationModeInteractive,
	})
	if err != nil {
		return fmt.Errorf("start support chat run: %w", err)
	}
	if len(pendingEvidence) > 0 && !s.persistSupportChildEvidence(ctx, run.ID, pendingEvidence) {
		slog.WarnContext(ctx, "support chat: child evidence persistence failed before successor launch",
			"workspace_id", workspaceID, "run_id", run.ID)
	}
	bound, bindErr := s.conversationRepo.BindAIRun(ctx, conv, run.ID, agent.ID)
	if bindErr != nil || !bound {
		s.supportAIService.cancelControlledRun(ctx, workspaceID, conv.ID, run.ID)
		if bindErr != nil {
			return fmt.Errorf("bind support run: %w", bindErr)
		}
		return nil
	}
	conv.AIActiveRunID = &run.ID
	if conv.Channel == "portal" {
		conv.AssignedAgentID = &agent.ID
	}
	s.agentService.publishRunEvent(run, "")
	return nil
}

// buildCarryForward renders recent transcript so a successor run keeps the
// conversation thread (support_messages is the canonical transcript — it
// includes teammate/system messages the run transcript wouldn't).
func (s *SupportChatService) buildCarryForward(ctx context.Context, workspaceID, conversationID string, previousRun *model.AgentRun) string {
	messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		messages = nil
	}
	if len(messages) > supportChatCarryForwardTurns {
		messages = messages[len(messages)-supportChatCarryForwardTurns:]
	}
	var b strings.Builder
	b.WriteString("<previous_conversation>\n")
	b.WriteString("This support conversation continues from an earlier session (previous run ")
	if previousRun != nil {
		b.WriteString(strings.TrimSpace(previousRun.Status))
	} else {
		b.WriteString("none; earlier replies are in the transcript")
	}
	b.WriteString("). Recent transcript:\n")
	total := 0
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" || message.IsInternal {
			continue
		}
		if len(content) > supportChatCarryForwardChar {
			content = content[:supportChatCarryForwardChar] + "…"
		}
		line := message.SenderType + ": " + content + "\n"
		if total+len(line) > supportChatCarryForwardTotal {
			break
		}
		b.WriteString(line)
		total += len(line)
	}
	b.WriteString("</previous_conversation>")
	return b.String()
}

// Email delivery of a widget reply does not authorize AI to answer inbound email.
func supportAIMessageEligible(message *model.SupportMessage) bool {
	if message == nil || message.SenderType != "customer" || message.IsInternal || model.IsSupportEmailNotice(message) {
		return false
	}
	channel := strings.ToLower(strings.TrimSpace(derefString(message.ViaChannel)))
	return (channel == "" || channel == "widget" || channel == "chat" || channel == "email" || channel == "portal") && (channel != "email" || !model.SupportEmailSuppressesAI(message))
}

func supportAIConversationSupported(conversation *model.SupportConversation) bool {
	if conversation == nil {
		return false
	}
	channel := strings.ToLower(strings.TrimSpace(conversation.Channel))
	return channel == "" || channel == "widget" || channel == "chat" || channel == "email" || channel == "portal"
}

// Recheck the saved choice when queued or child work is resumed. The pending
// source preserves email continuations of widget conversations.
func (s *SupportChatService) channelAllowsPendingTurn(ctx context.Context, settings model.SupportInboxSettings, conv *model.SupportConversation) (bool, error) {
	settings = effectiveSupportAISettings(settings, conv)
	if !shouldAutomaticallyProcessSupportAI(settings) || model.SupportAIConversationBlocked(conv) {
		return false, nil
	}
	var source *model.SupportMessage
	if s.processingRepo != nil {
		row, err := s.processingRepo.LatestProcessingForConversation(ctx, conv.WorkspaceID, conv.ID)
		if err != nil {
			return false, err
		}
		if row != nil {
			source, err = s.messageRepo.GetByID(ctx, row.SourceMessageID)
			if err != nil {
				return false, err
			}
		}
	}
	return model.SupportAIReplyAllowed(settings, conv, source), nil
}
