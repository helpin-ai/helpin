package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type supportConversationMoveOptions struct {
	EnforceMailboxAccess bool
	UseAccessibleLoad    bool
	RecordTriageFeedback bool
}

func normalizeSupportMailboxHandle(handle string) string {
	handle = strings.ToLower(strings.TrimSpace(handle))
	handle = strings.ReplaceAll(handle, " ", "-")
	return handle
}

func (s *SupportInboxService) isMailboxAccessible(ctx context.Context, workspaceID string, mailboxID *string) bool {
	if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
		return true
	}
	actor := supportActorFromContext(ctx, workspaceID)
	if actor == nil {
		return false
	}
	if actor.Role == model.RoleOwner || actor.Role == model.RoleAdmin {
		return true
	}
	if s.mailboxRepo == nil {
		return false
	}
	ok, err := s.mailboxRepo.IsMember(ctx, strings.TrimSpace(*mailboxID), actor.WorkspaceMemberID)
	return err == nil && ok
}

func (s *SupportInboxService) userCanAccessMailbox(ctx context.Context, workspaceID string, mailboxID *string, userID string) bool {
	if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
		return true
	}
	if s.workspaceRepo == nil || s.mailboxRepo == nil {
		return false
	}
	member, err := s.workspaceRepo.GetMembership(ctx, workspaceID, userID)
	if err != nil || member == nil {
		return false
	}
	if member.Role == model.RoleOwner || member.Role == model.RoleAdmin {
		return true
	}
	ok, err := s.mailboxRepo.IsMember(ctx, strings.TrimSpace(*mailboxID), member.ID)
	return err == nil && ok
}

func (s *SupportInboxService) requireMailboxAccess(ctx context.Context, workspaceID string, mailboxID *string) error {
	if s.isMailboxAccessible(ctx, workspaceID, mailboxID) {
		return nil
	}
	return fmt.Errorf("mailbox not found")
}

func (s *SupportInboxService) loadConversationAccessible(ctx context.Context, workspaceID, conversationID string) (*model.SupportConversation, error) {
	workspaceMemberID, role := s.actorMailboxScope(ctx, workspaceID)
	return s.conversationRepo.GetByID(ctx, workspaceID, conversationID, workspaceMemberID, role)
}

func (s *SupportInboxService) loadConversationUnscoped(ctx context.Context, workspaceID, conversationID string) (*model.SupportConversation, error) {
	return s.conversationRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
}

func (s *SupportInboxService) sanitizeMailboxSelection(ctx context.Context, workspaceID string, mailboxID *string) (*string, *model.SupportMailbox, error) {
	if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
		return nil, nil, nil
	}
	if s.mailboxRepo == nil {
		return nil, nil, fmt.Errorf("support mailbox repository is unavailable")
	}
	trimmed := strings.TrimSpace(*mailboxID)
	mailbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, trimmed)
	if err != nil {
		return nil, nil, err
	}
	if mailbox == nil || !mailbox.Active {
		return nil, nil, fmt.Errorf("mailbox not found")
	}
	return &trimmed, mailbox, nil
}

func (s *SupportInboxService) resolveDefaultMailbox(ctx context.Context, workspaceID string, settings *model.SupportInboxSettings) (*string, *model.SupportMailbox, error) {
	if settings == nil {
		return nil, nil, nil
	}
	return s.sanitizeMailboxSelection(ctx, workspaceID, settings.DefaultMailboxID)
}

func (s *SupportInboxService) resolveAIHandoffMailbox(ctx context.Context, workspaceID string, settings *model.SupportInboxSettings) (*string, *model.SupportMailbox, error) {
	if settings == nil {
		return nil, nil, nil
	}
	if settings.AIHandoffMailboxID != nil && strings.TrimSpace(*settings.AIHandoffMailboxID) != "" {
		return s.sanitizeMailboxSelection(ctx, workspaceID, settings.AIHandoffMailboxID)
	}
	return s.resolveDefaultMailbox(ctx, workspaceID, settings)
}

