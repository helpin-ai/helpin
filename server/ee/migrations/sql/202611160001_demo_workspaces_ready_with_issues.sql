-- A finished demo whose check questions did not pass is ready_with_issues.
ALTER TABLE demo_workspaces DROP CONSTRAINT IF EXISTS demo_workspaces_status_check;
ALTER TABLE demo_workspaces ADD CONSTRAINT demo_workspaces_status_check
    CHECK (status IN ('queued', 'building', 'ready', 'ready_with_issues', 'failed'));
