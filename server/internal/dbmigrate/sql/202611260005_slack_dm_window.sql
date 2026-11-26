ALTER TABLE slack_members
    ADD COLUMN IF NOT EXISTS dm_window jsonb NOT NULL DEFAULT '[]';
