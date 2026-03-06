package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// SupportService contains support business logic.
type SupportService struct {
	ticketRepo  *repository.SupportTicketRepository
	messageRepo *repository.SupportMessageRepository
	widgetRepo  *repository.WidgetInstallationRepository
	sessionRepo *repository.WidgetSessionRepository
	activitySvc *PMActivityService
	wsPublisher *websocket.Publisher
}

// NewSupportService creates a new SupportService.
func NewSupportService(
	ticketRepo *repository.SupportTicketRepository,
	messageRepo *repository.SupportMessageRepository,
	widgetRepo *repository.WidgetInstallationRepository,
	sessionRepo *repository.WidgetSessionRepository,
	activitySvc *PMActivityService,
	wsPublisher *websocket.Publisher,
) *SupportService {
	return &SupportService{
		ticketRepo:  ticketRepo,
		messageRepo: messageRepo,
		widgetRepo:  widgetRepo,
		sessionRepo: sessionRepo,
		activitySvc: activitySvc,
		wsPublisher: wsPublisher,
	}
}

// ListTickets returns tickets with optional filters.
func (s *SupportService) ListTickets(ctx context.Context, workspaceID, status, priority string, pagination model.PMPagination) ([]model.SupportTicket, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.ticketRepo.List(ctx, workspaceID, status, priority, pagination)
}

// GetTicket returns a single ticket.
func (s *SupportService) GetTicket(ctx context.Context, workspaceID, id string) (*model.SupportTicket, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket not found")
	}
	return ticket, nil
}

// CreateTicket creates a new support ticket.
func (s *SupportService) CreateTicket(ctx context.Context, req model.CreateTicketRequest, actorID string) (*model.SupportTicket, error) {
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

	ticket := &model.SupportTicket{
		WorkspaceID:    req.WorkspaceID,
		Subject:        strings.TrimSpace(req.Subject),
		Status:         "open",
		Priority:       priority,
		CustomerName:   req.CustomerName,
		CustomerEmail:  req.CustomerEmail,
		OpenedByUserID: &actorID,
		Source:         source,
	}

	if err := s.ticketRepo.Create(ctx, ticket); err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, ticket.WorkspaceID, "support_ticket", ticket.ID, &actorID, "created", nil, nil, &ticket.Subject, nil)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "support_ticket",
		EntityID:    ticket.ID,
		WorkspaceID: ticket.WorkspaceID,
		ActorID:     actorID,
	})

	return ticket, nil
}

// UpdateTicketStatus changes ticket status.
func (s *SupportService) UpdateTicketStatus(ctx context.Context, workspaceID, ticketID, status, actorID string) (*model.SupportTicket, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, workspaceID, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket not found")
	}

	oldStatus := ticket.Status
	ticket.Status = status

	if err := s.ticketRepo.Update(ctx, ticket); err != nil {
		return nil, err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "support_ticket", ticketID, &actorID, "updated", strPtr("status"), &oldStatus, &status, nil)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_ticket",
		EntityID:    ticketID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return ticket, nil
}

// ListMessages returns messages for a ticket.
func (s *SupportService) ListMessages(ctx context.Context, workspaceID, ticketID string, includeInternal bool) ([]model.SupportMessage, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.messageRepo.ListByTicket(ctx, workspaceID, ticketID, includeInternal)
}

// CreateMessage creates a message on a ticket.
func (s *SupportService) CreateMessage(ctx context.Context, workspaceID, ticketID string, req model.CreateMessageRequest, senderType string, senderUserID, senderAgentID *string, senderDisplayName *string) (*model.SupportMessage, error) {
	if strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("content is required")
	}

	msg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		TicketID:          ticketID,
		SenderType:        senderType,
		SenderUserID:      senderUserID,
		SenderAgentID:     senderAgentID,
		SenderDisplayName: senderDisplayName,
		Content:           strings.TrimSpace(req.Content),
		IsInternal:        req.IsInternal,
	}

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "support_message",
		EntityID:    msg.ID,
		WorkspaceID: workspaceID,
		ParentType:  "support_ticket",
		ParentID:    ticketID,
	})

	return msg, nil
}

