-- Reply-time expectations (Phase 1): enforce allowed preset values at the DB
-- layer for the mailbox-level override so invalid writes (e.g. from future
-- SDK callers or broken migrations) fail fast.
-- Plan: docs/plans/2026-04-15-reply-expectations-plan.md
--
-- Workspace-level settings live inside the JSONB `settings` column on
-- support_widget_installations — there is no dedicated table to constrain
-- today, so that side is enforced by the Go service + handler layer
-- (IsValidSupportReplyTimePreset / IsValidSupportReplyTimeCustomMinutes).
-- If settings are ever split into a dedicated table, re-add the CHECK here.

ALTER TABLE support_mailboxes
    ADD COLUMN IF NOT EXISTS reply_time_preset VARCHAR(20);
ALTER TABLE support_mailboxes
    ADD COLUMN IF NOT EXISTS reply_time_custom_minutes INTEGER;

-- Only recognized presets may be stored as a mailbox override.
-- NULL is allowed and means "inherit workspace default".
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'support_mailboxes_reply_preset_chk'
    ) THEN
        ALTER TABLE support_mailboxes
            ADD CONSTRAINT support_mailboxes_reply_preset_chk
            CHECK (reply_time_preset IS NULL OR reply_time_preset IN ('few_minutes','few_hours','same_day','custom'))
            NOT VALID;
        ALTER TABLE support_mailboxes VALIDATE CONSTRAINT support_mailboxes_reply_preset_chk;
    END IF;
END$$;

-- Clamp custom minutes to the same 1..10080 (7 days) range enforced in Go.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'support_mailboxes_reply_custom_minutes_chk'
    ) THEN
        ALTER TABLE support_mailboxes
            ADD CONSTRAINT support_mailboxes_reply_custom_minutes_chk
            CHECK (reply_time_custom_minutes IS NULL OR (reply_time_custom_minutes >= 1 AND reply_time_custom_minutes <= 10080))
            NOT VALID;
        ALTER TABLE support_mailboxes VALIDATE CONSTRAINT support_mailboxes_reply_custom_minutes_chk;
    END IF;
END$$;
