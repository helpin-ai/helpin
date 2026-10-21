package repository

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SaveDetectedLanguage initializes the automatic language once. A concurrent
// manual selection or another successful detector wins over this snapshot.
func (r *SupportTranslationRepository) SaveDetectedLanguage(ctx context.Context, policy model.SupportTranslationConversation, language string, evidence []model.SupportMessage) (string, error) {
	var current model.SupportTranslationConversation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ? AND anonymized_at IS NULL", policy.WorkspaceID, policy.ConversationID).First(&conv).Error; err != nil {
			return ErrTranslationUnavailable
		}
		for _, msg := range evidence {
			var source model.SupportMessage
			if err := tx.Where("workspace_id = ? AND conversation_id = ? AND id = ? AND content = ? AND sender_type = 'customer' AND message_type = 'reply' AND is_internal = false", policy.WorkspaceID, policy.ConversationID, msg.ID, msg.Content).First(&source).Error; err != nil {
				return ErrTranslationUnavailable
			}
			if (source.OriginConversationID != nil && *source.OriginConversationID != policy.ConversationID) || !primaryLanguageSender(source.Metadata, conv.CustomerEmail) {
				return ErrTranslationUnavailable
			}
		}
		if err := tx.Model(&model.SupportTranslationConversation{}).
			Where("workspace_id = ? AND conversation_id = ? AND revision = ? AND customer_language = '' AND detected_language = ''", policy.WorkspaceID, policy.ConversationID, policy.Revision).
			Updates(map[string]any{"detected_language": language, "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		return tx.Where("workspace_id = ? AND conversation_id = ?", policy.WorkspaceID, policy.ConversationID).First(&current).Error
	})
	if current.CustomerLanguage != "" {
		return current.CustomerLanguage, err
	}
	return current.DetectedLanguage, err
}
