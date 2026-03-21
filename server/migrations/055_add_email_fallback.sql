-- Reference only: this repo applies support email fallback schema changes via
-- GORM AutoMigrate plus repository.MigrateEmailFallbackSchema() in main.go.
-- This file documents the intended PostgreSQL shape for operators and reviews.

ALTER TABLE support_conversations
    ADD COLUMN IF NOT EXISTS email_unsubscribed BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE support_messages
    ADD COLUMN IF NOT EXISTS via_channel TEXT,
    ADD COLUMN IF NOT EXISTS email_notified_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS support_email_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    conversation_id UUID NOT NULL,
    direction TEXT NOT NULL,
    message_ids TEXT[] NOT NULL DEFAULT '{}',
    from_email TEXT,
    to_email TEXT,
    subject TEXT,
    rfc_message_id TEXT,
    in_reply_to TEXT,
    postmark_message_id TEXT,
    raw_body TEXT,
    stripped_text TEXT,
    status TEXT NOT NULL DEFAULT 'sent',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sel_postmark_msg_id
    ON support_email_logs (postmark_message_id)
    WHERE postmark_message_id IS NOT NULL;
