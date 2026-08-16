ALTER TABLE IF EXISTS crm_calendar_events
    ADD COLUMN IF NOT EXISTS meeting_url TEXT,
    ADD COLUMN IF NOT EXISTS organizer_email TEXT,
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'confirmed',
    ADD COLUMN IF NOT EXISTS visibility TEXT NOT NULL DEFAULT 'default',
    ADD COLUMN IF NOT EXISTS all_day BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE IF EXISTS crm_calendar_events
    ALTER COLUMN attendees SET DEFAULT '[]'::jsonb,
    ALTER COLUMN contact_ids SET DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS idx_crm_calendar_account_external_lookup
    ON crm_calendar_events (email_account_id, external_event_id)
    WHERE external_event_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_crm_meetings_workspace_calendar_lookup
    ON crm_meetings (workspace_id, calendar_event_id)
    WHERE calendar_event_id IS NOT NULL;
