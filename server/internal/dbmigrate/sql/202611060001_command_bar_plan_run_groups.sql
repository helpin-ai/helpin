-- Links command-bar plans to Agent Runtime run groups (COMMAND_BAR_RUN_GROUPS_ENABLED).
-- Plans without a row keep the existing event-and-sweep advancement.
CREATE TABLE IF NOT EXISTS command_bar_plan_run_groups (
    plan_id UUID PRIMARY KEY REFERENCES command_bar_plans(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL,
    group_id TEXT NOT NULL,
    completion_cursor BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_command_bar_plan_run_groups_workspace_id
    ON command_bar_plan_run_groups (workspace_id);
