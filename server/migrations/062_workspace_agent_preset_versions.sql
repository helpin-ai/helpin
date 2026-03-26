-- Workspace-local preset versions.

ALTER TABLE agents
  ADD COLUMN IF NOT EXISTS source_preset_key text;

ALTER TABLE agents
  ADD COLUMN IF NOT EXISTS source_preset_version_key text;

UPDATE agents
SET
  preset_key = NULL,
  preset_version_key = NULL,
  source_preset_key = NULL,
  source_preset_version_key = NULL
WHERE is_system = false
  AND (
    NULLIF(preset_key, '') IS NOT NULL OR
    NULLIF(preset_version_key, '') IS NOT NULL OR
    NULLIF(source_preset_key, '') IS NOT NULL OR
    NULLIF(source_preset_version_key, '') IS NOT NULL
  );

CREATE TABLE IF NOT EXISTS workspace_agent_preset_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL,
  family_key text NOT NULL,
  version_key text NOT NULL,
  label text NOT NULL,
  description text,
  source_version_key text,
  runtime_kind text NOT NULL,
  provider text,
  model text,
  system_prompt text,
  allowed_tools jsonb NOT NULL DEFAULT '[]'::jsonb,
  approval_mode text NOT NULL DEFAULT 'preset_default',
  default_invocation_mode text NOT NULL DEFAULT 'autonomous',
  created_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_agent_preset_versions_workspace_family_version
  ON workspace_agent_preset_versions (workspace_id, family_key, version_key);

CREATE INDEX IF NOT EXISTS idx_workspace_agent_preset_versions_workspace_family
  ON workspace_agent_preset_versions (workspace_id, family_key);
