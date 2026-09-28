package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var ErrPortalAINoPendingCustomerMessage = errors.New("no unanswered portal customer message is available for AI")

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

// EnqueueLatestPortalCustomerMessage is used only for an explicit teammate
// handoff of an existing request. Earlier AI-processed turns are not replayed.
func EnqueueLatestPortalCustomerMessage(ctx context.Context, db *gorm.DB, workspaceID, conversationID, mode string) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return EnqueueLatestPortalCustomerMessageTx(ctx, tx, workspaceID, conversationID, mode)
	})
}

func EnqueueLatestPortalCustomerMessageTx(ctx context.Context, tx *gorm.DB, workspaceID, conversationID, mode string) error {
	var candidate struct{ ID string }
	if err := tx.Raw(`SELECT msg.id FROM support_messages AS msg
			JOIN support_conversations AS conv ON conv.id = msg.conversation_id
			WHERE conv.workspace_id = ? AND conv.id = ? AND conv.channel = 'portal'
			AND conv.portal_visible = true AND conv.status = 'open'
			AND msg.sender_type = 'customer' AND msg.message_type = 'reply'
			AND msg.id = (SELECT latest.id FROM support_messages AS latest
				WHERE latest.workspace_id = ? AND latest.conversation_id = ?
				AND latest.sender_type = 'customer' AND latest.message_type = 'reply'
				ORDER BY latest.created_at DESC, latest.id DESC LIMIT 1)
			AND NOT EXISTS (SELECT 1 FROM ai_message_processing AS p WHERE p.source_message_id = msg.id)
			AND NOT EXISTS (SELECT 1 FROM support_messages AS reply
				WHERE reply.conversation_id = conv.id AND reply.created_at > msg.created_at
				AND reply.sender_type <> 'customer' AND reply.message_type = 'reply' AND reply.is_internal = false)
			LIMIT 1`, workspaceID, conversationID, workspaceID, conversationID).Scan(&candidate).Error; err != nil {
		return err
	}
	if candidate.ID == "" {
		return ErrPortalAINoPendingCustomerMessage
	}
	result := tx.WithContext(ctx).Exec(`INSERT INTO support_portal_ai_dispatches
			(source_message_id, workspace_id, conversation_id, mode_at_enqueue) VALUES (?, ?, ?, ?)
			ON CONFLICT (source_message_id) DO NOTHING`, candidate.ID, workspaceID, conversationID, mode)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPortalAINoPendingCustomerMessage
	}
	return nil
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
