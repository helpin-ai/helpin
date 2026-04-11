package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/mail"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// CreateWidgetSession creates a new session for external widget chat.
// Always creates a new session — multiple concurrent sessions per visitor are allowed.
func (s *SupportInboxService) CreateWidgetSession(ctx context.Context, widgetKey string, anonymousID string, customerName, customerEmail *string, userAgent, pageURL, timezone, locale *string) (*model.SupportWidgetSession, error) {
	inst, err := s.installationRepo.GetByWidgetKey(ctx, widgetKey)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("invalid widget key")
	}

	token, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}

	isAnonymous := customerEmail == nil || *customerEmail == ""

	session := &model.SupportWidgetSession{
		WorkspaceID:   inst.WorkspaceID,
		SessionToken:  token,
		AnonymousID:   anonymousID,
		IsAnonymous:   isAnonymous,
		CustomerName:  customerName,
		CustomerEmail: customerEmail,
		UserAgent:     userAgent,
		LastPageURL:   pageURL,
		Timezone:      timezone,
		Locale:        locale,
		ExpiresAt:     time.Now().Add(30 * 24 * time.Hour),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

// GetWidgetSession returns a valid session by token.
func (s *SupportInboxService) GetWidgetSession(ctx context.Context, token string) (*model.SupportWidgetSession, error) {
	session, err := s.sessionRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}
	if session.RevokedAt != nil {
		return nil, fmt.Errorf("session revoked")
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired")
	}
	return session, nil
}

// GetVisitorConversations returns all conversations for a visitor by anonymous_id.
func (s *SupportInboxService) GetVisitorConversations(ctx context.Context, workspaceID, anonymousID string) ([]model.SupportConversation, error) {
	conversations, err := s.conversationRepo.ListByAnonymousID(ctx, workspaceID, anonymousID)
	if err != nil {
		return nil, err
	}
	s.enrichWidgetConversationOwners(ctx, workspaceID, conversations)
	return conversations, nil
}

// UpgradeWidgetSession upgrades an anonymous session with email and name.
// It creates/promotes a CRM contact, backfills all conversations and sessions
// for the same anonymous_id, and broadcasts real-time updates.
// The source parameter controls lifecycle promotion: "identify" promotes lead→customer.
func (s *SupportInboxService) UpgradeWidgetSession(ctx context.Context, sessionToken string, identity model.WidgetIdentityPayload) error {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return err
	}

	resolved := resolveWidgetIdentityPayload(identity)

	// Run all state changes in a single transaction
	var contactID *string
	var updatedConvIDs []string

	db := s.conversationRepo.DB()
	txErr := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		convRepoTx := s.conversationRepo.WithTx(tx)
		sessionRepoTx := s.sessionRepo.WithTx(tx)
		contactRepoTx := s.contactRepo.WithTx(tx)

		// 1. Update current session
		session.CustomerEmail = &resolved.email
		if resolved.displayName != "" {
			session.CustomerName = &resolved.displayName
		}
		session.IsAnonymous = false
		if err := sessionRepoTx.Update(ctx, session); err != nil {
			return err
		}

		// 2. Create or match CRM contact — always as lead with source=live_chat
		contactID = s.matchOrCreateCRMContactIdentityTx(ctx, contactRepoTx, session.WorkspaceID, identity)

		// 3. Backfill ALL conversations for this anonymous_id
		ids, err := convRepoTx.UpdateIdentityByAnonymousID(ctx, session.WorkspaceID, session.AnonymousID, resolved.email, resolved.displayName, contactID)
		if err != nil {
			return err
		}
		updatedConvIDs = ids

		// 4. Backfill ALL sessions for this anonymous_id (multi-tab)
		if err := sessionRepoTx.UpdateSessionsByAnonymousID(ctx, session.WorkspaceID, session.AnonymousID, resolved.email, resolved.displayName); err != nil {
			return err
		}

		return nil
	})

	if txErr != nil {
		return txErr
	}

	// Broadcast WebSocket events AFTER commit for each affected conversation
	for _, convID := range updatedConvIDs {
		s.wsPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      "support_conversation",
			EntityID:    convID,
			WorkspaceID: session.WorkspaceID,
		})
	}

	slog.InfoContext(ctx, "widget session upgraded",
		"session_id", session.ID,
		"email", resolved.email,
		"source", resolved.source,
		"conversations_backfilled", len(updatedConvIDs),
	)
	return nil
}

