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
			AIEnabled:   settings.AIEnabled,
			FileUploads: false,
			PreChatForm: settings.RequireEmailBeforeChat,
			CSATRating:  settings.CSATEnabled,
		},
		HelpSpaces: helpSpaces,
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
	_, allowedSpaces, err := s.getAllowedWidgetHelpSpaces(ctx, widgetKey)
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

	return &model.WidgetHelpArticle{
		ID:          doc.ID,
		Title:       doc.Title,
		Slug:        doc.ID,
		Excerpt:     doc.Excerpt,
		Icon:        doc.Icon,
		ContentHTML: renderWidgetArticleHTML(contentJSON),
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
