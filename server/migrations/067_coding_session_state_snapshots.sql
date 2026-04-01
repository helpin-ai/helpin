CREATE TABLE IF NOT EXISTS coding_session_state_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    run_id uuid NOT NULL,
    schema_version text NOT NULL DEFAULT 'helpin.coding_session.stream.v1',
    snapshot_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_coding_session_state_snapshots_run
    ON coding_session_state_snapshots (workspace_id, run_id);
