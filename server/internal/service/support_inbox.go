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
	conversationRepo   *repository.SupportConversationRepository
	messageRepo        *repository.SupportMessageRepository
	agentRepo          *repository.AgentRepository
	assocRepo          *repository.CRMAssociationRepository
	installationRepo   *repository.SupportInboxInstallationRepository
	sessionRepo        *repository.SupportInboxSessionRepository
	cannedResponseRepo *repository.SupportCannedResponseRepository
	activitySvc        *PMActivityService
	wsPublisher        *websocket.Publisher
	contactRepo        *repository.CRMContactRepository
	userRepo           *repository.UserRepository
	docsSpaceRepo      *repository.DocsSpaceRepository
	docsCollectionRepo *repository.DocsCollectionRepository
	docsHelpcenterRepo *repository.DocsHelpcenterRepository
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

// ListConversations returns conversations with optional filters.
func (s *SupportInboxService) ListConversations(ctx context.Context, workspaceID, status, priority string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.conversationRepo.List(ctx, workspaceID, status, priority, pagination)
}

// ListConversationsWithMeta returns conversations plus aggregate unread stats.
func (s *SupportInboxService) ListConversationsWithMeta(ctx context.Context, workspaceID, userID, status, priority string, pagination model.PMPagination) (*model.ConversationListResponse, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	conversations, total, err := s.conversationRepo.List(ctx, workspaceID, status, priority, pagination)
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
	return nil
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
	return s.messageRepo.ListByConversation(ctx, workspaceID, ticketID, includeInternal)
}

// CreateConversationMessage creates a message on a conversation.
func (s *SupportInboxService) CreateConversationMessage(ctx context.Context, workspaceID, ticketID string, req model.CreateMessageRequest, senderType string, senderUserID, senderAgentID *string, senderDisplayName *string) (*model.SupportMessage, error) {
	if strings.TrimSpace(req.Content) == "" {
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

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}

	// IMPORTANT: Publish WS event only AFTER the DB write has succeeded.
	// Publishing before persist can cause phantom messages on other clients.
	hydratedJSON, _ := json.Marshal(model.WidgetMessageReceivedPayload{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		Content:        msg.Content,
		SenderType:     msg.SenderType,
		SenderName:     msg.SenderDisplayName,
		SenderAvatar:   msg.SenderAvatarURL,
		CreatedAt:      msg.CreatedAt.Format(time.RFC3339),
	})

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "support_conversation_message",
		EntityID:    msg.ID,
		WorkspaceID: workspaceID,
		ParentType:  "support_conversation",
		ParentID:    ticketID,
		Data:        hydratedJSON,
	})

	// Push visitor-scoped list refresh for widget unread when an agent/user/AI reply is sent.
	if !msg.IsInternal && msg.SenderType != "customer" && msg.MessageType == "reply" {
		conv, _ := s.conversationRepo.GetByID(ctx, workspaceID, ticketID)
		if conv != nil && conv.AnonymousID != nil && *conv.AnonymousID != "" {
			s.pushVisitorConversationsRefresh(ctx, workspaceID, *conv.AnonymousID)
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
	ticket, err := s.conversationRepo.GetByID(ctx, workspaceID, ticketID)
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
	if err := validateAgentTarget(agent, "support_ticket"); err != nil {
		return err
	}

	ticket.AssignedAgentID = &agentID
	if err := s.conversationRepo.Update(ctx, ticket); err != nil {
		return err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "support_conversation", ticketID, &actorID, "updated", strPtr("assigned_agent_id"), nil, &agentID, nil)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    ticketID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return nil
}

// ListContactConversations returns support conversations linked to a CRM contact.
func (s *SupportInboxService) ListContactConversations(ctx context.Context, workspaceID, contactID string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	return s.conversationRepo.ListByContact(ctx, workspaceID, contactID, pagination)
}

// matchOrCreateCRMContact looks up a CRM contact by email; if not found and
// we have a name, it auto-creates one with lifecycle_stage=subscriber, source=support.
func (s *SupportInboxService) matchOrCreateCRMContact(ctx context.Context, workspaceID string, email, name *string) *string {
	if email == nil || *email == "" {
		return nil
	}

	search := strings.TrimSpace(*email)
	contacts, _, err := s.contactRepo.List(ctx, workspaceID, model.CRMContactListFilters{
		Search: &search,
	}, model.PMPagination{Page: 1, PerPage: 1})
	if err == nil && len(contacts) > 0 {
		if contacts[0].Email != nil && strings.EqualFold(*contacts[0].Email, search) {
			return &contacts[0].ID
		}
	}

	// Auto-create contact if we have name + email.
	firstName := "Unknown"
	if name != nil && *name != "" {
		firstName = *name
	}
	source := "support"
	contact := &model.CRMContact{
		WorkspaceID:    workspaceID,
		FirstName:      firstName,
		Email:          email,
		LifecycleStage: model.CRMLifecycleSubscriber,
		LeadStatus:     model.CRMLeadStatusNew,
		Source:         &source,
	}
	displayID, err := s.contactRepo.GetNextDisplayID(ctx, workspaceID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get next display ID for CRM contact", "error", err, "workspace_id", workspaceID)
		return nil
	}
	contact.DisplayID = displayID
	if err := s.contactRepo.Create(ctx, contact); err != nil {
		slog.ErrorContext(ctx, "failed to auto-create CRM contact from support", "error", err, "workspace_id", workspaceID)
		return nil
	}
	slog.InfoContext(ctx, "auto-created CRM contact from support conversation", "contact_id", contact.ID, "workspace_id", workspaceID)
	return &contact.ID
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
