-- Cancel all in-flight v1 epic planning flow runs that were removed in the v2 migration.
-- These runs can no longer be processed since the pm.epic_planning_v1 template definition
-- was removed from the codebase.

UPDATE flow_runs
SET status = 'cancelled',
    cancellation_reason = 'v1_template_removed',
    completed_at = NOW()
WHERE template_id = 'pm.epic_planning_v1'
  AND status IN ('running', 'awaiting_input', 'awaiting_approval', 'failed');

-- Cancel any node runs still in progress for those flows.
UPDATE flow_node_runs
SET status = 'cancelled',
    completed_at = NOW()
WHERE flow_run_id IN (
    SELECT id FROM flow_runs
    WHERE template_id = 'pm.epic_planning_v1'
      AND cancellation_reason = 'v1_template_removed'
)
AND status IN ('running', 'awaiting_input', 'awaiting_approval', 'queued');

-- Clear active_flow_run_id on epics that reference cancelled v1 runs.
UPDATE pm_epics
SET active_flow_run_id = NULL
WHERE active_flow_run_id IN (
    SELECT id FROM flow_runs
    WHERE template_id = 'pm.epic_planning_v1'
      AND cancellation_reason = 'v1_template_removed'
);
