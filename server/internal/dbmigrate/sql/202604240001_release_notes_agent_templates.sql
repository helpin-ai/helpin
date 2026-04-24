ALTER TABLE agents
  ADD COLUMN IF NOT EXISTS source_template_id uuid,
  ADD COLUMN IF NOT EXISTS source_template_key text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_agents_source_template_id
  ON agents (source_template_id);

CREATE TABLE IF NOT EXISTS agent_templates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NULL,
  key text NOT NULL,
  name text NOT NULL,
  description text NULL,
  runtime_kind text NOT NULL DEFAULT 'native_sdk',
  default_role text NOT NULL DEFAULT '',
  execution_config jsonb NOT NULL DEFAULT '{}'::jsonb,
  system_prompt text NULL,
  planning_notes text NULL,
  skills jsonb NOT NULL DEFAULT '[]'::jsonb,
  allowed_tools jsonb NOT NULL DEFAULT '[]'::jsonb,
  allowed_commands jsonb NOT NULL DEFAULT '[]'::jsonb,
  allowed_targets jsonb NOT NULL DEFAULT '[]'::jsonb,
  approval_mode text NOT NULL DEFAULT 'preset_default',
  default_invocation_mode text NOT NULL DEFAULT 'autonomous',
  monthly_token_budget integer NULL,
  is_enabled boolean NOT NULL DEFAULT true,
  created_by uuid NULL,
  updated_by uuid NULL,
  deleted_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_templates_system_key
  ON agent_templates (key)
  WHERE workspace_id IS NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_templates_workspace_key
  ON agent_templates (workspace_id, key)
  WHERE workspace_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS docs_document_keys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  key_type text NOT NULL DEFAULT 'release_notes',
  key text NOT NULL,
  document_id uuid NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT fk_docs_document_keys_document
    FOREIGN KEY (document_id)
    REFERENCES docs_documents(id)
    ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_document_keys_workspace_key_type_key
  ON docs_document_keys (workspace_id, key_type, key);
