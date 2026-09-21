package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeleteIfEmpty removes a failed new conversation and system-only messages.
// Replies and notes, including soft-deleted ones, prevent cleanup. Workspace
// scoping and the emptiness check are performed in the deletion transaction.
func (r *SupportConversationRepository) DeleteIfEmpty(ctx context.Context, workspaceID, conversationID string) (bool, error) {
	deleted := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ? AND id = ?", workspaceID, conversationID).
			Find(&conversation).Error; err != nil {
			return fmt.Errorf("load failed outbound conversation: %w", err)
		}
		if conversation.ID == "" {
			return nil
		}
		var count int64
		if err := tx.Unscoped().Model(&model.SupportMessage{}).
			Where("workspace_id = ? AND conversation_id = ? AND message_type <> ?", workspaceID, conversationID, "system").
			Count(&count).Error; err != nil {
			return fmt.Errorf("check failed outbound messages: %w", err)
		}
		if count > 0 {
			return nil
		}
		// Some older installations do not have a cascading tag foreign key.
		if err := tx.Where("conversation_id = ?", conversationID).Delete(&model.SupportConversationTag{}).Error; err != nil {
			return fmt.Errorf("delete failed outbound tags: %w", err)
		}
		if err := r.WithTx(tx).Delete(ctx, workspaceID, conversationID); err != nil {
			return err
		}
		deleted = true
		return nil
	})
	return deleted && err == nil, err
}
