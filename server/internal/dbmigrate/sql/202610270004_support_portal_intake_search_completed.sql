-- An empty article list is a coverage gap only when search actually ran.
-- Historical submissions have no completion evidence and remain unknown.
ALTER TABLE support_portal_intake_events
    ADD COLUMN IF NOT EXISTS search_completed BOOLEAN NOT NULL DEFAULT false;
