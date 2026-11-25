package repository

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// FeedThread is independent of a conversation's customer-facing delivery address.
type FeedThread struct {
	ID, AccountID, ChannelID, ConversationID, RootTS, Permalink string
	WorkspaceSlug                                               string
	Active                                                      bool
}

func (r *SlackRepository) feedThreads(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("slack_feed_threads t").Select("t.*, w.slug AS workspace_slug").
		Joins("JOIN support_channel_accounts a ON a.id=t.account_id").
		Joins("JOIN workspaces w ON w.id=a.workspace_id")
}

func (r *SlackRepository) FeedThread(ctx context.Context, account, id string) (*FeedThread, error) {
	var row FeedThread
	err := r.feedThreads(ctx).Where("t.account_id=? AND t.id=?", account, id).Limit(1).Scan(&row).Error
	if row.ID == "" {
		return nil, err
	}
	return &row, err
}
func (r *SlackRepository) FeedByRoot(ctx context.Context, account, channel, root string) (*FeedThread, error) {
	var row FeedThread
	err := r.feedThreads(ctx).Where("t.account_id=? AND t.channel_id=? AND t.root_ts=?", account, channel, root).Limit(1).Scan(&row).Error
	if row.ID == "" {
		return nil, err
	}
	return &row, err
}
func (r *SlackRepository) FeedPermalink(ctx context.Context, workspace, conv string) (string, error) {
	var link string
	err := r.db.WithContext(ctx).Raw(`SELECT t.permalink FROM slack_feed_threads t JOIN support_channel_accounts a ON a.id=t.account_id JOIN support_conversations c ON c.id=t.conversation_id JOIN slack_feed_inboxes f ON f.account_id=t.account_id AND f.channel_id=t.channel_id AND f.inbox_key=coalesce(c.mailbox_id::text,'shared') WHERE a.workspace_id=? AND t.conversation_id=? AND a.disabled_at IS NULL AND t.active AND c.anonymized_at IS NULL AND c.merged_into_id IS NULL AND t.permalink IS NOT NULL LIMIT 1`, workspace, conv).Scan(&link).Error
	return link, err
}

// FeedMoveEvent matches the latest arrival in the current inbox to its routing
// history. An earlier visit to the same inbox or a suggestion alone is insufficient.
func (r *SlackRepository) FeedMoveEvent(ctx context.Context, workspace, conversation, current string, followed []string) (*model.SupportConversationTriageEvent, error) {
	var event model.SupportConversationTriageEvent
	err := r.db.WithContext(ctx).Raw(`WITH move AS (
		SELECT workspace_id, conversation_id, created_at,
		       old_values->>'mailbox_id' AS previous, new_values->>'mailbox_id' AS destination
		FROM support_inbox_conversation_changes
		WHERE workspace_id=? AND conversation_id=? AND mailbox_scope_id=? AND audience_type='shared' AND audience_id='shared'
		  AND old_values->>'mailbox_id' IS DISTINCT FROM new_values->>'mailbox_id'
		  AND coalesce(new_values->>'mailbox_id','shared')=mailbox_scope_id
		ORDER BY source_version DESC LIMIT 1
	)
	SELECT e.event_type, e.source, e.actor_user_id
	FROM move JOIN support_conversation_triage_events e USING (workspace_id, conversation_id)
	WHERE e.from_mailbox_id::text IS NOT DISTINCT FROM move.previous
	  AND e.to_mailbox_id::text IS NOT DISTINCT FROM move.destination AND e.created_at >= move.created_at
	  AND coalesce(move.previous,'shared') IN ? AND e.event_type IN ('auto_moved','accepted','corrected')
	ORDER BY e.created_at, e.id LIMIT 1`, workspace, conversation, current, followed).Scan(&event).Error
	if event.EventType == "" || err != nil {
		return nil, err
	}
	return &event, nil
}

