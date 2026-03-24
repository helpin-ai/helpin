package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// SupportInboxService contains support business logic.
type SupportInboxService struct {
	conversationRepo        *repository.SupportConversationRepository
	messageRepo             *repository.SupportMessageRepository
	agentRepo               *repository.AgentRepository
	assocRepo               *repository.CRMAssociationRepository
	installationRepo        *repository.SupportInboxInstallationRepository
	sessionRepo             *repository.SupportInboxSessionRepository
	cannedResponseRepo      *repository.SupportCannedResponseRepository
	activitySvc             *PMActivityService
	wsPublisher             *websocket.Publisher
	contactRepo             *repository.CRMContactRepository
	userRepo                *repository.UserRepository
	docsSpaceRepo           *repository.DocsSpaceRepository
	docsCollectionRepo      *repository.DocsCollectionRepository
	docsHelpcenterRepo      *repository.DocsHelpcenterRepository
	conversationAgentRunner func(ctx context.Context, workspaceID, conversationID string) (*model.AgentRun, error)
	supportAIService        *SupportAIService
	emailFallbackService    *EmailFallbackService
	notificationService     *NotificationService
	workspaceRepo           *repository.WorkspaceRepository
	attachmentService       *SupportAttachmentService
}

// NewSupportInboxService creates a new SupportInboxService.
func NewSupportInboxService(
	conversationRepo *repository.SupportConversationRepository,
	messageRepo *repository.SupportMessageRepository,
	agentRepo *repository.AgentRepository,
	assocRepo *repository.CRMAssociationRepository,
	installationRepo *repository.SupportInboxInstallationRepository,
	sessionRepo *repository.SupportInboxSessionRepository,
	cannedResponseRepo *repository.SupportCannedResponseRepository,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
	contactRepo *repository.CRMContactRepository,
	userRepo *repository.UserRepository,
	docsSpaceRepo *repository.DocsSpaceRepository,
	docsCollectionRepo *repository.DocsCollectionRepository,
	docsHelpcenterRepo *repository.DocsHelpcenterRepository,
) *SupportInboxService {
	return &SupportInboxService{
		conversationRepo:   conversationRepo,
		messageRepo:        messageRepo,
		agentRepo:          agentRepo,
		assocRepo:          assocRepo,
		installationRepo:   installationRepo,
		sessionRepo:        sessionRepo,
		cannedResponseRepo: cannedResponseRepo,
		activitySvc:        activitySvc,
		wsPublisher:        wsPublisher,
		contactRepo:        contactRepo,
		userRepo:           userRepo,
		docsSpaceRepo:      docsSpaceRepo,
		docsCollectionRepo: docsCollectionRepo,
		docsHelpcenterRepo: docsHelpcenterRepo,
	}
}

func renderWidgetArticleHTML(content json.RawMessage) *string {
	if len(content) == 0 {
		return nil
	}

	rendered, err := tiptap.RenderHTML(content)
	if err != nil || rendered == "" {
		return nil
	}

	return &rendered
}

// SetSupportAIService injects the AI-first auto-reply service.
func (s *SupportInboxService) SetSupportAIService(aiService *SupportAIService) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.supportAIService = aiService
	return s
}

// SetEmailFallbackService injects the email fallback service used for offline visitor replies.
func (s *SupportInboxService) SetEmailFallbackService(emailFallbackService *EmailFallbackService) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.emailFallbackService = emailFallbackService
	return s
}

// SetNotificationService injects the notification service and workspace repo for @mention support.
func (s *SupportInboxService) SetNotificationService(ns *NotificationService, wr *repository.WorkspaceRepository) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.notificationService = ns
	s.workspaceRepo = wr
	return s
}

// SetAttachmentService injects the support attachment service for file upload support.
func (s *SupportInboxService) SetAttachmentService(attachmentService *SupportAttachmentService) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.attachmentService = attachmentService
	return s
}

