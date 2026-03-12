package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
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
	}
}

// ListConversations returns conversations with optional filters.
func (s *SupportInboxService) ListConversations(ctx context.Context, workspaceID, status, priority string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.conversationRepo.List(ctx, workspaceID, status, priority, pagination)
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

	msg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    ticketID,
		SenderType:        senderType,
		SenderUserID:      senderUserID,
		SenderAgentID:     senderAgentID,
		SenderDisplayName: senderDisplayName,
		Content:           strings.TrimSpace(req.Content),
		IsInternal:        req.IsInternal,
		MessageType:       messageType,
	}

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "support_conversation_message",
		EntityID:    msg.ID,
		WorkspaceID: workspaceID,
		ParentType:  "support_conversation",
		ParentID:    ticketID,
	})

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

// --- Widget methods ---

// CreateWidgetSession creates a new session for external widget chat.
func (s *SupportInboxService) CreateWidgetSession(ctx context.Context, widgetKey string, customerName, customerEmail *string) (*model.SupportWidgetSession, error) {
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
func (s *SupportInboxService) GetWidgetSession(ctx context.Context, token string) (*model.SupportWidgetSession, error) {
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
func (s *SupportInboxService) WidgetCreateMessage(ctx context.Context, sessionToken, content string) (*model.SupportMessage, error) {
	session, err := s.GetWidgetSession(ctx, sessionToken)
	if err != nil {
		return nil, err
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

	s.wsPublisher.Publish(websocket.Event{
		Action:      "created",
		Entity:      "support_conversation_message",
		EntityID:    msg.ID,
		WorkspaceID: session.WorkspaceID,
		ParentType:  "support_conversation",
		ParentID:    *session.ConversationID,
	})

	return msg, nil
}

// GetWidgetConfig returns widget config by widget key (public).
func (s *SupportInboxService) GetWidgetConfig(ctx context.Context, widgetKey string) (*model.SupportWidgetInstallation, error) {
	return s.installationRepo.GetByWidgetKey(ctx, widgetKey)
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

// ListContactConversations returns support conversations linked to a CRM contact.
func (s *SupportInboxService) ListContactConversations(ctx context.Context, workspaceID, contactID string, pagination model.PMPagination) ([]model.SupportConversation, int64, error) {
	return s.conversationRepo.ListByContact(ctx, workspaceID, contactID, pagination)
}

// --- Installation settings methods ---

// parseSettings unmarshals the JSONB settings string, applying defaults for missing fields.
func parseSettings(raw string) model.SupportInboxSettings {
	defaults := model.DefaultSupportInboxSettings()
	if raw == "" || raw == "{}" {
		return defaults
	}
	if err := json.Unmarshal([]byte(raw), &defaults); err != nil {
		return model.DefaultSupportInboxSettings()
	}
	return defaults
}

// mergeSettingsUpdate applies non-nil patch fields onto current settings.
func mergeSettingsUpdate(current model.SupportInboxSettings, patch model.UpdateInstallationSettingsRequest) model.SupportInboxSettings {
	if patch.RequireEmailBeforeChat != nil {
		current.RequireEmailBeforeChat = *patch.RequireEmailBeforeChat
	}
	if patch.RequireNameAfterEmail != nil {
		current.RequireNameAfterEmail = *patch.RequireNameAfterEmail
	}
	if patch.WelcomeMessage != nil {
		current.WelcomeMessage = *patch.WelcomeMessage
	}
	if patch.AutoCreateCRMContact != nil {
		current.AutoCreateCRMContact = *patch.AutoCreateCRMContact
	}
	if patch.DefaultLifecycleStage != nil {
		current.DefaultLifecycleStage = *patch.DefaultLifecycleStage
	}
	if patch.AutoPromoteToLead != nil {
		current.AutoPromoteToLead = *patch.AutoPromoteToLead
	}
	if patch.AIEnabled != nil {
		current.AIEnabled = *patch.AIEnabled
	}
	if patch.AIConfidenceThreshold != nil {
		current.AIConfidenceThreshold = *patch.AIConfidenceThreshold
	}
	if patch.ShowTalkToHuman != nil {
		current.ShowTalkToHuman = *patch.ShowTalkToHuman
	}
	if patch.HandoffBehavior != nil {
		current.HandoffBehavior = *patch.HandoffBehavior
	}
	if patch.HandoffTeamID != nil {
		current.HandoffTeamID = patch.HandoffTeamID
	}
	if patch.BusinessHoursEnabled != nil {
		current.BusinessHoursEnabled = *patch.BusinessHoursEnabled
	}
	if patch.BusinessHoursTimezone != nil {
		current.BusinessHoursTimezone = *patch.BusinessHoursTimezone
	}
	if patch.BusinessHoursSchedule != nil {
		current.BusinessHoursSchedule = patch.BusinessHoursSchedule
	}
	if patch.OutsideHoursMessage != nil {
		current.OutsideHoursMessage = *patch.OutsideHoursMessage
	}
	if patch.BrandColor != nil {
		current.BrandColor = *patch.BrandColor
	}
	if patch.ShowBranding != nil {
		current.ShowBranding = *patch.ShowBranding
	}
	if patch.LauncherPosition != nil {
		current.LauncherPosition = *patch.LauncherPosition
	}
	if patch.LauncherIcon != nil {
		current.LauncherIcon = *patch.LauncherIcon
	}
	if patch.CSATEnabled != nil {
		current.CSATEnabled = *patch.CSATEnabled
	}
	return current
}

var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validateSettings checks settings field constraints.
func validateSettings(s model.SupportInboxSettings) error {
	if s.AIConfidenceThreshold < 0 || s.AIConfidenceThreshold > 1 {
		return fmt.Errorf("ai_confidence_threshold must be between 0.0 and 1.0")
	}
	if s.BrandColor != "" && !hexColorRegex.MatchString(s.BrandColor) {
		return fmt.Errorf("brand_color must be a valid hex color (e.g. #6366F1)")
	}
	validHandoff := map[string]bool{"unassigned": true, "assign_to_team": true, "round_robin": true}
	if !validHandoff[s.HandoffBehavior] {
		return fmt.Errorf("handoff_behavior must be unassigned, assign_to_team, or round_robin")
	}
	if s.HandoffBehavior == "assign_to_team" && (s.HandoffTeamID == nil || *s.HandoffTeamID == "") {
		return fmt.Errorf("handoff_team_id is required when handoff_behavior is assign_to_team")
	}
	validPosition := map[string]bool{"bottom_right": true, "bottom_left": true}
	if !validPosition[s.LauncherPosition] {
		return fmt.Errorf("launcher_position must be bottom_right or bottom_left")
	}
	validIcon := map[string]bool{"chat_bubble": true, "question_mark": true, "help": true}
	if !validIcon[s.LauncherIcon] {
		return fmt.Errorf("launcher_icon must be chat_bubble, question_mark, or help")
	}
	validLifecycle := map[string]bool{"subscriber": true, "lead": true, "opportunity": true}
	if !validLifecycle[s.DefaultLifecycleStage] {
		return fmt.Errorf("default_lifecycle_stage must be subscriber, lead, or opportunity")
	}
	return nil
}

// isOnline computes whether the widget is currently within business hours.
func isOnline(s model.SupportInboxSettings) bool {
	if !s.BusinessHoursEnabled {
		return true // always online when business hours not configured
	}

	loc, err := time.LoadLocation(s.BusinessHoursTimezone)
	if err != nil {
		return true // fallback to online if timezone invalid
	}

	now := time.Now().In(loc)
	dayNames := map[time.Weekday]string{
		time.Monday: "mon", time.Tuesday: "tue", time.Wednesday: "wed",
		time.Thursday: "thu", time.Friday: "fri", time.Saturday: "sat", time.Sunday: "sun",
	}
	dayKey := dayNames[now.Weekday()]
	day, ok := s.BusinessHoursSchedule[dayKey]
	if !ok || !day.Enabled {
		return false
	}

	currentMinutes := now.Hour()*60 + now.Minute()
	startMinutes := parseTimeToMinutes(day.Start)
	endMinutes := parseTimeToMinutes(day.End)
	return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

func parseTimeToMinutes(t string) int {
	var h, m int
	fmt.Sscanf(t, "%d:%d", &h, &m)
	return h*60 + m
}

// GetInstallation returns the installation and its parsed settings for a workspace.
func (s *SupportInboxService) GetInstallation(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	if inst == nil {
		return nil, nil, fmt.Errorf("no widget installation found for this workspace")
	}
	settings := parseSettings(inst.Settings)
	return inst, &settings, nil
}

// UpdateInstallationSettings merges, validates, and saves settings.
func (s *SupportInboxService) UpdateInstallationSettings(ctx context.Context, workspaceID string, req model.UpdateInstallationSettingsRequest) (*model.SupportWidgetInstallation, *model.SupportInboxSettings, error) {
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	if inst == nil {
		return nil, nil, fmt.Errorf("no widget installation found for this workspace")
	}

	current := parseSettings(inst.Settings)
	merged := mergeSettingsUpdate(current, req)
	if err := validateSettings(merged); err != nil {
		return nil, nil, err
	}

	raw, err := json.Marshal(merged)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal settings: %w", err)
	}
	inst.Settings = string(raw)

	if err := s.installationRepo.Update(ctx, inst); err != nil {
		return nil, nil, err
	}

	slog.InfoContext(ctx, "updated support installation settings", "workspace_id", workspaceID)
	return inst, &merged, nil
}

// RegenerateWidgetKey generates a new widget key + secret key.
func (s *SupportInboxService) RegenerateWidgetKey(ctx context.Context, workspaceID string) (*model.SupportWidgetInstallation, error) {
	inst, err := s.installationRepo.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("no widget installation found for this workspace")
	}

	newWidgetKey, err := generateSecureToken(16)
	if err != nil {
		return nil, fmt.Errorf("generate widget key: %w", err)
	}
	newSecretKey, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate secret key: %w", err)
	}

	if err := s.installationRepo.RegenerateKeys(ctx, inst.ID, newWidgetKey, newSecretKey); err != nil {
		return nil, err
	}

	inst.WidgetKey = newWidgetKey
	inst.SecretKey = newSecretKey

	slog.InfoContext(ctx, "regenerated support widget keys", "workspace_id", workspaceID, "installation_id", inst.ID)
	return inst, nil
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
		BrandColor:             settings.BrandColor,
		ShowBranding:           settings.ShowBranding,
		LauncherPosition:       settings.LauncherPosition,
		LauncherIcon:           settings.LauncherIcon,
		CSATEnabled:            settings.CSATEnabled,
	}, nil
}

// ListCannedResponses returns all canned responses for a workspace.
func (s *SupportInboxService) ListCannedResponses(ctx context.Context, workspaceID string) ([]model.SupportCannedResponse, error) {
	return s.cannedResponseRepo.List(ctx, workspaceID)
}

// SearchCannedResponses returns canned responses matching a query.
func (s *SupportInboxService) SearchCannedResponses(ctx context.Context, workspaceID, query string) ([]model.SupportCannedResponse, error) {
	return s.cannedResponseRepo.Search(ctx, workspaceID, query)
}

// CreateCannedResponse creates a new canned response.
func (s *SupportInboxService) CreateCannedResponse(ctx context.Context, workspaceID string, req model.CannedResponseRequest, createdByID string) (*model.SupportCannedResponse, error) {
	if strings.TrimSpace(req.ShortCode) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("short_code, title, and content are required")
	}

	response := &model.SupportCannedResponse{
		WorkspaceID: workspaceID,
		ShortCode:   req.ShortCode,
		Title:       req.Title,
		Content:     req.Content,
		CreatedByID: createdByID,
	}
	if err := s.cannedResponseRepo.Create(ctx, response); err != nil {
		return nil, fmt.Errorf("create canned response: %w", err)
	}
	return response, nil
}

// UpdateCannedResponse updates an existing canned response.
func (s *SupportInboxService) UpdateCannedResponse(ctx context.Context, workspaceID, id string, req model.CannedResponseRequest) (*model.SupportCannedResponse, error) {
	response, err := s.cannedResponseRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, fmt.Errorf("canned response not found")
	}

	response.ShortCode = req.ShortCode
	response.Title = req.Title
	response.Content = req.Content

	if err := s.cannedResponseRepo.Update(ctx, response); err != nil {
		return nil, fmt.Errorf("update canned response: %w", err)
	}
	return response, nil
}

// DeleteCannedResponse deletes a canned response.
func (s *SupportInboxService) DeleteCannedResponse(ctx context.Context, workspaceID, id string) error {
	return s.cannedResponseRepo.Delete(ctx, workspaceID, id)
}

// PublishTypingIndicator publishes a typing indicator event via WebSocket.
func (s *SupportInboxService) PublishTypingIndicator(ctx context.Context, workspaceID, conversationID string, isTyping bool) {
	action := "typing_stopped"
	if isTyping {
		action = "typing_started"
	}
	s.wsPublisher.Publish(websocket.Event{
		Action:      action,
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
	})
}
