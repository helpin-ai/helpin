CREATE TABLE IF NOT EXISTS codex_workspace_auths (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    provider TEXT NOT NULL,
    auth_mode TEXT NOT NULL,
    auth_json_encrypted TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_codex_workspace_auth_scope
    ON codex_workspace_auths (workspace_id, provider, auth_mode);
