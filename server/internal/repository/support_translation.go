package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrTranslationUnavailable = errors.New("translation is unavailable or no longer current")
var ErrTranslationLimit = errors.New("translation limit reached; try again later")

type SupportTranslationRepository struct{ db *gorm.DB }

func NewSupportTranslationRepository(db *gorm.DB) *SupportTranslationRepository {
	return &SupportTranslationRepository{db: db}
}

func (r *SupportTranslationRepository) Preferences(ctx context.Context, workspaceID, userID, conversationID string) (model.SupportTranslationPreference, model.SupportTranslationConversation, error) {
	p := model.SupportTranslationPreference{WorkspaceID: workspaceID, UserID: userID, ReadingLanguage: "en", AutoTranslateIncoming: true, AutoTranslateOutgoing: true}
	c := model.SupportTranslationConversation{WorkspaceID: workspaceID, ConversationID: conversationID, TranslationMode: "inherit"}
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(&p).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return p, c, err
	}
	err = r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id = ?", workspaceID, conversationID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return p, c, err
}
func (r *SupportTranslationRepository) SavePreference(ctx context.Context, p *model.SupportTranslationPreference) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"reading_language", "auto_translate_incoming", "auto_translate_outgoing", "updated_at"})}).Create(p).Error
}
func (r *SupportTranslationRepository) SaveConversation(ctx context.Context, c *model.SupportTranslationConversation) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}, {Name: "conversation_id"}}, DoUpdates: clause.AssignmentColumns([]string{"customer_language", "translation_mode", "updated_at"})}).Create(c).Error
}
func (r *SupportTranslationRepository) Get(ctx context.Context, workspaceID, conversationID, id string) (*model.SupportTranslation, error) {
	var result model.SupportTranslation
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id = ? AND id = ?", workspaceID, conversationID, id).First(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTranslationUnavailable
	}
	return &result, err
}

