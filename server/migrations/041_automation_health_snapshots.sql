CREATE TABLE IF NOT EXISTS automation_health_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    catalog_id TEXT NOT NULL,
    scope_type TEXT NOT NULL,
    scope_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unknown',
    last_seen_at TIMESTAMPTZ NULL,
    last_success_at TIMESTAMPTZ NULL,
    last_error_at TIMESTAMPTZ NULL,
    last_error_message TEXT NULL,
    metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_automation_health_scope
    ON automation_health_snapshots(workspace_id, catalog_id, scope_type, scope_id);
