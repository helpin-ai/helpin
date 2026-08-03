package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AIMessageProcessingRepository handles durable idempotency records for AI message processing.
type AIMessageProcessingRepository struct {
	db *gorm.DB
}

// NewAIMessageProcessingRepository creates a new AIMessageProcessingRepository.
func NewAIMessageProcessingRepository(db *gorm.DB) *AIMessageProcessingRepository {
	return &AIMessageProcessingRepository{db: db}
}

// BeginAttempt tries to claim processing ownership for a source message.
// Returns the record and true if the caller should proceed, false if already handled.
func (r *AIMessageProcessingRepository) BeginAttempt(ctx context.Context, workspaceID, sourceMessageID, conversationID string) (*model.AIMessageProcessing, bool) {
	var existing model.AIMessageProcessing
	err := r.db.WithContext(ctx).Where("source_message_id = ?", sourceMessageID).First(&existing).Error

	if err == nil {
		// Record exists — check status.
		switch existing.Status {
		case "completed":
			return &existing, false // already handled
		case "processing":
			// If updated_at is within 60s, another consumer owns it.
			if time.Since(existing.UpdatedAt) < 60*time.Second {
				return &existing, false
			}
			// Stale lock — reclaim.
			existing.Attempts++
			existing.UpdatedAt = time.Now()
			r.db.WithContext(ctx).Save(&existing)
			return &existing, true
		case "failed":
			// Retry.
			existing.Status = "processing"
			existing.Attempts++
			existing.UpdatedAt = time.Now()
			r.db.WithContext(ctx).Save(&existing)
			return &existing, true
		}
		return &existing, false
	}

	// No record — create one.
	record := model.AIMessageProcessing{
		WorkspaceID:     workspaceID,
		SourceMessageID: sourceMessageID,
		ConversationID:  conversationID,
		Status:          "processing",
		Attempts:        1,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, false
	}
	return &record, true
}

// MarkCompleted marks a processing record as completed with optional reply message ID and tokens.
func (r *AIMessageProcessingRepository) MarkCompleted(ctx context.Context, id string, replyMessageID *string, tokensUsed int) error {
	return r.db.WithContext(ctx).
		Model(&model.AIMessageProcessing{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":           "completed",
			"reply_message_id": replyMessageID,
			"tokens_used":      tokensUsed,
			"updated_at":       time.Now(),
		}).Error
}

// MarkFailed marks a processing record as failed.
func (r *AIMessageProcessingRepository) MarkFailed(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&model.AIMessageProcessing{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     "failed",
			"updated_at": time.Now(),
		}).Error
}

// MarkFailedBySourceMessageID marks a processing record as failed by the customer message ID.
func (r *AIMessageProcessingRepository) MarkFailedBySourceMessageID(ctx context.Context, sourceMessageID string) error {
	return r.db.WithContext(ctx).
		Model(&model.AIMessageProcessing{}).
		Where("source_message_id = ?", sourceMessageID).
		Updates(map[string]any{
			"status":     "failed",
			"updated_at": time.Now(),
		}).Error
}

// LatestProcessingForConversation returns the newest in-flight processing row
// for a conversation, or nil. The support chat turn currently executing
// corresponds to this row; support.send_reply/escalate settle it.
func (r *AIMessageProcessingRepository) LatestProcessingForConversation(ctx context.Context, workspaceID, conversationID string) (*model.AIMessageProcessing, error) {
	var row model.AIMessageProcessing
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ? AND status = ?", workspaceID, conversationID, "processing").
		Order("updated_at DESC").
		First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
