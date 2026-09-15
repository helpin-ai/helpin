-- Provisioning is separate from profile identity. Retries and credential rotation
-- must preserve agents intentionally reset to workspace inheritance.
ALTER TABLE ai_workspace_settings ADD COLUMN IF NOT EXISTS profiles_bootstrapped_at TIMESTAMPTZ;