// SetConversationAgentRunner injects the agent-run startup hook used for widget auto-replies.
func (s *SupportInboxService) SetConversationAgentRunner(runner func(ctx context.Context, workspaceID, conversationID string) (*model.AgentRun, error)) *SupportInboxService {
	if s == nil {
		return nil
	}
	s.conversationAgentRunner = runner
	return s
}

// ListConversations returns conversations with optional filters.
func (s *SupportInboxService) ListConversations(ctx context.Context, workspaceID, status, priority string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.conversationRepo.List(ctx, workspaceID, status, priority, pagination)
}

// ListConversationsWithMeta returns conversations plus aggregate unread stats.
func (s *SupportInboxService) ListConversationsWithMeta(ctx context.Context, workspaceID, userID, status, priority string, pagination model.PMPagination, aiState ...string) (*model.ConversationListResponse, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	conversations, total, err := s.conversationRepo.List(ctx, workspaceID, status, priority, pagination, aiState...)
	if err != nil {
		return nil, err
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}

	stats, err := s.conversationRepo.GetUnreadStats(ctx, workspaceID, userID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get unread stats", "error", err, "workspace_id", workspaceID)
		// Non-fatal: return conversations with zero stats
	}

	perPage := pagination.PerPage
	if perPage <= 0 {
		perPage = 50
	}
	page := pagination.Page
	if page <= 0 {
		page = 1
	}
	totalPages := 0
	if perPage > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}

	return &model.ConversationListResponse{
		Data:       conversations,
		Total:      int(total),
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
		Meta: model.ConversationListMeta{
			Unread: stats,
		},
	}, nil
}

// GetUnreadStats returns aggregate unread conversation counts for sidebar badges.
func (s *SupportInboxService) GetUnreadStats(ctx context.Context, workspaceID, userID string) (model.UnreadStats, error) {
	return s.conversationRepo.GetUnreadStats(ctx, workspaceID, userID)
}

// MarkConversationRead updates the team read cursor and broadcasts a read event.
func (s *SupportInboxService) MarkConversationRead(ctx context.Context, workspaceID, conversationID, userID string) error {
	conv, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}

	if err := s.conversationRepo.MarkInternalRead(ctx, conversationID); err != nil {
		return err
	}
	if s.notificationService != nil {
		recipients := []string{userID}
		if conv.OpenedByUserID != nil && strings.TrimSpace(*conv.OpenedByUserID) != "" && *conv.OpenedByUserID != userID {
			recipients = append(recipients, *conv.OpenedByUserID)
		}
		for _, recipientID := range recipients {
			if err := s.notificationService.MarkEntityCategoryAsRead(ctx, recipientID, workspaceID, "support_conversation", conversationID, model.NotifCategorySupportReplies); err != nil {
				slog.ErrorContext(ctx, "mark support reply notifications read", "error", err, "conversation_id", conversationID, "user_id", recipientID)
			}
		}
	}

	// Broadcast read event so other tabs/users can invalidate
	reasonJSON, _ := json.Marshal(map[string]string{"reason": "read"})
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     userID,
		Data:        reasonJSON,
	})
	return nil
}

// MarkConversationReadByVisitor updates the contact read cursor and broadcasts a list refresh.
func (s *SupportInboxService) MarkConversationReadByVisitor(ctx context.Context, workspaceID, conversationID, anonymousID string) error {
	conv, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}
	if conv.AnonymousID == nil || *conv.AnonymousID != anonymousID {
		return fmt.Errorf("conversation not found")
	}

	if err := s.conversationRepo.MarkContactRead(ctx, conversationID); err != nil {
		return err
	}

	// Push authoritative conversations:listed refresh to all visitor widget sessions
	s.pushVisitorConversationsRefresh(ctx, workspaceID, anonymousID)

	// Broadcast to agent dashboard so read receipts update in real-time
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
	})

	return nil
}

