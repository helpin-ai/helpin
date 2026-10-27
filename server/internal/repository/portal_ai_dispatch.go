package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type PortalAIDispatch struct {
	SourceMessageID string
	WorkspaceID     string
	ConversationID  string
	Attempts        int
}

type PortalAIDispatchRepository struct{ db *gorm.DB }

func NewPortalAIDispatchRepository(db *gorm.DB) *PortalAIDispatchRepository {
	return &PortalAIDispatchRepository{db: db}
}

// EnqueuePortalAIDispatch is called within the transaction that saved the
// customer message or published the confirmed anonymous request.
func EnqueuePortalAIDispatch(ctx context.Context, tx *gorm.DB, workspaceID, conversationID, messageID, mode string) error {
	return tx.WithContext(ctx).Exec(`INSERT INTO support_portal_ai_dispatches
		(source_message_id, workspace_id, conversation_id, mode_at_enqueue) VALUES (?, ?, ?, ?)
		ON CONFLICT (source_message_id) DO NOTHING`, messageID, workspaceID, conversationID, mode).Error
}

// ClaimDue leases a bounded batch across API replicas. Expired leases are
// retried; the consumer still deduplicates by source message ID.
func (r *PortalAIDispatchRepository) ClaimDue(ctx context.Context, now time.Time, limit int) ([]PortalAIDispatch, error) {
	var rows []PortalAIDispatch
	err := r.db.WithContext(ctx).Raw(`WITH due AS (
		SELECT source_message_id FROM support_portal_ai_dispatches
		WHERE (status = 'pending' AND next_attempt_at <= ?)
		   OR (status = 'publishing' AND lease_until <= ?)
		ORDER BY next_attempt_at, created_at LIMIT ? FOR UPDATE SKIP LOCKED
	)
	UPDATE support_portal_ai_dispatches AS d SET status = 'publishing',
		attempts = d.attempts + 1, lease_until = ?, updated_at = ?
	FROM due WHERE d.source_message_id = due.source_message_id
	RETURNING d.source_message_id, d.workspace_id, d.conversation_id, d.attempts`,
		now, now, limit, now.Add(time.Minute), now).Scan(&rows).Error
	return rows, err
}

func (r *PortalAIDispatchRepository) MarkPublished(ctx context.Context, messageID string) error {
	return r.db.WithContext(ctx).Exec(`UPDATE support_portal_ai_dispatches
		SET status = 'published', lease_until = NULL, updated_at = now()
		WHERE source_message_id = ? AND status = 'publishing'`, messageID).Error
}

func (r *PortalAIDispatchRepository) Retry(ctx context.Context, messageID string, next time.Time, failed bool) error {
	status := "pending"
	if failed {
		status = "failed"
	}
	return r.db.WithContext(ctx).Exec(`UPDATE support_portal_ai_dispatches
		SET status = ?, next_attempt_at = ?, lease_until = NULL, updated_at = now()
		WHERE source_message_id = ? AND status = 'publishing'`, status, next, messageID).Error
}

func (r *PortalAIDispatchRepository) MessageContent(ctx context.Context, row PortalAIDispatch) (string, error) {
	var message struct{ Content string }
	err := r.db.WithContext(ctx).Table("support_messages").Select("content").
		Where("id = ? AND workspace_id = ? AND conversation_id = ? AND sender_type = ? AND is_internal = false",
			row.SourceMessageID, row.WorkspaceID, row.ConversationID, "customer").Take(&message).Error
	return message.Content, err
}
