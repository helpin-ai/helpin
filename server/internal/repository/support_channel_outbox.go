package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportChannelOutboxRepository queues replies for provider delivery. Rows
// are written by the support_channel_outbox_enqueue trigger.
type SupportChannelOutboxRepository struct {
	db *gorm.DB
}

// NewSupportChannelOutboxRepository creates the repository.
func NewSupportChannelOutboxRepository(db *gorm.DB) *SupportChannelOutboxRepository {
	return &SupportChannelOutboxRepository{db: db}
}

// ClaimDue leases due replies, oldest first, at most one per customer at a
// time: a reply waits while an earlier one to the same person is still
// pending, so a chat never receives messages out of order. PostgreSQL only.
func (r *SupportChannelOutboxRepository) ClaimDue(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]model.SupportChannelOutbox, error) {
	var rows []model.SupportChannelOutbox
	err := r.db.WithContext(ctx).Raw(`
		WITH due AS (
			SELECT o.id FROM support_channel_outbox o
			 WHERE ((o.dispatch_status = 'pending' AND o.next_attempt_at <= ?)
			     OR (o.dispatch_status = 'dispatching' AND o.lease_expires_at <= ?))
			   AND NOT EXISTS (
			       SELECT 1 FROM support_channel_outbox earlier
			        WHERE earlier.channel_identity_id = o.channel_identity_id
			          AND earlier.dispatch_status IN ('pending', 'dispatching')
			          AND (earlier.created_at, earlier.id) < (o.created_at, o.id))
			 ORDER BY o.created_at, o.id
			 LIMIT ?
			 FOR UPDATE OF o SKIP LOCKED)
		UPDATE support_channel_outbox o
		   SET dispatch_status = 'dispatching', claim_token = ?, lease_expires_at = ?, attempts = o.attempts + 1, updated_at = ?
		  FROM due WHERE o.id = due.id
		RETURNING o.*`,
		now, now, limit, uuid.NewString(), now.Add(lease), now).Scan(&rows).Error
	return rows, err
}

// update changes a row this worker still holds.
func (r *SupportChannelOutboxRepository) update(ctx context.Context, row *model.SupportChannelOutbox, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	return r.db.WithContext(ctx).Model(&model.SupportChannelOutbox{}).
		Where("id = ? AND claim_token = ? AND dispatch_status = 'dispatching'", row.ID, row.ClaimToken).Updates(updates).Error
}

// RecordPart saves progress after a part is accepted, so a retry resumes after
// it instead of sending it again. The first part's provider key identifies
// the reply.
func (r *SupportChannelOutboxRepository) RecordPart(ctx context.Context, row *model.SupportChannelOutbox, partsSent int, externalKey string) error {
	updates := map[string]any{"parts_sent": partsSent}
	if row.ExternalMessageKey == nil {
		updates["external_message_key"] = externalKey
		row.ExternalMessageKey = &externalKey
	}
	return r.update(ctx, row, updates)
}

// MarkAccepted finishes a delivered reply and records its provider key on the
// message.
func (r *SupportChannelOutboxRepository) MarkAccepted(ctx context.Context, row *model.SupportChannelOutbox) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := &SupportChannelOutboxRepository{db: tx}
		if err := repo.update(ctx, row, map[string]any{
			"dispatch_status": "accepted", "receipt_status": "sent", "claim_token": nil, "lease_expires_at": nil,
			"last_error_code": nil, "last_error": nil,
		}); err != nil {
			return err
		}
		if row.ExternalMessageKey == nil {
			return nil
		}
		return tx.Model(&model.SupportMessage{}).Where("id = ? AND external_message_key IS NULL", row.MessageID).
			Updates(map[string]any{"channel_account_id": row.ChannelAccountID, "external_message_key": *row.ExternalMessageKey}).Error
	})
}

// ScheduleRetry releases the row for another attempt at next.
func (r *SupportChannelOutboxRepository) ScheduleRetry(ctx context.Context, row *model.SupportChannelOutbox, next time.Time, code, message string) error {
	return r.update(ctx, row, map[string]any{
		"dispatch_status": "pending", "next_attempt_at": next, "claim_token": nil, "lease_expires_at": nil,
		"last_error_code": code, "last_error": truncateString(message, 2000),
	})
}

// Finish ends the row as rejected, uncertain or cancelled.
func (r *SupportChannelOutboxRepository) Finish(ctx context.Context, row *model.SupportChannelOutbox, status, code, message string) error {
	updates := map[string]any{
		"dispatch_status": status, "claim_token": nil, "lease_expires_at": nil,
		"last_error_code": code, "last_error": truncateString(message, 2000),
	}
	if status == "rejected" {
		updates["receipt_status"] = "failed"
	}
	return r.update(ctx, row, updates)
}

// ListByMessageIDs returns delivery rows for messages.
func (r *SupportChannelOutboxRepository) ListByMessageIDs(ctx context.Context, workspaceID string, messageIDs []string) ([]model.SupportChannelOutbox, error) {
	var rows []model.SupportChannelOutbox
	if len(messageIDs) == 0 {
		return rows, nil
	}
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND message_id IN ?", workspaceID, messageIDs).Find(&rows).Error
	return rows, err
}

// Retry queues a reply that was not delivered or not confirmed again, and
// returns its conversation, or "" when there was nothing to retry.
func (r *SupportChannelOutboxRepository) Retry(ctx context.Context, workspaceID, messageID string) (string, error) {
	var rows []model.SupportChannelOutbox
	err := r.db.WithContext(ctx).Model(&rows).Clauses(clause.Returning{Columns: []clause.Column{{Name: "conversation_id"}}}).
		Where("workspace_id = ? AND message_id = ? AND dispatch_status IN ?", workspaceID, messageID, []string{"rejected", "uncertain"}).
		Updates(map[string]any{
			"dispatch_status": "pending", "receipt_status": "none", "attempts": 0, "next_attempt_at": time.Now().UTC(),
			"last_error_code": nil, "last_error": nil, "updated_at": time.Now().UTC(),
		}).Error
	if err != nil || len(rows) == 0 {
		return "", err
	}
	return rows[0].ConversationID, nil
}

// RecordAuthError counts a rejected credential and, at the threshold, marks
// the account as needing a new token.
func (r *SupportChannelAccountRepository) RecordAuthError(ctx context.Context, id string, threshold int, message string) error {
	return r.db.WithContext(ctx).Exec(`
		UPDATE support_channel_accounts
		   SET auth_error_count = auth_error_count + 1,
		       status = CASE WHEN auth_error_count + 1 >= ? THEN 'reauthorization_required' ELSE status END,
		       last_error_code = 'auth', last_error_message = ?, updated_at = ?
		 WHERE id = ? AND disabled_at IS NULL`, threshold, truncateString(message, 500), time.Now().UTC(), id).Error
}

func truncateString(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