// EscalateConversation delegates to the AI service to escalate a conversation to a human agent.
func (s *SupportInboxService) EscalateConversation(ctx context.Context, workspaceID, conversationID, reason string) error {
	if s.supportAIService == nil {
		return fmt.Errorf("ai service not configured")
	}
	return s.supportAIService.EscalateToHuman(ctx, workspaceID, conversationID, reason)
}

// pushVisitorConversationsRefresh sends an updated conversation list to all widget sessions for a visitor.
func (s *SupportInboxService) pushVisitorConversationsRefresh(ctx context.Context, workspaceID, anonymousID string) {
	conversations, err := s.conversationRepo.ListByAnonymousID(ctx, workspaceID, anonymousID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch visitor conversations for refresh", "error", err)
		return
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	listJSON, _ := json.Marshal(map[string]any{"conversations": conversations})
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_visitor_conversations",
		EntityID:    anonymousID,
		WorkspaceID: workspaceID,
		Data:        listJSON,
	})
}

// MarkConversationUnread resets the team read cursor so the conversation appears unread.
func (s *SupportInboxService) MarkConversationUnread(ctx context.Context, workspaceID, conversationID, userID string) error {
	conv, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}

	if err := s.conversationRepo.MarkUnread(ctx, conversationID); err != nil {
		return err
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     userID,
	})
	return nil
}

// UpdateConversationSubject changes the conversation subject.
func (s *SupportInboxService) UpdateConversationSubject(ctx context.Context, workspaceID, conversationID, subject, actorID string) (*model.SupportConversation, error) {
	if strings.TrimSpace(subject) == "" {
		return nil, fmt.Errorf("subject is required")
	}

	conv, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	if err := s.conversationRepo.UpdateSubject(ctx, conversationID, strings.TrimSpace(subject)); err != nil {
		return nil, err
	}
	conv.Subject = strings.TrimSpace(subject)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return conv, nil
}

// DeleteConversation permanently deletes a conversation and its messages.
func (s *SupportInboxService) DeleteConversation(ctx context.Context, workspaceID, conversationID, actorID string) error {
	conv, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}

	if err := s.conversationRepo.Delete(ctx, workspaceID, conversationID); err != nil {
		return err
	}

	slog.InfoContext(ctx, "conversation deleted", "conversation_id", conversationID, "workspace_id", workspaceID, "actor_id", actorID)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "deleted",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})
	return nil
}

// GetConversation returns a single conversation.
func (s *SupportInboxService) GetConversation(ctx context.Context, workspaceID, id string) (*model.SupportConversation, error) {
	ticket, err := s.conversationRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket not found")
	}
	return ticket, nil
}

// CreateConversation creates a new support conversation.
func (s *SupportInboxService) CreateConversation(ctx context.Context, req model.CreateConversationRequest, actorID string) (*model.SupportConversation, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Subject) == "" {
		return nil, fmt.Errorf("workspace_id and subject are required")
	}

	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}
	source := req.Source
	if source == "" {
		source = "internal"
	}

	ticket := &model.SupportConversation{
		WorkspaceID:    req.WorkspaceID,
		Subject:        strings.TrimSpace(req.Subject),
		Status:         "open",
		Priority:       priority,
		CustomerName:   req.CustomerName,
		CustomerEmail:  req.CustomerEmail,
		OpenedByUserID: &actorID,
		Source:         source,
	}

	if err := s.conversationRepo.Create(ctx, ticket); err != nil {
		return nil, err
	}

	// Auto-match or create CRM contact by email.
	if contactID := s.matchOrCreateCRMContact(ctx, ticket.WorkspaceID, ticket.CustomerEmail, ticket.CustomerName); contactID != nil {
		ticket.CRMContactID = contactID
		if err := s.conversationRepo.Update(ctx, ticket); err != nil {
			slog.ErrorContext(ctx, "failed to link CRM contact to ticket", "error", err, "ticket_id", ticket.ID)
		}
	}

	_ = s.activitySvc.Log(ctx, ticket.WorkspaceID, "support_conversation", ticket.ID, &actorID, "created", nil, nil, &ticket.Subject, nil)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "support_conversation",
		EntityID:    ticket.ID,
		WorkspaceID: ticket.WorkspaceID,
		ActorID:     actorID,
	})

	return ticket, nil
}