func (s *SupportInboxService) determineMailboxOwner(ctx context.Context, workspaceID string, mailbox *model.SupportMailbox, currentOwner *string) (*string, string, error) {
	if mailbox == nil {
		if currentOwner != nil && strings.TrimSpace(*currentOwner) != "" {
			return currentOwner, model.SupportConversationFlowStateAssignedToHuman, nil
		}
		return nil, model.SupportConversationFlowStateWaitingForHuman, nil
	}

	if mailbox.AssignmentMode == "round_robin" && s.mailboxRepo != nil {
		ownerID, err := s.mailboxRepo.SelectRoundRobinOwnerUserID(ctx, workspaceID, mailbox.ID)
		if err != nil {
			return nil, "", err
		}
		if ownerID != nil && strings.TrimSpace(*ownerID) != "" {
			return ownerID, model.SupportConversationFlowStateAssignedToHuman, nil
		}
	}

	if currentOwner != nil && strings.TrimSpace(*currentOwner) != "" && s.userCanAccessMailbox(ctx, workspaceID, &mailbox.ID, strings.TrimSpace(*currentOwner)) {
		return currentOwner, model.SupportConversationFlowStateAssignedToHuman, nil
	}

	return nil, model.SupportConversationFlowStateWaitingForHuman, nil
}

func (s *SupportInboxService) ListInboxScopes(ctx context.Context, workspaceID string) (*model.SupportInboxScopeListResponse, error) {
	actor := supportActorFromContext(ctx, workspaceID)
	if actor == nil {
		return nil, fmt.Errorf("authorization context missing")
	}
	if s.mailboxRepo == nil {
		return nil, fmt.Errorf("support mailbox repository is unavailable")
	}

	mailboxes, err := s.mailboxRepo.ListAccessible(ctx, workspaceID, actor.WorkspaceMemberID, actor.Role, false)
	if err != nil {
		return nil, err
	}

	inst, settings, err := s.GetInstallation(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	_ = inst

	sharedUnread, err := s.mailboxRepo.CountUnread(ctx, workspaceID, nil)
	if err != nil {
		return nil, err
	}
	sharedIsDefault := settings != nil && (settings.DefaultMailboxID == nil || strings.TrimSpace(derefString(settings.DefaultMailboxID)) == "")

	response := &model.SupportInboxScopeListResponse{
		SharedInbox: model.SupportInboxScope{
			ID:          "shared",
			Name:        "Shared Inbox",
			Handle:      "shared",
			Icon:        "inbox",
			IsShared:    true,
			IsDefault:   sharedIsDefault,
			UnreadCount: sharedUnread,
			Active:      true,
		},
		Mailboxes: make([]model.SupportInboxScope, 0, len(mailboxes)),
	}

	for _, mailbox := range mailboxes {
		mailboxID := mailbox.ID
		unreadCount, countErr := s.mailboxRepo.CountUnread(ctx, workspaceID, &mailboxID)
		if countErr != nil {
			return nil, countErr
		}
		response.Mailboxes = append(response.Mailboxes, model.SupportInboxScope{
			ID:           mailbox.ID,
			Name:         mailbox.Name,
			Handle:       mailbox.Handle,
			Icon:         mailbox.Icon,
			IsShared:     false,
			IsDefault:    settings != nil && settings.DefaultMailboxID != nil && strings.TrimSpace(*settings.DefaultMailboxID) == mailbox.ID,
			UnreadCount:  unreadCount,
			Active:       mailbox.Active,
			LinkedTeamID: mailbox.LinkedTeamID,
		})
	}

	return response, nil
}

func (s *SupportInboxService) ListMailboxesAdmin(ctx context.Context, workspaceID string) ([]model.SupportMailbox, error) {
	if s.mailboxRepo == nil {
		return nil, fmt.Errorf("support mailbox repository is unavailable")
	}
	return s.mailboxRepo.ListByWorkspace(ctx, workspaceID, true)
}

func (s *SupportInboxService) ListMailboxMembers(ctx context.Context, workspaceID, mailboxID string) ([]model.SupportMailboxMember, error) {
	if s.mailboxRepo == nil {
		return nil, fmt.Errorf("support mailbox repository is unavailable")
	}
	mailbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, mailboxID)
	if err != nil {
		return nil, err
	}
	if mailbox == nil {
		return nil, fmt.Errorf("mailbox not found")
	}
	return s.mailboxRepo.ListMembers(ctx, mailboxID)
}

