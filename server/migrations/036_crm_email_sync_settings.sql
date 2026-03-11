CREATE TABLE IF NOT EXISTS crm_email_sync_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL UNIQUE REFERENCES workspaces(id),
    historical_sync_days INTEGER NOT NULL DEFAULT 90,
    filter_mode VARCHAR NOT NULL DEFAULT 'blocklist',
    filter_patterns JSONB NOT NULL DEFAULT '[]',
    internal_exclusion VARCHAR NOT NULL DEFAULT 'none',
    include_private_meetings BOOLEAN NOT NULL DEFAULT false,
    include_solo_meetings BOOLEAN NOT NULL DEFAULT false,
    record_creation_mode VARCHAR NOT NULL DEFAULT 'selective',
    blocked_record_prefixes JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