// IdentifyByAnonymousID is the HTTP-based identity path for headless SDK usage.
// It looks up sessions by anonymous_id + widget key, then performs the same
// CRM contact creation, lifecycle promotion, and conversation backfill as UpgradeWidgetSession.
func (s *SupportInboxService) IdentifyByAnonymousID(ctx context.Context, widgetKey, anonymousID string, identity model.WidgetIdentityPayload) error {
	inst, err := s.installationRepo.GetByWidgetKey(ctx, widgetKey)
	if err != nil {
		return err
	}
	if inst == nil {
		return fmt.Errorf("invalid widget key")
	}

	workspaceID := inst.WorkspaceID
	resolved := resolveWidgetIdentityPayload(identity)

	var contactID *string
	var updatedConvIDs []string

	db := s.conversationRepo.DB()
	txErr := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		convRepoTx := s.conversationRepo.WithTx(tx)
		sessionRepoTx := s.sessionRepo.WithTx(tx)
		contactRepoTx := s.contactRepo.WithTx(tx)

		// 1. Create or match CRM contact
		contactID = s.matchOrCreateCRMContactIdentityTx(ctx, contactRepoTx, workspaceID, identity)

		// 2. Backfill ALL conversations for this anonymous_id
		ids, err := convRepoTx.UpdateIdentityByAnonymousID(ctx, workspaceID, anonymousID, resolved.email, resolved.displayName, contactID)
		if err != nil {
			return err
		}
		updatedConvIDs = ids

		// 3. Backfill ALL sessions for this anonymous_id
		if err := sessionRepoTx.UpdateSessionsByAnonymousID(ctx, workspaceID, anonymousID, resolved.email, resolved.displayName); err != nil {
			return err
		}

		return nil
	})

	if txErr != nil {
		return txErr
	}

	// Broadcast after commit
	for _, convID := range updatedConvIDs {
		s.wsPublisher.Publish(websocket.Event{
			Action:      "updated",
			Entity:      "support_conversation",
			EntityID:    convID,
			WorkspaceID: workspaceID,
		})
	}

	slog.InfoContext(ctx, "widget identify via HTTP",
		"anonymous_id", anonymousID,
		"email", resolved.email,
		"source", resolved.source,
		"conversations_backfilled", len(updatedConvIDs),
	)
	return nil
}

// UpdateSessionPageURL updates the last_page_url on a session using a targeted query.
func (s *SupportInboxService) UpdateSessionPageURL(ctx context.Context, sessionToken, url string) error {
	return s.sessionRepo.UpdatePageURL(ctx, sessionToken, url)
}

// RevokeWidgetSession marks a session as revoked.
func (s *SupportInboxService) RevokeWidgetSession(ctx context.Context, sessionToken string) error {
	session, err := s.sessionRepo.GetByToken(ctx, sessionToken)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session not found")
	}

	now := time.Now()
	session.RevokedAt = &now
	return s.sessionRepo.Update(ctx, session)
}

// ClearSessionConversation clears the conversation_id on a session so the next message creates a new conversation.
func (s *SupportInboxService) ClearSessionConversation(ctx context.Context, sessionToken string) error {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return err
	}
	session.ConversationID = nil
	return s.sessionRepo.Update(ctx, session)
}

// SetSessionConversation updates the active conversation on a widget session.
func (s *SupportInboxService) SetSessionConversation(ctx context.Context, sessionToken, conversationID string) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return fmt.Errorf("conversation_id is required")
	}

	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return err
	}

	conversation, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return err
	}
	if conversation == nil || conversation.AnonymousID == nil || *conversation.AnonymousID != session.AnonymousID {
		return fmt.Errorf("conversation not found")
	}

	session.ConversationID = &conversationID
	return s.sessionRepo.Update(ctx, session)
}

// GetInstallationByWidgetKey returns an installation by widget key.
func (s *SupportInboxService) GetInstallationByWidgetKey(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, error) {
	return s.installationRepo.GetByWidgetKey(ctx, widgetKey)
}

func (s *SupportInboxService) SendWidgetConversationTranscript(ctx context.Context, sessionToken, conversationID, email string) (*model.WidgetTranscriptResponse, error) {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(conversationID) == "" {
		return nil, fmt.Errorf("conversation_id is required")
	}

	conversation, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conversation.AnonymousID == nil || strings.TrimSpace(*conversation.AnonymousID) == "" || strings.TrimSpace(*conversation.AnonymousID) != strings.TrimSpace(session.AnonymousID) {
		return nil, fmt.Errorf("conversation not found")
	}

	recipientEmail := strings.TrimSpace(email)
	if recipientEmail == "" {
		recipientEmail = strings.TrimSpace(derefString(session.CustomerEmail))
	}
	if recipientEmail == "" {
		recipientEmail = strings.TrimSpace(derefString(conversation.CustomerEmail))
	}
	if recipientEmail == "" {
		return nil, fmt.Errorf("email is required")
	}
	if _, err := mail.ParseAddress(recipientEmail); err != nil {
		return nil, fmt.Errorf("invalid email address")
	}

	if session.IsAnonymous && strings.TrimSpace(email) != "" {
		name := strings.TrimSpace(derefString(session.CustomerName))
		if name == "" {
			name = strings.TrimSpace(derefString(conversation.CustomerName))
		}
		if err := s.UpgradeWidgetSession(ctx, sessionToken, model.WidgetIdentityPayload{
			Email:  recipientEmail,
			Name:   name,
			Source: "transcript_request",
		}); err != nil {
			slog.ErrorContext(ctx, "failed to upgrade widget session during transcript request", "error", err, "conversation_id", conversationID)
		}
	}

	messages, err := s.ListConversationMessages(ctx, session.WorkspaceID, conversationID, false)
	if err != nil {
		return nil, err
	}

	workspaceName := "Support"
	if s.workspaceRepo != nil {
		if workspace, err := s.workspaceRepo.GetByID(ctx, session.WorkspaceID); err == nil && workspace != nil && strings.TrimSpace(workspace.Name) != "" {
			workspaceName = strings.TrimSpace(workspace.Name)
		}
	}

	htmlBody, textBody := renderSupportTranscriptBodies(workspaceName, conversation, messages)
	subject := fmt.Sprintf("Your conversation transcript with %s", workspaceName)

	if s.emailFallbackService == nil || s.emailFallbackService.emailClient == nil {
		return nil, fmt.Errorf("email is not configured")
	}
	if err := s.emailFallbackService.emailClient.SendEmail(recipientEmail, subject, htmlBody, textBody); err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "widget transcript sent", "workspace_id", session.WorkspaceID, "conversation_id", conversationID, "email", recipientEmail)
	return &model.WidgetTranscriptResponse{
		Success: true,
		Message: fmt.Sprintf("Transcript sent to %s", recipientEmail),
	}, nil
}