func (s *SupportInboxService) CreateMailbox(ctx context.Context, workspaceID string, req model.CreateSupportMailboxRequest, actorID string) (*model.SupportMailbox, error) {
	if s.mailboxRepo == nil {
		return nil, fmt.Errorf("support mailbox repository is unavailable")
	}
	name := strings.TrimSpace(req.Name)
	handle := normalizeSupportMailboxHandle(req.Handle)
	icon := strings.TrimSpace(req.Icon)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if handle == "" {
		return nil, fmt.Errorf("handle is required")
	}
	if icon == "" {
		icon = "inbox"
	}
	if req.AssignmentMode == "" {
		req.AssignmentMode = "manual"
	}
	if req.AssignmentMode != "manual" && req.AssignmentMode != "round_robin" {
		return nil, fmt.Errorf("assignment_mode must be manual or round_robin")
	}
	if existing, err := s.mailboxRepo.GetByHandle(ctx, workspaceID, handle); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, fmt.Errorf("mailbox handle already exists")
	}

	mailbox := &model.SupportMailbox{
		WorkspaceID:    workspaceID,
		Name:           name,
		Handle:         handle,
		Icon:           icon,
		Description:    req.Description,
		RoutingPrompt:  req.RoutingPrompt,
		TriageEligible: true,
		LinkedTeamID:   req.LinkedTeamID,
		VisibilityMode: "members_only",
		AssignmentMode: req.AssignmentMode,
		Active:         true,
		CreatedByID:    actorID,
	}
	if req.TriageEligible != nil {
		mailbox.TriageEligible = *req.TriageEligible
	}
	if req.LinkedTeamID != nil && strings.TrimSpace(*req.LinkedTeamID) == "" {
		mailbox.LinkedTeamID = nil
	}
	if req.RoutingPrompt != nil && strings.TrimSpace(*req.RoutingPrompt) == "" {
		mailbox.RoutingPrompt = nil
	}

	if err := s.mailboxRepo.Create(ctx, mailbox); err != nil {
		return nil, err
	}

	if err := s.mailboxRepo.AddMembers(ctx, mailbox.ID, req.WorkspaceMemberIDs); err != nil {
		return nil, err
	}

	return s.mailboxRepo.GetByID(ctx, workspaceID, mailbox.ID)
}

func (s *SupportInboxService) UpdateMailbox(ctx context.Context, workspaceID, mailboxID string, req model.UpdateSupportMailboxRequest) (*model.SupportMailbox, error) {
	if s.mailboxRepo == nil {
		return nil, fmt.Errorf("support mailbox repository is unavailable")
	}
	mailbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, mailboxID)
	if err != nil {
		return nil, err
	}
	if mailbox == nil {
		return nil, fmt.Errorf("mailbox not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}
		mailbox.Name = name
	}
	if req.Handle != nil {
		handle := normalizeSupportMailboxHandle(*req.Handle)
		if handle == "" {
			return nil, fmt.Errorf("handle is required")
		}
		if existing, err := s.mailboxRepo.GetByHandle(ctx, workspaceID, handle); err != nil {
			return nil, err
		} else if existing != nil && existing.ID != mailbox.ID {
			return nil, fmt.Errorf("mailbox handle already exists")
		}
		mailbox.Handle = handle
	}
	if req.Icon != nil {
		icon := strings.TrimSpace(*req.Icon)
		if icon == "" {
			icon = "inbox"
		}
		mailbox.Icon = icon
	}
	if req.Description != nil {
		if strings.TrimSpace(*req.Description) == "" {
			mailbox.Description = nil
		} else {
			mailbox.Description = req.Description
		}
	}
	if req.RoutingPrompt != nil {
		if strings.TrimSpace(*req.RoutingPrompt) == "" {
			mailbox.RoutingPrompt = nil
		} else {
			trimmed := strings.TrimSpace(*req.RoutingPrompt)
			mailbox.RoutingPrompt = &trimmed
		}
	}
	if req.TriageEligible != nil {
		mailbox.TriageEligible = *req.TriageEligible
	}
	if req.LinkedTeamID != nil {
		if strings.TrimSpace(*req.LinkedTeamID) == "" {
			mailbox.LinkedTeamID = nil
		} else {
			trimmed := strings.TrimSpace(*req.LinkedTeamID)
			mailbox.LinkedTeamID = &trimmed
		}
	}
	if req.AssignmentMode != nil {
		mode := strings.TrimSpace(*req.AssignmentMode)
		if mode != "manual" && mode != "round_robin" {
			return nil, fmt.Errorf("assignment_mode must be manual or round_robin")
		}
		mailbox.AssignmentMode = mode
	}
	if err := s.mailboxRepo.Update(ctx, mailbox); err != nil {
		return nil, err
	}
	if req.WorkspaceMemberIDs != nil {
		if err := s.mailboxRepo.ReplaceMembers(ctx, mailbox.ID, req.WorkspaceMemberIDs); err != nil {
			return nil, err
		}
	}
	return s.mailboxRepo.GetByID(ctx, workspaceID, mailbox.ID)
}

