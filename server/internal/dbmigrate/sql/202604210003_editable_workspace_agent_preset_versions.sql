-- Migration: editable_workspace_agent_preset_versions
-- Add lifecycle metadata so workspace agent preset versions can be updated and
-- soft-deleted while preserving agent pins.

ALTER TABLE IF EXISTS workspace_agent_preset_versions
  ADD COLUMN IF NOT EXISTS updated_by uuid;

ALTER TABLE IF EXISTS workspace_agent_preset_versions
  ADD COLUMN IF NOT EXISTS last_edited_at timestamptz;

ALTER TABLE IF EXISTS workspace_agent_preset_versions
  ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_workspace_agent_preset_versions_workspace_family_deleted
  ON workspace_agent_preset_versions (workspace_id, family_key, deleted_at);
