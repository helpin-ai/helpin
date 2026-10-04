-- External A2A agents: workspace-owned connections to remote agents that speak
-- the Agent2Agent protocol. Each connection is linked to one Helpin agent with
-- runtime_kind 'a2a'; deleting that agent removes the connection.
CREATE TABLE IF NOT EXISTS external_a2a_agents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    agent_id uuid NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    card_url text NOT NULL,
    interface_url text NOT NULL,
    protocol_binding text NOT NULL DEFAULT 'JSONRPC',
    protocol_version text NOT NULL DEFAULT '',
    provider_name text NOT NULL DEFAULT '',
    version text NOT NULL DEFAULT '',
    skills jsonb NOT NULL DEFAULT '[]'::jsonb,
    capabilities jsonb NOT NULL DEFAULT '{}'::jsonb,
    agent_card jsonb NOT NULL DEFAULT '{}'::jsonb,
    encrypted_token text NOT NULL DEFAULT '',
    token_hint text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'error')),
    last_checked_at timestamptz,
    last_error text NOT NULL DEFAULT '',
    allowed_team_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT external_a2a_agents_agent_unique UNIQUE (agent_id)
);

CREATE INDEX IF NOT EXISTS idx_external_a2a_agents_workspace
    ON external_a2a_agents(workspace_id, status);

-- The remote conversation (A2A contextId) continued by later runs on the same
-- Helpin task.
CREATE TABLE IF NOT EXISTS a2a_task_contexts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    task_id uuid NOT NULL REFERENCES pm_tasks(id) ON DELETE CASCADE,
    external_a2a_agent_id uuid NOT NULL REFERENCES external_a2a_agents(id) ON DELETE CASCADE,
    context_id text NOT NULL,
    last_remote_task_id text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT a2a_task_contexts_task_agent_unique UNIQUE (task_id, external_a2a_agent_id)
);

CREATE INDEX IF NOT EXISTS idx_a2a_task_contexts_workspace
    ON a2a_task_contexts(workspace_id);

-- Per-run upload links. Only the SHA-256 of the bearer token is stored.
CREATE TABLE IF NOT EXISTS a2a_run_upload_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    agent_run_id uuid NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    pm_task_id uuid NOT NULL REFERENCES pm_tasks(id) ON DELETE CASCADE,
    external_a2a_agent_id uuid NOT NULL REFERENCES external_a2a_agents(id) ON DELETE CASCADE,
    token_sha256 text NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    bytes_used bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT a2a_run_upload_tokens_hash_unique UNIQUE (token_sha256)
);

CREATE INDEX IF NOT EXISTS idx_a2a_run_upload_tokens_run
    ON a2a_run_upload_tokens(agent_run_id);

-- Idempotency claims for side effects projected from a2a.task events (agent
-- comments per message and attachments per file URL). Events can be replayed.
CREATE TABLE IF NOT EXISTS a2a_projected_items (
    agent_run_id uuid NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    item_key text NOT NULL,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_run_id, item_key)
);

-- Attachments uploaded or imported by an agent keep the agent attribution.
ALTER TABLE pm_attachments ADD COLUMN IF NOT EXISTS uploaded_by_agent_id uuid REFERENCES agents(id) ON DELETE SET NULL;
