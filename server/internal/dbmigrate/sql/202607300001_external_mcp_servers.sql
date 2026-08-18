CREATE TABLE IF NOT EXISTS external_mcp_servers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name text NOT NULL,
    server_name text NOT NULL,
    provider text NOT NULL CHECK (provider IN ('customer_io', 'custom')),
    endpoint_url text NOT NULL,
    transport text NOT NULL DEFAULT 'streamable_http' CHECK (transport = 'streamable_http'),
    auth_type text NOT NULL CHECK (auth_type IN ('oauth', 'bearer_token', 'headers', 'none')),
    status text NOT NULL CHECK (status IN ('pending_oauth', 'connected', 'reauthorization_required', 'insufficient_scope', 'remote_disabled', 'error', 'disconnected')),
    enabled boolean NOT NULL DEFAULT true,
    oauth_scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
    remote_identity jsonb NOT NULL DEFAULT '{}'::jsonb,
    authorized_by_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    access_token_expires_at timestamptz,
    last_health_checked_at timestamptz,
    last_tool_sync_at timestamptz,
    last_error_code text,
    last_error_message text,
    auth_incident_key text,
    auth_incident_notified_at timestamptz,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT external_mcp_servers_workspace_name_unique UNIQUE (workspace_id, server_name)
);

CREATE INDEX IF NOT EXISTS idx_external_mcp_servers_workspace_status
    ON external_mcp_servers(workspace_id, enabled, status);

CREATE TABLE IF NOT EXISTS external_mcp_credentials (
    server_id uuid PRIMARY KEY REFERENCES external_mcp_servers(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    encrypted_access_token text NOT NULL DEFAULT '',
    encrypted_refresh_token text NOT NULL DEFAULT '',
    encrypted_headers text NOT NULL DEFAULT '',
    encrypted_client_secret text NOT NULL DEFAULT '',
    client_id text NOT NULL DEFAULT '',
    token_type text NOT NULL DEFAULT '',
    authorization_endpoint text NOT NULL DEFAULT '',
    token_endpoint text NOT NULL DEFAULT '',
    registration_endpoint text NOT NULL DEFAULT '',
    resource_url text NOT NULL DEFAULT '',
    token_endpoint_auth_method text NOT NULL DEFAULT '',
    access_token_expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_external_mcp_credentials_workspace
    ON external_mcp_credentials(workspace_id);

CREATE TABLE IF NOT EXISTS external_mcp_tools (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    server_id uuid NOT NULL REFERENCES external_mcp_servers(id) ON DELETE CASCADE,
    remote_name text NOT NULL,
    runtime_alias text NOT NULL,
    description text NOT NULL DEFAULT '',
    input_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    access text NOT NULL DEFAULT 'write' CHECK (access IN ('read', 'write')),
    enabled boolean NOT NULL DEFAULT true,
    schema_hash text NOT NULL,
    last_seen_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT external_mcp_tools_server_remote_unique UNIQUE (server_id, remote_name),
    CONSTRAINT external_mcp_tools_workspace_alias_unique UNIQUE (workspace_id, runtime_alias)
);

CREATE INDEX IF NOT EXISTS idx_external_mcp_tools_workspace_enabled
    ON external_mcp_tools(workspace_id, enabled);

CREATE TABLE IF NOT EXISTS external_mcp_oauth_states (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    state_hash text NOT NULL UNIQUE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    server_id uuid NOT NULL REFERENCES external_mcp_servers(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    encrypted_pkce_verifier text NOT NULL,
    requested_scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
    return_path text NOT NULL DEFAULT '/settings/mcp',
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_external_mcp_oauth_states_expiry
    ON external_mcp_oauth_states(expires_at) WHERE consumed_at IS NULL;

CREATE TABLE IF NOT EXISTS agent_run_external_mcp_bindings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    agent_run_id uuid NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    runtime_run_id text NOT NULL,
    server_id uuid NOT NULL REFERENCES external_mcp_servers(id) ON DELETE CASCADE,
    server_name text NOT NULL,
    tool_aliases jsonb NOT NULL DEFAULT '[]'::jsonb,
    credential_expiry timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT agent_run_external_mcp_binding_unique UNIQUE (agent_run_id, server_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_run_external_mcp_runtime
    ON agent_run_external_mcp_bindings(workspace_id, runtime_run_id);
