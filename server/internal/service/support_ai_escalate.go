package service

// Escalation, NATS publish, and mailbox routing for the support AI.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// PublishAIRequest publishes an AI processing event to JetStream.
func (s *SupportAIService) PublishAIRequest(ctx context.Context, workspaceID, conversationID, messageID, content string) error {
	if s == nil || s.js == nil {
		return fmt.Errorf("support AI service not initialized")
	}
	payload, err := json.Marshal(AIRequestEvent{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		MessageID:      messageID,
		Content:        content,
	})
	if err != nil {
		return fmt.Errorf("marshal AI request event: %w", err)
	}

	_, err = s.js.Publish(
		"support.ai.request."+workspaceID,
		payload,
		nats.MsgId(messageID), // JetStream dedup via Nats-Msg-Id header
	)
	if err != nil {
		return fmt.Errorf("publish AI request: %w", err)
	}

	slog.InfoContext(ctx, "published AI request event",
		"workspace_id", workspaceID,
		"conversation_id", conversationID,
		"message_id", messageID,
	)
	return nil
}

// EscalateToHuman transitions a conversation from AI handling to human pickup.
func (s *SupportAIService) EscalateToHuman(ctx context.Context, workspaceID, conversationID, reason string) error {
	return s.escalateToHuman(ctx, workspaceID, conversationID, "", reason, "", "", "")
}

// EscalateToHumanForMessage transitions a conversation from AI handling to human pickup,
// using the triggering customer message to reuse triage routing when available.
func (s *SupportAIService) EscalateToHumanForMessage(ctx context.Context, workspaceID, conversationID, messageID, reason string) error {
	return s.escalateToHuman(ctx, workspaceID, conversationID, messageID, reason, "", "", "")
}

// EscalateToHumanForMessageWithIssue transitions a conversation from AI handling to human pickup,
// including the issue key and summary from the query plan for coverage tracking.
func (s *SupportAIService) EscalateToHumanForMessageWithIssue(ctx context.Context, workspaceID, conversationID, messageID, reason, issueKey, issueSummary string) error {
	return s.escalateToHuman(ctx, workspaceID, conversationID, messageID, reason, issueKey, issueSummary, "")
}

func (s *SupportAIService) escalateToHumanForMessageWithIssueAndReply(ctx context.Context, workspaceID, conversationID, messageID, reason, issueKey, issueSummary, transitionReply string) error {
	return s.escalateToHuman(ctx, workspaceID, conversationID, messageID, reason, issueKey, issueSummary, transitionReply)
}

