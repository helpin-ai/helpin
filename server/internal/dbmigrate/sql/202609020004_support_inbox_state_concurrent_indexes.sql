-- dbmigrate:no-transaction

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_messages_public_customer_replies
    ON support_messages (conversation_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL
      AND is_internal = FALSE
      AND sender_type = 'customer'
      AND message_type = 'reply'
      AND system_event_type IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_messages_public_last_reply
    ON support_messages (conversation_id, created_at DESC, id DESC)
    INCLUDE (sender_type, sender_display_name, content)
    WHERE deleted_at IS NULL
      AND is_internal = FALSE
      AND message_type = 'reply'
      AND system_event_type IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_messages_metadata_path_ops
    ON support_messages USING GIN (metadata jsonb_path_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_widget_sessions_workspace_anonymous_created
    ON support_widget_sessions (workspace_id, anonymous_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_widget_sessions_conversation_created
    ON support_widget_sessions (conversation_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_conversations_workspace_updated_v2
    ON support_conversations (workspace_id, updated_at DESC, id DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_conversations_workspace_status_v2
    ON support_conversations (workspace_id, status, updated_at DESC, id DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_conversation_user_states_user_conversation
    ON support_conversation_user_states (workspace_id, user_id, conversation_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_conversation_user_states_contributors
    ON support_conversation_user_states (conversation_id, user_id)
    WHERE unread_customer_message_count > 0 OR manually_unread OR relevance_mask <> 0;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_conversation_user_states_unread
    ON support_conversation_user_states (workspace_id, user_id, conversation_id)
    WHERE unread_customer_message_count > 0 OR manually_unread;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_conversation_user_states_mentions
    ON support_conversation_user_states (workspace_id, user_id, mentioned_at DESC)
    WHERE mentioned_at IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_inbox_counter_contributions_scope
    ON support_inbox_counter_contributions
       (workspace_id, mailbox_scope_id, audience_type, audience_id, bucket_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_inbox_changes_pending_scope
    ON support_inbox_conversation_changes
       (workspace_id, mailbox_scope_id, audience_type, audience_id, source_version)
    WHERE applied_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_realtime_outbox_due
    ON support_realtime_outbox (status, next_attempt_at, lease_expires_at)
    WHERE status IN ('pending', 'processing');
