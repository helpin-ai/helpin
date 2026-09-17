package repository

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
			result := r.db.WithContext(ctx).Model(&model.AIMessageProcessing{}).Where("id = ? AND status = ? AND reply_message_id IS NULL", existing.ID, "processing").Updates(map[string]any{"attempts": existing.Attempts, "updated_at": existing.UpdatedAt})
			return &existing, result.Error == nil && result.RowsAffected == 1
		case "failed":
			// Retry.
			existing.Status = "processing"
			existing.Attempts++
			existing.UpdatedAt = time.Now()
			result := r.db.WithContext(ctx).Model(&model.AIMessageProcessing{}).Where("id = ? AND status = ? AND reply_message_id IS NULL", existing.ID, "failed").Updates(map[string]any{"status": "processing", "attempts": existing.Attempts, "updated_at": time.Now()})
			return &existing, result.Error == nil && result.RowsAffected == 1
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
		Where("id = ? AND status <> ?", id, "completed").
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
		Where("id = ? AND status <> ?", id, "completed").
		Updates(map[string]any{
			"status":     "failed",
			"updated_at": time.Now(),
		}).Error
}

// MarkFailedBySourceMessageID marks a processing record as failed by the customer message ID.
func (r *AIMessageProcessingRepository) MarkFailedBySourceMessageID(ctx context.Context, sourceMessageID string) error {
	return r.db.WithContext(ctx).
		Model(&model.AIMessageProcessing{}).
		Where("source_message_id = ? AND status <> ?", sourceMessageID, "completed").
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

// MarkDeferred parks a visitor message that arrived while a chat turn was
// executing; the pause hook drains deferred rows into one coalesced resume.
func (r *AIMessageProcessingRepository) MarkDeferred(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&model.AIMessageProcessing{}).
		Where("id = ? AND status <> ?", id, "completed").
		Update("status", "deferred").Error
}

// ListDeferredForConversation returns parked messages oldest-first.
func (r *AIMessageProcessingRepository) ListDeferredForConversation(ctx context.Context, workspaceID, conversationID string) ([]model.AIMessageProcessing, error) {
	var rows []model.AIMessageProcessing
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND conversation_id = ? AND status = ?", workspaceID, conversationID, "deferred").
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// MarkProcessing reclaims a row for an in-flight turn (used when a deferred
// batch is drained: the newest row becomes the turn being settled).
func (r *AIMessageProcessingRepository) MarkProcessing(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&model.AIMessageProcessing{}).
		Where("id = ? AND status <> ?", id, "completed").
		Update("status", "processing").Error
}

// ListDeferredOlderThan returns parked rows (sweep backstop for missed pause
// events), oldest first.
func (r *AIMessageProcessingRepository) ListDeferredOlderThan(ctx context.Context, cutoff time.Time, limit int) ([]model.AIMessageProcessing, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []model.AIMessageProcessing
	err := r.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", "deferred", cutoff).
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// IncrementAttempts bumps a row's attempt counter (used as the once-only
// nudge marker for unsettled chat turns).
func (r *AIMessageProcessingRepository) IncrementAttempts(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&model.AIMessageProcessing{}).
		Where("id = ? AND status <> ?", id, "completed").
		Update("attempts", gorm.Expr("attempts + 1")).Error
}

// CreateReply atomically saves the one reply belonging to an in-flight customer
// turn. Concurrent tool calls and retries cannot publish a second reply.
func (r *AIMessageProcessingRepository) CreateReply(ctx context.Context, processingID string, message *model.SupportMessage, runID ...string) (bool, error) {
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND workspace_id = ?", message.ConversationID, message.WorkspaceID).First(&conversation).Error; err != nil {
			return err
		}
		if len(runID) > 0 && runID[0] != "" && (conversation.AIControlVersion > 0 || conversation.AIActiveRunID != nil) && (conversation.AIActiveRunID == nil || *conversation.AIActiveRunID != runID[0]) {
			return nil
		}
		if model.SupportAIConversationBlocked(&conversation) {
			return nil
		}
		settings := model.DefaultSupportInboxSettings()
		installation, err := NewSupportInboxInstallationRepository(tx).GetByWorkspace(ctx, message.WorkspaceID)
		if err != nil {
			return err
		}
		if installation != nil {
			if err := json.Unmarshal([]byte(installation.Settings), &settings); err != nil {
				return err
			}
			if !settings.AIEnabled || (settings.AIResponseMode != "ai_first" && settings.AIResponseMode != "internal_note") {
				return nil
			}
		}
		var turn model.AIMessageProcessing
		if err := tx.Where("id = ? AND workspace_id = ? AND conversation_id = ?", processingID, message.WorkspaceID, message.ConversationID).First(&turn).Error; err != nil {
			return err
		}
		source, err := NewSupportMessageRepository(tx).GetByID(ctx, turn.SourceMessageID)
		if err != nil {
			return err
		}
		if !model.SupportAIReplyAllowed(settings, &conversation, source) {
			return nil
		}
		result := tx.Model(&model.AIMessageProcessing{}).
			Where("id = ? AND workspace_id = ? AND conversation_id = ? AND status = ? AND reply_message_id IS NULL", processingID, message.WorkspaceID, message.ConversationID, "processing").
			Updates(map[string]any{"status": "completed", "updated_at": time.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if !message.IsInternal && model.SupportAIReplyChannel(&conversation, source) == "email" {
			metadata := map[string]any{}
			if err := json.Unmarshal([]byte(message.Metadata), &metadata); err != nil && message.Metadata != "" {
				return err
			}
			metadata["delivery_mode"] = model.SupportDeliveryEmailOnly
			raw, err := json.Marshal(metadata)
			if err != nil {
				return err
			}
			message.Metadata = string(raw)
			emailChannel := "email"
			message.ViaChannel = &emailChannel
		}
		if err := NewSupportMessageRepository(tx).Create(ctx, message); err != nil {
			return err
		}
		if err := tx.Model(&model.AIMessageProcessing{}).Where("id = ?", processingID).Update("reply_message_id", message.ID).Error; err != nil {
			return err
		}
		if !message.IsInternal {
			// A fresh customer turn may have been parked while the preceding AI
			// turn resolved the conversation. Reopen it with its saved answer.
			if conversation.Status == "resolved" {
				if err := tx.Model(&model.SupportConversation{}).Where("id = ? AND workspace_id = ?", message.ConversationID, message.WorkspaceID).Updates(map[string]any{
					"status": "open", "resolved_at": nil, "ai_resolved_at": nil, "ai_resolution_type": nil,
				}).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(&model.SupportConversation{}).Where("id = ? AND workspace_id = ?", message.ConversationID, message.WorkspaceID).Updates(map[string]any{
				"ai_state": "pending", "assigned_agent_id": message.SenderAgentID, "ai_turn_count": gorm.Expr("ai_turn_count + 1"), "flow_state": model.SupportConversationFlowStateAIHandling,
			}).Error; err != nil {
				return err
			}
		}
		created = true
		return nil
	})
	return created && err == nil, err
}

// CompleteSource settles only the customer message captured by a handoff.
func (r *AIMessageProcessingRepository) CompleteSource(ctx context.Context, workspaceID, conversationID, sourceMessageID string) error {
	return r.db.WithContext(ctx).Model(&model.AIMessageProcessing{}).
		Where("workspace_id = ? AND conversation_id = ? AND source_message_id = ? AND status <> 'completed'", workspaceID, conversationID, sourceMessageID).
		Updates(map[string]any{"status": "completed", "updated_at": time.Now()}).Error
}