func (s *SupportAIService) escalateToHuman(ctx context.Context, workspaceID, conversationID, messageID, reason, issueKey, issueSummary, transitionReply string) error {
	escalationLockKey := "support:ai:escalation-lock:" + conversationID
	if !s.acquireLock(ctx, escalationLockKey) {
		slog.InfoContext(ctx, "support escalation skipped — escalation already in progress",
			"workspace_id", workspaceID,
			"conversation_id", conversationID,
			"reason", reason,
		)
		return nil
	}
	defer s.releaseLock(ctx, escalationLockKey)

	conv, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return fmt.Errorf("get conversation for escalation: %w", err)
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}

	// Guard against duplicate escalation: the "Talk to human" button and the
	// AI's own message-level escalation path can race and each append a system
	// message. If the conversation is already escalated, skip.
	if conv.AIState != nil && *conv.AIState == "escalated" {
		slog.InfoContext(ctx, "support escalation skipped — already escalated",
			"workspace_id", workspaceID,
			"conversation_id", conversationID,
			"reason", reason,
		)
		return nil
	}

	now := time.Now()
	settings, availability, err := loadSupportAvailability(ctx, s.installationRepo, workspaceID, now)
	if err != nil {
		slog.ErrorContext(ctx, "load support availability for escalation", "error", err, "workspace_id", workspaceID, "conversation_id", conversationID)
		settings = model.DefaultSupportInboxSettings()
		availability = resolveSupportAvailability(settings, now)
	}

	handoffMailboxID, mailboxSelectionSource := s.resolveEscalationMailbox(ctx, workspaceID, conversationID, messageID, conv, settings)

	var selection *supportRecipientSelection
	if strings.TrimSpace(settings.HandoffBehavior) != "unassigned" {
		var selectErr error
		selection, selectErr = selectSupportConversationRecipient(
			ctx,
			s.workspaceRepo,
			s.mailboxRepo,
			s.installationRepo,
			nil,
			s.presence,
			s.statusOverrideRepo,
			supportRecipientSelectorInput{
				WorkspaceID:         workspaceID,
				MailboxID:           handoffMailboxID,
				OwnerUserID:         conv.AssignedUserID,
				HandoffBehavior:     settings.HandoffBehavior,
				HandoffTeamID:       settings.HandoffTeamID,
				RequireAvailability: true,
				Now:                 now,
			},
		)
		if selectErr != nil {
			slog.ErrorContext(ctx, "select escalation recipient", "error", selectErr, "conversation_id", conversationID)
		}
	}

	// Resolve the customer-facing handoff state and render the escalation message
	// for that state. Computed outside the dedupe guard below so handoffState is
	// persisted even when the customer-facing message is skipped.
	// handoff_state must reflect real teammate presence (the same source the
	// widget's pre-chat availability uses), NOT whether an assignee was selected:
	// with HandoffBehavior "unassigned" (the default) selection is always nil even
	// when a teammate is online. selection stays purely about routing/assignment.
	hasAvailableTeammate := anySupportTeammateOnline(ctx, s.workspaceRepo, s.presence, s.statusOverrideRepo, workspaceID, now)
	handoffState := resolveHandoffState(hasAvailableTeammate, availability.IsWithinOfficeHours)

	// Short phrase for the {reply_time} token. Do NOT use
	// availability.WidgetAvailability.ReplyTimeText — that is full-sentence copy
	// (and the offline branch sets it to the outside-hours message).
	replyTime := shortReplyTimePhrase(
		availability.WidgetAvailability.ReplyTimePreset,
		availability.WidgetAvailability.ReplyTimeMinutes,
	)

	// Next-open text (only meaningful for after_hours).
	var nextOpenText string
	if loc, err := time.LoadLocation(settings.BusinessHoursTimezone); err == nil {
		localNow := now.In(loc)
		nextOpenText = humanizeNextOpen(nextBusinessHoursStart(settings, localNow), localNow)
	}

	escalationContent := renderEscalationMessage(
		selectEscalationTemplate(settings, handoffState),
		replyTime,
		nextOpenText,
	)
	if reply := strings.TrimSpace(transitionReply); reply != "" {
		escalationContent = stripConversationPII(reply, conv.CustomerEmail, conv.CustomerPhone)
	}

	var replyMsg *model.SupportMessage
	var escalationSystemMsg *model.SupportMessage
	escalationAlreadyMessaged := false
	// Load with includeInternal=true so dedupe can see the new internal
	// handoff system events (and the legacy ai_escalated rows that were
	// public but are still recognized).
	history, historyErr := s.messageRepo.ListByConversation(ctx, workspaceID, conversationID, true)
	if historyErr != nil {
		slog.WarnContext(ctx, "load conversation history for escalation dedupe failed",
			"workspace_id", workspaceID,
			"conversation_id", conversationID,
			"error", historyErr,
		)
	} else {
		escalationAlreadyMessaged = hasEscalationMessageInHistory(history)
	}

	// 1. Create escalation messages — a customer-facing reply plus an
	// internal-only system event describing why the escalation happened.
	if !escalationAlreadyMessaged {
		createSystemEventFirst := systemEventForEscalationReason(reason) == model.SystemEventCustomerRequestedHuman
		if createSystemEventFirst {
			escalationSystemMsg = &model.SupportMessage{
				WorkspaceID:       workspaceID,
				ConversationID:    conversationID,
				SenderType:        "agent",
				MessageType:       "system",
				SystemEventType:   model.SupportSystemEventTypeStrPtr(systemEventForEscalationReason(reason)),
				SenderDisplayName: strPtr(helpinAIDisplayName),
				Content:           "",
				IsInternal:        true,
			}
			if err := s.messageRepo.Create(ctx, escalationSystemMsg); err != nil {
				return fmt.Errorf("create escalation system event: %w", err)
			}
		}

		sendPublicHandoff := model.SupportAIReplyAllowed(settings, conv, nil)
		replyChannel := model.SupportAIReplyChannel(conv, nil)
		if messageID != "" {
			source, err := s.messageRepo.GetByID(ctx, messageID)
			if err != nil {
				return fmt.Errorf("load escalation source: %w", err)
			}
			if source != nil && source.SenderType == "customer" {
				sendPublicHandoff = model.SupportAIReplyAllowed(settings, conv, source)
				replyChannel = model.SupportAIReplyChannel(conv, source)
			}
		}
		if sendPublicHandoff {
			replyMsg = &model.SupportMessage{
				WorkspaceID:       workspaceID,
				ConversationID:    conversationID,
				SenderType:        "ai",
				MessageType:       "reply",
				SenderDisplayName: strPtr(helpinAIDisplayName),
				Content:           escalationContent,
			}
			if replyChannel == "email" {
				replyMsg.Metadata = `{"delivery_mode":"email_only"}`
				replyMsg.ViaChannel = strPtr("email")
			}
			if err := s.messageRepo.Create(ctx, replyMsg); err != nil {
				return fmt.Errorf("create escalation reply: %w", err)
			}
		}

		if !createSystemEventFirst {
			escalationSystemMsg = &model.SupportMessage{
				WorkspaceID:       workspaceID,
				ConversationID:    conversationID,
				SenderType:        "agent",
				MessageType:       "system",
				SystemEventType:   model.SupportSystemEventTypeStrPtr(systemEventForEscalationReason(reason)),
				SenderDisplayName: strPtr(helpinAIDisplayName),
				Content:           "",
				IsInternal:        true,
			}
			if err := s.messageRepo.Create(ctx, escalationSystemMsg); err != nil {
				return fmt.Errorf("create escalation system event: %w", err)
			}
		}
	} else {
		slog.InfoContext(ctx, "support escalation system message skipped — already present",
			"workspace_id", workspaceID,
			"conversation_id", conversationID,
			"reason", reason,
		)
	}

	// 2. Transition AI state: pending → escalated
	flowState := escalatedConversationFlowState(settings, now)
	if selection != nil {
		flowState = model.SupportConversationFlowStateAssignedToHuman
	}
	fields := map[string]any{
		"ai_state":          "escalated",
		"ai_escalated_at":   now,
		"assigned_user_id":  nil,
		"assigned_agent_id": nil,
		"mailbox_id":        handoffMailboxID,
		"flow_state":        flowState,
		"handoff_state":     handoffState,
		"human_takeover":    true,
	}
	if selection != nil {
		fields["assigned_user_id"] = selection.UserID
	}
	if reason == "customer_requested" || reason == "customer_requested_human" {
		fields["customer_requested_human_at"] = now
	}
	if !availability.IsWithinOfficeHours && selection == nil {
		fields["flow_state"] = model.SupportConversationFlowStateAfterHoursQueue
	}
	if err := s.conversationRepo.UpdateFields(ctx, workspaceID, conversationID, fields); err != nil {
		return fmt.Errorf("update conversation for escalation: %w", err)
	}
	if selection != nil && strings.TrimSpace(selection.UserID) != "" && s.assignmentSystemMessageEmitter != nil {
		s.assignmentSystemMessageEmitter(ctx, workspaceID, conversationID, selection.UserID)
	}

	// 3. Record handoff for analytics
	handoffCtx, _ := json.Marshal(map[string]string{"handoff_state": handoffState})
	if err := s.handoffRepo.Create(ctx, &model.AgentHandoff{
		WorkspaceID:    workspaceID,
		ConversationID: &conversationID,
		HandoffType:    "agent_to_human",
		Reason:         reason,
		Context:        handoffCtx,
	}); err != nil {
		slog.ErrorContext(ctx, "record handoff failed", "error", err)
	}

	// Map escalation reason to coverage failure mode constant.
	failureMode := model.SupportCoverageFailureUnknown
	switch reason {
	case "low_confidence":
		failureMode = model.SupportCoverageFailureLowConfidence
	case "evidence_incomplete":
		failureMode = model.SupportCoverageFailureMissingContent
	case "no_retrieval":
		failureMode = model.SupportCoverageFailureNoRetrieval
	case "weak_retrieval":
		failureMode = model.SupportCoverageFailureWeakRetrieval
	case "stuck":
		failureMode = model.SupportCoverageFailureStuck
	case "customer_requested", "customer_requested_human":
		failureMode = model.SupportCoverageFailureCustomerRequestedHuman
	case "action_unavailable":
		failureMode = model.SupportCoverageFailureActionUnavailable
	case "context_unavailable", "answer_generation_failed", "planner_unavailable", "llm_provider_unavailable":
		failureMode = model.SupportCoverageFailureContextUnavailable
	case "policy_blocked":
		failureMode = model.SupportCoverageFailurePolicyBlocked
	}
	if strings.HasPrefix(reason, "answer_validation_") {
		failureMode = model.SupportCoverageFailureWeakRetrieval
		if strings.Contains(reason, supportValidationIncomplete) {
			failureMode = model.SupportCoverageFailureMissingContent
		}
	}

	handoffEvent := SupportEventInput{
		WorkspaceID:    workspaceID,
		EventType:      model.SupportEventAIHandoffTriggered,
		ConversationID: &conversationID,
		FailureMode:    failureMode,
		SourceSignal:   model.SupportCoverageSourceAIHandoff,
		ActorType:      model.SupportEventActorAI,
		Channel:        "widget",
		IssueKey:       issueKey,
		IssueSummary:   issueSummary,
	}
	if messageID != "" {
		handoffEvent.MessageID = &messageID
	}
	s.recordSupportEvent(handoffEvent)

	// 4. Broadcast events
	// Publish in persisted order so inbox clients that refetch on either signal
	// render the same sequence as a later full reload.
	if escalationSystemMsg != nil && systemEventForEscalationReason(reason) == model.SystemEventCustomerRequestedHuman {
		s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, escalationSystemMsg, "ai:escalation"))
	}
	if replyMsg != nil {
		s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, replyMsg, "ai:escalation"))
	}
	if escalationSystemMsg != nil && systemEventForEscalationReason(reason) != model.SystemEventCustomerRequestedHuman {
		s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, escalationSystemMsg, "ai:escalation"))
	}
	escalatedData, _ := json.Marshal(map[string]any{
		"conversation_id":    conversationID,
		"flow_state":         fields["flow_state"],
		"handoff_state":      handoffState,
		"handoff_started_at": now,
	})
	s.wsPublisher.Publish(websocket.Event{
		Action:      "escalated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		Data:        escalatedData,
	})

	// Push a refreshed visitor conversation list so the widget can observe the new
	// ai_state / flow_state and render the "waiting for a teammate" indicator.
	if conv.AnonymousID != nil && strings.TrimSpace(*conv.AnonymousID) != "" {
		s.publishVisitorConversationsRefresh(ctx, workspaceID, *conv.AnonymousID)
	}

	slog.InfoContext(ctx, "AI escalated to human",
		"workspace_id", workspaceID,
		"conversation_id", conversationID,
		"reason", reason,
		"mailbox_id", derefString(handoffMailboxID),
		"mailbox_selection_source", mailboxSelectionSource,
	)
	return nil
}

