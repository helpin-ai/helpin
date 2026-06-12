ALTER TABLE IF EXISTS workspace_agent_preset_versions
  ADD COLUMN IF NOT EXISTS allowed_targets jsonb NOT NULL DEFAULT '[]'::jsonb;
