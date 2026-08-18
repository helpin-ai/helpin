CREATE INDEX IF NOT EXISTS idx_support_messages_conversation_created_id
    ON support_messages (conversation_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_support_conversations_workspace_updated_id
    ON support_conversations (workspace_id, updated_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_support_conversations_workspace_status_updated
    ON support_conversations (workspace_id, status, updated_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_support_conversations_workspace_mailbox_status_updated
    ON support_conversations (workspace_id, mailbox_id, status, updated_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_support_email_logs_message_ids
    ON support_email_logs USING GIN (message_ids);
