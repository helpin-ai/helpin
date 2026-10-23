-- Keep each member's preferred destination team separately for each support inbox.
ALTER TABLE workspace_members
    ADD COLUMN IF NOT EXISTS support_task_team_by_inbox jsonb NOT NULL DEFAULT '{}'::jsonb;