func (s *SupportAIService) publishVisitorConversationsRefresh(ctx context.Context, workspaceID, anonymousID string) {
	conversations, err := s.conversationRepo.ListByAnonymousID(ctx, workspaceID, anonymousID)
	if err != nil {
		slog.ErrorContext(ctx, "fetch visitor conversations for escalation refresh",
			"error", err, "workspace_id", workspaceID, "anonymous_id", anonymousID)
		return
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	listJSON, err := json.Marshal(map[string]any{"conversations": conversations})
	if err != nil {
		slog.ErrorContext(ctx, "marshal visitor conversations for escalation refresh", "error", err)
		return
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_visitor_conversations",
		EntityID:    anonymousID,
		WorkspaceID: workspaceID,
		Data:        listJSON,
	})
}

func (s *SupportAIService) resolveEscalationMailbox(ctx context.Context, workspaceID, conversationID, messageID string, conv *model.SupportConversation, settings model.SupportInboxSettings) (*string, string) {
	if triageMailboxID := s.resolveEscalationTriageMailbox(ctx, workspaceID, conversationID, messageID); triageMailboxID != nil {
		return triageMailboxID, "triage"
	}
	if currentMailboxID := s.resolveActiveMailboxID(ctx, workspaceID, conv.MailboxID); currentMailboxID != nil {
		return currentMailboxID, "current"
	}
	if fallbackMailboxID := s.resolveConfiguredHandoffMailbox(ctx, workspaceID, settings); fallbackMailboxID != nil {
		return fallbackMailboxID, "configured_handoff"
	}
	return nil, "shared"
}