// WidgetCreateConversation eagerly creates a new conversation for a widget session
// and returns the conversation with its server-assigned ID.
func (s *SupportInboxService) WidgetCreateConversation(ctx context.Context, sessionToken string) (*model.SupportConversation, error) {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return nil, err
	}

	ticket := &model.SupportConversation{
		WorkspaceID:   session.WorkspaceID,
		Subject:       "New conversation",
		Status:        "open",
		FlowState:     strPtr(model.SupportConversationFlowStateWaitingForHuman),
		Priority:      "medium",
		Channel:       "widget",
		CustomerName:  session.CustomerName,
		CustomerEmail: session.CustomerEmail,
		AnonymousID:   &session.AnonymousID,
		Source:        "widget",
	}

	mailboxID, mailbox, err := s.maybeApplyMailboxRoutingForChannel(ctx, session.WorkspaceID, nil, true, "widget")
	if err != nil {
		return nil, err
	}
	ticket.MailboxID = mailboxID
	if mailbox != nil {
		ownerID, flowState, ownerErr := s.determineMailboxOwner(ctx, session.WorkspaceID, mailbox, nil)
		if ownerErr != nil {
			return nil, ownerErr
		}
		ticket.OpenedByUserID = ownerID
		ticket.FlowState = strPtr(flowState)
	}

	if contactID := s.matchOrCreateCRMContact(ctx, session.WorkspaceID, session.CustomerEmail, session.CustomerName); contactID != nil {
		ticket.CRMContactID = contactID
	}

	if err := s.conversationRepo.Create(ctx, ticket); err != nil {
		return nil, err
	}

	session.ConversationID = &ticket.ID
	if err := s.sessionRepo.Update(ctx, session); err != nil {
		return nil, err
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "support_conversation",
		EntityID:    ticket.ID,
		WorkspaceID: session.WorkspaceID,
	})
	if s.triageService != nil {
		if err := s.triageService.HydrateConversation(ctx, ticket); err != nil {
			slog.ErrorContext(ctx, "hydrate widget support conversation triage", "error", err, "workspace_id", session.WorkspaceID, "conversation_id", ticket.ID)
		}
	}

	return ticket, nil
}

