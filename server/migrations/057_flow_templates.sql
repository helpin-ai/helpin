-- Flow template definitions (DB-backed, replaces hardcoded Go templates over time).

CREATE TABLE IF NOT EXISTS flow_templates (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID,
    name              TEXT NOT NULL,
    description       TEXT,
    template_slug     TEXT NOT NULL,
    version           INT NOT NULL DEFAULT 1,
    target_type       TEXT NOT NULL,
    initial_node_slug TEXT NOT NULL,
    is_builtin        BOOLEAN NOT NULL DEFAULT FALSE,
    status            TEXT NOT NULL DEFAULT 'active',
    created_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_flow_templates_slug_workspace
    ON flow_templates (template_slug, workspace_id) WHERE workspace_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_flow_templates_slug_builtin
    ON flow_templates (template_slug) WHERE workspace_id IS NULL AND is_builtin = TRUE;

CREATE INDEX IF NOT EXISTS idx_flow_templates_workspace ON flow_templates (workspace_id);
CREATE INDEX IF NOT EXISTS idx_flow_templates_status ON flow_templates (status);

CREATE TABLE IF NOT EXISTS flow_template_nodes (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id            UUID NOT NULL REFERENCES flow_templates(id) ON DELETE CASCADE,
    node_slug              TEXT NOT NULL,
    label                  TEXT NOT NULL,
    node_type              TEXT NOT NULL,
    position               INT NOT NULL DEFAULT 0,
    next_node_slug         TEXT,
    loopback_node_slug     TEXT,
    agent_input_key        TEXT,
    system_prompt          TEXT,
    allowed_tools          JSONB NOT NULL DEFAULT '[]',
    output_tag             TEXT,
    actions                JSONB NOT NULL DEFAULT '[]',
    retryable              BOOLEAN NOT NULL DEFAULT FALSE,
    command_name           TEXT,
    approve_command_name   TEXT,
    feedback_from_node     TEXT,
    additional_context_key TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_flow_template_nodes_template ON flow_template_nodes (template_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_flow_template_nodes_slug ON flow_template_nodes (template_id, node_slug);
