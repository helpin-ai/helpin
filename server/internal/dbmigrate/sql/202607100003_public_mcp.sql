-- Public Helpin MCP authorization, policy, OAuth, service identity, audit, and replay state.

CREATE TABLE IF NOT EXISTS mcp_workspace_policies (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    enabled boolean NOT NULL DEFAULT false,
    enforce_read_only boolean NOT NULL DEFAULT true,
    service_accounts_enabled boolean NOT NULL DEFAULT false,
    allowed_toolsets jsonb NOT NULL DEFAULT '["context","pm","docs","agents"]'::jsonb,
    allowed_scopes jsonb NOT NULL DEFAULT '["helpin.context.read","helpin.pm.read","helpin.docs.read","helpin.agents.read"]'::jsonb,
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_mcp_policy_toolsets_array CHECK (jsonb_typeof(allowed_toolsets) = 'array'),
    CONSTRAINT chk_mcp_policy_scopes_array CHECK (jsonb_typeof(allowed_scopes) = 'array')
);

CREATE TABLE IF NOT EXISTS mcp_client_registrations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id text NOT NULL UNIQUE,
    client_name text NOT NULL,
    client_uri text,
    redirect_uris jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_mcp_client_redirects_array CHECK (jsonb_typeof(redirect_uris) = 'array')
);

CREATE TABLE IF NOT EXISTS mcp_connections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id text NOT NULL,
    client_name text NOT NULL,
    scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
    toolsets jsonb NOT NULL DEFAULT '[]'::jsonb,
    read_only boolean NOT NULL DEFAULT true,
    status text NOT NULL DEFAULT 'active',
    token_version integer NOT NULL DEFAULT 1,
    last_used_at timestamptz,
    revoked_at timestamptz,
    revoked_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_mcp_connection_status CHECK (status IN ('active', 'revoked')),
    CONSTRAINT chk_mcp_connection_scopes_array CHECK (jsonb_typeof(scopes) = 'array'),
    CONSTRAINT chk_mcp_connection_toolsets_array CHECK (jsonb_typeof(toolsets) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_mcp_connections_workspace_status
    ON mcp_connections(workspace_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mcp_connections_user_status
    ON mcp_connections(user_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS mcp_oauth_authorization_codes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash text NOT NULL UNIQUE,
    connection_id uuid NOT NULL REFERENCES mcp_connections(id) ON DELETE CASCADE,
    client_id text NOT NULL,
    redirect_uri text NOT NULL,
    code_challenge text NOT NULL,
    code_challenge_method text NOT NULL DEFAULT 'S256',
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_mcp_code_method CHECK (code_challenge_method = 'S256')
);

CREATE INDEX IF NOT EXISTS idx_mcp_codes_expiry
    ON mcp_oauth_authorization_codes(expires_at) WHERE consumed_at IS NULL;

CREATE TABLE IF NOT EXISTS mcp_refresh_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash text NOT NULL UNIQUE,
    family_id uuid NOT NULL,
    connection_id uuid NOT NULL REFERENCES mcp_connections(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    revoked_at timestamptz,
    replaced_by_id uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mcp_refresh_family ON mcp_refresh_tokens(family_id);
CREATE INDEX IF NOT EXISTS idx_mcp_refresh_connection ON mcp_refresh_tokens(connection_id);
CREATE INDEX IF NOT EXISTS idx_mcp_refresh_expiry
    ON mcp_refresh_tokens(expires_at) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS mcp_service_principals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text,
    actor_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
    toolsets jsonb NOT NULL DEFAULT '[]'::jsonb,
    read_only boolean NOT NULL DEFAULT true,
    status text NOT NULL DEFAULT 'active',
    expires_at timestamptz,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_mcp_service_status CHECK (status IN ('active', 'revoked')),
    CONSTRAINT chk_mcp_service_scopes_array CHECK (jsonb_typeof(scopes) = 'array'),
    CONSTRAINT chk_mcp_service_toolsets_array CHECK (jsonb_typeof(toolsets) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_mcp_service_workspace_status
    ON mcp_service_principals(workspace_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS mcp_service_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    service_principal_id uuid NOT NULL REFERENCES mcp_service_principals(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    token_prefix text NOT NULL,
    expires_at timestamptz,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mcp_service_tokens_active
    ON mcp_service_tokens(service_principal_id, created_at DESC) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS mcp_audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    connection_id uuid REFERENCES mcp_connections(id) ON DELETE SET NULL,
    service_principal_id uuid REFERENCES mcp_service_principals(id) ON DELETE SET NULL,
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    client_name text NOT NULL DEFAULT '',
    event_type text NOT NULL,
    tool_name text,
    outcome text NOT NULL,
    reason_code text,
    request_hash text,
    result_hash text,
    duration_ms bigint,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_mcp_audit_outcome CHECK (outcome IN ('success', 'denied', 'error'))
);

CREATE INDEX IF NOT EXISTS idx_mcp_audit_workspace_created
    ON mcp_audit_events(workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mcp_audit_security_created
    ON mcp_audit_events(workspace_id, outcome, created_at DESC);

CREATE TABLE IF NOT EXISTS mcp_idempotency_records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    principal_key text NOT NULL,
    key text NOT NULL,
    tool_name text NOT NULL,
    request_hash text NOT NULL,
    result jsonb NOT NULL DEFAULT '{}'::jsonb,
    is_error boolean NOT NULL DEFAULT false,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_mcp_idempotency_principal_key UNIQUE (principal_key, key)
);

CREATE INDEX IF NOT EXISTS idx_mcp_idempotency_expiry ON mcp_idempotency_records(expires_at);

CREATE TABLE IF NOT EXISTS mcp_agent_run_attributions (
    run_id uuid PRIMARY KEY REFERENCES agent_runs(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    connection_id uuid REFERENCES mcp_connections(id) ON DELETE SET NULL,
    service_principal_id uuid REFERENCES mcp_service_principals(id) ON DELETE SET NULL,
    client_name text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mcp_run_attribution_workspace
    ON mcp_agent_run_attributions(workspace_id, created_at DESC);
