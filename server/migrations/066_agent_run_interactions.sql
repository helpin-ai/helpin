CREATE TABLE IF NOT EXISTS agent_run_interactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    runtime_kind TEXT NOT NULL,
    interaction_kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    request_schema_version TEXT NOT NULL,
    response_schema_version TEXT,
    request_id TEXT,
    thread_id TEXT,
    turn_id TEXT,
    item_id TEXT,
    approval_id TEXT,
    assistant_message_sequence_no INTEGER,
    title TEXT,
    summary TEXT,
    request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_payload JSONB,
    runtime_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_run_interactions_run_created
    ON agent_run_interactions (workspace_id, run_id, created_at);

CREATE INDEX IF NOT EXISTS idx_agent_run_interactions_run_status
    ON agent_run_interactions (workspace_id, run_id, status);

CREATE INDEX IF NOT EXISTS idx_agent_run_interactions_kind_status
    ON agent_run_interactions (workspace_id, run_id, interaction_kind, status);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_run_interactions_request_id
    ON agent_run_interactions (workspace_id, run_id, request_id)
    WHERE request_id IS NOT NULL;
