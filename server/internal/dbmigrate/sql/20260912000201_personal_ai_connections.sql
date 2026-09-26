CREATE TABLE IF NOT EXISTS ai_connections (
 id UUID PRIMARY KEY,
 workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 name TEXT NOT NULL,
 provider TEXT NOT NULL,
 status TEXT NOT NULL,
 account_id TEXT NOT NULL DEFAULT '',
 expires_at TIMESTAMPTZ,
 encrypted_secret BYTEA,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ai_connections_owner ON ai_connections(workspace_id,user_id);
