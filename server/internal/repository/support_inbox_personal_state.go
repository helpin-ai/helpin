package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *SupportConversationRepository) LatestReadableCustomerMessageID(
	ctx context.Context,
	workspaceID, conversationID string,
) (string, error) {
	var message model.SupportMessage
	if err := r.db.WithContext(ctx).
		Select("id").
		Where(`workspace_id = ? AND conversation_id = ?
			AND deleted_at IS NULL AND is_internal = false AND sender_type = 'customer'
			AND message_type = 'reply' AND system_event_type IS NULL`, workspaceID, conversationID).
		Order("created_at DESC, id DESC").
		First(&message).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", fmt.Errorf("load latest customer message: %w", err)
	}
	return message.ID, nil
}

// MarkPersonalRead advances only the requesting user's cursor through a
// concrete rendered customer message. The legacy shared cursor is maintained
// in the same transaction so rollback remains coherent.
func (r *SupportConversationRepository) MarkPersonalRead(
	ctx context.Context,
	workspaceID, conversationID, userID, throughMessageID string,
) (*model.SupportConversationUserState, error) {
	var result model.SupportConversationUserState
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "workspace_id", "mailbox_id", "support_state_version").
			Where("id = ? AND workspace_id = ?", conversationID, workspaceID).
			First(&conversation).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("conversation not found")
			}
			return fmt.Errorf("lock conversation: %w", err)
		}

		var message model.SupportMessage
		if err := tx.Select("id", "created_at").
			Where(`id = ? AND workspace_id = ? AND conversation_id = ?
				AND deleted_at IS NULL AND is_internal = false AND sender_type = 'customer'
				AND message_type = 'reply' AND system_event_type IS NULL`,
				throughMessageID, workspaceID, conversationID).
			First(&message).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("read-through message not found")
			}
			return fmt.Errorf("load read-through message: %w", err)
		}

		state := model.SupportConversationUserState{
			WorkspaceID: workspaceID, ConversationID: conversationID, UserID: userID,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
			return fmt.Errorf("initialize personal read state: %w", err)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("conversation_id = ? AND user_id = ?", conversationID, userID).
			First(&state).Error; err != nil {
			return fmt.Errorf("lock personal read state: %w", err)
		}

		var remaining int64
		if err := tx.Model(&model.SupportMessage{}).
			Where(`workspace_id = ? AND conversation_id = ?
				AND deleted_at IS NULL AND is_internal = false AND sender_type = 'customer'
				AND message_type = 'reply' AND system_event_type IS NULL
				AND (created_at > ? OR (created_at = ? AND id > ?))`,
				workspaceID, conversationID, message.CreatedAt, message.CreatedAt, message.ID).
			Count(&remaining).Error; err != nil {
			return fmt.Errorf("count customer replies after read cursor: %w", err)
		}

		wasManual := state.ManuallyUnread
		advanced := state.MarkReadThrough(message.ID, message.CreatedAt, int(remaining))
		if !advanced && wasManual {
			state.Version++
		}
		state.UpdatedAt = time.Now().UTC()
		if err := tx.Model(&model.SupportConversationUserState{}).
			Where("conversation_id = ? AND user_id = ?", conversationID, userID).
			Updates(map[string]any{
				"last_read_customer_message_id": state.LastReadCustomerMessageID,
				"last_read_customer_message_at": state.LastReadCustomerMessageAt,
				"unread_customer_message_count": state.UnreadCustomerMessageCount,
				"manually_unread":               state.ManuallyUnread,
				"version":                       state.Version,
				"updated_at":                    state.UpdatedAt,
			}).Error; err != nil {
			return fmt.Errorf("update personal read state: %w", err)
		}

		if err := markLegacyInternalReadTx(tx, conversationID, r.epochExpr()); err != nil {
			return err
		}
		if advanced || wasManual {
			if err := recordPersonalStateChangeTx(tx, &conversation, &state, "read"); err != nil {
				return err
			}
		}
		result = state
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// MarkPersonalUnread creates a reminder only for the requesting user while
// retaining the legacy shared mark-unread mapping for rollback.
func (r *SupportConversationRepository) MarkPersonalUnread(
	ctx context.Context,
	workspaceID, conversationID, userID string,
) (*model.SupportConversationUserState, error) {
	var result model.SupportConversationUserState
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "workspace_id", "mailbox_id", "support_state_version").
			Where("id = ? AND workspace_id = ?", conversationID, workspaceID).
			First(&conversation).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("conversation not found")
			}
			return fmt.Errorf("lock conversation: %w", err)
		}

		state := model.SupportConversationUserState{
			WorkspaceID: workspaceID, ConversationID: conversationID, UserID: userID,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
			return fmt.Errorf("mark personal unread: %w", err)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("conversation_id = ? AND user_id = ?", conversationID, userID).First(&state).Error; err != nil {
			return fmt.Errorf("load personal unread state: %w", err)
		}
		changed := !state.ManuallyUnread
		if changed {
			state.ManuallyUnread = true
			state.Version++
			state.UpdatedAt = time.Now().UTC()
			if err := tx.Save(&state).Error; err != nil {
				return fmt.Errorf("mark personal unread: %w", err)
			}
		}
		if err := tx.Model(&model.SupportConversation{}).
			Where("id = ?", conversationID).
			UpdateColumn("team_last_seen_at", gorm.Expr(r.epochExpr())).Error; err != nil {
			return fmt.Errorf("mark legacy unread: %w", err)
		}
		if changed {
			if err := recordPersonalStateChangeTx(tx, &conversation, &state, "manual_unread"); err != nil {
				return err
			}
		}
		result = state
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ClearPersonalRead clears a manual reminder when no surviving customer
// message exists to use as a cursor. It still persists authoritative personal
// state, unlike the legacy no-op cursor update.
func (r *SupportConversationRepository) ClearPersonalRead(
	ctx context.Context,
	workspaceID, conversationID, userID string,
) (*model.SupportConversationUserState, error) {
	var result model.SupportConversationUserState
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation model.SupportConversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "workspace_id", "mailbox_id", "support_state_version").
			Where("id = ? AND workspace_id = ?", conversationID, workspaceID).
			First(&conversation).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("conversation not found")
			}
			return fmt.Errorf("lock conversation: %w", err)
		}

		state := model.SupportConversationUserState{
			WorkspaceID: workspaceID, ConversationID: conversationID, UserID: userID,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
			return fmt.Errorf("initialize personal read state: %w", err)
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("conversation_id = ? AND user_id = ?", conversationID, userID).
			First(&state).Error; err != nil {
			return fmt.Errorf("lock personal read state: %w", err)
		}
		changed := state.ManuallyUnread || state.UnreadCustomerMessageCount != 0
		if changed {
			state.ManuallyUnread = false
			state.UnreadCustomerMessageCount = 0
			state.Version++
			state.UpdatedAt = time.Now().UTC()
			if err := tx.Save(&state).Error; err != nil {
				return fmt.Errorf("clear personal read state: %w", err)
			}
		}
		if changed {
			if err := recordPersonalStateChangeTx(tx, &conversation, &state, "read"); err != nil {
				return err
			}
		}
		result = state
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func recordPersonalStateChangeTx(tx *gorm.DB, conversation *model.SupportConversation, state *model.SupportConversationUserState, reason string) error {
	if tx.Dialector.Name() != "postgres" {
		return nil
	}
	scope := "shared"
	if conversation.MailboxID != nil && *conversation.MailboxID != "" {
		scope = *conversation.MailboxID
	}
	result := tx.Exec(`
		WITH next_head AS (
			INSERT INTO support_inbox_user_scope_heads (
				workspace_id, user_id, mailbox_scope_id, personal_version, updated_at
			) VALUES (?, ?, ?, 1, NOW())
			ON CONFLICT (workspace_id, user_id, mailbox_scope_id)
			DO UPDATE SET personal_version = support_inbox_user_scope_heads.personal_version + 1, updated_at = NOW()
			RETURNING personal_version
		), logged_change AS (
			INSERT INTO support_inbox_conversation_changes (
				mutation_id, workspace_id, conversation_id, mailbox_scope_id,
				audience_type, audience_id, source_version, new_values, affected_user_ids
			)
			SELECT gen_random_uuid(), ?, ?, ?, 'user', ?, personal_version,
				jsonb_build_object('reason', ?, 'unread_count', ?, 'personal_state_version', ?),
				jsonb_build_array(?::UUID)
			FROM next_head
		)
		INSERT INTO support_realtime_outbox (
			workspace_id, target_user_id, event_type, entity_id,
			state_version, personal_state_version, payload
		) VALUES (?, ?, 'support.personal_read_changed', ?, ?, ?,
			jsonb_build_object('reason', ?, 'unread_count', ?, 'personal_state_version', ?))
	`,
		conversation.WorkspaceID, state.UserID, scope,
		conversation.WorkspaceID, conversation.ID, scope, state.UserID,
		reason, state.EffectiveUnreadCount(), state.Version, state.UserID,
		conversation.WorkspaceID, state.UserID, conversation.ID,
		conversation.SupportStateVersion, state.Version,
		reason, state.EffectiveUnreadCount(), state.Version,
	)
	if result.Error != nil {
		return fmt.Errorf("record personal support state change: %w", result.Error)
	}
	return nil
}

func markLegacyInternalReadTx(tx *gorm.DB, conversationID, epochExpr string) error {
	result := tx.Exec(fmt.Sprintf(`
		UPDATE support_conversations
		SET team_last_seen_at = (
			SELECT MAX(read_messages.created_at)
			FROM support_messages read_messages
			WHERE read_messages.conversation_id = support_conversations.id
			  AND read_messages.deleted_at IS NULL
			  AND read_messages.is_internal = false
			  AND read_messages.sender_type = 'customer'
			  AND read_messages.message_type = 'reply'
			  AND read_messages.system_event_type IS NULL
		)
		WHERE id = ?
		  AND EXISTS (
			SELECT 1 FROM support_messages sm
			WHERE sm.conversation_id = support_conversations.id
			  AND sm.deleted_at IS NULL
			  AND sm.is_internal = false
			  AND sm.sender_type = 'customer'
			  AND sm.message_type = 'reply'
			  AND sm.system_event_type IS NULL
			  AND sm.created_at > COALESCE(support_conversations.team_last_seen_at, %s)
		  )
	`, epochExpr), conversationID)
	if result.Error != nil {
		return fmt.Errorf("mark legacy internal read: %w", result.Error)
	}
	return nil
}
