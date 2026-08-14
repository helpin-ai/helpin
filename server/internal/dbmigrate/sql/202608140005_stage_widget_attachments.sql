-- Widget visitors may upload a file before sending their first message. Keep
-- those uploads attached to the authenticated widget session instead of
-- creating an empty customer-visible conversation.

ALTER TABLE support_attachments
    ALTER COLUMN conversation_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_support_attachments_staged_session
    ON support_attachments (workspace_id, session_id, created_at)
    WHERE conversation_id IS NULL AND message_id IS NULL;
