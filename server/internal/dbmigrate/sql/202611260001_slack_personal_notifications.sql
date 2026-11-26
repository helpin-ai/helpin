ALTER TABLE slack_members
    ADD COLUMN IF NOT EXISTS notify_assignments boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS notify_mentions boolean NOT NULL DEFAULT false;
