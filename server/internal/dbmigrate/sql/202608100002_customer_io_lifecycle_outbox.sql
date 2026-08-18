CREATE TABLE IF NOT EXISTS customer_io_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    semantic_key TEXT NOT NULL,
    workspace_id UUID NULL REFERENCES workspaces(id) ON DELETE SET NULL,
    event_name TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    recipient_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claim_token TEXT NULL,
    claimed_at TIMESTAMPTZ NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT customer_io_outbox_status_check
        CHECK (status IN ('pending', 'processing', 'delivered', 'failed')),
    CONSTRAINT customer_io_outbox_attempts_check CHECK (attempts >= 0),
    CONSTRAINT customer_io_outbox_semantic_key_key UNIQUE (semantic_key)
);

CREATE INDEX IF NOT EXISTS idx_customer_io_outbox_due_work
    ON customer_io_outbox (status, next_attempt_at, lease_expires_at)
    WHERE status IN ('pending', 'processing');