// Different API replicas can claim jobs for the same conversation. Serialize Slack writes,
// keeping each acknowledged receipt durable independently of the next network request.
func (r *SlackRepository) WithFeedLock(ctx context.Context, id string, fn func() error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "slack-feed:"+id).Error; err != nil {
			return err
		}
		return fn()
	})
}
func (r *SlackRepository) UpdateFeedRoot(ctx context.Context, id, root, link string) error {
	return r.db.WithContext(ctx).Table("slack_feed_threads").Where("id=?", id).Updates(map[string]any{"root_ts": root, "permalink": link}).Error
}
func (r *SlackRepository) DeactivateFeed(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM slack_feed_receipts WHERE thread_id=? AND part_key='card'", id).Error; err != nil {
			return err
		}
		return tx.Table("slack_feed_threads").Where("id=?", id).Update("active", false).Error
	})
}
func (r *SlackRepository) FeedReceipt(ctx context.Context, id, key string) (string, error) {
	var value string
	err := r.db.WithContext(ctx).Raw("SELECT slack_ts FROM slack_feed_receipts WHERE thread_id=? AND part_key=?", id, key).Scan(&value).Error
	return value, err
}
func (r *SlackRepository) SaveFeedReceipt(ctx context.Context, id, key, ts string) error {
	return r.db.WithContext(ctx).Exec("INSERT INTO slack_feed_receipts(thread_id,part_key,slack_ts) VALUES(?,?,?) ON CONFLICT(thread_id,part_key) DO UPDATE SET slack_ts=excluded.slack_ts", id, key, ts).Error
}
func (r *SlackRepository) FeedLatestReply(ctx context.Context, conv string) (string, error) {
	return latestPublicReply(r.db.WithContext(ctx), conv)
}

func latestPublicReply(db *gorm.DB, conv string) (string, error) {
	var id string
	err := db.Raw("SELECT id FROM support_messages WHERE conversation_id=? AND NOT is_internal AND message_type='reply' AND deleted_at IS NULL ORDER BY created_at DESC,id DESC LIMIT 1", conv).Scan(&id).Error
	return id, err
}

func (r *SlackRepository) FeedPreview(ctx context.Context, workspace, conv string) (*model.SupportMessage, string, error) {
	var first struct {
		model.SupportMessage
		AttachmentName string
	}
	result := r.db.WithContext(ctx).Raw(`SELECT m.*,
		COALESCE((SELECT a.file_name FROM support_attachments a WHERE a.workspace_id=m.workspace_id
			AND a.message_id=m.id AND a.is_uploaded AND a.content_id='' ORDER BY a.created_at,a.id LIMIT 1),'') AS attachment_name
		FROM support_messages m WHERE m.workspace_id=? AND m.conversation_id=? AND m.sender_type='customer'
		AND m.message_type='reply' AND NOT m.is_internal AND m.deleted_at IS NULL ORDER BY m.created_at,m.id LIMIT 1`, workspace, conv).Scan(&first)
	if result.Error != nil || result.RowsAffected == 0 {
		return nil, "", result.Error
	}
	return &first.SupportMessage, first.AttachmentName, nil
}

// FeedHandoffForCheck finds the handoff that shared this code check with this thread.
// The job or receipt prevents a late result from importing a handoff from another inbox.
func (r *SlackRepository) FeedHandoffForCheck(ctx context.Context, workspace string, thread *FeedThread, checkID string) (*model.SupportMessage, error) {
	var m model.SupportMessage
	err := r.db.WithContext(ctx).Where("workspace_id=? AND conversation_id=? AND message_type='note' AND is_internal AND sender_type='agent' AND sender_user_id IS NULL", workspace, thread.ConversationID).
		Where("metadata->'handoff'->>'code_check_id' = ?", checkID).
		Where(`EXISTS (SELECT 1 FROM support_inbound_jobs j WHERE j.message_id=support_messages.id AND j.kind='slack_inbox_feed' AND j.channel_account_id=? AND j.payload::jsonb->>'thread'=?)
			OR EXISTS (SELECT 1 FROM slack_feed_receipts r WHERE r.thread_id=? AND r.part_key=support_messages.id::text)`, thread.AccountID, thread.ID, thread.ID).
		Order("created_at DESC, id DESC").Limit(1).Find(&m).Error
	if m.ID == "" {
		return nil, err
	}
	return &m, err
}
