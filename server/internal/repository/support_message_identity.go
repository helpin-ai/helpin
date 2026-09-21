package repository

import (
	"context"
	"errors"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// FindTeammateReplyByClientID keeps translated and original retries on the same
// send identity. Scope includes the actor; a teammate cannot reuse another's send.
func (r *SupportMessageRepository) FindTeammateReplyByClientID(ctx context.Context, workspaceID, conversationID, userID, clientID string) (*model.SupportMessage, error) {
	query := r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id = ? AND sender_user_id = ? AND sender_type = 'user' AND message_type = 'reply' AND is_internal = false", workspaceID, conversationID, userID)
	if r.db.Dialector.Name() == "postgres" {
		query = query.Where("metadata ->> 'client_message_id' = ?", clientID)
	} else {
		query = query.Where("json_extract(NULLIF(metadata, ''), '$.client_message_id') = ?", clientID)
	}
	var message model.SupportMessage
	err := query.First(&message).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &message, nil
}
