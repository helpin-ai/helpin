-- Migration: workspace_key_org_scope
-- Scope workspace_key uniqueness to organization instead of global.

DROP INDEX IF EXISTS idx_workspaces_workspace_key;
DROP INDEX IF EXISTS uni_workspaces_workspace_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ws_key_org
    ON workspaces (workspace_key, organization_id);