// WidgetCreateMessage creates a message from an external widget user.
func (s *SupportInboxService) WidgetCreateMessage(ctx context.Context, sessionToken, content string, attachmentIDs []string) (*model.SupportMessage, error) {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return nil, err
	}

	// Update conversation subject from first message if it was eagerly created with placeholder.
	if session.ConversationID != nil {
		conv, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, *session.ConversationID, "", model.RoleOwner)
		if err == nil && conv != nil && conv.Subject == "New conversation" {
			s.conversationRepo.UpdateSubject(ctx, conv.ID, truncate(strings.TrimSpace(content), 100))
		}
	}

	// If no conversation yet, create one.
	if session.ConversationID == nil {
		ticket := &model.SupportConversation{
			WorkspaceID:   session.WorkspaceID,
			Subject:       truncate(content, 100),
			Status:        "open",
			FlowState:     strPtr(model.SupportConversationFlowStateWaitingForHuman),
			Priority:      "medium",
			Channel:       "widget",
			CustomerName:  session.CustomerName,
			CustomerEmail: session.CustomerEmail,
			AnonymousID:   &session.AnonymousID,
			Source:        "widget",
		}

		mailboxID, mailbox, mailboxErr := s.maybeApplyMailboxRoutingForChannel(ctx, session.WorkspaceID, nil, true, "widget")
		if mailboxErr != nil {
			return nil, mailboxErr
		}
		ticket.MailboxID = mailboxID
		if mailbox != nil {
			ownerID, flowState, ownerErr := s.determineMailboxOwner(ctx, session.WorkspaceID, mailbox, nil)
			if ownerErr != nil {
				return nil, ownerErr
			}
			ticket.OpenedByUserID = ownerID
			ticket.FlowState = strPtr(flowState)
		}

		// Auto-match or create CRM contact by email.
		if contactID := s.matchOrCreateCRMContact(ctx, session.WorkspaceID, session.CustomerEmail, session.CustomerName); contactID != nil {
			ticket.CRMContactID = contactID
		}

		if err := s.conversationRepo.Create(ctx, ticket); err != nil {
			return nil, err
		}
		session.ConversationID = &ticket.ID
		if err := s.sessionRepo.Update(ctx, session); err != nil {
			return nil, err
		}

		s.wsPublisher.Publish(websocket.Event{
			Action:      "created",
			Entity:      "support_conversation",
			EntityID:    ticket.ID,
			WorkspaceID: session.WorkspaceID,
		})
	}

	displayName := "Customer"
	if session.CustomerName != nil && *session.CustomerName != "" {
		displayName = *session.CustomerName
	}

	msg := &model.SupportMessage{
		WorkspaceID:       session.WorkspaceID,
		ConversationID:    *session.ConversationID,
		SenderType:        "customer",
		SenderDisplayName: &displayName,
		Content:           strings.TrimSpace(content),
		IsInternal:        false,
		MessageType:       "reply",
		ViaChannel:        strPtr("widget"),
	}
	if s.linkPreviewService != nil {
		s.linkPreviewService.EnrichMessage(ctx, msg)
	}

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}

	// Link pre-uploaded attachments to this message.
	if s.attachmentService != nil && len(attachmentIDs) > 0 {
		if err := s.attachmentService.LinkToMessage(ctx, attachmentIDs, msg.ID); err != nil {
			slog.ErrorContext(ctx, "link widget attachments to message", "error", err, "message_id", msg.ID)
		}
		msgs := []model.SupportMessage{*msg}
		if err := s.attachmentService.HydrateMessages(ctx, msgs); err == nil {
			msg.Attachments = msgs[0].Attachments
		}
	}

	s.wsPublisher.Publish(websocket.SupportMessageEvent(session.WorkspaceID, msg, "widget:"+session.ID))

	if conv, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, *session.ConversationID, "", model.RoleOwner); err == nil {
		ProcessSupportCustomerReplyNotification(ctx, s.notificationService, conv, msg.Content, displayName)
	}

	go s.runWidgetPostMessageAutomation(context.WithoutCancel(ctx), session.WorkspaceID, *session.ConversationID, msg.ID, msg.Content)

	return msg, nil
}

func (s *SupportInboxService) runWidgetPostMessageAutomation(ctx context.Context, workspaceID, conversationID, messageID, content string) {
	if s == nil || s.installationRepo == nil {
		return
	}

	if s.triageService != nil {
		if _, err := s.triageService.EvaluateAndRoute(ctx, workspaceID, conversationID, messageID); err != nil {
			slog.ErrorContext(ctx, "widget support triage failed", "workspace_id", workspaceID, "conversation_id", conversationID, "message_id", messageID, "error", err)
		}
	}

	inst, instErr := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	settings := model.DefaultSupportInboxSettings()
	if instErr == nil && inst != nil {
		settings = parseSettings(inst.Settings)
	}

	if settings.AIEnabled && settings.AIResponseMode == "ai_first" && settings.AIAgentID != nil && s.supportAIService != nil {
		if pubErr := s.supportAIService.PublishAIRequest(ctx, workspaceID, conversationID, messageID, content); pubErr != nil {
			slog.ErrorContext(ctx, "failed to publish AI request event",
				"workspace_id", workspaceID,
				"conversation_id", conversationID,
				"error", pubErr,
			)
			go s.maybeAutoRunConversationAgent(context.WithoutCancel(ctx), workspaceID, conversationID)
			return
		}

		agentID := strings.TrimSpace(*settings.AIAgentID)
		pending := "pending"
		if err := s.conversationRepo.UpdateFields(ctx, workspaceID, conversationID, map[string]any{
			"ai_state":          &pending,
			"assigned_agent_id": &agentID,
			"flow_state":        model.SupportConversationFlowStateAIHandling,
		}); err != nil {
			slog.ErrorContext(ctx, "set AI handling state after widget triage", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		}
		return
	}

	go s.maybeAutoRunConversationAgent(context.WithoutCancel(ctx), workspaceID, conversationID)
}

// PublishWidgetTypingIndicator publishes a widget visitor typing event using session context.
func (s *SupportInboxService) PublishWidgetTypingIndicator(ctx context.Context, sessionToken string, isTyping bool) error {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return err
	}
	if session.ConversationID == nil || *session.ConversationID == "" {
		return nil
	}

	s.PublishTypingIndicator(ctx, session.WorkspaceID, *session.ConversationID, "widget:"+session.ID, isTyping, "")
	return nil
}

