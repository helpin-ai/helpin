-- A portal request and its AI dispatch are committed together. The publisher
-- retries after crashes; source_message_id also keys consumer idempotency.
CREATE TABLE IF NOT EXISTS support_portal_ai_dispatches (
    source_message_id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    conversation_id UUID NOT NULL,
    mode_at_enqueue TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_support_portal_ai_dispatch_status CHECK (status IN ('pending', 'publishing', 'published', 'failed')),
    CONSTRAINT chk_support_portal_ai_dispatch_mode CHECK (mode_at_enqueue IN ('internal_note', 'ai_first'))
);
CREATE INDEX IF NOT EXISTS idx_support_portal_ai_dispatch_due
    ON support_portal_ai_dispatches (next_attempt_at, created_at)
    WHERE status IN ('pending', 'publishing');
