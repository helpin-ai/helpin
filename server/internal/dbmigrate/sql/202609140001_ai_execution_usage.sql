CREATE TABLE IF NOT EXISTS ai_execution_usage (
    id text PRIMARY KEY,
    workspace_id text NOT NULL,
    idempotency_key text NOT NULL,
    run_id text NOT NULL DEFAULT '',
    feature_key text NOT NULL DEFAULT '',
    provider text NOT NULL,
    model text NOT NULL,
    input_tokens bigint NOT NULL DEFAULT 0,
    output_tokens bigint NOT NULL DEFAULT 0,
    reasoning_tokens bigint NOT NULL DEFAULT 0,
    cache_read_tokens bigint NOT NULL DEFAULT 0,
    cache_write_tokens bigint NOT NULL DEFAULT 0,
    measurement_status text NOT NULL DEFAULT '',
    paid_tools jsonb NOT NULL DEFAULT 'null',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_execution_usage_identity
    ON ai_execution_usage (workspace_id, idempotency_key);
