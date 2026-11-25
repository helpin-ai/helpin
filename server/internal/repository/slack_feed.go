package repository

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// FeedThread is independent of a conversation's customer-facing delivery address.
type FeedThread struct {
	ID, AccountID, ChannelID, ConversationID, RootTS, Permalink string
	Active                                                      bool
}

func (r *SlackRepository) FeedThread(ctx context.Context, account, id string) (*FeedThread, error) {
	var row FeedThread
	err := r.db.WithContext(ctx).Table("slack_feed_threads").Where("account_id=? AND id=?", account, id).Limit(1).Scan(&row).Error
	if row.ID == "" {
		return nil, err
	}
	return &row, err
}
func (r *SlackRepository) FeedByRoot(ctx context.Context, account, channel, root string) (*FeedThread, error) {
	var row FeedThread
	err := r.db.WithContext(ctx).Table("slack_feed_threads").Where("account_id=? AND channel_id=? AND root_ts=?", account, channel, root).Limit(1).Scan(&row).Error
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

func (r *SlackRepository) FeedPreview(ctx context.Context, conv string) (string, error) {
	var body string
	err := r.db.WithContext(ctx).Raw("SELECT content FROM support_messages WHERE conversation_id=? AND sender_type='customer' AND NOT is_internal AND deleted_at IS NULL ORDER BY created_at,id LIMIT 1", conv).Scan(&body).Error
	return body, err
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