// LinkStory links a ticket to a story.
func (s *SupportService) LinkStory(ctx context.Context, workspaceID, ticketID, storyID, actorID string) error {
	ticket, err := s.ticketRepo.GetByID(ctx, workspaceID, ticketID)
	if err != nil {
		return err
	}
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	ticket.LinkedStoryID = &storyID
	if err := s.ticketRepo.Update(ctx, ticket); err != nil {
		return err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "support_ticket", ticketID, &actorID, "updated", strPtr("linked_story_id"), nil, &storyID, nil)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_ticket",
		EntityID:    ticketID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return nil
}

// AssignAgent assigns an agent to a ticket.
func (s *SupportService) AssignAgent(ctx context.Context, workspaceID, ticketID, agentID, actorID string) error {
	ticket, err := s.ticketRepo.GetByID(ctx, workspaceID, ticketID)
	if err != nil {
		return err
	}
	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	ticket.AssignedAgentID = &agentID
	if err := s.ticketRepo.Update(ctx, ticket); err != nil {
		return err
	}

	_ = s.activitySvc.Log(ctx, workspaceID, "support_ticket", ticketID, &actorID, "updated", strPtr("assigned_agent_id"), nil, &agentID, nil)

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_ticket",
		EntityID:    ticketID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	return nil
}

// --- Widget methods ---

// CreateWidgetSession creates a new session for external widget chat.
func (s *SupportService) CreateWidgetSession(ctx context.Context, widgetKey string, customerName, customerEmail *string) (*model.SupportWidgetSession, error) {
	inst, err := s.widgetRepo.GetByWidgetKey(ctx, widgetKey)
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

	session := &model.SupportWidgetSession{
		WorkspaceID:   inst.WorkspaceID,
		SessionToken:  token,
		CustomerName:  customerName,
		CustomerEmail: customerEmail,
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

// GetWidgetSession returns a valid session by token.
func (s *SupportService) GetWidgetSession(ctx context.Context, token string) (*model.SupportWidgetSession, error) {
	session, err := s.sessionRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired")
	}
	return session, nil
}

// WidgetCreateMessage creates a message from an external widget user.
func (s *SupportService) WidgetCreateMessage(ctx context.Context, sessionToken, content string) (*model.SupportMessage, error) {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return nil, err
	}

	// If no ticket yet, create one.
	if session.TicketID == nil {
		ticket := &model.SupportTicket{
			WorkspaceID:   session.WorkspaceID,
			Subject:       truncate(content, 100),
			Status:        "open",
			Priority:      "medium",
			CustomerName:  session.CustomerName,
			CustomerEmail: session.CustomerEmail,
			Source:        "widget",
		}
		if err := s.ticketRepo.Create(ctx, ticket); err != nil {
			return nil, err
		}
		session.TicketID = &ticket.ID
		if err := s.sessionRepo.Update(ctx, session); err != nil {
			return nil, err
		}

		s.wsPublisher.Publish(websocket.Event{
			Action:      "created",
			Entity:      "support_ticket",
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
		TicketID:          *session.TicketID,
		SenderType:        "customer",
		SenderDisplayName: &displayName,
		Content:           strings.TrimSpace(content),
		IsInternal:        false,
	}

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "support_message",
		EntityID:    msg.ID,
		WorkspaceID: session.WorkspaceID,
		ParentType:  "support_ticket",
		ParentID:    *session.TicketID,
	})

	return msg, nil
}

// GetWidgetConfig returns widget config by widget key (public).
func (s *SupportService) GetWidgetConfig(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, error) {
	return s.widgetRepo.GetByWidgetKey(ctx, widgetKey)
}

func generateSecureToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
