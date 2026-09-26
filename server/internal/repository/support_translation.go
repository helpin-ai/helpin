package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
		// Keep a send identity stable across pipeline upgrades. Completed sends
		// stay deduplicated; an unsent draft can be retried with the fixed pipeline.
		if candidate.Purpose == "outgoing_reply" {
			var previous model.SupportTranslation
			err := tx.Where("workspace_id = ? AND conversation_id = ? AND created_by_user_id = ? AND send_key = ?", candidate.WorkspaceID, candidate.ConversationID, candidate.CreatedByUserID, candidate.SendKey).First(&previous).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				if previous.SourceText != candidate.SourceText || previous.SourceHash != candidate.SourceHash || (previous.TargetLanguage != candidate.TargetLanguage && previous.PolicyRevision == candidate.PolicyRevision) {
					return ErrTranslationUnavailable
				}
				if previous.SentMessageID != nil || (previous.Status == "pending" && now.Sub(previous.UpdatedAt) < time.Minute) {
					result = &previous
					return nil
				}
				if previous.PipelineVersion != candidate.PipelineVersion || previous.PolicyRevision != candidate.PolicyRevision {
					candidate.ID = previous.ID
					candidate.CreatedAt = previous.CreatedAt
					// Increment rather than reset so completion from an older attempt
					// cannot overwrite this generation.
					candidate.Attempts = previous.Attempts + 1
					if err := tx.Save(candidate).Error; err != nil {
						return err
					}
					owned = true
					return nil
				}
			}
		}
		var cached model.SupportTranslation
		err := tx.Where("workspace_id = ? AND cache_key = ?", candidate.WorkspaceID, candidate.CacheKey).First(&cached).Error
		if err == nil {
			result = &cached
			retryReview := candidate.ReviewStatus == "pending" && cached.ReviewStatus == "unavailable" && cached.SentMessageID == nil
			if !retryReview && cached.Status == "ready" && (cached.ExpiresAt == nil || now.Before(*cached.ExpiresAt) || cached.SentMessageID != nil) {
				return nil
			}
			// Back off repeated failures without permanently poisoning a message's
			// cache entry after a temporary provider outage. Keep attempts monotonic
			// so an older worker cannot settle a newer generation.
			cooldown := time.Minute
			if cached.Attempts >= 3 {
				cooldown = 15 * time.Minute
			}
			if now.Sub(cached.UpdatedAt) < cooldown && (candidate.RetryAttempt <= cached.Attempts || cached.Attempts >= 3 || cached.Status == "pending") {
				return nil
			}
			nextAttempt := cached.Attempts + 1
			if err := tx.Model(&cached).Updates(map[string]any{"status": "pending", "error_code": "", "updated_at": now, "attempts": nextAttempt, "expires_at": candidate.ExpiresAt, "review_status": "not_requested", "jev_assessment_id": nil}).Error; err != nil {
				return err
			}
			cached.Status = "pending"
			cached.ErrorCode = ""
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
		if err := tx.Model(&model.SupportTranslation{}).Where("workspace_id = ? AND purpose = ? AND created_at >= ?", candidate.WorkspaceID, candidate.Purpose, now.Add(-24*time.Hour)).Count(&count).Error; err != nil {
			return err
		}
		limit := int64(1000)
		if candidate.Purpose == "outgoing_reply" {
			limit = 5000
		}
		if count >= limit {
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id=? AND id=? AND anonymized_at IS NULL", t.WorkspaceID, t.ConversationID).First(&conv).Error; err != nil {
			return ErrTranslationUnavailable
		}

		// Read the policy under the same lock used by toggles and arrival snapshots.
		if t.PolicyRevision > 0 && (t.Purpose == "message_display" || t.Purpose == "outgoing_reply") {
			var p model.SupportTranslationConversation
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND conversation_id = ?", t.WorkspaceID, t.ConversationID).First(&p).Error; err != nil {
				return err
			}
			if p.Revision != t.PolicyRevision || p.TranslationMode != "on" {
				t.Status = "failed"
				t.ErrorCode = "policy_changed"
				t.TranslatedText = ""
			}
		}
		result := tx.Model(&model.SupportTranslation{}).Where("workspace_id = ? AND id = ? AND status = 'pending' AND attempts = ?", t.WorkspaceID, t.ID, t.Attempts).Updates(map[string]any{"status": t.Status, "source_language": t.SourceLanguage, "translated_text": t.TranslatedText, "provider": t.Provider, "model": t.Model, "review_status": t.ReviewStatus, "jev_assessment_id": t.JevAssessmentID, "error_code": t.ErrorCode, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrTranslationUnavailable
		}
		return nil
	})
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
		var installation model.SupportWidgetInstallation
		err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("workspace_id = ?", msg.WorkspaceID).First(&installation).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		settings := model.DefaultSupportInboxSettings()
		if installation.Settings != "" {
			if err := json.Unmarshal([]byte(installation.Settings), &settings); err != nil {
				return ErrTranslationUnavailable
			}
		}
		var live model.SupportTranslationConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workspace_id = ? AND conversation_id = ?", msg.WorkspaceID, msg.ConversationID).First(&live).Error; err != nil {
			return err
		}
		if (artifact.PolicyRevision != 0 && artifact.PolicyRevision != live.Revision) || live.TranslationMode != "on" || (live.CustomerLanguage != "" && live.CustomerLanguage != artifact.TargetLanguage) {
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

	type evidence struct{ SourceLanguage, SourceText, Metadata string }
	var rows []evidence
	err := r.db.WithContext(ctx).Table("support_translations t").Select("t.source_language,t.source_text,m.metadata").Joins("JOIN support_messages m ON m.id=t.source_message_id AND m.workspace_id=t.workspace_id AND m.conversation_id=t.conversation_id").Where("t.workspace_id = ? AND t.conversation_id = ? AND t.purpose IN ('message_display','language_detection') AND t.status='ready' AND t.source_language NOT IN ('','und','mul') AND m.deleted_at IS NULL AND m.content=t.source_text", workspaceID, conversationID).Order("m.created_at DESC,m.id DESC").Limit(100).Scan(&rows).Error
	if err != nil {
		return "", err
	}
	var conv model.SupportConversation
	if err := r.db.WithContext(ctx).Where("workspace_id=? AND id=?", workspaceID, conversationID).First(&conv).Error; err != nil {
		return "", err
	}
	for _, row := range rows {
		if !model.MeaningfulSupportLanguageText(row.SourceText) {
			continue
		}

		if !primaryLanguageSender(row.Metadata, conv.CustomerEmail) {
			continue
		}
		return row.SourceLanguage, nil
	}
	return "", nil
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
	var conv model.SupportConversation
	if err := r.db.WithContext(ctx).Where("workspace_id=? AND id=?", workspaceID, conversationID).First(&conv).Error; err != nil {
		return "", err
	}
	var messages []model.SupportMessage
	err := r.db.WithContext(ctx).Select("id,metadata").Where("workspace_id=? AND conversation_id=? AND sender_type='customer' AND message_type='reply' AND is_internal=false AND deleted_at IS NULL AND content<>''", workspaceID, conversationID).Order("created_at DESC,id DESC").Limit(100).Find(&messages).Error
	if err != nil {
		return "", err
	}
	for _, msg := range messages {
		if primaryLanguageSender(msg.Metadata, conv.CustomerEmail) {
			return msg.ID, nil
		}
	}
	return "", nil
}
func primaryLanguageSender(metadata string, customerEmail *string) bool {
	var meta struct {
		Sender      string `json:"email_sender"`
		Original    string `json:"original_sender_email"`
		Participant bool   `json:"email_participant_sender"`
	}
	_ = json.Unmarshal([]byte(metadata), &meta)
	sender := meta.Sender
	if meta.Original != "" {
		sender = meta.Original
	}
	return !meta.Participant || (customerEmail != nil && strings.EqualFold(sender, *customerEmail))
}