// validConversationStatuses defines allowed status transitions.
var validConversationStatuses = map[string]bool{
	"open": true, "in_progress": true, "waiting": true, "resolved": true, "closed": true,
}

// UpdateConversationStatus changes conversation status.
func (s *SupportInboxService) UpdateConversationStatus(ctx context.Context, workspaceID, ticketID, status, actorID string) (*model.SupportConversation, error) {
	if !validConversationStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	ticket, err := s.conversationRepo.GetByID(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket not found")
	}

	oldStatus := ticket.Status
	ticket.Status = status

	now := time.Now()
	switch status {
	case "resolved":
		ticket.ResolvedAt = &now
	case "closed":
		ticket.ClosedAt = &now
	}

	if err := s.conversationRepo.Update(ctx, ticket); err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", ticketID, &actorID, "updated", strPtr("status"), &oldStatus, &status, nil)

	// Insert a system message for status transitions visible in the thread.
	if oldStatus != status && (status == "resolved" || status == "closed" || (oldStatus == "resolved" && status == "open")) {
		label := "Resolved conversation"
		if status == "closed" {
			label = "Closed conversation"
		} else if status == "open" && oldStatus == "resolved" {
			label = "Reopened conversation"
		}

		// Resolve actor display name and avatar.
		var senderDisplayName *string
		var senderAvatarURL *string
		if actorID != "" && s.userRepo != nil {
			user, _ := s.userRepo.GetByID(ctx, actorID)
			if user != nil {
				senderDisplayName = &user.FullName
				senderAvatarURL = user.AvatarURL
			}
		}
		senderUserID := &actorID

		sysMsg := &model.SupportMessage{
			WorkspaceID:       workspaceID,
			ConversationID:    ticketID,
			SenderType:        "user",
			SenderUserID:      senderUserID,
			SenderDisplayName: senderDisplayName,
			SenderAvatarURL:   senderAvatarURL,
			Content:           label,
			MessageType:       "system",
			IsInternal:        false,
		}
		if err := s.messageRepo.Create(ctx, sysMsg); err != nil {
			slog.ErrorContext(ctx, "create system message for status change", "error", err, "conversation_id", ticketID)
		} else {
			s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, sysMsg, actorID))
		}
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    ticketID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return ticket, nil
}

// ListConversationMessages returns messages for a conversation.
func (s *SupportInboxService) ListConversationMessages(ctx context.Context, workspaceID, ticketID string, includeInternal bool) ([]model.SupportMessage, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, ticketID, includeInternal)
	if err != nil {
		return nil, err
	}

	// Hydrate file attachments onto messages.
	if s.attachmentService != nil && len(messages) > 0 {
		if err := s.attachmentService.HydrateMessages(ctx, messages); err != nil {
			slog.ErrorContext(ctx, "hydrate support message attachments", "error", err, "conversation_id", ticketID)
		}
	}
	return messages, nil
}

