-- Migration: null_pm_task_assigned_agents
-- PM tasks no longer use persistent assigned agents. Workflow automation rules
-- and explicit runs are now the execution model, so clear legacy task-level
-- assignment data while leaving the column in place for a later drop.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'pm_tasks' AND column_name = 'assigned_agent_id'
    ) THEN
        UPDATE pm_tasks
        SET assigned_agent_id = NULL
        WHERE assigned_agent_id IS NOT NULL;
    END IF;
END $$;
