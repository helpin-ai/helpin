package service

import (
	"context"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ProcessSupportMentions emits mention notifications for the given user IDs.
// Mention resolution should happen before calling this (to store IDs in metadata).
func ProcessSupportMentions(ctx context.Context, notifService *NotificationService, workspaceID, conversationID, subject, messageContent, actorID string, mentionedUserIDs []string) {
	if len(mentionedUserIDs) == 0 {
		return
	}

	slog.InfoContext(ctx, "emitting support mention notifications",
		"conversation_id", conversationID,
		"mentioned_user_ids", mentionedUserIDs,
	)

	if err := notifService.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:        workspaceID,
		ActorID:            actorID,
		EventType:          "support_conversation.mentioned",
		EntityType:         "support_conversation",
		EntityID:           conversationID,
		Title:              subject,
		Body:               truncate(messageContent, 200),
		Category:           "mention",
		Priority:           "high",
		ExplicitRecipients: mentionedUserIDs,
		SkipFollowers:      true,
	}); err != nil {
		slog.ErrorContext(ctx, "emit support mention notification", "error", err, "conversation_id", conversationID)
	}
}