// CreateConversationMessage creates a message on a conversation.
func (s *SupportInboxService) CreateConversationMessage(ctx context.Context, workspaceID, ticketID string, req model.CreateMessageRequest, senderType string, senderUserID, senderAgentID *string, senderDisplayName *string) (*model.SupportMessage, error) {
	if strings.TrimSpace(req.Content) == "" && len(req.AttachmentIDs) == 0 {
		return nil, fmt.Errorf("content is required")
	}

	messageType := req.MessageType
	if messageType == "" {
		messageType = "reply"
	}

	// Auto-resolve sender display name and avatar from user record.
	var senderAvatarURL *string
	if senderUserID != nil && s.userRepo != nil {
		user, _ := s.userRepo.GetByID(ctx, *senderUserID)
		if user != nil {
			if senderDisplayName == nil {
				senderDisplayName = &user.FullName
			}
			senderAvatarURL = user.AvatarURL
		}
	}

	// Pre-resolve @mentions for internal notes.
	var mentionedUserIDs []string
	if req.IsInternal && s.notificationService != nil && s.workspaceRepo != nil {
		ids, err := resolveMentionRecipients(ctx, s.workspaceRepo, workspaceID, strings.TrimSpace(req.Content), derefString(senderUserID), nil)
		if err != nil {
			slog.ErrorContext(ctx, "resolve support mentions", "error", err, "conversation_id", ticketID)
		}
		mentionedUserIDs = ids
	}

	msg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    ticketID,
		SenderType:        senderType,
		SenderUserID:      senderUserID,
		SenderAgentID:     senderAgentID,
		SenderDisplayName: senderDisplayName,
		SenderAvatarURL:   senderAvatarURL,
		Content:           strings.TrimSpace(req.Content),
		IsInternal:        req.IsInternal,
		MessageType:       messageType,
	}

	if len(mentionedUserIDs) > 0 {
		metaJSON, _ := json.Marshal(map[string]any{"mentioned_user_ids": mentionedUserIDs})
		msg.Metadata = string(metaJSON)
	}

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}

	// Link pre-uploaded attachments to this message.
	if s.attachmentService != nil && len(req.AttachmentIDs) > 0 {
		if err := s.attachmentService.LinkToMessage(ctx, req.AttachmentIDs, msg.ID); err != nil {
			slog.ErrorContext(ctx, "link attachments to support message", "error", err, "message_id", msg.ID)
		}
		// Hydrate for WS broadcast.
		msgs := []model.SupportMessage{*msg}
		if err := s.attachmentService.HydrateMessages(ctx, msgs); err == nil {
			msg.Attachments = msgs[0].Attachments
		}
	}

	s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, msg, derefString(senderUserID)))

	var conv *model.SupportConversation
	if len(mentionedUserIDs) > 0 || (!msg.IsInternal && msg.MessageType == "reply") {
		conv, _ = s.conversationRepo.GetByID(ctx, workspaceID, ticketID)
	}

	// Emit mention notifications after message creation.
	if len(mentionedUserIDs) > 0 {
		ProcessSupportMentions(ctx, s.notificationService, conv, msg.Content, derefString(senderUserID), mentionedUserIDs)
	}

	previousOwnerID := ""
	if conv != nil && conv.OpenedByUserID != nil {
		previousOwnerID = strings.TrimSpace(*conv.OpenedByUserID)
	}

	if !msg.IsInternal && msg.MessageType == "reply" && msg.SenderType == "customer" {
		senderName := derefString(msg.SenderDisplayName)
		ProcessSupportCustomerReplyNotification(ctx, s.notificationService, conv, msg.Content, senderName)
	}

	if !msg.IsInternal && msg.MessageType == "reply" && msg.SenderType == "user" && senderUserID != nil && conv != nil {
		if conv.OpenedByUserID == nil || *conv.OpenedByUserID != *senderUserID {
			conv.OpenedByUserID = senderUserID
			if err := s.conversationRepo.Update(ctx, conv); err != nil {
				slog.ErrorContext(ctx, "failed to set support conversation owner", "error", err, "conversation_id", ticketID)
			}
		}
	}

	if !msg.IsInternal && msg.MessageType == "reply" && msg.SenderType != "customer" && previousOwnerID != "" && s.notificationService != nil {
		if err := s.notificationService.MarkEntityCategoryAsRead(ctx, previousOwnerID, workspaceID, "support_conversation", ticketID, model.NotifCategorySupportReplies); err != nil {
			slog.ErrorContext(ctx, "mark support reply notifications handled after teammate reply", "error", err, "conversation_id", ticketID, "recipient_id", previousOwnerID)
		}
	}

	// Push visitor-scoped list refresh for widget unread when an agent/user/AI reply is sent.
	if !msg.IsInternal && msg.SenderType != "customer" && msg.MessageType == "reply" && conv != nil {
		if conv.AnonymousID != nil && *conv.AnonymousID != "" {
			s.pushVisitorConversationsRefresh(ctx, workspaceID, *conv.AnonymousID)
		}
		if s.emailFallbackService != nil {
			go func(convSnapshot *model.SupportConversation) {
				if err := s.emailFallbackService.OnAgentReply(context.WithoutCancel(ctx), workspaceID, msg, convSnapshot); err != nil {
					slog.ErrorContext(ctx, "enqueue email fallback failed", "conversation_id", ticketID, "message_id", msg.ID, "error", err)
				}
			}(conv)
		}
	}

	return msg, nil
}

