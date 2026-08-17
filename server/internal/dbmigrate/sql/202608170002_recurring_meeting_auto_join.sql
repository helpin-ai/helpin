ALTER TABLE crm_calendar_events
    ADD COLUMN IF NOT EXISTS recurring_series_id TEXT;

ALTER TABLE crm_calendar_events
    ADD COLUMN IF NOT EXISTS auto_join_override BOOLEAN;

-- Existing calendar-linked meetings were explicitly selected in the old UI.
-- Preserve that intent before workspace and series defaults begin applying.
UPDATE crm_calendar_events AS event
SET auto_join_override = TRUE
WHERE auto_join_override IS NULL
  AND EXISTS (SELECT 1 FROM crm_meetings AS meeting WHERE meeting.calendar_event_id = event.id);

CREATE INDEX IF NOT EXISTS idx_crm_calendar_events_recurring_series
    ON crm_calendar_events (workspace_id, email_account_id, recurring_series_id, start_time);

CREATE TABLE IF NOT EXISTS crm_calendar_series_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email_account_id UUID NOT NULL REFERENCES crm_email_accounts(id) ON DELETE CASCADE,
    series_external_id TEXT NOT NULL,
    auto_join BOOLEAN NOT NULL DEFAULT FALSE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_calendar_series_preference
    ON crm_calendar_series_preferences (workspace_id, email_account_id, series_external_id);

CREATE INDEX IF NOT EXISTS idx_crm_calendar_series_preferences_workspace
    ON crm_calendar_series_preferences (workspace_id);

CREATE INDEX IF NOT EXISTS idx_crm_calendar_series_preferences_account
    ON crm_calendar_series_preferences (email_account_id);
