-- dbmigrate:no-transaction

-- Finds the teammates who reply to customers. Support availability uses it to
-- decide who counts as available to customers.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_support_messages_teammate_replies
    ON support_messages (workspace_id, created_at DESC)
    WHERE deleted_at IS NULL
      AND is_internal = FALSE
      AND message_type = 'reply'
      AND sender_user_id IS NOT NULL;