// LinkConversationStory links a conversation to a story.
func (s *SupportInboxService) LinkConversationStory(ctx context.Context, workspaceID, ticketID, storyID, actorID string) error {
	ticket, err := s.conversationRepo.GetByID(ctx, workspaceID, ticketID)
	if err != nil {
		return err
	}
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	ticket.LinkedStoryID = &storyID
	if err := s.conversationRepo.Update(ctx, ticket); err != nil {
		return err
	}

	assoc := &model.CRMAssociation{
		WorkspaceID:    workspaceID,
		FromObjectType: model.CRMObjectSupportConversation,
		FromObjectID:   ticketID,
		ToObjectType:   model.CRMObjectStory,
		ToObjectID:     storyID,
	}
	if err := s.assocRepo.Create(ctx, assoc); err != nil {
		return err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", ticketID, &actorID, "updated", strPtr("linked_story_id"), nil, &storyID, nil)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    ticketID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return nil
}

// AssignConversationAgent assigns an agent to a conversation.
func (s *SupportInboxService) AssignConversationAgent(ctx context.Context, workspaceID, ticketID, agentID, actorID string) error {
	return s.assignConversationAgent(ctx, workspaceID, ticketID, agentID, &actorID)
}

// ListContactConversations returns support conversations linked to a CRM contact.
func (s *SupportInboxService) ListContactConversations(ctx context.Context, workspaceID, contactID string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	return s.conversationRepo.ListByContact(ctx, workspaceID, contactID, pagination)
}

// matchOrCreateCRMContact looks up a CRM contact by email; if not found,
// creates one as lead with source=live_chat. Always promotes subscriber→lead.
func (s *SupportInboxService) matchOrCreateCRMContact(ctx context.Context, workspaceID string, email, name *string) *string {
	return s.matchOrCreateCRMContactTx(ctx, s.contactRepo, workspaceID, email, name, "widget_prechat")
}

// matchOrCreateCRMContactTx is the transactional version of matchOrCreateCRMContact.
// It uses the provided contactRepo (which may be wrapped in a transaction).
// The source param controls lifecycle promotion: "identify" promotes lead→customer.
func (s *SupportInboxService) matchOrCreateCRMContactTx(ctx context.Context, contactRepo *repository.CRMContactRepository, workspaceID string, email, name *string, source string) *string {
	if email == nil || *email == "" {
		return nil
	}

	trimmedEmail := strings.TrimSpace(*email)

	// Look up existing contact by exact email match
	existing, err := contactRepo.GetByEmail(ctx, workspaceID, trimmedEmail)
	if err != nil {
		slog.ErrorContext(ctx, "CRM contact lookup failed", "error", err, "workspace_id", workspaceID)
		return nil
	}

	if existing != nil {
		// Promote lifecycle stage if appropriate (never downgrade)
		promoted := s.promoteContactLifecycle(ctx, contactRepo, existing, source)
		if promoted {
			slog.InfoContext(ctx, "promoted CRM contact lifecycle",
				"contact_id", existing.ID, "stage", existing.LifecycleStage, "source", source)
		}
		return &existing.ID
	}

	// Auto-create new contact as lead with source=live_chat
	firstName := "Unknown"
	if name != nil && *name != "" {
		firstName = *name
	}
	contactSource := "live_chat"
	contact := &model.CRMContact{
		WorkspaceID:    workspaceID,
		FirstName:      firstName,
		Email:          &trimmedEmail,
		LifecycleStage: model.CRMLifecycleLead,
		LeadStatus:     model.CRMLeadStatusNew,
		Source:         &contactSource,
	}
	displayID, err := contactRepo.GetNextDisplayID(ctx, workspaceID)
	if err != nil {
		slog.ErrorContext(ctx, "get next display ID for CRM contact", "error", err, "workspace_id", workspaceID)
		return nil
	}
	contact.DisplayID = displayID
	if err := contactRepo.Create(ctx, contact); err != nil {
		slog.ErrorContext(ctx, "auto-create CRM contact from widget", "error", err, "workspace_id", workspaceID)
		return nil
	}

	// If source is "identify", promote new lead → customer
	if source == "sdk_identify" {
		s.promoteContactLifecycle(ctx, contactRepo, contact, source)
	}

	slog.InfoContext(ctx, "auto-created CRM lead from widget",
		"contact_id", contact.ID, "workspace_id", workspaceID, "source", contactSource)
	return &contact.ID
}

// promoteContactLifecycle promotes a CRM contact's lifecycle stage based on the event source.
// subscriber → lead (always), lead → customer (only on "identify" source).
// Never downgrades.
func (s *SupportInboxService) promoteContactLifecycle(ctx context.Context, contactRepo *repository.CRMContactRepository, contact *model.CRMContact, source string) bool {
	var targetStage string

	switch {
	case contact.LifecycleStage == model.CRMLifecycleSubscriber:
		// Always promote subscriber → lead
		targetStage = model.CRMLifecycleLead
	case contact.LifecycleStage == model.CRMLifecycleLead && source == "sdk_identify":
		// SDK identify() promotes lead → customer
		targetStage = model.CRMLifecycleCustomer
	default:
		return false
	}

	// Guard: never downgrade
	if model.CRMLifecycleIsHigherOrEqual(contact.LifecycleStage, targetStage) {
		return false
	}

	contact.LifecycleStage = targetStage
	if err := contactRepo.Update(ctx, contact); err != nil {
		slog.ErrorContext(ctx, "promote CRM contact lifecycle", "error", err, "contact_id", contact.ID)
		return false
	}
	return true
}

func generateSecureToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// ListConversationsWithMentions returns conversations where the given user was mentioned.
func (s *SupportInboxService) ListConversationsWithMentions(ctx context.Context, workspaceID, userID string) (*model.ConversationListResponse, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	ids, err := s.conversationRepo.ListConversationIDsWithMentions(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return &model.ConversationListResponse{
			Data: []model.SupportConversation{},
		}, nil
	}
	conversations, err := s.conversationRepo.ListByIDs(ctx, workspaceID, ids)
	if err != nil {
		return nil, err
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	return &model.ConversationListResponse{
		Data:       conversations,
		Total:      len(conversations),
		Page:       1,
		PerPage:    len(conversations),
		TotalPages: 1,
		Meta:       model.ConversationListMeta{},
	}, nil
}

func (s *SupportInboxService) assignConversationAgent(ctx context.Context, workspaceID, conversationID, agentID string, actorID *string) error {
	ticket, err := s.conversationRepo.GetByID(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	agent, err := s.agentRepo.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	if err := validateAgentTarget(agent, "support_conversation"); err != nil {
		return err
	}

	ticket.AssignedAgentID = &agentID
	if err := s.conversationRepo.Update(ctx, ticket); err != nil {
		return err
	}

	if s.activitySvc != nil {
		_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversationID, actorID, "updated", strPtr("assigned_agent_id"), nil, &agentID, nil)
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     derefString(actorID),
	})

	return nil
}
