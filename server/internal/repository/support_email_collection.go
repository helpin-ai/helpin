package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CreateEmailCollectionStatus records each email milestone once per conversation.
func (r *SupportMessageRepository) CreateEmailCollectionStatus(ctx context.Context, workspaceID, conversationID, eventType, content string) (*model.SupportMessage, error) {
	var created *model.SupportMessage
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ?", workspaceID, conversationID).First(&conv).Error; err != nil {
			return err
		}
		// Identification may have finished while an AI answer was being published.
		if eventType == model.SystemEventVisitorEmailRequested && conv.CustomerEmail != nil && *conv.CustomerEmail != "" {
			return nil
		}
		var count int64
		if err := tx.Model(&model.SupportMessage{}).Where("workspace_id = ? AND conversation_id = ? AND system_event_type = ?", workspaceID, conversationID, eventType).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return nil
		}
		created = &model.SupportMessage{WorkspaceID: workspaceID, ConversationID: conversationID, SenderType: "system", MessageType: "system", IsInternal: true, Content: content, SystemEventType: &eventType}
		return r.WithTx(tx).Create(ctx, created)
	})
	return created, err
}
