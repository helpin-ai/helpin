package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportTeammateReply is one public reply an active workspace member sent to a customer.
type SupportTeammateReply struct {
	UserID          string
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
// sent since the given time. Internal notes and system events are excluded.
func (r *SupportMessageRepository) ListTeammateReplies(ctx context.Context, workspaceID string, since time.Time) ([]SupportTeammateReply, error) {
	var replies []SupportTeammateReply
	err := r.db.WithContext(ctx).
		Table("support_messages AS m").
		Select("m.sender_user_id AS user_id, m.created_at AS replied_at, wm.accepted_at AS accepted_at, wm.created_at AS member_created_at").
		Joins("JOIN workspace_members AS wm ON wm.workspace_id = m.workspace_id AND wm.user_id = m.sender_user_id AND wm.status = ?", model.WorkspaceMemberStatusActive).
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