// GetWidgetConfig returns widget config by widget key (public).
func (s *SupportInboxService) GetWidgetConfig(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, error) {
	return s.installationRepo.GetByWidgetKey(ctx, widgetKey)
}

// GetPublicWidgetConfig returns the public-facing widget config.
func (s *SupportInboxService) GetPublicWidgetConfig(ctx context.Context, widgetKey string) (*model.WidgetConfigResponse, error) {
	inst, err := s.installationRepo.GetByWidgetKey(ctx, widgetKey)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("widget not found")
	}
	return s.buildWidgetConfigResponse(ctx, inst)
}

// GetPublicWidgetConfigByID returns the public-facing widget config by installation ID.
func (s *SupportInboxService) GetPublicWidgetConfigByID(ctx context.Context, id string) (*model.WidgetConfigResponse, error) {
	inst, err := s.installationRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("widget not found")
	}
	return s.buildWidgetConfigResponse(ctx, inst)
}

func (s *SupportInboxService) maybeAutoRunConversationAgent(ctx context.Context, workspaceID, conversationID string) {
	if s == nil || s.conversationAgentRunner == nil || s.installationRepo == nil || s.agentRepo == nil {
		return
	}

	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil || inst == nil {
		if err != nil {
			slog.ErrorContext(ctx, "support widget auto-run: get installation failed", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		}
		return
	}

	settings := parseSettings(inst.Settings)
	if !settings.AIEnabled || settings.AIAgentID == nil || strings.TrimSpace(*settings.AIAgentID) == "" {
		return
	}

	conversation, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil || conversation == nil {
		if err != nil {
			slog.ErrorContext(ctx, "support widget auto-run: get conversation failed", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		}
		return
	}

	configuredAgentID := strings.TrimSpace(*settings.AIAgentID)
	if conversation.AssignedAgentID == nil || strings.TrimSpace(*conversation.AssignedAgentID) == "" {
		if err := s.assignConversationAgent(ctx, workspaceID, conversationID, configuredAgentID, nil); err != nil {
			slog.ErrorContext(ctx, "support widget auto-run: assign ai agent failed", "workspace_id", workspaceID, "conversation_id", conversationID, "agent_id", configuredAgentID, "error", err)
			return
		}
	} else if strings.TrimSpace(*conversation.AssignedAgentID) != configuredAgentID {
		assignedAgent, err := s.agentRepo.GetByID(ctx, workspaceID, strings.TrimSpace(*conversation.AssignedAgentID))
		if err != nil {
			slog.ErrorContext(ctx, "support widget auto-run: get assigned agent failed", "workspace_id", workspaceID, "conversation_id", conversationID, "agent_id", *conversation.AssignedAgentID, "error", err)
			return
		}
		if assignedAgent == nil {
			return
		}
		if err := validateAgentTarget(assignedAgent, "support_conversation"); err != nil {
			slog.InfoContext(ctx, "support widget auto-run skipped for non-support assignee", "workspace_id", workspaceID, "conversation_id", conversationID, "agent_id", assignedAgent.ID, "error", err)
			return
		}
	}

	if _, err := s.conversationAgentRunner(ctx, workspaceID, conversationID); err != nil {
		slog.ErrorContext(ctx, "support widget auto-run failed", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
	}
}

func (s *SupportInboxService) enrichWidgetConversationOwners(ctx context.Context, workspaceID string, conversations []model.SupportConversation) {
	if len(conversations) == 0 {
		return
	}

	ownerIDs := make(map[string]struct{}, len(conversations))
	for _, conversation := range conversations {
		if conversation.OpenedByUserID == nil {
			continue
		}
		ownerID := strings.TrimSpace(*conversation.OpenedByUserID)
		if ownerID != "" {
			ownerIDs[ownerID] = struct{}{}
		}
	}
	if len(ownerIDs) == 0 {
		return
	}

	owners := make(map[string]model.WidgetActiveTeammate, len(ownerIDs))
	statusByUserID := make(map[string]string, len(ownerIDs))
	statuses, err := resolveSupportTeammatePresenceStatuses(
		ctx,
		s.workspaceRepo,
		s.presence,
		s.statusOverrideRepo,
		workspaceID,
		time.Now(),
	)
	if err == nil {
		for _, status := range statuses {
			statusByUserID[status.UserID] = status.Status
		}
	}
	if s.workspaceRepo != nil {
		if members, err := s.workspaceRepo.ListMembers(ctx, workspaceID); err == nil {
			for _, member := range members {
				if member.UserID == "" || strings.TrimSpace(member.FullName) == "" {
					continue
				}
				owners[member.UserID] = model.WidgetActiveTeammate{
					UserID:    member.UserID,
					Name:      member.FullName,
					AvatarURL: member.AvatarURL,
					Status:    statusByUserID[member.UserID],
				}
			}
		}
	}

	if s.userRepo != nil {
		for ownerID := range ownerIDs {
			if _, ok := owners[ownerID]; ok {
				continue
			}
			user, err := s.userRepo.GetByID(ctx, ownerID)
			if err != nil || user == nil || strings.TrimSpace(user.FullName) == "" {
				continue
			}
			owners[ownerID] = model.WidgetActiveTeammate{
				UserID:    user.ID,
				Name:      user.FullName,
				AvatarURL: user.AvatarURL,
				Status:    statusByUserID[user.ID],
			}
		}
	}

	for i := range conversations {
		if conversations[i].OpenedByUserID == nil {
			continue
		}
		ownerID := strings.TrimSpace(*conversations[i].OpenedByUserID)
		if ownerID == "" {
			continue
		}
		owner, ok := owners[ownerID]
		if !ok {
			continue
		}
		conversations[i].OpenedByDisplayName = strPtr(owner.Name)
		conversations[i].OpenedByAvatarURL = owner.AvatarURL
		if strings.TrimSpace(owner.Status) != "" {
			conversations[i].OpenedByStatus = strPtr(owner.Status)
		}
	}
}

func supportTeammateStatusRank(status string) int {
	switch strings.TrimSpace(status) {
	case model.SupportTeammateStatusOnline:
		return 0
	case model.SupportTeammateStatusAway:
		return 1
	default:
		return 2
	}
}

func (s *SupportInboxService) listWidgetTeammates(ctx context.Context, workspaceID string, limit int) []model.WidgetActiveTeammate {
	if s.workspaceRepo == nil {
		return []model.WidgetActiveTeammate{}
	}

	supportUserIDs, err := s.workspaceRepo.ListSupportAccessibleUserIDs(ctx, workspaceID)
	if err != nil || len(supportUserIDs) == 0 {
		return []model.WidgetActiveTeammate{}
	}
	supportUserIDSet := make(map[string]struct{}, len(supportUserIDs))
	for _, userID := range supportUserIDs {
		supportUserIDSet[userID] = struct{}{}
	}

	statusByUserID := map[string]string{}
	statuses, err := resolveSupportTeammatePresenceStatuses(
		ctx,
		s.workspaceRepo,
		s.presence,
		s.statusOverrideRepo,
		workspaceID,
		time.Now(),
	)
	if err == nil {
		for _, status := range statuses {
			statusByUserID[status.UserID] = status.Status
		}
	}

	members, err := s.workspaceRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return []model.WidgetActiveTeammate{}
	}

	teammates := make([]model.WidgetActiveTeammate, 0, len(members))
	for _, member := range members {
		if strings.TrimSpace(member.UserID) == "" || strings.TrimSpace(member.FullName) == "" {
			continue
		}
		if _, ok := supportUserIDSet[member.UserID]; !ok {
			continue
		}
		status := statusByUserID[member.UserID]
		if status == "" {
			status = model.SupportTeammateStatusOffline
		}
		teammates = append(teammates, model.WidgetActiveTeammate{
			UserID:    member.UserID,
			Name:      member.FullName,
			AvatarURL: member.AvatarURL,
			Status:    status,
		})
	}

	sort.SliceStable(teammates, func(i, j int) bool {
		left := teammates[i]
		right := teammates[j]
		if supportTeammateStatusRank(left.Status) != supportTeammateStatusRank(right.Status) {
			return supportTeammateStatusRank(left.Status) < supportTeammateStatusRank(right.Status)
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})

	if limit > 0 && len(teammates) > limit {
		return teammates[:limit]
	}
	return teammates
}

func widgetActiveTeammateFromConversation(conversation *model.SupportConversation) *model.WidgetActiveTeammate {
	if conversation == nil || conversation.OpenedByUserID == nil || conversation.OpenedByDisplayName == nil {
		return nil
	}
	userID := strings.TrimSpace(*conversation.OpenedByUserID)
	name := strings.TrimSpace(*conversation.OpenedByDisplayName)
	if userID == "" || name == "" {
		return nil
	}
	return &model.WidgetActiveTeammate{
		UserID:    userID,
		Name:      name,
		AvatarURL: conversation.OpenedByAvatarURL,
		Status: func() string {
			if conversation.OpenedByStatus == nil {
				return ""
			}
			return strings.TrimSpace(*conversation.OpenedByStatus)
		}(),
	}
}

// buildWidgetConfigResponse maps installation settings to the nested WidgetConfig
// shape expected by the widget-core TypeScript interface.
func (s *SupportInboxService) buildWidgetConfigResponse(ctx context.Context, inst *model.SupportWidgetInstallation) (*model.WidgetConfigResponse, error) {
	settings := parseSettings(inst.Settings)

	position := settings.LauncherPosition
	if position == "" || position == "bottom_right" {
		position = "bottom-right"
	} else if position == "bottom_left" {
		position = "bottom-left"
	}

	primaryColor := settings.BrandColor
	if primaryColor == "" {
		primaryColor = "#6366f1"
	}

	helpSpaces, err := s.resolveWidgetHelpSpaces(ctx, inst.WorkspaceID, settings.WidgetHelpSpaceIDs)
	if err != nil {
		return nil, err
	}

	return &model.WidgetConfigResponse{
		WorkspaceID:   inst.WorkspaceID,
		WorkspaceName: settings.WidgetName,
		Branding: model.WidgetConfigBranding{
			PrimaryColor:    primaryColor,
			LogoURL:         settings.LogoURL,
			WelcomeMessage:  settings.WelcomeMessage,
			WidgetPosition:  position,
			ShowBranding:    settings.ShowBranding,
			LauncherIcon:    settings.LauncherIcon,
			ColorScheme:     settings.ColorScheme,
			ButtonColor:     settings.ButtonColor,
			ButtonIconColor: settings.ButtonIconColor,
		},
		Features: model.WidgetConfigFeatures{
			AIEnabled:         settings.AIEnabled,
			AIFirst:           settings.AIEnabled && settings.AIResponseMode == "ai_first",
			ShowTalkToHuman:   settings.ShowTalkToHuman,
			EscalationMessage: settings.EscalationMessage,
			FileUploads:       settings.FileUploadsEnabled,
			PreChatForm:       settings.RequireEmailBeforeChat,
			RequirePhone:      settings.RequirePhoneAfterEmail,
			CSATRating:        settings.CSATEnabled,
			ForceIdentify:     settings.ForceVisitorIdentity,
		},
		Availability:       buildWidgetAvailability(settings, time.Now()),
		AvailableTeammates: s.listWidgetTeammates(ctx, inst.WorkspaceID, 5),
		HelpSpaces:         helpSpaces,
	}, nil
}

func (s *SupportInboxService) resolveWidgetHelpSpaces(ctx context.Context, workspaceID string, configuredIDs []string) ([]model.WidgetHelpSpace, error) {
	if len(configuredIDs) == 0 {
		return []model.WidgetHelpSpace{}, nil
	}
	if s.docsSpaceRepo == nil {
		return nil, fmt.Errorf("docs spaces repository not configured")
	}

	spaces, err := s.docsSpaceRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]model.DocsSpace, len(spaces))
	for _, space := range spaces {
		if space.Type == model.SpaceTypeExternalCapable {
			byID[space.ID] = space
		}
	}

	result := make([]model.WidgetHelpSpace, 0, len(configuredIDs))
	seen := make(map[string]struct{}, len(configuredIDs))
	for _, id := range configuredIDs {
		if _, alreadySeen := seen[id]; alreadySeen {
			continue
		}
		space, ok := byID[id]
		if !ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, model.WidgetHelpSpace{
			ID:   space.ID,
			Name: space.Name,
			Slug: space.Slug,
			Icon: space.Icon,
		})
	}

	return result, nil
}

