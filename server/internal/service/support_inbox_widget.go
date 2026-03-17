package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// CreateWidgetSession creates a new session for external widget chat.
// Always creates a new session — multiple concurrent sessions per visitor are allowed.
func (s *SupportInboxService) CreateWidgetSession(ctx context.Context, widgetKey string, anonymousID string, customerName, customerEmail *string, userAgent, pageURL *string) (*model.SupportWidgetSession, error) {
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
	return s.conversationRepo.ListByAnonymousID(ctx, workspaceID, anonymousID)
}

// UpgradeWidgetSession upgrades an anonymous session with email and name.
func (s *SupportInboxService) UpgradeWidgetSession(ctx context.Context, sessionToken, email, name string) error {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return err
	}

	session.CustomerEmail = &email
	session.CustomerName = &name
	session.IsAnonymous = false

	if err := s.sessionRepo.Update(ctx, session); err != nil {
		return err
	}

	// Update conversation contact info if conversation exists
	if session.ConversationID != nil {
		conv, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, *session.ConversationID)
		if err == nil && conv != nil {
			conv.CustomerEmail = &email
			conv.CustomerName = &name
			_ = s.conversationRepo.Update(ctx, conv)
		}
	}

	// Auto-match or create CRM contact by email
	if contactID := s.matchOrCreateCRMContact(ctx, session.WorkspaceID, &email, &name); contactID != nil {
		if session.ConversationID != nil {
			conv, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, *session.ConversationID)
			if err == nil && conv != nil {
				conv.CRMContactID = contactID
				_ = s.conversationRepo.Update(ctx, conv)
			}
		}
	}

	slog.InfoContext(ctx, "widget session upgraded", "session_id", session.ID, "email", email)
	return nil
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

	conversation, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, conversationID)
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
		Priority:      "medium",
		CustomerName:  session.CustomerName,
		CustomerEmail: session.CustomerEmail,
		AnonymousID:   &session.AnonymousID,
		Source:        "widget",
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

	return ticket, nil
}

// WidgetCreateMessage creates a message from an external widget user.
func (s *SupportInboxService) WidgetCreateMessage(ctx context.Context, sessionToken, content string) (*model.SupportMessage, error) {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return nil, err
	}

	// Update conversation subject from first message if it was eagerly created with placeholder.
	if session.ConversationID != nil {
		conv, err := s.conversationRepo.GetByID(ctx, session.WorkspaceID, *session.ConversationID)
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
			Priority:      "medium",
			CustomerName:  session.CustomerName,
			CustomerEmail: session.CustomerEmail,
			AnonymousID:   &session.AnonymousID,
			Source:        "widget",
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
		CreatedAt:      msg.CreatedAt.Format(time.RFC3339),
	})

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "support_conversation_message",
		EntityID:    msg.ID,
		WorkspaceID: session.WorkspaceID,
		ActorID:     "widget:" + session.ID, // exclude sender from Hub broadcast (handler echoes directly)
		ParentType:  "support_conversation",
		ParentID:    *session.ConversationID,
		Data:        hydratedJSON,
	})

	return msg, nil
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

	settings := parseSettings(inst.Settings)
	return &model.WidgetConfigResponse{
		WidgetKey:              inst.WidgetKey,
		Active:                 inst.Active,
		IsOnline:               isOnline(settings),
		RequireEmailBeforeChat: settings.RequireEmailBeforeChat,
		RequireNameAfterEmail:  settings.RequireNameAfterEmail,
		WelcomeMessage:         settings.WelcomeMessage,
		AIEnabled:              settings.AIEnabled,
		ShowTalkToHuman:        settings.ShowTalkToHuman,
		BusinessHoursEnabled:   settings.BusinessHoursEnabled,
		OutsideHoursMessage:    settings.OutsideHoursMessage,
		WidgetName:             settings.WidgetName,
		WidgetAvatarURL:        settings.WidgetAvatarURL,
		BrandColor:             settings.BrandColor,
		ShowBranding:           settings.ShowBranding,
		ColorScheme:            settings.ColorScheme,
		ButtonColor:            settings.ButtonColor,
		ButtonIconColor:        settings.ButtonIconColor,
		LogoURL:                settings.LogoURL,
		LauncherPosition:       settings.LauncherPosition,
		LauncherIcon:           settings.LauncherIcon,
		CSATEnabled:            settings.CSATEnabled,
	}, nil
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

	settings := parseSettings(inst.Settings)
	return &model.WidgetConfigResponse{
		WidgetKey:              inst.WidgetKey,
		Active:                 inst.Active,
		IsOnline:               isOnline(settings),
		RequireEmailBeforeChat: settings.RequireEmailBeforeChat,
		RequireNameAfterEmail:  settings.RequireNameAfterEmail,
		WelcomeMessage:         settings.WelcomeMessage,
		AIEnabled:              settings.AIEnabled,
		ShowTalkToHuman:        settings.ShowTalkToHuman,
		BusinessHoursEnabled:   settings.BusinessHoursEnabled,
		OutsideHoursMessage:    settings.OutsideHoursMessage,
		WidgetName:             settings.WidgetName,
		WidgetAvatarURL:        settings.WidgetAvatarURL,
		BrandColor:             settings.BrandColor,
		ShowBranding:           settings.ShowBranding,
		ColorScheme:            settings.ColorScheme,
		ButtonColor:            settings.ButtonColor,
		ButtonIconColor:        settings.ButtonIconColor,
		LogoURL:                settings.LogoURL,
		LauncherPosition:       settings.LauncherPosition,
		LauncherIcon:           settings.LauncherIcon,
		CSATEnabled:            settings.CSATEnabled,
	}, nil
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
