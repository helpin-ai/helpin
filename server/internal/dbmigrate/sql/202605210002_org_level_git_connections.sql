-- Migration: org_level_git_connections
-- Make git provider connections organization-owned and allow the same repo/project
-- to be enabled in multiple workspace catalogs.

ALTER TABLE git_integrations
  ALTER COLUMN workspace_id DROP NOT NULL;

DROP INDEX IF EXISTS git_repositories_integration_external_live_uidx;

CREATE UNIQUE INDEX IF NOT EXISTS git_repositories_workspace_integration_external_live_uidx
  ON git_repositories (workspace_id, integration_id, external_id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS git_repositories_integration_external_live_idx
  ON git_repositories (integration_id, external_id)
  WHERE deleted_at IS NULL;
