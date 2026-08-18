-- Migration: mcp_enabled_by_default
-- MCP is now enabled by default for workspaces; safety still comes from the
-- platform kill switch, read-only default, per-connection OAuth consent, and RBAC.
-- Existing policy rows are explicit admin choices and are left untouched.

ALTER TABLE mcp_workspace_policies ALTER COLUMN enabled SET DEFAULT true;