// Reserve deduplicates generation across readers and bounds new artifacts/retries.
// A crashed generation can be reclaimed after its bounded provider deadline.
func (r *SupportTranslationRepository) Reserve(ctx context.Context, candidate *model.SupportTranslation) (*model.SupportTranslation, bool, error) {
	owned := false
	result := candidate
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "support_translation:"+candidate.WorkspaceID).Error; err != nil {
				return err
			}
		}
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ? AND anonymized_at IS NULL", candidate.WorkspaceID, candidate.ConversationID).First(&conv).Error; err != nil {
			return ErrTranslationUnavailable
		}
		if candidate.SourceMessageID != nil {
			var source model.SupportMessage
			if err := tx.Where("workspace_id = ? AND conversation_id = ? AND id = ? AND content = ?", candidate.WorkspaceID, candidate.ConversationID, *candidate.SourceMessageID, candidate.SourceText).First(&source).Error; err != nil {
				return ErrTranslationUnavailable
			}
		}
		now := time.Now().UTC()
		var cached model.SupportTranslation
		err := tx.Where("workspace_id = ? AND cache_key = ?", candidate.WorkspaceID, candidate.CacheKey).First(&cached).Error
		if err == nil {
			result = &cached
			retryReview := candidate.ReviewStatus == "pending" && cached.ReviewStatus == "unavailable" && cached.SentMessageID == nil
			if !retryReview && cached.Status == "ready" && (cached.ExpiresAt == nil || now.Before(*cached.ExpiresAt) || cached.SentMessageID != nil) {
				return nil
			}
			if cached.Attempts >= 3 || now.Sub(cached.UpdatedAt) < time.Minute {
				return nil
			}
			nextAttempt := cached.Attempts + 1
			if err := tx.Model(&cached).Updates(map[string]any{"status": "pending", "error_code": "", "updated_at": now, "attempts": nextAttempt, "expires_at": candidate.ExpiresAt, "review_status": "not_requested", "jev_assessment_id": nil}).Error; err != nil {
				return err
			}
			cached.Status = "pending"
			cached.ExpiresAt = candidate.ExpiresAt
			cached.ReviewStatus = "not_requested"
			cached.JevAssessmentID = nil
			cached.Attempts = nextAttempt
			owned = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Remove expired unsent draft snapshots on workspace activity. Sent originals
		// remain tied to their messages and privacy deletion triggers.
		if err := tx.Where("workspace_id = ? AND sent_message_id IS NULL AND expires_at < ?", candidate.WorkspaceID, now).Delete(&model.SupportTranslation{}).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.SupportTranslation{}).Where("workspace_id = ? AND created_at >= ?", candidate.WorkspaceID, now.Add(-24*time.Hour)).Count(&count).Error; err != nil {
			return err
		}
		if count >= 1000 {
			return ErrTranslationLimit
		}
		if err := tx.Create(candidate).Error; err != nil {
			return err
		}
		owned = true
		return nil
	})
	return result, owned, err
}
func (r *SupportTranslationRepository) Finish(ctx context.Context, t *model.SupportTranslation) error {
	result := r.db.WithContext(ctx).Model(&model.SupportTranslation{}).Where("workspace_id = ? AND id = ? AND status = 'pending' AND attempts = ?", t.WorkspaceID, t.ID, t.Attempts).Updates(map[string]any{"status": t.Status, "source_language": t.SourceLanguage, "translated_text": t.TranslatedText, "provider": t.Provider, "model": t.Model, "review_status": t.ReviewStatus, "jev_assessment_id": t.JevAssessmentID, "error_code": t.ErrorCode, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrTranslationUnavailable
	}
	return nil
}

// createTranslatedMessage executes inside the same transaction as explicit email
// queue preparation. The artifact row lock prevents two sends consuming a translation.
func (r *SupportMessageRepository) createTranslatedMessage(ctx context.Context, msg *model.SupportMessage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND id = ? AND anonymized_at IS NULL", msg.WorkspaceID, msg.ConversationID).First(&conv).Error; err != nil {
			return ErrTranslationUnavailable
		}
		var artifact model.SupportTranslation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND conversation_id = ? AND id = ?", msg.WorkspaceID, msg.ConversationID, msg.TranslationID).First(&artifact).Error; err != nil {
			return ErrTranslationUnavailable
		}
		if artifact.Purpose != "outgoing_reply" || artifact.Status != "ready" || artifact.ReviewStatus == "needs_review" || artifact.SentMessageID != nil || artifact.CreatedByUserID == nil || msg.SenderUserID == nil || *artifact.CreatedByUserID != *msg.SenderUserID || artifact.TranslatedText != msg.Content || artifact.ExpiresAt == nil || !time.Now().Before(*artifact.ExpiresAt) {
			return ErrTranslationUnavailable
		}
		var config model.SupportTranslationConversation
		err := tx.Where("workspace_id = ? AND conversation_id = ?", msg.WorkspaceID, msg.ConversationID).First(&config).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if config.TranslationMode == "off" || (config.CustomerLanguage != "" && config.CustomerLanguage != artifact.TargetLanguage) {
			return ErrTranslationUnavailable
		}
		if err := r.WithTx(tx).create(ctx, msg); err != nil {
			return err
		}
		result := tx.Model(&artifact).Updates(map[string]any{"sent_message_id": msg.ID, "sent_by_user_id": *msg.SenderUserID, "sent_at": time.Now().UTC(), "expires_at": nil})
		if result.Error != nil {
			return fmt.Errorf("consume translation: %w", result.Error)
		}
		return nil
	})
}

// DetectedLanguage is a hint from the latest translated customer message, never
// a replacement for an explicit conversation language override.
func (r *SupportTranslationRepository) DetectedLanguage(ctx context.Context, workspaceID, conversationID string) (string, error) {
	var result model.SupportTranslation
	err := r.db.WithContext(ctx).Table("support_translations t").Select("t.*").Joins("JOIN support_messages m ON m.id=t.source_message_id AND m.workspace_id=t.workspace_id AND m.conversation_id=t.conversation_id").Where("t.workspace_id = ? AND t.conversation_id = ? AND t.purpose = 'message_display' AND t.status = 'ready' AND t.source_language NOT IN ('','und','mul') AND m.deleted_at IS NULL AND m.content=t.source_text", workspaceID, conversationID).Order("m.created_at DESC").First(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return result.SourceLanguage, err
}

// ForSentMessage is staff-only provenance; widget projections never call it.
func (r *SupportTranslationRepository) ForSentMessage(ctx context.Context, workspaceID, conversationID, messageID string) (*model.SupportTranslation, error) {
	var result model.SupportTranslation
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND conversation_id = ? AND sent_message_id = ?", workspaceID, conversationID, messageID).First(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &result, err
}

// LatestCustomerMessageID supplies language-detection evidence when a teammate
// sends before an incoming display translation has populated the language hint.
func (r *SupportTranslationRepository) LatestCustomerMessageID(ctx context.Context, workspaceID, conversationID string) (string, error) {
	var msg model.SupportMessage
	err := r.db.WithContext(ctx).Select("id").Where("workspace_id = ? AND conversation_id = ? AND sender_type = 'customer' AND message_type = 'reply' AND is_internal = false AND content <> ''", workspaceID, conversationID).Order("created_at DESC").First(&msg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return msg.ID, err
}
