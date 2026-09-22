package repository

import (
	"context"
	"errors"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

func (r *SupportTranslationRepository) LiveConversation(ctx context.Context, workspaceID, conversationID string, enabled bool, language string) (*model.SupportTranslationConversation, error) {
	mode := "off"
	if enabled {
		mode = "on"
	}
	initial := model.SupportTranslationConversation{WorkspaceID: workspaceID, ConversationID: conversationID, TranslationMode: mode, CustomerLanguage: language, Revision: 1}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&initial).Error; err != nil {
		return nil, err
	}
	var result model.SupportTranslationConversation
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).First(&result).Error
	return &result, err
}
func (r *SupportTranslationRepository) SetLiveConversation(ctx context.Context, workspaceID, conversationID string, enabled bool, language string) error {
	mode := "off"
	if enabled {
		mode = "on"
	}
	return r.db.WithContext(ctx).Model(&model.SupportTranslationConversation{}).Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).Updates(map[string]any{"translation_mode": mode, "customer_language": language, "revision": gorm.Expr("revision + 1"), "updated_at": time.Now().UTC()}).Error
}
func (r *SupportTranslationRepository) CachedMessages(ctx context.Context, workspaceID, conversationID, target string, ids []string) ([]model.SupportTranslation, error) {
	result := []model.SupportTranslation{}
	if len(ids) == 0 {
		return result, nil
	}
	err := r.db.WithContext(ctx).Table("support_translations t").Select("t.*").Joins("JOIN support_messages m ON m.id=t.source_message_id AND m.workspace_id=t.workspace_id AND m.content=t.source_text AND m.deleted_at IS NULL").Where("t.workspace_id = ? AND t.conversation_id = ? AND t.target_language = ? AND t.source_message_id IN ? AND t.purpose IN ('message_display','manual_display')", workspaceID, conversationID, target, ids).Order("t.updated_at ASC").Find(&result).Error

	for i := range result {
		if result[i].Status == "pending" && time.Since(result[i].UpdatedAt) > 2*time.Minute {
			result[i].Status = "failed"
			result[i].ErrorCode = "generation_timeout"
		}
	}
	return result, err
}
func (r *SupportTranslationRepository) BrowserLanguage(ctx context.Context, workspaceID, conversationID string) string {
	var result struct{ Locale string }
	// A conversation is joined to the visitor session; never infer locale from IP.
	err := r.db.WithContext(ctx).Table("support_widget_sessions s").Select("s.locale").Joins("JOIN support_conversations c ON c.workspace_id=s.workspace_id AND c.anonymous_id=s.anonymous_id").Where("c.workspace_id = ? AND c.id = ?", workspaceID, conversationID).Order("s.created_at DESC").Limit(1).Scan(&result).Error
	if err != nil {
		return ""
	}
	return result.Locale
}
func (r *SupportTranslationRepository) ClaimLiveMessage(ctx context.Context) (*model.SupportLiveMessage, error) {
	var job model.SupportLiveMessage
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Where("status = 'queued' OR (status = 'processing' AND updated_at < ?) OR (status = 'failed' AND attempts < 3 AND updated_at < ?)", time.Now().Add(-2*time.Minute), time.Now().Add(-16*time.Minute)).Order("updated_at ASC")
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		if err := q.First(&job).Error; err != nil {
			return err
		}
		job.Attempts++
		return tx.Model(&job).Updates(map[string]any{"status": "processing", "attempts": job.Attempts, "updated_at": time.Now().UTC()}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &job, err
}
func (r *SupportTranslationRepository) FinishLiveMessage(ctx context.Context, job *model.SupportLiveMessage, status string) error {
	return r.db.WithContext(ctx).Model(&model.SupportLiveMessage{}).Where("message_id = ? AND status = 'processing' AND attempts = ?", job.MessageID, job.Attempts).Updates(map[string]any{"status": status, "updated_at": time.Now().UTC()}).Error
}

// Lock only at delivery, never during model calls. Toggles and privacy erasure
// cannot cross this commit; a reclaimed worker cannot publish another reply.
func (r *SupportMessageRepository) createPendingMessage(ctx context.Context, msg *model.SupportMessage) error {
	guard := msg.PendingGuard
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ? AND anonymized_at IS NULL", msg.ConversationID, msg.WorkspaceID).First(&conv).Error; err != nil {
			return ErrTranslationUnavailable
		}
		recipient := ""
		if conv.CustomerEmail != nil {
			recipient = strings.ToLower(strings.TrimSpace(*conv.CustomerEmail))
		}
		if recipient != guard.RecipientEmail {
			return ErrTranslationUnavailable
		}
		var policy model.SupportTranslationConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND conversation_id = ? AND revision = ?", msg.WorkspaceID, msg.ConversationID, guard.Revision).First(&policy).Error; err != nil {
			return ErrTranslationUnavailable
		}
		if guard.TargetLanguage != "" && policy.CustomerLanguage == "" {
			detected, err := NewSupportTranslationRepository(tx).DetectedLanguage(ctx, msg.WorkspaceID, msg.ConversationID)
			if err != nil {
				return err
			}
			if detected != "" && detected != guard.TargetLanguage {
				return ErrTranslationUnavailable
			}
		}
		var active model.SupportPendingSend
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ? AND user_id = ? AND attempts = ? AND status = 'sending'", guard.ID, msg.WorkspaceID, guard.UserID, guard.Attempts).First(&active).Error; err != nil {
			return ErrTranslationUnavailable
		}
		copy := *msg
		copy.PendingGuard = nil
		if err := r.WithTx(tx).create(ctx, &copy); err != nil {
			return err
		}
		for _, id := range msg.PendingAttachmentIDs {
			result := tx.Model(&model.SupportAttachment{}).Where("id = ? AND workspace_id = ? AND uploaded_by_id = ? AND is_uploaded = true AND message_id IS NULL AND (conversation_id IS NULL OR conversation_id = ?)", id, msg.WorkspaceID, guard.UserID, msg.ConversationID).Updates(map[string]any{"message_id": copy.ID, "conversation_id": msg.ConversationID})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrTranslationUnavailable
			}
		}
		*msg = copy
		return nil
	})
}
