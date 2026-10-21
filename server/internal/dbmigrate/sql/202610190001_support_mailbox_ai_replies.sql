-- Preserve existing AI behavior while allowing inbox-specific opt-out.
ALTER TABLE support_mailboxes
    ADD COLUMN IF NOT EXISTS ai_replies_disabled boolean NOT NULL DEFAULT false;