func (s *SupportInboxService) ArchiveMailbox(ctx context.Context, workspaceID, mailboxID string) (*model.SupportMailbox, error) {
	if s.mailboxRepo == nil {
		return nil, fmt.Errorf("support mailbox repository is unavailable")
	}
	mailbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, mailboxID)
	if err != nil {
		return nil, err
	}
	if mailbox == nil {
		return nil, fmt.Errorf("mailbox not found")
	}
	mailbox.Active = false
	if err := s.mailboxRepo.Update(ctx, mailbox); err != nil {
		return nil, err
	}
	return mailbox, nil
}

func (s *SupportInboxService) ReorderMailboxes(ctx context.Context, workspaceID string, mailboxIDs []string) error {
	if s.mailboxRepo == nil {
		return fmt.Errorf("support mailbox repository is unavailable")
	}
	return s.mailboxRepo.Reorder(ctx, workspaceID, mailboxIDs)
}

func (s *SupportInboxService) MoveConversation(ctx context.Context, workspaceID, conversationID string, mailboxID *string, actorID string) (*model.SupportConversation, error) {
	return s.moveConversationInternal(ctx, workspaceID, conversationID, mailboxID, actorID, supportConversationMoveOptions{
		EnforceMailboxAccess: true,
		UseAccessibleLoad:    true,
		RecordTriageFeedback: true,
	})
}

func (s *SupportInboxService) moveConversationInternal(ctx context.Context, workspaceID, conversationID string, mailboxID *string, actorID string, options supportConversationMoveOptions) (*model.SupportConversation, error) {
	loadConversation := s.loadConversationAccessible
	if !options.UseAccessibleLoad {
		loadConversation = s.loadConversationUnscoped
	}

	conv, err := loadConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	targetMailboxID, mailbox, err := s.sanitizeMailboxSelection(ctx, workspaceID, mailboxID)
	if err != nil {
		return nil, err
	}
	if options.EnforceMailboxAccess {
		if err := s.requireMailboxAccess(ctx, workspaceID, targetMailboxID); err != nil {
			return nil, err
		}
	}

	ownerID, flowState, err := s.determineMailboxOwner(ctx, workspaceID, mailbox, conv.OpenedByUserID)
	if err != nil {
		return nil, err
	}

	fields := map[string]any{
		"mailbox_id":        targetMailboxID,
		"team_last_seen_at": nil,
		"opened_by_user_id": ownerID,
		"flow_state":        flowState,
	}
	if err := s.conversationRepo.UpdateFields(ctx, workspaceID, conversationID, fields); err != nil {
		return nil, err
	}

	if options.RecordTriageFeedback && s.triageService != nil && strings.TrimSpace(actorID) != "" {
		s.triageService.RecordManualMoveFeedback(ctx, workspaceID, conv, targetMailboxID, actorID)
	}
	if strings.TrimSpace(actorID) != "" && !sameMailboxID(conv.MailboxID, targetMailboxID) {
		s.createMailboxMoveSystemMessage(ctx, workspaceID, conversationID, actorID, targetMailboxID, mailbox)
	}

	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		ActorID:     actorID,
	})

	updatedConversation, err := loadConversation(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if s.triageService != nil && updatedConversation != nil {
		if hydrateErr := s.triageService.HydrateConversation(ctx, updatedConversation); hydrateErr != nil {
			slog.ErrorContext(ctx, "hydrate moved support conversation triage", "error", hydrateErr, "workspace_id", workspaceID, "conversation_id", conversationID)
		}
	}
	return updatedConversation, nil
}

