-- 033_fix_ws_member_unique_index.sql
-- Fix: idx_ws_member_ws_user was incorrectly created as a single-column unique
-- index on user_id only. It must be a composite index on (workspace_id, user_id)
-- so that a user can belong to multiple workspaces.
-- GORM AutoMigrate will recreate it as composite after this migration runs.

DROP INDEX IF EXISTS idx_ws_member_ws_user;
