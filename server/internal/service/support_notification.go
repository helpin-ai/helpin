package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func buildSupportConversationEntitySnapshot(conv *model.SupportConversation) model.JSONB {
	if conv == nil {
		return model.JSONB{}
	}

	snapshot := model.JSONB{
		"title": conv.Subject,
		"state": conv.Status,
	}
	if conv.DisplayID > 0 {
		snapshot["identifier"] = fmt.Sprintf("#c%d", conv.DisplayID)
	}
	return snapshot
}

func buildSupportActorSnapshot(name string) model.JSONB {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return model.JSONB{}
	}
	return model.JSONB{"name": trimmed}
}

// resolveSupportActorPushTitle resolves a display name for actorID to use as
// a push notification title. Falls back to the given fallback when the
// actor cannot be resolved (deleted user, lookup failure, missing repo,
// etc) so the push is never dropped over a missing name.
func resolveSupportActorPushTitle(ctx context.Context, notifService *NotificationService, actorID, fallback string) string {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" || notifService == nil || notifService.userRepo == nil {
		return fallback
	}
	actor, err := notifService.userRepo.GetByID(ctx, actorID)
	if err != nil || actor == nil || strings.TrimSpace(actor.FullName) == "" {
		return fallback
	}
	return actor.FullName
}

// buildSupportPushData resolves the workspace slug for conv and returns the
// push Data payload (workspace_slug, conversation_id, deep_link). conv only
// carries WorkspaceID, so the slug is looked up through workspaceRepo. If
// the lookup fails, this logs a warning and returns conversation_id alone —
// the mobile client's routePushTap falls back to the app's default screen
// rather than the notification being dropped.
func buildSupportPushData(ctx context.Context, workspaceRepo *repository.WorkspaceRepository, conv *model.SupportConversation) map[string]string {
	data := map[string]string{"conversation_id": conv.ID}
	if workspaceRepo == nil {
		return data
	}
	ws, err := workspaceRepo.GetByID(ctx, conv.WorkspaceID)
	if err != nil || ws == nil {
		slog.WarnContext(ctx, "resolve workspace slug for push notification failed",
			"error", err,
			"workspace_id", conv.WorkspaceID,
			"conversation_id", conv.ID,
		)
		return data
	}
	data["workspace_slug"] = ws.Slug
	data["deep_link"] = fmt.Sprintf("helpin://w/%s/support/%s", ws.Slug, conv.ID)
	return data
}

func ProcessSupportMentions(
	ctx context.Context,
	notifService *NotificationService,
	pushSender *PushSenderService,
	conv *model.SupportConversation,
	messageContent,
	actorID string,
	mentionedUserIDs []string,
) {
	if notifService == nil || conv == nil || len(mentionedUserIDs) == 0 {
		return
	}

	slog.InfoContext(ctx, "emitting support mention notifications",
		"conversation_id", conv.ID,
		"mentioned_user_ids", mentionedUserIDs,
	)

	if err := notifService.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:        conv.WorkspaceID,
		ActorID:            actorID,
		EventType:          "support_conversation.mentioned",
		EntityType:         "support_conversation",
		EntityID:           conv.ID,
		Title:              conv.Subject,
		Body:               truncate(messageContent, 200),
		Category:           model.NotifCategorySupportMentions,
		Priority:           "high",
		EntitySnapshot:     buildSupportConversationEntitySnapshot(conv),
		ExplicitRecipients: mentionedUserIDs,
		SkipFollowers:      true,
	}); err != nil {
		slog.ErrorContext(ctx, "emit support mention notification", "error", err, "conversation_id", conv.ID)
	}

	notifService.sendSupportPush(ctx, pushSender, mentionedUserIDs, conv, "support_conversation.mentioned", "", PushNotification{
		Title: resolveSupportActorPushTitle(ctx, notifService, actorID, conv.Subject),
		Body:  truncate(messageContent, 140),
		Data:  buildSupportPushData(ctx, notifService.workspaceRepo, conv),
	})
}

func ProcessSupportCustomerReplyNotification(
	ctx context.Context,
	notifService *NotificationService,
	pushSender *PushSenderService,
	conv *model.SupportConversation,
	content,
	senderName string,
) {
	if notifService == nil || conv == nil {
		return
	}

	selection, err := selectSupportConversationRecipient(
		ctx,
		notifService.workspaceRepo,
		notifService.mailboxRepo,
		notifService.installationRepo,
		notifService.prefRepo,
		notifService.presence,
		notifService.statusOverrideRepo,
		supportRecipientSelectorInput{
			WorkspaceID:        conv.WorkspaceID,
			MailboxID:          conv.MailboxID,
			OwnerUserID:        conv.OpenedByUserID,
			EventType:          "support_conversation.customer_reply",
			Channel:            "any",
			RequirePreferences: true,
			Now:                time.Now(),
		},
	)
	if err != nil {
		slog.ErrorContext(ctx, "select support customer reply recipient", "error", err, "conversation_id", conv.ID)
		return
	}
	if selection == nil {
		return
	}
	recipientID := strings.TrimSpace(selection.UserID)

	slog.InfoContext(ctx, "emitting support customer reply notification",
		"conversation_id", conv.ID,
		"recipient_id", recipientID,
		"selection_reason", selection.Reason,
	)

	if err := notifService.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:         conv.WorkspaceID,
		EventType:           "support_conversation.customer_reply",
		EntityType:          "support_conversation",
		EntityID:            conv.ID,
		Title:               fmt.Sprintf("Customer replied in %s", conv.Subject),
		Body:                truncate(content, 200),
		Category:            model.NotifCategorySupportReplies,
		Priority:            "high",
		ActorSnapshot:       buildSupportActorSnapshot(senderName),
		EntitySnapshot:      buildSupportConversationEntitySnapshot(conv),
		ExplicitRecipients:  []string{recipientID},
		SkipFollowers:       true,
		TeamID:              selection.TeamID,
		DelayedEmailChannel: "support_reply_email",
	}); err != nil {
		slog.ErrorContext(ctx, "emit support customer reply notification", "error", err, "conversation_id", conv.ID)
	}

	pushTitle := strings.TrimSpace(senderName)
	if pushTitle == "" {
		pushTitle = "Customer replied"
	}
	notifService.sendSupportPush(ctx, pushSender, []string{recipientID}, conv, "support_conversation.customer_reply", selection.TeamID, PushNotification{
		Title: pushTitle,
		Body:  truncate(content, 140),
		Data:  buildSupportPushData(ctx, notifService.workspaceRepo, conv),
	})
}

// Push follows the in-app category preference as well as the global pause/mute controls.
func (s *NotificationService) sendSupportPush(ctx context.Context, sender *PushSenderService, users []string, conv *model.SupportConversation, eventType, teamID string, push PushNotification) {
	allowedUsers := []string{}
	for _, userID := range users {
		allowed, err := s.canReceive(ctx, userID, model.NotificationEventInput{WorkspaceID: conv.WorkspaceID, EntityType: "support_conversation", EntityID: conv.ID, EventType: eventType})
		if err != nil {
			s.logger.ErrorContext(ctx, "check support push access", "error", err)
			continue
		}
		if !allowed {
			continue
		}
		enabled, err := s.prefRepo.ShouldNotify(ctx, userID, conv.WorkspaceID, eventType, "in_app", teamID)
		if err != nil {
			s.logger.ErrorContext(ctx, "check support push preferences", "error", err)
			continue
		}
		if enabled {
			allowedUsers = append(allowedUsers, userID)
		}
	}
	sender.NotifyUsers(ctx, allowedUsers, push)
}
