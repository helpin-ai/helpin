ALTER TABLE cli_executions ADD COLUMN IF NOT EXISTS busy_id text NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS cli_generations (
 id text PRIMARY KEY, execution_id text NOT NULL, request_hash text NOT NULL,
 response jsonb, input_tokens bigint NOT NULL DEFAULT 0,
 output_tokens bigint NOT NULL DEFAULT 0, cached_input_tokens bigint NOT NULL DEFAULT 0,
 reasoning_output_tokens bigint NOT NULL DEFAULT 0, created_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_cli_generations_execution_id ON cli_generations(execution_id);
