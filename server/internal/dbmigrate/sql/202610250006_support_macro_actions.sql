-- Shortcuts can carry actions (status, priority, assignee, inbox, tags,
-- snooze) that run when a teammate uses them.

ALTER TABLE support_canned_responses
    ADD COLUMN IF NOT EXISTS actions jsonb NOT NULL DEFAULT '{}';
