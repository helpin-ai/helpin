-- Additive, gated by CLI_ENABLED. IDs are text to match the CLI GORM models.
-- run_id is reserved before agent_runs insertion, so it deliberately has no FK.
CREATE TABLE IF NOT EXISTS cli_connections (
    id text PRIMARY KEY, user_id text NOT NULL, workspace_id text NOT NULL,
    client_id text NOT NULL, resource text NOT NULL, scope text NOT NULL,
    mfa_satisfied boolean NOT NULL DEFAULT false,
    revoked_at timestamptz, created_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_cli_connections_user_id ON cli_connections(user_id);
CREATE INDEX IF NOT EXISTS idx_cli_connections_workspace_id ON cli_connections(workspace_id);
CREATE TABLE IF NOT EXISTS cli_tokens (
    hash text PRIMARY KEY, connection_id text NOT NULL, kind text NOT NULL,
    redirect_uri text, code_challenge text, expires_at timestamptz NOT NULL,
    used_at timestamptz, created_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_cli_tokens_connection_id ON cli_tokens(connection_id);
CREATE INDEX IF NOT EXISTS idx_cli_tokens_expires_at ON cli_tokens(expires_at);
CREATE TABLE IF NOT EXISTS cli_executions (
    id text PRIMARY KEY, connection_id text NOT NULL, request_id text NOT NULL,
    request_hash text NOT NULL, workspace_id text NOT NULL, user_id text NOT NULL,
    run_id text NOT NULL, epoch bigint NOT NULL, local_run_id text,
    policy_hash text, snapshot jsonb, lease_expires_at timestamptz,
    revoked_at timestamptz, created_at timestamptz, updated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cli_admission ON cli_executions(connection_id, request_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cli_executions_run_id ON cli_executions(run_id);
CREATE INDEX IF NOT EXISTS idx_cli_executions_workspace_id ON cli_executions(workspace_id);