func (s *SupportInboxService) getAllowedWidgetHelpSpaces(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, []model.WidgetHelpSpace, error) {
	inst, err := s.installationRepo.GetByWidgetKey(ctx, widgetKey)
	if err != nil {
		return nil, nil, err
	}
	if inst == nil {
		return nil, nil, fmt.Errorf("widget not found")
	}

	settings := parseSettings(inst.Settings)
	spaces, err := s.resolveWidgetHelpSpaces(ctx, inst.WorkspaceID, settings.WidgetHelpSpaceIDs)
	if err != nil {
		return nil, nil, err
	}

	return inst, spaces, nil
}

func findWidgetHelpSpaceBySlug(spaces []model.WidgetHelpSpace, slug string) *model.WidgetHelpSpace {
	for i := range spaces {
		if spaces[i].Slug == slug {
			return &spaces[i]
		}
	}
	return nil
}

func widgetHelpSpaceIDs(spaces []model.WidgetHelpSpace) []string {
	ids := make([]string, 0, len(spaces))
	for _, space := range spaces {
		ids = append(ids, space.ID)
	}
	return ids
}

// ListWidgetHelpCollections returns widget-visible collections for a selected space.
func (s *SupportInboxService) ListWidgetHelpCollections(ctx context.Context, widgetKey, spaceSlug string) ([]model.WidgetHelpCollection, error) {
	_, allowedSpaces, err := s.getAllowedWidgetHelpSpaces(ctx, widgetKey)
	if err != nil {
		return nil, err
	}

	space := findWidgetHelpSpaceBySlug(allowedSpaces, spaceSlug)
	if space == nil {
		return nil, fmt.Errorf("space not found")
	}
	if s.docsHelpcenterRepo == nil {
		return nil, fmt.Errorf("docs helpcenter repository not configured")
	}

	return s.docsHelpcenterRepo.ListWidgetCollections(ctx, space.ID)
}

