CREATE TABLE IF NOT EXISTS product_analytics_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    semantic_key TEXT NOT NULL,
    user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    anonymous_id TEXT NULL,
    workspace_id UUID NULL REFERENCES workspaces(id) ON DELETE SET NULL,
    event_name TEXT NOT NULL,
    source TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claim_token TEXT NULL,
    claimed_at TIMESTAMPTZ NULL,
    lease_expires_at TIMESTAMPTZ NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT product_analytics_outbox_status_check
        CHECK (status IN ('pending', 'processing', 'delivered', 'failed')),
    CONSTRAINT product_analytics_outbox_attempts_check CHECK (attempts >= 0),
    CONSTRAINT product_analytics_outbox_semantic_key_key UNIQUE (semantic_key)
);

CREATE INDEX IF NOT EXISTS idx_product_analytics_outbox_due_work
    ON product_analytics_outbox (status, next_attempt_at, lease_expires_at)
    WHERE status IN ('pending', 'processing');
