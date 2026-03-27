ALTER TABLE workspace_agent_preset_versions
  ADD COLUMN IF NOT EXISTS supported_modes jsonb NOT NULL DEFAULT '[]'::jsonb;