func (s *SupportAIService) resolveEscalationTriageMailbox(ctx context.Context, workspaceID, conversationID, messageID string) *string {
	if s == nil || s.triageService == nil || strings.TrimSpace(messageID) == "" {
		return nil
	}
	triage, err := s.triageService.EvaluateAndRoute(ctx, workspaceID, conversationID, strings.TrimSpace(messageID))
	if err != nil {
		slog.WarnContext(ctx, "resolve escalation triage mailbox failed", "workspace_id", workspaceID, "conversation_id", conversationID, "message_id", messageID, "error", err)
		return nil
	}
	if triage == nil {
		return nil
	}
	return s.resolveActiveMailboxID(ctx, workspaceID, triage.SuggestedMailboxID)
}

func (s *SupportAIService) resolveConfiguredHandoffMailbox(ctx context.Context, workspaceID string, settings model.SupportInboxSettings) *string {
	if mailboxID := s.resolveActiveMailboxID(ctx, workspaceID, settings.AIHandoffMailboxID); mailboxID != nil {
		return mailboxID
	}
	return nil
}

func (s *SupportAIService) resolveActiveMailboxID(ctx context.Context, workspaceID string, mailboxID *string) *string {
	if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*mailboxID)
	if s == nil || s.mailboxRepo == nil {
		return &trimmed
	}
	mailbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, trimmed)
	if err != nil {
		slog.WarnContext(ctx, "load support mailbox during escalation", "workspace_id", workspaceID, "mailbox_id", trimmed, "error", err)
		return nil
	}
	if mailbox == nil || !mailbox.Active {
		return nil
	}
	return &trimmed
}

// loadSettings loads AI settings for a workspace.
func (s *SupportAIService) loadSettings(ctx context.Context, workspaceID string) (*model.SupportInboxSettings, error) {
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("no widget installation for workspace %s", workspaceID)
	}
	settings := parseSettings(inst.Settings)
	return &settings, nil
}
