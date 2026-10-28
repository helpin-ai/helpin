package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportTeammateReply is one public reply an active workspace member sent to a customer.
type SupportTeammateReply struct {
	UserID string
	// MailboxID is the inbox the conversation is in now; nil for the Shared Inbox.
	MailboxID *string
	// MovedAfter is true when the conversation moved to another inbox after
	// this reply, so the reply was written in an earlier inbox.
	MovedAfter      bool
	RepliedAt       time.Time
	AcceptedAt      *time.Time
	MemberCreatedAt time.Time
}

// JoinedAt returns when the teammate joined the workspace.
func (r SupportTeammateReply) JoinedAt() time.Time {
	if r.AcceptedAt != nil {
		return *r.AcceptedAt
	}
	return r.MemberCreatedAt
}

// ListTeammateReplies returns the public customer replies active workspace members
// sent since the given time, with the inbox each conversation is in now.
// Internal notes and system events are excluded.
func (r *SupportMessageRepository) ListTeammateReplies(ctx context.Context, workspaceID string, since time.Time) ([]SupportTeammateReply, error) {
	var replies []SupportTeammateReply
	err := r.db.WithContext(ctx).
		Table("support_messages AS m").
		Select("m.sender_user_id AS user_id, c.mailbox_id AS mailbox_id, "+
			"EXISTS (SELECT 1 FROM support_messages AS mv WHERE mv.conversation_id = m.conversation_id AND "+supportInboxMoveCondition("mv")+" AND mv.created_at > m.created_at) AS moved_after, "+
			"m.created_at AS replied_at, wm.accepted_at AS accepted_at, wm.created_at AS member_created_at").
		Joins("JOIN workspace_members AS wm ON wm.workspace_id = m.workspace_id AND wm.user_id = m.sender_user_id AND wm.status = ?", model.WorkspaceMemberStatusActive).
		Joins("LEFT JOIN support_conversations AS c ON c.id = m.conversation_id").
		Where("m.workspace_id = ? AND m.created_at >= ?", workspaceID, since).
		// Literal predicates so idx_support_messages_teammate_replies applies.
		Where("m.deleted_at IS NULL AND m.is_internal = FALSE AND m.message_type = 'reply' AND m.sender_user_id IS NOT NULL").
		Where("m.sender_type IN ?", []string{"user", "agent"}).
		Scan(&replies).Error
	if err != nil {
		return nil, fmt.Errorf("list teammate replies: %w", err)
	}
	return replies, nil
}

// MovedSinceLastTeammateReply reports whether a conversation moved to another
// inbox after a teammate last replied to the customer in it.
func (r *SupportMessageRepository) MovedSinceLastTeammateReply(ctx context.Context, workspaceID, conversationID string) (bool, error) {
	var moved bool
	err := r.db.WithContext(ctx).Raw(`SELECT EXISTS (
		SELECT 1 FROM support_messages AS mv
		WHERE mv.workspace_id = ? AND mv.conversation_id = ? AND `+supportInboxMoveCondition("mv")+`
		  AND mv.created_at > COALESCE((
			SELECT MAX(r.created_at) FROM support_messages AS r
			WHERE r.workspace_id = ? AND r.conversation_id = ? AND r.deleted_at IS NULL
			  AND r.is_internal = FALSE AND r.message_type = 'reply' AND r.sender_user_id IS NOT NULL
			  AND r.sender_type IN ('user', 'agent')
		  ), mv.created_at)
	)`, workspaceID, conversationID, workspaceID, conversationID).Scan(&moved).Error
	if err != nil {
		return false, fmt.Errorf("check inbox move since teammate reply: %w", err)
	}
	return moved, nil
}

// supportInboxMoveCondition matches the system messages written whenever a
// conversation changes inbox, by hand or by routing.
func supportInboxMoveCondition(alias string) string {
	return alias + ".deleted_at IS NULL AND " + alias + ".message_type = 'system' AND " +
		alias + ".system_event_type IN ('" + string(model.SystemEventMailboxMoved) + "', '" + string(model.SystemEventTriageRouted) + "')"
}