// ListWidgetHelpArticles returns widget-visible articles for a selected collection.
func (s *SupportInboxService) ListWidgetHelpArticles(ctx context.Context, widgetKey, collectionSlug string) ([]model.WidgetHelpArticleSummary, error) {
	inst, allowedSpaces, err := s.getAllowedWidgetHelpSpaces(ctx, widgetKey)
	if err != nil {
		return nil, err
	}
	if s.docsHelpcenterRepo == nil || s.docsCollectionRepo == nil {
		return nil, fmt.Errorf("docs repositories not configured")
	}

	if strings.HasPrefix(collectionSlug, "uncategorized:") {
		spaceID := strings.TrimPrefix(collectionSlug, "uncategorized:")
		for _, space := range allowedSpaces {
			if space.ID == spaceID {
				return s.docsHelpcenterRepo.ListWidgetArticlesBySpaceUncategorized(ctx, spaceID)
			}
		}
		return nil, fmt.Errorf("collection not found")
	}

	collection, err := s.docsCollectionRepo.GetByID(ctx, collectionSlug)
	if err != nil {
		return nil, err
	}
	if collection == nil || collection.WorkspaceID != inst.WorkspaceID {
		return nil, fmt.Errorf("collection not found")
	}

	allowedByID := make(map[string]struct{}, len(allowedSpaces))
	for _, space := range allowedSpaces {
		allowedByID[space.ID] = struct{}{}
	}
	if _, ok := allowedByID[collection.SpaceID]; !ok {
		return nil, fmt.Errorf("collection not found")
	}

	return s.docsHelpcenterRepo.ListWidgetArticlesByCollectionID(ctx, collection.ID)
}

