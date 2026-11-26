-- Keep previously stored choices. Newly linked members start with direct alerts.
ALTER TABLE slack_members
    ADD COLUMN IF NOT EXISTS notify_categories jsonb NOT NULL DEFAULT '{}',
    ALTER COLUMN notify_assignments SET DEFAULT true,
    ALTER COLUMN notify_mentions SET DEFAULT true;
