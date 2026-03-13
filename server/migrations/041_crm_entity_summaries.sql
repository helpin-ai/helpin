CREATE TABLE IF NOT EXISTS crm_entity_summaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID NOT NULL,
    summary_markdown TEXT NOT NULL DEFAULT '',
    highlights JSONB NOT NULL DEFAULT '[]'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending_refresh',
    computed_at TIMESTAMPTZ NULL,
    source_window_start TIMESTAMPTZ NULL,
    source_window_end TIMESTAMPTZ NULL,
    last_triggered_at TIMESTAMPTZ NULL,
    last_error TEXT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_entity_summaries_ws_entity
    ON crm_entity_summaries (workspace_id, entity_type, entity_id);

CREATE INDEX IF NOT EXISTS idx_crm_entity_summaries_status
    ON crm_entity_summaries (status);

CREATE INDEX IF NOT EXISTS idx_crm_entity_summaries_last_triggered_at
    ON crm_entity_summaries (last_triggered_at);
