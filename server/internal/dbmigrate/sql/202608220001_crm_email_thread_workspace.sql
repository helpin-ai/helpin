-- Thread-aware CRM email workspace state. This migration is intentionally new;
-- previously applied CRM email migrations must remain checksum-stable.

ALTER TABLE crm_email_messages
    ADD COLUMN IF NOT EXISTS rfc_message_id TEXT;

ALTER TABLE crm_email_messages
    ADD COLUMN IF NOT EXISTS in_reply_to TEXT;

ALTER TABLE crm_email_messages
    ADD COLUMN IF NOT EXISTS references_header TEXT;

CREATE TABLE IF NOT EXISTS crm_email_thread_dismissals (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    thread_id UUID NOT NULL REFERENCES crm_email_threads(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    dismissed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, thread_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_crm_email_thread_dismissals_user
    ON crm_email_thread_dismissals (workspace_id, user_id, dismissed_at DESC);

CREATE INDEX IF NOT EXISTS idx_crm_email_messages_thread_latest
    ON crm_email_messages (thread_id, sent_at DESC, id DESC);