// GetWidgetHelpArticle returns a widget-visible article with rendered HTML content.
func (s *SupportInboxService) GetWidgetHelpArticle(ctx context.Context, widgetKey, articleSlug string) (*model.WidgetHelpArticle, error) {
	inst, allowedSpaces, err := s.getAllowedWidgetHelpSpaces(ctx, widgetKey)
	if err != nil {
		return nil, err
	}
	if s.docsHelpcenterRepo == nil {
		return nil, fmt.Errorf("docs helpcenter repository not configured")
	}

	doc, _, content, err := s.docsHelpcenterRepo.GetPublicArticleByDocumentIDInSpaces(ctx, widgetHelpSpaceIDs(allowedSpaces), articleSlug)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("article not found")
	}

	var contentJSON json.RawMessage
	if content != nil {
		contentJSON = content.Content
	}

	publicPath, err := s.buildWidgetHelpArticlePublicPath(ctx, inst.WorkspaceID, doc)
	if err != nil {
		return nil, err
	}

	return &model.WidgetHelpArticle{
		ID:          doc.ID,
		Title:       doc.Title,
		Slug:        doc.ID,
		Excerpt:     doc.Excerpt,
		Icon:        doc.Icon,
		ContentHTML: renderWidgetArticleHTML(contentJSON),
		PublicPath:  publicPath,
	}, nil
}

func (s *SupportInboxService) buildWidgetHelpArticlePublicPath(ctx context.Context, workspaceID string, doc *model.DocsDocument) (*string, error) {
	if doc == nil || s.docsHelpcenterRepo == nil {
		return nil, nil
	}

	cfg, err := s.docsHelpcenterRepo.GetConfig(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get help center config: %w", err)
	}
	if cfg == nil || strings.TrimSpace(cfg.Subdomain) == "" {
		return nil, nil
	}

	article, err := s.docsHelpcenterRepo.GetArticle(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("get help center article: %w", err)
	}
	if article == nil || strings.TrimSpace(article.Slug) == "" {
		return nil, nil
	}

	articleSlug := strings.TrimSpace(article.Slug)
	publicID := strings.TrimSpace(article.PublicID)
	if publicID == "" {
		return nil, nil
	}
	articleKey := buildDocsHelpcenterArticleKey(articleSlug, publicID)

	// Only build a public URL when the workspace has a custom domain configured.
	// Without a custom domain the help center may not be publicly reachable,
	// so we return nil and the widget hides the external link.
	if cfg.CustomDomain == nil || strings.TrimSpace(*cfg.CustomDomain) == "" {
		return nil, nil
	}

	domain := strings.TrimSpace(*cfg.CustomDomain)
	if !strings.HasPrefix(domain, "http") {
		domain = "https://" + domain
	}
	baseURL := strings.TrimRight(domain, "/")

	path := fmt.Sprintf("%s/articles/%s", baseURL, articleKey)
	return &path, nil
}

// ListWidgetTokens returns all active widget installations formatted as tokens
// for the events-pipeline rust-capture service.
func (s *SupportInboxService) ListWidgetTokens(ctx context.Context) ([]model.WidgetToken, error) {
	installations, err := s.installationRepo.ListAllActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list widget tokens: %w", err)
	}

	tokens := make([]model.WidgetToken, 0, len(installations))
	for _, inst := range installations {
		tokens = append(tokens, model.WidgetToken{
			ID:           inst.ID,
			ClientSecret: inst.WidgetKey,
			ServerSecret: inst.SecretKey,
			Origins:      []string{"*"},
		})
	}
	return tokens, nil
}
