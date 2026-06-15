ALTER TABLE IF EXISTS agents
  ADD COLUMN IF NOT EXISTS active_version_id UUID;

ALTER TABLE IF EXISTS agent_runs
  ADD COLUMN IF NOT EXISTS agent_version_id UUID;

CREATE TABLE IF NOT EXISTS agent_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL,
  agent_id UUID NOT NULL,
  version_key TEXT NOT NULL,
  label TEXT NOT NULL,
  description TEXT,
  runtime_kind TEXT NOT NULL,
  provider TEXT,
  model TEXT,
  execution_config JSONB NOT NULL DEFAULT '{}'::jsonb,
  system_prompt TEXT,
  skills JSONB NOT NULL DEFAULT '[]'::jsonb,
  allowed_tools JSONB NOT NULL DEFAULT '[]'::jsonb,
  allowed_targets JSONB NOT NULL DEFAULT '[]'::jsonb,
  supported_modes JSONB NOT NULL DEFAULT '[]'::jsonb,
  default_invocation_mode TEXT NOT NULL DEFAULT 'autonomous',
  created_by UUID,
  updated_by UUID,
  deleted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_versions_workspace_agent_deleted
  ON agent_versions (workspace_id, agent_id, deleted_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_versions_agent_key_active
  ON agent_versions (agent_id, version_key)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_agents_active_version_id
  ON agents (active_version_id);

CREATE INDEX IF NOT EXISTS idx_agent_runs_agent_version_id
  ON agent_runs (agent_version_id);

INSERT INTO agent_versions (
  workspace_id,
  agent_id,
  version_key,
  label,
  description,
  runtime_kind,
  provider,
  model,
  execution_config,
  system_prompt,
  skills,
  allowed_tools,
  allowed_targets,
  supported_modes,
  default_invocation_mode,
  created_at,
  updated_at
)
SELECT
  a.workspace_id,
  a.id,
  'default',
  'Default',
  NULL,
  COALESCE(NULLIF(a.runtime_kind, ''), 'native_sdk'),
  a.provider,
  a.model,
  COALESCE(a.execution_config, '{}'::jsonb),
  a.system_prompt,
  COALESCE(a.skills, '[]'::jsonb),
  COALESCE(a.allowed_tools, '[]'::jsonb),
  COALESCE(a.allowed_targets, '["task"]'::jsonb),
  CASE
    WHEN COALESCE(NULLIF(a.default_invocation_mode, ''), 'autonomous') = 'interactive'
      THEN '["autonomous","interactive"]'::jsonb
    ELSE '["autonomous"]'::jsonb
  END,
  COALESCE(NULLIF(a.default_invocation_mode, ''), 'autonomous'),
  COALESCE(a.created_at, now()),
  COALESCE(a.updated_at, now())
FROM agents a
WHERE COALESCE(a.is_system, false) = false
  AND NOT EXISTS (
    SELECT 1
    FROM agent_versions av
    WHERE av.agent_id = a.id
      AND av.version_key = 'default'
      AND av.deleted_at IS NULL
  );

UPDATE agents a
SET active_version_id = av.id
FROM agent_versions av
WHERE av.agent_id = a.id
  AND av.version_key = 'default'
  AND av.deleted_at IS NULL
  AND COALESCE(a.is_system, false) = false
  AND a.active_version_id IS NULL;
