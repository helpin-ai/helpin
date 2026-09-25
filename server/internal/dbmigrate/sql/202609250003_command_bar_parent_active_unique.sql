-- A terminal child can be replaced when one DAG step is retried. Keep the
-- uniqueness guard for in-flight children so concurrent schedulers cannot
-- start two active successors for the same parent.
DROP INDEX IF EXISTS idx_agent_runs_command_bar_parent_run_unique;
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_runs_command_bar_parent_run_unique
  ON agent_runs (parent_run_id)
  WHERE parent_run_id IS NOT NULL
    AND (input->'trigger'->>'source') = 'command_bar'
    AND (input->'trigger'->>'trigger_type') = 'command_bar'
    AND status IN ('queued', 'running', 'paused');