func (s *SupportInboxService) createMailboxMoveSystemMessage(ctx context.Context, workspaceID, conversationID, actorID string, mailboxID *string, mailbox *model.SupportMailbox) {
	if s == nil || s.messageRepo == nil || strings.TrimSpace(actorID) == "" {
		return
	}

	displayName := "Team member"
	var avatarURL *string
	if s.userRepo != nil {
		user, err := s.userRepo.GetByID(ctx, actorID)
		if err == nil && user != nil {
			if trimmed := strings.TrimSpace(user.FullName); trimmed != "" {
				displayName = trimmed
			}
			avatarURL = user.AvatarURL
		}
	}

	mailboxName := "Shared Inbox"
	if mailbox != nil {
		mailboxName = mailbox.Name
	} else if mailboxID != nil && strings.TrimSpace(*mailboxID) != "" {
		if resolved := s.loadMailboxFromConversation(ctx, workspaceID, mailboxID); resolved != nil && strings.TrimSpace(resolved.Name) != "" {
			mailboxName = resolved.Name
		} else {
			mailboxName = "Selected Inbox"
		}
	}

	msg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderUserID:      &actorID,
		SenderDisplayName: &displayName,
		SenderAvatarURL:   avatarURL,
		Content:           fmt.Sprintf("Moved to %s by %s", mailboxName, displayName),
		IsInternal:        false,
		MessageType:       "system",
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		slog.ErrorContext(ctx, "create support mailbox move system message", "error", err, "workspace_id", workspaceID, "conversation_id", conversationID, "actor_id", actorID)
		return
	}
	s.wsPublisher.Publish(websocket.SupportMessageEvent(workspaceID, msg, actorID))
}

func (s *SupportInboxService) maybeApplyMailboxRouting(ctx context.Context, workspaceID string, explicitMailboxID *string, useWorkspaceDefault bool) (*string, *model.SupportMailbox, error) {
	return s.maybeApplyMailboxRoutingForChannel(ctx, workspaceID, explicitMailboxID, useWorkspaceDefault, "")
}

func (s *SupportInboxService) maybeApplyMailboxRoutingForChannel(ctx context.Context, workspaceID string, explicitMailboxID *string, useWorkspaceDefault bool, channel string) (*string, *model.SupportMailbox, error) {
	if explicitMailboxID != nil {
		return s.sanitizeMailboxSelection(ctx, workspaceID, explicitMailboxID)
	}
	if !useWorkspaceDefault {
		return nil, nil, nil
	}
	_, settings, err := s.GetInstallation(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	if settings != nil && settings.TriageEnabled && triageChannelEnabled(*settings, strings.ToLower(strings.TrimSpace(channel))) && settings.TriageFallbackBehavior == "shared" {
		return nil, nil, nil
	}
	return s.resolveDefaultMailbox(ctx, workspaceID, settings)
}

func (s *SupportInboxService) loadMailboxFromConversation(ctx context.Context, workspaceID string, mailboxID *string) *model.SupportMailbox {
	if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" || s.mailboxRepo == nil {
		return nil
	}
	mailbox, err := s.mailboxRepo.GetByID(ctx, workspaceID, strings.TrimSpace(*mailboxID))
	if err != nil || mailbox == nil {
		if err != nil {
			slog.ErrorContext(ctx, "load support mailbox", "error", err, "workspace_id", workspaceID, "mailbox_id", derefString(mailboxID))
		}
		return nil
	}
	return mailbox
}

func supportActorIsElevated(actor *authorization.Actor) bool {
	return actor != nil && (actor.Role == model.RoleOwner || actor.Role == model.RoleAdmin)
}
