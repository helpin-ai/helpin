package repository

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Called after routeSupportMessage holds the conversation lock. Concurrent
// first replies cannot both announce a join, and any reply failure rolls it back.
func (r *SupportMessageRepository) createTeammateJoined(ctx context.Context, reply *model.SupportMessage) error {
	joined := reply.TeammateJoined
	reply.TeammateJoined = nil
	if joined == nil || reply.SenderUserID == nil || reply.SenderType != "user" || reply.MessageType != "reply" || !reply.WidgetVisible() {
		return nil
	}
	var prior []string
	err := r.db.WithContext(ctx).Model(&model.SupportMessage{}).
		Where("workspace_id = ? AND conversation_id = ? AND sender_user_id = ? AND sender_type = 'user' AND is_internal = false", reply.WorkspaceID, reply.ConversationID, *reply.SenderUserID).
		Where("message_type = 'reply' OR system_event_type = ?", model.SystemEventTeammateJoined).
		Where(supportDeliveryModeSQL(r.db, "support_messages")+" <> 'email_only'").
		Limit(1).Pluck("id", &prior).Error
	if err != nil || len(prior) > 0 {
		return err
	}
	joined.WorkspaceID = reply.WorkspaceID
	joined.ConversationID = reply.ConversationID
	joined.CreatedAt = reply.CreatedAt
	reply.CreatedAt = reply.CreatedAt.Add(time.Microsecond)
	if err := r.create(ctx, joined); err != nil {
		return err
	}
	reply.TeammateJoined = joined
	return nil
}
