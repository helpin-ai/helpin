-- What happens while a customer writes a portal request: articles shown,
-- articles opened, and the request submitted. One intake id covers one form,
-- so an opened article without a submission is a likely deflection.
CREATE TABLE IF NOT EXISTS support_portal_intake_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    intake_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    article_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    article_count INTEGER NOT NULL DEFAULT 0,
    conversation_id UUID,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_support_portal_intake_event_type CHECK (event_type IN ('shown', 'opened', 'submitted'))
);
CREATE INDEX IF NOT EXISTS idx_support_portal_intake_events_time
    ON support_portal_intake_events (workspace_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_support_portal_intake_events_intake
    ON support_portal_intake_events (workspace_id, intake_id);
