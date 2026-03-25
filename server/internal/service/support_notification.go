package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
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

func ProcessSupportMentions(
	ctx context.Context,
	notifService *NotificationService,
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
}

func ProcessSupportCustomerReplyNotification(
	ctx context.Context,
	notifService *NotificationService,
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
		notifService.installationRepo,
		notifService.prefRepo,
		notifService.presence,
		notifService.statusOverrideRepo,
		supportRecipientSelectorInput{
			WorkspaceID:        conv.WorkspaceID,
			OwnerUserID:        conv.OpenedByUserID,
			EventType:          "support_conversation.customer_reply",
			Channel:            "in_app",
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
}
